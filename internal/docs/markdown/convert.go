package markdown

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Options resolve the references a Markdown file cannot carry on its own.
type Options struct {
	// ResolveAttachment maps an image/link destination (usually a relative
	// path inside an import bundle, or an /api/v1/docs/attachments/<id> URL)
	// to an attachment id. ok=false keeps the destination as an external URL
	// when it is absolute, or drops the image with a warning otherwise.
	ResolveAttachment func(dest string) (attachmentID string, ok bool)
	// ResolvePage maps a link destination to a page id, turning the link into
	// a pageLink node. ok=false keeps a plain link.
	ResolvePage func(dest string) (pageID string, ok bool)
}

// Report lists what the conversion could not represent faithfully.
type Report struct {
	Warnings []string
}

func (r *Report) warnf(format string, args ...any) {
	if len(r.Warnings) < 200 {
		r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
	}
}

// ToDocument parses Markdown and returns a document that satisfies the
// default schema. A non-nil error means the input produced an invalid tree,
// which is a bug in this package rather than in the input.
func ToDocument(source []byte, opts Options) (*schema.Node, *Report, error) {
	root := newParser().Parser().Parse(text.NewReader(source))
	c := &converter{src: source, opts: opts, report: &Report{}, footnoteLabels: map[int]string{}}
	c.indexFootnotes(root)
	doc := &schema.Node{Type: schema.NodeDoc, Content: c.blocks(root)}
	if len(doc.Content) == 0 {
		doc.Content = []*schema.Node{{Type: schema.NodeParagraph}}
	}
	if len(c.footnotes) > 0 {
		doc.Content = append(doc.Content, &schema.Node{Type: schema.NodeFootnotes, Content: c.footnotes})
	}
	if _, err := schema.Default().ValidateNode(doc); err != nil {
		return nil, c.report, fmt.Errorf("markdown: converted document is invalid: %w", err)
	}
	return doc, c.report, nil
}

type converter struct {
	src            []byte
	opts           Options
	report         *Report
	footnoteLabels map[int]string
	footnotes      []*schema.Node
}

var (
	columnsOpenRE  = regexp.MustCompile(`^<!--\s*columns:(\d+)\s*-->$`)
	columnSepRE    = regexp.MustCompile(`^<!--\s*column\s*-->$`)
	columnsCloseRE = regexp.MustCompile(`^<!--\s*/columns\s*-->$`)
	detailsOpenRE  = regexp.MustCompile(`(?is)^<details(\s+open)?\s*>\s*(?:<summary>(.*?)</summary>)?\s*$`)
	detailsCloseRE = regexp.MustCompile(`(?i)^</details>\s*$`)
	pageBreakRE    = regexp.MustCompile(`(?i)^<div\s+style="page-break-after:\s*always"\s*>\s*</div>\s*$`)
	tocRE          = regexp.MustCompile(`^\[\[toc\]\]$`)
	blockIDRE      = regexp.MustCompile(`^[A-Za-z0-9_-]{6,40}$`)
	spanColorRE    = regexp.MustCompile(`(?i)^<span\s+style="\s*color:\s*([^";]+)\s*;?\s*"\s*>$`)
)

// blocks converts the children of a container, handling the multi-block
// HTML marker constructs (columns, details) that span siblings.
func (c *converter) blocks(parent ast.Node) []*schema.Node {
	var out []*schema.Node
	var pending []ast.Node
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		pending = append(pending, n)
	}
	for i := 0; i < len(pending); i++ {
		n := pending[i]
		if hb, ok := n.(*ast.HTMLBlock); ok {
			raw := strings.TrimSpace(c.htmlBlockText(hb))
			if m := columnsOpenRE.FindStringSubmatch(raw); m != nil {
				node, next := c.columns(pending, i+1)
				if node != nil {
					out = append(out, node)
					i = next
					continue
				}
			}
			if m := detailsOpenRE.FindStringSubmatch(raw); m != nil {
				node, next := c.details(pending, i+1, m[1] != "", m[2])
				out = append(out, node)
				i = next
				continue
			}
		}
		out = append(out, c.block(n)...)
	}
	return out
}

