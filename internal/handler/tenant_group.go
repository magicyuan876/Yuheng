package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// TenantGroupHandler serves /groups/**, the workspace's groups.
//
// The route layer decides who may call what (router.RegisterGroupRoutes:
// every member reads, Admin and above write), so nothing here re-checks a
// role. The workspace is the caller's active one, as the auth middleware
// resolved it; groups are not addressed under /tenants/:id, so there is no
// path id to cross-check.
type TenantGroupHandler struct {
	groups interfaces.TenantGroupService
}

// NewTenantGroupHandler wires the dependencies.
func NewTenantGroupHandler(groups interfaces.TenantGroupService) *TenantGroupHandler {
	return &TenantGroupHandler{groups: groups}
}

// CreateTenantGroupRequest is the body of POST /groups.
type CreateTenantGroupRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	MemberIDs   []string `json:"member_ids"`
}

// UpdateTenantGroupRequest is the body of PATCH /groups/{gid}.
type UpdateTenantGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// AddTenantGroupMembersRequest is the body of PUT /groups/{gid}/members.
type AddTenantGroupMembersRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}

// activeTenant reads the caller's workspace from the context. The auth
// middleware always sets it for an authenticated request, so a miss is a
// wiring mistake and is answered as such rather than as a silent empty
// listing of tenant 0.
func activeTenant(c *gin.Context) (uint64, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		_ = c.Error(apperrors.NewUnauthorizedError("no active workspace"))
		return 0, false
	}
	return tenantID, true
}

// failGroup hands the service's error to the shared error middleware. The
// service speaks in AppErrors; anything else is a database or programming
// fault, logged here and answered as a 500 without its text.
func failGroup(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		_ = c.Error(appErr)
		return
	}
	logger.Errorf(c.Request.Context(), "[tenant_group] %s %s: %v", c.Request.Method, c.FullPath(), err)
	_ = c.Error(apperrors.NewInternalServerError("internal error"))
}

// List godoc
// @Summary      列出工作区组
// @Description  默认组（全体成员）在前；每个组附成员数
// @Tags         工作区组
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups [get]
func (h *TenantGroupHandler) List(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	views, err := h.groups.List(c.Request.Context(), tenantID)
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

// Create godoc
// @Summary      创建工作区组
// @Description  组名在工作区内唯一（不区分大小写）；可同时指定初始成员
// @Tags         工作区组
// @Accept       json
// @Produce      json
// @Param        request  body  CreateTenantGroupRequest  true  "工作区组"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups [post]
func (h *TenantGroupHandler) Create(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	var req CreateTenantGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	view, err := h.groups.Create(c.Request.Context(), tenantID, types.CreateTenantGroupInput{
		Name: req.Name, Description: req.Description, MemberIDs: req.MemberIDs,
	})
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
}

// Get godoc
// @Summary      获取工作区组
// @Tags         工作区组
// @Produce      json
// @Param        gid  path  string  true  "工作区组 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid} [get]
func (h *TenantGroupHandler) Get(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	view, err := h.groups.Get(c.Request.Context(), tenantID, c.Param("gid"))
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// Update godoc
// @Summary      更新工作区组
// @Description  默认组不可改名
// @Tags         工作区组
// @Accept       json
// @Produce      json
// @Param        gid      path  string                    true  "工作区组 ID"
// @Param        request  body  UpdateTenantGroupRequest  true  "变更字段"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid} [patch]
func (h *TenantGroupHandler) Update(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	var req UpdateTenantGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	view, err := h.groups.Update(c.Request.Context(), tenantID, c.Param("gid"), types.UpdateTenantGroupInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// Delete godoc
// @Summary      删除工作区组
// @Description  同时移除该组在所有模块中持有的授权（如文档空间与页面）；默认组不可删除
// @Tags         工作区组
// @Param        gid  path  string  true  "工作区组 ID"
// @Success      204
// @Security     Bearer
// @Router       /groups/{gid} [delete]
func (h *TenantGroupHandler) Delete(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	if err := h.groups.Delete(c.Request.Context(), tenantID, c.Param("gid")); err != nil {
		failGroup(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListMembers godoc
// @Summary      分页列出工作区组成员
// @Tags         工作区组
// @Produce      json
// @Param        gid        path   string  true   "工作区组 ID"
// @Param        q          query  string  false  "按用户名/邮箱筛选"
// @Param        page       query  int     false  "页码（从 1 起）"  default(1)
// @Param        page_size  query  int     false  "每页数量（最大 100）"  default(20)
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid}/members [get]
func (h *TenantGroupHandler) ListMembers(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	// Unparsable paging falls back to the defaults inside the service, the
	// same forgiving treatment the docs module gave these parameters.
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	out, err := h.groups.ListMembers(c.Request.Context(), tenantID, c.Param("gid"), c.Query("q"), page, pageSize)
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

// AddMembers godoc
// @Summary      向工作区组添加成员
// @Description  已在组内的用户被跳过；默认组的成员关系是隐式的，不接受添加
// @Tags         工作区组
// @Accept       json
// @Produce      json
// @Param        gid      path  string                        true  "工作区组 ID"
// @Param        request  body  AddTenantGroupMembersRequest  true  "用户 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /groups/{gid}/members [put]
func (h *TenantGroupHandler) AddMembers(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	var req AddTenantGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	ctx := c.Request.Context()
	if err := h.groups.AddMembers(ctx, tenantID, c.Param("gid"), req.UserIDs); err != nil {
		failGroup(c, err)
		return
	}
	// Answer with the group as it now stands, so the client can refresh its
	// member count without a second round-trip.
	view, err := h.groups.Get(ctx, tenantID, c.Param("gid"))
	if err != nil {
		failGroup(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// RemoveMember godoc
// @Summary      从工作区组移除成员
// @Tags         工作区组
// @Param        gid  path  string  true  "工作区组 ID"
// @Param        uid  path  string  true  "用户 ID"
// @Success      204
// @Security     Bearer
// @Router       /groups/{gid}/members/{uid} [delete]
func (h *TenantGroupHandler) RemoveMember(c *gin.Context) {
	tenantID, ok := activeTenant(c)
	if !ok {
		return
	}
	if err := h.groups.RemoveMember(c.Request.Context(), tenantID, c.Param("gid"), c.Param("uid")); err != nil {
		failGroup(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
