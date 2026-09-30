package service

import (
	"context"
	"errors"
	"time"

	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Stewardship: who maintains a knowledge entry, and confirming an entry is
// still right. See types/knowledge_stewardship.go for the model.
//
// Both kinds of change schedule a knowledge-health check of the entry. The
// check is what routes the entry's problems to people, so a transfer re-routes
// its open problems to the new owner, and a review settles the problems that
// only asked for one (an overdue review, answers people disputed).

type knowledgeStewardshipService struct {
	repo    interfaces.KnowledgeStewardshipRepository
	users   interfaces.UserRepository
	trigger interfaces.KnowledgeFindingsTrigger
	audit   interfaces.AuditLogService
	now     func() time.Time
}

// NewKnowledgeStewardshipService returns the stewardship service.
func NewKnowledgeStewardshipService(
	repo interfaces.KnowledgeStewardshipRepository,
	users interfaces.UserRepository,
	trigger interfaces.KnowledgeFindingsTrigger,
	audit interfaces.AuditLogService,
) interfaces.KnowledgeStewardshipService {
	return &knowledgeStewardshipService{repo: repo, users: users, trigger: trigger, audit: audit, now: time.Now}
}

// Get implements interfaces.KnowledgeStewardshipService.
func (s *knowledgeStewardshipService) Get(ctx context.Context, tenantID uint64, knowledgeID string,
) (*types.KnowledgeStewardshipView, error) {
	steward, err := s.steward(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, steward)
}

// SetOwner implements interfaces.KnowledgeStewardshipService.
func (s *knowledgeStewardshipService) SetOwner(ctx context.Context, tenantID uint64, knowledgeID, ownerID string,
) (*types.KnowledgeStewardshipView, error) {
	steward, err := s.steward(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	if steward.Origin == types.KnowledgeOriginDocs {
		return nil, werrors.NewConflictError("this entry mirrors a docs page; its owner is changed on the page")
	}
	if ownerID != "" {
		ok, err := s.repo.CanMaintain(ctx, tenantID, steward.KnowledgeBaseID, ownerID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, werrors.NewValidationError(
				"the new owner must be an active member who can edit this knowledge base")
		}
	}
	if ownerID != steward.OwnerID {
		if err := s.repo.SetOwner(ctx, tenantID, knowledgeID, ownerID); err != nil {
			return nil, s.notFound(err)
		}
		recordKBActivity(ctx, s.audit, tenantID, steward.KnowledgeBaseID, types.AuditActionKnowledgeOwnerChanged,
			"knowledge", knowledgeID, types.AuditOutcomeSuccess, map[string]any{
				"title": steward.Title, "from": steward.OwnerID, "to": ownerID,
			})
		s.recheck(ctx, tenantID, steward.KnowledgeBaseID, knowledgeID)
	}
	return s.Get(ctx, tenantID, knowledgeID)
}

// ConfirmReviewed implements interfaces.KnowledgeStewardshipService.
func (s *knowledgeStewardshipService) ConfirmReviewed(ctx context.Context, tenantID uint64, knowledgeID string,
) (*types.KnowledgeStewardshipView, error) {
	actor, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(actor) {
		// A review is a person's word for the content. An API key speaks
		// for no one in particular, and the review clock would then say
		// somebody looked when nobody did.
		return nil, werrors.NewForbiddenError("confirming an entry needs a signed-in person")
	}
	steward, err := s.steward(ctx, tenantID, knowledgeID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkReviewed(ctx, tenantID, knowledgeID, actor, s.now()); err != nil {
		return nil, err
	}
	recordKBActivity(ctx, s.audit, tenantID, steward.KnowledgeBaseID, types.AuditActionKnowledgeReviewed,
		"knowledge", knowledgeID, types.AuditOutcomeSuccess, map[string]any{"title": steward.Title})
	s.recheck(ctx, tenantID, steward.KnowledgeBaseID, knowledgeID)
	return s.Get(ctx, tenantID, knowledgeID)
}

// SyncFromSource implements interfaces.KnowledgeStewardshipService.
//
// No membership check and no activity entry: the source decided, and recorded
// the decision where it was made. The owner is copied as it is, so a page
// whose owner left the workspace shows that on its mirror too, and the
// routing falls back past them.
func (s *knowledgeStewardshipService) SyncFromSource(ctx context.Context, tenantID uint64, knowledgeID, ownerID,
	reviewedBy string, reviewedAt time.Time,
) error {
	steward, err := s.steward(ctx, tenantID, knowledgeID)
	if err != nil {
		return err
	}
	changed := false
	if steward.OwnerID != ownerID {
		if err := s.repo.SetOwner(ctx, tenantID, knowledgeID, ownerID); err != nil {
			return s.notFound(err)
		}
		changed = true
	}
	if reviewedBy != "" && !reviewedAt.IsZero() &&
		(steward.ReviewedAt == nil || steward.ReviewedAt.Before(reviewedAt) || steward.ReviewedBy != reviewedBy) {
		if err := s.repo.MarkReviewed(ctx, tenantID, knowledgeID, reviewedBy, reviewedAt); err != nil {
			return err
		}
		changed = true
	}
	if changed {
		s.recheck(ctx, tenantID, steward.KnowledgeBaseID, knowledgeID)
	}
	return nil
}

// recheck schedules a knowledge-health check of the entry. Best effort: the
// stewardship change is made, and the next change of the entry checks it
// anyway.
func (s *knowledgeStewardshipService) recheck(ctx context.Context, tenantID uint64, kbID, knowledgeID string) {
	if s.trigger == nil {
		return
	}
	if err := s.trigger.TriggerKnowledgeFindings(ctx, tenantID, kbID, knowledgeID); err != nil {
		logger.Warnf(ctx, "[Stewardship] scheduling the check of %s failed: %v", knowledgeID, err)
	}
}

func (s *knowledgeStewardshipService) steward(ctx context.Context, tenantID uint64, knowledgeID string,
) (*types.KnowledgeSteward, error) {
	stewards, err := s.repo.Stewards(ctx, tenantID, []string{knowledgeID})
	if err != nil {
		return nil, err
	}
	steward, ok := stewards[knowledgeID]
	if !ok {
		return nil, werrors.NewNotFoundError("knowledge not found")
	}
	return steward, nil
}

func (s *knowledgeStewardshipService) notFound(err error) error {
	if errors.Is(err, interfaces.ErrKnowledgeNotFound) {
		return werrors.NewNotFoundError("knowledge not found")
	}
	return err
}

func (s *knowledgeStewardshipService) view(ctx context.Context, st *types.KnowledgeSteward,
) (*types.KnowledgeStewardshipView, error) {
	people, err := namePeople(ctx, s.users, map[string]bool{
		st.OwnerID: st.OwnerActive, st.ReviewedBy: st.ReviewerActive,
	})
	if err != nil {
		return nil, err
	}
	return &types.KnowledgeStewardshipView{
		KnowledgeID:        st.KnowledgeID,
		Origin:             st.Origin,
		Owner:              people[st.OwnerID],
		OwnerEditable:      st.Origin != types.KnowledgeOriginDocs,
		ReviewedAt:         st.ReviewedAt,
		ReviewedBy:         people[st.ReviewedBy],
		ReviewIntervalDays: st.ReviewIntervalDays,
		ReviewDueAt:        st.ReviewDueAt(),
		Overdue:            st.Overdue(s.now()),
	}, nil
}

// namePeople resolves user IDs to PersonRefs. active says, per ID, whether the
// person can still be asked; the empty ID is skipped.
func namePeople(ctx context.Context, users interfaces.UserRepository, active map[string]bool,
) (map[string]*types.PersonRef, error) {
	ids := make([]string, 0, len(active))
	for id := range active {
		if id != "" {
			ids = append(ids, id)
		}
	}
	out := make(map[string]*types.PersonRef, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	found, err := users.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		ref := &types.PersonRef{ID: id, Active: active[id]}
		if u, ok := found[id]; ok && u != nil {
			ref.Username, ref.Avatar = u.Username, u.Avatar
		} else {
			ref.Active = false
		}
		out[id] = ref
	}
	return out, nil
}
