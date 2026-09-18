package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// markdownRenderer emits the Yuheng Markdown dialect (技术方案 §6.3):
// CommonMark + GFM tables/task lists, `:::kind` fences for callouts (the
// convention VitePress and Docmost users already know), `$…$` / `$$…$$` for
// math, ```mermaid fences, `[^id]` footnotes and `[[toc]]`. Nodes with no
// Markdown equivalent degrade to HTML blocks or links so no content is
// lost; the Markdown→JSON converter (T0.6) reads the same dialect back.
type markdownRenderer struct {
	opts      *Options
	st        Structure
	footnotes map[string]int
	depth     int
}

func newMarkdownRenderer(opts *Options, st Structure) *markdownRenderer {
	r := &markdownRenderer{opts: opts, st: st, footnotes: map[string]int{}}
	for i, id := range st.FootnoteIDs {
		r.footnotes[id] = i + 1
	}
	return r
}

func (r *markdownRenderer) render(doc *schema.Node) string {
	parts := r.blocks(doc.Content)
	return strings.TrimRight(strings.Join(parts, "\n\n"), "\n") + "\n"
}

// blocks renders sibling blocks as separate paragraphs.
func (r *markdownRenderer) blocks(nodes []*schema.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s := r.block(n); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (r *markdownRenderer) block(n *schema.Node) string {
	switch n.Type {
	case schema.NodeParagraph:
		return r.inlines(n.Content)
	case schema.NodeHeading:
		return strings.Repeat("#", attrInt(n, "level", 1)) + " " + r.inlines(n.Content)
	case schema.NodeBlockquote:
		return prefixLines(strings.Join(r.blocks(n.Content), "\n\n"), "> ")
	case schema.NodeHorizontalRule:
		return "---"
	case schema.NodeBulletList:
		return r.list(n, func(int) string { return "- " })
	case schema.NodeOrderedList:
		start := attrInt(n, "start", 1)
		return r.list(n, func(i int) string { return strconv.Itoa(start+i) + ". " })
	case schema.NodeTaskList:
		return r.list(n, func(int) string { return "- " })
	case schema.NodeCodeBlock:
		var text strings.Builder
		for _, c := range n.Content {
			text.WriteString(c.Text)
		}
		fence := "```"
		for strings.Contains(text.String(), fence) {
			fence += "`"
		}
		return fence + attr(n, "language") + "\n" + text.String() + "\n" + fence
	case schema.NodeTable:
		return r.table(n)
	case schema.NodeImage:
		src := attr(n, "src")
		if aid := attr(n, "attachmentId"); aid != "" {
			src = r.opts.attachmentURL(aid)
		} else {
			src = safeURL(src)
		}
		return "!" + link(attr(n, "alt"), src, attr(n, "title"))
	case schema.NodeAttachment:
		return link(nameOr(attr(n, "name"), r.opts.str("attachment")),
			r.opts.attachmentURL(attr(n, "attachmentId")), "")
	case schema.NodeVideo, schema.NodeAudio:
		return link(r.opts.attachmentURL(attr(n, "attachmentId")), r.opts.attachmentURL(attr(n, "attachmentId")), "")
	case schema.NodePdfEmbed:
		return link(nameOr(attr(n, "name"), r.opts.str("pdf")), r.opts.attachmentURL(attr(n, "attachmentId")), "")
	case schema.NodeCallout:
		kind := attr(n, "kind")
		if kind == "" {
			kind = "info"
		}
		return "::: " + kind + "\n" + strings.Join(r.blocks(n.Content), "\n\n") + "\n:::"
	case schema.NodeColumns:
		// Layout is not expressible; keep the content in reading order and
		// leave a marker the importer can use to rebuild the columns.
		var parts []string
		parts = append(parts, fmt.Sprintf("<!-- columns:%d -->", len(n.Content)))
		for i, col := range n.Content {
			if i > 0 {
				parts = append(parts, "<!-- column -->")
			}
			parts = append(parts, r.blocks(col.Content)...)
		}
		parts = append(parts, "<!-- /columns -->")
		return strings.Join(parts, "\n\n")
	case schema.NodeDetails:
		var summary, body string
		for _, c := range n.Content {
			switch c.Type {
			case schema.NodeDetailsSummary:
				summary = r.inlines(c.Content)
			case schema.NodeDetailsContent:
				body = strings.Join(r.blocks(c.Content), "\n\n")
			}
		}
		openAttr := ""
		if attrBool(n, "open", true) {
			openAttr = " open"
		}
		return "<details" + openAttr + ">\n<summary>" + summary + "</summary>\n\n" + body + "\n\n</details>"
	case schema.NodeMathBlock:
		return "$$\n" + attr(n, "latex") + "\n$$"
	case schema.NodeMermaid:
		return "```mermaid\n" + attr(n, "source") + "\n```"
	case schema.NodeDrawio, schema.NodeExcalidraw:
		if pid := attr(n, "previewAttachmentId"); pid != "" {
			return "!" + link(r.opts.str("diagram"), r.opts.attachmentURL(pid), "")
		}
		return link(r.opts.str("diagram"), r.opts.attachmentURL(attr(n, "attachmentId")), "")
	case schema.NodeEmbed:
		url := safeURL(attr(n, "url"))
		if url == "" {
			return r.opts.str("embedUnavailable")
		}
		return link(nameOr(attr(n, "provider"), url), url, "")
	case schema.NodeTransclusion:
		pageID, blockID := attr(n, "sourcePageId"), attr(n, "sourceBlockId")
		if r.opts.TransclusionContent != nil && r.depth == 0 {
			if node, ok := r.opts.TransclusionContent(pageID, blockID); ok && node != nil {
				r.depth++
				s := r.block(node)
				r.depth--
				return s
			}
		}
		return prefixLines(r.opts.str("transclusionEmpty")+" "+link(r.opts.str("pageUnavailable"),
			r.opts.pageURL(pageID)+"#"+blockID, ""), "> ")
	case schema.NodeFootnotes:
		var parts []string
		for _, fn := range n.Content {
			id := attr(fn, "footnoteId")
			body := strings.Join(r.blocks(fn.Content), "\n\n")
			// Continuation lines of a footnote are indented by four spaces.
			body = strings.ReplaceAll(body, "\n", "\n    ")
			parts = append(parts, "[^"+id+"]: "+body)
		}
		return strings.Join(parts, "\n\n")
	case schema.NodePageBreak:
		return `<div style="page-break-after:always"></div>`
	case schema.NodeToc:
		return "[[toc]]"
	case schema.NodeSubpages:
		if r.opts.Subpages == nil {
			return ""
		}
		var parts []string
		for _, ref := range r.opts.Subpages() {
			parts = append(parts, "- "+link(nameOr(ref.Title, r.opts.str("untitledPage")), ref.URL, ""))
		}
		return strings.Join(parts, "\n")
	case schema.NodeListItem, schema.NodeTaskItem, schema.NodeColumn, schema.NodeDetailsSummary,
		schema.NodeDetailsContent, schema.NodeTableRow, schema.NodeTableCell, schema.NodeTableHeader,
		schema.NodeFootnote:
		// Only reachable when rendered outside their container; degrade to
		// their content.
		return strings.Join(r.blocks(n.Content), "\n\n")
	}
	return strings.Join(r.blocks(n.Content), "\n\n")
}

// list renders list items; nested blocks are indented under the marker.
func (r *markdownRenderer) list(n *schema.Node, marker func(i int) string) string {
	var lines []string
	for i, item := range n.Content {
		m := marker(i)
		if item.Type == schema.NodeTaskItem {
			if attrBool(item, "checked", false) {
				m += "[x] "
			} else {
				m += "[ ] "
			}
		}
		indent := strings.Repeat(" ", len(m))
		var body []string
		for j, c := range item.Content {
			s := r.block(c)
			if s == "" {
				continue
			}
			if j == 0 && c.Type == schema.NodeParagraph {
				body = append(body, m+indentContinuation(s, indent))
				continue
			}
			if len(body) == 0 {
				body = append(body, strings.TrimRight(m, " "))
			}
			body = append(body, prefixLines(s, indent))
		}
		if len(body) == 0 {
			body = append(body, strings.TrimRight(m, " "))
		}
		lines = append(lines, strings.Join(body, "\n"))
	}
	return strings.Join(lines, "\n")
}

func (r *markdownRenderer) table(n *schema.Node) string {
	if len(n.Content) == 0 {
		return ""
	}
	// Expand colspans into empty cells so every row has the same width; GFM
	// has no spanning cells.
	var rows [][]string
	width := 0
	for _, row := range n.Content {
		var cells []string
		for _, cell := range row.Content {
			cells = append(cells, r.cell(cell))
			for extra := attrInt(cell, "colspan", 1); extra > 1; extra-- {
				cells = append(cells, "")
			}
		}
		if len(cells) > width {
			width = len(cells)
		}
		rows = append(rows, cells)
	}
	var b strings.Builder
	for i, cells := range rows {
		for len(cells) < width {
			cells = append(cells, "")
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (r *markdownRenderer) cell(cell *schema.Node) string {
	parts := r.blocks(cell.Content)
	s := strings.Join(parts, "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return strings.ReplaceAll(s, "|", `\|`)
}

// ---- inline ------------------------------------------------------------------

func (r *markdownRenderer) inlines(nodes []*schema.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(r.inline(n))
	}
	return b.String()
}

func (r *markdownRenderer) inline(n *schema.Node) string {
	switch n.Type {
	case schema.NodeText:
		return r.markedText(n)
	case schema.NodeHardBreak:
		return "  \n"
	case schema.NodeMention:
		return "@" + attr(n, "label")
	case schema.NodePageLink:
		id := attr(n, "pageId")
		title, ok := r.opts.pageTitle(id)
		if !ok {
			title = r.opts.str("pageUnavailable")
		} else if title == "" {
			title = r.opts.str("untitledPage")
		}
		return link(title, r.opts.pageURL(id), "")
	case schema.NodeMathInline:
		return "$" + attr(n, "latex") + "$"
	case schema.NodeStatus:
		return "`" + strings.ReplaceAll(attr(n, "text"), "`", "'") + "`"
	case schema.NodeFootnoteRef:
		return "[^" + attr(n, "footnoteId") + "]"
	}
	return inlineText(&schema.Node{Content: []*schema.Node{n}})
}

func (r *markdownRenderer) markedText(n *schema.Node) string {
	text := n.Text
	if hasMark(n, schema.MarkCode) {
		fence := "`"
		for strings.Contains(text, fence) {
			fence += "`"
		}
		text = fence + text + fence
	} else {
		text = escapeMarkdown(text)
	}
	// Apply inner marks first, then wrap outward, mirroring the HTML nesting.
	// Whitespace at either end stays outside every wrapper: CommonMark
	// requires it for emphasis to parse, and "[ link](url)" reads badly.
	lead, core, trail := splitSpace(text)
	if core == "" {
		return text
	}
	marks := orderedMarks(n.Marks)
	for i := len(marks) - 1; i >= 0; i-- {
		m := marks[i]
		switch m.Type {
		case schema.MarkBold:
			core = "**" + core + "**"
		case schema.MarkItalic:
			core = "*" + core + "*"
		case schema.MarkStrike:
			core = "~~" + core + "~~"
		case schema.MarkHighlight:
			core = "==" + core + "=="
		case schema.MarkUnderline:
			core = "<u>" + core + "</u>"
		case schema.MarkSubscript:
			core = "<sub>" + core + "</sub>"
		case schema.MarkSuperscript:
			core = "<sup>" + core + "</sup>"
		case schema.MarkTextStyle:
			if c := markAttr(m, "color"); c != "" {
				core = `<span style="color:` + c + `">` + core + "</span>"
			}
		case schema.MarkLink:
			core = link(core, safeURL(markAttr(m, "href")), markAttr(m, "title"))
		}
	}
	return lead + core + trail
}

// splitSpace separates leading and trailing whitespace from the text.
func splitSpace(text string) (lead, core, trail string) {
	core = strings.TrimLeft(text, " \t\n")
	lead = text[:len(text)-len(core)]
	core = strings.TrimRight(core, " \t\n")
	trail = text[len(lead)+len(core):]
	return lead, core, trail
}

func link(text, href, title string) string {
	text = strings.ReplaceAll(text, "]", `\]`)
	if title != "" {
		return "[" + text + "](" + href + ` "` + strings.ReplaceAll(title, `"`, `\"`) + `")`
	}
	return "[" + text + "](" + href + ")"
}

var markdownEscaper = strings.NewReplacer(
	`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `[`, `\[`, `]`, `\]`, `<`, `\<`,
)

// escapeMarkdown protects characters that would otherwise be read as syntax:
// inline delimiters anywhere, block starters (#, >, -, +, "1.") only at the
// beginning of the text. Pipes are escaped by the table renderer alone,
// since they only matter inside a table.
func escapeMarkdown(s string) string {
	if s == "" {
		return s
	}
	out := markdownEscaper.Replace(s)
	switch out[0] {
	case '#', '>', '-', '+':
		out = `\` + out
	}
	if out[0] >= '0' && out[0] <= '9' {
		if i := strings.IndexAny(out, ".)"); i > 0 && i < 10 && isDigits(out[:i]) {
			out = out[:i] + `\` + out[i:]
		}
	}
	return out
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = strings.TrimRight(prefix, " ")
		} else {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

// indentContinuation indents every line after the first.
func indentContinuation(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}
