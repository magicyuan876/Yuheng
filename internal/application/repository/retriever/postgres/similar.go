package postgres

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Comparing the vectors already stored, for the knowledge-health detectors.
//
// A duplicate check asks, for every chunk of one document, which chunks of the
// other documents of its knowledge base lie closest. That is one nearest-
// neighbour search per chunk, and it is answered here as one statement per
// vector dimension: the chunk rows of the document, each joined LATERAL to its
// nearest neighbours. The neighbour search uses exactly the expression the
// per-dimension HNSW index is built on (see vectorindex.go), because pgvector
// only uses an index whose expression matches the ORDER BY verbatim.

const (
	// defaultSimilarPerChunk is how many neighbours each chunk looks at when
	// the caller does not say.
	defaultSimilarPerChunk = 5
	// maxSimilarPerChunk bounds the neighbour list. HNSW needs ef_search to
	// be at least the LIMIT, and a long list makes every search walk more of
	// the graph for pairs that fall under any sensible threshold anyway.
	maxSimilarPerChunk = 20
	// maxSimilarSubjectRows bounds how many chunks of one document are
	// compared. A document beyond this is compared by its first chunks (in
	// insertion order); the cost of the check is linear in it, and a document
	// this long whose first few thousand passages match nothing is not a
	// duplicate of anything.
	maxSimilarSubjectRows = 5000
)

var _ interfaces.SimilarChunkFinder = (*pgRepository)(nil)

// SimilarChunks implements interfaces.SimilarChunkFinder.
func (g *pgRepository) SimilarChunks(ctx context.Context, kbID, knowledgeID string, minScore float64,
	perChunk int,
) ([]types.ChunkSimilarity, error) {
	if kbID == "" || knowledgeID == "" {
		return nil, nil
	}
	if perChunk <= 0 {
		perChunk = defaultSimilarPerChunk
	}
	if perChunk > maxSimilarPerChunk {
		perChunk = maxSimilarPerChunk
	}

	// A knowledge base normally uses one embedding model, but changing the
	// model leaves rows of the old dimension until the documents are
	// re-indexed. Vectors of different dimensions cannot be compared, so each
	// dimension the document has is searched on its own.
	var dimensions []int
	if err := g.db.WithContext(ctx).Raw(`
		SELECT DISTINCT dimension FROM embeddings
		WHERE knowledge_base_id = ? AND knowledge_id = ? AND source_id = chunk_id AND dimension > 0
		  AND (is_enabled IS NULL OR is_enabled = TRUE)`,
		kbID, knowledgeID,
	).Scan(&dimensions).Error; err != nil {
		return nil, fmt.Errorf("listing the vector dimensions of knowledge %s: %w", knowledgeID, err)
	}

	var out []types.ChunkSimilarity
	for _, dimension := range dimensions {
		rows, err := g.similarChunksOfDimension(ctx, kbID, knowledgeID, dimension, minScore, perChunk)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func (g *pgRepository) similarChunksOfDimension(ctx context.Context, kbID, knowledgeID string, dimension int,
	minScore float64, perChunk int,
) ([]types.ChunkSimilarity, error) {
	// The dimension is an integer from the table, formatted by %d; everything
	// else is a bound parameter. source_id = chunk_id keeps the chunk rows
	// and drops the rows that index a generated question under its chunk.
	query := fmt.Sprintf(`
		SELECT s.chunk_id AS subject_chunk_id,
		       n.chunk_id AS related_chunk_id,
		       n.knowledge_id AS related_knowledge_id,
		       1 - n.distance AS score
		FROM (
			SELECT chunk_id, embedding FROM embeddings
			WHERE knowledge_base_id = ? AND knowledge_id = ? AND dimension = %[1]d
			  AND source_id = chunk_id AND (is_enabled IS NULL OR is_enabled = TRUE)
			ORDER BY id
			LIMIT ?
		) AS s
		CROSS JOIN LATERAL (
			SELECT e.chunk_id, e.knowledge_id,
			       e.embedding::halfvec(%[1]d) <=> s.embedding::halfvec(%[1]d) AS distance
			FROM embeddings e
			WHERE e.dimension = %[1]d AND e.knowledge_base_id = ? AND e.knowledge_id <> ?
			  AND e.source_id = e.chunk_id AND (e.is_enabled IS NULL OR e.is_enabled = TRUE)
			ORDER BY e.embedding::halfvec(%[1]d) <=> s.embedding::halfvec(%[1]d)
			LIMIT ?
		) AS n
		WHERE n.distance <= ?
		ORDER BY s.chunk_id, n.distance, n.chunk_id`, dimension)
	args := []any{kbID, knowledgeID, maxSimilarSubjectRows, kbID, knowledgeID, perChunk, 1 - minScore}

	// As in VectorRetrieve: HNSW returns at most ef_search candidates before
	// the knowledge-base filter is applied, so without a larger budget and an
	// iterative scan a busy table would hide the neighbours that belong to
	// this base. strict_order keeps the neighbours in true distance order,
	// which the duplicate detector relies on to see the same pairs from
	// either document.
	efSearch := perChunk * 4
	if efSearch < 40 {
		efSearch = 40
	}
	var rows []types.ChunkSimilarity
	err := g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", efSearch)).Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL hnsw.iterative_scan = strict_order").Error; err != nil {
			return err
		}
		return tx.Raw(query, args...).Scan(&rows).Error
	})
	if err != nil && (strings.Contains(err.Error(), "hnsw.ef_search") ||
		strings.Contains(err.Error(), "hnsw.iterative_scan")) {
		// An older pgvector without these settings still answers, with the
		// default recall.
		logger.GetLogger(ctx).Warnf("[Postgres] similar-chunk search without HNSW overrides: %v", err)
		rows = nil
		err = g.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	}
	if err != nil {
		return nil, fmt.Errorf("finding chunks similar to knowledge %s (dimension %d): %w", knowledgeID, dimension, err)
	}
	return rows, nil
}
