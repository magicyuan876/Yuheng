package findings

import (
	"context"
	"errors"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// FinderResolver finds the component that compares a knowledge base's stored
// vectors, if its retrieval engine has one.
type FinderResolver interface {
	// SimilarChunkFinder returns the knowledge base's finder, false when its
	// engine cannot compare stored vectors, or an error when the answer is
	// not known right now (the engine or the database is unreachable).
	SimilarChunkFinder(ctx context.Context, kb *types.KnowledgeBase) (interfaces.SimilarChunkFinder, bool, error)
}

// registryResolver resolves the finder the way every other background worker
// resolves a knowledge base's engine: through the engine registry, from the
// base's own vector-store binding, with ownership verified.
type registryResolver struct {
	registry  interfaces.RetrieveEngineRegistry
	ownership retriever.TenantStoreOwnership
	tenants   interfaces.TenantRepository
}

// NewFinderResolver returns the registry-backed resolver.
func NewFinderResolver(
	registry interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	tenants interfaces.TenantRepository,
) FinderResolver {
	return &registryResolver{registry: registry, ownership: ownership, tenants: tenants}
}

func (r *registryResolver) SimilarChunkFinder(ctx context.Context, kb *types.KnowledgeBase,
) (interfaces.SimilarChunkFinder, bool, error) {
	if kb == nil {
		return nil, false, nil
	}
	// A knowledge base not bound to a store uses its owner's default
	// engines, which the factory reads from the tenant in the context. The
	// owner, not the caller: for a shared base those differ.
	tenant, err := r.tenants.GetTenantByID(ctx, kb.TenantID)
	if err != nil {
		return nil, false, fmt.Errorf("loading tenant %d: %w", kb.TenantID, err)
	}
	if tenant == nil {
		return nil, false, nil
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	engine, err := retriever.CreateRetrieveEngineForKB(ctx, r.registry, r.ownership, kb.TenantID, kb.VectorStoreID)
	switch {
	case errors.Is(err, retriever.ErrVectorStoreNotFound), errors.Is(err, retriever.ErrVectorStoreForbidden):
		// Waiting does not bring either back.
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	finder, ok := engine.SimilarChunkFinder()
	return finder, ok, nil
}
