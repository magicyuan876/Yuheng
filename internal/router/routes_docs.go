package router

import (
	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	dochandler "github.com/magicyuan876/yuheng/internal/docs/handler"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Online documents module routes (技术方案 §13).
//
// Three authorities stack on every route:
//
//  1. The API-key gate: each route declares which API-key capability reaches
//     it (docs_read / docs_write / docs_admin, or full access). Undeclared
//     routes are denied to API keys by default, like everywhere else.
//  2. The tenant role floor: Viewer for reads, Contributor for writes, Admin
//     for tenant-wide administration (groups).
//  3. The docs ACL guard: the caller's effective space/page role, resolved
//     from membership, groups and page restrictions, with 404 for resources
//     the caller cannot see.
//
// Handlers that belong to a later work package are wired to NotImplemented
// (501) so the contract — paths, methods, policies — is fixed and tested now
// and the module is never half-exposed: the whole tree is registered only when
// YUHENG_DOCS_ENABLED=true.

func apiKeyDocsRead(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityDocsRead)
}

func apiKeyDocsWrite(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityDocsWrite)
}

func apiKeyDocsAdmin(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityDocsAdmin)
}

// RegisterDocsRoutes mounts /api/v1/docs/** and the tenant group routes.
// A disabled module registers nothing.
func RegisterDocsRoutes(r *gin.RouterGroup, m *docs.Module, g *rbacGuards) {
	if m == nil || !m.Enabled || m.Handler == nil {
		return
	}
	h := m.Handler
	guard := m.Guard
	idem := h.Idempotency()
	ni := gin.HandlerFunc(dochandlerNotImplemented)

	read := g.apiKeyGroup(r.Group("/docs"), apiKeyDocsRead(apiKeyFullAccess()))
	write := read.With(apiKeyDocsWrite(apiKeyFullAccess()))
	admin := read.With(apiKeyDocsAdmin(apiKeyFullAccess()))

	// Live updates. One stream per tab, narrowed by ?space= / ?page=.
	read.GET("/events", g.Viewer(), guard.RequireMember(), h.Events.Handle)

	// ---- spaces (T1.1) ----------------------------------------------------------
	sp := h.Spaces
	read.GET("/spaces", g.Viewer(), guard.RequireMember(), sp.List)
	write.POST("/spaces", g.Contributor(), guard.RequireMember(), idem, sp.Create)
	read.GET("/spaces/:sid", g.Viewer(), guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), sp.Get)
	read.GET("/spaces/by-slug/:slug", g.Viewer(),
		guard.RequireSpace("slug", acl.SpaceBySlug, model.RoleReader), sp.GetBySlug)
	write.PATCH("/spaces/:sid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, sp.Update)
	write.DELETE("/spaces/:sid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, sp.Delete)
	write.POST("/spaces/:sid/restore", g.Contributor(), guard.RequireMember(), idem, sp.Restore)
	read.GET("/spaces/:sid/members", g.Viewer(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), sp.ListMembers)
	write.PUT("/spaces/:sid/members", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, sp.SetMembers)
	write.DELETE("/spaces/:sid/members/:ptype/:pid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, sp.RemoveMember)
	write.PUT("/spaces/:sid/knowledge-base", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, sp.BindKnowledgeBase)
	// ---- page tree and trash (T1.2) ---------------------------------------------
	pg := h.Pages
	read.GET("/spaces/:sid/tree", g.Viewer(), guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), pg.Tree)
	read.GET("/spaces/:sid/trash", g.Viewer(), guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), pg.Trash)
	write.DELETE("/spaces/:sid/trash", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, pg.EmptyTrash)
	write.DELETE("/spaces/:sid/trash/:pid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, pg.Purge)
	// Labels belong to a space, so they are addressed under it -- and so a
	// label id from elsewhere cannot be reached by naming it, the same rule
	// revisions and comments follow. The service checks that too.
	read.GET("/spaces/:sid/labels", g.Viewer(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), pg.Labels)
	write.POST("/spaces/:sid/labels", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleWriter), idem, pg.CreateLabel)
	write.PATCH("/spaces/:sid/labels/:lid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleWriter), idem, pg.UpdateLabel)
	// Deleting takes the label off every page carrying it; the service asks
	// for an admin, and the guard says so too.
	write.DELETE("/spaces/:sid/labels/:lid", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), idem, pg.DeleteLabel)
	read.GET("/spaces/:sid/home", g.Viewer(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), pg.SpaceHome)
	read.GET("/spaces/:sid/pages-by-label", g.Viewer(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), pg.PagesWithLabels)
	// ---- attachments (T1.6) -------------------------------------------------
	// Uploading is a space-level right; reading one is decided by the page it
	// belongs to, which the guard cannot know from the URL, so the service
	// resolves it. No idempotency middleware: an upload is deduplicated by
	// content digest, which is a better key than one the client invents.
	fh := h.Files
	write.POST("/spaces/:sid/attachments", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleWriter), fh.Upload)
	write.POST("/spaces/:sid/imports", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleWriter), idem, ni)
	write.POST("/spaces/:sid/export", g.Contributor(),
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), idem, ni)

	// ---- pages ---------------------------------------------------------------
	// Under their own prefix rather than /pages/*: a literal segment and a
	// path parameter cannot share a position in gin's route tree, and
	// "suggest" would collide with a page id.
	// ---- embeds (T2.3) ------------------------------------------------------
	// Resolving is a read that happens to POST, for the same reason the title
	// lookup does: the address does not belong in a URL. Neither call reaches
	// a page, so membership is the only gate; the allow-list is what actually
	// decides, and it is checked again on save.
	read.GET("/embeds/policy", g.Viewer(), guard.RequireMember(), pg.EmbedPolicy)
	read.POST("/embeds/resolve", g.Viewer(), guard.RequireMember(), pg.ResolveEmbed)

	read.GET("/page-links/suggest", g.Viewer(), guard.RequireMember(), pg.SuggestPages)
	// A POST that only reads: the id list is as long as the open page has
	// links, which does not belong in a URL. Declared with the read
	// capability, which is what actually governs access.
	read.POST("/page-links/titles", g.Viewer(), guard.RequireMember(), pg.ResolveTitles)
	// Block references, resolved the same way and for the same reasons. Access
	// to each source page is decided inside the service, per page: a reference
	// to a page the reader may not open resolves exactly as a deleted one does.
	read.POST("/block-refs/resolve", g.Viewer(), guard.RequireMember(), pg.ResolveTransclusions)

	write.POST("/pages", g.Contributor(), guard.RequireMember(), idem, pg.Create)
	read.GET("/pages/:pid", g.Viewer(), guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Get)
	read.GET("/pages/by-short-id/:short", g.Viewer(),
		guard.RequirePage("short", acl.PageByShortID, model.RoleReader), pg.GetByShortID)
	read.GET("/pages/:pid/content", g.Viewer(), guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Content)
	// The single entry point for writing a body without typing it (T1.7);
	// import, history restore and the AI write-back reach the same service
	// method from inside the server.
	write.PUT("/pages/:pid/content", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.ReplaceContent)
	write.PATCH("/pages/:pid", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.Update)
	write.POST("/pages/:pid/move", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.Move)
	write.POST("/pages/:pid/duplicate", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.Duplicate)
	write.DELETE("/pages/:pid", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.Delete)
	write.POST("/pages/:pid/restore", g.Contributor(), guard.RequireMember(), idem, pg.Restore)
	read.GET("/pages/:pid/ancestors", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Ancestors)
	read.GET("/pages/:pid/children", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Children)
	read.GET("/pages/:pid/attachments", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), fh.ListForPage)
	// ---- links, mentions and backlinks (T2.2) ------------------------------
	// Every one of these is permission-filtered inside the service: a
	// suggestion list must never become a way to enumerate pages or people
	// the caller cannot otherwise see.
	read.GET("/pages/:pid/backlinks", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Backlinks)
	read.GET("/pages/:pid/mention-candidates", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.SuggestMentions)
	// Page-level permissions. Reading the panel needs only read access:
	// somebody who can open a page is entitled to know why they can, which is
	// what makes "why can't my colleague see this" answerable without an
	// administrator. Changing it needs admin on the page -- and since a grant
	// can only narrow, never promote, that means a space administrator.
	read.GET("/pages/:pid/effective-permission", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.EffectivePermission)
	read.GET("/pages/:pid/access", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.PageAccess)
	admin.PUT("/pages/:pid/access", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleAdmin), idem, pg.SetPageAccess)
	read.GET("/pages/:pid/grants", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.PageGrants)
	admin.POST("/pages/:pid/grants", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleAdmin), idem, pg.AddPageGrant)
	admin.DELETE("/pages/:pid/grants/:ptype/:principal", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleAdmin), idem, pg.RemovePageGrant)
	// History. Every version is addressed under its page rather than by its
	// own id, because a version's permissions are the page's permissions --
	// it is that page at an earlier moment. Addressing one on its own would
	// mean a guard that can only check membership and a second, hand-written
	// permission check inside each handler, which is the shape mistakes are
	// made in. The service additionally refuses a revision id that belongs to
	// a different page, so the guard and the row cannot disagree.
	read.GET("/pages/:pid/revisions", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.History)
	read.GET("/pages/:pid/revisions/:rid", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Revision)
	read.GET("/pages/:pid/revisions/:rid/diff", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.RevisionDiff)
	write.POST("/pages/:pid/revisions/:rid/restore", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.RestoreRevision)
	// Comments, addressed under their page for the same reason revisions are:
	// a comment's permissions are the page's, and authorship decides the rest.
	//
	// Every one of these takes the *reader* role, writes included. Commenting
	// is not editing -- somebody invited to review a page has to be able to
	// say what they think of it without being handed the ability to change it,
	// which is this work package's own acceptance criterion.
	read.GET("/pages/:pid/comments", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Comments)
	write.POST("/pages/:pid/comments", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.CreateComment)
	write.PATCH("/pages/:pid/comments/:cid", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.UpdateComment)
	write.DELETE("/pages/:pid/comments/:cid", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.DeleteComment)
	write.POST("/pages/:pid/comments/:cid/resolve", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.ResolveComment)
	// Locking and publication state. Locking needs admin on the page: the
	// resolver caps everybody else at reader on a locked page, so an admin is
	// the only one who could undo it anyway. Marking a draft is a writer's
	// act, because a draft is a label rather than a permission.
	admin.PUT("/pages/:pid/lock", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleAdmin), idem, pg.SetLocked)
	write.PUT("/pages/:pid/status", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.SetPageStatus)

	// Public links. Listing them needs only read access, because "this page
	// is published on the internet" is something every reader of it should be
	// able to see; publishing one needs write access.
	read.GET("/pages/:pid/shares", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.Shares)
	write.POST("/pages/:pid/shares", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.CreateShare)
	write.PATCH("/pages/:pid/shares/:shid", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.UpdateShare)
	write.DELETE("/pages/:pid/shares/:shid", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.RevokeShare)
	write.POST("/pages/:pid/export", g.Viewer(), guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, ni)
	write.PUT("/pages/:pid/labels", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), idem, pg.SetPageLabels)
	// Starring is a bookmark rather than a change, so a reader may do it.
	write.PUT("/pages/:pid/favourite", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.SetFavourite)
	// Watching is a reader's right, like commenting: you can follow a page you
	// are not allowed to change.
	read.GET("/pages/:pid/watch", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), pg.WatchState)
	write.PUT("/pages/:pid/watch", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.SetWatch)
	write.PUT("/pages/:pid/mute", g.Viewer(),
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), idem, pg.SetMute)
	// ---- exclusive editing (T1.5) -------------------------------------------
	// Only a deployment without a collaboration service uses these; the rest
	// answer 409 so a misconfigured client fails loudly instead of editing
	// against a lease nobody honours. Reading the lease needs read access
	// (that is how the read-only view knows who is typing); taking it needs
	// write access, checked again in the service against the page's role.
	//
	// These three writes deliberately carry no Idempotency-Key middleware,
	// unlike every other write in this file. Each already has a natural
	// idempotency key of its own — the session identifier for the lease, the
	// base version for the save — and they repeat on a timer, so a client
	// that reused one Idempotency-Key across heartbeats would be served a
	// cached answer for a day.
	ls := h.Leases
	read.GET("/pages/:pid/lease", g.Viewer(), guard.RequirePage("pid", acl.PageByID, model.RoleReader), ls.Get)
	write.POST("/pages/:pid/lease", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), ls.Acquire)
	write.DELETE("/pages/:pid/lease", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), ls.Release)
	read.GET("/pages/:pid/ydoc", g.Viewer(), guard.RequirePage("pid", acl.PageByID, model.RoleReader), ls.LoadYDoc)
	write.PUT("/pages/:pid/ydoc", g.Contributor(),
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), ls.SaveYDoc)

	// ---- shares and attachments addressed by their own id ----
	// Revisions and comments are deliberately absent: both are addressed under
	// their page above, because the page's permissions are theirs and a guard
	// that can only check membership would push the real check into each
	// handler by hand. T4.4's route audit should find this note rather than a
	// missing endpoint.
	// A share link is addressed under its page above, for the same reason
	// revisions, comments and labels are: its permissions are the page's. The
	// T0.5 placeholder that stood here is gone rather than unimplemented.
	read.GET("/attachments/:aid", g.Viewer(), guard.RequireMember(), fh.Download)
	write.DELETE("/attachments/:aid", g.Contributor(), guard.RequireMember(), idem, fh.Delete)

	// ---- templates, labels, search, notifications, imports/exports -------------
	// Templates are addressed by their own id, unlike most of this module,
	// because a tenant-wide one has no space to sit under. The guard can only
	// establish membership; each template's real scope is checked in the
	// service, which knows whether it belongs to a space or to everybody.
	read.GET("/templates", g.Viewer(), guard.RequireMember(), pg.Templates)
	write.POST("/templates", g.Contributor(), guard.RequireMember(), idem, pg.CreateTemplate)
	read.GET("/templates/:tid", g.Viewer(), guard.RequireMember(), pg.Template)
	write.PATCH("/templates/:tid", g.Contributor(), guard.RequireMember(), idem, pg.UpdateTemplate)
	write.DELETE("/templates/:tid", g.Contributor(), guard.RequireMember(), idem, pg.DeleteTemplate)
	// A label is addressed under its space above, not by its own id: a
	// label's permissions *are* the space's, and giving it its own route
	// would mean every handler re-deriving them. The T0.5 placeholders that
	// stood here are gone for that reason — see the same note on revisions
	// and comments.
	read.GET("/search", g.Viewer(), guard.RequireMember(), ni)
	// An inbox belongs to its reader rather than to any page, so these are
	// guarded by membership alone: a notification says what happened, and
	// somebody who has since lost access to a page can still read and dismiss
	// the row telling them they were mentioned on it.
	read.GET("/notifications", g.Viewer(), guard.RequireMember(), pg.Notifications)
	write.POST("/notifications/read", g.Viewer(), guard.RequireMember(), idem,
		pg.MarkNotificationsRead)
	write.POST("/notifications/archive", g.Viewer(), guard.RequireMember(), idem,
		pg.ArchiveNotifications)
	read.GET("/imports/:jid", g.Viewer(), guard.RequireMember(), ni)
	read.GET("/exports/:jid", g.Viewer(), guard.RequireMember(), ni)
	// Somebody's own starred pages, across every space they can see. Filtered
	// by what they may still open: a page starred and since restricted is
	// left out rather than shown as a row that cannot be opened.
	read.GET("/favourites", g.Viewer(), guard.RequireMember(), pg.Favourites)
	// There is no /recent. "Recently edited" is per space and lives on the
	// space home above; "recently viewed" is deliberately not a server
	// concern at all (see the note in internal/docs/service/home.go). The
	// T0.5 placeholder that stood here is gone rather than unimplemented: an
	// endpoint that will always answer 501 is a worse answer than no endpoint.

	// ---- tenant groups (tenant-level administration; docs is the first consumer) ----
	gr := h.Groups
	groups := g.apiKeyGroup(r.Group("/groups"), apiKeyDocsAdmin(apiKeyFullAccess()))
	groups.GET("", g.Viewer(), guard.RequireMember(), gr.List)
	groups.POST("", g.Admin(), guard.RequireMember(), idem, gr.Create)
	groups.GET("/:gid", g.Viewer(), guard.RequireMember(), gr.Get)
	groups.PATCH("/:gid", g.Admin(), guard.RequireMember(), idem, gr.Update)
	groups.DELETE("/:gid", g.Admin(), guard.RequireMember(), idem, gr.Delete)
	groups.GET("/:gid/members", g.Viewer(), guard.RequireMember(), gr.ListMembers)
	groups.PUT("/:gid/members", g.Admin(), guard.RequireMember(), idem, gr.AddMembers)
	groups.DELETE("/:gid/members/:uid", g.Admin(), guard.RequireMember(), idem, gr.RemoveMember)
}

