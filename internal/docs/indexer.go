package docs

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// The indexer: what turns an edit into a knowledge-base update.
//
// It has two halves. Listening to the module's own event bus, it writes down
// which pages need another look (QueueIndex; a row in the database, not a
// note in memory). Working through the queue, it takes due pages and mirrors
// them (ProcessIndexQueue), and now and then asks for pages that changed
// without anyone saying so (RequeueIndex). Every instance runs both halves;
// the queue is what keeps them from stepping on each other, and from losing
// work when one of them restarts.
//
// Edits are debounced, because re-indexing runs a document through chunking and
// embedding. Somebody writing a page saves it every few seconds for half an
// hour; without the debounce that is several hundred rebuilds of a document
// nobody has finished writing, each one costing money on a hosted model.
//
// Which events matter, and why each one:
//
//   - PageContent: the body changed, so the mirror is stale.
//   - PageMeta: the title, the exclusion flag or the lock changed. A title is
//     indexed, and a page being excluded has to LEAVE the knowledge base.
//   - PageReplaced: the body was replaced wholesale (a restored revision).
//   - PageAccess: somebody restricted or unrestricted a page. This is the
//     one that must not be missed — restricting an indexed page has to
//     remove it, or the restriction is cosmetic. A restriction is inherited,
//     so the pages below are looked at too.
//   - PageMoved: the page has new ancestors, so the restrictions it inherits
//     differ, and it may now be in a space bound to another knowledge base.
//     The pages below move with it.
//   - PageDeleted, PageRestored: trashing a page takes its children with it,
//     and restoring brings them back.
//
// Events that take content away from readers (restricting, moving, trashing)
// are due at once: a delay there is time during which people who should no
// longer see a page can still find it.
//
// Not handled, on purpose: PagePurged (the page's queue row goes with it, and
// its entry went when it was trashed), and SpaceUpdated (a space bound to a
// knowledge base, or unbound from one, is found by the periodic RequeueIndex
// without re-queuing a whole space on every rename).
//
// Everything is funnelled through the same synchronisation, which re-derives
// the decision from scratch. The events say "look at this page again", never
// "do this to it" — so an event that arrives late, twice, or out of order
// costs one redundant look rather than a wrong answer.

// indexQueue is what the indexer needs of the page service, so a test can stand
// in for it.
type indexQueue interface {
	QueueIndex(ctx context.Context, tenantID uint64, pageID string, subtree bool, delay time.Duration) error
	ProcessIndexQueue(ctx context.Context, limit int) (int, error)
	RequeueIndex(ctx context.Context, limit int) (int64, error)
}

const (
	// DefaultIndexTick is how often the workers look for due pages. It is
	// independent of the debounce: a short tick with a long debounce is cheap
	// (one indexed query), and means a page becomes searchable promptly once its
	// author stops typing.
	DefaultIndexTick = 10 * time.Second

	// defaultDebounce is how long an edited page is left alone.
	defaultDebounce = time.Minute

	// indexBatch is how many pages one worker takes at a time. Small, because
	// each is a round trip to an embedding service.
	indexBatch = 10

	// requeueEvery and requeueBatch bound the net that finds pages nobody
	// queued: it runs every few minutes and examines a limited number of pages,
	// so a large backlog is worked through over several rounds and never as one
	// burst of embedding calls.
	requeueEvery = 5 * time.Minute
	requeueBatch = 200

	// queueWriteTimeout bounds the write an event makes, so a slow database
	// cannot pin the bus goroutine that delivers it.
	queueWriteTimeout = 10 * time.Second
)

