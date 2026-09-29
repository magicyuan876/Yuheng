package file

import (
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestNewFileServiceFromStorageConfig(t *testing.T) {
	baseDir := t.TempDir()

	t.Run("local", func(t *testing.T) {
		sec := &types.StorageEngineConfig{DefaultProvider: "LOCAL"}
		svc, provider, err := NewFileServiceFromStorageConfig("", sec, baseDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider != "local" {
			t.Errorf("provider = %q, want local", provider)
		}
		if _, ok := svc.(*localFileService); !ok {
			t.Errorf("service = %T, want *localFileService", svc)
		}
	})

	t.Run("s3 with an unsafe endpoint fails before any network call", func(t *testing.T) {
		sec := &types.StorageEngineConfig{S3: &types.S3EngineConfig{
			Endpoint: "http://169.254.169.254", Region: "us-east-1", BucketName: "b", AccessKey: "ak", SecretKey: "sk",
		}}
		_, provider, err := NewFileServiceFromStorageConfig("s3", sec, baseDir)
		if provider != "s3" || err == nil || !strings.Contains(err.Error(), "unsafe S3 endpoint") {
			t.Fatalf("provider = %q, err = %v; want an unsafe endpoint error", provider, err)
		}
	})

	t.Run("s3 with an invalid addressing style", func(t *testing.T) {
		sec := &types.StorageEngineConfig{S3: &types.S3EngineConfig{
			Region: "us-east-1", BucketName: "b", AddressingStyle: "sideways",
		}}
		if _, _, err := NewFileServiceFromStorageConfig("s3", sec, baseDir); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("incomplete s3 config", func(t *testing.T) {
		tests := map[string]*types.StorageEngineConfig{
			"nil config":     nil,
			"no s3 section":  {},
			"missing bucket": {S3: &types.S3EngineConfig{Region: "us-east-1"}},
			"missing region": {S3: &types.S3EngineConfig{BucketName: "b"}},
			"lone access key": {S3: &types.S3EngineConfig{
				Region: "us-east-1", BucketName: "b", AccessKey: "ak",
			}},
		}
		for name, sec := range tests {
			t.Run(name, func(t *testing.T) {
				_, _, err := NewFileServiceFromStorageConfig("s3", sec, baseDir)
				if err == nil || !strings.Contains(err.Error(), "incomplete s3 config") {
					t.Fatalf("err = %v, want incomplete s3 config", err)
				}
			})
		}
	})

	t.Run("removed and unknown providers name the supported ones", func(t *testing.T) {
		for _, provider := range []string{"minio", "cos", "tos", "oss", "obs", "ks3", "nope"} {
			_, _, err := NewFileServiceFromStorageConfig(provider, nil, baseDir)
			if err == nil || !strings.Contains(err.Error(), "unsupported storage provider") ||
				!strings.Contains(err.Error(), "local, s3") {
				t.Errorf("provider %q: err = %v", provider, err)
			}
		}
	})

	t.Run("empty provider", func(t *testing.T) {
		if _, _, err := NewFileServiceFromStorageConfig("", nil, baseDir); err == nil {
			t.Fatal("expected an error")
		}
	})
}
