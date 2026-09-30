package file

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractTenantIDFromPresignedURL pulls the tenant_id query parameter from a
// signed URL. Returns "" when the URL is not parseable as a presigned URL.
func extractTenantIDFromPresignedURL(t *testing.T, presigned string) string {
	t.Helper()
	u, err := url.Parse(presigned)
	require.NoError(t, err)
	return u.Query().Get("tenant_id")
}

// TestLocalGetFileURL_TenantIDFromPath verifies that tenant ID is extracted
// from the storage path — which encodes the resource owner, so cross-tenant
// shared resources resolve to the correct owning tenant's storage config.
func TestLocalGetFileURL_TenantIDFromPath(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "yuheng-test-aes-key-32bytes!!!")

	svc := NewLocalFileService("/data/files", "https://yuheng.example.com")

	got, err := svc.GetFileURL(context.Background(), "local://7/abc/img.png")
	require.NoError(t, err)
	assert.Equal(t, "7", extractTenantIDFromPresignedURL(t, got))
}

// TestLocalGetFileURL_NoExternalURL verifies that without APP_EXTERNAL_URL
// there is nothing to sign, so GetFileURL returns the local:// path unchanged.
func TestLocalGetFileURL_NoExternalURL(t *testing.T) {
	svc := NewLocalFileService("/data/files", "")

	got, err := svc.GetFileURL(context.Background(), "local://1/abc/img.png")
	require.NoError(t, err)
	assert.Equal(t, "local://1/abc/img.png", got)
}

// Every path this service hands out is local://; a bare or absolute path is
// not one of its own and must not be opened, deleted or signed.
func TestLocalFileService_RejectsPathsWithoutTheLocalScheme(t *testing.T) {
	base := t.TempDir()
	svc := NewLocalFileService(base, "https://yuheng.example.com")
	ctx := context.Background()

	for _, p := range []string{filepath.Join(base, "1", "a.png"), "1/a.png", "/etc/passwd"} {
		_, err := svc.GetFile(ctx, p)
		assert.Error(t, err, "GetFile(%q)", p)
		assert.Error(t, svc.DeleteFile(ctx, p), "DeleteFile(%q)", p)
		_, err = svc.GetFileURL(ctx, p)
		assert.Error(t, err, "GetFileURL(%q)", p)
	}
}
