package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// PageHandler serves /docs/pages/** and the tree/trash routes under a space.
type PageHandler struct{ svc *service.PageService }

func (h *PageHandler) ready(c *gin.Context) bool {
	if h == nil || h.svc == nil {
		NotImplemented(c)
		return false
	}
	return true
}

// decision returns the page decision the guard resolved.
func decision(c *gin.Context) (acl.Decision, bool) {
	d, found := acl.DecisionFromGin(c)
	if !found || d.Page == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		c.Abort()
		return acl.Decision{}, false
	}
	return d, true
}

// OptionalID distinguishes an absent JSON field from an explicit null and
// from a value, which a plain *string cannot.
type OptionalID struct {
	Set   bool
	Value *string
}

// UnmarshalJSON records presence; "null" leaves Value nil.
func (o *OptionalID) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.Value = &s
	return nil
}

// CreatePageRequest is the body of POST /docs/pages. The page is appended
// after its siblings; content and markdown are mutually exclusive.
type CreatePageRequest struct {
	SpaceID  string          `json:"space_id" binding:"required"`
	ParentID *string         `json:"parent_id"`
	Title    string          `json:"title"`
	Icon     *string         `json:"icon"`
	Content  json.RawMessage `json:"content"`
	Markdown string          `json:"markdown"`
}

// UpdatePageRequest is the body of PATCH /docs/pages/{pid}; absent fields
// are unchanged, an empty icon or cover clears it.
type UpdatePageRequest struct {
	Title *string `json:"title"`
	Icon  *string `json:"icon"`
	Cover *string `json:"cover"`
}

// MovePageRequest is the body of POST /docs/pages/{pid}/move.
//
//   - parent_id: the new parent; null or absent means the space root.
//   - space_id: a different space to move into (with the subtree).
//   - after_id: the sibling to follow; null puts the page first; absent
//     appends it after the last sibling.
//   - position: an explicit order key for clients that keep their own tree;
//     overrides after_id.
type MovePageRequest struct {
	ParentID *string    `json:"parent_id"`
	SpaceID  string     `json:"space_id"`
	AfterID  OptionalID `json:"after_id"`
	Position string     `json:"position"`
}

// DuplicatePageRequest is the body of POST /docs/pages/{pid}/duplicate.
type DuplicatePageRequest struct {
	// SpaceID copies into another space; empty duplicates next to the original.
	SpaceID string `json:"space_id"`
	// ParentID (other space only) places the copy under a page there.
	ParentID *string `json:"parent_id"`
	// Title overrides the copy's title.
	Title string `json:"title"`
}

// Create godoc
// @Summary      创建页面
// @Description  在父页面（或空间根）末尾新建页面；可携带 JSON 文档或 Markdown 作为初始内容
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  CreatePageRequest  true  "页面"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages [post]
func (h *PageHandler) Create(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	var req CreatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.Create(c.Request.Context(), id, service.CreatePageInput{
		SpaceID: req.SpaceID, ParentID: req.ParentID, Title: req.Title, Icon: req.Icon,
		Content: req.Content, Markdown: req.Markdown,
	})
	if err != nil {
		fail(c, err)
		return
	}
	created(c, view)
}

// Get godoc
// @Summary      获取页面
// @Description  返回页面元数据、调用者的有效角色与是否有子页面；回收站中的页面返回 410
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid} [get]
func (h *PageHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// GetByShortID godoc
// @Summary      按短标识获取页面
// @Tags         在线文档
// @Produce      json
// @Param        short  path  string  true  "页面短标识"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/by-short-id/{short} [get]
func (h *PageHandler) GetByShortID(c *gin.Context) { h.Get(c) }

// Content godoc
// @Summary      获取页面正文
// @Description  返回 ProseMirror JSON 正文与版本号；format=html 时附带只读 HTML 渲染
// @Tags         在线文档
// @Produce      json
// @Param        pid     path   string  true   "页面 ID"
// @Param        format  query  string  false  "json（默认）或 html"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/content [get]
func (h *PageHandler) Content(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "html" {
		badRequest(c, "format must be json or html")
		return
	}
	out, err := h.svc.Content(c.Request.Context(), d, format == "html")
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, out)
}

