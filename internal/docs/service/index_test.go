package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/index"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// fakeKnowledge stands in for Yuheng's knowledge service.
type fakeKnowledge struct {
	mu      sync.Mutex
	seq     int
	entries map[string]knowledgeEntry
	// failUpdate makes the next update report the entry as gone.
	failUpdate bool
	// failDelete makes every delete fail, leaving the entry where it is.
	failDelete bool
	// failCreate makes every create fail, as an embedding service that is down
	// would.
	failCreate bool
	// updates counts rewrites, each of which costs an embedding run.
	updates int
	// tenants records the tenant each write was made for.
	tenants []uint64
}

type knowledgeEntry struct {
	kbID, title, body string
}

func newFakeKnowledge() *fakeKnowledge {
	return &fakeKnowledge{entries: map[string]knowledgeEntry{}}
}

func (f *fakeKnowledge) CreateKnowledgeFromText(
	_ context.Context, tenantID uint64, kbID, title, body string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tenants = append(f.tenants, tenantID)
	if f.failCreate {
		return "", errors.New("embedding service unavailable")
	}
	f.seq++
	id := "knowledge-" + string(rune('a'+f.seq-1))
	f.entries[id] = knowledgeEntry{kbID: kbID, title: title, body: body}
	return id, nil
}

func (f *fakeKnowledge) UpdateKnowledgeContent(
	_ context.Context, tenantID uint64, id, title, body string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tenants = append(f.tenants, tenantID)
	if f.failUpdate {
		return false, nil
	}
	if _, ok := f.entries[id]; !ok {
		return false, nil
	}
	f.entries[id] = knowledgeEntry{kbID: f.entries[id].kbID, title: title, body: body}
	f.updates++
	return true, nil
}

func (f *fakeKnowledge) KnowledgeBaseOf(_ context.Context, id string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	return e.kbID, ok, nil
}

func (f *fakeKnowledge) DeleteKnowledge(_ context.Context, tenantID uint64, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tenants = append(f.tenants, tenantID)
	if f.failDelete {
		return errors.New("knowledge base unavailable")
	}
	delete(f.entries, id)
	return nil
}

func (f *fakeKnowledge) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}

func (f *fakeKnowledge) bodies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.entries))
	for _, entry := range f.entries {
		out = append(out, entry.body)
	}
	return out
}

// newIndexEnv builds a fixture whose space is bound to a knowledge base.
func newIndexEnv(t *testing.T) (*pageEnv, *fakeKnowledge) {
	t.Helper()
	kb := newFakeKnowledge()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Knowledge = kb
		d.KnowledgeBases = fakeKBs{"kb-1": 1}
	})
	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, strPtr("kb-1"), nil)
	require.NoError(t, err)
	// The bind returns a view; refresh the fixture's space so later calls see
	// the binding.
	fresh, err := p.repos.Spaces.Get(ctx(), 1, p.space.ID)
	require.NoError(t, err)
	p.space = fresh
	return p, kb
}

func strPtr(s string) *string { return &s }

// indexed re-reads a page and reports whether it carries a knowledge entry.
func (p *pageEnv) indexed(t *testing.T, pageID string) bool {
	t.Helper()
	page, err := p.repos.Pages.GetAny(ctx(), 1, pageID)
	require.NoError(t, err)
	return page.KnowledgeID != nil && *page.KnowledgeID != ""
}

func TestAPageIsMirroredIntoTheKnowledgeBase(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.Page.ID, "每个空间的配额由工作区管理员设置。")

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.True(t, res.Indexed)
	assert.Equal(t, 1, kb.count())
	assert.True(t, p.indexed(t, page.Page.ID))

	// Markdown, not JSON and not HTML.
	body := kb.bodies()[0]
	assert.Contains(t, body, "每个空间的配额")
	assert.NotContains(t, body, "paragraph", "not the ProseMirror JSON")
	assert.NotContains(t, body, "<p>", "not the HTML")
}

// Running it twice is one entry, not two.
func TestIndexingIsIdempotent(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.Page.ID, "足够长的正文内容在这里。")

	for i := 0; i < 3; i++ {
		_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, kb.count())
}

