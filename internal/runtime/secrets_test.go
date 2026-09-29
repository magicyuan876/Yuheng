package runtime

import (
	"context"
	"strings"
	"testing"
)

// goodJWT and goodAES are valid secrets: 64 hex characters and exactly 32 bytes.
const (
	goodJWT = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	goodAES = "0123456789abcdef0123456789abcdef"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestValidateSecrets(t *testing.T) {
	const (
		exJWT  = "yuheng-jwt-secret"
		exAES  = "yuheng-system-aes-key-32bytes!!"
		exGRPC = "your-secret-token-at-least-16-bytes"
	)
	// e builds an environment from key/value pairs.
	e := func(kv ...string) map[string]string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	good := []string{"JWT_SECRET", goodJWT, "SYSTEM_AES_KEY", goodAES}
	with := func(extra ...string) map[string]string { return e(append(append([]string{}, good...), extra...)...) }
	both := []string{"JWT_SECRET", "SYSTEM_AES_KEY"}

	cases := []struct {
		name      string
		env       map[string]string
		wantNames []string // secrets that must be named in the error; nil means no error
	}{
		{"valid", with(), nil},
		{"both empty", e(), both},
		{"jwt empty", e("SYSTEM_AES_KEY", goodAES), []string{"JWT_SECRET"}},
		{"jwt published example", with("JWT_SECRET", exJWT), []string{"JWT_SECRET"}},
		{"jwt too short", with("JWT_SECRET", strings.Repeat("a", 31)), []string{"JWT_SECRET"}},
		{"jwt exactly 32 ok", with("JWT_SECRET", strings.Repeat("a", 32)), nil},
		{"aes empty", e("JWT_SECRET", goodJWT), []string{"SYSTEM_AES_KEY"}},
		{"aes published example", with("SYSTEM_AES_KEY", exAES), []string{"SYSTEM_AES_KEY"}},
		{"aes 31 bytes", with("SYSTEM_AES_KEY", goodAES[:31]), []string{"SYSTEM_AES_KEY"}},
		{"aes 33 bytes", with("SYSTEM_AES_KEY", goodAES+"x"), []string{"SYSTEM_AES_KEY"}},
		{"grpc token unset is allowed", with(), nil},
		{"grpc token example", with("GRPC_AUTH_TOKEN", exGRPC), []string{"GRPC_AUTH_TOKEN"}},
		{"grpc token short", with("GRPC_AUTH_TOKEN", "short"), []string{"GRPC_AUTH_TOKEN"}},
		{"grpc token strong", with("GRPC_AUTH_TOKEN", strings.Repeat("t", 24)), nil},
		{"dev opt-out waves everything through", e("YUHENG_INSECURE_DEV", "true", "JWT_SECRET", exJWT), nil},
		{"dev opt-out is case-insensitive", e("YUHENG_INSECURE_DEV", "TRUE"), nil},
		{"dev opt-out must be exactly true", e("YUHENG_INSECURE_DEV", "1"), both},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSecrets(envOf(tc.env))
			if tc.wantNames == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			msg := err.Error()
			for _, name := range tc.wantNames {
				if !strings.Contains(msg, name) {
					t.Errorf("error does not name %s:\n%s", name, msg)
				}
			}
			// The message must say how to fix it and how to opt out.
			if !strings.Contains(msg, "openssl rand") {
				t.Errorf("error lacks a generation hint:\n%s", msg)
			}
			if !strings.Contains(msg, InsecureDevEnv) {
				t.Errorf("error does not mention the developer opt-out:\n%s", msg)
			}
			if strings.Count(msg, "\n") < 4 {
				t.Errorf("error is not multi-line:\n%s", msg)
			}
		})
	}
}

func TestValidateSecretsNeverEchoesValues(t *testing.T) {
	err := ValidateSecrets(envOf(map[string]string{
		"JWT_SECRET":     "super-private-but-short",
		"SYSTEM_AES_KEY": "another-private-value",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, leak := range []string{"super-private-but-short", "another-private-value"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("error leaks a secret value %q", leak)
		}
	}
}

func TestEnforceSecretsOrExit(t *testing.T) {
	code := -1
	orig := exitFunc
	exitFunc = func(c int) { code = c }
	t.Cleanup(func() { exitFunc = orig })

	t.Setenv("JWT_SECRET", "yuheng-jwt-secret")
	t.Setenv("SYSTEM_AES_KEY", "")
	t.Setenv("GRPC_AUTH_TOKEN", "")
	t.Setenv(InsecureDevEnv, "")
	enforceSecretsOrExit(context.Background())
	if code != 1 {
		t.Fatalf("unsafe secrets must exit 1, got %d", code)
	}

	code = -1
	t.Setenv(InsecureDevEnv, "true")
	enforceSecretsOrExit(context.Background())
	if code != -1 {
		t.Fatalf("dev opt-out must not exit, got %d", code)
	}

	code = -1
	t.Setenv(InsecureDevEnv, "")
	t.Setenv("JWT_SECRET", goodJWT)
	t.Setenv("SYSTEM_AES_KEY", goodAES)
	enforceSecretsOrExit(context.Background())
	if code != -1 {
		t.Fatalf("valid secrets must not exit, got %d", code)
	}
}
