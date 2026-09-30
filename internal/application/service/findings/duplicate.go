package findings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/magicyuan876/yuheng/internal/types"
)

// The duplicate detector: documents of one knowledge base whose passages are,
// to the embedding model, the same text.
//
// It compares the vectors indexing already stored, so it costs a few database
// queries and no model call. For the changed document it asks the engine for
// the nearest passages of every other document, groups the close pairs by the
// document they point at, and reports one finding per pair of documents.
//
// Nearest-neighbour lists are not symmetric: the passages closest to A's are
// not necessarily those to which B's passages are closest. A finding computed
// from A's side alone would come out different when B changes and triggers
// the check, and a dismissed finding would reopen for no reason. So for every
// document found, the search is also run from its side, and the finding is
// built from the union of both directions — the same pairs whichever document
// triggered the check.
//
// The matched passages are then compared as text (divergence.go). Word-for-word
// copies make a duplicate finding, taken to whoever wrote the newer text; pairs
// that are alike but differ make a divergent one, taken to whoever answers for
// the document nobody has vouched for the longest.

// DuplicateDetectorName is the detector's stored name.
const DuplicateDetectorName = "duplicate"

const (
	// duplicateNeighbours is how many nearest passages of other documents
	// each passage is compared with.
	duplicateNeighbours = 5
	// duplicateMaxRelated bounds the documents one check reports against;
	// each costs a search from its side. A document duplicated in more than
	// this many others has its strongest duplicates reported.
	duplicateMaxRelated = 20
	// duplicateMinPassageRunes keeps very short passages out of the
	// comparison. Headings and one-line boilerplate ("Contact us", a page
	// footer) embed alike across unrelated documents; counting them would
	// make every pair of documents from one template look duplicated.
	duplicateMinPassageRunes = 40
	// duplicateWarningOverlap is the share of a document's passages that must
	// have a copy elsewhere for the finding to be a warning rather than a
	// note: past half, the document is mostly a copy.
	duplicateWarningOverlap = 0.5
	// duplicateEvidencePairs is how many passage pairs a finding shows.
	duplicateEvidencePairs = 3
	// duplicateExcerptRunes bounds each excerpt shown.
	duplicateExcerptRunes = 300
	// duplicateExcerptLead is how much text an excerpt keeps before the
	// difference it is centred on.
	duplicateExcerptLead = 60
)

// ChunkReader is the part of the chunk repository the detector reads.
// interfaces.ChunkRepository satisfies it.
type ChunkReader interface {
	ListChunksByKnowledgeID(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.Chunk, error)
}

// DuplicateDetector reports near-duplicate documents.
type DuplicateDetector struct {
	chunks   ChunkReader
	finders  FinderResolver
	minScore float64
}

// NewDuplicateDetector returns the detector. minScore is the cosine similarity
// at which two passages count as the same text.
func NewDuplicateDetector(chunks ChunkReader, finders FinderResolver, minScore float64) *DuplicateDetector {
	return &DuplicateDetector{chunks: chunks, finders: finders, minScore: minScore}
}

// Name implements Detector.
func (d *DuplicateDetector) Name() string { return DuplicateDetectorName }

// Supports implements SupportChecker: a document knowledge base with vector
// indexing, on an engine that can compare stored vectors. FAQ bases are left
// out; their import already refuses duplicate questions.
func (d *DuplicateDetector) Supports(ctx context.Context, kb *types.KnowledgeBase) (bool, error) {
	if !d.applies(kb) {
		return false, nil
	}
	_, ok, err := d.finders.SimilarChunkFinder(ctx, kb)
	return ok, err
}

func (d *DuplicateDetector) applies(kb *types.KnowledgeBase) bool {
	return kb != nil && kb.Type != types.KnowledgeBaseTypeFAQ && kb.IndexingStrategy.VectorEnabled
}

// passage is one chunk as the detector sees it.
type passage struct {
	id      string
	content string
}

