package schema

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Matcher decides whether a sequence of child node types satisfies a
// ProseMirror content expression such as "paragraph block*" or
// "(tableCell | tableHeader)+".
//
// The expression grammar is the one ProseMirror uses:
//
//	expr    := seq ('|' seq)*
//	seq     := piece+
//	piece   := atom ('+' | '*' | '?' | '{' n '}' | '{' n ',' '}' | '{' n ',' m '}')?
//	atom    := name | '(' expr ')'
//
// where name is a node type or a group name (which expands to every node in
// the group). Expressions compile to an NFA once at load time; matching is a
// subset simulation in O(children × states), with no backtracking, so hostile
// documents cannot make validation expensive.
type Matcher struct {
	expr   string
	states []nfaState
	start  int
}

type nfaState struct {
	// edges keyed by node type name; "" is the epsilon edge list.
	edges  map[string][]int
	accept bool
}

// MatchResult describes where a child sequence stopped satisfying the
// expression.
type MatchResult struct {
	OK bool
	// FailedAt is the index of the first child that could not be consumed, or
	// len(children) when the sequence ended before the expression was
	// satisfied.
	FailedAt int
	// Expected lists node types that would have been acceptable at FailedAt,
	// sorted, for error messages.
	Expected []string
}

// Match runs the matcher over child node type names.
func (m *Matcher) Match(children []string) MatchResult {
	current := m.closure(map[int]bool{m.start: true})
	for i, child := range children {
		next := map[int]bool{}
		for st := range current {
			for _, to := range m.states[st].edges[child] {
				next[to] = true
			}
		}
		if len(next) == 0 {
			return MatchResult{FailedAt: i, Expected: m.expected(current)}
		}
		current = m.closure(next)
	}
	for st := range current {
		if m.states[st].accept {
			return MatchResult{OK: true, FailedAt: len(children)}
		}
	}
	return MatchResult{FailedAt: len(children), Expected: m.expected(current)}
}

// String returns the source expression.
func (m *Matcher) String() string { return m.expr }

func (m *Matcher) closure(set map[int]bool) map[int]bool {
	stack := make([]int, 0, len(set))
	for st := range set {
		stack = append(stack, st)
	}
	for len(stack) > 0 {
		st := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, to := range m.states[st].edges[""] {
			if !set[to] {
				set[to] = true
				stack = append(stack, to)
			}
		}
	}
	return set
}

