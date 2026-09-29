package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageBackendPathRoundTrip(t *testing.T) {
	path := BuildStorageBackendPath("backend-a", "s3://bucket/7/file.pdf")
	id, inner, ok := ParseStorageBackendPath(path)
	require.True(t, ok)
	assert.Equal(t, "backend-a", id)
	assert.Equal(t, "s3://bucket/7/file.pdf", inner)
	assert.Equal(t, "s3", ParseProviderScheme(path))
}

func TestSharesStorageBackendWithUsesConcreteInstance(t *testing.T) {
	aID, bID := "s3-a", "s3-b"
	a := &KnowledgeBase{StorageBackendID: &aID, StorageProviderConfig: &StorageProviderConfig{Provider: "s3"}}
	b := &KnowledgeBase{StorageBackendID: &bID, StorageProviderConfig: &StorageProviderConfig{Provider: "s3"}}
	assert.False(t, a.SharesStorageBackendWith(b, "", "s3"))

	b.StorageBackendID = &aID
	assert.True(t, a.SharesStorageBackendWith(b, "", "s3"))
}

func TestNewStorageBackendResponseMasksCredentials(t *testing.T) {
	backend := &StorageBackend{Config: StorageBackendConfig{AccessKeyID: "id", SecretAccessKey: "secret"}}
	response := NewStorageBackendResponse(backend)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.AccessKeyID)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.SecretAccessKey)
	assert.Equal(t, "id", backend.Config.AccessKeyID)
}

func TestStorageBackendFromEnvironment(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("S3_REGION", "ap-test-1")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_BUCKET_NAME", "bucket")
	t.Setenv("S3_ADDRESSING_STYLE", " Virtual ")

	backend := StorageBackendFromEnvironment(42)
	require.NotNil(t, backend)
	assert.Equal(t, uint64(42), backend.TenantID)
	assert.Equal(t, "s3", backend.Provider)
	assert.Equal(t, StorageBackendSourceEnv, backend.Source)
	assert.True(t, backend.LegacyAlias)
	assert.Equal(t, "bucket", backend.Config.BucketName)
	assert.Equal(t, "virtual", backend.Config.AddressingStyle)
}

func TestStorageBackendFromEnvironmentIgnoresRemovedProviders(t *testing.T) {
	for _, provider := range []string{"minio", "cos", "tos", "oss", "obs", "ks3"} {
		t.Setenv("STORAGE_TYPE", provider)
		assert.Nil(t, StorageBackendFromEnvironment(1), provider)
	}
}

func TestStorageBackendValidate(t *testing.T) {
	validS3 := StorageBackendConfig{Region: "us-east-1", BucketName: "docs"}
	tests := []struct {
		name     string
		provider string
		config   StorageBackendConfig
		wantErr  string
	}{
		{name: "local", provider: "local"},
		{name: "s3 with the default credential chain", provider: "s3", config: validS3},
		{
			name: "s3 with keys and a style", provider: "s3",
			config: StorageBackendConfig{
				Endpoint: "http://rustfs:9000", Region: "us-east-1", BucketName: "docs",
				AccessKeyID: "ak", SecretAccessKey: "sk", AddressingStyle: "path",
			},
		},
		{
			name: "s3 without a region", provider: "s3",
			config:  StorageBackendConfig{BucketName: "docs"},
			wantErr: "region is required",
		},
		{
			name: "s3 without a bucket", provider: "s3",
			config:  StorageBackendConfig{Region: "us-east-1"},
			wantErr: "bucket_name is required",
		},
		{
			name: "s3 with a lone access key", provider: "s3",
			config:  StorageBackendConfig{Region: "us-east-1", BucketName: "docs", AccessKeyID: "ak"},
			wantErr: "must be provided together",
		},
		{
			name: "s3 with an unknown addressing style", provider: "s3",
			config:  StorageBackendConfig{Region: "us-east-1", BucketName: "docs", AddressingStyle: "sideways"},
			wantErr: "addressing_style",
		},
		{name: "minio was removed", provider: "minio", config: validS3, wantErr: "unsupported storage provider"},
		{name: "cos was removed", provider: "cos", config: validS3, wantErr: "unsupported storage provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &StorageBackend{TenantID: 1, Name: "backend", Provider: tt.provider, Config: tt.config}
			err := backend.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestStorageBackendToStorageEngineConfig(t *testing.T) {
	backend := StorageBackend{Provider: "s3", Config: StorageBackendConfig{
		Endpoint: "http://rustfs:9000", Region: "us-east-1", BucketName: "docs", AccessKeyID: "ak",
		SecretAccessKey: "sk", PathPrefix: "yuheng", UseSSL: true, AddressingStyle: "path",
	}}
	cfg := backend.ToStorageEngineConfig()
	require.NotNil(t, cfg.S3)
	assert.Equal(t, "s3", cfg.DefaultProvider)
	assert.Equal(t, "path", cfg.S3.AddressingStyle)
	assert.Equal(t, "sk", cfg.S3.SecretKey)
	assert.True(t, cfg.S3.UseSSL)
}

func TestStorageEngineConfigValidate(t *testing.T) {
	assert.NoError(t, (*StorageEngineConfig)(nil).Validate())
	assert.NoError(t, (&StorageEngineConfig{DefaultProvider: "local"}).Validate())
	for _, style := range []string{"", "auto", "path", "virtual"} {
		cfg := &StorageEngineConfig{S3: &S3EngineConfig{AddressingStyle: style}}
		assert.NoError(t, cfg.Validate(), style)
	}
	bad := &StorageEngineConfig{S3: &S3EngineConfig{AddressingStyle: "bucket-first"}}
	assert.Error(t, bad.Validate())
}

func TestStorageBackendRejectsTraversingPathPrefix(t *testing.T) {
	backend := &StorageBackend{
		TenantID: 1, Name: "unsafe", Provider: "local",
		Config: StorageBackendConfig{PathPrefix: "../outside"},
	}
	require.Error(t, backend.Validate())
}
