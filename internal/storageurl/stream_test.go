package storageurl

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestFindIncompleteRef(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int // expected return; -1 means no match expected
	}{
		{
			"complete handle terminated by )",
			// The handle ends at `)`, which is a terminator, so the match
			// does not reach the end of the string.
			"![img](resource://xifDo7NTSL300Lp1goVutw)",
			-1,
		},
		{"complete handle terminated by space", "text resource://xifDo7NTSL300Lp1goVutw more text", -1},
		{"truncated handle at end", "text ![img](resource://xifDo7NT", 12},
		{"just scheme at end", "text resource://", 5},
		{"no storage reference", "just plain text http://example.com", -1},
		{"handle at very end", "resource://xifDo7NTSL300Lp1goVutw", 0},
		// Storage locators are never stored content, so they are not held.
		{"a raw locator is plain text", "text local://1/abc/im", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FindIncompleteRef(tt.in), "FindIncompleteRef(%q)", tt.in)
		})
	}
}

func TestFindIncompleteMarkdownImage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"complete image", "![img](resource://xifDo7NTSL300Lp1goVutw)", -1},
		{"complete then text", "![img](resource://xifDo7NTSL300Lp1goVutw) trailing", -1},
		{"truncated handle in image", `![知识助理"知识库"管理视图界面](resource://xifDo7NT`, 0},
		{"open paren only", "text ![alt](", 5},
		{"bare handle suffix without markdown", "text resource://xifDo7", -1},
		{"two images complete", "![a](resource://aaaabbbbccccddddeeeeff) ![b](resource://xifDo7NTSL300Lp1goVutw)", -1},
		{"first complete second incomplete", "![a](resource://aaaabbbbccccddddeeeeff) ![b](resource://xi", 40},
		{"bracket inside alt text", "![a[b]](resource://xifDo7NT", 0},
		{"destination with whitespace is prose, not a link", "![alt](see the figure below", -1},
		{
			"destination too long to be a link",
			"![alt](" + strings.Repeat("x", maxIncompleteImageBytes),
			-1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FindIncompleteMarkdownImage(tt.in), "FindIncompleteMarkdownImage(%q)", tt.in)
		})
	}
}

func TestHoldbackCutoff(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int // -1 means "expect len(in)", i.e. no holdback
	}{
		{"no holdback needed", "plain text with complete ![img](resource://xifDo7NTSL300Lp1goVutw) content", -1},
		{"truncated handle inside markdown image", "text ![img](resource://xifDo7", 5},
		{"bare truncated reference", "text resource://xifDo7", 5},
		{"unopened image destination", "prefix ![alt](", 7},
		{"empty chunk", "", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.want
			if want == -1 {
				want = len(tt.in)
			}
			assert.Equal(t, want, HoldbackCutoff(tt.in), "HoldbackCutoff(%q)", tt.in)
		})
	}
}

// A reference split across two deltas must be held back and rewritten once
// complete, never emitted as a broken fragment.
func TestStreamRewriter_HoldsSplitReference(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	ctx := context.Background()

	first := sr.Push(ctx, "answer-1", "here it is: ![img](resource://xifDo7", false, nil)
	assert.Equal(t, "here it is: ", first, "the incomplete image must be held back")

	second := sr.Push(ctx, "answer-1", "NTSL300Lp1goVutw) done", false, nil)
	assert.Equal(t, "![img](https://cdn.example.com/x.png) done", second)

	assert.Empty(t, sr.Push(ctx, "answer-1", "", true, nil))
	assert.Empty(t, sr.FlushAll(ctx), "nothing should remain held")
}

// Streams are keyed independently so interleaved events do not corrupt each
// other's holdback buffers.
func TestStreamRewriter_KeysAreIndependent(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	ctx := context.Background()

	assert.Empty(t, sr.Push(ctx, "a", "![x](resource://aaaa", false, nil))
	assert.Equal(t, "plain b", sr.Push(ctx, "b", "plain b", false, nil))
	assert.Equal(t,
		"![x](https://cdn.example.com/x.png)",
		sr.Push(ctx, "a", "bbbbccccdddddd)", false, nil),
	)
}

// A stream that ends without a terminal chunk must not silently drop the tail.
func TestStreamRewriter_FlushAllReleasesHeldTail(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	ctx := context.Background()

	meta := map[string]interface{}{"event_id": "answer-1", "is_fallback": true}
	assert.Equal(t, "tail ",
		sr.Push(ctx, "answer-1", "tail ![img](resource://partial", false, meta))
	assert.Equal(t,
		map[string]Held{"answer-1": {
			Content: "![img](https://cdn.example.com/x.png",
			Meta:    meta,
		}},
		sr.FlushAll(ctx),
		"the held tail must still reach the client, rewritten, with its metadata",
	)
}

// Text that merely looks like an unfinished image must not stall the stream:
// prose can contain a literal "](", and a URL never runs this long.
func TestStreamRewriter_LongUnclosedImageIsNotHeld(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	chunk := "![never closed](" + strings.Repeat("x", maxIncompleteImageBytes)
	assert.Equal(t, chunk, sr.Push(context.Background(), "answer-1", chunk, false, nil))
}

// Holdback must be bounded so a stream that never terminates a reference cannot
// buffer the whole answer, and the byte-based release must not split a rune.
func TestStreamRewriter_HoldbackIsBounded(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	ctx := context.Background()

	var emitted strings.Builder
	emitted.WriteString(sr.Push(ctx, "answer-1", "开头 resource://", false, nil))
	for i := 0; i < 20; i++ {
		emitted.WriteString(sr.Push(ctx, "answer-1", strings.Repeat("中", 1024), false, nil))
	}
	assert.Greater(t, emitted.Len(), 0, "bounded holdback must release the excess")
	assert.LessOrEqual(t, len(sr.held["answer-1"].content), maxHeldBytes)
	assert.True(t, utf8.ValidString(emitted.String()), "the release must not split a rune")
	assert.True(t, utf8.ValidString(sr.held["answer-1"].content), "the retained tail must stay valid")
}

// A disabled rewriter is a pass-through: the default API mode must not add
// latency or buffering.
func TestStreamRewriter_DisabledIsPassThrough(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(nil, "TEST"))
	in := "![img](resource://xifDo7"
	assert.Equal(t, in, sr.Push(context.Background(), "answer-1", in, false, nil))
	assert.False(t, sr.Enabled())
}

// Two goroutines pushing different streams share one resolver, so resolution
// must be serialised — this fails under -race if it is not.
func TestStreamRewriter_ConcurrentPushIsSafe(t *testing.T) {
	sr := NewStreamRewriter(NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST"))
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "answer-" + strconv.Itoa(i)
			for j := 0; j < 20; j++ {
				// A distinct reference per push so every call really reaches the
				// resolver instead of hitting the memo.
				ref := "s3://bucket/10000/" + strconv.Itoa(i) + "-" + strconv.Itoa(j) + ".png"
				sr.Push(ctx, key, "![x]("+ref+") ", false, nil)
			}
		}(i)
	}
	wg.Wait()
	assert.Empty(t, sr.FlushAll(ctx))
}
