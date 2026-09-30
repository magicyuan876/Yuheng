package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// MessageFeedbackHandler serves feedback on answers. The session must be the
// caller's, which the service checks.
type MessageFeedbackHandler struct {
	svc interfaces.MessageFeedbackService
}

// NewMessageFeedbackHandler returns the handler.
func NewMessageFeedbackHandler(svc interfaces.MessageFeedbackService) *MessageFeedbackHandler {
	return &MessageFeedbackHandler{svc: svc}
}

// ListFeedback godoc
// @Summary      我对会话中回答的反馈
// @Description  当前用户对该会话各条回答的反馈（有帮助 / 没帮助及意见），以消息 ID 为键
// @Tags         会话
// @Produce      json
// @Param        id  path  string  true  "会话 ID"
// @Success      200  {object}  map[string]interface{}  "data: {message_id: feedback}"
// @Failure      403  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /sessions/{id}/feedback [get]
func (h *MessageFeedbackHandler) ListFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	out, err := h.svc.List(ctx, secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

// SetFeedback godoc
// @Summary      评价一条回答
// @Description  rating=up（有帮助）/ down（没帮助）/ 空（撤回）。“没帮助”会以“回答被反馈有误”出现在回答所引用文档的知识健康里，
// @Description  交给文档负责人，知识库成员可见：显示意见和回答开头，share_question=true 时附上提问。只对本空间的文档生效。需要登录用户
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        id          path  string                           true  "会话 ID"
// @Param        message_id  path  string                           true  "回答的消息 ID"
// @Param        request     body  types.SetMessageFeedbackRequest  true  "评价"
// @Success      200  {object}  map[string]interface{}  "data: 反馈，撤回时为 null"
// @Failure      400  {object}  apperrors.AppError
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /sessions/{id}/messages/{message_id}/feedback [put]
func (h *MessageFeedbackHandler) SetFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.SetMessageFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数不合法").WithDetails(err.Error()))
		return
	}
	view, err := h.svc.Set(ctx, secutils.SanitizeForLog(c.Param("id")),
		secutils.SanitizeForLog(c.Param("message_id")), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}
