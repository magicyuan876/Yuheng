package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/types"
)

// A page is maintained by its creator until handed over, and only the
// maintainer or an administrator of the page may hand it over — to somebody
// who can edit it.
func TestAPageIsHandedOverByItsMaintainerOrAnAdministrator(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.bob, nil, "Leave policy")
	assert.Equal(t, "bob", page.StewardID, "the creator maintains a page nobody handed over")

	_, err := p.svc.Pages.SetOwner(ctx(), p.carol, p.decision(t, p.carol, page.ID), "carol")
	require.Error(t, err, "a reader may not take a page over")

	_, err = p.svc.Pages.SetOwner(ctx(), p.bob, p.decision(t, p.bob, page.ID), "carol")
	require.Error(t, err, "the new maintainer must be able to edit the page")
	_, err = p.svc.Pages.SetOwner(ctx(), p.bob, p.decision(t, p.bob, page.ID), "stranger")
	require.Error(t, err, "nor somebody from outside the workspace")

	view, err := p.svc.Pages.SetOwner(ctx(), p.bob, p.decision(t, p.bob, page.ID), "alice")
	require.NoError(t, err, "the maintainer hands it over")
	assert.Equal(t, "alice", view.StewardID)
	assert.True(t, p.audit.has(audit.PageOwnerChanged))

	_, err = p.svc.Pages.SetOwner(ctx(), p.bob, p.decision(t, p.bob, page.ID), "bob")
	require.Error(t, err, "and, no longer the maintainer, a writer cannot take it back")
	view, err = p.svc.Pages.SetOwner(ctx(), p.alice, p.decision(t, p.alice, page.ID), "bob")
	require.NoError(t, err, "an administrator of the page can")
	assert.Equal(t, "bob", view.StewardID)
}

// The page view tells its caller whether they may hand the page over, by the
// same rule SetOwner applies.
func TestThePageViewSaysWhoMayHandItOver(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	as := func(user string) context.Context {
		return context.WithValue(ctx(), types.UserIDContextKey, user)
	}
	d := p.decision(t, p.bob, page.ID)
	assert.False(t, canChangeOwner(as("bob"), d), "a writer who does not maintain it")
	assert.True(t, canChangeOwner(as("alice"), p.decision(t, p.alice, page.ID)))

	_, err := p.svc.Pages.SetOwner(ctx(), p.alice, p.decision(t, p.alice, page.ID), "bob")
	require.NoError(t, err)
	assert.True(t, canChangeOwner(as("bob"), p.decision(t, p.bob, page.ID)), "its maintainer")
	assert.False(t, canChangeOwner(as("carol"), p.decision(t, p.carol, page.ID)))
}

// The mirror entry carries the page's maintainer and its last edit as a
// review, and a hand-over reaches it without the page being re-embedded.
func TestTheMirrorCarriesThePagesStewardship(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.bob, nil, "Leave policy")
	p.write(t, p.alice, page.ID, "Everybody has fifteen days of paid leave a year.")
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.NoError(t, err)

	fresh, err := p.repos.Pages.GetAny(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.KnowledgeID)
	st, ok := kb.steward(*fresh.KnowledgeID)
	require.True(t, ok)
	assert.Equal(t, "bob", st.owner, "the creator maintains it")
	assert.Equal(t, "alice", st.reviewedBy, "whoever last rewrote it vouched for it")
	assert.False(t, st.reviewedAt.IsZero())

	updates := kb.updates
	_, err = p.svc.Pages.SetOwner(ctx(), p.bob, p.decision(t, p.bob, page.ID), "alice")
	require.NoError(t, err)
	_, err = p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.NoError(t, err)
	st, _ = kb.steward(*fresh.KnowledgeID)
	assert.Equal(t, "alice", st.owner)
	assert.Equal(t, updates, kb.updates, "a hand-over is not a content change")
}

// A writer vouches for the page as it stands: the mirror's review is theirs,
// now. A reader cannot, and a page outside the knowledge base has nothing to
// confirm.
func TestAWriterConfirmsAPageIsStillRight(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "VPN guide")
	p.write(t, p.alice, page.ID, "Connect to vpn.example.com with your badge number.")
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.NoError(t, err)
	fresh, err := p.repos.Pages.GetAny(ctx(), 1, page.ID)
	require.NoError(t, err)
	before, _ := kb.steward(*fresh.KnowledgeID)

	require.Error(t, p.svc.Pages.ConfirmReviewed(ctx(), p.carol, p.decision(t, p.carol, page.ID)))
	require.NoError(t, p.svc.Pages.ConfirmReviewed(ctx(), p.bob, p.decision(t, p.bob, page.ID)))
	after, _ := kb.steward(*fresh.KnowledgeID)
	assert.Equal(t, "bob", after.reviewedBy)
	assert.Equal(t, "alice", after.owner, "confirming is not taking the page over")
	assert.True(t, after.reviewedAt.After(before.reviewedAt))
	assert.True(t, p.audit.has(audit.PageReviewed))

	unbound := newPageEnv(t)
	loose := unbound.create(t, unbound.alice, nil, "Scratch")
	err = unbound.svc.Pages.ConfirmReviewed(ctx(), unbound.alice, unbound.decision(t, unbound.alice, loose.ID))
	require.Error(t, err)
}
