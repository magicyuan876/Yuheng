package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// LeaseHandler serves the exclusive-edit transport: the lease that says who
// may type, and the Yjs state that editor loads and posts back. A deployment
// with a collaboration service answers every one of these with 409; the
// browser learns which mode it is in from docs_collab_url in the deployment
// capabilities and never calls them there.
type LeaseHandler struct{ svc *service.LeaseService }

func (h *LeaseHandler) ready(c *gin.Context) bool {
	if h == nil || h.svc == nil {
		NotImplemented(c)
		return false
	}
	return true
}

// LeaseRequest carries the caller's per-tab session identifier. The server
// never interprets it beyond comparing it to the stored one: it exists so two
// tabs of the same person cannot silently overwrite each other.
type LeaseRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// SaveYDocRequest is one save from the REST provider: the whole Yjs state
// plus its ProseMirror projection, written under the version the editor
// started from.
type SaveYDocRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	// BaseVersion is the ydoc_version this state was derived from.
	BaseVersion int64 `json:"base_version"`
	// YDoc is the full Yjs state, base64-encoded.
	YDoc string `json:"ydoc" binding:"required"`
	// Content is the ProseMirror body the server validates, renders and indexes.
	Content json.RawMessage `json:"content" binding:"required"`
}

// Get godoc
// @Summary      查询页面编辑租约
// @Description  独占编辑模式（未部署协同服务）下返回当前谁在编辑该页面；部署了协同服务时返回 409
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/lease [get]
func (h *LeaseHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	view, err := h.svc.Current(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Acquire godoc
// @Summary      获取或续期页面编辑租约
// @Description  独占编辑模式下取得页面的写入权；同一 session 重复调用即续期，他人持有且未过期时返回持有者信息
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  LeaseRequest  true  "会话标识"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/lease [post]
func (h *LeaseHandler) Acquire(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	var req LeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.Acquire(c.Request.Context(), actor, d, req.SessionID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Release godoc
// @Summary      释放页面编辑租约
// @Description  独占编辑模式下主动交还写入权；只有持有该租约的会话能释放
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  LeaseRequest  true  "会话标识"
// @Success      204
// @Security     Bearer
// @Router       /docs/pages/{pid}/lease [delete]
func (h *LeaseHandler) Release(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	req := LeaseRequest{SessionID: c.Query("session_id")}
	// A release sent from a closing tab goes out as a keepalive beacon, which
	// cannot always carry a body; the session identifier is accepted in the
	// query string for that case.
	if req.SessionID == "" {
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, "invalid request body: "+err.Error())
			return
		}
	}
	if err := h.svc.Release(c.Request.Context(), actor, d, req.SessionID); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// LoadYDoc godoc
// @Summary      获取页面协同状态
// @Description  独占编辑模式下返回页面的 Yjs 全量状态（base64）；从未协同编辑过的页面改为返回正文 JSON 供客户端首次构建
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/ydoc [get]
func (h *LeaseHandler) LoadYDoc(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	view, err := h.svc.LoadYDoc(c.Request.Context(), d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// SaveYDoc godoc
// @Summary      提交页面协同状态
// @Description  独占编辑模式下提交 Yjs 全量状态与正文 JSON；需持有未过期的编辑租约，base_version 落后时返回 409
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string           true  "页面 ID"
// @Param        request  body  SaveYDocRequest  true  "协同状态"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/ydoc [put]
func (h *LeaseHandler) SaveYDoc(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	var req SaveYDocRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	res, err := h.svc.SaveYDoc(c.Request.Context(), actor, d, service.SaveYDocInput{
		SessionID: req.SessionID, BaseVersion: req.BaseVersion, YDoc: req.YDoc, Content: req.Content,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}
