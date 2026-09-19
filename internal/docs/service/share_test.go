package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/share"
)

// sharing builds a fixture with public sharing switched on, which is not the
// default anywhere.
func newSharingEnv(t *testing.T) *pageEnv {
	t.Helper()
	return newPageEnvWith(t, func(d *Deps) { d.PublicSharing = true })
}

func (p *pageEnv) publish(t *testing.T, pageID string, in CreateShareInput) *ShareView {
	t.Helper()
	view, err := p.svc.Pages.CreateShare(ctx(), p.alice, p.decision(t, p.alice, pageID), in)
	require.NoError(t, err)
	return view
}

func (p *pageEnv) visit(t *testing.T, key, token, wantShort string) *ShareResult {
	t.Helper()
	res, err := p.svc.Pages.ResolveShare(ctx(), key, token, wantShort)
	require.NoError(t, err)
	return res
}

func TestALinkIsCreatedAndResolves(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Release notes")

	link := p.publish(t, page.Page.ID, CreateShareInput{})
	assert.Len(t, link.Key, share.KeyLength)
	assert.True(t, link.Live)
	assert.False(t, link.HasPassword)

	res := p.visit(t, link.Key, "", "")
	require.Equal(t, share.StateOK, res.State)
	require.NotNil(t, res.Page)
	assert.Equal(t, "Release notes", res.Page.Title)
	assert.Equal(t, page.Page.ShortID, res.Page.ShortID)
}

// The whole feature is off unless a deployment asks for it.
func TestSharingIsOffUnlessTheDeploymentTurnsItOn(t *testing.T) {
	p := newPageEnv(t) // no PublicSharing
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.CreateShare(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateShareInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "switched off")
}

// A link that was made while sharing was on must stop resolving when the
// deployment turns it off, not keep working until somebody notices.
func TestTurningSharingOffStopsExistingLinks(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	p.svc.Pages.d.PublicSharing = false
	_, err := p.svc.Pages.ResolveShare(ctx(), link.Key, "", "")
	require.Error(t, err)
}

func TestAKeyThatWasNeverIssuedIsNotFound(t *testing.T) {
	p := newSharingEnv(t)
	for _, key := range []string{"", "nonsense", "ZZZZZZZZZZZZZZZZZZZZZZZZZZ"} {
		_, err := p.svc.Pages.ResolveShare(ctx(), key, "", "")
		require.Error(t, err, "key %q", key)
	}
}

// Rule 1, and the most important test here: restricting a page silently
// stops every link to it, without anybody revoking anything.
func TestRestrictingAPageStopsItsLinks(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})
	require.Equal(t, share.StateOK, p.visit(t, link.Key, "", "").State)

	p.cut(t, p.alice, page.Page.ID)

	res := p.visit(t, link.Key, "", "")
	assert.Equal(t, share.StateGone, res.State)
	assert.Nil(t, res.Page)
}

// The same, from above: an ancestor being restricted takes the page with it.
func TestRestrictingAnAncestorStopsAChildsLinks(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	link := p.publish(t, child.Page.ID, CreateShareInput{})
	require.Equal(t, share.StateOK, p.visit(t, link.Key, "", "").State)

	p.cut(t, p.alice, parent.Page.ID)
	assert.Equal(t, share.StateGone, p.visit(t, link.Key, "", "").State)
}

func TestARestrictedPageCannotBeSharedInTheFirstPlace(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.CreateShare(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateShareInput{})
	require.Error(t, err)
}

func TestDeletingAPageRevokesItsLinks(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)

	assert.Equal(t, share.StateRevoked, p.visit(t, link.Key, "", "").State)
}

// Deleting a parent takes the whole subtree's links with it.
func TestDeletingAParentRevokesTheSubtreesLinks(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	link := p.publish(t, child.Page.ID, CreateShareInput{})

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, parent.Page.ID))
	require.NoError(t, err)

	assert.Equal(t, share.StateRevoked, p.visit(t, link.Key, "", "").State)
}

func TestRevokingALinkStopsIt(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	require.NoError(t, p.svc.Pages.RevokeShare(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), link.ID))
	assert.Equal(t, share.StateRevoked, p.visit(t, link.Key, "", "").State)

	// And it is gone from the owner's list.
	rows, err := p.svc.Pages.Shares(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestAnExpiredLinkStops(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	soon := time.Now().Add(50 * time.Millisecond)
	link := p.publish(t, page.Page.ID, CreateShareInput{ExpiresAt: &soon})

	require.Equal(t, share.StateOK, p.visit(t, link.Key, "", "").State)
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, share.StateExpired, p.visit(t, link.Key, "", "").State)
}