func (m *Matcher) expected(set map[int]bool) []string {
	seen := map[string]bool{}
	for st := range set {
		for name := range m.states[st].edges {
			if name != "" {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sortStrings(out)
	return out
}

// ---- parsing --------------------------------------------------------------

type exprKind int

const (
	kindName exprKind = iota
	kindSeq
	kindChoice
	kindRepeat
)

type exprNode struct {
	kind     exprKind
	names    []string // kindName: the node types this atom accepts (group expanded)
	children []*exprNode
	min, max int // kindRepeat; max < 0 means unbounded
}

type parser struct {
	tokens []string
	pos    int
	schema *Schema
}

func compileContent(expr string, s *Schema) (*Matcher, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, schema: s}
	ast, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, fmt.Errorf("unexpected %q", p.tokens[p.pos])
	}
	b := &nfaBuilder{}
	start := b.newState()
	end := b.build(ast, start)
	b.states[end].accept = true
	return &Matcher{expr: expr, states: b.states, start: start}, nil
}

func tokenize(expr string) ([]string, error) {
	var tokens []string
	i := 0
	for i < len(expr) {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			i++
		case strings.ContainsRune("()|+*?", rune(c)):
			tokens = append(tokens, string(c))
			i++
		case c == '{':
			j := strings.IndexByte(expr[i:], '}')
			if j < 0 {
				return nil, fmt.Errorf("unterminated '{' at %d", i)
			}
			tokens = append(tokens, expr[i:i+j+1])
			i += j + 1
		case unicode.IsLetter(rune(c)) || c == '_':
			j := i
			for j < len(expr) && (unicode.IsLetter(rune(expr[j])) || unicode.IsDigit(rune(expr[j])) || expr[j] == '_') {
				j++
			}
			tokens = append(tokens, expr[i:j])
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q at %d", c, i)
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty expression")
	}
	return tokens, nil
}

func (p *parser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *parser) parseExpr() (*exprNode, error) {
	first, err := p.parseSeq()
	if err != nil {
		return nil, err
	}
	if p.peek() != "|" {
		return first, nil
	}
	choice := &exprNode{kind: kindChoice, children: []*exprNode{first}}
	for p.peek() == "|" {
		p.pos++
		alt, err := p.parseSeq()
		if err != nil {
			return nil, err
		}
		choice.children = append(choice.children, alt)
	}
	return choice, nil
}

func (p *parser) parseSeq() (*exprNode, error) {
	seq := &exprNode{kind: kindSeq}
	for {
		tok := p.peek()
		if tok == "" || tok == "|" || tok == ")" {
			break
		}
		piece, err := p.parsePiece()
		if err != nil {
			return nil, err
		}
		seq.children = append(seq.children, piece)
	}
	if len(seq.children) == 0 {
		return nil, fmt.Errorf("empty sequence")
	}
	if len(seq.children) == 1 {
		return seq.children[0], nil
	}
	return seq, nil
}

func (p *parser) parsePiece() (*exprNode, error) {
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	tok := p.peek()
	switch {
	case tok == "+":
		p.pos++
		return &exprNode{kind: kindRepeat, children: []*exprNode{atom}, min: 1, max: -1}, nil
	case tok == "*":
		p.pos++
		return &exprNode{kind: kindRepeat, children: []*exprNode{atom}, min: 0, max: -1}, nil
	case tok == "?":
		p.pos++
		return &exprNode{kind: kindRepeat, children: []*exprNode{atom}, min: 0, max: 1}, nil
	case strings.HasPrefix(tok, "{"):
		p.pos++
		lo, hi, err := parseRange(tok)
		if err != nil {
			return nil, err
		}
		return &exprNode{kind: kindRepeat, children: []*exprNode{atom}, min: lo, max: hi}, nil
	}
	return atom, nil
}

func parseRange(tok string) (int, int, error) {
	body := strings.TrimSuffix(strings.TrimPrefix(tok, "{"), "}")
	lo, hi := body, body
	if i := strings.IndexByte(body, ','); i >= 0 {
		lo, hi = strings.TrimSpace(body[:i]), strings.TrimSpace(body[i+1:])
	}
	minN, err := strconv.Atoi(lo)
	if err != nil || minN < 0 {
		return 0, 0, fmt.Errorf("bad repeat range %q", tok)
	}
	if hi == "" {
		return minN, -1, nil
	}
	maxN, err := strconv.Atoi(hi)
	if err != nil || maxN < minN {
		return 0, 0, fmt.Errorf("bad repeat range %q", tok)
	}
	if maxN > 10000 {
		return 0, 0, fmt.Errorf("repeat range %q too large", tok)
	}
	return minN, maxN, nil
}

func (p *parser) parseAtom() (*exprNode, error) {
	tok := p.peek()
	switch {
	case tok == "(":
		p.pos++
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing ')'")
		}
		p.pos++
		return inner, nil
	case tok == "":
		return nil, fmt.Errorf("unexpected end of expression")
	case unicode.IsLetter(rune(tok[0])) || tok[0] == '_':
		p.pos++
		if _, ok := p.schema.Nodes[tok]; ok {
			return &exprNode{kind: kindName, names: []string{tok}}, nil
		}
		if members := p.schema.groups[tok]; len(members) > 0 {
			return &exprNode{kind: kindName, names: members}, nil
		}
		return nil, fmt.Errorf("unknown node or group %q", tok)
	}
	return nil, fmt.Errorf("unexpected %q", tok)
}

// ---- Thompson construction -------------------------------------------------

type nfaBuilder struct {
	states []nfaState
}

func (b *nfaBuilder) newState() int {
	b.states = append(b.states, nfaState{edges: map[string][]int{}})
	return len(b.states) - 1
}

func (b *nfaBuilder) edge(from int, label string, to int) {
	b.states[from].edges[label] = append(b.states[from].edges[label], to)
}

// build wires the expression starting at `from` and returns its end state.
func (b *nfaBuilder) build(e *exprNode, from int) int {
	switch e.kind {
	case kindName:
		to := b.newState()
		for _, name := range e.names {
			b.edge(from, name, to)
		}
		return to
	case kindSeq:
		cur := from
		for _, child := range e.children {
			cur = b.build(child, cur)
		}
		return cur
	case kindChoice:
		end := b.newState()
		for _, child := range e.children {
			childEnd := b.build(child, from)
			b.edge(childEnd, "", end)
		}
		return end
	case kindRepeat:
		child := e.children[0]
		cur := from
		for i := 0; i < e.min; i++ {
			cur = b.build(child, cur)
		}
		if e.max < 0 {
			// loop: cur -> child -> back to cur; epsilon out
			loopEnd := b.build(child, cur)
			b.edge(loopEnd, "", cur)
			end := b.newState()
			b.edge(cur, "", end)
			return end
		}
		end := b.newState()
		b.edge(cur, "", end)
		for i := e.min; i < e.max; i++ {
			cur = b.build(child, cur)
			b.edge(cur, "", end)
		}
		return end
	}
	panic("schema: unknown expression node")
}

func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
