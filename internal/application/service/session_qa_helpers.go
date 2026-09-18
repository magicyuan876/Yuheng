package service

import (
	"context"
	"strings"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// ---------------------------------------------------------------------------
// Shared QA helpers: KB resolution, model resolution, retrieval tenant
// ---------------------------------------------------------------------------

// resolveKnowledgeBases resolves the effective knowledge base IDs and knowledge IDs
// for a QA request. Priority:
//  1. Explicit @mentions (request-specified kbIDs / knowledgeIDs)
//  2. RetrieveKBOnlyWhenMentioned -> disable KB if no mention
func (s *sessionService) resolveKnowledgeBases(
	ctx context.Context,
	req *types.QARequest,
) (kbIDs []string, knowledgeIDs []string, err error) {
	kbIDs = req.KnowledgeBaseIDs
	knowledgeIDs = req.KnowledgeIDs
	requestedKBIDs := append([]string(nil), req.KnowledgeBaseIDs...)

	if len(kbIDs) > 0 || len(knowledgeIDs) > 0 || len(req.TagScopes) > 0 {
		logger.Infof(ctx, "Using request-specified targets: kbs=%v, docs=%v", kbIDs, knowledgeIDs)
	}

	if err := types.AuthorizeTenantAPIKeyKnowledgeTargets(ctx, requestedKBIDs, req.KnowledgeIDs); err != nil {
		return nil, nil, err
	}
	kbIDs, err = types.FilterKnowledgeBasesForTenantAPIKeyScope(ctx, requestedKBIDs, kbIDs)
	if err != nil {
		return nil, nil, err
	}
	return kbIDs, knowledgeIDs, nil
}

// resolveChatModelID resolves the effective chat model ID for a QA request.
// A request-level override may choose a valid model; otherwise the legacy
// KB / session / system fallback chain is used.
func (s *sessionService) resolveChatModelID(
	ctx context.Context,
	req *types.QARequest,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
) (string, error) {
	summaryModelID := strings.TrimSpace(req.SummaryModelID)
	if summaryModelID != "" {
		if model, err := s.modelService.GetModelByID(ctx, summaryModelID); err == nil && model != nil &&
			model.Type == types.ModelTypeKnowledgeQA {
			logger.Infof(ctx, "Using request's summary model override: %s", summaryModelID)
			return summaryModelID, nil
		}
		logger.Warnf(ctx, "Request provided invalid summary model ID %s, falling back", summaryModelID)
	}
	return s.selectChatModelID(ctx, req.Session, knowledgeBaseIDs, knowledgeIDs)
}

// resolveRetrievalTenantID determines the tenant ID to use for retrieval scope.
// Priority: context tenant > session tenant.
func (s *sessionService) resolveRetrievalTenantID(
	ctx context.Context,
	req *types.QARequest,
) uint64 {
	retrievalTenantID := req.Session.TenantID
	if v := ctx.Value(types.TenantIDContextKey); v != nil {
		if tid, ok := v.(uint64); ok && tid != 0 {
			retrievalTenantID = tid
			logger.Infof(ctx, "Using effective tenant %d for retrieval from context", retrievalTenantID)
		}
	}
	return retrievalTenantID
}
