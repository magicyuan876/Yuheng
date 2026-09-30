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
// reopening, assigning, the findings assigned to the caller, and a full
// re-check. The detection itself lives in the findings package; this is what
// people do with its results.

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
	repo     interfaces.KnowledgeFindingRepository
	kbs      interfaces.KnowledgeBaseRepository
	trigger  interfaces.KnowledgeFindingsTrigger
	runner   *findings.Runner
	audit    interfaces.AuditLogService
	stewards interfaces.KnowledgeStewardshipRepository
	users    interfaces.UserRepository
	enabled  bool
}

// NewKnowledgeFindingService returns the finding API service.
func NewKnowledgeFindingService(
	repo interfaces.KnowledgeFindingRepository,
	kbs interfaces.KnowledgeBaseRepository,
	trigger interfaces.KnowledgeFindingsTrigger,
	runner *findings.Runner,
	audit interfaces.AuditLogService,
	stewards interfaces.KnowledgeStewardshipRepository,
	users interfaces.UserRepository,
	cfg *config.Config,
) interfaces.KnowledgeFindingService {
	return &knowledgeFindingService{
		repo: repo, kbs: kbs, trigger: trigger, runner: runner, audit: audit, stewards: stewards, users: users,
		enabled: cfg != nil && cfg.Findings.IsEnabled(),
	}
}

