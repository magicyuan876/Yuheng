package service

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/stretchr/testify/require"
)

func httpCode(t *testing.T, err error) int {
	t.Helper()
	require.Error(t, err)
	appErr, ok := err.(*apperrors.AppError)
	require.True(t, ok, "expected AppError, got %T: %v", err, err)
	return appErr.HTTPCode
}

// Acceptance (T1.1): open/private spaces are visible to non-members exactly
// as 技术方案 §3.1 says — private: invisible; open: readable with the
// default role; tenant admins: admin everywhere.
func TestSpaceVisibilityForNonMembers(t *testing.T) {
	e := newEnv(t)
	alice, bob, viewer, admin := e.identity("alice"), e.identity("bob"), e.identity("viewer"), e.identity("admin")

	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Engineering", Description: "eng docs"})
	require.NoError(t, err)
	require.Equal(t, model.RoleAdmin, sp.Role)
	require.Equal(t, "engineering", sp.Slug)
	require.Equal(t, model.VisibilityPrivate, sp.Visibility)
	require.Equal(t, 1, sp.MemberCount)

	// Private: bob sees nothing.
	list, err := e.svc.Spaces.List(ctx(), bob)
	require.NoError(t, err)
	require.Empty(t, list)
	role, err := e.resolver.SpaceRole(ctx(), bob, sp.Space)
	require.NoError(t, err)
	require.Equal(t, model.RoleNone, role)

	// Tenant admin sees it as admin without being a member.
	list, err = e.svc.Spaces.List(ctx(), admin)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, model.RoleAdmin, list[0].Role)

	// Open with default writer: bob reads and writes, tenant viewer is capped.
	open := model.VisibilityOpen
	writer := model.RoleWriter
	_, err = e.svc.Spaces.Update(ctx(), alice, sp.Space, UpdateSpaceInput{Visibility: &open, DefaultRole: &writer})
	require.NoError(t, err)
	list, err = e.svc.Spaces.List(ctx(), bob)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, model.RoleWriter, list[0].Role)
	list, err = e.svc.Spaces.List(ctx(), viewer)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, model.RoleReader, list[0].Role, "tenant viewers never exceed reader")

	// Back to private forces default_role to none.
	private := model.VisibilityPrivate
	v, err := e.svc.Spaces.Update(ctx(), alice, sp.Space, UpdateSpaceInput{Visibility: &private})
	require.NoError(t, err)
	require.Equal(t, model.RoleNone, v.DefaultRole)
	list, err = e.svc.Spaces.List(ctx(), bob)
	require.NoError(t, err)
	require.Empty(t, list)

	// Audit: docs.space.* rows were recorded.
	require.True(t, e.audit.has(audit.SpaceCreated))
	require.True(t, e.audit.has(audit.SpaceUpdated))
	require.Contains(t, e.eventTypes(), events.SpaceCreated)
	require.Contains(t, e.eventTypes(), events.SpaceUpdated)
}

