package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// PR2 additions: env store builder, response DTO, types metadata
// ---------------------------------------------------------------------------

func TestIsEnvStoreID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{"env postgres ID", "__env_postgres__", true},
		{"env other-engine ID", "__env_signed__", true},
		{"env prefix only", "__env_", true},
		{"UUID ID", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty string", "", false},
		{"similar but not prefix", "_env_postgres__", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsEnvStoreID(tt.id))
		})
	}
}

func TestNewVectorStoreResponse(t *testing.T) {
	store := &VectorStore{
		ID:         "test-id",
		Name:       "test-store",
		EngineType: PostgresRetrieverEngineType,
		ConnectionConfig: ConnectionConfig{
			Addr:     "http://es:9200",
			Password: "secret",
			APIKey:   "my-api-key",
		},
	}

	t.Run("masks sensitive fields", func(t *testing.T) {
		resp := NewVectorStoreResponse(store, "user", false)
		assert.Equal(t, RedactedSecretPlaceholder, resp.ConnectionConfig.Password)
		assert.Equal(t, RedactedSecretPlaceholder, resp.ConnectionConfig.APIKey)
		assert.Equal(t, "http://es:9200", resp.ConnectionConfig.Addr) // non-sensitive preserved
	})

	t.Run("preserves source and readonly", func(t *testing.T) {
		resp := NewVectorStoreResponse(store, "env", true)
		assert.Equal(t, "env", resp.Source)
		assert.True(t, resp.ReadOnly)
	})

	t.Run("does not mutate original store", func(t *testing.T) {
		_ = NewVectorStoreResponse(store, "user", false)
		assert.Equal(t, "secret", store.ConnectionConfig.Password)
		assert.Equal(t, "my-api-key", store.ConnectionConfig.APIKey)
	})

	t.Run("empty sensitive fields not masked to ***", func(t *testing.T) {
		noSecret := &VectorStore{
			ID:               "test-id",
			ConnectionConfig: ConnectionConfig{Addr: "http://es:9200"},
		}
		resp := NewVectorStoreResponse(noSecret, "user", false)
		assert.Equal(t, "", resp.ConnectionConfig.Password)
		assert.Equal(t, "", resp.ConnectionConfig.APIKey)
	})
}

// testAESKey is a 32-byte key for testing AES-GCM encryption.
const testAESKey = "01234567890123456789012345678901"

// ---------------------------------------------------------------------------
// VectorStore
// ---------------------------------------------------------------------------