func TestAnEditUpdatesTheSameEntry(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.Page.ID, "第一版的内容在这里。")
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)

	p.write(t, p.alice, page.Page.ID, "第二版的内容完全不同。")
	_, err = p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)

	require.Equal(t, 1, kb.count())
	assert.Contains(t, kb.bodies()[0], "第二版")
	assert.NotContains(t, kb.bodies()[0], "第一版")
}

// THE rule: retrieval has no per-entry permission filter, so a restricted
// page must never reach the knowledge base.
func TestARestrictedPageIsNotIndexed(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "Q3 裁员名单")
	p.write(t, p.alice, page.Page.ID, "这是不该被检索到的机密内容。")
	p.cut(t, p.alice, page.Page.ID)

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.False(t, res.Indexed)
	assert.Equal(t, index.ReasonRestricted, res.Reason)
	assert.Equal(t, 0, kb.count())
}

// The case that makes this a removal rather than a skip: restricting a page
// that was already indexed has to take its entry away, or the restriction is
// cosmetic.
func TestRestrictingAnIndexedPageRemovesItsEntry(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "先公开后收回")
	p.write(t, p.alice, page.Page.ID, "一开始所有人都能看到的内容。")

	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	require.Equal(t, 1, kb.count())

	p.cut(t, p.alice, page.Page.ID)
	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)

	assert.True(t, res.Removed)
	assert.Equal(t, 0, kb.count(), "the entry is gone from retrieval")
	assert.False(t, p.indexed(t, page.Page.ID), "and the page knows it")
}

// A restricted ancestor restricts the page, so the same rule applies.
func TestAPageUnderARestrictedParentIsNotIndexed(t *testing.T) {
	p, kb := newIndexEnv(t)
	parent := p.create(t, p.alice, nil, "机密目录")
	child := p.create(t, p.alice, &parent.Page.ID, "子页面")
	p.write(t, p.alice, child.Page.ID, "继承了上级限制的内容。")
	p.cut(t, p.alice, parent.Page.ID)

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, child.Page.ID)
	require.NoError(t, err)
	assert.Equal(t, index.ReasonRestricted, res.Reason)
	assert.Equal(t, 0, kb.count())
}

func TestATrashedPageIsRemovedFromTheKnowledgeBase(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "会被删掉的页面")
	p.write(t, p.alice, page.Page.ID, "这些内容随页面一起消失。")
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	require.Equal(t, 1, kb.count())

	_, err = p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.True(t, res.Removed)
	assert.Equal(t, 0, kb.count())
}

// A draft is somebody's unfinished thought.
func TestADraftIsNotIndexed(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "草稿页面")
	p.write(t, p.alice, page.Page.ID, "还没写完的内容在这里。")
	_, err := p.svc.Pages.SetPageStatus(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), model.PageDraft)
	require.NoError(t, err)

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Equal(t, index.ReasonDraft, res.Reason)
	assert.Equal(t, 0, kb.count())
}

// A space nobody bound sends nothing, and that is not an error.
func TestAnUnboundSpaceIndexesNothing(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.Knowledge = newFakeKnowledge() })
	page := p.create(t, p.alice, nil, "普通页面")
	p.write(t, p.alice, page.Page.ID, "一段普通的内容在这里。")

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.False(t, res.Indexed)
	assert.Equal(t, index.ReasonNoBinding, res.Reason)
}

// A build with no knowledge service leaves every space unindexed rather than
// failing.
func TestNoKnowledgeServiceIsNotAnError(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "普通页面")

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.False(t, res.Indexed)
}

func TestAnEmptyPageIsNotIndexed(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "空页面")

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.Equal(t, index.ReasonEmpty, res.Reason)
	assert.Equal(t, 0, kb.count())
}

// An entry deleted from the knowledge base side must not leave the page
// unindexed for ever.
func TestAnEntryThatVanishedIsRecreated(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.Page.ID, "足够长的正文内容在这里。")
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)

	// Somebody emptied the knowledge base.
	for id := range kb.entries {
		require.NoError(t, kb.DeleteKnowledge(ctx(), 1, id))
	}
	require.Equal(t, 0, kb.count())

	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.Page.ID)
	require.NoError(t, err)
	assert.True(t, res.Indexed)
	assert.Equal(t, 1, kb.count(), "a new entry rather than a page left unindexed")
}