// List implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) List(ctx context.Context, tenantID uint64, kbID string,
	filter types.KnowledgeFindingFilter,
) (*types.KnowledgeFindingPage, error) {
	if err := validListStatus(&filter.Status); err != nil {
		return nil, err
	}
	filter.AssigneeID = ""
	if filter.Mine {
		me, ok := types.UserIDFromContext(ctx)
		if !ok || types.IsSyntheticUserID(me) {
			return &types.KnowledgeFindingPage{
				Items: []*types.KnowledgeFindingView{}, Page: filter.Page, PageSize: filter.PageSize,
			}, nil
		}
		filter.AssigneeID = me
	}
	rows, total, err := s.repo.List(ctx, tenantID, kbID, filter)
	if err != nil {
		return nil, err
	}
	items, err := s.views(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &types.KnowledgeFindingPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// ListAssigned implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) ListAssigned(ctx context.Context, tenantID uint64, status string, page,
	pageSize int,
) (*types.KnowledgeFindingPage, error) {
	if err := validListStatus(&status); err != nil {
		return nil, err
	}
	me, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(me) {
		return &types.KnowledgeFindingPage{Items: []*types.KnowledgeFindingView{}, Page: page, PageSize: pageSize}, nil
	}
	rows, total, err := s.repo.ListAssigned(ctx, tenantID, me, status, page, pageSize)
	if err != nil {
		return nil, err
	}
	items, err := s.views(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &types.KnowledgeFindingPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// CountAssigned implements interfaces.KnowledgeFindingService.
func (s *knowledgeFindingService) CountAssigned(ctx context.Context, tenantID uint64) (int64, error) {
	me, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(me) {
		return 0, nil
	}
	return s.repo.CountOpenAssigned(ctx, tenantID, me)
}

// views renders rows for the API, with their assignees named.
func (s *knowledgeFindingService) views(ctx context.Context, rows []*types.KnowledgeFindingRow,
) ([]*types.KnowledgeFindingView, error) {
	items := make([]*types.KnowledgeFindingView, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.View())
	}
	if err := s.nameAssignees(ctx, items...); err != nil {
		return nil, err
	}
	return items, nil
}

// nameAssignees replaces the bare IDs of assignees with their names. Whether
// an assignee can still be asked is the routing's business, checked when they
// were chosen; here a user who no longer exists shows as inactive.
func (s *knowledgeFindingService) nameAssignees(ctx context.Context, views ...*types.KnowledgeFindingView) error {
	if s.users == nil {
		return nil
	}
	ids := map[string]bool{}
	for _, v := range views {
		if v.Assignee != nil {
			ids[v.Assignee.ID] = true
		}
	}
	people, err := namePeople(ctx, s.users, ids)
	if err != nil {
		return err
	}
	for _, v := range views {
		if v.Assignee != nil {
			v.Assignee = people[v.Assignee.ID]
		}
	}
	return nil
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

// validListStatus defaults an empty status filter to open and refuses unknown
// ones.
func validListStatus(status *string) error {
	switch *status {
	case "":
		*status = types.FindingStatusOpen
	case types.FindingStatusOpen, types.FindingStatusDismissed, types.FindingStatusResolved, "all":
	default:
		return werrors.NewValidationError("status must be one of open, dismissed, resolved, all")
	}
	return nil
}

// UpdateStatus implements interfaces.KnowledgeFindingService.
//
// Only two moves are a person's to make: dismissing an open finding, with the
// reason, and reopening a dismissed one. Resolved means the problem went away,
// which the detectors decide; "reopening" it by hand would be undone by the
// next check.
func (s *knowledgeFindingService) UpdateStatus(ctx context.Context, tenantID uint64, kbID, findingID,
	status, reason string,
) (*types.KnowledgeFindingView, error) {
	switch status {
	case types.FindingStatusDismissed:
		if !types.ValidDismissReason(reason) {
			return nil, werrors.NewValidationError("dismissing a finding needs a reason: distinct_scope or intentional")
		}
	case types.FindingStatusOpen:
		reason = ""
	default:
		return nil, werrors.NewValidationError("status must be dismissed or open")
	}
	current, err := s.finding(ctx, tenantID, kbID, findingID)
	if err != nil {
		return nil, err
	}
	if current.Status == status {
		return s.view(ctx, current)
	}
	allowed := (status == types.FindingStatusDismissed && current.Status == types.FindingStatusOpen) ||
		(status == types.FindingStatusOpen && current.Status == types.FindingStatusDismissed)
	if !allowed {
		return nil, werrors.NewConflictError("a " + current.Status + " finding cannot be set to " + status)
	}
	actor, _ := types.UserIDFromContext(ctx)
	updated, err := s.repo.SetStatus(ctx, tenantID, kbID, findingID, status, actor, reason)
	if errors.Is(err, interfaces.ErrFindingNotFound) {
		return nil, werrors.NewNotFoundError("finding not found")
	}
	if err != nil {
		return nil, err
	}
	details := findingActivityDetails(current)
	details["from"], details["to"] = current.Status, status
	if reason != "" {
		details["reason"] = reason
	}
	recordKBActivity(ctx, s.audit, tenantID, kbID, types.AuditActionFindingStatusChanged,
		"finding", findingID, types.AuditOutcomeSuccess, details)
	return s.view(ctx, updated)
}

// Assign implements interfaces.KnowledgeFindingService.
//
// The person must be able to act on the finding: edit the knowledge base, or,
// for a finding that involves a docs page, at least be an active member of
// the workspace — a page is changed by its writers, whom the docs module and
// not the knowledge base knows. An empty assigneeID hands the finding back to
// the automatic routing, which a check applies at once.
func (s *knowledgeFindingService) Assign(ctx context.Context, tenantID uint64, kbID, findingID, assigneeID string,
) (*types.KnowledgeFindingView, error) {
	current, err := s.finding(ctx, tenantID, kbID, findingID)
	if err != nil {
		return nil, err
	}
	actor, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(actor) {
		return nil, werrors.NewForbiddenError("assigning a finding needs a signed-in person")
	}
	if assigneeID != "" {
		if err := s.checkAssignee(ctx, tenantID, current, assigneeID); err != nil {
			return nil, err
		}
	}
	assignedBy := ""
	if assigneeID != "" {
		assignedBy = actor
	}
	updated, err := s.repo.SetAssignee(ctx, tenantID, kbID, findingID, assigneeID, assignedBy)
	if errors.Is(err, interfaces.ErrFindingNotFound) {
		return nil, werrors.NewNotFoundError("finding not found")
	}
	if err != nil {
		return nil, err
	}
	details := findingActivityDetails(current)
	details["assignee_id"] = assigneeID
	recordKBActivity(ctx, s.audit, tenantID, kbID, types.AuditActionFindingAssigned,
		"finding", findingID, types.AuditOutcomeSuccess, details)
	if assigneeID == "" && s.trigger != nil {
		if err := s.trigger.TriggerKnowledgeFindings(ctx, tenantID, kbID, current.SubjectKnowledgeID); err != nil {
			logger.Warnf(ctx, "[Findings] re-routing finding %s failed: %v", findingID, err)
		}
	}
	return s.view(ctx, updated)
}

func (s *knowledgeFindingService) checkAssignee(ctx context.Context, tenantID uint64,
	f *types.KnowledgeFindingRow, assigneeID string,
) error {
	if s.stewards == nil {
		return werrors.NewInternalServerError("assignment is not available")
	}
	ok, err := s.stewards.CanMaintain(ctx, tenantID, f.KnowledgeBaseID, assigneeID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	ids := []string{f.SubjectKnowledgeID}
	if f.RelatedKnowledgeID != nil {
		ids = append(ids, *f.RelatedKnowledgeID)
	}
	stewards, err := s.stewards.Stewards(ctx, tenantID, ids)
	if err != nil {
		return err
	}
	involvesPage := false
	for _, st := range stewards {
		involvesPage = involvesPage || st.Origin == types.KnowledgeOriginDocs
	}
	if involvesPage {
		member, err := s.stewards.IsActiveMember(ctx, tenantID, assigneeID)
		if err != nil || member {
			return err
		}
	}
	return werrors.NewValidationError("the assignee must be an active member who can act on these documents")
}

// finding loads one finding, as a not-found error when it does not exist.
func (s *knowledgeFindingService) finding(ctx context.Context, tenantID uint64, kbID, findingID string,
) (*types.KnowledgeFindingRow, error) {
	current, err := s.repo.Get(ctx, tenantID, kbID, findingID)
	if errors.Is(err, interfaces.ErrFindingNotFound) {
		return nil, werrors.NewNotFoundError("finding not found")
	}
	return current, err
}

func (s *knowledgeFindingService) view(ctx context.Context, row *types.KnowledgeFindingRow,
) (*types.KnowledgeFindingView, error) {
	v := row.View()
	if err := s.nameAssignees(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// findingActivityDetails is what the knowledge base's activity records about
// a finding a person acted on.
func findingActivityDetails(f *types.KnowledgeFindingRow) map[string]any {
	details := map[string]any{"finding_type": f.Type, "subject_knowledge_id": f.SubjectKnowledgeID}
	titles := []string{f.SubjectTitle}
	if f.RelatedKnowledgeID != nil {
		details["related_knowledge_id"] = *f.RelatedKnowledgeID
		titles = append(titles, f.RelatedTitle)
	}
	kbActivityAppendSampleTitles(details, titles...)
	return details
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
	out, err := s.views(ctx, rows)
	if err != nil {
		return nil, err
	}
	for i, v := range out {
		out[i] = orientTowards(v, knowledgeID)
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
