package repository

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

// vectorStoreRepository implements the VectorStoreRepository interface
type vectorStoreRepository struct {
	db *gorm.DB
}

// NewVectorStoreRepository creates a new vector store repository
func NewVectorStoreRepository(db *gorm.DB) interfaces.VectorStoreRepository {
	return &vectorStoreRepository{db: db}
}

// Create creates a new vector store
func (r *vectorStoreRepository) Create(ctx context.Context, store *types.VectorStore) error {
	return r.db.WithContext(ctx).Create(store).Error
}

// GetByID retrieves a vector store visible to the tenant: its own rows plus
// any platform-shared (is_builtin) row.
// Returns (nil, nil) when the record is not found (not an error).
//
// Reads are intentionally wider than writes. The knowledge-base editor has to
// list and resolve a platform-provided store so a workspace can select one;
// every mutation below stays pinned to the owning tenant.
func (r *vectorStoreRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.VectorStore, error) {
	var store types.VectorStore
	if err := r.db.WithContext(ctx).Where("id = ?", id).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &store, nil
}

// List lists the vector stores visible to a tenant — its own plus every
// platform-shared one (newest first).
func (r *vectorStoreRepository) List(ctx context.Context, tenantID uint64) ([]*types.VectorStore, error) {
	var stores []*types.VectorStore
	if err := r.db.WithContext(ctx).Where(
		"(tenant_id = ? OR is_builtin = ?)", tenantID, true,
	).Order("created_at DESC").Find(&stores).Error; err != nil {
		return nil, err
	}
	return stores, nil
}

// Update updates a vector store (only mutable fields: name).
// engine_type, connection_config, index_config are immutable and excluded via Select.
// updated_at is handled by the DB trigger, so it is not included in Select.
func (r *vectorStoreRepository) Update(ctx context.Context, store *types.VectorStore) error {
	return r.db.WithContext(ctx).Model(&types.VectorStore{}).Where(
		"id = ? AND tenant_id = ?", store.ID, store.TenantID,
	).Select("name").Updates(store).Error
}

// UpdateConnectionConfig updates only the connection_config JSONB column.
// Used for saving auto-detected metadata (e.g., server version) without
// touching user-immutable fields like engine_type or index_config.
func (r *vectorStoreRepository) UpdateConnectionConfig(ctx context.Context, store *types.VectorStore) error {
	return r.db.WithContext(ctx).Model(&types.VectorStore{}).Where(
		"id = ? AND tenant_id = ?", store.ID, store.TenantID,
	).Select("connection_config").Updates(store).Error
}

// SetSharing flips the platform-sharing flag on one row.
//
// A dedicated method rather than a field on Update: Update deliberately
// Selects only "name" (engine_type / connection_config / index_config are
// immutable after creation), so sharing would be silently dropped there.
func (r *vectorStoreRepository) SetSharing(
	ctx context.Context, tenantID uint64, id string, shared bool,
) error {
	return r.db.WithContext(ctx).Model(&types.VectorStore{}).Where(
		"id = ? AND tenant_id = ?", id, tenantID,
	).Update("is_builtin", shared).Error
}

// Delete soft-deletes a vector store
func (r *vectorStoreRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where(
		"id = ? AND tenant_id = ?", id, tenantID,
	).Delete(&types.VectorStore{}).Error
}

// ExistsByEndpointAndIndex checks if a store with the same endpoint and index already exists.
// A stored row that names no index is compared under defaultIndexName, the engine's default.
// Comparison is done at the application level, where the endpoint and index
// are read through the same connection-config decoding the rest of the code
// uses; the row count is small (a few per tenant).
func (r *vectorStoreRepository) ExistsByEndpointAndIndex(
	ctx context.Context,
	tenantID uint64,
	engineType types.RetrieverEngineType,
	endpoint string,
	indexName string,
	defaultIndexName string,
) (bool, error) {
	var stores []*types.VectorStore
	if err := r.db.WithContext(ctx).Where(
		"tenant_id = ? AND engine_type = ?", tenantID, string(engineType),
	).Find(&stores).Error; err != nil {
		return false, err
	}
	for _, s := range stores {
		effective := s.IndexConfig.IndexName
		if effective == "" {
			effective = defaultIndexName
		}
		if s.ConnectionConfig.GetEndpoint() == endpoint && effective == indexName {
			return true, nil
		}
	}
	return false, nil
}
