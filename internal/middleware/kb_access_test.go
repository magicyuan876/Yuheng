package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubKBLookup is a tiny KBLookup stand-in for tests; satisfies the
// KBLookup interface (a single method) without dragging in the full
// KnowledgeBaseService surface.
type stubKBLookup struct {
	kbs    map[string]*types.KnowledgeBase
	getErr error
}

func (s *stubKBLookup) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if kb, ok := s.kbs[id]; ok {
		return kb, nil
	}
	return nil, apprepo.ErrKnowledgeBaseNotFound
}

// stubKBShareForGuard implements just the methods the guard touches —
// CheckTenantKBPermission and GetKBSourceTenant. The other methods on
// the interface panic so any unintended new dependency surfaces
// immediately.
type stubKBShareForGuard struct {
	permission map[string]types.OrgMemberRole
	shared     map[string]bool
	source     map[string]uint64
}

func (s *stubKBShareForGuard) CheckTenantKBPermission(_ context.Context, kbID string, _ uint64, _ types.TenantRole) (types.OrgMemberRole, bool, error) {
	if s.shared[kbID] {
		return s.permission[kbID], true, nil
	}
	return "", false, nil
}

func (s *stubKBShareForGuard) GetKBSourceTenant(_ context.Context, kbID string) (uint64, error) {
	if v, ok := s.source[kbID]; ok {
		return v, nil
	}
	return 0, errors.New("not found")
}

func (s *stubKBShareForGuard) ShareKnowledgeBase(context.Context, string, string, string, uint64, types.OrgMemberRole) (*types.KnowledgeBaseShare, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) UpdateSharePermission(context.Context, string, types.OrgMemberRole, string, uint64) error {
	panic("not implemented")
}

func (s *stubKBShareForGuard) RemoveShare(context.Context, string, string, uint64) error {
	panic("not implemented")
}

func (s *stubKBShareForGuard) ListSharesByKnowledgeBase(context.Context, string, uint64) ([]*types.KnowledgeBaseShare, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) ListSharesByOrganization(context.Context, string) ([]*types.KnowledgeBaseShare, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) ListSharedKnowledgeBases(context.Context, uint64, types.TenantRole) ([]*types.SharedKnowledgeBaseInfo, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) ListSharedKnowledgeBasesInOrganization(context.Context, string, uint64, types.TenantRole) ([]*types.OrganizationSharedKnowledgeBaseItem, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) ListSharedKnowledgeBaseIDsByOrganizations(context.Context, []string, uint64) (map[string][]string, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) GetShare(context.Context, string) (*types.KnowledgeBaseShare, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) GetShareByKBAndOrg(context.Context, string, string) (*types.KnowledgeBaseShare, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) HasTenantKBPermission(context.Context, string, uint64, types.TenantRole, types.OrgMemberRole) (bool, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) CountSharesByKnowledgeBaseIDs(context.Context, []string) (map[string]int64, error) {
	panic("not implemented")
}

func (s *stubKBShareForGuard) CountByOrganizations(context.Context, []string) (map[string]int64, error) {
	panic("not implemented")
}

// guardOpts collects optional knobs for runGuard. Keeps the call site
// readable when most tests only care about a couple of dimensions.
type guardOpts struct{}

// runGuard fires a single request through the guard and returns the
// gin recorder + the kb access (if any) the guard stashed. Defaults
// to EnableRBAC=true; the EnableRBAC=false fail-open path has its own
// dedicated tests further below.
func runGuard(
	t *testing.T,
	tenantID uint64,
	kbID string,
	requiredPerm types.OrgMemberRole,
	kb *types.KnowledgeBase,
	share *stubKBShareForGuard,
	opts guardOpts,
) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: kbID}}

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenantID)
	c.Request = req.WithContext(ctx)

	kbsvc := &stubKBLookup{kbs: map[string]*types.KnowledgeBase{}}
	if kb != nil {
		kbsvc.kbs[kbID] = kb
	}

	// Convert the package-local concrete stub types to typed interface
	// nils when they aren't supplied — otherwise the interface wraps a
	// nil pointer and `iface != nil` evaluates true on the guard side
	// (classic Go typed-nil trap).
	var shareSvc interfaces.KBShareService
	if share != nil {
		shareSvc = share
	}
	guard := RequireKBAccess(
		KBIDFromParam("id"),
		requiredPerm,
		kbsvc,
		shareSvc,
		cfgRBAC(true),
	)
	guard(c)
	return rec, c
}

func TestRequireKBAccess_OwnKB(t *testing.T) {
	rec, c := runGuard(t, 100, "kb-1",
		types.OrgRoleViewer,
		&types.KnowledgeBase{ID: "kb-1", TenantID: 100},
		nil,
		guardOpts{},
	)
	require.False(t, c.IsAborted(), "should pass through")
	require.Equal(t, 200, rec.Code) // gin's default; nothing wrote a status
	access, ok := KBAccessFromContext(c)
	require.True(t, ok)
	require.Equal(t, uint64(100), access.EffectiveTenantID)
	require.Equal(t, types.OrgRoleAdmin, access.Permission, "own KB grants admin")
	// The request context's tenant should still be the caller's own.
	got, ok := types.TenantIDFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, uint64(100), got)
}

