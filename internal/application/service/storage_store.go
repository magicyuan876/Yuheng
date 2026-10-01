package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	filesvc "github.com/magicyuan876/yuheng/internal/application/service/file"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// defaultResourceURLTTL is how long a public URL minted for a resource stays
// valid when the caller does not say.
const defaultResourceURLTTL = 2 * time.Hour

// fileStore implements interfaces.FileStore.
//
// Every read starts from the resource row: the row names the backend the
// bytes were written to and the driver-native locator on it, so resolution
// needs nothing else — not the caller's tenant, not a knowledge base, not a
// provider guess. Writes name their backend explicitly. That split is what
// lets a binding change without breaking a single existing reference.
type fileStore struct {
	backends interfaces.StorageBackendRepository
	catalog  interfaces.ResourceCatalog
	tenants  interfaces.TenantRepository
	// externalURL is APP_EXTERNAL_URL. When set, public URLs are resource
	// grants on this server (/r/<token>), which work for every backend and
	// keep the backend's own address private.
	externalURL string
	// sharedDir is DOCREADER_SHARED_DATA_DIR: where the docreader container
	// mounts the local storage base directory.
	sharedDir string

	mu      sync.Mutex
	drivers map[string]cachedDriver
}

// cachedDriver is a driver built for one version of a backend row. A row's
// updated_at changes whenever its configuration does (credential rotation,
// the startup sync of the environment backend), so keying on it invalidates
// the driver without any explicit signal.
type cachedDriver struct {
	updatedAt time.Time
	driver    interfaces.FileService
}

// NewFileStore builds the storage runtime.
func NewFileStore(
	backends interfaces.StorageBackendRepository,
	catalog interfaces.ResourceCatalog,
	tenants interfaces.TenantRepository,
) interfaces.FileStore {
	return &fileStore{
		backends:    backends,
		catalog:     catalog,
		tenants:     tenants,
		externalURL: strings.TrimRight(strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL")), "/"),
		sharedDir:   strings.TrimSpace(os.Getenv("DOCREADER_SHARED_DATA_DIR")),
		drivers:     make(map[string]cachedDriver),
	}
}

// ---- backend resolution ---------------------------------------------------

// backend loads a backend row by id. The deployment backend's row carries no
// credentials; they are read from the environment here, every time, so they
// never touch the database.
func (s *fileStore) backend(ctx context.Context, id string) (*types.StorageBackend, error) {
	backend, err := s.backends.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, fmt.Errorf("storage backend %s not found", id)
	}
	if backend.Source == types.StorageBackendSourceEnv {
		backend.Config.AccessKeyID, backend.Config.SecretAccessKey = types.EnvStorageCredentials()
	}
	return backend, nil
}

// driver returns the cached driver for a backend row, building it on first
// use or when the row has changed since.
func (s *fileStore) driver(backend *types.StorageBackend) (interfaces.FileService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.drivers[backend.ID]; ok && cached.updatedAt.Equal(backend.UpdatedAt) {
		return cached.driver, nil
	}
	driver, err := filesvc.NewDriver(backend)
	if err != nil {
		return nil, fmt.Errorf("storage backend %s: %w", backend.ID, err)
	}
	s.drivers[backend.ID] = cachedDriver{updatedAt: backend.UpdatedAt, driver: driver}
	return driver, nil
}

// location is one resolved reference: the row, the backend it names, a driver
// for that backend and the driver-native locator.
type location struct {
	resource *types.StoredResource
	backend  *types.StorageBackend
	driver   interfaces.FileService
	physical string
}

// locate resolves a resource:// reference down to its driver. Anything that
// is not a resource handle is refused: handles are the only reference the
// application persists or accepts.
func (s *fileStore) locate(ctx context.Context, ref string) (*location, error) {
	if _, ok := types.ParseResourcePath(ref); !ok {
		return nil, fmt.Errorf("not a resource reference")
	}
	resource, err := s.catalog.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	backend, err := s.backend(ctx, resource.StorageBackendID)
	if err != nil {
		return nil, err
	}
	driver, err := s.driver(backend)
	if err != nil {
		return nil, err
	}
	return &location{resource: resource, backend: backend, driver: driver, physical: resource.PhysicalPath}, nil
}

// ---- reads ------------------------------------------------------------------