func TestSpaceSlugsAndValidation(t *testing.T) {
	e := newEnv(t)
	alice := e.identity("alice")

	a, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Product Specs"})
	require.NoError(t, err)
	require.Equal(t, "product-specs", a.Slug)
	b, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Product  Specs!"})
	require.NoError(t, err)
	require.Equal(t, "product-specs-2", b.Slug, "generated slugs get a numeric suffix on collision")

	cjk, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "研发知识库"})
	require.NoError(t, err)
	require.Regexp(t, `^space-[a-z2-9]{6}$`, cjk.Slug, "names without ASCII get a generated slug")

	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Dup", Slug: "product-specs"})
	require.Equal(t, 409, httpCode(t, err), "an explicit duplicate slug is a conflict, not silently renamed")

	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Bad", Slug: "Has Space"})
	require.Equal(t, 400, httpCode(t, err))
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "  "})
	require.Equal(t, 400, httpCode(t, err))
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Public", Visibility: model.VisibilityPublic})
	require.Equal(t, 400, httpCode(t, err), "public spaces arrive with sharing (T4.2)")
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{
		Name: "Open", Visibility: model.VisibilityOpen, DefaultRole: model.RoleAdmin,
	})
	require.Equal(t, 400, httpCode(t, err), "an open space cannot hand out admin by default")
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Settings", Settings: []byte(`[1,2]`)})
	require.Equal(t, 400, httpCode(t, err))

	// Explicit slug is normalised to lower case; renaming to a taken slug conflicts.
	c, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Design", Slug: "DESIGN"})
	require.NoError(t, err)
	require.Equal(t, "design", c.Slug)
	_, err = e.svc.Spaces.Update(ctx(), alice, c.Space, UpdateSpaceInput{Slug: strp("product-specs")})
	require.Equal(t, 409, httpCode(t, err))
	same, err := e.svc.Spaces.Update(ctx(), alice, c.Space, UpdateSpaceInput{Slug: strp("design")})
	require.NoError(t, err, "re-submitting the current slug is not a conflict")
	require.Equal(t, "design", same.Slug)

	// Deleting frees the slug; restoring needs a tenant admin and a free slug.
	require.NoError(t, e.svc.Spaces.Delete(ctx(), alice, c.Space))
	list, err := e.svc.Spaces.List(ctx(), alice)
	require.NoError(t, err)
	for _, v := range list {
		require.NotEqual(t, c.ID, v.ID)
	}
	_, err = e.svc.Spaces.Restore(ctx(), alice, c.ID)
	require.Equal(t, 403, httpCode(t, err))
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Design 2", Slug: "design"})
	require.NoError(t, err)
	_, err = e.svc.Spaces.Restore(ctx(), e.identity("owner"), c.ID)
	require.Equal(t, 409, httpCode(t, err), "slug reused while in the trash")
	require.True(t, e.audit.has(audit.SpaceDeleted))
}

func TestSpaceMembersKeepAnAdmin(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.identity("alice"), e.identity("bob")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Team"})
	require.NoError(t, err)

	// Validation of principals.
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "ghost", Role: model.RoleReader}})
	require.Equal(t, 400, httpCode(t, err), "non-members cannot be granted")
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalGroup, ID: "nope", Role: model.RoleReader}})
	require.Equal(t, 400, httpCode(t, err))
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "bob", Role: model.RoleNone}})
	require.Equal(t, 400, httpCode(t, err))

	// The only admin cannot leave or be demoted.
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "alice", Role: model.RoleWriter}})
	require.Equal(t, 400, httpCode(t, err))
	err = e.svc.Spaces.RemoveMember(ctx(), alice, sp.Space, model.UserPrincipal("alice"))
	require.Equal(t, 400, httpCode(t, err))

	// Add bob as writer; re-adding as admin updates the role (idempotent add).
	members, err := e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "bob", Role: model.RoleWriter}})
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, model.RoleWriter, e.mustSpaceRole(bob, sp.Space))
	members, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "bob", Role: model.RoleAdmin}})
	require.NoError(t, err)
	require.Len(t, members, 2, "no duplicate row")
	require.Equal(t, model.RoleAdmin, members[0].Role)
	require.Equal(t, model.RoleAdmin, members[1].Role)
	require.Equal(t, "alice@example.test", memberByID(members, "alice").Email, "users are hydrated from the directory")

	// Now alice may step down and leave.
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{{Type: model.PrincipalUser, ID: "alice", Role: model.RoleReader}})
	require.NoError(t, err)
	require.NoError(t, e.svc.Spaces.RemoveMember(ctx(), bob, sp.Space, model.UserPrincipal("alice")))
	require.Equal(t, model.RoleNone, e.mustSpaceRole(alice, sp.Space), "removal takes effect immediately (cache dropped)")
	err = e.svc.Spaces.RemoveMember(ctx(), bob, sp.Space, model.UserPrincipal("alice"))
	require.Equal(t, 404, httpCode(t, err))

	actions := e.audit.actions()
	require.Contains(t, actions, string(audit.SpaceMemberAdded))
	require.Contains(t, actions, string(audit.SpaceMemberRoleChanged))
	require.Contains(t, actions, string(audit.SpaceMemberRemoved))
	require.Contains(t, e.eventTypes(), events.SpaceMembersChange)
}

