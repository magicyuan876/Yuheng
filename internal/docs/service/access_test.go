package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// access reads the panel as somebody.
func (p *pageEnv) access(t *testing.T, who *acl.Identity, pageID string) *PageAccessView {
	t.Helper()
	view, err := p.svc.Pages.PageAccess(ctx(), who, p.decision(t, who, pageID))
	require.NoError(t, err)
	return view
}

// cut restricts a page through the service (not the fixture's raw helper).
func (p *pageEnv) cut(t *testing.T, who *acl.Identity, pageID string) *PageAccessView {
	t.Helper()
	view, err := p.svc.Pages.SetPageRestricted(ctx(), who, p.decision(t, who, pageID), true)
	require.NoError(t, err)
	return view
}

func (p *pageEnv) grant(t *testing.T, who *acl.Identity, pageID string, in GrantInput) *PageAccessView {
	t.Helper()
	view, err := p.svc.Pages.SetPageGrant(ctx(), who, p.decision(t, who, pageID), in)
	require.NoError(t, err)
	return view
}

func user(id string, role model.SpaceRole) GrantInput {
	return GrantInput{PrincipalType: model.PrincipalUser, PrincipalID: id, Role: role}
}

func TestAnUnrestrictedPageSaysSo(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	view := p.access(t, p.alice, page.Page.ID)
	assert.False(t, view.Restricted)
	assert.Empty(t, view.InheritedFrom)
	assert.Empty(t, view.Grants, "an inheriting page has no list of its own")
	assert.True(t, view.CanManage)
}

// Somebody who can open a page is entitled to know why; making that an
// administrator's question makes "why can't my colleague see this"
// unanswerable.
func TestAReaderMaySeeThePanelButNotChangeIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	view := p.access(t, p.carol, page.Page.ID)
	assert.False(t, view.CanManage)

	_, err := p.svc.Pages.SetPageRestricted(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID), true)
	require.Error(t, err)
}

// A writer can edit the page but not decide who else sees it.
func TestAWriterMayNotRestrictAPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetPageRestricted(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID), true)
	require.Error(t, err)
}

// Restricting is meant to narrow the audience, not to take the page off the
// person who wrote it.
func TestRestrictingSeedsThePagesAuthor(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Bob's draft")

	view := p.cut(t, p.alice, page.Page.ID)
	require.True(t, view.Restricted)
	require.Len(t, view.Grants, 1)
	assert.Equal(t, "bob", view.Grants[0].PrincipalID)
	assert.Equal(t, model.RoleAdmin, view.Grants[0].Role)

	// And Bob can still open his own page.
	assert.True(t, p.decision(t, p.bob, page.Page.ID).Allows(model.RoleWriter))
}

// The caller is a space admin and gets in regardless; a row for them would
// claim their access comes from somewhere it does not.
func TestRestrictingDoesNotSeedTheAdminDoingIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Bob's draft")

	view := p.cut(t, p.alice, page.Page.ID)
	for _, g := range view.Grants {
		assert.NotEqual(t, "alice", g.PrincipalID)
	}
}

func TestRestrictingHidesThePageFromEverybodyElse(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	assert.Equal(t, model.RoleNone, p.decision(t, p.carol, page.Page.ID).Role)
}

// This is rule 1, and the single most important test in the file: a page
// grant is a ceiling, never a promotion.
func TestAGrantCannotLiftSomebodyAboveTheirSpaceRole(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	// Carol is a reader in the space. Granting her admin on the page must
	// leave her a reader.
	view := p.grant(t, p.alice, page.Page.ID, user("carol", model.RoleAdmin))

	d := p.decision(t, p.carol, page.Page.ID)
	assert.Equal(t, model.RoleReader, d.Role)
	assert.False(t, d.Allows(model.RoleWriter))

	// And the panel says so rather than letting somebody believe otherwise.
	var carol *GrantView
	for _, g := range view.Grants {
		if g.PrincipalID == "carol" {
			carol = g
		}
	}
	require.NotNil(t, carol)
	assert.Equal(t, model.RoleAdmin, carol.Role, "what was asked for")
	assert.Equal(t, model.RoleReader, carol.Effective, "what it actually does")
}

