package wiki

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/magicyuan876/yuheng/internal/datasource/connector/feishu/core"
	"github.com/magicyuan876/yuheng/internal/types"
)

// ingestTrackingHandler is a recordingHandler that also implements
// datasource.RetryableIngestTracker, modelling the service-side handler: Emit
// never fails the stream, but items matching failFor are counted as retryable
// ingest failures (an oversized attachment rejected by the transport, docreader
// unavailable, ...).
type ingestTrackingHandler struct {
	emitted     []types.FetchedItem
	checkpoints []core.FeishuCursor
	// failFor reports whether this item counts as a retryable ingest failure.
	failFor  func(types.FetchedItem) bool
	failures int
}

func (h *ingestTrackingHandler) Emit(_ context.Context, item types.FetchedItem) error {
	h.emitted = append(h.emitted, item)
	if h.failFor != nil && h.failFor(item) {
		h.failures++
	}
	return nil
}

func (h *ingestTrackingHandler) Checkpoint(_ context.Context, cursor *types.SyncCursor) error {
	var fc core.FeishuCursor
	b, _ := json.Marshal(cursor.ConnectorCursor)
	_ = json.Unmarshal(b, &fc)
	h.checkpoints = append(h.checkpoints, fc)
	return nil
}

func (h *ingestTrackingHandler) RetryableIngestFailures() int { return h.failures }

func cursorTimes(t *testing.T, cursor *types.SyncCursor) map[string]map[string]string {
	t.Helper()
	var fc core.FeishuCursor
	b, _ := json.Marshal(cursor.ConnectorCursor)
	if err := json.Unmarshal(b, &fc); err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	return fc.SpaceNodeTimes
}

// A node whose attachment failed to ingest retryably must not advance its
// cursor entry: the parent doc's edit time never changes, so an advanced entry
// would make every later incremental sync skip the node and the attachment
// would never be retried despite the sync log promising one.
func TestFetchStream_RetryableIngestFailureWithholdsCursorAdvance(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	nodes := []core.WikiNode{
		{NodeToken: "nt1", ObjToken: "doc1", ObjType: "docx", Title: "Doc", ObjEditTime: "100"},
	}
	ts, cfg := fakeFeishuWithBlocks(nodes, "doc1", "att-1", "big.pdf", make([]byte, 4096))
	defer ts.Close()

	// Prior sync recorded an older edit time, so the node is processed now.
	cursor := makeStreamCursor(t, map[string]map[string]string{"space1": {"nt1": "50"}})

	h := &ingestTrackingHandler{failFor: func(it types.FetchedItem) bool {
		return it.FileName == "big.pdf" // the attachment fails, the doc body lands
	}}
	next, err := NewConnector(core.RegionFeishu).
		FetchStream(context.Background(), makeConfig(cfg, []string{"space1"}), cursor, h)
	if err != nil {
		t.Fatalf("FetchStream: %v", err)
	}
	if h.failures == 0 {
		t.Fatal("test setup: expected the attachment to be counted as failed")
	}

	got := cursorTimes(t, next)["space1"]["nt1"]
	if got != "50" {
		t.Fatalf("cursor entry = %q, want the prior %q so the next sync retries", got, "50")
	}
}

// The cursor is a per-node map, not a high-water mark: withholding one node's
// advance must leave every other node's entry intact, so a single failing
// document cannot stall the rest of the space.
func TestFetchStream_WithheldNodeDoesNotAffectOthers(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	nodes := []core.WikiNode{
		{NodeToken: "nt-bad", ObjToken: "doc1", ObjType: "docx", Title: "Bad", ObjEditTime: "100"},
		{NodeToken: "nt-good", ObjToken: "doc1", ObjType: "docx", Title: "Good", ObjEditTime: "200"},
	}
	ts, cfg := fakeFeishuWithBlocks(nodes, "doc1", "att-1", "big.pdf", make([]byte, 4096))
	defer ts.Close()

	cursor := makeStreamCursor(t, map[string]map[string]string{
		"space1": {"nt-bad": "50", "nt-good": "150"},
	})

	// Only the failing node's attachment fails to ingest.
	h := &ingestTrackingHandler{failFor: func(it types.FetchedItem) bool {
		return it.FileName == "big.pdf" && it.Metadata["parent_node_token"] == "nt-bad"
	}}
	next, err := NewConnector(core.RegionFeishu).
		FetchStream(context.Background(), makeConfig(cfg, []string{"space1"}), cursor, h)
	if err != nil {
		t.Fatalf("FetchStream: %v", err)
	}

	times := cursorTimes(t, next)["space1"]
	if times["nt-bad"] != "50" {
		t.Errorf("failing node = %q, want prior %q", times["nt-bad"], "50")
	}
	if times["nt-good"] != "200" {
		t.Errorf("healthy node = %q, want advanced %q — one bad node must not stall others",
			times["nt-good"], "200")
	}
}

