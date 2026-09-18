package render

import (
	"strings"
	"unicode"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// Heading is one heading in document order.
type Heading struct {
	Level int
	Text  string
	// ID is the block id, if the editor assigned one.
	ID string
}

// Transclusion is a reference to a block of another page.
type Transclusion struct {
	SourcePageID  string
	SourceBlockID string
}

// Structure lists the facts other subsystems need about a document.
type Structure struct {
	Headings      []Heading
	PageLinks     []string // distinct page ids, document order
	Mentions      []string // distinct user ids
	Transclusions []Transclusion
	AttachmentIDs []string // distinct, includes diagram previews
	ExternalLinks []string // distinct hrefs of link marks and embeds
	BlockIDs      []string // every non-empty block id, document order
	FootnoteIDs   []string // footnote ids in reference order
	WordCount     int
	CharCount     int
	HasTOC        bool
	HasSubpages   bool
}

// Extract walks the tree once and collects Structure.
func Extract(doc *schema.Node) Structure {
	x := &extractor{
		seenPage: map[string]bool{}, seenUser: map[string]bool{}, seenAtt: map[string]bool{},
		seenLink: map[string]bool{}, seenFoot: map[string]bool{},
	}
	x.walk(doc)
	x.st.WordCount, x.st.CharCount = countWords(x.text.String())
	return x.st
}

type extractor struct {
	st       Structure
	text     strings.Builder
	seenPage map[string]bool
	seenUser map[string]bool
	seenAtt  map[string]bool
	seenLink map[string]bool
	seenFoot map[string]bool
}

func (x *extractor) walk(n *schema.Node) {
	if id := attr(n, "id"); id != "" {
		x.st.BlockIDs = append(x.st.BlockIDs, id)
	}
	switch n.Type {
	case schema.NodeText:
		x.text.WriteString(n.Text)
		for _, m := range n.Marks {
			if m.Type == schema.MarkLink {
				x.addLink(markAttr(m, "href"))
			}
		}
	case schema.NodeHeading:
		x.st.Headings = append(x.st.Headings, Heading{
			Level: attrInt(n, "level", 1), Text: inlineText(n), ID: attr(n, "id"),
		})
	case schema.NodePageLink:
		if id := attr(n, "pageId"); id != "" && !x.seenPage[id] {
			x.seenPage[id] = true
			x.st.PageLinks = append(x.st.PageLinks, id)
		}
	case schema.NodeMention:
		if id := attr(n, "userId"); id != "" && !x.seenUser[id] {
			x.seenUser[id] = true
			x.st.Mentions = append(x.st.Mentions, id)
		}
		writeAtom(&x.text, attr(n, "label"))
	case schema.NodeTransclusion:
		x.st.Transclusions = append(x.st.Transclusions, Transclusion{
			SourcePageID: attr(n, "sourcePageId"), SourceBlockID: attr(n, "sourceBlockId"),
		})
	case schema.NodeEmbed:
		x.addLink(attr(n, "url"))
	case schema.NodeFootnoteRef:
		if id := attr(n, "footnoteId"); id != "" && !x.seenFoot[id] {
			x.seenFoot[id] = true
			x.st.FootnoteIDs = append(x.st.FootnoteIDs, id)
		}
	case schema.NodeStatus:
		writeAtom(&x.text, attr(n, "text"))
	case schema.NodeMathInline, schema.NodeMathBlock:
		writeAtom(&x.text, attr(n, "latex"))
	case schema.NodeToc:
		x.st.HasTOC = true
	case schema.NodeSubpages:
		x.st.HasSubpages = true
	}
	for _, key := range []string{"attachmentId", "previewAttachmentId"} {
		if id := attr(n, key); id != "" && !x.seenAtt[id] {
			x.seenAtt[id] = true
			x.st.AttachmentIDs = append(x.st.AttachmentIDs, id)
		}
	}
	for _, c := range n.Content {
		x.walk(c)
	}
	if isBlock(n) {
		x.text.WriteByte('\n')
	}
}

// writeAtom appends the visible text of an inline atom, separated from the
// surrounding text by spaces so "test@Ming" does not become one word.
func writeAtom(b *strings.Builder, s string) {
	if s == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.WriteString(s)
	b.WriteByte(' ')
}

func (x *extractor) addLink(href string) {
	if href == "" || x.seenLink[href] {
		return
	}
	x.seenLink[href] = true
	x.st.ExternalLinks = append(x.st.ExternalLinks, href)
}

// isBlock reports whether a node separates text lines.
func isBlock(n *schema.Node) bool {
	spec, ok := schema.Default().Nodes[n.Type]
	if !ok {
		return false
	}
	return !spec.Inline && !spec.Text && n.Type != schema.NodeDoc
}

// inlineText concatenates the text of a node's inline content.
func inlineText(n *schema.Node) string {
	var b strings.Builder
	var walk func(*schema.Node)
	walk = func(c *schema.Node) {
		switch c.Type {
		case schema.NodeText:
			b.WriteString(c.Text)
		case schema.NodeMention:
			b.WriteString("@" + attr(c, "label"))
		case schema.NodeStatus:
			b.WriteString(attr(c, "text"))
		case schema.NodeMathInline:
			b.WriteString(attr(c, "latex"))
		case schema.NodeHardBreak:
			b.WriteByte(' ')
		}
		for _, cc := range c.Content {
			walk(cc)
		}
	}
	for _, c := range n.Content {
		walk(c)
	}
	return b.String()
}

// countWords counts words the way readers of mixed Chinese/English text
// expect: every CJK character is a word, and each run of other letters or
// digits is a word. It also returns the number of non-space characters.
func countWords(s string) (words, chars int) {
	inWord := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r),
			unicode.Is(unicode.Hangul, r):
			words++
			chars++
			inWord = false
		case unicode.IsLetter(r), unicode.IsDigit(r):
			if !inWord {
				words++
				inWord = true
			}
			chars++
		case unicode.IsSpace(r):
			inWord = false
		default:
			inWord = false
			chars++
		}
	}
	return words, chars
}

