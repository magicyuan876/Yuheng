package storageallowlist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedMap_DefaultAllowsAll(t *testing.T) {
	t.Setenv(AllowListEnv, "")
	allowed := AllowedMap()
	for _, provider := range Supported() {
		assert.True(t, allowed[provider], provider)
	}
}

func TestSupported(t *testing.T) {
	assert.Equal(t, []string{"local", "s3"}, Supported())
}

func TestAllowedMap_RespectsEnv(t *testing.T) {
	t.Setenv(AllowListEnv, "s3")
	allowed := AllowedMap()
	assert.True(t, allowed["s3"])
	assert.False(t, allowed["local"])
}

func TestAllowedMap_IgnoresRemovedProviders(t *testing.T) {
	t.Setenv(AllowListEnv, "minio,cos,oss,local")
	assert.Equal(t, []string{"local"}, AllowedList())
	assert.False(t, IsAllowed("minio"))
}

func TestFirstAllowed(t *testing.T) {
	t.Setenv(AllowListEnv, "s3")
	assert.Equal(t, "s3", FirstAllowed())
}

func TestAllowedList(t *testing.T) {
	t.Setenv(AllowListEnv, "s3,local")
	assert.Equal(t, []string{"local", "s3"}, AllowedList())
}

func TestIsAllowed_EmptyProvider(t *testing.T) {
	t.Setenv(AllowListEnv, "s3")
	require.True(t, IsAllowed(""))
}
