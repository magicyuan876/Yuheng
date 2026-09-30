package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMinerUParseMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
		want   string
	}{
		{name: "empty is auto", want: MinerUParseMethodAuto},
		{name: "explicit auto", method: "auto", want: MinerUParseMethodAuto},
		{name: "explicit OCR is normalized", method: " OCR ", want: MinerUParseMethodOCR},
		{name: "explicit text", method: "txt", want: MinerUParseMethodText},
		{name: "invalid method falls back safely", method: "invalid", want: MinerUParseMethodAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveMinerUParseMethod(tt.method))
		})
	}
}

func TestParserEngineConfigToOverridesMapResolvesMinerUParseMethod(t *testing.T) {
	explicit := (&ParserEngineConfig{MinerUParseMethod: " OCR "}).ToOverridesMap()
	assert.Equal(t, MinerUParseMethodOCR, explicit["mineru_parse_method"])

	unset := (&ParserEngineConfig{}).ToOverridesMap()
	assert.NotContains(t, unset, "mineru_parse_method")
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
