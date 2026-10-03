package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/magicyuan876/yuheng/internal/utils"
	"gorm.io/gorm"
)

// Tenant represents the tenant
type Tenant struct {
	// ID
	ID uint64 `yaml:"id"                  json:"id"                  gorm:"primaryKey"`
	// Name
	Name string `yaml:"name"                json:"name"`
	// Description
	Description string `yaml:"description"         json:"description"`
	// Status
	Status string `yaml:"status"              json:"status"              gorm:"default:'active'"`
	// Retriever engines
	RetrieverEngines RetrieverEngines `yaml:"retriever_engines"   json:"retriever_engines"   gorm:"type:json"`
	// Business
	Business string `yaml:"business"            json:"business"`
	// Storage quota in bytes, covering vectors, original files, text and
	// indexes. 0 means unlimited. No GORM default: GORM omits a zero-valued
	// field with a default tag from the INSERT, which would let the
	// column's historical 10 GiB default silently replace "unlimited".
	StorageQuota int64 `yaml:"storage_quota"       json:"storage_quota"`
	// Storage used (Bytes)
	StorageUsed int64 `yaml:"storage_used"        json:"storage_used"        gorm:"default:0"`
	// Global Context configuration for this workspace (default for all sessions)
	ContextConfig *ContextConfig `yaml:"context_config"      json:"context_config"      gorm:"type:jsonb"`
	// Global WebSearch configuration for this workspace
	WebSearchConfig *WebSearchConfig `yaml:"web_search_config"   json:"web_search_config"   gorm:"type:jsonb"`
	// Parser engine config overrides (MinerU endpoint, API key, etc.). Used when parsing documents; overrides env.
	ParserEngineConfig *ParserEngineConfig `yaml:"parser_engine_config" json:"parser_engine_config" gorm:"type:jsonb"`
	// Credentials config: tenant-level third-party provider credentials.
	// See CredentialsConfig — currently an empty extension point.
	Credentials *CredentialsConfig `yaml:"credentials" json:"credentials" gorm:"type:jsonb"`
	// DefaultStorageBackendID is the backend the workspace's own writes go
	// to (chat images, session attachments, temporary documents) and the one
	// new knowledge bases and docs spaces bind to. Always set: a new workspace
	// starts on the deployment backend, id "env".
	DefaultStorageBackendID string `json:"default_storage_backend_id" gorm:"not null;default:'env'"`
	// Chat history config: knowledge base configuration for indexing and searching chat messages via vector search
	ChatHistoryConfig *ChatHistoryConfig `yaml:"chat_history_config" json:"chat_history_config" gorm:"type:jsonb"`
	// Retrieval config: global search/retrieval parameters shared by knowledge search and message search
	RetrievalConfig *RetrievalConfig `yaml:"retrieval_config" json:"retrieval_config" gorm:"type:jsonb"`
	// API principal config: controls how X-API-Key requests map to terminal principals.
	APIPrincipalConfig *APIPrincipalConfig `yaml:"api_principal_config" json:"-" gorm:"type:jsonb"`
	// Creation time
	CreatedAt time.Time `yaml:"created_at"          json:"created_at"`
	// Last updated time
	UpdatedAt time.Time `yaml:"updated_at"          json:"updated_at"`
	// Deletion time
	DeletedAt gorm.DeletedAt `yaml:"deleted_at"          json:"deleted_at"          gorm:"index"`
}

// RetrieverEngines represents the retriever engines for a tenant
type RetrieverEngines struct {
	Engines []RetrieverEngineParams `yaml:"engines" json:"engines" gorm:"type:json"`
}

// defaultRetrieverEngines is what a tenant with no engines of its own uses.
// The engine catalog knows what the deployment offers, and the model must not
// depend on it, so the application sets this once at start-up.
var defaultRetrieverEngines atomic.Pointer[[]RetrieverEngineParams]

// SetDefaultRetrieverEngines sets the engines used by tenants that configure
// none. Call it once, before serving requests.
func SetDefaultRetrieverEngines(engines []RetrieverEngineParams) {
	cp := append([]RetrieverEngineParams(nil), engines...)
	defaultRetrieverEngines.Store(&cp)
}

