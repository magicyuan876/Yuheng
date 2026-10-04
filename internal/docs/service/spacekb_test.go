package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
)

// fakeMaker stands in for the adapter over the knowledge-base service.
type fakeMaker struct {
	mu      sync.Mutex
	made    []madeKB
	deleted []string
	// noEmbedding makes every create fail as a workspace without an
	// embedding model does.
	noEmbedding bool
}

type madeKB struct {
	tenantID                       uint64
	id, name, description, storage string
}

func (f *fakeMaker) CreateForSpace(_ context.Context, tenantID uint64, name, description, storage string) (
	*types.KnowledgeBase, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.noEmbedding {
		return nil, apperrors.NewEmbeddingModelRequiredError()
	}
	id := "kb-made-" + string(rune('a'+len(f.made)))
	f.made = append(f.made, madeKB{tenantID: tenantID, id: id, name: name, description: description, storage: storage})
	return &types.KnowledgeBase{ID: id, TenantID: tenantID, Name: name, Type: types.KnowledgeBaseTypeDocument}, nil
}

func (f *fakeMaker) DeleteKnowledgeBase(_ context.Context, _ uint64, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeMaker) snapshot() ([]madeKB, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]madeKB(nil), f.made...), append([]string(nil), f.deleted...)
}

// oneBackend resolves every request to one backend, so a test can see which
// backend the new knowledge base was put on.
type oneBackend struct{}

func (oneBackend) ResolveBackend(_ context.Context, _ uint64, id string) (*types.StorageBackend, error) {
	if id == "" {
		id = "sb-default"
	}
	return &types.StorageBackend{ID: id}, nil
}

func createKB() *KnowledgeBaseChoice { return &KnowledgeBaseChoice{Mode: KnowledgeBaseCreate} }

func appCode(t *testing.T, err error) apperrors.ErrorCode {
	t.Helper()
	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "expected AppError, got %T: %v", err, err)
	return appErr.Code
}

// failSpaceWrites makes every insert and update of a space fail inside the
// database, after the service has checked everything it can: the one failure
// that can only be found once the knowledge base exists.
func (e *env) failSpaceWrites() {
	e.t.Helper()
	require.NoError(e.t, e.gorm.Exec(`
CREATE FUNCTION refuse_space_write() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'space writes refused by the test'; END $$;
CREATE TRIGGER refuse_space_write BEFORE INSERT OR UPDATE ON docs_spaces
FOR EACH ROW EXECUTE FUNCTION refuse_space_write();`).Error)
}

func TestCreatingASpaceCanMakeItsKnowledgeBase(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) {
		d.KnowledgeBaseMaker = maker
		d.StorageBackends = oneBackend{}
	})
	v, err := e.svc.Spaces.Create(ctx(), e.identity("alice"), CreateSpaceInput{
		Name: "Handbook", Description: "How we work", KnowledgeBase: createKB(), StorageBackendID: strp("sb-team"),
	})
	require.NoError(t, err)

	made, deleted := maker.snapshot()
	require.Len(t, made, 1)
	assert.Equal(t, madeKB{
		tenantID: 1, id: "kb-made-a", name: "Handbook", description: "How we work", storage: "sb-team",
	}, made[0], "named like the space, on the space's own storage backend")
	assert.Empty(t, deleted)
	require.NotNil(t, v.KnowledgeBaseID)
	assert.Equal(t, "kb-made-a", *v.KnowledgeBaseID)
}

func TestWithoutAnEmbeddingModelNeitherTheSpaceNorAKnowledgeBaseIsMade(t *testing.T) {
	maker := &fakeMaker{noEmbedding: true}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBaseMaker = maker })
	alice := e.identity("alice")
	_, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Handbook", KnowledgeBase: createKB()})
	require.Error(t, err)
	assert.Equal(t, apperrors.ErrEmbeddingModelRequired, appCode(t, err),
		"a code the form can point at the model settings with")

	spaces, err := e.svc.Spaces.List(ctx(), alice)
	require.NoError(t, err)
	assert.Empty(t, spaces)
}

// Everything that can refuse the space is asked before the knowledge base is
// made, so the common refusals leave nothing to clean up.
func TestARefusedSpaceMakesNoKnowledgeBase(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBaseMaker = maker })
	alice := e.identity("alice")
	_, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "First", Slug: "taken"})
	require.NoError(t, err)

	_, err = e.svc.Spaces.Create(ctx(), alice,
		CreateSpaceInput{Name: "Second", Slug: "taken", KnowledgeBase: createKB()})
	assert.Equal(t, 409, httpCode(t, err))
	_, err = e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: " ", KnowledgeBase: createKB()})
	assert.Equal(t, 400, httpCode(t, err))

	made, _ := maker.snapshot()
	assert.Empty(t, made)
}

