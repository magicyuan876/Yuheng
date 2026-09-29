package retriever

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/application/repository/retriever/postgres"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// PostgresEnvStoreID identifies the virtual store that RETRIEVE_DRIVER=postgres
// implies.
const PostgresEnvStoreID = types.EnvStoreIDPrefix + "postgres__"

// PostgresDescriptor describes the built-in engine: the application's own
// PostgreSQL (ParadeDB for keyword search, pgvector for vectors).
//
// It is not registrable. The embeddings table has a fixed name and no
// per-store partitioning, so a second "store" on the same server would not
// separate anything; the engine is the environment's, bound to the
// application's own connection.
func PostgresDescriptor() EngineDescriptor {
	return EngineDescriptor{
		Type:        types.PostgresRetrieverEngineType,
		Driver:      "postgres",
		DisplayName: "PostgreSQL",
		Capabilities: EngineCapabilities{
			Retrievers: []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType},
			// pgvector reports (1 - cosine distance), which stays in [0, 1]
			// for the L2-normalised embeddings retrieval models produce.
			ScoreScale: ScoreUnit,
			// Declared, not yet enforced: the embeddings table carries no
			// subjects and the repository ignores RetrieveParams.Subjects.
			SupportsACL: true,
		},
		DefaultIndexName: "embeddings",
		// The engine lives in the application's own database, so it is
		// reachable whenever the application is running.
		TestConnection: func(context.Context, types.ConnectionConfig) (string, error) { return "", nil },
		EnvStore: func(types.EnvLookupFunc) *types.VectorStore {
			return &types.VectorStore{
				ID:               PostgresEnvStoreID,
				Name:             "PostgreSQL",
				EngineType:       types.PostgresRetrieverEngineType,
				ConnectionConfig: types.ConnectionConfig{UseDefaultConnection: true},
			}
		},
		New: func(
			_ context.Context, store types.VectorStore, deps EngineDeps,
		) (interfaces.RetrieveEngineService, error) {
			if !store.ConnectionConfig.UseDefaultConnection {
				return nil, errors.New("postgres stores must use the application's own connection")
			}
			repo := postgres.NewPostgresRetrieveEngineRepository(deps.DB)
			return NewKVHybridRetrieveEngine(repo, types.PostgresRetrieverEngineType), nil
		},
	}
}
