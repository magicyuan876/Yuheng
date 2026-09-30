package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// messageFeedbackRepository stores people's feedback on answers.
type messageFeedbackRepository struct {
	db *gorm.DB
}

// NewMessageFeedbackRepository returns the PostgreSQL feedback store.
func NewMessageFeedbackRepository(db *gorm.DB) interfaces.MessageFeedbackRepository {
	return &messageFeedbackRepository{db: db}
}

// Upsert implements interfaces.MessageFeedbackRepository. One statement: the
// CTE reads the row as it was before the write, so the previous rating comes
// back with the write, whoever else is writing.
func (r *messageFeedbackRepository) Upsert(ctx context.Context, fb *types.MessageFeedback) (string, error) {
	if fb.ID == "" {
		fb.ID = uuid.NewString()
	}
	var rows []struct{ Previous *string }
	err := r.db.WithContext(ctx).Raw(`
		WITH prev AS (SELECT rating FROM message_feedback WHERE message_id = @message AND user_id = @user)
		INSERT INTO message_feedback (id, tenant_id, session_id, message_id, user_id, rating, comment,
		                              share_question, created_at, updated_at)
		VALUES (@id, @tenant, @session, @message, @user, @rating, @comment, @share, NOW(), NOW())
		ON CONFLICT (message_id, user_id) DO UPDATE
		   SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, share_question = EXCLUDED.share_question,
		       updated_at = NOW()
		RETURNING (SELECT rating FROM prev) AS previous`,
		map[string]any{
			"id": fb.ID, "tenant": fb.TenantID, "session": fb.SessionID, "message": fb.MessageID,
			"user": fb.UserID, "rating": fb.Rating, "comment": fb.Comment, "share": fb.ShareQuestion,
		}).Scan(&rows).Error
	if err != nil || len(rows) == 0 || rows[0].Previous == nil {
		return "", err
	}
	return *rows[0].Previous, nil
}

// Delete implements interfaces.MessageFeedbackRepository.
func (r *messageFeedbackRepository) Delete(ctx context.Context, tenantID uint64, messageID, userID string,
) (string, error) {
	var ratings []string
	err := r.db.WithContext(ctx).Raw(`
		DELETE FROM message_feedback WHERE tenant_id = ? AND message_id = ? AND user_id = ?
		RETURNING rating`, tenantID, messageID, userID).Scan(&ratings).Error
	if err != nil || len(ratings) == 0 {
		return "", err
	}
	return ratings[0], nil
}

// ListForSession implements interfaces.MessageFeedbackRepository.
func (r *messageFeedbackRepository) ListForSession(ctx context.Context, tenantID uint64, sessionID,
	userID string,
) (map[string]*types.MessageFeedback, error) {
	var rows []*types.MessageFeedback
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND user_id = ?", tenantID, sessionID, userID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]*types.MessageFeedback, len(rows))
	for _, row := range rows {
		out[row.MessageID] = row
	}
	return out, nil
}

// disputeExcerptRunes bounds the question and the answer a report carries.
const disputeExcerptRunes = 300

// DisputesSince implements interfaces.MessageFeedbackRepository.
//
// An answer cites an entry when its knowledge_references carry the entry's
// ID; the containment test runs only over the tenant's down-votes since the
// cut-off, which the partial index finds, never over the messages table as a
// whole. The question, the user message of the same request, is included only
// where the person chose to attach it.
func (r *messageFeedbackRepository) DisputesSince(ctx context.Context, tenantID uint64, knowledgeID string,
	since time.Time, limit int,
) (int, []types.DisputeReport, error) {
	const from = `
		FROM message_feedback fb
		JOIN messages m ON m.id = fb.message_id AND m.deleted_at IS NULL
		WHERE fb.tenant_id = @tenant AND fb.rating = 'down' AND fb.updated_at > @since
		  AND m.knowledge_references::jsonb @>
		      jsonb_build_array(jsonb_build_object('knowledge_id', CAST(@knowledge AS text)))`
	args := map[string]any{
		"tenant": tenantID, "since": since, "knowledge": knowledgeID, "limit": limit, "runes": disputeExcerptRunes,
	}
	var count int
	if err := r.db.WithContext(ctx).Raw(`SELECT COUNT(*)`+from, args).Scan(&count).Error; err != nil {
		return 0, nil, err
	}
	if count == 0 || limit <= 0 {
		return count, nil, nil
	}
	var reports []types.DisputeReport
	err := r.db.WithContext(ctx).Raw(`
		SELECT fb.id AS feedback_id, fb.comment, fb.updated_at AS at, LEFT(m.content, @runes) AS answer,
		       CASE WHEN fb.share_question THEN
		            COALESCE((SELECT LEFT(q.content, @runes) FROM messages q
		                       WHERE q.session_id = m.session_id AND q.request_id = m.request_id
		                         AND q.role = 'user' AND q.deleted_at IS NULL
		                       ORDER BY q.created_at LIMIT 1), '')
		       ELSE '' END AS question`+from+`
		ORDER BY fb.updated_at DESC, fb.id
		LIMIT @limit`, args).Scan(&reports).Error
	return count, reports, err
}
