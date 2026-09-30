package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Only the upstream prefix is stale. The check once matched YUHENG_ itself and
// told every correctly configured deployment its settings were ignored.
func TestLegacyEnvNamesFindsOnlyTheUpstreamPrefix(t *testing.T) {
	got := legacyEnvNames([]string{
		"YUHENG_DOCS_ENABLED=true",
		legacyEnvPrefix + "TENANT_ENABLE_RBAC=true",
		"PATH=/usr/bin",
		legacyEnvPrefix + "AUTH_REGISTRATION_MODE=auto",
	})
	assert.Equal(t, []string{legacyEnvPrefix + "AUTH_REGISTRATION_MODE", legacyEnvPrefix + "TENANT_ENABLE_RBAC"}, got)
	assert.Empty(t, legacyEnvNames([]string{"YUHENG_COLLAB_URL=ws://x", "YUHENG_DOCS_ENABLED=true"}))
}
