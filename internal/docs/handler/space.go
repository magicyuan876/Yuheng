package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// SpaceHandler serves /docs/spaces/**.
type SpaceHandler struct{ svc *service.SpaceService }

// identity returns the caller the guard resolved; a missing identity means
// the route was registered without a guard, which is a wiring bug.
func identity(c *gin.Context) (*acl.Identity, bool) {
	id, ok := acl.IdentityFromGin(c)
	if !ok || id == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no docs identity"})
		c.Abort()
		return nil, false
	}
	return id, true
}

// space returns the space the guard resolved.
func space(c *gin.Context) (*model.Space, model.SpaceRole, bool) {
	sp, role, ok := acl.SpaceFromGin(c)
	if !ok || sp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		c.Abort()
		return nil, model.RoleNone, false
	}
	return sp, role, true
}

func (h *SpaceHandler) ready(c *gin.Context) bool {
	if h == nil || h.svc == nil {
		NotImplemented(c)
		return false
	}
	return true
}

// CreateSpaceRequest is the body of POST /docs/spaces.
type CreateSpaceRequest struct {
	Name        string  `json:"name"        binding:"required"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Icon        *string `json:"icon"`
	// Visibility: private (default) or open.
	Visibility string `json:"visibility"`
	// DefaultRole for open spaces: reader (default) or writer.
	DefaultRole string `json:"default_role"`
	// KnowledgeBase is what the space syncs its pages into; omitted means
	// none. The web form sends {"mode":"create"} unless told otherwise.
	KnowledgeBase *KnowledgeBaseChoiceRequest `json:"knowledge_base"`
	// LegacyKnowledgeBaseID is the field knowledge_base replaced; it is
	// refused rather than ignored, so an old client learns its binding did
	// not happen.
	LegacyKnowledgeBaseID *string         `json:"knowledge_base_id" swaggerignore:"true"`
	StorageBackendID      *string         `json:"storage_backend_id"`
	Settings              json.RawMessage `json:"settings" swaggertype:"object"`
}

// KnowledgeBaseChoiceRequest says what a space syncs its pages into.
type KnowledgeBaseChoiceRequest struct {
	// Mode: none (pages stay out of every knowledge base), existing (bind the
	// knowledge base named by id) or create (make a new document knowledge
	// base named like the space, on the space's storage backend, with the
	// workspace's default models, and bind it).
	Mode string `json:"mode" enums:"none,existing,create"`
	// ID names the knowledge base for mode existing; empty otherwise.
	ID string `json:"id"`
}

func (r *KnowledgeBaseChoiceRequest) choice() *service.KnowledgeBaseChoice {
	if r == nil {
		return nil
	}
	return &service.KnowledgeBaseChoice{Mode: service.KnowledgeBaseMode(r.Mode), ID: r.ID}
}

// legacyKnowledgeBaseField is the answer to a request still using
// knowledge_base_id.
const legacyKnowledgeBaseField = `knowledge_base_id was replaced by knowledge_base: ` +
	`{"mode":"existing","id":"..."}, {"mode":"create"} or {"mode":"none"}`

// UpdateSpaceRequest is the body of PATCH /docs/spaces/{sid}; absent fields
// are left unchanged, an empty icon clears it.
type UpdateSpaceRequest struct {
	Name        *string         `json:"name"`
	Slug        *string         `json:"slug"`
	Description *string         `json:"description"`
	Icon        *string         `json:"icon"`
	Visibility  *string         `json:"visibility"`
	DefaultRole *string         `json:"default_role"`
	Settings    json.RawMessage `json:"settings" swaggertype:"object"`
}

// SpaceMemberRequest names one principal and the role to grant.
type SpaceMemberRequest struct {
	PrincipalType string `json:"principal_type" binding:"required"`
	PrincipalID   string `json:"principal_id"   binding:"required"`
	Role          string `json:"role"           binding:"required"`
}

// SetSpaceMembersRequest is the body of PUT /docs/spaces/{sid}/members.
type SetSpaceMembersRequest struct {
	Members []SpaceMemberRequest `json:"members" binding:"required"`
}

// BindKnowledgeBaseRequest is the body of PUT /docs/spaces/{sid}/knowledge-base.
// Absent fields are unchanged; knowledge_base {"mode":"none"} unbinds, an
// empty storage_backend_id rebinds the space to the workspace default.
type BindKnowledgeBaseRequest struct {
	KnowledgeBase *KnowledgeBaseChoiceRequest `json:"knowledge_base"`
	// LegacyKnowledgeBaseID: see CreateSpaceRequest.
	LegacyKnowledgeBaseID *string `json:"knowledge_base_id" swaggerignore:"true"`
	StorageBackendID      *string `json:"storage_backend_id"`
}

// List godoc
// @Summary      列出可见的文档空间
// @Description  返回当前用户可读的空间（成员空间、开放空间；工作区管理员见全部）及其在每个空间中的角色
// @Tags         在线文档
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces [get]
func (h *SpaceHandler) List(c *gin.Context) {
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
// @Summary      创建文档空间
// @Description  创建者自动成为空间管理员；slug 省略时由名称生成并保证唯一。
// @Description  knowledge_base 决定页面同步到哪个知识库：省略或 mode=none 不同步；
// @Description  mode=existing 绑定 id 指定的文档型知识库（调用者须是该知识库的创建者或工作区管理员）；
// @Description  mode=create 同时新建一个与空间同名的文档型知识库并绑定（与空间同一存储后端，使用工作区默认模型）。
// @Description  工作区没有 Embedding 模型时返回 400、错误码 2300；空间写入失败时新建的知识库会被删除。
// @Description  API Key 用 mode=create 需要 manage_kbs 或完全访问，且不能是限定知识库范围的 key
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  CreateSpaceRequest  true  "空间"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}  "参数错误；错误码 2300 表示工作区没有 Embedding 模型"
// @Failure      403  {object}  map[string]interface{}  "无权绑定该知识库或无权新建知识库"
// @Security     Bearer
// @Router       /docs/spaces [post]
func (h *SpaceHandler) Create(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	var req CreateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.LegacyKnowledgeBaseID != nil {
		badRequest(c, legacyKnowledgeBaseField)
		return
	}
	view, err := h.svc.Create(c.Request.Context(), id, service.CreateSpaceInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, Icon: req.Icon,
		Visibility: model.SpaceVisibility(req.Visibility), DefaultRole: model.SpaceRole(req.DefaultRole),
		KnowledgeBase: req.KnowledgeBase.choice(), StorageBackendID: req.StorageBackendID, Settings: req.Settings,
	})
	if err != nil {
		fail(c, err)
		return
	}
	created(c, view)
}

// Get godoc
// @Summary      获取文档空间
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid} [get]
func (h *SpaceHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	sp, role, found := space(c)
	if !found {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), sp, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// GetBySlug godoc
// @Summary      按 slug 获取文档空间
// @Tags         在线文档
// @Produce      json
// @Param        slug  path  string  true  "空间 slug"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/by-slug/{slug} [get]
func (h *SpaceHandler) GetBySlug(c *gin.Context) { h.Get(c) }

// Update godoc
// @Summary      更新文档空间
// @Description  空间管理员可修改名称、slug、描述、图标、可见性与默认角色；可见性 private 时默认角色固定为 none
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string              true  "空间 ID"
// @Param        request  body  UpdateSpaceRequest  true  "变更字段"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid} [patch]
func (h *SpaceHandler) Update(c *gin.Context) {
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
	var req UpdateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	in := service.UpdateSpaceInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, Icon: req.Icon, Settings: req.Settings,
	}
	if req.Visibility != nil {
		v := model.SpaceVisibility(*req.Visibility)
		in.Visibility = &v
	}
	if req.DefaultRole != nil {
		r := model.SpaceRole(*req.DefaultRole)
		in.DefaultRole = &r
	}
	view, err := h.svc.Update(c.Request.Context(), id, sp, in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// Delete godoc
// @Summary      删除文档空间（移入回收站）
// @Tags         在线文档
// @Param        sid  path  string  true  "空间 ID"
// @Success      204
// @Security     Bearer
// @Router       /docs/spaces/{sid} [delete]
func (h *SpaceHandler) Delete(c *gin.Context) {
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
	if err := h.svc.Delete(c.Request.Context(), id, sp); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// Restore godoc
// @Summary      从回收站恢复文档空间
// @Description  仅工作区管理员可恢复
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/restore [post]
func (h *SpaceHandler) Restore(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id, found := identity(c)
	if !found {
		return
	}
	view, err := h.svc.Restore(c.Request.Context(), id, c.Param("sid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// ListMembers godoc
// @Summary      列出空间成员
// @Description  直接成员（用户与用户组），管理员优先，同一角色内用户组在前
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/members [get]
func (h *SpaceHandler) ListMembers(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	sp, _, found := space(c)
	if !found {
		return
	}
	views, err := h.svc.ListMembers(c.Request.Context(), sp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, views)
}

// SetMembers godoc
// @Summary      添加成员或修改成员角色
// @Description  幂等：已是成员的主体更新为新角色；空间至少保留一名管理员
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string                  true  "空间 ID"
// @Param        request  body  SetSpaceMembersRequest  true  "成员列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/members [put]
func (h *SpaceHandler) SetMembers(c *gin.Context) {
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
	var req SetSpaceMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	in := make([]service.MemberInput, 0, len(req.Members))
	for _, m := range req.Members {
		in = append(in, service.MemberInput{
			Type: model.PrincipalType(m.PrincipalType), ID: m.PrincipalID, Role: model.SpaceRole(m.Role),
		})
	}
	views, err := h.svc.SetMembers(c.Request.Context(), id, sp, in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, views)
}

// RemoveMember godoc
// @Summary      移除空间成员
// @Tags         在线文档
// @Param        sid    path  string  true  "空间 ID"
// @Param        ptype  path  string  true  "主体类型 user|group"
// @Param        pid    path  string  true  "主体 ID"
// @Success      204
// @Security     Bearer
// @Router       /docs/spaces/{sid}/members/{ptype}/{pid} [delete]
func (h *SpaceHandler) RemoveMember(c *gin.Context) {
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
	p := model.Principal{Type: model.PrincipalType(c.Param("ptype")), ID: c.Param("pid")}
	if err := h.svc.RemoveMember(c.Request.Context(), id, sp, p); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// BindKnowledgeBase godoc
// @Summary      绑定/解绑知识库与存储后端
// @Description  需要空间管理员。字段缺省表示不变；knowledge_base 取值同创建空间：
// @Description  none 解绑、existing 改绑已有知识库、create 新建同名知识库并绑定；
// @Description  storage_backend_id 为空字符串表示改回工作区默认。知识库变化后空间里的全部页面立即排队重新同步：
// @Description  解绑时已镜像的条目从原知识库删除，改绑时从原知识库删除并在新知识库重建
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string                    true  "空间 ID"
// @Param        request  body  BindKnowledgeBaseRequest  true  "绑定"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}  "参数错误；错误码 2300 表示工作区没有 Embedding 模型"
// @Failure      403  {object}  map[string]interface{}  "不是空间管理员，或无权绑定该知识库 / 新建知识库"
// @Security     Bearer
// @Router       /docs/spaces/{sid}/knowledge-base [put]
func (h *SpaceHandler) BindKnowledgeBase(c *gin.Context) {
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
	var req BindKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	if req.LegacyKnowledgeBaseID != nil {
		badRequest(c, legacyKnowledgeBaseField)
		return
	}
	view, err := h.svc.BindKnowledgeBase(c.Request.Context(), id, sp, req.KnowledgeBase.choice(), req.StorageBackendID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}
