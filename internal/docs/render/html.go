package render

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// htmlRenderer emits a fragment (no <html>/<body>) using plain elements and
// a small set of class names / data attributes the reading stylesheet and
// the client-side enhancers (KaTeX, Mermaid) key on. It never emits inline
// event handlers or scripts, and every dynamic value is escaped.
type htmlRenderer struct {
	opts      *Options
	st        Structure
	b         strings.Builder
	footnotes map[string]int // footnote id -> number, in reference order
	depth     int            // transclusion expansion depth
}

func newHTMLRenderer(opts *Options, st Structure) *htmlRenderer {
	r := &htmlRenderer{opts: opts, st: st, footnotes: map[string]int{}}
	for i, id := range st.FootnoteIDs {
		r.footnotes[id] = i + 1
	}
	return r
}

func (r *htmlRenderer) render(doc *schema.Node) string {
	r.b.Reset()
	for _, c := range doc.Content {
		r.block(c)
	}
	return r.b.String()
}

func esc(s string) string { return html.EscapeString(s) }

// open writes an opening tag with escaped attributes; empty values are
// skipped so callers can pass optional attributes freely.
func (r *htmlRenderer) open(tag string, attrs ...string) {
	r.b.WriteByte('<')
	r.b.WriteString(tag)
	for i := 0; i+1 < len(attrs); i += 2 {
		if attrs[i+1] == "" {
			continue
		}
		r.b.WriteByte(' ')
		r.b.WriteString(attrs[i])
		r.b.WriteString(`="`)
		r.b.WriteString(esc(attrs[i+1]))
		r.b.WriteByte('"')
	}
	r.b.WriteByte('>')
}

func (r *htmlRenderer) close(tag string) {
	r.b.WriteString("</")
	r.b.WriteString(tag)
	r.b.WriteString(">\n")
}

func blockStyle(n *schema.Node) string {
	var parts []string
	if a := attr(n, "textAlign"); a != "" && a != "left" {
		parts = append(parts, "text-align:"+a)
	}
	if ind := attrInt(n, "indent", 0); ind > 0 {
		parts = append(parts, fmt.Sprintf("margin-left:%dem", ind*2))
	}
	return strings.Join(parts, ";")
}

