package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// localLockoutMaxEntries bounds the in-process failure table. Keys are
// attacker-chosen (any e-mail address string), so without a cap an
// unauthenticated client could grow the map without limit. When the cap is hit
// after purging expired entries, new keys are simply not tracked: the per-IP
// limiter still applies to that client, and the table cannot be used to
// exhaust memory.
const localLockoutMaxEntries = 100_000

// lockoutFailScript records one failure. When the count reaches the maximum it
// sets the lock key for lockFor milliseconds and clears the counter, all in one
// round trip so concurrent attempts cannot slip past the threshold.
var lockoutFailScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
if c >= tonumber(ARGV[2]) then
    redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
    redis.call('DEL', KEYS[1])
    return 1
end
return 0
`)

// LockoutConfig tunes a Lockout.
type LockoutConfig struct {
	// MaxFailures failures within Window lock the key.
	MaxFailures int
	// Window is how long a run of failures is remembered, counted from the
	// first failure of the run.
	Window time.Duration
	// LockFor is how long a locked key stays locked.
	LockFor time.Duration
}

// Lockout is a failed-attempt counter with a temporary lock, used to slow
// online password guessing per account. It is keyed by an arbitrary string
// (the normalised e-mail address) which is hashed before it reaches Redis, so
// no address is stored there. Redis is used when configured and reachable; on
// any Redis error the in-process table takes over, the same pattern as Limiter.
type Lockout struct {
	mu     sync.RWMutex
	redis  *redis.Client
	prefix string
	cfg    LockoutConfig
	now    func() time.Time

	local sync.Mutex
	table map[string]*lockoutEntry
}

type lockoutEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

// NewLockout builds a Lockout. redisClient may be nil (in-process only).
func NewLockout(redisClient *redis.Client, prefix string, cfg LockoutConfig) *Lockout {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.Window <= 0 {
		cfg.Window = 15 * time.Minute
	}
	if cfg.LockFor <= 0 {
		cfg.LockFor = 15 * time.Minute
	}
	return &Lockout{
		redis:  redisClient,
		prefix: prefix,
		cfg:    cfg,
		now:    time.Now,
		table:  make(map[string]*lockoutEntry),
	}
}

// SetRedis switches the shared store on (or off with nil). Safe to call while
// requests are in flight.
func (l *Lockout) SetRedis(c *redis.Client) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.redis = c
	l.mu.Unlock()
}

// SetClock replaces the time source; for tests, which must not sleep. It only
// affects the in-process table, since Redis owns its own expiry.
func (l *Lockout) SetClock(now func() time.Time) {
	l.mu.Lock()
	l.now = now
	l.mu.Unlock()
}

func (l *Lockout) redisClient() *redis.Client {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.redis
}

func (l *Lockout) clock() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.now()
}

func (l *Lockout) hash(key string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(key))))
	return hex.EncodeToString(sum[:])
}

// Check reports whether key is currently locked and, if so, for how much longer.
func (l *Lockout) Check(ctx context.Context, key string) (locked bool, retryAfter time.Duration) {
	if l == nil {
		return false, 0
	}
	h := l.hash(key)
	if rc := l.redisClient(); rc != nil {
		ttl, err := rc.PTTL(ctx, l.prefix+"lock:"+h).Result()
		if err == nil {
			if ttl > 0 {
				return true, ttl
			}
			return false, 0
		}
	}
	return l.localCheck(h)
}

// RecordFailure counts one failed attempt and reports whether the key is now
// locked.
func (l *Lockout) RecordFailure(ctx context.Context, key string) (locked bool) {
	if l == nil {
		return false
	}
	h := l.hash(key)
	if rc := l.redisClient(); rc != nil {
		res, err := lockoutFailScript.Run(ctx, rc,
			[]string{l.prefix + "fail:" + h, l.prefix + "lock:" + h},
			l.cfg.Window.Milliseconds(), l.cfg.MaxFailures, l.cfg.LockFor.Milliseconds(),
		).Int64()
		if err == nil {
			return res == 1
		}
	}
	return l.localFail(h)
}

// Reset forgets every failure and any lock for key. Called after a successful
// login.
func (l *Lockout) Reset(ctx context.Context, key string) {
	if l == nil {
		return
	}
	h := l.hash(key)
	if rc := l.redisClient(); rc != nil {
		if err := rc.Del(ctx, l.prefix+"fail:"+h, l.prefix+"lock:"+h).Err(); err == nil {
			l.localReset(h)
			return
		}
	}
	l.localReset(h)
}

func (l *Lockout) localCheck(h string) (bool, time.Duration) {
	now := l.clock()
	l.local.Lock()
	defer l.local.Unlock()
	e := l.table[h]
	if e == nil {
		return false, 0
	}
	if now.Before(e.lockedUntil) {
		return true, e.lockedUntil.Sub(now)
	}
	return false, 0
}

func (l *Lockout) localFail(h string) bool {
	now := l.clock()
	l.local.Lock()
	defer l.local.Unlock()

	e := l.table[h]
	if e == nil {
		if len(l.table) >= localLockoutMaxEntries {
			l.purgeExpiredLocked(now)
			if len(l.table) >= localLockoutMaxEntries {
				return false
			}
		}
		e = &lockoutEntry{}
		l.table[h] = e
	}
	if now.Before(e.lockedUntil) {
		return true
	}
	// A new run starts when the previous one aged out of the window (or a lock
	// has expired): stale failures must not count towards a fresh lock.
	if e.failures == 0 || now.Sub(e.windowStart) >= l.cfg.Window || !e.lockedUntil.IsZero() {
		e.failures = 0
		e.windowStart = now
		e.lockedUntil = time.Time{}
	}
	e.failures++
	if e.failures >= l.cfg.MaxFailures {
		e.lockedUntil = now.Add(l.cfg.LockFor)
		e.failures = 0
		return true
	}
	return false
}

func (l *Lockout) localReset(h string) {
	l.local.Lock()
	delete(l.table, h)
	l.local.Unlock()
}

func (l *Lockout) purgeExpiredLocked(now time.Time) {
	for k, e := range l.table {
		if !now.Before(e.lockedUntil) && now.Sub(e.windowStart) >= l.cfg.Window {
			delete(l.table, k)
		}
	}
}
