package render

import "github.com/magicyuan876/yuheng/internal/docs/schema"

// Making a document portable.
//
// A template is a page body that will be used somewhere else — another space,
// another tenant's idea of who is who, months later. Most of a document
// travels fine: headings, text, tables, callouts, code. Some of it does not,
// and the parts that do not are exactly the parts that would either break or
// leak if they were copied verbatim:
//
//   - A page link and a block reference point at one specific page. Copied
//     into a new document they either dangle or, worse, quietly point at
//     somebody else's page that the reader of the new one may not open.
//   - A mention names one person. A template that greets Alice by name every
//     time it is used is wrong in a way nobody notices until it is embarrassing.
//   - An attachment belongs to the space it was uploaded to — that is what
//     makes its permissions and its cleanup tractable (see T1.6). A template
//     used in another space cannot carry it: the copy would either be a
//     cross-space reference that one space's tidying can break, or a silent
//     duplication of somebody's file into a space they never put it in.
//
// So Portable strips those and keeps everything else. The rule is worth
// stating plainly because it is a product boundary and not only a technical
// one: a template carries structure and words, not a specific page's ties to
// its surroundings.

// portableDrops are the node types a template cannot carry.
var portableDrops = map[string]bool{
	"transclusion": true, // points at one block of one page
	"image":        true, // attachment, belongs to its space
	"attachment":   true,
	"drawing":      true, // stores two attachments (source + preview)
	"excalidraw":   true,
}

// portableFlatten are the node types replaced by their own text: the words
// survive, the pointer does not.
var portableFlatten = map[string]bool{
	"pageLink": true,
	"mention":  true,
}

// Portable returns a copy of doc with the parts that cannot travel removed.
//
// The returned document is still valid against the schema: nodes are dropped
// whole rather than emptied, and the two flattened types are inline leaves
// replaced by text, so no content expression can be left unsatisfied by the
// substitution itself. Callers should still validate, because they are about
// to store it.
func Portable(doc *schema.Node) *schema.Node {
	if doc == nil {
		return nil
	}
	out := portableNode(doc)
	if out == nil {
		// Only possible if the whole document were droppable, which the
		// schema does not allow of a doc node; belt and braces.
		return &schema.Node{Type: doc.Type}
	}
	return out
}

func portableNode(n *schema.Node) *schema.Node {
	if n == nil || portableDrops[n.Type] {
		return nil
	}
	if portableFlatten[n.Type] {
		return portableText(n)
	}

	copied := *n
	copied.Content = nil
	for _, child := range n.Content {
		if kept := portableNode(child); kept != nil {
			copied.Content = append(copied.Content, kept)
		}
	}
	// A block whose only content was droppable becomes empty rather than
	// disappearing: an empty paragraph where a diagram was is a visible
	// reminder that something belonged there, which is what somebody editing
	// the template needs to see.
	return &copied
}

// portableText turns a pointing node into the words it displayed.
func portableText(n *schema.Node) *schema.Node {
	text := attr(n, "label")
	if text == "" {
		text = attr(n, "text")
	}
	if text == "" {
		// A pageLink stores only its target id, deliberately, so that
		// renaming a page updates every link to it (T2.2). That leaves
		// nothing to keep here, and nothing is better than a stale name.
		return nil
	}
	return &schema.Node{Type: "text", Text: text}
}

// PortableDrops reports whether a document contains anything Portable would
// remove, so a caller can warn before it happens rather than after.
func PortableDrops(doc *schema.Node) bool {
	if doc == nil {
		return false
	}
	if portableDrops[doc.Type] || portableFlatten[doc.Type] {
		return true
	}
	for _, child := range doc.Content {
		if PortableDrops(child) {
			return true
		}
	}
	return false
}