func (r *htmlRenderer) block(n *schema.Node) {
	id := attr(n, "id")
	switch n.Type {
	case schema.NodeParagraph:
		r.open("p", "id", id, "style", blockStyle(n))
		r.inlines(n.Content)
		r.close("p")
	case schema.NodeHeading:
		tag := "h" + strconv.Itoa(attrInt(n, "level", 1))
		r.open(tag, "id", id, "style", blockStyle(n))
		r.inlines(n.Content)
		r.close(tag)
	case schema.NodeBlockquote:
		r.open("blockquote", "id", id)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("blockquote")
	case schema.NodeHorizontalRule:
		r.open("hr", "id", id)
		r.b.WriteByte('\n')
	case schema.NodeBulletList:
		r.open("ul", "id", id)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("ul")
	case schema.NodeOrderedList:
		start := ""
		if s := attrInt(n, "start", 1); s != 1 {
			start = strconv.Itoa(s)
		}
		r.open("ol", "id", id, "start", start)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("ol")
	case schema.NodeListItem:
		r.open("li")
		r.listItemBody(n)
		r.close("li")
	case schema.NodeTaskList:
		r.open("ul", "id", id, "class", "task-list")
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("ul")
	case schema.NodeTaskItem:
		checked := attrBool(n, "checked", false)
		r.open("li", "class", "task-item", "data-checked", strconv.FormatBool(checked))
		if checked {
			r.b.WriteString(`<input type="checkbox" checked disabled>`)
		} else {
			r.b.WriteString(`<input type="checkbox" disabled>`)
		}
		r.listItemBody(n)
		r.close("li")
	case schema.NodeCodeBlock:
		lang := attr(n, "language")
		class := ""
		if lang != "" {
			class = "language-" + lang
		}
		wrap := ""
		if attrBool(n, "wrap", false) {
			wrap = "true"
		}
		r.open("pre", "id", id, "data-wrap", wrap)
		r.open("code", "class", class)
		for _, c := range n.Content {
			r.b.WriteString(esc(c.Text))
		}
		r.b.WriteString("</code>")
		r.close("pre")
	case schema.NodeTable:
		r.table(n)
	case schema.NodeImage:
		r.image(n)
	case schema.NodeAttachment:
		r.open("div", "id", id, "class", "attachment", "data-attachment-id", attr(n, "attachmentId"))
		r.open("a", "href", r.opts.attachmentURL(attr(n, "attachmentId")), "download", attr(n, "name"))
		r.b.WriteString(esc(nameOr(attr(n, "name"), r.opts.str("attachment"))))
		r.b.WriteString("</a>")
		if size := attrInt(n, "size", 0); size > 0 {
			r.b.WriteString(` <span class="attachment-size">` + humanSize(size) + `</span>`)
		}
		r.close("div")
	case schema.NodeVideo:
		r.open("figure", "id", id, "class", "video "+alignClass(n))
		r.open("video", "controls", "controls", "preload", "metadata",
			"src", r.opts.attachmentURL(attr(n, "attachmentId")),
			"width", dim(n, "width"), "height", dim(n, "height"))
		r.b.WriteString("</video>")
		r.close("figure")
	case schema.NodeAudio:
		r.open("figure", "id", id, "class", "audio")
		r.open("audio", "controls", "controls", "preload", "metadata",
			"src", r.opts.attachmentURL(attr(n, "attachmentId")))
		r.b.WriteString("</audio>")
		r.close("figure")
	case schema.NodePdfEmbed:
		url := r.opts.attachmentURL(attr(n, "attachmentId"))
		r.open("figure", "id", id, "class", "pdf-embed", "data-attachment-id", attr(n, "attachmentId"))
		r.open("iframe", "src", url, "title", nameOr(attr(n, "name"), r.opts.str("pdf")),
			"height", dim(n, "height"), "loading", "lazy")
		r.b.WriteString("</iframe>")
		r.open("figcaption")
		r.open("a", "href", url)
		r.b.WriteString(esc(nameOr(attr(n, "name"), r.opts.str("pdf"))))
		r.b.WriteString("</a></figcaption>")
		r.close("figure")
	case schema.NodeCallout:
		kind := attr(n, "kind")
		if kind == "" {
			kind = "info"
		}
		r.open("div", "id", id, "class", "callout callout-"+kind, "data-icon", attr(n, "icon"))
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("div")
	case schema.NodeColumns:
		mode := attr(n, "mode")
		if mode == "" {
			mode = "normal"
		}
		r.open("div", "id", id, "class", "columns columns-"+mode)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("div")
	case schema.NodeColumn:
		style := ""
		if w, ok := attrFloat(n, "width"); ok {
			style = fmt.Sprintf("flex-basis:%g%%", w)
		}
		r.open("div", "class", "column", "style", style)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("div")
	case schema.NodeDetails:
		openAttr := ""
		if attrBool(n, "open", true) {
			openAttr = "open"
		}
		r.open("details", "id", id, "open", openAttr)
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("details")
	case schema.NodeDetailsSummary:
		r.open("summary")
		r.inlines(n.Content)
		r.close("summary")
	case schema.NodeDetailsContent:
		r.open("div", "class", "details-content")
		r.b.WriteByte('\n')
		r.blocks(n.Content)
		r.close("div")
	case schema.NodeMathBlock:
		r.open("div", "id", id, "class", "math-block", "data-latex", attr(n, "latex"))
		r.b.WriteString("$$" + esc(attr(n, "latex")) + "$$")
		r.close("div")
	case schema.NodeMermaid:
		r.open("pre", "id", id, "class", "mermaid")
		r.b.WriteString(esc(attr(n, "source")))
		r.close("pre")
	case schema.NodeDrawio, schema.NodeExcalidraw:
		r.diagram(n)
	case schema.NodeEmbed:
		r.embed(n)
	case schema.NodeTransclusion:
		r.transclusion(n)
	case schema.NodeFootnotes:
		r.footnoteSection(n)
	case schema.NodePageBreak:
		r.open("div", "id", id, "class", "page-break", "style", "page-break-after:always")
		r.close("div")
	case schema.NodeToc:
		r.toc(n)
	case schema.NodeSubpages:
		r.subpages(n)
	default:
		// A block type the renderer does not know is a schema/renderer drift;
		// render its children so no text is lost and mark the gap.
		r.open("div", "class", "unknown-node", "data-node-type", n.Type)
		r.blocks(n.Content)
		r.close("div")
	}
}

