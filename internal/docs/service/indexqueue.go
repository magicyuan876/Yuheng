package service

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
)

// The queue behind the mirror.
//
// Something that changes a page says so with QueueIndex, which writes a row and
// returns; workers on every instance take due pages with ProcessIndexQueue and
// do the slow part (rendering, chunking, embedding). Because the queue is a
// table, a restart forgets nothing, a failed page is retried with a growing
// delay instead of being dropped, and any number of instances can work at once
// without two of them taking the same page. RequeueIndex is the net under it
// all: it finds pages that changed without anyone saying so.

const (
	// indexRetryBase and indexRetryMax bound the delay before a failed page is
	// tried again: it doubles from the first to the last, and a page that keeps
	// failing is still tried every hour rather than given up on, since the cause
	// (an embedding service that is down) is usually temporary.
	indexRetryBase = 30 * time.Second
	indexRetryMax  = time.Hour

	// indexReverifyAfter is how long an indexed page may go without being
	// looked at. The look costs a query and a render when nothing changed.
	indexReverifyAfter = 24 * time.Hour
)

// indexRetryDelay is the wait after a page has failed attempts times before.
func indexRetryDelay(attempts int) time.Duration {
	if attempts >= 12 {
		return indexRetryMax
	}
	d := indexRetryBase << attempts
	if d > indexRetryMax || d <= 0 {
		return indexRetryMax
	}
	return d
}

// QueueIndex asks for a page to be synchronised with its knowledge base.
//
// With subtree set, the pages beneath it too: whether a page may be indexed
// depends on its ancestors (a restriction is inherited, a trashed page takes
// its children with it, a moved page arrives under different ancestors), so an
// event that names one page can change the answer for all below it.
//
// A positive delay debounces, for edits, where waiting spares an embedding run
// per keystroke. A zero delay is for changes that take content away from
// readers, where every second is a second in which people who may no longer see
// a page can still find it.
func (s *PageService) QueueIndex(ctx context.Context, tenantID uint64, pageID string, subtree bool,
	delay time.Duration,
) error {
	if s.d.Knowledge == nil {
		return nil
	}
	if subtree {
		return s.d.Repos.IndexQueue.MarkSubtree(ctx, tenantID, pageID, delay)
	}
	return s.d.Repos.IndexQueue.Mark(ctx, tenantID, []string{pageID}, delay)
}

// ProcessIndexQueue synchronises up to limit due pages and reports how many it
// took. A page that fails is scheduled for a retry and does not stop the rest.
func (s *PageService) ProcessIndexQueue(ctx context.Context, limit int) (int, error) {
	if s.d.Knowledge == nil {
		return 0, nil
	}
	claims, err := s.d.Repos.IndexQueue.Claim(ctx, limit, indexLease)
	if err != nil {
		return 0, err
	}
	for _, claim := range claims {
		if _, err := s.syncClaimed(ctx, claim); err != nil {
			logger.Warnf(ctx, "[docs] indexing page %s failed (attempt %d): %v", claim.PageID, claim.Attempts+1, err)
		}
	}
	return len(claims), nil
}

// RequeueIndex queues pages whose entry may be out of date though nobody said
// so: an event lost in a crash, a knowledge base bound to a space after its
// pages were written, an entry not looked at for a day. It examines a bounded
// number of pages per call.
func (s *PageService) RequeueIndex(ctx context.Context, limit int) (int64, error) {
	if s.d.Knowledge == nil {
		return 0, nil
	}
	return s.d.Repos.IndexQueue.Requeue(ctx, limit, indexReverifyAfter)
}