// columns gathers siblings after a `<!-- columns:n -->` marker up to the
// closing marker; returns nil when the closing marker is missing.
func (c *converter) columns(siblings []ast.Node, start int) (*schema.Node, int) {
	var cols [][]ast.Node
	var cur []ast.Node
	for i := start; i < len(siblings); i++ {
		if hb, ok := siblings[i].(*ast.HTMLBlock); ok {
			raw := strings.TrimSpace(c.htmlBlockText(hb))
			if columnSepRE.MatchString(raw) {
				cols = append(cols, cur)
				cur = nil
				continue
			}
			if columnsCloseRE.MatchString(raw) {
				cols = append(cols, cur)
				if len(cols) < 2 {
					// One column is not a columns node; keep the content flat.
					c.report.warnf("columns marker with fewer than two columns flattened")
					content := c.blockList(cols[0])
					if len(content) == 0 {
						content = []*schema.Node{{Type: schema.NodeParagraph}}
					}
					return &schema.Node{Type: schema.NodeBlockquote, Content: content}, i
				}
				if len(cols) > 5 {
					c.report.warnf("columns with %d columns truncated to 5", len(cols))
					cols = cols[:5]
				}
				node := &schema.Node{Type: schema.NodeColumns}
				for _, col := range cols {
					content := c.blockList(col)
					if len(content) == 0 {
						content = []*schema.Node{{Type: schema.NodeParagraph}}
					}
					node.Content = append(node.Content, &schema.Node{Type: schema.NodeColumn, Content: content})
				}
				return node, i
			}
		}
		cur = append(cur, siblings[i])
	}
	c.report.warnf("unterminated columns marker; content kept in reading order")
	return nil, start - 1
}

func (c *converter) details(siblings []ast.Node, start int, open bool, summary string) (*schema.Node, int) {
	var body []ast.Node
	end := len(siblings) - 1
	for i := start; i < len(siblings); i++ {
		if hb, ok := siblings[i].(*ast.HTMLBlock); ok {
			if detailsCloseRE.MatchString(strings.TrimSpace(c.htmlBlockText(hb))) {
				end = i
				break
			}
		}
		body = append(body, siblings[i])
	}
	content := c.blockList(body)
	if len(content) == 0 {
		content = []*schema.Node{{Type: schema.NodeParagraph}}
	}
	summaryNode := &schema.Node{Type: schema.NodeDetailsSummary}
	if s := strings.TrimSpace(summary); s != "" {
		summaryNode.Content = c.inlineMarkdown(s)
	}
	return &schema.Node{
		Type:  schema.NodeDetails,
		Attrs: map[string]any{"open": open},
		Content: []*schema.Node{
			summaryNode,
			{Type: schema.NodeDetailsContent, Content: content},
		},
	}, end
}

// inlineMarkdown parses a short Markdown fragment as inline content (used for
// summaries recovered from HTML).
func (c *converter) inlineMarkdown(s string) []*schema.Node {
	sub := &converter{src: []byte(s), opts: c.opts, report: c.report, footnoteLabels: map[int]string{}}
	root := newParser().Parser().Parse(text.NewReader(sub.src))
	if p, ok := root.FirstChild().(*ast.Paragraph); ok {
		return sub.inlines(p)
	}
	return []*schema.Node{{Type: schema.NodeText, Text: s}}
}

func (c *converter) blockList(nodes []ast.Node) []*schema.Node {
	var out []*schema.Node
	for _, n := range nodes {
		out = append(out, c.block(n)...)
	}
	return out
}

func (c *converter) htmlBlockText(n *ast.HTMLBlock) string {
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		b.Write(seg.Value(c.src))
	}
	if n.HasClosure() {
		b.Write(n.ClosureLine.Value(c.src))
	}
	return b.String()
}

