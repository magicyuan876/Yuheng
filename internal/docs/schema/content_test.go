package schema

import (
	"strings"
	"testing"
)

func testSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := Load(Embedded())
	if err != nil {
		t.Fatalf("load embedded schema: %v", err)
	}
	return s
}

func TestContentMatcher(t *testing.T) {
	s := testSchema(t)
	cases := []struct {
		expr     string
		children string // space separated, "" for none
		ok       bool
		failedAt int
	}{
		{"block+", "paragraph heading", true, 2},
		{"block+", "", false, 0},
		{"block+", "paragraph text", false, 1},
		{"inline*", "", true, 0},
		{"inline*", "text hardBreak mention", true, 3},
		{"inline*", "paragraph", false, 0},
		{"paragraph block*", "paragraph", true, 1},
		{"paragraph block*", "paragraph bulletList codeBlock", true, 3},
		{"paragraph block*", "bulletList", false, 0},
		{"paragraph block*", "paragraph paragraph text", false, 2},
		{"(tableCell | tableHeader)+", "tableHeader tableCell tableCell", true, 3},
		{"(tableCell | tableHeader)+", "tableCell paragraph", false, 1},
		{"detailsSummary detailsContent", "detailsSummary detailsContent", true, 2},
		{"detailsSummary detailsContent", "detailsSummary", false, 1},
		{"detailsSummary detailsContent", "detailsContent detailsSummary", false, 0},
		{"column{2,5}", "column", false, 1},
		{"column{2,5}", "column column", true, 2},
		{"column{2,5}", "column column column column column", true, 5},
		{"column{2,5}", "column column column column column column", false, 5},
		{"heading? paragraph*", "", true, 0},
		{"heading? paragraph*", "heading paragraph paragraph", true, 3},
		{"heading? paragraph*", "heading heading", false, 1},
		{"text*", "text text", true, 2},
		{"footnote*", "footnote footnote", true, 2},
		{"footnote*", "", true, 0},
	}
	for _, c := range cases {
		m, err := compileContent(c.expr, s)
		if err != nil {
			t.Fatalf("compile %q: %v", c.expr, err)
		}
		res := m.Match(strings.Fields(c.children))
		if res.OK != c.ok || res.FailedAt != c.failedAt {
			t.Errorf("%q on [%s]: got ok=%v failedAt=%d expected=%v; want ok=%v failedAt=%d",
				c.expr, c.children, res.OK, res.FailedAt, res.Expected, c.ok, c.failedAt)
		}
	}
}

func TestContentMatcherExpected(t *testing.T) {
	s := testSchema(t)
	m, err := compileContent("detailsSummary detailsContent", s)
	if err != nil {
		t.Fatal(err)
	}
	res := m.Match([]string{"detailsSummary"})
	if res.OK || len(res.Expected) != 1 || res.Expected[0] != "detailsContent" {
		t.Fatalf("expected hint should name detailsContent, got %+v", res)
	}
	m, err = compileContent("block+", s)
	if err != nil {
		t.Fatal(err)
	}
	res = m.Match([]string{"text"})
	if res.OK || len(res.Expected) != len(s.Group("block")) {
		t.Fatalf("group expansion should list every block node, got %d of %d", len(res.Expected), len(s.Group("block")))
	}
}

func TestContentExpressionErrors(t *testing.T) {
	s := testSchema(t)
	bad := []string{"", "paragraph |", "(paragraph", "paragraph)", "nosuchnode", "paragraph{3,1}", "paragraph{a}", "+"}
	for _, expr := range bad {
		if _, err := compileContent(expr, s); err == nil {
			t.Errorf("expression %q should not compile", expr)
		}
	}
}

func TestMatcherIsLinearOnAdversarialInput(t *testing.T) {
	s := testSchema(t)
	m, err := compileContent("(paragraph | block)* heading", s)
	if err != nil {
		t.Fatal(err)
	}
	children := make([]string, 20000)
	for i := range children {
		children[i] = "paragraph"
	}
	// Ends with a paragraph, so the trailing heading is missing: must fail at
	// the end without blowing up in time or memory.
	res := m.Match(children)
	if res.OK || res.FailedAt != len(children) {
		t.Fatalf("expected failure at end, got %+v", res)
	}
}
