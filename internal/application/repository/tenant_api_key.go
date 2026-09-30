package repository

import (
	"context"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrTenantAPIKeyNotFound = errors.New("tenant api key not found")

type tenantAPIKeyRepository struct {
	db *gorm.DB
}

func NewTenantAPIKeyRepository(db *gorm.DB) interfaces.TenantAPIKeyRepository {
	return &tenantAPIKeyRepository{db: db}
}

func (r *tenantAPIKeyRepository) CreateAPIKey(ctx context.Context, key *types.TenantAPIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *tenantAPIKeyRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*types.TenantAPIKey, error) {
	var key types.TenantAPIKey
	err := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Where("key_hash = ?", hash).
		First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *tenantAPIKeyRepository) ListAPIKeys(ctx context.Context, tenantID uint64) ([]*types.TenantAPIKey, error) {
	var keys []*types.TenantAPIKey
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND revoked_at IS NULL", tenantID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *tenantAPIKeyRepository) ListPlatformAPIKeys(ctx context.Context) ([]*types.TenantAPIKey, error) {
	var keys []*types.TenantAPIKey
	err := r.db.WithContext(ctx).
		Where("scope_type = ? AND revoked_at IS NULL", types.APIKeyScopePlatform).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// UpdateAPIKey 更新租户 API Key 的可配置属性。
// tenant_id 和 scope_type 同时参与条件，避免跨租户或误改平台级 Key。
func (r *tenantAPIKeyRepository) UpdateAPIKey(
	ctx context.Context, tenantID uint64, id uint64, update *types.TenantAPIKey,
) (*types.TenantAPIKey, error) {
	res := r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Where("id = ? AND tenant_id = ? AND scope_type = ? AND revoked_at IS NULL",
			id, tenantID, types.APIKeyScopeTenant).
		Updates(map[string]any{
			"name":               update.Name,
			"full_access":        update.FullAccess,
			"knowledge_base_ids": update.KnowledgeBaseIDs,
			"capabilities":       update.Capabilities,
			"expires_at":         update.ExpiresAt,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrTenantAPIKeyNotFound
	}

	var updatedKey types.TenantAPIKey
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND revoked_at IS NULL", id, tenantID).
		First(&updatedKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantAPIKeyNotFound
		}
		return nil, err
	}
	return &updatedKey, nil
}

func (r *tenantAPIKeyRepository) RevokeAPIKey(ctx context.Context, tenantID uint64, id uint64) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Where("id = ? AND tenant_id = ? AND revoked_at IS NULL", id, tenantID).
		Update("revoked_at", &now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTenantAPIKeyNotFound
	}
	return nil
}

func (r *tenantAPIKeyRepository) RevokePlatformAPIKey(ctx context.Context, id uint64) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Where("id = ? AND scope_type = ? AND revoked_at IS NULL", id, types.APIKeyScopePlatform).
		Update("revoked_at", &now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTenantAPIKeyNotFound
	}
	return nil
}

// placeholderKeyHashPrefix mirrors the value written by migration
// 000065_tenant_api_keys.up.sql ('migrated-tenant-' || id): a key copied from
// tenants.api_key before its real hash was known.
const placeholderKeyHashPrefix = "migrated-tenant-"

// ListStoredKeySecrets returns every row, revoked ones included, that still
// holds its key in the legacy api_key column.
func (r *tenantAPIKeyRepository) ListStoredKeySecrets(ctx context.Context) ([]types.StoredAPIKeySecret, error) {
	var rows []types.StoredAPIKeySecret
	err := r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Select("id, api_key AS secret, key_hash LIKE ? AS needs_hash", placeholderKeyHashPrefix+"%").
		Where("api_key <> ''").
		Order("id").
		Scan(&rows).Error
	return rows, err
}

// SealKey records a key's hint (and its real hash, when hash is not empty)
// and empties the legacy api_key column, in one statement, so a row is never
// left without the secret and without the hint derived from it.
func (r *tenantAPIKeyRepository) SealKey(ctx context.Context, id uint64, hint, hash string) error {
	fields := map[string]any{"key_hint": hint, "api_key": ""}
	if hash != "" {
		fields["key_hash"] = hash
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *tenantAPIKeyRepository) UpdateAPIKeyLastUsed(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&types.TenantAPIKey{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("last_used_at", &at).Error
}
