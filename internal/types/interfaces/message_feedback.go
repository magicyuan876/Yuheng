package interfaces

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// MessageFeedbackRepository stores people's feedback on answers.
type MessageFeedbackRepository interface {
	// Upsert records a person's rating of an answer, replacing the one they
	// gave before, and returns the previous rating ("" for none).
	Upsert(ctx context.Context, fb *types.MessageFeedback) (previous string, err error)
	// Delete takes a person's feedback on an answer back and returns the
	// rating it had ("" for none).
	Delete(ctx context.Context, tenantID uint64, messageID, userID string) (previous string, err error)
	// ListForSession returns a person's feedback on the answers of a
	// session, keyed by message ID.
	ListForSession(ctx context.Context, tenantID uint64, sessionID,
		userID string) (map[string]*types.MessageFeedback, error)
	// DisputesSince returns how many answers citing the entry people of the
	// tenant marked as not helpful after since, and the latest limit of them,
	// newest first. Answers whose conversation was deleted do not count.
	DisputesSince(ctx context.Context, tenantID uint64, knowledgeID string, since time.Time,
		limit int) (int, []types.DisputeReport, error)
}

// MessageFeedbackService is the API over feedback on answers. The caller is
// the person in ctx; the session must be theirs.
type MessageFeedbackService interface {
	// List returns the caller's feedback on the answers of a session.
	List(ctx context.Context, sessionID string) (map[string]*types.MessageFeedbackView, error)
	// Set records the caller's rating of an answer, or with an empty rating
	// takes it back, and has the documents the answer cites checked for
	// disputes. It returns nil after taking feedback back.
	Set(ctx context.Context, sessionID, messageID string, req types.SetMessageFeedbackRequest,
	) (*types.MessageFeedbackView, error)
}
