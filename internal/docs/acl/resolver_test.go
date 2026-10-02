package acl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// openRepos opens a private PostgreSQL database with the production schema
// and the docs repositories over it.
func openRepos(t *testing.T) *repository.Repositories {
	t.Helper()
	return repository.New(pgtest.New(t))
}

// fakeRoles is a TenantRoleSource backed by a map of "tenant/user" -> role.
type fakeRoles map[string]types.TenantRole

func (f fakeRoles) TenantRole(_ context.Context, tenantID uint64, userID string) (types.TenantRole, bool, error) {
	role, ok := f[fmt.Sprintf("%d/%s", tenantID, userID)]
	return role, ok, nil
}

type world struct {
	t     *testing.T
	repos *repository.Repositories
	roles fakeRoles
	res   *Resolver
	space *model.Space
	pages map[string]*model.Page
	seq   int
}

func newWorld(t *testing.T, opts ...Option) *world {
	t.Helper()
	repos := openRepos(t)
	roles := fakeRoles{
		"1/owner": types.TenantRoleOwner, "1/admin": types.TenantRoleAdmin,
		"1/alice": types.TenantRoleContributor, "1/bob": types.TenantRoleContributor,
		"1/carol": types.TenantRoleContributor, "1/viewer": types.TenantRoleViewer,
		"2/alice": types.TenantRoleOwner, // alice is owner of ANOTHER tenant
	}
	w := &world{
		t: t, repos: repos, roles: roles, res: NewResolver(repos, roles, opts...), pages: map[string]*model.Page{},
	}
	w.space = &model.Space{TenantID: 1, Slug: "eng", Name: "Engineering", Visibility: model.VisibilityPrivate}
	require.NoError(t, repos.Spaces.Create(ctx(), w.space))
	return w
}

func ctx() context.Context { return context.Background() }

func (w *world) page(name string, parent string) *model.Page {
	w.seq++
	p := &model.Page{
		TenantID: 1, SpaceID: w.space.ID, Title: name, ShortID: fmt.Sprintf("p%07d", w.seq), Position: "a0",
	}
	if parent != "" {
		p.ParentID = &w.pages[parent].ID
	}
	require.NoError(w.t, w.repos.Pages.Create(ctx(), p))
	w.pages[name] = p
	return p
}

func (w *world) member(principal model.Principal, role model.SpaceRole) {
	require.NoError(w.t, w.repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: w.space.ID, TenantID: 1, PrincipalType: principal.Type, PrincipalID: principal.ID, Role: role,
	}))
	w.res.Invalidate(ctx(), 1)
}

func (w *world) restrict(page string, grants map[model.Principal]model.SpaceRole) {
	p := w.pages[page]
	require.NoError(w.t, w.repos.Access.SetRestricted(ctx(), 1, w.space.ID, p.ID, "owner"))
	for principal, role := range grants {
		require.NoError(w.t, w.repos.Access.UpsertGrant(ctx(), &model.PageGrant{
			PageID: p.ID, TenantID: 1, PrincipalType: principal.Type, PrincipalID: principal.ID, Role: role,
		}))
	}
	w.res.Invalidate(ctx(), 1)
}

func (w *world) role(user, page string) model.SpaceRole {
	id, err := w.res.Identity(ctx(), 1, user)
	require.NoError(w.t, err)
	d, err := w.res.Page(ctx(), id, w.pages[page].ID)
	require.NoError(w.t, err)
	return d.Role
}

func TestTenantLayer(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.member(model.UserPrincipal("viewer"), model.RoleAdmin) // granted admin, but tenant Viewer

	require.Equal(t, model.RoleAdmin, w.role("owner", "root"), "tenant owner is admin everywhere")
	require.Equal(t, model.RoleAdmin, w.role("admin", "root"), "tenant admin is admin everywhere")
	require.Equal(t, model.RoleNone, w.role("alice", "root"), "private space, no membership")
	require.Equal(t, model.RoleReader, w.role("viewer", "root"), "tenant Viewer is capped at reader")
	require.Equal(t, model.RoleNone, w.role("stranger", "root"), "non-members get nothing")

	// alice owns tenant 2; that must mean nothing in tenant 1.
	id, err := w.res.Identity(ctx(), 1, "alice")
	require.NoError(t, err)
	require.False(t, id.IsTenantAdmin())
}

