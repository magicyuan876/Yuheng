package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

func TestTenantOptionalAPISurface(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/api/v1/auth/me", true},
		{http.MethodPut, "/api/v1/auth/me", true},
		{http.MethodPut, "/api/v1/auth/me/preferences", true},
		{http.MethodPost, "/api/v1/tenants", true},
		{http.MethodGet, "/api/v1/me/invitations", true},
		{http.MethodPost, "/api/v1/me/invitations/12/accept", true},
		{http.MethodGet, "/api/v1/knowledge-bases", false},
		{http.MethodGet, "/api/v1/tenants", false},
	}
	for _, tt := range tests {
		if got := isTenantOptionalAPI(tt.path, tt.method); got != tt.want {
			t.Errorf("isTenantOptionalAPI(%s %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

// fakeUserService answers ResolveActiveTenantID with a fixed workspace and
// records how often it was consulted; the middleware must only fall back to
// it when neither the header nor the JWT names a workspace.
type fakeUserService struct {
	interfaces.UserService
	resolved uint64
	calls    int
}

func (f *fakeUserService) ResolveActiveTenantID(context.Context, *types.User) uint64 {
	f.calls++
	return f.resolved
}

func newResolveTargetContext(t *testing.T, tenantHeader string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	if tenantHeader != "" {
		c.Request.Header.Set("X-Tenant-ID", tenantHeader)
	}
	return c
}

// The resolution order the whole auth stack shares: header, then JWT claim,
// then the user service's remembered-or-earliest workspace.
func TestResolveTargetTenantOrder(t *testing.T) {
	user := &types.User{ID: "u1"}
	members := newFakeMemberService()
	members.seedActive("u1", 42, types.TenantRoleViewer)
	members.seedActive("u1", 7, types.TenantRoleViewer)
	tenants := &fakeTenantService{tenant: &types.Tenant{ID: 7}}

	t.Run("header wins when the user is a member there", func(t *testing.T) {
		users := &fakeUserService{resolved: 42}
		c := newResolveTargetContext(t, "7")
		got, tenant, ok := resolveTargetTenant(c, tenants, users, members, cfgWithRBAC(true), user, 42)
		if !ok || got != 7 || tenant == nil || tenant.ID != 7 || users.calls != 0 {
			t.Fatalf("got (%d, %v, %v) calls=%d, want 7 with its tenant and no fallback", got, tenant, ok, users.calls)
		}
	})
	t.Run("JWT claim is used as is", func(t *testing.T) {
		users := &fakeUserService{resolved: 7}
		c := newResolveTargetContext(t, "")
		got, _, ok := resolveTargetTenant(c, tenants, users, members, cfgWithRBAC(true), user, 42)
		if !ok || got != 42 || users.calls != 0 {
			t.Fatalf("got (%d, %v) calls=%d, want the claim 42 and no fallback", got, ok, users.calls)
		}
	})
	t.Run("a token without a workspace falls back to the user service", func(t *testing.T) {
		users := &fakeUserService{resolved: 42}
		c := newResolveTargetContext(t, "")
		got, _, ok := resolveTargetTenant(c, tenants, users, members, cfgWithRBAC(true), user, 0)
		if !ok || got != 42 || users.calls != 1 {
			t.Fatalf("got (%d, %v) calls=%d, want 42 from exactly one fallback", got, ok, users.calls)
		}
	})
	t.Run("no workspace anywhere is tenantless, not an error", func(t *testing.T) {
		users := &fakeUserService{resolved: 0}
		c := newResolveTargetContext(t, "")
		got, _, ok := resolveTargetTenant(c, tenants, users, members, cfgWithRBAC(true), user, 0)
		if !ok || got != 0 || c.IsAborted() {
			t.Fatalf("got (%d, %v) aborted=%v, want (0, true) with the response untouched", got, ok, c.IsAborted())
		}
	})
	t.Run("header for a workspace without membership is refused", func(t *testing.T) {
		users := &fakeUserService{resolved: 42}
		c := newResolveTargetContext(t, "99")
		_, _, ok := resolveTargetTenant(c, tenants, users, members, cfgWithRBAC(true), user, 42)
		if ok || !c.IsAborted() || c.Writer.Status() != http.StatusForbidden {
			t.Fatalf("ok=%v aborted=%v status=%d, want a 403 abort", ok, c.IsAborted(), c.Writer.Status())
		}
	})
}