func TestSpaceMembersThroughGroupsAndOrdering(t *testing.T) {
	e := newEnv(t)
	owner, alice := e.identity("owner"), e.identity("alice")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Team"})
	require.NoError(t, err)
	g, err := e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "Backend", MemberIDs: []string{"bob", "carol"}})
	require.NoError(t, err)
	groups, err := e.svc.Groups.List(ctx(), owner)
	require.NoError(t, err)
	var everyone *GroupView
	for _, gv := range groups {
		if gv.IsDefault {
			everyone = gv
		}
	}
	require.NotNil(t, everyone)

	members, err := e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{
		{Type: model.PrincipalGroup, ID: g.ID, Role: model.RoleWriter},
		{Type: model.PrincipalUser, ID: "bob", Role: model.RoleReader},
		{Type: model.PrincipalGroup, ID: everyone.ID, Role: model.RoleReader},
	})
	require.NoError(t, err)
	// Ordering: admins (alice), then writers (group Backend), then readers with groups first.
	require.Equal(t, []string{"alice", "Backend", model.DefaultGroupName, "bob"}, names(members))
	require.True(t, memberByID(members, everyone.ID).IsDefaultGroup)
	require.EqualValues(t, 7, *memberByID(members, everyone.ID).GroupMemberCount, "default group counts every tenant member")
	require.EqualValues(t, 2, *memberByID(members, g.ID).GroupMemberCount)

	// Effective roles: bob writer through the group beats direct reader; carol
	// writer through group; viewer reader through "everyone" (capped anyway).
	require.Equal(t, model.RoleWriter, e.mustSpaceRole(e.identity("bob"), sp.Space))
	require.Equal(t, model.RoleWriter, e.mustSpaceRole(e.identity("carol"), sp.Space))
	require.Equal(t, model.RoleReader, e.mustSpaceRole(e.identity("viewer"), sp.Space))
}

func TestBindKnowledgeBase(t *testing.T) {
	e := newEnv(t)
	alice := e.identity("alice")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "KB"})
	require.NoError(t, err)
	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), alice, sp.Space, strp("kb-1"), nil)
	require.Equal(t, 400, httpCode(t, err), "no knowledge base source configured")

	e2 := newEnv(t, func(d *Deps) { d.KnowledgeBases = fakeKBs{"kb-1": 1, "kb-other": 2} })
	alice = e2.identity("alice")
	sp, err = e2.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "KB"})
	require.NoError(t, err)
	_, err = e2.svc.Spaces.BindKnowledgeBase(ctx(), alice, sp.Space, strp("kb-other"), nil)
	require.Equal(t, 400, httpCode(t, err), "another tenant's knowledge base is rejected")
	v, err := e2.svc.Spaces.BindKnowledgeBase(ctx(), alice, sp.Space, strp("kb-1"), nil)
	require.NoError(t, err)
	require.Equal(t, "kb-1", *v.KnowledgeBaseID)
	require.True(t, e2.audit.has(audit.SpaceKBBound))
	v, err = e2.svc.Spaces.BindKnowledgeBase(ctx(), alice, v.Space, strp(""), nil)
	require.NoError(t, err)
	require.Nil(t, v.KnowledgeBaseID)
	require.True(t, e2.audit.has(audit.SpaceKBUnbound))
	_, err = e2.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "KB2", KnowledgeBaseID: strp("kb-1")})
	require.NoError(t, err)
}

func names(members []*MemberView) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Name)
	}
	return out
}

func memberByID(members []*MemberView, id string) *MemberView {
	for _, m := range members {
		if m.PrincipalID == id {
			return m
		}
	}
	return nil
}
