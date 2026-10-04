package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

func (f *fixture) indexState(t *testing.T, pageID string) (dueIn time.Duration, hasDue bool, seq int64) {
	t.Helper()
	st, err := f.repos.IndexQueue.Get(ctx(), 1, pageID)
	require.NoError(t, err)
	if st.DueAt == nil {
		return 0, false, st.Seq
	}
	return time.Until(*st.DueAt), true, st.Seq
}

func TestMarkWithADelayDebouncesAndAZeroDelayIsUrgent(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")

	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, time.Minute))
	dueIn, has, _ := f.indexState(t, page.ID)
	require.True(t, has)
	assert.InDelta(t, time.Minute.Seconds(), dueIn.Seconds(), 5)

	// A later edit pushes the deadline out.
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 2*time.Minute))
	dueIn, _, seq := f.indexState(t, page.ID)
	assert.InDelta(t, (2 * time.Minute).Seconds(), dueIn.Seconds(), 5)
	assert.EqualValues(t, 2, seq)

	// An urgent request pulls it forward ...
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	dueIn, _, _ = f.indexState(t, page.ID)
	assert.LessOrEqual(t, dueIn, time.Second)

	// ... and an edit after it does not push it back.
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, time.Minute))
	dueIn, _, _ = f.indexState(t, page.ID)
	assert.LessOrEqual(t, dueIn, time.Second, "an overdue page stays overdue")
}

func TestMarkSubtreeReachesEveryDescendant(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, "Root", nil, "a0")
	child := f.page(t, "Child", &root.ID, "a0")
	grand := f.page(t, "Grand", &child.ID, "a0")
	sibling := f.page(t, "Sibling", nil, "a1")

	require.NoError(t, f.repos.IndexQueue.MarkSubtree(ctx(), 1, root.ID, 0))

	for _, id := range []string{root.ID, child.ID, grand.ID} {
		_, has, _ := f.indexState(t, id)
		assert.True(t, has, id)
	}
	_, err := f.repos.IndexQueue.Get(ctx(), 1, sibling.ID)
	assert.True(t, errors.Is(err, ErrNotFound), "a page outside the subtree is untouched")
}

func TestMarkSpaceReachesEveryPageOfTheSpaceAndNoOther(t *testing.T) {
	f := newFixture(t)
	root := f.page(t, "Root", nil, "a0")
	child := f.page(t, "Child", &root.ID, "a0")
	trashed := f.page(t, "Trashed", nil, "a1")
	_, err := f.repos.Pages.SoftDeleteSubtree(ctx(), 1, trashed.ID, "u1")
	require.NoError(t, err)
	elsewhere := &model.Page{
		TenantID: 1, SpaceID: f.other.ID, Position: "a0", Title: "Elsewhere", ShortID: "zz0000",
		Content: model.JSON(`{"type":"doc","content":[{"type":"paragraph"}]}`), CreatorID: ptr("u1"),
	}
	require.NoError(t, f.repos.Pages.Create(ctx(), elsewhere))
	// One page is already waiting with a debounce; the space-wide request
	// is urgent and pulls it forward.
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{child.ID}, time.Hour))

	require.NoError(t, f.repos.IndexQueue.MarkSpace(ctx(), 1, f.space.ID, 0))

	for _, id := range []string{root.ID, child.ID, trashed.ID} {
		dueIn, has, _ := f.indexState(t, id)
		assert.True(t, has, id)
		assert.LessOrEqual(t, dueIn, time.Second, id)
	}
	_, err = f.repos.IndexQueue.Get(ctx(), 1, elsewhere.ID)
	assert.True(t, errors.Is(err, ErrNotFound), "a page of another space is untouched")
}

func TestClaimGivesEachDuePageToOneWorker(t *testing.T) {
	f := newFixture(t)
	a := f.page(t, "A", nil, "a0")
	b := f.page(t, "B", nil, "a1")
	later := f.page(t, "Later", nil, "a2")
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{a.ID, b.ID}, 0))
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{later.ID}, time.Hour))

	first, err := f.repos.IndexQueue.Claim(ctx(), 10, time.Minute)
	require.NoError(t, err)
	assert.Len(t, first, 2, "only the due pages")

	second, err := f.repos.IndexQueue.Claim(ctx(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, second, "a claimed page is not handed out again while the lease holds")
}

func TestAnExpiredClaimIsClaimedAgain(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	first, err := f.repos.IndexQueue.Claim(ctx(), 1, -time.Second) // already expired
	require.NoError(t, err)
	require.Len(t, first, 1)

	again, err := f.repos.IndexQueue.Claim(ctx(), 1, time.Minute)
	require.NoError(t, err)
	assert.Len(t, again, 1, "a worker that died leaves its page to the next one")
}