func TestVectorStore_Validate(t *testing.T) {
	valid := VectorStore{
		Name:       "test-store",
		EngineType: PostgresRetrieverEngineType,
		TenantID:   1,
	}

	t.Run("valid input returns nil", func(t *testing.T) {
		assert.NoError(t, valid.Validate())
	})

	t.Run("empty name returns error", func(t *testing.T) {
		s := valid
		s.Name = ""
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("empty engine type returns error", func(t *testing.T) {
		s := valid
		s.EngineType = ""
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "engine_type is required")
	})

	t.Run("engine type is not checked against a fixed list", func(t *testing.T) {
		// Which engines exist is the catalog's question, not the model's.
		s := valid
		s.EngineType = "signed"
		assert.NoError(t, s.Validate())
	})

	t.Run("zero tenant_id returns error", func(t *testing.T) {
		s := valid
		s.TenantID = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant_id is required")
	})
}

func TestVectorStore_BeforeCreate(t *testing.T) {
	t.Run("generates UUID when ID is empty", func(t *testing.T) {
		v := &VectorStore{}
		err := v.BeforeCreate(&gorm.DB{})
		require.NoError(t, err)
		assert.NotEmpty(t, v.ID)
		assert.Len(t, v.ID, 36) // UUID format: 8-4-4-4-12
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		v := &VectorStore{ID: "existing-id"}
		err := v.BeforeCreate(&gorm.DB{})
		require.NoError(t, err)
		assert.Equal(t, "existing-id", v.ID)
	})
}

func TestVectorStore_TableName(t *testing.T) {
	assert.Equal(t, "vector_stores", VectorStore{}.TableName())
}

// ---------------------------------------------------------------------------
// ConnectionConfig
// ---------------------------------------------------------------------------

func TestConnectionConfig_ValueScan(t *testing.T) {
	t.Run("encrypts password and api_key on Value, decrypts on Scan", func(t *testing.T) {
		t.Setenv("SYSTEM_AES_KEY", testAESKey)

		original := ConnectionConfig{
			Addr:     "http://es:9200",
			Username: "elastic",
			Password: "secret-pass",
			APIKey:   "sk-api-key",
		}

		// Value — encrypt
		raw, err := original.Value()
		require.NoError(t, err)

		// Verify the serialized JSON has encrypted fields
		var intermediate map[string]interface{}
		require.NoError(t, json.Unmarshal(raw.([]byte), &intermediate))
		assert.True(t, strings.HasPrefix(intermediate["password"].(string), "enc:v1:"))
		assert.True(t, strings.HasPrefix(intermediate["api_key"].(string), "enc:v1:"))
		// Non-sensitive fields remain plaintext
		assert.Equal(t, "http://es:9200", intermediate["addr"])
		assert.Equal(t, "elastic", intermediate["username"])

		// Scan — decrypt
		var scanned ConnectionConfig
		err = scanned.Scan(raw.([]byte))
		require.NoError(t, err)
		assert.Equal(t, "secret-pass", scanned.Password)
		assert.Equal(t, "sk-api-key", scanned.APIKey)
		assert.Equal(t, "http://es:9200", scanned.Addr)
		assert.Equal(t, "elastic", scanned.Username)
	})

	t.Run("skips encryption when fields are empty", func(t *testing.T) {
		t.Setenv("SYSTEM_AES_KEY", testAESKey)

		original := ConnectionConfig{Addr: "http://es:9200"}
		raw, err := original.Value()
		require.NoError(t, err)

		var intermediate map[string]interface{}
		require.NoError(t, json.Unmarshal(raw.([]byte), &intermediate))
		_, hasPassword := intermediate["password"]
		_, hasAPIKey := intermediate["api_key"]
		assert.False(t, hasPassword)
		assert.False(t, hasAPIKey)
	})

	t.Run("skips encryption when AES key is not set", func(t *testing.T) {
		t.Setenv("SYSTEM_AES_KEY", "")

		original := ConnectionConfig{
			Password: "secret-pass",
			APIKey:   "sk-api-key",
		}
		raw, err := original.Value()
		require.NoError(t, err)

		var intermediate map[string]interface{}
		require.NoError(t, json.Unmarshal(raw.([]byte), &intermediate))
		assert.Equal(t, "secret-pass", intermediate["password"])
		assert.Equal(t, "sk-api-key", intermediate["api_key"])
	})

	t.Run("does not double-encrypt already encrypted values", func(t *testing.T) {
		t.Setenv("SYSTEM_AES_KEY", testAESKey)

		original := ConnectionConfig{Password: "secret-pass"}
		raw1, err := original.Value()
		require.NoError(t, err)

		// Scan to get the encrypted form, then re-serialize
		var scanned ConnectionConfig
		require.NoError(t, json.Unmarshal(raw1.([]byte), &scanned))
		// scanned.Password is now "enc:v1:..."
		raw2, err := scanned.Value()
		require.NoError(t, err)

		// Both serialized forms should produce the same decrypted result
		var result ConnectionConfig
		require.NoError(t, result.Scan(raw2.([]byte)))
		assert.Equal(t, "secret-pass", result.Password)
	})

	t.Run("Scan nil value returns no error", func(t *testing.T) {
		var c ConnectionConfig
		assert.NoError(t, c.Scan(nil))
	})

	t.Run("Scan non-byte value returns no error", func(t *testing.T) {
		var c ConnectionConfig
		assert.NoError(t, c.Scan(42))
	})

	t.Run("original struct is not mutated by Value", func(t *testing.T) {
		t.Setenv("SYSTEM_AES_KEY", testAESKey)

		original := ConnectionConfig{Password: "secret-pass"}
		_, err := original.Value()
		require.NoError(t, err)
		assert.Equal(t, "secret-pass", original.Password, "value receiver should not mutate original")
	})
}

func TestConnectionConfig_GetEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		config   ConnectionConfig
		expected string
	}{
		{
			name:     "returns Addr when set",
			config:   ConnectionConfig{Addr: "http://es:9200"},
			expected: "http://es:9200",
		},
		{
			name:     "returns sentinel for default postgres connection",
			config:   ConnectionConfig{UseDefaultConnection: true},
			expected: "__default_postgres__",
		},
		{
			name:     "returns empty string when nothing is set",
			config:   ConnectionConfig{},
			expected: "",
		},
		{
			name:     "Addr takes precedence over the default-connection sentinel",
			config:   ConnectionConfig{Addr: "http://es:9200", UseDefaultConnection: true},
			expected: "http://es:9200",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetEndpoint())
		})
	}
}