// block converts one block node; it may yield several schema blocks (a
// paragraph with images) or none (footnote definitions).
func (c *converter) block(n ast.Node) []*schema.Node {
	switch v := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return c.paragraph(n)
	case *ast.Heading:
		level := v.Level
		if level > 6 {
			level = 6
		}
		return []*schema.Node{{
			Type: schema.NodeHeading, Attrs: map[string]any{"level": level}, Content: c.inlines(v),
		}}
	case *ast.Blockquote:
		content := c.blocks(v)
		if len(content) == 0 {
			content = []*schema.Node{{Type: schema.NodeParagraph}}
		}
		return []*schema.Node{{Type: schema.NodeBlockquote, Content: content}}
	case *ast.ThematicBreak:
		return []*schema.Node{{Type: schema.NodeHorizontalRule}}
	case *ast.List:
		return []*schema.Node{c.list(v)}
	case *ast.FencedCodeBlock:
		lang := strings.TrimSpace(string(v.Language(c.src)))
		body := c.linesText(v.Lines())
		if lang == "mermaid" {
			return []*schema.Node{{Type: schema.NodeMermaid, Attrs: map[string]any{"source": body}}}
		}
		return []*schema.Node{codeBlock(lang, body)}
	case *ast.CodeBlock:
		return []*schema.Node{codeBlock("", c.linesText(v.Lines()))}
	case *ast.HTMLBlock:
		return c.htmlBlock(v)
	case *east.Table:
		return []*schema.Node{c.table(v)}
	case *Callout:
		content := c.blocks(v)
		if len(content) == 0 {
			content = []*schema.Node{{Type: schema.NodeParagraph}}
		}
		return []*schema.Node{{Type: schema.NodeCallout, Attrs: map[string]any{"kind": v.Variant}, Content: content}}
	case *MathBlock:
		return []*schema.Node{{
			Type: schema.NodeMathBlock, Attrs: map[string]any{"latex": strings.TrimSpace(c.linesText(v.Lines()))},
		}}
	case *east.FootnoteList:
		for fn := v.FirstChild(); fn != nil; fn = fn.NextSibling() {
			if f, ok := fn.(*east.Footnote); ok {
				c.footnotes = append(c.footnotes, c.footnote(f))
			}
		}
		return nil
	case *east.Footnote:
		c.footnotes = append(c.footnotes, c.footnote(v))
		return nil
	}
	c.report.warnf("unsupported block %s kept as text", n.Kind())
	return []*schema.Node{{Type: schema.NodeParagraph, Content: textNodes(string(c.nodeText(n)), nil)}}
}

func codeBlock(lang, body string) *schema.Node {
	attrs := map[string]any{"language": nil}
	if lang != "" {
		if len(lang) > 64 {
			lang = lang[:64]
		}
		attrs["language"] = lang
	}
	node := &schema.Node{Type: schema.NodeCodeBlock, Attrs: attrs}
	if body != "" {
		node.Content = []*schema.Node{{Type: schema.NodeText, Text: body}}
	}
	return node
}

func (c *converter) linesText(lines *text.Segments) string {
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(c.src))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (c *converter) htmlBlock(v *ast.HTMLBlock) []*schema.Node {
	raw := strings.TrimSpace(c.htmlBlockText(v))
	switch {
	case pageBreakRE.MatchString(raw):
		return []*schema.Node{{Type: schema.NodePageBreak}}
	case detailsCloseRE.MatchString(raw), columnsCloseRE.MatchString(raw), columnSepRE.MatchString(raw):
		return nil // stray markers without an opener
	case strings.HasPrefix(raw, "<!--"):
		return nil // comments carry nothing for the reader
	}
	c.report.warnf("HTML block kept as text: %.40s", raw)
	return []*schema.Node{{Type: schema.NodeParagraph, Content: textNodes(raw, nil)}}
}

// paragraph converts a paragraph, hoisting images (block-level in our
// schema) out of it and recognising the [[toc]] marker.
func (c *converter) paragraph(n ast.Node) []*schema.Node {
	if tocRE.Match(c.nodeText(n)) {
		return []*schema.Node{{Type: schema.NodeToc}}
	}
	ic := &inlineConverter{c: c}
	ic.walk(n, nil)
	var out []*schema.Node
	content := mergeText(ic.out)
	if len(content) > 0 || len(ic.blocks) == 0 {
		out = append(out, &schema.Node{Type: schema.NodeParagraph, Content: content})
	}
	out = append(out, ic.blocks...)
	return out
}

