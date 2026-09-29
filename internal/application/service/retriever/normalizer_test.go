package retriever

import (
	"context"
	"math"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestEngineAwareNormalizer_KeywordPassthrough(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	cases := []float64{-12.5, 0, 0.7, 1, 27.3, math.Inf(1)}
	for _, s := range cases {
		// Any keyword retriever, any engine — must be identity.
		got := n.Normalize(context.Background(), s, types.KeywordsRetrieverType,
			stubSignedType)
		if got != s {
			// math.IsNaN special handling for the Inf case.
			if math.IsInf(s, 1) && math.IsInf(got, 1) {
				continue
			}
			t.Fatalf("keyword passthrough expected %v, got %v", s, got)
		}
	}
}

func TestEngineAwareNormalizer_CosineRange(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	// An engine whose descriptor declares ScoreSignedCosine surfaces the raw
	// cosine range [-1, 1]; it is shifted onto [0, 1].
	cases := []struct {
		score float64
		want  float64
	}{
		{-1.0, 0},
		{-0.5, 0.25},
		{0, 0.5},
		{0.5, 0.75},
		{1.0, 1.0},
		// Drift beyond [-1, 1] is clamped.
		{-1.5, 0},
		{1.5, 1.0},
	}
	for _, tc := range cases {
		got := n.Normalize(context.Background(), tc.score, types.VectorRetrieverType, stubSignedType)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("cosine[%s] score=%v: want %v, got %v", stubSignedType, tc.score, tc.want, got)
		}
	}
}

func TestEngineAwareNormalizer_UnitInterval(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	// An engine declaring ScoreUnit already reports in [0, 1] (pgvector's
	// 1 - distance does, for L2-normalised embeddings); the normalizer only
	// clamps. Engines the catalog does not know are treated the same way.
	cases := []struct {
		score float64
		want  float64
	}{
		{-0.1, 0},
		{0, 0},
		{0.25, 0.25},
		{0.999, 0.999},
		{1, 1},
		{1.5, 1},
	}
	for _, engine := range []types.RetrieverEngineType{
		types.PostgresRetrieverEngineType,
		stubUnitType,
		types.RetrieverEngineType("nosuch"),
	} {
		for _, tc := range cases {
			got := n.Normalize(context.Background(), tc.score, types.VectorRetrieverType, engine)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("unit[%s] score=%v: want %v, got %v", engine, tc.score, tc.want, got)
			}
		}
	}
}

func TestEngineAwareNormalizer_Knows(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	if !n.Knows(stubSignedType) || !n.Knows(types.PostgresRetrieverEngineType) {
		t.Fatal("engines in the catalog must be known")
	}
	if n.Knows(types.RetrieverEngineType("nosuch")) {
		t.Fatal("an engine outside the catalog must not be known")
	}
	if NewEngineAwareNormalizer(nil).Knows(types.PostgresRetrieverEngineType) {
		t.Fatal("a nil catalog knows no engine")
	}
}

func TestEngineAwareNormalizer_Unknown_ClampsAndDoesNotPanic(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	got := n.Normalize(context.Background(), 0.42,
		types.VectorRetrieverType, types.RetrieverEngineType("nosuch"))
	if got != 0.42 {
		t.Fatalf("unknown engine passthrough-clamp: want 0.42, got %v", got)
	}
	got = n.Normalize(context.Background(), 5.0,
		types.VectorRetrieverType, types.RetrieverEngineType("nosuch"))
	if got != 1.0 {
		t.Fatalf("unknown engine clamp on >1: want 1, got %v", got)
	}
}

func TestEngineAwareNormalizer_NilCtx_DoesNotPanic(t *testing.T) {
	// nil ctx must not panic. Normalize is IO-free by contract — it never
	// reads from ctx and never logs. The unknown-engine warning is
	// emitted by the caller (retrieveFromStores), where ctx is always
	// live.
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Normalize panicked on nil ctx: %v", r)
		}
	}()
	_ = NewEngineAwareNormalizer(stubCatalog(t)).Normalize(
		nil, 0.5, types.VectorRetrieverType,
		types.RetrieverEngineType("nosuch"),
	)
}

func TestEngineAwareNormalizer_NaNAndInf(t *testing.T) {
	t.Parallel()
	n := NewEngineAwareNormalizer(stubCatalog(t))
	// NaN → 0 (so SortFunc does not get a comparator-poisoning value).
	got := n.Normalize(context.Background(), math.NaN(),
		types.VectorRetrieverType, types.PostgresRetrieverEngineType)
	if got != 0 {
		t.Fatalf("NaN clamp: want 0, got %v", got)
	}
	// +Inf → 1.
	got = n.Normalize(context.Background(), math.Inf(1),
		types.VectorRetrieverType, types.PostgresRetrieverEngineType)
	if got != 1 {
		t.Fatalf("+Inf clamp: want 1, got %v", got)
	}
	// -Inf → 0.
	got = n.Normalize(context.Background(), math.Inf(-1),
		types.VectorRetrieverType, types.PostgresRetrieverEngineType)
	if got != 0 {
		t.Fatalf("-Inf clamp: want 0, got %v", got)
	}
	// NaN through cosine formula too.
	got = n.Normalize(context.Background(), math.NaN(),
		types.VectorRetrieverType, stubSignedType)
	if got != 0 {
		t.Fatalf("NaN cosine: want 0, got %v", got)
	}
}

func TestClamp01(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want float64
	}{
		{-1, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{2, 1},
		{math.NaN(), 0},
		{math.Inf(1), 1},
		{math.Inf(-1), 0},
	}
	for _, tc := range cases {
		got := clamp01(tc.in)
		if math.Abs(got-tc.want) > 1e-9 {
			// math.IsNaN comparison would have already failed via the
			// equality above; this branch covers the rest.
			t.Fatalf("clamp01(%v): want %v, got %v", tc.in, tc.want, got)
		}
	}
}

// TestEngineAwareNormalizer_InterfaceSatisfied is a compile-time check that
// catches accidental breakage of the ScoreNormalizer interface via the
// package-scope var assertion in normalizer.go. The runtime body is a
// no-op; failure would surface at build time.
func TestEngineAwareNormalizer_InterfaceSatisfied(t *testing.T) {
	var _ ScoreNormalizer = EngineAwareNormalizer{}
}
