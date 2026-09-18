package history

import (
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// Comparing two versions of a page.
//
// Two different comparisons, because two different questions get asked of a
// history page and one answer cannot serve both:
//
//   - "What words changed?" — a line diff over the rendered text. This is what
//     somebody reads to see what was said differently.
//   - "What happened to the document?" — a diff over block ids. This is what
//     tells apart a paragraph that was rewritten from one that was deleted and
//     another added in its place, and it is the only way to see a block that
//     merely moved. Block ids make it possible; without them a move is
//     indistinguishable from a delete plus an insert.
//
// Both are pure functions over values, and neither knows about storage.

// ChangeKind names what happened to one line or one block.
type ChangeKind string

const (
	// Unchanged is present in both versions, in the same place.
	Unchanged ChangeKind = "same"
	// Added exists only in the newer version.
	Added ChangeKind = "added"
	// Removed existed only in the older version.
	Removed ChangeKind = "removed"
	// Moved is the same block in both, at a different position.
	Moved ChangeKind = "moved"
	// Changed is the same block in both, with different content.
	Changed ChangeKind = "changed"
)

// LineChange is one line of a text diff.
type LineChange struct {
	Kind ChangeKind `json:"kind"`
	Text string     `json:"text"`
	// OldLine and NewLine are 1-based line numbers, 0 where the line does not
	// exist on that side.
	OldLine int `json:"old_line,omitempty"`
	NewLine int `json:"new_line,omitempty"`
}

// MaxDiffLines bounds one comparison. Two versions of a very long page differ
// in more places than anybody reads, and an unbounded diff is a way to make
// the server do arbitrary work for one request.
const MaxDiffLines = 20000

// DiffText compares two rendered texts line by line.
//
// The algorithm is the classic one: a longest common subsequence over lines,
// walked backwards to produce the edit script. Lines rather than words
// because a document is written in paragraphs, and a word diff of a rewritten
// paragraph is noise rather than information.
func DiffText(before, after string) []LineChange {
	oldLines := splitLines(before)
	newLines := splitLines(after)
	if len(oldLines) > MaxDiffLines {
		oldLines = oldLines[:MaxDiffLines]
	}
	if len(newLines) > MaxDiffLines {
		newLines = newLines[:MaxDiffLines]
	}

	table := lcsTable(oldLines, newLines)
	out := make([]LineChange, 0, len(oldLines)+len(newLines))

	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			out = append(out, LineChange{
				Kind: Unchanged, Text: oldLines[i], OldLine: i + 1, NewLine: j + 1,
			})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, LineChange{Kind: Removed, Text: oldLines[i], OldLine: i + 1})
			i++
		default:
			out = append(out, LineChange{Kind: Added, Text: newLines[j], NewLine: j + 1})
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		out = append(out, LineChange{Kind: Removed, Text: oldLines[i], OldLine: i + 1})
	}
	for ; j < len(newLines); j++ {
		out = append(out, LineChange{Kind: Added, Text: newLines[j], NewLine: j + 1})
	}
	return out
}

