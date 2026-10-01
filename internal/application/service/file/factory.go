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

// NewFileServiceFromStorageConfig builds a provider-specific FileService from tenant storage config.
// provider can be empty; in that case it falls back to sec.DefaultProvider.
// Returns the resolved provider name together with the service.
func NewFileServiceFromStorageConfig(
	provider string,
	sec *types.StorageEngineConfig,
	localBaseDir string,
) (interfaces.FileService, string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" && sec != nil {
		p = strings.ToLower(strings.TrimSpace(sec.DefaultProvider))
	}
	if p == "" {
		return nil, "", fmt.Errorf("empty provider")
	}

	if localBaseDir == "" {
		localBaseDir = strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	}
	if localBaseDir == "" {
		localBaseDir = "/data/files"
	}

	switch p {
	case types.StorageProviderLocal:
		baseDir := localBaseDir
		if sec != nil && sec.Local != nil {
			rawPrefix := strings.TrimSpace(sec.Local.PathPrefix)
			prefix := strings.Trim(rawPrefix, "/\\")
			if prefix != "" {
				candidate := filepath.Join(baseDir, prefix)
				if safeBaseDir, err := secutils.SafePathUnderBase(baseDir, candidate); err == nil {
					baseDir = safeBaseDir
				}
			}
		}
		return NewLocalFileService(baseDir), p, nil

	case types.StorageProviderS3:
		if sec == nil || sec.S3 == nil || sec.S3.Region == "" || sec.S3.BucketName == "" || (sec.S3.AccessKey == "") != (sec.S3.SecretKey == "") {
			return nil, p, fmt.Errorf("incomplete s3 config")
		}
		pathPrefix := strings.TrimSpace(sec.S3.PathPrefix)
		if pathPrefix == "" {
			pathPrefix = "yuheng/"
		}
		svc, err := NewS3FileService(S3Options{
			Endpoint: sec.S3.Endpoint, Region: sec.S3.Region, AccessKey: sec.S3.AccessKey, SecretKey: sec.S3.SecretKey,
			BucketName: sec.S3.BucketName, PathPrefix: pathPrefix, UseSSL: sec.S3.UseSSL,
			AddressingStyle: sec.S3.AddressingStyle,
		})
		return svc, p, err

	default:
		return nil, p, fmt.Errorf("unsupported storage provider %q (supported: local, s3)", p)
	}
}

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
