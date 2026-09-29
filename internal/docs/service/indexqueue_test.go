package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/index"
)

func TestTheRetryDelayGrowsAndIsCapped(t *testing.T) {
	assert.Equal(t, 30*time.Second, indexRetryDelay(0))
	assert.Equal(t, time.Minute, indexRetryDelay(1))
	assert.Equal(t, 2*time.Minute, indexRetryDelay(2))
	assert.Equal(t, time.Hour, indexRetryDelay(7), "30s doubled seven times is past the cap")
	assert.Equal(t, time.Hour, indexRetryDelay(500), "and stays there, without overflowing")
}

// An edit waits out its debounce; the worker takes nothing before then.
func TestAnEditedPageIsLeftAloneUntilItsDebounceExpires(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")

	require.NoError(t, p.svc.Pages.QueueIndex(ctx(), 1, page.ID, false, time.Hour))
	n, err := p.svc.Pages.ProcessIndexQueue(ctx(), 10)

	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Zero(t, kb.count())
}

// A page whose embedding fails is not lost: it is scheduled for a retry, the
// reason is kept, and once the service is back the page arrives.
func TestAFailedPageIsRetriedAndTheReasonKept(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	kb.failCreate = true

	require.NoError(t, p.svc.Pages.QueueIndex(ctx(), 1, page.ID, false, 0))
	n, err := p.svc.Pages.ProcessIndexQueue(ctx(), 10)
	require.NoError(t, err, "one page failing is not the queue failing")
	assert.Equal(t, 1, n)

	st, err := p.repos.IndexQueue.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Attempts)
	assert.Contains(t, st.LastError, "embedding service unavailable")
	require.NotNil(t, st.DueAt, "still pending")
	assert.True(t, st.DueAt.After(time.Now()), "but not at once: the retry backs off")
	assert.Zero(t, kb.count())

	// The service comes back and the retry falls due.
	kb.failCreate = false
	require.NoError(t, p.repos.IndexQueue.Mark(ctx(), 1, []string{page.ID}, 0))
	require.NoError(t, p.drainIndex())

	assert.Equal(t, 1, kb.count())
	st, err = p.repos.IndexQueue.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Zero(t, st.Attempts)
	assert.Empty(t, st.LastError)
	assert.Nil(t, st.DueAt)
}

// Several workers, on one instance or many, never make two entries for a page.
func TestConcurrentWorkersDoNotDuplicateEntries(t *testing.T) {
	p, kb := newIndexEnv(t)
	const pages = 12
	ids := make([]string, 0, pages)
	for i := 0; i < pages; i++ {
		page := p.create(t, p.alice, nil, fmt.Sprintf("页面%d", i))
		p.write(t, p.alice, page.ID, fmt.Sprintf("足够长的正文内容在这里：%d", i))
		ids = append(ids, page.ID)
	}
	require.NoError(t, p.repos.IndexQueue.Mark(ctx(), 1, ids, 0))

	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- p.drainIndex()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, pages, kb.count(), "one entry per page")
	for _, id := range ids {
		assert.True(t, p.indexed(t, id), id)
	}
}

// An administrator's explicit rebuild and a worker never work on one page at
// once.
func TestASyncOfAPageAWorkerHoldsIsLeftToTheWorker(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	_, ok, err := p.repos.IndexQueue.ClaimPage(ctx(), 1, page.ID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)

	require.NoError(t, err)
	assert.Equal(t, ReasonBusy, res.Reason)
	assert.Zero(t, kb.count())
}

// Binding a knowledge base to a space that already has pages fills it, with
// nobody pressing rebuild.
func TestBindingAKnowledgeBaseIndexesTheExistingPages(t *testing.T) {
	kb := newFakeKnowledge()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Knowledge = kb
		d.KnowledgeBases = fakeKBs{"kb-1": 1}
	})
	page := p.create(t, p.alice, nil, "早就写好的页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	require.NoError(t, p.drainIndex())
	assert.Zero(t, kb.count(), "no binding yet")

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, strPtr("kb-1"), nil)
	require.NoError(t, err)
	n, err := p.svc.Pages.RequeueIndex(ctx(), 100)
	require.NoError(t, err)
	assert.Positive(t, n)
	require.NoError(t, p.drainIndex())

	assert.Equal(t, 1, kb.count())
	assert.True(t, p.indexed(t, page.ID))
}

// An edit whose event was lost (the process died between the write and the
// queue) is found by the periodic check.
func TestALostEditEventIsFoundByThePeriodicCheck(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "第一版的内容在这里。")
	p.indexAll(t, page.ID)
	require.Equal(t, 1, kb.count())

	// The body changes and nobody queues it.
	p.write(t, p.alice, page.ID, "第二版的内容完全不同。")
	require.NoError(t, p.drainIndex())
	assert.Contains(t, kb.bodies()[0], "第一版", "nothing was queued, so nothing has moved")

	n, err := p.svc.Pages.RequeueIndex(ctx(), 100)
	require.NoError(t, err)
	assert.Positive(t, n)
	require.NoError(t, p.drainIndex())

	assert.Contains(t, kb.bodies()[0], "第二版")
}

// A page nobody touches does not keep being requeued: once looked at, it stays
// quiet until it changes.
func TestALookedAtPageIsNotRequeuedAgain(t *testing.T) {
	p, _ := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	require.NoError(t, p.svc.Pages.QueueIndex(ctx(), 1, page.ID, false, 0))
	require.NoError(t, p.drainIndex())

	n, err := p.svc.Pages.RequeueIndex(ctx(), 100)

	require.NoError(t, err)
	assert.Zero(t, n)
}

// A restriction whose event was lost is caught by the daily re-verification, in
// the direction that matters: the entry is removed.
func TestARestrictionWhoseEventWasLostIsCaughtByReverification(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "先公开后收回")
	p.write(t, p.alice, page.ID, "一开始所有人都能看到的内容。")
	p.indexAll(t, page.ID)
	require.Equal(t, 1, kb.count())

	p.cut(t, p.alice, page.ID) // restricted; no event reaches the indexer
	require.NoError(t, p.repos.DB().Exec(
		`UPDATE docs_index_state SET indexed_at = NOW() - INTERVAL '2 days' WHERE page_id = ?`, page.ID).Error)
	n, err := p.svc.Pages.RequeueIndex(ctx(), 100)
	require.NoError(t, err)
	assert.Positive(t, n)
	require.NoError(t, p.drainIndex())

	assert.Zero(t, kb.count(), "the restricted page left the knowledge base")
	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Equal(t, index.ReasonRestricted, res.Reason)
}
