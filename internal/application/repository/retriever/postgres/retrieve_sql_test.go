package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

// The retrieval SQL is hand-written: pgvector's cosine operator behind a
// partial HNSW index for vectors, ParadeDB's BM25 operator for keywords, and a
// pile of optional filters glued on as strings. Its only earlier tests used one
// three-number vector in one knowledge base, so nothing pinned the properties a
// user relies on: the nearest chunk comes first, a query for one knowledge base
// never returns another's chunks, a disabled chunk stays hidden, and a Chinese
// phrase is found.
//
// The embeddings table has no tenant column: tenants are separated by the
// knowledge-base IDs the caller passes, and that is the isolation asserted
// here. "Tenant 1" owns kb-a, "tenant 2" owns kb-b; the two hold deliberately
// identical text and vectors, so a filter that leaks shows up as a wrong ID
// rather than as an unrelated one.
func TestRetrievalSQL(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	ctx := context.Background()

	type seed struct {
		chunk, kn, kb, tag, content string
		enabled                     bool
		vec                         []float32
	}
	seeds := []seed{
		{"a1", "kn-a1", "kb-a", "t1", "quarterly storage quota policy for every team", true, []float32{1, 0, 0, 0}},
		{"a2", "kn-a1", "kb-a", "t2", "空间的存储配额由管理员设置 quota note", true, []float32{0.9, 0.1, 0, 0}},
		{"a3", "kn-a2", "kb-a", "t1", "unrelated onboarding checklist", true, []float32{0, 1, 0, 0}},
		{"a4", "kn-a2", "kb-a", "t1", "quota entry that was switched off", false, []float32{1, 0, 0, 0}},
		// Same text and vector as a1, in another tenant's knowledge base.
		{"b1", "kn-b1", "kb-b", "t1", "quarterly storage quota policy for every team", true, []float32{1, 0, 0, 0}},
		// A different embedding model's dimension in kb-a.
		{"d3", "kn-a3", "kb-a", "t1", "three dimensional vector quota", true, []float32{1, 0, 0}},
	}
	for _, s := range seeds {
		err := repo.BatchSave(ctx, []*types.IndexInfo{{
			SourceID: s.chunk, SourceType: 1, ChunkID: s.chunk, KnowledgeID: s.kn, KnowledgeBaseID: s.kb,
			TagID: s.tag, Content: s.content, IsEnabled: s.enabled,
		}}, map[string]any{"embedding": map[string][]float32{s.chunk: s.vec}})
		require.NoError(t, err, s.chunk)
	}

	// hits returns the chunk IDs in result order and the scores.
	hits := func(t *testing.T, p types.RetrieveParams) ([]string, []float64) {
		t.Helper()
		res, err := repo.Retrieve(ctx, p)
		require.NoError(t, err)
		var ids []string
		var scores []float64
		for _, r := range res {
			for _, item := range r.Results {
				ids = append(ids, item.ChunkID)
				scores = append(scores, item.Score)
			}
		}
		return ids, scores
	}

	inKBs := func(kbs ...string) func(*types.RetrieveParams) {
		return func(p *types.RetrieveParams) { p.KnowledgeBaseIDs = kbs }
	}
	inKnowledge := func(ids ...string) func(*types.RetrieveParams) {
		return func(p *types.RetrieveParams) { p.KnowledgeIDs = ids }
	}
	vector := func(vec []float32, mut func(*types.RetrieveParams)) types.RetrieveParams {
		p := types.RetrieveParams{
			Embedding: vec, TopK: 10, RetrieverType: types.VectorRetrieverType,
			KnowledgeBaseIDs: []string{"kb-a"},
		}
		if mut != nil {
			mut(&p)
		}
		return p
	}
	keywords := func(query string, mut func(*types.RetrieveParams)) types.RetrieveParams {
		p := types.RetrieveParams{
			Query: query, TopK: 10, RetrieverType: types.KeywordsRetrieverType,
			KnowledgeBaseIDs: []string{"kb-a"},
		}
		if mut != nil {
			mut(&p)
		}
		return p
	}

	t.Run("vector: nearest first, scores in range and descending", func(t *testing.T) {
		ids, scores := hits(t, vector([]float32{1, 0, 0, 0}, nil))
		// a1 is the query itself, a2 close, a3 orthogonal; a4 is disabled, d3 has
		// another dimension and b1 belongs to another knowledge base.
		assert.Equal(t, []string{"a1", "a2", "a3"}, ids)
		require.Len(t, scores, 3)
		assert.InDelta(t, 1.0, scores[0], 0.01, "an identical vector scores about 1")
		for i, s := range scores {
			assert.GreaterOrEqual(t, s, 0.0, "score %d", i)
			assert.LessOrEqual(t, s, 1.0+1e-3, "score %d", i)
			if i > 0 {
				assert.LessOrEqual(t, s, scores[i-1], "scores must not increase down the list")
			}
		}
		assert.InDelta(t, 0.0, scores[2], 0.01, "an orthogonal vector scores about 0")
	})

	t.Run("vector: the query picks the neighbour, not the insertion order", func(t *testing.T) {
		ids, _ := hits(t, vector([]float32{0, 1, 0, 0}, nil))
		require.NotEmpty(t, ids)
		assert.Equal(t, "a3", ids[0])
	})

	t.Run("vector: threshold and top-k", func(t *testing.T) {
		ids, _ := hits(t, vector([]float32{1, 0, 0, 0}, func(p *types.RetrieveParams) { p.Threshold = 0.5 }))
		assert.Equal(t, []string{"a1", "a2"}, ids, "the orthogonal chunk is below the threshold")

		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, func(p *types.RetrieveParams) { p.TopK = 1 }))
		assert.Equal(t, []string{"a1"}, ids)
	})

	t.Run("vector: knowledge base isolation", func(t *testing.T) {
		ids, _ := hits(t, vector([]float32{1, 0, 0, 0}, nil))
		assert.NotContains(t, ids, "b1", "a query scoped to kb-a returned kb-b's chunk")

		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, inKBs("kb-b")))
		assert.Equal(t, []string{"b1"}, ids)

		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, inKBs("kb-nobody")))
		assert.Empty(t, ids)
	})

	t.Run("vector: knowledge and tag filters", func(t *testing.T) {
		ids, _ := hits(t, vector([]float32{1, 0, 0, 0}, inKnowledge("kn-a2")))
		assert.Equal(t, []string{"a3"}, ids, "kn-a2 also holds the disabled a4, which stays hidden")

		// Knowledge base and knowledge IDs are ANDed: a document of another
		// knowledge base is not reachable by naming it.
		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, inKnowledge("kn-b1")))
		assert.Empty(t, ids)

		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, func(p *types.RetrieveParams) { p.TagIDs = []string{"t2"} }))
		assert.Equal(t, []string{"a2"}, ids)
	})

	t.Run("vector: a disabled chunk is never returned", func(t *testing.T) {
		ids, _ := hits(t, vector([]float32{1, 0, 0, 0}, nil))
		assert.NotContains(t, ids, "a4")
		// ... including when it is the exact match and named directly.
		ids, _ = hits(t, vector([]float32{1, 0, 0, 0}, func(p *types.RetrieveParams) {
			p.KnowledgeIDs = []string{"kn-a2"}
			p.TagIDs = []string{"t1"}
		}))
		assert.NotContains(t, ids, "a4")
	})

	t.Run("vector: dimensions do not mix and a mismatch is not an error", func(t *testing.T) {
		// Only the 3-dimensional chunk is comparable with a 3-dimensional query.
		ids, _ := hits(t, vector([]float32{1, 0, 0}, nil))
		assert.Equal(t, []string{"d3"}, ids)

		// No chunk has 5 dimensions: an empty answer, not a failure (a model
		// swapped under a knowledge base must degrade to "nothing found").
		ids, _ = hits(t, vector([]float32{1, 0, 0, 0, 0}, nil))
		assert.Empty(t, ids)
	})

	t.Run("keywords: BM25 finds English and Chinese text", func(t *testing.T) {
		ids, scores := hits(t, keywords("onboarding", nil))
		assert.Equal(t, []string{"a3"}, ids)
		require.Len(t, scores, 1)
		assert.Greater(t, scores[0], 0.0)

		ids, _ = hits(t, keywords("存储配额", nil))
		assert.Contains(t, ids, "a2", "the Chinese phrase must be found (chinese_lindera tokenizer)")

		ids, scores = hits(t, keywords("quota", nil))
		assert.ElementsMatch(t, []string{"a1", "a2", "d3"}, ids, "every enabled kb-a chunk that says quota")
		for i := 1; i < len(scores); i++ {
			assert.LessOrEqual(t, scores[i], scores[i-1], "keyword scores must not increase down the list")
		}
	})

	t.Run("keywords: isolation and filters", func(t *testing.T) {
		ids, _ := hits(t, keywords("quarterly", nil))
		assert.Equal(t, []string{"a1"}, ids, "kb-b holds the same sentence and must not appear")

		ids, _ = hits(t, keywords("quarterly", inKBs("kb-b")))
		assert.Equal(t, []string{"b1"}, ids)

		ids, _ = hits(t, keywords("quota", func(p *types.RetrieveParams) { p.KnowledgeIDs = []string{"kn-a1"} }))
		assert.ElementsMatch(t, []string{"a1", "a2"}, ids)

		ids, _ = hits(t, keywords("quota", inKnowledge("kn-b1")))
		assert.Empty(t, ids, "knowledge base and knowledge IDs are ANDed")

		ids, _ = hits(t, keywords("quota", func(p *types.RetrieveParams) { p.TagIDs = []string{"t2"} }))
		assert.Equal(t, []string{"a2"}, ids)

		ids, _ = hits(t, keywords("switched", nil))
		assert.Empty(t, ids, "the only chunk saying this is disabled")

		ids, _ = hits(t, keywords("quota", func(p *types.RetrieveParams) { p.TopK = 1 }))
		assert.Len(t, ids, 1)

		ids, _ = hits(t, keywords("nonexistentterm", nil))
		assert.Empty(t, ids)
	})

	t.Run("an unknown retriever type is refused", func(t *testing.T) {
		_, err := repo.Retrieve(ctx, types.RetrieveParams{RetrieverType: "telepathy"})
		assert.Error(t, err)
	})
}

