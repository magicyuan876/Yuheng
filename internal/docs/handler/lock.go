package handler

import (
	"github.com/gin-gonic/gin"
)

// Locking a page, and whether it takes part in the knowledge base.

// LockRequest is the body of PUT /docs/pages/:pid/lock.
type LockRequest struct {
	Locked bool `json:"locked"`
}

// KnowledgeRequest is the body of PUT /docs/pages/:pid/knowledge.
type KnowledgeRequest struct {
	// Excluded keeps the page out of the knowledge base; false lets it take part.
	Excluded bool `json:"excluded"`
}

// SetLocked godoc
// @Summary      锁定或解锁页面
// @Description  锁定后除空间管理员外一律降为只读，正在编辑的会话会被立即断开
// @Description  加锁与解锁都需要空间管理员：否则「这页是否写完」就由先点的人说了算
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string       true  "页面 ID"
// @Param        request  body  LockRequest  true  "locked=false 表示解锁"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/lock [put]
func (h *PageHandler) SetLocked(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req LockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.SetLocked(c.Request.Context(), actor, d, req.Locked)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// SetKnowledgeExcluded godoc
// @Summary      让页面不参与（或重新参与）知识库检索
// @Description  开启后，这个页面不会被 AI 问答引用，页面本身对有权限的人照常可见。
// @Description  它**不是**权限：能读这个页面的人照样能读。受限页面无论如何都不会进入知识库
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string            true  "页面 ID"
// @Param        request  body  KnowledgeRequest  true  "excluded=true 表示不参与"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/knowledge [put]
func (h *PageHandler) SetKnowledgeExcluded(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req KnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.SetKnowledgeExcluded(c.Request.Context(), actor, d, req.Excluded)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}
