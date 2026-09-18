package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

// fakeTokens maps "token-<user>" to that user, the way the real token
// service maps a JWT to its subject.
type fakeTokens struct {
	members  *fakeMembers
	tenantID uint64
	inactive map[string]bool
}

func (f *fakeTokens) ValidateToken(_ context.Context, token string) (*types.User, uint64, error) {
	const prefix = "token-"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return nil, 0, fmt.Errorf("invalid token")
	}
	id := token[len(prefix):]
	user, ok := f.members.users[id]
	if !ok {
		return nil, 0, fmt.Errorf("unknown user")
	}
	copied := *user
	copied.IsActive = !f.inactive[id]
	return &copied, f.tenantID, nil
}

// recordingCollab captures the evictions the services request.
type recordingCollab struct {
	evicted chan string
}

func newRecordingCollab() *recordingCollab { return &recordingCollab{evicted: make(chan string, 64)} }

func (r *recordingCollab) Configured() bool { return true }

func (r *recordingCollab) Replace(context.Context, uint64, string, json.RawMessage,
	string,
) (*collab.ReplaceResult, error) {
	return nil, fmt.Errorf("replacing content arrives with the write-back path")
}

func (r *recordingCollab) Evict(_ context.Context, pageID string) error {
	r.evicted <- pageID
	return nil
}

func newCollabEnv(t *testing.T) (*pageEnv, *fakeTokens, *recordingCollab) {
	t.Helper()
	tokens := &fakeTokens{tenantID: 1, inactive: map[string]bool{}}
	client := newRecordingCollab()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Tokens = tokens
		d.Collab = client
	})
	tokens.members = p.members
	return p, tokens, client
}

func TestCollabAuthenticateResolvesAccess(t *testing.T) {
	p, tokens, _ := newCollabEnv(t)
	page := p.create(t, p.alice, nil, "Spec")

	// A writer gets a read-write connection with their display data.
	res, err := p.svc.Collab.Authenticate(ctx(), AuthenticateInput{Token: "token-bob", TenantID: 1, PageID: page.ID})
	require.NoError(t, err)
	require.Equal(t, AccessReadWrite, res.Access)
	require.Equal(t, "bob", res.UserID)
	require.Equal(t, "bob", res.DisplayName)
	require.Equal(t, "1", res.TenantID)
	require.Equal(t, p.space.ID, res.SpaceID)
	require.Equal(t, int64(0), res.YDocVersion)

	// A reader gets a read-only connection.
	res, err = p.svc.Collab.Authenticate(ctx(), AuthenticateInput{Token: "token-carol", TenantID: 1, PageID: page.ID})
	require.NoError(t, err)
	require.Equal(t, AccessReadOnly, res.Access)

	// A locked page is read-only even for writers, but not for space admins.
	require.NoError(t, p.repos.Pages.UpdateMeta(ctx(), 1, page.ID, map[string]any{"is_locked": true}))
	p.resolver.Invalidate(ctx(), 1)
	res, err = p.svc.Collab.Authenticate(ctx(), AuthenticateInput{Token: "token-bob", TenantID: 1, PageID: page.ID})
	require.NoError(t, err)
	require.Equal(t, AccessReadOnly, res.Access)
	res, err = p.svc.Collab.Authenticate(ctx(), AuthenticateInput{Token: "token-alice", TenantID: 1, PageID: page.ID})
	require.NoError(t, err)
	require.Equal(t, AccessReadWrite, res.Access, "space admins may edit a locked page")

	// Everything else is a refusal, with no hint about why.
	for _, in := range []AuthenticateInput{
		{Token: "garbage", TenantID: 1, PageID: page.ID},
		{Token: "token-viewer", TenantID: 1, PageID: page.ID},
		{Token: "token-bob", TenantID: 1, PageID: "no-such-page"},
		{Token: "token-bob", TenantID: 99, PageID: page.ID},
	} {
		_, err = p.svc.Collab.Authenticate(ctx(), in)
		require.ErrorIs(t, err, ErrCollabUnauthorized, "input %+v", in)
	}

	tokens.inactive["bob"] = true
	_, err = p.svc.Collab.Authenticate(ctx(), AuthenticateInput{Token: "token-bob", TenantID: 1, PageID: page.ID})
	require.ErrorIs(t, err, ErrCollabUnauthorized, "a disabled account cannot connect")
}

