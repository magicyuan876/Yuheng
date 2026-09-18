// Package markdown converts Markdown in the Yuheng dialect (技术方案 §6.3, the
// output of internal/docs/render) back into a validated ProseMirror document.
// It is the entry point for Markdown imports and for AI write-back, and the
// round-trip counterpart of the renderer: render.Markdown(ToDocument(md))
// reproduces md for every construct the dialect can express.
//
// Parsing is goldmark (CommonMark + GFM tables, strikethrough, task lists,
// footnotes) extended with the dialect's own syntax: `::: kind` callout
// containers, `$$` math blocks, `$…$` inline math and `==text==` highlights.
// HTML that the renderer emits for nodes without Markdown syntax (details,
// page breaks, column markers, <u>/<sub>/<sup>/<span style="color">) is
// recognised and mapped back; other HTML is kept as text.
package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ---- custom AST nodes ------------------------------------------------------

// Callout is a `::: kind … :::` container.
type Callout struct {
	ast.BaseBlock
	// Variant is the callout kind (info/success/warning/danger).
	Variant string
}

// KindCallout is the node kind of Callout.
var KindCallout = ast.NewNodeKind("DocsCallout")

// Kind implements ast.Node.
func (n *Callout) Kind() ast.NodeKind { return KindCallout }

// Dump implements ast.Node.
func (n *Callout) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Variant": n.Variant}, nil)
}

// MathBlock is a `$$ … $$` display-math block.
type MathBlock struct{ ast.BaseBlock }

// KindMathBlock is the node kind of MathBlock.
var KindMathBlock = ast.NewNodeKind("DocsMathBlock")

// Kind implements ast.Node.
func (n *MathBlock) Kind() ast.NodeKind { return KindMathBlock }

// Dump implements ast.Node.
func (n *MathBlock) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// IsRaw marks the block's lines as raw text (no inline parsing).
func (n *MathBlock) IsRaw() bool { return true }

// MathInline is `$…$`.
type MathInline struct {
	ast.BaseInline
	Segment text.Segment
}

// KindMathInline is the node kind of MathInline.
var KindMathInline = ast.NewNodeKind("DocsMathInline")

// Kind implements ast.Node.
func (n *MathInline) Kind() ast.NodeKind { return KindMathInline }

// Dump implements ast.Node.
func (n *MathInline) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// Highlight is `==text==`.
type Highlight struct{ ast.BaseInline }

// KindHighlight is the node kind of Highlight.
var KindHighlight = ast.NewNodeKind("DocsHighlight")

// Kind implements ast.Node.
func (n *Highlight) Kind() ast.NodeKind { return KindHighlight }

// Dump implements ast.Node.
func (n *Highlight) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// ---- block parsers -----------------------------------------------------------

var calloutOpenRE = regexp.MustCompile(`^:::\s*([a-zA-Z]+)\s*$`)

type calloutParser struct{}

func (calloutParser) Trigger() []byte { return []byte{':'} }

func (calloutParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockIndent()
	if pos < 0 || pos >= len(line) {
		return nil, parser.NoChildren
	}
	m := calloutOpenRE.FindSubmatch(bytes.TrimRight(line[pos:], "\r\n"))
	if m == nil {
		return nil, parser.NoChildren
	}
	kind := string(bytes.ToLower(m[1]))
	switch kind {
	case "info", "success", "warning", "danger":
	case "tip", "note":
		kind = "info"
	case "caution", "error":
		kind = "danger"
	default:
		kind = "info"
	}
	reader.Advance(segment.Len() - 1)
	return &Callout{Variant: kind}, parser.HasChildren
}

func (calloutParser) Continue(_ ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if bytes.Equal(bytes.TrimSpace(line), []byte(":::")) {
		reader.Advance(segment.Len() - 1)
		return parser.Close
	}
	return parser.Continue | parser.HasChildren
}

func (calloutParser) Close(ast.Node, text.Reader, parser.Context) {}
func (calloutParser) CanInterruptParagraph() bool                 { return true }
func (calloutParser) CanAcceptIndentedLine() bool                 { return false }

type mathBlockParser struct{}

func (mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (mathBlockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockIndent()
	if pos < 0 || pos >= len(line) {
		return nil, parser.NoChildren
	}
	if !bytes.Equal(bytes.TrimSpace(line[pos:]), []byte("$$")) {
		return nil, parser.NoChildren
	}
	reader.Advance(segment.Len() - 1)
	return &MathBlock{}, parser.NoChildren
}

func (mathBlockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if bytes.Equal(bytes.TrimSpace(line), []byte("$$")) {
		reader.Advance(segment.Len() - 1)
		return parser.Close
	}
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return parser.Continue | parser.NoChildren
}

func (mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}
func (mathBlockParser) CanInterruptParagraph() bool                 { return true }
func (mathBlockParser) CanAcceptIndentedLine() bool                 { return false }

// ---- inline parsers ----------------------------------------------------------

type mathInlineParser struct{}

func (mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (mathInlineParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) < 3 || line[0] != '$' || line[1] == '$' || line[1] == ' ' {
		return nil
	}
	end := -1
	for i := 1; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '$' {
			end = i
			break
		}
	}
	if end <= 1 || line[end-1] == ' ' {
		return nil
	}
	n := &MathInline{Segment: text.NewSegment(segment.Start+1, segment.Start+end)}
	block.Advance(end + 1)
	return n
}

func (mathInlineParser) CloseBlock(ast.Node, text.Reader, parser.Context) {}

type highlightParser struct{}

func (highlightParser) Trigger() []byte { return []byte{'='} }

func (highlightParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) < 5 || !bytes.HasPrefix(line, []byte("==")) || line[2] == '=' || line[2] == ' ' {
		return nil
	}
	end := bytes.Index(line[2:], []byte("=="))
	if end <= 0 || line[2+end-1] == ' ' {
		return nil
	}
	n := &Highlight{}
	n.AppendChild(n, ast.NewTextSegment(text.NewSegment(segment.Start+2, segment.Start+2+end)))
	block.Advance(end + 4)
	return n
}

func (highlightParser) CloseBlock(ast.Node, text.Reader, parser.Context) {}

// newParser builds the goldmark instance with the dialect's extensions.
func newParser() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithBlockParsers(
				util.Prioritized(calloutParser{}, 700),
				util.Prioritized(mathBlockParser{}, 710),
			),
			parser.WithInlineParsers(
				util.Prioritized(mathInlineParser{}, 150),
				util.Prioritized(highlightParser{}, 160),
			),
		),
	)
}