// RegisterDocsInternalRoutes mounts the collaboration callbacks.
//
// They are registered on the engine BEFORE the session middleware, like the
// IM callbacks: the collaboration service has no user session and proves
// itself with an HMAC signature over every request. Nothing is registered
// when the module is off or no shared secret is configured.
func RegisterDocsInternalRoutes(r *gin.Engine, m *docs.Module) {
	if m == nil || !m.Enabled || m.Handler == nil || !m.Handler.Collab.Enabled() {
		return
	}
	h := m.Handler.Collab
	internal := r.Group("/internal/collab")
	internal.POST("/authenticate", h.Authenticate)
	internal.GET("/load/:pid", h.Load)
	internal.POST("/store", h.Store)
	internal.GET("/health", h.Health)
}

// dochandlerNotImplemented adapts the docs handler package's placeholder.
func dochandlerNotImplemented(c *gin.Context) { dochandler.NotImplemented(c) }

// RegisterDocsPublicRoutes mounts the anonymous share-link endpoints.
//
// Registered on the engine BEFORE the authentication middleware, like the IM
// callbacks and the presigned-file routes, because a visitor following a
// shared URL has no session and must not be asked for one. The link key is
// the whole credential; everything the key does not cover is refused inside
// the service rather than by a guard here, because there is no principal for
// a guard to reason about.
//
// Nothing is registered when the module is off or when the deployment has not
// switched public sharing on, so an installation that never wanted anything
// on the public internet does not even expose the routes.
func RegisterDocsPublicRoutes(r *gin.Engine, m *docs.Module) {
	if m == nil || !m.Enabled || m.Handler == nil || !m.Config.PublicSharing {
		return
	}
	pg := m.Handler.Pages
	public := r.Group("/api/v1/docs/public")
	public.GET("/:key", pg.PublicPage)
	public.POST("/:key/unlock", pg.UnlockPublicPage)

	// Public spaces live under their own prefix rather than under /public,
	// because gin cannot have a literal segment and a ":key" parameter at the
	// same position in its route tree -- the same constraint that put page
	// links under /docs/page-links in T2.2.
	spaces := r.Group("/api/v1/docs/public-spaces")
	spaces.GET("/:sid", pg.PublicSpace)
	spaces.GET("/:sid/pages/:short", pg.PublicSpacePage)
}
