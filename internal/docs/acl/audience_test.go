package acl

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/stretchr/testify/require"
)

func (w *world) audience(user string) *Audience {
	w.t.Helper()
	a, err := w.res.Audience(ctx(), mustIdentity(w.t, w, user))
	require.NoError(w.t, err)
	return a
}

func admits(t *testing.T, a *Audience, e events.Event) bool {
	t.Helper()
	ok, err := a.Admits(ctx(), e)
	require.NoError(t, err)
	return ok
}

func pageEvent(t events.Type, p *model.Page) events.Event {
	return events.New(t, p.TenantID).WithSpace(p.SpaceID).WithPage(p.ID).With("title", p.Title)
}

func TestAudienceScopesPageEventsToReaders(t *testing.T) {
	w := newWorld(t)
	w.page("open", "")
	w.page("secret", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	w.member(model.UserPrincipal("bob"), model.RoleWriter)
	w.restrict("secret", map[model.Principal]model.SpaceRole{model.UserPrincipal("alice"): model.RoleReader})

	alice, bob, carol := w.audience("alice"), w.audience("bob"), w.audience("carol")

	require.True(t, admits(t, alice, pageEvent(events.PageMeta, w.pages["open"])))
	require.True(t, admits(t, alice, pageEvent(events.PageMeta, w.pages["secret"])), "alice holds a grant")
	require.True(t, admits(t, bob, pageEvent(events.PageMeta, w.pages["open"])))
	require.False(t, admits(t, bob, pageEvent(events.PageMeta, w.pages["secret"])),
		"a restricted page's title must not reach a space member without a grant")
	require.False(t, admits(t, carol, pageEvent(events.PageCreated, w.pages["open"])),
		"carol is not a member of the private space")
	require.True(t, admits(t, w.audience("owner"), pageEvent(events.PageMeta, w.pages["secret"])),
		"tenant admins see everything")
}

func TestAudienceNotificationsReachOnlyTheRecipient(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	w.member(model.UserPrincipal("bob"), model.RoleWriter)

	e := pageEvent(events.NotificationCreated, p).With("user_id", "alice").With("notification_id", "n1")
	require.True(t, admits(t, w.audience("alice"), e))
	require.False(t, admits(t, w.audience("bob"), e), "bob can read the page but the notification is alice's")
	require.False(t, admits(t, w.audience("owner"), e), "not even a tenant admin receives someone else's")
}

func TestAudienceRevocationTakesEffectMidConnection(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	alice := w.audience("alice")
	require.True(t, admits(t, alice, pageEvent(events.PageContent, p)))

	w.restrict("root", map[model.Principal]model.SpaceRole{model.UserPrincipal("bob"): model.RoleReader})
	require.True(t, admits(t, alice, pageEvent(events.PageAccess, p)),
		"the event that takes the page away still reaches the view showing it")
	require.False(t, admits(t, alice, pageEvent(events.PageMeta, p)), "and nothing after it")
}

func TestAudienceTrashedAndPurgedPages(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	secret := w.page("secret", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	w.member(model.UserPrincipal("bob"), model.RoleReader)
	w.restrict("secret", map[model.Principal]model.SpaceRole{model.UserPrincipal("alice"): model.RoleWriter})

	_, err := w.repos.Pages.SoftDeleteSubtree(ctx(), 1, secret.ID, "alice")
	require.NoError(t, err)
	w.res.Invalidate(ctx(), 1)
	deleted := pageEvent(events.PageDeleted, secret)
	require.True(t, admits(t, w.audience("alice"), deleted), "a trashed page is judged by its own chain")
	require.False(t, admits(t, w.audience("bob"), deleted), "so its restriction still holds in the trash")

	_, err = w.repos.Pages.PurgeOne(ctx(), 1, secret.ID)
	require.NoError(t, err)
	purged := events.New(events.PagePurged, 1).WithSpace(w.space.ID).WithPage(secret.ID)
	require.True(t, admits(t, w.audience("bob"), purged),
		"once the row is gone the event is about the space's trash and carries only ids")
	require.False(t, admits(t, w.audience("carol"), purged), "carol cannot read the space")
}

func TestAudienceDeletedSpaceReachesThoseWhoSawIt(t *testing.T) {
	w := newWorld(t)
	w.member(model.UserPrincipal("alice"), model.RoleReader)
	alice, carol := w.audience("alice"), w.audience("carol")

	require.NoError(t, w.repos.Spaces.SoftDelete(ctx(), 1, w.space.ID))
	w.res.Invalidate(ctx(), 1)
	e := events.New(events.SpaceDeleted, 1).WithSpace(w.space.ID)
	require.True(t, admits(t, alice, e))
	require.False(t, admits(t, alice, events.New(events.SpaceUpdated, 1).WithSpace(w.space.ID)),
		"only once: the space is forgotten with its deletion")
	require.False(t, admits(t, carol, e))
}

func TestAudienceCrossSpaceMoveIsJudgedPerSpace(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	other := &model.Space{TenantID: 1, Slug: "ops", Name: "Ops", Visibility: model.VisibilityPrivate}
	require.NoError(t, w.repos.Spaces.Create(ctx(), other))
	w.member(model.UserPrincipal("alice"), model.RoleReader) // source space only

	require.NoError(t, w.repos.Pages.Move(ctx(), 1, p.ID, repository.MoveTarget{SpaceID: other.ID, Position: "a0"}))
	w.res.Invalidate(ctx(), 1)

	left := events.New(events.PageMoved, 1).WithSpace(w.space.ID).WithPage(p.ID).With("left", true)
	arrived := events.New(events.PageMoved, 1).WithSpace(other.ID).WithPage(p.ID).With("title", p.Title)
	alice := w.audience("alice")
	require.True(t, admits(t, alice, left), "the source space's tree changed and alice reads it")
	require.False(t, admits(t, alice, arrived), "the page now lives where alice cannot read")
}

func TestAudienceTenantWideAndForeignEvents(t *testing.T) {
	w := newWorld(t)
	alice := w.audience("alice")
	require.True(t, admits(t, alice, events.New(events.GroupChanged, 1).With("group_id", "g1")))
	require.False(t, admits(t, alice, events.New(events.GroupChanged, 2).With("group_id", "g1")))
}

func TestAudienceEndsWhenMembershipEnds(t *testing.T) {
	w := newWorld(t)
	p := w.page("root", "")
	w.member(model.UserPrincipal("alice"), model.RoleWriter)
	alice := w.audience("alice")

	delete(w.roles, "1/alice")
	w.res.Invalidate(ctx(), 1)
	_, err := alice.Admits(ctx(), pageEvent(events.PageMeta, p))
	require.ErrorIs(t, err, ErrNoLongerMember)
}

func TestAudienceForgetsOldestPagesFirst(t *testing.T) {
	a := &Audience{pages: map[string]uint64{}}
	for i := 0; i < rememberedPages+10; i++ {
		a.rememberPage(string(rune('a'+i%26)) + string(rune(i)))
	}
	require.Len(t, a.pages, rememberedPages)

	// A page forgotten and remembered again survives the eviction of its own
	// older entry.
	b := &Audience{pages: map[string]uint64{}}
	b.rememberPage("p")
	require.True(t, b.forgetPage("p"))
	b.rememberPage("p")
	for i := 0; i < rememberedPages-1; i++ {
		b.rememberPage(string(rune(0x4e00 + i)))
	}
	require.Contains(t, b.pages, "p")
	require.LessOrEqual(t, len(b.order), 2*rememberedPages)
}
