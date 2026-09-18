// Package collab is the Go half of the collaboration contract: the three
// callbacks the collaboration service calls (authenticate, load, store), the
// two calls this server makes back (replace, evict), and the HMAC signing
// that authorises both directions.
//
// The service has no database and no user table. It asks this package who a
// connection belongs to and what it may do, and hands back the Yjs state to
// persist. Everything that decides or stores lives here.
package collab

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Headers carried by every internal request in both directions.
const (
	TimestampHeader = "X-Collab-Timestamp"
	SignatureHeader = "X-Collab-Signature"
)

// MaxSkew bounds how far a request's timestamp may be from the receiver's
// clock. The same value is hard-coded in collab/src/signing.ts.
const MaxSkew = 5 * time.Minute

// Errors returned by Verify.
var (
	ErrMissingSignature = errors.New("collab: signature headers are missing")
	ErrMalformed        = errors.New("collab: signature headers are malformed")
	ErrStale            = errors.New("collab: timestamp is outside the allowed window")
	ErrSignature        = errors.New("collab: signature does not match")
)

// Canonical is the string both sides sign:
//
//	<timestamp>\n<METHOD>\n<path?query>\n<sha256-hex of the body>
//
// It must stay byte-identical to canonical() in collab/src/signing.ts.
func Canonical(timestamp, method, pathWithQuery string, body []byte) string {
	sum := sha256.Sum256(body)
	return timestamp + "\n" + method + "\n" + pathWithQuery + "\n" + hex.EncodeToString(sum[:])
}

// Sign returns the hex HMAC-SHA256 of the canonical string.
func Sign(secret, timestamp, method, pathWithQuery string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(Canonical(timestamp, method, pathWithQuery, body)))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignRequest stamps an outgoing request with the two headers.
func SignRequest(req *http.Request, secret string, body []byte, now time.Time) {
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	req.Header.Set(TimestampHeader, ts)
	req.Header.Set(SignatureHeader, Sign(secret, ts, req.Method, pathWithQuery(req), body))
}

// Verify checks an incoming request's signature and freshness.
func Verify(req *http.Request, secret string, body []byte, now time.Time) error {
	ts := req.Header.Get(TimestampHeader)
	sig := req.Header.Get(SignatureHeader)
	if ts == "" || sig == "" {
		return ErrMissingSignature
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: timestamp %q", ErrMalformed, ts)
	}
	skew := now.Sub(time.UnixMilli(ms))
	if skew < 0 {
		skew = -skew
	}
	if skew > MaxSkew {
		return ErrStale
	}
	given, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("%w: signature is not hex", ErrMalformed)
	}
	expected, err := hex.DecodeString(Sign(secret, ts, req.Method, pathWithQuery(req), body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if !hmac.Equal(given, expected) {
		return ErrSignature
	}
	return nil
}

// pathWithQuery reproduces what the signer used: the request target without
// scheme or host.
func pathWithQuery(req *http.Request) string {
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		return path + "?" + req.URL.RawQuery
	}
	return path
}
