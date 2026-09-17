package docparser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// allowTianshuLocalhost whitelists the httptest server's loopback address for
// the SSRF-safe client used by the reader.
func allowTianshuLocalhost(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1,localhost")
	secutils.ResetSSRFWhitelistForTest()
}

// newTianshuTestReader points a reader at a fake server with fast polling.
func newTianshuTestReader(serverURL string, extra map[string]string) *MinerUTianshuReader {
	overrides := map[string]string{"mineru_tianshu_endpoint": serverURL}
	for k, v := range extra {
		overrides[k] = v
	}
	r := NewMinerUTianshuReader(overrides)
	r.pollTimeout = 5 * time.Second
	r.pollMaxInterval = 10 * time.Millisecond
	return r
}

func TestTianshuReadSubmitPollCompleted(t *testing.T) {
	allowTianshuLocalhost(t)
	var polls int32
	var gotAuth atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/submit":
			gotAuth.Store(r.Header.Get("X-API-Key"))
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("submit missing file part: %v", err)
			} else {
				file.Close()
				if header.Filename != "课件.pdf" {
					t.Errorf("upload filename = %q, want 课件.pdf", header.Filename)
				}
			}
			if got := r.FormValue("formula_enable"); got != "true" {
				t.Errorf("formula_enable = %q, want true", got)
			}
			// backend/lang/method omitted when unset so server defaults apply.
			if got := r.FormValue("backend"); got != "" {
				t.Errorf("backend = %q, want omitted", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true, "task_id": "task-1", "status": "pending",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/task-1":
			// First poll still processing, second completes.
			if atomic.AddInt32(&polls, 1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "status": "processing"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true, "status": "completed",
				"data": map[string]any{"content": "# 解析结果\n\n正文"},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, map[string]string{"mineru_tianshu_api_key": "sk-test"})
	result, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte("%PDF-1.4 fake"),
		FileName:    "课件.pdf",
		FileType:    "pdf",
	})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if result.MarkdownContent != "# 解析结果\n\n正文" {
		t.Fatalf("MarkdownContent = %q, result.Error = %q", result.MarkdownContent, result.Error)
	}
	if got := gotAuth.Load(); got != "sk-test" {
		t.Errorf("X-API-Key header = %v, want sk-test", got)
	}
	if atomic.LoadInt32(&polls) < 2 {
		t.Errorf("polls = %d, want >= 2", polls)
	}
}

func TestTianshuReadTaskFailed(t *testing.T) {
	allowTianshuLocalhost(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "task_id": "task-2", "status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "status": "failed", "error_message": "GPU OOM",
		})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, nil)
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte("x"), FileName: "a.pdf", FileType: "pdf",
	})
	if err == nil || !strings.Contains(err.Error(), "GPU OOM") {
		t.Fatalf("expected failure containing GPU OOM, got %v", err)
	}
}

func TestTianshuReadCompletedWithoutData(t *testing.T) {
	allowTianshuLocalhost(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "task_id": "task-3", "status": "pending"})
			return
		}
		// Result files cleaned up: completed, data == null.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "status": "completed", "data": nil,
			"message": "result files have been cleaned up",
		})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, nil)
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte("x"), FileName: "a.pdf", FileType: "pdf",
	})
	if err == nil || !strings.Contains(err.Error(), "no content returned") {
		t.Fatalf("expected no-content error, got %v", err)
	}
}

func TestTianshuReadSubmitRejected(t *testing.T) {
	allowTianshuLocalhost(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "queue full"})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, nil)
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte("x"), FileName: "a.pdf", FileType: "pdf",
	})
	if err == nil || !strings.Contains(err.Error(), "queue full") {
		t.Fatalf("expected submit rejection, got %v", err)
	}
}

func TestTianshuReadUnconfigured(t *testing.T) {
	allowTianshuLocalhost(t)
	reader := NewMinerUTianshuReader(map[string]string{})
	result, err := reader.Read(context.Background(), &types.ReadRequest{FileContent: []byte("x")})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected soft configuration error in result")
	}
}

