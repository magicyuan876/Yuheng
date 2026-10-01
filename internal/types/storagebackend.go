package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/utils"
	"gorm.io/gorm"
)

const (
	StorageBackendSourceUser     = "user"
	StorageBackendSourceEnv      = "env"
	StorageBackendStatusActive   = "active"
	StorageBackendStatusDisabled = "disabled"
)

// Storage provider names. Object storage converges to two providers: the
// zero-dependency local filesystem and any S3-compatible service (RustFS,
// MinIO, AWS S3, Aliyun OSS, Tencent COS, Volcengine TOS, ...) reached through
// its S3 endpoint.
const (
	StorageProviderLocal = "local"
	StorageProviderS3    = "s3"
)

// S3 addressing styles. See StorageBackendConfig.AddressingStyle.
const (
	S3AddressingAuto    = "auto"
	S3AddressingPath    = "path"
	S3AddressingVirtual = "virtual"
)

// ValidateS3AddressingStyle accepts the empty string (meaning auto) and the
// three named styles, and rejects everything else so a typo cannot silently
// fall back to a style the operator did not ask for.
func ValidateS3AddressingStyle(style string) error {
	switch style {
	case "", S3AddressingAuto, S3AddressingPath, S3AddressingVirtual:
		return nil
	default:
		return fmt.Errorf("addressing_style must be one of auto, path or virtual, got %q", style)
	}
}

// EnvStorageBackendID is the id of the deployment's storage backend: the one
// row the environment (STORAGE_TYPE, S3_*, LOCAL_STORAGE_PATH_PREFIX)
// describes. Every workspace starts with it as its default.
const EnvStorageBackendID = "env"

