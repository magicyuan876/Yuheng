package findings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// KnowledgeLookup is the part of the knowledge repository the task reads.
type KnowledgeLookup interface {
	GetKnowledgeByIDOnly(ctx context.Context, id string) (*types.Knowledge, error)
}

// KnowledgeBaseLookup is the part of the knowledge-base repository the task
// reads.
type KnowledgeBaseLookup interface {
	GetKnowledgeBaseByIDAndTenant(ctx context.Context, id string, tenantID uint64) (*types.KnowledgeBase, error)
}

// TaskHandler runs TypeKnowledgeFindings.
type TaskHandler struct {
	knowledge KnowledgeLookup
	kbs       KnowledgeBaseLookup
	runner    *Runner
	enabled   bool
}

// NewTaskHandler returns the handler, as an interfaces.TaskHandler for the
// task registries.
func NewTaskHandler(
	knowledge interfaces.KnowledgeRepository,
	kbs interfaces.KnowledgeBaseRepository,
	runner *Runner,
	cfg *config.Config,
) interfaces.TaskHandler {
	return newTaskHandler(knowledge, kbs, runner, cfg != nil && cfg.Findings.IsEnabled())
}

func newTaskHandler(knowledge KnowledgeLookup, kbs KnowledgeBaseLookup, runner *Runner, enabled bool) *TaskHandler {
	return &TaskHandler{knowledge: knowledge, kbs: kbs, runner: runner, enabled: enabled}
}

// checkableStatuses are the parse states in which a document's chunks are all
// indexed. finalizing is included: summaries and questions may still be on
// their way, but the passages being compared are in place.
var checkableStatuses = map[string]bool{
	types.ParseStatusCompleted:  true,
	types.ParseStatusFinalizing: true,
}

// Handle implements interfaces.TaskHandler.
//
// Everything that makes the check pointless — the feature switched off since
// the task was queued, the document deleted, moved or being re-indexed, the
// knowledge base gone — ends the task successfully: retrying would not change
// the answer, and whatever changes the document next schedules a new check.
func (h *TaskHandler) Handle(ctx context.Context, task *asynq.Task) error {
	if !h.enabled {
		return nil
	}
	var payload types.KnowledgeFindingsPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("%w: decoding the findings payload: %v", asynq.SkipRetry, err)
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)

	knowledge, err := h.knowledge.GetKnowledgeByIDOnly(ctx, payload.KnowledgeID)
	if errors.Is(err, interfaces.ErrKnowledgeNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading knowledge %s: %w", payload.KnowledgeID, err)
	}
	if knowledge.TenantID != payload.TenantID || knowledge.KnowledgeBaseID != payload.KnowledgeBaseID {
		// Moved to another knowledge base since: the trigger on knowledges
		// has already forgotten its findings here, and the move re-indexes
		// it where it now lives.
		return nil
	}
	if !checkableStatuses[knowledge.ParseStatus] {
		logger.Infof(ctx, "[Findings] knowledge %s is %s; its next indexing schedules the check",
			knowledge.ID, knowledge.ParseStatus)
		return nil
	}
	kb, err := h.kbs.GetKnowledgeBaseByIDAndTenant(ctx, payload.KnowledgeBaseID, payload.TenantID)
	if errors.Is(err, repository.ErrKnowledgeBaseNotFound) || (err == nil && kb == nil) {
		logger.Infof(ctx, "[Findings] knowledge base %s is gone, skipping the check of %s",
			payload.KnowledgeBaseID, knowledge.ID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading knowledge base %s: %w", payload.KnowledgeBaseID, err)
	}
	return h.runner.Run(ctx, Scope{TenantID: payload.TenantID, KnowledgeBase: kb, Knowledge: knowledge})
}
