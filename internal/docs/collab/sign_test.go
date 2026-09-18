package collab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type signingVectors struct {
	Secret string `json:"secret"`
	Cases  []struct {
		Name      string `json:"name"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		Timestamp string `json:"timestamp"`
		Body      string `json:"body"`
		Canonical string `json:"canonical"`
		Signature string `json:"signature"`
	} `json:"cases"`
}

func loadVectors(t *testing.T) signingVectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash("testdata/signing_vectors.json"))
	require.NoError(t, err)
	var v signingVectors
	require.NoError(t, json.Unmarshal(raw, &v))
	require.NotEmpty(t, v.Cases)
	return v
}

// The vectors are shared with collab/test/signing.test.ts; if either side
// stops matching them the two implementations have drifted apart and the
// collaboration service can no longer talk to this server.
func TestSigningMatchesTheSharedVectors(t *testing.T) {
	v := loadVectors(t)
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			require.Equal(t, c.Canonical, Canonical(c.Timestamp, c.Method, c.Path, []byte(c.Body)))
			require.Equal(t, c.Signature, Sign(v.Secret, c.Timestamp, c.Method, c.Path, []byte(c.Body)))
		})
	}
}

func TestSignRequestRoundTrip(t *testing.T) {
	const secret = "a-shared-secret-of-sufficient-length"
	body := []byte(`{"page_id":"p1"}`)
	now := time.UnixMilli(1_700_000_000_000)

	req := httptest.NewRequest(http.MethodPost, "/internal/collab/store", strings.NewReader(string(body)))
	SignRequest(req, secret, body, now)
	require.NoError(t, Verify(req, secret, body, now))
	require.NoError(t, Verify(req, secret, body, now.Add(MaxSkew-time.Second)), "clock skew within the window is fine")

	require.ErrorIs(t, Verify(req, secret, body, now.Add(MaxSkew+time.Second)), ErrStale)
	require.ErrorIs(t, Verify(req, secret, append(body, ' '), now), ErrSignature, "a changed body is rejected")
	require.ErrorIs(t, Verify(req, "another-secret-of-sufficient-length", body, now), ErrSignature)

	// The query string is signed as well.
	withQuery := httptest.NewRequest(http.MethodGet, "/internal/collab/load/p1?tenant=42", nil)
	SignRequest(withQuery, secret, nil, now)
	require.NoError(t, Verify(withQuery, secret, nil, now))
	tampered := httptest.NewRequest(http.MethodGet, "/internal/collab/load/p1?tenant=43", nil)
	tampered.Header = withQuery.Header.Clone()
	require.ErrorIs(t, Verify(tampered, secret, nil, now), ErrSignature)
}

func TestVerifyRejectsMalformedHeaders(t *testing.T) {
	const secret = "a-shared-secret-of-sufficient-length"
	now := time.UnixMilli(1_700_000_000_000)

	bare := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	require.ErrorIs(t, Verify(bare, secret, nil, now), ErrMissingSignature)

	noSig := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	noSig.Header.Set(TimestampHeader, "1700000000000")
	require.ErrorIs(t, Verify(noSig, secret, nil, now), ErrMissingSignature)

	badTS := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	badTS.Header.Set(TimestampHeader, "not-a-number")
	badTS.Header.Set(SignatureHeader, "ab")
	require.ErrorIs(t, Verify(badTS, secret, nil, now), ErrMalformed)

	badSig := httptest.NewRequest(http.MethodGet, "/internal/collab/health", nil)
	badSig.Header.Set(TimestampHeader, "1700000000000")
	badSig.Header.Set(SignatureHeader, "not-hex!")
	require.ErrorIs(t, Verify(badSig, secret, nil, now), ErrMalformed)
}

func TestNewClientIsNilWithoutConfiguration(t *testing.T) {
	require.Nil(t, NewClient("", "secret", time.Second))
	require.Nil(t, NewClient("http://collab:1234", "", time.Second))
	require.False(t, (*Client)(nil).Configured())

	c := NewClient("http://collab:1234/", "a-secret", 0)
	require.NotNil(t, c)
	require.True(t, c.Configured())
	require.Equal(t, "http://collab:1234", c.baseURL, "a trailing slash would double up in paths")
}

// A nil client is what the Lite edition holds; its calls must report that
// rather than panic.
func TestNilClientReportsNotConfigured(t *testing.T) {
	var c *Client
	_, err := c.Replace(t.Context(), 1, "p1", json.RawMessage(`{}`), "restore")
	require.ErrorIs(t, err, ErrNotConfigured)
	require.ErrorIs(t, c.Evict(t.Context(), "p1"), ErrNotConfigured)
}

func TestClientSignsAndParses(t *testing.T) {
	const secret = "a-shared-secret-of-sufficient-length"
	var seen struct {
		method string
		path   string
		body   string
		signed bool
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(body)
		}
		seen.method, seen.path, seen.body = r.Method, r.URL.RequestURI(), string(body)
		seen.signed = Verify(r, secret, body, time.Now()) == nil
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ydoc_version":7}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, secret, 5*time.Second)
	res, err := c.Replace(t.Context(), 42, "page id/with slash", json.RawMessage(`{"type":"doc"}`), "restore")
	require.NoError(t, err)
	require.Equal(t, int64(7), res.YDocVersion)
	require.True(t, seen.signed, "the request carried a valid signature")
	require.Equal(t, "/internal/collab/replace/page%20id%2Fwith%20slash", seen.path, "the page id is escaped")
	require.Contains(t, seen.body, `"tenant_id":"42"`)
	require.Contains(t, seen.body, `"reason":"restore"`)

	require.NoError(t, c.Evict(t.Context(), "p1"))
	require.Equal(t, http.MethodDelete, seen.method)
	require.True(t, seen.signed)
}

func TestClientReportsServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"content rejected: unknown node"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "a-shared-secret-of-sufficient-length", time.Second)
	_, err := c.Replace(t.Context(), 1, "p1", json.RawMessage(`{}`), "import")
	require.Error(t, err)
	require.Contains(t, err.Error(), "422")
	require.Contains(t, err.Error(), "unknown node")
}
