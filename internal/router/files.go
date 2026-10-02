package router

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// files.go hosts every file-proxy surface the router exposes:
//
//   - /files                              tenant-scoped resource proxy
//   - /api/v1/knowledge-bases/:id/files   KB-scoped proxy (shared-KB images)
//   - /api/v1/sessions/:id/messages/:message_id/files
//                                          message-scoped proxy (assistant-message resources)
//   - /api/v1/files/presigned-preview     Admin-only URL diagnostics
//   - /r/:token                           short-lived capability URLs
//
// Every one of them takes a resource:// handle and nothing else. The handle's
// row is authoritative twice over: its tenant decides who may read it, and its
// backend decides where the bytes are read from. The handlers differ only in
// how they establish that the caller may see the resource; reading is always
// FileStore.Open.

// getRouteRegistrar is the minimal registration surface serveFiles* needs;
// both *gin.Engine and *gin.RouterGroup satisfy it, which keeps the file
// routes testable without building a full engine.
type getRouteRegistrar interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
}

// messageFileLookup is the narrow message-service surface needed by the
// message-scoped file proxy. Keeping it small makes the authorization boundary
// independently testable.
type messageFileLookup interface {
	GetMessage(ctx context.Context, sessionID, messageID string) (*types.Message, error)
}

// requireResourceQuery reads the file_path query parameter every proxy route
// accepts and resolves it to its resource row. On failure the response has
// been written and ok=false: 400 for anything that is not a resource handle,
// 404 for a handle that names no live resource.
func requireResourceQuery(c *gin.Context, catalog interfaces.ResourceCatalog) (string, *types.StoredResource, bool) {
	ref := strings.TrimSpace(c.Query("file_path"))
	if ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameter: file_path"})
		return "", nil, false
	}
	if _, ok := types.ParseResourcePath(ref); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_path must be a resource:// reference"})
		return "", nil, false
	}
	resource, err := catalog.Resolve(c.Request.Context(), ref)
	if err != nil || resource == nil {
		c.Status(http.StatusNotFound)
		return "", nil, false
	}
	return ref, resource, true
}

// resourceContentType picks the response type for a resource: the type its
// name implies, narrowed to a safe set, with the recorded MIME type preferred
// for the inline-safe kinds.
func resourceContentType(resource *types.StoredResource) (string, bool) {
	name := resource.OriginalName
	if name == "" {
		name = resource.PhysicalPath
	}
	contentType, inline := secutils.SafeContentTypeByFilename(name)
	if resource.MimeType != "" && inline {
		contentType = resource.MimeType
	}
	return contentType, inline
}

// streamResource reads ref through the file store and writes the shared
// success response of every file proxy: safe content type, nosniff,
// disposition for non-inline types, the route's cache policy, then the body
// (skipped for HEAD). The object is opened even for HEAD so a link to bytes
// that no longer exist reports 404 rather than a misleading 200.
func streamResource(c *gin.Context, files interfaces.FileStore, ref, cacheControl, logTag string) {
	ctx := c.Request.Context()
	reader, resource, err := files.Open(ctx, ref)
	if err != nil {
		logger.Warnf(ctx, "[Router] %s read %s failed: %v", logTag, ref, err)
		c.Status(http.StatusNotFound)
		return
	}
	defer reader.Close()
	contentType, inline := resourceContentType(resource)
	c.Header("Content-Type", contentType)
	c.Header("X-Content-Type-Options", "nosniff")
	if !inline {
		c.Header("Content-Disposition", "attachment")
	}
	c.Header("Cache-Control", cacheControl)
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Warnf(ctx, "[Router] %s write response failed: %v", logTag, err)
	}
}

// newFileServeHandler builds the /files handler. It reads the tenant from the
// request context, set by the auth middleware that precedes it; the resource
// must belong to that tenant.
func newFileServeHandler(files interfaces.FileStore, catalog interfaces.ResourceCatalog) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, _ := c.Request.Context().Value(types.TenantInfoContextKey).(*types.Tenant)
		if tenant == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
			return
		}
		ref, resource, ok := requireResourceQuery(c, catalog)
		if !ok {
			return
		}
		if resource.TenantID != tenant.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: resource not accessible"})
			return
		}
		streamResource(c, files, ref, "public, max-age=86400", "/files")
	}
}

