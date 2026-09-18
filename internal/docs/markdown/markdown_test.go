package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/stretchr/testify/require"
)

const goldenDocs = "../../../packages/docs-schema/golden/valid"

// roundTripExcluded lists golden documents whose Markdown rendering is, by
// design, lossy: the construct has no Markdown syntax and degrades to a link
// or placeholder text. They are checked for stability instead of equality.
var roundTripExcluded = map[string]bool{
	"attachment": true, "audio": true, "video": true, "pdfEmbed": true, "drawio": true, "excalidraw": true,
	"embed": true, "mention": true, "pageLink": true, "status": true, "transclusion": true, "subpages": true,
	"image":     true, // attachment id needs a resolver; covered by TestImageResolution
	"empty-doc": true,
}

// TestRoundTripGoldens renders each golden document to Markdown, converts it
// back, renders again and expects identical Markdown. This is the contract
// between the renderer's dialect and this parser.
func TestRoundTripGoldens(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(filepath.FromSlash(goldenDocs), "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	opts := render.Options{
		AttachmentURL: func(id string) string { return "/att/" + id },
		PageURL:       func(id string) string { return "/p/" + id },
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			original, _, err := schema.Default().Validate(data)
			require.NoError(t, err)
			md1 := render.Markdown(original, opts)

			doc, report, err := ToDocument([]byte(md1), Options{})
			require.NoError(t, err, "converted document must validate")
			md2 := render.Markdown(doc, opts)
			if roundTripExcluded[name] {
				// Lossy by design: still, converting the second rendering must be stable.
				doc2, _, err := ToDocument([]byte(md2), Options{})
				require.NoError(t, err)
				require.Equal(t, md2, render.Markdown(doc2, opts), "lossy construct must reach a fixed point")
				return
			}
			require.Equal(t, md1, md2, "round trip changed the Markdown; warnings: %v", report.Warnings)
		})
	}
}

func parse(t *testing.T, md string) *schema.Node {
	t.Helper()
	doc, _, err := ToDocument([]byte(md), Options{})
	require.NoError(t, err)
	return doc
}

func toJSON(t *testing.T, n *schema.Node) string {
	t.Helper()
	b, err := json.Marshal(n)
	require.NoError(t, err)
	return string(b)
}

func TestBasicBlocks(t *testing.T) {
	doc := parse(t, "# Title\n\nSome *em* and **strong** and ~~gone~~ and `code`.\n\n---\n\n> quoted\n")
	require.Len(t, doc.Content, 4)
	require.Equal(t, "heading", doc.Content[0].Type)
	require.Equal(t, 1, doc.Content[0].Attrs["level"])
	p := doc.Content[1]
	require.Equal(t, "paragraph", p.Type)
	js := toJSON(t, p)
	require.Contains(t, js, `"marks":[{"type":"italic"}],"text":"em"`)
	require.Contains(t, js, `"marks":[{"type":"bold"}],"text":"strong"`)
	require.Contains(t, js, `"marks":[{"type":"strike"}],"text":"gone"`)
	require.Contains(t, js, `"marks":[{"type":"code"}],"text":"code"`)
	require.Equal(t, "horizontalRule", doc.Content[2].Type)
	require.Equal(t, "blockquote", doc.Content[3].Type)
}

func TestListsAndTasks(t *testing.T) {
	doc := parse(t, "3. three\n   - nested\n4. four\n\n- [x] done\n- [ ] todo\n")
	ol := doc.Content[0]
	require.Equal(t, "orderedList", ol.Type)
	require.Equal(t, 3, ol.Attrs["start"])
	require.Len(t, ol.Content, 2)
	first := ol.Content[0]
	require.Equal(t, "listItem", first.Type)
	require.Equal(t, "paragraph", first.Content[0].Type)
	require.Equal(t, "bulletList", first.Content[1].Type)
	tl := doc.Content[1]
	require.Equal(t, "taskList", tl.Type)
	require.Equal(t, true, tl.Content[0].Attrs["checked"])
	require.Equal(t, false, tl.Content[1].Attrs["checked"])
	require.Equal(t, "done", tl.Content[0].Content[0].Content[0].Text)
}