func (s *fileStore) Open(ctx context.Context, ref string) (io.ReadCloser, *types.StoredResource, error) {
	loc, err := s.locate(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	reader, err := loc.driver.GetFile(ctx, loc.physical)
	if err != nil {
		return nil, nil, err
	}
	return reader, loc.resource, nil
}

func (s *fileStore) URL(ctx context.Context, ref string, ttl time.Duration) (string, bool, error) {
	loc, err := s.locate(ctx, ref)
	if err != nil {
		return "", false, err
	}
	if ttl <= 0 {
		ttl = defaultResourceURLTTL
	}
	// A grant on this server is preferred whenever there is an address to put
	// it on: it works for every provider, it is revocable, and it does not
	// publish the bucket's endpoint.
	if s.externalURL != "" {
		token, err := s.catalog.CreateAccessGrant(ctx, ref, ttl)
		if err != nil {
			return "", false, err
		}
		return s.externalURL + "/r/" + token, true, nil
	}
	// Without one, only an object store can hand out a link of its own.
	if loc.backend.Provider != types.StorageProviderS3 {
		return "", false, nil
	}
	url, err := loc.driver.GetFileURL(ctx, loc.physical)
	if err != nil {
		return "", false, err
	}
	return url, isHTTPURL(url), nil
}

func (s *fileStore) Delete(ctx context.Context, ref string) error {
	loc, err := s.locate(ctx, ref)
	if err != nil {
		return err
	}
	if err := loc.driver.DeleteFile(ctx, loc.physical); err != nil {
		return err
	}
	return s.catalog.MarkDeleted(ctx, ref)
}

func (s *fileStore) LocalPath(ctx context.Context, ref string) (string, bool) {
	if s.sharedDir == "" {
		return "", false
	}
	loc, err := s.locate(ctx, ref)
	if err != nil {
		logger.Warnf(ctx, "[storage] resolve %s for shared-volume handoff failed: %v", ref, err)
		return "", false
	}
	if loc.backend.Provider != types.StorageProviderLocal {
		return "", false
	}
	rel, ok := strings.CutPrefix(loc.physical, "local://")
	if !ok || rel == "" {
		return "", false
	}
	// The shared volume is the local base directory, and a local backend
	// lives in its path_prefix subdirectory of it; the driver path is
	// relative to that subdirectory. Both parts are cleaned on a rooted copy
	// so neither can climb out of the shared mount.
	prefix := strings.TrimPrefix(path.Clean("/"+strings.Trim(loc.backend.Config.PathPrefix, "/\\")), "/")
	cleaned := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if cleaned == "" || cleaned == "." {
		return "", false
	}
	return path.Join(s.sharedDir, prefix, cleaned), true
}

// ---- writers ----------------------------------------------------------------

func (s *fileStore) Writer(ctx context.Context, backendID string) (interfaces.FileService, error) {
	backendID = strings.TrimSpace(backendID)
	if backendID == "" {
		return nil, fmt.Errorf("a storage backend must be named for writing")
	}
	backend, err := s.backend(ctx, backendID)
	if err != nil {
		return nil, err
	}
	if backend.Status != types.StorageBackendStatusActive {
		return nil, fmt.Errorf("storage backend %s is %s", backend.ID, backend.Status)
	}
	driver, err := s.driver(backend)
	if err != nil {
		return nil, err
	}
	return &backendWriter{store: s, backend: backend, driver: driver}, nil
}

func (s *fileStore) ForKnowledgeBase(ctx context.Context, kb *types.KnowledgeBase) (interfaces.FileService, error) {
	if kb == nil {
		return nil, fmt.Errorf("knowledge base is required")
	}
	if kb.StorageBackendID == "" {
		return nil, fmt.Errorf("knowledge base %s has no storage backend", kb.ID)
	}
	return s.Writer(ctx, kb.StorageBackendID)
}

func (s *fileStore) ForTenantDefault(ctx context.Context, tenantID uint64) (interfaces.FileService, error) {
	tenant, err := s.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, fmt.Errorf("workspace %d not found", tenantID)
	}
	if tenant.DefaultStorageBackendID == "" {
		return nil, fmt.Errorf("workspace %d has no default storage backend", tenantID)
	}
	return s.Writer(ctx, tenant.DefaultStorageBackendID)
}

// backendWriter is the FileService a FileStore hands out for one backend.
// Saves land on that backend and come back as resource handles; every other
// method takes a handle and goes wherever the handle's row says.
type backendWriter struct {
	store   *fileStore
	backend *types.StorageBackend
	driver  interfaces.FileService
}

func (w *backendWriter) CheckConnectivity(ctx context.Context) error {
	return w.driver.CheckConnectivity(ctx)
}

func (w *backendWriter) SaveFile(
	ctx context.Context, file *multipart.FileHeader, tenantID uint64, knowledgeID string,
) (string, error) {
	physical, err := w.driver.SaveFile(ctx, file, tenantID, knowledgeID)
	if err != nil {
		return "", err
	}
	ref, err := w.register(ctx, physical, tenantID, file.Filename, file.Size, false, "")
	if err != nil {
		return "", err
	}
	return w.bindKnowledge(ctx, ref, knowledgeID)
}

