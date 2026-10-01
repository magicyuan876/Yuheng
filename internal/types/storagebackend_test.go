package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two knowledge bases share storage only when they are bound to the same
// backend; the same provider is not enough (two S3 buckets are two stores).
func TestSharesStorageBackendWithComparesBackendIDs(t *testing.T) {
	a := &KnowledgeBase{StorageBackendID: "s3-a"}
	b := &KnowledgeBase{StorageBackendID: "s3-b"}
	assert.False(t, a.SharesStorageBackendWith(b))

	b.StorageBackendID = "s3-a"
	assert.True(t, a.SharesStorageBackendWith(b))
	assert.False(t, (&KnowledgeBase{}).SharesStorageBackendWith(&KnowledgeBase{}), "unbound is not shared")
}

func TestNewStorageBackendResponseMasksCredentials(t *testing.T) {
	backend := &StorageBackend{Config: StorageBackendConfig{AccessKeyID: "id", SecretAccessKey: "secret"}}
	response := NewStorageBackendResponse(backend)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.AccessKeyID)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.SecretAccessKey)
	assert.Equal(t, "id", backend.Config.AccessKeyID)
}

func TestEnvStorageBackend(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("S3_REGION", "ap-test-1")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_BUCKET_NAME", "bucket")
	t.Setenv("S3_ADDRESSING_STYLE", " Virtual ")

	backend, err := EnvStorageBackend()
	require.NoError(t, err)
	assert.Equal(t, EnvStorageBackendID, backend.ID)
	assert.Zero(t, backend.TenantID, "the deployment backend belongs to no workspace")
	assert.True(t, backend.IsBuiltin)
	assert.Equal(t, "s3", backend.Provider)
	assert.Equal(t, StorageBackendSourceEnv, backend.Source)
	assert.Equal(t, "bucket", backend.Config.BucketName)
	assert.Equal(t, "virtual", backend.Config.AddressingStyle)
	assert.Equal(t, "access", backend.Config.AccessKeyID)
}

func TestEnvStorageBackendDefaultsToLocal(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "")
	t.Setenv("LOCAL_STORAGE_PATH_PREFIX", "deploy")
	backend, err := EnvStorageBackend()
	require.NoError(t, err)
	assert.Equal(t, StorageProviderLocal, backend.Provider)
	assert.Equal(t, "deploy", backend.Config.PathPrefix)
}

func TestEnvStorageBackendRefusesWhatItCannotUse(t *testing.T) {
	for _, provider := range []string{"minio", "cos", "tos", "oss", "obs", "ks3", "dummy"} {
		t.Setenv("STORAGE_TYPE", provider)
		_, err := EnvStorageBackend()
		assert.Error(t, err, provider)
	}
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_REGION", "")
	t.Setenv("S3_BUCKET_NAME", "bucket")
	_, err := EnvStorageBackend()
	assert.Error(t, err, "an incomplete S3 configuration")
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

func TestValidateS3AddressingStyle(t *testing.T) {
	for _, style := range []string{"", "auto", "path", "virtual"} {
		assert.NoError(t, ValidateS3AddressingStyle(style), style)
	}
	assert.Error(t, ValidateS3AddressingStyle("bucket-first"))
}

func TestStorageBackendConfigMergeSecrets(t *testing.T) {
	stored := StorageBackendConfig{AccessKeyID: "stored-ak", SecretAccessKey: "stored-sk", Region: "us-east-1"}

	kept := StorageBackendConfig{AccessKeyID: RedactedSecretPlaceholder, SecretAccessKey: RedactedSecretPlaceholder}.
		MergeSecrets(stored)
	assert.Equal(t, "stored-ak", kept.AccessKeyID, "the placeholder the client was shown keeps the stored key")
	assert.Equal(t, "stored-sk", kept.SecretAccessKey)

	cleared := StorageBackendConfig{}.MergeSecrets(stored)
	assert.Empty(t, cleared.AccessKeyID, "empty keys select the default credential chain")
	assert.Empty(t, cleared.SecretAccessKey)

	replaced := StorageBackendConfig{AccessKeyID: "new-ak", SecretAccessKey: "new-sk"}.MergeSecrets(stored)
	assert.Equal(t, "new-ak", replaced.AccessKeyID)
}

func TestStorageBackendRejectsTraversingPathPrefix(t *testing.T) {
	backend := &StorageBackend{
		TenantID: 1, Name: "unsafe", Provider: "local",
		Config: StorageBackendConfig{PathPrefix: "../outside"},
	}
	require.Error(t, backend.Validate())
}
