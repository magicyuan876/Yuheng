package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sourceOfTruth is where the editor reads the model from; the embedded copy
// must be byte-identical or the two ends will drift.
const sourceOfTruth = "../../../packages/docs-schema/schema.json"

func TestEmbeddedSchemaMatchesSource(t *testing.T) {
	src, err := os.ReadFile(filepath.FromSlash(sourceOfTruth))
	if err != nil {
		t.Fatalf("read %s: %v", sourceOfTruth, err)
	}
	if !bytes.Equal(normalizeNewlines(src), normalizeNewlines(Embedded())) {
		t.Fatalf("internal/docs/schema/schema.json differs from %s; run `go generate ./internal/docs/schema/...`",
			sourceOfTruth)
	}
}

func TestGeneratedNamesMatchSchema(t *testing.T) {
	s := Default()
	// Spot-check a few generated constants against the loaded definition so a
	// stale names_gen.go is caught even when schema.json was regenerated.
	for _, name := range []string{NodeDoc, NodeParagraph, NodeHeading, NodeTable, NodeTransclusion, NodeStatus} {
		if _, ok := s.Nodes[name]; !ok {
			t.Errorf("generated node constant %q is not in the schema", name)
		}
	}
	for _, name := range []string{MarkBold, MarkLink, MarkTextStyle} {
		if _, ok := s.Marks[name]; !ok {
			t.Errorf("generated mark constant %q is not in the schema", name)
		}
	}
	if len(s.NodeNames()) != nodeConstCount(t) {
		t.Errorf("schema has %d nodes but names_gen.go has %d Node constants; run go generate",
			len(s.NodeNames()), nodeConstCount(t))
	}
}

func nodeConstCount(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile("names_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n\tNode")
}

func TestDefaultSchemaShape(t *testing.T) {
	s := Default()
	if s.TopNode != "doc" {
		t.Fatalf("topNode = %q", s.TopNode)
	}
	if got := s.Nodes["heading"].Attrs["level"]; got == nil || !got.Required {
		t.Fatalf("heading.level must be required")
	}
	if got := s.Nodes["paragraph"].Attrs["id"]; got == nil || !got.Nullable || !got.HasDefault {
		t.Fatalf("paragraph.id must come from the blockId attrSet as nullable with a default")
	}
	if s.Nodes["codeBlock"].AllowsMark("bold") {
		t.Fatalf("codeBlock must not allow marks")
	}
	if !s.Nodes["paragraph"].AllowsMark("bold") {
		t.Fatalf("paragraph must allow marks")
	}
	if !s.Marks["code"].excludeAl || !s.Marks["subscript"].excludes["superscript"] {
		t.Fatalf("mark exclusion rules not compiled")
	}
	inline := s.Group("inline")
	for _, want := range []string{"text", "hardBreak", "mention", "pageLink", "mathInline", "status", "footnoteRef"} {
		if !contains(inline, want) {
			t.Errorf("inline group missing %q", want)
		}
	}
	for _, n := range s.Nodes {
		if n.Inline && !contains(inline, n.Name) {
			t.Errorf("inline node %q not in inline group", n.Name)
		}
	}
}

func TestLoadRejectsInconsistentDefinitions(t *testing.T) {
	base := map[string]any{}
	if err := json.Unmarshal(Embedded(), &base); err != nil {
		t.Fatal(err)
	}
	mutate := func(t *testing.T, f func(m map[string]any)) []byte {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal(Embedded(), &m); err != nil {
			t.Fatal(err)
		}
		f(m)
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	nodes := func(m map[string]any) map[string]any { return m["nodes"].(map[string]any) }
	cases := map[string]func(m map[string]any){
		"content names unknown node": func(m map[string]any) {
			nodes(m)["blockquote"].(map[string]any)["content"] = "nosuch+"
		},
		"unknown attrSet": func(m map[string]any) {
			nodes(m)["blockquote"].(map[string]any)["use"] = []any{"nosuch"}
		},
		"topNode missing": func(m map[string]any) { m["topNode"] = "nosuch" },
		"required with default": func(m map[string]any) {
			nodes(m)["heading"].(map[string]any)["attrs"].(map[string]any)["level"] = map[string]any{
				"type": "int", "required": true, "default": 1,
			}
		},
		"neither required nor default": func(m map[string]any) {
			nodes(m)["heading"].(map[string]any)["attrs"].(map[string]any)["level"] = map[string]any{"type": "int"}
		},
		"null default without nullable": func(m map[string]any) {
			nodes(m)["callout"].(map[string]any)["attrs"].(map[string]any)["icon"] = map[string]any{
				"type": "string", "default": nil,
			}
		},
		"unknown format": func(m map[string]any) {
			nodes(m)["pageLink"].(map[string]any)["attrs"].(map[string]any)["pageId"] = map[string]any{
				"type": "string", "required": true, "format": "nosuch",
			}
		},
		"mark excludes unknown mark": func(m map[string]any) {
			m["marks"].(map[string]any)["bold"] = map[string]any{"excludes": "nosuch"}
		},
		"node allows unknown mark": func(m map[string]any) {
			nodes(m)["paragraph"].(map[string]any)["marks"] = "bold nosuch"
		},
		"unknown top-level key": func(m map[string]any) { m["extra"] = true },
	}
	for name, f := range cases {
		if _, err := Load(mutate(t, f)); err == nil {
			t.Errorf("%s: Load should fail", name)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func normalizeNewlines(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