func TestCollabLoadReturnsYDocOrJSON(t *testing.T) {
	p, _, _ := newCollabEnv(t)
	page, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: p.space.ID, Title: "Guide", Markdown: "# Title\n\nbody text",
	})
	require.NoError(t, err)

	// No collaborative state yet: the JSON body is handed over instead.
	loaded, err := p.svc.Collab.Load(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.YDoc)
	require.Contains(t, string(loaded.Content), "body text")
	require.Equal(t, int64(0), loaded.YDocVersion)

	// After a store, the state is what comes back.
	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0, YDoc: []byte{1, 2, 3},
		Content: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`), EditorIDs: []string{"bob"},
	})
	require.NoError(t, err)
	loaded, err = p.svc.Collab.Load(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2, 3}, loaded.YDoc)
	require.Equal(t, int64(1), loaded.YDocVersion)

	_, err = p.svc.Collab.Load(ctx(), 1, "missing")
	require.ErrorIs(t, err, repository.ErrNotFound)
	_, err = p.svc.Collab.Load(ctx(), 2, page.ID)
	require.ErrorIs(t, err, repository.ErrNotFound, "tenants are isolated")
}

// Acceptance (T1.3): a store renders the document, records contributors and
// announces the change; a stale base version is a conflict the service can
// recover from; oversized states and invalid documents are refused.
func TestCollabPersistWritesBothRepresentations(t *testing.T) {
	p, _, _ := newCollabEnv(t)
	page := p.create(t, p.alice, nil, "Spec")
	doc := `{"type":"doc","content":[{"type":"heading","attrs":{"level":1},` +
		`"content":[{"type":"text","text":"Title"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"hello collaborative world"}]}]}`

	res, err := p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0, YDoc: []byte("yjs-state"),
		Content: json.RawMessage(doc), EditorIDs: []string{"bob", "bob", "carol"}, AwarenessCount: 2,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.YDocVersion)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("yjs-state"), stored.YDoc)
	require.Contains(t, stored.TextContent, "hello collaborative world")
	require.Equal(t, 4, stored.WordCount, "the heading and the paragraph together")
	require.Equal(t, "carol", *stored.LastEditorID, "the last editor of the batch is recorded")
	require.ElementsMatch(t, []string{"alice", "bob", "carol"}, []string(stored.ContributorIDs),
		"contributors accumulate and include the creator")
	require.NotNil(t, stored.ContentUpdatedAt)
	require.True(t, p.hasEvent(events.PageContent, func(ev events.Event) bool {
		return ev.PageID == page.ID && ev.SpaceID == p.space.ID
	}))

	// The same body again is not an edit: content_updated_at stays put.
	first := *stored.ContentUpdatedAt
	res, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 1, YDoc: []byte("yjs-state-2"),
		Content: json.RawMessage(doc), EditorIDs: []string{"bob"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), res.YDocVersion)
	stored, err = p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.True(t, stored.ContentUpdatedAt.Equal(first))

	// A stale base version conflicts; the collaboration service reloads and retries.
	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 1, YDoc: []byte("stale"),
		Content: json.RawMessage(doc),
	})
	require.ErrorIs(t, err, repository.ErrConflict)

	// Invalid documents never reach the database.
	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 2, YDoc: []byte("x"),
		Content: json.RawMessage(`{"type":"doc","content":[{"type":"notARealNode"}]}`),
	})
	require.Equal(t, 400, httpCode(t, err))

	// Oversized states are refused before any work is done.
	small := newPageEnvWith(t, func(d *Deps) { d.MaxYDocBytes = 16 })
	smallPage := small.create(t, small.alice, nil, "Big")
	_, err = small.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: smallPage.ID, BaseVersion: 0, YDoc: make([]byte, 64),
		Content: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`),
	})
	require.Equal(t, 400, httpCode(t, err))
	require.Contains(t, err.Error(), "split the page")

	// A locked page cannot be written even if a connection slipped through.
	require.NoError(t, p.repos.Pages.UpdateMeta(ctx(), 1, page.ID, map[string]any{"is_locked": true}))
	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 2, YDoc: []byte("x"), Content: json.RawMessage(doc),
	})
	require.Equal(t, 403, httpCode(t, err))
}

// Deleting a page must drop the people editing it, including in subpages.
func TestDeleteEvictsLiveConnections(t *testing.T) {
	p, _, client := newCollabEnv(t)
	root := p.create(t, p.alice, nil, "Root")
	child := p.create(t, p.alice, &root.ID, "Child")

	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, root.ID))
	require.NoError(t, err)

	evicted := map[string]bool{}
	for range 2 {
		select {
		case id := <-client.evicted:
			evicted[id] = true
		case <-t.Context().Done():
			t.Fatal("timed out waiting for evictions")
		}
	}
	require.True(t, evicted[root.ID])
	require.True(t, evicted[child.ID], "descendants are evicted too")
}
