package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// label adds one to the fixture's space.
func (p *pageEnv) label(t *testing.T, name, color string) *LabelView {
	t.Helper()
	view, err := p.svc.Pages.CreateLabel(ctx(), p.alice, p.space, model.RoleAdmin,
		CreateLabelInput{Name: name, Color: color})
	require.NoError(t, err)
	return view
}

func (p *pageEnv) labels(t *testing.T, role model.SpaceRole) []*LabelView {
	t.Helper()
	rows, err := p.svc.Pages.Labels(ctx(), p.alice, p.space, role)
	require.NoError(t, err)
	return rows
}

func TestALabelIsCreatedAndListed(t *testing.T) {
	p := newPageEnv(t)
	p.label(t, "draft", "orange")

	rows := p.labels(t, model.RoleReader)
	require.Len(t, rows, 1)
	assert.Equal(t, "draft", rows[0].Name)
	assert.Equal(t, "orange", rows[0].Color)
	assert.Equal(t, int64(0), rows[0].PageCount)
}

func TestALabelNameIsTidiedAndBounded(t *testing.T) {
	p := newPageEnv(t)

	view := p.label(t, "  needs   review  ", "")
	assert.Equal(t, "needs review", view.Name, "whitespace is collapsed")
	assert.Equal(t, LabelColors[0], view.Color, "and a colour is chosen when none was given")

	_, err := p.svc.Pages.CreateLabel(ctx(), p.alice, p.space, model.RoleAdmin,
		CreateLabelInput{Name: "   "})
	require.Error(t, err, "a label needs a name")

	long := make([]rune, MaxLabelNameRunes+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err = p.svc.Pages.CreateLabel(ctx(), p.alice, p.space, model.RoleAdmin,
		CreateLabelInput{Name: string(long)})
	require.Error(t, err)
}

// An arbitrary colour would be somebody else's contrast problem, on every
// list the label appears in.
func TestOnlyThisProductsColoursAreAccepted(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.CreateLabel(ctx(), p.alice, p.space, model.RoleAdmin,
		CreateLabelInput{Name: "x", Color: "#ff00ff"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "colour")
}

func TestTwoLabelsCannotShareAName(t *testing.T) {
	p := newPageEnv(t)
	p.label(t, "draft", "")

	_, err := p.svc.Pages.CreateLabel(ctx(), p.alice, p.space, model.RoleAdmin,
		CreateLabelInput{Name: "draft"})
	require.Error(t, err)
}

// A writer files their own work; asking an admin for permission to do it
// would make the feature useless.
func TestAWriterMayMakeLabelsButOnlyAnAdminMayDeleteThem(t *testing.T) {
	p := newPageEnv(t)

	view, err := p.svc.Pages.CreateLabel(ctx(), p.bob, p.space, model.RoleWriter,
		CreateLabelInput{Name: "in progress"})
	require.NoError(t, err)

	// Deleting takes it off everybody's pages, which is a different act.
	err = p.svc.Pages.DeleteLabel(ctx(), p.bob, p.space, model.RoleWriter, view.ID)
	require.Error(t, err)

	require.NoError(t, p.svc.Pages.DeleteLabel(ctx(), p.alice, p.space, model.RoleAdmin, view.ID))
	assert.Empty(t, p.labels(t, model.RoleReader))
}

func TestAReaderMaySeeLabelsButNotChangeThem(t *testing.T) {
	p := newPageEnv(t)
	p.label(t, "draft", "")

	assert.Len(t, p.labels(t, model.RoleReader), 1)

	_, err := p.svc.Pages.CreateLabel(ctx(), p.carol, p.space, model.RoleReader,
		CreateLabelInput{Name: "mine"})
	require.Error(t, err)
}

func TestLabellingAPageAndReadingItBack(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	draft := p.label(t, "draft", "orange")
	review := p.label(t, "review", "blue")

	set, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		[]string{draft.ID, review.ID})
	require.NoError(t, err)
	assert.Len(t, set, 2)

	back, err := p.svc.Pages.PageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{draft.ID, review.ID}, []string{back[0].ID, back[1].ID})
}

// The set is replaced rather than added to, which is what "these are its
// labels now" means.
func TestSettingLabelsReplacesWhatWasThere(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	first := p.label(t, "first", "")
	second := p.label(t, "second", "")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, d, []string{first.ID})
	require.NoError(t, err)

	set, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, d, []string{second.ID})
	require.NoError(t, err)
	require.Len(t, set, 1)
	assert.Equal(t, second.ID, set[0].ID)

	cleared, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, d, nil)
	require.NoError(t, err)
	assert.Empty(t, cleared)
}

