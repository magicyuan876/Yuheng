package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLockout(rc *redis.Client) (*Lockout, *fakeClock) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	l := NewLockout(rc, "t:", LockoutConfig{MaxFailures: 5, Window: 15 * time.Minute, LockFor: 15 * time.Minute})
	l.SetClock(clk.now)
	return l, clk
}

func TestLockoutLocksAfterMaxFailures(t *testing.T) {
	ctx := context.Background()
	l, _ := newTestLockout(nil)
	for i := 1; i <= 4; i++ {
		if l.RecordFailure(ctx, "a@example.com") {
			t.Fatalf("locked after only %d failures", i)
		}
		if locked, _ := l.Check(ctx, "a@example.com"); locked {
			t.Fatalf("Check reports locked after %d failures", i)
		}
	}
	if !l.RecordFailure(ctx, "a@example.com") {
		t.Fatal("fifth failure must lock")
	}
	locked, retry := l.Check(ctx, "a@example.com")
	if !locked || retry <= 0 || retry > 15*time.Minute {
		t.Fatalf("locked=%v retry=%v, want locked with <=15m", locked, retry)
	}
}

func TestLockoutExpiresAndStartsFresh(t *testing.T) {
	ctx := context.Background()
	l, clk := newTestLockout(nil)
	for i := 0; i < 5; i++ {
		l.RecordFailure(ctx, "a@example.com")
	}
	clk.advance(15*time.Minute - time.Second)
	if locked, _ := l.Check(ctx, "a@example.com"); !locked {
		t.Fatal("still locked just before the lock expires")
	}
	clk.advance(2 * time.Second)
	if locked, _ := l.Check(ctx, "a@example.com"); locked {
		t.Fatal("lock must expire after 15 minutes")
	}
	// After expiry the counter starts from zero: four failures do not lock.
	for i := 0; i < 4; i++ {
		if l.RecordFailure(ctx, "a@example.com") {
			t.Fatal("stale failures counted towards a new lock")
		}
	}
}

func TestLockoutFailuresAgeOutOfWindow(t *testing.T) {
	ctx := context.Background()
	l, clk := newTestLockout(nil)
	for i := 0; i < 4; i++ {
		l.RecordFailure(ctx, "a@example.com")
	}
	clk.advance(16 * time.Minute)
	if l.RecordFailure(ctx, "a@example.com") {
		t.Fatal("a failure 16 minutes after the run began must not complete the run")
	}
}

func TestLockoutResetClearsCounterAndLock(t *testing.T) {
	ctx := context.Background()
	l, _ := newTestLockout(nil)
	for i := 0; i < 4; i++ {
		l.RecordFailure(ctx, "a@example.com")
	}
	l.Reset(ctx, "a@example.com")
	for i := 0; i < 4; i++ {
		if l.RecordFailure(ctx, "a@example.com") {
			t.Fatal("counter survived Reset")
		}
	}
	l.RecordFailure(ctx, "a@example.com")
	l.Reset(ctx, "a@example.com")
	if locked, _ := l.Check(ctx, "a@example.com"); locked {
		t.Fatal("Reset must lift a lock")
	}
}

func TestLockoutKeysAreIndependentAndNormalised(t *testing.T) {
	ctx := context.Background()
	l, _ := newTestLockout(nil)
	for i := 0; i < 5; i++ {
		l.RecordFailure(ctx, "  A@Example.com ")
	}
	if locked, _ := l.Check(ctx, "a@example.com"); !locked {
		t.Fatal("case and surrounding space must not create a second bucket")
	}
	if locked, _ := l.Check(ctx, "b@example.com"); locked {
		t.Fatal("another account must not be locked")
	}
}

func TestLockoutRedisBackend(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	l, _ := newTestLockout(rc)

	for i := 1; i <= 4; i++ {
		if l.RecordFailure(ctx, "a@example.com") {
			t.Fatalf("locked after %d failures", i)
		}
	}
	if !l.RecordFailure(ctx, "a@example.com") {
		t.Fatal("fifth failure must lock in Redis")
	}
	if locked, retry := l.Check(ctx, "a@example.com"); !locked || retry <= 0 {
		t.Fatalf("locked=%v retry=%v", locked, retry)
	}
	// A second Lockout on the same Redis (another instance) sees the lock.
	other, _ := newTestLockout(rc)
	if locked, _ := other.Check(ctx, "a@example.com"); !locked {
		t.Fatal("lock must be shared through Redis")
	}
	mr.FastForward(16 * time.Minute)
	if locked, _ := l.Check(ctx, "a@example.com"); locked {
		t.Fatal("Redis lock must expire")
	}
	l.RecordFailure(ctx, "b@example.com")
	l.Reset(ctx, "b@example.com")
	for i := 0; i < 4; i++ {
		if l.RecordFailure(ctx, "b@example.com") {
			t.Fatal("Reset must clear the Redis counter")
		}
	}
	for _, k := range mr.Keys() {
		if len(k) < 10 || containsPlainEmail(k) {
			t.Fatalf("Redis key %q exposes the address", k)
		}
	}
}

func containsPlainEmail(k string) bool {
	for i := 0; i+1 < len(k); i++ {
		if k[i] == '@' {
			return true
		}
	}
	return false
}

func TestLockoutFallsBackWhenRedisFails(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	l, _ := newTestLockout(rc)
	mr.Close() // Redis goes away

	for i := 0; i < 4; i++ {
		l.RecordFailure(ctx, "a@example.com")
	}
	if !l.RecordFailure(ctx, "a@example.com") {
		t.Fatal("in-process fallback must still lock")
	}
	if locked, _ := l.Check(ctx, "a@example.com"); !locked {
		t.Fatal("in-process fallback must report the lock")
	}
}

func TestLimiterSetClockMovesWindow(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(nil, "t:", time.Minute, "i")
	l.SetClock(clk.now)
	ctx := context.Background()
	if !l.Allow(ctx, "k", 1) || l.Allow(ctx, "k", 1) {
		t.Fatal("budget of 1 not enforced")
	}
	clk.advance(61 * time.Second)
	if !l.Allow(ctx, "k", 1) {
		t.Fatal("budget must refill after the window")
	}
}
