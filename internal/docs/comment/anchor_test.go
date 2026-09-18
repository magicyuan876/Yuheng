package comment

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wellFormed = `{
	"start": {"type": {"client": 1, "clock": 2}, "item": {"client": 1, "clock": 7}, "assoc": 0},
	"end":   {"type": {"client": 1, "clock": 2}, "item": {"client": 1, "clock": 19}, "assoc": -1}
}`

func TestAWellFormedAnchorIsKept(t *testing.T) {
	anchor, err := ParseAnchor([]byte(wellFormed))
	require.NoError(t, err)

	require.NotNil(t, anchor.Start.Item)
	assert.Equal(t, uint64(7), anchor.Start.Item.Clock)
	assert.Equal(t, -1, anchor.End.Assoc, "which side of the character it binds to is kept")
}

// A page comment and a broken anchor mean different things to a reader, so
// the absent case has its own error rather than being a parse failure.
func TestAnAbsentAnchorIsNotAnError(t *testing.T) {
	for _, raw := range []string{"", "   ", "null"} {
		_, err := ParseAnchor([]byte(raw))
		assert.ErrorIs(t, err, ErrNoAnchor, "%q", raw)
	}
}

func TestPlacementTellsThemApart(t *testing.T) {
	assert.Equal(t, Page, PlacementOf(nil))
	assert.Equal(t, Page, PlacementOf([]byte("null")))
	assert.Equal(t, Inline, PlacementOf([]byte(wellFormed)))

	// An anchor that no longer parses belongs to a comment that has lost its
	// place, not its subject. Calling it a page comment would silently move
	// somebody's remark about one paragraph onto the whole document.
	assert.Equal(t, Inline, PlacementOf([]byte(`{"start": "nonsense"}`)))
}

// The value is stored as JSON and handed to every reader of the page, so an
// unchecked field is a way to put arbitrary data into somebody else's editor.
func TestAnythingThatIsNotAnAnchorIsRefused(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`"a string"`,
		`42`,
		`{"start": 1, "end": 2}`,
		`{`,
		`{"start": {"assoc": 0}, "end": {"assoc": 0}} {"start": {"assoc": 0}, "end": {"assoc": 0}}`,
	} {
		_, err := ParseAnchor([]byte(raw))
		assert.Error(t, err, raw)
		assert.NotErrorIs(t, err, ErrNoAnchor, raw)
	}
}

// A field this build does not know is either a newer client's — which this
// server must not claim to have stored faithfully — or somebody's payload.
func TestAnUnknownFieldIsRefusedRatherThanDropped(t *testing.T) {
	_, err := ParseAnchor([]byte(`{"start": {"assoc": 0}, "end": {"assoc": 0}, "payload": "x"}`))
	require.Error(t, err)

	_, err = ParseAnchor([]byte(`{"start": {"assoc": 0, "extra": 1}, "end": {"assoc": 0}}`))
	require.Error(t, err, "including one nested inside a position")
}

func TestAnOversizedAnchorIsRefused(t *testing.T) {
	padded := `{"start": {"tname": "` + strings.Repeat("x", MaxAnchorBytes) + `", "assoc": 0}, "end": {"assoc": 0}}`
	_, err := ParseAnchor([]byte(padded))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bytes")
}

// Yjs spells "the very beginning of this type" with neither an item nor a
// name, so an anchor with both absent is legitimate.
func TestAPositionAtTheVeryBeginningIsValid(t *testing.T) {
	_, err := ParseAnchor([]byte(`{"start": {"assoc": 0}, "end": {"assoc": 0}}`))
	assert.NoError(t, err)
}

func TestAPositionNamingAnEmptyTypeIsRefused(t *testing.T) {
	_, err := ParseAnchor([]byte(`{"start": {"tname": "  ", "assoc": 0}, "end": {"assoc": 0}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start")

	_, err = ParseAnchor([]byte(`{"start": {"assoc": 0}, "end": {"tname": "", "assoc": 0}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end", "and the side is named, so the message is useful")
}

// ---- the quotation kept beside an anchor -------------------------------------

// Collapsing whitespace is what makes the fallback survive an edit that only
// re-wrapped a paragraph.
func TestAQuotationIsNormalised(t *testing.T) {
	assert.Equal(t, "one two three", CleanQuotedText("  one\n\ttwo   three  "))
	assert.Equal(t, "", CleanQuotedText("   \n\t "))
	assert.Equal(t, "already clean", CleanQuotedText("already clean"))
}

func TestAQuotationIsBounded(t *testing.T) {
	long := strings.Repeat("a", MaxQuotedRunes+50)
	assert.Equal(t, MaxQuotedRunes, len([]rune(CleanQuotedText(long))))
}

// Truncating on bytes would end a quotation inside a character and produce
// text no search could match.
func TestTruncationDoesNotSplitACharacter(t *testing.T) {
	long := strings.Repeat("文", MaxQuotedRunes+50)
	got := CleanQuotedText(long)

	assert.Equal(t, MaxQuotedRunes, len([]rune(got)))
	assert.True(t, len(got) > MaxQuotedRunes, "each of those characters is several bytes")
	for _, r := range got {
		assert.Equal(t, '文', r)
	}
}

func TestErrNoAnchorIsDistinguishable(t *testing.T) {
	_, err := ParseAnchor(nil)
	assert.True(t, errors.Is(err, ErrNoAnchor))
}
