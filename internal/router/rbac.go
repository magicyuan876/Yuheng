package router

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// CHOOSING THE RIGHT GUARD (read this before adding a new route)
// ==============================================================
//
// The four role-only guards (Viewer / Contributor / Admin / Owner) ask
// "what is the caller's role in this tenant?". The two ownership
// guards (OwnedKBOrAdmin and the per-sub-resource variants) ask "is the
// caller the creator of THIS resource OR at least Admin+?".
//
// Picking the wrong one is the single most common source of RBAC
// bugs in this repo (we caught FAQ/Tag and KB share wired against the
// wrong axis). Two
// questions decide it:
//
// Q1. Does the resource have a creator?
//
//	YES — KB, Knowledge document, Chunk, WikiPage, FAQ entry,
//	      KB tag, anything stamped with creator_id / created_by.
//	      => Mutating routes use OwnedXxxOrAdmin.
//	      The creator passes regardless of role; everyone else needs
//	      Admin+. This is what makes "Contributor in my own KB acts
//	      like Owner; Contributor in someone else's KB acts like
//	      Viewer" hold uniformly.
//
//	NO  — Tenant-wide infrastructure: Model, VectorStore,
//	      WebSearchProvider, DataSource credentials.
//	      => Mutating routes use Admin().
//	      There is no "creator-of-the-vector-store" concept; configuring
//	      it affects everyone, so only Admin+ may touch it.
//
//	ENTRY POINT — Routes that CREATE a new owned resource (POST
//	      /knowledge-bases).
//	      => Use Contributor() (or whatever the floor is).
//	      No resource exists yet, so we can only gate on role. Once
//	      created, future mutations on /:id flip to OwnedXxxOrAdmin.
//
// Q2. Is the side effect "private to me" or "visible to others"?
//
//	PRIVATE — Action only affects the caller's own state.
//	      => Contributor() is fine.
//
//	PUBLIC — Action exposes a resource beyond its current scope or
//	      changes state visible to other tenants/users (sharing a KB
//	      to an org, transferring ownership).
//	      => OwnedXxxOrAdmin (when the action targets a specific
//	      owned resource) or Admin (when it's tenant-wide).
//	      Contributor is wrong here even though the role floor passes:
//	      "I am a Contributor in this tenant" does not mean "I may
//	      expose my colleague's KB to the world".
//
// User experience this matrix produces
// ------------------------------------
// The user never sees the guard names. They see this:
//
//   - As Owner / Admin: I can manage everything in my tenant.
//   - As Contributor: I can manage what I created. Other people's
//     resources behave like read-only, regardless of which UI tab.
//   - As Viewer: read everything, mutate nothing.
//   - Creating new resources (KB, chat session) requires being at
//     least Contributor.
//   - Configuring tenant infrastructure (models, vector stores, etc.)
//     requires Admin+.
//
// If a route makes a Contributor surprised that they CAN'T do
// something they own, the gate is too tight (probably Admin where it
// should be OwnedXxxOrAdmin). If a route makes a Contributor surprised
// they CAN do something to someone else's resource, the gate is too
// loose (Contributor where it should be OwnedXxxOrAdmin). Both
// surprises are bugs.
//
// Sub-resources must align with their parent
// ------------------------------------------
// Chunks/wiki pages/FAQ entries/tags inherit their parent KB's gate.
// The KBCreatorLookupFromKnowledgeID / KBCreatorLookupFromKBPath /
// etc. lookups walk the URL param up to the KB and reuse its
// creator_id. Don't add a new sub-resource with a freshly-invented
// gate (a recurring source of "Contributor everywhere" drift).
//
// rbacGuards is the centralised role-matrix bundle for tenant-level RBAC
// (issue #1303 PR 2). NewRouter constructs it once and threads it into
// each Register* function that registers gated routes.
//
// Each method returns a fresh gin.HandlerFunc; routes call the method
// and inline the guard, so a glance at a route line tells you what
// authority it requires:
//
//	kb.PUT("/:id", g.OwnedKBOrAdmin(), handler.UpdateKnowledgeBase)
//
// All guards honour cfg.Tenant.EnableRBAC: when the flag is off they log
// the would-be rejection and let the request through, preserving today's
// "anyone in the tenant can edit anything" behaviour during the rollout
// window. When the flag flips to true, the same code paths start
// rejecting unauthorised callers.
type rbacGuards struct {
	cfg *config.Config

	// Lookup closures resolve a request's :id into the resource's creator
	// user ID. Captured up front so the handler-level methods don't have
	// to be exported into every Register* function as well.
	kbCreator middleware.CreatorLookup
	// kbCreatorFromKbIDParam reads :kbId (not :id) for the
	// /initialization/* routes whose KB is addressed by :kbId.
	kbCreatorFromKbIDParam middleware.CreatorLookup
	// Per-KB-ownership lookups for knowledge / chunk / wiki page routes
	// (PR 5, #1303). They walk the URL param back to KB.CreatorID so a
	// Contributor who owns the KB can edit/delete its sub-resources
	// (documents, chunks, wiki pages); a Contributor who merely belongs
	// to the tenant gets 403 unless they're also Admin+.
	knowledgeKBCreator   middleware.CreatorLookup
	chunkKBCreator       middleware.CreatorLookup
	chunkKBCreatorFromID middleware.CreatorLookup // chunk routes that address chunks by :id (no knowledge id in URL)
	wikiKBCreator        middleware.CreatorLookup

	// Services for the KB-access guard. Captured here so route lines can
	// reference g.KBAccess() without having to plumb the services through
	// every Register* function.
	kbService        middleware.KBLookup
	knowledgeService middleware.KnowledgeLookup
	chunkService     middleware.ChunkLookup

	// apiKeyAuthorizer is the single source of truth for which routes an
	// X-API-Key principal may call. Routes opt in via the apiKeyGroup
	// helpers below; anything not declared is denied by the gate. See
	// middleware.APIKeyRouteAuthorizer.
	apiKeyAuthorizer *middleware.APIKeyRouteAuthorizer

	// centralizedInfra reports the live governance.centralized_infra
	// setting; PlatformManaged() consults it per request. Nil when the
	// system-setting service is unavailable, which PlatformManaged treats
	// as "decentralised" so a wiring gap cannot lock workspace admins out
	// of their own infrastructure.
	centralizedInfra middleware.CentralizedInfraResolver
}

