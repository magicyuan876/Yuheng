package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Importing a bundle of Markdown into a space.
//
// Asynchronous, so this is two endpoints: one that accepts the upload and
// returns a job, one that reports on it. The upload is multipart rather than
// JSON because the thing being sent is a file, and base64 in a JSON body
// would make a 200MB archive into a 270MB request for no gain.

// StartImport godoc
// @Summary      导入 Markdown
// @Description  上传一个 .md 文件或 .zip 压缩包（multipart，字段名 file），按目录结构创建页面树。
// @Description  异步任务：立即返回作业 ID，用 /docs/imports/{jid} 轮询。
// @Description  压缩包内部的相对链接会还原成页面链接，图片会存为附件；
// @Description  单个文件失败不会让整次导入失败，结果里会逐条列出跳过的原因
// @Tags         在线文档
// @Accept       multipart/form-data
// @Produce      json
// @Param        sid        path      string  true   "空间 ID"
// @Param        file       formData  file    true   "Markdown 文件或 zip 压缩包"
// @Param        parent_id  formData  string  false  "导入到这个页面下面，不填则导入到空间根部"
// @Success      202  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/imports [post]
func (h *PageHandler) StartImport(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, sp, role, found := spaceScope(c)
	if !found {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "a file is required in the 'file' field: "+err.Error())
		return
	}
	job, err := h.svc.StartImport(c.Request.Context(), actor, sp, role, service.ImportInput{
		File: file, ParentID: c.PostForm("parent_id"),
	})
	if err != nil {
		fail(c, err)
		return
	}
	accepted(c, job)
}

// ImportJob godoc
// @Summary      查询导入作业
// @Description  done 为 true 时任务已结束。skipped 列出没能导入的文件和原因
// @Tags         在线文档
// @Produce      json
// @Param        jid  path  string  true  "作业 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/imports/{jid} [get]
func (h *PageHandler) ImportJob(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	job, err := h.svc.ImportJob(c.Request.Context(), actor, c.Param("jid"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, job)
}