func (r *htmlRenderer) blocks(nodes []*schema.Node) {
	for _, c := range nodes {
		r.block(c)
	}
}

// listItemBody renders "paragraph block*": the first paragraph inline (so a
// simple item reads as <li>text</li>), nested blocks after it.
func (r *htmlRenderer) listItemBody(n *schema.Node) {
	for i, c := range n.Content {
		if i == 0 && c.Type == schema.NodeParagraph {
			if style := blockStyle(c); style != "" {
				r.open("p", "style", style)
				r.inlines(c.Content)
				r.b.WriteString("</p>")
			} else {
				r.inlines(c.Content)
			}
			continue
		}
		if i == 1 {
			r.b.WriteByte('\n')
		}
		r.block(c)
	}
}

func (r *htmlRenderer) table(n *schema.Node) {
	r.open("table", "id", attr(n, "id"))
	r.b.WriteString("\n<tbody>\n")
	for _, row := range n.Content {
		r.b.WriteString("<tr>")
		for _, cell := range row.Content {
			tag := "td"
			if cell.Type == schema.NodeTableHeader {
				tag = "th"
			}
			colspan, rowspan := "", ""
			if v := attrInt(cell, "colspan", 1); v > 1 {
				colspan = strconv.Itoa(v)
			}
			if v := attrInt(cell, "rowspan", 1); v > 1 {
				rowspan = strconv.Itoa(v)
			}
			style := ""
			if bg := attr(cell, "backgroundColor"); bg != "" {
				style = "background-color:" + bg
			}
			r.open(tag, "colspan", colspan, "rowspan", rowspan, "style", style)
			r.cellBody(cell)
			r.b.WriteString("</" + tag + ">")
		}
		r.b.WriteString("</tr>\n")
	}
	r.b.WriteString("</tbody>\n")
	r.close("table")
}

// cellBody renders a single paragraph inline, anything richer as blocks.
func (r *htmlRenderer) cellBody(cell *schema.Node) {
	if len(cell.Content) == 1 && cell.Content[0].Type == schema.NodeParagraph &&
		blockStyle(cell.Content[0]) == "" {
		r.inlines(cell.Content[0].Content)
		return
	}
	r.blocks(cell.Content)
}

func (r *htmlRenderer) image(n *schema.Node) {
	src := attr(n, "src")
	if aid := attr(n, "attachmentId"); aid != "" {
		src = r.opts.attachmentURL(aid)
	} else {
		src = safeURL(src)
	}
	r.open("figure", "id", attr(n, "id"), "class", "image "+alignClass(n),
		"data-attachment-id", attr(n, "attachmentId"))
	r.open("img", "src", src, "alt", attr(n, "alt"), "title", attr(n, "title"),
		"width", dim(n, "width"), "height", dim(n, "height"), "loading", "lazy")
	if t := attr(n, "title"); t != "" {
		r.open("figcaption")
		r.b.WriteString(esc(t))
		r.b.WriteString("</figcaption>")
	}
	r.close("figure")
}

func (r *htmlRenderer) diagram(n *schema.Node) {
	kind := "drawio"
	if n.Type == schema.NodeExcalidraw {
		kind = "excalidraw"
	}
	r.open("figure", "id", attr(n, "id"), "class", "diagram diagram-"+kind+" "+alignClass(n),
		"data-attachment-id", attr(n, "attachmentId"), "data-preview-attachment-id", attr(n, "previewAttachmentId"))
	if pid := attr(n, "previewAttachmentId"); pid != "" {
		r.open("img", "src", r.opts.attachmentURL(pid), "alt", r.opts.str("diagram"),
			"width", dim(n, "width"), "height", dim(n, "height"), "loading", "lazy")
	} else {
		r.open("a", "class", "diagram-source", "href", r.opts.attachmentURL(attr(n, "attachmentId")))
		r.b.WriteString(esc(r.opts.str("diagram")))
		r.b.WriteString("</a>")
	}
	r.close("figure")
}

