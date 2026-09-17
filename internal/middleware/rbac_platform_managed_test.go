package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
)

// platformManagedHarness mirrors rbacTestHarness but also seeds the
// system-admin flag and (optionally) an API-key scope, which are the two
// axes RequirePlatformManaged consults beyond the tenant role.
func platformManagedHarness(
	role types.TenantRole, systemAdmin bool, apiKey bool, mw gin.HandlerFunc,
) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.SystemAdminContextKey, systemAdmin)
		if apiKey {
			ctx = types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{FullAccess: true})
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/protected", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	return w
}

func centralized(on bool) CentralizedInfraResolver {
	return func(context.Context) bool { return on }
}

func platformManaged(cfg *config.Config, on bool) gin.HandlerFunc {
	return RequirePlatformManaged(types.TenantRoleAdmin, cfg, centralized(on))
}

// ---------- switch OFF: behaves like RequireRoleOrSystemAdmin ----------

func TestPlatformManaged_SwitchOffAllowsWorkspaceAdmin(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleAdmin, false, false,
		platformManaged(cfgRBAC(true), false))
	assert.Equal(t, http.StatusOK, w.Code,
		"with centralised mode off the workspace admin still owns their infrastructure")
}

func TestPlatformManaged_SwitchOffStillBlocksContributor(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleContributor, false, false,
		platformManaged(cfgRBAC(true), false))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// The routes this guard replaced used RequireRoleOrSystemAdmin so a platform
// operator could maintain built-in models while holding no role in whatever
// workspace they were browsing. Losing that when the switch is off would be a
// silent regression, so it is pinned here.
func TestPlatformManaged_SwitchOffAllowsSystemAdminWithoutRole(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleViewer, true, false,
		platformManaged(cfgRBAC(true), false))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- switch ON: platform-owned ----------

func TestPlatformManaged_SwitchOnBlocksWorkspaceOwner(t *testing.T) {
	// Owner, not merely Admin: every self-registered user owns their personal
	// workspace, so Owner is the role that actually has to be rejected for
	// centralised mode to mean anything.
	w := platformManagedHarness(types.TenantRoleOwner, false, false,
		platformManaged(cfgRBAC(true), true))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPlatformManaged_SwitchOnAllowsSystemAdmin(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleViewer, true, false,
		platformManaged(cfgRBAC(true), true))
	assert.Equal(t, http.StatusOK, w.Code)
}

// EnableRBAC is the rollout flag for the per-tenant role ladder; centralised
// mode is a separate, explicit opt-in by a system administrator. Gating one
// behind the other would mean an operator flips the switch and nothing
// happens, so the denial is enforced regardless (mirroring RequireSystemAdmin).
func TestPlatformManaged_SwitchOnEnforcedEvenWithRBACDisabled(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleOwner, false, false,
		platformManaged(cfgRBAC(false), true))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// With the switch off we delegate to RequireRole, so EnableRBAC=false keeps
// its documented "log but pass through" rollout behaviour.
func TestPlatformManaged_SwitchOffHonoursRBACRolloutFlag(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleContributor, false, false,
		platformManaged(cfgRBAC(false), false))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- API keys ----------

// Machine principals are authorised solely by the APIKeyGate (capability +
// KB scope + default-deny). Re-checking them here would double-gate them
// against a role they do not carry.
func TestPlatformManaged_APIKeyShortCircuits(t *testing.T) {
	w := platformManagedHarness(types.TenantRoleViewer, false, true,
		platformManaged(cfgRBAC(true), true))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- resolver wiring gaps ----------

// A nil resolver means the system-setting service was not wired. Failing open
// to "decentralised" keeps a wiring gap from locking every workspace admin out
// of their own infrastructure.
func TestPlatformManaged_NilResolverFallsBackToRole(t *testing.T) {
	mw := RequirePlatformManaged(types.TenantRoleAdmin, cfgRBAC(true), nil)
	assert.Equal(t, http.StatusOK,
		platformManagedHarness(types.TenantRoleAdmin, false, false, mw).Code)
	assert.Equal(t, http.StatusForbidden,
		platformManagedHarness(types.TenantRoleViewer, false, false, mw).Code)
}
