package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTenantGroupsAndMembership(t *testing.T) {
	ctx := context.Background()
	repo := NewTenantGroupRepository(pgtest.New(t))

	def, err := repo.EnsureDefault(ctx, 1, "u1")
	require.NoError(t, err)
	require.True(t, def.IsDefault)
	again, err := repo.EnsureDefault(ctx, 1, "u2")
	require.NoError(t, err)
	require.Equal(t, def.ID, again.ID, "EnsureDefault is idempotent")
	other, err := repo.EnsureDefault(ctx, 2, "")
	require.NoError(t, err)
	require.NotEqual(t, def.ID, other.ID)

	g := &types.TenantGroup{TenantID: 1, Name: "backend"}
	require.NoError(t, repo.Create(ctx, g, []string{"u1", "u2", "u2", ""}, "admin"))
	require.NotEmpty(t, g.ID, "an id is assigned")
	require.Equal(t, types.TenantGroupSourceManual, g.Source, "an empty source is manual")
	require.ErrorIs(t, repo.Create(ctx, &types.TenantGroup{TenantID: 1, Name: "backend"}, nil, ""),
		ErrTenantGroupNameTaken)
	require.NoError(t, repo.Create(ctx, &types.TenantGroup{TenantID: 2, Name: "backend"}, nil, ""),
		"names are unique per workspace only")

	require.NoError(t, repo.AddMembers(ctx, 1, g.ID, []string{"u2", "u3"}, "admin"), "re-adding is a no-op")
	ids, err := repo.ListMemberIDs(ctx, 1, g.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"u1", "u2", "u3"}, ids)

	groups, err := repo.GroupIDsForUser(ctx, 1, "u2")
	require.NoError(t, err)
	require.Equal(t, []string{g.ID}, groups, "the default group is implicit, not listed")

	require.NoError(t, repo.RemoveMembers(ctx, 1, g.ID, []string{"u2"}))
	groups, err = repo.GroupIDsForUser(ctx, 1, "u2")
	require.NoError(t, err)
	require.Empty(t, groups)

	require.ErrorIs(t, repo.AddMembers(ctx, 2, g.ID, []string{"u9"}, ""), ErrTenantGroupNotFound,
		"a group cannot be addressed from another workspace")
	_, err = repo.Get(ctx, 2, g.ID)
	require.ErrorIs(t, err, ErrTenantGroupNotFound)

	list, err := repo.List(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].IsDefault, "default group sorts first")
}

func TestTenantGroupUpdateAllowsOnlyTheEditableColumns(t *testing.T) {
	ctx := context.Background()
	repo := NewTenantGroupRepository(pgtest.New(t))
	g := &types.TenantGroup{TenantID: 1, Name: "backend"}
	require.NoError(t, repo.Create(ctx, g, nil, ""))

	require.Error(t, repo.Update(ctx, 1, g.ID, map[string]any{"is_default": true}),
		"is_default is not a column a caller may write")
	require.NoError(t, repo.Update(ctx, 1, g.ID, map[string]any{"name": "platform", "description": "svc"}))
	got, err := repo.Get(ctx, 1, g.ID)
	require.NoError(t, err)
	require.Equal(t, "platform", got.Name)
	require.Equal(t, "svc", got.Description)
	require.ErrorIs(t, repo.Update(ctx, 1, "missing", map[string]any{"name": "x"}), ErrTenantGroupNotFound)
}

func TestTenantGroupSoftDeleteRunsTheCleanupInTheSameTransaction(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	repo := NewTenantGroupRepository(db)
	def, err := repo.EnsureDefault(ctx, 1, "owner")
	require.NoError(t, err)
	g := &types.TenantGroup{TenantID: 1, Name: "backend"}
	require.NoError(t, repo.Create(ctx, g, []string{"u1"}, ""))

	require.ErrorIs(t, repo.SoftDelete(ctx, 1, def.ID, nil), ErrTenantGroupNotFound,
		"the default group cannot be deleted")

	// A failing cleanup rolls the deletion back.
	boom := errors.New("cleanup failed")
	err = repo.SoftDelete(ctx, 1, g.ID, func(*gorm.DB) error { return boom })
	require.ErrorIs(t, err, boom)
	_, err = repo.Get(ctx, 1, g.ID)
	require.NoError(t, err, "the group is still there")

	// A cleanup that writes through tx sees its row committed with the
	// deletion; one that writes through another handle would deadlock
	// against the row lock, which is the point of handing tx over.
	var ran bool
	require.NoError(t, repo.SoftDelete(ctx, 1, g.ID, func(tx *gorm.DB) error {
		ran = true
		var inTx types.TenantGroup
		return tx.Where("id = ? AND deleted_at IS NULL", g.ID).First(&inTx).Error
	}))
	require.True(t, ran, "the cleanup ran before the row was hidden")

	groups, err := repo.GroupIDsForUser(ctx, 1, "u1")
	require.NoError(t, err)
	require.Empty(t, groups, "memberships of a deleted group no longer count")
	require.NoError(t, repo.Create(ctx, &types.TenantGroup{TenantID: 1, Name: "backend"}, nil, ""),
		"the name is free again after soft delete")
}
