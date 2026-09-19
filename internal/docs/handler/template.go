package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Templates.
//
// Addressed by their own id rather than under a space, unlike most of this
// module, and for once that is right: a template may belong to the whole
// tenant and have no space to sit under. The permission check is therefore
// inside the service, which knows each template's scope; the route guard can
// only establish membership.

// TemplateRequest is the body of a template create or update.
type TemplateRequest struct {
	// SpaceID empty makes the template tenant-wide (needs an administrator).
	SpaceID     string          `json:"space_id,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Category    string          `json:"category,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
	// FromPageID saves an existing page's body instead of a supplied one.
	FromPageID string `json:"from_page_id,omitempty"`
}

// Templates godoc
// @Summary      列出可用模板
// @Description  返回全工作区共享的模板，以及（给定 space 时）该空间自己的模板
// @Tags         在线文档
// @Produce      json
// @Param        space  query  string  false  "空间 ID；缺省只返回共享模板"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/templates [get]
func (h *PageHandler) Templates(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	rows, err := h.svc.Templates(c.Request.Context(), actor, c.Query("space"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// Template godoc
// @Summary      读取单个模板（含正文）
// @Tags         在线文档
// @Produce      json
// @Param        tid  path  string  true  "模板 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/templates/{tid} [get]
func (h *PageHandler) Template(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	view, err := h.svc.Template(c.Request.Context(), actor, c.Param("tid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// CreateTemplate godoc
// @Summary      保存模板
// @Description  可以直接给正文，也可以用 from_page_id 把某个页面存为模板（需要能读该页面）
// @Description  保存时会移除无法跨页复用的内容：页面链接、块引用、提及与附件
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  TemplateRequest  true  "模板内容"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/templates [post]
func (h *PageHandler) CreateTemplate(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	var req TemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.CreateTemplate(c.Request.Context(), actor, service.CreateTemplateInput{
		SpaceID: req.SpaceID, Name: req.Name, Description: req.Description,
		Icon: req.Icon, Category: req.Category,
		Content: req.Content, FromPageID: req.FromPageID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// UpdateTemplate godoc
// @Summary      修改模板
// @Description  只传要改的字段；共享模板需要工作区管理员
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        tid      path  string           true  "模板 ID"
// @Param        request  body  TemplateRequest  true  "要修改的字段"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/templates/{tid} [patch]
func (h *PageHandler) UpdateTemplate(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	// Decoded into a map first so that "absent" and "empty" stay different:
	// clearing an icon and leaving it alone are different requests.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	in := service.UpdateTemplateInput{}
	if v, present := raw["name"]; present {
		var s string
		if json.Unmarshal(v, &s) == nil {
			in.Name = &s
		}
	}
	if v, present := raw["description"]; present {
		var s string
		if json.Unmarshal(v, &s) == nil {
			in.Description = &s
		}
	}
	if v, present := raw["icon"]; present {
		var s string
		if json.Unmarshal(v, &s) == nil {
			in.Icon = &s
		}
	}
	if v, present := raw["category"]; present {
		var s string
		if json.Unmarshal(v, &s) == nil {
			in.Category = &s
		}
	}
	if v, present := raw["content"]; present {
		in.Content = v
	}

	view, err := h.svc.UpdateTemplate(c.Request.Context(), actor, c.Param("tid"), in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// DeleteTemplate godoc
// @Summary      删除模板
// @Description  删除会把模板从所有人的列表里移除，所以需要该作用域的管理员
// @Tags         在线文档
// @Produce      json
// @Param        tid  path  string  true  "模板 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/templates/{tid} [delete]
func (h *PageHandler) DeleteTemplate(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	if err := h.svc.DeleteTemplate(c.Request.Context(), actor, c.Param("tid")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}