func (r *htmlRenderer) embed(n *schema.Node) {
	provider, raw := attr(n, "provider"), attr(n, "url")
	url := safeURL(raw)
	if r.opts.EmbedURL != nil {
		// The allow-list decides, and it decides now rather than at the
		// document's last save, so tightening the policy takes effect on the
		// next page load.
		url = safeURL(r.opts.EmbedURL(provider, raw))
	}
	r.open("div", "id", attr(n, "id"), "class", "embed embed-"+provider+" "+alignClass(n),
		"data-provider", provider)
	if url == "" {
		r.b.WriteString(esc(r.opts.str("embedUnavailable")))
	} else {
		r.open("iframe", "src", url, "width", dim(n, "width"), "height", dim(n, "height"),
			"sandbox", "allow-scripts allow-same-origin allow-popups allow-presentation",
			"referrerpolicy", "no-referrer", "loading", "lazy", "allowfullscreen", "allowfullscreen")
		r.b.WriteString("</iframe>")
	}
	r.close("div")
}

func (r *htmlRenderer) transclusion(n *schema.Node) {
	pageID, blockID := attr(n, "sourcePageId"), attr(n, "sourceBlockId")
	r.open("div", "id", attr(n, "id"), "class", "transclusion",
		"data-source-page-id", pageID, "data-source-block-id", blockID)
	r.b.WriteByte('\n')
	if r.opts.TransclusionContent != nil && r.depth == 0 {
		if node, ok := r.opts.TransclusionContent(pageID, blockID); ok && node != nil {
			r.depth++
			r.block(node)
			r.depth--
			r.close("div")
			return
		}
	}
	r.open("p", "class", "transclusion-empty")
	r.b.WriteString(esc(r.opts.str("transclusionEmpty")))
	r.close("p")
	r.close("div")
}

func (r *htmlRenderer) footnoteSection(n *schema.Node) {
	if len(n.Content) == 0 {
		return
	}
	r.open("section", "id", attr(n, "id"), "class", "footnotes", "aria-label", r.opts.str("footnotes"))
	r.b.WriteString("\n<ol>\n")
	for _, fn := range n.Content {
		id := attr(fn, "footnoteId")
		num := r.footnotes[id]
		r.open("li", "id", "fn-"+id, "value", strconv.Itoa(num))
		r.b.WriteByte('\n')
		r.blocks(fn.Content)
		r.open("a", "class", "footnote-backref", "href", "#fnref-"+id, "aria-label", "back")
		r.b.WriteString("↩︎</a>")
		r.close("li")
	}
	r.b.WriteString("</ol>\n")
	r.close("section")
}

func (r *htmlRenderer) toc(n *schema.Node) {
	r.open("nav", "id", attr(n, "id"), "class", "toc", "aria-label", r.opts.str("toc"))
	r.b.WriteString("\n<ol>\n")
	for _, h := range r.st.Headings {
		r.open("li", "class", "toc-level-"+strconv.Itoa(h.Level))
		if h.ID != "" {
			r.open("a", "href", "#"+h.ID)
			r.b.WriteString(esc(h.Text))
			r.b.WriteString("</a>")
		} else {
			r.b.WriteString(esc(h.Text))
		}
		r.b.WriteString("</li>\n")
	}
	r.b.WriteString("</ol>\n")
	r.close("nav")
}

func (r *htmlRenderer) subpages(n *schema.Node) {
	r.open("div", "id", attr(n, "id"), "class", "subpages", "data-subpages", "true")
	if r.opts.Subpages != nil {
		refs := r.opts.Subpages()
		if len(refs) > 0 {
			r.b.WriteString("\n<ul>\n")
			for _, ref := range refs {
				r.b.WriteString("<li>")
				r.open("a", "href", ref.URL, "data-page-id", ref.ID)
				r.b.WriteString(esc(nameOr(ref.Title, r.opts.str("untitledPage"))))
				r.b.WriteString("</a></li>\n")
			}
			r.b.WriteString("</ul>\n")
		}
	}
	r.close("div")
}

// ---- inline ------------------------------------------------------------------

func (r *htmlRenderer) inlines(nodes []*schema.Node) {
	for _, c := range nodes {
		r.inline(c)
	}
}

