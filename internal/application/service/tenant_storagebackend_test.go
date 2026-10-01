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
)

// A new workspace starts on the deployment backend; it gets no storage row of
// its own (there used to be a per-workspace copy of the environment storage).
func TestCreateTenantDefaultsToTheDeploymentBackend(t *testing.T) {
	db := pgtest.New(t)
	tenantSvc := service.NewTenantService(repository.NewTenantRepository(db))

	tenant, err := tenantSvc.CreateTenant(context.Background(), &types.Tenant{Name: "workspace"})
	require.NoError(t, err)
	assert.Equal(t, types.EnvStorageBackendID, tenant.DefaultStorageBackendID)

	var stored string
	require.NoError(t, db.Raw("SELECT default_storage_backend_id FROM tenants WHERE id = ?", tenant.ID).
		Scan(&stored).Error)
	assert.Equal(t, types.EnvStorageBackendID, stored)

	var owned int64
	require.NoError(t, db.Model(&types.StorageBackend{}).Where("tenant_id = ?", tenant.ID).Count(&owned).Error)
	assert.Zero(t, owned)
}