func TestDialectConstructs(t *testing.T) {
	md := "::: warning\ncareful\n:::\n\n$$\n\\int_0^1 x\n$$\n\nInline $e^x$ and ==hl== here.\n\n" +
		"```mermaid\nflowchart LR\n  A-->B\n```\n\n[[toc]]\n\n" +
		"<div style=\"page-break-after:always\"></div>\n"
	doc := parse(t, md)
	types := []string{}
	for _, b := range doc.Content {
		types = append(types, b.Type)
	}
	require.Equal(t, []string{"callout", "mathBlock", "paragraph", "mermaid", "toc", "pageBreak"}, types)
	require.Equal(t, "warning", doc.Content[0].Attrs["kind"])
	require.Equal(t, "careful", doc.Content[0].Content[0].Content[0].Text)
	require.Equal(t, `\int_0^1 x`, doc.Content[1].Attrs["latex"])
	js := toJSON(t, doc.Content[2])
	require.Contains(t, js, `"type":"mathInline","attrs":{"latex":"e^x"}`)
	require.Contains(t, js, `"marks":[{"type":"highlight"`)
	require.Contains(t, js, `"text":"hl"`)
	require.Equal(t, "flowchart LR\n  A-->B", doc.Content[3].Attrs["source"])
}

func TestCalloutAliasesAndNesting(t *testing.T) {
	doc := parse(t, "::: tip\n- item\n\n> quote inside\n:::\n")
	require.Equal(t, "callout", doc.Content[0].Type)
	require.Equal(t, "info", doc.Content[0].Attrs["kind"], "tip maps to info")
	require.Equal(t, "bulletList", doc.Content[0].Content[0].Type)
	require.Equal(t, "blockquote", doc.Content[0].Content[1].Type)
}

func TestTablesAndCells(t *testing.T) {
	doc := parse(t, "| a\\|b | h2 |\n| --- | --- |\n| **wide** | x<br>y |\n")
	table := doc.Content[0]
	require.Equal(t, "table", table.Type)
	require.Len(t, table.Content, 2)
	header := table.Content[0]
	require.Equal(t, "tableHeader", header.Content[0].Type)
	require.Equal(t, "a|b", header.Content[0].Content[0].Content[0].Text)
	body := table.Content[1]
	require.Equal(t, "tableCell", body.Content[0].Type)
	require.Equal(t, "wide", body.Content[0].Content[0].Content[0].Text)
	require.Equal(t, "bold", body.Content[0].Content[0].Content[0].Marks[0].Type)
	cell2 := body.Content[1].Content[0].Content
	require.Equal(t, "hardBreak", cell2[1].Type, "<br> in a cell becomes a hard break")
}

func TestFootnotes(t *testing.T) {
	doc := parse(t, "Claim[^fn_0001] and more[^weird].\n\n[^fn_0001]: Source.\n\n[^weird]: Other.\n")
	p := doc.Content[0]
	refs := 0
	for _, n := range p.Content {
		if n.Type == "footnoteRef" {
			refs++
		}
	}
	require.Equal(t, 2, refs)
	fns := doc.Content[len(doc.Content)-1]
	require.Equal(t, "footnotes", fns.Type)
	require.Len(t, fns.Content, 2)
	require.Equal(t, "fn_0001", fns.Content[0].Attrs["footnoteId"], "valid labels are kept")
	require.Regexp(t, `^fn_\d{4}$`, fns.Content[1].Attrs["footnoteId"], "invalid labels are replaced")
}

func TestHTMLMarksDetailsAndColumns(t *testing.T) {
	md := "plain <u>under</u> and <sub>sub</sub> and <span style=\"color:#ff0000\">red</span>\n\n" +
		"<details open>\n<summary>Toggle **me**</summary>\n\nhidden body\n\n</details>\n\n" +
		"<!-- columns:2 -->\n\nleft\n\n<!-- column -->\n\nright\n\n<!-- /columns -->\n"
	doc := parse(t, md)
	js := toJSON(t, doc.Content[0])
	require.Contains(t, js, `"marks":[{"type":"underline"}],"text":"under"`)
	require.Contains(t, js, `"marks":[{"type":"subscript"}],"text":"sub"`)
	require.Contains(t, js, `"marks":[{"type":"textStyle","attrs":{"color":"#ff0000"}}],"text":"red"`)
	details := doc.Content[1]
	require.Equal(t, "details", details.Type)
	require.Equal(t, true, details.Attrs["open"])
	require.Equal(t, "detailsSummary", details.Content[0].Type)
	require.Equal(t, "me", details.Content[0].Content[1].Text)
	require.Equal(t, "bold", details.Content[0].Content[1].Marks[0].Type)
	require.Equal(t, "hidden body", details.Content[1].Content[0].Content[0].Text)
	cols := doc.Content[2]
	require.Equal(t, "columns", cols.Type)
	require.Len(t, cols.Content, 2)
	require.Equal(t, "left", cols.Content[0].Content[0].Content[0].Text)
	require.Equal(t, "right", cols.Content[1].Content[0].Content[0].Text)
}

