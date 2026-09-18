// Package render turns validated ProseMirror JSON into HTML, Markdown, plain
// text and structural facts (headings, links, mentions, attachments, block
// ids, word count). It is a pure function package: no database, no I/O.
//
// Everything that touches a document body downstream of the editor goes
// through here — the persist path (text for search, links for backlinks),
// history diffs, exports, server-rendered share pages and knowledge-base
// ingestion — so the three outputs stay consistent with each other and with
// the schema in packages/docs-schema/schema.json.
//
// Input contract: the node tree MUST have passed schema.Validate. The
// renderer still escapes every string and refuses unexpected URL schemes, so
// a bug upstream cannot turn into script injection, but it does not re-check
// structure.
package render

import (
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// PageRef names a page for links, transclusions and sub-page lists.
type PageRef struct {
	ID    string
	Title string
	URL   string
}

// Options supply the environment-dependent parts of rendering. Every field is
// optional; the defaults produce correct, self-contained output with relative
// application URLs.
type Options struct {
	// AttachmentURL maps an attachment id to the URL that serves it.
	AttachmentURL func(attachmentID string) string
	// PageURL maps a page id to its URL.
	PageURL func(pageID string) string
	// PageTitle resolves a page id to its current title; ok=false renders a
	// neutral placeholder (the page may be deleted or invisible).
	PageTitle func(pageID string) (title string, ok bool)
	// TransclusionContent resolves an embedded block; ok=false renders a
	// "content unavailable" placeholder. Nested transclusions inside the
	// returned block are not expanded.
	TransclusionContent func(pageID, blockID string) (node *schema.Node, ok bool)
	// Subpages lists the child pages of the rendered page for the subpages
	// node. nil renders an empty container the client fills in.
	Subpages func() []PageRef
	// Locale selects fixed strings (placeholders, "Attachment", ...). Only
	// "zh-CN" and "en-US" are known; others fall back to en-US.
	Locale string
}

func (o *Options) attachmentURL(id string) string {
	if o.AttachmentURL != nil {
		return o.AttachmentURL(id)
	}
	return "/api/v1/docs/attachments/" + id
}

func (o *Options) pageURL(id string) string {
	if o.PageURL != nil {
		return o.PageURL(id)
	}
	return "/docs/pages/" + id
}

func (o *Options) pageTitle(id string) (string, bool) {
	if o.PageTitle != nil {
		return o.PageTitle(id)
	}
	return "", false
}

func (o *Options) str(key string) string {
	if o.Locale == "zh-CN" {
		if s, ok := zhCN[key]; ok {
			return s
		}
	}
	return enUS[key]
}

var enUS = map[string]string{
	"untitledPage":      "Untitled page",
	"pageUnavailable":   "Page unavailable",
	"attachment":        "Attachment",
	"diagram":           "Diagram",
	"embedUnavailable":  "Embedded content unavailable",
	"transclusionEmpty": "Embedded block unavailable",
	"toc":               "Contents",
	"subpages":          "Sub-pages",
	"footnotes":         "Footnotes",
	"pdf":               "PDF",
}

var zhCN = map[string]string{
	"untitledPage":      "无标题页面",
	"pageUnavailable":   "页面不可用",
	"attachment":        "附件",
	"diagram":           "图表",
	"embedUnavailable":  "嵌入内容不可用",
	"transclusionEmpty": "引用块不可用",
	"toc":               "目录",
	"subpages":          "子页面",
	"footnotes":         "脚注",
	"pdf":               "PDF",
}

// Output bundles every rendering of one document.
type Output struct {
	HTML      string
	Markdown  string
	Text      string
	Structure Structure
}

// Render produces every output for a validated document.
func Render(doc *schema.Node, opts Options) (*Output, error) {
	if doc == nil {
		return nil, fmt.Errorf("render: nil document")
	}
	st := Extract(doc)
	h := newHTMLRenderer(&opts, st)
	md := newMarkdownRenderer(&opts, st)
	return &Output{
		HTML:      h.render(doc),
		Markdown:  md.render(doc),
		Text:      Text(doc),
		Structure: st,
	}, nil
}

// RenderJSON validates raw ProseMirror JSON against the default schema and
// renders it. Validation errors are returned as *schema.ValidationError.
func RenderJSON(data []byte, opts Options) (*Output, error) {
	node, _, err := schema.Default().Validate(data)
	if err != nil {
		return nil, err
	}
	return Render(node, opts)
}

// HTML renders only the HTML output.
func HTML(doc *schema.Node, opts Options) string {
	st := Extract(doc)
	return newHTMLRenderer(&opts, st).render(doc)
}

// Markdown renders only the Markdown output.
func Markdown(doc *schema.Node, opts Options) string {
	st := Extract(doc)
	return newMarkdownRenderer(&opts, st).render(doc)
}

// ---- shared helpers ----------------------------------------------------------

// attr reads a string attribute; missing or non-string yields "".
func attr(n *schema.Node, name string) string {
	if n == nil || n.Attrs == nil {
		return ""
	}
	if s, ok := n.Attrs[name].(string); ok {
		return s
	}
	return ""
}

// attrInt reads a numeric attribute; missing yields def.
func attrInt(n *schema.Node, name string, def int) int {
	if n == nil || n.Attrs == nil {
		return def
	}
	switch v := n.Attrs[name].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case interface{ Int64() (int64, error) }: // json.Number
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return def
}

// attrFloat reads a numeric attribute as float64.
func attrFloat(n *schema.Node, name string) (float64, bool) {
	if n == nil || n.Attrs == nil {
		return 0, false
	}
	switch v := n.Attrs[name].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case interface{ Float64() (float64, error) }: // json.Number
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

func attrBool(n *schema.Node, name string, def bool) bool {
	if n == nil || n.Attrs == nil {
		return def
	}
	if b, ok := n.Attrs[name].(bool); ok {
		return b
	}
	return def
}

func markAttr(m schema.Mark, name string) string {
	if m.Attrs == nil {
		return ""
	}
	if s, ok := m.Attrs[name].(string); ok {
		return s
	}
	return ""
}

func hasMark(n *schema.Node, name string) bool {
	for _, m := range n.Marks {
		if m.Type == name {
			return true
		}
	}
	return false
}

// safeURL admits the schemes the schema allows and nothing else; anything
// odd (javascript:, data:, vbscript:, protocol-relative, ...) renders as an
// empty href so it can never execute. Characters inside an admitted URL are
// handled by attribute escaping, not here.
func safeURL(u string) string {
	trimmed := strings.TrimSpace(u)
	lower := strings.ToLower(trimmed)
	for _, p := range []string{"https://", "http://", "mailto:", "tel:"} {
		if strings.HasPrefix(lower, p) {
			return trimmed
		}
	}
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		return trimmed
	}
	return ""
}

// markOrder is the canonical nesting of inline marks, outermost first, so
// the same logical formatting always yields identical markup.
var markOrder = []string{
	schema.MarkLink, schema.MarkBold, schema.MarkItalic, schema.MarkUnderline, schema.MarkStrike,
	schema.MarkHighlight, schema.MarkTextStyle, schema.MarkSubscript, schema.MarkSuperscript, schema.MarkCode,
}

func orderedMarks(marks []schema.Mark) []schema.Mark {
	out := make([]schema.Mark, 0, len(marks))
	for _, name := range markOrder {
		for _, m := range marks {
			if m.Type == name {
				out = append(out, m)
			}
		}
	}
	return out
}