func TestAPageThatNoLongerExistsIsNotAnError(t *testing.T) {
	p, _ := newIndexEnv(t)
	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, "no-such-page")
	require.NoError(t, err)
	assert.False(t, res.Indexed)
}

// ---- rebuilding a space ---------------------------------------------------------

func TestASpaceIsRebuiltInOneCall(t *testing.T) {
	p, kb := newIndexEnv(t)
	for _, title := range []string{"一", "二", "三"} {
		page := p.create(t, p.alice, nil, title)
		p.write(t, p.alice, page.Page.ID, "足够长的正文内容"+title+"。")
	}

	results, next, err := p.svc.Pages.SyncSpaceToKnowledge(ctx(), p.alice, p.space,
		model.RoleAdmin, "", 0)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Empty(t, next, "no more pages")
	assert.Equal(t, 3, kb.count())
}

// A rebuild costs embedding calls and changes what everybody's retrieval
// returns.
func TestRebuildingASpaceNeedsAnAdministrator(t *testing.T) {
	p, _ := newIndexEnv(t)
	_, _, err := p.svc.Pages.SyncSpaceToKnowledge(ctx(), p.bob, p.space, model.RoleWriter, "", 0)
	require.Error(t, err)
}

// A space with ten thousand pages must not become one transaction.
func TestARebuildIsPagedAndResumable(t *testing.T) {
	p, kb := newIndexEnv(t)
	for i := 0; i < 5; i++ {
		page := p.create(t, p.alice, nil, "页面")
		p.write(t, p.alice, page.Page.ID, "足够长的正文内容在这里。")
	}

	seen := 0
	cursor := ""
	for round := 0; round < 10; round++ {
		results, next, err := p.svc.Pages.SyncSpaceToKnowledge(ctx(), p.alice, p.space,
			model.RoleAdmin, cursor, 2)
		require.NoError(t, err)
		seen += len(results)
		cursor = next
		if cursor == "" {
			break
		}
	}
	assert.Equal(t, 5, seen)
	assert.Equal(t, 5, kb.count())
}

// A rebuild that gives up halfway leaves a space half-indexed.
func TestARebuildReportsPerPageReasons(t *testing.T) {
	p, kb := newIndexEnv(t)
	good := p.create(t, p.alice, nil, "可以索引的页面")
	p.write(t, p.alice, good.Page.ID, "足够长的正文内容在这里。")
	secret := p.create(t, p.alice, nil, "受限页面")
	p.write(t, p.alice, secret.Page.ID, "同样足够长的机密内容。")
	p.cut(t, p.alice, secret.Page.ID)

	results, _, err := p.svc.Pages.SyncSpaceToKnowledge(ctx(), p.alice, p.space,
		model.RoleAdmin, "", 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	reasons := map[string]string{}
	for _, r := range results {
		reasons[r.PageID] = r.Reason
	}
	assert.Empty(t, reasons[good.Page.ID])
	assert.Equal(t, index.ReasonRestricted, reasons[secret.Page.ID])
	assert.Equal(t, 1, kb.count(), "only the unrestricted one")
}

// ---- the pages below a page that changed ---------------------------------------

// syncSubtree does what the indexer does for an event that takes content away
// from readers: queue the page and everything beneath it at once, then work the
// queue until it is empty.
func (p *pageEnv) syncSubtree(pageID string) error {
	if err := p.svc.Pages.QueueIndex(ctx(), 1, pageID, true, 0); err != nil {
		return err
	}
	return p.drainIndex()
}

// drainIndex works the queue as a worker would.
func (p *pageEnv) drainIndex() error {
	for {
		n, err := p.svc.Pages.ProcessIndexQueue(ctx(), 50)
		if err != nil || n == 0 {
			return err
		}
	}
}

// indexAll gives every page an entry, as an earlier rebuild would have.
func (p *pageEnv) indexAll(t *testing.T, ids ...string) {
	t.Helper()
	for _, id := range ids {
		_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, id)
		require.NoError(t, err)
	}
}

