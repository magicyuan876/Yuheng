// Package export decides what a page looks like once it leaves the system.
//
// An exported file is read somewhere this product has no control over: an
// email attachment, a shared drive, somebody's laptop. Three consequences
// shape everything here.
//
// ---- a page link cannot travel as a link
//
// A link to another page is an address inside this installation. In an
// exported file it either dangles, or — when the export is a whole space —
// should point at the other exported file. Neither is the raw URL, so the
// renderer is given a resolver that knows which pages are in this export and
// rewrites accordingly: a relative path when the target is included, the
// page's title as plain text when it is not. The second case is the
// important one: a link to a page the reader cannot open must not become a
// URL that tells them it exists.
//
// ---- a transcluded block must be flattened
//
// A block shown by reference is, to the reader, part of the page. In an
// export there is nothing to resolve it against later, so it is rendered
// inline at export time or it is lost. The permission check is the caller's:
// this package renders what it is handed.
//
// ---- the file name is not the title
//
// Titles contain slashes, colons, emoji and four hundred characters. A file
// name has to survive Windows, macOS and a zip reader written in 2003.
package export

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// Format is what an export produces.
type Format string

const (
	// FormatMarkdown is the portable one, and the default.
	FormatMarkdown Format = "markdown"
	// FormatHTML keeps the rendering, for somebody who wants to look at it
	// rather than edit it.
	FormatHTML Format = "html"
)

// ParseFormat reads a requested format, defaulting to Markdown.
func ParseFormat(raw string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "markdown", "md":
		return FormatMarkdown, nil
	case "html", "htm":
		return FormatHTML, nil
	default:
		return "", fmt.Errorf("%q is not a format this exports to", raw)
	}
}

// Extension is the file suffix for a format.
func (f Format) Extension() string {
	if f == FormatHTML {
		return ".html"
	}
	return ".md"
}

// MediaType is what the file is served as.
func (f Format) MediaType() string {
	if f == FormatHTML {
		return "text/html; charset=utf-8"
	}
	return "text/markdown; charset=utf-8"
}

// MaxFileNameRunes bounds a generated file name.
//
// 80 rather than the 255 most filesystems allow: an exported tree nests, and
// a path is the sum of its segments. Windows still enforces 260 characters
// on a full path in its default configuration, and a zip somebody cannot
// extract is worse than one with shortened names.
const MaxFileNameRunes = 80

// FileName turns a page title into something a filesystem will accept.
//
// Keeps letters and digits of any script — a Chinese title stays Chinese,
// because transliterating it would make the file unfindable by the person
// who wrote it — and replaces everything a filesystem objects to.
func FileName(title string, fallback string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.' || unicode.IsSpace(r):
			// Runs of punctuation and space collapse into one separator.
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			// Slashes, colons, quotes, emoji, control characters.
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	name := strings.Trim(b.String(), "-.")
	if runes := []rune(name); len(runes) > MaxFileNameRunes {
		name = strings.Trim(string(runes[:MaxFileNameRunes]), "-.")
	}
	if name == "" {
		// A title of nothing but emoji, or an untitled page.
		name = fallback
	}
	// Windows refuses these whatever the extension.
	if reservedNames[strings.ToLower(name)] {
		name = "_" + name
	}
	return name
}

// reservedNames are the device names Windows will not allow a file to have.
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// Namer allocates unique paths inside one export.
//
// Two sibling pages called "Notes" must not overwrite each other, and a page
// and a folder of the same name must not collide either. Collisions are
// resolved by a numeric suffix rather than by including the page id, because
// "Notes-2.md" is something a person can make sense of and
// "Notes-a3f9c1.md" is not.
type Namer struct {
	taken map[string]bool
}

// NewNamer builds an empty allocator.
func NewNamer() *Namer { return &Namer{taken: map[string]bool{}} }

// Allocate returns a unique path under dir for a page.
func (n *Namer) Allocate(dir, title, fallback, extension string) string {
	base := FileName(title, fallback)
	for attempt := 1; ; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		full := path.Join(dir, candidate+extension)
		// Compared case-insensitively: macOS and Windows would treat
		// "Notes.md" and "notes.md" as the same file, and an export that
		// loses a page on one operating system and not another is worse
		// than one that renames on all of them.
		key := strings.ToLower(full)
		if n.taken[key] {
			continue
		}
		n.taken[key] = true
		return full
	}
}

// Reserve marks a path as used, for directories.
func (n *Namer) Reserve(p string) {
	n.taken[strings.ToLower(p)] = true
}

// RelativeLink is the href from one exported file to another.
//
// Both paths are export-relative ("Handbook/Onboarding.md"), and the result
// is what a Markdown or HTML reader in the same folder tree will follow.
func RelativeLink(from, to string) string {
	fromDir := path.Dir(from)
	if fromDir == "." {
		return to
	}
	rel, err := relativeTo(fromDir, to)
	if err != nil {
		return to
	}
	return rel
}

// relativeTo computes a relative path without touching the filesystem.
func relativeTo(baseDir, target string) (string, error) {
	baseParts := splitPath(baseDir)
	targetParts := splitPath(target)

	common := 0
	for common < len(baseParts) && common < len(targetParts) &&
		baseParts[common] == targetParts[common] {
		common++
	}
	var out []string
	for range baseParts[common:] {
		out = append(out, "..")
	}
	out = append(out, targetParts[common:]...)
	if len(out) == 0 {
		return "", fmt.Errorf("export: no relative path from %q to %q", baseDir, target)
	}
	return path.Join(out...), nil
}

func splitPath(p string) []string {
	p = strings.Trim(path.Clean(p), "/")
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}