// newRBACGuards wires the guards from the live configuration and the
// already-built handlers. Called once from NewRouter.
func newRBACGuards(
	cfg *config.Config,
	kbHandler *handler.KnowledgeBaseHandler,
	knowledgeHandler *handler.KnowledgeHandler,
	chunkHandler *handler.ChunkHandler,
	wikiHandler *handler.WikiPageHandler,
	kbService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	chunkService interfaces.ChunkService,
	systemSettingService interfaces.SystemSettingService,
) *rbacGuards {
	g := &rbacGuards{cfg: cfg, apiKeyAuthorizer: middleware.NewAPIKeyRouteAuthorizer()}
	if systemSettingService != nil {
		g.centralizedInfra = func(ctx context.Context) bool {
			return systemSettingService.GetBool(
				ctx, types.SettingKeyCentralizedInfra, types.SettingEnvCentralizedInfra, false,
			)
		}
	}
	if kbHandler != nil {
		g.kbCreator = kbHandler.KBCreatorLookup
		g.kbCreatorFromKbIDParam = kbHandler.KBCreatorLookupFromKbIDParam
	}
	if knowledgeHandler != nil {
		g.knowledgeKBCreator = knowledgeHandler.KBCreatorLookupFromKnowledgeID
	}
	if chunkHandler != nil {
		g.chunkKBCreator = chunkHandler.KBCreatorLookupFromKnowledgeIDParam
		g.chunkKBCreatorFromID = chunkHandler.KBCreatorLookupFromChunkIDParam
	}
	if wikiHandler != nil {
		g.wikiKBCreator = wikiHandler.KBCreatorLookupFromKBPath
	}
	g.kbService = kbService
	g.knowledgeService = knowledgeService
	g.chunkService = chunkService
	return g
}

