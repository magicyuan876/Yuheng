package docs

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/index"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// The indexer: what turns an edit into a knowledge-base update.
//
// It listens to the module's own event bus and holds each touched page for a
// debounce period before re-indexing it, because re-indexing runs a document
// through chunking and embedding. Somebody writing a page saves it every few
// seconds for half an hour; without the debounce that is several hundred
// rebuilds of a document nobody has finished writing, each one costing money
// on a hosted model.
//
// Which events matter, and why each one:
//
//   - PageContent: the body changed, so the mirror is stale.
//   - PageMeta: the title, the draft flag or the lock changed. A title is
//     indexed, and a page becoming a draft has to LEAVE the knowledge base.
//   - PageAccess: somebody restricted or unrestricted a page. This is the
//     one that must not be missed — restricting an indexed page has to
//     remove it, or the restriction is cosmetic. A restriction is inherited,
//     so the pages below are looked at too.
//   - PageMoved: the page has new ancestors, so the restrictions it inherits
//     differ, and it may now be in a space bound to another knowledge base.
//     The pages below move with it.
//   - PageDeleted, PageRestored: trashing a page takes its children with it,
//     and restoring brings them back.
//   - PagePurged: the page is gone; so is its entry.
//   - SpaceUpdated: a space may have been bound to a knowledge base, or
//     unbound from one.
//
// Events that take content away from readers (restricting, moving, trashing)
// skip the debounce: a delay there is time during which people who should no
// longer see a page can still find it. Edits keep the debounce.
//
// Everything is funnelled through SyncPageToKnowledge, which re-derives the
// decision from scratch. The events say "look at this page again", never
// "do this to it" — so an event that arrives late, twice, or out of order
// costs one redundant look rather than a wrong answer.

// pageSyncer is what the indexer needs of the page service, so a test can stand
// in for it.
type pageSyncer interface {
	SyncPageToKnowledge(ctx context.Context, tenantID uint64, pageID string) (*service.IndexResult, error)
	SyncSubtreeToKnowledge(ctx context.Context, tenantID uint64, pageID string) ([]*service.IndexResult, error)
}

