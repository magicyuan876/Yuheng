package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/utils"
	"gorm.io/gorm"
)

// EnvStoreIDPrefix is the prefix for virtual env store IDs.
const EnvStoreIDPrefix = "__env_"

// IsEnvStoreID checks if the given ID is an env store virtual ID.
func IsEnvStoreID(id string) bool {
	return strings.HasPrefix(id, EnvStoreIDPrefix)
}

// EnvLookupFunc is a function type for looking up environment variables.
// In production: os.Getenv, in tests: custom lookup function.
type EnvLookupFunc func(string) string

// VectorStore represents a configured vector database instance for a workspace.
// Each workspace can register multiple VectorStore entries (even of the same engine type)
// to support multi-store scenarios (e.g., a hot and a warm cluster).
type VectorStore struct {
	// Unique identifier (UUID, auto-generated)
	ID string `yaml:"id" json:"id" gorm:"type:varchar(36);primaryKey"`
	// Workspace ID for scoping
	TenantID uint64 `yaml:"tenant_id" json:"tenant_id"`
	// User-friendly name, e.g., "search-hot"
	Name string `yaml:"name" json:"name" gorm:"type:varchar(255);not null"`
	// Engine type, as named by an engine descriptor (see the retriever package)
	EngineType RetrieverEngineType `yaml:"engine_type" json:"engine_type" gorm:"type:varchar(50);not null"`
	// Driver-specific connection parameters (sensitive fields encrypted with AES-GCM)
	ConnectionConfig ConnectionConfig `yaml:"connection_config" json:"connection_config" gorm:"type:json"`
	// Optional index/collection configuration (engine-specific defaults if empty)
	IndexConfig IndexConfig `yaml:"index_config" json:"index_config" gorm:"type:json"`
	// IsBuiltin marks a platform-shared row: visible to EVERY workspace, not
	// just the owning one. Reads OR this against tenant_id; writes stay pinned
	// to the owner and, at the service layer, to system administrators, which
	// also keeps credentials out of the response for everyone else.
	//
	// Exists because tenant roles cannot express platform ownership: every
	// self-registered user is Owner of their own personal workspace, so no
	// role threshold means "only the platform operator". See the
	// governance.centralized_infra system setting.
	IsBuiltin bool `yaml:"is_builtin" json:"is_builtin" gorm:"default:false"`
	// Timestamps
	CreatedAt time.Time      `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time      `yaml:"updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `yaml:"deleted_at" json:"deleted_at" gorm:"index"`
}

// TableName returns the table name for VectorStore
func (VectorStore) TableName() string {
	return "vector_stores"
}

// BeforeCreate is a GORM hook that runs before creating a new record.
// Automatically generates a UUID for new vector stores.
func (v *VectorStore) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return nil
}