// A chunk indexed as disabled must be stored as disabled, through both write
// paths. is_enabled has `default:true` and GORM turns a false struct field into
// that default, so an FAQ entry that was switched off and then re-indexed (old
// rows deleted, new rows saved with the chunk's flag) came back searchable.
func TestSaveKeepsTheDisabledFlag(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	ctx := context.Background()
	emb := func(id string) map[string]any {
		return map[string]any{"embedding": map[string][]float32{id: {1, 0, 0, 0}}}
	}
	info := func(id string, enabled bool) *types.IndexInfo {
		return &types.IndexInfo{
			SourceID: id, SourceType: 1, ChunkID: id, KnowledgeID: "kn", KnowledgeBaseID: "kb",
			Content: "content " + id, IsEnabled: enabled,
		}
	}

	require.NoError(t, repo.Save(ctx, info("save-off", false), emb("save-off")))
	require.NoError(t, repo.Save(ctx, info("save-on", true), emb("save-on")))
	require.NoError(t, repo.BatchSave(ctx, []*types.IndexInfo{info("batch-off", false), info("batch-on", true)},
		map[string]any{"embedding": map[string][]float32{"batch-off": {1, 0, 0, 0}, "batch-on": {1, 0, 0, 0}}}))

	for id, want := range map[string]bool{"save-off": false, "save-on": true, "batch-off": false, "batch-on": true} {
		assert.Equal(t, want, indexRow(t, db, id).IsEnabled, id)
	}
}
