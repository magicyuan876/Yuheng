package service_test

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/service"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newGovernanceFixture is two workspaces and a storage service over a fresh
// database. Workspace 1 owns the backends under test; workspace 2 is "another
// workspace" for the cross-workspace checks.
func newGovernanceFixture(t *testing.T) (*gorm.DB, *service.StorageBackendService) {
	t.Helper()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	db := pgtest.New(t)
	for _, id := range []uint64{1, 2} {
		require.NoError(t, db.Create(&types.Tenant{ID: id, Name: "workspace"}).Error)
	}
	return db, service.NewStorageBackendService(repository.NewStorageBackendRepository(db), db)
}

// ownedBackend inserts a user backend of workspace 1, bypassing the
// connectivity test Create runs.
func ownedBackend(t *testing.T, db *gorm.DB, name string, shared bool) *types.StorageBackend {
	t.Helper()
	backend := &types.StorageBackend{TenantID: 1, Name: name, Provider: types.StorageProviderLocal, IsBuiltin: shared}
	require.NoError(t, db.Create(backend).Error)
	return backend
}

func asWorkspace(id uint64, systemAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, id)
	return context.WithValue(ctx, types.SystemAdminContextKey, systemAdmin)
}

// B6: a workspace may make any backend it can see its default, a shared one
// owned by another workspace included.
func TestSetDefaultAcceptsASharedBackend(t *testing.T) {
	db, svc := newGovernanceFixture(t)
	shared := ownedBackend(t, db, "shared", true)
	private := ownedBackend(t, db, "private", false)

	require.NoError(t, svc.SetDefault(asWorkspace(2, false), 2, shared.ID))
	var tenant types.Tenant
	require.NoError(t, db.First(&tenant, 2).Error)
	assert.Equal(t, shared.ID, tenant.DefaultStorageBackendID)

	require.NoError(t, svc.SetDefault(asWorkspace(2, false), 2, types.EnvStorageBackendID),
		"the deployment backend is visible to every workspace")
	assert.Error(t, svc.SetDefault(asWorkspace(2, false), 2, private.ID),
		"another workspace's private backend is not")
}

// B7: disabling counts bindings in every workspace, not just the owner's.
func TestDisableIsRefusedWhileAnotherWorkspaceUsesTheBackend(t *testing.T) {
	db, svc := newGovernanceFixture(t)
	shared := ownedBackend(t, db, "shared", true)
	require.NoError(t, db.Create(&types.KnowledgeBase{
		ID: "kb-2", TenantID: 2, Name: "borrower", StorageBackendID: shared.ID,
	}).Error)

	err := svc.Update(asWorkspace(1, true), &types.StorageBackend{
		ID: shared.ID, TenantID: 1, Name: shared.Name, Status: types.StorageBackendStatusDisabled,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge base")
}

// B8: a docs space binding keeps a backend from being deleted, and so does
// every other kind of binding, in any workspace.
func TestDeleteIsRefusedWhileAnythingIsBound(t *testing.T) {
	cases := map[string]func(t *testing.T, db *gorm.DB, id string){
		"docs space": func(t *testing.T, db *gorm.DB, id string) {
			require.NoError(t, db.Exec(`INSERT INTO docs_spaces (id, tenant_id, slug, name, storage_backend_id)
				VALUES ('sp-1', 2, 'eng', 'Engineering', ?)`, id).Error)
		},
		"knowledge base": func(t *testing.T, db *gorm.DB, id string) {
			require.NoError(t, db.Create(&types.KnowledgeBase{
				ID: "kb-1", TenantID: 1, Name: "kb", StorageBackendID: id,
			}).Error)
		},
		"workspace default": func(t *testing.T, db *gorm.DB, id string) {
			require.NoError(t, db.Model(&types.Tenant{}).Where("id = 2").
				Update("default_storage_backend_id", id).Error)
		},
		"stored file": func(t *testing.T, db *gorm.DB, id string) {
			require.NoError(t, db.Create(&types.StoredResource{
				Handle: "AbCdEfGhIjKlMnOpQrStUv", TenantID: 2, StorageBackendID: id,
				PhysicalPath: "local://2/exports/a.png", LocationHash: "h",
			}).Error)
		},
	}
	for name, bind := range cases {
		t.Run(name, func(t *testing.T) {
			db, svc := newGovernanceFixture(t)
			backend := ownedBackend(t, db, "doomed", false)
			bind(t, db, backend.ID)

			err := svc.Delete(asWorkspace(1, false), 1, backend.ID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "still in use")
		})
	}

	t.Run("nothing bound", func(t *testing.T) {
		db, svc := newGovernanceFixture(t)
		backend := ownedBackend(t, db, "unused", false)
		require.NoError(t, svc.Delete(asWorkspace(1, false), 1, backend.ID))
	})
}

// Un-sharing withdraws a backend from everyone but its owner, so only other
// workspaces' bindings block it.
func TestUnshareIsRefusedWhileAnotherWorkspaceUsesTheBackend(t *testing.T) {
	db, svc := newGovernanceFixture(t)
	shared := ownedBackend(t, db, "shared", true)
	require.NoError(t, db.Create(&types.KnowledgeBase{
		ID: "kb-own", TenantID: 1, Name: "owner", StorageBackendID: shared.ID,
	}).Error)

	_, err := svc.SetSharing(asWorkspace(1, true), shared.ID, false)
	require.NoError(t, err, "the owner's own binding does not block un-sharing")

	_, err = svc.SetSharing(asWorkspace(1, true), shared.ID, true)
	require.NoError(t, err)
	require.NoError(t, db.Model(&types.Tenant{}).Where("id = 2").
		Update("default_storage_backend_id", shared.ID).Error)
	_, err = svc.SetSharing(asWorkspace(1, true), shared.ID, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace default")
}

// The deployment backend is the environment's: the API cannot edit, delete or
// re-share it.
func TestDeploymentBackendIsReadOnly(t *testing.T) {
	db, svc := newGovernanceFixture(t)

	err := svc.Update(asWorkspace(1, true), &types.StorageBackend{
		ID: types.EnvStorageBackendID, TenantID: 1, Name: "renamed",
	})
	assert.ErrorContains(t, err, "read-only")
	assert.ErrorContains(t, svc.Delete(asWorkspace(1, true), 1, types.EnvStorageBackendID), "read-only")
	_, err = svc.SetSharing(asWorkspace(1, true), types.EnvStorageBackendID, false)
	assert.ErrorContains(t, err, "read-only")

	var row types.StorageBackend
	require.NoError(t, db.First(&row, "id = ?", types.EnvStorageBackendID).Error)
	assert.Equal(t, "Deployment storage", row.Name)
}

// A new binding takes the workspace default when it names nothing, and never
// lands on a disabled backend.
func TestResolveBackend(t *testing.T) {
	db, svc := newGovernanceFixture(t)
	ctx := asWorkspace(1, false)

	backend, err := svc.ResolveBackend(ctx, 1, "")
	require.NoError(t, err)
	assert.Equal(t, types.EnvStorageBackendID, backend.ID)

	disabled := ownedBackend(t, db, "disabled", false)
	require.NoError(t, db.Model(disabled).Update("status", types.StorageBackendStatusDisabled).Error)
	_, err = svc.ResolveBackend(ctx, 1, disabled.ID)
	assert.ErrorContains(t, err, "not active")
}
