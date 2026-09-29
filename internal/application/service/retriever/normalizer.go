package retriever

import (
	"context"
	"math"

	"github.com/magicyuan876/yuheng/internal/types"
)

// ScoreNormalizer maps raw retriever scores to a common [0, 1] scale so that
// vector scores produced by different engines can be compared in a single
// ranked list. Implementations MUST be safe for concurrent use and MUST be
// IO-free (Normalize is called inside a hot loop and may not log or block).
//
// Only vector scores are normalized. Keyword (BM25) scores have an unbounded
// positive range; rescaling them would collapse the long tail. Downstream
// RRF fusion is rank-based and immune to scale, so keyword scores pass
// through unchanged.
type ScoreNormalizer interface {
	Normalize(
		ctx context.Context,
		score float64,
		retrieverType types.RetrieverType,
		engineType types.RetrieverEngineType,
	) float64
}

// EngineAwareNormalizer puts each engine's vector scores on [0, 1] according
// to the ScoreScale its descriptor declares. What an engine reports, and why,
// is documented with the engine.
//
// The caller enforces a same-embedding-model precondition, so after
// normalization the values from different engines are comparable. Results from
// a single engine keep their native scale (the caller only normalizes when
// engines are mixed).
//
// An engine the catalog does not know is clamped to [0, 1]; the fan-out caller
// logs it once per request.
type EngineAwareNormalizer struct {
	catalog *Catalog
}

var _ ScoreNormalizer = EngineAwareNormalizer{}

// NewEngineAwareNormalizer builds the normalizer over a catalog.
func NewEngineAwareNormalizer(catalog *Catalog) EngineAwareNormalizer {
	return EngineAwareNormalizer{catalog: catalog}
}

// Knows reports whether the engine has a declared score scale.
func (n EngineAwareNormalizer) Knows(engineType types.RetrieverEngineType) bool {
	_, ok := n.catalog.ByType(engineType)
	return ok
}

// Normalize implements ScoreNormalizer.
func (n EngineAwareNormalizer) Normalize(
	_ context.Context,
	score float64,
	retrieverType types.RetrieverType,
	engineType types.RetrieverEngineType,
) float64 {
	if retrieverType != types.VectorRetrieverType {
		// BM25 and other non-vector retrievers: passthrough. RRF rank-based
		// fusion handles scale-mixed input correctly.
		return score
	}
	if d, ok := n.catalog.ByType(engineType); ok && d.Capabilities.ScoreScale == ScoreSignedCosine {
		// Raw cosine in [-1, 1] → [0, 1]. Clamped once more so that an engine
		// returning 1.0000002 does not leak past the envelope (the caller
		// sorts by score afterwards).
		return clamp01((score + 1) / 2)
	}
	return clamp01(score)
}

// clamp01 maps any float64 into [0, 1] safely, including NaN/Inf inputs that
// could otherwise break slices.SortFunc's strict-weak-ordering invariant
// downstream (NaN compares neither greater nor less than anything).
func clamp01(s float64) float64 {
	if math.IsNaN(s) {
		return 0
	}
	if s <= 0 || math.IsInf(s, -1) {
		return 0
	}
	if s >= 1 || math.IsInf(s, 1) {
		return 1
	}
	return s
}
