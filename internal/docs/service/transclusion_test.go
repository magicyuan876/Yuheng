package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// mustPage reads a page back, for its current version.
func (p *pageEnv) mustPage(t *testing.T, pageID string) *model.Page {
	t.Helper()
	page, err := p.repos.Pages.Get(ctx(), 1, pageID)
	require.NoError(t, err)
	return page
}

// blockBody builds a document of paragraphs, each carrying a block id.
func blockBody(blocks map[string]string, order ...string) json.RawMessage {
	content := make([]any, 0, len(order))
	for _, id := range order {
		content = append(content, map[string]any{
			"type":    "paragraph",
			"attrs":   map[string]any{"id": id},
			"content": []any{map[string]any{"type": "text", "text": blocks[id]}},
		})
	}
	raw, err := json.Marshal(map[string]any{"type": "doc", "content": content})
	if err != nil {
		panic(err)
	}
	return raw
}

// refBody builds a document holding one reference to a block of another page.
func refBody(text, sourcePageID, sourceBlockID string) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": text}},
			},
			map[string]any{
				"type": "transclusion",
				"attrs": map[string]any{
					"sourcePageId": sourcePageID, "sourceBlockId": sourceBlockID,
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

// resolve asks for one reference as the given actor.
func (p *pageEnv) resolveRef(t *testing.T, actor *acl.Identity, pageID, blockID string) *TransclusionView {
	t.Helper()
	rows, err := p.svc.Pages.ResolveTransclusions(ctx(), actor,
		[]repository.BlockRef{{PageID: pageID, BlockID: blockID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	return rows[0]
}

// Acceptance (T2.5): a block shown on another page is the source block as it
// is now, and correcting the source corrects every page that shows it.
func TestAReferenceShowsTheSourceBlockAsItIsNow(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Definitions")
	reader := p.create(t, p.alice, nil, "Guide")

	// The source page has the block, but nobody references it yet.
	p.save(t, source.Page.ID, source.Page.YDocVersion, blockBody(
		map[string]string{"defblk": "A widget is a thing."}, "defblk"))

	// Referencing it records the reference; there is nothing to show yet,
	// because the source page has not been saved since.
	p.save(t, reader.Page.ID, reader.Page.YDocVersion, refBody("See:", source.Page.ID, "defblk"))
	pending := p.resolveRef(t, p.alice, source.Page.ID, "defblk")
	assert.Equal(t, TransclusionPending, pending.State)

	// The next save of the source page fills it in.
	current := p.mustPage(t, source.Page.ID)
	p.save(t, source.Page.ID, current.YDocVersion, blockBody(
		map[string]string{"defblk": "A widget is a thing."}, "defblk"))

	got := p.resolveRef(t, p.alice, source.Page.ID, "defblk")
	require.Equal(t, TransclusionOK, got.State)
	assert.Contains(t, string(got.Content), "A widget is a thing.")
	assert.Equal(t, "Definitions", got.Title, "a reference can be attributed")

	// Correcting the source corrects the reference, with no edit to the page
	// that shows it.
	current = p.mustPage(t, source.Page.ID)
	p.save(t, source.Page.ID, current.YDocVersion, blockBody(
		map[string]string{"defblk": "A widget is a small thing."}, "defblk"))

	got = p.resolveRef(t, p.alice, source.Page.ID, "defblk")
	require.Equal(t, TransclusionOK, got.State)
	assert.Contains(t, string(got.Content), "A widget is a small thing.")
}

// Deleting the block is different from never having had one: the reference
// must say so rather than keep showing text that is no longer on the page.
func TestDeletingTheSourceBlockIsReported(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Definitions")
	reader := p.create(t, p.alice, nil, "Guide")

	p.save(t, source.Page.ID, source.Page.YDocVersion, blockBody(
		map[string]string{"defblk": "original", "otherblk": "kept"}, "defblk", "otherblk"))
	p.save(t, reader.Page.ID, reader.Page.YDocVersion, refBody("See:", source.Page.ID, "defblk"))

	current := p.mustPage(t, source.Page.ID)
	p.save(t, source.Page.ID, current.YDocVersion, blockBody(
		map[string]string{"defblk": "original", "otherblk": "kept"}, "defblk", "otherblk"))
	require.Equal(t, TransclusionOK, p.resolveRef(t, p.alice, source.Page.ID, "defblk").State)

	// The block is removed from the source document.
	current = p.mustPage(t, source.Page.ID)
	p.save(t, source.Page.ID, current.YDocVersion, blockBody(
		map[string]string{"otherblk": "kept"}, "otherblk"))

	got := p.resolveRef(t, p.alice, source.Page.ID, "defblk")
	assert.Equal(t, TransclusionMissing, got.State)
	assert.Empty(t, got.Content, "and nothing stale is left behind")
}

// A reference cannot be used to find out whether a page exists, so one state
// covers both "deleted" and "you may not read it".
func TestAReferenceToAnUnreadablePageResolvesAsMissing(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Secrets")
	reader := p.create(t, p.alice, nil, "Guide")

	p.save(t, source.Page.ID, source.Page.YDocVersion, blockBody(
		map[string]string{"defblk": "confidential"}, "defblk"))
	p.save(t, reader.Page.ID, reader.Page.YDocVersion, refBody("See:", source.Page.ID, "defblk"))
	current := p.mustPage(t, source.Page.ID)
	p.save(t, source.Page.ID, current.YDocVersion, blockBody(
		map[string]string{"defblk": "confidential"}, "defblk"))

	// Alice can see it; carol, once the page is restricted, cannot.
	require.Equal(t, TransclusionOK, p.resolveRef(t, p.alice, source.Page.ID, "defblk").State)
	p.restrict(t, source.Page.ID, "alice")

	got := p.resolveRef(t, p.carol, source.Page.ID, "defblk")
	assert.Equal(t, TransclusionMissing, got.State)
	assert.Empty(t, got.Content, "the text must not leak")
	assert.Empty(t, got.Title, "nor the title")

	// Exactly the state a reference to a page that never existed gets.
	absent := p.resolveRef(t, p.carol, "no-such-page", "defblk")
	assert.Equal(t, got.State, absent.State)
}

// Nobody references most pages, and those must not pay for this feature.
func TestAPageNobodyReferencesStoresNoSnapshots(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Ordinary")

	p.save(t, source.Page.ID, source.Page.YDocVersion, blockBody(
		map[string]string{"blockaa": "one", "blockbb": "two"}, "blockaa", "blockbb"))

	tracked, err := p.repos.Blocks.TrackedFor(ctx(), 1, source.Page.ID)
	require.NoError(t, err)
	assert.Empty(t, tracked, "a page nobody quotes stores nothing at all")
}

// The rule that makes a cycle impossible rather than merely bounded.
func TestASnapshotNeverContainsAReferenceOfItsOwn(t *testing.T) {
	p := newPageEnv(t)
	first := p.create(t, p.alice, nil, "First")
	second := p.create(t, p.alice, nil, "Second")

	// Second's block is itself a quote of something on First.
	p.save(t, first.Page.ID, first.Page.YDocVersion, blockBody(
		map[string]string{"originblk": "the original words"}, "originblk"))
	nested, err := json.Marshal(map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type":  "blockquote",
			"attrs": map[string]any{"id": "wrapperblk"},
			"content": []any{
				map[string]any{
					"type":    "paragraph",
					"content": []any{map[string]any{"type": "text", "text": "context"}},
				},
				map[string]any{"type": "transclusion", "attrs": map[string]any{
					"sourcePageId": first.Page.ID, "sourceBlockId": "originblk",
				}},
			},
		}},
	})
	require.NoError(t, err)
	p.save(t, second.Page.ID, second.Page.YDocVersion, nested)

	// A third page quotes Second's wrapper.
	third := p.create(t, p.alice, nil, "Third")
	p.save(t, third.Page.ID, third.Page.YDocVersion, refBody("See:", second.Page.ID, "wrapperblk"))
	current := p.mustPage(t, second.Page.ID)
	p.save(t, second.Page.ID, current.YDocVersion, nested)

	got := p.resolveRef(t, p.alice, second.Page.ID, "wrapperblk")
	require.Equal(t, TransclusionOK, got.State)
	assert.Contains(t, string(got.Content), "context")
	assert.NotContains(t, string(got.Content), "transclusion",
		"the chain is cut at one level, so no pair of pages can resolve forever")
}

func TestResolvingNothingAsksTheDatabaseNothing(t *testing.T) {
	p := newPageEnv(t)

	rows, err := p.svc.Pages.ResolveTransclusions(ctx(), p.alice, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// A reference missing half its address is not a reference.
	rows, err = p.svc.Pages.ResolveTransclusions(ctx(), p.alice, []repository.BlockRef{
		{PageID: "", BlockID: "x"}, {PageID: "y", BlockID: ""},
	})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestResolvingIsBounded(t *testing.T) {
	p := newPageEnv(t)
	refs := make([]repository.BlockRef, 0, MaxTransclusionLookup+1)
	for i := 0; i <= MaxTransclusionLookup; i++ {
		refs = append(refs, repository.BlockRef{PageID: "p", BlockID: blockID(i)})
	}

	_, err := p.svc.Pages.ResolveTransclusions(ctx(), p.alice, refs)
	require.Error(t, err)
}

// The same block quoted many times is one row and one answer.
func TestTheSameReferenceTwiceIsResolvedOnce(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Definitions")

	rows, err := p.svc.Pages.ResolveTransclusions(ctx(), p.alice, []repository.BlockRef{
		{PageID: source.Page.ID, BlockID: "defblk"},
		{PageID: source.Page.ID, BlockID: "defblk"},
	})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// blockID builds a distinct id matching docs-schema's blockId format, which
// requires at least six characters.
func blockID(i int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	return "block" + string(alphabet[i%len(alphabet)]) + string(alphabet[(i/len(alphabet))%len(alphabet)])
}
