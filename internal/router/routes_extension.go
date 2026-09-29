package router

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/middleware"
)

// extensionRoutes is the window RegisterExtensionRoutes hands to a registrar.
// It exposes the core's own declaration helpers and nothing else, so an
// extension's routes are subject to the API-key gate (default deny) and the
// startup self-check exactly like core routes.
type extensionRoutes struct {
	v1 *gin.RouterGroup
	g  *rbacGuards
}

func (e extensionRoutes) Group() *gin.RouterGroup { return e.v1 }

func (e extensionRoutes) Viewer() gin.HandlerFunc      { return e.g.Viewer() }
func (e extensionRoutes) Contributor() gin.HandlerFunc { return e.g.Contributor() }
func (e extensionRoutes) Admin() gin.HandlerFunc       { return e.g.Admin() }
func (e extensionRoutes) Owner() gin.HandlerFunc       { return e.g.Owner() }

func (e extensionRoutes) APIKeys(policy extension.APIKeyPolicy) extension.APIKeyRoutes {
	if !policy.Declared() {
		// Nothing to declare: the routes stay closed to API keys, as on Group.
		return rawRoutes{e.v1}
	}
	return e.g.apiKeyGroup(e.v1, toGatePolicy(policy))
}

// toGatePolicy maps the public policy onto the gate's. Capabilities never make
// a route reachable by a key that lacks them; a full-access key always may.
func toGatePolicy(p extension.APIKeyPolicy) middleware.APIKeyRoutePolicy {
	if p.Any() {
		return apiKeyAny()
	}
	policy := apiKeyFullAccess()
	for _, c := range p.Capabilities() {
		policy = policy.WithCapability(c)
	}
	return policy
}

// rawRoutes adapts a gin group to extension.APIKeyRoutes without declaring
// anything.
type rawRoutes struct{ grp *gin.RouterGroup }

func (r rawRoutes) GET(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return r.grp.Handle(http.MethodGet, rel, h...)
}

func (r rawRoutes) POST(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return r.grp.Handle(http.MethodPost, rel, h...)
}

func (r rawRoutes) PUT(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return r.grp.Handle(http.MethodPut, rel, h...)
}

func (r rawRoutes) PATCH(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return r.grp.Handle(http.MethodPatch, rel, h...)
}

func (r rawRoutes) DELETE(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return r.grp.Handle(http.MethodDelete, rel, h...)
}

// RegisterExtensionRoutes lets each registrar add its routes to the
// authenticated /api/v1 group. It must run after the core routes, so that a
// registrar that reuses a core path fails loudly, and before the API-key
// self-check, so that the extension's declarations are verified too.
func RegisterExtensionRoutes(v1 *gin.RouterGroup, registrars []extension.RouteRegistrar, g *rbacGuards) error {
	for i, reg := range registrars {
		if reg == nil {
			continue
		}
		if err := reg.Register(extensionRoutes{v1: v1, g: g}); err != nil {
			return fmt.Errorf("route registrar %d (%T): %w", i, reg, err)
		}
	}
	return nil
}
