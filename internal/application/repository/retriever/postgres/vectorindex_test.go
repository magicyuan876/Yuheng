package postgres

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

// indexState reports whether the named index exists and whether it is valid.
func indexState(t *testing.T, db *gorm.DB, name string) (exists, valid bool) {
	t.Helper()
	var rows []bool
	require.NoError(t, db.Raw(
		`SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid WHERE c.relname = ?`, name,
	).Scan(&rows).Error)
	if len(rows) == 0 {
		return false, false
	}
	return true, rows[0]
}

// A dimension the migrations did not anticipate gets its index the first time
// vectors of that dimension are written, and searching them can then use it.
func TestEnsureBuildsTheIndexOfANewDimension(t *testing.T) {
	db := pgtest.New(t)
	idx := newVectorIndexes(db)

	exists, _ := indexState(t, db, vectorIndexName(768))
	require.False(t, exists, "precondition: no index for 768 yet")

	require.NoError(t, idx.Ensure(context.Background(), 768))

	exists, valid := indexState(t, db, vectorIndexName(768))
	assert.True(t, exists)
	assert.True(t, valid)

	// Again: nothing to do, and no error from the index existing.
	require.NoError(t, idx.Ensure(context.Background(), 768))
	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), 768),
		"a fresh process finds the index a previous one built")
}

func TestEnsureLeavesDimensionsTheMigrationsCovered(t *testing.T) {
	db := pgtest.New(t)

	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), 1024))

	exists, valid := indexState(t, db, vectorIndexName(1024))
	assert.True(t, exists)
	assert.True(t, valid)
}

// A concurrent build that was interrupted leaves an invalid index occupying the
// name; CREATE INDEX IF NOT EXISTS would skip it and search would stay unindexed.
func TestEnsureReplacesAnInvalidIndex(t *testing.T) {
	db := pgtest.New(t)
	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), 384))
	require.NoError(t, db.Exec(
		`UPDATE pg_index SET indisvalid = false WHERE indexrelid = ?::regclass`, vectorIndexName(384)).Error)
	_, valid := indexState(t, db, vectorIndexName(384))
	require.False(t, valid, "precondition: the index is now invalid")

	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), 384))

	exists, valid := indexState(t, db, vectorIndexName(384))
	assert.True(t, exists)
	assert.True(t, valid, "rebuilt")
}

// Vectors beyond what HNSW can index are stored anyway; refusing them would make
// the embedding model unusable.
func TestEnsureToleratesADimensionTooLargeToIndex(t *testing.T) {
	db := pgtest.New(t)

	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), maxIndexedDimension+96))

	exists, _ := indexState(t, db, vectorIndexName(maxIndexedDimension+96))
	assert.False(t, exists)
}

func TestEnsureIgnoresRowsWithoutAVector(t *testing.T) {
	db := pgtest.New(t)

	require.NoError(t, newVectorIndexes(db).Ensure(context.Background(), 0))
}

// Several writers meeting a new dimension at once build the index once and all
// see it usable.
func TestEnsureSharesOneBuildBetweenConcurrentCallers(t *testing.T) {
	db := pgtest.New(t)
	idx := newVectorIndexes(db)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = idx.Ensure(context.Background(), 1536)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}
	exists, valid := indexState(t, db, vectorIndexName(1536))
	assert.True(t, exists)
	assert.True(t, valid)
}

// The write path is what triggers the build, so a knowledge base whose model
// has an unusual dimension is searchable from its first document.
func TestBatchSaveBuildsTheIndexOfTheDimensionItWrites(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	vec := make([]float32, 512)
	vec[0] = 1

	err := repo.BatchSave(context.Background(), []*types.IndexInfo{{
		Content: "content", SourceID: "s1", SourceType: types.ChunkSourceType, ChunkID: "c1",
		KnowledgeID: "k1", KnowledgeBaseID: "kb1",
	}}, map[string]any{"embedding": map[string][]float32{"s1": vec}})
	require.NoError(t, err)

	exists, valid := indexState(t, db, vectorIndexName(512))
	assert.True(t, exists)
	assert.True(t, valid)
	var stored int64
	require.NoError(t, db.Model(&pgVector{}).Where("dimension = ?", 512).Count(&stored).Error)
	assert.EqualValues(t, 1, stored)
}