// Role-only guards — pure RequireRole convenience wrappers, named after
// the matrix entries so route lines stay readable.

func (g *rbacGuards) Viewer() gin.HandlerFunc {
	return middleware.RequireRole(types.TenantRoleViewer, g.cfg)
}

func (g *rbacGuards) Contributor() gin.HandlerFunc {
	return middleware.RequireRole(types.TenantRoleContributor, g.cfg)
}

func (g *rbacGuards) Admin() gin.HandlerFunc {
	return middleware.RequireRole(types.TenantRoleAdmin, g.cfg)
}

func (g *rbacGuards) AdminOrSystemAdmin() gin.HandlerFunc {
	return middleware.RequireRoleOrSystemAdmin(types.TenantRoleAdmin, g.cfg)
}

// PlatformManaged gates a WRITE to shared infrastructure (models,
// web-search providers, vector stores, storage backends, parser
// engines, Ollama). It is Admin+ while
// governance.centralized_infra is off and SystemAdmin-only once it is on.
//
// Pair it only with writes. The matching read routes stay on Viewer() so the
// knowledge-base editor can still list and select platform resources at
// point of use — that read path is what makes centralised mode usable rather
// than merely restrictive.
func (g *rbacGuards) PlatformManaged() gin.HandlerFunc {
	return middleware.RequirePlatformManaged(types.TenantRoleAdmin, g.cfg, g.centralizedInfra)
}

func (g *rbacGuards) Owner() gin.HandlerFunc {
	return middleware.RequireRole(types.TenantRoleOwner, g.cfg)
}

// API-key authorization — a SEPARATE authority from the JWT role/ownership
// guards above. Instead of stacking a per-route guard that also had to know
// the caller's ownership, every API-key-accessible route declares one
// APIKeyRoutePolicy via the apiKeyGroup helpers; the gate on /api/v1 enforces
// it and denies any undeclared route by default. JWT sessions ignore all of
// this (they short-circuit the gate).
//
// Policy constructors. API keys do not reuse tenant-member roles: a key is
// either full-access, or it carries explicit capabilities. KB allow-lists are
// pure data filters applied downstream by KBAccess guards and handlers.

func apiKeyAny() middleware.APIKeyRoutePolicy {
	return middleware.APIKeyRoutePolicy{}
}

func apiKeyFullAccess() middleware.APIKeyRoutePolicy {
	return middleware.APIKeyRoutePolicy{RequireFullAccess: true}
}

func apiKeyPlatform(capabilities ...types.APIKeyCapability) middleware.APIKeyRoutePolicy {
	policy := middleware.APIKeyRoutePolicy{PlatformOnly: true}
	for _, capability := range capabilities {
		policy = policy.WithCapability(capability)
	}
	return policy
}

// apiKeyRetrieve grants read/search access to knowledge-base data.
func apiKeyRetrieve(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityRetrieve)
}

// apiKeyChat layers the "chat" capability on top of a base policy: keys that
// carry the chat capability can use the conversation flow (sessions)
// without full tenant access.
func apiKeyChat(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityChat)
}

// apiKeyIngest layers the "ingest" capability on top of a base policy so a
// scoped key can write content into its allowed knowledge bases (documents,
// chunks, FAQ, tags, wiki).
func apiKeyIngest(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityIngest)
}

// apiKeyManageKnowledgeBases layers the "manage_kbs" capability on top of a
// base policy so a scoped key can manage the KB lifecycle (create/copy/
// duplicate/update/delete + config). Existing-KB operations stay bounded by
// the key's allow-list downstream; create has no source to bound.
func apiKeyManageKnowledgeBases(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageKnowledgeBases)
}

// apiKeyMessageHistory layers the "message_history" capability on top of a
// base policy so an explicitly granted key can search or inspect tenant chat
// history without being promoted to full Owner.
func apiKeyMessageHistory(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityMessageHistory)
}