func TestImageResolution(t *testing.T) {
	md := "![alt text](images/pic.png \"Title\")\n\n![ext](https://img.test/a.png)\n\n![lost](../nowhere.png)\n"
	doc, report, err := ToDocument([]byte(md), Options{
		ResolveAttachment: func(dest string) (string, bool) {
			if dest == "images/pic.png" {
				return "0b2a5d4e-6f3c-4a1b-9c8d-7e6f5a4b3c2d", true
			}
			return "", false
		},
	})
	require.NoError(t, err)
	require.Equal(t, "image", doc.Content[0].Type)
	require.Equal(t, "0b2a5d4e-6f3c-4a1b-9c8d-7e6f5a4b3c2d", doc.Content[0].Attrs["attachmentId"])
	require.Equal(t, "Title", doc.Content[0].Attrs["title"])
	require.Equal(t, "image", doc.Content[1].Type)
	require.Equal(t, "https://img.test/a.png", doc.Content[1].Attrs["src"])
	require.Equal(t, "paragraph", doc.Content[2].Type, "unresolvable relative image degrades to text")
	require.Equal(t, "[lost]", doc.Content[2].Content[0].Text)
	require.NotEmpty(t, report.Warnings)
}

func TestLinksAndPageLinks(t *testing.T) {
	doc, _, err := ToDocument([]byte("see [Target](/docs/pages/a1B2c3D4) or [ext](https://e.test \"t\") "+
		"or [bad](javascript:alert(1)) or <https://auto.link>\n"), Options{
		ResolvePage: func(dest string) (string, bool) {
			if strings.HasPrefix(dest, "/docs/pages/") {
				return strings.TrimPrefix(dest, "/docs/pages/"), true
			}
			return "", false
		},
	})
	require.NoError(t, err)
	js := toJSON(t, doc.Content[0])
	require.Contains(t, js, `{"type":"pageLink","attrs":{"pageId":"a1B2c3D4"}}`)
	require.Contains(t, js, `"href":"https://e.test","title":"t"`)
	require.NotContains(t, js, "javascript:")
	require.Contains(t, js, `"text":" or bad or "`)
	require.Contains(t, js, `"href":"https://auto.link"`)
}

func TestImagesInsideParagraphAreHoisted(t *testing.T) {
	doc := parse(t, "before ![a](https://x.test/a.png) after\n")
	require.Len(t, doc.Content, 2)
	require.Equal(t, "paragraph", doc.Content[0].Type)
	require.Equal(t, "before  after", plain(doc.Content[0]))
	require.Equal(t, "image", doc.Content[1].Type)
}

func TestEmptyAndOddInput(t *testing.T) {
	doc := parse(t, "")
	require.Len(t, doc.Content, 1)
	require.Equal(t, "paragraph", doc.Content[0].Type)

	doc = parse(t, "<custom-tag>weird</custom-tag>\n\n<!-- just a comment -->\n\n  \n")
	for _, b := range doc.Content {
		require.Equal(t, "paragraph", b.Type)
	}

	// Deeply nested quotes stay within the schema's depth limit.
	deep := strings.Repeat("> ", 40) + "deep\n"
	_, _, err := ToDocument([]byte(deep), Options{})
	require.NoError(t, err)
}

func TestEscapesRoundTrip(t *testing.T) {
	opts := render.Options{}
	src := parse(t, "\\# not a heading\n\n1\\. not a list\n\nsnake\\_case and 2\\*3 and \\[brackets\\]\n")
	md := render.Markdown(src, opts)
	again := parse(t, md)
	require.Equal(t, md, render.Markdown(again, opts))
	require.Equal(t, "# not a heading", plain(src.Content[0]))
	require.Equal(t, "snake_case and 2*3 and [brackets]", plain(src.Content[2]))
}
