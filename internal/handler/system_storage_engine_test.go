package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStorageBackendRepo is a minimal StorageBackendRepository used to exercise
// GetStorageEngineStatus' multi-instance awareness without a database.
type fakeStorageBackendRepo struct{ backends []*types.StorageBackend }

func (f *fakeStorageBackendRepo) Create(context.Context, *types.StorageBackend) error {
	return nil
}

func (f *fakeStorageBackendRepo) GetByID(context.Context, uint64, string) (*types.StorageBackend, error) {
	return nil, nil
}

func (f *fakeStorageBackendRepo) List(context.Context, uint64) ([]*types.StorageBackend, error) {
	return f.backends, nil
}
func (f *fakeStorageBackendRepo) Update(context.Context, *types.StorageBackend) error { return nil }
func (f *fakeStorageBackendRepo) Delete(context.Context, uint64, string) error        { return nil }
func (f *fakeStorageBackendRepo) FindLegacyAlias(context.Context, uint64, string) (*types.StorageBackend, error) {
	return nil, nil
}

func TestGetStorageEngineStatus_ListsOnlyLocalAndS3(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(storageallowlist.AllowListEnv, "")

	resp := getStorageEngineStatus(t, &SystemHandler{}, nil)

	names := make([]string, 0, len(resp.Data.Engines))
	byName := map[string]StorageEngineStatusItem{}
	for _, engine := range resp.Data.Engines {
		names = append(names, engine.Name)
		byName[engine.Name] = engine
	}
	assert.Equal(t, []string{"local", "s3"}, names)
	assert.Equal(t, []string{"local", "s3"}, resp.Data.AllowedProviders)
	assert.True(t, byName["local"].Available)
	assert.True(t, byName["s3"].Allowed)
	assert.False(t, byName["s3"].Available, "S3 is unavailable until configured")
}

func TestGetStorageEngineStatus_RespectsAllowList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(storageallowlist.AllowListEnv, "s3")

	resp := getStorageEngineStatus(t, &SystemHandler{}, nil)

	byName := map[string]StorageEngineStatusItem{}
	for _, engine := range resp.Data.Engines {
		byName[engine.Name] = engine
	}
	assert.False(t, byName["local"].Allowed)
	assert.True(t, byName["s3"].Allowed)
	assert.Equal(t, []string{"s3"}, resp.Data.AllowedProviders)
}

func TestGetStorageEngineStatus_S3ConfiguredFromTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(storageallowlist.AllowListEnv, "")

	tenant := &types.Tenant{
		StorageEngineConfig: &types.StorageEngineConfig{
			S3: &types.S3EngineConfig{
				Endpoint:   "http://rustfs.example.com:9000",
				Region:     "us-east-1",
				AccessKey:  "ak",
				SecretKey:  "sk",
				BucketName: "bucket",
			},
		},
	}
	resp := getStorageEngineStatus(t, &SystemHandler{}, tenant)

	for _, engine := range resp.Data.Engines {
		if engine.Name == "s3" {
			assert.True(t, engine.Available)
			return
		}
	}
	t.Fatal("s3 engine missing from the status response")
}

type storageEngineStatusResponse struct {
	Data struct {
		Engines          []StorageEngineStatusItem `json:"engines"`
		AllowedProviders []string                  `json:"allowed_providers"`
	} `json:"data"`
}

func getStorageEngineStatus(t *testing.T, h *SystemHandler, tenant *types.Tenant) storageEngineStatusResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/storage-engine-status", nil)
	if tenant != nil {
		c.Set(types.TenantInfoContextKey.String(), tenant)
	}

	h.GetStorageEngineStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp storageEngineStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// A workspace that configured S3 only through the new multi-instance Storage
// settings (storage_backends), with an empty legacy StorageEngineConfig, must
// still be reported as available.
func TestGetStorageEngineStatus_AvailableFromActiveBackend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(storageallowlist.AllowListEnv, "")

	h := &SystemHandler{storageBackendRepo: &fakeStorageBackendRepo{backends: []*types.StorageBackend{
		{Provider: "s3", Status: types.StorageBackendStatusActive},
	}}}
	resp := getStorageEngineStatus(t, h, &types.Tenant{ID: 42})

	status := map[string]bool{}
	for _, engine := range resp.Data.Engines {
		status[engine.Name] = engine.Available
	}
	assert.True(t, status["s3"], "active S3 backend should be available")
}

func TestGetStorageEngineStatus_DisabledBackendDoesNotMakeS3Available(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(storageallowlist.AllowListEnv, "")

	h := &SystemHandler{storageBackendRepo: &fakeStorageBackendRepo{backends: []*types.StorageBackend{
		{Provider: "s3", Status: types.StorageBackendStatusDisabled},
	}}}
	resp := getStorageEngineStatus(t, h, &types.Tenant{ID: 42})

	for _, engine := range resp.Data.Engines {
		if engine.Name == "s3" {
			assert.False(t, engine.Available)
		}
	}
}
