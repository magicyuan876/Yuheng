package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite golden outputs")

const goldenDocs = "../../../packages/docs-schema/golden/valid"

const (
	uuidA = "0b2a5d4e-6f3c-4a1b-9c8d-7e6f5a4b3c2d"
	uuidB = "1c3b6e5f-7a4d-4b2c-8d9e-6f7a5b4c3d2e"
)

func testOptions() Options {
	return Options{
		AttachmentURL: func(id string) string { return "/att/" + id },
		PageURL:       func(id string) string { return "/p/" + id },
		PageTitle: func(id string) (string, bool) {
			if id == "a1B2c3D4" {
				return "Target Page", true
			}
			return "", false
		},
		TransclusionContent: func(pageID, blockID string) (*schema.Node, bool) {
			if blockID == "blk_para_001" {
				return &schema.Node{
					Type: "paragraph", Content: []*schema.Node{{Type: "text", Text: "embedded text"}},
				}, true
			}
			return nil, false
		},
		Subpages: func() []PageRef {
			return []PageRef{{ID: "c1", Title: "Child one", URL: "/p/c1"}, {ID: "c2", Title: "", URL: "/p/c2"}}
		},
	}
}

// TestGoldenOutputs renders every schema golden document to HTML, Markdown
// and text and compares with the checked-in expectations under
// testdata/golden. Run `go test ./internal/docs/render -update` after an
// intentional rendering change and review the diff.
func TestGoldenOutputs(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(filepath.FromSlash(goldenDocs), "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			out, err := RenderJSON(data, testOptions())
			require.NoError(t, err)
			compareGolden(t, name+".html", out.HTML)
			compareGolden(t, name+".md", out.Markdown)
			compareGolden(t, name+".txt", out.Text)
		})
	}
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s; run with -update", name)
	require.Equal(t, normalize(string(want)), normalize(got),
		"golden %s differs; run with -update if intended", name)
}

