package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
)

// catalogGuardRequest runs RequireTenantCatalogAccess with the given
// identity seeded on the request context, the way the auth middleware
// seeds it, and reports the status the guard let through or produced.
func catalogGuardRequest(t *testing.T, enableCrossTenant bool, seed func(ctx context.Context) context.Context) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(seed(c.Request.Context()))
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 && !c.Writer.Written() {
			c.JSON(http.StatusForbidden, gin.H{"error": c.Errors.Last().Error()})
		}
	})
	r.GET("/catalog", RequireTenantCatalogAccess(cfgCrossTenant(enableCrossTenant)), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/catalog", nil))
	return w.Code
}

// The catalog guard admits exactly the principals that run the deployment:
// system administrators, cross-tenant superusers (only while the flag is
// on) and platform API keys. A workspace Owner is just a user here.
func TestRequireTenantCatalogAccess_Identities(t *testing.T) {
	cases := []struct {
		name              string
		enableCrossTenant bool
		seed              func(ctx context.Context) context.Context
		want              int
	}{
		{
			name: "system administrator",
			seed: func(ctx context.Context) context.Context {
				ctx = context.WithValue(ctx, types.UserContextKey, &types.User{ID: "admin", IsSystemAdmin: true})
				return context.WithValue(ctx, types.SystemAdminContextKey, true)
			},
			want: http.StatusOK,
		},
		{
			name: "tenantless system administrator (no workspace at all)",
			seed: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, types.SystemAdminContextKey, true)
			},
			want: http.StatusOK,
		},
		{
			name: "workspace owner",
			seed: func(ctx context.Context) context.Context {
				ctx = context.WithValue(ctx, types.UserContextKey, &types.User{ID: "owner"})
				ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
				return context.WithValue(ctx, types.SystemAdminContextKey, false)
			},
			want: http.StatusForbidden,
		},
		{
			name:              "cross-tenant superuser with the flag on",
			enableCrossTenant: true,
			seed: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, types.UserContextKey, &types.User{ID: "su", CanAccessAllTenants: true})
			},
			want: http.StatusOK,
		},
		{
			name: "cross-tenant attribute with the flag off",
			seed: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, types.UserContextKey, &types.User{ID: "su", CanAccessAllTenants: true})
			},
			want: http.StatusForbidden,
		},
		{
			name: "platform API key",
			seed: func(ctx context.Context) context.Context {
				return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{ScopeType: types.APIKeyScopePlatform})
			},
			want: http.StatusOK,
		},
		{
			name: "workspace API key, even a full-access one",
			seed: func(ctx context.Context) context.Context {
				// A workspace key's user is the synthetic system user, which the
				// auth middleware marks as not a system administrator.
				ctx = context.WithValue(ctx, types.SystemAdminContextKey, false)
				return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{FullAccess: true})
			},
			want: http.StatusForbidden,
		},
		{
			name: "nothing on the context",
			seed: func(ctx context.Context) context.Context { return ctx },
			want: http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := catalogGuardRequest(t, tc.enableCrossTenant, tc.seed); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}
