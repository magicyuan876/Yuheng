package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// Locking a page and marking it draft or published.

// LockRequest is the body of PUT /docs/pages/:pid/lock.
type LockRequest struct {
	Locked bool `json:"locked"`
}

// StatusRequest is the body of PUT /docs/pages/:pid/status.
type StatusRequest struct {
	Status string `json:"status"`
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

// SetPageStatus godoc
// @Summary      标记页面为草稿或已发布
// @Description  草稿**不是**权限：能读这个页面的人照样能读。它只是一个标记，
// @Description  让客户端可以排序、筛选，或者把未完成的内容排除在「成品」列表之外
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string         true  "页面 ID"
// @Param        request  body  StatusRequest  true  "draft 或 published"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/status [put]
func (h *PageHandler) SetPageStatus(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req StatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.SetPageStatus(c.Request.Context(), actor, d, model.PageStatus(req.Status))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}