func TestAPasswordIsAskedForAndThenRemembered(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{Password: "open sesame"})
	assert.True(t, link.HasPassword)

	locked := p.visit(t, link.Key, "", "")
	assert.Equal(t, share.StatePassword, locked.State)
	assert.Nil(t, locked.Page, "no content before the password")

	_, err := p.svc.Pages.UnlockShare(ctx(), link.Key, "wrong")
	require.Error(t, err)

	unlocked, err := p.svc.Pages.UnlockShare(ctx(), link.Key, "open sesame")
	require.NoError(t, err)
	require.NotEmpty(t, unlocked.UnlockToken)

	opened := p.visit(t, link.Key, unlocked.UnlockToken, "")
	assert.Equal(t, share.StateOK, opened.State)
	require.NotNil(t, opened.Page)
}

// The token is signed with the password hash, so changing the password
// invalidates every outstanding token with no session store to clear.
func TestChangingThePasswordInvalidatesOutstandingTokens(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{Password: "first one"})
	unlocked, err := p.svc.Pages.UnlockShare(ctx(), link.Key, "first one")
	require.NoError(t, err)
	require.Equal(t, share.StateOK, p.visit(t, link.Key, unlocked.UnlockToken, "").State)

	next := "second one"
	_, err = p.svc.Pages.UpdateShare(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		link.ID, UpdateShareInput{Password: &next})
	require.NoError(t, err)

	assert.Equal(t, share.StatePassword, p.visit(t, link.Key, unlocked.UnlockToken, "").State)
}

func TestAForgedUnlockTokenDoesNotOpenALink(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{Password: "open sesame"})

	for _, token := range []string{
		"", "rubbish", "9999999999.AAAA",
		"9999999999." + "x",
	} {
		assert.Equal(t, share.StatePassword, p.visit(t, link.Key, token, "").State, "token %q", token)
	}
}

// A dead link must not tell a guesser whether their password was right.
func TestARevokedLinkWithAPasswordDoesNotCheckIt(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{Password: "open sesame"})
	require.NoError(t, p.svc.Pages.RevokeShare(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), link.ID))

	res, err := p.svc.Pages.UnlockShare(ctx(), link.Key, "open sesame")
	require.NoError(t, err)
	assert.Equal(t, share.StateRevoked, res.State)
	assert.Empty(t, res.UnlockToken, "the right password buys nothing on a revoked link")
}

func TestASubtreeIsOnlyReachableWhenTheLinkSaysSo(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")

	alone := p.publish(t, parent.Page.ID, CreateShareInput{})
	assert.Equal(t, share.StateGone, p.visit(t, alone.Key, "", child.Page.ShortID).State)
	assert.Empty(t, p.visit(t, alone.Key, "", "").Page.Children)

	withKids := p.publish(t, parent.Page.ID, CreateShareInput{IncludeChildren: true})
	root := p.visit(t, withKids.Key, "", "")
	require.Len(t, root.Page.Children, 1)
	assert.Equal(t, "Child", root.Page.Children[0].Title)

	inside := p.visit(t, withKids.Key, "", child.Page.ShortID)
	require.Equal(t, share.StateOK, inside.State)
	assert.Equal(t, "Child", inside.Page.Title)
	require.Len(t, inside.Page.Breadcrumb, 1)
	assert.Equal(t, "Parent", inside.Page.Breadcrumb[0].Title)
}

// A restricted page inside a shared subtree is not shared, and is not even
// listed: its title is information too.
func TestARestrictedChildIsNotInASharedSubtree(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	p.create(t, p.alice, &parent.Page.ID, "Open")
	secret := p.create(t, p.alice, &parent.Page.ID, "Q3 Redundancies")
	p.cut(t, p.alice, secret.Page.ID)

	link := p.publish(t, parent.Page.ID, CreateShareInput{IncludeChildren: true})
	root := p.visit(t, link.Key, "", "")
	require.Len(t, root.Page.Children, 1)
	assert.Equal(t, "Open", root.Page.Children[0].Title)

	assert.Equal(t, share.StateGone, p.visit(t, link.Key, "", secret.Page.ShortID).State)
}

// A page from somewhere else entirely cannot be made to resolve through
// somebody's link.
func TestAPageOutsideTheLinkCannotBeReachedThroughIt(t *testing.T) {
	p := newSharingEnv(t)
	shared := p.create(t, p.alice, nil, "Shared")
	elsewhere := p.create(t, p.alice, nil, "Elsewhere")
	link := p.publish(t, shared.Page.ID, CreateShareInput{IncludeChildren: true})

	assert.Equal(t, share.StateGone, p.visit(t, link.Key, "", elsewhere.Page.ShortID).State)
}

