package render

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// portableOf parses a document, makes it portable and validates the result,
// because a template that cannot be stored is not a template.
func portableOf(t *testing.T, raw string) *schema.Node {
	t.Helper()
	doc, _, err := schema.Default().Validate(json.RawMessage(raw))
	require.NoError(t, err, "fixture must be valid to begin with")

	out := Portable(doc)
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	_, _, err = schema.Default().Validate(encoded)
	require.NoError(t, err, "a portable document must still be storable")
	return out
}

// plain is the document's text, for asserting what survived.
func plain(doc *schema.Node) string { return Text(doc) }

func TestOrdinaryContentTravelsUnchanged(t *testing.T) {
	raw := `{"type":"doc","content":[
		{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Agenda"}]},
		{"type":"paragraph","content":[{"type":"text","text":"Discuss the thing."}]},
		{"type":"bulletList","content":[{"type":"listItem","content":[
			{"type":"paragraph","content":[{"type":"text","text":"One"}]}]}]}]}`
	out := portableOf(t, raw)
	assert.Contains(t, plain(out), "Agenda")
	assert.Contains(t, plain(out), "Discuss the thing.")
	assert.Contains(t, plain(out), "One")
}

// A link to one specific page either dangles in a new document or quietly
// points at somebody else's page.
func TestAPageLinkDoesNotTravel(t *testing.T) {
	raw := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"See "},
		{"type":"pageLink","attrs":{"pageId":"page-abcdefgh"}},
		{"type":"text","text":" for details."}]}]}`
	out := portableOf(t, raw)

	assert.False(t, has(out, "pageLink"))
	body := plain(out)
	assert.Contains(t, body, "See ")
	assert.Contains(t, body, "for details.")
	assert.NotContains(t, body, "page-abcdefgh")
}

// A template that greets one person by name every time it is used is wrong
// in a way nobody notices until it is embarrassing.
func TestAMentionBecomesItsLabel(t *testing.T) {
	raw := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Owner: "},
		{"type":"mention","attrs":{"userId":"user-abcdefgh","label":"Alice"}}]}]}`
	out := portableOf(t, raw)

	assert.False(t, has(out, "mention"))
	assert.Contains(t, plain(out), "Alice", "the word survives")
	assert.NotContains(t, plain(out), "user-abcdefgh", "the pointer does not")
}

func TestAMentionWithNoLabelIsDroppedRatherThanLeftBlank(t *testing.T) {
	raw := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Owner: "},
		{"type":"mention","attrs":{"userId":"user-abcdefgh"}}]}]}`
	out := portableOf(t, raw)
	assert.False(t, has(out, "mention"))
	assert.Contains(t, plain(out), "Owner:")
}

// A block reference points at one block of one page, and the reader of the
// new document may not be allowed to see it at all.
func TestABlockReferenceDoesNotTravel(t *testing.T) {
	raw := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"Before"}]},
		{"type":"transclusion","attrs":{"sourcePageId":"page-abcdefgh","sourceBlockId":"block-1"}},
		{"type":"paragraph","content":[{"type":"text","text":"After"}]}]}`
	out := portableOf(t, raw)

	assert.False(t, has(out, "transclusion"))
	assert.Contains(t, plain(out), "Before")
	assert.Contains(t, plain(out), "After")
}

// An attachment belongs to the space it was uploaded to; that is what makes
// its permissions and its cleanup tractable.
func TestAttachmentsDoNotTravel(t *testing.T) {
	raw := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"Above"}]},
		{"type":"image","attrs":{"attachmentId":"file-abcdefgh"}},
		{"type":"attachment","attrs":{"attachmentId":"file-ijklmnop","name":"budget.xlsx"}},
		{"type":"paragraph","content":[{"type":"text","text":"Below"}]}]}`
	out := portableOf(t, raw)

	assert.False(t, has(out, "image"))
	assert.False(t, has(out, "attachment"))
	assert.Contains(t, plain(out), "Above")
	assert.Contains(t, plain(out), "Below")
}

// An empty paragraph where a pointer was is a visible reminder that
// something belonged there, which is what somebody editing the template
// needs to see.
func TestABlockEmptiedOfItsOnlyChildSurvivesAsAnEmptyOne(t *testing.T) {
	raw := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"pageLink","attrs":{"pageId":"page-abcdefgh"}}]},
		{"type":"paragraph","content":[{"type":"text","text":"After"}]}]}`
	out := portableOf(t, raw)
	require.Len(t, out.Content, 2, "the emptied block is still there")
	assert.Equal(t, "paragraph", out.Content[0].Type)
	assert.Empty(t, out.Content[0].Content)
	assert.Contains(t, plain(out), "After")
}

// A whole block that cannot travel goes, rather than leaving a husk: unlike
// an inline pointer inside a paragraph, there is no surrounding block whose
// shape would tell the reader something was removed.
func TestADroppedBlockLeavesNothingBehind(t *testing.T) {
	raw := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"Above"}]},
		{"type":"image","attrs":{"attachmentId":"file-abcdefgh"}},
		{"type":"paragraph","content":[{"type":"text","text":"Below"}]}]}`
	out := portableOf(t, raw)
	require.Len(t, out.Content, 2)
	assert.Contains(t, plain(out), "Above")
	assert.Contains(t, plain(out), "Below")
}

func TestNestedStructureIsWalkedAllTheWayDown(t *testing.T) {
	raw := `{"type":"doc","content":[{"type":"bulletList","content":[
		{"type":"listItem","content":[{"type":"paragraph","content":[
			{"type":"text","text":"Item "},
			{"type":"pageLink","attrs":{"pageId":"page-abcdefgh"}}]}]}]}]}`
	out := portableOf(t, raw)
	assert.False(t, has(out, "pageLink"))
	assert.Contains(t, plain(out), "Item")
}

func TestPortableDropsAnswersBeforeAnythingIsRemoved(t *testing.T) {
	noPointers, _, err := schema.Default().Validate(json.RawMessage(
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`))
	require.NoError(t, err)
	assert.False(t, PortableDrops(noPointers))

	withLink, _, err := schema.Default().Validate(json.RawMessage(
		`{"type":"doc","content":[{"type":"paragraph","content":[
			{"type":"pageLink","attrs":{"pageId":"page-abcdefgh"}}]}]}`))
	require.NoError(t, err)
	assert.True(t, PortableDrops(withLink))
}

func TestPortableOfNothingIsNothing(t *testing.T) {
	assert.Nil(t, Portable(nil))
	assert.False(t, PortableDrops(nil))
}

// has reports whether a node type appears anywhere in the tree.
func has(n *schema.Node, typ string) bool {
	if n == nil {
		return false
	}
	if n.Type == typ {
		return true
	}
	for _, child := range n.Content {
		if has(child, typ) {
			return true
		}
	}
	return false
}
