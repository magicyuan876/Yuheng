package repository

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

type storageBackendRepository struct{ db *gorm.DB }

func NewStorageBackendRepository(db *gorm.DB) interfaces.StorageBackendRepository {
	return &storageBackendRepository{db: db}
}

func (r *storageBackendRepository) Create(ctx context.Context, backend *types.StorageBackend) error {
	return r.db.WithContext(ctx).Create(backend).Error
}

// GetByID resolves a backend visible to the tenant: its own rows plus any
// platform-shared (is_builtin) one. Reads are wider than writes so a workspace
// can select a platform-provided backend for a knowledge base; every mutation
// stays pinned to the owning tenant.
func (r *storageBackendRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.StorageBackend, error) {
	var backend types.StorageBackend
	if err := r.db.WithContext(ctx).Where("id = ?", id).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).First(&backend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backend, nil
}

// Find resolves a backend by id alone, whichever workspace owns it. Only the
// storage runtime uses it: a stored resource names the backend it was written
// to, and reading it back must not depend on who is asking — a borrower of a
// shared knowledge base reads the owner's files through the owner's backend.
// Callers authorize the reference before it gets here.
func (r *storageBackendRepository) Find(ctx context.Context, id string) (*types.StorageBackend, error) {
	var backend types.StorageBackend
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&backend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backend, nil
}

// List returns the backends visible to a tenant: its own plus every
// platform-shared one. Object keys are already namespaced per tenant
// (prefix/{tenantID}/{knowledgeID}/uuid.ext), so two workspaces sharing one
// backend do not collide in the bucket.
func (r *storageBackendRepository) List(ctx context.Context, tenantID uint64) ([]*types.StorageBackend, error) {
	var backends []*types.StorageBackend
	err := r.db.WithContext(ctx).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).Order("created_at DESC").Find(&backends).Error
	return backends, err
}

func (r *storageBackendRepository) Update(ctx context.Context, backend *types.StorageBackend) error {
	return r.db.WithContext(ctx).Model(&types.StorageBackend{}).
		Where("tenant_id = ? AND id = ?", backend.TenantID, backend.ID).
		Select("name", "config", "status", "updated_at").Updates(backend).Error
}

func (r *storageBackendRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&types.StorageBackend{}).Error
}

func (r *storageBackendRepository) FindLegacyAlias(ctx context.Context, tenantID uint64, provider string) (*types.StorageBackend, error) {
	var backend types.StorageBackend
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND provider = ? AND legacy_alias = ?", tenantID, provider, true).First(&backend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backend, nil
}
