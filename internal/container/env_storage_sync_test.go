package container

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setS3Env(t *testing.T, bucket string) {
	t.Helper()
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_ENDPOINT", "http://rustfs:9000")
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET_NAME", bucket)
	t.Setenv("S3_ACCESS_KEY", "access-"+bucket)
	t.Setenv("S3_SECRET_KEY", "secret-"+bucket)
	t.Setenv("S3_USE_SSL", "false")
	t.Setenv(storageallowlist.AllowListEnv, "")
}

func envRow(t *testing.T, db *gorm.DB) types.StorageBackend {
	t.Helper()
	var row types.StorageBackend
	require.NoError(t, db.First(&row, "id = ?", types.EnvStorageBackendID).Error)
	return row
}

// The migration leaves a placeholder; the first start writes the
// environment's storage into it — the location, never the keys.
func TestEnvStorageSyncWritesTheEnvironmentWithoutCredentials(t *testing.T) {
	setS3Env(t, "yuheng")
	db := pgtest.New(t)

	require.NoError(t, syncEnvStorageBackend(context.Background(), db))
	row := envRow(t, db)
	assert.Equal(t, types.StorageProviderS3, row.Provider)
	assert.Equal(t, "yuheng", row.Config.BucketName)
	assert.Equal(t, "http://rustfs:9000", row.Config.Endpoint)
	assert.True(t, row.IsBuiltin)
	assert.Zero(t, row.TenantID)
	assert.Empty(t, row.Config.AccessKeyID)
	assert.Empty(t, row.Config.SecretAccessKey)

	var raw string
	require.NoError(t, db.Raw("SELECT config::text FROM storage_backends WHERE id = 'env'").Scan(&raw).Error)
	assert.NotContains(t, raw, "access-yuheng")
	assert.NotContains(t, raw, "secret-yuheng")

	// A key rotation is a restart with new variables; nothing is stored.
	t.Setenv("S3_ACCESS_KEY", "rotated")
	require.NoError(t, syncEnvStorageBackend(context.Background(), db))
	assert.Empty(t, envRow(t, db).Config.AccessKeyID)
}

func TestEnvStorageSyncDefaultsToLocal(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "")
	t.Setenv("LOCAL_STORAGE_PATH_PREFIX", "")
	t.Setenv(storageallowlist.AllowListEnv, "")
	db := pgtest.New(t)

	require.NoError(t, syncEnvStorageBackend(context.Background(), db))
	assert.Equal(t, types.StorageProviderLocal, envRow(t, db).Provider)
}

// Pointing the deployment at a different bucket while files live in the old
// one would make every one of them unreadable; the server refuses to start
// instead. Without stored files the move is just a configuration change.
func TestEnvStorageSyncRefusesToMoveStoredFiles(t *testing.T) {
	setS3Env(t, "first")
	db := pgtest.New(t)
	require.NoError(t, syncEnvStorageBackend(context.Background(), db))

	setS3Env(t, "second")
	require.NoError(t, syncEnvStorageBackend(context.Background(), db), "nothing is stored yet")
	assert.Equal(t, "second", envRow(t, db).Config.BucketName)

	require.NoError(t, db.Create(&types.StoredResource{
		Handle: "AbCdEfGhIjKlMnOpQrStUv", TenantID: 1, StorageBackendID: types.EnvStorageBackendID,
		Provider: "s3", PhysicalPath: "s3://second/yuheng/1/exports/a.png", LocationHash: "h",
	}).Error)

	setS3Env(t, "third")
	err := syncEnvStorageBackend(context.Background(), db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 stored file")
	assert.Equal(t, "second", envRow(t, db).Config.BucketName, "the stored row is left as it was")

	// Settings that do not move the files may still change.
	setS3Env(t, "second")
	t.Setenv("S3_ADDRESSING_STYLE", "path")
	require.NoError(t, syncEnvStorageBackend(context.Background(), db))
	assert.Equal(t, "path", envRow(t, db).Config.AddressingStyle)
}

func TestEnvStorageSyncRefusesUnusableConfiguration(t *testing.T) {
	db := pgtest.New(t)

	t.Setenv(storageallowlist.AllowListEnv, "")
	t.Setenv("STORAGE_TYPE", "minio")
	assert.ErrorContains(t, syncEnvStorageBackend(context.Background(), db), "not a supported storage provider")

	t.Setenv("STORAGE_TYPE", "local")
	t.Setenv(storageallowlist.AllowListEnv, "s3")
	assert.ErrorContains(t, syncEnvStorageBackend(context.Background(), db), "STORAGE_ALLOW_LIST")

	setS3Env(t, "bucket")
	t.Setenv("S3_REGION", "")
	assert.ErrorContains(t, syncEnvStorageBackend(context.Background(), db), "region is required")
}

// A database migrated out of band may lack the row; the sync creates it.
func TestEnvStorageSyncCreatesAMissingRow(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "local")
	t.Setenv(storageallowlist.AllowListEnv, "")
	db := pgtest.New(t)
	require.NoError(t, db.Exec("DELETE FROM storage_backends WHERE id = 'env'").Error)

	require.NoError(t, syncEnvStorageBackend(context.Background(), db))
	row := envRow(t, db)
	assert.Equal(t, types.StorageBackendSourceEnv, row.Source)
	var tenantID *uint64
	require.NoError(t, db.Raw("SELECT tenant_id FROM storage_backends WHERE id = 'env'").Scan(&tenantID).Error)
	assert.Nil(t, tenantID, "the deployment row belongs to no workspace")
}
