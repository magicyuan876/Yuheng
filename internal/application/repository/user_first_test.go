package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

func newFirstUserCandidate(n int) *types.User {
	return &types.User{
		ID:           fmt.Sprintf("user-%d", n),
		Username:     fmt.Sprintf("user%d", n),
		Email:        fmt.Sprintf("user%d@example.com", n),
		PasswordHash: "x",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func newDefaultWorkspaceCandidate(n int) *types.Tenant {
	return &types.Tenant{Name: fmt.Sprintf("Workspace %d", n), Status: "active"}
}

// TestBootstrapFirstUserCreatesUserWorkspaceAndOwnership pins the whole
// bootstrap: the administrator, the deployment's default workspace, the Owner
// row joining them and the preference that makes the workspace the
// administrator's current one. A second attempt changes nothing.
func TestBootstrapFirstUserCreatesUserWorkspaceAndOwnership(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	repo := NewUserRepository(db)

	has, err := repo.HasAnyUser(ctx)
	if err != nil || has {
		t.Fatalf("empty database: HasAnyUser = %v, %v", has, err)
	}

	first := newFirstUserCandidate(1)
	workspace := newDefaultWorkspaceCandidate(1)
	if err := repo.BootstrapFirstUser(ctx, first, workspace); err != nil {
		t.Fatalf("first user: %v", err)
	}
	if workspace.ID == 0 {
		t.Fatal("the workspace was not created")
	}
	stored, err := repo.GetUserByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsSystemAdmin {
		t.Fatal("the first user must be the system administrator")
	}
	if pref := stored.Preferences.LastActiveTenantID; pref == nil || *pref != workspace.ID {
		t.Fatalf("last_active_tenant_id = %v, want the default workspace %d", pref, workspace.ID)
	}
	var member types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", first.ID, workspace.ID).First(&member).Error; err != nil {
		t.Fatalf("owner membership: %v", err)
	}
	if member.Role != types.TenantRoleOwner || member.Status != types.TenantMemberStatusActive {
		t.Fatalf("membership role=%s status=%s, want an active owner", member.Role, member.Status)
	}
	var tenant types.Tenant
	if err := db.First(&tenant, workspace.ID).Error; err != nil || tenant.Name != "Workspace 1" {
		t.Fatalf("workspace row = %+v, %v", tenant, err)
	}
	if has, _ := repo.HasAnyUser(ctx); !has {
		t.Fatal("HasAnyUser must be true once a user exists")
	}

	err = repo.BootstrapFirstUser(ctx, newFirstUserCandidate(2), newDefaultWorkspaceCandidate(2))
	if !errors.Is(err, types.ErrRegistrationClosed) {
		t.Fatalf("second BootstrapFirstUser = %v, want ErrRegistrationClosed", err)
	}
	var tenants int64
	db.Model(&types.Tenant{}).Count(&tenants)
	if tenants != 1 {
		t.Fatalf("%d workspaces after the refused second bootstrap, want 1", tenants)
	}
}

// TestBootstrapFirstUserIsAtomic: when the user insert fails after the
// workspace insert succeeded, nothing is left behind. The failure is forced
// with a username longer than the column allows.
func TestBootstrapFirstUserIsAtomic(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	repo := NewUserRepository(db)

	bad := newFirstUserCandidate(1)
	for len(bad.Username) <= 100 {
		bad.Username += "overflow"
	}
	if err := repo.BootstrapFirstUser(ctx, bad, newDefaultWorkspaceCandidate(1)); err == nil {
		t.Fatal("an oversized username was accepted")
	}
	var users, tenants, members int64
	db.Model(&types.User{}).Count(&users)
	db.Model(&types.Tenant{}).Count(&tenants)
	db.Model(&types.TenantMember{}).Count(&members)
	if users != 0 || tenants != 0 || members != 0 {
		t.Fatalf("users=%d tenants=%d members=%d after a failed bootstrap, want all 0", users, tenants, members)
	}
	if has, _ := repo.HasAnyUser(ctx); has {
		t.Fatal("a failed bootstrap must leave registration open")
	}
}

// TestBootstrapFirstUserRace fires many simultaneous first registrations; the
// advisory lock must let exactly one through, so there is exactly one
// administrator, one workspace and one owner.
func TestBootstrapFirstUserRace(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	repo := NewUserRepository(db)

	const n = 12
	var wg sync.WaitGroup
	results := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = repo.BootstrapFirstUser(ctx, newFirstUserCandidate(i), newDefaultWorkspaceCandidate(i))
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, types.ErrRegistrationClosed):
		default:
			t.Fatalf("attempt %d: unexpected error %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("%d registrations won, want exactly 1", winners)
	}
	var admins, total, tenants, members int64
	db.Model(&types.User{}).Count(&total)
	db.Model(&types.User{}).Where("is_system_admin = ?", true).Count(&admins)
	db.Model(&types.Tenant{}).Count(&tenants)
	db.Model(&types.TenantMember{}).Count(&members)
	if total != 1 || admins != 1 || tenants != 1 || members != 1 {
		t.Fatalf("users=%d admins=%d tenants=%d members=%d, want 1 each", total, admins, tenants, members)
	}
}
