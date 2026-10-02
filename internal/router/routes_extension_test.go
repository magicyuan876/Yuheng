package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/types"
)

// fakeRegistrar registers through the seam the way an extension would.
type fakeRegistrar struct {
	features extension.Features
	err      error
}

func (f fakeRegistrar) Register(r extension.Routes) error {
	if f.err != nil {
		return f.err
	}
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	// Closed to API keys: registered on the plain group.
	r.Group().GET("/acme/private", ok)
	// Open to any key.
	r.APIKeys(extension.APIKeyAnyKey()).GET("/acme/any", ok)
	// Full-access keys, or a key with the ingest capability.
	r.APIKeys(extension.APIKeyCapabilities(types.APIKeyCapabilityIngest)).POST("/acme/ingest", ok)
	// A zero policy declares nothing: closed like Group.
	r.APIKeys(extension.APIKeyPolicy{}).GET("/acme/undeclared", ok)
	// Feature-gated, declared open to any key.
	r.APIKeys(extension.APIKeyAnyKey()).GET("/acme/gated", extension.RequireFeature(f.features, "acme"), ok)
	return nil
}

// newExtensionEngine mounts the registrar the way NewRouter does: after an
// authentication stand-in, on /api/v1 behind the API-key gate.
func newExtensionEngine(t *testing.T, regs ...extension.RouteRegistrar) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// Stand-in for middleware.Auth: a bearer token is a JWT session, X-API-Key
	// installs an API-key scope named by its value, anything else is 401.
	engine.Use(func(c *gin.Context) {
		switch {
		case c.GetHeader("Authorization") != "":
		case c.GetHeader("X-API-Key") != "":
			scope := types.TenantAPIKeyScope{}
			switch c.GetHeader("X-API-Key") {
			case "full":
				scope.FullAccess = true
			case "ingest":
				scope.Capabilities = types.StringArray{"ingest"}
			}
			c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
		default:
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})
	g := newRBACGuards(&config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	v1 := engine.Group("/api/v1")
	v1.Use(g.apiKeyAuthorizer.Middleware())
	require.NoError(t, RegisterExtensionRoutes(v1, regs, g))
	// The router's own start-up check must accept what the extension declared.
	require.NotPanics(t, func() { g.assertAPIKeyPoliciesMatchRoutes(engine) })
	return engine
}

func do(engine *gin.Engine, method, path string, header ...string) int {
	req := httptest.NewRequest(method, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code
}

func TestExtensionRoutesAppearAndRequireAuth(t *testing.T) {
	engine := newExtensionEngine(t, fakeRegistrar{})

	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/acme/private", "Authorization", "Bearer x"))
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/api/v1/acme/private"))
	assert.Equal(t, http.StatusUnauthorized, do(engine, http.MethodGet, "/api/v1/acme/any"))
	assert.Equal(t, http.StatusNotFound,
		do(engine, http.MethodGet, "/api/v1/acme/missing", "Authorization", "Bearer x"))
}

func TestExtensionRoutesAreDeniedToAPIKeysUnlessDeclared(t *testing.T) {
	engine := newExtensionEngine(t, fakeRegistrar{})

	// Undeclared: closed even to a full-access key, both via Group and via a
	// zero policy.
	assert.Equal(t, http.StatusForbidden, do(engine, http.MethodGet, "/api/v1/acme/private", "X-API-Key", "full"))
	assert.Equal(t, http.StatusForbidden, do(engine, http.MethodGet, "/api/v1/acme/undeclared", "X-API-Key", "full"))
	// A session is not subject to the gate.
	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/acme/undeclared", "Authorization", "Bearer x"))

	// Declared any-key.
	assert.Equal(t, http.StatusOK, do(engine, http.MethodGet, "/api/v1/acme/any", "X-API-Key", "plain"))

	// Declared capability: full access and the capability pass, a bare key does not.
	assert.Equal(t, http.StatusOK, do(engine, http.MethodPost, "/api/v1/acme/ingest", "X-API-Key", "full"))
	assert.Equal(t, http.StatusOK, do(engine, http.MethodPost, "/api/v1/acme/ingest", "X-API-Key", "ingest"))
	assert.Equal(t, http.StatusForbidden, do(engine, http.MethodPost, "/api/v1/acme/ingest", "X-API-Key", "plain"))
}

func TestExtensionRoutesHonourRequireFeature(t *testing.T) {
	off := extension.StaticFeatures{"acme": {Enabled: false, Reason: "license_expired"}}
	on := extension.StaticFeatures{"acme": {Enabled: true}}

	get := func(f extension.Features) int {
		engine := newExtensionEngine(t, fakeRegistrar{features: f})
		return do(engine, http.MethodGet, "/api/v1/acme/gated", "Authorization", "Bearer x")
	}
	assert.Equal(t, http.StatusForbidden, get(off))
	assert.Equal(t, http.StatusOK, get(on))
}

func TestRegisterExtensionRoutesReportsAFailingRegistrar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := newRBACGuards(&config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	v1 := gin.New().Group("/api/v1")
	err := RegisterExtensionRoutes(v1, []extension.RouteRegistrar{nil, fakeRegistrar{err: errors.New("boom")}}, g)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Contains(t, err.Error(), "fakeRegistrar")
}

func TestExtensionRoutesCannotShadowACoreRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := newRBACGuards(&config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	v1.GET("/models", func(*gin.Context) {})
	shadow := shadowRegistrar{}
	assert.Panics(t, func() { _ = RegisterExtensionRoutes(v1, []extension.RouteRegistrar{shadow}, g) })
}

type shadowRegistrar struct{}

func (shadowRegistrar) Register(r extension.Routes) error {
	r.Group().GET("/models", func(*gin.Context) {})
	return nil
}

// The dig tag on RouterParams is a string literal, so nothing ties it to the
// constant extensions provide into; this does.
func TestRouterParamsCollectsTheRouteRegistrarGroup(t *testing.T) {
	f, ok := reflect.TypeOf(RouterParams{}).FieldByName("RouteRegistrars")
	require.True(t, ok)
	assert.Equal(t, extension.RouteRegistrarGroup, f.Tag.Get("group"))
}
