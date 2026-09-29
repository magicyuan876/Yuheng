package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMinerUParseMethod(t *testing.T) {
	trueValue := true
	falseValue := false
	tests := []struct {
		name      string
		method    string
		legacyOCR *bool
		want      string
	}{
		{name: "new default", want: MinerUParseMethodAuto},
		{name: "legacy enabled", legacyOCR: &trueValue, want: MinerUParseMethodAuto},
		{name: "legacy disabled", legacyOCR: &falseValue, want: MinerUParseMethodText},
		{name: "explicit auto overrides legacy", method: "auto", legacyOCR: &falseValue, want: MinerUParseMethodAuto},
		{name: "explicit OCR is normalized", method: " OCR ", want: MinerUParseMethodOCR},
		{name: "explicit text overrides legacy", method: "txt", legacyOCR: &trueValue, want: MinerUParseMethodText},
		{name: "invalid method falls back safely", method: "invalid", want: MinerUParseMethodAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveMinerUParseMethod(tt.method, tt.legacyOCR))
		})
	}
}

func TestParserEngineConfigToOverridesMapResolvesMinerUParseMethod(t *testing.T) {
	falseValue := false
	explicit := (&ParserEngineConfig{
		MinerUParseMethod: MinerUParseMethodOCR,
		MinerUEnableOCR:   &falseValue,
	}).ToOverridesMap()
	assert.Equal(t, MinerUParseMethodOCR, explicit["mineru_parse_method"])

	legacy := (&ParserEngineConfig{MinerUEnableOCR: &falseValue}).ToOverridesMap()
	assert.Equal(t, MinerUParseMethodText, legacy["mineru_parse_method"])
}

func TestGetEffectiveEngines(t *testing.T) {
	defaultEngines := []RetrieverEngineParams{
		{RetrieverType: KeywordsRetrieverType, RetrieverEngineType: PostgresRetrieverEngineType},
	}
	SetDefaultRetrieverEngines(defaultEngines)
	t.Cleanup(func() { SetDefaultRetrieverEngines(nil) })

	t.Run("tenant engines win", func(t *testing.T) {
		own := []RetrieverEngineParams{
			{RetrieverType: VectorRetrieverType, RetrieverEngineType: RetrieverEngineType("signed")},
		}
		tenant := &Tenant{RetrieverEngines: RetrieverEngines{Engines: own}}
		assert.Equal(t, own, tenant.GetEffectiveEngines())
	})

	t.Run("defaults are used and copied", func(t *testing.T) {
		got := (&Tenant{}).GetEffectiveEngines()
		assert.Equal(t, defaultEngines, got)

		got[0].RetrieverType = VectorRetrieverType
		assert.Equal(t, KeywordsRetrieverType, (&Tenant{}).GetEffectiveEngines()[0].RetrieverType,
			"mutating a returned slice must not change the defaults")

		// The caller's own slice must not alias the stored defaults either.
		defaultEngines[0].RetrieverType = VectorRetrieverType
		assert.Equal(t, KeywordsRetrieverType, (&Tenant{}).GetEffectiveEngines()[0].RetrieverType)
	})

	t.Run("unset default returns an empty slice", func(t *testing.T) {
		SetDefaultRetrieverEngines(nil)
		got := (&Tenant{}).GetEffectiveEngines()
		assert.Empty(t, got)
	})
}
