package docs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/events"
)

// The indexer's own behaviour, without a database: which events queue a page
// and how, and that the loop survives a failing or panicking queue and stops.

type queued struct {
	tenantID uint64
	pageID   string
	subtree  bool
	delay    time.Duration
}

// fakeQueue stands in for the page service.
type fakeQueue struct {
	mu        sync.Mutex
	queued    []queued
	queueErr  error
	processed int
	// batches is what successive ProcessIndexQueue calls report.
	batches   []int
	panicNext bool
	requeued  int
}

func (f *fakeQueue) QueueIndex(_ context.Context, tenantID uint64, pageID string, subtree bool,
	delay time.Duration,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queued = append(f.queued, queued{tenantID, pageID, subtree, delay})
	return f.queueErr
}

func (f *fakeQueue) ProcessIndexQueue(context.Context, int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed++
	if f.panicNext {
		f.panicNext = false
		panic("boom")
	}
	if len(f.batches) == 0 {
		return 0, nil
	}
	n := f.batches[0]
	f.batches = f.batches[1:]
	return n, nil
}

func (f *fakeQueue) RequeueIndex(context.Context, int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requeued++
	return 0, nil
}

func newTestIndexer(q *fakeQueue) *Indexer {
	ix := newIndexer(q, events.NewMemoryBus(), time.Minute)
	ix.tick = time.Hour
	return ix
}

func TestEditsAreQueuedWithTheDebounce(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	for _, typ := range []events.Type{events.PageContent, events.PageMeta, events.PageReplaced} {
		ix.onEvent(events.Event{Type: typ, TenantID: 1, PageID: "page-" + string(typ)})
	}

	require.Len(t, q.queued, 3)
	for _, got := range q.queued {
		assert.False(t, got.subtree, got.pageID)
		assert.Equal(t, time.Minute, got.delay, got.pageID)
	}
}

// Moving, restricting or trashing a page changes what everything below it may
// show, and takes content away from readers: it is queued at once, with its
// subtree.
func TestChangesThatTakeContentAwayAreQueuedAtOnceWithTheirSubtree(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	for _, typ := range []events.Type{events.PageMoved, events.PageAccess, events.PageDeleted} {
		ix.onEvent(events.Event{Type: typ, TenantID: 1, PageID: "page-" + string(typ)})
	}

	require.Len(t, q.queued, 3)
	for _, got := range q.queued {
		assert.True(t, got.subtree, got.pageID)
		assert.Zero(t, got.delay, got.pageID)
	}
}

// Nothing is being taken away by a restore, so it may wait; but the pages below
// come back with it.
func TestARestoreQueuesTheSubtreeWithoutHurrying(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	ix.onEvent(events.Event{Type: events.PageRestored, TenantID: 1, PageID: "page-1"})

	require.Len(t, q.queued, 1)
	assert.True(t, q.queued[0].subtree)
	assert.Equal(t, time.Minute, q.queued[0].delay)
}

func TestTheTenantTravelsWithTheRequest(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	ix.onEvent(events.Event{Type: events.PageContent, TenantID: 42, PageID: "page-1"})

	assert.EqualValues(t, 42, q.queued[0].tenantID)
}

func TestUnrelatedEventsAreIgnored(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	for _, typ := range []events.Type{
		events.CommentChanged, events.NotificationCreated, events.LabelChanged,
		events.SpaceUpdated, events.PageLease, events.PagePurged,
	} {
		ix.onEvent(events.Event{Type: typ, TenantID: 1, PageID: "page-1"})
	}

	assert.Empty(t, q.queued)
}

func TestAnEventWithNoPageIsIgnored(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)

	ix.onEvent(events.Event{Type: events.PageContent, TenantID: 1})

	assert.Empty(t, q.queued)
}

// A database that is down must not take the bus goroutine with it.
func TestAFailingQueueWriteDoesNotPanic(t *testing.T) {
	q := &fakeQueue{queueErr: errors.New("database down")}
	ix := newTestIndexer(q)

	assert.NotPanics(t, func() {
		ix.onEvent(events.Event{Type: events.PageContent, TenantID: 1, PageID: "page-1"})
	})
}

func TestNegativeAndZeroDebounceMeanImmediateAndDefault(t *testing.T) {
	assert.Equal(t, defaultDebounce, newIndexer(&fakeQueue{}, events.NewMemoryBus(), 0).debounce)
	assert.Zero(t, newIndexer(&fakeQueue{}, events.NewMemoryBus(), -1).debounce)
}

// A backlog is worked through in one go, not one batch per tick.
func TestADrainKeepsGoingWhileBatchesAreFull(t *testing.T) {
	q := &fakeQueue{batches: []int{indexBatch, indexBatch, 3}}
	ix := newTestIndexer(q)

	ix.drain(context.Background())

	assert.Equal(t, 3, q.processed, "two full batches, then the short one that ends it")
}

// A panic in one batch must not stop indexing for ever.
func TestAPanickingBatchIsContained(t *testing.T) {
	q := &fakeQueue{panicNext: true}
	ix := newTestIndexer(q)

	assert.NotPanics(t, func() { ix.drain(context.Background()) })
	ix.drain(context.Background())

	assert.Equal(t, 2, q.processed, "the next round runs")
}

// The first round after a start also looks for pages nobody queued.
func TestTheLoopRequeuesOnStartAndStopsWhenAsked(t *testing.T) {
	q := &fakeQueue{}
	ix := newTestIndexer(q)
	ix.tick = 5 * time.Millisecond
	ix.Start(context.Background())

	require.Eventually(t, func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		return q.requeued >= 1 && q.processed >= 1
	}, 2*time.Second, 5*time.Millisecond)

	done := make(chan struct{})
	go func() {
		ix.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
}

func TestANilIndexerIsInert(t *testing.T) {
	var ix *Indexer
	ix.Start(context.Background())
	ix.Stop()
}

func TestStopIsSafeWithoutStartAndTwice(t *testing.T) {
	ix := newTestIndexer(&fakeQueue{})
	done := make(chan struct{})
	go func() {
		ix.Stop()
		ix.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked on an indexer that was never started")
	}
}

func TestNewIndexerRefusesMissingDependencies(t *testing.T) {
	assert.Nil(t, NewIndexer(nil, events.NewMemoryBus(), time.Minute))
}
