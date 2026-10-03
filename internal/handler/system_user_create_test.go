package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/service"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// createUserService records the request handed to AdminCreateUser so
// tests can pin byte-for-byte password pass-through.
type createUserService struct {
	interfaces.UserService
	createdUser *types.User
	generated   string
	err         error
	gotReq      *types.AdminCreateUserRequest
	remembered  []uint64
}

func (s *createUserService) AdminCreateUser(
	_ context.Context,
	req *types.AdminCreateUserRequest,
) (*types.User, string, error) {
	record := *req
	s.gotReq = &record
	return s.createdUser, s.generated, s.err
}

func createSystemUserRouter(h *SystemHandler, actorID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, actorID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/system/admin/users/create", h.CreateSystemUser)
	return r
}

func performCreateSystemUser(t *testing.T, r *gin.Engine, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateSystemUserCreatesUserWithExplicitPassword(t *testing.T) {
	users := &createUserService{createdUser: &types.User{
		ID: "u1", Username: "alice", Email: "alice@example.com",
	}}
	audits := &capturingAuditService{}
	h := &SystemHandler{userSvc: users, auditSvc: audits}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "alice@example.com", "password": "PlainPass9",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp CreateSystemUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.User == nil || resp.User.Username != "alice" || resp.User.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", resp.User)
	}
	if resp.GeneratedPassword != "" {
		t.Fatalf(
			"generated_password must be absent when the caller supplied a password, got %q",
			resp.GeneratedPassword,
		)
	}
	if users.gotReq == nil || users.gotReq.Password == nil || *users.gotReq.Password != "PlainPass9" {
		t.Fatalf("service received unexpected request: %+v", users.gotReq)
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemUserCreated {
		t.Fatalf("expected one %s audit entry, got %+v", types.AuditActionSystemUserCreated, audits.entries)
	}
	if strings.Contains(string(audits.entries[0].Details), "PlainPass9") {
		t.Fatal("audit details leaked the password")
	}
	if !strings.Contains(string(audits.entries[0].Details), `"password_generated":false`) {
		t.Fatalf("audit details must mark password_generated=false, got %s", audits.entries[0].Details)
	}
	if !strings.Contains(string(audits.entries[0].Details), `"idempotent":false`) {
		t.Fatalf("audit details must mark idempotent=false, got %s", audits.entries[0].Details)
	}
}

func TestCreateSystemUserAutoGeneratesPasswordWhenEmpty(t *testing.T) {
	users := &createUserService{
		createdUser: &types.User{ID: "u2", Username: "bob", Email: "bob@example.com"},
		generated:   "G3n3r4t3dP4ssw0rd",
	}
	audits := &capturingAuditService{}
	h := &SystemHandler{userSvc: users, auditSvc: audits}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "bob", "email": "bob@example.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp CreateSystemUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.User == nil || resp.User.Username != "bob" {
		t.Fatalf("unexpected user: %+v", resp.User)
	}
	if resp.GeneratedPassword != "G3n3r4t3dP4ssw0rd" {
		t.Fatalf("generated_password=%q, want the once-only generated password", resp.GeneratedPassword)
	}
	if users.gotReq == nil || users.gotReq.Password != nil {
		t.Fatalf("absent password must reach the service as nil, got %+v", users.gotReq)
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemUserCreated {
		t.Fatalf("expected one %s audit entry, got %+v", types.AuditActionSystemUserCreated, audits.entries)
	}
	if !strings.Contains(string(audits.entries[0].Details), "password_generated") {
		t.Fatal("audit details must carry the password_generated flag")
	}
	if strings.Contains(string(audits.entries[0].Details), "G3n3r4t3dP4ssw0rd") {
		t.Fatal("audit details leaked the generated password")
	}
	if !strings.Contains(string(audits.entries[0].Details), `"password_generated":true`) {
		t.Fatalf("audit details must mark password_generated=true, got %s", audits.entries[0].Details)
	}
	if !strings.Contains(string(audits.entries[0].Details), `"idempotent":false`) {
		t.Fatalf("audit details must mark idempotent=false, got %s", audits.entries[0].Details)
	}
}

func TestCreateSystemUserDoesNotRewritePassword(t *testing.T) {
	// Password bytes must reach the service unmodified for valid credentials.
	users := &createUserService{createdUser: &types.User{ID: "u3", Username: "carol", Email: "carol@example.com"}}
	h := &SystemHandler{userSvc: users}
	r := createSystemUserRouter(h, "admin-user")

	for _, pw := range []string{"  PlainPass9  ", "\tPlainPass9\n"} {
		w := performCreateSystemUser(t, r, map[string]string{
			"username": "carol", "email": "carol@example.com", "password": pw,
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("password=%q status=%d body=%s", pw, w.Code, w.Body.String())
		}
		if users.gotReq == nil || users.gotReq.Password == nil || *users.gotReq.Password != pw {
			t.Fatalf("password=%q service received %+v", pw, users.gotReq)
		}
	}
}

func TestCreateSystemUserMapsEmptyPasswordTo400(t *testing.T) {
	users := &createUserService{err: service.ErrPasswordPolicy}
	h := &SystemHandler{userSvc: users}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "carol", "email": "carol@example.com", "password": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateSystemUserMapsIdentityConflictTo409(t *testing.T) {
	users := &createUserService{err: service.ErrUserIdentityConflict}
	audits := &capturingAuditService{}
	h := &SystemHandler{userSvc: users, auditSvc: audits}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "bob@example.com",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(audits.entries) != 0 {
		t.Fatalf("conflict emitted audit entries: %+v", audits.entries)
	}
}

func TestCreateSystemUserMapsPasswordPolicyTo400(t *testing.T) {
	users := &createUserService{err: service.ErrPasswordPolicy}
	h := &SystemHandler{userSvc: users}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "alice@example.com", "password": "password",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateSystemUserDuplicateIdentityReturnsExistingUser(t *testing.T) {
	// Idempotent contract: when the identity already exists the service
	// returns the existing user with ErrUserEmailExists/ErrUserUsernameExists,
	// and the handler answers 200 with the existing UserInfo, an empty
	// generated_password and an audit row marked idempotent.
	users := &createUserService{
		createdUser: &types.User{ID: "existing", Username: "alice", Email: "alice@example.com"},
		err:         service.ErrUserUsernameExists,
	}
	audits := &capturingAuditService{}
	h := &SystemHandler{userSvc: users, auditSvc: audits}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "alice@example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp CreateSystemUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.User == nil || resp.User.ID != "existing" {
		t.Fatalf("unexpected user: %+v", resp.User)
	}
	if resp.GeneratedPassword != "" {
		t.Fatalf("generated_password=%q, want empty for an existing user", resp.GeneratedPassword)
	}
	if len(audits.entries) != 1 || audits.entries[0].Action != types.AuditActionSystemUserCreated {
		t.Fatalf("expected one %s audit entry, got %+v", types.AuditActionSystemUserCreated, audits.entries)
	}
	if !strings.Contains(string(audits.entries[0].Details), `"idempotent":true`) {
		t.Fatalf("audit details must mark idempotent=true, got %s", audits.entries[0].Details)
	}
	if !strings.Contains(string(audits.entries[0].Details), `"password_generated":false`) {
		t.Fatalf("audit details must mark password_generated=false, got %s", audits.entries[0].Details)
	}
}

func TestCreateSystemUserMapsInternalErrorTo500(t *testing.T) {
	users := &createUserService{err: errors.New("transient db hiccup")}
	audits := &capturingAuditService{}
	h := &SystemHandler{userSvc: users, auditSvc: audits}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "alice@example.com",
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(audits.entries) != 0 {
		t.Fatalf("failed creation emitted audit entries: %+v", audits.entries)
	}
}

func TestCreateSystemUserRejectsMissingFields(t *testing.T) {
	h := &SystemHandler{}
	r := createSystemUserRouter(h, "admin-user")

	cases := []map[string]string{
		{"email": "alice@example.com"},
		{"username": "alice"},
		{"username": "", "email": "alice@example.com"},
		{"username": "alice", "email": ""},
		{"username": "   ", "email": "alice@example.com"},
		// Binding's min=2 ran on the raw JSON; the trimmed value "a" (1
		// rune) must be rejected by the post-trim re-check.
		{"username": "  a  ", "email": "alice@example.com"},
	}
	for _, body := range cases {
		w := performCreateSystemUser(t, r, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%v status=%d body=%s", body, w.Code, w.Body.String())
		}
	}
}

func TestCreateSystemUserRejectsInvalidEmail(t *testing.T) {
	h := &SystemHandler{}
	r := createSystemUserRouter(h, "admin-user")

	w := performCreateSystemUser(t, r, map[string]string{
		"username": "alice", "email": "not-an-email",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateSystemUserRejectsMalformedJSON(t *testing.T) {
	h := &SystemHandler{}
	r := createSystemUserRouter(h, "admin-user")

	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create",
		bytes.NewReader([]byte(`{"username": "alice"`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// createUserWorkspaceFixture adds the workspace half of the create-user
// request: a tenant catalog with one workspace and a member service that
// records what was written.
type createUserTenants struct {
	interfaces.TenantService
	known map[uint64]*types.Tenant
}

func (s *createUserTenants) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	if t, ok := s.known[id]; ok {
		return t, nil
	}
	return nil, repository.ErrTenantNotFound
}

type createUserMembers struct {
	interfaces.TenantMemberService
	added     []*types.TenantMember
	addErr    error
	invitedBy *string
}

func (s *createUserMembers) AddMember(
	_ context.Context, userID string, tenantID uint64, role types.TenantRole, invitedBy *string,
) (*types.TenantMember, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.invitedBy = invitedBy
	m := &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive}
	s.added = append(s.added, m)
	return m, nil
}

func (s *createUserMembers) GetMembership(
	_ context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	for _, m := range s.added {
		if m.UserID == userID && m.TenantID == tenantID {
			return m, nil
		}
	}
	return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: types.TenantRoleAdmin}, nil
}

func (s *createUserService) RememberFirstWorkspace(_ context.Context, _ string, tenantID uint64) error {
	s.remembered = append(s.remembered, tenantID)
	return nil
}

func TestCreateSystemUserPlacesTheAccountInTheRequestedWorkspace(t *testing.T) {
	users := &createUserService{createdUser: &types.User{ID: "u9", Username: "dana", Email: "dana@example.com"}}
	members := &createUserMembers{}
	audits := &capturingAuditService{}
	h := &SystemHandler{
		userSvc:   users,
		tenantSvc: &createUserTenants{known: map[uint64]*types.Tenant{5: {ID: 5, Name: "Sales"}}},
		memberSvc: members,
		auditSvc:  audits,
	}
	r := createSystemUserRouter(h, "admin-user")

	payload, _ := json.Marshal(map[string]any{
		"username": "dana", "email": "dana@example.com", "tenant_id": 5, "role": "contributor",
	})
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(members.added) != 1 || members.added[0].TenantID != 5 ||
		members.added[0].Role != types.TenantRoleContributor {
		t.Fatalf("membership written = %+v, want contributor of workspace 5", members.added)
	}
	if members.invitedBy == nil || *members.invitedBy != "admin-user" {
		t.Fatalf("invited_by = %v, want the administrator", members.invitedBy)
	}
	if len(users.remembered) != 1 || users.remembered[0] != 5 {
		t.Fatalf("the new account must have the workspace remembered as its active one, got %v", users.remembered)
	}
	var resp CreateSystemUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Membership == nil || resp.Membership.Role != types.TenantRoleContributor ||
		resp.Membership.Email != "dana@example.com" {
		t.Fatalf("response membership = %+v", resp.Membership)
	}
	if len(audits.entries) != 1 || !strings.Contains(string(audits.entries[0].Details), `"tenant_id":5`) {
		t.Fatalf("audit details must record the workspace: %+v", audits.entries)
	}
}

func TestCreateSystemUserDefaultsTheWorkspaceRoleToViewer(t *testing.T) {
	users := &createUserService{createdUser: &types.User{ID: "u9", Username: "dana", Email: "dana@example.com"}}
	members := &createUserMembers{}
	h := &SystemHandler{
		userSvc:   users,
		tenantSvc: &createUserTenants{known: map[uint64]*types.Tenant{5: {ID: 5}}},
		memberSvc: members,
	}
	r := createSystemUserRouter(h, "admin-user")

	payload, _ := json.Marshal(map[string]any{"username": "dana", "email": "dana@example.com", "tenant_id": 5})
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(members.added) != 1 || members.added[0].Role != types.TenantRoleViewer {
		t.Fatalf("membership = %+v, want viewer", members.added)
	}
}

func TestCreateSystemUserRejectsAnUnknownWorkspaceBeforeCreatingTheAccount(t *testing.T) {
	users := &createUserService{createdUser: &types.User{ID: "u9", Username: "dana", Email: "dana@example.com"}}
	h := &SystemHandler{
		userSvc:   users,
		tenantSvc: &createUserTenants{known: map[uint64]*types.Tenant{}},
		memberSvc: &createUserMembers{},
	}
	r := createSystemUserRouter(h, "admin-user")

	payload, _ := json.Marshal(map[string]any{"username": "dana", "email": "dana@example.com", "tenant_id": 404})
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if users.gotReq != nil {
		t.Fatal("a typo in tenant_id must not leave an account that belongs nowhere")
	}
}

func TestCreateSystemUserRejectsAnInvalidWorkspaceRole(t *testing.T) {
	users := &createUserService{createdUser: &types.User{ID: "u9"}}
	h := &SystemHandler{
		userSvc:   users,
		tenantSvc: &createUserTenants{known: map[uint64]*types.Tenant{5: {ID: 5}}},
		memberSvc: &createUserMembers{},
	}
	r := createSystemUserRouter(h, "admin-user")

	payload, _ := json.Marshal(map[string]any{
		"username": "dana", "email": "dana@example.com", "tenant_id": 5, "role": "king",
	})
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if users.gotReq != nil {
		t.Fatal("an invalid role must be refused before the account is created")
	}
}

func TestCreateSystemUserRetryAfterMembershipFailureOnlyAddsTheMember(t *testing.T) {
	// First attempt: the account is created but the membership write fails.
	existing := &types.User{ID: "u9", Username: "dana", Email: "dana@example.com"}
	users := &createUserService{createdUser: existing}
	members := &createUserMembers{addErr: errors.New("tenant_members is down")}
	h := &SystemHandler{
		userSvc:   users,
		tenantSvc: &createUserTenants{known: map[uint64]*types.Tenant{5: {ID: 5}}},
		memberSvc: members,
	}
	r := createSystemUserRouter(h, "admin-user")
	send := func() *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]any{"username": "dana", "email": "dana@example.com", "tenant_id": 5})
		req := httptest.NewRequest(http.MethodPost, "/system/admin/users/create", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := send(); w.Code != http.StatusInternalServerError {
		t.Fatalf("first attempt status=%d body=%s", w.Code, w.Body.String())
	}

	// Retry: the identity now exists (idempotent path) and the membership
	// write succeeds, so the request completes with 200 and the membership.
	users.err = service.ErrUserEmailExists
	members.addErr = nil
	w := send()
	if w.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", w.Code, w.Body.String())
	}
	var resp CreateSystemUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Membership == nil || resp.Membership.Role != types.TenantRoleViewer {
		t.Fatalf("retry must report the membership it wrote, got %+v", resp.Membership)
	}
	if len(members.added) != 1 {
		t.Fatalf("memberships written = %d, want exactly one", len(members.added))
	}
}
