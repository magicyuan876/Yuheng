package postgres

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

// The duplicate detector reads nothing but this query, so what it may and may
// not return is pinned against a real ParadeDB: the nearest chunk of another
// document comes back with its similarity, the threshold holds, and neither
// the document's own chunks, nor another knowledge base, nor a disabled chunk,
// nor a generated-question row, nor a vector of another dimension ever does.
func TestSimilarChunksSQL(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	finder, ok := repo.(interface {
		SimilarChunks(context.Context, string, string, float64, int) ([]types.ChunkSimilarity, error)
	})
	require.True(t, ok, "the postgres engine compares its stored vectors")
	ctx := context.Background()

	type seed struct {
		source, chunk, kn, kb string
		enabled               bool
		vec                   []float32
	}
	seeds := []seed{
		// The document asked about: two copies of one passage, one distinct
		// passage, and a passage embedded by an older, 3-dimensional model.
		{"c1", "c1", "kn-1", "kb-a", true, []float32{1, 0, 0, 0}},
		{"c4", "c4", "kn-1", "kb-a", true, []float32{1, 0, 0, 0}},
		{"c2", "c2", "kn-1", "kb-a", true, []float32{0, 1, 0, 0}},
		{"c3", "c3", "kn-1", "kb-a", true, []float32{1, 0, 0}},
		// Another document of the base: an exact and a close copy of c1.
		{"d1", "d1", "kn-2", "kb-a", true, []float32{1, 0, 0, 0}},
		{"d2", "d2", "kn-2", "kb-a", true, []float32{0.9, 0.1, 0, 0}},
		// A switched-off copy.
		{"e1", "e1", "kn-3", "kb-a", true, []float32{1, 0, 0, 0}},
		// A generated question indexed under its chunk: same vector as c2,
		// but not a passage of any document.
		{"q9-question", "q9", "kn-4", "kb-a", true, []float32{0, 1, 0, 0}},
		// The same passage in another tenant's knowledge base.
		{"f1", "f1", "kn-5", "kb-b", true, []float32{1, 0, 0, 0}},
		// A 3-dimensional neighbour of c3.
		{"g1", "g1", "kn-6", "kb-a", true, []float32{1, 0, 0}},
	}
	for _, s := range seeds {
		err := repo.BatchSave(ctx, []*types.IndexInfo{{
			SourceID: s.source, SourceType: types.ChunkSourceType, ChunkID: s.chunk, KnowledgeID: s.kn,
			KnowledgeBaseID: s.kb, Content: "passage " + s.source, IsEnabled: s.enabled,
		}}, map[string]any{"embedding": map[string][]float32{s.source: s.vec}})
		require.NoError(t, err, s.source)
	}
	require.NoError(t, repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{"e1": false}))

	pairs := func(t *testing.T, minScore float64, perChunk int) ([]string, map[string]float64) {
		t.Helper()
		rows, err := finder.SimilarChunks(ctx, "kb-a", "kn-1", minScore, perChunk)
		require.NoError(t, err)
		keys := make([]string, 0, len(rows))
		scores := map[string]float64{}
		for _, r := range rows {
			key := fmt.Sprintf("%s>%s@%s", r.SubjectChunkID, r.RelatedChunkID, r.RelatedKnowledgeID)
			keys = append(keys, key)
			scores[key] = r.Score
		}
		sort.Strings(keys)
		return keys, scores
	}

	got, scores := pairs(t, 0.95, 5)
	assert.Equal(t, []string{
		"c1>d1@kn-2", "c1>d2@kn-2",
		"c3>g1@kn-6",
		"c4>d1@kn-2", "c4>d2@kn-2",
	}, got, "own chunks, other bases, disabled chunks, question rows and other dimensions stay out")
	assert.InDelta(t, 1.0, scores["c1>d1@kn-2"], 1e-3, "score is 1 - cosine distance")
	assert.InDelta(t, 0.9939, scores["c1>d2@kn-2"], 2e-3)

	// The threshold is applied to the similarity, inclusively of the exact
	// copies and exclusively of the close one.
	got, _ = pairs(t, 0.999, 5)
	assert.Equal(t, []string{"c1>d1@kn-2", "c3>g1@kn-6", "c4>d1@kn-2"}, got)

	// perChunk keeps the nearest neighbours only.
	got, _ = pairs(t, 0.5, 1)
	assert.Equal(t, []string{"c1>d1@kn-2", "c3>g1@kn-6", "c4>d1@kn-2"}, got)

	// A document with no vectors, and one nobody asked about, answer nothing.
	rows, err := finder.SimilarChunks(ctx, "kb-a", "no-such-knowledge", 0.5, 5)
	require.NoError(t, err)
	assert.Empty(t, rows)
	rows, err = finder.SimilarChunks(ctx, "kb-b", "kn-1", 0.5, 5)
	require.NoError(t, err)
	assert.Empty(t, rows, "the knowledge base is part of the question")
}
