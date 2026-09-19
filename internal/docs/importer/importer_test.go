package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entries(paths ...string) []Entry {
	out := make([]Entry, 0, len(paths))
	for _, p := range paths {
		out = append(out, Entry{Path: p, Size: 10})
	}
	return out
}

// titlesOf flattens a plan into "parent/child" paths of titles, which is a
// readable way to assert a tree.
func titlesOf(plan *Plan) []string {
	var out []string
	var walk func(nodes []*Node, prefix string)
	walk = func(nodes []*Node, prefix string) {
		for _, node := range nodes {
			full := prefix + node.Title
			out = append(out, full)
			walk(node.Children, full+"/")
		}
	}
	walk(plan.Roots, "")
	return out
}

func TestAFlatBundleBecomesFlatPages(t *testing.T) {
	plan := BuildPlan(entries("Onboarding.md", "Expenses.md"))
	assert.Equal(t, []string{"Expenses", "Onboarding"}, titlesOf(plan))
	assert.Empty(t, plan.Skipped)
}

// The round trip: the export writes Handbook.md beside Handbook/Leave.md, and
// importing that must not produce two "Handbook" pages.
func TestAFileBesideItsOwnFolderIsOnePage(t *testing.T) {
	plan := BuildPlan(entries("Handbook.md", "Handbook/Leave.md"))
	require.Equal(t, []string{"Handbook", "Handbook/Leave"}, titlesOf(plan))
	require.Len(t, plan.Roots, 1)
	assert.Equal(t, "Handbook.md", plan.Roots[0].Path, "the folder page keeps its own content")
}

// A folder of notes from somewhere that is not this product still needs a
// parent, or the import scatters its files across the target.
func TestAFolderWithNoFileOfItsOwnBecomesAPage(t *testing.T) {
	plan := BuildPlan(entries("Notes/One.md", "Notes/Two.md"))
	assert.Equal(t, []string{"Notes", "Notes/One", "Notes/Two"}, titlesOf(plan))
	require.Len(t, plan.Roots, 1)
	assert.Empty(t, plan.Roots[0].Path, "a folder page has no file, so it starts empty")
}

func TestNestingGoesAsDeepAsTheBundle(t *testing.T) {
	plan := BuildPlan(entries("a/b/c/Deep.md"))
	assert.Equal(t, []string{"a", "a/b", "a/b/c", "a/b/c/Deep"}, titlesOf(plan))
}

func TestParentsAlwaysComeBeforeTheirChildren(t *testing.T) {
	plan := BuildPlan(entries("Handbook.md", "Handbook/Leave.md", "Handbook/Leave/Sick.md"))
	seen := map[*Node]bool{}
	plan.Walk(func(node, parent *Node) {
		if parent != nil {
			assert.True(t, seen[parent], "%s was visited before its parent", node.Title)
		}
		seen[node] = true
	})
	assert.Equal(t, 3, plan.Count())
}

func TestNonMarkdownFilesAreAssetsNotPages(t *testing.T) {
	plan := BuildPlan(entries("Notes.md", "images/diagram.png", "data.csv"))
	assert.Equal(t, []string{"Notes", "images"}, titlesOf(plan))
	assert.Equal(t, []string{"data.csv", "images/diagram.png"}, plan.Assets)
}

// A zip entry can say anything; a path that climbs out of the bundle would let
// one entry masquerade as another.
func TestAPathThatEscapesTheBundleIsRefused(t *testing.T) {
	plan := BuildPlan(entries("../../etc/passwd", "/etc/shadow", "ok.md"))
	assert.Equal(t, []string{"ok"}, titlesOf(plan))
	assert.Len(t, plan.Skipped, 2)
}

func TestArchiverDebrisIsIgnoredSilently(t *testing.T) {
	plan := BuildPlan(entries("Notes.md", ".DS_Store", "__MACOSX/._Notes.md", ".git/config"))
	assert.Equal(t, []string{"Notes"}, titlesOf(plan))
	assert.Empty(t, plan.Skipped, "debris is not worth reporting to the person importing")
	assert.Empty(t, plan.Assets)
}

func TestAnOversizedFileIsSkippedWithAReason(t *testing.T) {
	plan := BuildPlan([]Entry{
		{Path: "Huge.md", Size: MaxFileBytes + 1},
		{Path: "Small.md", Size: 10},
	})
	assert.Equal(t, []string{"Small"}, titlesOf(plan))
	require.Len(t, plan.Skipped, 1)
	assert.Contains(t, plan.Skipped[0], "Huge.md")
}

