package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/magicyuan876/yuheng/internal/application/repository"
	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/tracing/langfuse"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Rebuilding a knowledge base's index re-parses every document in it, so that
// a change to how the base indexes (vector, keyword, wiki, graph) reaches the
// documents that were already there. It is the batch reparse applied to the
// whole base: each document goes through ReparseKnowledge, with the overrides
// stored on it, and so is handled exactly as a reparse of that one document
// would handle it — manual entries and docs mirrors from their stored text,
// files and URLs from their source.

const (
	// kbRebuildIndexPageSize is how many ids the worker reads at a time.
	kbRebuildIndexPageSize = 200
	// kbRebuildIndexTimeout bounds one run. Each document costs a synchronous
	// cleanup of its old chunks and index entries before its parse is
	// queued, so a base of tens of thousands of documents takes hours.
	kbRebuildIndexTimeout = 12 * time.Hour
)

// kbRebuildIndexTaskID is the task's asynq id. One id per knowledge base makes
// a second request while a rebuild is queued or running fail with a conflict,
// instead of reparsing every document twice.
func kbRebuildIndexTaskID(kbID string) string {
	return "kb-rebuild-index:" + kbID
}

// RebuildKnowledgeBaseIndex queues a rebuild of the knowledge base's index and
// reports how many documents it will re-process. A base with nothing to
// re-process queues nothing.
func (s *knowledgeService) RebuildKnowledgeBaseIndex(
	ctx context.Context, kb *types.KnowledgeBase,
) (*types.KBRebuildIndexResult, error) {
	if kb.Type == types.KnowledgeBaseTypeFAQ {
		// FAQ entries are indexed from their question and answer rows when
		// they are written; there is no document behind them to re-parse.
		return nil, werrors.NewBadRequestError("an FAQ knowledge base has no documents to rebuild the index from")
	}
	count, err := s.repo.CountRebuildableKnowledge(ctx, kb.TenantID, kb.ID)
	if err != nil {
		return nil, fmt.Errorf("count knowledge to rebuild: %w", err)
	}
	result := &types.KBRebuildIndexResult{DocumentCount: count}
	if count == 0 {
		return result, nil
	}

	payload := types.KBRebuildIndexPayload{
		TenantID:        kb.TenantID,
		KnowledgeBaseID: kb.ID,
		Initiator:       types.TaskInitiatorFromContext(ctx),
	}
	langfuse.InjectTracing(ctx, &payload)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal rebuild index payload: %w", err)
	}
	// The options go to Enqueue rather than NewTask because the executor
	// that stands in for asynq without Redis reads them only from there, and
	// the task id is what keeps a second request from running a second copy.
	info, err := s.task.Enqueue(asynq.NewTask(types.TypeKBRebuildIndex, payloadBytes),
		asynq.TaskID(kbRebuildIndexTaskID(kb.ID)), asynq.Queue(types.QueueMaintenance),
		asynq.MaxRetry(3), asynq.Timeout(kbRebuildIndexTimeout))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		// Not folded into the running rebuild: the documents it has already
		// re-processed were indexed under the settings in force before this
		// request, so pretending it covers the new ones would be false.
		return nil, werrors.NewConflictError("an index rebuild of this knowledge base is already in progress")
	}
	if err != nil {
		return nil, fmt.Errorf("enqueue rebuild index task: %w", err)
	}
	result.TaskID = info.ID
	logger.Infof(ctx, "Knowledge base index rebuild queued: task=%s kb=%s documents=%d", info.ID, kb.ID, count)
	return result, nil
}

// ProcessKBRebuildIndex handles TypeKBRebuildIndex: it pages through the
// knowledge base's documents and re-parses each.
//
// A failure before the first document is submitted is retried. Once
// documents have been submitted, a failure is final (see reparseRun.err):
// a retry would start again from the first document and re-process the ones
// already done.
func (s *knowledgeService) ProcessKBRebuildIndex(ctx context.Context, t *asynq.Task) error {
	var payload types.KBRebuildIndexPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("%w: unmarshal rebuild index payload: %w", asynq.SkipRetry, err)
	}
	ctx = payload.Initiator.Apply(ctx)
	taskID, _ := asynq.GetTaskID(ctx)
	ctx = withKBActivityTask(ctx, taskID, kbActivityTrigger(ctx))

	tenant, err := s.tenantRepo.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		return fmt.Errorf("load tenant %d: %w", payload.TenantID, err)
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	var run reparseRun
	err = s.forEachRebuildablePage(ctx, payload.TenantID, payload.KnowledgeBaseID, func(ids []string) {
		run.reparse(ids, func(id string) error {
			_, err := s.ReparseKnowledge(ctx, id, nil)
			return err
		})
	})
	logger.Infof(ctx, "Knowledge base index rebuild finished: kb=%s submitted=%d failed=%d",
		payload.KnowledgeBaseID, run.Submitted, run.Failed)
	if err != nil {
		if run.Submitted > 0 {
			return fmt.Errorf("%w: rebuild index of %s stopped after %d document(s): %w",
				asynq.SkipRetry, payload.KnowledgeBaseID, run.Submitted, err)
		}
		return err
	}
	return run.err()
}

// forEachRebuildablePage hands visit the knowledge base's rebuildable ids a
// page at a time, until there are no more or the base is gone. The base is
// looked up before every page because deleting it does not stop a running
// task; without the check a rebuild would keep re-parsing the documents of a
// base that is being torn down.
func (s *knowledgeService) forEachRebuildablePage(
	ctx context.Context, tenantID uint64, kbID string, visit func(ids []string),
) error {
	after := ""
	for {
		if _, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID); err != nil {
			if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
				logger.Infof(ctx, "Knowledge base %s is gone; index rebuild stops", kbID)
				return nil
			}
			return fmt.Errorf("load knowledge base %s: %w", kbID, err)
		}
		ids, err := s.repo.ListRebuildableKnowledgeIDs(ctx, tenantID, kbID, after, kbRebuildIndexPageSize)
		if err != nil {
			return fmt.Errorf("list knowledge to rebuild: %w", err)
		}
		if len(ids) == 0 {
			return nil
		}
		visit(ids)
		after = ids[len(ids)-1]
		if len(ids) < kbRebuildIndexPageSize {
			return nil
		}
	}
}