// A restriction is inherited. Re-syncing only the restricted page left the
// pages below it in the knowledge base, searchable by people the restriction
// was meant to keep out.
func TestRestrictingAPageRemovesTheEntriesBelowIt(t *testing.T) {
	p, kb := newIndexEnv(t)
	parent := p.create(t, p.alice, nil, "机密目录")
	child := p.create(t, p.alice, &parent.ID, "子页面")
	grandchild := p.create(t, p.alice, &child.ID, "孙页面")
	sibling := p.create(t, p.alice, nil, "无关页面")
	for _, pg := range []*PageView{parent, child, grandchild, sibling} {
		p.write(t, p.alice, pg.ID, "足够长的正文内容在这里："+pg.Title)
	}
	p.indexAll(t, parent.ID, child.ID, grandchild.ID, sibling.ID)
	require.Equal(t, 4, kb.count())

	p.cut(t, p.alice, parent.ID)
	err := p.syncSubtree(parent.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, kb.count(), "only the page outside the restricted subtree remains")
	assert.False(t, p.indexed(t, child.ID))
	assert.False(t, p.indexed(t, grandchild.ID))
	assert.True(t, p.indexed(t, sibling.ID))
}

func TestTrashingAPageRemovesTheEntriesBelowItAndRestoringBringsThemBack(t *testing.T) {
	p, kb := newIndexEnv(t)
	parent := p.create(t, p.alice, nil, "目录")
	child := p.create(t, p.alice, &parent.ID, "子页面")
	for _, pg := range []*PageView{parent, child} {
		p.write(t, p.alice, pg.ID, "足够长的正文内容在这里："+pg.Title)
	}
	p.indexAll(t, parent.ID, child.ID)
	require.Equal(t, 2, kb.count())

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, parent.ID))
	require.NoError(t, err)
	err = p.syncSubtree(parent.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, kb.count(), "nothing of a trashed subtree stays searchable")

	_, err = p.svc.Pages.Restore(ctx(), p.alice, parent.ID)
	require.NoError(t, err)
	err = p.syncSubtree(parent.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, kb.count(), "and it all comes back")
}

// Visiting a whole subtree must not re-embed the pages that did not change.
func TestARepeatedSyncLeavesUnchangedEntriesAlone(t *testing.T) {
	p, kb := newIndexEnv(t)
	parent := p.create(t, p.alice, nil, "目录")
	child := p.create(t, p.alice, &parent.ID, "子页面")
	for _, pg := range []*PageView{parent, child} {
		p.write(t, p.alice, pg.ID, "足够长的正文内容在这里："+pg.Title)
	}
	p.indexAll(t, parent.ID, child.ID)

	err := p.syncSubtree(parent.ID)
	require.NoError(t, err)

	// Nothing changed, so nothing is sent again, the page named by the event
	// included: sending means embedding, which costs money.
	assert.Zero(t, kb.updates)
	assert.Equal(t, 2, kb.count())
}

// A page below the one that changed that has no entry, and should, gets one.
func TestASubtreeSyncIndexesPagesThatShouldNowBeThere(t *testing.T) {
	p, kb := newIndexEnv(t)
	parent := p.create(t, p.alice, nil, "目录")
	child := p.create(t, p.alice, &parent.ID, "子页面")
	p.write(t, p.alice, parent.ID, "足够长的正文内容在这里：目录")
	p.write(t, p.alice, child.ID, "足够长的正文内容在这里：子页面")
	p.cut(t, p.alice, parent.ID)
	p.indexAll(t, parent.ID, child.ID)
	require.Equal(t, 0, kb.count())

	// Lifting the restriction makes both eligible.
	p.access(t, p.alice, parent.ID)
	_, err := p.svc.Pages.SetPageRestricted(ctx(), p.alice, p.decision(t, p.alice, parent.ID), false)
	require.NoError(t, err)
	err = p.syncSubtree(parent.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, kb.count())
}

// ---- an entry lives in the knowledge base its space is bound to ----------------