// Detect implements Detector.
func (d *DuplicateDetector) Detect(ctx context.Context, scope Scope) ([]Candidate, error) {
	kb := scope.KnowledgeBase
	if !d.applies(kb) {
		return nil, fmt.Errorf("%w: knowledge base %s is not a vector-indexed document base",
			ErrUnsupported, scope.KnowledgeBaseID())
	}
	finder, ok, err := d.finders.SimilarChunkFinder(ctx, kb)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: the retrieval engine of %s cannot compare stored vectors", ErrUnsupported, kb.ID)
	}

	subjectID := scope.KnowledgeID()
	subject, err := d.passages(ctx, scope.TenantID, subjectID)
	if err != nil {
		return nil, err
	}
	if len(subject) == 0 {
		return nil, nil
	}
	forward, err := finder.SimilarChunks(ctx, kb.ID, subjectID, d.minScore, duplicateNeighbours)
	if err != nil {
		return nil, err
	}

	// Group the close pairs by the document they point at, keeping only
	// passages of the subject that count, and the best score per document to
	// choose which documents to look at from their side.
	byRelated := map[string][]types.ChunkSimilarity{}
	best := map[string]float64{}
	for _, pair := range forward {
		if _, ok := subject[pair.SubjectChunkID]; !ok || pair.RelatedKnowledgeID == "" {
			continue
		}
		byRelated[pair.RelatedKnowledgeID] = append(byRelated[pair.RelatedKnowledgeID], pair)
		if pair.Score > best[pair.RelatedKnowledgeID] {
			best[pair.RelatedKnowledgeID] = pair.Score
		}
	}
	related := make([]string, 0, len(byRelated))
	for id := range byRelated {
		related = append(related, id)
	}
	sort.Slice(related, func(i, j int) bool {
		if best[related[i]] != best[related[j]] {
			return best[related[i]] > best[related[j]]
		}
		return related[i] < related[j]
	})
	if len(related) > duplicateMaxRelated {
		related = related[:duplicateMaxRelated]
	}

	var out []Candidate
	for _, otherID := range related {
		other, err := d.passages(ctx, scope.TenantID, otherID)
		if err != nil {
			return nil, err
		}
		if len(other) == 0 {
			continue
		}
		backward, err := finder.SimilarChunks(ctx, kb.ID, otherID, d.minScore, duplicateNeighbours)
		if err != nil {
			return nil, err
		}
		pairs := unionPairs(subject, other, byRelated[otherID], backward, subjectID)
		if len(pairs) == 0 {
			continue
		}
		out = append(out, buildDuplicate(kb.ID, subjectID, subject, otherID, other, pairs))
	}
	return out, nil
}

// passages loads the chunks of a document that take part in the comparison:
// enabled text chunks long enough to mean something.
func (d *DuplicateDetector) passages(ctx context.Context, tenantID uint64, knowledgeID string,
) (map[string]passage, error) {
	chunks, err := d.chunks.ListChunksByKnowledgeID(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, fmt.Errorf("loading the chunks of knowledge %s: %w", knowledgeID, err)
	}
	out := make(map[string]passage, len(chunks))
	for _, c := range chunks {
		if c == nil || !c.IsEnabled || c.ChunkType != types.ChunkTypeText {
			continue
		}
		content := strings.TrimSpace(c.Content)
		if utf8.RuneCountInString(content) < duplicateMinPassageRunes {
			continue
		}
		out[c.ID] = passage{id: c.ID, content: content}
	}
	return out, nil
}

// passagePair is one matched pair, subject side first.
type passagePair struct {
	subject, related string
	score            float64
}