// GetEffectiveEngines returns the tenant's engines if configured, otherwise the
// deployment's defaults.
func (t *Tenant) GetEffectiveEngines() []RetrieverEngineParams {
	if len(t.RetrieverEngines.Engines) > 0 {
		return t.RetrieverEngines.Engines
	}
	if d := defaultRetrieverEngines.Load(); d != nil {
		return append([]RetrieverEngineParams(nil), *d...)
	}
	return []RetrieverEngineParams{}
}

// BeforeCreate is a hook function that is called before creating a tenant
func (t *Tenant) BeforeCreate(tx *gorm.DB) error {
	if t.RetrieverEngines.Engines == nil {
		t.RetrieverEngines.Engines = []RetrieverEngineParams{}
	}
	return nil
}

// Value implements the driver.Valuer interface, used to convert RetrieverEngines to database value
func (c RetrieverEngines) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface, used to convert database value to RetrieverEngines.
// It accepts both the object-wrapped format Value writes ({"engines": [...]}) and a
// bare array: the column's schema default is '[]', so a row nobody has saved yet
// still holds one.
func (c *RetrieverEngines) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}

	// Try the current object format first: {"engines": [...]}
	if err := json.Unmarshal(b, c); err == nil {
		return nil
	}

	// Bare array: the untouched column default.
	var engines []RetrieverEngineParams
	if err := json.Unmarshal(b, &engines); err != nil {
		return fmt.Errorf("retriever_engines: cannot unmarshal as object or array: %w", err)
	}
	c.Engines = engines
	return nil
}

// CredentialsConfig holds third-party provider credentials at the tenant level.
// Stored as a single JSONB column; each provider is a nested object so new
// providers can be added without schema changes.
//
// Currently empty: the one provider that used it (a hosted document/model
// service operated by the upstream project) was removed when this fork became
// independent. The type and its column are kept as the extension point they
// were designed to be — adding a provider here needs no migration, and old
// rows carrying the removed provider's JSON simply unmarshal to nothing.
type CredentialsConfig struct{}

type APIPrincipalMode string

const (
	APIPrincipalModeTenant      APIPrincipalMode = "tenant"
	APIPrincipalModeDirect      APIPrincipalMode = "direct_header"
	APIPrincipalModeSignedToken APIPrincipalMode = "signed_token"
)

// APIPrincipalConfig controls how tenant API-key requests map to terminal
// principals. Direct header mode is low-assurance and should only be used for
// trusted server-to-server calls; signed-token mode verifies the user claim.
type APIPrincipalConfig struct {
	Mode                  APIPrincipalMode `json:"mode"`
	DirectHeaderName      string           `json:"direct_header_name,omitempty"`
	SignedTokenHeaderName string           `json:"signed_token_header_name,omitempty"`
	// RequireDirectHeader, when true in direct_header mode, rejects API-key
	// requests that omit the configured user-id header instead of falling
	// back to the tenant-level principal.
	RequireDirectHeader bool   `json:"require_direct_header,omitempty"`
	HMACSecret          string `json:"hmac_secret,omitempty"`
}

func (c *APIPrincipalConfig) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	cp := *c
	if cp.HMACSecret != "" {
		if key := utils.GetAESKey(); key != nil {
			if encrypted, err := utils.EncryptAESGCM(cp.HMACSecret, key); err == nil {
				cp.HMACSecret = encrypted
			}
		}
	}
	return json.Marshal(&cp)
}

func (c *APIPrincipalConfig) Scan(value interface{}) error {
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
	if plain, ok := utils.DecryptStoredSecretLenient(c.HMACSecret); ok {
		c.HMACSecret = plain
	} else {
		log.Printf("[crypto] tenant api_principal_config.hmac_secret: decrypt failed (SYSTEM_AES_KEY missing/rotated?), treating as unconfigured")
		c.HMACSecret = ""
	}
	return nil
}

// Value implements the driver.Valuer interface for CredentialsConfig
func (c *CredentialsConfig) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(*c)
}

