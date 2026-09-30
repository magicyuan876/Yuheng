package findings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// DebounceWindow is how long a check waits after a change before it runs, and
// the window within which further changes to the same document join it.
//
// A document is often changed several times in a row — an upload followed by
// its summary, a page saved every few seconds while somebody types, a batch
// re-parse — and checking after each would repeat the same work.
const DebounceWindow = 30 * time.Second

// findingsTaskTimeout bounds one check. The work is database queries; a check
// that runs this long is stuck, not busy.
const findingsTaskTimeout = 10 * time.Minute

// findingsTaskMaxRetry is how often a failed check is retried, as for the
// other maintenance tasks.
const findingsTaskMaxRetry = 3

// Trigger schedules checks, debounced per document. It implements
// interfaces.KnowledgeFindingsTrigger.
//
// The debounce is a deterministic asynq task ID: the document and the
// DebounceWindow-sized slot its check would run in. Every change whose check
// would run in the same slot names the same task, and asynq (or the Redis-less
// executor) refuses the second as a conflict. A change can only join a check
// that runs after it — it lands in the slot of the check that was already
// scheduled only when its own run time falls before that slot ends, which
// means the change came before the check starts — so no change is ever
// swallowed by a check that had already read the document. asynq's Unique
// option was the other candidate; it holds its lock while the task runs, and a
// change made during the check would then be dropped.
type Trigger struct {
	enqueuer interfaces.TaskEnqueuer
	enabled  bool
	window   time.Duration
	now      func() time.Time
}

// NewTrigger returns the trigger. With the feature switched off it schedules
// nothing.
func NewTrigger(enqueuer interfaces.TaskEnqueuer, cfg *config.Config) *Trigger {
	return &Trigger{
		enqueuer: enqueuer, enabled: cfg != nil && cfg.Findings.IsEnabled(),
		window: DebounceWindow, now: time.Now,
	}
}

// WithWindow returns a copy of the trigger debouncing over d instead of
// DebounceWindow; tests use it to see a check run without waiting half a
// minute. d is rounded down to whole seconds, with a floor of one.
func (t *Trigger) WithWindow(d time.Duration) *Trigger {
	copied := *t
	copied.window = max(d.Truncate(time.Second), time.Second)
	return &copied
}

// TriggerKnowledgeFindings implements interfaces.KnowledgeFindingsTrigger.
func (t *Trigger) TriggerKnowledgeFindings(ctx context.Context, tenantID uint64, kbID, knowledgeID string) error {
	_, err := t.ScheduleKnowledgeFindings(ctx, tenantID, kbID, knowledgeID, 0)
	return err
}

// ScheduleKnowledgeFindings implements interfaces.KnowledgeFindingsTrigger.
func (t *Trigger) ScheduleKnowledgeFindings(_ context.Context, tenantID uint64, kbID, knowledgeID string,
	extraDelay time.Duration,
) (bool, error) {
	if !t.enabled || t.enqueuer == nil {
		return false, nil
	}
	if tenantID == 0 || kbID == "" || knowledgeID == "" {
		return false, errors.New("findings: a check needs a tenant, a knowledge base and a knowledge entry")
	}
	payload, err := json.Marshal(types.KnowledgeFindingsPayload{
		TenantID: tenantID, KnowledgeBaseID: kbID, KnowledgeID: knowledgeID,
	})
	if err != nil {
		return false, err
	}
	if extraDelay < 0 {
		extraDelay = 0
	}
	window := t.window
	if window <= 0 {
		window = DebounceWindow
	}
	delay := window + extraDelay
	slot := t.now().Add(delay).Unix() / int64(window/time.Second)
	// The options go to Enqueue rather than NewTask: the Redis-less executor
	// only sees options passed there, and asynq honours both.
	task := asynq.NewTask(types.TypeKnowledgeFindings, payload)
	_, err = t.enqueuer.Enqueue(task,
		asynq.Queue(types.QueueMaintenance),
		asynq.TaskID(fmt.Sprintf("knowledge-findings:%s:%d", knowledgeID, slot)),
		asynq.ProcessIn(delay),
		asynq.MaxRetry(findingsTaskMaxRetry),
		asynq.Timeout(findingsTaskTimeout),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("scheduling the check of knowledge %s: %w", knowledgeID, err)
	}
	return true, nil
}
