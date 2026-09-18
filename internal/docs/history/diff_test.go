package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// kinds renders a line diff as a compact string, so a test reads as the
// answer rather than as a walk over a slice.
func kinds(changes []LineChange) string {
	out := ""
	for _, change := range changes {
		switch change.Kind {
		case Unchanged:
			out += "="
		case Added:
			out += "+"
		case Removed:
			out += "-"
		}
	}
	return out
}

func texts(changes []LineChange, want ChangeKind) []string {
	var out []string
	for _, change := range changes {
		if change.Kind == want {
			out = append(out, change.Text)
		}
	}
	return out
}

func TestIdenticalTextHasNoChanges(t *testing.T) {
	changes := DiffText("one\ntwo\nthree", "one\ntwo\nthree")
	assert.Equal(t, "===", kinds(changes))
	assert.Equal(t, DiffSummary{Unchanged: 3}, SummariseLines(changes))
}

func TestALineAddedInTheMiddle(t *testing.T) {
	changes := DiffText("one\nthree", "one\ntwo\nthree")
	assert.Equal(t, "=+=", kinds(changes))
	assert.Equal(t, []string{"two"}, texts(changes, Added))
}

func TestALineRemovedFromTheMiddle(t *testing.T) {
	changes := DiffText("one\ntwo\nthree", "one\nthree")
	assert.Equal(t, "=-=", kinds(changes))
	assert.Equal(t, []string{"two"}, texts(changes, Removed))
}

// A rewritten line is a removal and an addition, which is what a reader of a
// line diff expects to see.
func TestARewrittenLineIsRemovedAndAdded(t *testing.T) {
	changes := DiffText("one\ntwo\nthree", "one\nTWO\nthree")
	assert.Equal(t, "=-+=", kinds(changes))
	assert.Equal(t, []string{"two"}, texts(changes, Removed))
	assert.Equal(t, []string{"TWO"}, texts(changes, Added))
}

func TestLineNumbersPointAtBothSides(t *testing.T) {
	changes := DiffText("one\ntwo", "one\ninserted\ntwo")
	require.Len(t, changes, 3)

	assert.Equal(t, 1, changes[0].OldLine)
	assert.Equal(t, 1, changes[0].NewLine)

	assert.Equal(t, 0, changes[1].OldLine, "an added line is on no old line")
	assert.Equal(t, 2, changes[1].NewLine)

	assert.Equal(t, 2, changes[2].OldLine)
	assert.Equal(t, 3, changes[2].NewLine)
}

func TestEverythingAddedAndEverythingRemoved(t *testing.T) {
	assert.Equal(t, "++", kinds(DiffText("", "one\ntwo")))
	assert.Equal(t, "--", kinds(DiffText("one\ntwo", "")))
	assert.Empty(t, DiffText("", ""))
}

// "a\n" and "a" are the same one line; a trailing newline is punctuation.
func TestATrailingNewlineIsNotALine(t *testing.T) {
	assert.Equal(t, "=", kinds(DiffText("a\n", "a")))
	assert.Equal(t, "==", kinds(DiffText("a\nb\n", "a\nb")))
}

func TestWindowsLineEndingsCompareAsTheSameText(t *testing.T) {
	assert.Equal(t, "==", kinds(DiffText("a\r\nb", "a\nb")))
}

// The diff is found rather than approximated: moving a line to the far end
// must not report every line in between as changed.
func TestOnlyWhatMovedIsReported(t *testing.T) {
	changes := DiffText("a\nb\nc\nd\ne", "b\nc\nd\ne\na")
	assert.Equal(t, 1, SummariseLines(changes).Removed)
	assert.Equal(t, 1, SummariseLines(changes).Added)
	assert.Equal(t, 4, SummariseLines(changes).Unchanged)
}

func TestAVeryLongTextIsBounded(t *testing.T) {
	long := ""
	for i := 0; i < MaxDiffLines+500; i++ {
		long += "line\n"
	}
	assert.LessOrEqual(t, len(DiffText(long, "")), MaxDiffLines)
}

// ---- structural diff ---------------------------------------------------------

func blockPara(id, text string) *schema.Node {
	return &schema.Node{
		Type:    schema.NodeParagraph,
		Attrs:   map[string]any{"id": id},
		Content: []*schema.Node{{Type: schema.NodeText, Text: text}},
	}
}

func docOf(children ...*schema.Node) *schema.Node {
	return &schema.Node{Type: schema.NodeDoc, Content: children}
}

func blockKind(changes []BlockChange, id string) ChangeKind {
	for _, change := range changes {
		if change.BlockID == id {
			return change.Kind
		}
	}
	return ""
}

