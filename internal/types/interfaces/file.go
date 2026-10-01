package interfaces

import (
	"context"
	"io"
	"mime/multipart"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// FileService is the interface for file services.
// FileService provides methods to save, retrieve, and delete files.
//
// Two kinds of value implement it. A storage driver (local disk, S3) speaks
// the driver-native locator it returns from SaveFile/SaveBytes and accepts
// nothing else. A writer obtained from FileStore returns resource:// handles
// instead, writes to the one backend it was obtained for, and resolves every
// handle it is given through that handle's own resource row, so reading or
// deleting a file never depends on which backend the writer targets.
type FileService interface {
	// CheckConnectivity verifies that the storage backend is reachable and
	// properly configured (e.g. bucket exists, credentials valid).
	CheckConnectivity(ctx context.Context) error
	// SaveFile saves a file.
	SaveFile(ctx context.Context, file *multipart.FileHeader, tenantID uint64, knowledgeID string) (string, error)
	// SaveBytes saves bytes data to a file and returns the file path.
	// If temp is true, the file will be saved to a temporary storage that may auto-expire.
	SaveBytes(ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool) (string, error)
	// GetFile retrieves a file.
	GetFile(ctx context.Context, filePath string) (io.ReadCloser, error)
	// GetFileURL returns a download URL for the file (if supported by the storage backend).
	GetFileURL(ctx context.Context, filePath string) (string, error)
	// DeleteFile deletes a file.
	DeleteFile(ctx context.Context, filePath string) error
	// CopyFile copies an existing stored object to a NEW object owned by
	// (tenantID, knowledgeID), returning the new path. The copy is
	// independent: deleting the source never affects it. Returns ErrCrossBackendCopy
	// when srcPath lives on a different storage backend than this service writes to.
	CopyFile(ctx context.Context, srcPath string, tenantID uint64, knowledgeID string) (string, error)
}

// FileStore is the storage runtime: the one place that turns a stored
// reference into bytes, and a backend binding into somewhere new bytes go.
//
// Reading and writing are deliberately asymmetric. A binding (a knowledge
// base's, a docs space's, a workspace default) decides only where NEW bytes
// land; an existing file is always found through its resource row, which
// records the backend it was written to. Rebinding therefore never strands a
// file, and a reader never needs to know which tenant, knowledge base or
// provider the file came from. Authorization is the caller's job: FileStore
// resolves any valid handle it is given.
type FileStore interface {
	// Writer returns a FileService that writes to the named backend. Its read
	// and delete methods resolve any reference, whichever backend holds it.
	Writer(ctx context.Context, backendID string) (FileService, error)
	// ForKnowledgeBase is Writer for the knowledge base's bound backend.
	ForKnowledgeBase(ctx context.Context, kb *types.KnowledgeBase) (FileService, error)
	// ForTenantDefault is Writer for the workspace's default backend, which
	// takes the writes that belong to no knowledge base: chat images,
	// session attachments, temporary documents.
	ForTenantDefault(ctx context.Context, tenantID uint64) (FileService, error)
	// Open reads a resource:// reference from the backend that holds it.
	Open(ctx context.Context, ref string) (io.ReadCloser, *types.StoredResource, error)
	// URL returns an address an external client can fetch the reference
	// from. ok=false means none can exist for this deployment and backend
	// (local storage without APP_EXTERNAL_URL); the caller then keeps the
	// reference and serves it through an authenticated proxy.
	URL(ctx context.Context, ref string, ttl time.Duration) (url string, ok bool, err error)
	// Delete removes the bytes and retires the resource row.
	Delete(ctx context.Context, ref string) error
	// LocalPath maps a reference on local storage to the path the docreader
	// sees on the shared volume (DOCREADER_SHARED_DATA_DIR), so large files
	// are handed over by path instead of streamed. ok=false when the file is
	// not on local storage or no shared volume is configured.
	LocalPath(ctx context.Context, ref string) (path string, ok bool)
}
