package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/ratelimit"
)

// Per-IP budgets for the unauthenticated credential endpoints. They are
// generous enough for a person retyping a password behind a shared office NAT
// and tight enough that a single address cannot sweep a password list; the
// per-account lockout in the auth handler is the real brake on a distributed
// guess, this is the brake on volume from one source.
const (
	authIPWindow = time.Minute

	// AuthLoginIPLimit is the per-IP budget for POST /auth/login per minute.
	AuthLoginIPLimit = 30
	// AuthRegisterIPLimit is the per-IP budget for POST /auth/register per minute.
	AuthRegisterIPLimit = 10
	// AuthSwitchTenantIPLimit is the per-IP budget for POST /auth/switch-tenant
	// per minute. It is higher because a real session may switch often.
	AuthSwitchTenantIPLimit = 60
	// DocsUnlockIPLimit is the per-IP budget for unlocking a password-protected
	// public page per minute; the password is the only credential there.
	DocsUnlockIPLimit = 10
)

// authIPLimiter is shared by every AuthIPRateLimit middleware: scopes keep the
// buckets apart through the key, so one Redis client (and one cleanup loop)
// serves them all.
var authIPLimiter = newAuthIPLimiter()

func newAuthIPLimiter() *ratelimit.Limiter {
	l := ratelimit.New(nil, "auth:ip:", authIPWindow, "")
	// The cleanup goroutine lives as long as the process, like the limiter.
	go l.StartCleanup(make(chan struct{}))
	return l
}

// SetAuthRateLimitRedis makes the per-IP auth limiters count in Redis so the
// budget holds across several server instances. Without it (or when Redis is
// unreachable) each process counts on its own, which is correct for the usual
// single-instance deployment.
func SetAuthRateLimitRedis(c *redis.Client) { authIPLimiter.SetRedis(c) }

// AuthIPRateLimit limits requests to limit per minute per client IP, in its own
// bucket named by scope. The IP is c.ClientIP(), which honours only the trusted
// proxies configured on the engine, so a spoofed X-Forwarded-For cannot pick a
// fresh bucket.
func AuthIPRateLimit(scope string, limit int) gin.HandlerFunc {
	return authIPRateLimit(authIPLimiter, scope, limit)
}

func authIPRateLimit(l *ratelimit.Limiter, scope string, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			// A proxy that strips the address must not switch the limit off.
			ip = "_unknown_"
		}
		if !l.Allow(c.Request.Context(), scope+":"+ip, limit) {
			c.Header("Retry-After", "60")
			_ = c.Error(&apperrors.AppError{
				Code:     apperrors.ErrTooManyRequests,
				Message:  "too many requests; please retry shortly",
				HTTPCode: http.StatusTooManyRequests,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
