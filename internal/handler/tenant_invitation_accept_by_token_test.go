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
	"github.com/magicyuan876/yuheng/internal/application/service"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// acceptByTokenInvitationSvc only implements AcceptByToken; the embedded
// interface panics on any other call so the test stays focused.
type acceptByTokenInvitationSvc struct {
	interfaces.TenantInvitationService
	member    *types.TenantMember
	acceptErr error
}

func (s *acceptByTokenInvitationSvc) AcceptByToken(_ context.Context, _ string, userID string) (*types.TenantMember, error) {
	if s.acceptErr != nil {
		return nil, s.acceptErr
	}
	if s.member != nil {
		return s.member, nil
	}
	return &types.TenantMember{TenantID: 42, Role: types.TenantRoleViewer, Status: types.TenantMemberStatusActive}, nil
}

// acceptByTokenUserSvc records which workspace the handler asked to
// remember as the caller's active one; whether that write happens is the
// user service's decision, so the handler test only checks the request.
type acceptByTokenUserSvc struct {
	interfaces.UserService
	remembered []uint64
}

func (s *acceptByTokenUserSvc) RememberFirstWorkspace(_ context.Context, _ string, tenantID uint64) error {
	s.remembered = append(s.remembered, tenantID)
	return nil
}

type acceptByTokenTenantSvc struct {
	interfaces.TenantService
}

func (s *acceptByTokenTenantSvc) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id, Name: "Invited Workspace"}, nil
}

func newAcceptByTokenTestRouter(h *TenantInvitationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Production auth middleware injects the caller via the request
		// context (context.WithValue), which is what UserIDFromContext
		// reads — NOT gin's c.Keys. Mirror that here.
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-test"))
		c.Next()
	}, errorCapture())
	r.POST("/me/invitations/accept-by-token", h.AcceptMyInvitationByToken)
	return r
}

func TestAcceptMyInvitationByTokenSuccess(t *testing.T) {
	users := &acceptByTokenUserSvc{}
	h := &TenantInvitationHandler{
		invitationService: &acceptByTokenInvitationSvc{},
		userService:       users,
		tenantService:     &acceptByTokenTenantSvc{},
	}
	r := newAcceptByTokenTestRouter(h)

	body := []byte(`{"token":"invite-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(users.remembered) != 1 || users.remembered[0] != 42 {
		t.Fatalf("remembered workspaces = %v, want [42] so a first workspace becomes the active one",
			users.remembered)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Membership struct {
				TenantID uint64 `json:"tenant_id"`
				Role     string `json:"role"`
			} `json:"membership"`
			TenantName string `json:"tenant_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if !resp.Success || resp.Data.Membership.TenantID != 42 || resp.Data.TenantName != "Invited Workspace" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAcceptMyInvitationByTokenInvalidTokenIsGone(t *testing.T) {
	h := &TenantInvitationHandler{
		invitationService: &acceptByTokenInvitationSvc{acceptErr: service.ErrInvitationTokenInvalid},
		userService:       &acceptByTokenUserSvc{},
		tenantService:     &acceptByTokenTenantSvc{},
	}
	r := newAcceptByTokenTestRouter(h)

	body := []byte(`{"token":"stale-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s, want 410 for invalid token", w.Code, w.Body.String())
	}
}

func TestAcceptMyInvitationByTokenMissingTokenIsBadRequest(t *testing.T) {
	h := &TenantInvitationHandler{
		invitationService: &acceptByTokenInvitationSvc{},
		userService:       &acceptByTokenUserSvc{},
		tenantService:     &acceptByTokenTenantSvc{},
	}
	r := newAcceptByTokenTestRouter(h)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 for missing token", w.Code, w.Body.String())
	}
}

// AcceptByToken is idempotent: an existing membership is returned
// untouched. The handler treats that member the same as a freshly
// created one (200 + membership), so a user clicking the same link
// twice from two devices never sees a role downgrade or an error.
func TestAcceptMyInvitationByTokenIdempotent(t *testing.T) {
	users := &acceptByTokenUserSvc{}
	h := &TenantInvitationHandler{
		invitationService: &acceptByTokenInvitationSvc{
			member: &types.TenantMember{TenantID: 42, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive},
		},
		userService:   users,
		tenantService: &acceptByTokenTenantSvc{},
	}
	r := newAcceptByTokenTestRouter(h)

	body := []byte(`{"token":"invite-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Membership struct {
				Role string `json:"role"`
			} `json:"membership"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Membership.Role != string(types.TenantRoleContributor) {
		t.Fatalf("role=%s, want contributor (existing membership preserved)", resp.Data.Membership.Role)
	}
	// Remembering is idempotent in the user service (it only fills an empty
	// preference), so the handler asks every time.
	if len(users.remembered) != 1 || users.remembered[0] != 42 {
		t.Fatalf("remembered workspaces = %v, want [42]", users.remembered)
	}
}

func TestAcceptMyInvitationByTokenUnexpectedErrorIs500(t *testing.T) {
	h := &TenantInvitationHandler{
		invitationService: &acceptByTokenInvitationSvc{acceptErr: errors.New("boom")},
		userService:       &acceptByTokenUserSvc{},
		tenantService:     &acceptByTokenTenantSvc{},
	}
	r := newAcceptByTokenTestRouter(h)

	body := []byte(`{"token":"invite-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500 for unexpected error", w.Code, w.Body.String())
	}
}
