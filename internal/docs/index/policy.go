// Package index decides what of a space reaches its knowledge base.
//
// A space can be bound to a knowledge base (T1.1), which until now did
// nothing: the binding was stored, and no page ever arrived. This package is
// the rule that closes that gap, and the rule is mostly about what NOT to
// send.
//
// ---- decision: only unrestricted pages are indexed
//
// Yuheng's retrieval pipeline has no per-entry permission filter. Anything in
// a knowledge base is answerable to anybody who may query that knowledge
// base. A restricted page — one whose whole point is that a subset of the
// space may read it — would therefore become readable, through retrieval, by
// everybody the knowledge base is exposed to. That is not a degraded
// experience; it is the leak the entire page-permission layer exists to
// prevent, arriving by a side door.
//
// So a page is indexed only when it is visible to its whole space. Who a page
// is visible to can be worked out (acl.PageSubjects), but nothing stores it
// with the entry, because nothing yet reads it: the day retrieval learns to
// filter by subject is the day it has to be stored, and this rule can be
// relaxed. Until then the rule is the simple one, because a simple rule is one
// an auditor can check.
//
// ---- decision: drafts and the trash are not indexed either
//
// A draft is somebody's unfinished thought, and the trash is a decision to
// remove something. Neither belongs in an answer given to a colleague who
// asked a question, and both are cheap to exclude.
package index

import (
	"strings"
)

// Decision is why a page is or is not indexed. Returned rather than a bare
// bool so that a caller — or an operator reading a log — is told which rule
// applied, instead of being left to guess.
type Decision struct {
	Index bool
	// Reason names the rule. Empty when Index is true.
	Reason string
}

// Reasons a page is kept out. Stable strings: they appear in logs and in the
// per-space indexing report.
const (
	ReasonRestricted = "restricted"
	ReasonTrashed    = "trashed"
	ReasonDraft      = "draft"
	ReasonEmpty      = "empty"
	ReasonNoBinding  = "no-knowledge-base"
)

// Candidate is what the policy needs to know about a page.
type Candidate struct {
	// Restricted is true when the page, or any ancestor, cuts permission
	// inheritance.
	Restricted bool
	// Trashed is true for a page in the bin.
	Trashed bool
	// Draft is true for a page its author has not published.
	Draft bool
	// Text is the page's plain-text content.
	Text string
	// BoundKB is the knowledge base the space is bound to; empty means the
	// space is not bound to one.
	BoundKB string
}

// MinIndexableRunes is the shortest page worth indexing.
//
// A page holding a title and two words adds nothing to retrieval but does
// add a row somebody has to scroll past in the knowledge base. Five is
// arbitrary but deliberately low: the cost of indexing something trivial is
// small, and the cost of silently skipping something somebody wrote is not.
const MinIndexableRunes = 5

// Decide applies the rules above, in the order an operator would explain
// them: no binding, then gone, then not-for-sharing, then too small.
func Decide(c Candidate) Decision {
	if strings.TrimSpace(c.BoundKB) == "" {
		return Decision{Reason: ReasonNoBinding}
	}
	if c.Trashed {
		return Decision{Reason: ReasonTrashed}
	}
	if c.Restricted {
		return Decision{Reason: ReasonRestricted}
	}
	if c.Draft {
		return Decision{Reason: ReasonDraft}
	}
	if len([]rune(strings.TrimSpace(c.Text))) < MinIndexableRunes {
		return Decision{Reason: ReasonEmpty}
	}
	return Decision{Index: true}
}
