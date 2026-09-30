package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// FindingsConfig configures knowledge health: the automatic checks that run
// over a knowledge base when its content changes (see the findings service
// package). Every value comes from the environment (group C3 of .env.example).
type FindingsConfig struct {
	// Enabled schedules a check after every indexed change. Off, nothing new
	// is detected; findings already stored stay readable.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// DuplicateMinScore is the cosine similarity at which two passages of
	// different documents count as the same text.
	DuplicateMinScore float64 `yaml:"duplicate_min_score" json:"duplicate_min_score"`
}

// Defaults and bounds of the findings settings.
const (
	DefaultFindingsDuplicateMinScore = 0.95
	// MinFindingsDuplicateMinScore is the lowest similarity accepted. Below
	// it, passages that are merely about the same topic match, and every
	// knowledge base would be reported as full of duplicates.
	MinFindingsDuplicateMinScore = 0.5
)

// IsEnabled is nil-safe.
func (f *FindingsConfig) IsEnabled() bool { return f != nil && f.Enabled }

// loadFindingsConfig reads the findings environment.
//
// Unlike most settings here, a malformed value stops the server instead of
// falling back: a threshold typed as "95" or "0.3" would not fail anything
// visibly — it would silently report nothing, or report everything, and the
// operator would have no way to tell which setting was to blame.
func loadFindingsConfig() (*FindingsConfig, error) {
	f := &FindingsConfig{Enabled: true, DuplicateMinScore: DefaultFindingsDuplicateMinScore}
	if raw := strings.TrimSpace(os.Getenv("YUHENG_FINDINGS_ENABLED")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("YUHENG_FINDINGS_ENABLED=%q is not a boolean", raw)
		}
		f.Enabled = enabled
	}
	if raw := strings.TrimSpace(os.Getenv("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE")); raw != "" {
		score, err := strconv.ParseFloat(raw, 64)
		if err != nil || score < MinFindingsDuplicateMinScore || score > 1 {
			return nil, fmt.Errorf("YUHENG_FINDINGS_DUPLICATE_MIN_SCORE=%q must be a number between %.1f and 1",
				raw, MinFindingsDuplicateMinScore)
		}
		f.DuplicateMinScore = score
	}
	return f, nil
}
