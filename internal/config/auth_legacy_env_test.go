package config

import "testing"

// TestApplyAuthAndTenantDefaults_DisableRegistrationDrivesRegistrationMode
// pins down the env-vs-YAML contract for DISABLE_REGISTRATION. Without this,
// DISABLE_REGISTRATION=true would block /auth/register at the handler layer
// but leave /auth/config reporting self_serve, so the frontend would keep
// showing the (broken) Register entry. Coercing registration_mode here keeps
// both gates in sync, and matches the "env always wins over YAML" rule in
// website-docs/03-features/01-tenant-auth.md §2.1.
func TestApplyAuthAndTenantDefaults_DisableRegistrationDrivesRegistrationMode(t *testing.T) {
	cases := []struct {
		name     string
		disable  string // value for DISABLE_REGISTRATION (empty == unset)
		cfgMode  string // pre-set value on cfg.Auth.RegistrationMode
		expected string
	}{
		{"true coerces empty YAML to invite_only", "true", "", AuthRegistrationModeInviteOnly},
		{"case-insensitive TRUE also coerces", "TRUE", "", AuthRegistrationModeInviteOnly},
		{"true overrides explicit self_serve YAML", "true", AuthRegistrationModeSelfServe, AuthRegistrationModeInviteOnly},
		{"true is a no-op when YAML already invite_only", "true", AuthRegistrationModeInviteOnly, AuthRegistrationModeInviteOnly},
		{"false is the explicit opt-in to open registration", "false", "", AuthRegistrationModeSelfServe},
		{"false overrides invite_only YAML", "false", AuthRegistrationModeInviteOnly, AuthRegistrationModeSelfServe},
		{"unset falls back to the auto default", "", "", AuthRegistrationModeAuto},
		{"auto is the same as unset", "auto", "", AuthRegistrationModeAuto},
		{"auto keeps invite_only YAML", "auto", AuthRegistrationModeInviteOnly, AuthRegistrationModeInviteOnly},
		{"unset keeps explicit invite_only YAML", "", AuthRegistrationModeInviteOnly, AuthRegistrationModeInviteOnly},
		{"unset keeps explicit self_serve YAML", "", AuthRegistrationModeSelfServe, AuthRegistrationModeSelfServe},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISABLE_REGISTRATION", tc.disable)
			// Other tenant env vars must not leak between cases.
			t.Setenv("YUHENG_TENANT_ENABLE_RBAC", "")

			cfg := &Config{Auth: &AuthConfig{RegistrationMode: tc.cfgMode}}
			applyAuthAndTenantDefaults(cfg)

			if cfg.Auth.RegistrationMode != tc.expected {
				t.Fatalf("registration_mode = %q, want %q", cfg.Auth.RegistrationMode, tc.expected)
			}
		})
	}
}

// TestApplyAuthAndTenantDefaults_CrossTenantAccess is a regression test for the
// env-binding gap: viper.AutomaticEnv has no SetEnvPrefix, so
// YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS is never bound to the nested struct
// automatically. applyAuthAndTenantDefaults must read it explicitly (like RBAC);
// without that, only config.yaml's enable_cross_tenant_access would take effect
// and the documented env override would be silently ignored.
func TestApplyAuthAndTenantDefaults_CrossTenantAccess(t *testing.T) {
	t.Run("environment true enables cross-tenant access", func(t *testing.T) {
		t.Setenv("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS", "true")
		cfg := &Config{Tenant: &TenantConfig{EnableCrossTenantAccess: false}}

		applyAuthAndTenantDefaults(cfg)

		if !cfg.Tenant.EnableCrossTenantAccess {
			t.Fatal("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS=true should enable cross-tenant access")
		}
	})

	t.Run("environment false overrides yaml true", func(t *testing.T) {
		t.Setenv("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS", "false")
		cfg := &Config{Tenant: &TenantConfig{EnableCrossTenantAccess: true}}

		applyAuthAndTenantDefaults(cfg)

		if cfg.Tenant.EnableCrossTenantAccess {
			t.Fatal("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS=false should disable cross-tenant access")
		}
	})

	t.Run("case-insensitive TRUE also enables", func(t *testing.T) {
		t.Setenv("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS", "TRUE")
		cfg := &Config{Tenant: &TenantConfig{EnableCrossTenantAccess: false}}

		applyAuthAndTenantDefaults(cfg)

		if !cfg.Tenant.EnableCrossTenantAccess {
			t.Fatal("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS=TRUE should enable cross-tenant access (case-insensitive)")
		}
	})

	t.Run("unset leaves yaml value untouched", func(t *testing.T) {
		t.Setenv("YUHENG_TENANT_ENABLE_CROSS_TENANT_ACCESS", "")
		cfg := &Config{Tenant: &TenantConfig{EnableCrossTenantAccess: true}}

		applyAuthAndTenantDefaults(cfg)

		if !cfg.Tenant.EnableCrossTenantAccess {
			t.Fatal("empty env should leave the YAML-provided cross-tenant access value untouched")
		}
	})
}