func TestAGrantCanNarrowSomebodyBelowTheirSpaceRole(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	p.grant(t, p.alice, page.Page.ID, user("bob", model.RoleReader))

	// Bob writes in this space, but not here.
	d := p.decision(t, p.bob, page.Page.ID)
	assert.Equal(t, model.RoleReader, d.Role)
}

// Restricting a page restricts everything under it, without a row per
// descendant and without anything to re-run when a page moves.
func TestARestrictionReachesTheWholeSubtree(t *testing.T) {
	p := newPageEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	grandchild := p.create(t, p.alice, &child.Page.ID, "Grandchild")

	p.cut(t, p.alice, parent.Page.ID)

	assert.Equal(t, model.RoleNone, p.decision(t, p.carol, child.Page.ID).Role)
	assert.Equal(t, model.RoleNone, p.decision(t, p.carol, grandchild.Page.ID).Role)
}

// A page narrowed from above is not broken, and the panel has to say where
// that came from or it looks like it is.
func TestAPanelNamesTheAncestorThatNarrowedIt(t *testing.T) {
	p := newPageEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	p.cut(t, p.alice, parent.Page.ID)

	view := p.access(t, p.alice, child.Page.ID)
	assert.False(t, view.Restricted, "the child does not cut inheritance itself")
	require.Len(t, view.InheritedFrom, 1)
	assert.Equal(t, parent.Page.ID, view.InheritedFrom[0].ID)
	assert.True(t, view.InheritedFrom[0].Visible)
	assert.Equal(t, "Parent", view.InheritedFrom[0].Title)
}

// Which pages exist above is itself information.
func TestAnAncestorTheCallerCannotOpenIsNamelessInThePanel(t *testing.T) {
	p := newPageEnv(t)
	parent := p.create(t, p.alice, nil, "Q3 Redundancies")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")

	// Parent restricted to alice only; the child granted to bob as well, so
	// bob can open the child but not its parent.
	p.restrict(t, parent.Page.ID, "alice", "bob")
	p.restrict(t, child.Page.ID, "alice", "bob")
	require.NoError(t, p.repos.Access.RemoveGrant(ctx(), 1, parent.Page.ID,
		model.Principal{Type: model.PrincipalUser, ID: "bob"}))
	p.resolver.Invalidate(ctx(), 1)

	// Bob cannot open the child either now (the parent narrows it), which is
	// the point: ask as alice about a chain member carol cannot see.
	view := p.access(t, p.alice, child.Page.ID)
	require.Len(t, view.InheritedFrom, 1)
	assert.True(t, view.InheritedFrom[0].Visible, "alice can see it")

	asCarol, err := p.svc.Pages.PageAccess(ctx(), p.carol, p.decision(t, p.alice, child.Page.ID))
	require.NoError(t, err)
	require.Len(t, asCarol.InheritedFrom, 1)
	assert.False(t, asCarol.InheritedFrom[0].Visible)
	assert.Empty(t, asCarol.InheritedFrom[0].Title, "no title for a page she cannot open")
}

// Grants only mean anything while the page is restricted. Leaving them would
// resurrect a permission set somebody thought they had removed.
func TestRestoringInheritanceTakesTheGrantsWithIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	p.grant(t, p.alice, page.Page.ID, user("carol", model.RoleReader))

	view, err := p.svc.Pages.SetPageRestricted(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), false)
	require.NoError(t, err)
	assert.False(t, view.Restricted)
	assert.Empty(t, view.Grants)

	// Cutting again starts from the seed, not from what was there before.
	again := p.cut(t, p.alice, page.Page.ID)
	for _, g := range again.Grants {
		assert.NotEqual(t, "carol", g.PrincipalID, "the old list did not come back")
	}
	// And Carol can read it again through the space in the meantime.
	assert.Equal(t, model.RoleNone, p.decision(t, p.carol, page.Page.ID).Role)
}

