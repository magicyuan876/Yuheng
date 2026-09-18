// Package schema is the Go side of the Yuheng document model.
//
// The model is defined once, in packages/docs-schema/schema.json, and shared
// with the Tiptap editor. This package loads that definition, checks that it is
// internally consistent (every node named in a content expression exists, every
// attribute set referenced is declared, ...), compiles the ProseMirror content
// expressions into matchers, and validates ProseMirror JSON documents against
// it. Everything downstream of an untrusted document — the renderer, search
// indexing, exports — relies on Validate having run first.
//
// The copy of schema.json embedded here is kept identical to the source of
// truth by `go generate` and by TestEmbeddedSchemaMatchesSource.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:generate go run ./gen

//go:embed schema.json
var embedded []byte

// Embedded returns the raw bytes of the embedded schema definition.
func Embedded() []byte { return embedded }

// AttrType enumerates the value types an attribute may declare.
type AttrType string

// Attribute value types. "list" carries an Items type; "enum" carries Values.
const (
	TypeString AttrType = "string"
	TypeInt    AttrType = "int"
	TypeNumber AttrType = "number"
	TypeBool   AttrType = "bool"
	TypeEnum   AttrType = "enum"
	TypeList   AttrType = "list"
)

// AttrSpec describes one attribute of a node or mark.
type AttrSpec struct {
	Type      AttrType `json:"type"`
	Values    []string `json:"values,omitempty"`
	Default   any      `json:"default,omitempty"`
	Nullable  bool     `json:"nullable,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MaxLength int      `json:"maxLength,omitempty"`
	Items     AttrType `json:"items,omitempty"`
	Format    string   `json:"format,omitempty"`

	// HasDefault distinguishes an explicit `"default": null` from an absent
	// default. Set while loading.
	HasDefault bool `json:"-"`
	format     *regexp.Regexp
}

// NodeSpec describes one node type after attribute sets have been resolved.
type NodeSpec struct {
	Name         string
	Group        string
	Content      string
	Inline       bool
	Atom         bool
	Text         bool
	Marks        *string // nil = all marks ("_"), "" = none, else space-separated
	Use          []string
	Attrs        map[string]*AttrSpec
	RequireOneOf [][]string

	groups    []string
	matcher   *Matcher
	markAllow map[string]bool // nil means all marks allowed
	attrOrder []string
}

// MarkSpec describes one mark type.
type MarkSpec struct {
	Name     string
	Excludes string // "" none, "_" all, else space-separated mark names
	Attrs    map[string]*AttrSpec

	excludes  map[string]bool
	excludeAl bool
	attrOrder []string
}

// Limits bounds the size of a document the validator will accept.
type Limits struct {
	MaxDepth           int `json:"maxDepth"`
	MaxNodes           int `json:"maxNodes"`
	MaxTextBytes       int `json:"maxTextBytes"`
	MaxAttrStringBytes int `json:"maxAttrStringBytes"`
}

// Schema is a loaded, verified, compiled document model.
type Schema struct {
	Version int
	TopNode string
	Limits  Limits
	Nodes   map[string]*NodeSpec
	Marks   map[string]*MarkSpec

	formats   map[string]*regexp.Regexp
	groups    map[string][]string // group name -> node names, sorted
	nodeOrder []string
	markOrder []string
}

type rawNode struct {
	Group        string               `json:"group"`
	Content      string               `json:"content"`
	Inline       bool                 `json:"inline"`
	Atom         bool                 `json:"atom"`
	Text         bool                 `json:"text"`
	Marks        *string              `json:"marks"`
	Use          []string             `json:"use"`
	Attrs        map[string]*AttrSpec `json:"attrs"`
	RequireOneOf [][]string           `json:"requireOneOf"`
}

type rawMark struct {
	Excludes string               `json:"excludes"`
	Attrs    map[string]*AttrSpec `json:"attrs"`
}

type rawSchema struct {
	Comment  string                          `json:"$comment"`
	Version  int                             `json:"version"`
	TopNode  string                          `json:"topNode"`
	Limits   Limits                          `json:"limits"`
	Formats  map[string]string               `json:"formats"`
	AttrSets map[string]map[string]*AttrSpec `json:"attrSets"`
	Nodes    map[string]*rawNode             `json:"nodes"`
	Marks    map[string]*rawMark             `json:"marks"`
}

var (
	defaultOnce   sync.Once
	defaultSchema *Schema
	defaultErr    error
)

// Default returns the schema compiled from the embedded definition. The
// embedded file is verified at build time by tests, so a failure here is a
// programming error and is reported as a panic.
func Default() *Schema {
	defaultOnce.Do(func() { defaultSchema, defaultErr = Load(embedded) })
	if defaultErr != nil {
		panic(fmt.Sprintf("docs schema: embedded schema.json is invalid: %v", defaultErr))
	}
	return defaultSchema
}

// Load parses and compiles a schema definition. It fails on any internal
// inconsistency so that a broken definition can never reach production.
func Load(data []byte) (*Schema, error) {
	var raw rawSchema
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("schema: parse: %w", err)
	}
	if raw.Version != 1 {
		return nil, fmt.Errorf("schema: unsupported version %d", raw.Version)
	}
	if raw.TopNode == "" {
		return nil, fmt.Errorf("schema: topNode is required")
	}
	if raw.Limits.MaxDepth <= 0 || raw.Limits.MaxNodes <= 0 || raw.Limits.MaxTextBytes <= 0 ||
		raw.Limits.MaxAttrStringBytes <= 0 {
		return nil, fmt.Errorf("schema: every limit must be positive")
	}

	s := &Schema{
		Version: raw.Version,
		TopNode: raw.TopNode,
		Limits:  raw.Limits,
		Nodes:   make(map[string]*NodeSpec, len(raw.Nodes)),
		Marks:   make(map[string]*MarkSpec, len(raw.Marks)),
		formats: make(map[string]*regexp.Regexp, len(raw.Formats)),
		groups:  make(map[string][]string),
	}
	for name, pattern := range raw.Formats {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("schema: format %q: %w", name, err)
		}
		s.formats[name] = re
	}

	// The raw decoder cannot tell `"default": null` from a missing default, so
	// re-scan the attribute objects for the presence of the key.
	defaultKeys, err := scanDefaultKeys(data)
	if err != nil {
		return nil, err
	}

	for setName, attrs := range raw.AttrSets {
		for attrName, a := range attrs {
			a.HasDefault = defaultKeys["attrSets."+setName+"."+attrName]
			if err := s.checkAttrSpec("attrSets."+setName+"."+attrName, a); err != nil {
				return nil, err
			}
		}
	}

	for name, rn := range raw.Nodes {
		if rn == nil {
			return nil, fmt.Errorf("schema: node %q has no definition", name)
		}
		n := &NodeSpec{
			Name: name, Group: rn.Group, Content: rn.Content, Inline: rn.Inline, Atom: rn.Atom,
			Text: rn.Text, Marks: rn.Marks, Use: rn.Use, RequireOneOf: rn.RequireOneOf,
			Attrs: make(map[string]*AttrSpec),
		}
		for _, setName := range rn.Use {
			set, ok := raw.AttrSets[setName]
			if !ok {
				return nil, fmt.Errorf("schema: node %q uses unknown attrSet %q", name, setName)
			}
			for attrName, a := range set {
				if _, dup := n.Attrs[attrName]; dup {
					return nil, fmt.Errorf("schema: node %q: attribute %q declared twice via attrSets", name, attrName)
				}
				n.Attrs[attrName] = a
			}
		}
		for attrName, a := range rn.Attrs {
			if _, dup := n.Attrs[attrName]; dup {
				return nil, fmt.Errorf("schema: node %q: attribute %q overrides an attrSet attribute", name, attrName)
			}
			a.HasDefault = defaultKeys["nodes."+name+"."+attrName]
			if err := s.checkAttrSpec("nodes."+name+"."+attrName, a); err != nil {
				return nil, err
			}
			n.Attrs[attrName] = a
		}
		for _, group := range n.RequireOneOf {
			for _, attrName := range group {
				if _, ok := n.Attrs[attrName]; !ok {
					return nil, fmt.Errorf("schema: node %q: requireOneOf names unknown attribute %q", name, attrName)
				}
			}
		}
		if n.Text && (n.Content != "" || len(n.Attrs) > 0) {
			return nil, fmt.Errorf("schema: text node %q cannot declare content or attrs", name)
		}
		if n.Inline && n.Group == "" {
			return nil, fmt.Errorf("schema: inline node %q must declare a group", name)
		}
		n.groups = strings.Fields(n.Group)
		n.attrOrder = sortedKeys(n.Attrs)
		for _, g := range n.groups {
			s.groups[g] = append(s.groups[g], name)
		}
		s.Nodes[name] = n
		s.nodeOrder = append(s.nodeOrder, name)
	}
	sort.Strings(s.nodeOrder)
	for g := range s.groups {
		sort.Strings(s.groups[g])
	}
	if _, ok := s.Nodes[s.TopNode]; !ok {
		return nil, fmt.Errorf("schema: topNode %q is not a declared node", s.TopNode)
	}
	if s.Nodes[s.TopNode].Group != "" {
		return nil, fmt.Errorf("schema: topNode %q must not belong to a group", s.TopNode)
	}

	for name, rm := range raw.Marks {
		if rm == nil {
			rm = &rawMark{}
		}
		m := &MarkSpec{Name: name, Excludes: rm.Excludes, Attrs: rm.Attrs}
		if m.Attrs == nil {
			m.Attrs = map[string]*AttrSpec{}
		}
		for attrName, a := range m.Attrs {
			a.HasDefault = defaultKeys["marks."+name+"."+attrName]
			if err := s.checkAttrSpec("marks."+name+"."+attrName, a); err != nil {
				return nil, err
			}
		}
		m.attrOrder = sortedKeys(m.Attrs)
		s.Marks[name] = m
		s.markOrder = append(s.markOrder, name)
	}
	sort.Strings(s.markOrder)
	for _, m := range s.Marks {
		switch m.Excludes {
		case "":
		case "_":
			m.excludeAl = true
		default:
			m.excludes = map[string]bool{}
			for _, other := range strings.Fields(m.Excludes) {
				if _, ok := s.Marks[other]; !ok {
					return nil, fmt.Errorf("schema: mark %q excludes unknown mark %q", m.Name, other)
				}
				m.excludes[other] = true
			}
		}
	}

	// Content expressions and mark allow-lists can only be compiled once every
	// node and mark is known.
	for _, n := range s.Nodes {
		if n.Content != "" {
			matcher, err := compileContent(n.Content, s)
			if err != nil {
				return nil, fmt.Errorf("schema: node %q content %q: %w", n.Name, n.Content, err)
			}
			n.matcher = matcher
		}
		if n.Marks != nil && *n.Marks != "_" {
			n.markAllow = map[string]bool{}
			for _, mk := range strings.Fields(*n.Marks) {
				if _, ok := s.Marks[mk]; !ok {
					return nil, fmt.Errorf("schema: node %q allows unknown mark %q", n.Name, mk)
				}
				n.markAllow[mk] = true
			}
		}
	}
	return s, nil
}

func (s *Schema) checkAttrSpec(path string, a *AttrSpec) error {
	if a == nil {
		return fmt.Errorf("schema: %s: attribute has no definition", path)
	}
	switch a.Type {
	case TypeString, TypeInt, TypeNumber, TypeBool:
	case TypeEnum:
		if len(a.Values) == 0 {
			return fmt.Errorf("schema: %s: enum needs values", path)
		}
	case TypeList:
		switch a.Items {
		case TypeString, TypeInt, TypeNumber, TypeBool:
		default:
			return fmt.Errorf("schema: %s: list needs items of string|int|number|bool", path)
		}
	default:
		return fmt.Errorf("schema: %s: unknown type %q", path, a.Type)
	}
	if a.Required && a.HasDefault {
		return fmt.Errorf("schema: %s: required attributes cannot have a default", path)
	}
	if !a.Required && !a.HasDefault {
		return fmt.Errorf("schema: %s: attribute must be required or declare a default", path)
	}
	if a.HasDefault && a.Default == nil && !a.Nullable {
		return fmt.Errorf("schema: %s: default null requires nullable", path)
	}
	if a.Format != "" {
		re, ok := s.formats[a.Format]
		if !ok {
			return fmt.Errorf("schema: %s: unknown format %q", path, a.Format)
		}
		a.format = re
	}
	if a.HasDefault && a.Default != nil {
		if err := a.checkValue(a.Default, s.Limits.MaxAttrStringBytes); err != nil {
			return fmt.Errorf("schema: %s: default is not a valid value: %w", path, err)
		}
	}
	return nil
}

// scanDefaultKeys records which attribute objects carry an explicit "default"
// key, as dotted paths ("nodes.<node>.<attr>", "marks.<mark>.<attr>",
// "attrSets.<set>.<attr>").
func scanDefaultKeys(data []byte) (map[string]bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("schema: parse: %w", err)
	}
	out := map[string]bool{}
	scan := func(section string, attrsOf func(json.RawMessage) (map[string]json.RawMessage, error)) error {
		var entries map[string]json.RawMessage
		if raw, ok := top[section]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				return fmt.Errorf("schema: parse %s: %w", section, err)
			}
		}
		for name, raw := range entries {
			attrs, err := attrsOf(raw)
			if err != nil {
				return fmt.Errorf("schema: parse %s.%s: %w", section, name, err)
			}
			for attrName, attrRaw := range attrs {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(attrRaw, &fields); err != nil {
					return fmt.Errorf("schema: parse %s.%s.%s: %w", section, name, attrName, err)
				}
				if _, has := fields["default"]; has {
					out[section+"."+name+"."+attrName] = true
				}
			}
		}
		return nil
	}
	nested := func(raw json.RawMessage) (map[string]json.RawMessage, error) {
		var obj struct {
			Attrs map[string]json.RawMessage `json:"attrs"`
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, nil
		}
		return obj.Attrs, json.Unmarshal(raw, &obj)
	}
	direct := func(raw json.RawMessage) (map[string]json.RawMessage, error) {
		var attrs map[string]json.RawMessage
		return attrs, json.Unmarshal(raw, &attrs)
	}
	if err := scan("nodes", nested); err != nil {
		return nil, err
	}
	if err := scan("marks", nested); err != nil {
		return nil, err
	}
	if err := scan("attrSets", direct); err != nil {
		return nil, err
	}
	return out, nil
}

// NodeNames returns every node type name, sorted.
func (s *Schema) NodeNames() []string { return append([]string(nil), s.nodeOrder...) }

// MarkNames returns every mark type name, sorted.
func (s *Schema) MarkNames() []string { return append([]string(nil), s.markOrder...) }

// Group returns the node names belonging to a group, sorted, or nil.
func (s *Schema) Group(name string) []string { return append([]string(nil), s.groups[name]...) }

// AttrNames returns a node's attribute names, sorted.
func (n *NodeSpec) AttrNames() []string { return append([]string(nil), n.attrOrder...) }

// IsLeaf reports whether the node can never have children.
func (n *NodeSpec) IsLeaf() bool { return n.Content == "" }

// AllowsMark reports whether text directly inside this node may carry the mark.
func (n *NodeSpec) AllowsMark(mark string) bool {
	if n.markAllow == nil {
		return true
	}
	return n.markAllow[mark]
}

// AttrNames returns a mark's attribute names, sorted.
func (m *MarkSpec) AttrNames() []string { return append([]string(nil), m.attrOrder...) }

func sortedKeys(m map[string]*AttrSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
