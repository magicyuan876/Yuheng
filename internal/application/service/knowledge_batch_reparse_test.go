package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type reparseFailureKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	knowledge   *types.Knowledge
	updateCalls int
}

func (r *reparseFailureKnowledgeRepo) GetKnowledgeByID(
	_ context.Context,
	_ uint64,
	_ string,
) (*types.Knowledge, error) {
	return r.knowledge, nil
}

func (r *reparseFailureKnowledgeRepo) UpdateKnowledge(
	_ context.Context,
	_ *types.Knowledge,
) error {
	r.updateCalls++
	return nil
}

func (r *reparseFailureKnowledgeRepo) UpdateKnowledgeColumn(
	_ context.Context,
	_ string,
	_ string,
	_ interface{},
) error {
	return nil
}

type reparseFailureKBService struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *reparseFailureKBService) GetKnowledgeBaseByID(
	_ context.Context,
	_ string,
) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

type failingReparseTaskEnqueuer struct {
	err error
}

func (e failingReparseTaskEnqueuer) Enqueue(
	_ *asynq.Task,
	_ ...asynq.Option,
) (*asynq.TaskInfo, error) {
	return nil, e.err
}

func TestReparseKnowledgeManualEnqueueFailureIsVisible(t *testing.T) {
	enqueueErr := errors.New("queue unavailable")
	knowledge := &types.Knowledge{
		ID:              "knowledge-1",
		TenantID:        7,
		KnowledgeBaseID: "kb-1",
		Type:            types.KnowledgeTypeManual,
		ParseStatus:     types.ParseStatusCompleted,
		EnableStatus:    "enabled",
	}
	require.NoError(t, knowledge.SetManualMetadata(
		types.NewManualKnowledgeMetadata("# content", types.ManualKnowledgeStatusPublish, 1),
	))
	repo := &reparseFailureKnowledgeRepo{knowledge: knowledge}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &reparseFailureKBService{kb: &types.KnowledgeBase{ID: "kb-1"}},
		task:      failingReparseTaskEnqueuer{err: enqueueErr},
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))

	got, err := svc.ReparseKnowledge(ctx, knowledge.ID, nil)

	require.Error(t, err)
	require.Same(t, knowledge, got)
	require.Equal(t, types.ParseStatusFailed, knowledge.ParseStatus)
	require.Equal(t, "disabled", knowledge.EnableStatus)
	require.Equal(t, "Failed to enqueue processing task", knowledge.ErrorMessage)
	require.GreaterOrEqual(t, repo.updateCalls, 2, "pending and failed states must both be persisted")
}

func TestReparseRunReportsPartialFailure(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	var attempted []string

	var run reparseRun
	run.reparse([]string{"ok-1", "bad-1", "ok-2", "bad-2"}, func(id string) error {
		attempted = append(attempted, id)
		switch id {
		case "bad-1":
			return firstErr
		case "bad-2":
			return secondErr
		default:
			return nil
		}
	})
	err := run.err()

	require.Equal(t, []string{"ok-1", "bad-1", "ok-2", "bad-2"}, attempted)
	require.Equal(t, 2, run.Submitted)
	require.Equal(t, 2, run.Failed)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.ErrorContains(t, err, "knowledge bad-1")
	require.ErrorContains(t, err, "knowledge bad-2")
}

func TestReparseRunSucceeds(t *testing.T) {
	var run reparseRun
	run.reparse([]string{"knowledge-1"}, func(string) error { return nil })
	// A second call adds to the tally: the index rebuild feeds the run one
	// page at a time.
	run.reparse([]string{"knowledge-2"}, func(string) error { return nil })

	require.NoError(t, run.err())
	require.Equal(t, 2, run.Submitted)
	require.Zero(t, run.Failed)
}

// A run over thousands of failing documents reports all of them in its
// counts but keeps only the first few errors, so the task's last error stays
// readable.
func TestReparseRunCapsReportedFailures(t *testing.T) {
	ids := make([]string, maxReportedReparseFailures+5)
	for i := range ids {
		ids[i] = fmt.Sprintf("bad-%d", i)
	}
	var run reparseRun
	run.reparse(ids, func(string) error { return errors.New("parser down") })

	require.Equal(t, len(ids), run.Failed)
	require.Len(t, run.failures, maxReportedReparseFailures)
	require.ErrorContains(t, run.err(), fmt.Sprintf("failed %d", len(ids)))
}
