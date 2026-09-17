package service

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The generic system-settings page renders one input per listed key and can
// only meaningfully edit scalars. A structured value listed there shows up as a
// lone text box asking the operator to hand-write JSON, which is worse than
// useless when a purpose-built editor already owns the field — exactly what
// happened to parser.engine_config_default before it was flagged.
//
// So: every non-scalar key must be marked EditedElsewhere. This catches the
// next one at review time instead of in a screenshot.
func TestRegistry_StructuredKeysAreNotListedGenerically(t *testing.T) {
	for key, spec := range registry {
		if spec.Type == "json" {
			assert.Truef(t, spec.EditedElsewhere,
				"%q holds a structured value, so it needs EditedElsewhere plus a real editor; "+
					"the generic settings page would render it as a raw JSON text box", key)
		}
	}
}

func TestRegistry_ParserEngineDefaultIsEditedElsewhere(t *testing.T) {
	spec, ok := registry[types.SettingKeyParserEngineDefault]
	require.True(t, ok, "the parser engine platform default must stay registered — "+
		"the resolver reads it through the settings service")
	assert.Equal(t, "json", spec.Type)
	assert.True(t, spec.EditedElsewhere,
		"edited at 设置 → 解析引擎 → 平台默认, not on the generic settings page")
	assert.True(t, spec.IsSecret, "it carries MinerU / PaddleOCR / Tianshu credentials")
}

// Storage stays in system_settings even though the listing hides it: the value
// is platform-wide state and the resolver reads it back by key. Hiding it from
// List must not make it unreadable or unwritable.
func TestRegistry_HiddenKeysRemainAddressable(t *testing.T) {
	for key, spec := range registry {
		if !spec.EditedElsewhere {
			continue
		}
		_, found := registry[key]
		assert.Truef(t, found,
			"%q is hidden from the listing but must still resolve through Get/Update", key)
		assert.NotEmptyf(t, spec.Type, "%q still needs a declared type for encodeForType", key)
	}
}

// encodeForType gained a "json" branch for the parser config. It must accept an
// object and reject the scalars the other branches handle, otherwise a typo in
// a registry entry would silently store the wrong shape.
func TestEncodeForType_JSON(t *testing.T) {
	t.Run("accepts an object", func(t *testing.T) {
		raw, err := encodeForType("json", map[string]any{
			"mineru_endpoint":       "http://mineru:8000",
			"mineru_enable_formula": true,
		})
		require.NoError(t, err)
		assert.Contains(t, string(raw), "mineru_endpoint")
	})

	t.Run("accepts an empty object", func(t *testing.T) {
		raw, err := encodeForType("json", map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "{}", string(raw))
	})

	for _, bad := range []any{"a string", 42, true, []any{"a", "list"}, nil} {
		t.Run("rejects non-object", func(t *testing.T) {
			_, err := encodeForType("json", bad)
			assert.Errorf(t, err, "value_type would stop meaning anything if %T passed", bad)
		})
	}
}
