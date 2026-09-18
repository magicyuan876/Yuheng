package repository

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

func TestGroupsAndMembership(t *testing.T) {
	repos := New(openTestDB(t))

	def, err := repos.Groups.EnsureDefault(ctx(), 1, "u1")
	require.NoError(t, err)
	require.True(t, def.IsDefault)
	again, err := repos.Groups.EnsureDefault(ctx(), 1, "u2")
	require.NoError(t, err)
	require.Equal(t, def.ID, again.ID, "EnsureDefault is idempotent")
	other, err := repos.Groups.EnsureDefault(ctx(), 2, "")
	require.NoError(t, err)
	require.NotEqual(t, def.ID, other.ID)

	g := &model.TenantGroup{TenantID: 1, Name: "backend"}
	require.NoError(t, repos.Groups.Create(ctx(), g))
	require.ErrorIs(t, repos.Groups.Create(ctx(), &model.TenantGroup{TenantID: 1, Name: "backend"}), ErrDuplicate)
	require.NoError(t, repos.Groups.Create(ctx(), &model.TenantGroup{TenantID: 2, Name: "backend"}),
		"names are unique per tenant only")

	require.NoError(t, repos.Groups.AddMembers(ctx(), 1, g.ID, []string{"u1", "u2", "u2", ""}, "admin"))
	require.NoError(t, repos.Groups.AddMembers(ctx(), 1, g.ID, []string{"u2", "u3"}, "admin"), "re-adding is a no-op")
	ids, err := repos.Groups.ListMemberIDs(ctx(), 1, g.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"u1", "u2", "u3"}, ids)

	groups, err := repos.Groups.GroupIDsForUser(ctx(), 1, "u2")
	require.NoError(t, err)
	require.Equal(t, []string{g.ID}, groups, "the default group is implicit, not listed")

	require.NoError(t, repos.Groups.RemoveMembers(ctx(), 1, g.ID, []string{"u2"}))
	groups, err = repos.Groups.GroupIDsForUser(ctx(), 1, "u2")
	require.NoError(t, err)
	require.Empty(t, groups)

	require.ErrorIs(t, repos.Groups.AddMembers(ctx(), 2, g.ID, []string{"u9"}, ""), ErrNotFound,
		"a group cannot be addressed from another tenant")

	list, err := repos.Groups.List(ctx(), 1)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].IsDefault, "default group sorts first")

	require.ErrorIs(t, repos.Groups.SoftDelete(ctx(), 1, def.ID), ErrNotFound, "the default group cannot be deleted")
	require.NoError(t, repos.Groups.SoftDelete(ctx(), 1, g.ID))
	groups, err = repos.Groups.GroupIDsForUser(ctx(), 1, "u1")
	require.NoError(t, err)
	require.Empty(t, groups, "memberships of a deleted group no longer count")
	require.NoError(t, repos.Groups.Create(ctx(), &model.TenantGroup{TenantID: 1, Name: "backend"}),
		"the name is free again after soft delete")
}
