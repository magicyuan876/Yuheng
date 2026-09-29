package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

func retrievedChunks(t *testing.T, results []*types.RetrieveResult) []string {
	t.Helper()
	var ids []string
	for _, r := range results {
		for _, item := range r.Results {
			ids = append(ids, item.ChunkID)
		}
	}
	return ids
}

// RetrieveParams.Subjects is declared but not yet enforced by this engine. The
// test pins that a release which adds the field changes nothing: an unset
// Subjects, an empty one and a populated one all return the same chunks. When
// the engine starts filtering, this test must be replaced by one that says so.
func TestVectorRetrieveIgnoresSubjectsForNow(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	seedIndexRow(t, db, "kb-1", "kn-1", "chunk-a", true, "")
	seedIndexRow(t, db, "kb-1", "kn-1", "chunk-b", true, "")

	base := types.RetrieveParams{
		Embedding:        []float32{0.1, 0.2, 0.3},
		KnowledgeBaseIDs: []string{"kb-1"},
		TopK:             10,
		Threshold:        0,
		RetrieverType:    types.VectorRetrieverType,
	}

	without, err := repo.Retrieve(context.Background(), base)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"chunk-a", "chunk-b"}, retrievedChunks(t, without))

	for name, subjects := range map[string][]string{
		"empty":     {},
		"populated": {"user:1", "group:9"},
	} {
		p := base
		p.Subjects = subjects
		got, err := repo.Retrieve(context.Background(), p)
		require.NoError(t, err, name)
		assert.ElementsMatch(t, retrievedChunks(t, without), retrievedChunks(t, got), name)
	}
}