// newTwoKBEnv binds the fixture's space to kb-1 and makes kb-2 available.
func newTwoKBEnv(t *testing.T) (*pageEnv, *fakeKnowledge) {
	t.Helper()
	kb := newFakeKnowledge()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Knowledge = kb
		d.KnowledgeBases = fakeKBs{"kb-1": 1, "kb-2": 1}
	})
	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, strPtr("kb-1"), nil)
	require.NoError(t, err)
	fresh, err := p.repos.Spaces.Get(ctx(), 1, p.space.ID)
	require.NoError(t, err)
	p.space = fresh
	return p, kb
}

func (f *fakeKnowledge) inKB(kbID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.entries {
		if e.kbID == kbID {
			n++
		}
	}
	return n
}

// Updating in place would leave the page in the old knowledge base, answerable
// to that base's audience rather than that of the space it now belongs to.
func TestAPageMovedToASpaceWithAnotherKnowledgeBaseFollowsIt(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	target, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Target"})
	require.NoError(t, err)
	_, err = p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, target.Space, strPtr("kb-2"), nil)
	require.NoError(t, err)

	page := p.create(t, p.alice, nil, "会搬家的页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)
	require.Equal(t, 1, kb.inKB("kb-1"))

	_, err = p.svc.Pages.Move(ctx(), p.alice, p.decision(t, p.alice, page.ID),
		MovePageInput{SpaceID: target.ID})
	require.NoError(t, err)
	err = p.syncSubtree(page.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, kb.inKB("kb-1"), "gone from the old knowledge base")
	assert.Equal(t, 1, kb.inKB("kb-2"), "and present in the new one")
	assert.Equal(t, 1, kb.count(), "never in both")
}

func TestReBindingASpaceMovesItsEntriesOnTheNextSync(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	page := p.create(t, p.alice, nil, "页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, strPtr("kb-2"), nil)
	require.NoError(t, err)
	_, _, err = p.svc.Pages.SyncSpaceToKnowledge(ctx(), p.alice, p.space, model.RoleAdmin, "", 0)
	require.NoError(t, err)

	assert.Equal(t, 0, kb.inKB("kb-1"))
	assert.Equal(t, 1, kb.inKB("kb-2"))
}

// If the stray entry cannot be removed, a second copy must not be made beside
// it: the page would be searchable in both places.
func TestAStrayEntryThatCannotBeRemovedIsNotDuplicated(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	page := p.create(t, p.alice, nil, "页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, strPtr("kb-2"), nil)
	require.NoError(t, err)
	kb.failDelete = true
	_, err = p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)

	require.Error(t, err)
	assert.Equal(t, 1, kb.count())
	assert.Equal(t, 1, kb.inKB("kb-1"))
}

// A restricted page whose entry cannot be deleted is still searchable. The
// failure must surface, with the pointer kept, so a later attempt finds it.
func TestAFailedRemovalIsReportedAndRemembered(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "先公开后收回")
	p.write(t, p.alice, page.ID, "一开始所有人都能看到的内容。")
	p.indexAll(t, page.ID)

	p.cut(t, p.alice, page.ID)
	kb.failDelete = true
	_, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.Error(t, err)
	assert.True(t, p.indexed(t, page.ID), "the pointer stays so the removal is retried")

	kb.failDelete = false
	res, err := p.svc.Pages.SyncPageToKnowledge(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.True(t, res.Removed)
	assert.Equal(t, 0, kb.count())
}

// The workers run outside any request, so each write has to say whose knowledge
// base it is for.
func TestEveryKnowledgeWriteNamesThePagesTenant(t *testing.T) {
	p, kb := newIndexEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.ID, "第一版的内容在这里。")
	p.indexAll(t, page.ID)
	p.write(t, p.alice, page.ID, "第二版的内容完全不同。")
	p.indexAll(t, page.ID)
	p.cut(t, p.alice, page.ID)
	p.indexAll(t, page.ID) // create, update, delete

	require.Len(t, kb.tenants, 3)
	for _, tenant := range kb.tenants {
		assert.EqualValues(t, 1, tenant)
	}
}
