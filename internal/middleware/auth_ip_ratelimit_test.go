package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/ratelimit"
)

type ipTestClock struct{ t time.Time }

func (c *ipTestClock) now() time.Time { return c.t }

func newIPTestRouter(l *ratelimit.Limiter, trusted []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(trusted)
	r.Use(ErrorHandler())
	r.POST("/login", authIPRateLimit(l, "login", 3), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/register", authIPRateLimit(l, "register", 1), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func hit(r *gin.Engine, path, remote, xff string) int {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestAuthIPRateLimitBudgetPerIPAndScope(t *testing.T) {
	clk := &ipTestClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := ratelimit.New(nil, "t:", time.Minute, "i")
	l.SetClock(clk.now)
	r := newIPTestRouter(l, nil)

	for i := 0; i < 3; i++ {
		if c := hit(r, "/login", "10.0.0.1:1000", ""); c != http.StatusOK {
			t.Fatalf("login %d = %d", i, c)
		}
	}
	if c := hit(r, "/login", "10.0.0.1:1000", ""); c != http.StatusTooManyRequests {
		t.Fatalf("4th login = %d, want 429", c)
	}
	if c := hit(r, "/login", "10.0.0.2:1000", ""); c != http.StatusOK {
		t.Fatalf("other IP = %d, want 200", c)
	}
	if c := hit(r, "/register", "10.0.0.1:1000", ""); c != http.StatusOK {
		t.Fatalf("scopes must be independent, register = %d", c)
	}
	clk.t = clk.t.Add(61 * time.Second)
	if c := hit(r, "/login", "10.0.0.1:1000", ""); c != http.StatusOK {
		t.Fatalf("budget must refill, login = %d", c)
	}
}

func TestAuthIPRateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	l := ratelimit.New(nil, "t:", time.Minute, "i")
	// No trusted proxies: X-Forwarded-For must be ignored entirely, so
	// rotating it cannot buy a fresh bucket.
	r := newIPTestRouter(l, nil)
	for i := 0; i < 3; i++ {
		hit(r, "/login", "10.0.0.9:1000", "1.1.1."+string(rune('0'+i)))
	}
	if c := hit(r, "/login", "10.0.0.9:1000", "9.9.9.9"); c != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Forwarded-For escaped the limiter: %d", c)
	}
}