// unionPairs merges the pairs found from the subject's side with those found
// from the other document's side, keeping only passages that count on both
// sides and the best score of a pair seen twice.
func unionPairs(subject, other map[string]passage, forward, backward []types.ChunkSimilarity,
	subjectID string,
) []passagePair {
	scores := map[[2]string]float64{}
	add := func(s, r string, score float64) {
		if _, ok := subject[s]; !ok {
			return
		}
		if _, ok := other[r]; !ok {
			return
		}
		key := [2]string{s, r}
		if score > scores[key] {
			scores[key] = score
		}
	}
	for _, p := range forward {
		add(p.SubjectChunkID, p.RelatedChunkID, p.Score)
	}
	for _, p := range backward {
		if p.RelatedKnowledgeID == subjectID {
			add(p.RelatedChunkID, p.SubjectChunkID, p.Score)
		}
	}
	out := make([]passagePair, 0, len(scores))
	for key, score := range scores {
		out = append(out, passagePair{subject: key[0], related: key[1], score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		if out[i].subject != out[j].subject {
			return out[i].subject < out[j].subject
		}
		return out[i].related < out[j].related
	})
	return out
}

// buildDuplicate turns the matched pairs of two documents into a finding.
//
// The finding is oriented so that its subject is the document more fully
// contained in the other — the one somebody would consider removing — and its
// overlap ratio is that document's. Both depend only on the pairs, so the
// finding reads the same whichever document triggered the check.
func buildDuplicate(kbID, aID string, a map[string]passage, bID string, b map[string]passage,
	pairs []passagePair,
) Candidate {
	matchedA, matchedB := map[string]bool{}, map[string]bool{}
	for _, p := range pairs {
		matchedA[p.subject] = true
		matchedB[p.related] = true
	}
	overlapA := float64(len(matchedA)) / float64(len(a))
	overlapB := float64(len(matchedB)) / float64(len(b))

	subjectID, relatedID := aID, bID
	subject, related := a, b
	overlap := overlapA
	flipped := false
	if overlapB > overlapA || (overlapB == overlapA && bID < aID) {
		subjectID, relatedID = bID, aID
		subject, related = b, a
		overlap = overlapB
		flipped = true
	}

	// Each pair in the finding's orientation, compared as text: whether the
	// two passages are copies or differ decides what the finding is.
	oriented := make([]orientedPair, 0, len(pairs))
	divergent := false
	for _, p := range pairs {
		s, r := p.subject, p.related
		if flipped {
			s, r = r, s
		}
		c := comparePassages(subject[s].content, related[r].content)
		divergent = divergent || c.Differs
		oriented = append(oriented, orientedPair{subject: s, related: r, score: p.score, cmp: c})
	}

	evidence := make([]types.FindingEvidence, 0, duplicateEvidencePairs)
	usedSubject, usedRelated := map[string]bool{}, map[string]bool{}
	pick := func(requireDistinct, differing bool) {
		for _, p := range oriented {
			if len(evidence) >= duplicateEvidencePairs {
				return
			}
			if differing && !p.cmp.Differs {
				continue
			}
			if usedSubject[p.subject] && usedRelated[p.related] {
				continue
			}
			if requireDistinct && (usedSubject[p.subject] || usedRelated[p.related]) {
				continue
			}
			usedSubject[p.subject], usedRelated[p.related] = true, true
			e := types.FindingEvidence{
				SubjectChunkID: p.subject, RelatedChunkID: p.related, Score: p.score, Differs: p.cmp.Differs,
				SubjectExcerpt: excerpt(subject[p.subject].content),
				RelatedExcerpt: excerpt(related[p.related].content),
			}
			if p.cmp.Differs {
				e.SubjectExcerpt = excerptAround(subject[p.subject].content, p.cmp.SubjectAt)
				e.RelatedExcerpt = excerptAround(related[p.related].content, p.cmp.RelatedAt)
			}
			evidence = append(evidence, e)
		}
	}
	// The differences first, when there are any: they are what somebody has
	// to decide about. Then different passages before the same passage
	// again: three pairs showing one paragraph matched three times say less
	// than three paragraphs matched once.
	if divergent {
		pick(true, true)
		pick(false, true)
	}
	pick(true, false)
	pick(false, false)

	c := Candidate{
		Type: types.FindingTypeDuplicate, Severity: types.FindingSeverityInfo, Assign: AssignLatestHand,
		SubjectKnowledgeID: subjectID, RelatedKnowledgeID: relatedID,
		Score:       pairs[0].score,
		Fingerprint: PairFingerprint(types.FindingTypeDuplicate, kbID, aID, bID),
		Details: types.FindingDetails{
			Evidence: evidence, OverlapRatio: overlap,
			EvidenceHash: evidenceHash(aID, a, bID, b, pairs),
		},
	}
	switch {
	case divergent:
		// Always a warning: whichever account is wrong is answering
		// questions now.
		c.Type, c.Severity, c.Assign = types.FindingTypeDivergent, types.FindingSeverityWarning, AssignStalestOwner
		c.Fingerprint = PairFingerprint(types.FindingTypeDivergent, kbID, aID, bID)
	case overlap >= duplicateWarningOverlap:
		c.Severity = types.FindingSeverityWarning
	}
	return c
}

// orientedPair is a matched pair in the finding's orientation, with the
// outcome of comparing its passages as text.
type orientedPair struct {
	subject, related string
	score            float64
	cmp              passageComparison
}

// evidenceHash summarises the matched text, not the chunk IDs: re-parsing a
// document gives its chunks new IDs without changing a word, and a dismissed
// finding must not reopen for that. Each pair is written with the passage of
// the document whose ID sorts first on the left, so the hash does not depend
// on the finding's orientation either.
func evidenceHash(aID string, a map[string]passage, bID string, b map[string]passage, pairs []passagePair) string {
	lines := make([]string, 0, len(pairs))
	for _, p := range pairs {
		left, right := contentHash(a[p.subject].content), contentHash(b[p.related].content)
		if bID < aID {
			left, right = right, left
		}
		lines = append(lines, left+"|"+right)
	}
	sort.Strings(lines)
	lines = dedupeSorted(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func dedupeSorted(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// excerpt shortens a passage for display to at most duplicateExcerptRunes
// runes, marking a cut with an ellipsis.
func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= duplicateExcerptRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:duplicateExcerptRunes-1]) + "…"
}