func TestGrantingNeedsThePageToBeRestrictedFirst(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetPageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		user("carol", model.RoleReader))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restrict")
}

func TestAGrantIsRemoved(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	p.grant(t, p.alice, page.Page.ID, user("carol", model.RoleReader))
	require.Equal(t, model.RoleReader, p.decision(t, p.carol, page.Page.ID).Role)

	view, err := p.svc.Pages.RemovePageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		model.PrincipalUser, "carol")
	require.NoError(t, err)
	for _, g := range view.Grants {
		assert.NotEqual(t, "carol", g.PrincipalID)
	}
	assert.Equal(t, model.RoleNone, p.decision(t, p.carol, page.Page.ID).Role)
}

func TestRemovingAGrantThatIsNotThereIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.RemovePageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		model.PrincipalUser, "carol")
	require.Error(t, err)
}

// Granting the same principal twice is a change of role, not an error: a
// panel whose rows cannot be edited would need a remove-then-add dance.
func TestGrantingTwiceChangesTheRole(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	p.grant(t, p.alice, page.Page.ID, user("bob", model.RoleReader))
	view := p.grant(t, p.alice, page.Page.ID, user("bob", model.RoleWriter))

	count := 0
	for _, g := range view.Grants {
		if g.PrincipalID == "bob" {
			count++
			assert.Equal(t, model.RoleWriter, g.Role)
		}
	}
	assert.Equal(t, 1, count, "one row, not two")
}

// A name on a list that cannot open the page is worse than no row: it says
// the job is done when it is not.
func TestThePanelFlagsAGrantToSomebodyOutsideTheSpace(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	// viewer is an active tenant member but not a member of this space.
	view := p.grant(t, p.alice, page.Page.ID, user("viewer", model.RoleWriter))

	var row *GrantView
	for _, g := range view.Grants {
		if g.PrincipalID == "viewer" {
			row = g
		}
	}
	require.NotNil(t, row)
	assert.False(t, row.InSpace, "the panel says the grant is inert")
	assert.Equal(t, model.RoleNone, row.Effective)
	assert.Equal(t, model.RoleNone, p.decision(t, p.viewer, page.Page.ID).Role)
}

// Most people reach a space through a group; calling them "not in this
// space" would be wrong about the common case.
func TestSomebodyWhoIsInTheSpaceThroughAGroupCountsAsInIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	// reviewer-01 joins only through the group, so the panel has nothing but
	// the group to find them by.
	require.NoError(t, p.svc.Spaces.RemoveMember(ctx(), p.alice, p.space,
		model.Principal{Type: model.PrincipalUser, ID: "reviewer-01"}))

	group, err := p.svc.Groups.Create(ctx(), p.alice, CreateGroupInput{Name: "Editors"})
	require.NoError(t, err)
	require.NoError(t, p.svc.Groups.AddMembers(ctx(), p.alice, group.ID, []string{"reviewer-01"}))
	_, err = p.svc.Spaces.SetMembers(ctx(), p.alice, p.space, []MemberInput{
		{Type: model.PrincipalGroup, ID: group.ID, Role: model.RoleWriter},
	})
	require.NoError(t, err)
	// Identities are cached and this one predates the group.
	p.resolver.Invalidate(ctx(), 1)
	reviewer := p.identity("reviewer-01")

	p.cut(t, p.alice, page.Page.ID)
	view := p.grant(t, p.alice, page.Page.ID, user("reviewer-01", model.RoleWriter))

	var row *GrantView
	for _, g := range view.Grants {
		if g.PrincipalID == "reviewer-01" {
			row = g
		}
	}
	require.NotNil(t, row)
	assert.True(t, row.InSpace, "they are in the space, through the group")
	assert.Equal(t, model.RoleWriter, row.Effective)
	assert.Equal(t, model.RoleWriter, p.decision(t, reviewer, page.Page.ID).Role)
}

