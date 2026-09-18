package service

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

// Acceptance (T1.1): a group membership change is reflected in permission
// decisions immediately, i.e. the ACL cache is invalidated.
func TestGroupMembershipInvalidatesPermissionCache(t *testing.T) {
	e := newEnv(t)
	owner, alice := e.identity("owner"), e.identity("alice")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Team"})
	require.NoError(t, err)
	g, err := e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "Backend"})
	require.NoError(t, err)
	_, err = e.svc.Spaces.SetMembers(ctx(), alice, sp.Space, []MemberInput{
		{Type: model.PrincipalGroup, ID: g.ID, Role: model.RoleWriter},
	})
	require.NoError(t, err)

	carol := e.identity("carol")
	require.Equal(t, model.RoleNone, e.mustSpaceRole(carol, sp.Space), "warm the cache with a negative answer")

	require.NoError(t, e.svc.Groups.AddMembers(ctx(), owner, g.ID, []string{"carol", "carol"}))
	require.Equal(t, model.RoleWriter, e.mustSpaceRole(carol, sp.Space), "group grant reaches carol at once")
	view, err := e.svc.Groups.Get(ctx(), owner, g.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, view.MemberCount, "duplicates in one request collapse")
	require.NoError(t, e.svc.Groups.AddMembers(ctx(), owner, g.ID, []string{"carol"}), "re-adding is a no-op")

	require.NoError(t, e.svc.Groups.RemoveMember(ctx(), owner, g.ID, "carol"))
	require.Equal(t, model.RoleNone, e.mustSpaceRole(carol, sp.Space), "removal reaches carol at once")
	require.Equal(t, 404, httpCode(t, e.svc.Groups.RemoveMember(ctx(), owner, g.ID, "carol")))

	// Deleting the group removes the space membership it carried.
	require.NoError(t, e.svc.Groups.AddMembers(ctx(), owner, g.ID, []string{"bob"}))
	require.Equal(t, model.RoleWriter, e.mustSpaceRole(e.identity("bob"), sp.Space))
	require.NoError(t, e.svc.Groups.Delete(ctx(), owner, g.ID))
	require.Equal(t, model.RoleNone, e.mustSpaceRole(e.identity("bob"), sp.Space))
	members, err := e.svc.Spaces.ListMembers(ctx(), sp.Space)
	require.NoError(t, err)
	require.Len(t, members, 1, "the group's membership row is gone")

	require.Contains(t, e.eventTypes(), events.GroupChanged)
	for _, a := range []string{
		string(audit.GroupCreated), string(audit.GroupMemberAdded), string(audit.GroupMemberRemoved), string(audit.GroupDeleted),
	} {
		require.Contains(t, e.audit.actions(), a)
	}
}

func TestDefaultGroupRules(t *testing.T) {
	e := newEnv(t)
	owner := e.identity("owner")

	groups, err := e.svc.Groups.List(ctx(), owner)
	require.NoError(t, err)
	require.Len(t, groups, 1, "listing creates the default group on first use")
	def := groups[0]
	require.True(t, def.IsDefault)
	require.Equal(t, model.DefaultGroupName, def.Name)
	require.EqualValues(t, 7, def.MemberCount, "every active tenant member is implicitly in it")

	_, err = e.svc.Groups.Update(ctx(), owner, def.ID, UpdateGroupInput{Name: strp("staff")})
	require.Equal(t, 400, httpCode(t, err), "the default group keeps its name")
	v, err := e.svc.Groups.Update(ctx(), owner, def.ID, UpdateGroupInput{Description: strp("all hands")})
	require.NoError(t, err, "but may be described")
	require.Equal(t, "all hands", v.Description)
	require.Equal(t, 400, httpCode(t, e.svc.Groups.Delete(ctx(), owner, def.ID)))
	require.Equal(t, 400, httpCode(t, e.svc.Groups.AddMembers(ctx(), owner, def.ID, []string{"bob"})))
	require.Equal(t, 400, httpCode(t, e.svc.Groups.RemoveMember(ctx(), owner, def.ID, "bob")))

	page, err := e.svc.Groups.ListMembers(ctx(), owner, def.ID, "", 1, 4)
	require.NoError(t, err)
	require.EqualValues(t, 7, page.Total)
	require.Len(t, page.Members, 4)
	page, err = e.svc.Groups.ListMembers(ctx(), owner, def.ID, "bob", 1, 20)
	require.NoError(t, err)
	require.Len(t, page.Members, 1)
	require.Equal(t, "bob@example.test", page.Members[0].Email)

	// Names: reserved, unique (case-insensitive), bounded.
	_, err = e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "Everyone"})
	require.Equal(t, 400, httpCode(t, err))
	g, err := e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "Backend", Description: "svc"})
	require.NoError(t, err)
	_, err = e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "backend"})
	require.Equal(t, 409, httpCode(t, err))
	_, err = e.svc.Groups.Create(ctx(), owner, CreateGroupInput{Name: "Frontend", MemberIDs: []string{"ghost"}})
	require.Equal(t, 400, httpCode(t, err), "initial members must be tenant members")
	renamed, err := e.svc.Groups.Update(ctx(), owner, g.ID, UpdateGroupInput{Name: strp("Platform")})
	require.NoError(t, err)
	require.Equal(t, "Platform", renamed.Name)

	// Member paging of an explicit group, with search.
	require.NoError(t, e.svc.Groups.AddMembers(ctx(), owner, g.ID, []string{"alice", "bob", "carol"}))
	page, err = e.svc.Groups.ListMembers(ctx(), owner, g.ID, "", 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, page.Members, 1)
	require.Equal(t, "carol", page.Members[0].UserID)
	page, err = e.svc.Groups.ListMembers(ctx(), owner, g.ID, "ALICE@", 1, 20)
	require.NoError(t, err)
	require.Len(t, page.Members, 1)

	groups, err = e.svc.Groups.List(ctx(), owner)
	require.NoError(t, err)
	require.Equal(t, []string{model.DefaultGroupName, "Platform"}, []string{groups[0].Name, groups[1].Name})
	require.EqualValues(t, 3, groups[1].MemberCount)
}