// lcsTable builds the length table for the longest common subsequence.
//
// table[i][j] is the length of the LCS of old[i:] and new[j:], so the walk
// above can read it forwards.
func lcsTable(oldLines, newLines []string) [][]int {
	table := make([][]int, len(oldLines)+1)
	for i := range table {
		table[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	return table
}

// splitLines splits text into lines, dropping a trailing empty one so that
// "a\n" and "a" compare as the same single line.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// BlockChange is one block in a structural diff.
type BlockChange struct {
	BlockID string     `json:"block_id"`
	Kind    ChangeKind `json:"kind"`
	// Type is the node type, taken from whichever version has the block.
	Type string `json:"type"`
	// Text is the block's rendered text in the newer version, or in the older
	// one for a block that was removed.
	Text string `json:"text,omitempty"`
	// OldIndex and NewIndex are 0-based positions among the blocks this diff
	// compares — that is, the top-level blocks carrying an id — and -1 where
	// the block is absent from that side.
	//
	// Counted over that sequence rather than over the document's children so
	// that a block with no id sitting between two others cannot shift every
	// position after it and have the whole page report as moved.
	OldIndex int `json:"old_index"`
	NewIndex int `json:"new_index"`
}

// DiffBlocks compares two documents by block id.
//
// A block present in both is unchanged, changed, or moved — and it can be
// both changed and moved, in which case it is reported as changed, because
// what it says now matters more than where it sits. A block in only one
// version was added or removed.
//
// Blocks with no id are skipped rather than guessed at. They come from
// documents written before ids were assigned, or from a node type that
// carries none, and pairing them by position would report confident nonsense.
func DiffBlocks(before, after *schema.Node) []BlockChange {
	oldBlocks := blockIndex(before)
	newBlocks := blockIndex(after)

	out := make([]BlockChange, 0, len(newBlocks)+len(oldBlocks))
	seen := make(map[string]bool, len(newBlocks))

	for _, nb := range newBlocks {
		seen[nb.id] = true
		ob, existed := findBlock(oldBlocks, nb.id)
		switch {
		case !existed:
			out = append(out, BlockChange{
				BlockID: nb.id, Kind: Added, Type: nb.nodeType,
				Text: nb.text, OldIndex: -1, NewIndex: nb.index,
			})
		case ob.text != nb.text:
			out = append(out, BlockChange{
				BlockID: nb.id, Kind: Changed, Type: nb.nodeType,
				Text: nb.text, OldIndex: ob.index, NewIndex: nb.index,
			})
		case ob.index != nb.index:
			out = append(out, BlockChange{
				BlockID: nb.id, Kind: Moved, Type: nb.nodeType,
				Text: nb.text, OldIndex: ob.index, NewIndex: nb.index,
			})
		default:
			out = append(out, BlockChange{
				BlockID: nb.id, Kind: Unchanged, Type: nb.nodeType,
				Text: nb.text, OldIndex: ob.index, NewIndex: nb.index,
			})
		}
	}

	for _, ob := range oldBlocks {
		if seen[ob.id] {
			continue
		}
		out = append(out, BlockChange{
			BlockID: ob.id, Kind: Removed, Type: ob.nodeType,
			Text: ob.text, OldIndex: ob.index, NewIndex: -1,
		})
	}
	return out
}

type indexedBlock struct {
	id       string
	nodeType string
	text     string
	index    int
}

// blockIndex lists the top-level blocks of a document that carry an id.
//
// Top level only: a diff that descends reports a changed list item and the
// list containing it as two separate changes, which reads as twice as much
// having happened as did.
func blockIndex(doc *schema.Node) []indexedBlock {
	if doc == nil {
		return nil
	}
	out := make([]indexedBlock, 0, len(doc.Content))
	for _, node := range doc.Content {
		id := blockID(node)
		if id == "" {
			continue
		}
		out = append(out, indexedBlock{
			id: id, nodeType: node.Type, text: render.Text(node), index: len(out),
		})
	}
	return out
}

func findBlock(blocks []indexedBlock, id string) (indexedBlock, bool) {
	for _, block := range blocks {
		if block.id == id {
			return block, true
		}
	}
	return indexedBlock{}, false
}

func blockID(n *schema.Node) string {
	if n == nil || n.Attrs == nil {
		return ""
	}
	id, _ := n.Attrs["id"].(string)
	return id
}

// DiffSummary counts a comparison, for a list that shows "+12 −3" beside an
// entry without loading either version.
type DiffSummary struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Changed   int `json:"changed"`
	Moved     int `json:"moved"`
	Unchanged int `json:"unchanged"`
}

// SummariseBlocks counts a block diff by kind.
func SummariseBlocks(changes []BlockChange) DiffSummary {
	var out DiffSummary
	for _, change := range changes {
		switch change.Kind {
		case Added:
			out.Added++
		case Removed:
			out.Removed++
		case Changed:
			out.Changed++
		case Moved:
			out.Moved++
		case Unchanged:
			out.Unchanged++
		}
	}
	return out
}

// SummariseLines counts a text diff by kind.
func SummariseLines(changes []LineChange) DiffSummary {
	var out DiffSummary
	for _, change := range changes {
		switch change.Kind {
		case Added:
			out.Added++
		case Removed:
			out.Removed++
		case Unchanged:
			out.Unchanged++
		}
	}
	return out
}