func TestAnUntouchedDocumentHasNoStructuralChanges(t *testing.T) {
	doc := docOf(blockPara("blockaa", "one"), blockPara("blockbb", "two"))
	summary := SummariseBlocks(DiffBlocks(doc, doc))
	assert.Equal(t, DiffSummary{Unchanged: 2}, summary)
}

func TestAnEditedBlockIsChangedRatherThanReplaced(t *testing.T) {
	before := docOf(blockPara("blockaa", "one"), blockPara("blockbb", "two"))
	after := docOf(blockPara("blockaa", "one"), blockPara("blockbb", "TWO"))

	changes := DiffBlocks(before, after)
	assert.Equal(t, Changed, blockKind(changes, "blockbb"))
	assert.Equal(t, DiffSummary{Unchanged: 1, Changed: 1}, SummariseBlocks(changes))
}

// The whole reason block ids are in the document: without them this is
// indistinguishable from a delete plus an insert.
func TestAMovedBlockIsRecognisedAsAMove(t *testing.T) {
	before := docOf(blockPara("blockaa", "one"), blockPara("blockbb", "two"), blockPara("blockcc", "three"))
	after := docOf(blockPara("blockcc", "three"), blockPara("blockaa", "one"), blockPara("blockbb", "two"))

	changes := DiffBlocks(before, after)
	assert.Equal(t, Moved, blockKind(changes, "blockcc"))
	assert.Equal(t, DiffSummary{Moved: 3}, SummariseBlocks(changes),
		"all three sit somewhere new, and none of them was rewritten")
}

// Both moved and rewritten: what it says now matters more than where it sits.
func TestABlockThatMovedAndChangedIsReportedAsChanged(t *testing.T) {
	before := docOf(blockPara("blockaa", "one"), blockPara("blockbb", "two"))
	after := docOf(blockPara("blockbb", "TWO"), blockPara("blockaa", "one"))

	assert.Equal(t, Changed, blockKind(DiffBlocks(before, after), "blockbb"))
}

func TestAddedAndRemovedBlocksCarryTheirPositions(t *testing.T) {
	before := docOf(blockPara("blockaa", "one"), blockPara("gonegone", "old"))
	after := docOf(blockPara("blockaa", "one"), blockPara("freshone", "new"))

	changes := DiffBlocks(before, after)
	for _, change := range changes {
		switch change.BlockID {
		case "freshone":
			assert.Equal(t, Added, change.Kind)
			assert.Equal(t, -1, change.OldIndex, "an added block is nowhere in the old version")
			assert.Equal(t, 1, change.NewIndex)
			assert.Equal(t, "new", change.Text)
		case "gonegone":
			assert.Equal(t, Removed, change.Kind)
			assert.Equal(t, 1, change.OldIndex)
			assert.Equal(t, -1, change.NewIndex)
			assert.Equal(t, "old", change.Text, "a removed block still shows what it said")
		}
	}
	assert.Equal(t, DiffSummary{Unchanged: 1, Added: 1, Removed: 1}, SummariseBlocks(changes))
}

// Pairing them by position would report confident nonsense.
func TestBlocksWithNoIDAreSkippedRatherThanGuessedAt(t *testing.T) {
	plain := &schema.Node{
		Type:    schema.NodeParagraph,
		Content: []*schema.Node{{Type: schema.NodeText, Text: "no id"}},
	}
	before := docOf(plain, blockPara("blockaa", "one"))
	after := docOf(blockPara("blockaa", "one"))

	changes := DiffBlocks(before, after)
	assert.Equal(t, DiffSummary{Unchanged: 1}, SummariseBlocks(changes),
		"the block with no id is neither reported as removed nor paired with anything")
}

// A changed list item and the list around it would otherwise read as twice as
// much having happened as did.
func TestOnlyTopLevelBlocksAreCompared(t *testing.T) {
	list := func(itemText string) *schema.Node {
		return &schema.Node{
			Type:  schema.NodeBulletList,
			Attrs: map[string]any{"id": "listaaa"},
			Content: []*schema.Node{{
				Type:    schema.NodeListItem,
				Attrs:   map[string]any{"id": "itemaaa"},
				Content: []*schema.Node{blockPara("innerbb", itemText)},
			}},
		}
	}

	changes := DiffBlocks(docOf(list("before")), docOf(list("after")))
	require.Len(t, changes, 1, "one change, not three")
	assert.Equal(t, "listaaa", changes[0].BlockID)
	assert.Equal(t, Changed, changes[0].Kind)
}

func TestDiffingAgainstNothing(t *testing.T) {
	doc := docOf(blockPara("blockaa", "one"))

	assert.Equal(t, DiffSummary{Added: 1}, SummariseBlocks(DiffBlocks(nil, doc)))
	assert.Equal(t, DiffSummary{Removed: 1}, SummariseBlocks(DiffBlocks(doc, nil)))
	assert.Empty(t, DiffBlocks(nil, nil))
}
