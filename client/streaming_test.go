package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func knowledgeSSEEvent(t *testing.T, w io.Writer, resp StreamResponse, eventType string) {
	t.Helper()
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal knowledge event: %v", err)
	}
	if eventType != "" {
		fmt.Fprintf(w, "event:%s\n", eventType)
	}
	fmt.Fprintf(w, "data:%s\n\n", b)
}

func TestKnowledgeQAStream_LargeReferenceLineParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		knowledgeSSEEvent(t, w, StreamResponse{
			ResponseType: ResponseTypeReferences,
			KnowledgeReferences: []*SearchResult{{
				ID:      "c1",
				Content: strings.Repeat("a", 256*1024),
			}},
		}, "")
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var contentLen int
	err := c.KnowledgeQAStream(context.Background(), "sess", &KnowledgeQARequest{Query: "q"},
		func(e *StreamResponse) error {
			contentLen = len(e.KnowledgeReferences[0].Content)
			return nil
		})
	if err != nil {
		t.Fatalf("large knowledge stream event failed: %v", err)
	}
	if contentLen != 256*1024 {
		t.Errorf("content bytes=%d, want %d", contentLen, 256*1024)
	}
}

func TestContinueStream_LargeReferenceLineParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		knowledgeSSEEvent(t, w, StreamResponse{
			ResponseType: ResponseTypeReferences,
			KnowledgeReferences: []*SearchResult{{
				ID:      "c1",
				Content: strings.Repeat("a", 256*1024),
			}},
		}, "message")
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var contentLen int
	err := c.ContinueStream(context.Background(), "sess", "msg", func(e *StreamResponse) error {
		contentLen = len(e.KnowledgeReferences[0].Content)
		return nil
	})
	if err != nil {
		t.Fatalf("large continue-stream event failed: %v", err)
	}
	if contentLen != 256*1024 {
		t.Errorf("content bytes=%d, want %d", contentLen, 256*1024)
	}
}

// TestKnowledgeQAStream_DefaultBlanketTimeoutDoesNotCutStream is the
// regression test for doRequestStream: the client's ordinary default Timeout
// must NOT apply to SSE streams. Shrink that default to 100ms in-package so the
// test remains fast; the server completes at 300ms.
func TestKnowledgeQAStream_DefaultBlanketTimeoutDoesNotCutStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(300 * time.Millisecond)
		knowledgeSSEEvent(t, w, StreamResponse{ResponseType: ResponseTypeComplete, Done: true}, "")
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.httpClient.Timeout = 100 * time.Millisecond // simulate a short ordinary-request default
	var gotComplete bool
	err := c.KnowledgeQAStream(context.Background(), "sess", &KnowledgeQARequest{Query: "q"},
		func(e *StreamResponse) error {
			if e.ResponseType == ResponseTypeComplete {
				gotComplete = true
			}
			return nil
		})
	if err != nil {
		t.Fatalf("stream cut by blanket timeout: %v", err)
	}
	if !gotComplete {
		t.Error("complete event never arrived")
	}
}

// TestKnowledgeQAStream_ExplicitTimeoutStillApplies preserves the
// public WithTimeout contract: callers that explicitly request an upper bound
// get it for streaming calls as well as ordinary requests.
func TestKnowledgeQAStream_ExplicitTimeoutStillApplies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(300 * time.Millisecond)
		knowledgeSSEEvent(t, w, StreamResponse{ResponseType: ResponseTypeComplete, Done: true}, "")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithTimeout(100*time.Millisecond))
	err := c.KnowledgeQAStream(context.Background(), "sess", &KnowledgeQARequest{Query: "q"},
		func(*StreamResponse) error { return nil })
	if err == nil {
		t.Fatal("expected explicit WithTimeout to stop the stream")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Client.Timeout") {
		t.Errorf("err = %v, want a client-timeout error", err)
	}
}

func TestKnowledgeQAStream_TerminalErrorEndsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		knowledgeSSEEvent(t, w, StreamResponse{
			ResponseType: ResponseTypeError,
			Content:      "boom",
			Done:         true,
		}, "")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var got int
	start := time.Now()
	err := c.KnowledgeQAStream(context.Background(), "sess", &KnowledgeQARequest{Query: "q"},
		func(*StreamResponse) error {
			got++
			return nil
		})
	if err == nil || !strings.Contains(err.Error(), "SSE stream error: boom") {
		t.Fatalf("err = %v, want terminal SSE error", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("stream hung: elapsed=%v", elapsed)
	}
	if got != 1 {
		t.Errorf("callbacks=%d, want 1", got)
	}
}

// TestSearchResult_DecodesReferenceIndexes guards the KB and chunk-hierarchy
// fields used by the CLI's bounded reference projection.
func TestSearchResult_DecodesReferenceIndexes(t *testing.T) {
	raw := `{"id":"c1","knowledge_base_id":"kb1","parent_chunk_id":"p1","sub_chunk_id":["s1","s2"],"content":"x"}`
	var r SearchResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.ParentChunkID != "p1" {
		t.Errorf("ParentChunkID=%q, want p1", r.ParentChunkID)
	}
	if r.KnowledgeBaseID != "kb1" {
		t.Errorf("KnowledgeBaseID=%q, want kb1", r.KnowledgeBaseID)
	}
	if len(r.SubChunkID) != 2 || r.SubChunkID[0] != "s1" || r.SubChunkID[1] != "s2" {
		t.Errorf("SubChunkID=%v, want [s1 s2]", r.SubChunkID)
	}
}
