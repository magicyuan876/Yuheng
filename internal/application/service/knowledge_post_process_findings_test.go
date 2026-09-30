package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingFindingsTrigger stands in for the findings trigger.
type recordingFindingsTrigger struct {
	calls [][3]any
	err   error
}

func (r *recordingFindingsTrigger) TriggerKnowledgeFindings(_ context.Context, tenantID uint64, kbID,
	knowledgeID string,
) error {
	r.calls = append(r.calls, [3]any{tenantID, kbID, knowledgeID})
	return r.err
}

func (r *recordingFindingsTrigger) ScheduleKnowledgeFindings(context.Context, uint64, string, string,
	time.Duration,
) (bool, error) {
	return false, errors.New("not used by post-processing")
}

func newFindingsPostProcess(kb *types.KnowledgeBase, status string,
	trigger *recordingFindingsTrigger,
) *KnowledgePostProcessService {
	repo := &wikiEnqueueFailureKnowledgeRepo{
		knowledge: &types.Knowledge{ID: "doc-1", ParseStatus: status},
	}
	return &KnowledgePostProcessService{
		knowledgeRepo: repo,
		kbService:     &wikiEnqueueFailureKBService{kb: kb},
		chunkService: &wikiEnqueueFailureChunkService{chunks: []*types.Chunk{
			{ID: "chunk-1", ChunkType: types.ChunkTypeText},
		}},
		taskEnqueuer: &wikiEnqueueFailureTaskQueue{},
		findings:     trigger,
	}
}

func documentKB() *types.KnowledgeBase {
	return &types.KnowledgeBase{
		ID: "kb-wiki", Type: types.KnowledgeBaseTypeDocument,
		IndexingStrategy: types.IndexingStrategy{VectorEnabled: true},
	}
}

// Every indexed document schedules its health check — the one place all
// content changes (upload, re-parse, manual edit, docs mirror) pass through.
func TestPostProcessSchedulesTheHealthCheck(t *testing.T) {
	trigger := &recordingFindingsTrigger{}
	svc := newFindingsPostProcess(documentKB(), types.ParseStatusProcessing, trigger)

	require.NoError(t, svc.Handle(context.Background(), newWikiEnqueuePostProcessTask(t, "doc-1")))

	require.Len(t, trigger.calls, 1)
	assert.Equal(t, [3]any{uint64(7), "kb-wiki", "doc-1"}, trigger.calls[0])
}

// Which detector applies to which base is the detectors' decision, not
// post-processing's: an FAQ or keyword-only base is still scheduled (the core
// detector skips it, one an extension adds may not).
func TestPostProcessLeavesApplicabilityToTheDetectors(t *testing.T) {
	faq := documentKB()
	faq.Type = types.KnowledgeBaseTypeFAQ
	keywordsOnly := documentKB()
	keywordsOnly.IndexingStrategy = types.IndexingStrategy{KeywordEnabled: true}
	for name, kb := range map[string]*types.KnowledgeBase{"FAQ base": faq, "no vectors": keywordsOnly} {
		t.Run(name, func(t *testing.T) {
			trigger := &recordingFindingsTrigger{}
			svc := newFindingsPostProcess(kb, types.ParseStatusProcessing, trigger)
			require.NoError(t, svc.Handle(context.Background(), newWikiEnqueuePostProcessTask(t, "doc-1")))
			assert.Len(t, trigger.calls, 1)
		})
	}
}

// A document whose indexing was abandoned is not checked.
func TestPostProcessSkipsTheHealthCheckOfAbandonedIndexing(t *testing.T) {
	cases := map[string]struct {
		kb     *types.KnowledgeBase
		status string
	}{
		"cancelled meanwhile":  {documentKB(), types.ParseStatusCancelled},
		"already failed again": {documentKB(), types.ParseStatusFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trigger := &recordingFindingsTrigger{}
			svc := newFindingsPostProcess(tc.kb, tc.status, trigger)
			require.NoError(t, svc.Handle(context.Background(), newWikiEnqueuePostProcessTask(t, "doc-1")))
			assert.Empty(t, trigger.calls)
		})
	}
}

// Scheduling is best-effort: a queue that refuses it does not fail indexing.
func TestPostProcessSucceedsWhenTheHealthCheckCannotBeScheduled(t *testing.T) {
	trigger := &recordingFindingsTrigger{err: errors.New("redis unavailable")}
	svc := newFindingsPostProcess(documentKB(), types.ParseStatusProcessing, trigger)

	require.NoError(t, svc.Handle(context.Background(), newWikiEnqueuePostProcessTask(t, "doc-1")))
	assert.Len(t, trigger.calls, 1)
}
