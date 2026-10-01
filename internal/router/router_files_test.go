package router

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

const (
	testRef    = "resource://AbCdEfGhIjKlMnOpQrStUv"
	testHandle = "AbCdEfGhIjKlMnOpQrStUv"
)

// stubResourceCatalog knows one resource; every reference to its handle
// resolves to it, every other one is unknown.
type stubResourceCatalog struct {
	resource *types.StoredResource
}

func (s *stubResourceCatalog) Register(
	context.Context, uint64, string, string, interfaces.ResourceRegistration,
) (string, error) {
	panic("unexpected Register")
}

func (s *stubResourceCatalog) Resolve(_ context.Context, ref string) (*types.StoredResource, error) {
	if s.resource == nil || ref != types.BuildResourcePath(s.resource.Handle) {
		return nil, types.ErrResourceNotFound
	}
	return s.resource, nil
}

func (s *stubResourceCatalog) ResolvePath(context.Context, string) (string, *types.StoredResource, error) {
	panic("unexpected ResolvePath")
}

func (s *stubResourceCatalog) Bind(context.Context, string, string, string, string) error {
	panic("unexpected Bind")
}

func (s *stubResourceCatalog) MarkDeleted(context.Context, string) error {
	panic("unexpected MarkDeleted")
}

func (s *stubResourceCatalog) CreateAccessGrant(context.Context, string, time.Duration) (string, error) {
	panic("unexpected CreateAccessGrant")
}

func (s *stubResourceCatalog) ResolveAccessGrant(context.Context, string) (*types.StoredResource, error) {
	if s.resource == nil {
		return nil, types.ErrResourceNotFound
	}
	return s.resource, nil
}

// stubFileStore serves fixed bodies by reference and records what was read.
type stubFileStore struct {
	interfaces.FileStore
	bodies map[string]string
	opened []string
	// catalog supplies the resource Open returns, as the real store does.
	catalog *stubResourceCatalog
	// url is what URL answers; empty means no public URL exists.
	url string
}

func (s *stubFileStore) Open(ctx context.Context, ref string) (io.ReadCloser, *types.StoredResource, error) {
	s.opened = append(s.opened, ref)
	body, ok := s.bodies[ref]
	if !ok {
		return nil, nil, errors.New("no such object")
	}
	resource, err := s.catalog.Resolve(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	return io.NopCloser(strings.NewReader(body)), resource, nil
}

func (s *stubFileStore) URL(context.Context, string, time.Duration) (string, bool, error) {
	return s.url, s.url != "", nil
}

type stubMessageFileLookup struct {
	get func(ctx context.Context, sessionID, messageID string) (*types.Message, error)
}

func (s *stubMessageFileLookup) GetMessage(ctx context.Context, sessionID, messageID string) (*types.Message, error) {
	return s.get(ctx, sessionID, messageID)
}

// newStubStorage returns a catalog holding one resource of tenantID at
// physical, and a store serving body for it.
func newStubStorage(tenantID uint64, physical, name, body string) (*stubFileStore, *stubResourceCatalog) {
	catalog := &stubResourceCatalog{resource: &types.StoredResource{
		ID: "resource-1", Handle: testHandle, TenantID: tenantID, PhysicalPath: physical, OriginalName: name,
	}}
	return &stubFileStore{bodies: map[string]string{testRef: body}, catalog: catalog}, catalog
}

func filesRequest(t *testing.T, engine *gin.Engine, target string, tenantID uint64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if tenantID != 0 {
		ctx := context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: tenantID})
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
		req = req.WithContext(ctx)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func TestServeFilesReadsOwnResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/a.png", "a.png", "image")
	engine := gin.New()
	serveFiles(engine, files, catalog)

	recorder := filesRequest(t, engine, "/files?file_path="+url.QueryEscape(testRef), 42)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "image" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
}

func TestServeFilesRejectsCrossTenantResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(7, "local://7/exports/a.png", "a.png", "image")
	engine := gin.New()
	serveFiles(engine, files, catalog)

	recorder := filesRequest(t, engine, "/files?file_path="+url.QueryEscape(testRef), 42)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if len(files.opened) != 0 {
		t.Fatalf("a forbidden resource must not be read, opened %v", files.opened)
	}
}

