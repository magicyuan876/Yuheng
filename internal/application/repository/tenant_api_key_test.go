package repository

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// insertAPIKeyTenant creates the tenant a key belongs to: tenant_api_keys
// references tenants. The id is given explicitly so the tests can keep their
// fixed tenant ids rather than take whatever the sequence hands out.
func insertAPIKeyTenant(t *testing.T, db *gorm.DB, id uint64) {
	t.Helper()
	require.NoError(t, db.Create(&types.Tenant{ID: id, Name: "tenant-" + strconv.FormatUint(id, 10)}).Error)
}

func TestTenantAPIKeyRepositoryPersistsUTCExpiry(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")

	db := pgtest.New(t)
	insertAPIKeyTenant(t, db, 42)

	repo := NewTenantAPIKeyRepository(db)
	ctx := context.Background()

	expiresAt := time.Unix(time.Now().UTC().Add(5*time.Second).Unix(), 0).UTC()
	tenantID := uint64(42)
	key := &types.TenantAPIKey{
		TenantID:   &tenantID,
		ScopeType:  types.APIKeyScopeTenant,
		Name:       "integration",
		KeyHash:    "hash-expiry",
		KeyHint:    "sk-test",
		FullAccess: true,
		ExpiresAt:  &expiresAt,
	}
	require.NoError(t, repo.CreateAPIKey(ctx, key))

	loaded, err := repo.GetAPIKeyByHash(ctx, key.KeyHash)
	require.NoError(t, err)
	require.NotNil(t, loaded.ExpiresAt)
	require.True(t, loaded.ExpiresAt.Equal(expiresAt))

	// expires_at is timestamptz, so the database holds an instant and the
	// driver hands it back in the process's zone; the time.Location of the
	// loaded value says nothing about what was stored. What must hold is that
	// the stored instant, read as UTC wall-clock time, is the expiry that was
	// written, whatever zone the process runs in.
	var storedUTC string
	require.NoError(t, db.Raw(
		`SELECT to_char(expires_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS')
		FROM tenant_api_keys WHERE key_hash = ?`,
		key.KeyHash,
	).Scan(&storedUTC).Error)
	require.Equal(t, expiresAt.Format(time.DateTime), storedUTC)
}

// TestTenantAPIKeyRepositoryUpdateIsTenantScoped 验证通用更新不会越过租户边界。
// 输入同租户和其他租户的 Key；前者更新全部可配置字段，后者必须返回未找到。
func TestTenantAPIKeyRepositoryUpdateIsTenantScoped(t *testing.T) {
	db := pgtest.New(t)
	insertAPIKeyTenant(t, db, 42)
	insertAPIKeyTenant(t, db, 43)
	repo := NewTenantAPIKeyRepository(db)
	ctx := context.Background()
	tenant42, tenant43 := uint64(42), uint64(43)
	key := func(tenant *uint64, name string, fullAccess bool) *types.TenantAPIKey {
		return &types.TenantAPIKey{
			TenantID: tenant, ScopeType: types.APIKeyScopeTenant, Name: name,
			KeyHash: "hash-" + name, KeyHint: "sk-" + name, FullAccess: fullAccess,
		}
	}
	keys := []*types.TenantAPIKey{
		key(&tenant42, "scoped", false), key(&tenant43, "other", false), key(&tenant42, "full", true),
	}
	for _, key := range keys {
		require.NoError(t, repo.CreateAPIKey(ctx, key))
	}

	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	updated, err := repo.UpdateAPIKey(ctx, tenant42, keys[0].ID, &types.TenantAPIKey{
		Name: "updated", FullAccess: false,
		KnowledgeBaseIDs: types.StringArray{"kb-1", "kb-2"},
		Capabilities:     types.StringArray{"retrieve", "chat"},
		ExpiresAt:        &expiresAt,
	})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Name)
	require.Equal(t, types.StringArray{"kb-1", "kb-2"}, updated.KnowledgeBaseIDs)
	require.Equal(t, types.StringArray{"retrieve", "chat"}, updated.Capabilities)
	require.NotNil(t, updated.ExpiresAt)
	require.True(t, updated.ExpiresAt.Equal(expiresAt))

	_, err = repo.UpdateAPIKey(ctx, tenant42, keys[1].ID, &types.TenantAPIKey{Name: "blocked"})
	require.ErrorIs(t, err, ErrTenantAPIKeyNotFound)

	full, err := repo.UpdateAPIKey(ctx, tenant42, keys[2].ID, &types.TenantAPIKey{
		Name: "full updated", FullAccess: false, Capabilities: types.StringArray{"retrieve"},
	})
	require.NoError(t, err)
	require.False(t, full.FullAccess)
}
