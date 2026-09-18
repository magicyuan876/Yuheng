package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

func TestTheSpaceHomeGathersEverythingAtOnce(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.label(t, "draft", "")
	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), true))

	home, err := p.svc.Pages.Home(ctx(), p.alice, p.space, model.RoleAdmin)
	require.NoError(t, err)
	assert.Len(t, home.RecentlyEdited, 1)
	assert.Len(t, home.Labels, 1)
	require.Len(t, home.Favourites, 1)
	assert.Equal(t, page.Page.ID, home.Favourites[0].ID)
}

// A landing page is the first thing a new member sees; it has to render.
func TestAnEmptySpacesHomeIsEmptyListsRatherThanNulls(t *testing.T) {
	p := newPageEnv(t)
	home, err := p.svc.Pages.Home(ctx(), p.carol, p.space, model.RoleReader)
	require.NoError(t, err)
	assert.NotNil(t, home.RecentlyEdited)
	assert.NotNil(t, home.Labels)
	assert.NotNil(t, home.Favourites)
	assert.Empty(t, home.RecentlyEdited)
}

func TestSomebodyWithNoRoleInTheSpaceGetsNoHome(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.Home(ctx(), p.carol, p.space, model.RoleNone)
	require.Error(t, err)
}

// The newest edit belongs at the top; that is the whole point of the list.
func TestRecentlyEditedIsNewestFirst(t *testing.T) {
	p := newPageEnv(t)
	first := p.create(t, p.alice, nil, "First")
	second := p.create(t, p.alice, nil, "Second")

	rows, err := p.svc.Pages.RecentlyEdited(ctx(), p.alice, p.space, model.RoleReader, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, second.Page.ID, rows[0].ID)
	assert.Equal(t, first.Page.ID, rows[1].ID)
}

// A title is information: a restricted page must not appear even as a row.
func TestRecentlyEditedHidesPagesTheReaderCannotOpen(t *testing.T) {
	p := newPageEnv(t)
	secret := p.create(t, p.alice, nil, "Q3 Redundancies")
	open := p.create(t, p.alice, nil, "Open")
	p.restrict(t, secret.Page.ID, "alice")

	rows, err := p.svc.Pages.RecentlyEdited(ctx(), p.carol, p.space, model.RoleReader, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, open.Page.ID, rows[0].ID)
}

func TestDeletedPagesLeaveTheRecentList(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)

	rows, err := p.svc.Pages.RecentlyEdited(ctx(), p.alice, p.space, model.RoleReader, 0)
	require.NoError(t, err)
	assert.Empty(t, rows, "the trash is not recent work")
}

func TestStarringAndUnstarringAPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	d := p.decision(t, p.alice, page.Page.ID)

	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice, d, true))
	assert.True(t, p.svc.Pages.IsFavourite(ctx(), p.alice, page.Page.ID))

	// Starring twice is not two stars.
	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice, d, true))
	rows, err := p.svc.Pages.Favourites(ctx(), p.alice, nil)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice, d, false))
	assert.False(t, p.svc.Pages.IsFavourite(ctx(), p.alice, page.Page.ID))
}

// A favourite is a bookmark, not a change: reading is enough to make one.
func TestAReaderMayStarAPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.carol,
		p.decision(t, p.carol, page.Page.ID), true))
	assert.True(t, p.svc.Pages.IsFavourite(ctx(), p.carol, page.Page.ID))
}

// Losing access should quietly remove the row rather than leave an unopenable
// one, the same rule a backlink to an invisible page follows.
func TestAStarredPageDisappearsWhenAccessIsLost(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.carol,
		p.decision(t, p.carol, page.Page.ID), true))

	p.restrict(t, page.Page.ID, "alice")

	rows, err := p.svc.Pages.Favourites(ctx(), p.carol, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestFavouritesCanBeNarrowedToOneSpace(t *testing.T) {
	p := newPageEnv(t)
	here := p.create(t, p.alice, nil, "Here")
	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice,
		p.decision(t, p.alice, here.Page.ID), true))

	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	there, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{SpaceID: other.Space.ID, Title: "There"})
	require.NoError(t, err)
	require.NoError(t, p.svc.Pages.SetFavourite(ctx(), p.alice,
		p.decision(t, p.alice, there.Page.ID), true))

	all, err := p.svc.Pages.Favourites(ctx(), p.alice, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	mine, err := p.svc.Pages.Favourites(ctx(), p.alice, p.space)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, here.Page.ID, mine[0].ID)
}
