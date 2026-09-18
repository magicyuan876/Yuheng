package handler

import (
	"github.com/gin-gonic/gin"
)

// ResolveEmbedRequest is the body of POST /docs/embeds/resolve.
type ResolveEmbedRequest struct {
	URL string `json:"url" binding:"required"`
}

// ResolveEmbed godoc
// @Summary      解析可嵌入地址
// @Description  校验粘贴的地址是否在本部署的嵌入白名单内，返回要放进 iframe 的地址；不在白名单内返回 400。会尽力抓取标题（经 SSRF 校验，只访问服务商自己的 oEmbed 端点）
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  ResolveEmbedRequest  true  "地址"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/embeds/resolve [post]
func (h *PageHandler) ResolveEmbed(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	var req ResolveEmbedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	view, err := h.svc.ResolveEmbed(c.Request.Context(), req.URL)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, view)
}

// EmbedPolicy godoc
// @Summary      查询嵌入策略
// @Description  返回本部署允许的嵌入服务商列表，以及自建 draw.io 地址（未配置则为空，此时不能新建或编辑 draw.io 图）
// @Tags         在线文档
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/embeds/policy [get]
func (h *PageHandler) EmbedPolicy(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	ok(c, h.svc.EmbedPolicy())
}
