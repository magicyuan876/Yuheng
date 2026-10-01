package file

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackendScopedLocalPathRetainsBackendID(t *testing.T) {
	svc := NewBackendScopedFileService("backend-local-a", NewLocalFileService(t.TempDir()))

	path, err := svc.SaveBytes(context.Background(), []byte("hello"), 7, "exports/image.txt", false)
	require.NoError(t, err)
	assert.Contains(t, path, "storage://backend-local-a/local://")

	reader, err := svc.GetFile(context.Background(), path)
	require.NoError(t, err)
	defer reader.Close()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}
