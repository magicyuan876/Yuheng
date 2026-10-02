package service

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestIsRefreshTokenClaims(t *testing.T) {
	cases := []struct {
		name   string
		claims jwt.MapClaims
		want   bool
	}{
		{name: "refresh", claims: jwt.MapClaims{"type": "refresh"}, want: true},
		{name: "access", claims: jwt.MapClaims{"type": "access"}, want: false},
		{name: "missing type", claims: jwt.MapClaims{"user_id": "u1"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRefreshTokenClaims(tc.claims); got != tc.want {
				t.Fatalf("isRefreshTokenClaims(%v) = %v, want %v", tc.claims, got, tc.want)
			}
		})
	}
}

// tenantIDFromClaims is the JWT->tenant-id projection used by
// userService.ValidateToken. It must:
//   - return the claim when present (so /auth/switch-tenant takes effect)
//   - return 0 when the claim is absent: a tenantless session, which the
//     auth middleware resolves from the user's memberships
//   - treat zero / negative / malformed claim values as absent rather than
//     scoping the session to "tenant 0"
//
// These tests are pure-function so they don't need any DB/repo plumbing.
func TestTenantIDFromClaims(t *testing.T) {
	cases := []struct {
		name   string
		claims jwt.MapClaims
		want   uint64
	}{
		{name: "json number claim", claims: jwt.MapClaims{"tenant_id": float64(99)}, want: 99},
		{name: "missing claim is tenantless", claims: jwt.MapClaims{"user_id": "u1"}, want: 0},
		{name: "zero claim treated as missing", claims: jwt.MapClaims{"tenant_id": float64(0)}, want: 0},
		{name: "negative claim treated as missing", claims: jwt.MapClaims{"tenant_id": float64(-3)}, want: 0},
		{name: "string claim treated as missing", claims: jwt.MapClaims{"tenant_id": "12"}, want: 0},
		{name: "int64 claim accepted (test-built tokens)", claims: jwt.MapClaims{"tenant_id": int64(42)}, want: 42},
		{name: "uint64 claim accepted", claims: jwt.MapClaims{"tenant_id": uint64(123)}, want: 123},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tenantIDFromClaims(tc.claims); got != tc.want {
				t.Fatalf("tenantIDFromClaims(%v) = %d, want %d", tc.claims, got, tc.want)
			}
		})
	}
}
