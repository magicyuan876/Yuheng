package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/handler"
)

// RegisterUserFavoriteRoutes wires the per-user starred-resource endpoints.
//
// Authorization: the handler always derives (user_id, tenant_id) from the
// auth context — there is no admin-style "see another user's favorites"
// path — so a Viewer floor is the right gate. The endpoints intentionally
// don't follow the OwnedXOrAdmin pattern: favorites aren't owned by the
// resource's creator, they're owned by the user *doing* the starring.
func RegisterUserFavoriteRoutes(r *gin.RouterGroup, h *handler.UserResourceFavoriteHandler, g *rbacGuards) {
	// Favorites are per-user; not declared for API keys (default-deny).
	favs := r.Group("/user/favorites")
	{
		favs.GET("", g.Viewer(), h.ListFavorites)
		favs.POST("", g.Viewer(), h.AddFavorite)
		favs.DELETE("/:type/:id", g.Viewer(), h.RemoveFavorite)
	}
}

// RegisterOrganizationRoutes registers organization and sharing routes
func RegisterOrganizationRoutes(r *gin.RouterGroup, orgHandler *handler.OrganizationHandler, g *rbacGuards) {
	// Organization routes
	orgs := g.apiKeyGroup(r.Group("/organizations"), apiKeyManageSpaces(apiKeyFullAccess()))
	{
		// Create organization (Admin+ in caller's tenant only)
		orgs.POST("", g.Admin(), orgHandler.CreateOrganization)
		// List my organizations — Viewer+ floor so revoked/non-member
		// accounts whose JWT still validates can't enumerate org membership.
		orgs.GET("", g.Viewer(), orgHandler.ListMyOrganizations)
		// Preview organization by invite code (without joining) — Viewer+
		orgs.GET("/preview/:code", g.Viewer(), orgHandler.PreviewByInviteCode)
		// Join organization by invite code (Admin+ in caller's tenant only)
		orgs.POST("/join", g.Admin(), orgHandler.JoinByInviteCode)
		// Submit join request (for organizations that require approval) (Admin+)
		orgs.POST("/join-request", g.Admin(), orgHandler.SubmitJoinRequest)
		// Search searchable (discoverable) organizations — Viewer+
		orgs.GET("/search", g.Viewer(), orgHandler.SearchOrganizations)
		// Join searchable organization by ID (no invite code) (Admin+)
		orgs.POST("/join-by-id", g.Admin(), orgHandler.JoinByOrganizationID)
		// Get organization by ID — Viewer+
		orgs.GET("/:id", g.Viewer(), orgHandler.GetOrganization)
		// Update organization — Admin+ in caller's tenant.
		// Service still gates on "caller's tenant is the org owner";
		// the route guard adds a defence-in-depth layer that stops a
		// tenant Viewer/Contributor from ever reaching the service.
		orgs.PUT("/:id", g.Admin(), orgHandler.UpdateOrganization)
		// Delete organization — Admin+ in caller's tenant. Same
		// rationale as PUT above; deletion is irreversible so the
		// route-layer floor is at least as strict.
		orgs.DELETE("/:id", g.Admin(), orgHandler.DeleteOrganization)
		// Leave organization (Admin+ in caller's tenant only)
		orgs.POST("/:id/leave", g.Admin(), orgHandler.LeaveOrganization)
		// Request role upgrade (Admin+ in caller's tenant only).
		// An upgrade approval changes the whole tenant's org role, so it
		// must not be initiated by a tenant Viewer/Contributor.
		orgs.POST("/:id/request-upgrade", g.Admin(), orgHandler.RequestRoleUpgrade)
		// Generate invite code — Admin+ in caller's tenant. Issuing an
		// invite code is an admin action; the service layer additionally
		// requires the caller's tenant to be admin in the org.
		orgs.POST("/:id/invite-code", g.Admin(), orgHandler.GenerateInviteCode)
		// Search tenants for invite (admin only). Plan 3 changed the unit
		// of membership to "tenant"; this endpoint returns candidate
		// tenants (with one representative user attached) instead of one
		// row per user.
		orgs.GET("/:id/search-tenants", g.Admin(), orgHandler.SearchTenantsForInvite)
		// Invite member directly (admin only)
		orgs.POST("/:id/invite", g.Admin(), orgHandler.InviteMember)
		// List members — Viewer+
		orgs.GET("/:id/members", g.Viewer(), orgHandler.ListMembers)
		// Update member role (path parameter is the member tenant_id) —
		// Admin+ in caller's tenant. Changing another tenant's org role
		// is the symmetric counterpart to removing them; both are gated
		// the same way.
		orgs.PUT("/:id/members/:tenant_id", g.Admin(), orgHandler.UpdateMemberRole)
		// Remove member (path parameter is the member tenant_id).
		// Both self-removal (caller's own tenant) and admin-removal-of-other
		// take a whole tenant out of the org, so the route must be Admin+
		// in the caller's tenant — symmetric with POST /:id/leave above.
		orgs.DELETE("/:id/members/:tenant_id", g.Admin(), orgHandler.RemoveMember)
		// List join requests (admin only) — caller's tenant must be at
		// least Admin to even see the queue (a tenant Viewer has no
		// authority to act on it).
		orgs.GET("/:id/join-requests", g.Admin(), orgHandler.ListJoinRequests)
		// Review join request (admin only)
		orgs.PUT("/:id/join-requests/:request_id/review", g.Admin(), orgHandler.ReviewJoinRequest)
		// List knowledge bases shared to this organization — Viewer+
		orgs.GET("/:id/shares", g.Viewer(), orgHandler.ListOrgShares)
		// List all knowledge bases in this organization (including mine) for list-page space view — Viewer+
		orgs.GET("/:id/shared-knowledge-bases", g.Viewer(), orgHandler.ListOrganizationSharedKnowledgeBases)
	}

	// Knowledge base sharing routes (add to existing kb routes).
	// 分享 KB 到组织 = 让组织里所有人能读这个 KB；这跟"修改 KB 元信息"
	// 同等敏感，所以挂同款 OwnedKBOrAdmin 矩阵。Viewer 在自己空间里
	// 也不能私自把 KB 暴露出去。
	// 分享管理不通过 capability 授予（manage_spaces 也不含）；仅 full-access
	// key（空间级全权）可管理分享，scoped key 保持 default-deny。
	kbShares := g.apiKeyGroup(r.Group("/knowledge-bases/:id/shares"), apiKeyFullAccess())
	{
		// Share knowledge base
		kbShares.POST("", g.OwnedKBOrAdmin(), orgHandler.ShareKnowledgeBase)
		// List shares — Viewer+ 即可，纯读取
		kbShares.GET("", g.Viewer(), orgHandler.ListKBShares)
		// Update share permission
		kbShares.PUT("/:share_id", g.OwnedKBOrAdmin(), orgHandler.UpdateSharePermission)
		// Remove share
		kbShares.DELETE("/:share_id", g.OwnedKBOrAdmin(), orgHandler.RemoveShare)
	}

	// Shared knowledge bases route — Viewer+
	g.apiKeyRoute(r, http.MethodGet, "/shared-knowledge-bases", apiKeyManageSpaces(apiKeyFullAccess()), g.Viewer(), orgHandler.ListSharedKnowledgeBases)
}
