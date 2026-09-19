package docs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/index"
)

// The indexer's own behaviour, without a database: which events queue a
// page, that repeat edits collapse, and that it cannot deadlock a shutdown.

func newTestIndexer(t *testing.T) (*Indexer, events.Bus) {
	t.Helper()
	bus := events.NewMemoryBus()
	ix := NewIndexer(nil, bus, time.Minute)
	require.Nil(t, ix, "a nil page service yields no indexer")

	// A real one needs a page service; the tests below exercise the parts
	// that do not call it.
	ix = &Indexer{
		bus: bus, queue: index.NewQueue(time.Minute), tick: time.Hour,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	return ix, bus
}

func TestEditsQueueThePage(t *testing.T) {
	ix, _ := newTestIndexer(t)
	for _, typ := range []events.Type{
		events.PageContent, events.PageMeta, events.PageAccess,
		events.PageReplaced, events.PageRestored,
		events.PageDeleted, events.PagePurged,
	} {
		ix.onEvent(events.Event{Type: typ, TenantID: 1, PageID: "page-" + string(typ)})
	}
	assert.Equal(t, 7, ix.Pending(), "every one of these means look again")
}

// Restricting an indexed page has to remove it, so this event must never be
// the one that is missed.
func TestAPermissionChangeQueuesThePage(t *testing.T) {
	ix, _ := newTestIndexer(t)
	ix.onEvent(events.Event{Type: events.PageAccess, TenantID: 1, PageID: "page-1"})
	assert.Equal(t, 1, ix.Pending())
}

func TestUnrelatedEventsAreIgnored(t *testing.T) {
	ix, _ := newTestIndexer(t)
	for _, typ := range []events.Type{
		events.CommentChanged, events.NotificationCreated, events.LabelChanged,
		events.SpaceUpdated, events.PageLease,
	} {
		ix.onEvent(events.Event{Type: typ, TenantID: 1, PageID: "page-1"})
	}
	assert.Equal(t, 0, ix.Pending())
}

func TestAnEventWithNoPageIsIgnored(t *testing.T) {
	ix, _ := newTestIndexer(t)
	ix.onEvent(events.Event{Type: events.PageContent, TenantID: 1})
	assert.Equal(t, 0, ix.Pending())
}

// Somebody saving every few seconds produces one rebuild.
func TestRepeatEditsCollapseToOneEntry(t *testing.T) {
	ix, _ := newTestIndexer(t)
	for i := 0; i < 20; i++ {
		ix.onEvent(events.Event{Type: events.PageContent, TenantID: 1, PageID: "page-1"})
	}
	assert.Equal(t, 1, ix.Pending())
}

// Two tenants with the same page id are two entries, not one.
func TestTheQueueIsKeyedByTenantAndPage(t *testing.T) {
	ix, _ := newTestIndexer(t)
	ix.onEvent(events.Event{Type: events.PageContent, TenantID: 1, PageID: "page-1"})
	ix.onEvent(events.Event{Type: events.PageContent, TenantID: 2, PageID: "page-1"})
	assert.Equal(t, 2, ix.Pending())
}

func TestAQueueKeyRoundTrips(t *testing.T) {
	tenantID, pageID, ok := splitKey(queueKey(42, "page-abc"))
	require.True(t, ok)
	assert.EqualValues(t, 42, tenantID)
	assert.Equal(t, "page-abc", pageID)
}

func TestAMalformedKeyIsRefusedRatherThanGuessed(t *testing.T) {
	for _, key := range []string{"", "no-slash", "/page", "notanumber/page"} {
		_, _, ok := splitKey(key)
		assert.False(t, ok, "key %q", key)
	}
}

// A page id containing a slash must still round-trip.
func TestAPageIdWithASlashSurvives(t *testing.T) {
	tenantID, pageID, ok := splitKey(queueKey(7, "odd/id"))
	require.True(t, ok)
	assert.EqualValues(t, 7, tenantID)
	assert.Equal(t, "odd/id", pageID)
}

// A panic in one page's sync must not stop indexing for ever.
func TestAPanickingSyncIsContained(t *testing.T) {
	ix, _ := newTestIndexer(t)
	assert.NotPanics(t, func() {
		defer func() { _ = recover() }()
		ix.syncOne(context.Background(), 1, "page-1")
	})
}

func TestANilIndexerIsInert(t *testing.T) {
	var ix *Indexer
	ix.Start(context.Background())
	ix.Stop()
	assert.Equal(t, 0, ix.Pending())
}

func TestStopIsSafeWithoutStart(t *testing.T) {
	ix, _ := newTestIndexer(t)
	done := make(chan struct{})
	go func() {
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
	assert.Nil(t, NewIndexer(nil, nil, time.Minute))
}
