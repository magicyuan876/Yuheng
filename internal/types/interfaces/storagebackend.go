package interfaces

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/types"
)

type StorageBackendRepository interface {
	Create(ctx context.Context, backend *types.StorageBackend) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.StorageBackend, error)
	// Find resolves a backend by id regardless of the owning workspace; see
	// FileStore, its only caller.
	Find(ctx context.Context, id string) (*types.StorageBackend, error)
	List(ctx context.Context, tenantID uint64) ([]*types.StorageBackend, error)
	Update(ctx context.Context, backend *types.StorageBackend) error
	Delete(ctx context.Context, tenantID uint64, id string) error
}

type StorageBackendService interface {
	Create(ctx context.Context, backend *types.StorageBackend) error
	Update(ctx context.Context, backend *types.StorageBackend) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	SetDefault(ctx context.Context, tenantID uint64, id string) error
	// ResolveBackend returns the active backend tenantID binds new content
	// to: id, or the workspace default when id is empty.
	ResolveBackend(ctx context.Context, tenantID uint64, id string) (*types.StorageBackend, error)
	// SetSharing publishes a backend to every workspace, or withdraws it.
	// System administrators only; withdrawal is refused while a workspace
	// other than the owner still binds it as a default, from a knowledge base
	// or docs space, or through an active stored resource.
	SetSharing(ctx context.Context, id string, shared bool) (*types.StorageBackend, error)
	Test(ctx context.Context, backend *types.StorageBackend) error
}