func apiKeyManageModels(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageModels)
}

func apiKeyManageDataSources(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageDataSources)
}

func apiKeyManageVectorStores(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageVectorStores)
}

// apiKeyManageStorageBackends layers the "manage_storage_backends" capability
// on top of a base policy so a scoped key can manage object/file storage
// backend instances without carrying vector-store or full tenant access.
func apiKeyManageStorageBackends(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageStorageBackends)
}

func apiKeyManageWebSearch(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageWebSearch)
}

func apiKeyRunEvaluations(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityRunEvaluations)
}

func apiKeyManageMembers(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageMembers)
}

func apiKeyManageTenantSettings(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageTenantSettings)
}

// apiKeyRouteGroup wraps a *gin.RouterGroup so route registration also records
// the route's API-key policy into the authorizer. Use g.apiKeyGroup(grp,
// policy) then register the API-key-accessible routes through it; register
// API-key-denied routes on the raw *gin.RouterGroup so they stay undeclared
// (default-deny). Per-route overrides use With().
type apiKeyRouteGroup struct {
	g      *rbacGuards
	grp    *gin.RouterGroup
	policy middleware.APIKeyRoutePolicy
}

// ensureAPIKeyAuthorizer lazily allocates the authorizer so route
// registration is safe even when rbacGuards is built directly in tests
// (bypassing newRBACGuards). In production it is always pre-allocated.
func (g *rbacGuards) ensureAPIKeyAuthorizer() *middleware.APIKeyRouteAuthorizer {
	if g.apiKeyAuthorizer == nil {
		g.apiKeyAuthorizer = middleware.NewAPIKeyRouteAuthorizer()
	}
	return g.apiKeyAuthorizer
}

// apiKeyGroup returns a wrapper that declares `policy` for every route
// registered through it (unless overridden via With).
func (g *rbacGuards) apiKeyGroup(grp *gin.RouterGroup, policy middleware.APIKeyRoutePolicy) *apiKeyRouteGroup {
	g.ensureAPIKeyAuthorizer()
	return &apiKeyRouteGroup{g: g, grp: grp, policy: policy}
}

// With returns a sibling wrapper on the same gin group but with a different
// policy, for the odd route that differs from its group default (e.g. a read
// search inside an otherwise contributor-gated group).
func (a *apiKeyRouteGroup) With(policy middleware.APIKeyRoutePolicy) *apiKeyRouteGroup {
	return &apiKeyRouteGroup{g: a.g, grp: a.grp, policy: policy}
}

func (a *apiKeyRouteGroup) handle(method, rel string, handlers ...gin.HandlerFunc) gin.IRoutes {
	full := path.Join(a.grp.BasePath(), rel)
	a.g.ensureAPIKeyAuthorizer().Register(method, full, a.policy)
	return a.grp.Handle(method, rel, handlers...)
}

func (a *apiKeyRouteGroup) GET(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodGet, rel, h...)
}

func (a *apiKeyRouteGroup) POST(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPost, rel, h...)
}

func (a *apiKeyRouteGroup) PUT(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPut, rel, h...)
}

func (a *apiKeyRouteGroup) PATCH(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPatch, rel, h...)
}

func (a *apiKeyRouteGroup) DELETE(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodDelete, rel, h...)
}

// apiKeyRoute declares a single API-key-accessible route directly on a gin
// group (for routes registered outside an apiKeyGroup, e.g. top-level r.POST).
func (g *rbacGuards) apiKeyRoute(
	grp *gin.RouterGroup, method, rel string, policy middleware.APIKeyRoutePolicy, handlers ...gin.HandlerFunc,
) gin.IRoutes {
	full := path.Join(grp.BasePath(), rel)
	g.ensureAPIKeyAuthorizer().Register(method, full, policy)
	return grp.Handle(method, rel, handlers...)
}