func TestCompleteClearsWhatWasClaimedButNotWhatArrivedAfter(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	claims, err := f.repos.IndexQueue.Claim(ctx(), 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)

	// Somebody edits the page while it is being synchronised.
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	require.NoError(t, f.repos.IndexQueue.Complete(ctx(), claims[0], "hash-1"))

	_, has, _ := f.indexState(t, page.ID)
	assert.True(t, has, "the edit that arrived during the sync must not be lost")
	st, err := f.repos.IndexQueue.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Equal(t, "hash-1", st.IndexedHash)
	assert.Nil(t, st.ClaimedUntil)

	// Nothing arrives this time.
	claims, err = f.repos.IndexQueue.Claim(ctx(), 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.NoError(t, f.repos.IndexQueue.Complete(ctx(), claims[0], "hash-2"))
	_, has, _ = f.indexState(t, page.ID)
	assert.False(t, has)
}

func TestFailSchedulesARetryAndRemembersWhy(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")
	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	claims, err := f.repos.IndexQueue.Claim(ctx(), 1, time.Minute)
	require.NoError(t, err)

	require.NoError(t, f.repos.IndexQueue.Fail(ctx(), claims[0], errors.New("embedding service down"), time.Hour))

	st, err := f.repos.IndexQueue.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Attempts)
	assert.Equal(t, "embedding service down", st.LastError)
	require.NotNil(t, st.DueAt)
	assert.InDelta(t, time.Hour.Seconds(), time.Until(*st.DueAt).Seconds(), 5)
	assert.Nil(t, st.ClaimedUntil)
}

func TestClaimPageExcludesTheWorkersAndOtherCallers(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")

	claim, ok, err := f.repos.IndexQueue.ClaimPage(ctx(), 1, page.ID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, page.ID, claim.PageID)

	_, ok, err = f.repos.IndexQueue.ClaimPage(ctx(), 1, page.ID, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "a page somebody holds cannot be taken")

	require.NoError(t, f.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	workers, err := f.repos.IndexQueue.Claim(ctx(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, workers, "nor by a worker, even though it is due")

	require.NoError(t, f.repos.IndexQueue.Complete(ctx(), claim, "h"))
	_, ok, err = f.repos.IndexQueue.ClaimPage(ctx(), 1, page.ID, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "released once completed")
}

func TestClaimPageOfAMissingPageIsRefused(t *testing.T) {
	f := newFixture(t)

	_, ok, err := f.repos.IndexQueue.ClaimPage(ctx(), 1, "no-such-page", time.Minute)

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRequeueFindsPagesOfABoundSpaceThatNobodyMarked(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "A", nil, "a0")

	n, err := f.repos.IndexQueue.Requeue(ctx(), 100, 24*time.Hour)
	require.NoError(t, err)
	assert.Zero(t, n, "the space has no knowledge base, so nothing matters")

	kb := "kb-1"
	require.NoError(t, f.repos.Spaces.Update(ctx(), 1, f.space.ID, map[string]any{"knowledge_base_id": kb}))
	n, err = f.repos.IndexQueue.Requeue(ctx(), 100, 24*time.Hour)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	_, has, _ := f.indexState(t, page.ID)
	assert.True(t, has)
}

func TestRequeueFindsEditedPagesAndLongUnverifiedEntries(t *testing.T) {
	f := newFixture(t)
	edited := f.page(t, "Edited", nil, "a0")
	unverified := f.page(t, "Unverified", nil, "a1")
	fresh := f.page(t, "Fresh", nil, "a2")
	kb := "kb-1"
	require.NoError(t, f.repos.Spaces.Update(ctx(), 1, f.space.ID, map[string]any{"knowledge_base_id": kb}))
	for _, p := range []string{edited.ID, unverified.ID, fresh.ID} {
		claim, ok, err := f.repos.IndexQueue.ClaimPage(ctx(), 1, p, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.repos.IndexQueue.Complete(ctx(), claim, "h"))
	}
	// The space was bound before these were recorded; move its clock back so only
	// the page-level conditions are in play.
	require.NoError(t, f.repos.db.Exec(`UPDATE docs_spaces SET updated_at = NOW() - INTERVAL '1 day' WHERE id = ?`,
		f.space.ID).Error)
	require.NoError(t, f.repos.db.Exec(`UPDATE docs_pages SET updated_at = NOW() - INTERVAL '1 hour'`).Error)
	require.NoError(t, f.repos.db.Exec(`UPDATE docs_pages SET updated_at = NOW() WHERE id = ?`, edited.ID).Error)
	require.NoError(t, f.repos.db.Exec(`UPDATE docs_pages SET knowledge_id = 'entry' WHERE id IN (?, ?)`,
		unverified.ID, fresh.ID).Error)
	require.NoError(t, f.repos.db.Exec(
		`UPDATE docs_index_state SET indexed_at = NOW() - INTERVAL '2 days' WHERE page_id = ?`, unverified.ID).Error)

	n, err := f.repos.IndexQueue.Requeue(ctx(), 100, 24*time.Hour)
	require.NoError(t, err)

	assert.EqualValues(t, 2, n)
	_, has, _ := f.indexState(t, edited.ID)
	assert.True(t, has, "edited since it was last looked at")
	_, has, _ = f.indexState(t, unverified.ID)
	assert.True(t, has, "an entry not looked at for a day")
	_, has, _ = f.indexState(t, fresh.ID)
	assert.False(t, has, "recently verified and unchanged")
}
