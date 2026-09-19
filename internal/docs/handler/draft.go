package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// DraftRequest is the body of POST /docs/pages/:pid/draft.
type DraftRequest struct {
	// Instruction says what the draft should cover.
	Instruction string `json:"instruction" binding:"required"`
	// KnowledgeIDs are the entries to draw on. They must belong to the
	// knowledge base this page's space is bound to.
	KnowledgeIDs []string `json:"knowledge_ids"`
}

// DraftPage godoc
// @Summary      用知识库内容起草页面
// @Description  把选定的知识条目交给知识库配置的模型，生成页面正文并写入
// @Description  **来源必须由调用方指定**：自动检索出来的内容是使用者无法核对的内容，
// @Description  而且会把「这些条目谁能看」的判断复制到本模块里
// @Description  只能引用**本空间所绑定的那个知识库**里的条目
// @Description  写入走与其他写入相同的 replace 路径，所以会先留一条历史版本，可以回退
// @Description  返回 source_ids 并记录在页面上：生成的内容要能追溯来源
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        pid      path  string        true  "页面 ID"
// @Param        request  body  DraftRequest  true  "指令与来源条目"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/pages/{pid}/draft [post]
func (h *PageHandler) DraftPage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, d, found := pageScope(c)
	if !found {
		return
	}
	var req DraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	result, err := h.svc.DraftPageFromKnowledge(c.Request.Context(), actor, d, service.DraftInput{
		Instruction:  req.Instruction,
		KnowledgeIDs: req.KnowledgeIDs,
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, result)
}