// Validate checks the fields every engine needs. Whether the engine type is
// one this deployment offers is the engine catalog's question, not the model's.
func (v *VectorStore) Validate() error {
	if v.Name == "" {
		return errors.NewValidationError("name is required")
	}
	if v.EngineType == "" {
		return errors.NewValidationError("engine_type is required")
	}
	if v.TenantID == 0 {
		return errors.NewValidationError("tenant_id is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// ConnectionConfig
// ---------------------------------------------------------------------------

// ConnectionConfig holds driver-specific connection parameters.
// Sensitive fields (Password, APIKey) are encrypted with AES-GCM at rest.
type ConnectionConfig struct {
	Addr     string `yaml:"addr" json:"addr,omitempty"`
	Username string `yaml:"username" json:"username,omitempty"`
	Password string `yaml:"password" json:"password,omitempty"` // AES-GCM encrypted
	APIKey   string `yaml:"api_key" json:"api_key,omitempty"`   // AES-GCM encrypted
	// InsecureSkipVerify disables TLS certificate verification when talking
	// to the backing store over HTTPS. Defaults to false (secure). Set it
	// only for self-signed development clusters; production deployments
	// should provide trusted certificates through the system CA pool.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecure_skip_verify,omitempty"`
	// UseDefaultConnection binds the store to the application's own
	// PostgreSQL connection instead of a separate address.
	UseDefaultConnection bool `yaml:"use_default_connection" json:"use_default_connection,omitempty"`
	// Version is the detected server version (e.g., "7.10.1", "16.2").
	// Auto-populated by TestConnection on successful connectivity check.
	Version string `yaml:"version" json:"version,omitempty"`
}

// Value implements the driver.Valuer interface.
// Encrypts Password and APIKey before persisting to database.
func (c ConnectionConfig) Value() (driver.Value, error) {
	if key := utils.GetAESKey(); key != nil {
		if c.Password != "" {
			if encrypted, err := utils.EncryptAESGCM(c.Password, key); err == nil {
				c.Password = encrypted
			}
		}
		if c.APIKey != "" {
			if encrypted, err := utils.EncryptAESGCM(c.APIKey, key); err == nil {
				c.APIKey = encrypted
			}
		}
	}
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface.
// Decrypts Password and APIKey after loading from database.
func (c *ConnectionConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	if err := json.Unmarshal(b, c); err != nil {
		return err
	}
	password, err := utils.DecryptStoredSecret(c.Password)
	if err != nil {
		return fmt.Errorf("decrypt vector store connection password: %w", err)
	}
	c.Password = password
	apiKey, err := utils.DecryptStoredSecret(c.APIKey)
	if err != nil {
		return fmt.Errorf("decrypt vector store connection api_key: %w", err)
	}
	c.APIKey = apiKey
	return nil
}

// GetEndpoint returns a normalized endpoint string for duplicate detection.
func (c ConnectionConfig) GetEndpoint() string {
	if c.Addr != "" {
		return c.Addr
	}
	if c.UseDefaultConnection {
		return "__default_postgres__"
	}
	return ""
}

// MaskSensitiveFields returns a copy with Password and APIKey replaced by the
// shared RedactedSecretPlaceholder. Empty values stay empty so the frontend
// can distinguish "set (hidden)" from "not set" without an extra flag.
func (c ConnectionConfig) MaskSensitiveFields() ConnectionConfig {
	masked := c
	if masked.Password != "" {
		masked.Password = RedactedSecretPlaceholder
	}
	if masked.APIKey != "" {
		masked.APIKey = RedactedSecretPlaceholder
	}
	return masked
}

// ---------------------------------------------------------------------------
// IndexConfig
// ---------------------------------------------------------------------------

// IndexConfig holds optional index/collection configuration for the vector store.
// If empty, engine-specific defaults are used.
type IndexConfig struct {
	IndexName        string `yaml:"index_name" json:"index_name,omitempty"`
	NumberOfShards   int    `yaml:"number_of_shards" json:"number_of_shards,omitempty"`
	NumberOfReplicas int    `yaml:"number_of_replicas" json:"number_of_replicas,omitempty"`

	// k-NN HNSW settings, for engines that build an HNSW graph themselves.
	// Zero / empty values fall back to the driver defaults.
	//
	// HNSWM is the graph degree (M); HNSWEFConstruction the candidate list size
	// while building the index; HNSWEFSearch the one while searching; KNNEngine
	// the backend ("lucene" | "faiss").
	HNSWM              int    `yaml:"hnsw_m" json:"hnsw_m,omitempty"`
	HNSWEFConstruction int    `yaml:"hnsw_ef_construction" json:"hnsw_ef_construction,omitempty"`
	HNSWEFSearch       int    `yaml:"hnsw_ef_search" json:"hnsw_ef_search,omitempty"`
	KNNEngine          string `yaml:"knn_engine" json:"knn_engine,omitempty"`
}

// Value implements the driver.Valuer interface.
func (c IndexConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface.
func (c *IndexConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// GetNumberOfShards returns the configured shard count, or def if unset.
// The receiver may be nil.
func (c *IndexConfig) GetNumberOfShards(def int) int {
	if c != nil && c.NumberOfShards > 0 {
		return c.NumberOfShards
	}
	return def
}

// GetNumberOfReplicas returns the configured number_of_replicas, or def if unset/zero.
// Note: 0 replicas cannot be distinguished from "not set" because the int field with
// json:"omitempty" omits zero values. If zero-replica support is needed in the future,
// change the field type to *int. Currently 0 is treated as "use server default".
// GetNumberOfReplicas returns the configured replica count, or def if unset.
// The receiver may be nil.
func (c *IndexConfig) GetNumberOfReplicas(def int) int {
	if c != nil && c.NumberOfReplicas > 0 {
		return c.NumberOfReplicas
	}
	return def
}

// ---------------------------------------------------------------------------
// IndexConfig — resolve helpers (for Repository layer, with env var fallback)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// IndexConfig — validation
// ---------------------------------------------------------------------------

const (
	// maxShards is the upper bound for shard-related configuration values.
	maxShards = 64
	// maxReplicas is the upper bound for replication-related configuration values.
	maxReplicas = 10
)

// validIndexNamePattern restricts index/collection names to safe characters.
// Must start with a letter, followed by alphanumeric, underscore, or hyphen. Max 128 chars.
var validIndexNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,127}$`)

// ValidateIndexConfig bounds the settings every engine shares. Settings only
// one engine understands are validated by that engine's descriptor.
func ValidateIndexConfig(ic IndexConfig) error {
	if ic.IndexName != "" && !validIndexNamePattern.MatchString(ic.IndexName) {
		return errors.NewValidationError(
			"index_name must start with a letter and contain only alphanumeric, underscore, or hyphen characters (max 128)")
	}
	if ic.NumberOfShards < 0 || ic.NumberOfShards > maxShards {
		return errors.NewValidationError(fmt.Sprintf("number_of_shards must be between 0 and %d", maxShards))
	}
	if ic.NumberOfReplicas < 0 || ic.NumberOfReplicas > maxReplicas {
		return errors.NewValidationError(fmt.Sprintf("number_of_replicas must be between 0 and %d", maxReplicas))
	}
	return nil
}

// ---------------------------------------------------------------------------
// StoreDisplay — API-safe projection embedded in other resource responses
// ---------------------------------------------------------------------------

// Vector store source classifiers used by API responses.
// Kept as package-level constants so handlers and services share a single
// vocabulary instead of repeating magic strings.
const (
	StoreSourceEnv         = "env"         // env-driven (RETRIEVE_DRIVER)
	StoreSourceUser        = "user"        // DB-managed VectorStore row
	StoreSourceUnavailable = "unavailable" // bound store row missing / registry miss
)

// StoreDisplay is the API-safe projection of a VectorStore for embedding in
// other resource responses (notably KnowledgeBase). It carries only the
// display-safe identifiers — never connection credentials.
//
// Source is one of the StoreSource* constants. EngineType is the underlying
// engine name (e.g. "opensearch"). Status mirrors Source by default but
// is split out to give the UI a stable boolean-like signal independent of
// future Source value additions.
type StoreDisplay struct {
	Name       string `json:"vector_store_name,omitempty"`
	Source     string `json:"vector_store_source,omitempty"`
	EngineType string `json:"vector_store_engine_type,omitempty"`
	Status     string `json:"vector_store_status,omitempty"` // "available" / "unavailable"
}

// DefaultStoreDisplay is the display payload for KBs that fall back to the
// tenant's env stores (VectorStoreID == nil).
func DefaultStoreDisplay() StoreDisplay {
	return StoreDisplay{
		Name:   "System default",
		Source: StoreSourceEnv,
		Status: "available",
	}
}

// UnavailableStoreDisplay is used when the bound store cannot be resolved
// (deleted row, registry miss, transient infra error). The UI can branch on
// Status to guide recovery (admin tool, rebind, etc.).
func UnavailableStoreDisplay() StoreDisplay {
	return StoreDisplay{
		Source: StoreSourceUnavailable,
		Status: "unavailable",
	}
}

// ---------------------------------------------------------------------------
// VectorStoreResponse — API response DTO
// ---------------------------------------------------------------------------

// VectorStoreResponse is the API response DTO for vector store.
// Wraps VectorStore with additional metadata (source, readonly).
type VectorStoreResponse struct {
	VectorStore
	Source   string `json:"source"`   // "env" or "user"
	ReadOnly bool   `json:"readonly"` // env stores are read-only
}

// NewVectorStoreResponse creates a response DTO from a VectorStore
// with sensitive fields masked.
func NewVectorStoreResponse(store *VectorStore, source string, readonly bool) VectorStoreResponse {
	return NewVectorStoreResponseWithSharedDetail(store, source, readonly, true)
}

// NewVectorStoreResponseWithSharedDetail is NewVectorStoreResponse plus the
// platform-sharing redaction.
//
// When the store is platform-shared (is_builtin) and the caller does not
// administer the platform, the whole connection config is dropped rather than
// merely masked: a shared store's endpoint, index name and TLS settings
// describe the operator's infrastructure, which is not the viewing workspace's
// to read. Name, engine type and status survive — that is the capability
// surface a workspace picks from, and it is what the knowledge-base editor
// renders.
//
// A shared store is also force-marked read-only so the UI never offers edit
// affordances that the API would reject.
func NewVectorStoreResponseWithSharedDetail(
	store *VectorStore, source string, readonly bool, sharedDetail bool,
) VectorStoreResponse {
	masked := *store
	masked.ConnectionConfig = store.ConnectionConfig.MaskSensitiveFields()
	if store.IsBuiltin && !sharedDetail {
		masked.ConnectionConfig = ConnectionConfig{}
		masked.IndexConfig = IndexConfig{}
		readonly = true
	}
	return VectorStoreResponse{
		VectorStore: masked,
		Source:      source,
		ReadOnly:    readonly,
	}
}

// ---------------------------------------------------------------------------
// VectorStore type metadata — for /types endpoint
// ---------------------------------------------------------------------------

// VectorStoreTypeInfo describes a supported engine type and its configuration schema.
type VectorStoreTypeInfo struct {
	Type             string                 `json:"type"`
	DisplayName      string                 `json:"display_name"`
	ConnectionFields []VectorStoreFieldInfo `json:"connection_fields"`
	IndexFields      []VectorStoreFieldInfo `json:"index_fields,omitempty"`
}

// VectorStoreFieldInfo describes a single configuration field exposed
// by /api/v1/vector-stores/types for the registration UI. The optional
// validation hints (`Immutable`, `Min`, `Max`, `Enum`) are used by both
// the frontend (to disable / constrain inputs) and the backend
// (defense-in-depth validation in the service layer).
type VectorStoreFieldInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean"
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description,omitempty"`

	// Immutable marks a field whose value cannot be changed after the
	// VectorStore is first created. The UI shows the input as read-only
	// in edit mode; the backend rejects modification attempts. Used by
	// engines whose underlying index structure is fixed at create time
	// (e.g. an HNSW engine / M / ef_construction).
	Immutable bool `json:"immutable,omitempty"`

	// Min / Max set inclusive bounds for "number"-typed fields. nil
	// means no bound on that side. Frontend uses these to constrain
	// inputs; backend re-validates as defense-in-depth.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`

	// Enum constrains the allowed string values. Empty means no
	// constraint. Used by fields whose value space is closed (e.g.
	// a knn_engine ∈ {"lucene", "faiss"}).
	Enum []string `json:"enum,omitempty"`
}
