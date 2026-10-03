package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/types"
)

// The workspace-catalog and admin-membership routes are gated before the
// handler runs, so an ordinary workspace Owner never reaches them while a
// system administrator does. The handlers here are zero values: the test
// stops at the guards, which is where the 403 must come from. The admin
// 200 path with real services runs in container/e2e_admin_test.go.

// workspaceAdminRouter registers the tenant and system-admin routes behind
// the real guards. identity, when given, seeds the request context the way
// the auth middleware would, before any route group runs.
func workspaceAdminRouter(t *testing.T, identity gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{}}}
	engine := gin.New()
	if identity != nil {
		engine.Use(identity)
	}
	// Guards report through c.Error; render it as the status the
	// production ErrorHandler would.
	engine.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 && !c.Writer.Written() {
			c.Status(http.StatusForbidden)
		}
	})
	v1 := engine.Group("/api/v1")
	RegisterTenantRoutes(v1, &handler.TenantHandler{}, &handler.TenantMemberHandler{}, nil, nil, g)
	RegisterSystemAdminRoutes(v1, &handler.SystemHandler{}, &handler.TenantMemberHandler{}, nil, g)
	return engine
}

// asWorkspaceOwner seeds the context of a user who owns workspace 1 and is
// not a system administrator.
func asWorkspaceOwner(c *gin.Context) {
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, types.UserContextKey, &types.User{ID: "owner"})
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner")
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
	ctx = context.WithValue(ctx, types.SystemAdminContextKey, false)
	c.Request = c.Request.WithContext(ctx)
}

func TestWorkspaceCatalogAndAdminMembershipRoutesRefuseAWorkspaceOwner(t *testing.T) {
	engine := workspaceAdminRouter(t, asWorkspaceOwner)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/tenants/all"},
		{http.MethodGet, "/api/v1/tenants/search"},
		{http.MethodPost, "/api/v1/tenants"},
		{http.MethodGet, "/api/v1/system/admin/tenants/1/members"},
		{http.MethodPost, "/api/v1/system/admin/tenants/1/members"},
		{http.MethodPatch, "/api/v1/system/admin/tenants/1/members/u2"},
		{http.MethodDelete, "/api/v1/system/admin/tenants/1/members/u2"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 for a workspace owner who is not a system administrator", w.Code)
			}
		})
	}
}

// The admin membership routes coexist with the static
// /system/admin/tenants/apply-default-storage-quota route under the same
// prefix; gin must register both.
func TestAdminMembershipRoutesAreRegisteredNextToTheBulkQuotaRoute(t *testing.T) {
	engine := workspaceAdminRouter(t, nil)
	registered := map[string]bool{}
	for _, ri := range engine.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"POST /api/v1/system/admin/tenants/apply-default-storage-quota",
		"GET /api/v1/system/admin/tenants/:id/members",
		"POST /api/v1/system/admin/tenants/:id/members",
		"PATCH /api/v1/system/admin/tenants/:id/members/:user_id",
		"DELETE /api/v1/system/admin/tenants/:id/members/:user_id",
	} {
		if !registered[want] {
			t.Errorf("route %s is not registered", want)
		}
	}
}

func TestAdminMembershipRoutesDeclarePlatformCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterSystemAdminRoutes(v1, &handler.SystemHandler{}, &handler.TenantMemberHandler{}, nil, g)

	cases := []struct {
		method     string
		path       string
		capability types.APIKeyCapability
	}{
		{http.MethodGet, "/api/v1/system/admin/tenants/:id/members", types.APIKeyCapabilitySystemTenantsRead},
		{http.MethodPost, "/api/v1/system/admin/tenants/:id/members", types.APIKeyCapabilitySystemTenantsManage},
		{
			http.MethodPatch, "/api/v1/system/admin/tenants/:id/members/:user_id",
			types.APIKeyCapabilitySystemTenantsManage,
		},
		{
			http.MethodDelete, "/api/v1/system/admin/tenants/:id/members/:user_id",
			types.APIKeyCapabilitySystemTenantsManage,
		},
	}
	for _, tc := range cases {
		policy := mustLookupAPIKeyPolicy(t, g, tc.method, tc.path)
		if !policy.PlatformOnly {
			t.Fatalf("%s %s must be platform-only", tc.method, tc.path)
		}
		if !policyHasCapability(policy, tc.capability) {
			t.Fatalf("%s %s capabilities = %#v, want %s", tc.method, tc.path, policy.Capabilities, tc.capability)
		}
	}
}
