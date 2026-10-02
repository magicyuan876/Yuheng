package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// stubGroupService records the calls the handler makes and answers with
// what the test put in. Methods not set nil-panic if reached, which is the
// right outcome for "the handler should not have gotten here".
type stubGroupService struct {
	interfaces.TenantGroupService
	list        func(tenantID uint64) ([]*types.TenantGroupView, error)
	create      func(tenantID uint64, in types.CreateTenantGroupInput) (*types.TenantGroupView, error)
	get         func(tenantID uint64, id string) (*types.TenantGroupView, error)
	del         func(tenantID uint64, id string) error
	listMembers func(tenantID uint64, id, query string, page, pageSize int) (*types.TenantGroupMemberPage, error)
	addMembers  func(tenantID uint64, id string, userIDs []string) error
}

func (s *stubGroupService) List(_ context.Context, tenantID uint64) ([]*types.TenantGroupView, error) {
	return s.list(tenantID)
}

func (s *stubGroupService) Create(_ context.Context, tenantID uint64,
	in types.CreateTenantGroupInput,
) (*types.TenantGroupView, error) {
	return s.create(tenantID, in)
}

func (s *stubGroupService) Get(_ context.Context, tenantID uint64, id string) (*types.TenantGroupView, error) {
	return s.get(tenantID, id)
}

func (s *stubGroupService) Delete(_ context.Context, tenantID uint64, id string) error {
	return s.del(tenantID, id)
}

func (s *stubGroupService) ListMembers(_ context.Context, tenantID uint64, id, query string, page,
	pageSize int,
) (*types.TenantGroupMemberPage, error) {
	return s.listMembers(tenantID, id, query, page, pageSize)
}

func (s *stubGroupService) AddMembers(_ context.Context, tenantID uint64, id string, userIDs []string) error {
	return s.addMembers(tenantID, id, userIDs)
}

// groupTestRouter mounts the handler the way router.RegisterGroupRoutes
// does, minus the role guards, with the production error middleware's
// behaviour (errorCapture) so c.Error becomes a status code.
func groupTestRouter(svc interfaces.TenantGroupService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewTenantGroupHandler(svc)
	r := gin.New()
	r.Use(errorCapture())
	groups := r.Group("/groups")
	groups.GET("", h.List)
	groups.POST("", h.Create)
	groups.GET("/:gid", h.Get)
	groups.PATCH("/:gid", h.Update)
	groups.DELETE("/:gid", h.Delete)
	groups.GET("/:gid/members", h.ListMembers)
	groups.PUT("/:gid/members", h.AddMembers)
	groups.DELETE("/:gid/members/:uid", h.RemoveMember)
	return r
}