// assertAPIKeyPoliciesMatchRoutes verifies every declared API-key policy
// resolves to a real registered route. gin's c.FullPath() must match the
// authorizer key verbatim or the gate silently 403s the route for API keys;
// panicking here turns that latent misconfiguration into a startup failure.
func (g *rbacGuards) assertAPIKeyPoliciesMatchRoutes(engine *gin.Engine) {
	registered := map[string]struct{}{}
	for _, ri := range engine.Routes() {
		// Authorizer keys are stored normalized (trailing slash trimmed), so
		// normalize gin's reported path the same way. Otherwise a route
		// registered with a "/" rel (gin path ".../evaluation/") would look
		// missing against the normalized key (".../evaluation") even though
		// the gate — which also normalizes c.FullPath() — matches it fine.
		p := ri.Path
		if len(p) > 1 {
			p = strings.TrimRight(p, "/")
		}
		registered[ri.Method+" "+p] = struct{}{}
	}
	var missing []string
	for method, paths := range g.apiKeyAuthorizer.RegisteredRoutes() {
		for _, p := range paths {
			if _, ok := registered[method+" "+p]; !ok {
				missing = append(missing, method+" "+p)
			}
		}
	}
	if len(missing) > 0 {
		panic("api-key policy declared for non-existent route(s): " + strings.Join(missing, ", "))
	}
}

func (g *rbacGuards) SystemAdmin() gin.HandlerFunc {
	return middleware.RequireSystemAdmin(g.cfg)
}

// Ownership-or-role guards. Required role here is the privilege level
// that bypasses the ownership check; Contributors ALWAYS pass when they
// own the resource.

// OwnedKBOrAdmin: KB mutations (update/delete/pin/copy). The original
// creator may proceed; otherwise Admin+ is required. Contributors who
// did not create the KB get 403 (when enforcement is on).
func (g *rbacGuards) OwnedKBOrAdmin() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.kbCreator, g.cfg)
}

// OwnedKBOrAdminFromKbIDParam is the same matrix as OwnedKBOrAdmin but
// addresses the KB via :kbId (used by /initialization/* routes). KB
// configuration changes — picking the embedding/parser/storage
// engine, materialising indexes — are at least as sensitive as
// updating the KB itself, so they share the "creator OR Admin+" rule.
func (g *rbacGuards) OwnedKBOrAdminFromKbIDParam() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.kbCreatorFromKbIDParam, g.cfg)
}

// OwnedKnowledgeKBOrAdmin: per-knowledge mutations (update / delete /
// reparse / image edit) — the URL :id is a knowledge id, the lookup
// walks it back to the owning KB's CreatorID. Same "creator OR Admin+"
// rule as OwnedKBOrAdmin, just one chain hop deeper. PR 5 (#1303).
func (g *rbacGuards) OwnedKnowledgeKBOrAdmin() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.knowledgeKBCreator, g.cfg)
}

// OwnedChunkKBOrAdmin: chunk mutations addressed via :knowledge_id.
// Reuses the same chain helper as OwnedKnowledgeKBOrAdmin so a
// Contributor with KB ownership can manage all chunks under any of
// their documents. For chunk routes addressed via :id (no knowledge
// id in the URL — only chunks.DELETE("/by-id/:id/questions") today),
// see OwnedChunkKBOrAdminFromChunkID below: same matrix, walks one
// extra hop (chunk_id -> knowledge_id) before reusing this chain.
func (g *rbacGuards) OwnedChunkKBOrAdmin() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.chunkKBCreator, g.cfg)
}

// OwnedChunkKBOrAdminFromChunkID: chunk mutations addressed via :id
// (the chunk's own id, no knowledge id in the URL). Used by
// chunks.DELETE("/by-id/:id/questions"). Same OwnedKBOrAdmin matrix
// as the rest of the chunk routes — earlier this endpoint stayed at
// flat Contributor because the chunk-id -> knowledge-id -> kb chain
// wasn't wired; that's now plumbed through KBCreatorLookupFromChunkIDParam.
func (g *rbacGuards) OwnedChunkKBOrAdminFromChunkID() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.chunkKBCreatorFromID, g.cfg)
}