func TestSpaceLayer(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	require.NoError(t, w.repos.Groups.Create(ctx(), &types.TenantGroup{ID: "g-backend", TenantID: 1, Name: "backend"},
		[]string{"bob"}, ""))
	w.member(model.GroupPrincipal("g-backend"), model.RoleWriter)
	w.member(model.UserPrincipal("bob"), model.RoleReader)

	require.Equal(t, model.RoleWriter, w.role("bob", "root"), "strongest of user and group memberships wins")

	// Open space: default role reaches non-members of the space.
	require.NoError(t, w.repos.Spaces.Update(ctx(), 1, w.space.ID, map[string]any{
		"visibility": model.VisibilityOpen, "default_role": model.RoleReader,
	}))
	w.res.Invalidate(ctx(), 1)
	require.Equal(t, model.RoleReader, w.role("alice", "root"))
	require.Equal(t, model.RoleWriter, w.role("bob", "root"), "explicit membership still beats the default")

	// Default group: granting "everyone" reaches every tenant member.
	def, err := w.repos.Groups.EnsureDefault(ctx(), 1, "owner")
	require.NoError(t, err)
	require.NoError(t, w.repos.Spaces.Update(ctx(), 1, w.space.ID,
		map[string]any{"visibility": model.VisibilityPrivate}))
	w.member(model.GroupPrincipal(def.ID), model.RoleWriter)
	require.Equal(t, model.RoleWriter, w.role("carol", "root"), "carol is only in the implicit default group")
	require.Equal(t, model.RoleReader, w.role("viewer", "root"), "still capped by the tenant role")

	visible, err := w.res.VisibleSpaces(ctx(), mustIdentity(t, w, "carol"))
	require.NoError(t, err)
	require.Equal(t, map[string]model.SpaceRole{w.space.ID: model.RoleWriter}, visible)
}

