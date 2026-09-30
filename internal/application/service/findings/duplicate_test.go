package findings

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// fakeChunks serves chunks by document.
type fakeChunks map[string][]*types.Chunk

func (f fakeChunks) ListChunksByKnowledgeID(_ context.Context, _ uint64, knowledgeID string) ([]*types.Chunk, error) {
	return f[knowledgeID], nil
}

// fakeFinder answers SimilarChunks from a symmetric table of passage
// similarities, keeping, like the real query, the nearest perChunk neighbours
// in other documents above the threshold.
type fakeFinder struct {
	docOf  map[string]string // chunk -> document
	scores map[[2]string]float64
	calls  []string
}

func newFakeFinder() *fakeFinder {
	return &fakeFinder{docOf: map[string]string{}, scores: map[[2]string]float64{}}
}

func (f *fakeFinder) chunk(doc string, ids ...string) {
	for _, id := range ids {
		f.docOf[id] = doc
	}
}

func (f *fakeFinder) similar(a, b string, score float64) {
	f.scores[[2]string{a, b}] = score
	f.scores[[2]string{b, a}] = score
}

func (f *fakeFinder) SimilarChunks(_ context.Context, _ string, knowledgeID string, minScore float64,
	perChunk int,
) ([]types.ChunkSimilarity, error) {
	f.calls = append(f.calls, knowledgeID)
	var out []types.ChunkSimilarity
	for chunk, doc := range f.docOf {
		if doc != knowledgeID {
			continue
		}
		var near []types.ChunkSimilarity
		for other, otherDoc := range f.docOf {
			if otherDoc == knowledgeID {
				continue
			}
			if score, ok := f.scores[[2]string{chunk, other}]; ok && score >= minScore {
				near = append(near, types.ChunkSimilarity{
					SubjectChunkID: chunk, RelatedChunkID: other, RelatedKnowledgeID: otherDoc, Score: score,
				})
			}
		}
		sort.Slice(near, func(i, j int) bool { return near[i].Score > near[j].Score })
		if len(near) > perChunk {
			near = near[:perChunk]
		}
		out = append(out, near...)
	}
	return out, nil
}

type fixedResolver struct {
	finder interfaces.SimilarChunkFinder
	ok     bool
	err    error
}

func (r fixedResolver) SimilarChunkFinder(context.Context, *types.KnowledgeBase,
) (interfaces.SimilarChunkFinder, bool, error) {
	return r.finder, r.ok, r.err
}

func text(id, content string) *types.Chunk {
	return &types.Chunk{ID: id, ChunkType: types.ChunkTypeText, IsEnabled: true, Content: content}
}

// long pads a passage past the length below which passages are ignored.
func long(s string) string {
	return s + strings.Repeat(" and the rest of the paragraph", 3)
}

var docBase = &types.KnowledgeBase{
	ID: "kb-1", TenantID: 1, Type: types.KnowledgeBaseTypeDocument,
	IndexingStrategy: types.IndexingStrategy{VectorEnabled: true},
}

func scopeOf(id string) Scope {
	return Scope{TenantID: 1, KnowledgeBase: docBase, Knowledge: &types.Knowledge{ID: id, KnowledgeBaseID: "kb-1"}}
}

// small is a two-passage note both of whose passages are copied into big, a
// four-passage handbook; other shares one passage with big, weakly.
func duplicateFixture() (fakeChunks, *fakeFinder) {
	chunks := fakeChunks{
		"small": {text("s1", long("Refunds are paid within 14 days")), text("s2", long("Receipts must be kept"))},
		"big": {
			text("b1", long("Refunds are paid within 14 days")), text("b2", long("Receipts must be kept")),
			text("b3", long("Travel is booked centrally")), text("b4", long("Hotels are capped per city")),
			// Too short to count, whatever it matches.
			text("b5", "Contact us"),
		},
		"other": {text("o1", long("Hotels are capped per city"))},
	}
	finder := newFakeFinder()
	finder.chunk("small", "s1", "s2")
	finder.chunk("big", "b1", "b2", "b3", "b4", "b5")
	finder.chunk("other", "o1")
	finder.similar("s1", "b1", 0.99)
	finder.similar("s2", "b2", 0.97)
	finder.similar("b4", "o1", 0.96)
	finder.similar("s1", "b5", 0.995)
	return chunks, finder
}

