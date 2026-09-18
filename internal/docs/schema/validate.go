package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is a ProseMirror JSON node as stored in docs_pages.content.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []*Node        `json:"content,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// Mark is a ProseMirror JSON mark.
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Code classifies a validation problem so callers (and tests) can match on it
// without parsing messages.
type Code string

// Validation problem codes.
const (
	CodeParse          Code = "parse"
	CodeRootType       Code = "root_type"
	CodeUnknownNode    Code = "unknown_node"
	CodeUnknownMark    Code = "unknown_mark"
	CodeUnknownAttr    Code = "unknown_attr"
	CodeMissingAttr    Code = "missing_attr"
	CodeAttrType       Code = "attr_type"
	CodeAttrValue      Code = "attr_value"
	CodeRequireOneOf   Code = "require_one_of"
	CodeContent        Code = "content"
	CodeLeafContent    Code = "leaf_content"
	CodeTextNode       Code = "text_node"
	CodeMarkNotAllowed Code = "mark_not_allowed"
	CodeMarkConflict   Code = "mark_conflict"
	CodeDepth          Code = "depth"
	CodeNodeCount      Code = "node_count"
	CodeTextSize       Code = "text_size"
)

// Problem is one validation finding.
type Problem struct {
	Code Code
	// Path locates the node, e.g. "doc/content[2]/content[0]". Attribute
	// problems append "@attr".
	Path    string
	Message string
}

func (p Problem) String() string { return fmt.Sprintf("%s at %s: %s", p.Code, p.Path, p.Message) }

// ValidationError aggregates the problems found in one document.
type ValidationError struct {
	Problems []Problem
	// Truncated is set when collection stopped at MaxProblems.
	Truncated bool
}

func (e *ValidationError) Error() string {
	if len(e.Problems) == 0 {
		return "document is invalid"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "document has %d problem(s)", len(e.Problems))
	if e.Truncated {
		b.WriteString(" (truncated)")
	}
	b.WriteString(": ")
	b.WriteString(e.Problems[0].String())
	if len(e.Problems) > 1 {
		fmt.Fprintf(&b, "; and %d more", len(e.Problems)-1)
	}
	return b.String()
}

// Has reports whether any problem carries the code.
func (e *ValidationError) Has(code Code) bool {
	for _, p := range e.Problems {
		if p.Code == code {
			return true
		}
	}
	return false
}

// ValidateOptions tunes a validation run.
type ValidateOptions struct {
	// MaxProblems stops collection after this many findings (default 20).
	MaxProblems int
	// AllowedRoot overrides the schema's topNode, e.g. to validate a comment
	// body whose root is a paragraph.
	AllowedRoot string
}

// Stats summarises a validated document.
type Stats struct {
	Nodes     int
	Depth     int
	TextBytes int
	// TextRunes counts characters in text nodes; a cheap word-count proxy.
	TextRunes int
}

// ParseDocument decodes ProseMirror JSON strictly: unknown keys, duplicate
// keys and non-object nodes are rejected. It performs no schema checks.
func ParseDocument(data []byte) (*Node, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &ValidationError{Problems: []Problem{{CodeParse, "doc", err.Error()}}}
	}
	node, err := decodeNode(raw, "doc")
	if err != nil {
		return nil, &ValidationError{Problems: []Problem{{CodeParse, "doc", err.Error()}}}
	}
	return node, nil
}

type rawNodeJSON struct {
	Type    *string           `json:"type"`
	Attrs   map[string]any    `json:"attrs"`
	Content []json.RawMessage `json:"content"`
	Marks   []json.RawMessage `json:"marks"`
	Text    *string           `json:"text"`
}

type rawMarkJSON struct {
	Type  *string        `json:"type"`
	Attrs map[string]any `json:"attrs"`
}

func decodeNode(raw json.RawMessage, path string) (*Node, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%s: node must be a JSON object", path)
	}
	var rn rawNodeJSON
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(&rn); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if rn.Type == nil || *rn.Type == "" {
		return nil, fmt.Errorf("%s: node has no type", path)
	}
	n := &Node{Type: *rn.Type, Attrs: rn.Attrs}
	if rn.Text != nil {
		n.Text = *rn.Text
	}
	for i, m := range rn.Marks {
		var rm rawMarkJSON
		mdec := json.NewDecoder(bytes.NewReader(m))
		mdec.DisallowUnknownFields()
		mdec.UseNumber()
		if err := mdec.Decode(&rm); err != nil {
			return nil, fmt.Errorf("%s/marks[%d]: %w", path, i, err)
		}
		if rm.Type == nil || *rm.Type == "" {
			return nil, fmt.Errorf("%s/marks[%d]: mark has no type", path, i)
		}
		n.Marks = append(n.Marks, Mark{Type: *rm.Type, Attrs: rm.Attrs})
	}
	if rn.Content != nil {
		n.Content = make([]*Node, 0, len(rn.Content))
		for i, c := range rn.Content {
			child, err := decodeNode(c, fmt.Sprintf("%s/content[%d]", path, i))
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
	}
	return n, nil
}

// Validate parses and validates a ProseMirror JSON document. On success it
// returns the decoded tree and its statistics; on failure the error is a
// *ValidationError listing every problem found (up to MaxProblems).
func (s *Schema) Validate(data []byte, opts ...ValidateOptions) (*Node, Stats, error) {
	node, err := ParseDocument(data)
	if err != nil {
		return nil, Stats{}, err
	}
	stats, err := s.ValidateNode(node, opts...)
	if err != nil {
		return nil, stats, err
	}
	return node, stats, nil
}

// ValidateNode validates an already-decoded tree.
func (s *Schema) ValidateNode(root *Node, opts ...ValidateOptions) (Stats, error) {
	opt := ValidateOptions{MaxProblems: 20}
	if len(opts) > 0 {
		if opts[0].MaxProblems > 0 {
			opt.MaxProblems = opts[0].MaxProblems
		}
		opt.AllowedRoot = opts[0].AllowedRoot
	}
	v := &validator{schema: s, opt: opt}
	expectedRoot := s.TopNode
	if opt.AllowedRoot != "" {
		expectedRoot = opt.AllowedRoot
	}
	if root.Type != expectedRoot {
		v.report(CodeRootType, "doc", fmt.Sprintf("root must be %q, got %q", expectedRoot, root.Type))
	}
	v.walk(root, "doc", 1)
	if v.stats.Nodes > s.Limits.MaxNodes {
		v.report(CodeNodeCount, "doc", fmt.Sprintf("%d nodes exceeds limit %d", v.stats.Nodes, s.Limits.MaxNodes))
	}
	if v.stats.TextBytes > s.Limits.MaxTextBytes {
		v.report(CodeTextSize, "doc",
			fmt.Sprintf("%d text bytes exceeds limit %d", v.stats.TextBytes, s.Limits.MaxTextBytes))
	}
	if len(v.problems) > 0 {
		return v.stats, &ValidationError{Problems: v.problems, Truncated: v.truncated}
	}
	return v.stats, nil
}

type validator struct {
	schema    *Schema
	opt       ValidateOptions
	problems  []Problem
	truncated bool
	stats     Stats
}

func (v *validator) report(code Code, path, msg string) {
	if len(v.problems) >= v.opt.MaxProblems {
		v.truncated = true
		return
	}
	v.problems = append(v.problems, Problem{Code: code, Path: path, Message: msg})
}

func (v *validator) walk(n *Node, path string, depth int) {
	v.stats.Nodes++
	if depth > v.stats.Depth {
		v.stats.Depth = depth
	}
	if depth > v.schema.Limits.MaxDepth {
		v.report(CodeDepth, path, fmt.Sprintf("nesting deeper than %d", v.schema.Limits.MaxDepth))
		return // do not descend further into a hostile tree
	}
	spec, ok := v.schema.Nodes[n.Type]
	if !ok {
		v.report(CodeUnknownNode, path, fmt.Sprintf("unknown node type %q", n.Type))
		return
	}

	if spec.Text {
		v.checkText(n, path)
	} else if n.Text != "" {
		v.report(CodeTextNode, path, fmt.Sprintf("%q nodes cannot carry text", n.Type))
	}

	v.checkAttrs(n.Attrs, spec.Attrs, spec.RequireOneOf, path)

	// Marks live on inline nodes and are constrained by the parent's
	// allow-list; checked in walkChildren where the parent is known.

	if spec.IsLeaf() {
		if len(n.Content) > 0 {
			v.report(CodeLeafContent, path, fmt.Sprintf("%q cannot have children", n.Type))
		}
		return
	}
	if n.Attrs == nil && spec.Text {
		return
	}
	v.walkChildren(n, spec, path, depth)
}

func (v *validator) walkChildren(n *Node, spec *NodeSpec, path string, depth int) {
	types := make([]string, len(n.Content))
	for i, c := range n.Content {
		types[i] = c.Type
	}
	if res := spec.matcher.Match(types); !res.OK {
		got := "end of content"
		if res.FailedAt < len(types) {
			got = fmt.Sprintf("%q", types[res.FailedAt])
		}
		v.report(CodeContent, fmt.Sprintf("%s/content[%d]", path, res.FailedAt),
			fmt.Sprintf("%q expects %s; got %s (expected one of %s)",
				n.Type, spec.Content, got, strings.Join(res.Expected, ", ")))
	}
	for i, c := range n.Content {
		childPath := fmt.Sprintf("%s/content[%d]", path, i)
		v.checkMarks(c, spec, childPath)
		v.walk(c, childPath, depth+1)
	}
}

func (v *validator) checkText(n *Node, path string) {
	if n.Text == "" {
		v.report(CodeTextNode, path, "text nodes must not be empty")
	}
	if !utf8.ValidString(n.Text) {
		v.report(CodeTextNode, path, "text is not valid UTF-8")
	}
	if len(n.Content) > 0 {
		v.report(CodeLeafContent, path, "text nodes cannot have children")
	}
	v.stats.TextBytes += len(n.Text)
	v.stats.TextRunes += utf8.RuneCountInString(n.Text)
}

func (v *validator) checkMarks(child *Node, parent *NodeSpec, path string) {
	if len(child.Marks) == 0 {
		return
	}
	childSpec, known := v.schema.Nodes[child.Type]
	if known && !childSpec.Inline && !childSpec.Text {
		v.report(CodeMarkNotAllowed, path, fmt.Sprintf("block node %q cannot carry marks", child.Type))
		return
	}
	seen := map[string]*MarkSpec{}
	for i, m := range child.Marks {
		markPath := fmt.Sprintf("%s/marks[%d]", path, i)
		ms, ok := v.schema.Marks[m.Type]
		if !ok {
			v.report(CodeUnknownMark, markPath, fmt.Sprintf("unknown mark type %q", m.Type))
			continue
		}
		if !parent.AllowsMark(m.Type) {
			v.report(CodeMarkNotAllowed, markPath, fmt.Sprintf("mark %q is not allowed inside %q", m.Type, parent.Name))
		}
		if _, dup := seen[m.Type]; dup {
			v.report(CodeMarkConflict, markPath, fmt.Sprintf("mark %q appears twice", m.Type))
		}
		for otherName, other := range seen {
			if ms.excludeAl || other.excludeAl || ms.excludes[otherName] || other.excludes[m.Type] {
				v.report(CodeMarkConflict, markPath,
					fmt.Sprintf("marks %q and %q exclude each other", m.Type, otherName))
			}
		}
		seen[m.Type] = ms
		v.checkAttrs(m.Attrs, ms.Attrs, nil, markPath)
	}
}

func (v *validator) checkAttrs(attrs map[string]any, specs map[string]*AttrSpec, oneOf [][]string, path string) {
	for name := range attrs {
		if _, ok := specs[name]; !ok {
			v.report(CodeUnknownAttr, path+"@"+name, fmt.Sprintf("unknown attribute %q", name))
		}
	}
	for name, spec := range specs {
		val, present := attrs[name]
		if !present || val == nil {
			if spec.Required {
				v.report(CodeMissingAttr, path+"@"+name, fmt.Sprintf("attribute %q is required", name))
			} else if present && !spec.Nullable {
				v.report(CodeAttrType, path+"@"+name, fmt.Sprintf("attribute %q is not nullable", name))
			}
			continue
		}
		if err := spec.checkValue(val, v.schema.Limits.MaxAttrStringBytes); err != nil {
			code := CodeAttrValue
			var te *typeError
			if errors.As(err, &te) {
				code = CodeAttrType
			}
			v.report(code, path+"@"+name, fmt.Sprintf("attribute %q: %v", name, err))
		}
	}
	for _, group := range oneOf {
		satisfied := false
		for _, name := range group {
			if val, ok := attrs[name]; ok && val != nil {
				satisfied = true
				break
			}
		}
		if !satisfied {
			v.report(CodeRequireOneOf, path, fmt.Sprintf("one of %s is required", strings.Join(group, ", ")))
		}
	}
}

type typeError struct{ msg string }

func (e *typeError) Error() string { return e.msg }

// checkValue verifies a single attribute value against its spec. It accepts
// the shapes produced both by encoding/json with UseNumber (json.Number) and
// by plain decoding (float64), so callers may build Nodes programmatically.
func (a *AttrSpec) checkValue(val any, maxStringBytes int) error {
	switch a.Type {
	case TypeString, TypeEnum:
		s, ok := val.(string)
		if !ok {
			return &typeError{fmt.Sprintf("expected string, got %s", jsonKind(val))}
		}
		if len(s) > maxStringBytes {
			return fmt.Errorf("string of %d bytes exceeds limit %d", len(s), maxStringBytes)
		}
		if !utf8.ValidString(s) {
			return fmt.Errorf("string is not valid UTF-8")
		}
		if a.MaxLength > 0 && utf8.RuneCountInString(s) > a.MaxLength {
			return fmt.Errorf("length %d exceeds maxLength %d", utf8.RuneCountInString(s), a.MaxLength)
		}
		if a.Type == TypeEnum {
			for _, allowed := range a.Values {
				if s == allowed {
					return nil
				}
			}
			return fmt.Errorf("%q is not one of %s", s, strings.Join(a.Values, ", "))
		}
		if a.format != nil && !a.format.MatchString(s) {
			return fmt.Errorf("%q does not match format %s", truncate(s, 64), a.Format)
		}
		return nil
	case TypeInt:
		f, ok := toFloat(val)
		if !ok {
			return &typeError{fmt.Sprintf("expected integer, got %s", jsonKind(val))}
		}
		if f != math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) {
			return &typeError{fmt.Sprintf("expected integer, got %v", val)}
		}
		return a.checkRange(f)
	case TypeNumber:
		f, ok := toFloat(val)
		if !ok {
			return &typeError{fmt.Sprintf("expected number, got %s", jsonKind(val))}
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return &typeError{"expected finite number"}
		}
		return a.checkRange(f)
	case TypeBool:
		if _, ok := val.(bool); !ok {
			return &typeError{fmt.Sprintf("expected boolean, got %s", jsonKind(val))}
		}
		return nil
	case TypeList:
		items, ok := val.([]any)
		if !ok {
			return &typeError{fmt.Sprintf("expected array, got %s", jsonKind(val))}
		}
		if a.MaxLength > 0 && len(items) > a.MaxLength {
			return fmt.Errorf("%d items exceeds maxLength %d", len(items), a.MaxLength)
		}
		itemSpec := &AttrSpec{Type: a.Items, Min: a.Min, Max: a.Max}
		for i, it := range items {
			if it == nil {
				// ProseMirror's colwidth uses null for "unsized" columns.
				continue
			}
			if err := itemSpec.checkValue(it, maxStringBytes); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
		return nil
	}
	return fmt.Errorf("unsupported attribute type %q", a.Type)
}

func (a *AttrSpec) checkRange(f float64) error {
	if a.Min != nil && f < *a.Min {
		return fmt.Errorf("%v is below minimum %v", f, *a.Min)
	}
	if a.Max != nil && f > *a.Max {
		return fmt.Errorf("%v is above maximum %v", f, *a.Max)
	}
	return nil
}

func toFloat(val any) (float64, bool) {
	switch n := val.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := strconv.ParseFloat(n.String(), 64)
		return f, err == nil
	}
	return 0, false
}

func jsonKind(val any) string {
	switch val.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int32, int64, json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", val)
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}