func TestPageRestrictionsOnlyNarrow(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.page("child", "root")
	w.page("grand", "child")
	w.page("sibling", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	w.member(model.UserPrincipal("bob"), model.RoleWriter)
	w.member(model.UserPrincipal("carol"), model.RoleAdmin)

	w.restrict("child", map[model.Principal]model.SpaceRole{model.UserPrincipal("alice"): model.RoleReader})

	require.Equal(t, model.RoleWriter, w.role("alice", "root"), "unrestricted ancestor unaffected")
	require.Equal(t, model.RoleWriter, w.role("alice", "sibling"))
	require.Equal(t, model.RoleReader, w.role("alice", "child"), "grant narrows writer to reader")
	require.Equal(t, model.RoleReader, w.role("alice", "grand"), "restriction flows down the subtree")
	require.Equal(t, model.RoleNone, w.role("bob", "child"), "no grant on a restricted page = no access")
	require.Equal(t, model.RoleNone, w.role("bob", "grand"))
	require.Equal(t, model.RoleAdmin, w.role("carol", "grand"), "space admins bypass restrictions")
	require.Equal(t, model.RoleAdmin, w.role("owner", "grand"), "tenant admins too")

	// A deeper grant cannot widen what an ancestor narrowed.
	w.restrict("grand", map[model.Principal]model.SpaceRole{model.UserPrincipal("alice"): model.RoleAdmin})
	require.Equal(t, model.RoleReader, w.role("alice", "grand"), "min over the chain, not the last grant")

	// Bob granted admin on grand but not on child still sees nothing.
	w.restrict("grand", map[model.Principal]model.SpaceRole{model.UserPrincipal("bob"): model.RoleAdmin})
	require.Equal(t, model.RoleNone, w.role("bob", "grand"))

	// A Viewer with a writer grant is still a reader.
	w.member(model.UserPrincipal("viewer"), model.RoleWriter)
	w.restrict("child", map[model.Principal]model.SpaceRole{
		model.UserPrincipal("alice"): model.RoleReader, model.UserPrincipal("viewer"): model.RoleWriter,
	})
	require.Equal(t, model.RoleReader, w.role("viewer", "child"))

	// Clearing the restriction restores inheritance for everyone.
	require.NoError(t, w.repos.Access.ClearRestricted(ctx(), 1, w.pages["child"].ID))
	require.NoError(t, w.repos.Access.ClearRestricted(ctx(), 1, w.pages["grand"].ID))
	w.res.Invalidate(ctx(), 1)
	require.Equal(t, model.RoleWriter, w.role("bob", "grand"))
}

func TestLockedPageCapsNonAdmins(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	w.member(model.UserPrincipal("carol"), model.RoleAdmin)
	require.NoError(t, w.repos.Pages.UpdateMeta(ctx(), 1, p.ID, map[string]any{"is_locked": true}))
	w.res.Invalidate(ctx(), 1)
	require.Equal(t, model.RoleReader, w.role("alice", "root"))
	require.Equal(t, model.RoleAdmin, w.role("carol", "root"))
}

func TestTrashedAndForeignPagesAreNotFound(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	_, err := w.repos.Pages.SoftDeleteSubtree(ctx(), 1, p.ID, "alice")
	require.NoError(t, err)
	w.res.Invalidate(ctx(), 1)
	id := mustIdentity(t, w, "alice")
	_, err = w.res.Page(ctx(), id, p.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
	_, err = w.res.Page(ctx(), id, "no-such-page")
	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestSubjectsSnapshotAndQueryExpansion(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.page("child", "root")
	w.page("grand", "child")
	require.NoError(t, w.repos.Groups.Create(ctx(), &types.TenantGroup{ID: "g1", TenantID: 1, Name: "g1"},
		[]string{"alice"}, ""))
	w.member(model.UserPrincipal("alice"), model.RoleWriter)

	subjects, err := w.res.PageSubjects(ctx(), 1, w.pages["root"].ID)
	require.NoError(t, err)
	require.Equal(t, []string{
		SubjectSpaceAdmin + w.space.ID, SubjectSpace + w.space.ID, SubjectTenantAdmin + "1",
	}, subjects, "unrestricted: whole space plus admins")

	w.restrict("child", map[model.Principal]model.SpaceRole{
		model.UserPrincipal("alice"): model.RoleReader, model.GroupPrincipal("g1"): model.RoleReader,
		model.UserPrincipal("bob"): model.RoleReader,
	})
	w.restrict("grand", map[model.Principal]model.SpaceRole{
		model.GroupPrincipal("g1"): model.RoleReader, model.UserPrincipal("bob"): model.RoleWriter,
	})
	subjects, err = w.res.PageSubjects(ctx(), 1, w.pages["grand"].ID)
	require.NoError(t, err)
	require.Equal(t, []string{
		SubjectGroup + "g1", SubjectSpaceAdmin + w.space.ID, SubjectTenantAdmin + "1", SubjectUser + "bob",
	}, subjects, "restricted: intersection of grants at every level; alice only granted at one level")

	q, err := w.res.QuerySubjects(ctx(), mustIdentity(t, w, "alice"))
	require.NoError(t, err)
	require.Equal(t, []string{SubjectGroup + "g1", SubjectSpace + w.space.ID, SubjectUser + "alice"}, q)

	q, err = w.res.QuerySubjects(ctx(), mustIdentity(t, w, "owner"))
	require.NoError(t, err)
	require.Contains(t, q, SubjectTenantAdmin+"1")
	require.Contains(t, q, SubjectSpaceAdmin+w.space.ID)

	q, err = w.res.QuerySubjects(ctx(), mustIdentity(t, w, "stranger"))
	require.NoError(t, err)
	require.Empty(t, q)
}

func TestCacheServesUntilInvalidated(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleReader)
	require.Equal(t, model.RoleReader, w.role("alice", "root"))

	// Change membership behind the resolver's back: the cached decision
	// stands until someone invalidates the tenant.
	require.NoError(t, w.repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: w.space.ID, TenantID: 1, PrincipalType: model.PrincipalUser, PrincipalID: "alice",
		Role: model.RoleAdmin,
	}))
	require.Equal(t, model.RoleReader, w.role("alice", "root"), "stale by design within the TTL")
	w.res.Invalidate(ctx(), 1)
	require.Equal(t, model.RoleAdmin, w.role("alice", "root"))

	// Invalidating another tenant does not touch ours.
	w.res.Invalidate(ctx(), 2)
	require.Equal(t, model.RoleAdmin, w.role("alice", "root"))
}

