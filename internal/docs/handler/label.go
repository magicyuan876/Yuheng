package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Labels, favourites and a space's landing page.
//
// Managing the labels is a write to the space; putting one on a page is a
// write to the page. Different permissions, checked by different guards, so
// the routes are split accordingly.

// spaceScope is the three things a space-addressed handler needs, or false
// when the guard has already answered the request.
func spaceScope(c *gin.Context) (*acl.Identity, *model.Space, model.SpaceRole, bool) {
	actor, found := identity(c)
	if !found {
		return nil, nil, model.RoleNone, false
	}
	sp, role, found := space(c)
	if !found {
		return nil, nil, model.RoleNone, false
	}
	return actor, sp, role, true
}

// LabelRequest is the body of a label create or update.
type LabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// PageLabelsRequest is the body of PUT /docs/pages/:pid/labels. The list is
// the page's labels afterwards, not additions to them.
type PageLabelsRequest struct {
	LabelIDs []string `json:"label_ids"`
}

// FavouriteRequest is the body of PUT /docs/pages/:pid/favourite.
type FavouriteRequest struct {
	Favourite bool `json:"favourite"`
}

// Labels godoc
// @Summary      列出空间标签
// @Description  返回空间的标签及各自的页面数（只统计未删除的页面）
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/labels [get]
func (h *PageHandler) Labels(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	rows, err := h.svc.Labels(c.Request.Context(), actor, space, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// CreateLabel godoc
// @Summary      新建空间标签
// @Description  空间写入者即可新建：给自己的内容归档不该需要管理员批准
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string        true  "空间 ID"
// @Param        request  body  LabelRequest  true  "标签名与颜色"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/labels [post]
func (h *PageHandler) CreateLabel(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.CreateLabel(c.Request.Context(), actor, space, role,
		service.CreateLabelInput{Name: req.Name, Color: req.Color})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// UpdateLabel godoc
// @Summary      重命名标签或改颜色
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string        true  "空间 ID"
// @Param        lid      path  string        true  "标签 ID"
// @Param        request  body  LabelRequest  true  "新的标签名或颜色"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/labels/{lid} [patch]
func (h *PageHandler) UpdateLabel(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.UpdateLabel(c.Request.Context(), actor, space, role, c.Param("lid"),
		service.CreateLabelInput{Name: req.Name, Color: req.Color})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// DeleteLabel godoc
// @Summary      删除标签
// @Description  需要空间管理员：删除会把标签从所有页面上摘掉
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Param        lid  path  string  true  "标签 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/labels/{lid} [delete]
func (h *PageHandler) DeleteLabel(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	if err := h.svc.DeleteLabel(c.Request.Context(), actor, space, role, c.Param("lid")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// SetPageLabels godoc
// @Summary      设置页面的标签
// @Description  传入的列表是设置之后页面的全部标签，不是追加；只能使用本空间的标签
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string             true  "页面 ID"
// @Param        request  body  PageLabelsRequest  true  "标签 ID 列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/labels [put]
func (h *PageHandler) SetPageLabels(c *gin.Context) {
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
	var req PageLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	rows, err := h.svc.SetPageLabels(c.Request.Context(), actor, d, req.LabelIDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// SetFavourite godoc
// @Summary      收藏或取消收藏页面
// @Description  能阅读页面即可收藏——收藏是书签，不改动任何内容
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string            true  "页面 ID"
// @Param        request  body  FavouriteRequest  true  "favourite=false 表示取消收藏"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/favourite [put]
func (h *PageHandler) SetFavourite(c *gin.Context) {
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
	var req FavouriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if err := h.svc.SetFavourite(c.Request.Context(), actor, d, req.Favourite); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"favourite": req.Favourite})
}

// Favourites godoc
// @Summary      列出本人收藏的页面
// @Description  只返回调用者仍然可以打开的页面：收藏后失去权限的页面不再出现
// @Tags         在线文档
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/favourites [get]
func (h *PageHandler) Favourites(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	rows, err := h.svc.Favourites(c.Request.Context(), actor, nil)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// SpaceHome godoc
// @Summary      空间首页
// @Description  一次返回最近编辑、标签与本人收藏，所有列表都按调用者可见性过滤
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/home [get]
func (h *PageHandler) SpaceHome(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	home, err := h.svc.Home(c.Request.Context(), actor, space, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, home)
}

// PagesWithLabels godoc
// @Summary      按标签筛选页面
// @Description  传多个标签时取交集——同时带有全部这些标签的页面
// @Tags         在线文档
// @Produce      json
// @Param        sid     path   string  true   "空间 ID"
// @Param        labels  query  string  true   "标签 ID，逗号分隔"
// @Param        limit   query  int     false  "条数上限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/pages-by-label [get]
func (h *PageHandler) PagesWithLabels(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, space, role, found := spaceScope(c)
	if !found {
		return
	}
	ids := make([]string, 0, 4)
	for _, raw := range strings.Split(c.Query("labels"), ",") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.svc.PagesWithLabels(c.Request.Context(), actor, space, role, ids, limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}
