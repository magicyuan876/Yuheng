package router

import (
	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/handler"
)

// Workspace group routes.
//
// Groups are a workspace concept: the other half of membership, where a
// grant to a group reaches whoever is in it. They used to be registered by
// the docs module, their first consumer, and were therefore gone from a
// deployment that had the module switched off -- although nothing about a
// group is about documents. They now register unconditionally, next to the
// member routes they belong with, and under the same API-key capability:
// manage_members, because adding somebody to a group is a membership change
// in everything but the table it writes to.
//
// The role floor is the one the docs module applied: any member may list
// groups and their members, because the pickers that grant access to a
// group need the list; changing groups is workspace administration (Admin
// and above). Owner is not required, unlike /tenants/:id/members, because
// a group confers nothing by itself -- what a group may do is granted
// elsewhere, by whoever administers the thing being granted.
//
// The paths are unchanged (/api/v1/groups/**), so the clients that used them
// under the docs module keep working.

// RegisterGroupRoutes mounts /api/v1/groups/**.
func RegisterGroupRoutes(r *gin.RouterGroup, h *handler.TenantGroupHandler, g *rbacGuards) {
	groups := g.apiKeyGroup(r.Group("/groups"), apiKeyManageMembers(apiKeyFullAccess()))
	groups.GET("", g.Viewer(), h.List)
	groups.POST("", g.Admin(), h.Create)
	groups.GET("/:gid", g.Viewer(), h.Get)
	groups.PATCH("/:gid", g.Admin(), h.Update)
	groups.DELETE("/:gid", g.Admin(), h.Delete)
	groups.GET("/:gid/members", g.Viewer(), h.ListMembers)
	groups.PUT("/:gid/members", g.Admin(), h.AddMembers)
	groups.DELETE("/:gid/members/:uid", g.Admin(), h.RemoveMember)
}
