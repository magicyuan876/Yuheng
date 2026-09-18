package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/redis/go-redis/v9"
)

// Idempotency-Key support for write routes.
//
// A client that retries a POST after a timeout sends the same key; the
// server replays the stored response instead of executing twice. Keys are
// scoped to (tenant, user, method, route, key) so two users cannot collide,
// and a key that is still executing answers 409 rather than running the
// request a second time in parallel. Responses larger than maxStoredBody are
// executed normally but not stored.

// IdempotencyHeader is the request header carrying the client's key.
const IdempotencyHeader = "Idempotency-Key"

const (
	maxIdempotencyKeyLen = 128
	maxStoredBody        = 1 << 20 // 1 MiB
	inflightTTL          = 60 * time.Second
)

// StoredResponse is what gets replayed.
type StoredResponse struct {
	Status      int                 `json:"status"`
	Header      map[string][]string `json:"header,omitempty"`
	Body        []byte              `json:"body"`
	CompletedAt time.Time           `json:"completed_at"`
}

// IdempotencyStore persists responses and in-flight markers.
type IdempotencyStore interface {
	// Get returns the stored response, or ok=false. inflight is true when the
	// key is reserved by a request that has not finished.
	Get(ctx context.Context, key string) (resp *StoredResponse, inflight bool, ok bool, err error)
	// Reserve marks the key as in flight; returns false if it already exists.
	Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Complete replaces the reservation with the response.
	Complete(ctx context.Context, key string, resp *StoredResponse, ttl time.Duration) error
	// Release drops a reservation whose request failed before producing a
	// storable response, so the client can retry.
	Release(ctx context.Context, key string) error
}

// Idempotency returns the middleware. Requests without the header, or with a
// safe method, pass through untouched.
func Idempotency(store IdempotencyStore, ttl time.Duration) gin.HandlerFunc {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader(IdempotencyHeader))
		if key == "" || store == nil || !isWrite(c.Request.Method) {
			c.Next()
			return
		}
		if len(key) > maxIdempotencyKeyLen {
			c.AbortWithStatusJSON(http.StatusBadRequest,
				gin.H{"success": false, "error": "Idempotency-Key is too long"})
			return
		}
		ctx := c.Request.Context()
		tenantID, _ := types.TenantIDFromContext(ctx)
		userID, _ := types.UserIDFromContext(ctx)
		scoped := scopeKey(tenantID, userID, c.Request.Method, c.FullPath(), key)

		if resp, inflight, ok, err := store.Get(ctx, scoped); err == nil {
			if ok {
				replay(c, resp)
				return
			}
			if inflight {
				c.JSON(http.StatusConflict, gin.H{
					"success": false, "error": "a request with this Idempotency-Key is still in progress",
				})
				c.Abort()
				return
			}
		}
		reserved, err := store.Reserve(ctx, scoped, inflightTTL)
		if err == nil && !reserved {
			// Lost a race with an identical concurrent request.
			if resp, _, ok, err := store.Get(ctx, scoped); err == nil && ok {
				replay(c, resp)
				return
			}
			c.JSON(http.StatusConflict, gin.H{
				"success": false, "error": "a request with this Idempotency-Key is still in progress",
			})
			c.Abort()
			return
		}

		rec := &recordingWriter{ResponseWriter: c.Writer}
		c.Writer = rec
		c.Next()
		c.Writer = rec.ResponseWriter

		status := rec.Status()
		// Only successful outcomes are worth replaying; a 5xx must be retried
		// for real, and 4xx are cheap to recompute.
		if status >= 200 && status < 300 && !rec.overflow {
			_ = store.Complete(ctx, scoped, &StoredResponse{
				Status: status, Header: replayHeaders(rec.Header()), Body: rec.buf.Bytes(),
				CompletedAt: time.Now().UTC(),
			}, ttl)
			return
		}
		_ = store.Release(ctx, scoped)
	}
}

