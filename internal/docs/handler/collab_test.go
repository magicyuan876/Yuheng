package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

const collabSecret = "a-shared-secret-of-sufficient-length"

// tokenTable turns "token-<user>" into that user, like the real token service.
type tokenTable map[string]types.TenantRole

func (t tokenTable) ValidateToken(_ context.Context, token string) (*types.User, uint64, error) {
	const prefix = "token-"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return nil, 0, fmt.Errorf("invalid token")
	}
	id := token[len(prefix):]
	if _, ok := t[fmt.Sprintf("1/%s", id)]; !ok {
		return nil, 0, fmt.Errorf("unknown user")
	}
	return &types.User{ID: id, Username: id, Email: id + "@example.test", IsActive: true}, 1, nil
}

// newCollabRouter wires the internal callbacks exactly as the router does,
// on top of a private PostgreSQL database with one space and one page.
func newCollabRouter(t *testing.T) (*gin.Engine, *repository.Repositories, *model.Page) {
	t.Helper()
	db := openHandlerDB(t)
	repos := repository.New(db)
	members := tenantTable{
		"1/alice": types.TenantRoleContributor, "1/bob": types.TenantRoleContributor,
		"1/viewer": types.TenantRoleViewer,
	}
	tokens := tokenTable(members)
	resolver := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(members), acl.WithCache(acl.NewMemoryCache(0)))
	services := service.New(service.Deps{
		Repos: repos, Resolver: resolver, Bus: events.NewMemoryBus(), Audit: audit.NewRecorder(nil),
		Users: members, Members: members, Tokens: tokens, MaxYDocBytes: 1024,
	})
	h := New(Deps{
		Repos: repos, Resolver: resolver, Guard: acl.NewGuard(resolver), Services: services,
		Config: &config.DocsConfig{CollabSharedSecret: collabSecret, MaxYDocBytes: 1024},
	})
	require.True(t, h.Collab.Enabled())

	space := &model.Space{
		TenantID: 1, Slug: "handbook", Name: "Handbook", Visibility: model.VisibilityOpen,
		DefaultRole: model.RoleWriter,
	}
	require.NoError(t, repos.Spaces.Create(t.Context(), space))
	page := &model.Page{
		TenantID: 1, SpaceID: space.ID, ShortID: "collab0001", Title: "Spec", Position: "a0",
		Content: model.JSON(`{"type":"doc","content":[{"type":"paragraph"}]}`),
	}
	require.NoError(t, repos.Pages.Create(t.Context(), page))

	r := gin.New()
	internal := r.Group("/internal/collab")
	internal.POST("/authenticate", h.Collab.Authenticate)
	internal.GET("/load/:pid", h.Collab.Load)
	internal.POST("/store", h.Collab.Store)
	internal.GET("/health", h.Collab.Health)
	return r, repos, page
}

// signedCall performs a request the way the collaboration service does.
func signedCall(t *testing.T, r http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	collab.SignRequest(req, collabSecret, payload, time.Now())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// Acceptance (T1.3): the callbacks are reachable only with a valid signature
// and answer with the codes the collaboration service acts on.
func TestCollabCallbacksRequireASignature(t *testing.T) {
	r, _, page := newCollabRouter(t)

	// Unsigned, mis-signed and stale requests are all refused.
	unsigned := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, unsigned)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	wrong := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	collab.SignRequest(wrong, "another-secret-of-sufficient-length", nil, time.Now())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, wrong)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	stale := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	collab.SignRequest(stale, collabSecret, nil, time.Now().Add(-collab.MaxSkew-time.Minute))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, stale)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// A body that differs from the one signed is refused too.
	signedBody, err := json.Marshal(map[string]any{"token": "token-alice", "page_id": page.ID, "tenant_id": "1"})
	require.NoError(t, err)
	tampered := httptest.NewRequest(http.MethodPost, "/internal/collab/authenticate", bytes.NewReader(signedBody))
	collab.SignRequest(tampered, collabSecret, append(signedBody, ' '), time.Now())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, tampered)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	require.Equal(t, http.StatusOK, signedCall(t, r, http.MethodGet, "/internal/collab/health", nil).Code)
}