// Update godoc
// @Summary      更新页面元数据
// @Description  修改标题、图标或封面；正文通过协同服务或 replace 接口写入
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string             true  "页面 ID"
// @Param        request  body  UpdatePageRequest  true  "字段"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid} [patch]
func (h *PageHandler) Update(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	var req UpdatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.Update(c.Request.Context(), id, d, service.UpdatePageInput{
		Title: req.Title, Icon: req.Icon, Cover: req.Cover,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Move godoc
// @Summary      移动页面
// @Description  改变父页面、排序位置或所属空间；跨空间移动携带子树，调用者不可见的子页面留在原空间根部
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string           true  "页面 ID"
// @Param        request  body  MovePageRequest  true  "目标位置"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/move [post]
func (h *PageHandler) Move(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	var req MovePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	placement := service.Placement{Mode: service.PlaceEnd}
	switch {
	case req.Position != "":
		placement = service.Placement{Mode: service.PlaceAt, Position: req.Position}
	case req.AfterID.Set && req.AfterID.Value == nil:
		placement = service.Placement{Mode: service.PlaceFirst}
	case req.AfterID.Set:
		placement = service.Placement{Mode: service.PlaceAfter, AfterID: *req.AfterID.Value}
	}
	result, err := h.svc.Move(c.Request.Context(), id, d, service.MovePageInput{
		ParentID: req.ParentID, SpaceID: req.SpaceID, Placement: placement,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, result)
}

// Duplicate godoc
// @Summary      复制页面
// @Description  复制页面及调用者可见的子树到原位置之后或另一空间；副本沿用目标位置的权限
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string                true  "页面 ID"
// @Param        request  body  DuplicatePageRequest  false "目标"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/duplicate [post]
func (h *PageHandler) Duplicate(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	var req DuplicatePageRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, "invalid request body: "+err.Error())
			return
		}
	}
	result, err := h.svc.Duplicate(c.Request.Context(), id, d, service.DuplicatePageInput{
		SpaceID: req.SpaceID, ParentID: req.ParentID, Title: req.Title,
	})
	if err != nil {
		fail(c, err)
		return
	}
	created(c, result)
}

// Delete godoc
// @Summary      删除页面
// @Description  页面及其子页面进入回收站；可从空间回收站恢复
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid} [delete]
func (h *PageHandler) Delete(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	n, err := h.svc.Delete(c.Request.Context(), id, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"page_id": d.Page.ID, "deleted": n})
}

// Restore godoc
// @Summary      恢复页面
// @Description  把回收站中的页面及随之删除的子页面恢复；父页面仍在回收站时挂到空间根部
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/restore [post]
func (h *PageHandler) Restore(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	view, err := h.svc.Restore(c.Request.Context(), id, c.Param("pid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Ancestors godoc
// @Summary      页面面包屑
// @Description  从空间根到父页面的祖先链，不含页面本身
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/ancestors [get]
func (h *PageHandler) Ancestors(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	chain, err := h.svc.Ancestors(c.Request.Context(), d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, chain)
}

// Children godoc
// @Summary      子页面列表
// @Description  按阅读顺序分页返回一个页面的直接子页面（仅调用者可见的）
// @Tags         在线文档
// @Produce      json
// @Param        pid     path   string  true   "页面 ID"
// @Param        cursor  query  string  false  "上一页返回的 next_cursor"
// @Param        limit   query  int     false  "每页数量，默认 500，最多 2000"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/children [get]
func (h *PageHandler) Children(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	spaceRole, err := h.svc.SpaceRoleOf(c.Request.Context(), id, d.Space)
	if err != nil {
		fail(c, err)
		return
	}
	pid := d.Page.ID
	page, err := h.svc.Children(c.Request.Context(), id, d.Space, spaceRole, &pid, c.Query("cursor"),
		queryInt(c, "limit"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, page)
}

// Tree godoc
// @Summary      空间页面树（按父节点懒加载）
// @Description  返回 parent 下（省略则为根）的子页面一页；每行带 has_children 与 can_edit
// @Tags         在线文档
// @Produce      json
// @Param        sid     path   string  true   "空间 ID"
// @Param        parent  query  string  false  "父页面 ID；省略为根"
// @Param        cursor  query  string  false  "上一页返回的 next_cursor"
// @Param        limit   query  int     false  "每页数量，默认 500，最多 2000"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/tree [get]
func (h *PageHandler) Tree(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	sp, role, found := space(c)
	if !found {
		return
	}
	var parent *string
	if p := c.Query("parent"); p != "" {
		parent = &p
	}
	page, err := h.svc.Children(c.Request.Context(), id, sp, role, parent, c.Query("cursor"), queryInt(c, "limit"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, page)
}

// Trash godoc
// @Summary      空间回收站
// @Description  最近删除优先，列出调用者可见的已删除页面（删除根，不展开子页面）
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/trash [get]
func (h *PageHandler) Trash(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	sp, _, found := space(c)
	if !found {
		return
	}
	entries, err := h.svc.Trash(c.Request.Context(), id, sp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, entries)
}

// Purge godoc
// @Summary      彻底删除回收站中的页面
// @Description  空间管理员永久删除一个已删除页面及其子树，不可恢复
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/trash/{pid} [delete]
func (h *PageHandler) Purge(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	sp, _, found := space(c)
	if !found {
		return
	}
	ids, err := h.svc.Purge(c.Request.Context(), id, sp, c.Param("pid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"purged": ids})
}

// EmptyTrash godoc
// @Summary      清空空间回收站
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/trash [delete]
func (h *PageHandler) EmptyTrash(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	sp, _, found := space(c)
	if !found {
		return
	}
	n, err := h.svc.EmptyTrash(c.Request.Context(), id, sp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"purged": n})
}

func queryInt(c *gin.Context, key string) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return v
}
