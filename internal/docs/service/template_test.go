package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
)

const helloDoc = `{"type":"doc","content":[{"type":"paragraph","content":[` +
	`{"type":"text","text":"Weekly notes"}]}]}`

func (p *pageEnv) template(t *testing.T, who *acl.Identity, in CreateTemplateInput) *TemplateView {
	t.Helper()
	view, err := p.svc.Pages.CreateTemplate(ctx(), who, in)
	require.NoError(t, err)
	return view
}

func TestATemplateIsSavedAndListed(t *testing.T) {
	p := newPageEnv(t)
	made := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "  Weekly   report ", Category: "Meetings",
		Content: json.RawMessage(helloDoc),
	})
	assert.Equal(t, "Weekly report", made.Name, "whitespace is collapsed")
	assert.False(t, made.Shared)

	rows, err := p.svc.Pages.Templates(ctx(), p.alice, p.space.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Weekly report", rows[0].Name)
	assert.Empty(t, rows[0].Content, "a listing does not carry every body")

	full, err := p.svc.Pages.Template(ctx(), p.alice, made.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, full.Content, "a single read does")
}

func TestATemplateNeedsAName(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.CreateTemplate(ctx(), p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "   ", Content: json.RawMessage(helloDoc),
	})
	require.Error(t, err)
}

// A shared template appears in every space, including ones its author
// cannot see, so making one is an administrator's act.
func TestASharedTemplateNeedsAWorkspaceAdministrator(t *testing.T) {
	p := newPageEnv(t)

	// bob is a space writer but not a tenant administrator.
	_, err := p.svc.Pages.CreateTemplate(ctx(), p.bob, CreateTemplateInput{
		Name: "Org standard", Content: json.RawMessage(helloDoc),
	})
	require.Error(t, err)

	// owner administers the tenant.
	owner := p.identity("owner")
	shared := p.template(t, owner, CreateTemplateInput{
		Name: "Org standard", Content: json.RawMessage(helloDoc),
	})
	assert.True(t, shared.Shared)
	assert.Nil(t, shared.SpaceID)
}

// A space template is one team's way of writing a thing; writing in that
// space is the right that makes one.
func TestAWriterMakesASpaceTemplateAndAReaderDoesNot(t *testing.T) {
	p := newPageEnv(t)

	_, err := p.svc.Pages.CreateTemplate(ctx(), p.bob, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Bob's", Content: json.RawMessage(helloDoc),
	})
	require.NoError(t, err)

	_, err = p.svc.Pages.CreateTemplate(ctx(), p.carol, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Carol's", Content: json.RawMessage(helloDoc),
	})
	require.Error(t, err)
}

// The set somebody creating a page can choose from is both scopes.
func TestListingASpaceReturnsItsOwnAndTheSharedOnes(t *testing.T) {
	p := newPageEnv(t)
	owner := p.identity("owner")
	p.template(t, owner, CreateTemplateInput{Name: "Org standard", Content: json.RawMessage(helloDoc)})
	p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Team standard", Content: json.RawMessage(helloDoc),
	})

	rows, err := p.svc.Pages.Templates(ctx(), p.alice, p.space.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	names := []string{rows[0].Name, rows[1].Name}
	assert.Contains(t, names, "Org standard")
	assert.Contains(t, names, "Team standard")
}

// Another space's templates are not somebody's to browse: their names and
// descriptions were written for an audience.
func TestAnotherSpacesTemplatesAreNotListed(t *testing.T) {
	p := newPageEnv(t)
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	p.template(t, p.alice, CreateTemplateInput{
		SpaceID: other.Space.ID, Name: "Theirs", Content: json.RawMessage(helloDoc),
	})

	rows, err := p.svc.Pages.Templates(ctx(), p.alice, p.space.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// And naming a space somebody cannot see is refused rather than answered.
	_, err = p.svc.Pages.Templates(ctx(), p.viewer, other.Space.ID)
	require.Error(t, err)
}

func TestAPageIsCreatedFromATemplate(t *testing.T) {
	p := newPageEnv(t)
	tpl := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Weekly", Content: json.RawMessage(helloDoc),
	})

	page, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: p.space.ID, Title: "This week", TemplateID: tpl.ID,
	})
	require.NoError(t, err)
	assert.Contains(t, page.Page.TextContent, "Weekly notes")
	require.NotNil(t, page.Page.TemplateID, "where it came from is recorded")
	assert.Equal(t, tpl.ID, *page.Page.TemplateID)
}

func TestATemplateAndABodyAreMutuallyExclusive(t *testing.T) {
	p := newPageEnv(t)
	tpl := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Weekly", Content: json.RawMessage(helloDoc),
	})

	_, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: p.space.ID, Title: "x", TemplateID: tpl.ID,
		Content: json.RawMessage(helloDoc),
	})
	require.Error(t, err)
}

// A space's own template belongs to that space; applying it elsewhere is
// somewhere its author never intended it.
func TestASpaceTemplateCannotBeUsedInAnotherSpace(t *testing.T) {
	p := newPageEnv(t)
	tpl := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Ours", Content: json.RawMessage(helloDoc),
	})
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)

	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: other.Space.ID, Title: "x", TemplateID: tpl.ID,
	})
	require.Error(t, err)
}