func TestCollabAuthenticateAndLoadOverHTTP(t *testing.T) {
	r, _, page := newCollabRouter(t)

	rec := signedCall(t, r, http.MethodPost, "/internal/collab/authenticate",
		map[string]any{"token": "token-alice", "page_id": page.ID, "tenant_id": "1", "connection_id": "c1"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	require.Equal(t, "readwrite", body["access"])
	require.Equal(t, "alice", body["user_id"])
	require.Equal(t, page.SpaceID, body["space_id"])

	// A workspace viewer may read but not write.
	rec = signedCall(t, r, http.MethodPost, "/internal/collab/authenticate",
		map[string]any{"token": "token-viewer", "page_id": page.ID, "tenant_id": "1"})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "readonly", decode(t, rec)["access"])

	// An unknown token closes the socket.
	rec = signedCall(t, r, http.MethodPost, "/internal/collab/authenticate",
		map[string]any{"token": "nonsense", "page_id": page.ID, "tenant_id": "1"})
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Load hands over the JSON body while there is no collaborative state.
	rec = signedCall(t, r, http.MethodGet, "/internal/collab/load/"+page.ID+"?tenant=1", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	require.Equal(t, "0", rec.Header().Get("X-YDoc-Version"))
	require.NotNil(t, decode(t, rec)["content"])

	rec = signedCall(t, r, http.MethodGet, "/internal/collab/load/"+page.ID, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, "the workspace must be named")
	rec = signedCall(t, r, http.MethodGet, "/internal/collab/load/missing?tenant=1", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCollabStoreOverHTTP(t *testing.T) {
	r, repos, page := newCollabRouter(t)
	doc := map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": "persisted through the callback"},
		}},
	}}
	store := func(base int64, state []byte, content any) *httptest.ResponseRecorder {
		return signedCall(t, r, http.MethodPost, "/internal/collab/store", map[string]any{
			"tenant_id": "1", "page_id": page.ID, "base_version": base,
			"ydoc": base64.StdEncoding.EncodeToString(state), "content": content,
			"editor_ids": []string{"alice"}, "awareness_count": 1,
		})
	}

	rec := store(0, []byte("yjs"), doc)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, float64(1), decode(t, rec)["ydoc_version"])

	stored, err := repos.Pages.Get(t.Context(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("yjs"), stored.YDoc)
	require.Contains(t, stored.TextContent, "persisted through the callback")

	// Loading now returns the binary state with its version.
	rec = signedCall(t, r, http.MethodGet, "/internal/collab/load/"+page.ID+"?tenant=1", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "yjs", rec.Body.String())
	require.Equal(t, "1", rec.Header().Get("X-YDoc-Version"))

	// A stale base version is a conflict the service knows how to recover from.
	rec = store(0, []byte("stale"), doc)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, "conflict", decode(t, rec)["code"])

	// An invalid document is refused without touching the page.
	rec = store(1, []byte("x"), map[string]any{"type": "doc", "content": []any{map[string]any{"type": "bogus"}}})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "rejected", decode(t, rec)["code"])

	// So is a state over the configured limit.
	rec = store(1, bytes.Repeat([]byte("x"), 2048), doc)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Malformed input is rejected before any work happens.
	rec = signedCall(t, r, http.MethodPost, "/internal/collab/store", map[string]any{
		"tenant_id": "1", "page_id": page.ID, "ydoc": "not base64!!", "content": doc,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = signedCall(t, r, http.MethodPost, "/internal/collab/store", map[string]any{
		"page_id": page.ID, "ydoc": "", "content": doc,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	after, err := repos.Pages.Get(t.Context(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), after.YDocVersion, "no rejected call changed the page")
}

// Without a shared secret the routes are not wired at all, so a deployment
// that forgets to configure one cannot be probed.
func TestCollabHandlerIsDisabledWithoutASecret(t *testing.T) {
	var h *CollabHandler
	require.False(t, h.Enabled())
	require.False(t, NewCollabHandler(nil, "", 0).Enabled())

	built := New(Deps{Config: &config.DocsConfig{CollabSharedSecret: ""}})
	require.Nil(t, built.Collab)
	require.False(t, built.Collab.Enabled())
}
