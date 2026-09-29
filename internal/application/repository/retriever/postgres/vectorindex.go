package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/logger"
)

// Vector search needs an HNSW index, and an index over a vector column has a
// fixed dimension. The embeddings table holds vectors of every dimension in use
// (different knowledge bases use different embedding models), so the index is
// a partial one per dimension, matching the expression the search query uses:
//
//	CREATE INDEX ... USING hnsw ((embedding::halfvec(N)) halfvec_cosine_ops)
//	WHERE dimension = N
//
// The migrations created three of these (for the dimensions of the models the
// project first shipped with). Any other dimension — 768, 1536 and 3072 are as
// common as those — had none, and its searches read the whole table. The
// indexes are therefore created when a dimension first appears, at the write
// that introduces it.

const (
	// maxIndexedDimension is the largest dimension pgvector can put in an HNSW
	// index over half-precision vectors.
	maxIndexedDimension = 4000

	// indexBuildTimeout bounds one index build. CREATE INDEX CONCURRENTLY does
	// not block writers, but a build that never ends holds a connection.
	indexBuildTimeout = 30 * time.Minute
)

// vectorIndexName is the name the migrations already use for these indexes.
func vectorIndexName(dimension int) string {
	return fmt.Sprintf("embeddings_embedding_idx_%d", dimension)
}

// vectorIndexes creates the per-dimension indexes on demand, once each.
type vectorIndexes struct {
	db *gorm.DB

	mu       sync.Mutex
	ready    map[int]bool
	building map[int]*indexBuild
}

// indexBuild is a build in progress, shared by everyone waiting for it.
type indexBuild struct {
	done chan struct{}
	err  error
}

func newVectorIndexes(db *gorm.DB) *vectorIndexes {
	return &vectorIndexes{db: db, ready: map[int]bool{}, building: map[int]*indexBuild{}}
}

// Ensure makes sure vectors of the dimension can be searched with an index. It
// returns quickly once the dimension has been seen; the first call for a
// dimension builds the index and returns when it is usable. Callers waiting on
// a build that another caller started share its outcome.
//
// A dimension too large to index is not an error: the vectors are stored, and
// searching them scans the table, which is slow but correct. Refusing to store
// them would make the embedding model unusable, which is worse.
func (v *vectorIndexes) Ensure(ctx context.Context, dimension int) error {
	if dimension <= 0 {
		return nil // keyword-only rows carry no vector
	}

	v.mu.Lock()
	if v.ready[dimension] {
		v.mu.Unlock()
		return nil
	}
	build, inFlight := v.building[dimension]
	if !inFlight {
		build = &indexBuild{done: make(chan struct{})}
		v.building[dimension] = build
	}
	v.mu.Unlock()

	if inFlight {
		select {
		case <-build.done:
			return build.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// The build outlives the request that triggered it: cancelling
	// CREATE INDEX CONCURRENTLY half way leaves an invalid index behind, and
	// other callers may be waiting on the result.
	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), indexBuildTimeout)
	defer cancel()
	build.err = v.create(buildCtx, dimension)

	v.mu.Lock()
	if build.err == nil {
		v.ready[dimension] = true
	}
	delete(v.building, dimension)
	v.mu.Unlock()
	close(build.done)
	return build.err
}

// create builds the index unless a valid one exists.
func (v *vectorIndexes) create(ctx context.Context, dimension int) error {
	log := logger.GetLogger(ctx)
	if dimension > maxIndexedDimension {
		log.Warnf("[Postgres] embeddings of dimension %d exceed the HNSW limit of %d: "+
			"they are stored without an index, so vector search over them scans the table",
			dimension, maxIndexedDimension)
		return nil
	}
	name := vectorIndexName(dimension)

	// A CONCURRENTLY build that failed or was interrupted leaves an invalid
	// index that still occupies the name and is never used. IF NOT EXISTS
	// would then skip the build and search would stay unindexed, so it is
	// dropped first.
	var valid []bool
	if err := v.db.WithContext(ctx).Raw(
		`SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid WHERE c.relname = ?`, name,
	).Scan(&valid).Error; err != nil {
		return fmt.Errorf("looking up index %s: %w", name, err)
	}
	if len(valid) > 0 && valid[0] {
		return nil
	}
	if len(valid) > 0 {
		log.Warnf("[Postgres] dropping invalid index %s left by an interrupted build", name)
		drop := fmt.Sprintf(`DROP INDEX CONCURRENTLY IF EXISTS %s`, name)
		if err := v.db.WithContext(ctx).Exec(drop).Error; err != nil {
			return fmt.Errorf("dropping invalid index %s: %w", name, err)
		}
	}

	log.Infof("[Postgres] building the HNSW index for %d-dimensional embeddings", dimension)
	// The dimension is an integer formatted by %d, never user text.
	stmt := fmt.Sprintf(
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON embeddings `+
			`USING hnsw ((embedding::halfvec(%d)) halfvec_cosine_ops) `+
			`WITH (m = 16, ef_construction = 64) WHERE (dimension = %d)`,
		name, dimension, dimension)
	if err := v.db.WithContext(ctx).Exec(stmt).Error; err != nil {
		return fmt.Errorf("creating index %s: %w", name, err)
	}
	return nil
}

// ensureDimensions ensures the index of each distinct dimension in the rows.
func (g *pgRepository) ensureDimensions(ctx context.Context, rows []*pgVector) error {
	seen := make(map[int]bool, 1)
	for _, row := range rows {
		if seen[row.Dimension] {
			continue
		}
		seen[row.Dimension] = true
		if err := g.indexes.Ensure(ctx, row.Dimension); err != nil {
			return err
		}
	}
	return nil
}
