package file

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A directory on this server's disk is not addressable from outside it, so
// the driver hands its locator back unchanged; public links are resource
// grants issued by the file store, never by the driver.
func TestLocalGetFileURL_ReturnsTheLocator(t *testing.T) {
	svc := NewLocalFileService("/data/files")

	got, err := svc.GetFileURL(context.Background(), "local://1/abc/img.png")
	require.NoError(t, err)
	assert.Equal(t, "local://1/abc/img.png", got)
}

// Every path this service hands out is local://; a bare or absolute path is
// not one of its own and must not be opened, deleted or linked.
func TestLocalFileService_RejectsPathsWithoutTheLocalScheme(t *testing.T) {
	base := t.TempDir()
	svc := NewLocalFileService(base)
	ctx := context.Background()

	for _, p := range []string{filepath.Join(base, "1", "a.png"), "1/a.png", "/etc/passwd"} {
		_, err := svc.GetFile(ctx, p)
		assert.Error(t, err, "GetFile(%q)", p)
		assert.Error(t, svc.DeleteFile(ctx, p), "DeleteFile(%q)", p)
		_, err = svc.GetFileURL(ctx, p)
		assert.Error(t, err, "GetFileURL(%q)", p)
	}
}
