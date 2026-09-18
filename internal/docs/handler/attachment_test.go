package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// memStore is a storage backend in memory, enough for the HTTP layer's tests.
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	seq     int
	signed  bool
}

func newMemStore() *memStore { return &memStore{objects: map[string][]byte{}} }

func (m *memStore) CheckConnectivity(context.Context) error { return nil }

func (m *memStore) put(name string, data []byte) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	path := fmt.Sprintf("mem://%d/%s", m.seq, name)
	m.objects[path] = append([]byte(nil), data...)
	return path
}

func (m *memStore) SaveFile(_ context.Context, fh *multipart.FileHeader, _ uint64, _ string) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return m.put(fh.Filename, data), nil
}

func (m *memStore) SaveBytes(_ context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	return m.put(name, data), nil
}

func (m *memStore) GetFile(_ context.Context, path string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[path]
	if !ok {
		return nil, fmt.Errorf("no such object")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memStore) GetFileURL(_ context.Context, path string) (string, error) {
	if !m.signed {
		return "", fmt.Errorf("no URLs here")
	}
	return "https://cdn.example.test/" + path, nil
}

func (m *memStore) DeleteFile(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, path)
	return nil
}

func (m *memStore) CopyFile(context.Context, string, uint64, string) (string, error) {
	return "", fmt.Errorf("not needed here")
}

type memResolver struct{ store *memStore }

func (r memResolver) ResolveFileService(context.Context, *types.Tenant, string, string,
	string,
) (interfaces.FileService, string, error) {
	return r.store, "mem", nil
}

type memTenants struct{ used int64 }

func (m *memTenants) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id, StorageUsed: m.used}, nil
}

func (m *memTenants) AdjustStorageUsed(_ context.Context, _ uint64, delta int64) error {
	m.used += delta
	return nil
}

// attachRouter builds the space, its members and one page, with storage wired.
func attachRouter(t *testing.T, signed bool) (http.Handler, *memStore, string, string) {
	t.Helper()
	store := newMemStore()
	store.signed = signed
	r, _ := newSpaceRouter(t, func(d *service.Deps) {
		d.Storage = memResolver{store: store}
		d.Tenants = &memTenants{}
	})
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
	page := call(t, r, "alice", http.MethodPost, "/docs/pages",
		map[string]any{"space_id": sid, "title": "Page"})
	require.Equal(t, http.StatusCreated, page.Code, page.Body)
	return r, store, sid, data(page)["id"].(string)
}

// postFile sends one multipart upload the way a browser would.
func postFile(t *testing.T, r http.Handler, user, path, name string, content []byte,
	fields map[string]string,
) resp {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Test-User", user)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	out := resp{Code: rec.Code}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
	}
	return out
}

// raw performs a request and returns the recorder, for the header assertions
// the JSON helper cannot make.
func raw(t *testing.T, r http.Handler, user, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Test-User", user)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func smallPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestAttachmentRoutesUploadReadAndDelete(t *testing.T) {
	r, store, sid, pid := attachRouter(t, false)
	content := smallPNG(t, 12, 8)

	up := postFile(t, r, "bob", "/docs/spaces/"+sid+"/attachments", "photo.png", content,
		map[string]string{"page_id": pid})
	require.Equal(t, http.StatusCreated, up.Code, up.Body)
	aid := data(up)["id"].(string)
	require.Equal(t, "image/png", data(up)["mime"])
	require.Equal(t, pid, data(up)["page_id"])
	require.Equal(t, float64(12), data(up)["width"])

	// The page lists it.
	list := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/attachments", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body)
	require.Len(t, list.Body["data"], 1)

	// An image is served inline, under its real type, with sniffing disabled.
	rec := raw(t, r, "viewer", http.MethodGet, "/docs/attachments/"+aid)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "inline")
	require.Equal(t, content, rec.Body.Bytes())

	// A reader cannot delete it; a writer can.
	require.Equal(t, http.StatusForbidden,
		raw(t, r, "viewer", http.MethodDelete, "/docs/attachments/"+aid).Code)
	require.Equal(t, http.StatusNoContent,
		raw(t, r, "bob", http.MethodDelete, "/docs/attachments/"+aid).Code)
	require.Equal(t, http.StatusNotFound,
		raw(t, r, "viewer", http.MethodGet, "/docs/attachments/"+aid).Code)
	require.Empty(t, store.objects)
}

