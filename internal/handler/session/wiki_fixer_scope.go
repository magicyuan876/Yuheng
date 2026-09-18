package session

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// BuiltinWikiFixerAgentID is the request marker the wiki editor sends to ask
// for the wiki-fix chat behaviour (chat scoped to a single wiki KB, with
// shared-KB edits running in the source workspace). It no longer resolves to
// an agent; the backend only uses it to pick the retrieval/model tenant scope.
const BuiltinWikiFixerAgentID = "builtin-wiki-fixer"

type wikiFixerKBLookup interface {
	GetKnowledgeBaseByIDOnly(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

type wikiFixerKBSharePermission interface {
	CheckTenantKBPermission(ctx context.Context, kbID string, callerTenantID uint64, callerTenantRole types.TenantRole) (types.OrgMemberRole, bool, error)
}

// resolveWikiFixerTenantScope is the handler-bound wrapper used by the QA
// flow. Returns the effective tenant ID to run model/KB resolution in, or 0
// to keep the caller's own tenant.
func (h *Handler) resolveWikiFixerTenantScope(
	ctx context.Context,
	currentTenantID uint64,
	callerTenantRole types.TenantRole,
	kbIDs []string,
) uint64 {
	return resolveSingleSharedKBChatTenantScope(
		ctx,
		currentTenantID,
		callerTenantRole,
		kbIDs,
		h.knowledgebaseService,
		h.kbShareService,
	)
}

// resolveSingleSharedKBChatTenantScope scopes a single-KB chat to the source
// workspace of an organization-shared KB when the caller has edit permission
// there. That lets KB-scoped models and retrieval resolve in the owning
// workspace (e.g. the wiki fixer repairing a shared wiki) without granting
// viewers any write capability. Multi-KB chats and own-KB chats keep the
// caller's tenant.
func resolveSingleSharedKBChatTenantScope(
	ctx context.Context,
	currentTenantID uint64,
	callerTenantRole types.TenantRole,
	kbIDs []string,
	kbLookup wikiFixerKBLookup,
	kbShare wikiFixerKBSharePermission,
) uint64 {
	if currentTenantID == 0 || len(kbIDs) != 1 || kbLookup == nil || kbShare == nil {
		return 0
	}

	kbID := kbIDs[0]
	kb, err := kbLookup.GetKnowledgeBaseByIDOnly(ctx, kbID)
	if err != nil {
		logger.Warnf(ctx, "wiki fixer: failed to resolve KB %s for shared scope: %v", secutils.SanitizeForLog(kbID), err)
		return 0
	}
	if kb == nil {
		logger.Warnf(ctx, "wiki fixer: KB %s not found for shared scope", secutils.SanitizeForLog(kbID))
		return 0
	}
	if kb.TenantID == 0 || kb.TenantID == currentTenantID {
		return 0
	}

	permission, isShared, err := kbShare.CheckTenantKBPermission(ctx, kb.ID, currentTenantID, callerTenantRole)
	if err != nil {
		logger.Warnf(ctx, "wiki fixer: failed to check shared KB %s permission: %v", secutils.SanitizeForLog(kb.ID), err)
		return 0
	}
	if !isShared || !permission.HasPermission(types.OrgRoleEditor) {
		return 0
	}

	logger.Infof(ctx, "wiki fixer: using shared KB source tenant %d for KB %s", kb.TenantID, secutils.SanitizeForLog(kb.ID))
	return kb.TenantID
}
