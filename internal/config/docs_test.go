package config

import "testing"

// The cleanup interval keeps its documented meanings through the environment:
// unset is 60 minutes, 0 is left for the cleaner to default, and a negative
// value reaches the cleaner and switches the sweeps off. It used to be read
// as a non-negative integer, so a negative value was replaced by 60.
func TestDocsCleanupIntervalFromEnv(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want int
	}{
		{"", 60},
		{"0", 0},
		{"15", 15},
		{"-1", -1},
		{"soon", 60},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("YUHENG_DOCS_CLEANUP_INTERVAL_MINUTES", tc.env)
			if got := loadDocsConfig().CleanupIntervalMinutes; got != tc.want {
				t.Errorf("YUHENG_DOCS_CLEANUP_INTERVAL_MINUTES=%q gives %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}

// Settings that cannot be negative still refuse a negative value.
func TestDocsNonNegativeSettingsRejectNegative(t *testing.T) {
	t.Setenv("YUHENG_DOCS_TRASH_RETENTION_DAYS", "-3")
	if got := loadDocsConfig().TrashRetentionDays; got != 30 {
		t.Errorf("negative trash retention gives %d, want the default 30", got)
	}
}
