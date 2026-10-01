package service_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/service"
	filesvc "github.com/magicyuan876/yuheng/internal/application/service/file"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fileStoreFixture is a real file store over a fresh database, with local
// storage rooted in a temporary directory.
type fileStoreFixture struct {
	db       *gorm.DB
	store    interfaces.FileStore
	backends interfaces.StorageBackendRepository
	baseDir  string
}

// newFileStoreFixture builds the store. Environment that the store reads at
// construction (APP_EXTERNAL_URL, DOCREADER_SHARED_DATA_DIR) must be set by the
// caller before this runs.
func newFileStoreFixture(t *testing.T) *fileStoreFixture {
	t.Helper()
	baseDir := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", baseDir)
	db := pgtest.New(t)
	backends := repository.NewStorageBackendRepository(db)
	catalog := service.NewResourceCatalog(repository.NewResourceRepository(db))
	return &fileStoreFixture{
		db:       db,
		store:    service.NewFileStore(backends, catalog, repository.NewTenantRepository(db)),
		backends: backends,
		baseDir:  baseDir,
	}
}

// localBackend registers a local backend of tenantID under pathPrefix.
func (f *fileStoreFixture) localBackend(t *testing.T, tenantID uint64, name, pathPrefix string) *types.StorageBackend {
	t.Helper()
	backend := &types.StorageBackend{
		TenantID: tenantID, Name: name, Provider: types.StorageProviderLocal,
		Config: types.StorageBackendConfig{PathPrefix: pathPrefix},
	}
	require.NoError(t, f.backends.Create(context.Background(), backend))
	return backend
}

func (f *fileStoreFixture) save(t *testing.T, backendID string, tenantID uint64, name, body string) string {
	t.Helper()
	writer, err := f.store.Writer(context.Background(), backendID)
	require.NoError(t, err)
	ref, err := writer.SaveBytes(context.Background(), []byte(body), tenantID, name, false)
	require.NoError(t, err)
	return ref
}

func readAll(t *testing.T, store interfaces.FileStore, ref string) string {
	t.Helper()
	reader, _, err := store.Open(context.Background(), ref)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(data)
}

// A reference is resolved through its own row, so a file written to one
// backend reads back from that backend even when another backend of the same
// provider exists — and the reader needs no tenant, knowledge base or
// provider to find it (B1, B4).
func TestFileStoreReadsEachReferenceFromItsOwnBackend(t *testing.T) {
	f := newFileStoreFixture(t)
	a := f.localBackend(t, 7, "Local A", "alpha")
	b := f.localBackend(t, 8, "Local B", "beta")

	refA := f.save(t, a.ID, 7, "a.txt", "from-a")
	refB := f.save(t, b.ID, 8, "b.txt", "from-b")
	require.True(t, strings.HasPrefix(refA, types.ResourceScheme), refA)

	assert.Equal(t, "from-a", readAll(t, f.store, refA))
	assert.Equal(t, "from-b", readAll(t, f.store, refB))

	// The bytes really are under each backend's own prefix.
	assert.FileExists(t, findOne(t, filepath.Join(f.baseDir, "alpha", "7", "exports")))
	assert.FileExists(t, findOne(t, filepath.Join(f.baseDir, "beta", "8", "exports")))

	var resource types.StoredResource
	require.NoError(t, f.db.Where("handle = ?", strings.TrimPrefix(refA, types.ResourceScheme)).First(&resource).Error)
	assert.Equal(t, a.ID, resource.StorageBackendID)
	assert.True(t, strings.HasPrefix(resource.PhysicalPath, "local://7/exports/"),
		"the row keeps the driver-native locator, got %q", resource.PhysicalPath)
}

func findOne(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return filepath.Join(dir, entries[0].Name())
}

// Rebinding is a change to where new bytes go; existing files keep resolving
// through their rows (B3).
func TestFileStoreReferencesSurviveARebind(t *testing.T) {
	f := newFileStoreFixture(t)
	old := f.localBackend(t, 7, "Old", "old")
	next := f.localBackend(t, 7, "New", "new")

	kb := &types.KnowledgeBase{ID: "kb-1", TenantID: 7, StorageBackendID: old.ID}
	writer, err := f.store.ForKnowledgeBase(context.Background(), kb)
	require.NoError(t, err)
	before, err := writer.SaveBytes(context.Background(), []byte("before"), 7, "x.txt", false)
	require.NoError(t, err)

	kb.StorageBackendID = next.ID
	writer, err = f.store.ForKnowledgeBase(context.Background(), kb)
	require.NoError(t, err)
	after, err := writer.SaveBytes(context.Background(), []byte("after"), 7, "x.txt", false)
	require.NoError(t, err)

	assert.Equal(t, "before", readAll(t, f.store, before))
	assert.Equal(t, "after", readAll(t, f.store, after))
	// The new writer still reads and deletes the old file.
	reader, err := writer.GetFile(context.Background(), before)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.NoError(t, writer.DeleteFile(context.Background(), before))
	_, _, err = f.store.Open(context.Background(), before)
	assert.True(t, errors.Is(err, types.ErrResourceNotFound), "deleted reference: %v", err)
}

// With APP_EXTERNAL_URL set, a public URL is a grant on this server for every
// backend, including a user-registered one (B1). Without it, local storage has
// no public URL at all.
func TestFileStoreURL(t *testing.T) {
	t.Run("grant on the external address", func(t *testing.T) {
		t.Setenv("APP_EXTERNAL_URL", "https://yuheng.example.com/")
		f := newFileStoreFixture(t)
		backend := f.localBackend(t, 7, "User local", "user")
		ref := f.save(t, backend.ID, 7, "a.png", "png")

		url, ok, err := f.store.URL(context.Background(), ref, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		assert.True(t, strings.HasPrefix(url, "https://yuheng.example.com/r/"), url)
	})
	t.Run("local storage without an external address", func(t *testing.T) {
		t.Setenv("APP_EXTERNAL_URL", "")
		f := newFileStoreFixture(t)
		backend := f.localBackend(t, 7, "User local", "user")
		ref := f.save(t, backend.ID, 7, "a.png", "png")

		url, ok, err := f.store.URL(context.Background(), ref, 0)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, url)
	})
}

