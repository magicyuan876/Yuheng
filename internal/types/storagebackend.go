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

// StorageBackend is one concrete file/object storage instance. A workspace may
// register multiple instances of the same provider and bind each knowledge base
// to a different instance.
type StorageBackend struct {
	ID          string               `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64               `json:"tenant_id" gorm:"not null;index"`
	Name        string               `json:"name" gorm:"type:varchar(255);not null"`
	Provider    string               `json:"provider" gorm:"type:varchar(32);not null;index"`
	Config      StorageBackendConfig `json:"config" gorm:"type:json"`
	Source      string               `json:"source" gorm:"type:varchar(16);not null;default:'user'"`
	Status      string               `json:"status" gorm:"type:varchar(16);not null;default:'active'"`
	LegacyAlias bool                 `json:"legacy_alias" gorm:"not null;default:false"`
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
	// AddressingStyle is "", "auto", "path" or "virtual"; see S3EngineConfig.
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

func (c StorageBackendConfig) MergeSecrets(existing StorageBackendConfig) StorageBackendConfig {
	c.AccessKeyID = PreserveIfRedacted(c.AccessKeyID, existing.AccessKeyID)
	c.SecretAccessKey = PreserveIfRedacted(c.SecretAccessKey, existing.SecretAccessKey)
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

// ToStorageEngineConfig adapts the instance model to the existing provider
// implementations while those implementations are progressively normalized.
func (b StorageBackend) ToStorageEngineConfig() *StorageEngineConfig {
	c := b.Config
	result := &StorageEngineConfig{DefaultProvider: b.Provider}
	switch b.Provider {
	case StorageProviderLocal:
		result.Local = &LocalEngineConfig{PathPrefix: c.PathPrefix}
	case StorageProviderS3:
		result.S3 = &S3EngineConfig{
			Endpoint: c.Endpoint, Region: c.Region, AccessKey: c.AccessKeyID, SecretKey: c.SecretAccessKey,
			BucketName: c.BucketName, PathPrefix: c.PathPrefix, UseSSL: c.UseSSL, AddressingStyle: c.AddressingStyle,
		}
	}
	return result
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

// StorageBackendFromLegacy projects one provider entry from the old workspace
// singleton JSON into the multi-instance model.
func StorageBackendFromLegacy(tenantID uint64, provider string, legacy *StorageEngineConfig) *StorageBackend {
	if legacy == nil {
		return nil
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	b := &StorageBackend{TenantID: tenantID, Provider: provider, Source: StorageBackendSourceUser, Status: StorageBackendStatusActive, LegacyAlias: true}
	switch provider {
	case StorageProviderLocal:
		if legacy.Local == nil {
			return nil
		}
		b.Name, b.Config.PathPrefix = "Local", legacy.Local.PathPrefix
	case StorageProviderS3:
		if legacy.S3 == nil {
			return nil
		}
		c := legacy.S3
		b.Name = "S3"
		b.Config = StorageBackendConfig{
			Endpoint: c.Endpoint, Region: c.Region, AccessKeyID: c.AccessKey, SecretAccessKey: c.SecretKey,
			BucketName: c.BucketName, PathPrefix: c.PathPrefix, UseSSL: c.UseSSL, AddressingStyle: c.AddressingStyle,
		}
	default:
		return nil
	}
	return b
}

// StorageBackendFromEnvironment snapshots the process-wide storage backend for
// a workspace. The row is read-only in the UI and keeps env-only deployments
// on the same instance-resolution path as user-managed backends.
func StorageBackendFromEnvironment(tenantID uint64) *StorageBackend {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_TYPE")))
	if provider == "" {
		provider = "local"
	}
	b := &StorageBackend{
		TenantID: tenantID, Name: "System " + strings.ToUpper(provider), Provider: provider,
		Source: StorageBackendSourceEnv, Status: StorageBackendStatusActive, LegacyAlias: true,
	}
	switch provider {
	case StorageProviderLocal:
		b.Config.PathPrefix = strings.TrimSpace(os.Getenv("LOCAL_STORAGE_PATH_PREFIX"))
	case StorageProviderS3:
		b.Config = StorageBackendConfig{
			Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"),
			AccessKeyID: os.Getenv("S3_ACCESS_KEY"), SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
			BucketName: os.Getenv("S3_BUCKET_NAME"), PathPrefix: os.Getenv("S3_PATH_PREFIX"),
			UseSSL:          !strings.EqualFold(os.Getenv("S3_USE_SSL"), "false"),
			AddressingStyle: strings.ToLower(strings.TrimSpace(os.Getenv("S3_ADDRESSING_STYLE"))),
		}
	default:
		return nil
	}
	return b
}
