package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func preflight(t *testing.T, origin string) http.Header {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(cors.New(corsConfig()))
	engine.GET("/api/v1/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/x", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Header()
}

func TestCORSDefaultAllowsAnyOriginWithoutCredentials(t *testing.T) {
	t.Setenv("YUHENG_CORS_ALLOWED_ORIGINS", "")
	h := preflight(t, "https://other.example")
	assert.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
	// "*" together with credentials is forbidden by the spec and rejected by
	// browsers; it must not be sent.
	assert.Empty(t, h.Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, h.Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestCORSExplicitOriginsAreEchoedWithCredentials(t *testing.T) {
	t.Setenv("YUHENG_CORS_ALLOWED_ORIGINS", " https://app.example.com , https://admin.example.com ")

	h := preflight(t, "https://app.example.com")
	assert.Equal(t, "https://app.example.com", h.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", h.Get("Access-Control-Allow-Credentials"))

	h = preflight(t, "https://evil.example")
	assert.Empty(t, h.Get("Access-Control-Allow-Origin"), "an origin outside the list gets no CORS grant")
}

func TestCORSWildcardInTheListWins(t *testing.T) {
	origins, credentials := corsOrigins("https://a.example, *")
	assert.Equal(t, []string{"*"}, origins)
	assert.False(t, credentials)
}