func (c *converter) inlines(n ast.Node) []*schema.Node {
	ic := &inlineConverter{c: c}
	ic.walk(n, nil)
	// Block-level nodes cannot live in a heading/cell; degrade them to text.
	for _, b := range ic.blocks {
		if alt := attrString(b, "alt"); alt != "" {
			ic.out = append(ic.out, &schema.Node{Type: schema.NodeText, Text: alt})
		}
	}
	return mergeText(ic.out)
}

func attrString(n *schema.Node, key string) string {
	if n.Attrs == nil {
		return ""
	}
	s, _ := n.Attrs[key].(string)
	return s
}

// ---- lists -------------------------------------------------------------------

func (c *converter) list(v *ast.List) *schema.Node {
	isTask := false
	for item := v.FirstChild(); item != nil; item = item.NextSibling() {
		if hasCheckbox(item) {
			isTask = true
			break
		}
	}
	list := &schema.Node{}
	switch {
	case isTask:
		list.Type = schema.NodeTaskList
	case v.IsOrdered():
		list.Type = schema.NodeOrderedList
		if v.Start > 1 {
			list.Attrs = map[string]any{"start": v.Start}
		}
	default:
		list.Type = schema.NodeBulletList
	}
	for item := v.FirstChild(); item != nil; item = item.NextSibling() {
		list.Content = append(list.Content, c.listItem(item, isTask))
	}
	if len(list.Content) == 0 {
		list.Content = []*schema.Node{{
			Type: schema.NodeListItem, Content: []*schema.Node{{Type: schema.NodeParagraph}},
		}}
		if isTask {
			list.Content[0].Type = schema.NodeTaskItem
		}
	}
	return list
}

func hasCheckbox(item ast.Node) bool {
	first := item.FirstChild()
	if first == nil {
		return false
	}
	_, ok := first.FirstChild().(*east.TaskCheckBox)
	return ok
}

func (c *converter) listItem(item ast.Node, task bool) *schema.Node {
	node := &schema.Node{Type: schema.NodeListItem}
	if task {
		node.Type = schema.NodeTaskItem
		checked := false
		if first := item.FirstChild(); first != nil {
			if cb, ok := first.FirstChild().(*east.TaskCheckBox); ok {
				checked = cb.IsChecked
			}
		}
		node.Attrs = map[string]any{"checked": checked}
	}
	node.Content = c.blocks(item)
	// Schema: "paragraph block*". Guarantee a leading paragraph.
	if len(node.Content) == 0 || node.Content[0].Type != schema.NodeParagraph {
		node.Content = append([]*schema.Node{{Type: schema.NodeParagraph}}, node.Content...)
	}
	return node
}

// ---- tables ------------------------------------------------------------------

func (c *converter) table(v *east.Table) *schema.Node {
	table := &schema.Node{Type: schema.NodeTable}
	for row := v.FirstChild(); row != nil; row = row.NextSibling() {
		isHeader := false
		if _, ok := row.(*east.TableHeader); ok {
			isHeader = true
		}
		r := &schema.Node{Type: schema.NodeTableRow}
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			kind := schema.NodeTableCell
			if isHeader {
				kind = schema.NodeTableHeader
			}
			r.Content = append(r.Content, &schema.Node{
				Type:    kind,
				Content: []*schema.Node{{Type: schema.NodeParagraph, Content: c.inlines(cell)}},
			})
		}
		if len(r.Content) == 0 {
			r.Content = []*schema.Node{{
				Type: schema.NodeTableCell, Content: []*schema.Node{{Type: schema.NodeParagraph}},
			}}
		}
		table.Content = append(table.Content, r)
	}
	if len(table.Content) == 0 {
		table.Content = []*schema.Node{{Type: schema.NodeTableRow, Content: []*schema.Node{{
			Type: schema.NodeTableCell, Content: []*schema.Node{{Type: schema.NodeParagraph}},
		}}}}
	}
	return table
}

