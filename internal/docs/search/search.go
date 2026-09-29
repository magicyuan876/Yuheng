// Package search holds the parts of document search that are pure
// functions: what a query means, how a match is scored, and what excerpt a
// reader is shown.
//
// ---- decision: substring matching, not the tsvector column
//
// docs_pages carries a generated tsvector built with Postgres's `simple`
// configuration, and a GIN index over it. Search does NOT use it as its
// primary path, and that is deliberate rather than an oversight:
//
// `simple` splits on whitespace and punctuation. For English that is a word
// index. For Chinese — which is what most of this product's documents are
// written in — a sentence contains no spaces, so the whole sentence becomes
// one token, and a search for 「配额」 matches nothing at all unless somebody
// happened to write that word surrounded by spaces. A full-text index that
// silently returns nothing for the majority language is worse than no index,
// because it looks like it is working.
//
// Proper CJK tokenisation needs a Postgres extension (zhparser, pg_jieba)
// that a customer's managed database may not allow, and this module has to
// run on whatever the deployment has. So the primary path is a substring
// match, which is correct in every language, and the deployment's optional
// pg_trgm index accelerates it where the extension exists. The tsvector
// column stays because it costs nothing and an English-only deployment may
// want it later.
//
// If somebody ever "optimises" this into a to_tsvector query, Chinese search
// stops working and no test in the Go layer will notice unless it is written
// against real text — which is why the tests here use Chinese.
package search

import (
	"sort"
	"strings"
	"unicode"
)

// MaxQueryRunes bounds a query. Past this it is not a search, it is somebody
// pasting a document into the box.
const MaxQueryRunes = 128

// MinQueryRunes is the shortest query worth running.
//
// One character is a legitimate search in Chinese, where a single character
// is a word, so this is 1 rather than the 3 an English-only product would
// choose. The cost is a broad scan on a one-character query; the cost of the
// alternative is that a Chinese user cannot search at all.
const MinQueryRunes = 1

// Query is a cleaned search request.
type Query struct {
	// Text is the normalised query, as typed.
	Text string
	// Pattern is Text escaped for a SQL LIKE, without its surrounding %.
	Pattern string
	// Terms are the whitespace-separated pieces, for scoring and
	// highlighting. A CJK query with no spaces is one term, which is right.
	Terms []string
}

// Empty reports whether there is nothing to search for.
func (q Query) Empty() bool { return q.Text == "" }

// Parse cleans what somebody typed.
//
// Returns an empty query rather than an error for input that cannot be
// searched: an empty result is the honest answer to "find nothing", and
// making the caller distinguish that from a failure buys nothing.
func Parse(raw string) Query {
	text := strings.TrimSpace(raw)
	if text == "" {
		return Query{}
	}
	if runes := []rune(text); len(runes) > MaxQueryRunes {
		text = strings.TrimSpace(string(runes[:MaxQueryRunes]))
	}
	if len([]rune(text)) < MinQueryRunes {
		return Query{}
	}
	return Query{
		Text:    text,
		Pattern: EscapeLike(text),
		Terms:   terms(text),
	}
}

