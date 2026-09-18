package types

import (
	"os"
	"testing"
)

// 32-byte AES-256 key used across encryption round-trip tests.
const testAESKey32 = "0123456789abcdef0123456789abcdef"

func withAESKey(t *testing.T, key string) {
	t.Helper()
	prev := os.Getenv("SYSTEM_AES_KEY")
	t.Setenv("SYSTEM_AES_KEY", key)
	t.Cleanup(func() { _ = os.Setenv("SYSTEM_AES_KEY", prev) })
}
