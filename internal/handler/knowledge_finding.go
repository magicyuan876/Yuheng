package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// KnowledgeFindingHandler serves knowledge health: the problems found in a
// knowledge base by comparing its documents.
//
// Every route sits behind the KB-access guard, which rewrites the request's
// tenant to the base's owner (for a shared base), so the tenant read from the
// context below is always the one the findings are stored under.
type KnowledgeFindingHandler struct {
	svc interfaces.KnowledgeFindingService
}

// NewKnowledgeFindingHandler returns the handler.
func NewKnowledgeFindingHandler(svc interfaces.KnowledgeFindingService) *KnowledgeFindingHandler {
	return &KnowledgeFindingHandler{svc: svc}
}

// UpdateFindingRequest is the body of PATCH /knowledge-bases/{id}/findings/{finding_id}.
type UpdateFindingRequest struct {
	// Status is "dismissed" or "open".
	Status string `json:"status" binding:"required"`
}

// ListFindings godoc
// @Summary      列出知识库的健康问题
// @Description  列出自动检测到的问题（目前是重复文档）。status 默认 open，all 列出全部；knowledge_id 只看涉及该文档的问题
// @Tags         知识健康
// @Produce      json
// @Param        id            path   string  true   "知识库 ID"
// @Param        status        query  string  false  "open（默认）/ dismissed / resolved / all"
// @Param        type          query  string  false  "问题类型，如 duplicate"
// @Param        knowledge_id  query  string  false  "只列出涉及该文档的问题"
// @Param        page          query  int     false  "页码，默认 1"
// @Param        page_size     query  int     false  "每页数量，默认 20，最大 100"
// @Success      200  {object}  map[string]interface{}  "data: {items, total, page, page_size}"
// @Failure      400  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/findings [get]
func (h *KnowledgeFindingHandler) ListFindings(c *gin.Context) {
	ctx := c.Request.Context()
	page, pageSize, ok := parseListPagination(c)
	if !ok {
		return
	}
	result, err := h.svc.List(ctx, types.MustTenantIDFromContext(ctx), secutils.SanitizeForLog(c.Param("id")),
		types.KnowledgeFindingFilter{
			Status:      strings.TrimSpace(c.Query("status")),
			Type:        strings.TrimSpace(c.Query("type")),
			KnowledgeID: strings.TrimSpace(c.Query("knowledge_id")),
			Page:        page,
			PageSize:    pageSize,
		})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// FindingSummary godoc
// @Summary      知识库健康概览
// @Description  未处理问题总数及按类型计数、最近一次检测时间，以及本部署是否启用、这个知识库能否检测
// @Tags         知识健康
// @Produce      json
// @Param        id  path  string  true  "知识库 ID"
// @Success      200  {object}  map[string]interface{}  "data: {open_total, open_by_type, last_scan_at, ...}"
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/findings/summary [get]
func (h *KnowledgeFindingHandler) FindingSummary(c *gin.Context) {
	ctx := c.Request.Context()
	summary, err := h.svc.Summary(ctx, types.MustTenantIDFromContext(ctx), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// UpdateFinding godoc
// @Summary      忽略或重新打开一个问题
// @Description  status=dismissed 忽略（证据不变时不再出现），status=open 重新打开一个已忽略的问题。操作记入知识库动态
// @Tags         知识健康
// @Accept       json
// @Produce      json
// @Param        id          path  string                true  "知识库 ID"
// @Param        finding_id  path  string                true  "问题 ID"
// @Param        request     body  UpdateFindingRequest  true  "新状态"
// @Success      200  {object}  map[string]interface{}  "data: 更新后的问题"
// @Failure      400  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Failure      409  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/findings/{finding_id} [patch]
func (h *KnowledgeFindingHandler) UpdateFinding(c *gin.Context) {
	ctx := c.Request.Context()
	var req UpdateFindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数不合法").WithDetails(err.Error()))
		return
	}
	view, err := h.svc.UpdateStatus(ctx, types.MustTenantIDFromContext(ctx),
		secutils.SanitizeForLog(c.Param("id")), secutils.SanitizeForLog(c.Param("finding_id")),
		strings.TrimSpace(req.Status))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// ScanFindings godoc
// @Summary      重新检测整个知识库
// @Description  为知识库中所有已完成索引的文档安排一次健康检测（分批错开执行，已在排队的不重复安排），返回新安排的数量。仅知识库所属空间可发起
// @Tags         知识健康
// @Produce      json
// @Param        id  path  string  true  "知识库 ID"
// @Success      200  {object}  map[string]interface{}  "data: {queued}"
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/findings/scan [post]
func (h *KnowledgeFindingHandler) ScanFindings(c *gin.Context) {
	ctx := c.Request.Context()
	kbTenant := types.MustTenantIDFromContext(ctx)
	// The KB-access guard lets an organisation member with editor access
	// through to a shared base. Dismissing a finding is an edit like any
	// other; re-checking every document of somebody else's base is not a
	// sharee's to start. The caller's own tenant, which the guard leaves in
	// the gin keys, must be the owner.
	if own, ok := c.Get(types.TenantIDContextKey.String()); ok {
		if ownID, isID := own.(uint64); isID && ownID != kbTenant {
			_ = c.Error(apperrors.NewForbiddenError("only the workspace that owns the knowledge base can re-check it"))
			return
		}
	}
	kbID := secutils.SanitizeForLog(c.Param("id"))
	queued, err := h.svc.Scan(ctx, kbTenant, kbID)
	if err != nil {
		logger.Warnf(ctx, "[Findings] re-check of %s failed after %d scheduled: %v", kbID, queued, err)
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": types.KnowledgeFindingScanResult{Queued: queued}})
}