// A page in one space filed under another's vocabulary would also make that
// space's counts mean something it cannot see.
func TestAPageCannotCarryAnotherSpacesLabel(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	foreign, err := p.svc.Pages.CreateLabel(ctx(), p.alice, other.Space, model.RoleAdmin,
		CreateLabelInput{Name: "theirs"})
	require.NoError(t, err)

	_, err = p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		[]string{foreign.ID})
	require.Error(t, err)
}

func TestNamingAnotherSpacesLabelDoesNotReachIt(t *testing.T) {
	p := newPageEnv(t)
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	foreign, err := p.svc.Pages.CreateLabel(ctx(), p.alice, other.Space, model.RoleAdmin,
		CreateLabelInput{Name: "theirs"})
	require.NoError(t, err)

	_, err = p.svc.Pages.UpdateLabel(ctx(), p.alice, p.space, model.RoleAdmin, foreign.ID,
		CreateLabelInput{Name: "renamed"})
	require.Error(t, err)

	err = p.svc.Pages.DeleteLabel(ctx(), p.alice, p.space, model.RoleAdmin, foreign.ID)
	require.Error(t, err)
}

// The count should match what clicking the label shows, and clicking it does
// not show the trash.
func TestALabelsCountIsOfLivePages(t *testing.T) {
	p := newPageEnv(t)
	kept := p.create(t, p.alice, nil, "Kept")
	binned := p.create(t, p.alice, nil, "Binned")
	draft := p.label(t, "draft", "")

	for _, page := range []*PageView{kept, binned} {
		_, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
			[]string{draft.ID})
		require.NoError(t, err)
	}
	require.EqualValues(t, 2, p.labels(t, model.RoleReader)[0].PageCount)

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, binned.Page.ID))
	require.NoError(t, err)
	assert.EqualValues(t, 1, p.labels(t, model.RoleReader)[0].PageCount)
}

// Filtering by two labels means pages carrying both, which is what somebody
// narrowing a list expects.
func TestFilteringBySeveralLabelsMeansAllOfThem(t *testing.T) {
	p := newPageEnv(t)
	both := p.create(t, p.alice, nil, "Both")
	one := p.create(t, p.alice, nil, "One")
	draft := p.label(t, "draft", "")
	review := p.label(t, "review", "")

	_, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, both.Page.ID),
		[]string{draft.ID, review.ID})
	require.NoError(t, err)
	_, err = p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, one.Page.ID),
		[]string{draft.ID})
	require.NoError(t, err)

	byOne, err := p.svc.Pages.PagesWithLabels(ctx(), p.alice, p.space, model.RoleReader,
		[]string{draft.ID}, 0)
	require.NoError(t, err)
	assert.Len(t, byOne, 2)

	byBoth, err := p.svc.Pages.PagesWithLabels(ctx(), p.alice, p.space, model.RoleReader,
		[]string{draft.ID, review.ID}, 0)
	require.NoError(t, err)
	require.Len(t, byBoth, 1)
	assert.Equal(t, both.Page.ID, byBoth[0].ID)
}

// A title is information: "there is a page called Q3 Redundancies" is the
// leak, not its contents.
func TestALabelFilterDoesNotRevealPagesTheReaderCannotOpen(t *testing.T) {
	p := newPageEnv(t)
	secret := p.create(t, p.alice, nil, "Q3 Redundancies")
	open := p.create(t, p.alice, nil, "Open")
	draft := p.label(t, "draft", "")

	for _, page := range []*PageView{secret, open} {
		_, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
			[]string{draft.ID})
		require.NoError(t, err)
	}
	p.restrict(t, secret.Page.ID, "alice")

	asCarol, err := p.svc.Pages.PagesWithLabels(ctx(), p.carol, p.space, model.RoleReader,
		[]string{draft.ID}, 0)
	require.NoError(t, err)
	require.Len(t, asCarol, 1)
	assert.Equal(t, open.Page.ID, asCarol[0].ID)
}

func TestAPageMayNotCarryUnboundedLabels(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	ids := make([]string, 0, MaxLabelsPerPage+1)
	for i := 0; i <= MaxLabelsPerPage; i++ {
		ids = append(ids, "label-that-does-not-exist")
	}
	_, err := p.svc.Pages.SetPageLabels(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), ids)
	require.Error(t, err)
}
