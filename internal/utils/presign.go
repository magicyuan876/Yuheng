package utils

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// presignPath is the URL path for presigned file access.
	presignPath = "/api/v1/files/presigned"
	// presignDefaultTTL is the default validity period for presigned URLs.
	// Kept short because the HMAC key alone authorizes cross-tenant access —
	// a leaked URL should expire before it can be widely abused. Clients
	// typically fetch and cache images within seconds of receipt.
	presignDefaultTTL = 2 * time.Hour
)

// Purposes for deriveSubkey. Each names one use of the deployment secret, so a
// signature made for one purpose can never be replayed as another and the raw
// AES key is never itself used as an HMAC key.
const (
	subkeyPresign     = "yuheng/presign-hmac/v1"
	subkeySystemGrant = "yuheng/system-hmac/v1"
)

// deriveSubkey derives a 32-byte HMAC key for purpose from SYSTEM_AES_KEY with
// HKDF-SHA256 (RFC 5869), or returns nil when the secret is unset or too short
// to be a real secret. Callers must treat nil as "this deployment cannot sign",
// not as an empty key.
//
// Before this derivation the AES-256-GCM key doubled as the HMAC key. Deriving
// per-purpose subkeys means a weakness or leak in one use does not carry over to
// the other. Presigned URLs issued before the change no longer verify; they live
// for at most two hours (presignDefaultTTL), so nothing durable is affected.
func deriveSubkey(purpose string) []byte {
	secret := os.Getenv("SYSTEM_AES_KEY")
	if len(secret) < 16 {
		return nil
	}
	key, err := hkdf.Key(sha256.New, []byte(secret), nil, purpose, sha256.Size)
	if err != nil {
		return nil
	}
	return key
}

// SystemHMACKey returns the deployment-wide HMAC key for resource grants,
// derived from SYSTEM_AES_KEY (see deriveSubkey), or nil when the secret is not
// configured.
func SystemHMACKey() []byte {
	return deriveSubkey(subkeySystemGrant)
}

// getPresignKey returns the HMAC key used to sign presigned file URLs, or nil
// when the secret is not configured.
func getPresignKey() []byte {
	return deriveSubkey(subkeyPresign)
}

