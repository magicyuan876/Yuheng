package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/types"
)

// supersedeEnv is a page env with two pages mirrored into the knowledge base
// and one finding between them, seen from the new page.
type supersedeEnv struct {
	*pageEnv
	oldPage, newPage *PageView
}

func newSupersedeEnv(t *testing.T) *supersedeEnv {
	t.Helper()
	fake := &fakeFindings{knowledgeID: "k-new"}
	p := newPageEnvWith(t, func(d *Deps) { d.Findings = fake })
	oldPage := p.create(t, p.alice, nil, "Leave policy 2024")
	newPage := p.create(t, p.bob, nil, "Leave policy 2026")
	require.NoError(t, p.repos.Pages.SetKnowledgeID(ctx(), 1, oldPage.ID, strp("k-old")))
	require.NoError(t, p.repos.Pages.SetKnowledgeID(ctx(), 1, newPage.ID, strp("k-new")))
	f := duplicateWith("f1", "k-old")
	f.Subject = types.KnowledgeRef{KnowledgeID: "k-new"}
	fake.rows = []*types.KnowledgeFindingView{f}
	return &supersedeEnv{pageEnv: p, oldPage: oldPage, newPage: newPage}
}

// The writer of the new version says it replaces the old one: the old page
// leaves the knowledge base, stays readable, and says what replaced it;
// letting it back in clears the mark.
func TestAPageSupersedesAnotherFromItsOwnNotice(t *testing.T) {
	e := newSupersedeEnv(t)
	res, err := e.svc.Pages.SupersedeFromPage(ctx(), e.bob, e.decision(t, e.bob, e.newPage.ID), "f1")
	require.NoError(t, err)
	assert.Equal(t, &types.SupersedeResult{RetiredKnowledgeID: "k-old", How: types.RetiredExcluded}, res)

	old, err := e.repos.Pages.GetAny(ctx(), 1, e.oldPage.ID)
	require.NoError(t, err)
	assert.True(t, old.ExcludeFromKnowledge)
	require.NotNil(t, old.SupersededBy)
	assert.Equal(t, "k-new", old.SupersededBy.KnowledgeID)
	assert.Equal(t, e.newPage.ID, old.SupersededBy.PageID)
	assert.Equal(t, "Leave policy 2026", old.SupersededBy.Title)
	assert.Equal(t, "bob", old.SupersededBy.By)
	assert.False(t, old.SupersededBy.At.IsZero())
	assert.True(t, e.audit.has(audit.PageSuperseded))
	assert.Equal(t, "reader", string(e.decision(t, e.carol, e.oldPage.ID).Role), "a reader still reads it")

	_, err = e.svc.Pages.SetKnowledgeExcluded(ctx(), e.alice, e.decision(t, e.alice, e.oldPage.ID), false)
	require.NoError(t, err)
	old, err = e.repos.Pages.GetAny(ctx(), 1, e.oldPage.ID)
	require.NoError(t, err)
	assert.Nil(t, old.SupersededBy, "let back in, it is no longer superseded")
}

// Superseding changes the other page, so the caller must be able to change
// it; a finding that is not the page's, or whose other side is not a page, is
// refused.
func TestSupersedingFromAPageNeedsWriteAccessToBoth(t *testing.T) {
	e := newSupersedeEnv(t)
	_, err := e.svc.Pages.SupersedeFromPage(ctx(), e.carol, e.decision(t, e.carol, e.newPage.ID), "f1")
	require.Error(t, err, "a reader of the new page")

	_, err = e.svc.Pages.SetLocked(ctx(), e.alice, e.decision(t, e.alice, e.oldPage.ID), true)
	require.NoError(t, err)
	_, err = e.svc.Pages.SupersedeFromPage(ctx(), e.bob, e.decision(t, e.bob, e.newPage.ID), "f1")
	require.Error(t, err, "a writer capped by the old page's lock")

	_, err = e.svc.Pages.SupersedeFromPage(ctx(), e.bob, e.decision(t, e.bob, e.newPage.ID), "missing")
	require.Error(t, err)

	e.svc.Pages.d.Findings.(*fakeFindings).rows[0].Related.KnowledgeID = "k-upload"
	_, err = e.svc.Pages.SupersedeFromPage(ctx(), e.alice, e.decision(t, e.alice, e.newPage.ID), "f1")
	require.Error(t, err, "an uploaded file is the knowledge base's to remove")
}

// From the knowledge base's side the person in the request supersedes the
// page, under the same rule, and the mark names the page that stays.
func TestRetiringAMirrorFromTheKnowledgeBase(t *testing.T) {
	e := newSupersedeEnv(t)
	as := func(user string) context.Context { return context.WithValue(ctx(), types.UserIDContextKey, user) }
	replacement := types.KnowledgeRef{KnowledgeID: "k-new", Title: "stale title"}

	require.Error(t, e.svc.Pages.RetireMirror(as("carol"), 1, "k-old", replacement), "a reader may not")
	require.Error(t, e.svc.Pages.RetireMirror(ctx(), 1, "k-old", replacement), "nor nobody")
	require.Error(t, e.svc.Pages.RetireMirror(as("alice"), 1, "k-missing", replacement))

	require.NoError(t, e.svc.Pages.RetireMirror(as("alice"), 1, "k-old", replacement))
	old, err := e.repos.Pages.GetAny(ctx(), 1, e.oldPage.ID)
	require.NoError(t, err)
	assert.True(t, old.ExcludeFromKnowledge)
	require.NotNil(t, old.SupersededBy)
	assert.Equal(t, e.newPage.ID, old.SupersededBy.PageID)
	assert.Equal(t, "Leave policy 2026", old.SupersededBy.Title, "the page's own title, not the finding's snapshot")
}
