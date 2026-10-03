package handler

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/config"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// These tests pin the handler-level contract of workspace lifecycle under
// the "system administrators manage workspaces" model: every workspace is
// created with an Owner, a platform key must name one, and a workspace is
// deleted only when nobody else is in it and it is not the last one. The
// route guard (who may call at all) is tested in middleware/access_catalog_test.go
// and through the real router in container/e2e_admin_test.go.

// adminTenantErrors renders c.Error as the {code,message} envelope the
// production ErrorHandler emits, so tests can assert on the typed codes.
func adminTenantErrors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		if appErr, ok := c.Errors.Last().Err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": c.Errors.Last().Error()})
	}
}

// adminTenantUsers resolves the caller and a small address book.
type adminTenantUsers struct {
	interfaces.UserService
	caller     *types.User
	byEmail    map[string]*types.User
	remembered []uint64
}

func (s *adminTenantUsers) GetCurrentUser(context.Context) (*types.User, error) { return s.caller, nil }

func (s *adminTenantUsers) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	if u, ok := s.byEmail[email]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (s *adminTenantUsers) RememberFirstWorkspace(_ context.Context, _ string, tenantID uint64) error {
	s.remembered = append(s.remembered, tenantID)
	return nil
}

func (s *adminTenantUsers) BuildLoginMemberships(context.Context, *types.User, *types.Tenant) []types.Membership {
	return []types.Membership{}
}

// adminTenantStore records what the handler asked the tenant service to do.
type adminTenantStore struct {
	interfaces.TenantService
	created *types.Tenant
	deleted []uint64
	total   int64
}

func (s *adminTenantStore) CreateTenant(_ context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	tenant.ID = 42
	s.created = tenant
	return tenant, nil
}

func (s *adminTenantStore) DeleteTenant(_ context.Context, id uint64) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *adminTenantStore) CountTenants(context.Context) (int64, error) { return s.total, nil }

// adminTenantMembers records Owner bootstraps and serves a member roster.
type adminTenantMembers struct {
	interfaces.TenantMemberService
	owners    []string
	ownerErr  error
	roster    []*types.TenantMember
	rosterErr error
}

func (s *adminTenantMembers) EnsureOwner(
	_ context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	if s.ownerErr != nil {
		return nil, s.ownerErr
	}
	s.owners = append(s.owners, userID)
	return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: types.TenantRoleOwner}, nil
}

func (s *adminTenantMembers) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return s.roster, s.rosterErr
}

type adminTenantSettings struct {
	interfaces.SystemSettingService
}

func (adminTenantSettings) GetInt(_ context.Context, _ string, _ string, def int64) int64 { return def }

type adminTenantFixture struct {
	h       *TenantHandler
	users   *adminTenantUsers
	tenants *adminTenantStore
	members *adminTenantMembers
	audit   *capturingAuditService
}

func newAdminTenantFixture(caller *types.User) *adminTenantFixture {
	f := &adminTenantFixture{
		users: &adminTenantUsers{
			caller: caller,
			byEmail: map[string]*types.User{
				"lead@example.com": {ID: "u-lead", Email: "lead@example.com"},
			},
		},
		tenants: &adminTenantStore{total: 2},
		members: &adminTenantMembers{},
		audit:   &capturingAuditService{},
	}
	f.h = &TenantHandler{
		service:          f.tenants,
		userService:      f.users,
		memberService:    f.members,
		config:           &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: adminTenantSettings{},
		auditSvc:         f.audit,
	}
	return f
}

// serve runs one request through a bare router with the given identity on
// the context, the way the auth middleware would have put it there.
func (f *adminTenantFixture) serve(
	t *testing.T, method, path string, body string, seed func(ctx context.Context) context.Context,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if seed != nil {
			c.Request = c.Request.WithContext(seed(c.Request.Context()))
		}
		c.Next()
	})
	r.Use(adminTenantErrors())
	r.POST("/tenants", f.h.CreateTenant)
	r.DELETE("/tenants/:id", f.h.DeleteTenant)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func asCaller(id string) func(ctx context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, types.UserIDContextKey, id)
	}
}

func asPlatformKey(ctx context.Context) context.Context {
	return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{ScopeType: types.APIKeyScopePlatform})
}

