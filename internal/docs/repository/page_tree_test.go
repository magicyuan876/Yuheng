package repository

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

func TestPageTreePagingAndCounts(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, "Root", nil, "a0")
	var kids []*model.Page
	for i := 0; i < 5; i++ {
		kids = append(kids, f.page(t, "K"+string(rune('0'+i)), ptr(root.ID), "a"+string(rune('0'+i))))
	}
	f.page(t, "Grandchild", ptr(kids[2].ID), "a0")
	trashed := f.page(t, "Trashed", ptr(root.ID), "a9")
	_, err := f.repos.Pages.SoftDeleteSubtree(ctx(), 1, trashed.ID, "u1")
	require.NoError(t, err)

	// Page through the children two at a time.
	var got []string
	var cursor *TreeCursor
	for {
		batch, err := f.repos.Pages.ListChildrenAfter(ctx(), 1, f.space.ID, ptr(root.ID), cursor, 2)
		require.NoError(t, err)
		if len(batch) == 0 {
			break
		}
		got = append(got, titles(batch)...)
		last := batch[len(batch)-1]
		cursor = &TreeCursor{Position: last.Position, ID: last.ID}
	}
	require.Equal(t, []string{"K0", "K1", "K2", "K3", "K4"}, got, "trashed children are skipped")

	counts, err := f.repos.Pages.ChildCounts(ctx(), 1, []string{root.ID, kids[2].ID, kids[0].ID})
	require.NoError(t, err)
	require.Equal(t, int64(5), counts[root.ID])
	require.Equal(t, int64(1), counts[kids[2].ID])
	require.Zero(t, counts[kids[0].ID])

	last, ok, err := f.repos.Pages.LastPosition(ctx(), 1, f.space.ID, ptr(root.ID))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "a4", last, "the trashed a9 does not count")
	_, ok, err = f.repos.Pages.LastPosition(ctx(), 1, f.space.ID, ptr(kids[0].ID))
	require.NoError(t, err)
	require.False(t, ok)

	next, ok, err := f.repos.Pages.NextPosition(ctx(), 1, f.space.ID, ptr(root.ID), kids[1].Position, kids[1].ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "a2", next)
	_, ok, err = f.repos.Pages.NextPosition(ctx(), 1, f.space.ID, ptr(root.ID), kids[4].Position, kids[4].ID)
	require.NoError(t, err)
	require.False(t, ok)

	// Rebalance rewrites positions in bulk; order follows the new keys.
	require.NoError(t, f.repos.Pages.SetPositions(ctx(), 1, map[string]string{
		kids[0].ID: "a4", kids[4].ID: "a0",
	}))
	reordered, err := f.repos.Pages.ListChildren(ctx(), 1, f.space.ID, ptr(root.ID))
	require.NoError(t, err)
	require.Equal(t, []string{"K4", "K1", "K2", "K3", "K0"}, titles(reordered))
	require.ErrorIs(t, f.repos.Pages.SetPositions(ctx(), 1, map[string]string{"missing": "a0"}), ErrNotFound)

	byShort, err := f.repos.Pages.GetAnyByShortID(ctx(), 1, trashed.ShortID)
	require.NoError(t, err)
	require.NotNil(t, byShort.DeletedAt)
	_, err = f.repos.Pages.GetByShortID(ctx(), 1, trashed.ShortID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPageCrossSpaceMoveCarriesDependents(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, "Root", nil, "a0")
	child := f.page(t, "Child", ptr(root.ID), "a0")
	require.NoError(t, f.repos.Access.SetRestricted(ctx(), 1, f.space.ID, child.ID, "u1"))
	require.NoError(t, f.repos.DB().Exec(
		`INSERT INTO docs_comments (id, tenant_id, space_id, page_id, body, creator_id)
		 VALUES ('c1', 1, ?, ?, '{}', 'u1')`, f.space.ID, child.ID).Error)

	require.NoError(t, f.repos.Pages.Move(ctx(), 1, root.ID, MoveTarget{SpaceID: f.other.ID, Position: "a0"}))

	var accessSpace, commentSpace string
	require.NoError(t, f.repos.DB().Raw(
		"SELECT space_id FROM docs_page_access WHERE page_id = ?", child.ID).Scan(&accessSpace).Error)
	require.NoError(t, f.repos.DB().Raw(
		"SELECT space_id FROM docs_comments WHERE id = 'c1'").Scan(&commentSpace).Error)
	require.Equal(t, f.other.ID, accessSpace, "page access rows follow the page")
	require.Equal(t, f.other.ID, commentSpace, "comments follow the page")
}

func TestPageCreateBatchAndPurgeOne(t *testing.T) {
	f := newFixture(t)
	parent := &model.Page{TenantID: 1, SpaceID: f.space.ID, ShortID: "batch00001", Title: "P", Position: "a0"}
	parent.ID = NewID()
	child := &model.Page{
		TenantID: 1, SpaceID: f.space.ID, ShortID: "batch00002", Title: "C", Position: "a0", ParentID: &parent.ID,
	}
	require.NoError(t, f.repos.Pages.CreateBatch(ctx(), []*model.Page{parent, child}))
	sub, err := f.repos.Pages.SubtreeIDs(ctx(), 1, parent.ID)
	require.NoError(t, err)
	require.Len(t, sub, 2)

	_, err = f.repos.Pages.PurgeOne(ctx(), 1, parent.ID)
	require.ErrorIs(t, err, ErrInvalidMove, "live pages cannot be purged")
	_, err = f.repos.Pages.SoftDeleteSubtree(ctx(), 1, parent.ID, "u1")
	require.NoError(t, err)
	purged, err := f.repos.Pages.PurgeOne(ctx(), 1, parent.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{parent.ID, child.ID}, purged)
	_, err = f.repos.Pages.GetAny(ctx(), 1, child.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.repos.Pages.PurgeOne(ctx(), 1, parent.ID)
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, f.repos.Pages.LockSpace(ctx(), 1, f.space.ID), "a no-op on SQLite")
}