// ---- titles ---------------------------------------------------------------

func TestTheFileNameIsTheFallbackTitle(t *testing.T) {
	title, body := Title("notes/Getting-Started.md", []byte("hello"))
	assert.Equal(t, "Getting Started", title)
	assert.Equal(t, "hello", string(body))
}

func TestAChineseFileNameIsKept(t *testing.T) {
	title, _ := Title("存储配额说明.md", []byte("正文"))
	assert.Equal(t, "存储配额说明", title)
}

func TestFrontMatterSuppliesTheTitle(t *testing.T) {
	src := "---\ntitle: Expenses Policy\nauthor: someone\n---\n\nThe body.\n"
	title, body := Title("x.md", []byte(src))
	assert.Equal(t, "Expenses Policy", title)
	assert.Equal(t, "The body.\n", string(body))
}

// A document that opens with its own title as a heading would otherwise show
// it twice: once as the page name, once at the top of the body.
func TestALeadingHeadingBecomesTheTitleAndIsConsumed(t *testing.T) {
	title, body := Title("x.md", []byte("# Onboarding\n\nWelcome aboard.\n"))
	assert.Equal(t, "Onboarding", title)
	assert.Equal(t, "Welcome aboard.\n", string(body))
}

// Only the first heading, and only when it is first: a document that starts
// with a paragraph keeps everything it has.
func TestAHeadingLaterInTheDocumentIsLeftAlone(t *testing.T) {
	src := "Intro paragraph.\n\n# Section\n"
	title, body := Title("Notes.md", []byte(src))
	assert.Equal(t, "Notes", title)
	assert.Equal(t, src, string(body))
}

func TestAVeryLongTitleIsClamped(t *testing.T) {
	title, _ := Title("x.md", []byte("# "+strings.Repeat("字", 900)+"\n"))
	assert.LessOrEqual(t, len([]rune(title)), MaxTitleRunes)
	assert.NotEmpty(t, title)
}

// The fence has to close. A document that opens with a horizontal rule is not
// front matter, and eating the rest of it would lose somebody's page.
func TestAnUnclosedFenceIsNotFrontMatter(t *testing.T) {
	src := "---\nthis is just a rule and some text\n"
	body, meta := StripFrontMatter([]byte(src))
	assert.Equal(t, src, string(body))
	assert.Empty(t, meta)
}

func TestFrontMatterIsReadAsFlatPairs(t *testing.T) {
	_, meta := StripFrontMatter([]byte("---\nTitle: A\ntags: x, y\nbroken\n---\nbody"))
	assert.Equal(t, "A", meta["title"], "keys are matched case-insensitively")
	assert.Equal(t, "x, y", meta["tags"])
}

// ---- links ----------------------------------------------------------------

func TestALinkWithinTheBundleResolves(t *testing.T) {
	assert.Equal(t, "a/Other.md", ResolveTarget("a/Notes.md", "Other.md"))
	assert.Equal(t, "Top.md", ResolveTarget("a/Notes.md", "../Top.md"))
	assert.Equal(t, "b/Other.md", ResolveTarget("a/Notes.md", "../b/Other.md"))
	assert.Equal(t, "a/sub/Child.md", ResolveTarget("a/Notes.md", "sub/Child.md"))
}

// The pair with export.RelativeLink is what keeps cross-references through a
// round trip.
func TestAFragmentOrQueryStillNamesThePage(t *testing.T) {
	assert.Equal(t, "a/Other.md", ResolveTarget("a/Notes.md", "Other.md#heading"))
	assert.Equal(t, "a/Other.md", ResolveTarget("a/Notes.md", "Other.md?v=2"))
}

func TestAnEncodedPathIsDecoded(t *testing.T) {
	assert.Equal(t, "a/存储.md", ResolveTarget("a/Notes.md", "%E5%AD%98%E5%82%A8.md"))
	assert.Equal(t, "a/A B.md", ResolveTarget("a/Notes.md", "A%20B.md"))
}

func TestALinkOutOfTheBundleIsNotAPageLink(t *testing.T) {
	for _, dest := range []string{
		"https://example.test/x", "//example.test/x", "/absolute", "#anchor",
		"mailto:someone@example.test", "", "../../outside.md",
	} {
		assert.Equal(t, "", ResolveTarget("a/Notes.md", dest), "dest %q", dest)
	}
}
