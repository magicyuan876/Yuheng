package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// The rule the resolver has enforced since T0.3 and that nothing could
// trigger until T4.4's audit found it.
func TestLockingAPageCapsEverybodyElseAtReader(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Settled")

	require.Equal(t, model.RoleWriter, p.decision(t, p.bob, page.Page.ID).Role)

	view, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)
	assert.True(t, view.Page.IsLocked)

	assert.Equal(t, model.RoleReader, p.decision(t, p.bob, page.Page.ID).Role,
		"a writer is capped at reader")
	assert.Equal(t, model.RoleAdmin, p.decision(t, p.alice, page.Page.ID).Role,
		"a space admin is not")
}

func TestUnlockingRestoresEditing(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Settled")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.SetLocked(ctx(), p.alice, d, true)
	require.NoError(t, err)
	_, err = p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), false)
	require.NoError(t, err)

	assert.Equal(t, model.RoleWriter, p.decision(t, p.bob, page.Page.ID).Role)
}

// If any writer could lock, a disagreement about whether a page is finished
// would be settled by whoever clicked first.
func TestOnlyAnAdministratorLocksOrUnlocks(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetLocked(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID), true)
	require.Error(t, err, "a writer may not lock")

	_, err = p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)

	// And a writer, now capped at reader, cannot undo it either.
	_, err = p.svc.Pages.SetLocked(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID), false)
	require.Error(t, err)
}

func TestLockingIsIdempotent(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	for i := 0; i < 2; i++ {
		view, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
		require.NoError(t, err)
		assert.True(t, view.Page.IsLocked)
	}
}

func TestLockingIsAudited(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)
	assert.True(t, p.audit.has(audit.PageLocked))

	_, err = p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), false)
	require.NoError(t, err)
	assert.True(t, p.audit.has(audit.PageUnlocked))
}

// A locked page refuses the writes it is meant to refuse.
func TestALockedPageRefusesEdits(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	_, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)

	title := "Renamed"
	_, err = p.svc.Pages.Update(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID),
		UpdatePageInput{Title: &title})
	require.Error(t, err)
}

// Excluding a page from the knowledge base is a label, not a permission:
// everybody who could read the page can still read it.
func TestAnExcludedPageIsStillReadable(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Meeting notes")
	d := p.decision(t, p.alice, page.ID)

	view, err := p.svc.Pages.SetKnowledgeExcluded(ctx(), p.alice, d, true)
	require.NoError(t, err)
	assert.True(t, view.ExcludeFromKnowledge)

	assert.Equal(t, model.RoleReader, p.decision(t, p.carol, page.ID).Role,
		"a reader still reads it")
}

func TestExcludingAndIncludingArePairedAndAudited(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetKnowledgeExcluded(ctx(), p.alice, p.decision(t, p.alice, page.ID), true)
	require.NoError(t, err)
	assert.True(t, p.audit.has(audit.PageKnowledgeExcluded))
	view, err := p.svc.Pages.SetKnowledgeExcluded(ctx(), p.alice, p.decision(t, p.alice, page.ID), false)
	require.NoError(t, err)

	assert.False(t, view.ExcludeFromKnowledge)
	assert.True(t, p.audit.has(audit.PageKnowledgeIncluded))
}

func TestARepeatedExclusionChangesNothing(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetKnowledgeExcluded(ctx(), p.alice, p.decision(t, p.alice, page.ID), false)
	require.NoError(t, err)

	assert.False(t, p.audit.has(audit.PageKnowledgeIncluded), "already included: nothing to record")
}

func TestAReaderMayNotExcludeAPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.SetKnowledgeExcluded(ctx(), p.carol, p.decision(t, p.carol, page.ID), true)
	require.Error(t, err)
}

func TestALockedPageRefusesToChangeItsExclusion(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	_, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.ID), true)
	require.NoError(t, err)

	_, err = p.svc.Pages.SetKnowledgeExcluded(ctx(), p.bob, p.decision(t, p.bob, page.ID), true)
	require.Error(t, err)
}
