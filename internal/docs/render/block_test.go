package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// para builds a paragraph carrying a block id and one run of text.
func blockPara(blockID, text string) *schema.Node {
	n := &schema.Node{
		Type:    schema.NodeParagraph,
		Content: []*schema.Node{{Type: schema.NodeText, Text: text}},
	}
	if blockID != "" {
		n.Attrs = map[string]any{"id": blockID}
	}
	return n
}

func blockDoc(children ...*schema.Node) *schema.Node {
	return &schema.Node{Type: schema.NodeDoc, Content: children}
}

func TestFindBlockReturnsTheBlockWithThatID(t *testing.T) {
	d := blockDoc(blockPara("aaa", "first"), blockPara("bbb", "second"))

	got, ok := FindBlock(d, "bbb")
	require.True(t, ok)
	assert.Equal(t, "second", Text(got))
}

func TestFindBlockLooksInsideNestedStructure(t *testing.T) {
	inner := blockPara("deep", "buried")
	d := blockDoc(&schema.Node{
		Type:    schema.NodeBlockquote,
		Attrs:   map[string]any{"id": "quote"},
		Content: []*schema.Node{inner},
	})

	got, ok := FindBlock(d, "deep")
	require.True(t, ok)
	assert.Equal(t, "buried", Text(got))
}

func TestFindBlockMissesAreReportedRatherThanGuessed(t *testing.T) {
	d := blockDoc(blockPara("aaa", "first"))

	_, ok := FindBlock(d, "nope")
	assert.False(t, ok)

	_, ok = FindBlock(d, "")
	assert.False(t, ok, "an empty id matches nothing, not the first block without one")

	_, ok = FindBlock(nil, "aaa")
	assert.False(t, ok)
}

// A block id is unique within a document, but nothing stops somebody pasting
// a block from one page into another. Deterministic beats clever: the first
// in document order wins, rather than the reference failing because of an
// edit somewhere else on the page.
func TestFindBlockTakesTheFirstOfDuplicateIDs(t *testing.T) {
	d := blockDoc(blockPara("same", "original"), blockPara("same", "pasted copy"))

	got, ok := FindBlock(d, "same")
	require.True(t, ok)
	assert.Equal(t, "original", Text(got))
}

func TestFindBlocksWalksOnceForManyIDs(t *testing.T) {
	d := blockDoc(blockPara("a", "one"), blockPara("b", "two"), blockPara("c", "three"))

	got := FindBlocks(d, []string{"a", "c", "missing"})
	require.Len(t, got, 2)
	assert.Equal(t, "one", Text(got["a"]))
	assert.Equal(t, "three", Text(got["c"]))
	assert.NotContains(t, got, "missing")
}

func TestFindBlocksWithNothingToFind(t *testing.T) {
	d := blockDoc(blockPara("a", "one"))

	assert.Empty(t, FindBlocks(d, nil))
	assert.Empty(t, FindBlocks(d, []string{""}))
	assert.Empty(t, FindBlocks(nil, []string{"a"}))
}

// The rule that makes a cycle impossible rather than merely bounded: a block
// inside a reference belongs to another page and is not this page's to offer.
func TestFindBlockDoesNotDescendIntoAReference(t *testing.T) {
	reference := &schema.Node{
		Type: schema.NodeTransclusion,
		Attrs: map[string]any{
			"id": "ref", "sourcePageId": "page-2", "sourceBlockId": "hidden",
		},
		Content: []*schema.Node{blockPara("hidden", "belongs to another page")},
	}
	d := blockDoc(blockPara("own", "mine"), reference)

	_, ok := FindBlock(d, "hidden")
	assert.False(t, ok, "a block inside a reference is not this page's to offer")

	got, ok := FindBlock(d, "ref")
	require.True(t, ok, "the reference node itself is still findable")
	assert.Equal(t, schema.NodeTransclusion, got.Type)
}

func TestClipTransclusionsLeavesAnOrdinaryBlockAlone(t *testing.T) {
	original := blockPara("a", "just words")

	clipped := ClipTransclusions(original)
	require.NotNil(t, clipped)
	assert.Equal(t, "just words", Text(clipped))
	assert.Equal(t, "a", clipped.Attrs["id"])
}

func TestClipTransclusionsCopiesRatherThanSharing(t *testing.T) {
	original := blockPara("a", "before")

	clipped := ClipTransclusions(original)
	clipped.Attrs["id"] = "changed"
	clipped.Content[0].Text = "after"

	assert.Equal(t, "a", original.Attrs["id"], "the document must not move under the caller")
	assert.Equal(t, "before", original.Content[0].Text)
}

func TestClipTransclusionsReplacesANestedReference(t *testing.T) {
	quote := &schema.Node{
		Type:  schema.NodeBlockquote,
		Attrs: map[string]any{"id": "quote"},
		Content: []*schema.Node{
			blockPara("kept", "this stays"),
			{Type: schema.NodeTransclusion, Attrs: map[string]any{
				"sourcePageId": "page-3", "sourceBlockId": "x",
			}},
		},
	}

	clipped := ClipTransclusions(quote)
	require.Len(t, clipped.Content, 2, "the structure keeps its shape")
	assert.Equal(t, schema.NodeParagraph, clipped.Content[1].Type)
	assert.Empty(t, clipped.Content[1].Content, "and the reference leaves nothing behind")
	assert.Equal(t, "this stays", Text(clipped.Content[0]))
}

func TestClipTransclusionsOnAReferenceItself(t *testing.T) {
	clipped := ClipTransclusions(&schema.Node{
		Type:  schema.NodeTransclusion,
		Attrs: map[string]any{"sourcePageId": "p", "sourceBlockId": "b"},
	})

	require.NotNil(t, clipped)
	assert.Equal(t, schema.NodeParagraph, clipped.Type)
	assert.Nil(t, ClipTransclusions(nil))
}

func TestClipTransclusionsKeepsMarks(t *testing.T) {
	original := &schema.Node{
		Type:  schema.NodeParagraph,
		Attrs: map[string]any{"id": "a"},
		Content: []*schema.Node{{
			Type:  schema.NodeText,
			Text:  "bold words",
			Marks: []schema.Mark{{Type: schema.MarkBold}},
		}},
	}

	clipped := ClipTransclusions(original)
	require.Len(t, clipped.Content[0].Marks, 1)
	assert.Equal(t, schema.MarkBold, clipped.Content[0].Marks[0].Type)
}
