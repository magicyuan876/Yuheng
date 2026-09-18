package repository

import (
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	repos *Repositories
	space *model.Space
	other *model.Space
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repos := New(openTestDB(t))
	space := &model.Space{TenantID: 1, Slug: "docs", Name: "Docs"}
	other := &model.Space{TenantID: 1, Slug: "other", Name: "Other"}
	require.NoError(t, repos.Spaces.Create(ctx(), space))
	require.NoError(t, repos.Spaces.Create(ctx(), other))
	return &fixture{repos: repos, space: space, other: other}
}

var shortSeq int

func (f *fixture) page(t *testing.T, title string, parent *string, position string) *model.Page {
	t.Helper()
	shortSeq++
	p := &model.Page{
		TenantID: 1, SpaceID: f.space.ID, ParentID: parent, Position: position, Title: title,
		ShortID: "s" + string(rune('a'+shortSeq%26)) + string(rune('a'+(shortSeq/26)%26)) + "0000",
		Content: model.JSON(`{"type":"doc","content":[{"type":"paragraph"}]}`), CreatorID: ptr("u1"),
	}
	require.NoError(t, f.repos.Pages.Create(ctx(), p))
	return p
}

func titles(pages []*model.Page) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.Title
	}
	return out
}

func TestPageTreeOrderingAndAncestors(t *testing.T) {
	f := newFixture(t)
	root1 := f.page(t, "Root 1", nil, "a1")
	root0 := f.page(t, "Root 0", nil, "a0") // created later, sorts first by position
	child := f.page(t, "Child", ptr(root1.ID), "a0")
	grand := f.page(t, "Grandchild", ptr(child.ID), "a0")

	roots, err := f.repos.Pages.ListChildren(ctx(), 1, f.space.ID, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"Root 0", "Root 1"}, titles(roots))
	require.Nil(t, roots[0].Content, "tree listing must not load content")

	kids, err := f.repos.Pages.ListChildren(ctx(), 1, f.space.ID, ptr(root1.ID))
	require.NoError(t, err)
	require.Equal(t, []string{"Child"}, titles(kids))

	anc, err := f.repos.Pages.ListAncestors(ctx(), 1, grand.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Root 1", "Child"}, titles(anc), "root first, page itself excluded")

	sub, err := f.repos.Pages.SubtreeIDs(ctx(), 1, root1.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{root1.ID, child.ID, grand.ID}, sub)

	_, err = f.repos.Pages.Get(ctx(), 2, root0.ID)
	require.ErrorIs(t, err, ErrNotFound, "tenant isolation")

	byShort, err := f.repos.Pages.GetByShortID(ctx(), 1, root0.ShortID)
	require.NoError(t, err)
	require.Equal(t, root0.ID, byShort.ID)
	require.NotNil(t, byShort.Content, "opening a page loads content")

	dup := &model.Page{TenantID: 1, SpaceID: f.space.ID, ShortID: root0.ShortID, Title: "dup"}
	require.ErrorIs(t, f.repos.Pages.Create(ctx(), dup), ErrDuplicate, "short ids are unique per tenant")
	dup.TenantID = 2
	dup.SpaceID = f.space.ID // FK does not care about tenant; uniqueness is per tenant
	require.NoError(t, f.repos.Pages.Create(ctx(), dup))
}

func TestPageMoveRules(t *testing.T) {
	f := newFixture(t)
	a := f.page(t, "A", nil, "a0")
	b := f.page(t, "B", nil, "a1")
	aChild := f.page(t, "A child", ptr(a.ID), "a0")

	// Reparent B under A's child.
	require.NoError(t, f.repos.Pages.Move(ctx(), 1, b.ID,
		MoveTarget{SpaceID: f.space.ID, ParentID: ptr(aChild.ID), Position: "a0"}))
	anc, err := f.repos.Pages.ListAncestors(ctx(), 1, b.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"A", "A child"}, titles(anc))

	// Cycle: A under B (B is inside A's subtree).
	err = f.repos.Pages.Move(ctx(), 1, a.ID, MoveTarget{SpaceID: f.space.ID, ParentID: ptr(b.ID)})
	require.ErrorIs(t, err, ErrInvalidMove)
	// Self-parent.
	err = f.repos.Pages.Move(ctx(), 1, a.ID, MoveTarget{SpaceID: f.space.ID, ParentID: ptr(a.ID)})
	require.ErrorIs(t, err, ErrInvalidMove)
	// Parent in another space than the target space.
	err = f.repos.Pages.Move(ctx(), 1, b.ID, MoveTarget{SpaceID: f.other.ID, ParentID: ptr(aChild.ID)})
	require.ErrorIs(t, err, ErrInvalidMove)
	// Unknown parent.
	err = f.repos.Pages.Move(ctx(), 1, b.ID, MoveTarget{SpaceID: f.space.ID, ParentID: ptr("nope")})
	require.ErrorIs(t, err, ErrInvalidMove)

	// Cross-space move to the root of the other space carries the subtree.
	require.NoError(t, f.repos.Pages.Move(ctx(), 1, a.ID, MoveTarget{SpaceID: f.other.ID, Position: "a0"}))
	for _, id := range []string{a.ID, aChild.ID, b.ID} {
		p, err := f.repos.Pages.Get(ctx(), 1, id)
		require.NoError(t, err)
		require.Equal(t, f.other.ID, p.SpaceID, "subtree follows the moved page")
	}
	roots, err := f.repos.Pages.ListChildren(ctx(), 1, f.space.ID, nil)
	require.NoError(t, err)
	require.Empty(t, roots)
}