func TestConnectionConfig_MaskSensitiveFields(t *testing.T) {
	t.Run("masks password and api_key", func(t *testing.T) {
		c := ConnectionConfig{
			Addr:     "http://es:9200",
			Username: "elastic",
			Password: "secret-pass",
			APIKey:   "sk-api-key",
		}
		masked := c.MaskSensitiveFields()
		assert.Equal(t, RedactedSecretPlaceholder, masked.Password)
		assert.Equal(t, RedactedSecretPlaceholder, masked.APIKey)
		assert.Equal(t, "http://es:9200", masked.Addr)
		assert.Equal(t, "elastic", masked.Username)
	})

	t.Run("does not mask empty fields", func(t *testing.T) {
		c := ConnectionConfig{Addr: "http://es:9200"}
		masked := c.MaskSensitiveFields()
		assert.Empty(t, masked.Password)
		assert.Empty(t, masked.APIKey)
	})

	t.Run("does not mutate original", func(t *testing.T) {
		c := ConnectionConfig{Password: "secret-pass", APIKey: "sk-api-key"}
		_ = c.MaskSensitiveFields()
		assert.Equal(t, "secret-pass", c.Password)
		assert.Equal(t, "sk-api-key", c.APIKey)
	})

	t.Run("preserves insecure_skip_verify as a visible operator-facing knob", func(t *testing.T) {
		// InsecureSkipVerify is deliberately NOT redacted: operators
		// must be able to see whether TLS verification is disabled.
		// This pin prevents a well-meaning future change from quietly
		// masking the flag and hiding a misconfiguration.
		c := ConnectionConfig{
			Addr:               "https://os:9200",
			Password:           "secret-pass",
			InsecureSkipVerify: true,
		}
		masked := c.MaskSensitiveFields()
		assert.Equal(t, RedactedSecretPlaceholder, masked.Password)
		assert.True(t, masked.InsecureSkipVerify,
			"InsecureSkipVerify must remain visible after masking")
	})
}

// ---------------------------------------------------------------------------
// IndexConfig
// ---------------------------------------------------------------------------

func TestIndexConfig_ValueScan(t *testing.T) {
	t.Run("round-trip serialization", func(t *testing.T) {
		original := IndexConfig{
			IndexName:        "my_index",
			NumberOfShards:   3,
			NumberOfReplicas: 1,
		}
		raw, err := original.Value()
		require.NoError(t, err)

		var scanned IndexConfig
		require.NoError(t, scanned.Scan(raw.([]byte)))
		assert.Equal(t, original, scanned)
	})

	t.Run("empty config serializes to {}", func(t *testing.T) {
		raw, err := IndexConfig{}.Value()
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(raw.([]byte)))
	})

	t.Run("Scan nil value returns no error", func(t *testing.T) {
		var c IndexConfig
		assert.NoError(t, c.Scan(nil))
	})

	t.Run("Scan non-byte value returns no error", func(t *testing.T) {
		var c IndexConfig
		assert.NoError(t, c.Scan(42))
	})
}

