package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// GroupHandler serves /groups/** (tenant user groups).
type GroupHandler struct{ svc *service.GroupService }

func (h *GroupHandler) ready(c *gin.Context) bool {
	if h == nil || h.svc == nil {
		NotImplemented(c)
		return false
	}
	return true
}

// CreateGroupRequest is the body of POST /groups.
type CreateGroupRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	MemberIDs   []string `json:"member_ids"`
}

// UpdateGroupRequest is the body of PATCH /groups/{gid}.
type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// SetGroupMembersRequest is the body of PUT /groups/{gid}/members.
type SetGroupMembersRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}

// List godoc
// @Summary      列出用户组
// @Description  默认组（全体成员）在前；每个组附成员数
// @Tags         用户组
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups [get]
func (h *GroupHandler) List(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	views, err := h.svc.List(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, views)
}

// Create godoc
// @Summary      创建用户组
// @Description  组名在租户内唯一（不区分大小写）；可同时指定初始成员
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Param        request  body  CreateGroupRequest  true  "用户组"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups [post]
func (h *GroupHandler) Create(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.Create(c.Request.Context(), id, service.CreateGroupInput{
		Name: req.Name, Description: req.Description, MemberIDs: req.MemberIDs,
	})
	if err != nil {
		fail(c, err)
		return
	}
	created(c, view)
}

// Get godoc
// @Summary      获取用户组
// @Tags         用户组
// @Produce      json
// @Param        gid  path  string  true  "用户组 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid} [get]
func (h *GroupHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), id, c.Param("gid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Update godoc
// @Summary      更新用户组
// @Description  默认组不可改名
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Param        gid      path  string              true  "用户组 ID"
// @Param        request  body  UpdateGroupRequest  true  "变更字段"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid} [patch]
func (h *GroupHandler) Update(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.Update(c.Request.Context(), id, c.Param("gid"), service.UpdateGroupInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Delete godoc
// @Summary      删除用户组
// @Description  同时移除该组在所有空间与页面上的授权；默认组不可删除
// @Tags         用户组
// @Param        gid  path  string  true  "用户组 ID"
// @Success      204
// @Security     Bearer
// @Router       /groups/{gid} [delete]
func (h *GroupHandler) Delete(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id, c.Param("gid")); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// ListMembers godoc
// @Summary      分页列出用户组成员
// @Tags         用户组
// @Produce      json
// @Param        gid        path   string  true   "用户组 ID"
// @Param        q          query  string  false  "按用户名/邮箱筛选"
// @Param        page       query  int     false  "页码（从 1 起）"  default(1)
// @Param        page_size  query  int     false  "每页数量（最大 100）"  default(20)
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid}/members [get]
func (h *GroupHandler) ListMembers(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	out, err := h.svc.ListMembers(c.Request.Context(), id, c.Param("gid"), c.Query("q"), page, pageSize)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, out)
}

// AddMembers godoc
// @Summary      向用户组添加成员
// @Description  已在组内的用户被跳过；默认组的成员关系是隐式的，不接受添加
// @Tags         用户组
// @Accept       json
// @Produce      json
// @Param        gid      path  string                  true  "用户组 ID"
// @Param        request  body  SetGroupMembersRequest  true  "用户 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid}/members [put]
func (h *GroupHandler) AddMembers(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	var req SetGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if err := h.svc.AddMembers(c.Request.Context(), id, c.Param("gid"), req.UserIDs); err != nil {
		fail(c, err)
		return
	}
	view, err := h.svc.Get(c.Request.Context(), id, c.Param("gid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// RemoveMember godoc
// @Summary      从用户组移除成员
// @Tags         用户组
// @Param        gid  path  string  true  "用户组 ID"
// @Param        uid  path  string  true  "用户 ID"
// @Success      204
// @Security     Bearer
// @Router       /groups/{gid}/members/{uid} [delete]
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	if err := h.svc.RemoveMember(c.Request.Context(), id, c.Param("gid"), c.Param("uid")); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}
