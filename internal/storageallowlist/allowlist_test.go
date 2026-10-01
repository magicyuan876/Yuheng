package storageallowlist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedMap_DefaultAllowsAll(t *testing.T) {
	t.Setenv(AllowListEnv, "")
	allowed := AllowedMap()
	for _, provider := range []string{"local", "s3"} {
		assert.True(t, allowed[provider], provider)
	}
}

func TestIsSupported(t *testing.T) {
	assert.True(t, IsSupported("local"))
	assert.True(t, IsSupported("s3"))
	assert.False(t, IsSupported("minio"))
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

func TestAllowedList(t *testing.T) {
	t.Setenv(AllowListEnv, "s3,local")
	assert.Equal(t, []string{"local", "s3"}, AllowedList())
}

func TestIsAllowed_EmptyProvider(t *testing.T) {
	t.Setenv(AllowListEnv, "s3")
	require.True(t, IsAllowed(""))
}
