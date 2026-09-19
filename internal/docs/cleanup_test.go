package docs

import (
	"context"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// The cleaner's own guarantees, tested without a database: what matters here
// is that it cannot pile up, cannot deadlock a shutdown, and cannot take the
// server down with it.

func TestACleanerThatWasNeverStartedStopsImmediately(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, time.Hour)

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked on a cleaner that was never started")
	}
}

func TestStopIsSafeTwice(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, time.Hour)
	c.Start(context.Background())
	c.Stop()
	c.Stop()
}

func TestStartIsSafeTwiceAndRunsOneLoop(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, time.Hour)
	ctx := context.Background()
	c.Start(ctx)
	c.Start(ctx)
	c.Stop()
}

// A switched-off cleaner must still shut down cleanly, or an operator who
// disabled the sweeps would find the server hanging on exit.
func TestASwitchedOffCleanerStopsCleanly(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, -1)
	c.Start(context.Background())

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a switched-off cleaner did not stop")
	}
}

// A cancelled context ends the loop, which is how the server's shutdown
// reaches it.
func TestACancelledContextEndsTheLoop(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	cancel()

	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop ignored its cancelled context")
	}
	c.Stop()
}

// Background tidying must never take the server with it.
func TestAPanickingSweepIsContained(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, time.Hour)
	c.sweep(context.Background(), "explosive", func() (*service.SweepReport, error) {
		panic("boom")
	})
}

// A nil cleaner is a build without the module; every method must tolerate it
// rather than making the caller check.
func TestANilCleanerIsInert(t *testing.T) {
	var c *Cleaner
	c.Start(context.Background())
	c.RunOnce(context.Background())
	c.Stop()
}

// A round already in progress makes the next a no-op rather than a second
// concurrent sweep.
func TestOverlappingRoundsAreSkipped(t *testing.T) {
	c := NewCleaner(nil, nil, time.Hour, time.Hour)

	// Hold the lock the way a slow round would, then prove RunOnce returns
	// instead of waiting for it.
	c.running.Lock()
	defer c.running.Unlock()

	done := make(chan struct{})
	go func() {
		c.RunOnce(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunOnce waited for a round in progress instead of skipping")
	}
}

// The default fills in for a zero interval, and a negative one switches the
// sweeps off rather than running them constantly.
func TestTheIntervalIsInterpreted(t *testing.T) {
	if got := NewCleaner(nil, nil, time.Hour, 0).interval; got != DefaultCleanupInterval {
		t.Fatalf("zero interval = %v, want the default %v", got, DefaultCleanupInterval)
	}
	if got := NewCleaner(nil, nil, time.Hour, time.Minute).interval; got != time.Minute {
		t.Fatalf("interval = %v, want a minute", got)
	}
	if got := NewCleaner(nil, nil, time.Hour, -1).interval; got >= 0 {
		t.Fatalf("a negative interval must stay negative to switch the loop off, got %v", got)
	}
}
