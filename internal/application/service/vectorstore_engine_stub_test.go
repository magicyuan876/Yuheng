package service

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// The public build ships a single engine, PostgreSQL, which is not
// registrable. The store-registration, address-policy and score-normalisation
// logic is engine-independent, so its tests run against stub engines described
// here instead of against any real driver. Keeping one shared definition means
// a change to the descriptor contract breaks one file, not a dozen tests.

const (
	// stubEngineType is a registrable engine whose scores are already in [0, 1].
	stubEngineType types.RetrieverEngineType = "stub"
	// signedEngineType is the same engine but reporting the raw signed cosine,
	// which keeps the [-1, 1] normalisation branch under coverage.
	signedEngineType types.RetrieverEngineType = "signed"
)

// stubDescriptor builds a registrable engine descriptor. Addr is its only
// required connection field, and it is also the only address it "dials", so
// the SSRF policy has exactly one place to be exercised.
func stubDescriptor(engineType types.RetrieverEngineType, scale retriever.ScoreScale) retriever.EngineDescriptor {
	driver := string(engineType)
	return retriever.EngineDescriptor{
		Type:        engineType,
		Driver:      driver,
		DisplayName: "Stub " + driver,
		Capabilities: retriever.EngineCapabilities{
			Retrievers: []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType},
			ScoreScale: scale,
		},
		Registrable:      true,
		DefaultIndexName: driver + "_default",
		ValidateConnection: func(cc types.ConnectionConfig) error {
			if cc.Addr == "" {
				return errors.New("addr is required")
			}
			return nil
		},
		DialAddresses: func(cc types.ConnectionConfig) []string { return []string{cc.Addr} },
		// A fixed answer keeps CreateStore's connection probe off the network.
		TestConnection: func(context.Context, types.ConnectionConfig) (string, error) { return "1.0", nil },
		EnvStore: func(env types.EnvLookupFunc) *types.VectorStore {
			addr := env("STUB_ADDR")
			if addr == "" {
				return nil
			}
			return &types.VectorStore{
				ID:               types.EnvStoreIDPrefix + driver + "__",
				Name:             "Env " + driver,
				EngineType:       engineType,
				ConnectionConfig: types.ConnectionConfig{Addr: addr},
				IndexConfig:      types.IndexConfig{IndexName: env("STUB_INDEX")},
			}
		},
		New: func(context.Context, types.VectorStore, retriever.EngineDeps) (interfaces.RetrieveEngineService, error) {
			return &fakeRetrieveEngineService{
				engineType: engineType,
				support:    []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType},
			}, nil
		},
	}
}

// newTestCatalog returns the catalog the service tests run against: the real
// PostgreSQL engine plus the two stubs.
func newTestCatalog(t *testing.T) *retriever.Catalog {
	t.Helper()
	catalog, err := retriever.NewCatalog(
		retriever.PostgresDescriptor(),
		stubDescriptor(stubEngineType, retriever.ScoreUnit),
		stubDescriptor(signedEngineType, retriever.ScoreSignedCosine),
	)
	require.NoError(t, err)
	return catalog
}
