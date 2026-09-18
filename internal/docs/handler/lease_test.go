package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/stretchr/testify/require"
)

// b64 encodes a stand-in Yjs state. The server never parses it.
func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func pmDoc(text string) map[string]any {
	return map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": text},
		}},
	}}
}

// leaseFixture builds a space with a writer (bob) and a reader (viewer), and
// one page for them to fight over, in a deployment with no collaboration
// service.
func leaseFixture(t *testing.T) (http.Handler, string) {
	t.Helper()
	return leaseFixtureWith(t)
}

// collaborativeLeaseFixture is the same space in a deployment that does run a
// collaboration service.
func collaborativeLeaseFixture(t *testing.T) (http.Handler, string) {
	t.Helper()
	return leaseFixtureWith(t, func(d *service.Deps) { d.CollabURL = "ws://collab:1234" })
}

func leaseFixtureWith(t *testing.T, opts ...func(*service.Deps)) (http.Handler, string) {
	t.Helper()
	r, _ := newSpaceRouter(t, opts...)
	sp := call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"name": "Handbook"})
	require.Equal(t, http.StatusCreated, sp.Code, sp.Body)
	sid := data(sp)["id"].(string)
	set := call(t, r, "alice", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{
		"members": []map[string]any{
			{"principal_type": "user", "principal_id": "bob", "role": "writer"},
			{"principal_type": "user", "principal_id": "viewer", "role": "reader"},
		},
	})
	require.Equal(t, http.StatusOK, set.Code, set.Body)
	page := call(t, r, "alice", http.MethodPost, "/docs/pages", map[string]any{"space_id": sid, "title": "Page"})
	require.Equal(t, http.StatusCreated, page.Code, page.Body)
	return r, data(page)["id"].(string)
}

// Acceptance (T1.5): in a deployment with no collaboration service, one
// person at a time holds the page; everyone else sees who that is and edits
// nothing.
func TestLeaseRoutesHandOverThePageBetweenEditors(t *testing.T) {
	r, pid := leaseFixture(t)

	free := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/lease", nil)
	require.Equal(t, http.StatusOK, free.Code, free.Body)
	require.Equal(t, false, data(free)["held"])

	taken := call(t, r, "alice", http.MethodPost, "/docs/pages/"+pid+"/lease",
		map[string]any{"session_id": "alice-tab"})
	require.Equal(t, http.StatusOK, taken.Code, taken.Body)
	require.Equal(t, true, data(taken)["held_by_me"])
	require.Equal(t, float64(60), data(taken)["renew_after_seconds"])

	// A reader can see who holds it, which is what the banner needs.
	seen := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/lease", nil)
	require.Equal(t, http.StatusOK, seen.Code)
	require.Equal(t, false, data(seen)["held_by_me"])
	holder := data(seen)["holder"].(map[string]any)
	require.Equal(t, "alice", holder["username"])

	// Another writer is told no, politely, with the holder's name.
	blocked := call(t, r, "bob", http.MethodPost, "/docs/pages/"+pid+"/lease",
		map[string]any{"session_id": "bob-tab"})
	require.Equal(t, http.StatusOK, blocked.Code, blocked.Body)
	require.Equal(t, false, data(blocked)["held_by_me"])

	// Alice writes, then hands the page back.
	save := call(t, r, "alice", http.MethodPut, "/docs/pages/"+pid+"/ydoc", map[string]any{
		"session_id": "alice-tab", "base_version": 0, "ydoc": b64("state-1"), "content": pmDoc("alice was here"),
	})
	require.Equal(t, http.StatusOK, save.Code, save.Body)
	require.Equal(t, float64(1), data(save)["ydoc_version"])

	released := call(t, r, "alice", http.MethodDelete,
		"/docs/pages/"+pid+"/lease?session_id=alice-tab", nil)
	require.Equal(t, http.StatusNoContent, released.Code)

	// Bob now takes it and loads exactly what she wrote.
	got := call(t, r, "bob", http.MethodPost, "/docs/pages/"+pid+"/lease", map[string]any{"session_id": "bob-tab"})
	require.Equal(t, http.StatusOK, got.Code, got.Body)
	require.Equal(t, true, data(got)["held_by_me"])

	state := call(t, r, "bob", http.MethodGet, "/docs/pages/"+pid+"/ydoc", nil)
	require.Equal(t, http.StatusOK, state.Code, state.Body)
	raw, err := base64.StdEncoding.DecodeString(data(state)["ydoc"].(string))
	require.NoError(t, err)
	require.Equal(t, "state-1", string(raw))
	require.Equal(t, float64(1), data(state)["ydoc_version"])
}

