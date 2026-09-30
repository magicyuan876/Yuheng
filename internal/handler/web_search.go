package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/types"
)

// WebSearchHandler serves the catalogue of web search engine types. The
// configured providers themselves live under WebSearchProviderHandler.
type WebSearchHandler struct{}

// NewWebSearchHandler creates a new web search handler
func NewWebSearchHandler() *WebSearchHandler {
	return &WebSearchHandler{}
}

// GetProviders returns the list of available web search provider types.
//
// GetProviders godoc
// @Summary      获取可用网络搜索 Provider 列表
// @Description  返回所有已注册的网络搜索 provider（含元数据）
// @Tags         网络搜索
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "provider 列表"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /web-search/providers [get]
func (h *WebSearchHandler) GetProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.GetWebSearchProviderTypes(),
	})
}
