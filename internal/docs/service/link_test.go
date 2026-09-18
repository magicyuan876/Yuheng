package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

// linkBody builds a document that links to every given page.
func linkBody(text string, targets ...string) json.RawMessage {
	content := []any{map[string]any{"type": "text", "text": text}}
	for _, id := range targets {
		content = append(content, map[string]any{"type": "pageLink", "attrs": map[string]any{"pageId": id}})
	}
	raw, err := json.Marshal(map[string]any{
		"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": content}},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

// save writes a body through the shared persist path, the way an editor does.
func (p *pageEnv) save(t *testing.T, pageID string, version int64, body json.RawMessage) {
	t.Helper()
	_, err := p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: pageID, BaseVersion: version,
		YDoc: []byte("state"), Content: body, EditorIDs: []string{"alice"},
	})
	require.NoError(t, err)
}

func titles(rows []*PageRefView) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Title
	}
	return out
}

// Acceptance (T2.2): a page shows what points at it, and that list is rebuilt
// from the document rather than tracked edit by edit.
func TestBacklinksFollowWhatTheDocumentSays(t *testing.T) {
	p := newPageEnv(t)
	target := p.create(t, p.alice, nil, "Target")
	source := p.create(t, p.alice, nil, "Source")

	none, err := p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, target.ID))
	require.NoError(t, err)
	require.Empty(t, none)

	p.save(t, source.ID, 0, linkBody("see ", target.ID))
	rows, err := p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, target.ID))
	require.NoError(t, err)
	require.Equal(t, []string{"Source"}, titles(rows))

	// Removing the link is done by deleting the text around it, which produces
	// no event of its own — only a new document without it.
	p.save(t, source.ID, 1, docBody("nothing here any more"))
	rows, err = p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, target.ID))
	require.NoError(t, err)
	require.Empty(t, rows, "a link removed from the body must disappear from the list")
}

// Acceptance (T2.2): deleting the source page takes its backlink with it.
func TestBacklinksDisappearWithTheirSourcePage(t *testing.T) {
	p := newPageEnv(t)
	target := p.create(t, p.alice, nil, "Target")
	source := p.create(t, p.alice, nil, "Source")
	p.save(t, source.ID, 0, linkBody("see ", target.ID))

	// In the trash it is already gone from the list: the page is not readable,
	// so neither is the fact that it linked here.
	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, source.ID))
	require.NoError(t, err)
	rows, err := p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, target.ID))
	require.NoError(t, err)
	require.Empty(t, rows)

	// And purging removes the row outright, through the foreign key.
	_, err = p.svc.Pages.Purge(ctx(), p.alice, p.space, source.ID)
	require.NoError(t, err)
	stored, err := p.repos.Links.Outgoing(ctx(), 1, source.ID)
	require.NoError(t, err)
	require.Empty(t, stored)
}

// Acceptance (T2.2): renaming a page updates every link to it, because no
// document ever stored its title.
func TestAPageLinkCarriesOnlyAnIDSoRenamingIsEnough(t *testing.T) {
	p := newPageEnv(t)
	target := p.create(t, p.alice, nil, "Old name")
	source := p.create(t, p.alice, nil, "Source")
	p.save(t, source.ID, 0, linkBody("see ", target.ID))

	before, err := p.svc.Pages.ResolveTitles(ctx(), p.alice, []string{target.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"Old name"}, titles(before))

	_, err = p.svc.Pages.Update(ctx(), p.alice, p.decision(t, p.alice, target.ID),
		UpdatePageInput{Title: strp("New name")})
	require.NoError(t, err)

	after, err := p.svc.Pages.ResolveTitles(ctx(), p.alice, []string{target.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"New name"}, titles(after))
	require.True(t, after[0].Resolved)

	// The source document itself was never touched by the rename.
	stored, err := p.repos.Pages.Get(ctx(), 1, source.ID)
	require.NoError(t, err)
	require.NotContains(t, string(stored.Content), "Old name")
	require.NotContains(t, string(stored.Content), "New name")
}

func TestResolvingTitlesHidesWhatTheReaderCannotSee(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Secret plans")

	mine, err := p.svc.Pages.ResolveTitles(ctx(), p.alice, []string{page.ID})
	require.NoError(t, err)
	require.True(t, mine[0].Resolved)

	// Somebody outside the space gets the same answer as for a page that does
	// not exist, so a link cannot be used to probe for one.
	outsider, err := p.svc.Pages.ResolveTitles(ctx(), p.viewer, []string{page.ID})
	require.NoError(t, err)
	require.False(t, outsider[0].Resolved)
	require.Empty(t, outsider[0].Title)

	missing, err := p.svc.Pages.ResolveTitles(ctx(), p.alice, []string{"no-such-page"})
	require.NoError(t, err)
	require.False(t, missing[0].Resolved)
	require.Empty(t, missing[0].Title)

	// The batch is bounded.
	tooMany := make([]string, MaxTitleLookup+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("p%d", i)
	}
	_, err = p.svc.Pages.ResolveTitles(ctx(), p.alice, tooMany)
	require.Equal(t, 400, httpCode(t, err))
}