func detectFrom(t *testing.T, id string) []Candidate {
	t.Helper()
	chunks, finder := duplicateFixture()
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	out, err := d.Detect(context.Background(), scopeOf(id))
	require.NoError(t, err)
	sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint < out[j].Fingerprint })
	return out
}

func findingBetween(t *testing.T, cands []Candidate, a, b string) Candidate {
	t.Helper()
	for _, c := range cands {
		if (c.SubjectKnowledgeID == a && c.RelatedKnowledgeID == b) ||
			(c.SubjectKnowledgeID == b && c.RelatedKnowledgeID == a) {
			return c
		}
	}
	t.Fatalf("no finding between %s and %s in %+v", a, b, cands)
	return Candidate{}
}

// One finding per pair of documents, oriented at the document more fully
// copied, with its overlap, its strongest pair and the passages behind it.
func TestDuplicateDetectorAggregatesPairsIntoOneFindingPerDocumentPair(t *testing.T) {
	out := detectFrom(t, "small")
	require.Len(t, out, 1, "small matches big only")
	c := out[0]

	assert.Equal(t, types.FindingTypeDuplicate, c.Type)
	assert.Equal(t, "small", c.SubjectKnowledgeID, "the note wholly inside the handbook is the subject")
	assert.Equal(t, "big", c.RelatedKnowledgeID)
	assert.InDelta(t, 1.0, c.Details.OverlapRatio, 1e-9, "both of small's passages have a copy")
	assert.Equal(t, types.FindingSeverityWarning, c.Severity)
	assert.InDelta(t, 0.99, c.Score, 1e-9, "the score is the strongest pair")
	require.Len(t, c.Details.Evidence, 2)
	assert.Equal(t, "s1", c.Details.Evidence[0].SubjectChunkID)
	assert.Equal(t, "b1", c.Details.Evidence[0].RelatedChunkID, "the short passage b5 does not count")
	assert.Contains(t, c.Details.Evidence[0].SubjectExcerpt, "Refunds are paid")
	assert.Contains(t, c.Details.Evidence[0].RelatedExcerpt, "Refunds are paid")
	assert.NotEmpty(t, c.Details.EvidenceHash)
	assert.Equal(t, PairFingerprint(types.FindingTypeDuplicate, "kb-1", "small", "big"), c.Fingerprint)
}

// Whichever document triggers the check, the finding comes out the same:
// fingerprint, orientation, overlap, evidence and hash.
func TestDuplicateFindingIsTheSameFromEitherSide(t *testing.T) {
	fromSmall := findingBetween(t, detectFrom(t, "small"), "small", "big")
	fromBig := findingBetween(t, detectFrom(t, "big"), "small", "big")

	assert.Equal(t, fromSmall.Fingerprint, fromBig.Fingerprint)
	assert.Equal(t, fromSmall.SubjectKnowledgeID, fromBig.SubjectKnowledgeID)
	assert.Equal(t, fromSmall.Severity, fromBig.Severity)
	assert.InDelta(t, fromSmall.Details.OverlapRatio, fromBig.Details.OverlapRatio, 1e-9)
	assert.Equal(t, fromSmall.Details.Evidence, fromBig.Details.Evidence)
	assert.Equal(t, fromSmall.Details.EvidenceHash, fromBig.Details.EvidenceHash)
}

