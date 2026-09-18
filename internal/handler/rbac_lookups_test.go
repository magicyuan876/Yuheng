package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The lookup helpers translate handler/service errors into the
// middleware sentinel set; that translation is where bugs hide (404
// becoming 403, cross-tenant leaks, etc.), so we test those edges
// directly rather than via end-to-end HTTP.

// stubKBService implements just enough of interfaces.KnowledgeBaseService
// to drive KBCreatorLookup. Any other method panics so the test fails
// loudly if a future lookup refactor reaches outside the contract.
type stubKBService struct {
	interfaces.KnowledgeBaseService
	get func(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

func (s *stubKBService) GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error) {
	return s.get(ctx, id)
}

func newKBLookupCtx(t *testing.T, tenantID uint64, paramID string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenantID)
	c.Request = c.Request.WithContext(ctx)
	c.Params = gin.Params{{Key: "id", Value: paramID}}
	return c
}

func TestKBCreatorLookup_NotFoundMapsToSentinel(t *testing.T) {
	h := &KnowledgeBaseHandler{service: &stubKBService{
		get: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return nil, apprepo.ErrKnowledgeBaseNotFound
		},
	}}
	_, err := h.KBCreatorLookup(newKBLookupCtx(t, 1, "kb-1"))
	if !errors.Is(err, middleware.ErrResourceNotFound) {
		t.Fatalf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestKBCreatorLookup_CrossTenantIsHiddenAsNotFound(t *testing.T) {
	// A foreign-tenant KB must NEVER leak via the ownership shortcut.
	// Returning the row's CreatorID would let a user-id collision pass
	// the middleware's "creator == uid" branch; hiding it as not-found
	// keeps the lookup strictly tenant-scoped.
	h := &KnowledgeBaseHandler{service: &stubKBService{
		get: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb-1", TenantID: 999, CreatorID: "u1"}, nil
		},
	}}
	_, err := h.KBCreatorLookup(newKBLookupCtx(t, 1, "kb-1"))
	if !errors.Is(err, middleware.ErrResourceNotFound) {
		t.Fatalf("cross-tenant KB must surface as not-found, got %v", err)
	}
}

func TestKBCreatorLookup_OwnerMatchReturnsCreatorID(t *testing.T) {
	h := &KnowledgeBaseHandler{service: &stubKBService{
		get: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb-1", TenantID: 1, CreatorID: "u-creator"}, nil
		},
	}}
	creator, err := h.KBCreatorLookup(newKBLookupCtx(t, 1, "kb-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creator != "u-creator" {
		t.Fatalf("expected creator=u-creator, got %q", creator)
	}
}

func TestKBCreatorLookup_MissingTenantContext(t *testing.T) {
	// Without tenant context, the lookup can't decide scope. Surfacing
	// a real error (which middleware turns into 503) is safer than
	// silently returning ErrResourceNotFound: the request shouldn't be
	// happening at all.
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	c.Params = gin.Params{{Key: "id", Value: "kb-1"}}
	h := &KnowledgeBaseHandler{service: &stubKBService{
		get: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			t.Fatalf("service must not be called without tenant context")
			return nil, nil
		},
	}}
	_, err := h.KBCreatorLookup(c)
	if err == nil {
		t.Fatalf("expected error when workspace context missing")
	}
	if errors.Is(err, middleware.ErrResourceNotFound) {
		t.Fatalf("missing tenant must not be reported as not-found: %v", err)
	}
}
