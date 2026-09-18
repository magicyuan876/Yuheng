package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// Page history: listing versions, reading one, comparing two, going back.
//
// Access is governed by the page throughout — a snapshot is that page at an
// earlier moment — so every route here is guarded by the page, and the
// service checks that the revision named actually belongs to it.

// History godoc
// @Summary      列出页面历史版本
// @Description  按时间倒序返回页面的历史版本（不含正文），游标分页；
// @Description  可读该页面即可查看历史
// @Tags         在线文档
// @Produce      json
// @Param        pid     path   string  true   "页面 ID"
// @Param        cursor  query  string  false  "上一页返回的游标"
// @Param        limit   query  int     false  "每页条数，默认与上限均为 100"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/revisions [get]
func (h *PageHandler) History(c *gin.Context) {
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
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.svc.History(c.Request.Context(), actor, d, c.Query("cursor"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// Revision godoc
// @Summary      读取一个历史版本
// @Description  返回某个历史版本的完整正文，用于预览与恢复前确认
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Param        rid  path  string  true  "版本 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/revisions/{rid} [get]
func (h *PageHandler) Revision(c *gin.Context) {
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
	row, err := h.svc.Revision(c.Request.Context(), actor, d, c.Param("rid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, row)
}

// RevisionDiff godoc
// @Summary      比较两个版本
// @Description  返回逐行文本差异与按块 ID 的结构差异；不传 to 时与页面当前内容比较
// @Tags         在线文档
// @Produce      json
// @Param        pid  path   string  true   "页面 ID"
// @Param        rid  path   string  true   "较旧的版本 ID"
// @Param        to   query  string  false  "较新的版本 ID，缺省为页面当前内容"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/revisions/{rid}/diff [get]
func (h *PageHandler) RevisionDiff(c *gin.Context) {
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
	diff, err := h.svc.Diff(c.Request.Context(), actor, d, c.Param("rid"), c.Query("to"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, diff)
}

// RestoreRevision godoc
// @Summary      恢复到某个历史版本
// @Description  把页面正文恢复为该版本；恢复前会先为当前内容留一个版本，
// @Description  因此恢复本身也可以再被撤回。
// @Description  写入走与导入相同的统一入口，所以正在协同编辑的人会实时看到内容变化
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Param        rid  path  string  true  "版本 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/revisions/{rid}/restore [post]
func (h *PageHandler) RestoreRevision(c *gin.Context) {
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
	res, err := h.svc.RestoreRevision(c.Request.Context(), actor, d, c.Param("rid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}
