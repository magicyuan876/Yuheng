package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// Watching pages, and reading what happened on them.
//
// The watch routes are guarded by the page — watching is a reader's right,
// like commenting. The inbox routes are not: a notification belongs to its
// recipient, and it says what happened rather than what a page says, so
// somebody who has since lost access to a page can still see and dismiss the
// row telling them they were mentioned on it.

// WatchRequest is the body of PUT /docs/pages/:pid/watch.
type WatchRequest struct {
	Watching bool `json:"watching"`
}

// MuteRequest is the body of PUT /docs/pages/:pid/mute.
type MuteRequest struct {
	Muted bool `json:"muted"`
}

// NotificationIDsRequest names notifications to act on; empty means all of
// them, which is what "mark everything read" sends.
type NotificationIDsRequest struct {
	IDs []string `json:"ids"`
}

// WatchState godoc
// @Summary      查询本人对页面的关注状态
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/watch [get]
func (h *PageHandler) WatchState(c *gin.Context) {
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
	view, err := h.svc.WatchState(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// SetWatch godoc
// @Summary      关注或取消关注页面
// @Description  关注后会收到该页面的评论与正文更新通知；
// @Description  创建页面、评论、被提及会自动关注
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  WatchRequest  true  "watching=false 表示取消关注"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/watch [put]
func (h *PageHandler) SetWatch(c *gin.Context) {
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
	var req WatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.WatchPage(c.Request.Context(), actor, d, req.Watching)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// SetMute godoc
// @Summary      静音或取消静音页面
// @Description  静音后不再收到该页面的评论与更新通知，但仍保留关注关系；
// @Description  被 @ 提及仍然会通知——那是对你个人的提问，不是页面动静
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string       true  "页面 ID"
// @Param        request  body  MuteRequest  true  "muted=false 表示取消静音"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/mute [put]
func (h *PageHandler) SetMute(c *gin.Context) {
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
	var req MuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.MutePage(c.Request.Context(), actor, d, req.Muted)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Notifications godoc
// @Summary      列出本人的通知
// @Description  按时间倒序返回通知，游标分页；通知属于接收者本人，不受页面权限影响
// @Tags         在线文档
// @Produce      json
// @Param        unread  query  bool    false  "只看未读"
// @Param        cursor  query  string  false  "上一页返回的游标"
// @Param        limit   query  int     false  "每页条数，默认与上限均为 50"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/notifications [get]
func (h *PageHandler) Notifications(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	page, err := h.svc.Notifications(c.Request.Context(), actor,
		c.Query("unread") == "true", c.Query("cursor"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, page)
}

// MarkNotificationsRead godoc
// @Summary      标记通知为已读
// @Description  ids 为空表示全部标记为已读
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  NotificationIDsRequest  true  "通知 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/notifications/read [post]
func (h *PageHandler) MarkNotificationsRead(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	var req NotificationIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	n, err := h.svc.MarkNotificationsRead(c.Request.Context(), actor, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"updated": n})
}

// ArchiveNotifications godoc
// @Summary      归档通知
// @Description  从通知列表中移除，但不删除记录；ids 为空表示全部归档
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  NotificationIDsRequest  true  "通知 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/notifications/archive [post]
func (h *PageHandler) ArchiveNotifications(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	var req NotificationIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	n, err := h.svc.ArchiveNotifications(c.Request.Context(), actor, req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"updated": n})
}
