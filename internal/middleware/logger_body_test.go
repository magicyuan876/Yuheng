package middleware

import (
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func jsonRequest(body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// A small body reaches the handler whole and is what the log line shows.
func TestLoggerCaptureSmallBody(t *testing.T) {
	body := []byte(`{"name":"kb","password":"hunter2"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = jsonRequest(body)

	capture := captureRequestBody(c)
	if capture == nil {
		t.Fatal("a JSON body was not captured")
	}
	got, err := io.ReadAll(c.Request.Body)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("downstream read = %q, %v; want the original body", got, err)
	}

	logged := formatRequestBody(capture)
	if !strings.Contains(logged, `"name":"kb"`) || strings.Contains(logged, "hunter2") {
		t.Errorf("logged body = %q; want the name kept and the password masked", logged)
	}
}

// A 10 MB body streams through untouched while the capture stays at its cap.
func TestLoggerCaptureLargeBodyStreamsThrough(t *testing.T) {
	body := append([]byte(`{"blob":"`), bytes.Repeat([]byte("a"), 10<<20)...)
	body = append(body, `"}`...)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = jsonRequest(body)

	capture := captureRequestBody(c)
	h := sha256.New()
	n, err := io.Copy(h, c.Request.Body)
	if err != nil || n != int64(len(body)) {
		t.Fatalf("downstream read %d of %d bytes, err %v", n, len(body), err)
	}
	if want := sha256.Sum256(body); !bytes.Equal(h.Sum(nil), want[:]) {
		t.Error("downstream bytes differ from the original body")
	}

	if got, more := capture.captured(); len(got) != requestCaptureLimit || !more {
		t.Errorf("captured %d bytes, more=%v; want exactly %d and more=true", len(got), more, requestCaptureLimit)
	}
	logged := formatRequestBody(capture)
	if len(logged) > maxBodySize+64 || !strings.HasSuffix(logged, "[内容过长，已截断]") {
		t.Errorf("logged %d bytes, want a truncated line of about %d", len(logged), maxBodySize)
	}
}

// A secret cut in half by the capture limit must not be printed in part.
func TestLoggerCaptureMasksSecretCutByLimit(t *testing.T) {
	head, tail := `{"pad":"`, `","password":"abc`
	prefix := head + strings.Repeat("x", requestCaptureLimit-len(head)-len(tail)) + tail
	if len(prefix) != requestCaptureLimit {
		t.Fatalf("test setup: prefix is %d bytes", len(prefix))
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = jsonRequest([]byte(prefix + `defghi"}`))
	capture := captureRequestBody(c)
	if _, err := io.ReadAll(c.Request.Body); err != nil {
		t.Fatal(err)
	}
	if logged := formatRequestBody(capture); strings.Contains(logged, "abc") {
		t.Errorf("a cut-off password leaked into the log: ...%s", logged[max(0, len(logged)-80):])
	}
}

// Uploads and other non-text bodies are not wrapped at all.
func TestLoggerDoesNotWrapBinaryBodies(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("--x"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c.Request = req
	orig := req.Body
	if captureRequestBody(c) != nil || c.Request.Body != orig {
		t.Error("a multipart body was wrapped")
	}
}

// Through the real middleware, the handler sees the whole body at both sizes.
func TestLoggerMiddlewareLeavesBodyIntact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Logger())
	r.POST("/api/v1/x", func(c *gin.Context) {
		got, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, strconv.Itoa(len(got)))
	})

	for _, size := range []int{20, 10 << 20} {
		req := jsonRequest(bytes.Repeat([]byte("a"), size))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Body.String() != strconv.Itoa(size) {
			t.Errorf("handler read %s bytes of %d", w.Body.String(), size)
		}
	}
}
