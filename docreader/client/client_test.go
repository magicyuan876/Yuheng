package client

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/docreader/proto"
	"google.golang.org/grpc"
)

// These tests talk to a real docreader on port 50051 — the DocReader CI
// workflow starts one from the checked-out sources — and skip when none is
// listening. They are the one place where the committed Go stubs and the
// Python server are exercised together over the wire.
// The connection is built from this package's dial options, as the Go app
// builds it, so the TLS/token path is exercised whenever the environment
// configures it.

// An IPv4 literal, not "localhost": the workflow's readiness probe checks
// 127.0.0.1, and "localhost" resolves to ::1 first, where a connect can hang
// instead of being refused on some hosts, which would turn a running server
// into a skip.
const liveAddr = "127.0.0.1:50051"

func requireLiveDocReader(t *testing.T) proto.DocReaderClient {
	t.Helper()

	opts, err := LoadAuthConfigFromEnv().BuildDialOptions(50 * 1024 * 1024)
	if err != nil {
		t.Fatalf("build dial options: %v", err)
	}
	conn, err := grpc.NewClient(liveAddr, opts...)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := proto.NewDocReaderClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ListEngines(ctx, &proto.ListEnginesRequest{}); err != nil {
		t.Skipf("DocReader gRPC server not available at %s: %v", liveAddr, err)
	}
	return client
}

// readAll drains a ReadStream and enforces the frame order the Go app relies
// on: exactly one meta frame, and it comes first.
func readAll(t *testing.T, client proto.DocReaderClient, req *proto.ReadRequest) (
	*proto.ReadStreamMeta, []*proto.ImageRef,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := client.ReadStream(ctx, req)
	if err != nil {
		t.Fatalf("ReadStream: %v", err)
	}

	var meta *proto.ReadStreamMeta
	var images []*proto.ImageRef
	for i := 0; ; i++ {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadStream recv: %v", err)
		}
		if m := frame.GetMeta(); m != nil {
			if i != 0 || meta != nil {
				t.Fatalf("meta frame at position %d; want exactly one, first", i)
			}
			meta = m
			continue
		}
		if i == 0 {
			t.Fatalf("first frame is %T, want meta", frame.GetPayload())
		}
		if img := frame.GetImage(); img != nil {
			images = append(images, img)
		}
	}
	if meta == nil {
		t.Fatal("ReadStream sent no meta frame")
	}
	if meta.GetError() != "" {
		t.Fatalf("ReadStream reported error: %s", meta.GetError())
	}
	return meta, images
}

func TestReadStreamURL(t *testing.T) {
	client := requireLiveDocReader(t)

	meta, images := readAll(t, client, &proto.ReadRequest{Url: "https://example.com", Title: "test"})
	if meta.GetMarkdownContent() == "" {
		t.Error("expected non-empty markdown content")
	}
	t.Logf("content_len=%d images=%d", len(meta.GetMarkdownContent()), len(images))
}

func TestReadStreamFile(t *testing.T) {
	client := requireLiveDocReader(t)

	content, err := os.ReadFile("../testdata/test.md")
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	meta, images := readAll(t, client, &proto.ReadRequest{
		FileContent: content,
		FileName:    "test.md",
		FileType:    "md",
	})
	if meta.GetMarkdownContent() == "" {
		t.Error("expected non-empty markdown content")
	}
	// image_count is documented as best-effort, but when the server states a
	// count it must match what it then sends.
	if n := meta.GetImageCount(); n != 0 && int(n) != len(images) {
		t.Errorf("image_count=%d but %d image frames arrived", n, len(images))
	}
	t.Logf("content_len=%d images=%d", len(meta.GetMarkdownContent()), len(images))
}
