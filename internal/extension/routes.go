package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/types"
)

// RouteRegistrarGroup is the dig value group that route registrars are provided
// into. The router collects the group and calls each registrar after every core
// route exists, so it never needs to know what the extensions are.
//
//	c.Provide(newAcmeRoutes, dig.Group(extension.RouteRegistrarGroup))
//
// where newAcmeRoutes returns an extension.RouteRegistrar.
const RouteRegistrarGroup = "route_registrars"

// RouteRegistrar adds HTTP routes under /api/v1.
//
// The routes it adds sit behind the same authentication as the core's and are
// checked by the same API-key gate. That gate denies every route that is not
// declared, so a route registered through Routes.Group is closed to API keys
// and a route that should be open to them must say so through
// Routes.APIKeys. An extension cannot register a route outside /api/v1, before
// authentication, or in a way that skips the gate.
type RouteRegistrar interface {
	// Register adds the extension's routes. A returned error stops the server
	// from starting. Registering a path a core route already uses panics inside
	// gin, which stops it as well: an extension cannot replace a core route.
	Register(r Routes) error
}

// Routes is what a RouteRegistrar may use to register routes. It is a small
// window onto the router: enough to attach handlers under the core's
// authentication and authorization rules, and nothing that would let an
// extension bypass them.
type Routes interface {
	// Group is the authenticated /api/v1 group. A JWT session reaches routes
	// registered on it; an X-API-Key caller is refused (403) because the route
	// declares no API-key policy. Use it for anything a machine principal should
	// not call.
	Group() *gin.RouterGroup

	// APIKeys registers routes on the same group and declares, for each, the
	// policy under which an API key may call it. Paths are relative to /api/v1.
	// A zero APIKeyPolicy declares nothing, which leaves the route closed to
	// API keys exactly as Group would.
	APIKeys(policy APIKeyPolicy) APIKeyRoutes

	// The workspace-role guards the core's routes use. Each admits the role and
	// above; with role checks disabled in the configuration they pass every
	// authenticated caller through, as they do for core routes. Put them before
	// the handler: g.GET("/x", r.Viewer(), h).
	Viewer() gin.HandlerFunc
	Contributor() gin.HandlerFunc
	Admin() gin.HandlerFunc
	Owner() gin.HandlerFunc
}

// APIKeyRoutes registers routes that carry an API-key policy. It mirrors the
// registration methods of a gin group; rel is relative to /api/v1.
type APIKeyRoutes interface {
	GET(rel string, handlers ...gin.HandlerFunc) gin.IRoutes
	POST(rel string, handlers ...gin.HandlerFunc) gin.IRoutes
	PUT(rel string, handlers ...gin.HandlerFunc) gin.IRoutes
	PATCH(rel string, handlers ...gin.HandlerFunc) gin.IRoutes
	DELETE(rel string, handlers ...gin.HandlerFunc) gin.IRoutes
}

// APIKeyPolicy says which API keys may call a route. Build one with
// APIKeyAnyKey, APIKeyFullAccess or APIKeyCapabilities; the zero value admits
// no key. Role guards and knowledge-base allow-lists still apply on top.
type APIKeyPolicy struct {
	kind         apiKeyPolicyKind
	capabilities []types.APIKeyCapability
}

type apiKeyPolicyKind int

const (
	apiKeyForbidden apiKeyPolicyKind = iota
	apiKeyAny
	apiKeyFull
	apiKeyCaps
)

// APIKeyAnyKey admits any valid API key. Use it only for read-only, harmless
// routes such as the caller's own identity.
func APIKeyAnyKey() APIKeyPolicy { return APIKeyPolicy{kind: apiKeyAny} }

// APIKeyFullAccess admits only full-access API keys.
func APIKeyFullAccess() APIKeyPolicy { return APIKeyPolicy{kind: apiKeyFull} }

// APIKeyCapabilities admits full-access keys and scoped keys that carry at
// least one of the capabilities. With no capabilities it is APIKeyFullAccess.
func APIKeyCapabilities(capabilities ...types.APIKeyCapability) APIKeyPolicy {
	return APIKeyPolicy{kind: apiKeyCaps, capabilities: append([]types.APIKeyCapability(nil), capabilities...)}
}

// Declared reports whether the policy admits any API key at all.
func (p APIKeyPolicy) Declared() bool { return p.kind != apiKeyForbidden }

// Any reports whether the policy is APIKeyAnyKey.
func (p APIKeyPolicy) Any() bool { return p.kind == apiKeyAny }

// Capabilities returns the capabilities the policy lists.
func (p APIKeyPolicy) Capabilities() []types.APIKeyCapability {
	return append([]types.APIKeyCapability(nil), p.capabilities...)
}

// FeatureDisabledCode is the value of error.code in the body RequireFeature
// answers with. It is part of the API: clients may match on it.
const FeatureDisabledCode = "feature_disabled"

// RequireFeature returns a middleware that lets a request through only while
// the feature is enabled and otherwise answers 403 without calling the handler:
//
//	{"success": false,
//	 "error": {"code": "feature_disabled",
//	           "message": "...",
//	           "details": {"feature": "<name>", "reason": "<status reason>"}}}
//
// details.reason is FeatureStatus.Reason, empty when the feature is simply not
// installed. The status is read on every request, so a feature switched on or
// off at runtime takes effect at once. Put it after the authentication and role
// guards so an anonymous caller learns nothing about the deployment.
func RequireFeature(features Features, f Feature) gin.HandlerFunc {
	return func(c *gin.Context) {
		var st FeatureStatus
		if features != nil {
			st = features.Status(f)
		}
		if st.Enabled {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"error": gin.H{
				"code":    FeatureDisabledCode,
				"message": "This feature is not available in this deployment",
				"details": gin.H{"feature": string(f), "reason": st.Reason},
			},
		})
	}
}
