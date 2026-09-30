package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

type fakeFeedbackRepo struct {
	rows map[string]*types.MessageFeedback
}

func (r *fakeFeedbackRepo) Upsert(_ context.Context, fb *types.MessageFeedback) (string, error) {
	prev := ""
	if old, ok := r.rows[fb.MessageID+"/"+fb.UserID]; ok {
		prev = old.Rating
	}
	copied := *fb
	r.rows[fb.MessageID+"/"+fb.UserID] = &copied
	return prev, nil
}

func (r *fakeFeedbackRepo) Delete(_ context.Context, _ uint64, messageID, userID string) (string, error) {
	old, ok := r.rows[messageID+"/"+userID]
	if !ok {
		return "", nil
	}
	delete(r.rows, messageID+"/"+userID)
	return old.Rating, nil
}

func (r *fakeFeedbackRepo) ListForSession(_ context.Context, _ uint64, sessionID, userID string,
) (map[string]*types.MessageFeedback, error) {
	out := map[string]*types.MessageFeedback{}
	for _, fb := range r.rows {
		if fb.SessionID == sessionID && fb.UserID == userID {
			out[fb.MessageID] = fb
		}
	}
	return out, nil
}

func (r *fakeFeedbackRepo) DisputesSince(context.Context, uint64, string, time.Time, int,
) (int, []types.DisputeReport, error) {
	return 0, nil, nil
}

type fakeMessages struct {
	interfaces.MessageService
	byID map[string]*types.Message
}

func (m *fakeMessages) GetMessage(_ context.Context, sessionID, id string) (*types.Message, error) {
	msg, ok := m.byID[id]
	if !ok || msg.SessionID != sessionID {
		return nil, gorm.ErrRecordNotFound
	}
	return msg, nil
}

type knowledgeByID struct {
	interfaces.KnowledgeRepository
	byID map[string]*types.Knowledge
}

func (k *knowledgeByID) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	if kn, ok := k.byID[id]; ok {
		return kn, nil
	}
	return nil, interfaces.ErrKnowledgeNotFound
}

func feedback(rating, comment string) types.SetMessageFeedbackRequest {
	return types.SetMessageFeedbackRequest{Rating: rating, Comment: comment}
}

func newFeedbackFixture() (*messageFeedbackService, *recordingFindingsTrigger) {
	trigger := &recordingFindingsTrigger{}
	svc := NewMessageFeedbackService(
		&fakeFeedbackRepo{rows: map[string]*types.MessageFeedback{}},
		&fakeMessages{byID: map[string]*types.Message{
			"answer": {ID: "answer", SessionID: "s1", Role: "assistant", KnowledgeReferences: types.References{
				{KnowledgeID: "k-mine"}, {KnowledgeID: "k-mine"}, {KnowledgeID: "k-shared"}, {KnowledgeID: "k-gone"},
			}},
			"question": {ID: "question", SessionID: "s1", Role: "user"},
		}},
		&knowledgeByID{byID: map[string]*types.Knowledge{
			"k-mine":   {ID: "k-mine", TenantID: 1, KnowledgeBaseID: "kb1"},
			"k-shared": {ID: "k-shared", TenantID: 2, KnowledgeBaseID: "kb-other-workspace"},
		}},
		trigger,
	).(*messageFeedbackService)
	return svc, trigger
}

// A down-vote has the cited documents of the conversation's own workspace
// checked, once each; another workspace's shared document is not told about
// a question asked here.
func TestFeedbackDownChecksTheCitedDocumentsOfTheWorkspace(t *testing.T) {
	svc, trigger := newFeedbackFixture()
	view, err := svc.Set(asUser("u1"), "s1", "answer", feedback(types.FeedbackDown, "  It is ten days now  "))
	require.NoError(t, err)
	assert.Equal(t, "It is ten days now", view.Comment)
	assert.Equal(t, [][3]any{{uint64(1), "kb1", "k-mine"}}, trigger.calls)

	mine, err := svc.List(asUser("u1"), "s1")
	require.NoError(t, err)
	assert.Equal(t, types.FeedbackDown, mine["answer"].Rating)

	// Turning it into an up-vote resolves the dispute: checked again, and
	// the comment is dropped with it.
	view, err = svc.Set(asUser("u1"), "s1", "answer", feedback(types.FeedbackUp, "helpful after all"))
	require.NoError(t, err)
	assert.Empty(t, view.Comment)
	assert.Len(t, trigger.calls, 2)

	// An up-vote on its own disputes nothing.
	_, err = svc.Set(asUser("u2"), "s1", "answer", feedback(types.FeedbackUp, ""))
	require.NoError(t, err)
	assert.Len(t, trigger.calls, 2)

	// Taking a down-vote back is checked; taking back nothing is not.
	_, err = svc.Set(asUser("u3"), "s1", "answer", feedback(types.FeedbackDown, ""))
	require.NoError(t, err)
	view, err = svc.Set(asUser("u3"), "s1", "answer", feedback("", ""))
	require.NoError(t, err)
	assert.Nil(t, view)
	assert.Len(t, trigger.calls, 4)
}

func TestFeedbackIsRefusedWhereItMeansNothing(t *testing.T) {
	svc, _ := newFeedbackFixture()
	_, err := svc.Set(asUser("system-1"), "s1", "answer", feedback(types.FeedbackDown, ""))
	assert.Equal(t, http.StatusForbidden, httpCodeOf(t, err), "an API key is nobody")
	_, err = svc.Set(asUser("u1"), "s1", "question", feedback(types.FeedbackDown, ""))
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err), "only an answer is rated")
	_, err = svc.Set(asUser("u1"), "s2", "answer", feedback(types.FeedbackDown, ""))
	assert.Equal(t, http.StatusNotFound, httpCodeOf(t, err), "not in the caller's session")
	_, err = svc.Set(asUser("u1"), "s1", "answer", feedback("meh", ""))
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err))
	long := make([]rune, types.MaxFeedbackCommentRunes+1)
	for i := range long {
		long[i] = '字'
	}
	_, err = svc.Set(asUser("u1"), "s1", "answer", feedback(types.FeedbackDown, string(long)))
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err))
}