// A visitor gets a document, not the page object: no space id, no role, no
// edit flags, no author.
func TestAVisitorGetsNothingButTheDocument(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	view := p.visit(t, link.Key, "", "").Page
	require.NotNil(t, view)
	assert.Equal(t, "Handbook", view.SpaceName, "the source is named")
	assert.NotEmpty(t, view.ShortID)
	assert.NotEmpty(t, view.HTML)
}

func TestAWriterMayShareAndAReaderMayNot(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.CreateShare(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID),
		CreateShareInput{})
	require.NoError(t, err, "a writer publishes their own work")

	_, err = p.svc.Pages.CreateShare(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID),
		CreateShareInput{})
	require.Error(t, err, "a reader does not")
}

// Everybody who can read a page is entitled to know it is on the internet.
func TestAReaderCanSeeThatAPageIsShared(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.publish(t, page.Page.ID, CreateShareInput{})

	rows, err := p.svc.Pages.Shares(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID))
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestNamingAnotherPagesLinkDoesNotReachIt(t *testing.T) {
	p := newSharingEnv(t)
	mine := p.create(t, p.alice, nil, "Mine")
	theirs := p.create(t, p.alice, nil, "Theirs")
	link := p.publish(t, theirs.Page.ID, CreateShareInput{})

	err := p.svc.Pages.RevokeShare(ctx(), p.alice, p.decision(t, p.alice, mine.Page.ID), link.ID)
	require.Error(t, err)
}

func TestVisitsAreCounted(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	for i := 0; i < 3; i++ {
		p.visit(t, link.Key, "", "")
	}
	rows, err := p.svc.Pages.Shares(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 3, rows[0].ViewCount)
}

// §8.3's fifth row: the owner hears about growth, not about every visit.
func TestTheOwnerIsToldWhenALinkReachesAMilestone(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	for i := 0; i < int(share.ViewMilestones[0]); i++ {
		p.visit(t, link.Key, "", "")
	}
	inbox, err := p.svc.Pages.Notifications(ctx(), p.alice, false, "", 50)
	require.NoError(t, err)

	found := false
	for _, row := range inbox.Items {
		if string(row.Kind) == "share_viewed" {
			found = true
		}
	}
	assert.True(t, found, "reaching ten readers is worth knowing about")
}

func TestPasswordsAndExpiriesAreValidatedOnCreate(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.CreateShare(ctx(), p.alice, d, CreateShareInput{Password: "ab"})
	require.Error(t, err)

	past := time.Now().Add(-time.Hour)
	_, err = p.svc.Pages.CreateShare(ctx(), p.alice, d, CreateShareInput{ExpiresAt: &past})
	require.Error(t, err)

	tooFar := time.Now().Add(share.MaxLifetime + 48*time.Hour)
	_, err = p.svc.Pages.CreateShare(ctx(), p.alice, d, CreateShareInput{ExpiresAt: &tooFar})
	require.Error(t, err)
}

func TestALinksSettingsCanBeChanged(t *testing.T) {
	p := newSharingEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	p.create(t, p.alice, &parent.Page.ID, "Child")
	link := p.publish(t, parent.Page.ID, CreateShareInput{})
	d := p.decision(t, p.alice, parent.Page.ID)

	yes := true
	updated, err := p.svc.Pages.UpdateShare(ctx(), p.alice, d, link.ID,
		UpdateShareInput{IncludeChildren: &yes, AllowSearchIndex: &yes})
	require.NoError(t, err)
	assert.True(t, updated.IncludeChildren)
	assert.True(t, updated.AllowSearchIndex)
	assert.Len(t, p.visit(t, link.Key, "", "").Page.Children, 1)

	// Removing a password is a distinct act from leaving it alone.
	none := ""
	cleared, err := p.svc.Pages.UpdateShare(ctx(), p.alice, d, link.ID,
		UpdateShareInput{Password: &none})
	require.NoError(t, err)
	assert.False(t, cleared.HasPassword)
}

// The owner should be told a link resolves to nothing rather than handing
// out a URL that does not work.
func TestTheOwnersListSaysWhenALinkIsNotLive(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.publish(t, page.Page.ID, CreateShareInput{})
	p.cut(t, p.alice, page.Page.ID)

	rows, err := p.svc.Pages.Shares(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].Live)
}

// "Content of ours was read from the public internet" is a fact a compliance
// reviewer needs, and the first visit is when it becomes true.
func TestTheFirstVisitToALinkIsAudited(t *testing.T) {
	p := newSharingEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	link := p.publish(t, page.Page.ID, CreateShareInput{})

	assert.False(t, p.audit.has(audit.ShareAccessed), "nothing to record before anybody visits")

	p.visit(t, link.Key, "", "")
	assert.True(t, p.audit.has(audit.ShareAccessed))
}
