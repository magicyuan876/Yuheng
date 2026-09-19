package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// CollabHandler serves /internal/collab/**: the callbacks of the
// collaboration service. These routes are registered before the session
// middleware and carry no user session; each request is authorised by an
// HMAC signature over its body (internal/docs/collab/sign.go), so they must
// never be reachable from the public internet (the ingress only forwards the
// browser-facing WebSocket path).
type CollabHandler struct {
	svc    *service.CollabService
	secret string
	// maxBody bounds a store request; the Yjs state is base64 so a 20 MiB
	// document arrives as roughly 27 MiB of JSON.
	maxBody int64
}

// NewCollabHandler builds the handler. An empty secret disables the routes:
// they answer 404 so an accidental deployment cannot be probed.
func NewCollabHandler(svc *service.CollabService, secret string, maxYDocBytes int64) *CollabHandler {
	limit := maxYDocBytes
	if limit <= 0 {
		limit = service.DefaultMaxYDocBytes
	}
	// base64 plus the JSON envelope, with room for the projection.
	return &CollabHandler{svc: svc, secret: secret, maxBody: limit*2 + 4<<20}
}

// Enabled reports whether the callbacks are usable.
func (h *CollabHandler) Enabled() bool {
	return h != nil && h.svc != nil && h.secret != ""
}

// authorise reads the body and verifies the signature.
func (h *CollabHandler) authorise(c *gin.Context) ([]byte, bool) {
	if !h.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, h.maxBody))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read the request body"})
		return nil, false
	}
	if err := collab.Verify(c.Request, h.secret, body, time.Now()); err != nil {
		logger.Warnf(c.Request.Context(), "[docs.collab] rejected %s %s: %v",
			c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	return body, true
}

// AuthenticateRequest is the body of POST /internal/collab/authenticate.
type AuthenticateRequest struct {
	Token        string `json:"token"`
	PageID       string `json:"page_id"`
	TenantID     string `json:"tenant_id"`
	ConnectionID string `json:"connection_id"`
}

// Authenticate resolves who is connecting and what they may do.
func (h *CollabHandler) Authenticate(c *gin.Context) {
	body, ok := h.authorise(c)
	if !ok {
		return
	}
	var req AuthenticateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	tenantID, _ := strconv.ParseUint(req.TenantID, 10, 64)
	result, err := h.svc.Authenticate(c.Request.Context(), service.AuthenticateInput{
		Token: req.Token, TenantID: tenantID, PageID: req.PageID, ConnectionID: req.ConnectionID,
	})
	if err != nil {
		h.failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Load returns the Yjs state of a page, or its JSON body the first time it
// is opened collaboratively.
func (h *CollabHandler) Load(c *gin.Context) {
	body, ok := h.authorise(c)
	if !ok {
		return
	}
	_ = body
	tenantID, _ := strconv.ParseUint(c.Query("tenant"), 10, 64)
	if tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant is required"})
		return
	}
	result, err := h.svc.Load(c.Request.Context(), tenantID, c.Param("pid"))
	if err != nil {
		h.failInternal(c, err)
		return
	}
	c.Header("X-YDoc-Version", strconv.FormatInt(result.YDocVersion, 10))
	if len(result.YDoc) > 0 {
		c.Data(http.StatusOK, "application/octet-stream", result.YDoc)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": result.Content, "ydoc_version": result.YDocVersion})
}

// StoreRequest is the body of POST /internal/collab/store.
type StoreRequest struct {
	TenantID       string          `json:"tenant_id"`
	PageID         string          `json:"page_id"`
	BaseVersion    int64           `json:"base_version"`
	YDoc           string          `json:"ydoc"`
	Content        json.RawMessage `json:"content" swaggertype:"object"`
	EditorIDs      []string        `json:"editor_ids"`
	AwarenessCount int             `json:"awareness_count"`
}

// Store persists a merged document.
func (h *CollabHandler) Store(c *gin.Context) {
	body, ok := h.authorise(c)
	if !ok {
		return
	}
	var req StoreRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	state, err := base64.StdEncoding.DecodeString(req.YDoc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ydoc is not valid base64"})
		return
	}
	tenantID, _ := strconv.ParseUint(req.TenantID, 10, 64)
	if tenantID == 0 || req.PageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id and page_id are required"})
		return
	}
	result, err := h.svc.Persist(c.Request.Context(), service.PersistInput{
		TenantID: tenantID, PageID: req.PageID, BaseVersion: req.BaseVersion, YDoc: state,
		Content: req.Content, EditorIDs: req.EditorIDs, AwarenessCount: req.AwarenessCount,
	})
	if err != nil {
		h.failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Health lets the collaboration service prove it can reach this server.
func (h *CollabHandler) Health(c *gin.Context) {
	if _, ok := h.authorise(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// failInternal maps service errors onto the answers the collaboration
// service knows how to act on: 401 closes the socket, 409 makes it merge and
// retry, 404/410 makes it stop, 4xx is reported to the user, 5xx is retried.
func (h *CollabHandler) failInternal(c *gin.Context, err error) {
	ctx := c.Request.Context()
	switch {
	case errors.Is(err, service.ErrCollabUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	case errors.Is(err, repository.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "version conflict", "code": "conflict"})
	default:
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.HTTPCode < 500 {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr.Error(), "code": "rejected"})
			return
		}
		logger.Errorf(ctx, "[docs.collab] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