func TestCreateTenant_CallerIsTheDefaultOwner(t *testing.T) {
	admin := &types.User{ID: "u-admin", Email: "admin@example.com", IsSystemAdmin: true}
	f := newAdminTenantFixture(admin)

	w := f.serve(t, http.MethodPost, "/tenants", `{"name":"  Research  ","description":" shared "}`, asCaller(admin.ID))

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if f.tenants.created == nil || f.tenants.created.Name != "Research" || f.tenants.created.Description != "shared" {
		t.Fatalf("tenant not created from the trimmed request: %+v", f.tenants.created)
	}
	if f.tenants.created.StorageQuota != 0 {
		t.Fatalf("default quota = %d, want 0 (unlimited)", f.tenants.created.StorageQuota)
	}
	if len(f.members.owners) != 1 || f.members.owners[0] != admin.ID {
		t.Fatalf("owners bootstrapped = %v, want the caller", f.members.owners)
	}
	if len(f.users.remembered) != 1 || f.users.remembered[0] != 42 {
		t.Fatalf("remembered workspaces = %v, want [42]", f.users.remembered)
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != types.AuditActionSystemTenantCreated {
		t.Fatalf("audit = %+v, want one %s row", f.audit.entries, types.AuditActionSystemTenantCreated)
	}
	if f.audit.entries[0].TenantID != 0 || f.audit.entries[0].TargetID != "42" {
		t.Fatalf("audit row must be system-scope and name the workspace: %+v", f.audit.entries[0])
	}
	if !strings.Contains(string(f.audit.entries[0].Details), `"owner_email":"admin@example.com"`) {
		t.Fatalf("audit details must name the owner: %s", f.audit.entries[0].Details)
	}
}

func TestCreateTenant_OwnerEmailNamesAnotherUser(t *testing.T) {
	admin := &types.User{ID: "u-admin", IsSystemAdmin: true}
	f := newAdminTenantFixture(admin)

	w := f.serve(t, http.MethodPost, "/tenants",
		`{"name":"Sales","owner_email":"lead@example.com"}`, asCaller(admin.ID))

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(f.members.owners) != 1 || f.members.owners[0] != "u-lead" {
		t.Fatalf("owners bootstrapped = %v, want the named user, not the caller", f.members.owners)
	}
	if len(f.users.remembered) != 1 || f.users.remembered[0] != 42 {
		t.Fatalf("the named owner must have the workspace remembered, got %v", f.users.remembered)
	}
}

func TestCreateTenant_UnknownOwnerEmailIs404AndCreatesNothing(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-admin", IsSystemAdmin: true})

	w := f.serve(t, http.MethodPost, "/tenants",
		`{"name":"Sales","owner_email":"nobody@example.com"}`, asCaller("u-admin"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if f.tenants.created != nil {
		t.Fatal("an unknown owner must be rejected before the workspace is created")
	}
}

func TestCreateTenant_PlatformKeyMustNameAnOwner(t *testing.T) {
	// The API-key auth path attaches the synthetic system user as caller.
	f := newAdminTenantFixture(&types.User{ID: "system-1"})

	w := f.serve(t, http.MethodPost, "/tenants", `{"name":"Provisioned"}`, asPlatformKey)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":2006`) {
		t.Fatalf("response must carry the owner-required code: %s", w.Body.String())
	}
	if f.tenants.created != nil {
		t.Fatal("no workspace may exist without an owner")
	}

	w = f.serve(t, http.MethodPost, "/tenants",
		`{"name":"Provisioned","owner_email":"lead@example.com"}`, asPlatformKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("with an owner: status=%d body=%s", w.Code, w.Body.String())
	}
	if len(f.members.owners) != 1 || f.members.owners[0] != "u-lead" {
		t.Fatalf("owners = %v, want the named user", f.members.owners)
	}
}

func TestCreateTenant_RollsBackWhenTheOwnerCannotBeWritten(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-admin", IsSystemAdmin: true})
	f.members.ownerErr = stderrors.New("tenant_members is down")

	w := f.serve(t, http.MethodPost, "/tenants", `{"name":"Orphan"}`, asCaller("u-admin"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(f.tenants.deleted) != 1 || f.tenants.deleted[0] != 42 {
		t.Fatalf("the memberless workspace must be rolled back, deleted=%v", f.tenants.deleted)
	}
	if len(f.audit.entries) != 0 {
		t.Fatalf("a rolled-back create must not be audited as created: %+v", f.audit.entries)
	}
}

func TestCreateTenant_ExplicitQuotaIsKept(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-admin", IsSystemAdmin: true})

	w := f.serve(t, http.MethodPost, "/tenants", `{"name":"Capped","storage_quota":1024}`, asCaller("u-admin"))

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if f.tenants.created.StorageQuota != 1024 {
		t.Fatalf("quota = %d, want the requested 1024", f.tenants.created.StorageQuota)
	}
}

func TestDeleteTenant_RefusedWhileOthersAreMembers(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-owner"})
	f.members.roster = []*types.TenantMember{
		{UserID: "u-owner", Status: types.TenantMemberStatusActive},
		{UserID: "u-other", Status: types.TenantMemberStatusActive},
		{UserID: "u-gone", Status: types.TenantMemberStatusSuspended},
	}

	w := f.serve(t, http.MethodDelete, "/tenants/7", "", asCaller("u-owner"))

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":2007`) || !strings.Contains(w.Body.String(), "1 other member") {
		t.Fatalf("response must carry the has-members code and count only active others: %s", w.Body.String())
	}
	if len(f.tenants.deleted) != 0 {
		t.Fatal("nothing may be deleted while others are members")
	}
}

func TestDeleteTenant_RefusedForTheLastWorkspace(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-owner"})
	f.members.roster = []*types.TenantMember{{UserID: "u-owner", Status: types.TenantMemberStatusActive}}
	f.tenants.total = 1

	w := f.serve(t, http.MethodDelete, "/tenants/7", "", asCaller("u-owner"))

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":2008`) {
		t.Fatalf("response must carry the last-workspace code: %s", w.Body.String())
	}
	if len(f.tenants.deleted) != 0 {
		t.Fatal("the last workspace must survive")
	}
}

func TestDeleteTenant_DeletesAnOtherwiseEmptyWorkspace(t *testing.T) {
	f := newAdminTenantFixture(&types.User{ID: "u-owner"})
	f.members.roster = []*types.TenantMember{{UserID: "u-owner", Status: types.TenantMemberStatusActive}}

	w := f.serve(t, http.MethodDelete, "/tenants/7", "", asCaller("u-owner"))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(f.tenants.deleted) != 1 || f.tenants.deleted[0] != 7 {
		t.Fatalf("deleted=%v, want [7]", f.tenants.deleted)
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != types.AuditActionSystemTenantDeleted {
		t.Fatalf("audit = %+v, want one %s row", f.audit.entries, types.AuditActionSystemTenantDeleted)
	}
}

// The /auth/me capability must agree with the route guard: a system
// administrator sees the create button, an ordinary user does not, and
// the cross-tenant attribute counts only while the deployment flag is on.
func TestAuthMe_CanCreateTenantFollowsTheCatalogGuard(t *testing.T) {
	cases := []struct {
		name   string
		user   *types.User
		cfg    *config.TenantConfig
		seed   func(ctx context.Context) context.Context
		expect string
	}{
		{
			name: "system administrator",
			user: &types.User{ID: "a", IsSystemAdmin: true},
			cfg:  &config.TenantConfig{},
			seed: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, types.SystemAdminContextKey, true)
			},
			expect: `"can_create_tenant":true`,
		},
		{
			name:   "ordinary user with no workspace",
			user:   &types.User{ID: "u"},
			cfg:    &config.TenantConfig{},
			expect: `"can_create_tenant":false`,
		},
		{
			name:   "cross-tenant superuser, flag on",
			user:   &types.User{ID: "s", CanAccessAllTenants: true},
			cfg:    &config.TenantConfig{EnableCrossTenantAccess: true},
			expect: `"can_create_tenant":true`,
		},
		{
			name:   "cross-tenant attribute, flag off",
			user:   &types.User{ID: "s", CanAccessAllTenants: true},
			cfg:    &config.TenantConfig{},
			expect: `"can_create_tenant":false`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &AuthHandler{
				userService: &adminTenantUsers{caller: tc.user},
				configInfo:  &config.Config{Tenant: tc.cfg},
			}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				ctx := context.WithValue(c.Request.Context(), types.UserContextKey, tc.user)
				if tc.seed != nil {
					ctx = tc.seed(ctx)
				}
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			})
			r.GET("/auth/me", h.GetCurrentUser)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(w.Body.String(), tc.expect) {
				t.Fatalf("response = %s, want %s", w.Body.String(), tc.expect)
			}
		})
	}
}