// The write that fails after the knowledge base exists takes it with it.
func TestAKnowledgeBaseMadeForASpaceThatWasNotWrittenIsDeleted(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBaseMaker = maker })
	alice := e.identity("alice")
	e.failSpaceWrites()

	_, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Handbook", KnowledgeBase: createKB()})
	require.Error(t, err)

	made, deleted := maker.snapshot()
	require.Len(t, made, 1)
	assert.Equal(t, []string{made[0].id}, deleted, "no orphan knowledge base")
}

func TestRebindingToANewKnowledgeBaseDeletesItIfTheSpaceIsNotUpdated(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBaseMaker = maker })
	alice := e.identity("alice")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Handbook"})
	require.NoError(t, err)
	e.failSpaceWrites()

	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), alice, sp.Space, createKB(), nil)
	require.Error(t, err)
	made, deleted := maker.snapshot()
	require.Len(t, made, 1)
	assert.Equal(t, []string{made[0].id}, deleted)
}

func TestAnExistingSpaceCanHaveAKnowledgeBaseMade(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBaseMaker = maker })
	alice := e.identity("alice")
	sp, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Handbook"})
	require.NoError(t, err)

	v, err := e.svc.Spaces.BindKnowledgeBase(ctx(), alice, sp.Space, createKB(), nil)
	require.NoError(t, err)
	made, deleted := maker.snapshot()
	require.Len(t, made, 1)
	assert.Empty(t, deleted)
	assert.Equal(t, "Handbook", made[0].name)
	assert.Equal(t, made[0].id, *v.KnowledgeBaseID)
	assert.True(t, e.audit.has(audit.SpaceKBBound))
}

func TestKnowledgeBaseChoicesAreValidated(t *testing.T) {
	e := newEnv(t, func(d *Deps) {
		d.KnowledgeBaseMaker = &fakeMaker{}
		d.KnowledgeBases = fakeKBs{"kb-1": 1}
	})
	alice := e.identity("alice")
	for _, c := range []*KnowledgeBaseChoice{
		{Mode: "sometimes"},
		{Mode: KnowledgeBaseExisting},
		{Mode: KnowledgeBaseCreate, ID: "kb-1"},
		{Mode: KnowledgeBaseNone, ID: "kb-1"},
	} {
		_, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Handbook", KnowledgeBase: c})
		assert.Equal(t, 400, httpCode(t, err), "%+v", *c)
	}
}

// Binding adds every page of the space to the knowledge base, so it follows
// the knowledge base's own rule for adding documents.
func TestOnlyWhoMayFillAKnowledgeBaseMayBindIt(t *testing.T) {
	kbs := kbTable{
		"kb-alice":   {ID: "kb-alice", TenantID: 1, Type: types.KnowledgeBaseTypeDocument, CreatorID: "alice"},
		"kb-shared":  {ID: "kb-shared", TenantID: 1, Type: types.KnowledgeBaseTypeDocument},
		"kb-faq":     {ID: "kb-faq", TenantID: 1, Type: types.KnowledgeBaseTypeFAQ, CreatorID: "alice"},
		"kb-hidden":  {ID: "kb-hidden", TenantID: 1, Type: types.KnowledgeBaseTypeDocument, IsTemporary: true},
		"kb-foreign": {ID: "kb-foreign", TenantID: 2, Type: types.KnowledgeBaseTypeDocument, CreatorID: "alice"},
	}
	e := newEnv(t, func(d *Deps) { d.KnowledgeBases = kbs })
	alice, bob, admin := e.identity("alice"), e.identity("bob"), e.identity("admin")
	sp, err := e.svc.Spaces.Create(ctx(), bob, CreateSpaceInput{Name: "Bob's"})
	require.NoError(t, err)

	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), bob, sp.Space, useKB("kb-alice"), nil)
	assert.Equal(t, 403, httpCode(t, err), "a Contributor may not fill somebody else's knowledge base")
	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), bob, sp.Space, useKB("kb-shared"), nil)
	assert.Equal(t, 403, httpCode(t, err), "nor one nobody owns, which is the administrators'")
	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), admin, sp.Space, useKB("kb-shared"), nil)
	assert.NoError(t, err, "a workspace administrator may")

	mine, err := e.svc.Spaces.Create(ctx(), alice, CreateSpaceInput{Name: "Alice's"})
	require.NoError(t, err)
	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), alice, mine.Space, useKB("kb-alice"), nil)
	assert.NoError(t, err, "its creator may")
	for _, id := range []string{"kb-faq", "kb-hidden", "kb-foreign", "kb-missing"} {
		_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), admin, mine.Space, useKB(id), nil)
		assert.Equal(t, 400, httpCode(t, err), id)
	}
}

// machine is an API key's identity as acl.MachineIdentity builds it, cut down
// to what the binding rules read.
func machine(allowed ...string) *acl.Identity {
	id := &acl.Identity{
		TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:7", TenantRole: types.TenantRoleAdmin,
		Member: true, Machine: true,
	}
	if len(allowed) > 0 {
		id.KnowledgeBases = map[string]bool{}
		for _, kb := range allowed {
			id.KnowledgeBases[kb] = true
		}
	}
	return id
}