// A storage locator is internal; the proxy accepts handles and nothing else,
// so no client can name an object the catalog does not vouch for.
func TestServeFilesRejectsRawStorageLocators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/a.png", "a.png", "image")
	engine := gin.New()
	serveFiles(engine, files, catalog)

	for _, raw := range []string{
		"local://42/exports/a.png",
		"s3://bucket/yuheng/42/exports/a.png",
		"storage://backend-1/local://42/exports/a.png",
	} {
		recorder := filesRequest(t, engine, "/files?file_path="+url.QueryEscape(raw), 42)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want %d", raw, recorder.Code, http.StatusBadRequest)
		}
	}
	if len(files.opened) != 0 {
		t.Fatalf("raw locators must not be read, opened %v", files.opened)
	}
}

func TestServeFilesUnknownResourceIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/a.png", "a.png", "image")
	engine := gin.New()
	serveFiles(engine, files, catalog)

	recorder := filesRequest(t, engine, "/files?file_path="+url.QueryEscape("resource://ZZZZZZZZZZZZZZZZZZZZZZ"), 42)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestServeFilesForcesActiveContentDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/k1/payload.svg", "payload.svg", `<svg onload="alert(1)"></svg>`)
	engine := gin.New()
	serveFiles(engine, files, catalog)

	recorder := filesRequest(t, engine, "/files?file_path="+url.QueryEscape(testRef), 42)
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != "attachment" {
		t.Fatalf("Content-Disposition = %q, want attachment", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// /files carries its own API-key guard (middleware.AllowFileServeAPIKey):
// full-access and tenant-wide retrieve keys may read the tenant's resources,
// but KB-restricted keys (and keys lacking retrieve) are denied because an
// arbitrary resource cannot be bounded to a KB allow-list.
func TestServeFilesAPIKeyScopeMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name     string
		scope    types.TenantAPIKeyScope
		wantCode int
	}{
		{name: "full access allowed", scope: types.TenantAPIKeyScope{FullAccess: true}, wantCode: http.StatusOK},
		{
			name: "tenant-wide retrieve allowed",
			scope: types.TenantAPIKeyScope{
				Capabilities: types.StringArray{string(types.APIKeyCapabilityRetrieve)},
			},
			wantCode: http.StatusOK,
		},
		{
			name: "kb-restricted retrieve denied",
			scope: types.TenantAPIKeyScope{
				KnowledgeBaseIDs: types.StringArray{"kb-1"},
				Capabilities:     types.StringArray{string(types.APIKeyCapabilityRetrieve)},
			},
			wantCode: http.StatusForbidden,
		},
		{
			name: "non-retrieve capability denied",
			scope: types.TenantAPIKeyScope{
				Capabilities: types.StringArray{string(types.APIKeyCapabilityChat)},
			},
			wantCode: http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, catalog := newStubStorage(42, "local://42/exports/a.txt", "a.txt", "body")
			engine := gin.New()
			serveFiles(engine, files, catalog)

			req := httptest.NewRequest(http.MethodGet, "/files?file_path="+url.QueryEscape(testRef), nil)
			ctx := context.WithValue(req.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 42})
			ctx = types.WithTenantAPIKeyScope(ctx, tc.scope)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req.WithContext(ctx))
			if got := recorder.Code; got != tc.wantCode {
				t.Fatalf("status = %d, want %d body=%s", got, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

func TestResourceGrantServesShortPublicURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/a.png", "a.png", "image")
	catalog.resource.MimeType = "image/png"
	engine := gin.New()
	serveResourceGrants(engine, files, catalog)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/GrantTokenAbCdEfGhIjKlM", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "image" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}

	// HEAD answers with headers only, but still proves the object exists.
	head := httptest.NewRecorder()
	engine.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/r/GrantTokenAbCdEfGhIjKlM", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD status=%d body=%q", head.Code, head.Body.String())
	}
}

// newKBScopedFilesTestEngine wires newKBScopedFileServeHandler behind a
// middleware that injects effectiveTenantID into the request context, mirroring
// what RequireKBAccess does after resolving an org-shared KB to its source
// tenant. This lets the handler be exercised without the full RBAC stack.
func newKBScopedFilesTestEngine(
	effectiveTenantID uint64, files interfaces.FileStore, catalog interfaces.ResourceCatalog,
) *gin.Engine {
	engine := gin.New()
	engine.GET("/knowledge-bases/:id/files",
		func(c *gin.Context) {
			ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, effectiveTenantID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		},
		newKBScopedFileServeHandler(files, catalog),
	)
	return engine
}

