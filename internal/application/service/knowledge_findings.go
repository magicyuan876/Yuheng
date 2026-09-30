package service

import (
	"context"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/service/findings"
	"github.com/magicyuan876/yuheng/internal/config"
	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The API over knowledge-health findings: listing, the summary, dismissing and
// reopening, and a full re-check. The detection itself lives in the findings
// package; this is what people do with its results.

const (
	// findingScanMaxKnowledge bounds one full re-check. A larger base is
	// checked by its most recently changed documents; the rest are checked
	// as they change.
	findingScanMaxKnowledge = 5000
	// findingScanStagger spreads a full re-check out, so a large base does
	// not become thousands of checks starting in the same second — on the
	// Redis-less executor, thousands of goroutines hitting the database at
	// once.
	findingScanStagger = 100 * time.Millisecond
)

type knowledgeFindingService struct {
	repo    interfaces.KnowledgeFindingRepository
	kbs     interfaces.KnowledgeBaseRepository
	trigger interfaces.KnowledgeFindingsTrigger
	runner  *findings.Runner
	audit   interfaces.AuditLogService
	enabled bool
}

// NewKnowledgeFindingService returns the finding API service.
func NewKnowledgeFindingService(
	repo interfaces.KnowledgeFindingRepository,
	kbs interfaces.KnowledgeBaseRepository,
	trigger interfaces.KnowledgeFindingsTrigger,
	runner *findings.Runner,
	audit interfaces.AuditLogService,
	cfg *config.Config,
) interfaces.KnowledgeFindingService {
	return &knowledgeFindingService{
		repo: repo, kbs: kbs, trigger: trigger, runner: runner, audit: audit,
		enabled: cfg != nil && cfg.Findings.IsEnabled(),
	}
}

// List implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) List(ctx context.Context, tenantID uint64, kbID string,
	filter types.KnowledgeFindingFilter,
) (*types.KnowledgeFindingPage, error) {
	switch filter.Status {
	case "":
		filter.Status = types.FindingStatusOpen
	case types.FindingStatusOpen, types.FindingStatusDismissed, types.FindingStatusResolved, "all":
	default:
		return nil, werrors.NewValidationError("status must be one of open, dismissed, resolved, all")
	}
	rows, total, err := s.repo.List(ctx, tenantID, kbID, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*types.KnowledgeFindingView, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.View())
	}
	return &types.KnowledgeFindingPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// Summary implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) Summary(ctx context.Context, tenantID uint64, kbID string,
) (*types.KnowledgeFindingSummary, error) {
	kb, err := s.knowledgeBase(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	byType, err := s.repo.CountOpenByType(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, n := range byType {
		total += n
	}
	last, err := s.repo.LastScanAt(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	return &types.KnowledgeFindingSummary{
		OpenTotal: total, OpenByType: byType, LastScanAt: last,
		Enabled: s.enabled, Supported: s.runner != nil && s.runner.Supports(ctx, kb),
	}, nil
}

// UpdateStatus implements interfaces.KnowledgeFindingService.
//
// Only two moves are a person's to make: dismissing an open finding and
// reopening a dismissed one. Resolved means the problem went away, which the
// detectors decide; "reopening" it by hand would be undone by the next check.
func (s *knowledgeFindingService) UpdateStatus(ctx context.Context, tenantID uint64, kbID, findingID,
	status string,
) (*types.KnowledgeFindingView, error) {
	if status != types.FindingStatusDismissed && status != types.FindingStatusOpen {
		return nil, werrors.NewValidationError("status must be dismissed or open")
	}
	current, err := s.repo.Get(ctx, tenantID, kbID, findingID)
	if errors.Is(err, interfaces.ErrFindingNotFound) {
		return nil, werrors.NewNotFoundError("finding not found")
	}
	if err != nil {
		return nil, err
	}
	if current.Status == status {
		return current.View(), nil
	}
	allowed := (status == types.FindingStatusDismissed && current.Status == types.FindingStatusOpen) ||
		(status == types.FindingStatusOpen && current.Status == types.FindingStatusDismissed)
	if !allowed {
		return nil, werrors.NewConflictError("a " + current.Status + " finding cannot be set to " + status)
	}
	actor, _ := types.UserIDFromContext(ctx)
	updated, err := s.repo.SetStatus(ctx, tenantID, kbID, findingID, status, actor)
	if errors.Is(err, interfaces.ErrFindingNotFound) {
		return nil, werrors.NewNotFoundError("finding not found")
	}
	if err != nil {
		return nil, err
	}
	details := map[string]any{
		"from": current.Status, "to": status, "finding_type": current.Type,
		"subject_knowledge_id": current.SubjectKnowledgeID,
	}
	titles := []string{current.SubjectTitle}
	if current.RelatedKnowledgeID != nil {
		details["related_knowledge_id"] = *current.RelatedKnowledgeID
		titles = append(titles, current.RelatedTitle)
	}
	kbActivityAppendSampleTitles(details, titles...)
	recordKBActivity(ctx, s.audit, tenantID, kbID, types.AuditActionFindingStatusChanged,
		"finding", findingID, types.AuditOutcomeSuccess, details)
	return updated.View(), nil
}

// Scan implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) Scan(ctx context.Context, tenantID uint64, kbID string) (int, error) {
	kb, err := s.knowledgeBase(ctx, tenantID, kbID)
	if err != nil {
		return 0, err
	}
	if !s.enabled || s.trigger == nil || s.runner == nil || !s.runner.Supports(ctx, kb) {
		return 0, nil
	}
	ids, err := s.repo.ListCheckableKnowledgeIDs(ctx, tenantID, kbID, findingScanMaxKnowledge)
	if err != nil {
		return 0, err
	}
	queued := 0
	for i, id := range ids {
		ok, err := s.trigger.ScheduleKnowledgeFindings(ctx, tenantID, kbID, id, time.Duration(i)*findingScanStagger)
		if err != nil {
			// What was scheduled stays scheduled; the caller learns the
			// re-check is incomplete and may ask again.
			logger.Warnf(ctx, "[Findings] scheduling the re-check of %s stopped after %d of %d: %v",
				kbID, queued, len(ids), err)
			return queued, err
		}
		if ok {
			queued++
		}
	}
	recordKBActivity(ctx, s.audit, tenantID, kbID, types.AuditActionFindingScanRequested,
		"knowledge_base", kbID, types.AuditOutcomeAccepted, map[string]any{"queued": queued, "count": len(ids)})
	return queued, nil
}

// OpenForKnowledge implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) OpenForKnowledge(ctx context.Context, tenantID uint64, knowledgeID string,
) ([]*types.KnowledgeFindingView, error) {
	rows, err := s.repo.ListOpenForKnowledge(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	out := make([]*types.KnowledgeFindingView, 0, len(rows))
	for _, row := range rows {
		out = append(out, orientTowards(row.View(), knowledgeID))
	}
	return out, nil
}

// orientTowards makes the entry asked about the subject of the view, so a
// reader of that entry sees "this document" on the left and "the other" on
// the right whichever way the finding is stored.
func orientTowards(v *types.KnowledgeFindingView, knowledgeID string) *types.KnowledgeFindingView {
	if v.Subject.KnowledgeID == knowledgeID || v.Related == nil || v.Related.KnowledgeID != knowledgeID {
		return v
	}
	subject := *v.Related
	related := v.Subject
	v.Subject, v.Related = subject, &related
	for i, e := range v.Evidence {
		v.Evidence[i] = e.Flipped()
	}
	return v
}

func (s *knowledgeFindingService) knowledgeBase(ctx context.Context, tenantID uint64, kbID string,
) (*types.KnowledgeBase, error) {
	kb, err := s.kbs.GetKnowledgeBaseByIDAndTenant(ctx, kbID, tenantID)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
		return nil, werrors.NewNotFoundError("knowledge base not found")
	}
	if err != nil {
		return nil, err
	}
	return kb, nil
}
