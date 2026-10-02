package middleware

import (
	"context"
	stderrors "errors"

	"github.com/gin-gonic/gin"
	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// kb_access.go is the single place that answers "is this knowledge base
// reachable from the caller's workspace?" for KB-scoped routes. It
// replaced near-identical 30-line helpers in five handler files (chunk.go,
// faq.go, tag.go, knowledge.go, knowledgebase.go), so a change in the
// resolution propagates to every gated route at once.
//
// The answer is deliberately binary. A knowledge base belongs to exactly
// one workspace and is reachable only from there; what the caller may
// then *do* with it is the sibling role and ownership guards' business
// (RequireRole, RequireOwnershipOrRole). A knowledge base owned by another
// workspace does not exist as far as the caller is concerned and is
// reported as not found, so probing ids never reveals which workspace
// owns what.
//
// The guard also runs the per-API-key knowledge base allow-list
// (types.AuthorizeTenantAPIKeyKnowledgeBases) so a key restricted to some
// knowledge bases is confined to them on every route that names one,
// including routes whose only parameter is a document or chunk id.
//
// The ACL design hangs its per-knowledge-base access-mode gate here: a base
// whose entries carry their own permissions, served by a build that cannot
// evaluate them, must be refused as a whole at this point.

// KBAccess captures the result of a successful KB access resolution.
// Stashed on gin.Context under KBAccessContextKey so handlers that need
// the resolved knowledge base can pull it without re-running the lookup.
type KBAccess struct {
	KnowledgeBase *types.KnowledgeBase
}

// KBAccessContextKey is the gin.Context key under which a successful
// KB access resolution is stored.
const KBAccessContextKey = "rbac.kb_access"

// KBAccessFromContext returns the KBAccess stashed by the guard, if
// any. Handlers that only need the tenant keep reading it from the request
// context, which the guard leaves untouched.
func KBAccessFromContext(c *gin.Context) (*KBAccess, bool) {
	v, ok := c.Get(KBAccessContextKey)
	if !ok {
		return nil, false
	}
	a, ok := v.(*KBAccess)
	return a, ok
}

// KBLookup is the minimum surface ResolveKBAccess needs from the
// knowledge-base service: a single method that turns an ID into a
// KnowledgeBase pointer (or repo.ErrKnowledgeBaseNotFound). Defining
// it as a tiny dedicated interface keeps the guard testable without
// forcing test stubs to satisfy the full KnowledgeBaseService surface.
type KBLookup interface {
	GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

// KnowledgeLookup mirrors KBLookup but for resolving a knowledge id
// (document id) back to its parent KB. Used by the chunk routes whose
// URL param is a knowledge_id, not a kb_id.
type KnowledgeLookup interface {
	GetKnowledgeByIDOnly(ctx context.Context, id string) (*types.Knowledge, error)
}

// ChunkLookup mirrors KBLookup for resolving a chunk id back to its
// owning knowledge document, which then resolves to the parent KB.
// Used by the /chunks/by-id/:id routes that address chunks directly.
type ChunkLookup interface {
	GetChunkByIDOnly(ctx context.Context, id string) (*types.Chunk, error)
}

// KBIDResolver tells the guard how to find the kb_id for a given
// request. Built-in resolvers below cover the param shapes we use:
// :id, :kb_id, :kbId, :knowledge_id (-> parent KB).
//
// On error, resolvers MUST return either a 4xx apperror (bad request /
// not found) or a generic Go error for transient/internal failures;
// the guard maps the latter to 503.
type KBIDResolver func(c *gin.Context) (string, error)

// KBIDFromParam returns a resolver that reads a fixed gin param.
func KBIDFromParam(param string) KBIDResolver {
	return func(c *gin.Context) (string, error) {
		v := c.Param(param)
		if v == "" {
			return "", apperrors.NewBadRequestError("missing " + param + " in path")
		}
		return v, nil
	}
}

// KBIDFromKnowledgeIDParam reads `:knowledge_id` from the URL, looks
// up the knowledge document, and returns its KB id. Used by the chunk
// routes that address a chunk via /chunks/:knowledge_id.
//
// A genuine "not found" maps to 404; transient errors (DB hiccup,
// service unavailable) are surfaced as a plain Go error so the guard
// can return 503 instead of pretending the resource doesn't exist
// (a 404 here would also short-circuit any retry / monitoring).
func KBIDFromKnowledgeIDParam(param string, kgService KnowledgeLookup) KBIDResolver {
	return func(c *gin.Context) (string, error) {
		v := c.Param(param)
		if v == "" {
			return "", apperrors.NewBadRequestError("missing " + param + " in path")
		}
		k, err := kgService.GetKnowledgeByIDOnly(c.Request.Context(), v)
		if err != nil {
			if isResourceNotFound(err) {
				return "", apperrors.NewNotFoundError("Knowledge not found")
			}
			return "", err
		}
		if k == nil {
			return "", apperrors.NewNotFoundError("Knowledge not found")
		}
		return k.KnowledgeBaseID, nil
	}
}

// KBIDFromChunkIDParam walks chunk_id -> knowledge_id -> kb_id.
// Used by /chunks/by-id/:id routes that address a chunk directly. The
// chunk's KnowledgeBaseID is denormalised on the row, so a single
// lookup is enough — no need to chain through GetKnowledgeByIDOnly.
//
// Not-found / transient split mirrors KBIDFromKnowledgeIDParam.
func KBIDFromChunkIDParam(param string, chunkService ChunkLookup) KBIDResolver {
	return func(c *gin.Context) (string, error) {
		v := c.Param(param)
		if v == "" {
			return "", apperrors.NewBadRequestError("missing " + param + " in path")
		}
		ch, err := chunkService.GetChunkByIDOnly(c.Request.Context(), v)
		if err != nil {
			if isResourceNotFound(err) {
				return "", apperrors.NewNotFoundError("Chunk not found")
			}
			return "", err
		}
		if ch == nil {
			return "", apperrors.NewNotFoundError("Chunk not found")
		}
		if ch.KnowledgeBaseID == "" {
			// Should never happen: every writer sets the KB. If it does,
			// the chunk isn't resolvable to a KB, so the client gets the
			// same 404 they'd get for a missing chunk rather than a 500
			// that pollutes alerting.
			logger.Warnf(c.Request.Context(),
				"[kb_access] chunk %s has empty knowledge_base_id; treating as not-found", v)
			return "", apperrors.NewNotFoundError("Chunk not found")
		}
		return ch.KnowledgeBaseID, nil
	}
}

// isResourceNotFound recognises the various "not found" sentinels we
// might see from the underlying services. Keeps the resolvers above
// from forcing every service to standardise on a single error type
// before this refactor is useful.
func isResourceNotFound(err error) bool {
	// ErrChunkNotFound is defined in the repository layer and aliased by the
	// service; match the canonical repo sentinel so this predicate depends
	// only on the repository package (KB / Knowledge / Chunk are all here).
	return stderrors.Is(err, apprepo.ErrKnowledgeBaseNotFound) ||
		stderrors.Is(err, apprepo.ErrKnowledgeNotFound) ||
		stderrors.Is(err, apprepo.ErrChunkNotFound) ||
		stderrors.Is(err, ErrResourceNotFound)
}

// RequireKBAccess returns a gin.HandlerFunc that resolves the knowledge
// base a route addresses, checks that it belongs to the caller's workspace
// (and to the API key's allow-list, if the caller is a restricted key) and
// on success stores it under KBAccessContextKey for the handler.
//
// On failure the guard aborts with the appropriate HTTP status: 400 for an
// unresolvable id, 401 without a workspace, 404 for a missing or
// foreign-workspace base, 503 when the lookup itself failed. A base that
// cannot be found is not an authorisation event, so unlike the role
// guards this one has no rollout flag: it enforces unconditionally.
func RequireKBAccess(resolveKBID KBIDResolver, kbService KBLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID, err := resolveKBID(c)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		if err := types.AuthorizeTenantAPIKeyKnowledgeBases(ctx, kbID); err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		access, err := resolveKBAccess(ctx, kbID, kbService)
		switch {
		case stderrors.Is(err, errKBAccessUnauthorized):
			_ = c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
			c.Abort()
			return
		case stderrors.Is(err, errKBAccessNotFound):
			_ = c.Error(apperrors.NewNotFoundError("knowledge base not found"))
			c.Abort()
			return
		case err != nil:
			logger.ErrorWithFields(ctx, err, nil)
			// Transient/internal -> 503 so monitoring catches the
			// underlying failure rather than a misleading 500.
			_ = c.Error(apperrors.NewServiceUnavailableError("cannot verify KB access right now"))
			c.Abort()
			return
		}

		c.Set(KBAccessContextKey, access)
		c.Next()
	}
}

// resolveKBAccess performs the lookup and the ownership check. Kept
// unexported and using package-private sentinel errors so the guard's
// error mapping is the only public surface.
func resolveKBAccess(ctx context.Context, kbID string, kbService KBLookup) (*KBAccess, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, errKBAccessUnauthorized
	}

	kb, err := kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		if stderrors.Is(err, apprepo.ErrKnowledgeBaseNotFound) {
			return nil, errKBAccessNotFound
		}
		return nil, err
	}
	if kb == nil {
		return nil, errKBAccessNotFound
	}
	if kb.TenantID != tenantID {
		// Logged at warn level: a client that holds a valid session and asks
		// for another workspace's base is either stale or probing, and either
		// is worth seeing in the logs even though the response is a plain 404.
		logger.Warnf(ctx, "[kb_access] tenant %d -> KB %s owned by tenant %d; reported as not found",
			tenantID, kbID, kb.TenantID)
		return nil, errKBAccessNotFound
	}
	return &KBAccess{KnowledgeBase: kb}, nil
}

var (
	errKBAccessUnauthorized = stderrors.New("kb_access: unauthorized")
	errKBAccessNotFound     = stderrors.New("kb_access: not found")
)
