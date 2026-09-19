package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Page-level permissions.
//
// All four routes are addressed under their page, because a page's
// permissions are the page's: the guard that resolves the page has already
// worked out what the caller may do with it, and re-deriving that from a
// grant id would mean every handler here doing its own permission arithmetic.

// pageScope is the caller and the page decision a page-addressed handler
// needs, or false when a guard has already answered the request.
func pageScope(c *gin.Context) (*acl.Identity, acl.Decision, bool) {
	actor, found := identity(c)
	if !found {
		return nil, acl.Decision{}, false
	}
	d, found := decision(c)
	if !found {
		return nil, acl.Decision{}, false
	}
	return actor, d, true
}

// RestrictRequest is the body of PUT /docs/pages/:pid/access.
type RestrictRequest struct {
	// Restricted false restores inheritance and drops every grant.
	Restricted bool `json:"restricted"`
}

// GrantRequest is the body of POST /docs/pages/:pid/grants.
type GrantRequest struct {
	PrincipalType string `json:"principal_type"`
	PrincipalID   string `json:"principal_id"`
	Role          string `json:"role"`
}

// PageAccess godoc
// @Summary      读取页面权限
// @Description  返回本页是否切断继承、被哪些上级收窄、以及受限时的授权名单
// @Description  能打开页面即可查看：知道自己为什么能看见，是回答"同事为什么看不见"的前提
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/access [get]
func (h *PageHandler) PageAccess(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	view, err := h.svc.PageAccess(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// SetPageAccess godoc
// @Summary      切断或恢复页面的权限继承
// @Description  切断时会自动把页面作者加入名单——收窄受众不等于没收作者的页面
// @Description  恢复继承会一并删除本页的全部授权
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string           true  "页面 ID"
// @Param        request  body  RestrictRequest  true  "restricted=false 表示恢复继承"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/access [put]
func (h *PageHandler) SetPageAccess(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req RestrictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.SetPageRestricted(c.Request.Context(), actor, d, req.Restricted)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// PageGrants godoc
// @Summary      列出页面的授权名单
// @Description  与 GET /access 返回同一份名单，供只关心名单的客户端使用；页面未受限时为空
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/grants [get]
func (h *PageHandler) PageGrants(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	view, err := h.svc.PageAccess(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view.Grants)
}

// AddPageGrant godoc
// @Summary      给某人或某组授权
// @Description  授权只能收窄：给空间里的只读成员授予 admin，他仍然是只读
// @Description  返回的 effective 字段是这条授权实际生效的角色
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  GrantRequest  true  "主体与角色"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/grants [post]
func (h *PageHandler) AddPageGrant(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req GrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.SetPageGrant(c.Request.Context(), actor, d, service.GrantInput{
		PrincipalType: model.PrincipalType(req.PrincipalType),
		PrincipalID:   req.PrincipalID,
		Role:          model.SpaceRole(req.Role),
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// RemovePageGrant godoc
// @Summary      移除一条授权
// @Tags         在线文档
// @Produce      json
// @Param        pid        path  string  true  "页面 ID"
// @Param        ptype      path  string  true  "主体类型：user 或 group"
// @Param        principal  path  string  true  "用户 ID 或用户组 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/grants/{ptype}/{principal} [delete]
func (h *PageHandler) RemovePageGrant(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	view, err := h.svc.RemovePageGrant(c.Request.Context(), actor, d,
		model.PrincipalType(c.Param("ptype")), c.Param("principal"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// EffectivePermission godoc
// @Summary      本人在该页的有效权限
// @Description  供客户端决定画哪些控件：角色、能否编辑、能否评论、能否管理权限
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/effective-permission [get]
func (h *PageHandler) EffectivePermission(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	view, err := h.svc.EffectivePermission(c.Request.Context(), d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}
