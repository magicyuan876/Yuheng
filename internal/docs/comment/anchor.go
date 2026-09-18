// Package comment holds the decisions behind inline comments: what an anchor
// may contain, and what a comment's body is allowed to be.
//
// An anchor is a Yjs relative position, produced by the editor and handed
// back verbatim. The server never resolves one — that needs the Yjs document,
// which lives in the collaboration service and in the browser — but it does
// have to decide whether what arrived is an anchor at all.
//
// That check matters more than it looks. The value is stored as JSON and
// handed to every reader of the page, so an unchecked field is a way to put
// arbitrary data of arbitrary size into somebody else's editor. So the shape
// is pinned, the size is bounded, and anything else is refused rather than
// stored and puzzled over later.
package comment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxAnchorBytes bounds one stored anchor. A relative position is a handful
// of integers; anything approaching this is not one.
const MaxAnchorBytes = 2048

// MaxQuotedRunes bounds the quotation kept beside an anchor.
//
// The quotation is a fallback for finding the text again when the relative
// position no longer resolves, not a copy of the passage: a few lines is
// enough to locate it and short enough that a comment thread does not become
// a second copy of the document.
const MaxQuotedRunes = 250

// ErrNoAnchor is returned for an absent anchor, which is not an error at the
// call sites that allow a page-level comment.
var ErrNoAnchor = errors.New("docs: no anchor")

// Anchor is the editor's relative position for a commented range.
//
// The field names mirror what Yjs itself encodes, so the editor can hand its
// own value straight through without a translation layer nobody would keep in
// step.
type Anchor struct {
	Start RelativePosition `json:"start"`
	End   RelativePosition `json:"end"`
}

// RelativePosition is one end of a range, as Yjs describes it.
//
// A position is either inside an item (`item` set) or at the start of a named
// type (`tname` set); both being absent is how Yjs spells "the very
// beginning", so neither is required on its own.
type RelativePosition struct {
	// Type identifies the Yjs type the position belongs to.
	Type *YjsID `json:"type,omitempty"`
	// TName is the root type's name, when the position is in one.
	TName *string `json:"tname,omitempty"`
	// Item is the item the position sits in.
	Item *YjsID `json:"item,omitempty"`
	// Assoc says which side of the character the position sticks to: a
	// negative value binds to the character before, anything else to the one
	// after. That is what decides whether text typed at the boundary lands
	// inside the comment or outside it.
	Assoc int `json:"assoc"`
}

// YjsID is Yjs's (client, clock) pair.
type YjsID struct {
	Client uint64 `json:"client"`
	Clock  uint64 `json:"clock"`
}

// ParseAnchor validates a stored or submitted anchor.
//
// Returns ErrNoAnchor for an empty value, so a caller can tell "this is a
// page-level comment" from "this anchor is broken" — the two mean different
// things to a reader and must not be collapsed.
func ParseAnchor(raw []byte) (*Anchor, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, ErrNoAnchor
	}
	if len(raw) > MaxAnchorBytes {
		return nil, fmt.Errorf("docs: an anchor may not exceed %d bytes", MaxAnchorBytes)
	}

	var anchor Anchor
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	// Unknown fields are refused rather than dropped: a field this build does
	// not know is either a newer client's, which this server must not claim to
	// have stored faithfully, or somebody's payload.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&anchor); err != nil {
		return nil, fmt.Errorf("docs: malformed anchor: %w", err)
	}
	if decoder.More() {
		return nil, errors.New("docs: an anchor must be a single object")
	}

	if err := anchor.Start.validate("start"); err != nil {
		return nil, err
	}
	if err := anchor.End.validate("end"); err != nil {
		return nil, err
	}
	return &anchor, nil
}

func (p RelativePosition) validate(side string) error {
	// "Neither" is legitimate — it is how Yjs spells the very beginning of a
	// type — but a name that is present and empty is not.
	if p.TName != nil && strings.TrimSpace(*p.TName) == "" {
		return fmt.Errorf("docs: the %s position names an empty type", side)
	}
	if p.TName != nil && len(*p.TName) > 128 {
		return fmt.Errorf("docs: the %s position's type name is too long", side)
	}
	return nil
}

// CleanQuotedText normalises the quotation stored beside an anchor.
//
// Collapsing whitespace is what makes the fallback work across an edit that
// only re-wrapped a paragraph; truncation is on runes rather than bytes so a
// quotation never ends inside a character.
func CleanQuotedText(raw string) string {
	collapsed := strings.Join(strings.Fields(raw), " ")
	if utf8.RuneCountInString(collapsed) <= MaxQuotedRunes {
		return collapsed
	}
	runes := []rune(collapsed)
	return string(runes[:MaxQuotedRunes])
}

// Placement says how a comment attaches to the page.
type Placement string

const (
	// Inline is a comment on a passage, with an anchor.
	Inline Placement = "inline"
	// Page is a comment on the page as a whole.
	Page Placement = "page"
)

// PlacementOf reports how a comment is attached, from what was stored.
//
// A comment with no anchor is a page comment. One whose anchor is present but
// unreadable is still inline — it has lost its place, not its subject — and
// the editor shows it against its quotation. Collapsing that into "page
// comment" would silently move somebody's remark about one paragraph onto the
// whole document.
func PlacementOf(anchor []byte) Placement {
	if _, err := ParseAnchor(anchor); errors.Is(err, ErrNoAnchor) {
		return Page
	}
	return Inline
}
