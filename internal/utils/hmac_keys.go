package utils

import (
	"crypto/hkdf"
	"crypto/sha256"
	"os"
)

// Purposes for deriveSubkey. Each names one use of the deployment secret, so a
// signature made for one purpose can never be replayed as another and the raw
// AES key is never itself used as an HMAC key.
const subkeySystemGrant = "yuheng/system-hmac/v1"

// deriveSubkey derives a 32-byte HMAC key for purpose from SYSTEM_AES_KEY with
// HKDF-SHA256 (RFC 5869), or returns nil when the secret is unset or too short
// to be a real secret. Callers must treat nil as "this deployment cannot sign",
// not as an empty key.
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