func isWrite(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func scopeKey(tenantID uint64, userID, method, route, key string) string {
	sum := sha256.Sum256([]byte(userID + "|" + method + "|" + route + "|" + key))
	return "docs:idem:" + itoa(tenantID) + ":" + hex.EncodeToString(sum[:16])
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func replay(c *gin.Context, resp *StoredResponse) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.Header().Set("Idempotent-Replayed", "true")
	c.Data(resp.Status, firstOr(resp.Header["Content-Type"], "application/json; charset=utf-8"), resp.Body)
	c.Abort()
}

func firstOr(v []string, def string) string {
	if len(v) > 0 {
		return v[0]
	}
	return def
}

func replayHeaders(h http.Header) map[string][]string {
	out := map[string][]string{}
	for _, k := range []string{"Content-Type", "Location"} {
		if v := h.Values(k); len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

// recordingWriter tees the response body into a buffer (up to maxStoredBody).
type recordingWriter struct {
	gin.ResponseWriter
	buf      bytes.Buffer
	overflow bool
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if !w.overflow {
		if w.buf.Len()+len(b) > maxStoredBody {
			w.overflow = true
			w.buf.Reset()
		} else {
			w.buf.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *recordingWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

// ---- stores ----------------------------------------------------------------

type memEntry struct {
	resp     *StoredResponse
	inflight bool
	expires  time.Time
}

// MemoryIdempotencyStore is the single-process store.
type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]memEntry
	now     func() time.Time
}

// NewMemoryIdempotencyStore creates an empty store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{entries: map[string]memEntry{}, now: time.Now}
}

func (s *MemoryIdempotencyStore) get(key string) (memEntry, bool) {
	e, ok := s.entries[key]
	if ok && !s.now().Before(e.expires) {
		delete(s.entries, key)
		return memEntry{}, false
	}
	return e, ok
}

// Get implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Get(_ context.Context, key string) (*StoredResponse, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.get(key)
	if !ok {
		return nil, false, false, nil
	}
	if e.inflight {
		return nil, true, false, nil
	}
	return e.resp, false, true, nil
}

// Reserve implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Reserve(_ context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.get(key); ok {
		return false, nil
	}
	if len(s.entries) > 50000 {
		s.entries = map[string]memEntry{}
	}
	s.entries[key] = memEntry{inflight: true, expires: s.now().Add(ttl)}
	return true, nil
}

// Complete implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Complete(_ context.Context, key string, resp *StoredResponse,
	ttl time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = memEntry{resp: resp, expires: s.now().Add(ttl)}
	return nil
}

// Release implements IdempotencyStore.
func (s *MemoryIdempotencyStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[key]; ok && e.inflight {
		delete(s.entries, key)
	}
	return nil
}

// RedisIdempotencyStore shares keys across instances. The in-flight marker
// is the literal "inflight"; completed entries are JSON.
type RedisIdempotencyStore struct{ rdb redis.UniversalClient }

// NewRedisIdempotencyStore wraps a client.
func NewRedisIdempotencyStore(rdb redis.UniversalClient) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{rdb: rdb}
}

const inflightMarker = "inflight"

// Get implements IdempotencyStore.
func (s *RedisIdempotencyStore) Get(ctx context.Context, key string) (*StoredResponse, bool, bool, error) {
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, false, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	if string(raw) == inflightMarker {
		return nil, true, false, nil
	}
	var resp StoredResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, false, false, err
	}
	return &resp, false, true, nil
}

// Reserve implements IdempotencyStore.
func (s *RedisIdempotencyStore) Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, inflightMarker, ttl).Result()
}

// Complete implements IdempotencyStore.
func (s *RedisIdempotencyStore) Complete(ctx context.Context, key string, resp *StoredResponse,
	ttl time.Duration,
) error {
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, raw, ttl).Err()
}

// Release implements IdempotencyStore.
func (s *RedisIdempotencyStore) Release(ctx context.Context, key string) error {
	// Only drop the marker, never a completed response another request stored.
	val, err := s.rdb.Get(ctx, key).Result()
	if err != nil || val != inflightMarker {
		return nil
	}
	return s.rdb.Del(ctx, key).Err()
}