// OwnedWikiKBOrAdmin: wiki page CRUD and maintenance ops. Wiki routes
// use :kb_id directly so the lookup is a single hop into the KB
// service — no knowledge chain. Same matrix as OwnedKBOrAdmin.
func (g *rbacGuards) OwnedWikiKBOrAdmin() gin.HandlerFunc {
	return middleware.RequireOwnershipOrRole(types.TenantRoleAdmin, g.wikiKBCreator, g.cfg)
}

// Tenant-access guards. Distinct from the role guards above: these
// answer the orthogonal question "may this caller touch this tenant
// at all", before role membership inside the tenant is even
// considered. Both delegate to middleware/access.go which centralises
// the cross-tenant rules so the router stays declarative.

// CrossTenant gates a route on the caller being an org-level
// superuser (CanAccessAllTenants AND EnableCrossTenantAccess). Used by
// /tenants/all, /tenants/search, POST /tenants, GET /tenants — the
// endpoints that operate across tenants. Replaces the if-blocks that
// used to live inside ListAllTenants/SearchTenants/CreateTenant.
func (g *rbacGuards) CrossTenant() gin.HandlerFunc {
	return middleware.RequireCrossTenantAccess(g.cfg)
}

// PathTenantMatch enforces that the URL :id matches the caller's
// active tenant context (cross-tenant superusers bypass). Routes apply
// it at the /tenants/:id group level so every per-tenant endpoint —
// GetTenant / UpdateTenant / DeleteTenant / member
// management / leave — shares the same check. Replaces the
// authorizeTenantAccess helper that used to live inside the tenant
// handler.
func (g *rbacGuards) PathTenantMatch() gin.HandlerFunc {
	return middleware.RequirePathTenantMatch(g.cfg)
}

// KB-access guards — orthogonal to the role-and-ownership matrix
// above. They answer "does THIS knowledge base belong to the caller's
// workspace?" (and, for a restricted API key, "is it on the key's
// allow-list?"). A base owned by another workspace is reported as not
// found. What the caller may then do with the base is the role and
// ownership guards' decision, so one guard serves reads and writes alike;
// the three variants differ only in where the kb id comes from.
//
// On success the resolved knowledge base is stashed on c.Keys under
// middleware.KBAccessContextKey for handlers that want it without a
// second lookup.
//
// These guards replace the per-handler validate* helpers that used to be
// re-implemented in chunk.go, faq.go, tag.go, knowledge.go and
// knowledgebase.go; the resolution now lives in exactly one place
// (middleware/kb_access.go).

// KBAccess gates a KB-scoped route on the base belonging to the caller's
// workspace. The kbID is read from the gin param named in `param`
// (typically "id" for /knowledge-bases/:id/...).
func (g *rbacGuards) KBAccess(param string) gin.HandlerFunc {
	return middleware.RequireKBAccess(middleware.KBIDFromParam(param), g.kbService)
}

// KBAccessFromKnowledgeIDParam is like KBAccess but resolves the kb_id by
// walking a knowledge document (URL `:knowledge_id`) back to its parent
// KB. Used by the routes whose URL addresses a document rather than a
// base, such as /knowledge/:id and /chunks/:knowledge_id.
func (g *rbacGuards) KBAccessFromKnowledgeIDParam(param string) gin.HandlerFunc {
	return middleware.RequireKBAccess(
		middleware.KBIDFromKnowledgeIDParam(param, g.knowledgeService),
		g.kbService,
	)
}

// KBAccessFromChunkIDParam walks chunk_id -> kb_id (using the chunk's
// denormalised KnowledgeBaseID column). Used by /chunks/by-id/:id routes.
func (g *rbacGuards) KBAccessFromChunkIDParam(param string) gin.HandlerFunc {
	return middleware.RequireKBAccess(
		middleware.KBIDFromChunkIDParam(param, g.chunkService),
		g.kbService,
	)
}