// The docreader sees the local base directory on its shared volume; a local
// backend's path_prefix is part of the path it must be given (B5).
func TestFileStoreLocalPathHonoursThePathPrefix(t *testing.T) {
	t.Setenv("DOCREADER_SHARED_DATA_DIR", "/shared/files")
	f := newFileStoreFixture(t)
	prefixed := f.localBackend(t, 7, "Prefixed", "team/videos")
	plain := f.localBackend(t, 7, "Plain", "")

	ref := f.save(t, prefixed.ID, 7, "v.mp4", "video")
	got, ok := f.store.LocalPath(context.Background(), ref)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(got, "/shared/files/team/videos/7/exports/"), got)

	ref = f.save(t, plain.ID, 7, "v.mp4", "video")
	got, ok = f.store.LocalPath(context.Background(), ref)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(got, "/shared/files/7/exports/"), got)

	_, ok = f.store.LocalPath(context.Background(), "local://7/exports/v.mp4")
	assert.False(t, ok, "a raw locator is not a reference")
}

func TestFileStoreLocalPathNeedsTheSharedVolume(t *testing.T) {
	t.Setenv("DOCREADER_SHARED_DATA_DIR", "")
	f := newFileStoreFixture(t)
	backend := f.localBackend(t, 7, "Local", "")
	ref := f.save(t, backend.ID, 7, "v.mp4", "video")
	_, ok := f.store.LocalPath(context.Background(), ref)
	assert.False(t, ok)
}

// A server-side copy needs both ends on one backend; across backends the
// writer says so rather than writing somewhere unexpected.
func TestFileStoreCopyStaysOnOneBackend(t *testing.T) {
	f := newFileStoreFixture(t)
	a := f.localBackend(t, 7, "A", "a")
	b := f.localBackend(t, 7, "B", "b")
	ref := f.save(t, a.ID, 7, "doc.txt", "original")

	sameWriter, err := f.store.Writer(context.Background(), a.ID)
	require.NoError(t, err)
	copied, err := sameWriter.CopyFile(context.Background(), ref, 7, "knowledge-2")
	require.NoError(t, err)
	assert.NotEqual(t, ref, copied)
	assert.Equal(t, "original", readAll(t, f.store, copied))

	otherWriter, err := f.store.Writer(context.Background(), b.ID)
	require.NoError(t, err)
	_, err = otherWriter.CopyFile(context.Background(), ref, 7, "knowledge-3")
	assert.True(t, errors.Is(err, filesvc.ErrCrossBackendCopy), "cross-backend copy: %v", err)
}

func TestFileStoreRefusesWhatItCannotResolve(t *testing.T) {
	f := newFileStoreFixture(t)
	backend := f.localBackend(t, 7, "Local", "")

	_, _, err := f.store.Open(context.Background(), "local://7/exports/a.txt")
	assert.Error(t, err, "raw locators are not references")
	_, _, err = f.store.Open(context.Background(), "resource://ZZZZZZZZZZZZZZZZZZZZZZ")
	assert.True(t, errors.Is(err, types.ErrResourceNotFound), "unknown handle: %v", err)

	_, err = f.store.Writer(context.Background(), "")
	assert.Error(t, err, "a write must name its backend")
	_, err = f.store.ForKnowledgeBase(context.Background(), &types.KnowledgeBase{ID: "kb-unbound"})
	assert.Error(t, err, "an unbound knowledge base has nowhere to write")

	require.NoError(t, f.db.Model(&types.StorageBackend{}).Where("id = ?", backend.ID).
		Update("status", types.StorageBackendStatusDisabled).Error)
	_, err = f.store.Writer(context.Background(), backend.ID)
	assert.Error(t, err, "a disabled backend takes no writes")
}

// The driver for a backend is rebuilt when its row changes, so a credential
// rotation or a moved directory takes effect without a restart.
func TestFileStoreRebuildsTheDriverWhenTheRowChanges(t *testing.T) {
	f := newFileStoreFixture(t)
	backend := f.localBackend(t, 7, "Local", "first")
	f.save(t, backend.ID, 7, "a.txt", "one")

	require.NoError(t, f.db.Model(&types.StorageBackend{}).Where("id = ?", backend.ID).Updates(map[string]any{
		"config":     types.StorageBackendConfig{PathPrefix: "second"},
		"updated_at": time.Now().Add(time.Second),
	}).Error)
	f.save(t, backend.ID, 7, "b.txt", "two")
	assert.FileExists(t, findOne(t, filepath.Join(f.baseDir, "second", "7", "exports")))
}

func TestFileStoreWritesTheWorkspaceDefault(t *testing.T) {
	f := newFileStoreFixture(t)
	backend := f.localBackend(t, 7, "Default", "default")
	require.NoError(t, f.db.Create(&types.Tenant{ID: 7, Name: "w", DefaultStorageBackendID: backend.ID}).Error)

	writer, err := f.store.ForTenantDefault(context.Background(), 7)
	require.NoError(t, err)
	ref, err := writer.SaveBytes(context.Background(), []byte("chat image"), 7, "c.png", false)
	require.NoError(t, err)
	assert.Equal(t, "chat image", readAll(t, f.store, ref))
	assert.FileExists(t, findOne(t, filepath.Join(f.baseDir, "default", "7", "exports")))
}