func TestMemoryCacheExpiryAndGeneration(t *testing.T) {
	c := NewMemoryCache(2)
	clock := time.Unix(1000, 0)
	c.now = func() time.Time { return clock }
	c.Set(ctx(), 1, "k", []byte("v"), time.Minute)
	got, ok := c.Get(ctx(), 1, "k")
	require.True(t, ok)
	require.Equal(t, "v", string(got))
	clock = clock.Add(2 * time.Minute)
	_, ok = c.Get(ctx(), 1, "k")
	require.False(t, ok, "expired")

	c.Set(ctx(), 1, "k", []byte("v"), time.Minute)
	c.InvalidateTenant(ctx(), 1)
	_, ok = c.Get(ctx(), 1, "k")
	require.False(t, ok, "generation bumped")

	c.Set(ctx(), 1, "a", []byte("1"), time.Minute)
	c.Set(ctx(), 1, "b", []byte("2"), time.Minute)
	c.Set(ctx(), 1, "c", []byte("3"), time.Minute) // over capacity: everything dropped, then c stored
	_, ok = c.Get(ctx(), 1, "a")
	require.False(t, ok)
	_, ok = c.Get(ctx(), 1, "c")
	require.True(t, ok)
}

func TestRedisCache(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	c := NewRedisCache(rdb, "")
	c.Set(ctx(), 7, "pg:u:p", []byte(`{"role":"reader"}`), time.Minute)
	got, ok := c.Get(ctx(), 7, "pg:u:p")
	require.True(t, ok)
	require.JSONEq(t, `{"role":"reader"}`, string(got))

	// A second resolver instance sharing Redis sees the same entry...
	other := NewRedisCache(rdb, "")
	_, ok = other.Get(ctx(), 7, "pg:u:p")
	require.True(t, ok)
	// ...and the invalidation, with no pub/sub involved.
	c.InvalidateTenant(ctx(), 7)
	_, ok = other.Get(ctx(), 7, "pg:u:p")
	require.False(t, ok)
	_, ok = c.Get(ctx(), 8, "pg:u:p")
	require.False(t, ok, "other tenants are separate")

	// Redis down: misses, never errors.
	mr.Close()
	_, ok = c.Get(ctx(), 7, "pg:u:p")
	require.False(t, ok)
	c.Set(ctx(), 7, "x", []byte("y"), time.Minute)
}

func TestResolverWithRedisCache(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	w := newWorld(t, WithCache(NewRedisCache(rdb, "test:acl")))
	w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	require.Equal(t, model.RoleWriter, w.role("alice", "root"))
	require.NotEmpty(t, mr.Keys(), "decisions were written through to Redis")
}

func mustIdentity(t *testing.T, w *world, user string) *Identity {
	t.Helper()
	id, err := w.res.Identity(ctx(), 1, user)
	require.NoError(t, err)
	return id
}