// ---- footnotes ---------------------------------------------------------------

func (c *converter) indexFootnotes(root ast.Node) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if f, ok := n.(*east.Footnote); ok && entering {
			c.footnoteLabels[f.Index] = c.footnoteID(f)
		}
		return ast.WalkContinue, nil
	})
}

func (c *converter) footnoteID(f *east.Footnote) string {
	label := string(f.Ref)
	if blockIDRE.MatchString(label) {
		return label
	}
	return fmt.Sprintf("fn_%04d", f.Index)
}

func (c *converter) footnote(f *east.Footnote) *schema.Node {
	content := c.blocks(f)
	var paras []*schema.Node
	for _, b := range content {
		if b.Type == schema.NodeParagraph {
			paras = append(paras, b)
		} else {
			// Footnotes hold paragraphs only; flatten anything else to text.
			paras = append(paras, &schema.Node{Type: schema.NodeParagraph, Content: textNodes(plain(b), nil)})
		}
	}
	if len(paras) == 0 {
		paras = []*schema.Node{{Type: schema.NodeParagraph}}
	}
	return &schema.Node{
		Type: schema.NodeFootnote, Attrs: map[string]any{"footnoteId": c.footnoteID(f)}, Content: paras,
	}
}

// ---- inline --------------------------------------------------------------------

type inlineConverter struct {
	c      *converter
	out    []*schema.Node
	blocks []*schema.Node // images hoisted out of the paragraph
	// html marks opened by raw tags, in nesting order
	htmlMarks []schema.Mark
}

func (ic *inlineConverter) walk(n ast.Node, marks []schema.Mark) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		ic.inline(child, marks)
	}
}

func (ic *inlineConverter) inline(n ast.Node, marks []schema.Mark) {
	c := ic.c
	switch v := n.(type) {
	case *ast.Text:
		if v.IsRaw() {
			ic.text(string(v.Value(c.src)), marks)
		} else {
			ic.text(unescape(v.Value(c.src)), marks)
		}
		if v.HardLineBreak() {
			ic.out = append(ic.out, &schema.Node{Type: schema.NodeHardBreak})
		} else if v.SoftLineBreak() {
			ic.text(" ", marks)
		}
	case *ast.String:
		ic.text(string(v.Value), marks)
	case *ast.Emphasis:
		mark := schema.Mark{Type: schema.MarkItalic}
		if v.Level >= 2 {
			mark = schema.Mark{Type: schema.MarkBold}
		}
		ic.walk(v, appendMark(marks, mark))
	case *east.Strikethrough:
		ic.walk(v, appendMark(marks, schema.Mark{Type: schema.MarkStrike}))
	case *Highlight:
		ic.walk(v, appendMark(marks, schema.Mark{Type: schema.MarkHighlight, Attrs: map[string]any{"color": nil}}))
	case *ast.CodeSpan:
		var b strings.Builder
		for t := v.FirstChild(); t != nil; t = t.NextSibling() {
			if txt, ok := t.(*ast.Text); ok {
				b.Write(txt.Value(c.src))
			}
		}
		// Code excludes every other mark.
		ic.push(b.String(), []schema.Mark{{Type: schema.MarkCode}})
	case *ast.Link:
		dest := unescape(v.Destination)
		if c.opts.ResolvePage != nil {
			if pageID, ok := c.opts.ResolvePage(dest); ok {
				ic.out = append(ic.out, &schema.Node{
					Type: schema.NodePageLink, Attrs: map[string]any{"pageId": pageID},
				})
				return
			}
		}
		attrs := map[string]any{"href": dest}
		if len(v.Title) > 0 {
			attrs["title"] = unescape(v.Title)
		}
		if !isAllowedURL(dest) {
			c.report.warnf("link with unsupported destination kept as text: %.60s", dest)
			ic.walk(v, marks)
			return
		}
		ic.walk(v, appendMark(marks, schema.Mark{Type: schema.MarkLink, Attrs: attrs}))
	case *ast.AutoLink:
		url := string(v.URL(c.src))
		if v.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(url, "mailto:") {
			url = "mailto:" + url
		}
		if isAllowedURL(url) {
			link := schema.Mark{Type: schema.MarkLink, Attrs: map[string]any{"href": url}}
			ic.text(string(v.Label(c.src)), appendMark(marks, link))
		} else {
			ic.text(string(v.Label(c.src)), marks)
		}
	case *ast.Image:
		ic.image(v)
	case *ast.RawHTML:
		ic.rawHTML(v)
	case *east.FootnoteLink:
		id, ok := c.footnoteLabels[v.Index]
		if !ok {
			id = fmt.Sprintf("fn_%04d", v.Index)
		}
		ic.out = append(ic.out, &schema.Node{Type: schema.NodeFootnoteRef, Attrs: map[string]any{"footnoteId": id}})
	case *east.FootnoteBacklink, *east.TaskCheckBox:
		// Structural markers, not content.
	case *MathInline:
		latex := string(v.Segment.Value(c.src))
		if latex != "" {
			ic.out = append(ic.out, &schema.Node{Type: schema.NodeMathInline, Attrs: map[string]any{"latex": latex}})
		}
	default:
		if n.Type() == ast.TypeInline {
			ic.walk(n, marks)
			return
		}
		c.report.warnf("unsupported inline %s kept as text", n.Kind())
		ic.text(string(c.nodeText(n)), marks)
	}
}