// A partial copy is a note, not a warning: one of big's four passages is in
// other, and other's single passage is in big.
func TestDuplicateSeverityFollowsTheOverlap(t *testing.T) {
	out := detectFrom(t, "big")
	require.Len(t, out, 2)
	weak := findingBetween(t, out, "big", "other")
	assert.Equal(t, "other", weak.SubjectKnowledgeID, "other is wholly inside big")
	assert.InDelta(t, 1.0, weak.Details.OverlapRatio, 1e-9)

	// With other grown to three passages, one of which big has, neither
	// document is mostly a copy: other is a third copied, big a quarter.
	chunks, finder := duplicateFixture()
	chunks["other"] = append(chunks["other"],
		text("o2", long("Unrelated passage one")), text("o3", long("Unrelated passage two")))
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	res, err := d.Detect(context.Background(), scopeOf("other"))
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "other", res[0].SubjectKnowledgeID, "the larger share decides the subject")
	assert.InDelta(t, 1.0/3, res[0].Details.OverlapRatio, 1e-9)
	assert.Equal(t, types.FindingSeverityInfo, res[0].Severity)
}

// Passages that are alike and not the same make a divergent finding, whose
// evidence shows the difference, taken to whoever answers for the document
// vouched for the longest ago; copies make a duplicate one, taken to whoever
// wrote the newer text.
func TestDuplicateDetectorTellsACopyFromANearCopy(t *testing.T) {
	copyFinding := findingBetween(t, detectFrom(t, "small"), "small", "big")
	assert.Equal(t, types.FindingTypeDuplicate, copyFinding.Type)
	assert.Equal(t, AssignLatestHand, copyFinding.Assign)
	for _, e := range copyFinding.Details.Evidence {
		assert.False(t, e.Differs)
	}

	leave := "Every employee has fifteen days of paid leave a year, requested two weeks ahead in the HR system."
	changed := strings.Replace(leave, "fifteen", "ten", 1)
	chunks := fakeChunks{
		"old": {text("old1", leave), text("old2", long("Receipts must be kept"))},
		"new": {text("new1", changed), text("new2", long("Receipts must be kept"))},
	}
	finder := newFakeFinder()
	finder.chunk("old", "old1", "old2")
	finder.chunk("new", "new1", "new2")
	finder.similar("old1", "new1", 0.97)
	finder.similar("old2", "new2", 0.99)
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	out, err := d.Detect(context.Background(), scopeOf("new"))
	require.NoError(t, err)
	require.Len(t, out, 1)
	c := out[0]
	assert.Equal(t, types.FindingTypeDivergent, c.Type)
	assert.Equal(t, types.FindingSeverityWarning, c.Severity, "a disagreement is a warning however small")
	assert.Equal(t, AssignStalestOwner, c.Assign)
	assert.Equal(t, PairFingerprint(types.FindingTypeDivergent, "kb-1", "old", "new"), c.Fingerprint)
	require.Len(t, c.Details.Evidence, 2)
	first := c.Details.Evidence[0]
	assert.True(t, first.Differs, "the difference is shown first, though its pair scored lower")
	assert.Contains(t, first.SubjectExcerpt+first.RelatedExcerpt, "fifteen")
	assert.Contains(t, first.SubjectExcerpt+first.RelatedExcerpt, "ten days")
	assert.False(t, c.Details.Evidence[1].Differs)
}

// The hash is over the matched text, so re-parsing (new chunk IDs, same
// words) keeps a dismissal, while a changed passage reopens it.
func TestDuplicateEvidenceHashFollowsTheText(t *testing.T) {
	base := findingBetween(t, detectFrom(t, "small"), "small", "big")

	chunks, finder := duplicateFixture()
	chunks["small"] = []*types.Chunk{
		text("s1-new", long("Refunds are paid within 14 days")), text("s2-new", long("Receipts must be kept")),
	}
	finder.docOf = map[string]string{}
	finder.chunk("small", "s1-new", "s2-new")
	finder.chunk("big", "b1", "b2", "b3", "b4", "b5")
	finder.similar("s1-new", "b1", 0.99)
	finder.similar("s2-new", "b2", 0.97)
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	reparsed, err := d.Detect(context.Background(), scopeOf("small"))
	require.NoError(t, err)
	require.Len(t, reparsed, 1)
	assert.Equal(t, base.Details.EvidenceHash, reparsed[0].Details.EvidenceHash, "new IDs, same text")

	chunks["small"][1] = text("s2-new", long("Receipts must be kept for seven years"))
	edited, err := d.Detect(context.Background(), scopeOf("small"))
	require.NoError(t, err)
	require.Len(t, edited, 1)
	assert.NotEqual(t, base.Details.EvidenceHash, edited[0].Details.EvidenceHash)
}

