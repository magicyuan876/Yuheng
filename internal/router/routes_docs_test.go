package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	dochandler "github.com/magicyuan876/yuheng/internal/docs/handler"
	"github.com/magicyuan876/yuheng/internal/types"
)

func newDocsTestModule() *docs.Module {
	return &docs.Module{
		Enabled: true,
		Guard:   acl.NewGuard(nil),
		Handler: dochandler.New(dochandler.Deps{}),
	}
}

func TestDocsRoutesDeclareCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterDocsRoutes(v1, newDocsTestModule(), g)

	cases := []struct {
		method string
		path   string
		want   types.APIKeyCapability
	}{
		{http.MethodGet, "/api/v1/docs/spaces", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/spaces/:sid/tree", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/pages/:pid/content", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/search", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/events", types.APIKeyCapabilityDocsRead},
		{http.MethodPost, "/api/v1/docs/spaces", types.APIKeyCapabilityDocsWrite},
		{http.MethodPost, "/api/v1/docs/pages", types.APIKeyCapabilityDocsWrite},
		{http.MethodPut, "/api/v1/docs/pages/:pid/content", types.APIKeyCapabilityDocsWrite},
		{http.MethodGet, "/api/v1/docs/pages/:pid/lease", types.APIKeyCapabilityDocsRead},
		{http.MethodPost, "/api/v1/docs/pages/:pid/lease", types.APIKeyCapabilityDocsWrite},
		{http.MethodDelete, "/api/v1/docs/pages/:pid/lease", types.APIKeyCapabilityDocsWrite},
		{http.MethodGet, "/api/v1/docs/pages/:pid/ydoc", types.APIKeyCapabilityDocsRead},
		{http.MethodPut, "/api/v1/docs/pages/:pid/ydoc", types.APIKeyCapabilityDocsWrite},
		{http.MethodDelete, "/api/v1/docs/pages/:pid", types.APIKeyCapabilityDocsWrite},
		{http.MethodPost, "/api/v1/docs/spaces/:sid/attachments", types.APIKeyCapabilityDocsWrite},
		{http.MethodGet, "/api/v1/docs/attachments/:aid", types.APIKeyCapabilityDocsRead},
		{http.MethodDelete, "/api/v1/docs/attachments/:aid", types.APIKeyCapabilityDocsWrite},
		{http.MethodGet, "/api/v1/docs/pages/:pid/attachments", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/pages/:pid/backlinks", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/pages/:pid/mention-candidates", types.APIKeyCapabilityDocsRead},
		{http.MethodGet, "/api/v1/docs/page-links/suggest", types.APIKeyCapabilityDocsRead},
		// A POST that only reads, so it declares the read capability: the id
		// list is as long as the open page has links and does not fit a URL.
		{http.MethodPost, "/api/v1/docs/page-links/titles", types.APIKeyCapabilityDocsRead},
		{http.MethodPut, "/api/v1/docs/pages/:pid/access", types.APIKeyCapabilityDocsAdmin},
		{http.MethodPost, "/api/v1/docs/pages/:pid/grants", types.APIKeyCapabilityDocsAdmin},
		{http.MethodPost, "/api/v1/groups", types.APIKeyCapabilityDocsAdmin},
		{http.MethodDelete, "/api/v1/groups/:gid/members/:uid", types.APIKeyCapabilityDocsAdmin},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			policy := mustLookupAPIKeyPolicy(t, g, tc.method, tc.path)
			if !policy.RequireFullAccess {
				t.Fatal("policy should require full access without a matching capability")
			}
			if !policyHasCapability(policy, tc.want) {
				t.Fatalf("capabilities = %#v, want %s", policy.Capabilities, tc.want)
			}
		})
	}
}

func TestDocsRoutesEveryRouteHasAPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	engine := gin.New()
	RegisterDocsRoutes(engine.Group("/api/v1"), newDocsTestModule(), g)
	for _, route := range engine.Routes() {
		if _, ok := g.ensureAPIKeyAuthorizer().Lookup(route.Method, route.Path); !ok {
			t.Errorf("%s %s has no API-key policy; declare it through apiKeyGroup", route.Method, route.Path)
		}
	}
	if len(engine.Routes()) < 60 {
		t.Fatalf("expected the full docs route tree, got %d routes", len(engine.Routes()))
	}
}

func TestDocsRoutesAreAbsentWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	engine := gin.New()
	RegisterDocsRoutes(engine.Group("/api/v1"), &docs.Module{Enabled: false}, g)
	RegisterDocsRoutes(engine.Group("/api/v1"), nil, g)
	if n := len(engine.Routes()); n != 0 {
		t.Fatalf("disabled module must register nothing, got %d routes", n)
	}
}

func TestDocsPlaceholderRoutesAnswer501NotDenied(t *testing.T) {
	// A JWT caller with a tenant but no docs identity resolver: the guard
	// short-circuits before the placeholder, so exercise a guard-free route.
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/x", dochandlerNotImplemented)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("placeholder = %d, want 501", rec.Code)
	}
}
