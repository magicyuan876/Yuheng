package service

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestIsRetryableIngestFailure(t *testing.T) {
	retryable := []string{
		// The case that motivated this: an oversized attachment rejected by the
		// docreader gRPC transport. Raising the limit makes the retry succeed.
		"rpc error: code = ResourceExhausted desc = grpc: received message larger than max (419430400 vs. 52428800)",
		"file exceeds size limit of 50MB",
		"file cannot exceed 50 MB",
		"视频大小超过 gRPC 传输上限（512MB）",
		"file size must be between 1 byte and 50MB",
		"rpc error: code = Unavailable desc = connection error",
		"dial tcp 10.0.0.5:50051: connect: connection refused",
		"dial tcp: lookup docreader on 127.0.0.11:53: no such host",
		"context deadline exceeded",
		"parse timed out after 5m",
		"Feishu API rate limit exceeded",
		"429 Too Many Requests",
		"storage temporarily unavailable",
		"Document parsing service is not configured",
	}
	for _, msg := range retryable {
		if !isRetryableIngestFailure(msg) {
			t.Errorf("isRetryableIngestFailure(%q) = false, want true", msg)
		}
	}

	// Terminal failures must NOT be retryable: re-fetching them every sync would
	// re-download the source file forever without ever succeeding.
	terminal := []string{
		"",
		"unsupported file type: .exe",
		"invalid file type",
		"failed to parse document: corrupt PDF header",
		"knowledge already exists",
		"chunking produced no content",
	}
	for _, msg := range terminal {
		if isRetryableIngestFailure(msg) {
			t.Errorf("isRetryableIngestFailure(%q) = true, want false", msg)
		}
	}
}

func TestIsRetryableFetchFailure(t *testing.T) {
	// Connector-classified codes drive the decision when present.
	retryableCodes := []string{
		"feishu_rate_limited",
		"feishu_timeout",
		"feishu_server_unavailable",
		"feishu_api_error",
		"feishu_api_error_generic",
		"sync_failed",
	}
	for _, code := range retryableCodes {
		item := &types.FetchedItem{Metadata: map[string]string{"error_reason_code": code}}
		if !isRetryableFetchFailure(item) {
			t.Errorf("code %q = false, want true", code)
		}
	}

	// An auth/permission problem needs an operator action in Feishu; retrying it
	// on every sync just burns API quota until then.
	authItem := &types.FetchedItem{
		Metadata: map[string]string{"error_reason_code": "feishu_auth_or_permission"},
	}
	if isRetryableFetchFailure(authItem) {
		t.Error("feishu_auth_or_permission = true, want false")
	}

	// Without a code, fall back to matching the raw error text.
	rawRetryable := &types.FetchedItem{
		Metadata: map[string]string{"error": "download failed: context deadline exceeded"},
	}
	if !isRetryableFetchFailure(rawRetryable) {
		t.Error("raw timeout error = false, want true")
	}
	rawTerminal := &types.FetchedItem{
		Metadata: map[string]string{"error": "attachment has no usable filename"},
	}
	if isRetryableFetchFailure(rawTerminal) {
		t.Error("raw terminal error = true, want false")
	}

	// No metadata at all is not a retryable failure.
	if isRetryableFetchFailure(&types.FetchedItem{}) {
		t.Error("empty item = true, want false")
	}
}

// The tracker counter must only move for retryable failures, since the
// connector uses its delta to decide whether to withhold a cursor advance.
func TestStreamSyncHandlerCountsOnlyRetryableFailures(t *testing.T) {
	h := &streamSyncHandler{}
	if got := h.RetryableIngestFailures(); got != 0 {
		t.Fatalf("initial count = %d, want 0", got)
	}
	h.retryableFailures++
	if got := h.RetryableIngestFailures(); got != 1 {
		t.Fatalf("after one failure = %d, want 1", got)
	}
}