// Indexer keeps knowledge bases in step with pages.
type Indexer struct {
	pages    indexQueue
	bus      events.Bus
	debounce time.Duration

	// tick is how often the queue is worked. A field so tests can shorten it.
	tick time.Duration

	stop      chan struct{}
	done      chan struct{}
	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewIndexer builds the indexer. A nil bus or page service yields nil, which
// leaves the module working with no indexing at all. A zero debounce means the
// default of a minute; a negative one means edits are indexed at once, which
// is what tests and impatient operators want.
func NewIndexer(pages *service.PageService, bus events.Bus, debounce time.Duration) *Indexer {
	if pages == nil || bus == nil {
		return nil
	}
	return newIndexer(pages, bus, debounce)
}

func newIndexer(pages indexQueue, bus events.Bus, debounce time.Duration) *Indexer {
	switch {
	case debounce == 0:
		debounce = defaultDebounce
	case debounce < 0:
		debounce = 0
	}
	return &Indexer{
		pages: pages, bus: bus, debounce: debounce, tick: DefaultIndexTick,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

// Start subscribes to the bus and begins working the queue.
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

// onEvent decides whether an event means a page needs another look, and if so
// writes that down.
func (ix *Indexer) onEvent(ev events.Event) {
	var subtree bool
	delay := ix.debounce
	switch ev.Type {
	case events.PageContent, events.PageReplaced:
		// Look again, after the author has stopped typing.
	case events.PageMeta:
		// A page excluded from the knowledge base is taking content away from
		// readers of AI answers, so a metadata change is not left to wait; the
		// other changes it carries (a title, a lock) cost one cheap look.
		delay = 0
	case events.PageAccess, events.PageMoved, events.PageDeleted:
		// Look again, now, and at everything below.
		subtree, delay = true, 0
	case events.PageRestored:
		// The pages below come back with it. Nothing is being taken away, so
		// there is no need to hurry.
		subtree = true
	default:
		return
	}
	if ev.PageID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), queueWriteTimeout)
	defer cancel()
	// A failed write is not retried here: the periodic RequeueIndex finds a page
	// that changed after it was last looked at, and only the delay is lost.
	if err := ix.pages.QueueIndex(ctx, ev.TenantID, ev.PageID, subtree, delay); err != nil {
		logger.Warnf(ctx, "[docs] queuing page %s for indexing failed: %v", ev.PageID, err)
	}
}

func (ix *Indexer) loop(ctx context.Context) {
	defer close(ix.done)
	ticker := time.NewTicker(ix.tick)
	defer ticker.Stop()

	// The first round also looks for pages nobody queued, so a restart resumes
	// what the previous process left and a knowledge base bound while the
	// application was down starts filling.
	nextRequeue := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ix.stop:
			return
		case <-ticker.C:
			if !time.Now().Before(nextRequeue) {
				ix.requeue(ctx)
				nextRequeue = time.Now().Add(requeueEvery)
			}
			ix.drain(ctx)
		}
	}
}

// drain works the queue until it is empty or a round takes nothing, so a
// backlog does not wait one tick per batch.
func (ix *Indexer) drain(ctx context.Context) {
	for {
		select {
		case <-ix.stop:
			return
		case <-ctx.Done():
			return
		default:
		}
		n := ix.processOnce(ctx)
		if n < indexBatch {
			return
		}
	}
}

// processOnce takes one batch, surviving its panics.
//
// A panic here would take down the loop and silently stop all indexing, which
// is the kind of failure nobody notices until search results are a month stale.
// The pages of the batch stay claimed until their lease expires and are then
// taken again.
func (ix *Indexer) processOnce(ctx context.Context) (taken int) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] indexing panicked: %v", r)
			taken = 0
		}
	}()
	n, err := ix.pages.ProcessIndexQueue(ctx, indexBatch)
	if err != nil {
		logger.Warnf(ctx, "[docs] working the indexing queue failed: %v", err)
		return 0
	}
	return n
}

func (ix *Indexer) requeue(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] looking for stale pages panicked: %v", r)
		}
	}()
	n, err := ix.pages.RequeueIndex(ctx, requeueBatch)
	if err != nil {
		logger.Warnf(ctx, "[docs] looking for stale pages failed: %v", err)
		return
	}
	if n > 0 {
		logger.Infof(ctx, "[docs] %d page(s) queued for indexing by the periodic check", n)
	}
}

// Stop ends the loop and waits for any batch in flight.
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
