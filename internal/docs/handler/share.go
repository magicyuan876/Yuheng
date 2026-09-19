package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Public links, from both sides.
//
// The owner's routes sit under the page, like every other page-scoped thing
// in this module. The visitor's routes are different in kind: they are
// registered before the authentication middleware and are reached with no
// session at all, so they take a key rather than a page id and they return a
// type that was built for the purpose. See service/share.go for the rules.

// ShareRequest is the body of POST /docs/pages/:pid/shares.
type ShareRequest struct {
	IncludeChildren  bool       `json:"include_children"`
	AllowSearchIndex bool       `json:"allow_search_index"`
	Password         string     `json:"password,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// ShareUpdateRequest is the body of PATCH /docs/pages/:pid/shares/:shid.
// Absent fields are left alone, which is why they are pointers.
type ShareUpdateRequest struct {
	IncludeChildren  *bool      `json:"include_children,omitempty"`
	AllowSearchIndex *bool      `json:"allow_search_index,omitempty"`
	Password         *string    `json:"password,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ClearExpiry      bool       `json:"clear_expiry,omitempty"`
}

// UnlockRequest is the body of POST /docs/public/:key/unlock.
type UnlockRequest struct {
	Password string `json:"password"`
}

// Shares godoc
// @Summary      列出页面的公开链接
// @Description  能阅读页面即可查看：「这页已经发布到公网」是每个读者都该知道的事
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/shares [get]
func (h *PageHandler) Shares(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	rows, err := h.svc.Shares(c.Request.Context(), actor, d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}

// CreateShare godoc
// @Summary      发布公开链接
// @Description  受限页面不能发布；需要部署开启 YUHENG_DOCS_PUBLIC_SHARING
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  ShareRequest  true  "链接设置"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/shares [post]
func (h *PageHandler) CreateShare(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.CreateShare(c.Request.Context(), actor, d, service.CreateShareInput{
		IncludeChildren: req.IncludeChildren, AllowSearchIndex: req.AllowSearchIndex,
		Password: req.Password, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// UpdateShare godoc
// @Summary      修改公开链接的设置
// @Description  password 传空字符串表示取消密码；改密码会让已解锁的浏览器重新输入
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string              true  "页面 ID"
// @Param        shid     path  string              true  "链接 ID"
// @Param        request  body  ShareUpdateRequest  true  "要修改的设置"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/shares/{shid} [patch]
func (h *PageHandler) UpdateShare(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req ShareUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.UpdateShare(c.Request.Context(), actor, d, c.Param("shid"),
		service.UpdateShareInput{
			IncludeChildren: req.IncludeChildren, AllowSearchIndex: req.AllowSearchIndex,
			Password: req.Password, ExpiresAt: req.ExpiresAt, ClearExpiry: req.ClearExpiry,
		})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// RevokeShare godoc
// @Summary      停用公开链接
// @Description  停用是永久的：同一个 key 不会再指向任何页面
// @Tags         在线文档
// @Produce      json
// @Param        pid   path  string  true  "页面 ID"
// @Param        shid  path  string  true  "链接 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/shares/{shid} [delete]
func (h *PageHandler) RevokeShare(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	if err := h.svc.RevokeShare(c.Request.Context(), actor, d, c.Param("shid")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"revoked": true})
}

// ---- the visitor's side --------------------------------------------------------

// unlockHeader carries an unlock token from the browser. A header rather than
// a query parameter, so the token does not end up in access logs, referrers
// or somebody's pasted URL.
const unlockHeader = "X-Docs-Share-Unlock"

// PublicPage godoc
// @Summary      访问公开链接（无需登录）
// @Description  返回渲染后的文档。链接失效、过期、被停用或页面已受限时返回 state 而不是内容
// @Description  带密码的链接先返回 state=password，解锁后用 X-Docs-Share-Unlock 头再请求
// @Tags         在线文档
// @Produce      json
// @Param        key   path   string  true   "链接 key"
// @Param        page  query  string  false  "子树中某页的 short_id，缺省为链接根页"
// @Success      200  {object}  map[string]interface{}
// @Router       /docs/public/{key} [get]
func (h *PageHandler) PublicPage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	res, err := h.svc.ResolveShare(c.Request.Context(), c.Param("key"),
		c.GetHeader(unlockHeader), c.Query("page"))
	if err != nil {
		fail(c, err)
		return
	}
	// Always 200, never 404 or 410: the client renders a different thing for
	// each outcome (ask for a password, say the link expired, say it was
	// turned off), and an error status would turn that into a fetch failure
	// with nothing to show.
	//
	// Robots are told what to do on every response, including the ones that
	// carry no content: a "this link expired" page indexed under the
	// document's name would outlive the document.
	if res.Page != nil && res.Page.AllowSearchIndex {
		c.Header("X-Robots-Tag", "index, follow")
	} else {
		c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	}
	// Nothing here may be cached by a shared cache: the same URL answers
	// differently once a password is given, and once a link is revoked.
	c.Header("Cache-Control", "private, no-store")
	ok(c, res)
}

// UnlockPublicPage godoc
// @Summary      用密码解锁公开链接（无需登录）
// @Description  成功返回 unlock_token，客户端存起来并在后续请求用 X-Docs-Share-Unlock 头带上
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        key      path  string         true  "链接 key"
// @Param        request  body  UnlockRequest  true  "密码"
// @Success      200  {object}  map[string]interface{}
// @Router       /docs/public/{key}/unlock [post]
func (h *PageHandler) UnlockPublicPage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	var req UnlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	res, err := h.svc.UnlockShare(c.Request.Context(), c.Param("key"), req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	c.Header("Cache-Control", "private, no-store")
	ok(c, res)
}

// PublicSpace godoc
// @Summary      访问公开空间（无需登录）
// @Description  返回空间信息与顶层页面。空间必须是 public，且部署开启了公开分享
// @Description  用空间 ID 而不是 slug 寻址：slug 只在租户内唯一，而访客没有租户
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Router       /docs/public-spaces/{sid} [get]
func (h *PageHandler) PublicSpace(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	view, err := h.svc.PublicSpace(c.Request.Context(), c.Param("sid"))
	if err != nil {
		fail(c, err)
		return
	}
	// A public space is meant to be found, so robots are welcome here.
	c.Header("X-Robots-Tag", "index, follow")
	ok(c, view)
}

// PublicSpacePage godoc
// @Summary      访问公开空间里的一页（无需登录）
// @Description  受限页面与回收站里的页面一律返回 404，和私有页面无从区分
// @Tags         在线文档
// @Produce      json
// @Param        sid    path  string  true  "空间 ID"
// @Param        short  path  string  true  "页面 short_id"
// @Success      200  {object}  map[string]interface{}
// @Router       /docs/public-spaces/{sid}/pages/{short} [get]
func (h *PageHandler) PublicSpacePage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	view, err := h.svc.PublicSpacePage(c.Request.Context(), c.Param("sid"), c.Param("short"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("X-Robots-Tag", "index, follow")
	ok(c, view)
}
