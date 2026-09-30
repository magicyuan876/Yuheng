package findings

import (
	"strings"
	"unicode/utf8"
)

// Telling a copy from a near-copy.
//
// Embeddings cannot: a passage and the same passage with one number changed
// ("15 days of leave" / "10 days of leave") are, to the model, the same text —
// 0.97 cosine with bge-m3, above the duplicate threshold. Yet the two call for
// opposite actions. A copy is redundant, and the person who made it should link
// to the original instead. A near-copy that differs is a disagreement: one of
// the two is probably out of date, and deleting "the duplicate" might delete
// the right one.
//
// So every matched pair of passages is also compared as text. The comparison
// has to forgive one thing: two documents cut into passages at different
// places. A copied section chunked with different surroundings yields passages
// that share a long run of text and differ at their edges, where one passage
// starts earlier or ends later than the other. Differences at the edges are
// therefore ignored. A difference counts when it lies between two anchors: long
// runs of text the passages share, or the places where both begin or both end
// together. Passages without a single long shared run are rewordings of each
// other, and differ.

const (
	// divergenceAnchorRunes is the shortest shared run that anchors the
	// alignment. Shorter runs turn up by chance between unrelated edges
	// (a particle, a comma), and would pull those edges into the
	// comparison.
	divergenceAnchorRunes = 8
	// divergenceMaxEdits bounds the alignment. Passages that need more
	// insertions and deletions than this to turn into each other are
	// reworded rather than edited, which is a difference by any measure,
	// and aligning them precisely would only cost time.
	divergenceMaxEdits = 600
)

// passageComparison is the outcome of comparing two matched passages.
type passageComparison struct {
	// Differs is true when the passages differ inside the text they share.
	Differs bool
	// SubjectAt and RelatedAt are the rune offsets, in the
	// whitespace-normalised passages, of the first such difference.
	SubjectAt, RelatedAt int
}

// comparePassages compares two passages the embedding model found alike.
func comparePassages(subject, related string) passageComparison {
	a, b := []rune(normalizeSpace(subject)), []rune(normalizeSpace(related))
	if string(a) == string(b) {
		return passageComparison{}
	}
	ops, ok := diffRunes(a, b, divergenceMaxEdits)
	if !ok {
		at := commonPrefix(a, b)
		return passageComparison{Differs: true, SubjectAt: at, RelatedAt: at}
	}
	first, last, long := -1, -1, false
	for i, op := range ops {
		if op.kind != opEqual {
			continue
		}
		if op.aTo-op.aFrom >= divergenceAnchorRunes {
			long = true
		} else if !atCommonEdge(op, len(a), len(b)) {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if !long {
		// Nothing shared worth aligning on — a common full stop at the
		// end does not count: the passages say the same thing in other
		// words.
		return passageComparison{Differs: true}
	}
	for i := first + 1; i < last; i++ {
		if op := ops[i]; op.kind != opEqual {
			return passageComparison{Differs: true, SubjectAt: op.aFrom, RelatedAt: op.bFrom}
		}
	}
	return passageComparison{}
}

// atCommonEdge reports whether a shared run is where both passages begin or
// both end. Short as it may be, it pins the alignment there: the passages were
// not cut at different places on that side.
func atCommonEdge(op diffOp, lenA, lenB int) bool {
	return (op.aFrom == 0 && op.bFrom == 0) || (op.aTo == lenA && op.bTo == lenB)
}

// normalizeSpace collapses every run of white space into one space and trims
// the ends: line breaks and indentation are layout, not content, and the
// excerpts shown to people are normalised the same way, so offsets into one
// are offsets into the other.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func commonPrefix(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

type opKind byte

const (
	opEqual  opKind = '='
	opDelete opKind = '-' // present in a only
	opInsert opKind = '+' // present in b only
)

// diffOp is one run of an edit script: a[aFrom:aTo] and b[bFrom:bTo], of
// which one range is empty for a deletion or an insertion.
type diffOp struct {
	kind       opKind
	aFrom, aTo int
	bFrom, bTo int
}

// diffRunes computes a shortest edit script from a to b (Myers' algorithm)
// as runs of equal, deleted and inserted runes, in order. It gives up, with
// false, when the script needs more than maxEdits insertions and deletions.
//
// Its memory is quadratic in the number of edits rather than in the length of
// the text: for each edit count d it keeps the furthest-reaching paths on the
// diagonals -d-1..d+1 only, which is what backtracking reads.
func diffRunes(a, b []rune, maxEdits int) ([]diffOp, bool) {
	n, m := len(a), len(b)
	limit := min(n+m, maxEdits)
	off := limit + 1
	v := make([]int, 2*limit+3)
	var trace [][]int
	for d := 0; d <= limit; d++ {
		trace = append(trace, append([]int(nil), v[off-d-1:off+d+2]...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				return backtrack(trace, n, m), true
			}
		}
	}
	return nil, false
}

// backtrack walks the saved paths back from the end and returns the script.
// trace[d] holds, for diagonals k in -d-1..d+1 at index k+d+1, how far the
// paths with d-1 edits reached.
func backtrack(trace [][]int, n, m int) []diffOp {
	var rev []diffOp
	push := func(kind opKind, aFrom, aTo, bFrom, bTo int) {
		if last := len(rev) - 1; last >= 0 && rev[last].kind == kind &&
			rev[last].aFrom == aTo && rev[last].bFrom == bTo {
			rev[last].aFrom, rev[last].bFrom = aFrom, bFrom
			return
		}
		rev = append(rev, diffOp{kind: kind, aFrom: aFrom, aTo: aTo, bFrom: bFrom, bTo: bTo})
	}
	x, y := n, m
	for d := len(trace) - 1; d >= 0; d-- {
		saved := trace[d]
		at := func(k int) int { return saved[k+d+1] }
		k := x - y
		prevK := k - 1
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			prevK = k + 1
		}
		// At d = 0 this is the start: at(1) is 0, so prevY is -1 and the
		// diagonal below runs back to (0, 0).
		prevX := at(prevK)
		prevY := prevX - prevK
		if run := min(x-prevX, y-prevY); run > 0 {
			push(opEqual, x-run, x, y-run, y)
			x, y = x-run, y-run
		}
		if d == 0 {
			break
		}
		if x == prevX {
			push(opInsert, x, x, y-1, y)
		} else {
			push(opDelete, x-1, x, y, y)
		}
		x, y = prevX, prevY
	}
	ops := make([]diffOp, len(rev))
	for i, op := range rev {
		ops[len(rev)-1-i] = op
	}
	return ops
}

// excerptAround shortens a passage for display to at most
// duplicateExcerptRunes runes, keeping the rune at offset at (into the
// whitespace-normalised passage) in view with some context before it, and
// marking each cut with an ellipsis.
func excerptAround(s string, at int) string {
	s = normalizeSpace(s)
	if utf8.RuneCountInString(s) <= duplicateExcerptRunes {
		return s
	}
	runes := []rune(s)
	start := max(0, at-duplicateExcerptLead)
	end := min(len(runes), start+duplicateExcerptRunes)
	start = max(0, end-duplicateExcerptRunes)
	// Room for the ellipses comes out of the text, so the excerpt keeps
	// its length limit.
	if start > 0 {
		start++
	}
	if end < len(runes) {
		end--
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}