// ---------------------------------------------------------------------------
// IndexConfig — getter helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// IndexConfig — resolve helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// OptionalUint32
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ValidateIndexConfig
// ---------------------------------------------------------------------------

func TestValidateIndexConfig(t *testing.T) {
	t.Run("empty config is valid", func(t *testing.T) {
		assert.NoError(t, ValidateIndexConfig(IndexConfig{}))
	})

	t.Run("valid config with all fields", func(t *testing.T) {
		ic := IndexConfig{
			IndexName:        "my_index",
			NumberOfShards:   3,
			NumberOfReplicas: 1,
			HNSWM:            16,
			KNNEngine:        "lucene",
		}
		assert.NoError(t, ValidateIndexConfig(ic))
	})

	// --- Name validation ---
	t.Run("index_name with special chars rejected", func(t *testing.T) {
		ic := IndexConfig{IndexName: "my index*"}
		err := ValidateIndexConfig(ic)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index_name")
	})

	t.Run("index_name starting with number rejected", func(t *testing.T) {
		ic := IndexConfig{IndexName: "123abc"}
		err := ValidateIndexConfig(ic)
		require.Error(t, err)
	})

	t.Run("valid names with underscore and hyphen", func(t *testing.T) {
		ic := IndexConfig{
			IndexName: "my_index-v2",
		}
		assert.NoError(t, ValidateIndexConfig(ic))
	})

	// --- Numeric bounds ---
	t.Run("number_of_shards exceeds max", func(t *testing.T) {
		ic := IndexConfig{NumberOfShards: 100}
		err := ValidateIndexConfig(ic)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "number_of_shards")
	})

	t.Run("negative number_of_shards rejected", func(t *testing.T) {
		ic := IndexConfig{NumberOfShards: -1}
		err := ValidateIndexConfig(ic)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "number_of_shards")
	})

	t.Run("number_of_shards at max boundary is valid", func(t *testing.T) {
		assert.NoError(t, ValidateIndexConfig(IndexConfig{NumberOfShards: 64}))
	})

	t.Run("number_of_replicas exceeds max", func(t *testing.T) {
		err := ValidateIndexConfig(IndexConfig{NumberOfReplicas: 11})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "number_of_replicas")
	})

	t.Run("number_of_replicas at max boundary is valid", func(t *testing.T) {
		assert.NoError(t, ValidateIndexConfig(IndexConfig{NumberOfReplicas: 10}))
	})
}

// ---------------------------------------------------------------------------
// IndexConfig — scalability fields round-trip
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Phase 3 PR1 additions: ConnectionConfig.InsecureSkipVerify backward-compat
// and VectorStoreFieldInfo schema extensions (Immutable / Min / Max / Enum)
// ---------------------------------------------------------------------------

// TestConnectionConfig_InsecureSkipVerify_BackwardCompat ensures that
// ConnectionConfigs persisted before Phase 3 deserialize cleanly: the
// missing JSON field maps to false (Go zero-value). This is the
// foundational backward-compat guarantee for the schema extension.
func TestConnectionConfig_InsecureSkipVerify_BackwardCompat(t *testing.T) {
	t.Run("missing field defaults to false", func(t *testing.T) {
		legacy := `{"addr":"https://es:9200","username":"u","password":"p"}`
		var cfg ConnectionConfig
		require.NoError(t, json.Unmarshal([]byte(legacy), &cfg))
		assert.False(t, cfg.InsecureSkipVerify)
	})

	t.Run("explicit false deserializes correctly", func(t *testing.T) {
		raw := `{"addr":"https://es:9200","insecure_skip_verify":false}`
		var cfg ConnectionConfig
		require.NoError(t, json.Unmarshal([]byte(raw), &cfg))
		assert.False(t, cfg.InsecureSkipVerify)
	})

	t.Run("explicit true deserializes correctly", func(t *testing.T) {
		raw := `{"addr":"https://os:9200","insecure_skip_verify":true}`
		var cfg ConnectionConfig
		require.NoError(t, json.Unmarshal([]byte(raw), &cfg))
		assert.True(t, cfg.InsecureSkipVerify)
	})
}

