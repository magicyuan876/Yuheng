package types

import "testing"

func TestSessionListSourceRequiresAdmin(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"", false},
		{"web", false},
		{"WEB", false},
		{"api", true},
		{"feishu", true},
		{"qqbot", true},
	}
	for _, tt := range tests {
		if got := SessionListSourceRequiresAdmin(tt.source); got != tt.want {
			t.Fatalf("SessionListSourceRequiresAdmin(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestSessionRequiresAdminConsoleRead(t *testing.T) {
	api := &Session{UserID: SessionOwnerAPITenantKeyPrefix + "1:10"}
	if !SessionRequiresAdminConsoleRead(api) {
		t.Fatal("API-key session should require admin")
	}
	apiExternalUser := &Session{UserID: SessionOwnerAPIExternalUserPrefix + "1:alice"}
	if !SessionRequiresAdminConsoleRead(apiExternalUser) {
		t.Fatal("external-user API session should require admin")
	}

	web := &Session{UserID: "alice", Title: "my chat"}
	if SessionRequiresAdminConsoleRead(web) {
		t.Fatal("personal web session should not require admin")
	}
}
