package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// groupRoutes is the contract: method, path, and whether the route changes
// anything (Admin) or only reads (Viewer).
var groupRoutes = []struct {
	method string
	path   string
	admin  bool
}{
	{http.MethodGet, "/api/v1/groups", false},
	{http.MethodPost, "/api/v1/groups", true},
	{http.MethodGet, "/api/v1/groups/:gid", false},
	{http.MethodPatch, "/api/v1/groups/:gid", true},
	{http.MethodDelete, "/api/v1/groups/:gid", true},
	{http.MethodGet, "/api/v1/groups/:gid/members", false},
	{http.MethodPut, "/api/v1/groups/:gid/members", true},
	{http.MethodDelete, "/api/v1/groups/:gid/members/:uid", true},
}

// Groups are workspace membership, so an API key reaches them through
// manage_members -- the capability that reaches /tenants/:id/members -- and
// not through anything of the docs module's.
func TestGroupRoutesDeclareTheMembersCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	RegisterGroupRoutes(gin.New().Group("/api/v1"), &handler.TenantGroupHandler{}, g)
	for _, tc := range groupRoutes {
		policy := mustLookupAPIKeyPolicy(t, g, tc.method, tc.path)
		if !policy.RequireFullAccess {
			t.Errorf("%s %s: policy should require full access without a matching capability", tc.method, tc.path)
		}
		if !policyHasCapability(policy, types.APIKeyCapabilityManageMembers) {
			t.Errorf("%s %s: capabilities = %#v, want manage_members", tc.method, tc.path, policy.Capabilities)
		}
		if policyHasCapability(policy, types.APIKeyCapabilityDocsAdmin) {
			t.Errorf("%s %s: still reachable through docs_admin", tc.method, tc.path)
		}
	}
}

// listingGroupService answers List and nothing else, which is all a route
// test needs to see a request reach the handler.
type listingGroupService struct{ interfaces.TenantGroupService }

func (listingGroupService) List(context.Context, uint64) ([]*types.TenantGroupView, error) {
	return []*types.TenantGroupView{}, nil
}

func (listingGroupService) Create(_ context.Context, tenantID uint64,
	in types.CreateTenantGroupInput,
) (*types.TenantGroupView, error) {
	return &types.TenantGroupView{TenantGroup: &types.TenantGroup{ID: "g1", TenantID: tenantID, Name: in.Name}}, nil
}

// The reason the routes moved: a deployment with the docs module off still
// has workspace groups. Every route is registered, and a request by a
// member is served by the handler rather than falling to 404 -- with the
// role floor the docs module applied, Admin for writes and Viewer for reads.
func TestGroupRoutesServeRequestsWithTheDocsModuleDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// A nil config switches role enforcement off (fail-open, with a loud
	// log line); the role floor is part of what is asserted here.
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{}}}
	as := func(role types.TenantRole) gin.HandlerFunc {
		return func(c *gin.Context) {
			ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
			ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
			c.Request = c.Request.WithContext(ctx)
		}
	}
	// One engine per caller role, because the role is what the test varies
	// and gin middleware is fixed at registration.
	build := func(role types.TenantRole) *gin.Engine {
		e := gin.New()
		e.Use(as(role))
		v1 := e.Group("/api/v1")
		RegisterDocsRoutes(v1, &docs.Module{Enabled: false}, g)
		RegisterGroupRoutes(v1, handler.NewTenantGroupHandler(listingGroupService{}), g)
		return e
	}
	engine := build(types.TenantRoleViewer)

	registered := map[string]bool{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
		if strings.HasPrefix(route.Path, "/api/v1/docs") {
			t.Fatalf("the disabled docs module registered %s", route.Path)
		}
	}
	for _, tc := range groupRoutes {
		if !registered[tc.method+" "+tc.path] {
			t.Errorf("%s %s is not registered with the docs module disabled", tc.method, tc.path)
		}
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /groups as a viewer = %d %s, want 200", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{"name":"x"}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /groups as a viewer = %d, want 403", rec.Code)
	}

	admin := build(types.TenantRoleAdmin)
	rec = httptest.NewRecorder()
	admin.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{"name":"x"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /groups as an admin = %d %s, want 201", rec.Code, rec.Body.String())
	}
}