func (w *backendWriter) SaveBytes(
	ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool,
) (string, error) {
	physical, err := w.driver.SaveBytes(ctx, data, tenantID, fileName, temp)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return w.register(ctx, physical, tenantID, fileName, int64(len(data)), temp, hex.EncodeToString(sum[:]))
}

func (w *backendWriter) GetFile(ctx context.Context, ref string) (io.ReadCloser, error) {
	reader, _, err := w.store.Open(ctx, ref)
	return reader, err
}

// GetFileURL keeps the FileService contract of returning the reference itself
// when no fetchable URL exists, so the caller degrades to the authenticated
// proxy rather than failing.
func (w *backendWriter) GetFileURL(ctx context.Context, ref string) (string, error) {
	url, ok, err := w.store.URL(ctx, ref, 0)
	if err != nil {
		return "", err
	}
	if !ok {
		return ref, nil
	}
	return url, nil
}

func (w *backendWriter) DeleteFile(ctx context.Context, ref string) error {
	return w.store.Delete(ctx, ref)
}

// CopyFile duplicates an object within this writer's backend. A source on
// another backend is refused with ErrCrossBackendCopy: a server-side copy
// needs both ends on one store, and the callers that copy (knowledge clone
// and move) check the knowledge bases share a backend first.
func (w *backendWriter) CopyFile(
	ctx context.Context, ref string, tenantID uint64, knowledgeID string,
) (string, error) {
	src, err := w.store.locate(ctx, ref)
	if err != nil {
		return "", err
	}
	if src.backend.ID != w.backend.ID {
		return "", fmt.Errorf("copy %s from backend %s to %s: %w",
			ref, src.backend.ID, w.backend.ID, filesvc.ErrCrossBackendCopy)
	}
	copied, err := w.driver.CopyFile(ctx, src.physical, tenantID, knowledgeID)
	if err != nil {
		return "", err
	}
	name := src.resource.OriginalName
	if name == "" {
		name = filepath.Base(src.physical)
	}
	newRef, err := w.register(ctx, copied, tenantID, name, src.resource.Size, false, src.resource.ContentHash)
	if err != nil {
		return "", err
	}
	return w.bindKnowledge(ctx, newRef, knowledgeID)
}

// register records a freshly written object. If the row cannot be written the
// object is removed again: bytes nobody can reference are only a cost.
func (w *backendWriter) register(
	ctx context.Context, physical string, tenantID uint64, name string, size int64, temporary bool,
	contentHash string,
) (string, error) {
	kind, mimeType := resourceKind(name)
	ref, err := w.store.catalog.Register(ctx, tenantID, w.backend.ID, physical, interfaces.ResourceRegistration{
		Kind:         kind,
		MimeType:     mimeType,
		OriginalName: filepath.Base(name),
		Size:         size,
		ContentHash:  contentHash,
		Temporary:    temporary,
	})
	if err != nil {
		if deleteErr := w.driver.DeleteFile(ctx, physical); deleteErr != nil {
			logger.Warnf(ctx, "[storage] removing unregistered object %s failed: %v", physical, deleteErr)
		}
		return "", fmt.Errorf("register stored resource: %w", err)
	}
	return ref, nil
}

// bindKnowledge records that a knowledge entry owns the resource, so the
// entry's deletion can find and release it.
func (w *backendWriter) bindKnowledge(ctx context.Context, ref, knowledgeID string) (string, error) {
	if knowledgeID == "" {
		return ref, nil
	}
	if err := w.store.catalog.Bind(ctx, ref, "knowledge", knowledgeID, "source_file"); err != nil {
		if deleteErr := w.store.Delete(ctx, ref); deleteErr != nil {
			logger.Warnf(ctx, "[storage] removing unbound resource %s failed: %v", ref, deleteErr)
		}
		return "", fmt.Errorf("bind stored resource: %w", err)
	}
	return ref, nil
}

// resourceKind classifies a file by its extension for the resource row.
func resourceKind(name string) (string, string) {
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	kind := "file"
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		kind = "image"
	case strings.HasPrefix(mimeType, "audio/"):
		kind = "audio"
	case strings.HasPrefix(mimeType, "video/"):
		kind = "video"
	}
	return kind, mimeType
}

// isHTTPURL reports whether s is something a browser can fetch.
func isHTTPURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

var (
	_ interfaces.FileStore   = (*fileStore)(nil)
	_ interfaces.FileService = (*backendWriter)(nil)
)
