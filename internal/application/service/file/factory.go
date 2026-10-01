package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// LocalBaseDir is the directory every local-provider backend lives under:
// LOCAL_STORAGE_BASE_DIR, or the container default. A local backend's
// path_prefix is a subdirectory of it, never a separate root, so one volume
// mount serves every local backend.
func LocalBaseDir() string {
	if dir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR")); dir != "" {
		return dir
	}
	return "/data/files"
}

// LocalBackendDir is the directory a local backend with the given path_prefix
// reads and writes. The prefix is validated to stay under the base directory;
// one that would escape it is an error rather than silently ignored, because
// ignoring it would put the backend's files somewhere other than where its
// configuration says.
func LocalBackendDir(pathPrefix string) (string, error) {
	base := LocalBaseDir()
	prefix := strings.Trim(strings.TrimSpace(pathPrefix), "/\\")
	if prefix == "" {
		return base, nil
	}
	return secutils.SafePathUnderBase(base, filepath.Join(base, prefix))
}

// NewDriver builds the storage driver for one backend row: the object that
// speaks the backend's native locators (local://<rel>, s3://<bucket>/<key>).
// Drivers know nothing of resource handles; FileStore wraps them.
func NewDriver(backend *types.StorageBackend) (interfaces.FileService, error) {
	if backend == nil {
		return nil, fmt.Errorf("storage backend is required")
	}
	// Rows are validated when they are written; checking again here keeps a
	// row edited behind the application's back from producing a driver that
	// fails on first use with a less useful error.
	if err := backend.Config.ValidateForProvider(backend.Provider); err != nil {
		return nil, err
	}
	c := backend.Config
	switch backend.Provider {
	case types.StorageProviderLocal:
		dir, err := LocalBackendDir(c.PathPrefix)
		if err != nil {
			return nil, fmt.Errorf("local storage path prefix: %w", err)
		}
		return NewLocalFileService(dir), nil
	case types.StorageProviderS3:
		pathPrefix := strings.TrimSpace(c.PathPrefix)
		if pathPrefix == "" {
			pathPrefix = "yuheng/"
		}
		return NewS3FileService(S3Options{
			Endpoint: c.Endpoint, Region: c.Region, AccessKey: c.AccessKeyID, SecretKey: c.SecretAccessKey,
			BucketName: c.BucketName, PathPrefix: pathPrefix, UseSSL: c.UseSSL, AddressingStyle: c.AddressingStyle,
		})
	default:
		return nil, fmt.Errorf("unsupported storage provider %q (supported: local, s3)", backend.Provider)
	}
}
