package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// ResolveTitlesRequest is the body of POST /docs/page-links/titles.
//
// It is a POST because the list of ids is as long as the open page has links,
// which does not belong in a URL. It reads nothing and changes nothing, and is
// declared with the read capability accordingly.
type ResolveTitlesRequest struct {
	PageIDs []string `json:"page_ids" binding:"required"`
}

// Backlinks godoc
// @Summary      列出引用本页的页面
// @Description  返回引用了这个页面（页面链接或块引用）且调用者有权查看的页面；看不到的页面不出现在列表里
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/backlinks [get]
func (h *PageHandler) Backlinks(c *gin.Context) {
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
	rows, err := h.svc.Backlinks(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// Findings godoc
// @Summary      本页的知识健康问题
// @Description  本页在知识库中的镜像被检测出的未处理问题（与其他文档重复或内容有出入）。只列出另一方是调用者有权查看的页面的问题；另一方不是页面或调用者看不到的，只计入 other_count
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}  "data: {items, other_count}"
// @Security     Bearer
// @Router       /docs/pages/{pid}/findings [get]
func (h *PageHandler) Findings(c *gin.Context) {
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
	view, err := h.svc.Findings(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// ResolveTitles godoc
// @Summary      批量解析页面链接标题
// @Description  把页面 ID 解析成当前标题，供编辑器里的页面链接显示；页面已删除或调用者无权查看时 resolved=false，两种情况不作区分
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  ResolveTitlesRequest  true  "页面 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/page-links/titles [post]
func (h *PageHandler) ResolveTitles(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	var req ResolveTitlesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	rows, err := h.svc.ResolveTitles(c.Request.Context(), actor, req.PageIDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// SuggestPages godoc
// @Summary      页面链接建议
// @Description  按标题模糊匹配返回可插入为页面链接的页面；只返回调用者有权查看的页面。space 留空时在全部可见空间里找
// @Tags         在线文档
// @Produce      json
// @Param        q      query  string  false  "标题关键字"
// @Param        space  query  string  false  "限定空间 ID"
// @Param        limit  query  int     false  "条数上限（最多 12）"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/page-links/suggest [get]
func (h *PageHandler) SuggestPages(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.svc.SuggestPages(c.Request.Context(), actor, service.SuggestPagesInput{
		Query: c.Query("q"), SpaceID: c.Query("space"), Limit: limit,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// SuggestMentions godoc
// @Summary      提及人选建议
// @Description  返回能读到这个页面的人，供 @ 提及使用；受限页面会相应收窄候选人范围
// @Tags         在线文档
// @Produce      json
// @Param        pid    path   string  true   "页面 ID"
// @Param        q      query  string  false  "姓名或邮箱关键字"
// @Param        limit  query  int     false  "条数上限（最多 12）"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/mention-candidates [get]
func (h *PageHandler) SuggestMentions(c *gin.Context) {
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
	rows, err := h.svc.SuggestMentions(c.Request.Context(), actor, d, c.Query("q"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// SupersedeRequest is the body of POST /docs/pages/:pid/supersede.
type SupersedeRequest struct {
	// FindingID is one of the page's findings; its other page is the one
	// this page supersedes.
	FindingID string `json:"finding_id" binding:"required"`
}

// Supersede godoc
// @Summary      用本页取代另一个页面
// @Description  处理本页的一个知识健康问题：保留本页，把问题另一方的页面排除出知识库并标记为“已被本页取代”，该页面仍可阅读，重新参与知识库时标记清除。
// @Description  需要能编辑两个页面；另一方不是页面时返回 409（上传的文档由知识库编辑者在知识健康里处理）
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string            true  "页面 ID（保留的一方）"
// @Param        request  body  SupersedeRequest  true  "问题 ID"
// @Success      200  {object}  map[string]interface{}  "data: {retired_knowledge_id, how}"
// @Security     Bearer
// @Router       /docs/pages/{pid}/supersede [post]
func (h *PageHandler) Supersede(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req SupersedeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	result, err := h.svc.SupersedeFromPage(c.Request.Context(), actor, d, req.FindingID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, result)
}