// Acceptance (T1.6): the URL alone gets a stranger nothing.
func TestAttachmentRoutesRefuseSomebodyOutsideTheSpace(t *testing.T) {
	r, _, sid, pid := attachRouter(t, false)
	up := postFile(t, r, "alice", "/docs/spaces/"+sid+"/attachments", "photo.png",
		smallPNG(t, 8, 8), map[string]string{"page_id": pid})
	require.Equal(t, http.StatusCreated, up.Code, up.Body)
	aid := data(up)["id"].(string)

	require.Equal(t, http.StatusNotFound,
		raw(t, r, "carol", http.MethodGet, "/docs/attachments/"+aid).Code)
	// A reader of the space may not upload at all.
	require.Equal(t, http.StatusForbidden,
		postFile(t, r, "viewer", "/docs/spaces/"+sid+"/attachments", "x.png",
			smallPNG(t, 4, 4), nil).Code)
}

// A file a browser would execute is downloaded as opaque bytes, whatever the
// uploader called it.
func TestAttachmentDownloadNeverServesUserContentAsADocument(t *testing.T) {
	r, _, sid, _ := attachRouter(t, false)
	page := []byte("<html><body><script>alert(document.domain)</script></body></html>")

	up := postFile(t, r, "alice", "/docs/spaces/"+sid+"/attachments", "picture.png", page, nil)
	require.Equal(t, http.StatusCreated, up.Code, up.Body)
	aid := data(up)["id"].(string)

	rec := raw(t, r, "alice", http.MethodGet, "/docs/attachments/"+aid)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

func TestAttachmentDownloadSandboxesASanitisedSVG(t *testing.T) {
	r, _, sid, _ := attachRouter(t, false)
	drawing := []byte(`<svg xmlns="http://www.w3.org/2000/svg">` +
		`<script>alert(1)</script><circle r="5"/></svg>`)

	up := postFile(t, r, "alice", "/docs/spaces/"+sid+"/attachments", "d.svg", drawing, nil)
	require.Equal(t, http.StatusCreated, up.Code, up.Body)

	rec := raw(t, r, "alice", http.MethodGet, "/docs/attachments/"+data(up)["id"].(string))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox")
	require.NotContains(t, rec.Body.String(), "<script")
}

func TestAttachmentDownloadRedirectsToASignedURLForImages(t *testing.T) {
	r, _, sid, _ := attachRouter(t, true)
	up := postFile(t, r, "alice", "/docs/spaces/"+sid+"/attachments", "photo.png",
		smallPNG(t, 8, 8), nil)
	require.Equal(t, http.StatusCreated, up.Code, up.Body)

	rec := raw(t, r, "alice", http.MethodGet, "/docs/attachments/"+data(up)["id"].(string))
	require.Equal(t, http.StatusFound, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "cdn.example.test")
}

func TestAttachmentDownloadServesTheRequestedWidth(t *testing.T) {
	r, _, sid, _ := attachRouter(t, false)
	up := postFile(t, r, "alice", "/docs/spaces/"+sid+"/attachments", "wide.png",
		smallPNG(t, 900, 300), nil)
	require.Equal(t, http.StatusCreated, up.Code, up.Body)
	aid := data(up)["id"].(string)

	rec := raw(t, r, "alice", http.MethodGet, "/docs/attachments/"+aid+"?w=320")
	require.Equal(t, http.StatusOK, rec.Code)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	require.Equal(t, 320, cfg.Width)

	// A width nobody offers quietly yields the original rather than an error.
	rec = raw(t, r, "alice", http.MethodGet, "/docs/attachments/"+aid+"?w=999")
	require.Equal(t, http.StatusOK, rec.Code)
	cfg, _, err = image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	require.Equal(t, 900, cfg.Width)
}

func TestAttachmentUploadWithoutAFileIsABadRequest(t *testing.T) {
	r, _, sid, _ := attachRouter(t, false)
	req := httptest.NewRequest(http.MethodPost, "/docs/spaces/"+sid+"/attachments", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=zzz")
	req.Header.Set("X-Test-User", "alice")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