// A node that has never synced before and fails retryably must leave NO cursor
// entry, so the next run treats it as new rather than unchanged.
func TestFetchStream_RetryableFailureOnFirstSyncLeavesNoEntry(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	nodes := []core.WikiNode{
		{NodeToken: "nt1", ObjToken: "doc1", ObjType: "docx", Title: "Doc", ObjEditTime: "100"},
	}
	ts, cfg := fakeFeishuWithBlocks(nodes, "doc1", "att-1", "big.pdf", make([]byte, 4096))
	defer ts.Close()

	h := &ingestTrackingHandler{failFor: func(it types.FetchedItem) bool {
		return it.FileName == "big.pdf"
	}}
	next, err := NewConnector(core.RegionFeishu).
		FetchStream(context.Background(), makeConfig(cfg, []string{"space1"}), nil, h)
	if err != nil {
		t.Fatalf("FetchStream: %v", err)
	}

	if entry, ok := cursorTimes(t, next)["space1"]["nt1"]; ok {
		t.Fatalf("cursor entry = %q, want none so the next sync re-fetches", entry)
	}
}

// Terminal failures are never counted by the tracker, so a node whose items all
// land (or fail terminally) advances normally — this is what stops a
// permanently broken document from re-downloading on every sync forever.
func TestFetchStream_NoRetryableFailureAdvancesCursor(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	nodes := []core.WikiNode{
		{NodeToken: "nt1", ObjToken: "doc1", ObjType: "docx", Title: "Doc", ObjEditTime: "100"},
	}
	ts, cfg := fakeFeishuWithBlocks(nodes, "doc1", "att-1", "big.pdf", make([]byte, 4096))
	defer ts.Close()

	cursor := makeStreamCursor(t, map[string]map[string]string{"space1": {"nt1": "50"}})

	h := &ingestTrackingHandler{} // failFor nil: nothing counts as retryable
	next, err := NewConnector(core.RegionFeishu).
		FetchStream(context.Background(), makeConfig(cfg, []string{"space1"}), cursor, h)
	if err != nil {
		t.Fatalf("FetchStream: %v", err)
	}

	if got := cursorTimes(t, next)["space1"]["nt1"]; got != "100" {
		t.Fatalf("cursor entry = %q, want advanced to %q", got, "100")
	}
}

// Handlers that predate RetryableIngestTracker keep the old always-advance
// behaviour: the capability is optional, not required.
func TestFetchStream_HandlerWithoutTrackerAdvancesCursor(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	nodes := []core.WikiNode{
		{NodeToken: "nt1", ObjToken: "doc1", ObjType: "docx", Title: "Doc", ObjEditTime: "100"},
	}
	ts, cfg := fakeFeishuWithBlocks(nodes, "doc1", "att-1", "big.pdf", make([]byte, 4096))
	defer ts.Close()

	cursor := makeStreamCursor(t, map[string]map[string]string{"space1": {"nt1": "50"}})

	h := &recordingHandler{} // does not implement RetryableIngestTracker
	next, err := NewConnector(core.RegionFeishu).
		FetchStream(context.Background(), makeConfig(cfg, []string{"space1"}), cursor, h)
	if err != nil {
		t.Fatalf("FetchStream: %v", err)
	}

	if got := cursorTimes(t, next)["space1"]["nt1"]; got != "100" {
		t.Fatalf("cursor entry = %q, want advanced to %q", got, "100")
	}
}
