package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/draft"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// errorString is a test error that needs no package.
type errorString string

func (e errorString) Error() string { return string(e) }

var errNoSuchEntry = errorString("no such entry")

// fakeDrafter stands in for the model and the knowledge entries.
type fakeDrafter struct {
	entries map[string]fakeEntry
	// reply is what the model returns; err makes the call fail.
	reply string
	err   error
	// lastRequest records what the model was asked, for assertions.
	lastRequest draft.Request
	calls       int
}

type fakeEntry struct{ title, text, kbID string }

func newFakeDrafter() *fakeDrafter {
	return &fakeDrafter{
		entries: map[string]fakeEntry{},
		reply:   "## 配额规则\n\n每个空间有独立配额。",
	}
}

func (f *fakeDrafter) Entry(_ context.Context, id string) (string, string, string, error) {
	entry, ok := f.entries[id]
	if !ok {
		return "", "", "", errNoSuchEntry
	}
	return entry.title, entry.text, entry.kbID, nil
}

func (f *fakeDrafter) Draft(_ context.Context, _ string, req draft.Request) (string, error) {
	f.calls++
	f.lastRequest = req
	if f.err != nil {
		return "", f.err
	}
	return f.reply, nil
}

// newDraftEnv binds the fixture's space to a knowledge base holding entries.
func newDraftEnv(t *testing.T) (*pageEnv, *fakeDrafter) {
	t.Helper()
	drafter := newFakeDrafter()
	drafter.entries["k1"] = fakeEntry{title: "配额文档", text: "每个空间有独立配额。", kbID: "kb-1"}
	drafter.entries["k2"] = fakeEntry{title: "计费文档", text: "超出配额会被拒绝。", kbID: "kb-1"}
	drafter.entries["other"] = fakeEntry{title: "别处的文档", text: "机密内容。", kbID: "kb-other"}

	p := newPageEnvWith(t, func(d *Deps) {
		d.Drafter = drafter
		d.KnowledgeBases = fakeKBs{"kb-1": 1}
	})
	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, useKB("kb-1"), nil)
	require.NoError(t, err)
	fresh, err := p.repos.Spaces.Get(ctx(), 1, p.space.ID)
	require.NoError(t, err)
	p.space = fresh
	return p, drafter
}

func TestAPageIsDraftedFromKnowledgeEntries(t *testing.T) {
	p, drafter := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	res, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结配额规则", KnowledgeIDs: []string{"k1", "k2"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"k1", "k2"}, res.SourceIDs)
	assert.Contains(t, res.Markdown, "配额规则")

	// The model saw the instruction, the title and the material.
	assert.Equal(t, "总结配额规则", drafter.lastRequest.Instruction)
	assert.Equal(t, "存储配额", drafter.lastRequest.PageTitle)
	require.Len(t, drafter.lastRequest.Sources, 2)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.TextContent, "每个空间有独立配额")
}

// Asking where a paragraph came from has to have an answer.
func TestADraftRecordsWhereItCameFrom(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.NoError(t, err)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StringList{"k1"}, stored.SourceRefs)
}

// THE check: without it, naming any entry id would pull its text into a
// document in a space that has nothing to do with it.
func TestAnEntryFromAnotherKnowledgeBaseIsRefused(t *testing.T) {
	p, drafter := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"other"}})
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err), "not found rather than forbidden")
	assert.Equal(t, 0, drafter.calls, "the model was never asked")
}

// One bad id refuses the whole draft rather than quietly dropping it.
func TestOneForeignEntrySpoilsTheWholeRequest(t *testing.T) {
	p, drafter := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1", "other"}})
	require.Error(t, err)
	assert.Equal(t, 0, drafter.calls)
}

func TestAnEntryThatDoesNotExistIsNotFound(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"nope"}})
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(t, err))
}

func TestDraftingNeedsWriteAccess(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.carol,
		p.decision(t, p.carol, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)
}

func TestALockedPageRefusesADraft(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")
	_, err := p.svc.Pages.SetLocked(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)

	_, err = p.svc.Pages.DraftPageFromKnowledge(ctx(), p.bob,
		p.decision(t, p.bob, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)
}

func TestASpaceWithNoKnowledgeBaseCannotDraft(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.Drafter = newFakeDrafter() })
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge base")
}

// A build without a model says so rather than failing obscurely.
func TestADeploymentWithNoModelSaysSo(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)
	assert.Equal(t, 403, httpCode(t, err))
}

func TestAnInstructionIsRequired(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "   ", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)
}

func TestSourcesAreRequiredAndBounded(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice, d,
		DraftInput{Instruction: "总结"})
	require.Error(t, err, "at least one")

	many := make([]string, MaxDraftSources+1)
	for i := range many {
		many[i] = "k1"
	}
	_, err = p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice, d,
		DraftInput{Instruction: "总结", KnowledgeIDs: many})
	require.Error(t, err, "and not unbounded")
}

func TestAnEmptyDraftIsRefusedRatherThanWritten(t *testing.T) {
	p, drafter := newDraftEnv(t)
	drafter.reply = "   "
	page := p.create(t, p.alice, nil, "存储配额")
	p.write(t, p.alice, page.Page.ID, "原有的内容必须保留。")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.TextContent, "原有的内容", "the page was not emptied")
}

func TestAModelFailureLeavesThePageAlone(t *testing.T) {
	p, drafter := newDraftEnv(t)
	drafter.err = errorString("the model is unavailable")
	page := p.create(t, p.alice, nil, "存储配额")
	p.write(t, p.alice, page.Page.ID, "原有的内容必须保留。")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.Error(t, err)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.TextContent, "原有的内容")
}

// A fenced answer is common model behaviour and must not reach the page.
func TestAFencedAnswerIsUnwrapped(t *testing.T) {
	p, drafter := newDraftEnv(t)
	drafter.reply = "```markdown\n## 配额\n\n说明文字。\n```"
	page := p.create(t, p.alice, nil, "存储配额")

	res, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(res.Markdown, "```"))

	stored, err := p.repos.Pages.Get(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.NotContains(t, stored.TextContent, "```")
}

// The draft goes through ReplaceContent, so the previous version is kept.
func TestADraftLeavesTheOldVersionRestorable(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")
	p.write(t, p.alice, page.Page.ID, "人写的原始内容。")

	_, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1"}})
	require.NoError(t, err)

	history, err := p.svc.Pages.History(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), "", 20)
	require.NoError(t, err)
	assert.NotEmpty(t, history.Items, "the human version is still there to go back to")
}

func TestDuplicateSourceIdsAreCollapsed(t *testing.T) {
	p, _ := newDraftEnv(t)
	page := p.create(t, p.alice, nil, "存储配额")

	res, err := p.svc.Pages.DraftPageFromKnowledge(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID),
		DraftInput{Instruction: "总结", KnowledgeIDs: []string{"k1", "k1", "k2"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"k1", "k2"}, res.SourceIDs)
}