// terms splits a query for scoring. Whitespace-separated, and a run of CJK
// with no spaces stays whole: splitting it per character would make every
// document match every query.
func terms(text string) []string {
	fields := strings.Fields(text)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

// EscapeLike makes a literal safe inside a LIKE pattern.
//
// Every caller must pair this with an explicit ESCAPE clause, so the result
// does not depend on the server's default escape character — the same rule
// the page title search has followed since T2.2.
func EscapeLike(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '%', '_':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Kind is what a hit is.
type Kind string

const (
	// KindPage is a hit in a page's title or body.
	KindPage Kind = "page"
	// KindComment is a hit in a comment on a page. Comments are searchable
	// because a decision recorded in a comment thread is as much a part of
	// the record as one recorded in the document.
	KindComment Kind = "comment"
	// KindTransclusion is a hit in text a page shows by reference. The hit
	// belongs to the page doing the referencing, but it is only shown to
	// somebody who may read the page the text came FROM.
	KindTransclusion Kind = "transclusion"
)

// Field is where in a record a term was found, which is most of the score.
type Field string

const (
	FieldTitle Field = "title"
	FieldBody  Field = "body"
)

// Weights, highest first. A term in a title is a much stronger signal than
// one in a body: somebody searching "quota" wants the page called Quota
// before the twelve pages that mention one.
const (
	weightTitle        = 100.0
	weightBody         = 10.0
	weightComment      = 6.0
	weightTransclusion = 4.0
	// exactBonus rewards a whole-query match over a scattering of its terms.
	exactBonus = 25.0
	// allTermsBonus rewards a record containing every term.
	allTermsBonus = 15.0
)

// Score rates a candidate against a query.
//
// Deliberately simple and entirely in Go rather than in SQL: the candidates
// come from several queries (pages, comments, blocks), and one scoring
// function over all of them keeps their ranking comparable and testable
// without a database.
func Score(q Query, kind Kind, title, body string) float64 {
	if q.Empty() {
		return 0
	}
	lowerTitle, lowerBody := strings.ToLower(title), strings.ToLower(body)
	lowerText := strings.ToLower(q.Text)

	var score float64
	base := weightBody
	switch kind {
	case KindComment:
		base = weightComment
	case KindTransclusion:
		base = weightTransclusion
	}

	found := 0
	for _, term := range q.Terms {
		lower := strings.ToLower(term)
		if strings.Contains(lowerTitle, lower) {
			score += weightTitle
			found++
			continue
		}
		if strings.Contains(lowerBody, lower) {
			score += base
			found++
		}
	}
	if found == 0 {
		return 0
	}
	if found == len(q.Terms) && len(q.Terms) > 1 {
		score += allTermsBonus
	}
	if strings.Contains(lowerTitle, lowerText) || strings.Contains(lowerBody, lowerText) {
		score += exactBonus
	}
	return score
}

// Sortable is what Sort needs to know about a result. The service layer owns
// the result type — it carries ids and excerpts this package has no use for —
// so ordering is expressed over the three fields that decide it rather than
// over a type duplicated in two places.
type Sortable interface {
	// SortKey returns the score, the title, and a stable tie-break id.
	SortKey() (score float64, title, id string)
}

// Sort orders hits by score, then title, then id, so that two runs of the
// same search return the same order. An unstable order makes a result list
// that shifts under the reader between one keystroke and the next — and the
// id is in there because two untitled pages with equal scores would
// otherwise swap places between requests.
func Sort[T Sortable](hits []T) {
	sort.SliceStable(hits, func(i, j int) bool {
		scoreA, titleA, idA := hits[i].SortKey()
		scoreB, titleB, idB := hits[j].SortKey()
		if scoreA != scoreB {
			return scoreA > scoreB
		}
		if titleA != titleB {
			return titleA < titleB
		}
		return idA < idB
	})
}

// ExcerptRunes is how much context an excerpt carries.
const ExcerptRunes = 120

// Excerpt returns the part of body around the first match, with an ellipsis
// where text was cut.
//
// Measured in runes rather than bytes throughout: a byte-based window cuts a
// Chinese character in half and renders as a replacement glyph, which is the
// classic way this function is got wrong.
func Excerpt(q Query, body string) string {
	body = collapseSpace(body)
	runes := []rune(body)
	if len(runes) <= ExcerptRunes {
		return body
	}
	at := matchIndex(q, body)
	if at < 0 {
		return string(runes[:ExcerptRunes]) + "…"
	}

	// Centre the window on the match, then pull it back inside the text.
	start := at - ExcerptRunes/3
	if start < 0 {
		start = 0
	}
	end := start + ExcerptRunes
	if end > len(runes) {
		end = len(runes)
		start = end - ExcerptRunes
		if start < 0 {
			start = 0
		}
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

// matchIndex is the rune offset of the first term found in body, or -1.
func matchIndex(q Query, body string) int {
	lower := strings.ToLower(body)
	best := -1
	for _, term := range append([]string{q.Text}, q.Terms...) {
		at := strings.Index(lower, strings.ToLower(term))
		if at < 0 {
			continue
		}
		// Byte offset to rune offset: the window is measured in runes.
		runeAt := len([]rune(body[:at]))
		if best < 0 || runeAt < best {
			best = runeAt
		}
	}
	return best
}

// collapseSpace turns runs of whitespace into single spaces, so an excerpt
// of a document full of newlines reads as a sentence rather than a column.
func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}
