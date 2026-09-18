package render

import (
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// Finding one block of a document by its id.
//
// A block reference stores a page id and a block id and nothing else, so the
// text it shows has to be found in the source page every time that page is
// saved. This is the lookup, kept here beside Extract because it walks the
// same tree and answers a question about the same thing: what a document
// contains, as opposed to what it looks like.
//
// Two rules are worth stating because they are decisions rather than details.
// A block id is unique within a document but the editor cannot enforce that
// across a copy-and-paste, so the *first* match in document order wins and
// the rest are ignored -- deterministic beats clever, and the alternative
// (refusing to resolve an ambiguous id) would break a reference because of an
// edit somewhere else on the page.
//
// And the search does not descend into a transclusion node. A block that is
// itself a reference to a third page resolves to that reference, not through
// it, which is what stops a chain of references from being followed at all --
// and therefore what makes a cycle between two pages impossible rather than
// merely bounded. See ClipTransclusions.

// FindBlock returns the block carrying the given id, and whether it was found.
//
// The node returned is the one in the tree, not a copy: callers that keep it
// past the lifetime of doc, or that intend to change it, must clone it first.
func FindBlock(doc *schema.Node, blockID string) (*schema.Node, bool) {
	if doc == nil || blockID == "" {
		return nil, false
	}
	found := findBlocks(doc, map[string]bool{blockID: true}, map[string]*schema.Node{})
	node, ok := found[blockID]
	return node, ok
}

// FindBlocks returns every requested block that exists, keyed by block id.
//
// One walk for any number of ids: a page that is referenced forty times is
// not a reason to walk it forty times.
func FindBlocks(doc *schema.Node, blockIDs []string) map[string]*schema.Node {
	out := map[string]*schema.Node{}
	if doc == nil || len(blockIDs) == 0 {
		return out
	}
	want := make(map[string]bool, len(blockIDs))
	for _, id := range blockIDs {
		if id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return out
	}
	return findBlocks(doc, want, out)
}

func findBlocks(n *schema.Node, want map[string]bool, out map[string]*schema.Node) map[string]*schema.Node {
	if n == nil || len(out) == len(want) {
		return out
	}
	if id := blockIDOf(n); id != "" && want[id] {
		if _, seen := out[id]; !seen {
			out[id] = n
		}
	}
	// Not through a reference: see the note at the top of this file.
	if n.Type == schema.NodeTransclusion {
		return out
	}
	for _, child := range n.Content {
		findBlocks(child, want, out)
		if len(out) == len(want) {
			break
		}
	}
	return out
}

// blockIDOf reads a node's block id, if it carries one.
//
// The attribute is spelled "id" in docs-schema's blockId attribute set, which
// is also what Extract reads; keeping both on the same helper is what stops
// the two drifting apart.
func blockIDOf(n *schema.Node) string {
	return attr(n, "id")
}

// ClipTransclusions returns a copy of a block with any nested references
// replaced by an empty paragraph.
//
// This is what a snapshot stores. A reference shows the source block as it is
// now; if that block contained a reference of its own, showing it would mean
// resolving a second page while rendering the first, and two pages that
// reference each other would then resolve forever. Cutting the chain at one
// level removes the possibility rather than bounding it, which is the only
// version of this that cannot be got wrong later by raising a limit.
//
// The node is deep-copied, so the caller may store or change the result
// without touching the document it came from.
func ClipTransclusions(n *schema.Node) *schema.Node {
	if n == nil {
		return nil
	}
	if n.Type == schema.NodeTransclusion {
		// Something must stand in its place: an empty block keeps the
		// surrounding structure valid, which a nil child would not.
		return &schema.Node{Type: schema.NodeParagraph}
	}

	out := &schema.Node{Type: n.Type, Text: n.Text}
	if n.Attrs != nil {
		out.Attrs = make(map[string]any, len(n.Attrs))
		for k, v := range n.Attrs {
			out.Attrs[k] = v
		}
	}
	if n.Marks != nil {
		out.Marks = make([]schema.Mark, len(n.Marks))
		copy(out.Marks, n.Marks)
	}
	if n.Content != nil {
		out.Content = make([]*schema.Node, 0, len(n.Content))
		for _, child := range n.Content {
			out.Content = append(out.Content, ClipTransclusions(child))
		}
	}
	return out
}
