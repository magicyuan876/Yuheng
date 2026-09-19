package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Space storage quotas and the manual maintenance sweeps.

// QuotaRequest is the body of PUT /docs/spaces/:sid/quota.
type QuotaRequest struct {
	// QuotaBytes 0 means unlimited.
	QuotaBytes int64 `json:"quota_bytes"`
}

// SpaceUsage godoc
// @Summary      空间存储用量与配额
// @Description  任何空间成员都能查看：看不到为什么上传被拒，只会重试然后去问人
// @Description  quota_bytes 为 0 表示不限；from_default 表示该数字来自部署默认值而非本空间
// @Tags         在线文档
// @Produce      json
// @Param        sid  path  string  true  "空间 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/usage [get]
func (h *SpaceHandler) SpaceUsage(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	sp, role, found := space(c)
	if !found {
		return
	}
	usage, err := h.svc.SpaceUsage(c.Request.Context(), actor, sp, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, usage)
}

// SetSpaceQuota godoc
// @Summary      设置空间存储配额
// @Description  需要**工作区**管理员：空间管理员能给自己提额的配额不是配额
// @Description  把配额设到低于当前用量是允许的，它只是阻止继续增长，不会删除任何数据
// @Tags         在线文档
// @Accept       json
// @Produce      json
// @Param        sid      path  string        true  "空间 ID"
// @Param        request  body  QuotaRequest  true  "字节数，0 表示不限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/quota [put]
func (h *SpaceHandler) SetSpaceQuota(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	sp, _, found := space(c)
	if !found {
		return
	}
	var req QuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid request body: "+err.Error())
		return
	}
	usage, err := h.svc.SetSpaceQuota(c.Request.Context(), actor, sp, req.QuotaBytes)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, usage)
}

// sweepOptions reads the shared query parameters of a maintenance run.
func sweepOptions(c *gin.Context) (dryRun bool, limit int) {
	// Dry run is the default. A maintenance endpoint whose bare form deletes
	// things is one somebody will fire by accident while exploring.
	dryRun = c.Query("dry_run") != "false"
	limit, _ = strconv.Atoi(c.Query("limit"))
	return dryRun, limit
}

// SweepOrphans godoc
// @Summary      清理无人引用的附件（维护）
// @Description  只清理超过宽限期（24 小时）且没有绑定到任何页面的附件
// @Description  **默认是试运行**：要真的删除必须显式传 dry_run=false
// @Tags         在线文档
// @Produce      json
// @Param        dry_run  query  string  false  "传 false 才真正删除，默认只报告"
// @Param        limit    query  int     false  "本次处理条数上限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/maintenance/orphan-attachments [post]
func (h *PageHandler) SweepOrphans(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	dryRun, limit := sweepOptions(c)
	report, err := h.files.SweepOrphanAttachments(c.Request.Context(),
		service.SweepOptions{DryRun: dryRun, Limit: limit})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, report)
}

// SweepTrash godoc
// @Summary      清理超过保留期的回收站页面（维护）
// @Description  保留期由 YUHENG_DOCS_TRASH_RETENTION_DAYS 决定；走与手动清空回收站
// @Description  完全相同的清理路径，附件、历史、评论、分享链接一并释放
// @Description  **默认是试运行**：要真的删除必须显式传 dry_run=false
// @Tags         在线文档
// @Produce      json
// @Param        dry_run  query  string  false  "传 false 才真正删除，默认只报告"
// @Param        limit    query  int     false  "本次处理条数上限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/maintenance/expired-trash [post]
func (h *PageHandler) SweepTrash(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	dryRun, limit := sweepOptions(c)
	report, err := h.svc.SweepExpiredTrash(c.Request.Context(), h.retention,
		service.SweepOptions{DryRun: dryRun, Limit: limit})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, report)
}

// RebuildIndex godoc
// @Summary      重建空间的知识库索引
// @Description  把空间里符合条件的页面重新写入绑定的知识库。需要空间管理员：
// @Description  重建会产生向量化开销，并改变所有人的检索结果
// @Description  **受限页面永远不会入库**（检索没有逐条权限过滤），草稿与回收站页面同理；
// @Description  每页返回被跳过的原因
// @Description  分页返回，用 after 传上一次的 next_cursor 续做
// @Tags         在线文档
// @Produce      json
// @Param        sid    path   string  true   "空间 ID"
// @Param        after  query  string  false  "上一次返回的 next_cursor"
// @Param        limit  query  int     false  "本次处理条数上限"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /docs/spaces/{sid}/reindex [post]
func (h *PageHandler) RebuildIndex(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	actor, found := identity(c)
	if !found {
		return
	}
	sp, role, found := space(c)
	if !found {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	results, next, err := h.svc.SyncSpaceToKnowledge(c.Request.Context(), actor, sp, role,
		c.Query("after"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"results": results, "next_cursor": next})
}