// Indexer keeps knowledge bases in step with pages.
type Indexer struct {
	pages pageSyncer
	queue *index.Queue
	bus   events.Bus

	// tick is how often the queue is drained. Independent of the debounce:
	// a short tick with a long debounce is cheap, and means a page becomes
	// searchable promptly once its author stops typing.
	tick time.Duration

	mu sync.Mutex
	// subtree holds the queue keys whose pages must be synced together with
	// everything beneath them. Guarded by mu.
	subtree   map[string]struct{}
	stop      chan struct{}
	done      chan struct{}
	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// DefaultIndexTick is how often the debounce queue is drained.
const DefaultIndexTick = 10 * time.Second

// NewIndexer builds the indexer. A nil bus or page service yields nil, which
// leaves the module working with no indexing at all.
func NewIndexer(pages *service.PageService, bus events.Bus, debounce time.Duration) *Indexer {
	if pages == nil || bus == nil {
		return nil
	}
	return &Indexer{
		pages: pages, bus: bus, queue: index.NewQueue(debounce), subtree: map[string]struct{}{},
		tick: DefaultIndexTick,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

// Start subscribes to the bus and begins draining the queue.
func (ix *Indexer) Start(ctx context.Context) {
	if ix == nil {
		return
	}
	ix.startOnce.Do(func() {
		ix.started.Store(true)
		ix.bus.Subscribe(0, ix.onEvent)
		go ix.loop(ctx)
	})
}

// onEvent decides whether an event means a page needs another look.
//
// Deliberately cheap: it takes a lock, writes a map entry and returns. The
// bus delivers on its own goroutines and a slow subscriber would hold up
// every other consumer of the same event.
func (ix *Indexer) onEvent(ev events.Event) {
	pageID := ev.PageID
	var wholeSubtree, urgent bool
	switch ev.Type {
	case events.PageContent, events.PageMeta, events.PageReplaced:
		// Look again, after the author has stopped typing.
	case events.PageAccess, events.PageMoved, events.PageDeleted:
		// Look again, now, and at everything below.
		wholeSubtree, urgent = true, true
	case events.PageRestored:
		// The pages below come back with it. Nothing is being taken away, so
		// there is no need to hurry.
		wholeSubtree = true
	case events.PagePurged:
		// Also look again: the sync removes the entry of a page that is
		// gone. Handled by the same path rather than a special case,
		// because "decide from scratch" is the property that makes late and
		// duplicate events harmless.
	case events.SpaceUpdated:
		// A binding may have appeared or gone. There is no page to queue,
		// and re-indexing a whole space on a settings change would be a
		// surprise, so this is left to the explicit rebuild endpoint.
		return
	default:
		return
	}
	if pageID == "" {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	// Keyed by tenant AND page. The bus carries every tenant's events, and a
	// bare page id would let one tenant's queue entry be synced with another
	// tenant's id — the module filters by tenant in every query, so that
	// would be a lookup that finds nothing, but a queue that can express a
	// cross-tenant operation at all is one to avoid.
	key := queueKey(ev.TenantID, pageID)
	if wholeSubtree {
		ix.subtree[key] = struct{}{}
	}
	if urgent {
		ix.queue.Urgent(key, time.Now())
		return
	}
	ix.queue.Touch(key, time.Now())
}

// queueKey and splitKey pair a tenant with a page for the debounce queue.
func queueKey(tenantID uint64, pageID string) string {
	return strconv.FormatUint(tenantID, 10) + "/" + pageID
}

func splitKey(key string) (uint64, string, bool) {
	slash := strings.IndexByte(key, '/')
	if slash <= 0 {
		return 0, "", false
	}
	tenantID, err := strconv.ParseUint(key[:slash], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return tenantID, key[slash+1:], true
}

func (ix *Indexer) loop(ctx context.Context) {
	defer close(ix.done)
	ticker := time.NewTicker(ix.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ix.stop:
			return
		case <-ticker.C:
			ix.drain(ctx)
		}
	}
}

// drain re-indexes every page whose debounce has expired.
func (ix *Indexer) drain(ctx context.Context) {
	ix.mu.Lock()
	due := ix.queue.Due(time.Now())
	whole := make(map[string]bool, len(due))
	for _, key := range due {
		if _, ok := ix.subtree[key]; ok {
			whole[key] = true
			delete(ix.subtree, key)
		}
	}
	ix.mu.Unlock()

	for _, key := range due {
		tenantID, pageID, ok := splitKey(key)
		if !ok {
			continue
		}
		ix.syncOne(ctx, tenantID, pageID, whole[key])
	}
}

// syncOne indexes one page, surviving its panics.
//
// A panic here would take down the event loop and silently stop all
// indexing, which is the kind of failure nobody notices until search results
// are a month stale.
func (ix *Indexer) syncOne(ctx context.Context, tenantID uint64, pageID string, wholeSubtree bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] indexing page %s panicked: %v", pageID, r)
		}
	}()
	if wholeSubtree {
		if _, err := ix.pages.SyncSubtreeToKnowledge(ctx, tenantID, pageID); err != nil {
			logger.Warnf(ctx, "[docs] indexing the pages under %s failed: %v", pageID, err)
		}
		return
	}
	if _, err := ix.pages.SyncPageToKnowledge(ctx, tenantID, pageID); err != nil {
		logger.Warnf(ctx, "[docs] indexing page %s failed: %v", pageID, err)
	}
}

// Stop ends the loop and waits for any sync in flight.
//
// Safe on an indexer that was never started, and safe to call twice.
func (ix *Indexer) Stop() {
	if ix == nil {
		return
	}
	ix.stopOnce.Do(func() { close(ix.stop) })
	if ix.started.Load() {
		<-ix.done
	}
}

// Pending is how many pages are waiting, for tests and diagnostics.
func (ix *Indexer) Pending() int {
	if ix == nil {
		return 0
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.queue.Len()
}
