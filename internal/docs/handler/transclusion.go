package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// BlockRefRequest is one block reference to resolve.
type BlockRefRequest struct {
	SourcePageID  string `json:"source_page_id"  binding:"required"`
	SourceBlockID string `json:"source_block_id" binding:"required"`
}

// ResolveTransclusionsRequest is the body of POST /docs/block-refs/resolve.
//
// A POST for the same reason the title lookup is one: the list is as long as
// the open page has references, which does not belong in a URL. It reads and
// changes nothing, and is declared with the read capability accordingly.
type ResolveTransclusionsRequest struct {
	Refs []BlockRefRequest `json:"refs" binding:"required"`
}

// ResolveTransclusions godoc
// @Summary      批量解析块引用
// @Description  把 (页面 ID, 块 ID) 解析成该块当前的内容，供编辑器和阅读页渲染块引用。
// @Description  块已删除或调用者无权查看源页面时返回 missing，两种情况不作区分；
// @Description  引用已登记但源页面尚未保存过时返回 pending。
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        request  body  ResolveTransclusionsRequest  true  "块引用列表"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/block-refs/resolve [post]
func (h *PageHandler) ResolveTransclusions(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	var req ResolveTransclusionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	refs := make([]repository.BlockRef, 0, len(req.Refs))
	for _, ref := range req.Refs {
		refs = append(refs, repository.BlockRef{PageID: ref.SourcePageID, BlockID: ref.SourceBlockID})
	}
	rows, err := h.svc.ResolveTransclusions(c.Request.Context(), actor, refs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, rows)
}