func TestAGroupCanBeGrantedAccess(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	group, err := p.svc.Groups.Create(ctx(), p.alice, CreateGroupInput{Name: "Reviewers"})
	require.NoError(t, err)
	require.NoError(t, p.svc.Groups.AddMembers(ctx(), p.alice, group.ID, []string{"carol"}))
	// Carol's identity was resolved before the group existed, and identities
	// are cached with their principals.
	p.resolver.Invalidate(ctx(), 1)
	carol := p.identity("carol")

	p.cut(t, p.alice, page.Page.ID)
	view := p.grant(t, p.alice, page.Page.ID,
		GrantInput{PrincipalType: model.PrincipalGroup, PrincipalID: group.ID, Role: model.RoleReader})

	var row *GrantView
	for _, g := range view.Grants {
		if g.PrincipalID == group.ID {
			row = g
		}
	}
	require.NotNil(t, row)
	assert.Equal(t, "Reviewers", row.Name)
	require.NotNil(t, row.GroupMemberCount)
	assert.EqualValues(t, 1, *row.GroupMemberCount)

	// And Carol gets in through it.
	assert.Equal(t, model.RoleReader, p.decision(t, carol, page.Page.ID).Role)
}

func TestGrantingToSomebodyWhoDoesNotExistIsRefused(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.SetPageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		user("nobody-at-all", model.RoleReader))
	require.Error(t, err)

	_, err = p.svc.Pages.SetPageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		GrantInput{PrincipalType: model.PrincipalGroup, PrincipalID: "no-such-group", Role: model.RoleReader})
	require.Error(t, err)
}

func TestARoleThatIsNotARoleIsRefused(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)

	for _, role := range []model.SpaceRole{"", "none", "owner", "Reader"} {
		_, err := p.svc.Pages.SetPageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
			user("carol", role))
		require.Error(t, err, "role %q", role)
	}
}

// Restricting twice is not an error and does not re-seed: clients retry.
func TestRestrictingAnAlreadyRestrictedPageIsANoOp(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Notes")
	first := p.cut(t, p.alice, page.Page.ID)
	require.Len(t, first.Grants, 1)

	_, err := p.svc.Pages.RemovePageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		model.PrincipalUser, "bob")
	require.NoError(t, err)

	again := p.cut(t, p.alice, page.Page.ID)
	assert.True(t, again.Restricted)
	assert.Empty(t, again.Grants, "the seed did not come back")
}

// The whole point of the layer is that a space administrator can always undo
// what a restriction did.
func TestASpaceAdminIsNeverLockedOut(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	_, err := p.svc.Pages.RemovePageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		model.PrincipalUser, "bob")
	require.NoError(t, err)

	// No grants at all now.
	view := p.access(t, p.alice, page.Page.ID)
	assert.Empty(t, view.Grants)
	assert.True(t, view.CanManage, "and she can still put it back")

	_, err = p.svc.Pages.SetPageRestricted(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), false)
	require.NoError(t, err)
	assert.Equal(t, model.RoleWriter, p.decision(t, p.bob, page.Page.ID).Role)
}

func TestAccessChangesAreAudited(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.cut(t, p.alice, page.Page.ID)
	p.grant(t, p.alice, page.Page.ID, user("carol", model.RoleReader))
	_, err := p.svc.Pages.RemovePageGrant(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		model.PrincipalUser, "carol")
	require.NoError(t, err)
	_, err = p.svc.Pages.SetPageRestricted(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), false)
	require.NoError(t, err)

	assert.True(t, p.audit.has(audit.PageAccessRestricted))
	assert.True(t, p.audit.has(audit.PageGrantAdded))
	assert.True(t, p.audit.has(audit.PageGrantRemoved))
	assert.True(t, p.audit.has(audit.PageAccessInherited))
}