func TestTianshuOverridesToOptionalFields(t *testing.T) {
	allowTianshuLocalhost(t)
	seen := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			seen <- map[string]string{
				"backend": r.FormValue("backend"),
				"lang":    r.FormValue("lang"),
				"method":  r.FormValue("method"),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "task_id": "t", "status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "status": "completed", "data": map[string]any{"content": "ok"},
		})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, map[string]string{
		"mineru_tianshu_backend":      "auto",
		"mineru_tianshu_language":     "ch",
		"mineru_tianshu_parse_method": "ocr",
	})
	if _, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte("x"), FileName: "a.pdf", FileType: "pdf",
	}); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	select {
	case got := <-seen:
		if got["backend"] != "auto" || got["lang"] != "ch" || got["method"] != "ocr" {
			t.Fatalf("form fields = %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("submit request never reached the fake server")
	}
}

// The multipart body is now assembled as prefix+file+suffix with an explicit
// Content-Length instead of one buffer holding a second copy of the file. The
// wire format must be unchanged: a real server still parses fields and file.
func TestTianshuStreamingBodyKeepsWireFormat(t *testing.T) {
	allowTianshuLocalhost(t)
	const payload = "%PDF-1.4 streamed body"
	type got struct {
		name, content, priority, contentLength string
		chunked                                bool
	}
	seen := make(chan got, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			g := got{
				priority:      r.FormValue("priority"),
				contentLength: r.Header.Get("Content-Length"),
			}
			for _, te := range r.TransferEncoding {
				if te == "chunked" {
					g.chunked = true
				}
			}
			if f, hdr, err := r.FormFile("file"); err == nil {
				defer f.Close()
				b, _ := io.ReadAll(f)
				g.name, g.content = hdr.Filename, string(b)
			}
			seen <- g
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "task_id": "t", "status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "status": "completed", "data": map[string]any{"content": "ok"},
		})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, nil)
	if _, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: []byte(payload), FileName: "报告.pdf", FileType: "pdf",
	}); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}

	select {
	case g := <-seen:
		if g.content != payload {
			t.Errorf("uploaded content = %q, want %q", g.content, payload)
		}
		if g.name != "报告.pdf" {
			t.Errorf("filename = %q, want 报告.pdf", g.name)
		}
		if g.priority != "0" {
			t.Errorf("priority = %q, want 0", g.priority)
		}
		// A known length keeps the request off chunked encoding, which some
		// gateways in front of a self-hosted service reject outright.
		if g.chunked || g.contentLength == "" {
			t.Errorf("want a fixed Content-Length, got length=%q chunked=%v", g.contentLength, g.chunked)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("submit never reached the server")
	}
}

// A server that accepts the connection but never drains the body must fail on
// the stall watchdog with an actionable message, not hang until a wall-clock
// cap. The error has to name how far the upload got.
func TestTianshuUploadStallIsReportedWithProgress(t *testing.T) {
	allowTianshuLocalhost(t)
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never read the body, never respond
	}))
	// Defers run LIFO: Close() waits for outstanding handlers, so the handler
	// must be released first or the test deadlocks on cleanup.
	defer server.Close()
	defer close(block)

	reader := newTianshuTestReader(server.URL, nil)
	reader.uploadStall = 300 * time.Millisecond
	reader.progressInterval = 50 * time.Millisecond

	// Large enough that the kernel socket buffer cannot swallow it all, so the
	// transport genuinely blocks and the watchdog has something to detect.
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: make([]byte, 8<<20), FileName: "big.pdf", FileType: "pdf",
	})
	if err == nil {
		t.Fatal("expected a stall error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "stalled") {
		t.Errorf("error = %q, want it to name the stall", msg)
	}
	if !strings.Contains(msg, "sent") {
		t.Errorf("error = %q, want it to report how much was sent", msg)
	}
}

// A slow but progressing upload must NOT be aborted: on a high-latency link a
// legitimate transfer can take minutes, and a wall-clock cap would kill it.
func TestTianshuSlowButProgressingUploadSucceeds(t *testing.T) {
	allowTianshuLocalhost(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			// Drain slowly: small reads with pauses, always making progress.
			buf := make([]byte, 4096)
			for {
				n, err := r.Body.Read(buf)
				if n > 0 {
					time.Sleep(2 * time.Millisecond)
				}
				if err != nil {
					break
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "task_id": "slow", "status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "status": "completed", "data": map[string]any{"content": "done"},
		})
	}))
	defer server.Close()

	reader := newTianshuTestReader(server.URL, nil)
	reader.uploadStall = 500 * time.Millisecond // shorter than the total transfer
	reader.progressInterval = 50 * time.Millisecond

	result, err := reader.Read(context.Background(), &types.ReadRequest{
		FileContent: make([]byte, 512<<10), FileName: "slow.pdf", FileType: "pdf",
	})
	if err != nil {
		t.Fatalf("slow upload should succeed, got: %v", err)
	}
	if result.MarkdownContent != "done" {
		t.Fatalf("MarkdownContent = %q, want done", result.MarkdownContent)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0: "0B", 512: "512B", 2048: "2.0KB",
		5 << 20: "5.0MB", 47922436: "45.7MB", 3 << 30: "3.00GB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
