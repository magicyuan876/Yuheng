package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Feedback on answers, and what it does for knowledge health.
//
// A "not helpful" on an answer is the most direct signal there is that a
// document may be wrong: somebody asked a real question and the knowledge base
// answered it badly. The feedback is recorded here, and the documents the
// answer cites are checked; the dispute detector (findings/dispute.go) turns
// the down-votes on answers citing a document, since anybody last vouched for
// it, into a finding for its owner. The finding is read by whoever can read
// the knowledge base, so it shows what the person wrote for it and, only when
// they attached it, the question from their own conversation.
//
// Feedback stays in its workspace. A shared knowledge base from another
// workspace may be cited, but a question asked here is not shown to its
// maintainers there: only documents of the conversation's own workspace are
// checked.

// maxDisputedCitations bounds how many cited documents one piece of feedback
// has checked. Answers cite a handful; this only stops a pathological one.
const maxDisputedCitations = 10

type messageFeedbackService struct {
	repo      interfaces.MessageFeedbackRepository
	messages  interfaces.MessageService
	knowledge interfaces.KnowledgeRepository
	trigger   interfaces.KnowledgeFindingsTrigger
}

// NewMessageFeedbackService returns the feedback service.
func NewMessageFeedbackService(
	repo interfaces.MessageFeedbackRepository,
	messages interfaces.MessageService,
	knowledge interfaces.KnowledgeRepository,
	trigger interfaces.KnowledgeFindingsTrigger,
) interfaces.MessageFeedbackService {
	return &messageFeedbackService{repo: repo, messages: messages, knowledge: knowledge, trigger: trigger}
}

// List implements interfaces.MessageFeedbackService.
func (s *messageFeedbackService) List(ctx context.Context, sessionID string,
) (map[string]*types.MessageFeedbackView, error) {
	person, err := feedbackPerson(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListForSession(ctx, types.MustTenantIDFromContext(ctx), sessionID, person)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*types.MessageFeedbackView, len(rows))
	for id, row := range rows {
		out[id] = feedbackView(row)
	}
	return out, nil
}

// Set implements interfaces.MessageFeedbackService.
func (s *messageFeedbackService) Set(ctx context.Context, sessionID, messageID string,
	req types.SetMessageFeedbackRequest,
) (*types.MessageFeedbackView, error) {
	person, err := feedbackPerson(ctx)
	if err != nil {
		return nil, err
	}
	rating, comment, share := strings.TrimSpace(req.Rating), req.Comment, req.ShareQuestion
	switch rating {
	case types.FeedbackUp:
		// What helped is not something a document's maintainer acts on.
		comment, share = "", false
	case types.FeedbackDown, "":
		comment = strings.TrimSpace(comment)
		if utf8.RuneCountInString(comment) > types.MaxFeedbackCommentRunes {
			return nil, werrors.NewValidationError("the comment is too long")
		}
	default:
		return nil, werrors.NewValidationError("rating must be up, down, or empty to take feedback back")
	}
	// GetMessage checks that the session is the caller's.
	msg, err := s.messages.GetMessage(ctx, sessionID, messageID)
	if err != nil {
		if errors.Is(err, werrors.ErrSessionNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, werrors.NewNotFoundError("message not found")
		}
		return nil, err
	}
	if msg == nil {
		return nil, werrors.NewNotFoundError("message not found")
	}
	if msg.Role != "assistant" {
		return nil, werrors.NewValidationError("only an answer can be rated")
	}
	tenantID := types.MustTenantIDFromContext(ctx)

	var previous string
	var view *types.MessageFeedbackView
	if rating == "" {
		previous, err = s.repo.Delete(ctx, tenantID, messageID, person)
	} else {
		fb := &types.MessageFeedback{
			TenantID: tenantID, SessionID: sessionID, MessageID: messageID, UserID: person,
			Rating: rating, Comment: comment, ShareQuestion: share,
		}
		previous, err = s.repo.Upsert(ctx, fb)
		view = feedbackView(fb)
	}
	if err != nil {
		return nil, err
	}
	if previous == types.FeedbackDown || rating == types.FeedbackDown {
		s.recheckCited(ctx, tenantID, msg)
	}
	return view, nil
}

// recheckCited schedules a check of each document of the workspace the
// answer cites, so a dispute is recorded, updated or, when the down-vote was
// taken back, resolved. Best effort: the feedback is stored, and the next
// change of each document checks it anyway.
func (s *messageFeedbackService) recheckCited(ctx context.Context, tenantID uint64, msg *types.Message) {
	if s.trigger == nil || s.knowledge == nil {
		return
	}
	seen := map[string]bool{}
	for _, ref := range msg.KnowledgeReferences {
		if ref == nil || ref.KnowledgeID == "" || seen[ref.KnowledgeID] {
			continue
		}
		if len(seen) == maxDisputedCitations {
			break
		}
		seen[ref.KnowledgeID] = true
		k, err := s.knowledge.GetKnowledgeByIDOnly(ctx, ref.KnowledgeID)
		if err != nil || k == nil || k.TenantID != tenantID {
			continue
		}
		if err := s.trigger.TriggerKnowledgeFindings(ctx, k.TenantID, k.KnowledgeBaseID, k.ID); err != nil {
			logger.Warnf(ctx, "[Feedback] scheduling the check of %s failed: %v", k.ID, err)
		}
	}
}

// feedbackPerson is the person giving feedback. An API key is nobody in
// particular, and feedback is somebody's word about an answer.
func feedbackPerson(ctx context.Context) (string, error) {
	person, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(person) {
		return "", werrors.NewForbiddenError("feedback on answers needs a signed-in person")
	}
	return person, nil
}

func feedbackView(fb *types.MessageFeedback) *types.MessageFeedbackView {
	return &types.MessageFeedbackView{
		MessageID: fb.MessageID, Rating: fb.Rating, Comment: fb.Comment, ShareQuestion: fb.ShareQuestion,
		UpdatedAt: fb.UpdatedAt,
	}
}
