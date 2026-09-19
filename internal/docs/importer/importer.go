// Package importer works out what a bundle of Markdown files should become.
//
// It is the mirror of internal/docs/export and deliberately has the same
// shape: pure functions over paths and bytes, with no database, no storage
// and no permissions. Deciding "this folder is a page, that file is its
// child, this link points at that page" is fiddly and full of cases that are
// easy to get wrong and easy to test, so it is kept where it can be tested.
//
// The bundle this reads is the one internal/docs/export writes — a tree of
// .md files where a page with children is a file beside a folder of the same
// name — but it is not limited to that. A folder of notes from anywhere,
// with no matching file, imports as a folder page with the notes underneath.
package importer

import (
	"bytes"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

// Limits on one bundle. Past these it is not a document import any more.
const (
	// MaxEntries bounds the files in a bundle.
	MaxEntries = 5000
	// MaxFileBytes bounds a single Markdown file.
	MaxFileBytes = 8 << 20
	// MaxTotalBytes bounds the uncompressed bundle, which is what stops a
	// small zip from expanding into a very large import.
	MaxTotalBytes = 512 << 20
)

// Entry is one file in a bundle.
type Entry struct {
	// Path is the entry's path inside the bundle, with forward slashes.
	Path string
	Size int64
}

// Node is one page to create.
type Node struct {
	// Path is the Markdown file this page comes from, or "" for a folder
	// that has no file of its own.
	Path string
	// Dir is the folder whose children belong under this page.
	Dir string
	// Title is the name to give the page before its content is read; the
	// importer replaces it with a title found inside the file.
	Title    string
	Children []*Node
}

// Plan is a whole bundle, decided.
type Plan struct {
	// Roots are the pages to create under the import target.
	Roots []*Node
	// Assets are the non-Markdown files, which become attachments.
	Assets []string
	// Skipped are entries left out, with the reason, for the job's report.
	Skipped []string
}

// IsMarkdown reports whether a bundle entry is a document rather than an asset.
func IsMarkdown(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// BuildPlan decides what a bundle becomes.
//
// The rule in one sentence: every Markdown file is a page, every folder that
// contains one is a page too, and a folder whose name matches a Markdown file
// beside it is that file's children rather than a page of its own.
//
// That last clause is what makes a round trip work. An export writes
// Handbook.md next to Handbook/Leave.md; without the clause the import would
// produce a "Handbook" page and a separate empty "Handbook" folder page, and
// the tree would double every time somebody exported and imported it.
func BuildPlan(entries []Entry) *Plan {
	plan := &Plan{}

	// Normalise and filter first, so everything below deals with clean paths.
	files := make([]string, 0, len(entries))
	seen := map[string]bool{}
	var total int64
	for _, entry := range entries {
		clean, ok := cleanPath(entry.Path)
		if !ok {
			plan.Skipped = append(plan.Skipped, entry.Path+": the path is not inside the bundle")
			continue
		}
		if clean == "" || seen[clean] {
			continue
		}
		if ignored(clean) {
			continue
		}
		if IsMarkdown(clean) && entry.Size > MaxFileBytes {
			plan.Skipped = append(plan.Skipped, clean+": the file is too large")
			continue
		}
		total += entry.Size
		if total > MaxTotalBytes || len(files)+len(plan.Assets) >= MaxEntries {
			plan.Skipped = append(plan.Skipped, clean+": the bundle is too large")
			continue
		}
		seen[clean] = true
		if IsMarkdown(clean) {
			files = append(files, clean)
		} else {
			plan.Assets = append(plan.Assets, clean)
		}
	}
	sort.Strings(files)
	sort.Strings(plan.Assets)

	// A folder is "claimed" when a Markdown file beside it has its name: that
	// file is the folder's page, and the folder holds its children.
	claimed := map[string]*Node{}
	byDir := map[string][]*Node{}
	for _, file := range files {
		node := &Node{Path: file, Title: TitleFromPath(file)}
		dir := trimExt(file)
		if containsFilesUnder(files, plan.Assets, dir) {
			node.Dir = dir
			claimed[dir] = node
		}
		byDir[path.Dir(file)] = append(byDir[path.Dir(file)], node)
	}

	// Every folder that holds something and was not claimed becomes a page of
	// its own, so nothing is imported without a parent that says where it
	// came from.
	for _, dir := range foldersOf(files, plan.Assets) {
		if claimed[dir] != nil {
			continue
		}
		node := &Node{Dir: dir, Title: TitleFromPath(dir)}
		claimed[dir] = node
		byDir[path.Dir(dir)] = append(byDir[path.Dir(dir)], node)
	}

	// Assemble: a node's children are whatever sits in its folder.
	for dir, node := range claimed {
		node.Children = append(node.Children, byDir[dir]...)
		delete(byDir, dir)
	}
	// byDir["."] is the top level; anything still left belongs to a folder
	// that produced no node, which cannot happen, but is carried up rather
	// than dropped.
	for _, nodes := range byDir {
		plan.Roots = append(plan.Roots, nodes...)
	}
	sortTree(plan.Roots)
	return plan
}

// cleanPath normalises a bundle path and refuses one that escapes the bundle.
//
// A zip entry can say "../../etc/passwd" or "/etc/passwd"; nothing here ever
// writes to a filesystem, but a path that climbs out of the bundle would
// still let one entry masquerade as another when links are resolved.
func cleanPath(p string) (string, bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	if strings.HasPrefix(p, "/") {
		return "", false
	}
	cleaned := path.Clean(p)
	if cleaned == "." {
		return "", true
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

// ignored skips the debris archivers and operating systems leave behind,
// which would otherwise import as pages called ".DS_Store".
func ignored(p string) bool {
	base := path.Base(p)
	if strings.HasPrefix(base, ".") {
		return true
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "__MACOSX" || strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func trimExt(p string) string {
	ext := path.Ext(p)
	if ext == "" {
		return p
	}
	return p[:len(p)-len(ext)]
}

// containsFilesUnder reports whether anything in the bundle lives under dir.
func containsFilesUnder(files, assets []string, dir string) bool {
	prefix := dir + "/"
	for _, list := range [][]string{files, assets} {
		for _, p := range list {
			if strings.HasPrefix(p, prefix) {
				return true
			}
		}
	}
	return false
}

// foldersOf lists every folder that holds an entry, deepest last, so that a
// nested folder is created after the one containing it.
func foldersOf(files, assets []string) []string {
	set := map[string]bool{}
	for _, list := range [][]string{files, assets} {
		for _, p := range list {
			for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
				set[dir] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for dir := range set {
		out = append(out, dir)
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/")
		if di != dj {
			return di < dj
		}
		return out[i] < out[j]
	})
	return out
}

func sortTree(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Title != nodes[j].Title {
			return nodes[i].Title < nodes[j].Title
		}
		return nodes[i].Path < nodes[j].Path
	})
	for _, node := range nodes {
		sortTree(node.Children)
	}
}

// Walk visits the tree parents first, which is the order pages must be
// created in: a child needs its parent's id.
func (p *Plan) Walk(visit func(node *Node, parent *Node)) {
	var walk func(nodes []*Node, parent *Node)
	walk = func(nodes []*Node, parent *Node) {
		for _, node := range nodes {
			visit(node, parent)
			walk(node.Children, node)
		}
	}
	walk(p.Roots, nil)
}

// Count is how many pages a plan will create.
func (p *Plan) Count() int {
	n := 0
	p.Walk(func(*Node, *Node) { n++ })
	return n
}

// ---- titles ---------------------------------------------------------------

// TitleFromPath is the fallback title: the file name without its extension,
// with the separators an exporter put there turned back into spaces.
func TitleFromPath(p string) string {
	name := trimExt(path.Base(p))
	name = strings.ReplaceAll(name, "_", " ")
	// Hyphens become spaces only when the name looks like it was made from a
	// title: "Getting-Started" was, "utf-8-notes" probably was not, but the
	// cost of being wrong either way is a title somebody retypes once.
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "Untitled"
	}
	return name
}

// Title picks a page's title from its content, falling back to its path.
//
// Order: the front matter's title, then a first-line H1, then the file name.
// A document that opens with its own title as a heading would otherwise show
// that title twice — once as the page name and once at the top of the body —
// so the heading is consumed along with it.
func Title(filePath string, body []byte) (title string, rest []byte) {
	body, meta := StripFrontMatter(body)
	if t := strings.TrimSpace(meta["title"]); t != "" {
		return clampTitle(t), body
	}
	if heading, remainder, ok := leadingH1(body); ok {
		return clampTitle(heading), remainder
	}
	return TitleFromPath(filePath), body
}

// MaxTitleRunes matches what the page service accepts.
const MaxTitleRunes = 500

func clampTitle(t string) string {
	t = strings.Join(strings.Fields(t), " ")
	t = strings.Trim(t, `"'`)
	if utf8.RuneCountInString(t) > MaxTitleRunes {
		runes := []rune(t)
		t = strings.TrimSpace(string(runes[:MaxTitleRunes]))
	}
	if t == "" {
		return "Untitled"
	}
	return t
}

// leadingH1 returns a level-one ATX heading at the very top of a document.
func leadingH1(body []byte) (heading string, rest []byte, ok bool) {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("# ")) {
		return "", body, false
	}
	line, remainder, _ := bytes.Cut(trimmed, []byte("\n"))
	heading = strings.TrimSpace(strings.TrimPrefix(string(line), "# "))
	if heading == "" {
		return "", body, false
	}
	return heading, bytes.TrimLeft(remainder, "\r\n"), true
}

// StripFrontMatter removes a leading YAML front-matter block and returns the
// simple `key: value` pairs in it.
//
// Deliberately not a YAML parser. Front matter in exported Markdown is a flat
// list of scalars, and the only key this needs is the title; pulling in a
// YAML parser to read one string would mean deciding what to do about
// anchors, multi-document files and arbitrary nesting, none of which a page
// title can be.
func StripFrontMatter(body []byte) ([]byte, map[string]string) {
	meta := map[string]string{}
	trimmed := bytes.TrimLeft(body, "\r\n")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return body, meta
	}
	_, after, found := bytes.Cut(trimmed, []byte("\n"))
	if !found {
		return body, meta
	}
	block, rest, closed := cutFence(after)
	if !closed {
		// An unclosed fence is not front matter; it is a document that starts
		// with a horizontal rule, and eating the rest of it would be a
		// spectacular way to lose somebody's page.
		return body, meta
	}
	for _, line := range strings.Split(string(block), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		meta[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return bytes.TrimLeft(rest, "\r\n"), meta
}

// cutFence splits at the closing --- line of a front-matter block.
func cutFence(body []byte) (block, rest []byte, closed bool) {
	offset := 0
	for offset <= len(body) {
		line, remainder, found := bytes.Cut(body[offset:], []byte("\n"))
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "---" || trimmed == "..." {
			return body[:offset], remainder, true
		}
		if !found {
			return nil, body, false
		}
		offset += len(line) + 1
	}
	return nil, body, false
}

// ---- links ----------------------------------------------------------------

// ResolveTarget turns a link destination inside a bundle into the bundle path
// it points at, or "" when it points outside.
//
// This is the inverse of export.RelativeLink, and the pair is what makes a
// round trip keep its cross-references: a page exported with a link to
// ../b/Other.md imports with a page link back to that page.
func ResolveTarget(fromPath, dest string) string {
	if dest == "" {
		return ""
	}
	// An absolute URL, a fragment or a mail link is not a bundle path.
	if strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "/") ||
		strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") {
		return ""
	}
	// A fragment or query on a relative link is addressing part of the target
	// page, which is still that page.
	if i := strings.IndexAny(dest, "#?"); i >= 0 {
		dest = dest[:i]
	}
	if dest == "" {
		return ""
	}
	if decoded, err := decodePercent(dest); err == nil {
		dest = decoded
	}
	joined := path.Join(path.Dir(fromPath), dest)
	cleaned, ok := cleanPath(joined)
	if !ok {
		return ""
	}
	return cleaned
}

// decodePercent undoes the percent-encoding a Markdown writer applies to a
// path with spaces or non-ASCII characters in it.
func decodePercent(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		hi, ok1 := unhex(s[i+1])
		lo, ok2 := unhex(s[i+2])
		if !ok1 || !ok2 {
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(hi<<4 | lo)
		i += 2
	}
	return b.String(), nil
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
