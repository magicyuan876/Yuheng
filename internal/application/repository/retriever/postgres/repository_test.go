package postgres

import (
	"context"
	"testing"

	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
)

// seedIndexRow stores one embedding row the way indexing does, then applies the
// in-place updates FAQ code makes afterwards (disable, tag), which is how a row
// comes to be disabled or tagged at all.
func seedIndexRow(t *testing.T, db *gorm.DB, kbID, knowledgeID, chunkID string, enabled bool, tagID string) {
	t.Helper()
	row := &pgVector{
		SourceID:        chunkID,
		SourceType:      1,
		ChunkID:         chunkID,
		KnowledgeID:     knowledgeID,
		KnowledgeBaseID: kbID,
		Content:         "content of " + chunkID,
		Dimension:       3,
		Embedding:       pgvector.NewHalfVector([]float32{0.1, 0.2, 0.3}),
	}
	require.NoError(t, db.Create(row).Error)
	require.NoError(t, db.Exec(
		`UPDATE embeddings SET is_enabled = ?, tag_id = ? WHERE chunk_id = ?`, enabled, tagID, chunkID,
	).Error)
}

func indexRow(t *testing.T, db *gorm.DB, chunkID string) pgVector {
	t.Helper()
	var row pgVector
	require.NoError(t, db.Where("chunk_id = ?", chunkID).Take(&row).Error)
	return row
}

// A copy of a knowledge base must be a copy: a chunk that was switched off, or
// carries a tag, has to arrive that way. IsEnabled has `default:true`, so a
// struct built without it and inserted through GORM stores true, and a
// disabled FAQ entry would come back to life in the copy.
func TestCopyIndicesKeepsTheTagAndTheEnabledFlag(t *testing.T) {
	db := pgtest.New(t)
	repo := NewPostgresRetrieveEngineRepository(db)
	seedIndexRow(t, db, "kb-src", "kn-src", "chunk-on", true, "tag-a")
	seedIndexRow(t, db, "kb-src", "kn-src", "chunk-off", false, "tag-b")
	seedIndexRow(t, db, "kb-src", "kn-src", "chunk-untagged", false, "")

	err := repo.CopyIndices(context.Background(),
		"kb-src",
		map[string]string{"kn-src": "kn-dst"},
		map[string]string{"chunk-on": "dst-on", "chunk-off": "dst-off", "chunk-untagged": "dst-untagged"},
		"kb-dst", 3, "faq")
	require.NoError(t, err)

	on := indexRow(t, db, "dst-on")
	assert.True(t, on.IsEnabled)
	assert.Equal(t, "tag-a", on.TagID, "the tag comes along")

	off := indexRow(t, db, "dst-off")
	assert.False(t, off.IsEnabled, "a disabled chunk must not be re-enabled by copying it")
	assert.Equal(t, "tag-b", off.TagID)

	untagged := indexRow(t, db, "dst-untagged")
	assert.False(t, untagged.IsEnabled)
	assert.Empty(t, untagged.TagID)

	assert.Equal(t, "kb-dst", off.KnowledgeBaseID)
	assert.Equal(t, "kn-dst", off.KnowledgeID)

	// The source is left as it was.
	assert.False(t, indexRow(t, db, "chunk-off").IsEnabled)
}