// serveFiles registers the tenant-scoped resource proxy. It is registered
// after auth middleware, so tenant context comes from authentication.
//
// Route:
//   - GET /files?file_path=resource://<handle>
func serveFiles(r getRouteRegistrar, files interfaces.FileStore, catalog interfaces.ResourceCatalog) {
	logger.Infof(context.Background(), "[Router] Serving files from /files")
	// /files sits outside the /api/v1 APIKeyGate, so it carries its own
	// API-key guard. A KB-restricted key is denied (an arbitrary resource
	// cannot be bounded to its allow-list); full-access keys and tenant-wide
	// retrieve keys pass, since the handler still enforces same-tenant
	// resources.
	r.GET("/files", middleware.AllowFileServeAPIKey(), newFileServeHandler(files, catalog))
}

// serveResourceGrants exposes short, revocable capability URLs for clients
// that cannot attach a Yuheng bearer token (for example a third-party app
// rendering an image from an API response with resource_urls=public).
func serveResourceGrants(r *gin.Engine, files interfaces.FileStore, catalog interfaces.ResourceCatalog) {
	handler := func(c *gin.Context) {
		resource, err := catalog.ResolveAccessGrant(c.Request.Context(), c.Param("token"))
		if err != nil || resource == nil {
			c.Status(http.StatusNotFound)
			return
		}
		streamResource(c, files, types.BuildResourcePath(resource.Handle), "private, max-age=300",
			"resource grant (resource_id="+resource.ID+")")
	}
	r.GET("/r/:token", handler)
	r.HEAD("/r/:token", handler)
}

// serveKBScopedFiles registers the KB-scoped file proxy used to render images
// embedded in a knowledge base's content (chunks / wiki pages). It is gated by
// RequireKBAccess, so the base must belong to the caller's workspace, and it
// serves only that workspace's exported content — never raw uploads.
//
// Route:
//   - GET /api/v1/knowledge-bases/:id/files?file_path=resource://<handle>
func serveKBScopedFiles(
	r *gin.RouterGroup,
	g *rbacGuards,
	files interfaces.FileStore,
	catalog interfaces.ResourceCatalog,
) {
	logger.Infof(context.Background(), "[Router] Serving KB-scoped files from /knowledge-bases/:id/files")
	// API-key access mirrors /files: KB-restricted keys are denied (an
	// arbitrary resource of the workspace cannot be bounded to a key's
	// allow-list), while full-access and tenant-wide retrieve keys pass —
	// KBAccess still confines them to the workspace's own KBs, exactly as it
	// does for a JWT Viewer. The route is declared to the gate with the
	// retrieve policy so it is reachable at all; AllowFileServeAPIKey then
	// applies the stricter not-KB-restricted constraint.
	g.apiKeyRoute(r, http.MethodGet, "/knowledge-bases/:id/files",
		apiKeyRetrieve(apiKeyFullAccess()),
		middleware.AllowFileServeAPIKey(),
		g.Viewer(),
		g.KBAccess("id"),
		newKBScopedFileServeHandler(files, catalog),
	)
}

// newKBScopedFileServeHandler builds the handler backing serveKBScopedFiles.
// The workspace is taken from the request context.
//
// Two checks bound what a reader can get here. The resource must belong to the
// workspace, and its object must live in the workspace's exports/ namespace,
// where embedded chunk and wiki images are written: this route renders
// content, it never hands out raw uploads ({tenant}/{knowledgeID}/...), which
// go through /knowledge/{id}/download and its own, stricter, permission check.
func newKBScopedFileServeHandler(files interfaces.FileStore, catalog interfaces.ResourceCatalog) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID, ok := types.TenantIDFromContext(ctx)
		if !ok || tenantID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
			return
		}
		ref, resource, ok := requireResourceQuery(c, catalog)
		if !ok {
			return
		}
		if resource.TenantID != tenantID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: resource not accessible"})
			return
		}
		if !secutils.IsKBExportsPath(resource.PhysicalPath, tenantID) {
			logger.Warnf(ctx, "[Router] /knowledge-bases/:id/files denied resource outside the exports namespace: "+
				"tenant_id=%d resource=%s", tenantID, ref)
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: file path not accessible"})
			return
		}
		// Workspace content — keep it private so shared proxies / CDNs do not
		// cache one workspace's view for another.
		streamResource(c, files, ref, "private, max-age=86400", "/knowledge-bases/:id/files")
	}
}