func TestDuplicateExcerptsAreBounded(t *testing.T) {
	longText := strings.Repeat("长文本段落，", 200)
	got := excerpt(longText)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), duplicateExcerptRunes)
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.Equal(t, "a b", excerpt("  a \n\t b "), "whitespace is collapsed for display")
}

// Disabled and non-text chunks do not take part.
func TestDuplicateDetectorIgnoresWhatIsNotAPassage(t *testing.T) {
	chunks, finder := duplicateFixture()
	chunks["small"][0].IsEnabled = false
	chunks["small"][1].ChunkType = types.ChunkTypeSummary
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	out, err := d.Detect(context.Background(), scopeOf("small"))
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Empty(t, finder.calls, "a document with no passages is not searched at all")
}

// A base the detector cannot check is reported as such, not as a failure; an
// engine that cannot be reached is a failure.
func TestDuplicateDetectorSaysWhenItDoesNotApply(t *testing.T) {
	chunks, finder := duplicateFixture()
	ctx := context.Background()

	faq := *docBase
	faq.Type = types.KnowledgeBaseTypeFAQ
	d := NewDuplicateDetector(chunks, fixedResolver{finder: finder, ok: true}, 0.95)
	_, err := d.Detect(ctx, Scope{TenantID: 1, KnowledgeBase: &faq, Knowledge: &types.Knowledge{ID: "small"}})
	assert.ErrorIs(t, err, ErrUnsupported, "FAQ bases deduplicate on import")
	ok, err := d.Supports(ctx, &faq)
	require.NoError(t, err)
	assert.False(t, ok)

	noVectors := *docBase
	noVectors.IndexingStrategy = types.IndexingStrategy{KeywordEnabled: true}
	_, err = d.Detect(ctx, Scope{TenantID: 1, KnowledgeBase: &noVectors, Knowledge: &types.Knowledge{ID: "small"}})
	assert.ErrorIs(t, err, ErrUnsupported)

	unsupported := NewDuplicateDetector(chunks, fixedResolver{}, 0.95)
	_, err = unsupported.Detect(ctx, scopeOf("small"))
	assert.ErrorIs(t, err, ErrUnsupported, "an engine without the capability")
	ok, err = unsupported.Supports(ctx, docBase)
	require.NoError(t, err)
	assert.False(t, ok)

	down := errors.New("vector store unreachable")
	broken := NewDuplicateDetector(chunks, fixedResolver{err: down}, 0.95)
	_, err = broken.Detect(ctx, scopeOf("small"))
	assert.ErrorIs(t, err, down)
	assert.NotErrorIs(t, err, ErrUnsupported)

	supported, err := d.Supports(ctx, docBase)
	require.NoError(t, err)
	assert.True(t, supported)
}

func TestPairFingerprintIsSymmetricAndScoped(t *testing.T) {
	assert.Equal(t, PairFingerprint("duplicate", "kb", "a", "b"), PairFingerprint("duplicate", "kb", "b", "a"))
	assert.NotEqual(t, PairFingerprint("duplicate", "kb", "a", "b"), PairFingerprint("duplicate", "kb2", "a", "b"))
	assert.NotEqual(t, PairFingerprint("duplicate", "kb", "a", "b"), PairFingerprint("superseded", "kb", "a", "b"))
	assert.NotEqual(t, PairFingerprint("stale", "kb", "a", ""), PairFingerprint("stale", "kb", "b", ""))
	assert.LessOrEqual(t, len(PairFingerprint("contradiction", "kb", "a", "b")), 128)
}