func (r *htmlRenderer) inline(n *schema.Node) {
	switch n.Type {
	case schema.NodeText:
		r.markedText(n)
	case schema.NodeHardBreak:
		r.b.WriteString("<br>")
	case schema.NodeMention:
		r.open("span", "class", "mention", "data-user-id", attr(n, "userId"))
		r.b.WriteString("@" + esc(attr(n, "label")))
		r.b.WriteString("</span>")
	case schema.NodePageLink:
		id := attr(n, "pageId")
		title, ok := r.opts.pageTitle(id)
		class := "page-link"
		if !ok {
			class += " page-link-unresolved"
			title = r.opts.str("pageUnavailable")
		} else if title == "" {
			title = r.opts.str("untitledPage")
		}
		r.open("a", "class", class, "href", r.opts.pageURL(id), "data-page-id", id)
		r.b.WriteString(esc(title))
		r.b.WriteString("</a>")
	case schema.NodeMathInline:
		r.open("span", "class", "math-inline", "data-latex", attr(n, "latex"))
		r.b.WriteString("$" + esc(attr(n, "latex")) + "$")
		r.b.WriteString("</span>")
	case schema.NodeStatus:
		color := attr(n, "color")
		if color == "" {
			color = "gray"
		}
		r.open("span", "class", "status status-"+color)
		r.b.WriteString(esc(attr(n, "text")))
		r.b.WriteString("</span>")
	case schema.NodeFootnoteRef:
		id := attr(n, "footnoteId")
		num := r.footnotes[id]
		r.open("sup", "class", "footnote-ref", "id", "fnref-"+id)
		r.open("a", "href", "#fn-"+id)
		r.b.WriteString(strconv.Itoa(num))
		r.b.WriteString("</a></sup>")
	default:
		// Unknown inline: emit its text so nothing disappears.
		r.b.WriteString(esc(inlineText(&schema.Node{Content: []*schema.Node{n}})))
	}
}

// markedText wraps a text node in its marks, outermost first.
func (r *htmlRenderer) markedText(n *schema.Node) {
	marks := orderedMarks(n.Marks)
	var closers []string
	for _, m := range marks {
		switch m.Type {
		case schema.MarkLink:
			href := safeURL(markAttr(m, "href"))
			internal := false
			if b, ok := m.Attrs["internal"].(bool); ok {
				internal = b
			}
			if internal || strings.HasPrefix(href, "/") {
				r.open("a", "href", href, "title", markAttr(m, "title"))
			} else {
				r.open("a", "href", href, "title", markAttr(m, "title"),
					"rel", "noopener noreferrer", "target", "_blank")
			}
			closers = append(closers, "a")
		case schema.MarkBold:
			r.b.WriteString("<strong>")
			closers = append(closers, "strong")
		case schema.MarkItalic:
			r.b.WriteString("<em>")
			closers = append(closers, "em")
		case schema.MarkUnderline:
			r.b.WriteString("<u>")
			closers = append(closers, "u")
		case schema.MarkStrike:
			r.b.WriteString("<s>")
			closers = append(closers, "s")
		case schema.MarkHighlight:
			style := ""
			if c := markAttr(m, "color"); c != "" {
				style = "background-color:" + c
			}
			r.open("mark", "style", style)
			closers = append(closers, "mark")
		case schema.MarkTextStyle:
			style := ""
			if c := markAttr(m, "color"); c != "" {
				style = "color:" + c
			}
			r.open("span", "style", style)
			closers = append(closers, "span")
		case schema.MarkSubscript:
			r.b.WriteString("<sub>")
			closers = append(closers, "sub")
		case schema.MarkSuperscript:
			r.b.WriteString("<sup>")
			closers = append(closers, "sup")
		case schema.MarkCode:
			r.b.WriteString("<code>")
			closers = append(closers, "code")
		}
	}
	r.b.WriteString(esc(n.Text))
	for i := len(closers) - 1; i >= 0; i-- {
		r.b.WriteString("</" + closers[i] + ">")
	}
}

// ---- small helpers -------------------------------------------------------------

func alignClass(n *schema.Node) string {
	a := attr(n, "align")
	if a == "" {
		a = "center"
	}
	return "align-" + a
}

func dim(n *schema.Node, name string) string {
	if v := attrInt(n, name, 0); v > 0 {
		return strconv.Itoa(v)
	}
	return ""
}

func nameOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func humanSize(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := unit, 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
