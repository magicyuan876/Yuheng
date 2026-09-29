package service

import (
	"context"
	"os"
	"testing"

	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSSRFWhitelist sets SSRF_WHITELIST for the duration of a test, resetting
// the cached singleton both before and after so neither this test nor a
// neighbour sees a stale whitelist (see ResetSSRFWhitelistForTest docs).
func withSSRFWhitelist(t *testing.T, whitelist string) {
	t.Helper()
	utils.ResetSSRFWhitelistForTest()
	require.NoError(t, os.Setenv("SSRF_WHITELIST", whitelist))
	t.Cleanup(func() {
		_ = os.Unsetenv("SSRF_WHITELIST")
		utils.ResetSSRFWhitelistForTest()
	})
}

func TestValidateStoreConfig_SSRF(t *testing.T) {
	// Whitelist a benign host so "pass" cases have a deterministic, DNS-free
	// way through. Reject cases use direct private IPs, which are blocked
	// before any DNS lookup, keeping the test non-flaky.
	withSSRFWhitelist(t, "vector.allowed.test")
	engine := stubDescriptor(stubEngineType, retriever.ScoreUnit)

	tests := []struct {
		name      string
		config    types.ConnectionConfig
		wantError bool
	}{
		{name: "private IP blocked", config: types.ConnectionConfig{Addr: "http://10.0.0.5:9200"}, wantError: true},
		{name: "loopback blocked", config: types.ConnectionConfig{Addr: "http://127.0.0.1:9200"}, wantError: true},
		{
			name:      "link-local metadata blocked",
			config:    types.ConnectionConfig{Addr: "169.254.169.254:9200"},
			wantError: true,
		},
		{name: "192.168 blocked", config: types.ConnectionConfig{Addr: "192.168.1.10:9030"}, wantError: true},
		{name: "whitelisted host allowed", config: types.ConnectionConfig{Addr: "http://vector.allowed.test:9200"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStoreConfig(engine, tt.config, types.IndexConfig{})
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateStoreConfig_WhitelistSkipsPortBlock pins the intentional
// behaviour that a whitelisted host bypasses the port blocklist (whitelist
// trust is host-granular). If this ever changes, the bundled-service defaults
// in docker-compose would silently break, so the decision is asserted here.
func TestValidateStoreConfig_WhitelistSkipsPortBlock(t *testing.T) {
	withSSRFWhitelist(t, "stubhost")
	engine := stubDescriptor(stubEngineType, retriever.ScoreUnit)
	// 6379 (redis) is on the blocklist, but a whitelisted host skips all
	// checks including the port block.
	err := validateStoreConfig(engine, types.ConnectionConfig{Addr: "stubhost:6379"}, types.IndexConfig{})
	require.NoError(t, err)
}

// TestRegistrableEngine_FailsClosed pins that only an engine the catalog
// describes as registrable can be reached by user input. Anything else,
// including a type nobody has heard of, is refused instead of being probed
// with a default policy.
func TestRegistrableEngine_FailsClosed(t *testing.T) {
	svc := NewVectorStoreService(&mockVectorStoreRepo{}, nil, nil, nil, nil, newTestCatalog(t)).(*vectorStoreService)

	_, err := svc.registrableEngine(stubEngineType)
	require.NoError(t, err)

	for _, et := range []types.RetrieverEngineType{
		types.PostgresRetrieverEngineType, // real but not registrable
		"some-future-engine",              // unknown to the catalog
		"",
	} {
		_, err := svc.registrableEngine(et)
		require.Errorf(t, err, "engine %q must not be registrable", et)
	}
}

func TestTestRawConnection_Rejections(t *testing.T) {
	withSSRFWhitelist(t, "vector.allowed.test")
	repo := &mockVectorStoreRepo{}
	svc := NewVectorStoreService(repo, nil, nil, nil, nil, newTestCatalog(t))

	tests := []struct {
		name       string
		engineType types.RetrieverEngineType
		config     types.ConnectionConfig
	}{
		{
			// postgres is not a user-registerable vector store; raw-testing it
			// would otherwise dial the app's own DB host (credential oracle).
			name:       "postgres rejected by engine allowlist",
			engineType: types.PostgresRetrieverEngineType,
			config:     types.ConnectionConfig{Addr: "postgres://u:p@vector.allowed.test:5432/db"},
		},
		{
			name:       "unknown engine rejected",
			engineType: "some-future-engine",
			config:     types.ConnectionConfig{Addr: "http://vector.allowed.test"},
		},
		{
			// empty addr must be rejected by required-field validation before
			// a driver can fall back to a localhost default.
			name:       "empty addr rejected (no localhost fallback)",
			engineType: stubEngineType,
			config:     types.ConnectionConfig{},
		},
		{
			name:       "private IP rejected by SSRF",
			engineType: stubEngineType,
			config:     types.ConnectionConfig{Addr: "http://10.0.0.5:9200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.TestRawConnection(context.Background(), tt.engineType, tt.config)
			require.Error(t, err)
		})
	}
}

func TestTestRawConnection_ReturnsVersion(t *testing.T) {
	withSSRFWhitelist(t, "vector.allowed.test")
	svc := NewVectorStoreService(&mockVectorStoreRepo{}, nil, nil, nil, nil, newTestCatalog(t))

	version, err := svc.TestRawConnection(context.Background(), stubEngineType,
		types.ConnectionConfig{Addr: "http://vector.allowed.test:9200"})
	require.NoError(t, err)
	assert.Equal(t, "1.0", version)
}

func TestCreateStore_SSRFRejected(t *testing.T) {
	withSSRFWhitelist(t, "vector.allowed.test")
	repo := &mockVectorStoreRepo{}
	svc := NewVectorStoreService(repo, nil, nil, nil, nil, newTestCatalog(t))

	store := &types.VectorStore{
		TenantID:   1,
		Name:       "stub-internal",
		EngineType: stubEngineType,
		ConnectionConfig: types.ConnectionConfig{
			Addr: "http://169.254.169.254:9200", // cloud metadata endpoint
		},
	}

	err := svc.CreateStore(context.Background(), store)
	require.Error(t, err)
	// Rejected before persistence and before any connection probe.
	assert.Empty(t, repo.stores)
}
