package utils

import "testing"

// resetFileSizeResolvers clears installed resolvers so tests are independent.
// atomic.Value cannot store nil, so store a func returning 0 — resolvedMB
// treats non-positive values as "not resolved" and falls back to env/default.
func resetFileSizeResolvers() {
	RegisterMaxFileSizeResolvers(func() int64 { return 0 }, func() int64 { return 0 })
}

func TestGetMaxFileSizeMBEnvFallback(t *testing.T) {
	resetFileSizeResolvers()
	t.Setenv("MAX_FILE_SIZE_MB", "")
	if got := GetMaxFileSizeMB(); got != 50 {
		t.Fatalf("default = %d, want 50", got)
	}
	t.Setenv("MAX_FILE_SIZE_MB", "200")
	if got := GetMaxFileSizeMB(); got != 200 {
		t.Fatalf("env = %d, want 200", got)
	}
	if got := GetMaxFileSize(); got != 200*1024*1024 {
		t.Fatalf("bytes = %d, want %d", got, 200*1024*1024)
	}
	// Garbage / non-positive env values keep the default.
	t.Setenv("MAX_FILE_SIZE_MB", "abc")
	if got := GetMaxFileSizeMB(); got != 50 {
		t.Fatalf("garbage env = %d, want 50", got)
	}
	t.Setenv("MAX_FILE_SIZE_MB", "-1")
	if got := GetMaxFileSizeMB(); got != 50 {
		t.Fatalf("negative env = %d, want 50", got)
	}
}

func TestResolverOverridesEnv(t *testing.T) {
	t.Setenv("MAX_FILE_SIZE_MB", "50")
	t.Setenv("MAX_VIDEO_FILE_SIZE_MB", "2048")

	RegisterMaxFileSizeResolvers(
		func() int64 { return 512 },
		func() int64 { return 4096 },
	)
	defer resetFileSizeResolvers()

	if got := GetMaxFileSizeMB(); got != 512 {
		t.Fatalf("resolved file MB = %d, want 512", got)
	}
	if got := GetMaxVideoFileSizeMB(); got != 4096 {
		t.Fatalf("resolved video MB = %d, want 4096", got)
	}
}

func TestResolverNonPositiveFallsBack(t *testing.T) {
	t.Setenv("MAX_FILE_SIZE_MB", "80")
	// A resolver returning a non-positive value (mis-set DB row) must not
	// zero out the limit — env/default stays authoritative.
	RegisterMaxFileSizeResolvers(func() int64 { return 0 }, nil)
	defer resetFileSizeResolvers()

	if got := GetMaxFileSizeMB(); got != 80 {
		t.Fatalf("fallback = %d, want 80", got)
	}
}

func TestRegisterNilLeavesExisting(t *testing.T) {
	RegisterMaxFileSizeResolvers(func() int64 { return 123 }, nil)
	defer resetFileSizeResolvers()

	// nil arguments must not clobber previously installed resolvers.
	RegisterMaxFileSizeResolvers(nil, nil)
	if got := GetMaxFileSizeMB(); got != 123 {
		t.Fatalf("after nil register = %d, want 123", got)
	}
}
