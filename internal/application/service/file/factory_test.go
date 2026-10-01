package file

import (
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestNewDriver(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())

	t.Run("local", func(t *testing.T) {
		svc, err := NewDriver(&types.StorageBackend{Provider: types.StorageProviderLocal})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := svc.(*localFileService); !ok {
			t.Errorf("service = %T, want *localFileService", svc)
		}
	})

	t.Run("local path prefix stays under the base directory", func(t *testing.T) {
		svc, err := NewDriver(&types.StorageBackend{
			Provider: types.StorageProviderLocal, Config: types.StorageBackendConfig{PathPrefix: "team/a"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		local := svc.(*localFileService)
		if !strings.HasSuffix(local.baseDir, "team/a") {
			t.Errorf("baseDir = %q, want it to end in team/a", local.baseDir)
		}
		if _, err := NewDriver(&types.StorageBackend{
			Provider: types.StorageProviderLocal, Config: types.StorageBackendConfig{PathPrefix: "../escape"},
		}); err == nil {
			t.Error("a prefix that climbs out of the base directory must be refused")
		}
	})

	t.Run("s3 with an unsafe endpoint fails before any network call", func(t *testing.T) {
		_, err := NewDriver(&types.StorageBackend{Provider: types.StorageProviderS3, Config: types.StorageBackendConfig{
			Endpoint: "http://169.254.169.254", Region: "us-east-1", BucketName: "b",
			AccessKeyID: "ak", SecretAccessKey: "sk",
		}})
		if err == nil || !strings.Contains(err.Error(), "unsafe S3 endpoint") {
			t.Fatalf("err = %v; want an unsafe endpoint error", err)
		}
	})

	t.Run("incomplete s3 config", func(t *testing.T) {
		tests := map[string]types.StorageBackendConfig{
			"missing bucket":  {Region: "us-east-1"},
			"missing region":  {BucketName: "b"},
			"lone access key": {Region: "us-east-1", BucketName: "b", AccessKeyID: "ak"},
			"bad addressing":  {Region: "us-east-1", BucketName: "b", AddressingStyle: "sideways"},
		}
		for name, cfg := range tests {
			t.Run(name, func(t *testing.T) {
				backend := &types.StorageBackend{Provider: types.StorageProviderS3, Config: cfg}
				if _, err := NewDriver(backend); err == nil {
					t.Fatal("expected an error")
				}
			})
		}
	})

	t.Run("removed and unknown providers are refused", func(t *testing.T) {
		for _, provider := range []string{"minio", "cos", "tos", "oss", "obs", "ks3", "nope", ""} {
			if _, err := NewDriver(&types.StorageBackend{Provider: provider}); err == nil {
				t.Errorf("provider %q: expected an error", provider)
			}
		}
	})
}
