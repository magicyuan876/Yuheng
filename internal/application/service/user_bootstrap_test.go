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
	firstErr     error
	createdFirst *types.User
	createdPlain *types.User
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

func (r *bootstrapUserRepo) CreateFirstUser(_ context.Context, u *types.User) error {
	if r.firstErr != nil {
		return r.firstErr
	}
	u.IsSystemAdmin = true
	r.createdFirst = u
	return nil
}

type bootstrapTenantService struct {
	interfaces.TenantService
	created, deleted int
}

func (s *bootstrapTenantService) CreateTenant(context.Context, *types.Tenant) (*types.Tenant, error) {
	s.created++
	return &types.Tenant{ID: 5}, nil
}

func (s *bootstrapTenantService) DeleteTenant(context.Context, uint64) error {
	s.deleted++
	return nil
}

func TestRegisterBootstrapFirstUserGetsWorkspaceAndAdminRole(t *testing.T) {
	repo := &bootstrapUserRepo{}
	tenants := &bootstrapTenantService{}
	svc := &userService{userRepo: repo, tenantService: tenants}

	// Even when the deployment default is tenantless, the bootstrap admin
	// gets a workspace of their own.
	user, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "root", Email: "root@example.com", Password: "supersecret1",
		TenantProvisioning: types.TenantProvisioningTenantless,
		BootstrapFirstUser: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repo.createdFirst == nil || repo.createdPlain != nil {
		t.Fatal("bootstrap registration must go through CreateFirstUser only")
	}
	if !user.IsSystemAdmin || user.TenantID != 5 || tenants.created != 1 {
		t.Fatalf("admin=%v tenant=%d created=%d", user.IsSystemAdmin, user.TenantID, tenants.created)
	}
}

func TestRegisterBootstrapLosingTheRaceRollsBackTheTenant(t *testing.T) {
	repo := &bootstrapUserRepo{firstErr: types.ErrRegistrationClosed}
	tenants := &bootstrapTenantService{}
	svc := &userService{userRepo: repo, tenantService: tenants}

	_, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "late", Email: "late@example.com", Password: "supersecret1", BootstrapFirstUser: true,
	})
	if !errors.Is(err, types.ErrRegistrationClosed) {
		t.Fatalf("err = %v, want ErrRegistrationClosed", err)
	}
	if tenants.created != 1 || tenants.deleted != 1 {
		t.Fatalf("workspace created=%d deleted=%d; the loser must not leave one behind",
			tenants.created, tenants.deleted)
	}
}

func TestRegisterOrdinaryUserIsNeverAdmin(t *testing.T) {
	repo := &bootstrapUserRepo{}
	svc := &userService{userRepo: repo, tenantService: &bootstrapTenantService{}}
	user, err := svc.Register(context.Background(), &types.RegisterRequest{
		Username: "bob", Email: "bob@example.com", Password: "supersecret1",
		TenantProvisioning: types.TenantProvisioningTenantless,
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.IsSystemAdmin || repo.createdPlain == nil || repo.createdFirst != nil {
		t.Fatal("ordinary registration must use CreateUser and never grant admin")
	}
}