// Scan implements the sql.Scanner interface for CredentialsConfig
func (c *CredentialsConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	// Unknown keys (e.g. a removed provider's leftovers) are ignored by
	// encoding/json, so historical rows stay readable.
	if err := json.Unmarshal(b, c); err != nil {
		return err
	}
	return nil
}

// ParserEngineConfig holds tenant-level overrides for document parser engines (e.g. MinerU endpoint, API key).
// These values take precedence over environment variables when parsing documents.
type ParserEngineConfig struct {
	// ChatParserEngineRules selects parser engines for session-scoped chat
	// documents. Knowledge bases keep their own rules in ChunkingConfig.
	ChatParserEngineRules []ParserEngineRule `json:"chat_parser_engine_rules,omitempty"`
	MinerUEndpoint        string             `json:"mineru_endpoint"` // MinerU 自建服务端点
	MinerUAPIKey          string             `json:"mineru_api_key"`  // MinerU 云 API Key

	// MinerU 自建解析参数
	MinerUModel         string `json:"mineru_model,omitempty"`          // backend: pipeline, vlm-*, hybrid-*
	MinerUVLMServerURL  string `json:"mineru_vlm_server_url,omitempty"` // vLLM 服务器地址 (vlm-http-client / hybrid-http-client)
	MinerUEnableFormula *bool  `json:"mineru_enable_formula,omitempty"`
	MinerUEnableTable   *bool  `json:"mineru_enable_table,omitempty"`
	MinerUParseMethod   string `json:"mineru_parse_method,omitempty"`
	MinerULanguage      string `json:"mineru_language,omitempty"`

	// MinerU 云 API 解析参数
	MinerUCloudModel         string `json:"mineru_cloud_model,omitempty"` // model_version: pipeline, vlm, MinerU-HTML
	MinerUCloudEnableFormula *bool  `json:"mineru_cloud_enable_formula,omitempty"`
	MinerUCloudEnableTable   *bool  `json:"mineru_cloud_enable_table,omitempty"`
	MinerUCloudEnableOCR     *bool  `json:"mineru_cloud_enable_ocr,omitempty"`
	MinerUCloudLanguage      string `json:"mineru_cloud_language,omitempty"`

	// OpenDataLoader PDF (docreader engine); hybrid requires opendataloader-pdf-hybrid service.
	ODLHybrid           string `json:"odl_hybrid,omitempty"`      // off (default), docling-fast, hancom-ai
	ODLHybridURL        string `json:"odl_hybrid_url,omitempty"`  // e.g. http://odl-hybrid:5002
	ODLHybridMode       string `json:"odl_hybrid_mode,omitempty"` // auto, full
	ODLHybridFallback   *bool  `json:"odl_hybrid_fallback,omitempty"`
	ODLMarkdownWithHTML *bool  `json:"odl_markdown_with_html,omitempty"`

	// PaddleOCR-VL self-hosted pipeline service (full /layout-parsing API).
	PaddleOCRVLEndpoint            string `json:"paddleocr_vl_endpoint,omitempty"` // e.g. http://paddleocr-vl:8080
	PaddleOCRVLUseSealRecognition  *bool  `json:"paddleocr_vl_use_seal_recognition,omitempty"`
	PaddleOCRVLUseChartRecognition *bool  `json:"paddleocr_vl_use_chart_recognition,omitempty"`

	// PaddleOCR-VL AI Studio cloud API.
	PaddleOCRVLCloudToken               string `json:"paddleocr_vl_cloud_token,omitempty"`
	PaddleOCRVLCloudModel               string `json:"paddleocr_vl_cloud_model,omitempty"` // e.g. PaddleOCR-VL-1.6
	PaddleOCRVLCloudUseSealRecognition  *bool  `json:"paddleocr_vl_cloud_use_seal_recognition,omitempty"`
	PaddleOCRVLCloudUseChartRecognition *bool  `json:"paddleocr_vl_cloud_use_chart_recognition,omitempty"`

	// MinerU Tianshu (天枢) — self-hosted async task-queue service in front of
	// MinerU (https://github.com/.../mineru-tianshu): submit → poll → markdown.
	MinerUTianshuEndpoint string `json:"mineru_tianshu_endpoint,omitempty"` // e.g. http://tianshu:8000
	// MinerUTianshuAPIKey is optional — Tianshu itself is unauthenticated; the
	// key is for API gateways deployed in front of it. Sent as the value of
	// MinerUTianshuAuthHeader (default X-API-Key), mirroring the Gen-AI client.
	MinerUTianshuAPIKey     string `json:"mineru_tianshu_api_key,omitempty"`
	MinerUTianshuAuthHeader string `json:"mineru_tianshu_auth_header,omitempty"`
	// MinerUTianshuBackend selects the Tianshu processing backend
	// (pipeline / vlm-transformers / vlm-vllm-engine / auto on newer builds).
	// Empty omits the field so the server's own default applies.
	MinerUTianshuBackend       string `json:"mineru_tianshu_backend,omitempty"`
	MinerUTianshuLanguage      string `json:"mineru_tianshu_language,omitempty"`     // ch/en/... ; empty = server default
	MinerUTianshuParseMethod   string `json:"mineru_tianshu_parse_method,omitempty"` // auto/txt/ocr; empty = server default
	MinerUTianshuEnableFormula *bool  `json:"mineru_tianshu_enable_formula,omitempty"`
	MinerUTianshuEnableTable   *bool  `json:"mineru_tianshu_enable_table,omitempty"`
}