func (ic *inlineConverter) text(s string, marks []schema.Mark) {
	if s == "" {
		return
	}
	ic.push(s, mergeMarks(marks, ic.htmlMarks))
}

func (ic *inlineConverter) push(s string, marks []schema.Mark) {
	node := &schema.Node{Type: schema.NodeText, Text: s}
	if len(marks) > 0 {
		node.Marks = append([]schema.Mark(nil), marks...)
	}
	ic.out = append(ic.out, node)
}

func (ic *inlineConverter) image(v *ast.Image) {
	c := ic.c
	dest := unescape(v.Destination)
	alt := string(c.nodeText(v))
	attrs := map[string]any{}
	if alt != "" {
		attrs["alt"] = alt
	}
	if len(v.Title) > 0 {
		attrs["title"] = unescape(v.Title)
	}
	if c.opts.ResolveAttachment != nil {
		if id, ok := c.opts.ResolveAttachment(dest); ok {
			attrs["attachmentId"] = id
			ic.blocks = append(ic.blocks, &schema.Node{Type: schema.NodeImage, Attrs: attrs})
			return
		}
	}
	if isAllowedURL(dest) && (strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")) {
		attrs["src"] = dest
		ic.blocks = append(ic.blocks, &schema.Node{Type: schema.NodeImage, Attrs: attrs})
		return
	}
	c.report.warnf("image %q could not be resolved; kept as text", dest)
	if alt != "" {
		ic.text("["+alt+"]", nil)
	}
}

var (
	openTagRE  = regexp.MustCompile(`(?i)^<(u|sub|sup|span|br)\b[^>]*/?>$`)
	closeTagRE = regexp.MustCompile(`(?i)^</(u|sub|sup|span)>$`)
)

// rawHTML maps the inline tags the renderer emits back to marks; anything
// else is dropped (with a warning) rather than injected as text.
func (ic *inlineConverter) rawHTML(v *ast.RawHTML) {
	var b strings.Builder
	for i := 0; i < v.Segments.Len(); i++ {
		seg := v.Segments.At(i)
		b.Write(seg.Value(ic.c.src))
	}
	tag := strings.TrimSpace(b.String())
	if m := openTagRE.FindStringSubmatch(tag); m != nil {
		switch strings.ToLower(m[1]) {
		case "br":
			ic.out = append(ic.out, &schema.Node{Type: schema.NodeHardBreak})
		case "u":
			ic.htmlMarks = append(ic.htmlMarks, schema.Mark{Type: schema.MarkUnderline})
		case "sub":
			ic.htmlMarks = append(ic.htmlMarks, schema.Mark{Type: schema.MarkSubscript})
		case "sup":
			ic.htmlMarks = append(ic.htmlMarks, schema.Mark{Type: schema.MarkSuperscript})
		case "span":
			if cm := spanColorRE.FindStringSubmatch(tag); cm != nil {
				ic.htmlMarks = append(ic.htmlMarks, schema.Mark{
					Type: schema.MarkTextStyle, Attrs: map[string]any{"color": strings.TrimSpace(cm[1])},
				})
			} else {
				ic.htmlMarks = append(ic.htmlMarks, schema.Mark{}) // placeholder so the close pops correctly
			}
		}
		return
	}
	if closeTagRE.MatchString(tag) {
		if len(ic.htmlMarks) > 0 {
			ic.htmlMarks = ic.htmlMarks[:len(ic.htmlMarks)-1]
		}
		return
	}
	ic.c.report.warnf("inline HTML dropped: %.40s", tag)
}

