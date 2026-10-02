package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// invitedRegistrationUserService records what RegisterByInvite asks of the
// user service: the registration itself (which must carry no workspace
// instruction, registration never creates one) and the workspace it asks to
// remember as the new account's active one.
type invitedRegistrationUserService struct {
	interfaces.UserService
	registered  *types.RegisterRequest
	remembered  []uint64
	deleteCalls int
}

func (s *invitedRegistrationUserService) GetUserByEmail(context.Context, string) (*types.User, error) {
	return nil, nil
}

func (s *invitedRegistrationUserService) Register(_ context.Context, req *types.RegisterRequest) (*types.User, error) {
	cp := *req
	s.registered = &cp
	return &types.User{ID: "new-user", Username: req.Username, Email: req.Email, IsActive: true}, nil
}

func (s *invitedRegistrationUserService) RememberFirstWorkspace(_ context.Context, _ string, tenantID uint64) error {
	s.remembered = append(s.remembered, tenantID)
	return nil
}

func (s *invitedRegistrationUserService) DeleteUser(context.Context, string) error {
	s.deleteCalls++
	return nil
}

func (s *invitedRegistrationUserService) GenerateTokens(context.Context, *types.User) (string, string, error) {
	return "access", "refresh", nil
}

type invitedRegistrationInvitationService struct {
	interfaces.TenantInvitationService
	acceptErr error
}

func (s *invitedRegistrationInvitationService) LookupByToken(context.Context, string) (*types.TenantInvitation, error) {
	return &types.TenantInvitation{TenantID: 42, Role: types.TenantRoleViewer}, nil
}

func (s *invitedRegistrationInvitationService) AcceptByToken(context.Context, string, string) (*types.TenantMember, error) {
	if s.acceptErr != nil {
		return nil, s.acceptErr
	}
	return &types.TenantMember{TenantID: 42, Role: types.TenantRoleViewer}, nil
}

type invitedRegistrationTenantService struct {
	interfaces.TenantService
}

func (s *invitedRegistrationTenantService) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: 42, Name: "Invited Workspace"}, nil
}

func postRegisterByInvite(t *testing.T, h *AuthHandler) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/auth/register-by-invite", h.RegisterByInvite)

	body := []byte(`{"token":"invite-token","email":"alice@example.com","username":"alice","password":"supersecret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register-by-invite", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// An invitation that is revoked between lookup and acceptance leaves a valid
// account that simply belongs to no workspace; nothing is rolled back and no
// workspace is remembered for it.
func TestRegisterByInviteKeepsTheAccountWhenInviteExpiresDuringRegistration(t *testing.T) {
	users := &invitedRegistrationUserService{}
	h := &AuthHandler{
		userService:   users,
		tenantService: &invitedRegistrationTenantService{},
		invitationSvc: &invitedRegistrationInvitationService{acceptErr: errors.New("expired")},
	}
	w := postRegisterByInvite(t, h)

	if w.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if users.registered == nil {
		t.Fatal("the account must still be created")
	}
	if len(users.remembered) != 0 || users.deleteCalls != 0 {
		t.Fatalf("remembered=%v deletes=%d; want neither a remembered workspace nor a rollback",
			users.remembered, users.deleteCalls)
	}
}

func TestRegisterByInviteRegistersPlainlyAndRemembersTheInvitedWorkspace(t *testing.T) {
	users := &invitedRegistrationUserService{}
	h := &AuthHandler{
		userService:   users,
		tenantService: &invitedRegistrationTenantService{},
		invitationSvc: &invitedRegistrationInvitationService{},
	}
	w := postRegisterByInvite(t, h)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if users.registered == nil || users.registered.BootstrapFirstUser || users.registered.WorkspaceName != "" {
		t.Fatalf("register request = %+v; an invitee registers an account and nothing else", users.registered)
	}
	if len(users.remembered) != 1 || users.remembered[0] != 42 {
		t.Fatalf("remembered workspaces = %v, want [42]", users.remembered)
	}
}
