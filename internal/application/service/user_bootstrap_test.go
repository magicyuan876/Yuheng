package service

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

type bootstrapUserRepo struct {
	interfaces.UserRepository
	firstErr         error
	createdFirst     *types.User
	createdWorkspace *types.Tenant
	createdPlain     *types.User
}

func (r *bootstrapUserRepo) GetUserByEmail(context.Context, string) (*types.User, error) {
	return nil, nil
}

func (r *bootstrapUserRepo) GetUserByUsername(context.Context, string) (*types.User, error) {
	return nil, nil
}

func (r *bootstrapUserRepo) CreateUser(_ context.Context, u *types.User) error {
	r.createdPlain = u
	return nil
}

func (r *bootstrapUserRepo) BootstrapFirstUser(_ context.Context, u *types.User, w *types.Tenant) error {
	if r.firstErr != nil {
		return r.firstErr
	}
	// Mirror what the real repository does inside its transaction.
	w.ID = 5
	u.IsSystemAdmin = true
	u.Preferences.LastActiveTenantID = &w.ID
	r.createdFirst, r.createdWorkspace = u, w
	return nil
}

// bootstrapTenantService fails every call: registration must never reach the
// tenant service, because the bootstrap workspace is created inside the user
// repository's transaction and ordinary registration creates no workspace.
type bootstrapTenantService struct {
	interfaces.TenantService
	t *testing.T
}

func (s *bootstrapTenantService) CreateTenant(context.Context, *types.Tenant) (*types.Tenant, error) {
	s.t.Fatal("Register must not create a workspace through the tenant service")
	return nil, nil
}

func TestRegisterBootstrapFirstUserCreatesDefaultWorkspaceAtomically(t *testing.T) {
	repo := &bootstrapUserRepo{}
	svc := &userService{userRepo: repo, tenantService: &bootstrapTenantService{t: t}}

	user, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "root", Email: "root@example.com", Password: "supersecret1",
		BootstrapFirstUser: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repo.createdFirst == nil || repo.createdPlain != nil {
		t.Fatal("bootstrap registration must go through BootstrapFirstUser only")
	}
	if !user.IsSystemAdmin {
		t.Fatal("the bootstrap registrant must be the system administrator")
	}
	if pref := user.Preferences.LastActiveTenantID; pref == nil || *pref != 5 {
		t.Fatalf("last_active_tenant_id = %v, want the default workspace", pref)
	}
	ws := repo.createdWorkspace
	if ws.Name != types.DefaultWorkspaceName || ws.Status != "active" ||
		ws.DefaultStorageBackendID != types.EnvStorageBackendID {
		t.Fatalf("default workspace = %+v; want the default name, active, on the deployment backend", ws)
	}
}

func TestRegisterBootstrapHonoursTheWorkspaceName(t *testing.T) {
	repo := &bootstrapUserRepo{}
	svc := &userService{userRepo: repo, tenantService: &bootstrapTenantService{t: t}}

	if _, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "root", Email: "root@example.com", Password: "supersecret1",
		WorkspaceName: "  Acme Knowledge  ", BootstrapFirstUser: true,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repo.createdWorkspace.Name != "Acme Knowledge" {
		t.Fatalf("workspace name = %q, want the trimmed request value", repo.createdWorkspace.Name)
	}
}

func TestRegisterBootstrapLosingTheRaceIsRegistrationClosed(t *testing.T) {
	repo := &bootstrapUserRepo{firstErr: types.ErrRegistrationClosed}
	svc := &userService{userRepo: repo, tenantService: &bootstrapTenantService{t: t}}

	_, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "late", Email: "late@example.com", Password: "supersecret1", BootstrapFirstUser: true,
	})
	if !errors.Is(err, types.ErrRegistrationClosed) {
		t.Fatalf("err = %v, want ErrRegistrationClosed", err)
	}
}

func TestRegisterOrdinaryUserCreatesNoWorkspaceAndIsNeverAdmin(t *testing.T) {
	repo := &bootstrapUserRepo{}
	svc := &userService{userRepo: repo, tenantService: &bootstrapTenantService{t: t}}
	user, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "bob", Email: "bob@example.com", Password: "supersecret1",
		// Ignored outside the bootstrap: an ordinary registrant cannot name a
		// workspace into existence.
		WorkspaceName: "Bob's Empire",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.IsSystemAdmin || repo.createdPlain == nil || repo.createdFirst != nil {
		t.Fatal("ordinary registration must use CreateUser and never grant admin")
	}
	if user.Preferences.LastActiveTenantID != nil {
		t.Fatalf("an ordinary registrant belongs to no workspace, got preference %d",
			*user.Preferences.LastActiveTenantID)
	}
}
