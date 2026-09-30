package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// knowledgeFindingRepository stores knowledge-health findings.
type knowledgeFindingRepository struct {
	db *gorm.DB
}

// NewKnowledgeFindingRepository returns the PostgreSQL finding store.
func NewKnowledgeFindingRepository(db *gorm.DB) interfaces.KnowledgeFindingRepository {
	return &knowledgeFindingRepository{db: db}
}

// findingFromLive is the FROM clause of every read: a finding together with
// the live documents it names, in the knowledge base it belongs to. A finding
// whose subject is gone, or whose related document is gone, falls out of the
// join, so no read ever shows a document that was deleted or moved away —
// even in the moment before the trigger on knowledges removes the row.
const findingFromLive = `
	FROM knowledge_findings f` + findingLiveJoins

// findingFromLiveWithBase is findingFromLive with the finding's knowledge base
// joined in as kb, for listings that span knowledge bases and name them. A
// finding of a deleted knowledge base is not listed.
const findingFromLiveWithBase = `
	FROM knowledge_findings f
	JOIN knowledge_bases kb ON kb.id = f.knowledge_base_id AND kb.deleted_at IS NULL` + findingLiveJoins

const findingLiveJoins = `
	JOIN knowledges s ON s.id = f.subject_knowledge_id AND s.deleted_at IS NULL
	                 AND s.knowledge_base_id = f.knowledge_base_id
	LEFT JOIN knowledges r ON r.id = f.related_knowledge_id AND r.deleted_at IS NULL
	                      AND r.knowledge_base_id = f.knowledge_base_id
	WHERE (f.related_knowledge_id IS NULL OR r.id IS NOT NULL)`

const findingSelectColumns = `SELECT f.*, s.title AS subject_title, COALESCE(r.title, '') AS related_title`

// findingSelectWithBase adds the knowledge base's name; it goes with
// findingFromLiveWithBase.
const findingSelectWithBase = findingSelectColumns + `, kb.name AS knowledge_base_name`

// findingOrder puts the most urgent first, then the strongest evidence.
const findingOrder = ` ORDER BY CASE f.severity WHEN 'error' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
	f.score DESC NULLS LAST, f.updated_at DESC, f.id`