// StorageBackend is one concrete file/object storage instance. A workspace may
// register multiple instances of the same provider and bind each knowledge base
// to a different instance.
//
// storage_backends is the only place storage is configured. Besides the rows
// workspaces register (source "user"), there is exactly one row with source
// "env" and id EnvStorageBackendID: the deployment's own storage, rewritten
// from the environment at every start, owned by no workspace (TenantID 0,
// stored as NULL) and shared with all of them. It is read-only through the
// API, and it never stores credentials — those are read from the environment
// whenever a driver is built.
type StorageBackend struct {
	ID string `json:"id" gorm:"type:varchar(36);primaryKey"`
	// TenantID owns a user backend. The environment backend has none: zero
	// here, NULL in the column (the default tag makes gorm leave a zero value
	// to the database).
	TenantID uint64               `json:"tenant_id" gorm:"index;default:null"`
	Name     string               `json:"name" gorm:"type:varchar(255);not null"`
	Provider string               `json:"provider" gorm:"type:varchar(32);not null;index"`
	Config   StorageBackendConfig `json:"config" gorm:"type:json"`
	Source   string               `json:"source" gorm:"type:varchar(16);not null;default:'user'"`
	Status   string               `json:"status" gorm:"type:varchar(16);not null;default:'active'"`
	// IsBuiltin marks a platform-shared row: visible to EVERY workspace, not
	// just the owning one. Reads OR this against tenant_id; writes stay pinned
	// to the owner and, at the service layer, to system administrators, which
	// also keeps credentials out of the response for everyone else.
	//
	// Exists because tenant roles cannot express platform ownership: every
	// self-registered user is Owner of their own personal workspace, so no
	// role threshold means "only the platform operator". See the
	// governance.centralized_infra system setting.
	IsBuiltin bool           `json:"is_builtin" gorm:"not null;default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (StorageBackend) TableName() string { return "storage_backends" }

func (b *StorageBackend) BeforeCreate(_ *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if b.Source == "" {
		b.Source = StorageBackendSourceUser
	}
	if b.Status == "" {
		b.Status = StorageBackendStatusActive
	}
	return nil
}

// Validate checks a user backend. The environment backend is validated by
// EnvStorageBackend, from the environment it is built from.
func (b *StorageBackend) Validate() error {
	if b.TenantID == 0 {
		return errors.NewValidationError("tenant_id is required")
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return errors.NewValidationError("name is required")
	}
	b.Provider = strings.ToLower(strings.TrimSpace(b.Provider))
	if !storageallowlist.IsSupported(b.Provider) {
		return errors.NewValidationError(fmt.Sprintf("unsupported storage provider: %s", b.Provider))
	}
	if !storageallowlist.IsAllowed(b.Provider) {
		return errors.NewValidationError(fmt.Sprintf("storage provider %q is not allowed", b.Provider))
	}
	if b.Status == "" {
		b.Status = StorageBackendStatusActive
	}
	if b.Status != StorageBackendStatusActive && b.Status != StorageBackendStatusDisabled {
		return errors.NewValidationError("status must be active or disabled")
	}
	return b.Config.ValidateForProvider(b.Provider)
}

// StorageBackendConfig is the normalized union of provider-specific storage
// settings. AccessKeyID/SecretAccessKey are the access/secret key pair of the
// S3-compatible provider; both empty means the AWS default credential chain.
type StorageBackendConfig struct {
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	BucketName      string `json:"bucket_name,omitempty"`
	PathPrefix      string `json:"path_prefix,omitempty"`
	UseSSL          bool   `json:"use_ssl,omitempty"`
	// AddressingStyle selects how the bucket appears in request URLs:
	// "path" is endpoint/bucket/key, "virtual" is bucket.endpoint/key (Aliyun
	// OSS, Tencent COS, Volcengine TOS and Huawei OBS only accept this one),
	// and ""/"auto" picks virtual-hosted for AWS endpoints and path-style for
	// any other custom endpoint, which is what MinIO and RustFS need.
	AddressingStyle string `json:"addressing_style,omitempty"`
}

func (c StorageBackendConfig) Value() (driver.Value, error) {
	if key := utils.GetAESKey(); key != nil {
		if c.AccessKeyID != "" {
			if encrypted, err := utils.EncryptAESGCM(c.AccessKeyID, key); err == nil {
				c.AccessKeyID = encrypted
			}
		}
		if c.SecretAccessKey != "" {
			if encrypted, err := utils.EncryptAESGCM(c.SecretAccessKey, key); err == nil {
				c.SecretAccessKey = encrypted
			}
		}
	}
	return json.Marshal(c)
}

func (c *StorageBackendConfig) Scan(value interface{}) error {
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
	accessKey, err := utils.DecryptStoredSecret(c.AccessKeyID)
	if err != nil {
		return fmt.Errorf("decrypt storage backend access key: %w", err)
	}
	secretKey, err := utils.DecryptStoredSecret(c.SecretAccessKey)
	if err != nil {
		return fmt.Errorf("decrypt storage backend secret key: %w", err)
	}
	c.AccessKeyID = accessKey
	c.SecretAccessKey = secretKey
	return nil
}

func (c StorageBackendConfig) MaskSensitiveFields() StorageBackendConfig {
	out := c
	if out.AccessKeyID != "" {
		out.AccessKeyID = RedactedSecretPlaceholder
	}
	if out.SecretAccessKey != "" {
		out.SecretAccessKey = RedactedSecretPlaceholder
	}
	return out
}

// MergeSecrets keeps a stored credential where the client sent back the
// redaction placeholder it was shown. An empty value is a real change, not
// "keep": both keys empty selects the AWS default credential chain, and
// treating empty as preserve would make it impossible to move a backend from
// static keys to an IAM role. (The tenant storage-engine config, which this
// model replaced, had the same rule.)
func (c StorageBackendConfig) MergeSecrets(existing StorageBackendConfig) StorageBackendConfig {
	if c.AccessKeyID == RedactedSecretPlaceholder {
		c.AccessKeyID = existing.AccessKeyID
	}
	if c.SecretAccessKey == RedactedSecretPlaceholder {
		c.SecretAccessKey = existing.SecretAccessKey
	}
	return c
}

func (c StorageBackendConfig) ValidateForProvider(provider string) error {
	prefix := strings.ReplaceAll(strings.TrimSpace(c.PathPrefix), "\\", "/")
	cleanPrefix := path.Clean(prefix)
	if strings.HasPrefix(prefix, "/") || cleanPrefix == ".." || strings.HasPrefix(cleanPrefix, "../") {
		return errors.NewValidationError("path_prefix must be a relative path without parent traversal")
	}
	switch provider {
	case StorageProviderLocal:
		return nil
	case StorageProviderS3:
		for name, value := range map[string]string{"region": c.Region, "bucket_name": c.BucketName} {
			if strings.TrimSpace(value) == "" {
				return errors.NewValidationError(name + " is required")
			}
		}
		// Empty keys mean the AWS default credential chain; a lone key can
		// never authenticate, so reject it here instead of at first upload.
		if (strings.TrimSpace(c.AccessKeyID) == "") != (strings.TrimSpace(c.SecretAccessKey) == "") {
			return errors.NewValidationError("access_key_id and secret_access_key must be provided together")
		}
		if err := ValidateS3AddressingStyle(c.AddressingStyle); err != nil {
			return errors.NewValidationError(err.Error())
		}
		return nil
	default:
		return errors.NewValidationError(fmt.Sprintf("unsupported storage provider: %s", provider))
	}
}

// LocationKey identifies the physical destination. Credentials deliberately do
// not participate so they can be rotated without changing object identity.
func (c StorageBackendConfig) LocationKey(provider string) string {
	return strings.Join([]string{
		provider,
		strings.TrimSpace(c.Endpoint),
		strings.TrimSpace(c.Region),
		strings.TrimSpace(c.BucketName),
		strings.Trim(strings.TrimSpace(c.PathPrefix), "/"),
	}, "|")
}

// StorageBackendRef names a storage backend inside another resource's
// response (a knowledge base's): what a client needs to show the binding and
// offer the choice, and nothing about where the backend is or how it is
// reached. Clients use it instead of matching storage_backend_id against a
// separately fetched list.
type StorageBackendRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Source    string `json:"source"`
	IsBuiltin bool   `json:"is_builtin"`
}

// NewStorageBackendRef projects a backend row to its reference form.
func NewStorageBackendRef(b *StorageBackend) *StorageBackendRef {
	if b == nil {
		return nil
	}
	return &StorageBackendRef{ID: b.ID, Name: b.Name, Provider: b.Provider, Source: b.Source, IsBuiltin: b.IsBuiltin}
}

func NewStorageBackendResponse(backend *StorageBackend) StorageBackend {
	return NewStorageBackendResponseWithSharedDetail(backend, true)
}

// NewStorageBackendResponseWithSharedDetail is NewStorageBackendResponse plus
// the platform-sharing redaction.
//
// For a platform-shared backend shown to someone who does not administer the
// platform, the whole config is dropped rather than merely masked: endpoint,
// region and bucket describe the operator's infrastructure. Name, provider and
// status survive — that is what the knowledge-base editor renders so a
// workspace can pick a backend.
func NewStorageBackendResponseWithSharedDetail(
	backend *StorageBackend, sharedDetail bool,
) StorageBackend {
	out := *backend
	out.Config = backend.Config.MaskSensitiveFields()
	if backend.IsBuiltin && !sharedDetail {
		out.Config = StorageBackendConfig{}
	}
	return out
}

// EnvStorageBackend describes the deployment's storage as the environment
// configures it: STORAGE_TYPE (default local), LOCAL_STORAGE_PATH_PREFIX, and
// the S3_* variables, credentials included. The startup sync stores it,
// minus the credentials, as the EnvStorageBackendID row.
func EnvStorageBackend() (*StorageBackend, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_TYPE")))
	if provider == "" {
		provider = StorageProviderLocal
	}
	b := &StorageBackend{
		ID: EnvStorageBackendID, Name: "Deployment storage", Provider: provider,
		Source: StorageBackendSourceEnv, Status: StorageBackendStatusActive, IsBuiltin: true,
	}
	switch provider {
	case StorageProviderLocal:
		b.Config.PathPrefix = strings.TrimSpace(os.Getenv("LOCAL_STORAGE_PATH_PREFIX"))
	case StorageProviderS3:
		b.Config = StorageBackendConfig{
			Endpoint: strings.TrimSpace(os.Getenv("S3_ENDPOINT")), Region: strings.TrimSpace(os.Getenv("S3_REGION")),
			BucketName:      strings.TrimSpace(os.Getenv("S3_BUCKET_NAME")),
			PathPrefix:      strings.TrimSpace(os.Getenv("S3_PATH_PREFIX")),
			UseSSL:          !strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_SSL")), "false"),
			AddressingStyle: strings.ToLower(strings.TrimSpace(os.Getenv("S3_ADDRESSING_STYLE"))),
		}
		b.Config.AccessKeyID, b.Config.SecretAccessKey = EnvStorageCredentials()
	default:
		return nil, fmt.Errorf("STORAGE_TYPE=%q is not a supported storage provider (supported: local, s3)", provider)
	}
	if err := b.Config.ValidateForProvider(provider); err != nil {
		return nil, fmt.Errorf("deployment storage (STORAGE_TYPE=%s): %w", provider, err)
	}
	return b, nil
}

// EnvStorageCredentials returns the deployment storage's S3 key pair, read
// from the environment every time so the keys never reach the database and a
// rotation takes effect on restart.
func EnvStorageCredentials() (accessKey, secretKey string) {
	return strings.TrimSpace(os.Getenv("S3_ACCESS_KEY")), strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
}
