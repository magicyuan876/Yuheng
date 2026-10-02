package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
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

// runGuard fires a single request for kbID through the guard on behalf of
// tenantID and returns the gin context so the test can inspect the abort
// state, the recorded error and the stashed access. ctxExtra lets a test
// decorate the request context (e.g. with an API-key scope) before the guard
// sees it.
func runGuard(
	t *testing.T,
	tenantID uint64,
	kbID string,
	lookup KBLookup,
	ctxExtra func(context.Context) context.Context,
) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: kbID}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	if tenantID != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	}
	if ctxExtra != nil {
		ctx = ctxExtra(ctx)
	}
	c.Request = req.WithContext(ctx)

	RequireKBAccess(KBIDFromParam("id"), lookup)(c)
	return c
}

// firstErrorStatus returns the HTTP status of the first AppError the guard
// recorded, which is what ErrorHandler would turn into the response.
func firstErrorStatus(t *testing.T, c *gin.Context) int {
	t.Helper()
	require.NotEmpty(t, c.Errors, "guard aborted without recording an error")
	appErr, ok := apperrors.IsAppError(c.Errors[0].Err)
	require.True(t, ok, "recorded error is not an AppError: %v", c.Errors[0].Err)
	return appErr.HTTPCode
}

func TestRequireKBAccess_OwnKB(t *testing.T) {
	kb := &types.KnowledgeBase{ID: "kb-1", TenantID: 100}
	c := runGuard(t, 100, "kb-1", &stubKBLookup{kbs: map[string]*types.KnowledgeBase{"kb-1": kb}}, nil)

	require.False(t, c.IsAborted(), "own KB must pass through")
	require.Empty(t, c.Errors)
	access, ok := KBAccessFromContext(c)
	require.True(t, ok, "the resolved KB is stashed for the handler")
	require.Same(t, kb, access.KnowledgeBase)
	// The guard never rewrites the request context: the caller's own
	// workspace is the only one a knowledge base can be served from.
	got, ok := types.TenantIDFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, uint64(100), got)
}

// A knowledge base owned by another workspace does not exist from the
// caller's point of view: 404, not 403, so ids cannot be probed for which
// workspace owns them.
func TestRequireKBAccess_ForeignKB_NotFound(t *testing.T) {
	lookup := &stubKBLookup{kbs: map[string]*types.KnowledgeBase{
		"kb-foreign": {ID: "kb-foreign", TenantID: 200},
	}}
	c := runGuard(t, 100, "kb-foreign", lookup, nil)

	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusNotFound, firstErrorStatus(t, c))
	_, ok := KBAccessFromContext(c)
	require.False(t, ok, "no access should be stashed on failure")
}

func TestRequireKBAccess_MissingKB_NotFound(t *testing.T) {
	c := runGuard(t, 100, "kb-missing", &stubKBLookup{kbs: map[string]*types.KnowledgeBase{}}, nil)

	require.True(t, c.IsAborted(), "missing KB must abort")
	require.Equal(t, http.StatusNotFound, firstErrorStatus(t, c))
	_, ok := KBAccessFromContext(c)
	require.False(t, ok, "no access should be stashed on failure")
}

func TestRequireKBAccess_NoTenant_Unauthorized(t *testing.T) {
	c := runGuard(t, 0, "kb-1", &stubKBLookup{kbs: map[string]*types.KnowledgeBase{
		"kb-1": {ID: "kb-1", TenantID: 100},
	}}, nil)

	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusUnauthorized, firstErrorStatus(t, c))
}

// A lookup failure is not "not found": the guard answers 503 so the caller
// retries and monitoring sees the underlying fault.
func TestRequireKBAccess_LookupFailure_ServiceUnavailable(t *testing.T) {
	c := runGuard(t, 100, "kb-1", &stubKBLookup{getErr: errors.New("connection refused")}, nil)

	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusServiceUnavailable, firstErrorStatus(t, c))
}

// A key restricted to some knowledge bases is confined to them even for a
// base its own workspace owns; the allow-list runs before the lookup so a
// denied id is never fetched.
func TestRequireKBAccess_RestrictedAPIKey_OutsideAllowList_Forbidden(t *testing.T) {
	lookup := &stubKBLookup{kbs: map[string]*types.KnowledgeBase{
		"kb-1": {ID: "kb-1", TenantID: 100},
	}}
	restricted := func(ctx context.Context) context.Context {
		scope := types.TenantAPIKeyScope{KnowledgeBaseIDs: types.StringArray{"kb-other"}}
		return types.WithTenantAPIKeyScope(ctx, scope)
	}
	c := runGuard(t, 100, "kb-1", lookup, restricted)

	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusForbidden, firstErrorStatus(t, c))
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
