package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/attachment"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// AttachmentHandler serves uploads and downloads.
//
// The download route is the one place in the module where bytes a user
// supplied are handed back to a browser, so the response headers are as much
// of the feature as the permission check: a wrong Content-Type here would turn
// an uploaded file into same-origin code.
type AttachmentHandler struct{ svc *service.AttachmentService }

func (h *AttachmentHandler) ready(c *gin.Context) bool {
	if h == nil || h.svc == nil {
		NotImplemented(c)
		return false
	}
	return true
}

// Upload godoc
// @Summary      上传附件
// @Description  向空间上传一个文件（multipart，字段名 file）；可选 page_id 直接绑定到页面。按内容嗅探类型、SVG 去脚本、按 sha256 去重并计入租户配额
// @Tags         在线文档
// @Accept       multipart/form-data
// @Produce      json
// @Param        sid      path      string  true   "空间 ID"
// @Param        file     formData  file    true   "文件"
// @Param        page_id  formData  string  false  "页面 ID"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/attachments [post]
func (h *AttachmentHandler) Upload(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	sp, role, found := space(c)
	if !found {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "a file is required in the 'file' field: "+err.Error())
		return
	}
	view, err := h.svc.Upload(c.Request.Context(), actor, sp, role, service.UploadInput{
		File: file, PageID: c.PostForm("page_id"),
	})
	if err != nil {
		fail(c, err)
		return
	}
	created(c, view)
}

// Download godoc
// @Summary      读取附件
// @Description  校验调用者对所属页面的读权限后返回文件内容；w 可取 320/800/1600 获取图片的较小渲染
// @Tags         在线文档
// @Produce      octet-stream
// @Param        aid  path   string  true   "附件 ID"
// @Param        w    query  int     false  "图片宽度（320/800/1600）"
// @Success      200  {file}  file
// @Security     Bearer
// @Router       /docs/attachments/{aid} [get]
func (h *AttachmentHandler) Download(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	width, _ := strconv.Atoi(c.Query("w"))
	res, err := h.svc.Fetch(c.Request.Context(), actor, c.Param("aid"), width)
	if err != nil {
		fail(c, err)
		return
	}
	// Private: the response is permission-checked, so no shared cache may
	// keep it. The browser may, which is what makes an image in a long page
	// cheap to scroll past twice.
	serveAttachment(c, res, "private, max-age=3600")
}

// serveAttachment writes an authorised attachment: a redirect to the storage
// URL, a rendered variant, or the proxied bytes, under the headers that keep
// user content from being treated as the site's own.
func serveAttachment(c *gin.Context, res *service.ServeResult, cacheControl string) {
	defer func() { _ = res.Close() }()

	if res.Redirect != "" {
		// The signed URL is short-lived and unguessable; the permission check
		// before this is what decided the caller may have it at all.
		c.Header("Cache-Control", cacheControl)
		c.Redirect(http.StatusFound, res.Redirect)
		return
	}

	writeAttachmentHeaders(c, res, cacheControl)
	if res.Data != nil {
		c.Data(http.StatusOK, res.ContentType, res.Data)
		return
	}
	if res.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(res.Size, 10))
	}
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, res.Body); err != nil {
		logger.Warnf(c.Request.Context(), "[docs] streaming attachment %s failed: %v", c.Param("aid"), err)
	}
}

// writeAttachmentHeaders applies the rules that keep user content from being
// treated as the site's own: the sniffed type is never trusted by the browser,
// anything renderable is sandboxed, and everything else is a download.
func writeAttachmentHeaders(c *gin.Context, res *service.ServeResult, cacheControl string) {
	c.Header("Content-Type", res.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	disposition := "attachment"
	if res.Inline {
		disposition = "inline"
	}
	c.Header("Content-Disposition", attachment.ContentDisposition(disposition, res.FileName))
	if res.Sandbox {
		c.Header("Content-Security-Policy", attachment.SandboxPolicy)
	}
	c.Header("Cache-Control", cacheControl)
}

// Delete godoc
// @Summary      删除附件
// @Description  删除附件记录；底层对象在最后一条引用它的记录消失时才真正删除（去重共享）
// @Tags         在线文档
// @Produce      json
// @Param        aid  path  string  true  "附件 ID"
// @Success      204
// @Security     Bearer
// @Router       /docs/attachments/{aid} [delete]
func (h *AttachmentHandler) Delete(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), actor, c.Param("aid")); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

// ListForPage godoc
// @Summary      列出页面附件
// @Tags         在线文档
// @Produce      json
// @Param        pid  path  string  true  "页面 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/attachments [get]
func (h *AttachmentHandler) ListForPage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	d, found := decision(c)
	if !found {
		return
	}
	rows, err := h.svc.ListForPage(c.Request.Context(), d)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}
