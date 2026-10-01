// Package storageurl converts internal storage references into HTTP(S) URLs an
// external client can load directly.
//
// Yuheng persists files behind `resource://<handle>` references, which no
// browser or third-party app can fetch: they must otherwise call the
// authenticated `/files` proxy for every image.
//
// This package is the single implementation of the "give me a loadable link"
// translation. The HTTP API uses it, opt-in via resource_urls=public, so
// integrators (which may have no way to attach Yuheng credentials to an image
// fetch) receive ready-to-render URLs.
package storageurl

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
)

// Pattern matches every internal storage reference form: `resource://` handles,
// bare `provider://` paths (what a file service without a backend binding
// returns), and canonical `storage://<backend-id>/provider://` paths. The
// trailing character class stops at Markdown/HTML delimiters so a reference
// inside `![alt](…)` or `src="…"` is matched without them.
var Pattern = regexp.MustCompile(
	`\b(?:resource://[0-9A-Za-z_-]+|` +
		`(?:storage://[0-9A-Za-z_-]+/)?` +
		`(?:local|s3)://[^\s)\]>"]+)`,
)

// IsHTTPURL reports whether s is an http(s) URL — the only form an external
// client can fetch; any provider scheme (s3://, local://, …) is not. Scheme
// match is case-insensitive per RFC 3986 §3.1: a backend may emit an
// operator-configured host (e.g. a CDN domain) with an uppercase scheme.
func IsHTTPURL(s string) bool {
	return len(s) >= 7 && strings.EqualFold(s[:7], "http://") ||
		len(s) >= 8 && strings.EqualFold(s[:8], "https://")
}

// Resolver turns one storage reference into a URL an external client can
// fetch. interfaces.FileStore satisfies it: each reference is resolved through
// its own resource row, so a single answer may span several backends.
type Resolver interface {
	// URL returns ok=false when no fetchable URL can exist for ref.
	URL(ctx context.Context, ref string, ttl time.Duration) (string, bool, error)
}

// Rewriter replaces storage references with loadable HTTP URLs.
//
// Resolutions are memoised for the Rewriter's lifetime. Create one per request
// or per outbound message: a `resource://` handle costs one access-grant row per
// resolution, and a streamed answer repeats the same image across many chunks.
//
// Safe for concurrent use: mu covers both the memo and the resolver, which is
// itself single-threaded. Resolution therefore serialises across goroutines,
// which is what we want — two chunks naming the same image must not each pay for
// a signature.
//
// Logging policy:
//   - A successful rewrite logs the source reference at INFO. The signed URL
//     is DEBUG-only so log aggregation cannot hand out anonymously readable
//     links; operators can raise the log level when verifying reachability.
//   - Failure or no-op logs at WARN. The no-op case usually means
//     APP_EXTERNAL_URL is unset, the most common cause of "image broken in
//     my app" reports.
type Rewriter struct {
	resolver  Resolver
	logPrefix string

	mu   sync.Mutex
	memo map[string]string
}

// NewRewriter returns a Rewriter that resolves references through r. logPrefix
// tags log lines with the calling surface (for example "API"). A nil
// resolver yields a Rewriter that leaves content unchanged.
func NewRewriter(r Resolver, logPrefix string) *Rewriter {
	return &Rewriter{resolver: r, logPrefix: logPrefix, memo: make(map[string]string)}
}

// Enabled reports whether this Rewriter can actually rewrite anything.
func (w *Rewriter) Enabled() bool {
	return w != nil && w.resolver != nil
}

// String replaces every storage reference in content with an HTTP URL.
// References that are already HTTP, that no backend claims, or that resolve to
// a non-HTTP location are left untouched so the caller degrades to the
// authenticated file proxy rather than emitting an unfetchable URL.
func (w *Rewriter) String(ctx context.Context, content string) string {
	if !w.Enabled() || content == "" {
		return content
	}
	return Pattern.ReplaceAllStringFunc(content, func(ref string) string {
		return w.ref(ctx, ref)
	})
}

// Ref rewrites a value that is a bare storage reference rather than prose
// containing one — for example `MessageImage.URL`.
func (w *Rewriter) Ref(ctx context.Context, ref string) string {
	if !w.Enabled() || ref == "" {
		return ref
	}
	if !Pattern.MatchString(ref) {
		return ref
	}
	return w.String(ctx, ref)
}

// ref resolves one reference. The lock is held across resolve because it
// collapses a concurrent duplicate into one access grant instead of two.
func (w *Rewriter) ref(ctx context.Context, ref string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if cached, ok := w.memo[ref]; ok {
		return cached
	}
	resolved := w.resolve(ctx, ref)
	w.memo[ref] = resolved
	return resolved
}

func (w *Rewriter) resolve(ctx context.Context, ref string) string {
	httpURL, ok, err := w.resolver.URL(ctx, ref, 0)
	if err != nil {
		logger.Warnf(ctx, "[%s] storage URL rewrite failed: src=%s err=%v", w.logPrefix, ref, err)
		return ref
	}
	// A non-http(s) result cannot be fetched by the client. No URL at all is
	// the usual case for local storage without APP_EXTERNAL_URL, the most
	// common cause of "image broken in my app" reports.
	if !ok || !IsHTTPURL(httpURL) {
		logger.Warnf(ctx,
			"[%s] storage URL rewrite no-op (no public URL; for local storage set "+
				"APP_EXTERNAL_URL and ensure the reverse proxy forwards /r/): src=%s",
			w.logPrefix, ref)
		return ref
	}
	logger.Infof(ctx, "[%s] storage URL rewrite: src=%s", w.logPrefix, ref)
	logger.Debugf(ctx, "[%s] storage URL rewrite dst=%s", w.logPrefix, httpURL)
	return httpURL
}

// Rewrite is the one-shot form of Rewriter.String for callers that translate a
// single self-contained message.
func Rewrite(ctx context.Context, content string, r Resolver, logPrefix string) string {
	return NewRewriter(r, logPrefix).String(ctx, content)
}
