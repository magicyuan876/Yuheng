package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
)

func probe(t *testing.T, h gin.HandlerFunc) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ready", h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", w.Body.String(), err)
	}
	return w.Code, body
}

var clean = func() (bool, string) { return true, "" }

func TestReadyOKWithDatabaseAndNoRedis(t *testing.T) {
	code, body := probe(t, readyHandler(pgtest.New(t), nil, clean))
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("got %d %v; want 200 ok", code, body)
	}
	if body["checks"].(map[string]any)["redis"] != "disabled" {
		t.Errorf("redis check = %v; want disabled when not configured", body["checks"])
	}
}

func TestReadyFailsWhenDatabaseIsGone(t *testing.T) {
	db := pgtest.New(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	code, body := probe(t, readyHandler(db, nil, clean))
	if code != http.StatusServiceUnavailable || body["checks"].(map[string]any)["database"] != "failed" {
		t.Fatalf("got %d %v; want 503 with a failed database check", code, body)
	}
}

func TestReadyFailsWhenRedisIsConfiguredButDown(t *testing.T) {
	// Port 1 on loopback refuses connections at once.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	defer func() { _ = rdb.Close() }()

	code, body := probe(t, readyHandler(pgtest.New(t), rdb, clean))
	if code != http.StatusServiceUnavailable || body["checks"].(map[string]any)["redis"] != "failed" {
		t.Fatalf("got %d %v; want 503 with a failed redis check", code, body)
	}
}

func TestReadyFailsWhenMigrationsAreNotClean(t *testing.T) {
	dirty := func() (bool, string) { return false, "schema is dirty at version 7 with a secret-looking detail" }
	code, body := probe(t, readyHandler(pgtest.New(t), nil, dirty))
	if code != http.StatusServiceUnavailable || body["checks"].(map[string]any)["migrations"] != "failed" {
		t.Fatalf("got %d %v; want 503 with failed migrations", code, body)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "secret-looking") {
		t.Error("the unauthenticated response carries the internal reason")
	}
}
