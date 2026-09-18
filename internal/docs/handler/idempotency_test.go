package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func authed(tenant uint64, user string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func newIdemRouter(store IdempotencyStore, calls *atomic.Int32, block chan struct{}) *gin.Engine {
	r := gin.New()
	r.Use(authed(1, "alice"), Idempotency(store, time.Hour))
	r.POST("/things", func(c *gin.Context) {
		n := calls.Add(1)
		if block != nil {
			<-block
		}
		c.Header("Location", "/things/"+itoa(uint64(n)))
		c.JSON(http.StatusCreated, gin.H{"n": n})
	})
	r.POST("/fail", func(c *gin.Context) {
		calls.Add(1)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "boom"})
	})
	r.GET("/things", func(c *gin.Context) { calls.Add(1); c.Status(200) })
	return r
}

func do(r http.Handler, method, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	if key != "" {
		req.Header.Set(IdempotencyHeader, key)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestIdempotencyReplaysStoredResponse(t *testing.T) {
	var calls atomic.Int32
	r := newIdemRouter(NewMemoryIdempotencyStore(), &calls, nil)

	first := do(r, http.MethodPost, "/things", "k1")
	require.Equal(t, 201, first.Code)
	second := do(r, http.MethodPost, "/things", "k1")
	require.Equal(t, 201, second.Code)
	require.Equal(t, first.Body.String(), second.Body.String(), "byte-identical replay")
	require.Equal(t, "true", second.Header().Get("Idempotent-Replayed"))
	require.Equal(t, "/things/1", second.Header().Get("Location"))
	require.Equal(t, int32(1), calls.Load(), "handler ran once")

	// A different key runs the handler again.
	third := do(r, http.MethodPost, "/things", "k2")
	require.Equal(t, 201, third.Code)
	require.Equal(t, int32(2), calls.Load())

	// No header: every request executes.
	do(r, http.MethodPost, "/things", "")
	do(r, http.MethodPost, "/things", "")
	require.Equal(t, int32(4), calls.Load())

	// Safe methods ignore the header.
	do(r, http.MethodGet, "/things", "k1")
	require.Equal(t, int32(5), calls.Load())

	// Oversized key is rejected.
	require.Equal(t, 400, do(r, http.MethodPost, "/things", strings.Repeat("x", 129)).Code)
}

func TestIdempotencyDoesNotStoreFailures(t *testing.T) {
	var calls atomic.Int32
	r := newIdemRouter(NewMemoryIdempotencyStore(), &calls, nil)
	require.Equal(t, 500, do(r, http.MethodPost, "/fail", "k").Code)
	require.Equal(t, 500, do(r, http.MethodPost, "/fail", "k").Code)
	require.Equal(t, int32(2), calls.Load(), "a failed request is retried for real")
}

func TestIdempotencyKeysAreScopedPerUser(t *testing.T) {
	var calls atomic.Int32
	store := NewMemoryIdempotencyStore()
	alice := newIdemRouter(store, &calls, nil)
	bob := gin.New()
	bob.Use(authed(1, "bob"), Idempotency(store, time.Hour))
	bob.POST("/things", func(c *gin.Context) { calls.Add(1); c.JSON(201, gin.H{"who": "bob"}) })

	do(alice, http.MethodPost, "/things", "same")
	rec := do(bob, http.MethodPost, "/things", "same")
	require.Contains(t, rec.Body.String(), "bob")
	require.Equal(t, int32(2), calls.Load(), "same key, different users: both execute")
}

func TestIdempotencyConcurrentDuplicateGets409(t *testing.T) {
	var calls atomic.Int32
	block := make(chan struct{})
	r := newIdemRouter(NewMemoryIdempotencyStore(), &calls, block)

	var wg sync.WaitGroup
	wg.Add(1)
	var firstCode int
	go func() {
		defer wg.Done()
		firstCode = do(r, http.MethodPost, "/things", "dup").Code
	}()
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, 5*time.Millisecond)

	second := do(r, http.MethodPost, "/things", "dup")
	require.Equal(t, 409, second.Code, "in-flight duplicate is refused, not executed")
	close(block)
	wg.Wait()
	require.Equal(t, 201, firstCode)
	require.Equal(t, int32(1), calls.Load())

	// Once complete, the duplicate replays.
	require.Equal(t, 201, do(r, http.MethodPost, "/things", "dup").Code)
	require.Equal(t, int32(1), calls.Load())
}

func TestRedisIdempotencyStore(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewRedisIdempotencyStore(rdb)
	ctx := context.Background()

	okRes, err := store.Reserve(ctx, "k", time.Minute)
	require.NoError(t, err)
	require.True(t, okRes)
	okRes, err = store.Reserve(ctx, "k", time.Minute)
	require.NoError(t, err)
	require.False(t, okRes, "second reservation loses")
	_, inflight, ok, err := store.Get(ctx, "k")
	require.NoError(t, err)
	require.True(t, inflight)
	require.False(t, ok)

	require.NoError(t, store.Complete(ctx, "k", &StoredResponse{Status: 201, Body: []byte(`{"a":1}`)}, time.Minute))
	resp, inflight, ok, err := store.Get(ctx, "k")
	require.NoError(t, err)
	require.False(t, inflight)
	require.True(t, ok)
	require.Equal(t, 201, resp.Status)
	require.NoError(t, store.Release(ctx, "k"), "release never drops a completed response")
	_, _, ok, _ = store.Get(ctx, "k")
	require.True(t, ok)

	// End to end through the middleware.
	var calls atomic.Int32
	r := newIdemRouter(store, &calls, nil)
	do(r, http.MethodPost, "/things", "e2e")
	do(r, http.MethodPost, "/things", "e2e")
	require.Equal(t, int32(1), calls.Load())
}
