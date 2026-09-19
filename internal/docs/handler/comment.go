package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Comments.
//
// Every route is guarded by the page, and the service checks that the comment
// named actually belongs to it — a comment's permissions are the page's, with
// authorship deciding the rest.
//
// Note the guard levels: reading and *writing* a comment both take the reader
// role, because commenting is not editing. See the note at the top of
// internal/docs/service/comment.go.

// CreateCommentRequest is the body of POST /docs/pages/:pid/comments.
type CreateCommentRequest struct {
	// Body is a ProseMirror document — a small subset of the page schema.
	Body json.RawMessage `json:"body" binding:"required" swaggertype:"object"`
	// Anchor is the editor's Yjs relative position for the commented range.
	// Absent makes a comment about the page as a whole.
	Anchor json.RawMessage `json:"anchor,omitempty" swaggertype:"object"`
	// QuotedText is what that range covered, kept so the comment can still be
	// placed if the position stops resolving.
	QuotedText string `json:"quoted_text,omitempty"`
	// ParentID makes this a reply to an existing thread.
	ParentID string `json:"parent_id,omitempty"`
}

// UpdateCommentRequest is the body of PATCH /docs/pages/:pid/comments/:cid.
type UpdateCommentRequest struct {
	Body json.RawMessage `json:"body" binding:"required" swaggertype:"object"`
}

// ResolveCommentRequest is the body of POST /docs/pages/:pid/comments/:cid/resolve.
type ResolveCommentRequest struct {
	// Resolved false reopens a thread; the field is explicit rather than the
	// route being two verbs, so reopening is as ordinary as settling.
	Resolved bool `json:"resolved"`
}

// Comments godoc
// @Summary      列出页面评论
// @Description  按时间正序返回页面的评论线程，回复嵌在各自线程下；
// @Description  默认不含已解决的线程
// @Tags         在线文档
// @Produce      json
// @Param        pid       path   string  true   "页面 ID"
// @Param        resolved  query  bool    false  "是否包含已解决的线程，默认 false"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/comments [get]
func (h *PageHandler) Comments(c *gin.Context) {
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
	list, err := h.svc.Comments(c.Request.Context(), actor, d, c.Query("resolved") == "true")
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// CreateComment godoc
// @Summary      发表评论或回复
// @Description  在页面上发表评论；带 anchor 为行内评论，不带为页面级评论，
// @Description  带 parent_id 为回复。
// @Description  只读成员即可评论——评论不是编辑
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string                true  "页面 ID"
// @Param        request  body  CreateCommentRequest  true  "评论内容"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/comments [post]
func (h *PageHandler) CreateComment(c *gin.Context) {
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
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.CreateComment(c.Request.Context(), actor, d, service.CreateCommentInput{
		Body: req.Body, Anchor: req.Anchor, QuotedText: req.QuotedText, ParentID: req.ParentID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// UpdateComment godoc
// @Summary      修改评论
// @Description  只有评论作者可以修改自己的评论；修改正文不会移动它所指向的位置
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string                true  "页面 ID"
// @Param        cid      path  string                true  "评论 ID"
// @Param        request  body  UpdateCommentRequest  true  "新的评论内容"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/comments/{cid} [patch]
func (h *PageHandler) UpdateComment(c *gin.Context) {
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
	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.UpdateComment(c.Request.Context(), actor, d, c.Param("cid"), req.Body)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// ResolveComment godoc
// @Summary      解决或重开评论线程
// @Description  可写页面的人可以解决任何线程，评论作者可以解决自己的；
// @Description  回复随线程一起解决，不能单独解决
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string                 true  "页面 ID"
// @Param        cid      path  string                 true  "线程 ID"
// @Param        request  body  ResolveCommentRequest  true  "resolved=false 表示重开"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/comments/{cid}/resolve [post]
func (h *PageHandler) ResolveComment(c *gin.Context) {
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
	var req ResolveCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.ResolveComment(c.Request.Context(), actor, d, c.Param("cid"), req.Resolved)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// DeleteComment godoc
// @Summary      删除评论
// @Description  评论作者或空间管理员可以删除；删除线程会连同它的回复一起删除
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Param        cid  path  string  true  "评论 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/comments/{cid} [delete]
func (h *PageHandler) DeleteComment(c *gin.Context) {
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
	if err := h.svc.DeleteComment(c.Request.Context(), actor, d, c.Param("cid")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}