func TestASharedTemplateWorksInEverySpace(t *testing.T) {
	p := newPageEnv(t)
	owner := p.identity("owner")
	tpl := p.template(t, owner, CreateTemplateInput{
		Name: "Org standard", Content: json.RawMessage(helloDoc),
	})
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)

	for _, spaceID := range []string{p.space.ID, other.Space.ID} {
		_, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
			SpaceID: spaceID, Title: "From the standard", TemplateID: tpl.ID,
		})
		require.NoError(t, err)
	}
}

// Saving a page as a template is a read of that page.
func TestSavingAPageAsATemplateNeedsToBeAbleToReadIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Q3 Redundancies")
	p.restrict(t, page.Page.ID, "alice")

	_, err := p.svc.Pages.CreateTemplate(ctx(), p.carol, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Stolen", FromPageID: page.Page.ID,
	})
	require.Error(t, err)

	made, err := p.svc.Pages.CreateTemplate(ctx(), p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Mine", FromPageID: page.Page.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Mine", made.Name)
}

// The decision at the top of template.go, enforced: what is saved has
// already had its ties removed.
func TestSavingAPageStripsWhatCannotTravel(t *testing.T) {
	p := newPageEnv(t)
	target := p.create(t, p.alice, nil, "Target")
	source := p.create(t, p.alice, nil, "Source")

	withTies := `{"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"See "},` +
		`{"type":"pageLink","attrs":{"pageId":"` + target.Page.ID + `"}},` +
		`{"type":"text","text":" and ask "},` +
		`{"type":"mention","attrs":{"userId":"reviewer-01","label":"Reviewer"}}]}]}`
	_, err := p.svc.Pages.ReplaceContent(ctx(), p.alice,
		p.decision(t, p.alice, source.Page.ID),
		ReplaceInput{Content: json.RawMessage(withTies), Reason: "rest"})
	require.NoError(t, err)

	made, err := p.svc.Pages.CreateTemplate(ctx(), p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "From a page", FromPageID: source.Page.ID,
	})
	require.NoError(t, err)

	full, err := p.svc.Pages.Template(ctx(), p.alice, made.ID)
	require.NoError(t, err)
	body := string(full.Content)
	assert.NotContains(t, body, "pageLink", "a link to one page does not travel")
	assert.NotContains(t, body, target.Page.ID)
	assert.NotContains(t, body, "mention")
	assert.NotContains(t, body, "reviewer-01")
	assert.Contains(t, body, "Reviewer", "but the words do")
	assert.Contains(t, body, "See ")
}

func TestATemplateIsRenamedAndRecategorised(t *testing.T) {
	p := newPageEnv(t)
	made := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Draft", Content: json.RawMessage(helloDoc),
	})

	name, category := "Final", "Planning"
	updated, err := p.svc.Pages.UpdateTemplate(ctx(), p.alice, made.ID,
		UpdateTemplateInput{Name: &name, Category: &category})
	require.NoError(t, err)
	assert.Equal(t, "Final", updated.Name)
	assert.Equal(t, "Planning", updated.Category)
}

func TestAReaderMayNotChangeOrDeleteATemplate(t *testing.T) {
	p := newPageEnv(t)
	made := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Draft", Content: json.RawMessage(helloDoc),
	})

	name := "Hijacked"
	_, err := p.svc.Pages.UpdateTemplate(ctx(), p.carol, made.ID, UpdateTemplateInput{Name: &name})
	require.Error(t, err)
	require.Error(t, p.svc.Pages.DeleteTemplate(ctx(), p.carol, made.ID))
}

// Removing a template takes it away from everybody, so it is an admin's act.
func TestDeletingATemplateNeedsAnAdministrator(t *testing.T) {
	p := newPageEnv(t)
	made := p.template(t, p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Draft", Content: json.RawMessage(helloDoc),
	})

	require.Error(t, p.svc.Pages.DeleteTemplate(ctx(), p.bob, made.ID), "a writer may not")
	require.NoError(t, p.svc.Pages.DeleteTemplate(ctx(), p.alice, made.ID))

	rows, err := p.svc.Pages.Templates(ctx(), p.alice, p.space.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestASharedTemplateIsReadableByEverybodyAndChangedByAdminsOnly(t *testing.T) {
	p := newPageEnv(t)
	owner := p.identity("owner")
	made := p.template(t, owner, CreateTemplateInput{
		Name: "Org standard", Content: json.RawMessage(helloDoc),
	})

	full, err := p.svc.Pages.Template(ctx(), p.carol, made.ID)
	require.NoError(t, err)
	assert.Equal(t, "Org standard", full.Name)
	assert.False(t, full.CanEdit, "and she is told she may not change it")

	name := "Hijacked"
	_, err = p.svc.Pages.UpdateTemplate(ctx(), p.alice, made.ID, UpdateTemplateInput{Name: &name})
	require.Error(t, err, "a space admin is not a workspace admin")

	_, err = p.svc.Pages.UpdateTemplate(ctx(), owner, made.ID, UpdateTemplateInput{Name: &name})
	require.NoError(t, err)
}

func TestNamingATemplateThatIsNotThereIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.Template(ctx(), p.alice, "no-such-template")
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err))
}

func TestAnInvalidBodyIsRefused(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.CreateTemplate(ctx(), p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Bad", Content: json.RawMessage(`{"type":"nonsense"}`),
	})
	require.Error(t, err)
}

func TestABodyAndAPageAreMutuallyExclusive(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	_, err := p.svc.Pages.CreateTemplate(ctx(), p.alice, CreateTemplateInput{
		SpaceID: p.space.ID, Name: "Both", FromPageID: page.Page.ID,
		Content: json.RawMessage(helloDoc),
	})
	require.Error(t, err)
}
