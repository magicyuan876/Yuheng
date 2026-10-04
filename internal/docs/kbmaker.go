package docs

import (
	"context"
	"errors"
	"sort"

	"github.com/magicyuan876/yuheng/internal/docs/service"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The adapter that makes the knowledge base a new space asks for, through
// Yuheng's own knowledge-base service, so the space's knowledge base is an
// ordinary one: audited as created, owned by whoever created the space, and
// configurable afterwards like any other.
//
// ---- decision: the workspace's default models, the platform's defaults
//
// The form that creates a space asks nothing about chunking or models, and
// should not: somebody creating a docs space is not choosing an embedding
// model. The knowledge base gets what the knowledge-base editor would have
// preselected — the model marked default for each type, else the oldest
// active one — and the platform's default chunking and indexing (vector and
// keyword). Without an embedding model nothing is made: those defaults need
// one, and a knowledge base that stores pages and never answers from them is
// the failure nobody would notice.

type kbMaker struct {
	bases  interfaces.KnowledgeBaseService
	models interfaces.ModelService
}

// NewKnowledgeBaseMaker adapts Yuheng's services for creating a space's
// knowledge base. A missing dependency yields nil, which makes the "create"
// choice unavailable while binding an existing knowledge base still works.
func NewKnowledgeBaseMaker(bases interfaces.KnowledgeBaseService, models interfaces.ModelService,
) service.KnowledgeBaseMaker {
	if bases == nil || models == nil {
		return nil
	}
	return &kbMaker{bases: bases, models: models}
}

// CreateForSpace implements service.KnowledgeBaseMaker. It runs in the
// request of the person creating the space, so the knowledge-base service
// records them as the creator, exactly as if they had made it by hand.
func (m *kbMaker) CreateForSpace(ctx context.Context, tenantID uint64, name, description,
	storageBackendID string,
) (*types.KnowledgeBase, error) {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	models, err := m.models.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	embedding := defaultModelID(models, types.ModelTypeEmbedding)
	if embedding == "" {
		return nil, apperrors.NewEmbeddingModelRequiredError()
	}
	kb, err := m.bases.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
		Name:             name,
		Description:      description,
		Type:             types.KnowledgeBaseTypeDocument,
		EmbeddingModelID: embedding,
		// Optional: summaries and the features built on them simply stay
		// off without one, which is no reason to refuse the space.
		SummaryModelID:   defaultModelID(models, types.ModelTypeKnowledgeQA),
		StorageBackendID: storageBackendID,
	})
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, errors.New("docs: the knowledge-base service returned no knowledge base")
	}
	return kb, nil
}

// DeleteKnowledgeBase implements service.KnowledgeBaseMaker.
func (m *kbMaker) DeleteKnowledgeBase(ctx context.Context, tenantID uint64, id string) error {
	return m.bases.DeleteKnowledgeBase(context.WithValue(ctx, types.TenantIDContextKey, tenantID), id)
}

// defaultModelID picks the model the knowledge-base editor would preselect
// (frontend/src/utils/modelDefaults.ts): among the active models of the type,
// the one marked default, else the oldest. Oldest rather than "first", because
// the repository returns models in no particular order and the choice must not
// change from one space to the next.
func defaultModelID(models []*types.Model, modelType types.ModelType) string {
	var active []*types.Model
	for _, m := range models {
		if m == nil || m.ID == "" || m.Type != modelType {
			continue
		}
		if m.Status != "" && m.Status != types.ModelStatusActive {
			continue
		}
		active = append(active, m)
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].IsDefault != active[j].IsDefault {
			return active[i].IsDefault
		}
		if !active[i].CreatedAt.Equal(active[j].CreatedAt) {
			return active[i].CreatedAt.Before(active[j].CreatedAt)
		}
		return active[i].ID < active[j].ID
	})
	if len(active) == 0 {
		return ""
	}
	return active[0].ID
}
