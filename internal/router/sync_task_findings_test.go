package router

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/application/service/findings"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Without Redis a task ID is honoured the way asynq honours it: a second task
// with the ID of one still scheduled or running is refused, and the ID is free
// again once the first has finished.
func TestSyncTaskExecutorRefusesATaskIDThatIsStillPending(t *testing.T) {
	executor := NewSyncTaskExecutor()
	release := make(chan struct{})
	done := make(chan struct{}, 2)
	executor.RegisterHandler("test:id", func(context.Context, *asynq.Task) error {
		<-release
		done <- struct{}{}
		return nil
	})

	_, err := executor.Enqueue(asynq.NewTask("test:id", nil), asynq.TaskID("same"))
	require.NoError(t, err)
	_, err = executor.Enqueue(asynq.NewTask("test:id", nil), asynq.TaskID("same"))
	require.ErrorIs(t, err, asynq.ErrTaskIDConflict)
	_, err = executor.Enqueue(asynq.NewTask("test:id", nil), asynq.TaskID("other"))
	require.NoError(t, err, "a different ID is a different task")

	close(release)
	for range 2 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("tasks did not run")
		}
	}
	require.Eventually(t, func() bool {
		_, err := executor.Enqueue(asynq.NewTask("test:id", nil), asynq.TaskID("same"))
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "the ID is released when the task finishes")
}

// The knowledge-health trigger debounces through the Redis-less executor too:
// changes in quick succession become one check.
func TestFindingsChecksAreDebouncedWithoutRedis(t *testing.T) {
	executor := NewSyncTaskExecutor()
	var runs atomic.Int32
	payloads := make(chan types.KnowledgeFindingsPayload, 4)
	executor.RegisterHandler(types.TypeKnowledgeFindings, func(_ context.Context, task *asynq.Task) error {
		var p types.KnowledgeFindingsPayload
		if err := json.Unmarshal(task.Payload(), &p); err != nil {
			return errors.New("bad payload")
		}
		runs.Add(1)
		payloads <- p
		return nil
	})
	cfg := &config.Config{Findings: &config.FindingsConfig{Enabled: true, DuplicateMinScore: 0.95}}
	trigger := findings.NewTrigger(executor, cfg).WithWindow(time.Second)

	ctx := context.Background()
	for range 3 {
		require.NoError(t, trigger.TriggerKnowledgeFindings(ctx, 1, "kb-1", "doc-1"))
	}
	select {
	case p := <-payloads:
		assert.Equal(t, types.KnowledgeFindingsPayload{TenantID: 1, KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1"}, p)
	case <-time.After(5 * time.Second):
		t.Fatal("the check never ran")
	}
	time.Sleep(1500 * time.Millisecond)
	assert.LessOrEqual(t, runs.Load(), int32(2),
		"three changes within one window run at most once per slot they fall in")
	assert.GreaterOrEqual(t, runs.Load(), int32(1))
}
