package collab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the collaboration service. It is the only way this server
// reaches a live document: to replace a page's body (import, history
// restore, AI write-back) or to drop its connections (deletion, permission
// change).
//
// A nil Client means "no collaboration service configured" (the Lite
// edition); every method then reports ErrNotConfigured so callers can fall
// back to the exclusive-edit path instead of failing the request.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

// ErrNotConfigured is returned when no collaboration service is configured.
var ErrNotConfigured = fmt.Errorf("collab: no collaboration service is configured")

// NewClient builds a client for an HTTP base URL ("http://collab:1234").
// An empty URL yields nil, which every method handles.
func NewClient(baseURL, secret string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || secret == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{baseURL: baseURL, secret: secret, http: &http.Client{Timeout: timeout}}
}

// Configured reports whether calls can be made.
func (c *Client) Configured() bool { return c != nil }

// ReplaceResult is what the collaboration service reports after applying a
// new body.
type ReplaceResult struct {
	YDocVersion int64 `json:"ydoc_version"`
}

// Replace applies a ProseMirror document to the live page as one Yjs
// transaction, so people currently editing see the change immediately, and
// has it persisted. reason is recorded in the service's log ("import",
// "restore", "ai").
func (c *Client) Replace(ctx context.Context, tenantID uint64, pageID string, content json.RawMessage,
	reason string,
) (*ReplaceResult, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"tenant_id": fmt.Sprint(tenantID), "content": content, "reason": reason,
	})
	if err != nil {
		return nil, err
	}
	path := "/internal/collab/replace/" + url.PathEscape(pageID)
	raw, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var out ReplaceResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("collab: replace answered malformed JSON: %w", err)
	}
	return &out, nil
}

// Evict drops every connection to a page and clears the service's cached
// authorisation for it. Callers use it after deleting a page or narrowing
// its permissions; a missing collaboration service is not an error.
func (c *Client) Evict(ctx context.Context, pageID string) error {
	if c == nil {
		return ErrNotConfigured
	}
	_, err := c.do(ctx, http.MethodDelete, "/internal/collab/"+url.PathEscape(pageID), nil)
	return err
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	SignRequest(req, c.secret, body, time.Now())
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collab: %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("collab: reading the response of %s %s: %w", method, path, err)
	}
	if res.StatusCode >= 300 {
		var envelope struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &envelope)
		if envelope.Error == "" {
			envelope.Error = strings.TrimSpace(string(raw))
		}
		return nil, fmt.Errorf("collab: %s %s answered %d: %s", method, path, res.StatusCode, envelope.Error)
	}
	return raw, nil
}