func TestPageContentOptimisticConcurrency(t *testing.T) {
	f := newFixture(t)
	p := f.page(t, "Doc", nil, "a0")
	require.Equal(t, int64(0), p.YDocVersion)

	upd := ContentUpdate{
		Content: model.JSON(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hi"}]}]}`),
		YDoc:    []byte{1, 2, 3}, TextContent: "hi", WordCount: 1, EditorID: "u2",
		ContributorIDs: model.StringList{"u1", "u2"}, ContentChanged: true,
	}
	v, err := f.repos.Pages.UpdateContent(ctx(), 1, p.ID, 0, upd)
	require.NoError(t, err)
	require.Equal(t, int64(1), v)

	// Stale base version loses.
	_, err = f.repos.Pages.UpdateContent(ctx(), 1, p.ID, 0, upd)
	require.ErrorIs(t, err, ErrConflict)
	// Unknown page is not a conflict.
	_, err = f.repos.Pages.UpdateContent(ctx(), 1, "missing", 0, upd)
	require.ErrorIs(t, err, ErrNotFound)

	got, err := f.repos.Pages.Get(ctx(), 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.YDocVersion)
	require.Equal(t, []byte{1, 2, 3}, got.YDoc)
	require.Equal(t, "hi", got.TextContent)
	require.Equal(t, model.StringList{"u1", "u2"}, got.ContributorIDs)
	require.Equal(t, "u2", *got.LastEditorID)
	require.NotNil(t, got.ContentUpdatedAt)

	// A persist without a content change must not advance content_updated_at.
	first := *got.ContentUpdatedAt
	time.Sleep(2 * time.Millisecond)
	upd.ContentChanged = false
	_, err = f.repos.Pages.UpdateContent(ctx(), 1, p.ID, 1, upd)
	require.NoError(t, err)
	got, err = f.repos.Pages.Get(ctx(), 1, p.ID)
	require.NoError(t, err)
	require.True(t, got.ContentUpdatedAt.Equal(first))

	require.Error(t, f.repos.Pages.UpdateMeta(ctx(), 1, p.ID, map[string]any{"content": "x"}),
		"content is not writable through UpdateMeta")
	require.NoError(t, f.repos.Pages.UpdateMeta(ctx(), 1, p.ID, map[string]any{"title": "Renamed", "is_locked": true}))
	got, err = f.repos.Pages.Get(ctx(), 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Title)
	require.True(t, got.IsLocked)
}

func TestPageTrashSemantics(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, "Root", nil, "a0")
	child := f.page(t, "Child", ptr(root.ID), "a0")
	grand := f.page(t, "Grandchild", ptr(child.ID), "a0")
	sibling := f.page(t, "Sibling", nil, "a1")

	// Trash the grandchild on its own first.
	n, err := f.repos.Pages.SoftDeleteSubtree(ctx(), 1, grand.ID, "u1")
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	time.Sleep(2 * time.Millisecond)

	// Now trash the whole root: child follows, grandchild is already trashed.
	n, err = f.repos.Pages.SoftDeleteSubtree(ctx(), 1, root.ID, "u1")
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	live, err := f.repos.Pages.ListChildren(ctx(), 1, f.space.ID, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"Sibling"}, titles(live))

	trash, err := f.repos.Pages.ListTrash(ctx(), 1, f.space.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"Root", "Grandchild"}, titles(trash),
		"trash shows roots of deletion, not every descendant")

	// Restoring the root brings back the child but not the separately trashed grandchild.
	n, err = f.repos.Pages.RestoreSubtree(ctx(), 1, root.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	_, err = f.repos.Pages.Get(ctx(), 1, child.ID)
	require.NoError(t, err)
	_, err = f.repos.Pages.Get(ctx(), 1, grand.ID)
	require.ErrorIs(t, err, ErrNotFound)
	restored, err := f.repos.Pages.GetAny(ctx(), 1, root.ID)
	require.NoError(t, err)
	require.Nil(t, restored.DeletedBy)

	// Restoring a page whose parent is in the trash re-attaches it at the root.
	_, err = f.repos.Pages.SoftDeleteSubtree(ctx(), 1, root.ID, "u1")
	require.NoError(t, err)
	n, err = f.repos.Pages.RestoreSubtree(ctx(), 1, child.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	childNow, err := f.repos.Pages.Get(ctx(), 1, child.ID)
	require.NoError(t, err)
	require.Nil(t, childNow.ParentID)

	require.ErrorIs(t, func() error { _, err := f.repos.Pages.RestoreSubtree(ctx(), 1, sibling.ID); return err }(),
		ErrNotFound, "a live page cannot be restored")

	// Purge: root (trashed earlier) goes; FK cascade removes what is still under it.
	purged, err := f.repos.Pages.PurgeTrash(ctx(), 1, time.Now().Add(time.Second), 10)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{root.ID, grand.ID}, purged)
	_, err = f.repos.Pages.GetAny(ctx(), 1, root.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.repos.Pages.GetAny(ctx(), 1, grand.ID)
	require.ErrorIs(t, err, ErrNotFound, "cascade removed the trashed grandchild")
	_, err = f.repos.Pages.Get(ctx(), 1, child.ID)
	require.NoError(t, err, "the restored child survives because it was re-attached at the root")

	count, err := f.repos.Pages.CountLive(ctx(), 1, f.space.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestPageSpacePaging(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		f.page(t, "P", nil, "a"+string(rune('0'+i)))
	}
	seen := map[string]bool{}
	after := ""
	for {
		batch, err := f.repos.Pages.ListSpaceSummaries(ctx(), 1, f.space.ID, 2, after)
		require.NoError(t, err)
		if len(batch) == 0 {
			break
		}
		for _, p := range batch {
			require.False(t, seen[p.ID], "no page is returned twice")
			seen[p.ID] = true
		}
		after = batch[len(batch)-1].ID
	}
	require.Len(t, seen, 5)
}