// TestConnectionConfig_InsecureSkipVerify_RoundTrip verifies that the
// new field serializes correctly when set, and is omitted (omitempty)
// when false — so existing entries do not gain a new wire-format key
// after the Phase 3 schema extension lands.
func TestConnectionConfig_InsecureSkipVerify_RoundTrip(t *testing.T) {
	t.Run("true serializes the field", func(t *testing.T) {
		cfg := ConnectionConfig{Addr: "https://os:9200", InsecureSkipVerify: true}
		out, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.Contains(t, string(out), `"insecure_skip_verify":true`)
	})

	t.Run("false omits the field via omitempty", func(t *testing.T) {
		cfg := ConnectionConfig{Addr: "https://os:9200", InsecureSkipVerify: false}
		out, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.NotContains(t, string(out), `"insecure_skip_verify"`)
	})
}

// TestConnectionConfig_AESGCMRoundTrip_PreservesInsecureSkipVerify
// ensures the AES-GCM Value/Scan path round-trips the new field
// without corruption. The field is stored as plaintext (no AES-GCM)
// but travels alongside encrypted Password / APIKey.
func TestConnectionConfig_AESGCMRoundTrip_PreservesInsecureSkipVerify(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", testAESKey)

	original := ConnectionConfig{
		Addr:               "https://os:9200",
		Username:           "admin",
		Password:           "secret-pass",
		APIKey:             "sk-api-key",
		InsecureSkipVerify: true,
	}

	// Value — encrypt
	raw, err := original.Value()
	require.NoError(t, err)

	// Verify the serialized JSON has the plaintext flag alongside the
	// encrypted secret fields.
	var intermediate map[string]interface{}
	require.NoError(t, json.Unmarshal(raw.([]byte), &intermediate))
	assert.True(t, strings.HasPrefix(intermediate["password"].(string), "enc:v1:"),
		"password should still be encrypted")
	assert.Equal(t, true, intermediate["insecure_skip_verify"],
		"insecure_skip_verify should be plaintext bool")

	// Scan — decrypt secrets and preserve the flag
	var scanned ConnectionConfig
	require.NoError(t, scanned.Scan(raw.([]byte)))
	assert.Equal(t, "secret-pass", scanned.Password)
	assert.Equal(t, "sk-api-key", scanned.APIKey)
	assert.True(t, scanned.InsecureSkipVerify,
		"InsecureSkipVerify should round-trip through Value/Scan")
}

// TestVectorStoreFieldInfo_OmitemptyPreservesWire confirms that the
// four new optional fields (Immutable / Min / Max / Enum) added in
// Phase 3 PR 1 are omitted from JSON when unset, so existing
// VectorStoreFieldInfo entries (without these fields) serialize
// identically before and after the schema extension.
func TestVectorStoreFieldInfo_OmitemptyPreservesWire(t *testing.T) {
	legacy := VectorStoreFieldInfo{
		Name:        "addr",
		Type:        "string",
		Required:    true,
		Description: "URL",
		Default:     "http://localhost:9200",
	}
	out, err := json.Marshal(legacy)
	require.NoError(t, err)
	// Phase 3 new fields must all be omitted from the wire format.
	assert.NotContains(t, string(out), `"immutable"`)
	assert.NotContains(t, string(out), `"min"`)
	assert.NotContains(t, string(out), `"max"`)
	assert.NotContains(t, string(out), `"enum"`)
}

