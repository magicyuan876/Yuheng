package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// Search godoc
// @Summary      搜索文档
// @Description  同时搜索页面（标题与正文）、**评论**，以及页面通过块引用显示的文本
// @Description  结果一律按调用者的权限过滤；块引用文本按**来源页**的权限过滤——
// @Description  否则搜索就成了通过引用页阅读无权页面的侧信道
// @Description  匹配用子串而不是全文索引：`simple` 分词会把整句中文当成一个词，
// @Description  那样的全文索引对中文文档静默返回空结果
// @Tags         在线文档
// @Produce      json
// @Param        q      query  string  true   "搜索词"
// @Param        space  query  string  false  "只搜这一个空间"
// @Param        limit  query  int     false  "结果条数上限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/search [get]
func (h *PageHandler) Search(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	results, err := h.svc.Search(c.Request.Context(), actor, c.Query("q"), c.Query("space"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, results)
}