// A borrowing tenant renders the owner's (10008) embedded image through a
// shared KB: the effective tenant in context is the owner, the resource is
// the owner's and lives in its exports namespace, so it is served.
func TestKBScopedFilesServesOwnerExportsResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(10008, "s3://bucket/yuheng/10008/exports/img.jpg", "img.jpg", "shared-body")
	engine := newKBScopedFilesTestEngine(10008, files, catalog)

	recorder := filesRequest(t, engine, "/knowledge-bases/kb-1/files?file_path="+url.QueryEscape(testRef), 0)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "shared-body" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); !strings.HasPrefix(got, "private") {
		t.Fatalf("Cache-Control = %q, shared content must not be publicly cacheable", got)
	}
}

// The resource must belong to the effective (owner) tenant, so the guard
// cannot be used to reach arbitrary tenants' files.
func TestKBScopedFilesRejectsResourceOfAnotherTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(9999, "local://9999/exports/other.jpg", "other.jpg", "x")
	engine := newKBScopedFilesTestEngine(10008, files, catalog)

	recorder := filesRequest(t, engine, "/knowledge-bases/kb-1/files?file_path="+url.QueryEscape(testRef), 0)
	if recorder.Code != http.StatusForbidden || len(files.opened) != 0 {
		t.Fatalf("status = %d opened=%v, want 403 and no read", recorder.Code, files.opened)
	}
}

// The KB proxy serves embedded exports/ images only; a borrower must not be
// able to download the owner's raw uploads through it.
func TestKBScopedFilesRejectsRawUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(10008, "local://10008/knowledge-id/123.pdf", "123.pdf", "x")
	engine := newKBScopedFilesTestEngine(10008, files, catalog)

	recorder := filesRequest(t, engine, "/knowledge-bases/kb-1/files?file_path="+url.QueryEscape(testRef), 0)
	if recorder.Code != http.StatusForbidden || len(files.opened) != 0 {
		t.Fatalf("status = %d opened=%v, want 403 and no read", recorder.Code, files.opened)
	}
}

func TestKBScopedFilesRequiresFilePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(10008, "local://10008/exports/a.png", "a.png", "x")
	engine := newKBScopedFilesTestEngine(10008, files, catalog)

	recorder := filesRequest(t, engine, "/knowledge-bases/kb-1/files", 0)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func newMessageScopedFilesTestEngine(
	callerTenantID uint64,
	messageService messageFileLookup,
	files interfaces.FileStore,
	catalog interfaces.ResourceCatalog,
) *gin.Engine {
	engine := gin.New()
	engine.GET("/sessions/:id/messages/:message_id/files",
		func(c *gin.Context) {
			ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, callerTenantID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		},
		newMessageScopedFileServeHandler(messageService, files, catalog),
	)
	return engine
}

func ownedMessage(t *testing.T) *stubMessageFileLookup {
	return &stubMessageFileLookup{get: func(_ context.Context, sessionID, messageID string) (*types.Message, error) {
		if sessionID != "session-1" || messageID != "message-1" {
			t.Fatalf("unexpected message scope %s/%s", sessionID, messageID)
		}
		return &types.Message{}, nil
	}}
}

// A reply may cite an image owned by another workspace (an organization-shared
// knowledge base); the resource row says where it lives.
func TestMessageScopedFilesServesCrossTenantResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(7, "local://7/exports/chart.png", "chart.png", "cross-tenant-image")
	engine := newMessageScopedFilesTestEngine(42, ownedMessage(t), files, catalog)

	recorder := filesRequest(t, engine,
		"/sessions/session-1/messages/message-1/files?file_path="+url.QueryEscape(testRef), 0)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "cross-tenant-image" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestMessageScopedFilesHidesForeignMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/chart.png", "chart.png", "x")
	lookup := &stubMessageFileLookup{get: func(context.Context, string, string) (*types.Message, error) {
		return nil, errors.New("not found")
	}}
	engine := newMessageScopedFilesTestEngine(42, lookup, files, catalog)

	recorder := filesRequest(t, engine,
		"/sessions/session-1/messages/message-1/files?file_path="+url.QueryEscape(testRef), 0)
	if recorder.Code != http.StatusNotFound || len(files.opened) != 0 {
		t.Fatalf("status = %d opened=%v, want 404 and no read", recorder.Code, files.opened)
	}
}

func TestMessageScopedFilesRequiresFilePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	files, catalog := newStubStorage(42, "local://42/exports/chart.png", "chart.png", "x")
	engine := newMessageScopedFilesTestEngine(42, ownedMessage(t), files, catalog)

	recorder := filesRequest(t, engine, "/sessions/session-1/messages/message-1/files", 0)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