func TestLeaseRoutesEnforceRolesAndOwnership(t *testing.T) {
	r, pid := leaseFixture(t)

	// A reader may look but not take.
	require.Equal(t, http.StatusForbidden, call(t, r, "viewer", http.MethodPost,
		"/docs/pages/"+pid+"/lease", map[string]any{"session_id": "s"}).Code)
	// A stranger sees no page at all.
	require.Equal(t, http.StatusNotFound,
		call(t, r, "carol", http.MethodGet, "/docs/pages/"+pid+"/lease", nil).Code)
	// The session identifier is required.
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "alice", http.MethodPost, "/docs/pages/"+pid+"/lease", map[string]any{}).Code)

	held := call(t, r, "alice", http.MethodPost, "/docs/pages/"+pid+"/lease",
		map[string]any{"session_id": "alice-tab"})
	require.Equal(t, http.StatusOK, held.Code, held.Body)

	// Saving without the lease is refused rather than silently accepted.
	denied := call(t, r, "bob", http.MethodPut, "/docs/pages/"+pid+"/ydoc", map[string]any{
		"session_id": "bob-tab", "base_version": 0, "ydoc": b64("x"), "content": pmDoc("sneaky"),
	})
	require.Equal(t, http.StatusConflict, denied.Code, denied.Body)

	// And a stale base version is refused too, so a takeover cannot be undone.
	first := call(t, r, "alice", http.MethodPut, "/docs/pages/"+pid+"/ydoc", map[string]any{
		"session_id": "alice-tab", "base_version": 0, "ydoc": b64("one"), "content": pmDoc("one"),
	})
	require.Equal(t, http.StatusOK, first.Code, first.Body)
	stale := call(t, r, "alice", http.MethodPut, "/docs/pages/"+pid+"/ydoc", map[string]any{
		"session_id": "alice-tab", "base_version": 0, "ydoc": b64("two"), "content": pmDoc("two"),
	})
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body)

	// Releasing somebody else's lease does nothing but is not an error: a
	// closing tab should not have to know whether it still held the page.
	require.Equal(t, http.StatusNoContent,
		call(t, r, "bob", http.MethodDelete, "/docs/pages/"+pid+"/lease?session_id=bob-tab", nil).Code)
	after := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/lease", nil)
	require.Equal(t, true, data(after)["held"])
}

func TestLeaseReleaseAcceptsABodyAsWellAsAQueryString(t *testing.T) {
	r, pid := leaseFixture(t)
	require.Equal(t, http.StatusOK, call(t, r, "alice", http.MethodPost,
		"/docs/pages/"+pid+"/lease", map[string]any{"session_id": "alice-tab"}).Code)

	// The beacon a closing tab sends carries the session in the query string;
	// an ordinary request sends a body. Both must work.
	body := call(t, r, "alice", http.MethodDelete, "/docs/pages/"+pid+"/lease",
		map[string]any{"session_id": "alice-tab"})
	require.Equal(t, http.StatusNoContent, body.Code, body.Body)
	free := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/lease", nil)
	require.Equal(t, false, data(free)["held"])
}

func TestLeaseSaveRejectsAMalformedPayload(t *testing.T) {
	r, pid := leaseFixture(t)
	require.Equal(t, http.StatusOK, call(t, r, "alice", http.MethodPost,
		"/docs/pages/"+pid+"/lease", map[string]any{"session_id": "alice-tab"}).Code)

	for name, payload := range map[string]map[string]any{
		"no ydoc":    {"session_id": "alice-tab", "content": pmDoc("x")},
		"no content": {"session_id": "alice-tab", "ydoc": b64("x")},
		"no session": {"ydoc": b64("x"), "content": pmDoc("x")},
		"bad base64": {"session_id": "alice-tab", "ydoc": "!!!", "content": pmDoc("x")},
		"bad node": {
			"session_id": "alice-tab", "ydoc": b64("x"),
			"content": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "nope"}}},
		},
		"not a doc":   {"session_id": "alice-tab", "ydoc": b64("x"), "content": map[string]any{"type": "paragraph"}},
		"empty ydoc":  {"session_id": "alice-tab", "ydoc": "", "content": pmDoc("x")},
		"null fields": {"session_id": nil, "ydoc": nil, "content": nil},
	} {
		t.Run(name, func(t *testing.T) {
			res := call(t, r, "alice", http.MethodPut, "/docs/pages/"+pid+"/ydoc", payload)
			require.Equal(t, http.StatusBadRequest, res.Code, res.Body)
		})
	}
}

// The exclusive-edit routes must be inert wherever a collaboration service
// merges edits instead, so a client that gets the mode wrong finds out at
// once instead of losing somebody's work.
func TestLeaseRoutesRefusedWhenACollaborationServiceIsConfigured(t *testing.T) {
	r, pid := collaborativeLeaseFixture(t)

	require.Equal(t, http.StatusConflict,
		call(t, r, "alice", http.MethodGet, "/docs/pages/"+pid+"/lease", nil).Code)
	require.Equal(t, http.StatusConflict, call(t, r, "alice", http.MethodPost,
		"/docs/pages/"+pid+"/lease", map[string]any{"session_id": "s"}).Code)
	require.Equal(t, http.StatusConflict, call(t, r, "alice", http.MethodDelete,
		"/docs/pages/"+pid+"/lease?session_id=s", nil).Code)
	require.Equal(t, http.StatusConflict,
		call(t, r, "alice", http.MethodGet, "/docs/pages/"+pid+"/ydoc", nil).Code)
	require.Equal(t, http.StatusConflict, call(t, r, "alice", http.MethodPut,
		"/docs/pages/"+pid+"/ydoc", map[string]any{
			"session_id": "s", "ydoc": b64("x"), "content": pmDoc("x"),
		}).Code)
}

// A page with no Yjs state yet hands back its stored body so the client can
// build one; this is how a page written by an import or by page creation
// becomes editable.
func TestLeaseLoadYDocFallsBackToTheStoredBody(t *testing.T) {
	r, pid := leaseFixture(t)
	state := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/ydoc", nil)
	require.Equal(t, http.StatusOK, state.Code, state.Body)
	require.Empty(t, data(state)["ydoc"])
	content, err := json.Marshal(data(state)["content"])
	require.NoError(t, err)
	require.Contains(t, string(content), `"type":"doc"`)
	require.Equal(t, float64(0), data(state)["ydoc_version"])
}