// ---- helpers -------------------------------------------------------------------

func appendMark(marks []schema.Mark, m schema.Mark) []schema.Mark {
	for _, existing := range marks {
		if existing.Type == m.Type {
			return marks
		}
	}
	out := make([]schema.Mark, len(marks)+1)
	copy(out, marks)
	out[len(marks)] = m
	return out
}

func mergeMarks(a, b []schema.Mark) []schema.Mark {
	out := append([]schema.Mark(nil), a...)
	for _, m := range b {
		if m.Type == "" {
			continue
		}
		out = appendMark(out, m)
	}
	return out
}

// mergeText joins adjacent text nodes with identical marks.
func mergeText(nodes []*schema.Node) []*schema.Node {
	var out []*schema.Node
	for _, n := range nodes {
		if n.Type == schema.NodeText && n.Text == "" {
			continue
		}
		if len(out) > 0 {
			last := out[len(out)-1]
			if last.Type == schema.NodeText && n.Type == schema.NodeText && sameMarks(last.Marks, n.Marks) {
				last.Text += n.Text
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

func sameMarks(a, b []schema.Mark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || fmt.Sprint(a[i].Attrs) != fmt.Sprint(b[i].Attrs) {
			return false
		}
	}
	return true
}

func textNodes(s string, marks []schema.Mark) []*schema.Node {
	if s == "" {
		return nil
	}
	n := &schema.Node{Type: schema.NodeText, Text: s}
	if len(marks) > 0 {
		n.Marks = marks
	}
	return []*schema.Node{n}
}

var entityRE = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)

// unescape resolves CommonMark backslash escapes (backslash before ASCII
// punctuation) and well-formed entity references. goldmark leaves both in
// the text segments and resolves them only in its HTML writer.
func unescape(b []byte) string {
	if bytes.IndexByte(b, '\\') < 0 && bytes.IndexByte(b, '&') < 0 {
		return string(b)
	}
	var out strings.Builder
	out.Grow(len(b))
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c == '\\' && i+1 < len(b) && util.IsPunct(b[i+1]):
			out.WriteByte(b[i+1])
			i++
		case c == '&':
			if m := entityRE.Find(b[i:]); m != nil {
				if decoded := html.UnescapeString(string(m)); decoded != string(m) {
					out.WriteString(decoded)
					i += len(m) - 1
					continue
				}
			}
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func isAllowedURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "mailto:") || strings.HasPrefix(l, "tel:")
}

// nodeText returns the plain text under a goldmark node.
func (c *converter) nodeText(n ast.Node) []byte {
	var b strings.Builder
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := child.(type) {
		case *ast.Text:
			if t.IsRaw() {
				b.Write(t.Value(c.src))
			} else {
				b.WriteString(unescape(t.Value(c.src)))
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return []byte(b.String())
}

// plain flattens a schema node to text (used when a construct must degrade).
func plain(n *schema.Node) string {
	var b strings.Builder
	var walk func(*schema.Node)
	walk = func(x *schema.Node) {
		if x.Type == schema.NodeText {
			b.WriteString(x.Text)
		}
		for _, ch := range x.Content {
			walk(ch)
		}
	}
	walk(n)
	return b.String()
}
