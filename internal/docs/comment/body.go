package comment

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// What a comment may say.
//
// A comment body is a ProseMirror document, so that a mention is a mention
// rather than a string that looks like one — but it is a deliberately small
// subset of the page schema. A comment is a remark, not a document: it has no
// headings, no tables, no images, no diagrams, and no references to blocks of
// other pages.
//
// The subset is declared here as a list rather than enforced by a second
// schema file, because it is a handful of names and the alternative is two
// definitions of the document model that have to be kept in step. The page
// schema still validates the structure; this narrows what that structure may
// contain.

// Node types a comment body may use.
//
// Paragraphs, lists and quotes cover what people actually write in a comment
// thread; code is here because a review comment is very often a snippet.
var allowedNodes = map[string]bool{
	schema.NodeDoc:         true,
	schema.NodeParagraph:   true,
	schema.NodeText:        true,
	schema.NodeHardBreak:   true,
	schema.NodeBulletList:  true,
	schema.NodeOrderedList: true,
	schema.NodeListItem:    true,
	schema.NodeBlockquote:  true,
	schema.NodeCodeBlock:   true,
	// A mention is the reason a comment is structured at all: it carries a
	// user id, so renaming somebody updates every comment that names them,
	// and a notification knows who to reach.
	schema.NodeMention: true,
	// A link to another page keeps its id rather than a URL, for the same
	// reason a page link does.
	schema.NodePageLink: true,
}

// Marks a comment body may use.
var allowedMarks = map[string]bool{
	schema.MarkBold:   true,
	schema.MarkItalic: true,
	schema.MarkStrike: true,
	schema.MarkCode:   true,
	schema.MarkLink:   true,
}

// MaxBodyRunes bounds one comment. Long enough for a considered review note,
// short enough that a thread stays readable and a page's comments stay a
// sidebar rather than a second document.
const MaxBodyRunes = 10000

// MaxBodyNodes bounds the structure, so a body cannot be a thousand empty
// paragraphs that cost nothing to write and something to render.
const MaxBodyNodes = 500

// ParseBody validates a comment body and returns it with its plain text.
//
// The text is returned rather than derived later because every caller needs
// it: notifications quote it, the search index (T5.1) stores it, and a reply
// preview shows it. Deriving it once here keeps those three from disagreeing.
func ParseBody(raw json.RawMessage) (*schema.Node, string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, "", fmt.Errorf("docs: a comment needs something in it")
	}
	node, stats, err := schema.Default().Validate(raw)
	if err != nil {
		return nil, "", fmt.Errorf("docs: %w", err)
	}
	if stats.Nodes > MaxBodyNodes {
		return nil, "", fmt.Errorf("docs: a comment may not hold more than %d nodes", MaxBodyNodes)
	}
	if err := checkSubset(node); err != nil {
		return nil, "", err
	}

	text := strings.TrimSpace(render.Text(node))
	if text == "" && !hasNonTextContent(node) {
		return nil, "", fmt.Errorf("docs: a comment needs something in it")
	}
	if len([]rune(text)) > MaxBodyRunes {
		return nil, "", fmt.Errorf("docs: a comment may not exceed %d characters", MaxBodyRunes)
	}
	return node, text, nil
}

// checkSubset refuses a node type or mark a comment may not use.
//
// Named in the error, because "invalid comment" tells somebody who pasted a
// table into a comment box nothing about what to do next.
func checkSubset(n *schema.Node) error {
	if n == nil {
		return nil
	}
	if !allowedNodes[n.Type] {
		return fmt.Errorf("docs: a comment may not contain %s", n.Type)
	}
	for _, mark := range n.Marks {
		if !allowedMarks[mark.Type] {
			return fmt.Errorf("docs: a comment may not contain %s formatting", mark.Type)
		}
	}
	for _, child := range n.Content {
		if err := checkSubset(child); err != nil {
			return err
		}
	}
	return nil
}

// hasNonTextContent reports whether a body holds something worth keeping that
// renders as no text — a mention on its own is a real comment.
func hasNonTextContent(n *schema.Node) bool {
	if n == nil {
		return false
	}
	if n.Type == schema.NodeMention || n.Type == schema.NodePageLink {
		return true
	}
	for _, child := range n.Content {
		if hasNonTextContent(child) {
			return true
		}
	}
	return false
}

// Mentions lists the people a comment names, in order, without repeats.
//
// Used to decide who to notify, so it reads the document rather than the text
// — a mention is a node with a user id, not a string beginning with "@".
func Mentions(n *schema.Node) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(*schema.Node)
	walk = func(node *schema.Node) {
		if node == nil {
			return
		}
		if node.Type == schema.NodeMention && node.Attrs != nil {
			if id, _ := node.Attrs["userId"].(string); id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		for _, child := range node.Content {
			walk(child)
		}
	}
	walk(n)
	return out
}