// Acceptance (T2.2): a page the caller may not read never appears in the
// suggestion menu.
func TestPageSuggestionsOnlyOfferWhatTheCallerCanRead(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "Handbook onboarding")
	p.create(t, p.alice, nil, "Handbook payroll")

	// A second, private space the others are not members of.
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Board"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{SpaceID: other.Space.ID, Title: "Handbook secrets"})
	require.NoError(t, err)

	mine, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{Query: "handbook"})
	require.NoError(t, err)
	require.Len(t, mine, 3, "the owner sees all three")

	theirs, err := p.svc.Pages.SuggestPages(ctx(), p.bob, SuggestPagesInput{Query: "handbook"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"Handbook onboarding", "Handbook payroll"}, titles(theirs))

	// Somebody in no space at all gets nothing rather than an error.
	none, err := p.svc.Pages.SuggestPages(ctx(), p.viewer, SuggestPagesInput{Query: "handbook"})
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestPageSuggestionsMatchTitlesAndStayBounded(t *testing.T) {
	p := newPageEnv(t)
	for i := 0; i < MaxSuggestions+5; i++ {
		p.create(t, p.alice, nil, fmt.Sprintf("Report %02d", i))
	}
	p.create(t, p.alice, nil, "Unrelated")

	hits, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{Query: "report"})
	require.NoError(t, err)
	require.Len(t, hits, MaxSuggestions, "a menu under the cursor is not a search result page")
	for _, h := range hits {
		require.Contains(t, h.Title, "Report")
	}

	// Case does not matter, and a wildcard is a character rather than a way to
	// list the whole space.
	upper, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{Query: "REPORT 01"})
	require.NoError(t, err)
	require.Equal(t, []string{"Report 01"}, titles(upper))

	wildcard, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{Query: "%"})
	require.NoError(t, err)
	require.Empty(t, wildcard, "a LIKE wildcard must be matched literally")

	// An empty query offers the most recently touched pages instead of nothing.
	recent, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{})
	require.NoError(t, err)
	require.NotEmpty(t, recent)
}

func TestPageSuggestionsCanBeNarrowedToOneSpace(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "Shared name")
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Board"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{SpaceID: other.Space.ID, Title: "Shared name"})
	require.NoError(t, err)

	all, err := p.svc.Pages.SuggestPages(ctx(), p.alice, SuggestPagesInput{Query: "shared"})
	require.NoError(t, err)
	require.Len(t, all, 2)

	one, err := p.svc.Pages.SuggestPages(ctx(), p.alice,
		SuggestPagesInput{Query: "shared", SpaceID: p.space.ID})
	require.NoError(t, err)
	require.Len(t, one, 1)
	require.Equal(t, p.space.ID, one[0].SpaceID)

	// A space the caller cannot read is not found rather than empty, so the
	// answer does not confirm it exists.
	_, err = p.svc.Pages.SuggestPages(ctx(), p.bob,
		SuggestPagesInput{Query: "shared", SpaceID: other.Space.ID})
	require.Equal(t, 404, httpCode(t, err))
}

// Acceptance (T2.2): only somebody who can already read the page may be
// mentioned on it, so a mention never notifies a person about something they
// cannot open.
func TestMentionCandidatesAreTheReadersOfThatPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	rows, err := p.svc.Pages.SuggestMentions(ctx(), p.alice, p.decision(t, p.alice, page.ID), "", 0)
	require.NoError(t, err)
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Username
	}
	require.Subset(t, names, []string{"alice", "bob", "carol"}, "the space's members are candidates")
	require.NotContains(t, names, "viewer", "somebody outside the space is not")

	// The query filters by name or email.
	filtered, err := p.svc.Pages.SuggestMentions(ctx(), p.alice, p.decision(t, p.alice, page.ID), "bo", 0)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "bob", filtered[0].Username)
}

func TestLinksAreRecordedForTransclusionsToo(t *testing.T) {
	p := newPageEnv(t)
	target := p.create(t, p.alice, nil, "Target")
	source := p.create(t, p.alice, nil, "Source")

	body, err := json.Marshal(map[string]any{
		"type": "doc", "content": []any{map[string]any{
			"type": "transclusion",
			"attrs": map[string]any{
				"sourcePageId": target.ID, "sourceBlockId": "block1",
			},
		}},
	})
	require.NoError(t, err)
	p.save(t, source.ID, 0, body)

	stored, err := p.repos.Links.Outgoing(ctx(), 1, source.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, model.LinkTransclusion, stored[0].Kind)

	// It still shows up once in the backlinks, as one page rather than one
	// entry per kind of reference.
	p.save(t, source.ID, 1, mixedBody(t, target.ID))
	rows, err := p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, target.ID))
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// mixedBody refers to one page both as a link and as a transclusion.
func mixedBody(t *testing.T, targetID string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type": "doc", "content": []any{
			map[string]any{"type": "paragraph", "content": []any{
				map[string]any{"type": "pageLink", "attrs": map[string]any{"pageId": targetID}},
			}},
			map[string]any{"type": "transclusion", "attrs": map[string]any{
				"sourcePageId": targetID, "sourceBlockId": "block1",
			}},
		},
	})
	require.NoError(t, err)
	return raw
}

// A page linking to itself is not a backlink anybody wants to look at.
func TestAPageDoesNotBacklinkToItself(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	p.save(t, page.ID, 0, linkBody("see ", page.ID))

	rows, err := p.svc.Pages.Backlinks(ctx(), p.alice, p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	require.Empty(t, rows)
}

// A link to a page that no longer exists must not fail the save: the body is
// already what the author wrote, and the reference simply does not resolve.
func TestSavingSurvivesALinkToAMissingPage(t *testing.T) {
	p := newPageEnv(t)
	source := p.create(t, p.alice, nil, "Source")

	p.save(t, source.ID, 0, linkBody("see ", "no-such-page"))

	stored, err := p.repos.Links.Outgoing(ctx(), 1, source.ID)
	require.NoError(t, err)
	require.Empty(t, stored, "a reference to nothing records nothing")

	body, err := p.svc.Pages.Content(ctx(), p.decision(t, p.alice, source.ID), false)
	require.NoError(t, err)
	require.Contains(t, string(body.Content), "no-such-page", "but the document keeps what was written")
}
