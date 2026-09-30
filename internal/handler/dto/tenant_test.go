package dto

import (
	"encoding/json"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantResponse_ViewerOmitsSecrets(t *testing.T) {
	tenant := sampleSecretTenant()
	body, err := json.Marshal(NewTenantResponse(viewerContext(), tenant))
	require.NoError(t, err)
	s := string(body)
	assert.NotContains(t, s, "tenant-api-key-123")
	assert.NotContains(t, s, "wk-app-secret-def")
	assert.NotContains(t, s, "parser-secret-123")
	assert.NotContains(t, s, "s3-secret-789")
	assert.NotContains(t, s, "web_search_config")
	assert.NotContains(t, s, "parser_engine_config")
	assert.NotContains(t, s, "storage_engine_config")
	assert.NotContains(t, s, "credentials")
}

func TestTenantResponse_OwnerOmitsLegacyTenantAPIKey(t *testing.T) {
	tenant := sampleSecretTenant()
	body, err := json.Marshal(NewTenantResponse(ownerContext(), tenant))
	require.NoError(t, err)
	s := string(body)
	assert.NotContains(t, s, `"api_key"`)
	assert.NotContains(t, s, "parser-secret-123")
	assert.Contains(t, s, "web_search_config")
}

func TestTenantResponse_AdminGetsRedactedIntegrationConfigs(t *testing.T) {
	tenant := sampleSecretTenant()
	resp := NewTenantResponse(adminContext(), tenant)
	require.NotNil(t, resp.WebSearchConfig)
	assert.Equal(t, types.RedactedSecretPlaceholder, resp.WebSearchConfig.ProxyURL)
	require.NotNil(t, resp.ParserEngineConfig)
	assert.Equal(t, types.RedactedSecretPlaceholder, resp.ParserEngineConfig.MinerUAPIKey)
	require.NotNil(t, resp.StorageEngineConfig.S3)
	assert.Equal(t, types.RedactedSecretPlaceholder, resp.StorageEngineConfig.S3.SecretKey)
}

func TestTenantResponsesCrossTenant_RedactsEvenForOwnerContext(t *testing.T) {
	tenant := sampleSecretTenant()
	body, err := json.Marshal(NewTenantResponsesCrossTenant([]*types.Tenant{tenant}))
	require.NoError(t, err)
	s := string(body)
	assert.NotContains(t, s, "tenant-api-key-123")
	assert.NotContains(t, s, "parser-secret-123")
}

func sampleSecretTenant() *types.Tenant {
	return &types.Tenant{
		ID:   42,
		Name: "tenant",
		WebSearchConfig: &types.WebSearchConfig{
			ProxyURL: "http://proxy.internal:8080",
		},
		ParserEngineConfig: &types.ParserEngineConfig{
			MinerUAPIKey:          "parser-secret-123",
			PaddleOCRVLCloudToken: "paddle-secret-456",
		},
		StorageEngineConfig: &types.StorageEngineConfig{
			DefaultProvider: "s3",
			S3: &types.S3EngineConfig{
				AccessKey: "s3-access-id",
				SecretKey: "s3-secret-789",
			},
		},
	}
}
