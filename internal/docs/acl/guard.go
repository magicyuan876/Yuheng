package acl

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Gin context keys under which the guard leaves what it resolved, so the
// handler does not resolve twice.
const (
	IdentityContextKey = "docs.acl.identity"
	SpaceContextKey    = "docs.acl.space"
	SpaceRoleKey       = "docs.acl.space_role"
	DecisionContextKey = "docs.acl.decision"
)

// Guard is the gin-facing side of the resolver. Routes declare the minimum
// role they need on a space (by id or slug) or on a page (by id or short id);
// the guard resolves, denies, and stashes the result for the handler.
//
// Denials: a caller who cannot see the resource at all gets 404, so the
// existence of spaces and pages is not leaked by probing; a caller who can
// see it but lacks the role gets 403.
type Guard struct {
	res *Resolver
}

// NewGuard builds a guard over a resolver.
func NewGuard(res *Resolver) *Guard { return &Guard{res: res} }

// SpaceLookup says how a route names its space.
type SpaceLookup int

// Space lookup modes.
const (
	SpaceByID SpaceLookup = iota
	SpaceBySlug
)

// PageLookup says how a route names its page.
type PageLookup int

// Page lookup modes.
const (
	PageByID PageLookup = iota
	PageByShortID
)

// RequireSpace requires at least min on the space named by the URL parameter.
func (g *Guard) RequireSpace(param string, by SpaceLookup, min model.SpaceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := g.identity(c)
		if !ok {
			return
		}
		var (
			space *model.Space
			err   error
		)
		switch by {
		case SpaceBySlug:
			space, err = g.res.repos.Spaces.GetBySlug(c.Request.Context(), id.TenantID, c.Param(param))
		default:
			space, err = g.res.repos.Spaces.Get(c.Request.Context(), id.TenantID, c.Param(param))
		}
		if err != nil {
			g.fail(c, err)
			return
		}
		role, err := g.res.SpaceRole(c.Request.Context(), id, space)
		if err != nil {
			g.fail(c, err)
			return
		}
		if role == model.RoleNone {
			notFound(c)
			return
		}
		if !role.AtLeast(min) {
			forbidden(c, id, string(role), string(min), c.Request.URL.Path)
			return
		}
		c.Set(SpaceContextKey, space)
		c.Set(SpaceRoleKey, role)
		c.Next()
	}
}

// RequirePage requires at least min on the page named by the URL parameter.
func (g *Guard) RequirePage(param string, by PageLookup, min model.SpaceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := g.identity(c)
		if !ok {
			return
		}
		pageID := c.Param(param)
		if by == PageByShortID {
			p, err := g.res.repos.Pages.GetByShortID(c.Request.Context(), id.TenantID, pageID)
			if err != nil {
				g.fail(c, err)
				return
			}
			pageID = p.ID
		}
		d, err := g.res.Page(c.Request.Context(), id, pageID)
		if err != nil {
			g.fail(c, err)
			return
		}
		if d.Role == model.RoleNone {
			notFound(c)
			return
		}
		if !d.Role.AtLeast(min) {
			forbidden(c, id, string(d.Role), string(min), c.Request.URL.Path)
			return
		}
		c.Set(DecisionContextKey, d)
		c.Next()
	}
}

// RequireMember only requires an active tenant membership; used by routes
// that list what the caller can see (spaces, notifications).
func (g *Guard) RequireMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := g.identity(c); ok {
			c.Next()
		}
	}
}

// identity resolves the caller and stores it; on failure it has already
// written the response and returns ok=false.
func (g *Guard) identity(c *gin.Context) (*Identity, bool) {
	if v, exists := c.Get(IdentityContextKey); exists {
		if id, ok := v.(*Identity); ok {
			return id, true
		}
	}
	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no active workspace"})
		c.Abort()
		return nil, false
	}
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		// Machine principals (API keys) are authorised by the API-key gate and
		// carry no user; docs routes that admit them resolve as a synthetic
		// identity in the handler layer (T0.5), not here.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: docs routes require a user session"})
		c.Abort()
		return nil, false
	}
	id, err := g.res.Identity(ctx, tenantID, userID)
	if err != nil {
		g.fail(c, err)
		return nil, false
	}
	if !id.Member {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: not an active member of this workspace"})
		c.Abort()
		return nil, false
	}
	c.Set(IdentityContextKey, id)
	return id, true
}

func (g *Guard) fail(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		notFound(c)
		return
	}
	logger.Errorf(c.Request.Context(), "[docs.acl] resolution failed: %v", err)
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "permission check unavailable"})
	c.Abort()
}

func notFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	c.Abort()
}

func forbidden(c *gin.Context, id *Identity, have, need, path string) {
	logger.Warnf(c.Request.Context(), "[docs.acl] role insufficient: user=%s have=%s need=%s path=%s",
		id.UserID, have, need, path)
	c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: insufficient permission on this document"})
	c.Abort()
}

// IdentityFromGin returns the identity a guard stored, if any.
func IdentityFromGin(c *gin.Context) (*Identity, bool) {
	v, ok := c.Get(IdentityContextKey)
	if !ok {
		return nil, false
	}
	id, ok := v.(*Identity)
	return id, ok
}

// DecisionFromGin returns the page decision a guard stored, if any.
func DecisionFromGin(c *gin.Context) (Decision, bool) {
	v, ok := c.Get(DecisionContextKey)
	if !ok {
		return Decision{}, false
	}
	d, ok := v.(Decision)
	return d, ok
}

// SpaceFromGin returns the space and role a guard stored, if any.
func SpaceFromGin(c *gin.Context) (*model.Space, model.SpaceRole, bool) {
	v, ok := c.Get(SpaceContextKey)
	if !ok {
		return nil, model.RoleNone, false
	}
	space, ok := v.(*model.Space)
	if !ok {
		return nil, model.RoleNone, false
	}
	role, _ := c.Get(SpaceRoleKey)
	r, _ := role.(model.SpaceRole)
	return space, r, true
}
