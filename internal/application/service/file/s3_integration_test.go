package file

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/utils"
)

// The integration test talks to a real S3-compatible server and is skipped
// unless YUHENG_TEST_S3_ENDPOINT is set, so the default `go test ./...` never
// needs one. To run it against RustFS:
//
//	docker run -d --name rustfs -p 127.0.0.1:9000:9000 -e RUSTFS_ROOT_USER=rustfsadmin \
//	  -e RUSTFS_ROOT_PASSWORD=rustfsadmin -e RUSTFS_VOLUMES=/data rustfs/rustfs:latest rustfs
//	YUHENG_TEST_S3_ENDPOINT=http://127.0.0.1:9000 go test ./internal/application/service/file -run S3Integration -v
//
// YUHENG_TEST_S3_ACCESS_KEY, YUHENG_TEST_S3_SECRET_KEY, YUHENG_TEST_S3_REGION
// and YUHENG_TEST_S3_ADDRESSING_STYLE override the defaults below.
func TestS3Integration(t *testing.T) {
	endpoint := os.Getenv("YUHENG_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("YUHENG_TEST_S3_ENDPOINT is not set")
	}

	// A local test server is a loopback address, which the SSRF guard refuses
	// unless an operator whitelists it, exactly as in a real deployment.
	host := strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	utils.SetSSRFWhitelistFromRaw(host)
	t.Cleanup(func() { utils.SetSSRFWhitelistFromRaw("") })

	bucket := fmt.Sprintf("yuheng-it-%d", time.Now().UnixNano())
	opts := S3Options{
		Endpoint:        endpoint,
		Region:          envOr("YUHENG_TEST_S3_REGION", "us-east-1"),
		AccessKey:       envOr("YUHENG_TEST_S3_ACCESS_KEY", "rustfsadmin"),
		SecretKey:       envOr("YUHENG_TEST_S3_SECRET_KEY", "rustfsadmin"),
		BucketName:      bucket,
		PathPrefix:      "yuheng",
		AddressingStyle: os.Getenv("YUHENG_TEST_S3_ADDRESSING_STYLE"),
	}
	ctx := context.Background()

	// The bucket does not exist yet: the read-only probe says so, and testing
	// the backend creates it, as the driver would on first use.
	svc0, err := newS3Client(opts)
	if err != nil {
		t.Fatalf("newS3Client() error = %v", err)
	}
	if err := svc0.CheckConnectivity(ctx); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("CheckConnectivity on a missing bucket: err = %v, want a does-not-exist error", err)
	}
	if err := PrepareS3Backend(ctx, opts); err != nil {
		t.Fatalf("PrepareS3Backend() on a missing bucket: %v", err)
	}
	if err := svc0.CheckConnectivity(ctx); err != nil {
		t.Fatalf("CheckConnectivity after PrepareS3Backend: %v", err)
	}
	if err := PrepareS3Backend(ctx, opts); err != nil {
		t.Fatalf("PrepareS3Backend() on an existing bucket: %v", err)
	}
	svc, err := NewS3FileService(opts)
	if err != nil {
		t.Fatalf("NewS3FileService() error = %v", err)
	}

	const content = "hello from the s3 integration test"
	path, err := svc.SaveFile(ctx, multipartHeader(t, "note.txt", content), 42, "knowledge-1")
	if err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if !strings.HasPrefix(path, "s3://"+bucket+"/yuheng/42/knowledge-1/") {
		t.Fatalf("SaveFile() path = %q", path)
	}
	assertContent(t, svc, path, content)

	copied, err := svc.CopyFile(ctx, path, 43, "knowledge-2")
	if err != nil {
		t.Fatalf("CopyFile() error = %v", err)
	}
	assertContent(t, svc, copied, content)

	exported, err := svc.SaveBytes(ctx, []byte("exported"), 42, "export.txt", false)
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}
	assertContent(t, svc, exported, "exported")

	url, err := svc.GetFileURL(ctx, path)
	if err != nil {
		t.Fatalf("GetFileURL() error = %v", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET presigned URL: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != content {
		t.Fatalf("presigned GET = %d %q, want 200 %q", resp.StatusCode, body, content)
	}

	for _, p := range []string{path, copied, exported} {
		if err := svc.DeleteFile(ctx, p); err != nil {
			t.Fatalf("DeleteFile(%q) error = %v", p, err)
		}
	}
	if rc, err := svc.GetFile(ctx, path); err == nil {
		_ = rc.Close()
		t.Fatal("GetFile() after DeleteFile should fail")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func assertContent(t *testing.T, svc interface {
	GetFile(context.Context, string) (io.ReadCloser, error)
}, path, want string,
) {
	t.Helper()
	rc, err := svc.GetFile(context.Background(), path)
	if err != nil {
		t.Fatalf("GetFile(%q) error = %v", path, err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("content of %q = %q, want %q", path, got, want)
	}
}

func multipartHeader(t *testing.T, filename, content string) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["file"][0]
}