// doGroup sends a request as a member of tenant 7; tenantID 0 leaves the
// context without a workspace, as a request that slipped past the auth
// middleware would be.
func doGroup(t *testing.T, r http.Handler, method, path string, body any, tenantID uint64) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.UserIDContextKey, "admin")
	if tenantID != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestTenantGroupHandlerUsesTheActiveWorkspaceAndTheStandardEnvelope(t *testing.T) {
	var gotTenant uint64
	var gotInput types.CreateTenantGroupInput
	svc := &stubGroupService{
		list: func(tenantID uint64) ([]*types.TenantGroupView, error) {
			gotTenant = tenantID
			return []*types.TenantGroupView{
				{
					TenantGroup: &types.TenantGroup{ID: "g1", TenantID: tenantID, Name: "everyone", IsDefault: true},
					MemberCount: 3,
				},
			}, nil
		},
		create: func(tenantID uint64, in types.CreateTenantGroupInput) (*types.TenantGroupView, error) {
			gotTenant, gotInput = tenantID, in
			return &types.TenantGroupView{
				TenantGroup: &types.TenantGroup{ID: "g2", TenantID: tenantID, Name: in.Name},
			}, nil
		},
	}
	r := groupTestRouter(svc)

	rec := doGroup(t, r, http.MethodGet, "/groups", nil, 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	if gotTenant != 7 {
		t.Fatalf("list asked for tenant %d, want the active workspace 7", gotTenant)
	}
	env := decodeEnvelope(t, rec)
	if env["success"] != true {
		t.Fatalf("envelope = %v, want success:true", env)
	}
	rows, _ := env["data"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["member_count"] != float64(3) {
		t.Fatalf("data = %v, want one group with member_count 3", env["data"])
	}

	rec = doGroup(t, r, http.MethodPost, "/groups", map[string]any{
		"name": "Backend", "description": "svc", "member_ids": []string{"u1", "u2"},
	}, 7)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	if gotInput.Name != "Backend" || gotInput.Description != "svc" || len(gotInput.MemberIDs) != 2 {
		t.Fatalf("create input = %+v", gotInput)
	}

	rec = doGroup(t, r, http.MethodPost, "/groups", map[string]any{"description": "no name"}, 7)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create without a name = %d, want 400 from binding", rec.Code)
	}
}

func TestTenantGroupHandlerRefusesARequestWithoutAWorkspace(t *testing.T) {
	r := groupTestRouter(&stubGroupService{})
	rec := doGroup(t, r, http.MethodGet, "/groups", nil, 0)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no workspace = %d, want 401", rec.Code)
	}
}

func TestTenantGroupHandlerMapsServiceErrors(t *testing.T) {
	svc := &stubGroupService{
		get: func(_ uint64, id string) (*types.TenantGroupView, error) {
			switch id {
			case "missing":
				return nil, apperrors.NewNotFoundError("group not found")
			case "broken":
				return nil, errors.New("connection reset")
			}
			return &types.TenantGroupView{TenantGroup: &types.TenantGroup{ID: id}}, nil
		},
		del: func(uint64, string) error { return nil },
		addMembers: func(_ uint64, _ string, userIDs []string) error {
			if len(userIDs) == 0 {
				return apperrors.NewValidationError("user_ids is required")
			}
			return nil
		},
		listMembers: func(_ uint64, _, query string, page, pageSize int) (*types.TenantGroupMemberPage, error) {
			if query != "bo" || page != 2 || pageSize != 5 {
				t.Fatalf("paging reached the service as q=%q page=%d size=%d", query, page, pageSize)
			}
			return &types.TenantGroupMemberPage{
				Members: []types.TenantGroupMemberView{}, Page: page, PageSize: pageSize,
			}, nil
		},
	}
	r := groupTestRouter(svc)

	if rec := doGroup(t, r, http.MethodGet, "/groups/missing", nil, 7); rec.Code != http.StatusNotFound {
		t.Fatalf("missing group = %d, want 404", rec.Code)
	}
	rec := doGroup(t, r, http.MethodGet, "/groups/broken", nil, 7)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("database error = %d, want 500", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("connection reset")) {
		t.Fatalf("the error text leaked to the client: %s", rec.Body.String())
	}
	if rec := doGroup(t, r, http.MethodDelete, "/groups/g1", nil, 7); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", rec.Code)
	}
	rec = doGroup(t, r, http.MethodPut, "/groups/g1/members", map[string]any{"user_ids": []string{}}, 7)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty user_ids = %d, want 400", rec.Code)
	}
	// A successful add answers with the group as it now stands.
	rec = doGroup(t, r, http.MethodPut, "/groups/g1/members", map[string]any{"user_ids": []string{"u1"}}, 7)
	if rec.Code != http.StatusOK || decodeEnvelope(t, rec)["data"].(map[string]any)["id"] != "g1" {
		t.Fatalf("add members = %d %s, want 200 with the group", rec.Code, rec.Body.String())
	}
	rec = doGroup(t, r, http.MethodGet, "/groups/g1/members?q=bo&page=2&page_size=5", nil, 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("list members = %d %s", rec.Code, rec.Body.String())
	}
}