// newMessageScopedFileServeHandler serves resources rendered inside one
// assistant message. The message service first proves that the caller owns the
// containing session; the resource row then says where the bytes are, so a
// citation renders from wherever it was written without accepting a
// client-provided source workspace ID.
func newMessageScopedFileServeHandler(
	messageService messageFileLookup,
	files interfaces.FileStore,
	catalog interfaces.ResourceCatalog,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		callerTenantID, ok := types.TenantIDFromContext(ctx)
		if !ok || callerTenantID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
			return
		}
		ref, _, ok := requireResourceQuery(c, catalog)
		if !ok {
			return
		}
		if messageService == nil {
			c.Status(http.StatusNotFound)
			return
		}
		message, err := messageService.GetMessage(ctx, c.Param("id"), c.Param("message_id"))
		if err != nil || message == nil {
			// Do not reveal whether a message exists outside the caller's session.
			c.Status(http.StatusNotFound)
			return
		}
		streamResource(c, files, ref, "private, max-age=86400", "message files")
	}
}

// serveMessageScopedFiles registers the authenticated proxy used by the chat
// renderer for assistant-message resources. The chat API-key capability is
// sufficient because GetMessage enforces ownership of the API key's session.
func serveMessageScopedFiles(
	r *gin.RouterGroup,
	g *rbacGuards,
	messageService interfaces.MessageService,
	files interfaces.FileStore,
	catalog interfaces.ResourceCatalog,
) {
	g.apiKeyRoute(
		r,
		http.MethodGet,
		"/sessions/:id/messages/:message_id/files",
		apiKeyChat(apiKeyFullAccess()),
		g.Viewer(),
		newMessageScopedFileServeHandler(messageService, files, catalog),
	)
}

// servePresignedPreview registers an Admin-only diagnostic endpoint that
// returns the public URL that *would be* generated for one of the workspace's
// resources — exactly the URL an API response with resource_urls=public would
// carry. Operators can paste the result into a mobile browser to verify
// public reachability without driving a real client through it.
//
// Route:
//   - GET /api/v1/files/presigned-preview?file_path=resource://<handle>
func servePresignedPreview(
	r *gin.Engine, cfg *config.Config, files interfaces.FileStore, catalog interfaces.ResourceCatalog,
) {
	// This route is registered on the engine root, NOT the /api/v1 group,
	// so the APIKeyGate never runs for it. RequireRole short-circuits
	// API-key principals (deferring to that absent gate), which would let
	// any valid key past the Admin check. Deny API keys explicitly first.
	r.GET("/api/v1/files/presigned-preview",
		middleware.DenyAPIKeyPrincipal(),
		middleware.RequireRole(types.TenantRoleAdmin, cfg),
		func(c *gin.Context) {
			ctx := c.Request.Context()
			tenant, _ := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
			if tenant == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
				return
			}
			ref, resource, ok := requireResourceQuery(c, catalog)
			if !ok {
				return
			}
			if resource.TenantID != tenant.ID {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: resource not accessible"})
				return
			}
			url, ok, err := files.URL(ctx, ref, 0)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(), "storage_backend_id": resource.StorageBackendID,
				})
				return
			}
			// The no-URL case is the whole point of the endpoint: local
			// storage without APP_EXTERNAL_URL has no public address, and
			// that is what an operator debugging a broken image needs to see.
			hint := ""
			if !ok {
				hint = "no public URL: set APP_EXTERNAL_URL and make sure the reverse proxy forwards /r/"
			}
			c.JSON(http.StatusOK, gin.H{
				"file_path":          ref,
				"storage_backend_id": resource.StorageBackendID,
				"url":                url,
				"rewritten":          ok,
				"hint":               hint,
			})
		})
}
