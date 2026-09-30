package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// KnowledgeStewardshipHandler serves who maintains a knowledge entry, and
// confirming an entry is still right.
//
// The routes sit behind the knowledge-ID access guard, which rewrites the
// request's tenant to the one owning the entry's knowledge base, so the tenant
// read from the context is the one the entry is stored under.
type KnowledgeStewardshipHandler struct {
	svc interfaces.KnowledgeStewardshipService
}

// NewKnowledgeStewardshipHandler returns the handler.
func NewKnowledgeStewardshipHandler(svc interfaces.KnowledgeStewardshipService) *KnowledgeStewardshipHandler {
	return &KnowledgeStewardshipHandler{svc: svc}
}

// GetStewardship godoc
// @Summary      文档的负责人与复核状态
// @Description  负责人、最近一次有人确认或修改的时间和人、知识库的复核周期、下次复核时间以及是否已超期
// @Tags         知识健康
// @Produce      json
// @Param        id  path  string  true  "知识 ID"
// @Success      200  {object}  map[string]interface{}  "data: 负责人与复核状态"
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge/{id}/stewardship [get]
func (h *KnowledgeStewardshipHandler) GetStewardship(c *gin.Context) {
	ctx := c.Request.Context()
	view, err := h.svc.Get(ctx, types.MustTenantIDFromContext(ctx), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// SetOwner godoc
// @Summary      转交文档负责人
// @Description  把文档交给另一位能编辑该知识库的在职成员维护，owner_id 为空表示不设负责人。文档页镜像的负责人在页面上修改。操作记入知识库动态
// @Tags         知识健康
// @Accept       json
// @Produce      json
// @Param        id       path  string                          true  "知识 ID"
// @Param        request  body  types.SetKnowledgeOwnerRequest  true  "新负责人"
// @Success      200  {object}  map[string]interface{}  "data: 负责人与复核状态"
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Failure      409  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge/{id}/owner [put]
func (h *KnowledgeStewardshipHandler) SetOwner(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.SetKnowledgeOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数不合法").WithDetails(err.Error()))
		return
	}
	view, err := h.svc.SetOwner(ctx, types.MustTenantIDFromContext(ctx), secutils.SanitizeForLog(c.Param("id")),
		strings.TrimSpace(req.OwnerID))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// ConfirmReviewed godoc
// @Summary      确认文档仍然有效
// @Description  记录当前用户确认文档内容无误，重新开始复核计时，并关闭“需要复核”“回答被反馈有误”这类问题。需要登录用户，API Key 不能确认。操作记入知识库动态
// @Tags         知识健康
// @Produce      json
// @Param        id  path  string  true  "知识 ID"
// @Success      200  {object}  map[string]interface{}  "data: 负责人与复核状态"
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /knowledge/{id}/review [post]
func (h *KnowledgeStewardshipHandler) ConfirmReviewed(c *gin.Context) {
	ctx := c.Request.Context()
	view, err := h.svc.ConfirmReviewed(ctx, types.MustTenantIDFromContext(ctx), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}
