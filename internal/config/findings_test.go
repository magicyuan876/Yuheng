package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindingsConfigDefaults(t *testing.T) {
	t.Setenv("YUHENG_FINDINGS_ENABLED", "")
	t.Setenv("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE", "")

	f, err := loadFindingsConfig()
	require.NoError(t, err)
	assert.True(t, f.IsEnabled(), "checks run unless switched off")
	assert.InDelta(t, 0.95, f.DuplicateMinScore, 1e-9)
}

func TestFindingsConfigReadsTheEnvironment(t *testing.T) {
	t.Setenv("YUHENG_FINDINGS_ENABLED", "false")
	t.Setenv("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE", "0.9")

	f, err := loadFindingsConfig()
	require.NoError(t, err)
	assert.False(t, f.IsEnabled())
	assert.InDelta(t, 0.9, f.DuplicateMinScore, 1e-9)

	for _, edge := range []string{"0.5", "1"} {
		t.Setenv("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE", edge)
		_, err := loadFindingsConfig()
		assert.NoError(t, err, "%s is within bounds", edge)
	}
}

// A threshold outside the range would not fail anything visibly, so it stops
// the server instead.
func TestFindingsConfigRejectsWhatWouldSilentlyMisbehave(t *testing.T) {
	cases := map[string][2]string{
		"percent instead of fraction": {"", "95"},
		"below the floor":             {"", "0.3"},
		"negative":                    {"", "-1"},
		"not a number":                {"", "high"},
		"switch not a boolean":        {"sometimes", ""},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("YUHENG_FINDINGS_ENABLED", env[0])
			t.Setenv("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE", env[1])
			_, err := loadFindingsConfig()
			assert.Error(t, err)
		})
	}

	var nilConfig *FindingsConfig
	assert.False(t, nilConfig.IsEnabled())
}
