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
		externalURL := strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL"))
		return NewLocalFileService(baseDir, externalURL), p, nil

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
