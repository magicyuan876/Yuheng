package docs

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// The background cleaner: the thing that runs the sweeps in maintenance.go on
// a timer.
//
// It is deliberately the dullest kind of scheduler — a ticker and a mutex —
// rather than a queue job, because what it does must be safe to skip, safe to
// run twice, and impossible to pile up:
//
//   - Skippable: every sweep is bounded and leaves the rest for next time, so
//     a missed tick costs an hour of delay and nothing else. That is why this
//     does not need the durability of an Asynq job.
//
//   - Non-overlapping: a run that takes longer than the interval must not
//     have a second one start on top of it. The mutex is tried, not held, so
//     a slow run makes the next tick a no-op instead of a queue.
//
//   - Panic-proof: a panic in a maintenance sweep must not take the server
//     down. This is background tidying; the worst acceptable outcome is that
//     it stops until the next tick.
//
// Sweeps keep running to their limit each tick, so a backlog drains over
// several ticks rather than in one long transaction.

// DefaultCleanupInterval is used when the configuration asks for none.
const DefaultCleanupInterval = time.Hour

// Cleaner runs the module's maintenance sweeps on a timer.
type Cleaner struct {
	pages     *service.PageService
	files     *service.AttachmentService
	retention time.Duration
	interval  time.Duration

	// running is tried rather than locked, so a slow sweep makes the next
	// tick a no-op instead of queueing another one behind it.
	running sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	// started guards Stop against waiting on a loop that was never begun,
	// which would otherwise block a shutdown for ever in any build that
	// constructs a cleaner and does not start it.
	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewCleaner builds the background cleaner. A non-positive interval switches
// it off, which is how an operator who prefers to run the sweeps by hand
// says so.
func NewCleaner(pages *service.PageService, files *service.AttachmentService,
	retention, interval time.Duration,
) *Cleaner {
	if interval == 0 {
		interval = DefaultCleanupInterval
	}
	return &Cleaner{
		pages: pages, files: files, retention: retention, interval: interval,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

// Start begins the loop. It returns immediately; Stop waits for the current
// sweep to finish. Calling it twice starts one loop.
func (c *Cleaner) Start(ctx context.Context) {
	if c == nil {
		return
	}
	c.startOnce.Do(func() {
		c.started.Store(true)
		go c.loop(ctx)
	})
}

func (c *Cleaner) loop(ctx context.Context) {
	defer close(c.done)
	if c.interval <= 0 {
		logger.Infof(ctx, "[docs] maintenance sweeps are switched off")
		return
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Not on start-up: a server restart is exactly when something else is
	// probably wrong, and deleting things is the last thing that should
	// happen while somebody is still working out what.
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		case <-ticker.C:
			c.RunOnce(ctx)
		}
	}
}

// RunOnce performs one round of every sweep. Safe to call by hand.
//
// A round already in progress makes this a no-op rather than a second
// concurrent sweep.
func (c *Cleaner) RunOnce(ctx context.Context) {
	if c == nil {
		return
	}
	if !c.running.TryLock() {
		logger.Infof(ctx, "[docs] skipping a maintenance round: the previous one is still running")
		return
	}
	defer c.running.Unlock()

	c.sweep(ctx, "orphaned attachments", func() (*service.SweepReport, error) {
		return c.files.SweepOrphanAttachments(ctx, service.SweepOptions{})
	})
	c.sweep(ctx, "expired trash", func() (*service.SweepReport, error) {
		return c.pages.SweepExpiredTrash(ctx, c.retention, service.SweepOptions{})
	})
	c.sweep(ctx, "expired exports", func() (*service.SweepReport, error) {
		return c.pages.SweepExpiredExports(ctx, service.SweepOptions{})
	})
}

// sweep runs one job, surviving its panics and reporting what it did.
func (c *Cleaner) sweep(ctx context.Context, name string,
	run func() (*service.SweepReport, error),
) {
	defer func() {
		// Background tidying must never take the server with it.
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] the %s sweep panicked: %v", name, r)
		}
	}()
	if run == nil {
		return
	}
	report, err := run()
	if err != nil {
		logger.Warnf(ctx, "[docs] the %s sweep failed: %v", name, err)
		return
	}
	if report == nil || report.Considered == 0 {
		return
	}
	// Only when something happened: an hourly line saying "nothing to do" is
	// how operators learn to filter out the log this one lives in.
	logger.Infof(ctx, "[docs] %s sweep: considered %d, removed %d, failed %d, released %d bytes, more=%v",
		name, report.Considered, report.Deleted, report.Failed, report.BytesReleased, report.More)
}

// Stop ends the loop and waits for any sweep in flight.
//
// Safe on a cleaner that was never started, and safe to call twice: a
// shutdown path that cannot be called wrongly is worth the two extra fields.
func (c *Cleaner) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() { close(c.stop) })
	if c.started.Load() {
		<-c.done
	}
}