// Text renders the plain-text projection used for search indexing and
// snippets: one line per block, inline atoms reduced to their visible text.
func Text(doc *schema.Node) string {
	var b strings.Builder
	var walk func(n *schema.Node, depth int)
	walk = func(n *schema.Node, depth int) {
		switch n.Type {
		case schema.NodeText:
			b.WriteString(n.Text)
		case schema.NodeHardBreak:
			b.WriteByte('\n')
		case schema.NodeMention:
			writeAtom(&b, "@"+attr(n, "label"))
		case schema.NodeStatus:
			writeAtom(&b, attr(n, "text"))
		case schema.NodeMathInline:
			writeAtom(&b, attr(n, "latex"))
		case schema.NodeMathBlock:
			b.WriteString(attr(n, "latex"))
		case schema.NodeMermaid:
			b.WriteString(attr(n, "source"))
		case schema.NodeImage:
			b.WriteString(attr(n, "alt"))
		case schema.NodeAttachment, schema.NodePdfEmbed:
			b.WriteString(attr(n, "name"))
		case schema.NodeEmbed:
			b.WriteString(attr(n, "url"))
		case schema.NodeTableRow:
			for i, c := range n.Content {
				if i > 0 {
					b.WriteByte('\t')
				}
				b.WriteString(strings.TrimSpace(Text(c)))
			}
			b.WriteByte('\n')
			return
		}
		for _, c := range n.Content {
			walk(c, depth+1)
		}
		if isBlock(n) && n.Type != schema.NodeTableRow {
			b.WriteByte('\n')
		}
	}
	walk(doc, 0)
	return dropBlankLines(b.String())
}

// dropBlankLines trims each line and removes empty ones: the projection is
// for indexing, snippets and diffs, where paragraph spacing carries nothing.
func dropBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimRight(l, " \t\r")
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
