package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// publicSpace makes the fixture's space readable by anyone.
func (p *pageEnv) makePublic(t *testing.T) *model.Space {
	t.Helper()
	vis := model.VisibilityPublic
	view, err := p.svc.Spaces.Update(ctx(), p.alice, p.space, UpdateSpaceInput{Visibility: &vis})
	require.NoError(t, err)
	return view.Space
}

func TestAPublicSpaceIsReadableWithoutLoggingIn(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Handbook home")
	space := p.makePublic(t)

	view, err := p.svc.Pages.PublicSpace(ctx(), space.ID)
	require.NoError(t, err)
	assert.Equal(t, "Handbook", view.Name)
	require.Len(t, view.Pages, 1)
	assert.Equal(t, "Handbook home", view.Pages[0].Title)

	got, err := p.svc.Pages.PublicSpacePage(ctx(), space.ID, page.Page.ShortID)
	require.NoError(t, err)
	assert.Equal(t, "Handbook home", got.Title)
	assert.True(t, got.AllowSearchIndex, "a public space is meant to be found")
}

// Whether a private space exists under a given id is not a visitor's
// business, so it is reported the same way a missing one is.
func TestAPrivateSpaceIsNotFoundAnonymously(t *testing.T) {
	p := newSharingEnv(t)
	_, err := p.svc.Pages.PublicSpace(ctx(), p.space.ID)
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err))
}

func TestAPublicSpaceNeedsTheDeploymentSwitch(t *testing.T) {
	p := newSharingEnv(t)
	space := p.makePublic(t)

	p.svc.Pages.d.PublicSharing = false
	_, err := p.svc.Pages.PublicSpace(ctx(), space.ID)
	require.Error(t, err)
}

// The same rule as a share link: restricted means not published.
func TestRestrictedPagesAreInvisibleInAPublicSpace(t *testing.T) {
	p := newSharingEnv(t)
	open := p.create(t, p.alice, nil, "Open")
	secret := p.create(t, p.alice, nil, "Q3 Redundancies")
	p.cut(t, p.alice, secret.Page.ID)
	space := p.makePublic(t)

	view, err := p.svc.Pages.PublicSpace(ctx(), space.ID)
	require.NoError(t, err)
	require.Len(t, view.Pages, 1)
	assert.Equal(t, open.Page.ShortID, view.Pages[0].ShortID)

	_, err = p.svc.Pages.PublicSpacePage(ctx(), space.ID, secret.Page.ShortID)
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err))
}

func TestARestrictedAncestorHidesItsChildrenFromAPublicSpace(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	p.cut(t, p.alice, parent.Page.ID)
	space := p.makePublic(t)

	_, err := p.svc.Pages.PublicSpacePage(ctx(), space.ID, child.Page.ShortID)
	require.Error(t, err)
}

func TestATrashedPageIsGoneFromAPublicSpace(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	space := p.makePublic(t)

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)

	_, err = p.svc.Pages.PublicSpacePage(ctx(), space.ID, page.Page.ShortID)
	require.Error(t, err)
}

// A short id from another space must not render through a public space's
// address.
func TestAPageFromAnotherSpaceCannotBeReadThroughAPublicOne(t *testing.T) {
	p := newSharingEnv(t)
	space := p.makePublic(t)

	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Private"})
	require.NoError(t, err)
	hidden, err := p.svc.Pages.Create(ctx(), p.alice,
		CreatePageInput{SpaceID: other.Space.ID, Title: "Salaries"})
	require.NoError(t, err)

	_, err = p.svc.Pages.PublicSpacePage(ctx(), space.ID, hidden.Page.ShortID)
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err))
}

func TestAPublicSpacePageCarriesItsBreadcrumbAndChildren(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	p.create(t, p.alice, &child.Page.ID, "Grandchild")
	space := p.makePublic(t)

	got, err := p.svc.Pages.PublicSpacePage(ctx(), space.ID, child.Page.ShortID)
	require.NoError(t, err)
	require.Len(t, got.Breadcrumb, 1)
	assert.Equal(t, "Parent", got.Breadcrumb[0].Title)
	require.Len(t, got.Children, 1)
	assert.Equal(t, "Grandchild", got.Children[0].Title)
}