// signPayload computes HMAC-SHA256 over the canonical payload string.
func signPayload(key []byte, filePath string, tenantID uint64, expires int64) string {
	payload := fmt.Sprintf("file_path=%s&tenant_id=%d&expires=%d", filePath, tenantID, expires)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignFileURL generates a presigned HTTP URL for accessing a storage file.
// baseURL is the external URL of the Yuheng instance (e.g. "https://yuheng.example.com").
// filePath is the provider:// storage path (e.g. "local://1/abc/img.png").
// tenantID identifies the tenant that owns the file.
// ttl is how long the URL remains valid (0 uses the default presignDefaultTTL).
//
// Returns ("", error) if the signing key is not configured.
func SignFileURL(baseURL, filePath string, tenantID uint64, ttl time.Duration) (string, error) {
	key := getPresignKey()
	if key == nil {
		return "", fmt.Errorf("presign: SYSTEM_AES_KEY not configured")
	}
	if ttl <= 0 {
		ttl = presignDefaultTTL
	}
	expires := time.Now().Add(ttl).Unix()
	sig := signPayload(key, filePath, tenantID, expires)

	u, err := url.Parse(strings.TrimRight(baseURL, "/") + presignPath)
	if err != nil {
		return "", fmt.Errorf("presign: invalid base URL: %w", err)
	}
	q := u.Query()
	q.Set("file_path", filePath)
	q.Set("tenant_id", strconv.FormatUint(tenantID, 10))
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// VerifyFileURLSig checks the HMAC signature and expiry of a presigned URL.
// Returns true only if the signature is valid and the URL has not expired.
func VerifyFileURLSig(filePath string, tenantID uint64, expiresStr, sig string) bool {
	key := getPresignKey()
	if key == nil {
		return false
	}

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}

	// Check expiry.
	if time.Now().Unix() > expires {
		return false
	}

	// Verify signature.
	expected := signPayload(key, filePath, tenantID, expires)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// kbScopedExportsSegment is the only storage prefix served by the KB-scoped
// file proxy. Embedded wiki/chunk images land under exports/; raw knowledge
// uploads use {tenant}/{knowledgeID}/... and are served via
// /knowledge/{id}/download instead.
const kbScopedExportsSegment = "exports"

// ValidateStoragePathTenant ensures the tenant segment embedded in a provider://
// storage path matches the authenticated caller's tenant. Cross-tenant access
// for arbitrary tenant paths uses /api/v1/files/presigned with an HMAC bound to
// the resource owner; KB-scoped shared rendering uses ValidateKBScopedStoragePath.
func ValidateStoragePathTenant(filePath string, tenantID uint64) error {
	pathTenant := ParseTenantIDFromStoragePath(filePath)
	if pathTenant == 0 {
		return fmt.Errorf("storage path has no tenant segment")
	}
	if pathTenant != tenantID {
		return fmt.Errorf("storage path workspace mismatch")
	}
	return nil
}

// ValidateKBScopedStoragePath is used by GET /knowledge-bases/:id/files. It
// requires the path to belong to the KB owner tenant and to live under the
// exports/ namespace used for embedded images (SaveBytes / multimodal output).
// This prevents borrowers with shared-KB read access from using the proxy to
// fetch arbitrary owner-tenant objects such as raw knowledge uploads.
func ValidateKBScopedStoragePath(filePath string, tenantID uint64) error {
	if err := ValidateStoragePathTenant(filePath, tenantID); err != nil {
		return err
	}
	if !storagePathHasExportsScope(filePath, tenantID) {
		return fmt.Errorf("storage path is outside KB-scoped exports namespace")
	}
	return nil
}

// storageBackendScheme wraps a provider:// path with the concrete instance id:
// storage://<backendID>/<provider>://...  It is duplicated here (rather than
// reusing types.ParseStorageBackendPath) because internal/types already imports
// internal/utils, so a reverse import would create a cycle.
const storageBackendScheme = "storage://"

// unwrapStorageBackendPath strips a leading storage://<backendID>/ wrapper and
// returns the inner provider:// path. Non-wrapped paths are returned unchanged.
// This keeps tenant/exports parsing anchored on the provider path instead of
// relying on the backend id happening not to look like a tenant segment.
func unwrapStorageBackendPath(filePath string) string {
	if !strings.HasPrefix(filePath, storageBackendScheme) {
		return filePath
	}
	rest := strings.TrimPrefix(filePath, storageBackendScheme)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return filePath
	}
	return parts[1]
}

// storagePathHasExportsScope reports whether tenantID appears next to an
// exports segment: {tenant}/exports/... (local and s3).
func storagePathHasExportsScope(filePath string, tenantID uint64) bool {
	_, rest, ok := strings.Cut(unwrapStorageBackendPath(filePath), "://")
	if !ok {
		return false
	}
	tenantSeg := strconv.FormatUint(tenantID, 10)
	parts := strings.Split(rest, "/")
	for i, part := range parts {
		if part != tenantSeg {
			continue
		}
		if i+1 < len(parts) && parts[i+1] == kbScopedExportsSegment {
			return true
		}
	}
	return false
}

// ParseTenantIDFromStoragePath extracts the tenant ID from a provider:// storage path.
// Storage paths follow the convention: {scheme}://.../{tenantID}/...
// Returns 0 if the path does not contain a valid tenant ID.
//
// NOTE: For buckets or prefixes whose names are numeric and precede the tenant
// segment, the first numeric segment may not be the tenant.
// Callers that have an authoritative resource-owner tenant ID available
// should pass it directly to SignFileURL instead of relying on this parser.
func ParseTenantIDFromStoragePath(filePath string) uint64 {
	// Unwrap storage://<backendID>/ so the tenant scan is anchored on the inner
	// provider path, not the (opaque) backend id.
	filePath = unwrapStorageBackendPath(filePath)
	// Strip scheme: "local://1/abc/img.png" → "1/abc/img.png"
	_, rest, ok := strings.Cut(filePath, "://")
	if !ok {
		return 0
	}

	// Storage path layouts vary by provider:
	//   local://TENANT_ID/...
	//   s3://bucket/prefix/TENANT_ID/...
	// We try each slash-separated segment until we find a numeric tenant ID.
	parts := strings.Split(rest, "/")
	for _, part := range parts {
		if id, err := strconv.ParseUint(part, 10, 64); err == nil {
			return id
		}
	}

	return 0
}
