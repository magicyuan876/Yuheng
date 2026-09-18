package acl

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache stores resolution results for a short time. Entries are grouped by
// tenant and dropped together: any membership, grant or tree change in a
// tenant bumps that tenant's generation, so stale decisions cannot outlive
// the change by more than one request in flight.
type Cache interface {
	Get(ctx context.Context, tenantID uint64, key string) ([]byte, bool)
	Set(ctx context.Context, tenantID uint64, key string, value []byte, ttl time.Duration)
	// InvalidateTenant drops every entry of a tenant.
	InvalidateTenant(ctx context.Context, tenantID uint64)
}

// ---- in-process ------------------------------------------------------------

type memEntry struct {
	value   []byte
	expires time.Time
	gen     uint64
}

// MemoryCache is a process-local Cache: right for the Lite edition and for a
// single-instance deployment. Multi-instance deployments must use RedisCache,
// otherwise instance B keeps serving a decision instance A invalidated.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]memEntry
	gens    map[uint64]uint64
	max     int
	now     func() time.Time
}

// NewMemoryCache creates a cache holding at most max entries (default 10000).
func NewMemoryCache(max int) *MemoryCache {
	if max <= 0 {
		max = 10000
	}
	return &MemoryCache{entries: map[string]memEntry{}, gens: map[uint64]uint64{}, max: max, now: time.Now}
}

func memKey(tenantID uint64, key string) string { return strconv.FormatUint(tenantID, 10) + "|" + key }

// Get implements Cache.
func (c *MemoryCache) Get(_ context.Context, tenantID uint64, key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[memKey(tenantID, key)]
	if !ok || e.gen != c.gens[tenantID] || !c.now().Before(e.expires) {
		if ok {
			delete(c.entries, memKey(tenantID, key))
		}
		return nil, false
	}
	return e.value, true
}

// Set implements Cache.
func (c *MemoryCache) Set(_ context.Context, tenantID uint64, key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		// Crude but bounded: drop everything rather than track LRU order. The
		// cache only saves database round trips; correctness never depends
		// on a hit.
		c.entries = map[string]memEntry{}
	}
	c.entries[memKey(tenantID, key)] = memEntry{value: value, expires: c.now().Add(ttl), gen: c.gens[tenantID]}
}

// InvalidateTenant implements Cache.
func (c *MemoryCache) InvalidateTenant(_ context.Context, tenantID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gens[tenantID]++
}

// ---- Redis -----------------------------------------------------------------

// RedisCache shares decisions between instances. The tenant generation lives
// in Redis too, so an invalidation on one instance is seen by every other on
// its next lookup without any pub/sub fan-out.
type RedisCache struct {
	rdb    redis.UniversalClient
	prefix string
}

// NewRedisCache wraps a client; prefix namespaces the keys (default "docs:acl").
func NewRedisCache(rdb redis.UniversalClient, prefix string) *RedisCache {
	if prefix == "" {
		prefix = "docs:acl"
	}
	return &RedisCache{rdb: rdb, prefix: prefix}
}

func (c *RedisCache) genKey(tenantID uint64) string {
	return fmt.Sprintf("%s:%d:gen", c.prefix, tenantID)
}

func (c *RedisCache) generation(ctx context.Context, tenantID uint64) (string, bool) {
	gen, err := c.rdb.Get(ctx, c.genKey(tenantID)).Result()
	if err == redis.Nil {
		return "0", true
	}
	if err != nil {
		return "", false
	}
	return gen, true
}

func (c *RedisCache) entryKey(tenantID uint64, gen, key string) string {
	return fmt.Sprintf("%s:%d:%s:%s", c.prefix, tenantID, gen, key)
}

// Get implements Cache. Any Redis error is a miss; the resolver then falls
// back to the database, so a degraded Redis costs latency, never correctness.
func (c *RedisCache) Get(ctx context.Context, tenantID uint64, key string) ([]byte, bool) {
	gen, ok := c.generation(ctx, tenantID)
	if !ok {
		return nil, false
	}
	val, err := c.rdb.Get(ctx, c.entryKey(tenantID, gen, key)).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

// Set implements Cache.
func (c *RedisCache) Set(ctx context.Context, tenantID uint64, key string, value []byte, ttl time.Duration) {
	gen, ok := c.generation(ctx, tenantID)
	if !ok {
		return
	}
	_ = c.rdb.Set(ctx, c.entryKey(tenantID, gen, key), value, ttl).Err()
}

// InvalidateTenant implements Cache. Old-generation keys expire on their own.
func (c *RedisCache) InvalidateTenant(ctx context.Context, tenantID uint64) {
	_ = c.rdb.Incr(ctx, c.genKey(tenantID)).Err()
}

// NopCache disables caching (tests, debugging).
type NopCache struct{}

// Get implements Cache.
func (NopCache) Get(context.Context, uint64, string) ([]byte, bool) { return nil, false }

// Set implements Cache.
func (NopCache) Set(context.Context, uint64, string, []byte, time.Duration) {}

// InvalidateTenant implements Cache.
func (NopCache) InvalidateTenant(context.Context, uint64) {}
