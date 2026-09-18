package schema

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The golden corpus is shared with the TypeScript side, which validates the
// same files through prosemirror-model. Both implementations must agree.
const goldenDir = "../../../packages/docs-schema/golden"

func TestGoldenValidDocuments(t *testing.T) {
	s := Default()
	files, err := filepath.Glob(filepath.Join(filepath.FromSlash(goldenDir), "valid", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden documents found under %s/valid (%v)", goldenDir, err)
	}
	covered := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		node, stats, err := s.Validate(data)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if stats.Nodes == 0 {
			t.Errorf("%s: stats not collected", filepath.Base(f))
		}
		markTypes(node, covered)
	}
	for _, name := range s.NodeNames() {
		if !covered[name] {
			t.Errorf("no golden document exercises node %q", name)
		}
	}
	for _, name := range s.MarkNames() {
		if !covered["mark:"+name] {
			t.Errorf("no golden document exercises mark %q", name)
		}
	}
}

func markTypes(n *Node, seen map[string]bool) {
	seen[n.Type] = true
	for _, m := range n.Marks {
		seen["mark:"+m.Type] = true
	}
	for _, c := range n.Content {
		markTypes(c, seen)
	}
}

// Invalid goldens are {"expect": "<code>", "doc": {...}}.
func TestGoldenInvalidDocuments(t *testing.T) {
	s := Default()
	files, err := filepath.Glob(filepath.Join(filepath.FromSlash(goldenDir), "invalid", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no invalid golden documents found (%v)", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wrapper struct {
			Expect string          `json:"expect"`
			Doc    json.RawMessage `json:"doc"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		_, _, err = s.Validate(wrapper.Doc)
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: expected a ValidationError, got %v", filepath.Base(f), err)
			continue
		}
		if !verr.Has(Code(wrapper.Expect)) {
			t.Errorf("%s: expected code %q, got %v", filepath.Base(f), wrapper.Expect, verr.Problems)
		}
	}
}

func TestValidateReportsPaths(t *testing.T) {
	s := Default()
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"ok"}]},
	  {"type":"heading","attrs":{"level":9},"content":[{"type":"text","text":"x"}]}]}`
	_, _, err := s.Validate([]byte(doc))
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if len(verr.Problems) != 1 || verr.Problems[0].Code != CodeAttrValue ||
		verr.Problems[0].Path != "doc/content[1]@level" {
		t.Fatalf("unexpected problems: %v", verr.Problems)
	}
}

func TestValidateStrictJSON(t *testing.T) {
	s := Default()
	cases := map[string]string{
		"unknown key":   `{"type":"doc","content":[{"type":"paragraph","foo":1}]}`,
		"array root":    `[{"type":"doc"}]`,
		"missing type":  `{"content":[]}`,
		"mark no type":  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{}]}]}]}`,
		"trailing junk": `{"type":"doc","content":[{"type":"paragraph"}]} x`,
	}
	for name, doc := range cases {
		_, _, err := s.Validate([]byte(doc))
		var verr *ValidationError
		if !errors.As(err, &verr) || !verr.Has(CodeParse) {
			t.Errorf("%s: expected parse error, got %v", name, err)
		}
	}
}

func TestValidateLimits(t *testing.T) {
	small, err := Load([]byte(strings.Replace(string(Embedded()),
		`"maxNodes": 200000`, `"maxNodes": 5`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `{"type":"doc","content":[{"type":"paragraph"},{"type":"paragraph"},{"type":"paragraph"},
	  {"type":"paragraph"},{"type":"paragraph"},{"type":"paragraph"}]}`
	_, stats, err := small.Validate([]byte(doc))
	var verr *ValidationError
	if !errors.As(err, &verr) || !verr.Has(CodeNodeCount) {
		t.Fatalf("expected node_count, got %v (stats %+v)", err, stats)
	}

	deep := strings.Repeat(`{"type":"blockquote","content":[`, 70) + `{"type":"paragraph"}` + strings.Repeat(`]}`, 70)
	_, _, err = Default().Validate([]byte(`{"type":"doc","content":[` + deep + `]}`))
	if !errors.As(err, &verr) || !verr.Has(CodeDepth) {
		t.Fatalf("expected depth error, got %v", err)
	}
}

func TestValidateAllowedRoot(t *testing.T) {
	s := Default()
	body := `{"type":"paragraph","content":[{"type":"text","text":"a comment","marks":[{"type":"bold"}]}]}`
	if _, _, err := s.Validate([]byte(body), ValidateOptions{AllowedRoot: "paragraph"}); err != nil {
		t.Fatalf("comment body should validate with AllowedRoot: %v", err)
	}
	_, _, err := s.Validate([]byte(body))
	var verr *ValidationError
	if !errors.As(err, &verr) || !verr.Has(CodeRootType) {
		t.Fatalf("paragraph root must be rejected by default, got %v", err)
	}
}

func TestValidateMarkRules(t *testing.T) {
	s := Default()
	text := func(marks string) string {
		return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[` + marks + `]}]}]}`
	}
	cases := map[string]struct {
		doc  string
		code Code
	}{
		"code excludes bold":       {text(`{"type":"code"},{"type":"bold"}`), CodeMarkConflict},
		"sub excludes sup":         {text(`{"type":"subscript"},{"type":"superscript"}`), CodeMarkConflict},
		"duplicate link":           {text(`{"type":"link","attrs":{"href":"https://a"}},{"type":"link","attrs":{"href":"https://b"}}`), CodeMarkConflict},
		"unknown mark":             {text(`{"type":"glow"}`), CodeUnknownMark},
		"link without href":        {text(`{"type":"link"}`), CodeMissingAttr},
		"link javascript scheme":   {text(`{"type":"link","attrs":{"href":"javascript:alert(1)"}}`), CodeAttrValue},
		"bold inside codeBlock":    {`{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"x","marks":[{"type":"bold"}]}]}]}`, CodeMarkNotAllowed},
		"marks on block node":      {`{"type":"doc","content":[{"type":"paragraph","marks":[{"type":"bold"}]}]}`, CodeMarkNotAllowed},
		"highlight with bad color": {text(`{"type":"highlight","attrs":{"color":"url(x)"}}`), CodeAttrValue},
	}
	for name, c := range cases {
		_, _, err := s.Validate([]byte(c.doc))
		var verr *ValidationError
		if !errors.As(err, &verr) || !verr.Has(c.code) {
			t.Errorf("%s: expected %s, got %v", name, c.code, err)
		}
	}
	// bold + italic + link together is fine.
	if _, _, err := s.Validate([]byte(text(`{"type":"bold"},{"type":"italic"},{"type":"link","attrs":{"href":"https://example.com"}}`))); err != nil {
		t.Fatalf("compatible marks rejected: %v", err)
	}
}
