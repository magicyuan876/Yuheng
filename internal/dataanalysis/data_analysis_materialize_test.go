package dataanalysis

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

// fakeFileReader implements FileReader just enough to drive
// materializeKnowledgeFile: it serves the readers registered per reference.
type fakeFileReader struct {
	readers map[string]func() (io.ReadCloser, error)
}

func (f *fakeFileReader) Open(_ context.Context, ref string) (io.ReadCloser, *types.StoredResource, error) {
	fn, ok := f.readers[ref]
	if !ok {
		return nil, nil, errors.New("unknown reference: " + ref)
	}
	reader, err := fn()
	if err != nil {
		return nil, nil, err
	}
	return reader, &types.StoredResource{}, nil
}

// TestMaterializeKnowledgeFile_HandlesLocalScheme is the regression guard
// for the dev-mode failure where DuckDB was handed a local:// URL it can't
// resolve. The tool must pull bytes through FileStore.Open and hand DuckDB
// a concrete filesystem path with the right extension.
func TestMaterializeKnowledgeFile_HandlesLocalScheme(t *testing.T) {
	payload := []byte("col1,col2\n1,2\n3,4\n")

	fs := &fakeFileReader{
		readers: map[string]func() (io.ReadCloser, error){
			"tenants/42/data.csv": func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(payload)), nil
			},
		},
	}

	tool := &DataAnalysisTool{
		files:     fs,
		sessionID: "test-materialize",
	}

	k := &types.Knowledge{
		ID:       "k-abc",
		FileType: "csv",
		FilePath: "tenants/42/data.csv",
	}

	path, cleanup, err := tool.materializeKnowledgeFile(context.Background(), k)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	t.Cleanup(cleanup)

	// Path must be a real filesystem path (no provider scheme) so DuckDB
	// can st_read / read_xlsx / read_csv_auto it directly.
	if strings.Contains(path, "://") {
		t.Fatalf("expected bare filesystem path, got scheme URL: %q", path)
	}
	if filepath.Ext(path) != ".csv" {
		t.Errorf("expected .csv suffix preserved for DuckDB format detection, got %q", path)
	}

	// File must actually exist and have the exact bytes we fed in.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload roundtrip mismatch: got %q, want %q", got, payload)
	}

	// cleanup() must remove the temp file without error.
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected temp file to be removed after cleanup, stat err=%v", err)
	}
	// Double-cleanup must be safe (test harness may invoke it again).
	cleanup()
}

func TestMaterializeKnowledgeFile_PropagatesGetFileError(t *testing.T) {
	fs := &fakeFileReader{
		readers: map[string]func() (io.ReadCloser, error){
			"bad/path": func() (io.ReadCloser, error) {
				return nil, errors.New("boom")
			},
		},
	}

	tool := &DataAnalysisTool{files: fs, sessionID: "test-error"}

	_, cleanup, err := tool.materializeKnowledgeFile(
		context.Background(),
		&types.Knowledge{ID: "k-err", FileType: "xlsx", FilePath: "bad/path"},
	)
	if err == nil {
		defer cleanup()
		t.Fatal("expected error from underlying GetFile, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected wrapped 'boom' error, got: %v", err)
	}
}

func TestMaterializeKnowledgeFile_PreservesExtension(t *testing.T) {
	cases := []struct {
		fileType string
		wantExt  string
	}{
		{"xlsx", ".xlsx"},
		{"XLSX", ".xlsx"}, // normalized lowercase
		{"csv", ".csv"},
		{"xls", ".xls"},
	}
	for _, c := range cases {
		t.Run(c.fileType, func(t *testing.T) {
			fs := &fakeFileReader{
				readers: map[string]func() (io.ReadCloser, error){
					"p": func() (io.ReadCloser, error) {
						return io.NopCloser(bytes.NewReader([]byte("x"))), nil
					},
				},
			}
			tool := &DataAnalysisTool{files: fs, sessionID: "ext"}
			path, cleanup, err := tool.materializeKnowledgeFile(
				context.Background(),
				&types.Knowledge{ID: "k", FileType: c.fileType, FilePath: "p"},
			)
			if err != nil {
				t.Fatalf("materialize: %v", err)
			}
			defer cleanup()
			if filepath.Ext(path) != c.wantExt {
				t.Errorf("fileType=%q: extension mismatch got %q want %q", c.fileType, filepath.Ext(path), c.wantExt)
			}
		})
	}
}