func withKey(capabilities ...string) context.Context {
	return types.WithTenantAPIKeyScope(ctx(), types.TenantAPIKeyScope{KeyID: 7, Capabilities: capabilities})
}

func TestAnAPIKeyBindsWithinItsAllowListAndCreatesOnlyWithManageKBs(t *testing.T) {
	maker := &fakeMaker{}
	e := newEnv(t, func(d *Deps) {
		d.KnowledgeBaseMaker = maker
		d.KnowledgeBases = fakeKBs{"kb-1": 1, "kb-2": 1}
	})
	sp, err := e.svc.Spaces.Create(ctx(), e.identity("alice"), CreateSpaceInput{Name: "Handbook"})
	require.NoError(t, err)

	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), machine("kb-1"), sp.Space, useKB("kb-2"), nil)
	assert.Equal(t, 403, httpCode(t, err), "outside the key's allow-list")
	_, err = e.svc.Spaces.BindKnowledgeBase(ctx(), machine("kb-1"), sp.Space, useKB("kb-1"), nil)
	assert.NoError(t, err)

	_, err = e.svc.Spaces.BindKnowledgeBase(withKey("docs_admin"), machine(), sp.Space, createKB(), nil)
	assert.Equal(t, 403, httpCode(t, err), "creating a knowledge base is manage_kbs")
	_, err = e.svc.Spaces.BindKnowledgeBase(withKey("docs_admin", "manage_kbs"), machine("kb-1"), sp.Space,
		createKB(), nil)
	assert.Equal(t, 403, httpCode(t, err), "a key limited to some knowledge bases could not reach the new one")
	_, err = e.svc.Spaces.BindKnowledgeBase(withKey("docs_admin", "manage_kbs"), machine(), sp.Space, createKB(), nil)
	assert.NoError(t, err)
	made, _ := maker.snapshot()
	assert.Len(t, made, 1)
}

// A rebinding used to wait for the periodic sweep, which looks at a few hundred
// pages every five minutes; until it came, the pages of a space just taken out
// of a knowledge base could still be found there. Now the binding change queues
// the whole space at once, and draining the queue is all it takes.
func TestUnbindingRemovesTheMirroredPagesAtOnce(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	a := p.create(t, p.alice, nil, "页面一")
	p.write(t, p.alice, a.ID, "足够长的正文内容在这里。")
	b := p.create(t, p.alice, nil, "页面二")
	p.write(t, p.alice, b.ID, "另一段足够长的正文内容。")
	p.indexAll(t, a.ID, b.ID)
	require.Equal(t, 2, kb.inKB("kb-1"))
	require.NoError(t, p.drainIndex())
	require.Equal(t, 2, kb.inKB("kb-1"), "nothing queued before the binding changes")

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, noKB(), nil)
	require.NoError(t, err)
	require.NoError(t, p.drainIndex())

	assert.Zero(t, kb.count(), "the mirrors left the old knowledge base")
	assert.False(t, p.indexed(t, a.ID))
	assert.False(t, p.indexed(t, b.ID))
}

func TestRebindingMovesTheMirroredPagesAtOnce(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	page := p.create(t, p.alice, nil, "页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)
	require.NoError(t, p.drainIndex())

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, useKB("kb-2"), nil)
	require.NoError(t, err)
	require.NoError(t, p.drainIndex())

	assert.Equal(t, 0, kb.inKB("kb-1"), "gone from the old knowledge base")
	assert.Equal(t, 1, kb.inKB("kb-2"), "and made anew in the new one")
}

// Re-sending the same binding changes nothing, so it costs nothing.
func TestTheSameBindingAgainQueuesNothing(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	page := p.create(t, p.alice, nil, "页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)
	require.NoError(t, p.drainIndex())

	_, err := p.svc.Spaces.BindKnowledgeBase(ctx(), p.alice, p.space, useKB("kb-1"), nil)
	require.NoError(t, err)
	st, err := p.repos.IndexQueue.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	assert.Nil(t, st.DueAt)
	assert.Equal(t, 1, kb.inKB("kb-1"))
}

// Trashing a space takes its pages out of reach, so it takes their entries out
// of the knowledge base at once; restoring it brings them back.
func TestTrashingASpaceRemovesItsMirrorsAtOnceAndRestoringReturnsThem(t *testing.T) {
	p, kb := newTwoKBEnv(t)
	page := p.create(t, p.alice, nil, "页面")
	p.write(t, p.alice, page.ID, "足够长的正文内容在这里。")
	p.indexAll(t, page.ID)
	require.NoError(t, p.drainIndex())
	require.Equal(t, 1, kb.inKB("kb-1"))

	require.NoError(t, p.svc.Spaces.Delete(ctx(), p.alice, p.space))
	require.NoError(t, p.drainIndex())
	assert.Zero(t, kb.count())

	_, err := p.svc.Spaces.Restore(ctx(), p.identity("admin"), p.space.ID)
	require.NoError(t, err)
	require.NoError(t, p.drainIndex())
	assert.Equal(t, 1, kb.inKB("kb-1"))
}
