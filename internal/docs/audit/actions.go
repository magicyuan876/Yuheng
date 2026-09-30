// Package audit names the docs module's audit actions and writes them through
// Yuheng's audit log service. Body edits are not audited row by row (history
// snapshots carry them); everything that changes who can see what, or
// creates/destroys structure, is.
package audit

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Actions recorded by the docs module. Namespaced "docs." per the audit
// log's convention.
const (
	SpaceCreated       types.AuditAction = "docs.space.created"
	SpaceUpdated       types.AuditAction = "docs.space.updated"
	SpaceDeleted       types.AuditAction = "docs.space.deleted"
	SpaceRestored      types.AuditAction = "docs.space.restored"
	SpaceMemberAdded   types.AuditAction = "docs.space.member_added"
	SpaceMemberRemoved types.AuditAction = "docs.space.member_removed"
	// SpaceMemberRoleChanged: an existing membership was re-granted with a
	// different role.
	SpaceMemberRoleChanged types.AuditAction = "docs.space.member_role_changed"
	SpaceKBBound           types.AuditAction = "docs.space.kb_bound"
	SpaceKBUnbound         types.AuditAction = "docs.space.kb_unbound"

	GroupCreated        types.AuditAction = "docs.group.created"
	GroupUpdated        types.AuditAction = "docs.group.updated"
	GroupDeleted        types.AuditAction = "docs.group.deleted"
	GroupMembersChanged types.AuditAction = "docs.group.members_changed"
	GroupMemberAdded    types.AuditAction = "docs.group.member_added"
	GroupMemberRemoved  types.AuditAction = "docs.group.member_removed"

	PageCreated           types.AuditAction = "docs.page.created"
	PageDuplicated        types.AuditAction = "docs.page.duplicated"
	PageMoved             types.AuditAction = "docs.page.moved"
	PageDeleted           types.AuditAction = "docs.page.deleted"
	PageRestored          types.AuditAction = "docs.page.restored"
	PagePurged            types.AuditAction = "docs.page.purged"
	PageLocked            types.AuditAction = "docs.page.locked"
	PageUnlocked          types.AuditAction = "docs.page.unlocked"
	PageKnowledgeExcluded types.AuditAction = "docs.page.knowledge_excluded"
	PageKnowledgeIncluded types.AuditAction = "docs.page.knowledge_included"
	PageOwnerChanged      types.AuditAction = "docs.page.owner_changed"
	PageReplaced          types.AuditAction = "docs.page.content_replaced"
	PageRestoredTo        types.AuditAction = "docs.page.revision_restored"

	PageAccessRestricted types.AuditAction = "docs.page.access.restricted"
	PageAccessInherited  types.AuditAction = "docs.page.access.inherited"
	PageGrantAdded       types.AuditAction = "docs.page.grant_added"
	PageGrantRemoved     types.AuditAction = "docs.page.grant_removed"

	ShareCreated  types.AuditAction = "docs.share.created"
	ShareUpdated  types.AuditAction = "docs.share.updated"
	ShareRevoked  types.AuditAction = "docs.share.revoked"
	ShareAccessed types.AuditAction = "docs.share.accessed"

	AttachmentUploaded types.AuditAction = "docs.attachment.uploaded"
	AttachmentDeleted  types.AuditAction = "docs.attachment.deleted"

	CommentDeleted types.AuditAction = "docs.comment.deleted"
	Exported       types.AuditAction = "docs.exported"
	Imported       types.AuditAction = "docs.imported"
	TemplateSaved  types.AuditAction = "docs.template.saved"
	TemplateDelete types.AuditAction = "docs.template.deleted"
)

// Scope and target type names used in audit rows.
const (
	ScopeSpace       = "docs_space"
	TargetPage       = "docs_page"
	TargetSpace      = "docs_space"
	TargetGroup      = "tenant_group"
	TargetShare      = "docs_share"
	TargetAttachment = "docs_attachment"
	TargetComment    = "docs_comment"
	TargetTemplate   = "docs_template"
)

// Sink is the slice of the audit log service the recorder needs.
// interfaces.AuditLogService satisfies it.
type Sink interface {
	Log(ctx context.Context, entry *types.AuditLog) error
}

// Recorder writes audit rows; nil-safe so a deployment without an audit
// service keeps working.
type Recorder struct {
	svc Sink
}

// NewRecorder wraps the sink (nil allowed). A typed-nil service is treated
// as absent so an optional DI dependency needs no special casing.
func NewRecorder(svc Sink) *Recorder {
	if isNilSink(svc) {
		return &Recorder{}
	}
	return &Recorder{svc: svc}
}

func isNilSink(s Sink) bool {
	if s == nil {
		return true
	}
	if svc, ok := s.(interfaces.AuditLogService); ok && svc == nil {
		return true
	}
	return false
}

// Entry is what a service knows about an audited action.
type Entry struct {
	TenantID     uint64
	ActorUserID  string
	ActorRole    string
	Action       types.AuditAction
	SpaceID      string // scope
	TargetType   string
	TargetID     string
	TargetUserID string
}

// Record writes one row. Failures are logged, never returned: an audit
// outage must not fail the user's request, but it must be visible.
func (r *Recorder) Record(ctx context.Context, e Entry) {
	if r == nil || r.svc == nil {
		return
	}
	row := &types.AuditLog{
		TenantID:     e.TenantID,
		ActorUserID:  e.ActorUserID,
		ActorRole:    e.ActorRole,
		Action:       e.Action,
		ScopeType:    ScopeSpace,
		ScopeID:      e.SpaceID,
		TargetType:   e.TargetType,
		TargetID:     e.TargetID,
		TargetUserID: e.TargetUserID,
	}
	if e.SpaceID == "" {
		row.ScopeType = ""
	}
	if err := r.svc.Log(ctx, row); err != nil {
		logger.Errorf(ctx, "[docs.audit] failed to record %s: %v", e.Action, err)
	}
}

// RecordRequest is Record with the actor and request path taken from the gin
// context set by the auth middleware.
func (r *Recorder) RecordRequest(c *gin.Context, e Entry) {
	if r == nil || r.svc == nil {
		return
	}
	ctx := c.Request.Context()
	if e.ActorUserID == "" {
		e.ActorUserID, _ = types.UserIDFromContext(ctx)
	}
	if e.ActorRole == "" {
		e.ActorRole = string(types.TenantRoleFromContext(ctx))
	}
	if e.TenantID == 0 {
		e.TenantID, _ = types.TenantIDFromContext(ctx)
	}
	row := &types.AuditLog{
		TenantID:      e.TenantID,
		ActorUserID:   e.ActorUserID,
		ActorRole:     e.ActorRole,
		Action:        e.Action,
		ScopeType:     ScopeSpace,
		ScopeID:       e.SpaceID,
		TargetType:    e.TargetType,
		TargetID:      e.TargetID,
		TargetUserID:  e.TargetUserID,
		RequestPath:   c.FullPath(),
		RequestMethod: c.Request.Method,
	}
	if e.SpaceID == "" {
		row.ScopeType = ""
	}
	if err := r.svc.Log(ctx, row); err != nil {
		logger.Errorf(ctx, "[docs.audit] failed to record %s: %v", e.Action, err)
	}
}
