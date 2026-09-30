package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/magicyuan876/yuheng/internal/application/repository"
	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// rebuildKnowledgeRepo serves a fixed, id-ordered set of rebuildable ids the
// way the real repository pages them: strictly after the cursor, at most
// limit at a time.
type rebuildKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	ids   []string
	pages []string // the cursor each page was asked for
}

func (r *rebuildKnowledgeRepo) CountRebuildableKnowledge(context.Context, uint64, string) (int64, error) {
	return int64(len(r.ids)), nil
}

func (r *rebuildKnowledgeRepo) ListRebuildableKnowledgeIDs(
	_ context.Context, _ uint64, _ string, afterID string, limit int,
) ([]string, error) {
	r.pages = append(r.pages, afterID)
	var page []string
	for _, id := range r.ids {
		if id > afterID && len(page) < limit {
			page = append(page, id)
		}
	}
	return page, nil
}

// rebuildKBService finds the base until gone is set, then reports it deleted.
type rebuildKBService struct {
	interfaces.KnowledgeBaseService
	gone bool
}

func (s *rebuildKBService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	if s.gone {
		return nil, repository.ErrKnowledgeBaseNotFound
	}
	return &types.KnowledgeBase{ID: "kb-1"}, nil
}

type recordingTaskEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (e *recordingTaskEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.tasks = append(e.tasks, task)
	e.opts = append(e.opts, opts)
	return &asynq.TaskInfo{ID: "task-1"}, nil
}

func rebuildTestIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		// Zero-padded so that string order, which the database pages by,
		// is creation order.
		ids[i] = fmt.Sprintf("k-%04d", i)
	}
	return ids
}

func TestRebuildKnowledgeBaseIndexQueuesOneTaskPerBase(t *testing.T) {
	repo := &rebuildKnowledgeRepo{ids: rebuildTestIDs(3)}
	queue := &recordingTaskEnqueuer{}
	svc := &knowledgeService{repo: repo, task: queue}
	kb := &types.KnowledgeBase{ID: "kb-1", TenantID: 7, Type: types.KnowledgeBaseTypeDocument}

	result, err := svc.RebuildKnowledgeBaseIndex(context.Background(), kb)

	require.NoError(t, err)
	require.Equal(t, &types.KBRebuildIndexResult{TaskID: "task-1", DocumentCount: 3}, result)
	require.Len(t, queue.tasks, 1)
	require.Equal(t, types.TypeKBRebuildIndex, queue.tasks[0].Type())
	var payload types.KBRebuildIndexPayload
	require.NoError(t, json.Unmarshal(queue.tasks[0].Payload(), &payload))
	require.Equal(t, uint64(7), payload.TenantID)
	require.Equal(t, "kb-1", payload.KnowledgeBaseID)

	var taskID, queueName string
	for _, opt := range queue.opts[0] {
		switch opt.Type() {
		case asynq.TaskIDOpt:
			taskID, _ = opt.Value().(string)
		case asynq.QueueOpt:
			queueName, _ = opt.Value().(string)
		}
	}
	require.Equal(t, kbRebuildIndexTaskID("kb-1"), taskID, "the task id must be per base, so a repeat conflicts")
	wantQueue, _ := types.QueueForTaskType(types.TypeKBRebuildIndex)
	require.Equal(t, wantQueue, queueName)
}

func TestRebuildKnowledgeBaseIndexWithNothingToDoQueuesNothing(t *testing.T) {
	queue := &recordingTaskEnqueuer{}
	svc := &knowledgeService{repo: &rebuildKnowledgeRepo{}, task: queue}

	result, err := svc.RebuildKnowledgeBaseIndex(context.Background(), &types.KnowledgeBase{ID: "kb-1"})

	require.NoError(t, err)
	require.Equal(t, &types.KBRebuildIndexResult{}, result)
	require.Empty(t, queue.tasks)
}

func TestRebuildKnowledgeBaseIndexWhileOneIsRunningConflicts(t *testing.T) {
	svc := &knowledgeService{
		repo: &rebuildKnowledgeRepo{ids: rebuildTestIDs(1)},
		task: &recordingTaskEnqueuer{err: asynq.ErrTaskIDConflict},
	}

	_, err := svc.RebuildKnowledgeBaseIndex(context.Background(), &types.KnowledgeBase{ID: "kb-1"})

	appErr, ok := werrors.IsAppError(err)
	require.True(t, ok, "want an AppError, got %v", err)
	require.Equal(t, http.StatusConflict, appErr.HTTPCode)
}

func TestRebuildKnowledgeBaseIndexRefusesFAQBase(t *testing.T) {
	queue := &recordingTaskEnqueuer{}
	svc := &knowledgeService{repo: &rebuildKnowledgeRepo{ids: rebuildTestIDs(1)}, task: queue}

	_, err := svc.RebuildKnowledgeBaseIndex(context.Background(),
		&types.KnowledgeBase{ID: "kb-1", Type: types.KnowledgeBaseTypeFAQ})

	appErr, ok := werrors.IsAppError(err)
	require.True(t, ok, "want an AppError, got %v", err)
	require.Equal(t, http.StatusBadRequest, appErr.HTTPCode)
	require.Empty(t, queue.tasks)
}

// The worker visits every id exactly once, a page at a time, each page
// starting after the last id of the one before.
func TestForEachRebuildablePageVisitsEveryIDOnce(t *testing.T) {
	ids := rebuildTestIDs(kbRebuildIndexPageSize*2 + 5)
	repo := &rebuildKnowledgeRepo{ids: ids}
	svc := &knowledgeService{repo: repo, kbService: &rebuildKBService{}}

	var visited []string
	err := svc.forEachRebuildablePage(context.Background(), 7, "kb-1", func(page []string) {
		require.LessOrEqual(t, len(page), kbRebuildIndexPageSize)
		visited = append(visited, page...)
	})

	require.NoError(t, err)
	require.Equal(t, ids, visited)
	require.Equal(t, []string{"", ids[kbRebuildIndexPageSize-1], ids[2*kbRebuildIndexPageSize-1]}, repo.pages)
}

// Deleting the base mid-run stops the rebuild quietly at the next page.
func TestForEachRebuildablePageStopsWhenTheBaseIsGone(t *testing.T) {
	repo := &rebuildKnowledgeRepo{ids: rebuildTestIDs(kbRebuildIndexPageSize + 1)}
	kbs := &rebuildKBService{}
	svc := &knowledgeService{repo: repo, kbService: kbs}

	pages := 0
	err := svc.forEachRebuildablePage(context.Background(), 7, "kb-1", func([]string) {
		pages++
		kbs.gone = true
	})

	require.NoError(t, err)
	require.Equal(t, 1, pages)
}