func normalize(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func mustDoc(t *testing.T, json string) *schema.Node {
	t.Helper()
	node, _, err := schema.Default().Validate([]byte(json))
	require.NoError(t, err)
	return node
}

// p builds a paragraph JSON fragment from inline fragments.
func p(inlines ...string) string {
	return `{"type":"paragraph","content":[` + strings.Join(inlines, ",") + `]}`
}

func text(s string, marks ...string) string {
	if len(marks) == 0 {
		return `{"type":"text","text":"` + s + `"}`
	}
	return `{"type":"text","text":"` + s + `","marks":[` + strings.Join(marks, ",") + `]}`
}

func doc(blocks ...string) string {
	return `{"type":"doc","content":[` + strings.Join(blocks, ",") + `]}`
}

func TestHTMLEscapesEverything(t *testing.T) {
	d := mustDoc(t, doc(
		`{"type":"heading","attrs":{"level":1,"id":"blk_x_1"},"content":[`+text(`<script>alert(1)</script>`)+`]}`,
		p(text(`a & b`, `{"type":"link","attrs":{"href":"https://x.test/?a=1&b=\"q\""}}`)),
		`{"type":"codeBlock","attrs":{"language":"html"},"content":[`+text(`<b>raw</b>`)+`]}`,
		`{"type":"image","attrs":{"src":"https://img.test/a.png","alt":"\" onerror=\"alert(1)"}}`,
	))
	out := HTML(d, Options{})
	require.NotContains(t, out, "<script>")
	require.Contains(t, out, "&lt;script&gt;alert(1)&lt;/script&gt;")
	require.Contains(t, out, `href="https://x.test/?a=1&amp;b=&#34;q&#34;"`)
	require.Contains(t, out, "<code class=\"language-html\">&lt;b&gt;raw&lt;/b&gt;</code>")
	require.Contains(t, out, `alt="&#34; onerror=&#34;alert(1)"`)
	require.NotContains(t, out, `onerror="alert`)
}

func TestUnsafeURLsAreDropped(t *testing.T) {
	// The validator rejects javascript: hrefs, so build the node directly to
	// prove the renderer is a second line of defence.
	d := &schema.Node{Type: "doc", Content: []*schema.Node{
		{Type: "paragraph", Content: []*schema.Node{{
			Type: "text", Text: "x",
			Marks: []schema.Mark{{Type: "link", Attrs: map[string]any{"href": "javascript:alert(1)"}}},
		}}},
		{Type: "embed", Attrs: map[string]any{"provider": "p", "url": "data:text/html,<script>"}},
	}}
	out := HTML(d, Options{})
	require.NotContains(t, out, "javascript:")
	require.NotContains(t, out, "data:text")
	require.Contains(t, out, "Embedded content unavailable")
	md := Markdown(d, Options{})
	require.NotContains(t, md, "javascript:")
}

func TestMarkNestingIsCanonical(t *testing.T) {
	link := `{"type":"link","attrs":{"href":"https://e.test"}}`
	a := mustDoc(t, doc(p(text("x", `{"type":"italic"}`, `{"type":"bold"}`, link))))
	b := mustDoc(t, doc(p(text("x", link, `{"type":"bold"}`, `{"type":"italic"}`))))
	require.Equal(t, HTML(a, Options{}), HTML(b, Options{}), "mark order in JSON must not change the output")
	require.Contains(t, HTML(a, Options{}),
		`<a href="https://e.test" rel="noopener noreferrer" target="_blank"><strong><em>x</em></strong></a>`)
	require.Equal(t, "[***x***](https://e.test)\n", Markdown(a, Options{}))
}

func TestMarkdownKeepsWhitespaceOutsideWrappers(t *testing.T) {
	d := mustDoc(t, doc(p(
		text("plain"),
		text(" bold ", `{"type":"bold"}`),
		text("link ", `{"type":"link","attrs":{"href":"https://e.test"}}`),
		text(" under", `{"type":"underline"}`),
	)))
	// Two spaces: the source text nodes carry one each ("link " + " under").
	require.Equal(t, "plain **bold** [link](https://e.test)  <u>under</u>\n", Markdown(d, Options{}))
}

func TestMarkdownEscaping(t *testing.T) {
	d := mustDoc(t, doc(
		p(text("# not a heading")),
		p(text("1. not a list")),
		p(text("snake_case and 2*3 and [brackets] and a|b")),
		p(text("- not a bullet")),
	))
	require.Equal(t, "\\# not a heading\n\n1\\. not a list\n\n"+
		"snake\\_case and 2\\*3 and \\[brackets\\] and a|b\n\n\\- not a bullet\n", Markdown(d, Options{}))
}

func TestFootnoteNumberingFollowsReferenceOrder(t *testing.T) {
	fn := func(id, body string) string {
		return `{"type":"footnote","attrs":{"footnoteId":"` + id + `"},"content":[` + p(text(body)) + `]}`
	}
	d := mustDoc(t, doc(
		p(text("A"), `{"type":"footnoteRef","attrs":{"footnoteId":"fn_0002"}}`,
			text(" B"), `{"type":"footnoteRef","attrs":{"footnoteId":"fn_0001"}}`),
		`{"type":"footnotes","content":[`+fn("fn_0001", "note a")+`,`+fn("fn_0002", "note b")+`]}`,
	))
	out := HTML(d, Options{})
	require.Contains(t, out, `<sup class="footnote-ref" id="fnref-fn_0002"><a href="#fn-fn_0002">1</a></sup>`)
	require.Contains(t, out, `<sup class="footnote-ref" id="fnref-fn_0001"><a href="#fn-fn_0001">2</a></sup>`)
	require.Contains(t, out, `<li id="fn-fn_0002" value="1">`)
	require.Contains(t, out, `<li id="fn-fn_0001" value="2">`)
	require.Equal(t, []string{"fn_0002", "fn_0001"}, Extract(d).FootnoteIDs)
	require.Contains(t, Markdown(d, Options{}), "[^fn_0002]: note b")
}

func TestNestedListsInMarkdown(t *testing.T) {
	item := func(inner ...string) string {
		return `{"type":"listItem","content":[` + strings.Join(inner, ",") + `]}`
	}
	d := mustDoc(t, doc(`{"type":"orderedList","attrs":{"start":3},"content":[`+
		item(p(text("three")), `{"type":"bulletList","content":[`+item(p(text("nested")))+`]}`)+`,`+
		item(p(text("four")))+`]}`))
	require.Equal(t, "3. three\n   - nested\n4. four\n", Markdown(d, Options{}))
	html := HTML(d, Options{})
	require.Contains(t, html, `<ol start="3">`)
	require.Contains(t, html, "<li>three\n<ul>\n<li>nested</li>\n</ul>\n</li>")
}

func TestTableColspanPadsMarkdown(t *testing.T) {
	cell := func(kind, attrs, body string) string {
		return `{"type":"` + kind + `"` + attrs + `,"content":[` + p(text(body)) + `]}`
	}
	d := mustDoc(t, doc(`{"type":"table","content":[`+
		`{"type":"tableRow","content":[`+cell("tableHeader", "", "a|b")+`,`+cell("tableHeader", "", "h2")+`]},`+
		`{"type":"tableRow","content":[`+cell("tableCell", `,"attrs":{"colspan":2}`, "wide")+`]}]}`))
	require.Equal(t, "| a\\|b | h2 |\n| --- | --- |\n| wide |  |\n", Markdown(d, Options{}))
	require.Contains(t, HTML(d, Options{}), `<td colspan="2">wide</td>`)
}

func TestExtractCollectsStructure(t *testing.T) {
	d := mustDoc(t, doc(
		`{"type":"heading","attrs":{"level":2,"id":"blk_h2"},"content":[`+text("Intro 介绍")+`]}`,
		`{"type":"paragraph","attrs":{"id":"blk_p1"},"content":[`+
			text("hello 世界 test", `{"type":"link","attrs":{"href":"https://a.test"}}`)+`,`+
			`{"type":"mention","attrs":{"userId":"`+uuidA+`","label":"Ming"}},`+
			`{"type":"pageLink","attrs":{"pageId":"a1B2c3D4"}},{"type":"pageLink","attrs":{"pageId":"a1B2c3D4"}}]}`,
		`{"type":"image","attrs":{"attachmentId":"`+uuidA+`"}}`,
		`{"type":"drawio","attrs":{"attachmentId":"`+uuidB+`","previewAttachmentId":"`+uuidA+`"}}`,
		`{"type":"transclusion","attrs":{"sourcePageId":"`+uuidB+`","sourceBlockId":"blk_para_001"}}`,
		`{"type":"toc"}`,
	))
	st := Extract(d)
	require.Equal(t, []Heading{{Level: 2, Text: "Intro 介绍", ID: "blk_h2"}}, st.Headings)
	require.Equal(t, []string{"blk_h2", "blk_p1"}, st.BlockIDs)
	require.Equal(t, []string{"a1B2c3D4"}, st.PageLinks, "duplicates collapsed")
	require.Equal(t, []string{uuidA}, st.Mentions)
	require.Equal(t, []string{uuidA, uuidB}, st.AttachmentIDs)
	require.Equal(t, []string{"https://a.test"}, st.ExternalLinks)
	require.Len(t, st.Transclusions, 1)
	require.True(t, st.HasTOC)
	// Intro 介 绍 hello 世 界 test Ming = 8 words
	require.Equal(t, 8, st.WordCount)
}

func TestWordCountMixedScripts(t *testing.T) {
	words, chars := countWords("Hello, 世界！ GPT-4 是 model。")
	// Hello, 世, 界, GPT, 4, 是, model = 7 words
	require.Equal(t, 7, words)
	require.Greater(t, chars, 10)
	words, _ = countWords("")
	require.Equal(t, 0, words)
	words, _ = countWords("   \n\t ")
	require.Equal(t, 0, words)
	words, _ = countWords("こんにちは한국어")
	require.Equal(t, 8, words)
}

func TestTextProjection(t *testing.T) {
	cell := func(body string) string { return `{"type":"tableCell","content":[` + p(text(body)) + `]}` }
	d := mustDoc(t, doc(
		`{"type":"heading","attrs":{"level":1},"content":[`+text("Title")+`]}`,
		`{"type":"paragraph"}`,
		p(text("line one"), `{"type":"hardBreak"}`, text("line two")),
		`{"type":"table","content":[{"type":"tableRow","content":[`+cell("c1")+`,`+cell("c2")+`]}]}`,
		`{"type":"codeBlock","content":[`+text("x := 1")+`]}`,
		p(text("ping"), `{"type":"mention","attrs":{"userId":"`+uuidA+`","label":"Ming"}}`, text("now")),
	))
	require.Equal(t, "Title\nline one\nline two\nc1\tc2\nx := 1\nping @Ming now", Text(d))
}

func TestTransclusionExpandsOneLevel(t *testing.T) {
	d := mustDoc(t, doc(`{"type":"transclusion","attrs":{"sourcePageId":"`+uuidB+
		`","sourceBlockId":"blk_para_001"}}`))
	nested := &schema.Node{
		Type: "transclusion", Attrs: map[string]any{"sourcePageId": "x", "sourceBlockId": "blk_para_001"},
	}
	calls := 0
	opts := Options{TransclusionContent: func(pageID, blockID string) (*schema.Node, bool) {
		calls++
		return nested, true // always hands back another transclusion
	}}
	out := HTML(d, opts)
	require.Equal(t, 1, calls, "nested transclusions are not expanded")
	require.Contains(t, out, "Embedded block unavailable")
}

func TestLocaleStrings(t *testing.T) {
	d := mustDoc(t, doc(p(`{"type":"pageLink","attrs":{"pageId":"a1B2c3D4"}}`)))
	require.Contains(t, HTML(d, Options{Locale: "zh-CN"}), "页面不可用")
	require.Contains(t, HTML(d, Options{Locale: "fr-FR"}), "Page unavailable")
}
