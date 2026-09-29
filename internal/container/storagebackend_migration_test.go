package container

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyStorageBackends(t *testing.T) {
	// The production schema: tenants, knowledge_bases and storage_backends as
	// the migrations create them, so the test exercises only the
	// data-migration logic against the columns the server really has.
	db := pgtest.New(t)
	tenantConfig, err := (&types.StorageEngineConfig{DefaultProvider: "local", Local: &types.LocalEngineConfig{PathPrefix: "workspace-a"}}).Value()
	require.NoError(t, err)
	providerConfig, err := (types.StorageProviderConfig{Provider: "local"}).Value()
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO tenants(id, name, business, storage_engine_config) VALUES (?, ?, ?, ?)",
		7, "workspace", "test", tenantConfig).Error)
	require.NoError(t, db.Exec(`INSERT INTO knowledge_bases
		(id, tenant_id, name, embedding_model_id, summary_model_id, storage_provider_config, cos_config)
		VALUES (?, ?, ?, '', '', ?, ?)`, "kb-a", 7, "kb", providerConfig, "{}").Error)

	migrateLegacyStorageBackends(db)

	var backend types.StorageBackend
	require.NoError(t, db.Where("tenant_id = ? AND provider = ? AND legacy_alias = ?", 7, "local", true).First(&backend).Error)
	assert.Equal(t, "workspace-a", backend.Config.PathPrefix)

	var tenantDefault, kbBackend string
	require.NoError(t, db.Raw("SELECT default_storage_backend_id FROM tenants WHERE id = 7").Scan(&tenantDefault).Error)
	require.NoError(t, db.Raw("SELECT storage_backend_id FROM knowledge_bases WHERE id = 'kb-a'").Scan(&kbBackend).Error)
	assert.Equal(t, backend.ID, tenantDefault)
	assert.Equal(t, backend.ID, kbBackend)
}

// Rows written while the dedicated MinIO/COS/... providers existed must not
// stop the server from starting: the migration skips what it cannot map and
// leaves the affected workspace and knowledge bases unbound, so that using
// them fails later with a clear "unsupported storage provider" error instead.
func TestMigrateLegacyStorageBackendsToleratesRemovedProviders(t *testing.T) {
	db := pgtest.New(t)
	const oldConfig = `{"default_provider":"minio","minio":{"mode":"remote","bucket_name":"old"}}`
	require.NoError(t, db.Exec(`INSERT INTO tenants(id, name, business, storage_engine_config)
		VALUES (8, 'legacy', 'test', ?::jsonb)`, oldConfig).Error)
	require.NoError(t, db.Exec(`INSERT INTO knowledge_bases
		(id, tenant_id, name, embedding_model_id, summary_model_id, storage_provider_config, cos_config)
		VALUES ('kb-old', 8, 'kb', '', '', '{"provider":"minio"}'::jsonb, '{}')`).Error)

	require.NotPanics(t, func() { migrateLegacyStorageBackends(db) })

	var kbBackend *string
	require.NoError(t, db.Raw(
		"SELECT storage_backend_id FROM knowledge_bases WHERE id = 'kb-old'").Scan(&kbBackend).Error)
	assert.Nil(t, kbBackend, "a KB on a removed provider stays unbound")

	var minioBackends int64
	require.NoError(t, db.Model(&types.StorageBackend{}).
		Where("tenant_id = ? AND provider = ?", 8, "minio").Count(&minioBackends).Error)
	assert.Zero(t, minioBackends)
}