// Reconcile implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) Reconcile(ctx context.Context, run types.FindingRun) error {
	if run.TenantID == 0 || run.KnowledgeBaseID == "" || run.KnowledgeID == "" {
		return errors.New("finding run needs a tenant, a knowledge base and a knowledge entry")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reported := make([]string, 0, len(run.Findings))
		for _, f := range run.Findings {
			if err := upsertFinding(tx, run, f); err != nil {
				return err
			}
			reported = append(reported, f.Fingerprint)
		}
		if err := resolveUnreported(tx, run, reported); err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO knowledge_finding_scans (knowledge_id, tenant_id, knowledge_base_id, scanned_at)
			SELECT ?, ?, ?, NOW()
			WHERE EXISTS (SELECT 1 FROM knowledges WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL)
			ON CONFLICT (knowledge_id) DO UPDATE
			   SET tenant_id = EXCLUDED.tenant_id, knowledge_base_id = EXCLUDED.knowledge_base_id,
			       scanned_at = EXCLUDED.scanned_at`,
			run.KnowledgeID, run.TenantID, run.KnowledgeBaseID, run.KnowledgeID, run.TenantID,
		).Error
	})
}

// upsertFinding writes one reported finding.
//
// The status a re-reported finding ends in:
//
//   - open stays open, with the new evidence;
//   - resolved reopens: the problem is back;
//   - dismissed stays dismissed while the evidence hash is the same — the
//     person dismissed exactly this — and reopens when it differs.
//
// The assignee is the runner's routing, except on a finding somebody assigned
// by hand (assigned_by set), which keeps the person they chose.
//
// The row is only written while both documents exist in the knowledge base:
// a document deleted while its check was running must not be brought back
// into the findings by the check's late write.
func upsertFinding(tx *gorm.DB, run types.FindingRun, f *types.KnowledgeFinding) error {
	if f.Fingerprint == "" || f.SubjectKnowledgeID == "" || f.Type == "" || f.Detector == "" {
		return fmt.Errorf("finding of detector %q lacks a fingerprint, subject or type", f.Detector)
	}
	if !types.ValidFindingSeverity(f.Severity) {
		return fmt.Errorf("finding of detector %q has severity %q", f.Detector, f.Severity)
	}
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	details, err := f.Details.Value()
	if err != nil {
		return fmt.Errorf("encoding the details of finding %s: %w", f.Fingerprint, err)
	}
	const keepDismissed = `knowledge_findings.status = 'dismissed'
		AND knowledge_findings.details->>'evidence_hash' IS NOT DISTINCT FROM EXCLUDED.details->>'evidence_hash'`
	stmt := `
		INSERT INTO knowledge_findings (id, tenant_id, knowledge_base_id, type, detector, severity, status,
			fingerprint, subject_knowledge_id, related_knowledge_id, score, details, assignee_id,
			created_at, updated_at)
		SELECT ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, ?::jsonb, ?, NOW(), NOW()
		WHERE EXISTS (SELECT 1 FROM knowledges WHERE id = ? AND tenant_id = ? AND knowledge_base_id = ?
		                AND deleted_at IS NULL)
		  AND (?::varchar IS NULL OR EXISTS (SELECT 1 FROM knowledges WHERE id = ? AND tenant_id = ?
		                AND knowledge_base_id = ? AND deleted_at IS NULL))
		ON CONFLICT (tenant_id, fingerprint) DO UPDATE SET
			knowledge_base_id = EXCLUDED.knowledge_base_id,
			type = EXCLUDED.type,
			detector = EXCLUDED.detector,
			severity = EXCLUDED.severity,
			subject_knowledge_id = EXCLUDED.subject_knowledge_id,
			related_knowledge_id = EXCLUDED.related_knowledge_id,
			score = EXCLUDED.score,
			details = EXCLUDED.details,
			updated_at = NOW(),
			status = CASE WHEN ` + keepDismissed + ` THEN 'dismissed' ELSE 'open' END,
			resolved_at = CASE WHEN ` + keepDismissed + ` THEN knowledge_findings.resolved_at END,
			resolved_by = CASE WHEN ` + keepDismissed + ` THEN knowledge_findings.resolved_by END,
			resolution = CASE WHEN ` + keepDismissed + ` THEN knowledge_findings.resolution END,
			assignee_id = CASE WHEN knowledge_findings.assigned_by IS NULL THEN EXCLUDED.assignee_id
			                   ELSE knowledge_findings.assignee_id END`
	return tx.Exec(stmt,
		f.ID, run.TenantID, run.KnowledgeBaseID, f.Type, f.Detector, f.Severity,
		f.Fingerprint, f.SubjectKnowledgeID, f.RelatedKnowledgeID, f.Score, details, f.AssigneeID,
		f.SubjectKnowledgeID, run.TenantID, run.KnowledgeBaseID,
		f.RelatedKnowledgeID, f.RelatedKnowledgeID, run.TenantID, run.KnowledgeBaseID,
	).Error
}

// resolveUnreported closes the open findings of the detectors that ran which
// name the checked entry and were not reported this time. A dismissed finding
// is left as it is: somebody decided about it, and should the problem come
// back unchanged it must not reappear as though nobody had.
func resolveUnreported(tx *gorm.DB, run types.FindingRun, reported []string) error {
	if len(run.Detectors) == 0 {
		return nil
	}
	query := tx.Model(&types.KnowledgeFinding{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND status = ?",
			run.TenantID, run.KnowledgeBaseID, types.FindingStatusOpen).
		Where("detector IN ?", run.Detectors).
		Where("(subject_knowledge_id = ? OR related_knowledge_id = ?)", run.KnowledgeID, run.KnowledgeID)
	if len(reported) > 0 {
		query = query.Where("fingerprint NOT IN ?", reported)
	}
	return query.Updates(map[string]any{
		"status":      types.FindingStatusResolved,
		"resolved_at": gorm.Expr("NOW()"),
		"resolved_by": types.FindingResolvedBySystem,
		"resolution":  types.FindingResolutionCleared,
		"updated_at":  gorm.Expr("NOW()"),
	}).Error
}

// List implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) List(ctx context.Context, tenantID uint64, kbID string,
	filter types.KnowledgeFindingFilter,
) ([]*types.KnowledgeFindingRow, int64, error) {
	where := []string{"f.tenant_id = ?", "f.knowledge_base_id = ?"}
	args := []any{tenantID, kbID}
	if filter.Status != "" && filter.Status != "all" {
		where = append(where, "f.status = ?")
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		where = append(where, "f.type = ?")
		args = append(args, filter.Type)
	}
	if filter.KnowledgeID != "" {
		where = append(where, "(f.subject_knowledge_id = ? OR f.related_knowledge_id = ?)")
		args = append(args, filter.KnowledgeID, filter.KnowledgeID)
	}
	if filter.AssigneeID != "" {
		where = append(where, "f.assignee_id = ?")
		args = append(args, filter.AssigneeID)
	}
	return r.page(ctx, findingSelectColumns, findingFromLive+" AND "+strings.Join(where, " AND "), args,
		filter.Page, filter.PageSize)
}

// ListAssigned implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) ListAssigned(ctx context.Context, tenantID uint64, assigneeID, status string,
	page, pageSize int,
) ([]*types.KnowledgeFindingRow, int64, error) {
	cond := findingFromLiveWithBase + " AND f.tenant_id = ? AND f.assignee_id = ?"
	args := []any{tenantID, assigneeID}
	if status != "" && status != "all" {
		cond += " AND f.status = ?"
		args = append(args, status)
	}
	return r.page(ctx, findingSelectWithBase, cond, args, page, pageSize)
}

// page counts the findings a condition selects and returns one page of them,
// most urgent first.
func (r *knowledgeFindingRepository) page(ctx context.Context, columns, cond string, args []any, page, size int,
) ([]*types.KnowledgeFindingRow, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) "+cond, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var rows []*types.KnowledgeFindingRow
	err := r.db.WithContext(ctx).
		Raw(columns+cond+findingOrder+" LIMIT ? OFFSET ?", append(args, size, (page-1)*size)...).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Get implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) Get(ctx context.Context, tenantID uint64, kbID, id string,
) (*types.KnowledgeFindingRow, error) {
	var rows []*types.KnowledgeFindingRow
	err := r.db.WithContext(ctx).
		Raw(findingSelectColumns+findingFromLive+" AND f.tenant_id = ? AND f.knowledge_base_id = ? AND f.id = ?",
			tenantID, kbID, id).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, interfaces.ErrFindingNotFound
	}
	return rows[0], nil
}

// SetStatus implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) SetStatus(ctx context.Context, tenantID uint64, kbID, id, status, actor,
	resolution string,
) (*types.KnowledgeFindingRow, error) {
	if _, err := r.Get(ctx, tenantID, kbID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"status": status, "updated_at": gorm.Expr("NOW()")}
	switch status {
	case types.FindingStatusOpen:
		updates["resolved_at"] = nil
		updates["resolved_by"] = nil
		updates["resolution"] = nil
	default:
		updates["resolved_at"] = gorm.Expr("NOW()")
		updates["resolved_by"] = actor
		updates["resolution"] = resolution
	}
	err := r.db.WithContext(ctx).Model(&types.KnowledgeFinding{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND id = ?", tenantID, kbID, id).
		Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tenantID, kbID, id)
}

// SetAssignee implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) SetAssignee(ctx context.Context, tenantID uint64, kbID, id, assigneeID,
	assignedBy string,
) (*types.KnowledgeFindingRow, error) {
	if _, err := r.Get(ctx, tenantID, kbID, id); err != nil {
		return nil, err
	}
	nullable := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	updates := map[string]any{"assigned_by": nullable(assignedBy), "updated_at": gorm.Expr("NOW()")}
	if assignedBy != "" {
		// Back to automatic leaves the current assignee until the next
		// check routes the finding again.
		updates["assignee_id"] = nullable(assigneeID)
	}
	err := r.db.WithContext(ctx).Model(&types.KnowledgeFinding{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND id = ?", tenantID, kbID, id).
		Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tenantID, kbID, id)
}

// CountOpenAssigned implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) CountOpenAssigned(ctx context.Context, tenantID uint64, assigneeID string,
) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Raw("SELECT COUNT(*)"+findingFromLiveWithBase+" AND f.tenant_id = ? AND f.assignee_id = ? AND f.status = ?",
			tenantID, assigneeID, types.FindingStatusOpen).
		Scan(&n).Error
	return n, err
}

// CountOpenByType implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) CountOpenByType(ctx context.Context, tenantID uint64, kbID string,
) (map[string]int64, error) {
	var rows []struct {
		Type  string
		Count int64
	}
	err := r.db.WithContext(ctx).
		Raw("SELECT f.type AS type, COUNT(*) AS count"+findingFromLive+
			" AND f.tenant_id = ? AND f.knowledge_base_id = ? AND f.status = ? GROUP BY f.type",
			tenantID, kbID, types.FindingStatusOpen).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Type] = row.Count
	}
	return out, nil
}

// LastScanAt implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) LastScanAt(ctx context.Context, tenantID uint64, kbID string,
) (*time.Time, error) {
	var at *time.Time
	err := r.db.WithContext(ctx).
		Raw(`SELECT MAX(scanned_at) FROM knowledge_finding_scans WHERE tenant_id = ? AND knowledge_base_id = ?`,
			tenantID, kbID).
		Scan(&at).Error
	if err != nil {
		return nil, err
	}
	return at, nil
}

// ListOpenForKnowledge implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) ListOpenForKnowledge(ctx context.Context, tenantID uint64,
	knowledgeID string,
) ([]*types.KnowledgeFindingRow, error) {
	var rows []*types.KnowledgeFindingRow
	err := r.db.WithContext(ctx).
		Raw(findingSelectColumns+findingFromLive+
			" AND f.tenant_id = ? AND f.status = ? AND (f.subject_knowledge_id = ? OR f.related_knowledge_id = ?)"+
			findingOrder,
			tenantID, types.FindingStatusOpen, knowledgeID, knowledgeID).
		Scan(&rows).Error
	return rows, err
}

// ListCheckableKnowledgeIDs implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) ListCheckableKnowledgeIDs(ctx context.Context, tenantID uint64,
	kbID string, limit int,
) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&types.Knowledge{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND parse_status IN ?", tenantID, kbID,
			[]string{types.ParseStatusCompleted, types.ParseStatusFinalizing}).
		Order("updated_at DESC").Order("id").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

// reviewDueSQL is true for an entry k of knowledge base kb that is past its
// review date at @now. It mirrors types.KnowledgeSteward.Overdue: the clock
// runs from the last review, or from creation for an entry reviewed before it
// existed (a page's content older than a re-created mirror) or never.
const reviewDueSQL = `kb.review_interval_days > 0
	AND GREATEST(COALESCE(k.reviewed_at, k.created_at), k.created_at)
	    + make_interval(days => kb.review_interval_days) <= @now`

// ListReviewChecksDue implements interfaces.KnowledgeFindingRepository.
//
// Two kinds of entry, whose review state changed without anybody touching
// them: one that has come due and carries no stale finding of this cycle
// (none open, and no dismissal made since it was last vouched for — a
// dismissal from an earlier cycle is reconsidered by the check); and one whose
// open stale finding no longer holds, because the period was shortened,
// lengthened or switched off. Entries still being indexed are left for their
// indexing, which checks them anyway.
func (r *knowledgeFindingRepository) ListReviewChecksDue(ctx context.Context, now time.Time, limit int,
) ([]types.KnowledgeFindingsPayload, error) {
	var rows []types.KnowledgeFindingsPayload
	err := r.db.WithContext(ctx).Raw(`
		SELECT tenant_id, knowledge_base_id, knowledge_id FROM (
			(SELECT k.tenant_id, k.knowledge_base_id, k.id AS knowledge_id,
			        GREATEST(COALESCE(k.reviewed_at, k.created_at), k.created_at) AS vouched
			   FROM knowledges k
			   JOIN knowledge_bases kb ON kb.id = k.knowledge_base_id AND kb.deleted_at IS NULL
			  WHERE k.deleted_at IS NULL AND k.parse_status IN @checkable AND `+reviewDueSQL+`
			    AND NOT EXISTS (
			        SELECT 1 FROM knowledge_findings f
			         WHERE f.subject_knowledge_id = k.id AND f.type = @stale
			           AND (f.status = 'open' OR (f.status = 'dismissed'
			                AND f.updated_at >= GREATEST(COALESCE(k.reviewed_at, k.created_at), k.created_at))))
			  ORDER BY vouched, k.id
			  LIMIT @limit)
			UNION ALL
			(SELECT f.tenant_id, f.knowledge_base_id, f.subject_knowledge_id AS knowledge_id, f.updated_at AS vouched
			   FROM knowledge_findings f
			   JOIN knowledges k ON k.id = f.subject_knowledge_id AND k.deleted_at IS NULL
			   JOIN knowledge_bases kb ON kb.id = k.knowledge_base_id AND kb.deleted_at IS NULL
			  WHERE f.type = @stale AND f.status = 'open' AND k.parse_status IN @checkable
			    AND NOT (`+reviewDueSQL+`)
			  ORDER BY f.updated_at, f.id
			  LIMIT @limit)
		) due
		LIMIT @limit`,
		map[string]any{
			"now": now, "limit": limit, "stale": types.FindingTypeStale,
			"checkable": []string{types.ParseStatusCompleted, types.ParseStatusFinalizing},
		}).Scan(&rows).Error
	return rows, err
}

// DeleteForKnowledge implements interfaces.KnowledgeFindingRepository.
func (r *knowledgeFindingRepository) DeleteForKnowledge(ctx context.Context, tenantID uint64,
	knowledgeID string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND (subject_knowledge_id = ? OR related_knowledge_id = ?)",
			tenantID, knowledgeID, knowledgeID).Delete(&types.KnowledgeFinding{}).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM knowledge_finding_scans WHERE tenant_id = ? AND knowledge_id = ?`,
			tenantID, knowledgeID).Error
	})
}
