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
	// Reason is required to dismiss: "distinct_scope" (the documents apply
	// to different things) or "intentional" (the overlap is wanted).
	Reason string `json:"reason"`
}

// AssignFindingRequest is the body of PUT /knowledge-bases/{id}/findings/{finding_id}/assignee.
type AssignFindingRequest struct {
	// AssigneeID is the person to take the finding to; empty hands it back
	// to the automatic routing.
	AssigneeID string `json:"assignee_id"`
}

// ListFindings godoc
// @Summary      列出知识库的健康问题
// @Description  列出自动检测到的问题（重复、内容有出入等）。status 默认 open，all 列出全部；knowledge_id 只看涉及该文档的问题
// @Tags         知识健康
// @Produce      json
// @Param        id            path   string  true   "知识库 ID"
// @Param        status        query  string  false  "open（默认）/ dismissed / resolved / all"
// @Param        type          query  string  false  "问题类型，如 duplicate"
// @Param        knowledge_id  query  string  false  "只列出涉及该文档的问题"
// @Param        assignee      query  string  false  "me：只列出派给我的问题"
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
			Mine:        strings.TrimSpace(c.Query("assignee")) == "me",
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
// @Description  status=dismissed 忽略（证据不变时不再出现），须给出原因 reason：
// @Description  distinct_scope（适用范围不同）或 intentional（有意保留）；
// @Description  status=open 重新打开一个已忽略的问题。操作记入知识库动态
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
		strings.TrimSpace(req.Status), strings.TrimSpace(req.Reason))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// AssignFinding godoc
// @Summary      指派问题的处理人
// @Description  把问题交给指定成员处理（之后的自动检测不再改派）；assignee_id 为空表示交还自动派发，并立即重新派发。处理人须能编辑该知识库，涉及在线文档的问题也可以是文档空间的在职成员。操作记入知识库动态
// @Tags         知识健康
// @Accept       json
// @Produce      json
// @Param        id          path  string                true  "知识库 ID"
// @Param        finding_id  path  string                true  "问题 ID"
// @Param        request     body  AssignFindingRequest  true  "处理人"
// @Success      200  {object}  map[string]interface{}  "data: 更新后的问题"
// @Failure      400  {object}  apperrors.AppError
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /knowledge-bases/{id}/findings/{finding_id}/assignee [put]
func (h *KnowledgeFindingHandler) AssignFinding(c *gin.Context) {
	ctx := c.Request.Context()
	var req AssignFindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数不合法").WithDetails(err.Error()))
		return
	}
	view, err := h.svc.Assign(ctx, types.MustTenantIDFromContext(ctx),
		secutils.SanitizeForLog(c.Param("id")), secutils.SanitizeForLog(c.Param("finding_id")),
		strings.TrimSpace(req.AssigneeID))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// SupersedeFinding godoc
// @Summary      以一份取代另一份
// @Description  处理“内容重复”或“内容有出入”的问题：保留 keep_knowledge_id 指定的文档，另一份退出知识库。
// @Description  上传的文档会被删除；在线文档页面会被排除出知识库并标记为已被取代，页面本身仍可阅读（需要能编辑该页面）；
// @Description  数据源同步的文档无法在这里移除（下次同步会回来），返回 409。操作记入知识库动态
// @Tags         知识健康
// @Accept       json
// @Produce      json
// @Param        id          path  string                         true  "知识库 ID"
// @Param        finding_id  path  string                         true  "问题 ID"
// @Param        request     body  types.SupersedeFindingRequest  true  "保留哪一份"
// @Success      200  {object}  map[string]interface{}  "data: {retired_knowledge_id, how}"
// @Failure      400  {object}  apperrors.AppError
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Failure      409  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /knowledge-bases/{id}/findings/{finding_id}/supersede [post]
func (h *KnowledgeFindingHandler) SupersedeFinding(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.SupersedeFindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("请求参数不合法").WithDetails(err.Error()))
		return
	}
	result, err := h.svc.Supersede(ctx, types.MustTenantIDFromContext(ctx),
		secutils.SanitizeForLog(c.Param("id")), secutils.SanitizeForLog(c.Param("finding_id")),
		strings.TrimSpace(req.KeepKnowledgeID))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ListAssignedFindings godoc
// @Summary      我的知识待办
// @Description  当前工作区里派给我处理的知识健康问题，跨知识库，附知识库名称。status 默认 open，all 列出全部
// @Tags         知识健康
// @Produce      json
// @Param        status     query  string  false  "open（默认）/ dismissed / resolved / all"
// @Param        page       query  int     false  "页码，默认 1"
// @Param        page_size  query  int     false  "每页数量，默认 20，最大 100"
// @Success      200  {object}  map[string]interface{}  "data: {items, total, page, page_size}"
// @Failure      400  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /findings/assigned [get]
func (h *KnowledgeFindingHandler) ListAssignedFindings(c *gin.Context) {
	ctx := c.Request.Context()
	page, pageSize, ok := parseListPagination(c)
	if !ok {
		return
	}
	result, err := h.svc.ListAssigned(ctx, types.MustTenantIDFromContext(ctx), strings.TrimSpace(c.Query("status")),
		page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// CountAssignedFindings godoc
// @Summary      我的待办数量
// @Description  当前工作区里派给我、尚未处理的知识健康问题数量，用于导航角标
// @Tags         知识健康
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "data: {open_total}"
// @Security     Bearer
// @Router       /findings/assigned/count [get]
func (h *KnowledgeFindingHandler) CountAssignedFindings(c *gin.Context) {
	ctx := c.Request.Context()
	n, err := h.svc.CountAssigned(ctx, types.MustTenantIDFromContext(ctx))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"open_total": n}})
}

// ScanFindings godoc
// @Summary      重新检测整个知识库
// @Description  为知识库中所有已完成索引的文档安排一次健康检测（分批错开执行，已在排队的不重复安排），返回新安排的数量。仅知识库所属工作区可发起
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
	tenantID := types.MustTenantIDFromContext(ctx)
	kbID := secutils.SanitizeForLog(c.Param("id"))
	queued, err := h.svc.Scan(ctx, tenantID, kbID)
	if err != nil {
		logger.Warnf(ctx, "[Findings] re-check of %s failed after %d scheduled: %v", kbID, queued, err)
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": types.KnowledgeFindingScanResult{Queued: queued}})
}
