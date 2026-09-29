package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSearchConfigForResponse_MasksSecrets(t *testing.T) {
	cfg := &WebSearchConfig{
		APIKey:   "search-secret",
		ProxyURL: "http://proxy.internal:8080",
	}
	resp := WebSearchConfigForResponse(cfg, true)
	require.NotNil(t, resp)
	assert.Empty(t, resp.APIKey)
	assert.Equal(t, RedactedSecretPlaceholder, resp.ProxyURL)
}

func TestWebSearchConfigForResponse_Unmasked(t *testing.T) {
	cfg := &WebSearchConfig{APIKey: "search-secret", ProxyURL: "http://proxy"}
	resp := WebSearchConfigForResponse(cfg, false)
	require.NotNil(t, resp)
	assert.Equal(t, "search-secret", resp.APIKey)
	assert.Equal(t, "http://proxy", resp.ProxyURL)
}

func TestMergeWebSearchConfigForUpdate_PreservesRedactedSecrets(t *testing.T) {
	existing := &WebSearchConfig{APIKey: "stored-key", ProxyURL: "http://stored"}
	incoming := &WebSearchConfig{
		APIKey:     "",
		ProxyURL:   RedactedSecretPlaceholder,
		MaxResults: 10,
	}
	merged := MergeWebSearchConfigForUpdate(incoming, existing)
	require.NotNil(t, merged)
	assert.Equal(t, "stored-key", merged.APIKey)
	assert.Equal(t, "http://stored", merged.ProxyURL)
	assert.Equal(t, 10, merged.MaxResults)
}

func TestMergeParserEngineConfigForUpdate_PreservesRedactedSecrets(t *testing.T) {
	existing := &ParserEngineConfig{
		MinerUAPIKey:          "mineru-secret",
		PaddleOCRVLCloudToken: "paddle-secret",
		MinerUEndpoint:        "http://mineru",
	}
	incoming := &ParserEngineConfig{
		MinerUAPIKey:          RedactedSecretPlaceholder,
		PaddleOCRVLCloudToken: RedactedSecretPlaceholder,
		MinerUEndpoint:        "http://mineru-new",
	}
	merged := MergeParserEngineConfigForUpdate(incoming, existing)
	require.NotNil(t, merged)
	assert.Equal(t, "mineru-secret", merged.MinerUAPIKey)
	assert.Equal(t, "paddle-secret", merged.PaddleOCRVLCloudToken)
	assert.Equal(t, "http://mineru-new", merged.MinerUEndpoint)
}

func TestMergeParserEngineConfigForUpdate_PreservesLegacyChatParserRules(t *testing.T) {
	existing := &ParserEngineConfig{
		ChatParserEngineRules: []ParserEngineRule{
			{FileTypes: []string{"pdf"}, Engine: "mineru"},
		},
	}
	incoming := &ParserEngineConfig{
		MinerUEndpoint: "http://mineru-new",
	}
	merged := MergeParserEngineConfigForUpdate(incoming, existing)
	require.NotNil(t, merged)
	require.Len(t, merged.ChatParserEngineRules, 1)
	assert.Equal(t, "mineru", merged.ChatParserEngineRules[0].Engine)
}

func TestMergeStorageEngineConfigForUpdate_ClearsS3Credentials(t *testing.T) {
	existing := &StorageEngineConfig{
		DefaultProvider: "s3",
		S3: &S3EngineConfig{
			AccessKey:  "stored-access-key",
			SecretKey:  "stored-secret-key",
			Region:     "us-east-1",
			BucketName: "bucket",
		},
	}

	t.Run("empty credentials enable the default credential chain", func(t *testing.T) {
		incoming := &StorageEngineConfig{
			DefaultProvider: "s3",
			S3: &S3EngineConfig{
				AccessKey:  "",
				SecretKey:  "",
				Region:     "us-east-1",
				BucketName: "bucket",
			},
		}
		merged := MergeStorageEngineConfigForUpdate(incoming, existing)
		require.NotNil(t, merged)
		require.NotNil(t, merged.S3)
		assert.Empty(t, merged.S3.AccessKey)
		assert.Empty(t, merged.S3.SecretKey)
	})

	t.Run("redacted placeholders preserve stored credentials", func(t *testing.T) {
		incoming := &StorageEngineConfig{
			DefaultProvider: "s3",
			S3: &S3EngineConfig{
				AccessKey:  RedactedSecretPlaceholder,
				SecretKey:  RedactedSecretPlaceholder,
				Region:     "us-east-1",
				BucketName: "bucket",
			},
		}
		merged := MergeStorageEngineConfigForUpdate(incoming, existing)
		require.NotNil(t, merged)
		require.NotNil(t, merged.S3)
		assert.Equal(t, "stored-access-key", merged.S3.AccessKey)
		assert.Equal(t, "stored-secret-key", merged.S3.SecretKey)
	})
}

func TestParserEngineConfigForResponse_NilSafe(t *testing.T) {
	assert.Nil(t, ParserEngineConfigForResponse(nil, true))
}

func TestStorageEngineConfigForResponse_NilSafe(t *testing.T) {
	assert.Nil(t, StorageEngineConfigForResponse(nil, true))
}

func TestStorageEngineConfigForResponse_RedactsS3Credentials(t *testing.T) {
	cfg := &StorageEngineConfig{
		DefaultProvider: "s3",
		S3: &S3EngineConfig{
			AccessKey: "ak", SecretKey: "sk", BucketName: "bucket", AddressingStyle: "virtual",
		},
	}
	masked := StorageEngineConfigForResponse(cfg, true)
	require.NotNil(t, masked.S3)
	assert.Equal(t, RedactedSecretPlaceholder, masked.S3.AccessKey)
	assert.Equal(t, RedactedSecretPlaceholder, masked.S3.SecretKey)
	assert.Equal(t, "bucket", masked.S3.BucketName)
	assert.Equal(t, "virtual", masked.S3.AddressingStyle)
	assert.Equal(t, "ak", cfg.S3.AccessKey, "masking must not mutate the stored config")
}