const (
	MinerUParseMethodAuto = "auto"
	MinerUParseMethodOCR  = "ocr"
	MinerUParseMethodText = "txt"
)

// ResolveMinerUParseMethod normalizes the MinerU parse method. Anything
// unrecognized, empty included, resolves to auto rather than ocr, so digital
// PDFs keep their native text layer while scanned PDFs are still detected and
// OCRed by MinerU.
func ResolveMinerUParseMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case MinerUParseMethodAuto:
		return MinerUParseMethodAuto
	case MinerUParseMethodOCR:
		return MinerUParseMethodOCR
	case MinerUParseMethodText:
		return MinerUParseMethodText
	}
	return MinerUParseMethodAuto
}

func (c *ParserEngineConfig) ResolveChatParserEngine(fileType string) string {
	if c == nil {
		return ""
	}
	fileType = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
	for _, rule := range c.ChatParserEngineRules {
		for _, candidate := range rule.FileTypes {
			if strings.TrimPrefix(strings.ToLower(strings.TrimSpace(candidate)), ".") == fileType {
				return strings.TrimSpace(rule.Engine)
			}
		}
	}
	return ""
}

// ToOverridesMap returns a map suitable for ParserEngineOverrides in parse requests.
// Keys are snake_case (mineru_endpoint, mineru_api_key, etc.).
func (c *ParserEngineConfig) ToOverridesMap() map[string]string {
	if c == nil {
		return nil
	}
	m := make(map[string]string)
	if c.MinerUEndpoint != "" {
		m["mineru_endpoint"] = c.MinerUEndpoint
	}
	if c.MinerUAPIKey != "" {
		m["mineru_api_key"] = c.MinerUAPIKey
	}
	if c.MinerUModel != "" {
		m["mineru_model"] = c.MinerUModel
	}
	if c.MinerUVLMServerURL != "" {
		m["mineru_vlm_server_url"] = c.MinerUVLMServerURL
	}
	if c.MinerUEnableFormula != nil {
		m["mineru_enable_formula"] = fmt.Sprintf("%v", *c.MinerUEnableFormula)
	}
	if c.MinerUEnableTable != nil {
		m["mineru_enable_table"] = fmt.Sprintf("%v", *c.MinerUEnableTable)
	}
	if c.MinerUParseMethod != "" {
		m["mineru_parse_method"] = ResolveMinerUParseMethod(c.MinerUParseMethod)
	}
	if c.MinerULanguage != "" {
		m["mineru_language"] = c.MinerULanguage
	}
	if c.MinerUCloudModel != "" {
		m["mineru_cloud_model"] = c.MinerUCloudModel
	}
	if c.MinerUCloudEnableFormula != nil {
		m["mineru_cloud_enable_formula"] = fmt.Sprintf("%v", *c.MinerUCloudEnableFormula)
	}
	if c.MinerUCloudEnableTable != nil {
		m["mineru_cloud_enable_table"] = fmt.Sprintf("%v", *c.MinerUCloudEnableTable)
	}
	if c.MinerUCloudEnableOCR != nil {
		m["mineru_cloud_enable_ocr"] = fmt.Sprintf("%v", *c.MinerUCloudEnableOCR)
	}
	if c.MinerUCloudLanguage != "" {
		m["mineru_cloud_language"] = c.MinerUCloudLanguage
	}
	if c.ODLHybrid != "" {
		m["odl_hybrid"] = c.ODLHybrid
	}
	if c.ODLHybridURL != "" {
		m["odl_hybrid_url"] = c.ODLHybridURL
	}
	if c.ODLHybridMode != "" {
		m["odl_hybrid_mode"] = c.ODLHybridMode
	}
	if c.ODLHybridFallback != nil {
		m["odl_hybrid_fallback"] = fmt.Sprintf("%v", *c.ODLHybridFallback)
	}
	if c.ODLMarkdownWithHTML != nil {
		m["odl_markdown_with_html"] = fmt.Sprintf("%v", *c.ODLMarkdownWithHTML)
	}
	if c.PaddleOCRVLEndpoint != "" {
		m["paddleocr_vl_endpoint"] = c.PaddleOCRVLEndpoint
	}
	if c.PaddleOCRVLUseSealRecognition != nil {
		m["paddleocr_vl_use_seal_recognition"] = fmt.Sprintf("%v", *c.PaddleOCRVLUseSealRecognition)
	}
	if c.PaddleOCRVLUseChartRecognition != nil {
		m["paddleocr_vl_use_chart_recognition"] = fmt.Sprintf("%v", *c.PaddleOCRVLUseChartRecognition)
	}
	if c.PaddleOCRVLCloudToken != "" {
		m["paddleocr_vl_cloud_token"] = c.PaddleOCRVLCloudToken
	}
	if c.PaddleOCRVLCloudModel != "" {
		m["paddleocr_vl_cloud_model"] = c.PaddleOCRVLCloudModel
	}
	if c.PaddleOCRVLCloudUseSealRecognition != nil {
		m["paddleocr_vl_cloud_use_seal_recognition"] = fmt.Sprintf("%v", *c.PaddleOCRVLCloudUseSealRecognition)
	}
	if c.PaddleOCRVLCloudUseChartRecognition != nil {
		m["paddleocr_vl_cloud_use_chart_recognition"] = fmt.Sprintf("%v", *c.PaddleOCRVLCloudUseChartRecognition)
	}
	if c.MinerUTianshuEndpoint != "" {
		m["mineru_tianshu_endpoint"] = c.MinerUTianshuEndpoint
	}
	if c.MinerUTianshuAPIKey != "" {
		m["mineru_tianshu_api_key"] = c.MinerUTianshuAPIKey
	}
	if c.MinerUTianshuAuthHeader != "" {
		m["mineru_tianshu_auth_header"] = c.MinerUTianshuAuthHeader
	}
	if c.MinerUTianshuBackend != "" {
		m["mineru_tianshu_backend"] = c.MinerUTianshuBackend
	}
	if c.MinerUTianshuLanguage != "" {
		m["mineru_tianshu_language"] = c.MinerUTianshuLanguage
	}
	if c.MinerUTianshuParseMethod != "" {
		m["mineru_tianshu_parse_method"] = c.MinerUTianshuParseMethod
	}
	if c.MinerUTianshuEnableFormula != nil {
		m["mineru_tianshu_enable_formula"] = fmt.Sprintf("%v", *c.MinerUTianshuEnableFormula)
	}
	if c.MinerUTianshuEnableTable != nil {
		m["mineru_tianshu_enable_table"] = fmt.Sprintf("%v", *c.MinerUTianshuEnableTable)
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// Value implements the driver.Valuer interface for ParserEngineConfig
func (c *ParserEngineConfig) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface for ParserEngineConfig
func (c *ParserEngineConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}
