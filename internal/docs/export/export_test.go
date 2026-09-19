package export

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDefaultFormatIsMarkdown(t *testing.T) {
	for _, raw := range []string{"", "  ", "markdown", "MD", " md "} {
		got, err := ParseFormat(raw)
		require.NoError(t, err, "input %q", raw)
		assert.Equal(t, FormatMarkdown, got)
	}
}

func TestHTMLIsAccepted(t *testing.T) {
	for _, raw := range []string{"html", "HTML", "htm"} {
		got, err := ParseFormat(raw)
		require.NoError(t, err)
		assert.Equal(t, FormatHTML, got)
	}
}

func TestAFormatThisDoesNotProduceIsRefused(t *testing.T) {
	for _, raw := range []string{"pdf", "docx", "epub"} {
		_, err := ParseFormat(raw)
		require.Error(t, err, "input %q", raw)
	}
}

func TestEachFormatKnowsItsExtensionAndType(t *testing.T) {
	assert.Equal(t, ".md", FormatMarkdown.Extension())
	assert.Equal(t, ".html", FormatHTML.Extension())
	assert.Contains(t, FormatMarkdown.MediaType(), "markdown")
	assert.Contains(t, FormatHTML.MediaType(), "html")
}

// ---- file names -----------------------------------------------------------------

func TestAnOrdinaryTitleBecomesAnOrdinaryName(t *testing.T) {
	assert.Equal(t, "Onboarding", FileName("Onboarding", "page"))
	assert.Equal(t, "Getting-Started", FileName("Getting Started", "page"))
}

// Transliterating a Chinese title would make the file unfindable by the
// person who wrote it.
func TestAChineseTitleStaysChinese(t *testing.T) {
	assert.Equal(t, "存储配额说明", FileName("存储配额说明", "page"))
	assert.Equal(t, "配额-说明", FileName("配额 说明", "page"))
}

// The characters a filesystem or a zip reader objects to.
func TestCharactersAFilesystemRefusesAreReplaced(t *testing.T) {
	for _, title := range []string{
		"a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b",
	} {
		name := FileName(title, "page")
		assert.NotContains(t, name, title[1:2], "title %q", title)
		assert.Equal(t, "a-b", name, "title %q", title)
	}
}

func TestRunsOfPunctuationCollapse(t *testing.T) {
	assert.Equal(t, "a-b", FileName("a   ///   b", "page"))
	assert.Equal(t, "a-b", FileName("a - - - b", "page"))
}

func TestLeadingAndTrailingSeparatorsAreTrimmed(t *testing.T) {
	assert.Equal(t, "notes", FileName("  ...notes...  ", "page"))
	assert.Equal(t, "notes", FileName("///notes///", "page"))
}

// A title of nothing but emoji still has to become a file.
func TestATitleWithNothingUsableFallsBack(t *testing.T) {
	assert.Equal(t, "page-abc", FileName("🎉🎊", "page-abc"))
	assert.Equal(t, "page-abc", FileName("", "page-abc"))
	assert.Equal(t, "page-abc", FileName("   ", "page-abc"))
}

// An exported tree nests, and a path is the sum of its segments.
func TestALongTitleIsShortened(t *testing.T) {
	name := FileName(strings.Repeat("字", 300), "page")
	assert.LessOrEqual(t, len([]rune(name)), MaxFileNameRunes)
	assert.NotEmpty(t, name)
}

// Windows refuses these whatever the extension.
func TestWindowsDeviceNamesAreAvoided(t *testing.T) {
	for _, reserved := range []string{"CON", "con", "PRN", "nul", "COM1", "lpt9"} {
		name := FileName(reserved, "page")
		assert.NotEqual(t, strings.ToLower(reserved), strings.ToLower(name), "name %q", reserved)
		assert.True(t, strings.HasPrefix(name, "_"), "name %q became %q", reserved, name)
	}
}

// ---- allocating unique paths ----------------------------------------------------

func TestTwoSiblingsWithTheSameTitleDoNotOverwriteEachOther(t *testing.T) {
	n := NewNamer()
	first := n.Allocate("", "Notes", "page-1", ".md")
	second := n.Allocate("", "Notes", "page-2", ".md")

	assert.Equal(t, "Notes.md", first)
	assert.Equal(t, "Notes-2.md", second)
	assert.NotEqual(t, first, second)
}

func TestAThirdCollisionKeepsCounting(t *testing.T) {
	n := NewNamer()
	n.Allocate("", "Notes", "p1", ".md")
	n.Allocate("", "Notes", "p2", ".md")
	assert.Equal(t, "Notes-3.md", n.Allocate("", "Notes", "p3", ".md"))
}

// macOS and Windows treat these as the same file, and an export that loses a
// page on one operating system is worse than one that renames on all.
func TestCollisionsAreCaseInsensitive(t *testing.T) {
	n := NewNamer()
	n.Allocate("", "Notes", "p1", ".md")
	assert.Equal(t, "notes-2.md", n.Allocate("", "notes", "p2", ".md"))
}

func TestTheSameNameInDifferentFoldersIsFine(t *testing.T) {
	n := NewNamer()
	assert.Equal(t, "a/Notes.md", n.Allocate("a", "Notes", "p1", ".md"))
	assert.Equal(t, "b/Notes.md", n.Allocate("b", "Notes", "p2", ".md"))
}

// A page and the folder holding its children must not collide.
func TestAReservedPathIsNotHandedOut(t *testing.T) {
	n := NewNamer()
	n.Reserve("Notes.md")
	assert.Equal(t, "Notes-2.md", n.Allocate("", "Notes", "p1", ".md"))
}

// ---- links between exported files -----------------------------------------------

func TestALinkWithinTheSameFolderIsJustTheName(t *testing.T) {
	assert.Equal(t, "Other.md", RelativeLink("Notes.md", "Other.md"))
	assert.Equal(t, "Other.md", RelativeLink("a/Notes.md", "a/Other.md"))
}

func TestALinkIntoASubfolderDescends(t *testing.T) {
	assert.Equal(t, "a/Child.md", RelativeLink("Parent.md", "a/Child.md"))
}

func TestALinkOutOfAFolderClimbs(t *testing.T) {
	assert.Equal(t, "../Top.md", RelativeLink("a/Notes.md", "Top.md"))
	assert.Equal(t, "../../Top.md", RelativeLink("a/b/Notes.md", "Top.md"))
}

func TestALinkAcrossFoldersClimbsAndDescends(t *testing.T) {
	assert.Equal(t, "../b/Other.md", RelativeLink("a/Notes.md", "b/Other.md"))
}

func TestALinkToItselfIsNotEmpty(t *testing.T) {
	// Degenerate but must not produce "" — a link with no href is a broken
	// one, and this is cheap to be safe about.
	assert.NotEmpty(t, RelativeLink("a/Notes.md", "a/Notes.md"))
}