// TestIsResourceNotFound_RecognisesKnowledgeSentinel pins that a missing
// *document* (knowledge) is treated as not-found, not a transient error.
// Regression: ErrKnowledgeNotFound was absent from the predicate, so
// GET/DELETE /knowledge/:id and chunk list resolved a missing doc into a
// raw 500 instead of a 404 — which the CLI then surfaced as a retryable
// server.error (exit 7), looping agents on a permanently-absent doc.
func TestIsResourceNotFound_RecognisesKnowledgeSentinel(t *testing.T) {
	require.True(t, isResourceNotFound(apprepo.ErrKnowledgeNotFound),
		"missing document (ErrKnowledgeNotFound) must classify as not-found")
	require.True(t, isResourceNotFound(apprepo.ErrKnowledgeBaseNotFound),
		"missing KB must still classify as not-found")
	require.True(t, isResourceNotFound(apprepo.ErrChunkNotFound),
		"missing chunk (ErrChunkNotFound) must classify as not-found — chunk view/by-id resolved a missing chunk into a raw 500 (exit 7) otherwise")
	require.True(t, isResourceNotFound(ErrResourceNotFound),
		"generic resource-not-found sentinel must still classify as not-found")
	require.False(t, isResourceNotFound(errors.New("connection refused")),
		"a genuine transient error must NOT be classified as not-found")
}

func TestRequireKBAccess_NotFound_Aborts(t *testing.T) {
	_, c := runGuard(t, 100, "kb-missing", types.OrgRoleViewer, nil, nil, guardOpts{})
	require.True(t, c.IsAborted(), "missing KB must abort")
	require.NotEmpty(t, c.Errors)
	_, ok := KBAccessFromContext(c)
	require.False(t, ok, "no access should be stashed on failure")
}

func TestRequireKBAccess_SharedKB_RewritesTenantContext(t *testing.T) {
	share := &stubKBShareForGuard{
		permission: map[string]types.OrgMemberRole{"kb-shared": types.OrgRoleEditor},
		shared:     map[string]bool{"kb-shared": true},
		source:     map[string]uint64{"kb-shared": 200},
	}
	_, c := runGuard(t, 100, "kb-shared",
		types.OrgRoleEditor,
		&types.KnowledgeBase{ID: "kb-shared", TenantID: 200},
		share,
		guardOpts{},
	)
	require.False(t, c.IsAborted())
	access, ok := KBAccessFromContext(c)
	require.True(t, ok)
	require.Equal(t, uint64(200), access.EffectiveTenantID)
	got, _ := types.TenantIDFromContext(c.Request.Context())
	require.Equal(t, uint64(200), got, "guard must rewrite context to source tenant")
}

func TestRequireKBAccess_SharedKB_PermissionBelowMin_Aborts(t *testing.T) {
	share := &stubKBShareForGuard{
		permission: map[string]types.OrgMemberRole{"kb-shared": types.OrgRoleViewer},
		shared:     map[string]bool{"kb-shared": true},
		source:     map[string]uint64{"kb-shared": 200},
	}
	_, c := runGuard(t, 100, "kb-shared",
		types.OrgRoleEditor, // require Editor
		&types.KnowledgeBase{ID: "kb-shared", TenantID: 200},
		share,
		guardOpts{},
	)
	require.True(t, c.IsAborted(), "Viewer share must reject when Editor required")
}

func TestRequireKBAccess_NoTenant_Aborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "kb-x"}}
	c.Request = httptest.NewRequest("GET", "/", nil) // no tenant in context
	guard := RequireKBAccess(
		KBIDFromParam("id"),
		types.OrgRoleViewer,
		&stubKBLookup{},
		nil,
		cfgRBAC(true),
	)
	guard(c)
	require.True(t, c.IsAborted())
}

// ---------- Agent-share fallback ----------

func TestRequireKBAccess_Forbidden_FailOpenWhenRBACDisabled(t *testing.T) {
	// Same scenario as PermissionBelowMin (which aborts when enforcing),
	// but with EnableRBAC=false the guard logs and passes through.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "kb-shared"}}
	req := httptest.NewRequest("GET", "/", nil)
	c.Request = req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, uint64(100)))

	share := &stubKBShareForGuard{
		permission: map[string]types.OrgMemberRole{"kb-shared": types.OrgRoleViewer},
		shared:     map[string]bool{"kb-shared": true},
		source:     map[string]uint64{"kb-shared": 200},
	}
	kbsvc := &stubKBLookup{kbs: map[string]*types.KnowledgeBase{
		"kb-shared": {ID: "kb-shared", TenantID: 200},
	}}

	guard := RequireKBAccess(
		KBIDFromParam("id"),
		types.OrgRoleEditor, // would-deny
		kbsvc, share,
		cfgRBAC(false), // enforcement off
	)
	guard(c)
	require.False(t, c.IsAborted(), "guard must pass through when EnableRBAC is off")
	_ = rec
}

func TestRequireKBAccess_NotFound_FiresEvenWhenRBACDisabled(t *testing.T) {
	// Not-found is not an authorisation event; the client asked for a
	// resource that genuinely isn't there. We surface 404 regardless of
	// the rollout flag (matches the comment in RequireKBAccess).
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "kb-missing"}}
	req := httptest.NewRequest("GET", "/", nil)
	c.Request = req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, uint64(100)))

	guard := RequireKBAccess(
		KBIDFromParam("id"),
		types.OrgRoleViewer,
		&stubKBLookup{kbs: map[string]*types.KnowledgeBase{}},
		nil,
		cfgRBAC(false),
	)
	guard(c)
	require.True(t, c.IsAborted(), "404 still fires with enforcement off")
	_ = rec
}
