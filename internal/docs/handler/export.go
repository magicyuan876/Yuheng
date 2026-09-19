package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Exporting pages and spaces.
//
// A page export is synchronous: the file is built and returned by the request
// that asked for it. A space export is not — it is a job, started here and
// polled at /docs/exports/{jid}, because a space is a thousand pages and a zip
// and neither belongs inside an HTTP timeout.

// ExportRequest is the body of an export request.
type ExportRequest struct {
	// Format is markdown (the default) or html.
	Format string `json:"format"`
}

// ExportPage godoc
// @Summary      导出单个页面
// @Description  把页面渲染成 Markdown 或 HTML 文件直接返回。
// @Description  导出的内容与你在页面上能看到的完全一致：指向你无权打开的页面的链接
// @Description  不会泄露对方标题，嵌入块按同样的权限规则展开，附件保留为链接
// @Tags         在线文档
// @Accept       json
// @Produce      octet-stream
// @Param        pid      path  string         true   "页面 ID"
// @Param        request  body  ExportRequest  false  "导出格式"
// @Success      200  {string}  string  "文件内容"
// @Security     Bearer
// @Router       /docs/pages/{pid}/export [post]
func (h *PageHandler) ExportPage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req ExportRequest
	// An empty body is a Markdown export, which is the common case and not
	// worth making the caller spell out.
	_ = c.ShouldBindJSON(&req)

	result, err := h.svc.ExportPage(c.Request.Context(), actor, d, req.Format)
	if err != nil {
		fail(c, err)
		return
	}
	serveDownload(c, result.FileName, result.MediaType, result.Content)
}

// ExportSpace godoc
// @Summary      导出整个空间
// @Description  异步任务：立即返回作业 ID，用 /docs/exports/{jid} 轮询进度。
// @Description  压缩包里只包含**发起人**当时能读的页面，因此只有发起人本人可以下载，
// @Description  并且会在一天后自动删除
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string         true   "空间 ID"
// @Param        request  body  ExportRequest  false  "导出格式"
// @Success      202  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/export [post]
func (h *PageHandler) ExportSpace(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, sp, role, found := spaceScope(c)
	if !found {
		return
	}
	var req ExportRequest
	_ = c.ShouldBindJSON(&req)

	job, err := h.svc.StartSpaceExport(c.Request.Context(), actor, sp, role, req.Format)
	if err != nil {
		fail(c, err)
		return
	}
	accepted(c, job)
}

// ExportJob godoc
// @Summary      查询导出作业
// @Description  ready 为 true 时可以调用下载接口。只有发起导出的人能看到自己的作业
// @Tags         在线文档
// @Produce      json
// @Param        jid  path  string  true  "作业 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/exports/{jid} [get]
func (h *PageHandler) ExportJob(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	job, err := h.svc.ExportJob(c.Request.Context(), actor, c.Param("jid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, job)
}

// DownloadExport godoc
// @Summary      下载导出的压缩包
// @Description  只有发起导出的人能下载，且作业过期后不再可用
// @Tags         在线文档
// @Produce      octet-stream
// @Param        jid  path  string  true  "作业 ID"
// @Success      200  {string}  string  "zip 文件"
// @Security     Bearer
// @Router       /docs/exports/{jid}/download [get]
func (h *PageHandler) DownloadExport(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	result, err := h.svc.DownloadExport(c.Request.Context(), actor, c.Param("jid"))
	if err != nil {
		fail(c, err)
		return
	}
	serveDownload(c, result.FileName, result.MediaType, result.Content)
}

// serveDownload writes a file back as an attachment.
//
// The file name is sent twice. A title can be Chinese, and a bare filename=
// parameter may only carry ASCII, so the RFC 5987 filename*= form carries the
// real name and the plain parameter carries a stripped one for clients that
// do not understand it. Without both, a Chinese title arrives as mojibake or
// as the URL's last segment.
func serveDownload(c *gin.Context, fileName, mediaType string, content []byte) {
	if fileName == "" {
		fileName = "download"
	}
	ascii := asciiFallbackName(fileName)
	c.Header("Content-Disposition",
		`attachment; filename="`+ascii+`"; filename*=UTF-8''`+url.PathEscape(fileName))
	// Nothing here is a document a browser should render in place, and an
	// exported HTML file is somebody's content: sniffing it would be a way to
	// run their markup on this origin.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, mediaType, content)
}

// asciiFallbackName keeps the characters a bare filename= parameter may hold.
func asciiFallbackName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r > 0x20 && r < 0x7f && r != '"' && r != '\\':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	// Trailing filler only: trimming the leading underscores off a wholly
	// non-ASCII title would leave a bare extension like ".md", which some
	// clients treat as a hidden file rather than a name.
	out := strings.TrimRight(b.String(), "_")
	if out == "" || strings.Trim(out, "._") == "" {
		return "download"
	}
	return out
}
