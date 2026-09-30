package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

// message inserts a message; an answer cites the given knowledge entries.
func feedbackMessage(t *testing.T, db *gorm.DB, session, request, role, content string, cites ...string) string {
	t.Helper()
	refs := "["
	for i, k := range cites {
		if i > 0 {
			refs += ","
		}
		refs += `{"id":"c` + k + `","knowledge_id":"` + k + `","content":"x"}`
	}
	refs += "]"
	id := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO messages (id, request_id, session_id, role, content, knowledge_references,
		is_completed, created_at) VALUES (?, ?, ?, ?, ?, ?::jsonb, true, NOW())`,
		id, request, session, role, content, refs).Error)
	return id
}

// A person's rating is one row that they can change or take back, and each
// write reports what it replaced.
func TestMessageFeedbackUpsertAndDelete(t *testing.T) {
	db := pgtest.New(t)
	repo := NewMessageFeedbackRepository(db)
	ctx := context.Background()
	answer := feedbackMessage(t, db, "s1", "r1", "assistant", "An answer.", "k1")
	fb := func(rating string) *types.MessageFeedback {
		return &types.MessageFeedback{
			TenantID: 1, SessionID: "s1", MessageID: answer, UserID: "u1", Rating: rating, Comment: "because",
		}
	}

	prev, err := repo.Upsert(ctx, fb(types.FeedbackDown))
	require.NoError(t, err)
	assert.Empty(t, prev)
	prev, err = repo.Upsert(ctx, fb(types.FeedbackUp))
	require.NoError(t, err)
	assert.Equal(t, types.FeedbackDown, prev)

	mine, err := repo.ListForSession(ctx, 1, "s1", "u1")
	require.NoError(t, err)
	require.Contains(t, mine, answer)
	assert.Equal(t, types.FeedbackUp, mine[answer].Rating)
	theirs, err := repo.ListForSession(ctx, 1, "s1", "u2")
	require.NoError(t, err)
	assert.Empty(t, theirs)

	prev, err = repo.Delete(ctx, 1, answer, "u1")
	require.NoError(t, err)
	assert.Equal(t, types.FeedbackUp, prev)
	prev, err = repo.Delete(ctx, 1, answer, "u1")
	require.NoError(t, err)
	assert.Empty(t, prev, "nothing left to take back")
}

// Disputes of an entry: the workspace's down-votes on answers citing it after
// the cut-off, newest first, with the question where the person attached it;
// up-votes,
// answers citing something else, other workspaces and deleted conversations
// do not count.
func TestDisputesSince(t *testing.T) {
	db := pgtest.New(t)
	repo := NewMessageFeedbackRepository(db)
	ctx := context.Background()
	rate := func(tenant uint64, session, answer, user, rating, comment string, at time.Time, share ...bool) {
		require.NoError(t, db.Exec(`INSERT INTO message_feedback (id, tenant_id, session_id, message_id, user_id,
			rating, comment, share_question, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), tenant, session, answer, user, rating, comment, len(share) > 0 && share[0], at, at).Error)
	}
	now := time.Now().UTC()
	feedbackMessage(t, db, "s1", "r1", "user", "How many days of leave?")
	a1 := feedbackMessage(t, db, "s1", "r1", "assistant", "Fifteen days.", "k-leave", "k-other")
	feedbackMessage(t, db, "s2", "r2", "user", "Leave for part-timers?")
	a2 := feedbackMessage(t, db, "s2", "r2", "assistant", "Also fifteen.", "k-leave")
	a3 := feedbackMessage(t, db, "s3", "r3", "assistant", "Unrelated.", "k-other")
	a4 := feedbackMessage(t, db, "s4", "r4", "assistant", "Deleted chat.", "k-leave")
	require.NoError(t, db.Exec(`UPDATE messages SET deleted_at = NOW() WHERE id = ?`, a4).Error)

	rate(1, "s1", a1, "u1", types.FeedbackDown, "It is ten now", now.Add(-time.Hour), true)
	rate(1, "s2", a2, "u2", types.FeedbackDown, "", now.Add(-time.Minute))
	rate(1, "s1", a1, "u3", types.FeedbackUp, "", now)
	rate(1, "s3", a3, "u1", types.FeedbackDown, "", now)
	rate(2, "s2", a2, "u9", types.FeedbackDown, "other workspace", now)
	rate(1, "s4", a4, "u1", types.FeedbackDown, "", now)

	count, reports, err := repo.DisputesSince(ctx, 1, "k-leave", now.Add(-24*time.Hour), 3)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	require.Len(t, reports, 2)
	assert.Equal(t, "Also fifteen.", reports[0].Answer, "newest first")
	assert.Empty(t, reports[0].Question, "a question stays in its conversation unless attached")
	assert.Equal(t, "How many days of leave?", reports[1].Question, "attached")
	assert.Equal(t, "It is ten now", reports[1].Comment)

	count, _, err = repo.DisputesSince(ctx, 1, "k-leave", now.Add(-30*time.Minute), 3)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "only what came after the last review")
}