// TestVectorStoreFieldInfo_NewFieldsSerialize verifies the new fields
// appear in JSON when set — exercised by the OpenSearch IndexFields
// entry that lands in a later Phase 3 PR.
func TestVectorStoreFieldInfo_NewFieldsSerialize(t *testing.T) {
	t.Run("immutable", func(t *testing.T) {
		f := VectorStoreFieldInfo{Name: "knn_engine", Type: "string", Immutable: true}
		out, err := json.Marshal(f)
		require.NoError(t, err)
		assert.Contains(t, string(out), `"immutable":true`)
	})

	t.Run("min and max", func(t *testing.T) {
		minV, maxV := float64(4), float64(64)
		f := VectorStoreFieldInfo{Name: "hnsw_m", Type: "number", Min: &minV, Max: &maxV}
		out, err := json.Marshal(f)
		require.NoError(t, err)
		assert.Contains(t, string(out), `"min":4`)
		assert.Contains(t, string(out), `"max":64`)
	})

	t.Run("enum", func(t *testing.T) {
		f := VectorStoreFieldInfo{
			Name: "knn_engine", Type: "string",
			Enum: []string{"lucene", "faiss"},
		}
		out, err := json.Marshal(f)
		require.NoError(t, err)
		assert.Contains(t, string(out), `"enum":["lucene","faiss"]`)
	})

	t.Run("empty enum omits via omitempty", func(t *testing.T) {
		f := VectorStoreFieldInfo{Name: "addr", Type: "string", Enum: []string{}}
		out, err := json.Marshal(f)
		require.NoError(t, err)
		assert.NotContains(t, string(out), `"enum"`)
	})
}

// TestVectorStoreFieldInfo_RoundTrip ensures deserialization back into
// the struct works for all new fields, including the *float64 pointer
// distinction (nil vs explicit 0).
func TestVectorStoreFieldInfo_RoundTrip(t *testing.T) {
	t.Run("min=0 deserializes as &0, not nil", func(t *testing.T) {
		raw := `{"name":"replicas","type":"number","min":0,"max":5}`
		var f VectorStoreFieldInfo
		require.NoError(t, json.Unmarshal([]byte(raw), &f))
		require.NotNil(t, f.Min)
		assert.Equal(t, float64(0), *f.Min)
		require.NotNil(t, f.Max)
		assert.Equal(t, float64(5), *f.Max)
	})

	t.Run("missing min/max deserialize as nil", func(t *testing.T) {
		raw := `{"name":"addr","type":"string"}`
		var f VectorStoreFieldInfo
		require.NoError(t, json.Unmarshal([]byte(raw), &f))
		assert.Nil(t, f.Min)
		assert.Nil(t, f.Max)
	})

	t.Run("min and max are independent pointers", func(t *testing.T) {
		// Setting only Min must leave Max nil (and vice versa). A
		// refactor that accidentally aliases the two via a shared
		// local would still pass the previous two subtests because
		// they always set both ends.
		raw := `{"name":"upper_only","type":"number","max":10}`
		var f VectorStoreFieldInfo
		require.NoError(t, json.Unmarshal([]byte(raw), &f))
		assert.Nil(t, f.Min)
		require.NotNil(t, f.Max)
		assert.Equal(t, float64(10), *f.Max)
	})

	t.Run("nil enum omits via omitempty", func(t *testing.T) {
		// `nil` and `[]string{}` both round-trip through omitempty
		// identically — pin both shapes so a future Go encoder
		// behavior change cannot regress the wire format.
		f := VectorStoreFieldInfo{Name: "addr", Type: "string"} // Enum nil
		out, err := json.Marshal(f)
		require.NoError(t, err)
		assert.NotContains(t, string(out), `"enum"`)
	})
}

func TestIndexConfig_ShardAndReplicaGetters(t *testing.T) {
	var nilConfig *IndexConfig
	assert.Equal(t, 5, nilConfig.GetNumberOfShards(5))
	assert.Equal(t, -1, nilConfig.GetNumberOfReplicas(-1))

	unset := &IndexConfig{}
	assert.Equal(t, 5, unset.GetNumberOfShards(5))
	assert.Equal(t, -1, unset.GetNumberOfReplicas(-1))

	set := &IndexConfig{NumberOfShards: 3, NumberOfReplicas: 2}
	assert.Equal(t, 3, set.GetNumberOfShards(1))
	assert.Equal(t, 2, set.GetNumberOfReplicas(-1))

	negative := &IndexConfig{NumberOfShards: -1, NumberOfReplicas: -1}
	assert.Equal(t, 1, negative.GetNumberOfShards(1), "a negative value is treated as unset")
	assert.Equal(t, 4, negative.GetNumberOfReplicas(4))
}
