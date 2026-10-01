package storageurl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResolver is a Resolver whose answer the test decides. With no url
// function it signs every reference onto a fixed CDN host.
type stubResolver struct {
	url   func(ref string) (string, bool, error)
	calls int
}

func (s *stubResolver) URL(_ context.Context, ref string, _ time.Duration) (string, bool, error) {
	s.calls++
	if s.url != nil {
		return s.url(ref)
	}
	return "https://cdn.example.com/" + ref, true, nil
}

// fixedURL resolves every reference to the same URL.
func fixedURL(url string) *stubResolver {
	return &stubResolver{url: func(string) (string, bool, error) { return url, true, nil }}
}

// Only resource handles are references; a storage locator in text is never
// stored content and must not be sent to the resolver.
func TestRewriter_RewritesResourceHandlesOnly(t *testing.T) {
	svc := fixedURL("https://cdn.example.com/signed.png")
	w := NewRewriter(svc, "TEST")

	in := "handle ![a](resource://xifDo7NTSL300Lp1goVutw) " +
		"locator ![b](s3://bucket/10000/exports/b.png)"
	out := w.String(context.Background(), in)

	assert.Equal(t, "handle ![a](https://cdn.example.com/signed.png) "+
		"locator ![b](s3://bucket/10000/exports/b.png)", out)
	assert.Equal(t, 1, svc.calls)
}

// An already-public URL in the answer must be left alone.
func TestRewriter_LeavesHTTPURLsAlone(t *testing.T) {
	w := NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST")
	in := "![a](https://example.com/a.png) and ![b](http://example.com/b.png)"
	assert.Equal(t, in, w.String(context.Background(), in))
}

// Emitting an unfetchable URL is worse than leaving the handle: the client can
// still fall back to the authenticated /files proxy for a handle.
func TestRewriter_NonHTTPResultIsNoOp(t *testing.T) {
	w := NewRewriter(fixedURL("s3://bucket/10000/exports/a.png"), "TEST")
	in := "![img](resource://xifDo7NTSL300Lp1goVutw)"
	assert.Equal(t, in, w.String(context.Background(), in))
}

func TestRewriter_ResolveFailureIsNoOp(t *testing.T) {
	w := NewRewriter(&stubResolver{url: func(string) (string, bool, error) {
		return "", false, errors.New("backend unreachable")
	}}, "TEST")
	in := "![img](resource://xifDo7NTSL300Lp1goVutw)"
	assert.Equal(t, in, w.String(context.Background(), in))
}

// No public URL can exist for local storage without APP_EXTERNAL_URL; the
// handle must survive so the client falls back to the proxy.
func TestRewriter_NoPublicURLIsNoOp(t *testing.T) {
	w := NewRewriter(&stubResolver{url: func(string) (string, bool, error) { return "", false, nil }}, "TEST")
	in := "![img](resource://xifDo7NTSL300Lp1goVutw)"
	assert.Equal(t, in, w.String(context.Background(), in))
}

// Uppercase schemes are valid per RFC 3986 §3.1 (e.g. an OBS_PROXY_DOMAIN
// configured as HTTPS://…) and must be substituted, not dropped.
func TestRewriter_UppercaseSchemeIsSubstituted(t *testing.T) {
	w := NewRewriter(fixedURL("HTTPS://cdn.example.com/x.png"), "TEST")
	out := w.String(context.Background(), "![img](resource://xifDo7NTSL300Lp1goVutw)")
	assert.Contains(t, out, "HTTPS://cdn.example.com/x.png")
	assert.NotContains(t, out, "resource://")
}

// Each resource:// resolution writes an access-grant row, so a repeated image
// must be resolved once per request.
func TestRewriter_MemoisesRepeatedReferences(t *testing.T) {
	svc := &stubResolver{}
	w := NewRewriter(svc, "TEST")
	ctx := context.Background()

	ref := "resource://xifDo7NTSL300Lp1goVutw"
	first := w.String(ctx, "![a]("+ref+")")
	second := w.String(ctx, "![b]("+ref+")")

	assert.Equal(t, 1, svc.calls, "the same reference must resolve once per Rewriter")
	assert.Equal(t, "https://cdn.example.com/"+ref, first[5:len(first)-1])
	assert.Contains(t, second, "https://cdn.example.com/"+ref)
}

func TestRewriter_DisabledWithoutResolver(t *testing.T) {
	w := NewRewriter(nil, "TEST")
	in := "![img](resource://xifDo7NTSL300Lp1goVutw)"
	assert.False(t, w.Enabled())
	assert.Equal(t, in, w.String(context.Background(), in))
	assert.Equal(t, in, w.Ref(context.Background(), in))
}

// Ref handles a bare reference such as MessageImage.URL, and must not touch a
// value that is not a reference at all.
func TestRewriter_Ref(t *testing.T) {
	w := NewRewriter(fixedURL("https://cdn.example.com/x.png"), "TEST")
	ctx := context.Background()

	assert.Equal(t, "https://cdn.example.com/x.png",
		w.Ref(ctx, "resource://xifDo7NTSL300Lp1goVutw"))
	assert.Equal(t, "", w.Ref(ctx, ""))
	assert.Equal(t, "data:image/png;base64,AAAA", w.Ref(ctx, "data:image/png;base64,AAAA"))
}

func TestIsHTTPURL(t *testing.T) {
	for _, s := range []string{"http://a", "https://a", "HTTP://a", "HTTPS://a"} {
		assert.True(t, IsHTTPURL(s), s)
	}
	for _, s := range []string{"", "ftp://a", "resource://abc", "local://1/a.png", "http:/"} {
		assert.False(t, IsHTTPURL(s), s)
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"", ModeHandle, false},
		{"handle", ModeHandle, false},
		{"public", ModePublic, false},
		{"  PUBLIC ", ModePublic, false},
		{"true", ModeHandle, true},
		{"signed", ModeHandle, true},
	}
	for _, tt := range tests {
		got, err := ParseMode(tt.in)
		if tt.wantErr {
			require.Error(t, err, "ParseMode(%q)", tt.in)
		} else {
			require.NoError(t, err, "ParseMode(%q)", tt.in)
		}
		assert.Equal(t, tt.want, got, "ParseMode(%q)", tt.in)
	}
}
