package utils

import "testing"

func TestSystemHMACKeyIsDerived(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("SYSTEM_AES_KEY", secret)

	key := SystemHMACKey()
	if len(key) != 32 {
		t.Fatalf("derived key must be 32 bytes, got %d", len(key))
	}
	if string(key) == secret {
		t.Fatal("the AES key itself must never be used as an HMAC key")
	}
	// Deterministic: two instances of the same deployment must agree.
	if string(SystemHMACKey()) != string(key) {
		t.Fatal("derivation must be deterministic")
	}
	t.Setenv("SYSTEM_AES_KEY", "fedcba9876543210fedcba9876543210")
	if string(SystemHMACKey()) == string(key) {
		t.Fatal("a different secret must yield a different key")
	}
}

func TestSystemHMACKeyNilForShortSecret(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "short")
	if SystemHMACKey() != nil {
		t.Fatal("a too-short secret must disable signing, not yield a weak key")
	}
}
