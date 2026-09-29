package retriever

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Stub engine types for tests that need a catalog without a real driver.
const (
	stubSignedType types.RetrieverEngineType = "signed"
	stubUnitType   types.RetrieverEngineType = "unit"
)

// stubDescriptor builds a complete, non-registrable descriptor whose
// constructor is never expected to run.
func stubDescriptor(t types.RetrieverEngineType, scale ScoreScale) EngineDescriptor {
	return EngineDescriptor{
		Type:        t,
		Driver:      string(t),
		DisplayName: "Stub " + string(t),
		Capabilities: EngineCapabilities{
			Retrievers: []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType},
			ScoreScale: scale,
		},
		DefaultIndexName: "stub_" + string(t),
		EnvStore: func(types.EnvLookupFunc) *types.VectorStore {
			return &types.VectorStore{ID: types.EnvStoreIDPrefix + string(t) + "__", Name: string(t), EngineType: t}
		},
		New: func(context.Context, types.VectorStore, EngineDeps) (interfaces.RetrieveEngineService, error) {
			return nil, nil
		},
	}
}

// stubRegistrableDescriptor is a stubDescriptor a workspace may register
// stores of. dial is what the driver would connect to.
func stubRegistrableDescriptor(
	t types.RetrieverEngineType, dial func(types.ConnectionConfig) []string,
) EngineDescriptor {
	d := stubDescriptor(t, ScoreUnit)
	d.Registrable = true
	d.DialAddresses = dial
	d.TestConnection = func(context.Context, types.ConnectionConfig) (string, error) { return "1.0", nil }
	return d
}

// stubCatalog builds a catalog of the two score-scale stubs and PostgreSQL.
func stubCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := NewCatalog(
		PostgresDescriptor(),
		stubDescriptor(stubSignedType, ScoreSignedCosine),
		stubDescriptor(stubUnitType, ScoreUnit),
	)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return c
}
