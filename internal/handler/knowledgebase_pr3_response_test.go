package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// CreateKnowledgeBase typed-error preservation — the handler must surface
// the typed AppError (ErrVectorStoreBindingInvalid / ErrVectorStoreUnavailable)
// returned by validateVectorStoreBinding instead of stripping it into a
// generic 500 via NewInternalServerError. Without the IsAppError unwrap in
// the handler, the typed error codes would be silently nullified at the
// HTTP boundary and clients would lose the ability to branch on the cause.

// stubKBCreateService drives CreateKnowledgeBase end-to-end with a
// service that returns a chosen error. Embedding the interface keeps
// any other method nil-panic'ing on purpose.
type stubKBCreateService struct {
	interfaces.KnowledgeBaseService
	createErr error
}

func (s *stubKBCreateService) CreateKnowledgeBase(_ context.Context, kb *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	kb.ID = "kb-new"
	kb.TenantID = 1
	return kb, nil
}

func newCreateKBRouter(svc interfaces.KnowledgeBaseService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-test")
		c.Next()
	})
	h := &KnowledgeBaseHandler{service: svc}
	r.POST("/knowledge-bases", h.CreateKnowledgeBase)
	return r
}

func TestCreateKB_PreservesTypedErrorCode_2200(t *testing.T) {
	svc := &stubKBCreateService{
		createErr: apperrors.NewVectorStoreBindingInvalidError("vector store not found"),
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge-bases",
		strings.NewReader(`{"name":"kb"}`))
	req.Header.Set("Content-Type", "application/json")
	newCreateKBRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"code":2200`) {
		t.Fatalf("expected envelope to contain code 2200, got %s", body)
	}
	if strings.Contains(body, `"code":1007`) || strings.Contains(body, `"code":1000`) {
		t.Fatalf("typed error must not be wrapped into a generic code, got %s", body)
	}
}

func TestCreateKB_PreservesTypedErrorCode_2201(t *testing.T) {
	svc := &stubKBCreateService{
		createErr: apperrors.NewVectorStoreUnavailableError(""),
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge-bases",
		strings.NewReader(`{"name":"kb"}`))
	req.Header.Set("Content-Type", "application/json")
	newCreateKBRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":2201`) {
		t.Fatalf("expected envelope to contain code 2201, got %s", w.Body.String())
	}
}

func TestCreateKB_GenericErrorStillFallsThroughTo500(t *testing.T) {
	// A non-AppError must NOT be auto-rewritten to 200/400 — operational
	// monitoring still needs to see infrastructure failures as 5xx.
	svc := &stubKBCreateService{createErr: errSentinel("connection refused")}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge-bases",
		strings.NewReader(`{"name":"kb"}`))
	req.Header.Set("Content-Type", "application/json")
	newCreateKBRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for raw infra error, got %d body=%s", w.Code, w.Body.String())
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// buildKBResponse carries the store metadata alongside the row
// ---------------------------------------------------------------------------

func TestBuildKBResponse_KeepsVectorStoreIDForOwnerKB(t *testing.T) {
	// The owner sees the UUID alongside the resolved metadata.
	storeID := "aaaa-bbbb-cccc-dddd"
	kb := &types.KnowledgeBase{
		ID:               "kb-1",
		Name:             "owner-kb",
		TenantID:         1,
		EmbeddingModelID: "e",
		SummaryModelID:   "s",
		VectorStoreID:    &storeID,
	}
	view := types.StoreDisplay{
		Name:       "prod-es",
		Source:     types.StoreSourceUser,
		EngineType: "elasticsearch",
		Status:     "available",
	}
	got := buildKBResponse(kb, view, kbStorageView{}, nil)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", got)
	}
	if m["vector_store_id"] != storeID {
		t.Fatalf("owner KB must keep vector_store_id, got %v", m["vector_store_id"])
	}
	if m["vector_store_name"] != "prod-es" {
		t.Fatalf("owner KB must surface store name, got %v", m["vector_store_name"])
	}
}

// stubStorageBackendRepo serves a fixed set of visible backends.
type stubStorageBackendRepo struct {
	interfaces.StorageBackendRepository
	backends []*types.StorageBackend
}

func (r *stubStorageBackendRepo) GetByID(_ context.Context, _ uint64, id string) (*types.StorageBackend, error) {
	for _, b := range r.backends {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, nil
}

func (r *stubStorageBackendRepo) List(context.Context, uint64) ([]*types.StorageBackend, error) {
	return r.backends, nil
}

// The owner sees which backend its knowledge base writes to — name,
// provider and kind, never the location — in both the single and the list
// response.
func TestKBResponseNamesTheStorageBackend(t *testing.T) {
	h := &KnowledgeBaseHandler{storageBackends: &stubStorageBackendRepo{backends: []*types.StorageBackend{{
		ID: "env", Name: "Deployment storage", Provider: "s3", Source: types.StorageBackendSourceEnv, IsBuiltin: true,
		Config: types.StorageBackendConfig{BucketName: "secret-bucket"},
	}}}}
	kb := &types.KnowledgeBase{ID: "kb-1", Name: "kb", TenantID: 1, StorageBackendID: "env"}

	single := buildKBResponse(kb, types.DefaultStoreDisplay(), h.kbStorageBackend(context.Background(), kb, 1), nil)
	list := h.buildKBListResponse(context.Background(), []*types.KnowledgeBase{kb}, 1)
	for name, got := range map[string]interface{}{"single": single, "list": list[0]} {
		serialized, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			StorageBackendID string                   `json:"storage_backend_id"`
			StorageBackend   *types.StorageBackendRef `json:"storage_backend"`
		}
		if err := json.Unmarshal(serialized, &body); err != nil {
			t.Fatal(err)
		}
		want := types.StorageBackendRef{
			ID: "env", Name: "Deployment storage", Provider: "s3", Source: "env", IsBuiltin: true,
		}
		if body.StorageBackend == nil || *body.StorageBackend != want {
			t.Fatalf("%s: storage_backend = %+v, want %+v", name, body.StorageBackend, want)
		}
		if body.StorageBackendID != "env" {
			t.Fatalf("%s: storage_backend_id = %q", name, body.StorageBackendID)
		}
		if strings.Contains(string(serialized), "secret-bucket") {
			t.Fatalf("%s: the backend's location leaked: %s", name, serialized)
		}
	}
}
