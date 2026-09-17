package repository

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

// webSearchProviderRepository implements the WebSearchProviderRepository interface
type webSearchProviderRepository struct {
	db *gorm.DB
}

// NewWebSearchProviderRepository creates a new web search provider repository
func NewWebSearchProviderRepository(db *gorm.DB) interfaces.WebSearchProviderRepository {
	return &webSearchProviderRepository{db: db}
}

// Create creates a new web search provider
func (r *webSearchProviderRepository) Create(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// GetByID retrieves a provider visible to the tenant: its own rows plus any
// platform-shared (is_builtin) one. Writes below stay pinned to the owner.
func (r *webSearchProviderRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.WebSearchProviderEntity, error) {
	var provider types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).Where("id = ?", id).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// GetDefault retrieves the default provider for a tenant, or nil if none.
//
// The workspace's own default always wins; a platform-shared default is only
// the fallback. Doing it as two queries rather than one ORDER BY keeps that
// precedence explicit — a workspace that deliberately picked a provider must
// never be silently switched to the platform one because of row ordering.
func (r *webSearchProviderRepository) GetDefault(ctx context.Context, tenantID uint64) (*types.WebSearchProviderEntity, error) {
	var provider types.WebSearchProviderEntity
	err := r.db.WithContext(ctx).Where(
		"tenant_id = ? AND is_default = ?", tenantID, true,
	).First(&provider).Error
	if err == nil {
		return &provider, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var shared types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).Where(
		"is_builtin = ? AND is_default = ?", true, true,
	).First(&shared).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &shared, nil
}

// List lists all web search providers for a tenant
func (r *webSearchProviderRepository) List(ctx context.Context, tenantID uint64) ([]*types.WebSearchProviderEntity, error) {
	var providers []*types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).Order("created_at ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// Update updates a web search provider
func (r *webSearchProviderRepository) Update(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	return r.db.WithContext(ctx).Model(&types.WebSearchProviderEntity{}).Where(
		"id = ? AND tenant_id = ?", provider.ID, provider.TenantID,
	).Select("*").Updates(provider).Error
}

// Delete soft-deletes a web search provider
func (r *webSearchProviderRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where(
		"id = ? AND tenant_id = ?", id, tenantID,
	).Delete(&types.WebSearchProviderEntity{}).Error
}

// ClearDefault clears the default flag for all providers of a tenant, optionally excluding one
func (r *webSearchProviderRepository) ClearDefault(ctx context.Context, tenantID uint64, excludeID string) error {
	query := r.db.WithContext(ctx).Model(&types.WebSearchProviderEntity{}).Where(
		"tenant_id = ? AND is_default = ?", tenantID, true,
	)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	return query.Update("is_default", false).Error
}
