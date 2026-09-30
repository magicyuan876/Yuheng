package findings

import (
	"context"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Periodic review: knowledge nobody has vouched for in too long.
//
// Most knowledge goes out of date without anybody changing it — the VPN
// address moved, the policy was revised in a meeting — so no comparison of
// documents can notice. A knowledge base with a review period
// (review_interval_days) asks instead: an entry nobody has confirmed or changed
// within the period is due, and a stale finding takes it to its owner. The
// owner confirms it (which restarts the clock) or changes it (which does too),
// and the finding resolves at the check that follows.
//
// Two parts. The detector decides, for one entry, whether it is due; it runs in
// every check, like the duplicate detector, so a review or an edit settles the
// finding at once. The sweep finds the entries whose state changed without
// anybody touching them — the date passed, or the period was changed or
// switched off — and schedules checks for them.

// ReviewDetectorName is the review detector's stored name.
const ReviewDetectorName = "review"

// reviewOverdueWarning is how far past its due date an entry must be for its
// finding to be a warning rather than a note: a whole further period.
const reviewOverdueWarning = 2

// ReviewDetector reports entries past their review date.
type ReviewDetector struct {
	stewards StewardLookup
	now      func() time.Time
}

// NewReviewDetector returns the detector.
func NewReviewDetector(stewards StewardLookup) *ReviewDetector {
	return &ReviewDetector{stewards: stewards, now: time.Now}
}

// Name implements Detector.
func (d *ReviewDetector) Name() string { return ReviewDetectorName }

// Supports implements SupportChecker: a knowledge base with a review period.
func (d *ReviewDetector) Supports(_ context.Context, kb *types.KnowledgeBase) (bool, error) {
	return kb != nil && kb.ReviewIntervalDays > 0, nil
}

// Detect implements Detector.
//
// A base without a review period is not "unsupported" here but simply has
// nothing due: the period may just have been switched off, and the entry's
// open stale finding must then be resolved, which only a detector that ran
// can cause.
func (d *ReviewDetector) Detect(ctx context.Context, scope Scope) ([]Candidate, error) {
	id := scope.KnowledgeID()
	stewards, err := d.stewards.Stewards(ctx, scope.TenantID, []string{id})
	if err != nil {
		return nil, err
	}
	st, ok := stewards[id]
	now := d.now()
	if !ok || !st.Overdue(now) {
		return nil, nil
	}
	due := *st.ReviewDueAt()
	severity := types.FindingSeverityInfo
	if now.After(st.LastVouchedAt().AddDate(0, 0, reviewOverdueWarning*st.ReviewIntervalDays)) {
		severity = types.FindingSeverityWarning
	}
	return []Candidate{{
		Type: types.FindingTypeStale, Severity: severity, Assign: AssignSubjectOwner,
		SubjectKnowledgeID: id, Score: 1,
		Details: types.FindingDetails{
			// One finding per review cycle: a dismissal ("this page never
			// needs reviewing") holds until somebody vouches for the entry
			// and the next due date comes round.
			EvidenceHash: due.UTC().Format(time.RFC3339),
			Extra: map[string]any{
				"due_at":          due.UTC().Format(time.RFC3339),
				"last_vouched_at": st.LastVouchedAt().UTC().Format(time.RFC3339),
				"interval_days":   st.ReviewIntervalDays,
			},
		},
	}}, nil
}

// ReviewSweepRepository finds the entries whose review state changed on its
// own. interfaces.KnowledgeFindingRepository satisfies it.
type ReviewSweepRepository interface {
	ListReviewChecksDue(ctx context.Context, now time.Time, limit int) ([]types.KnowledgeFindingsPayload, error)
}

// ReviewScheduler schedules checks. interfaces.KnowledgeFindingsTrigger
// satisfies it.
type ReviewScheduler interface {
	ScheduleKnowledgeFindings(ctx context.Context, tenantID uint64, kbID, knowledgeID string,
		extraDelay time.Duration) (bool, error)
}

const (
	// ReviewSweepInterval is how often the sweep looks. Review dates are
	// days apart; an hour's delay in noticing one is nothing.
	ReviewSweepInterval = time.Hour
	// reviewSweepBatch bounds one sweep; the rest wait for the next.
	reviewSweepBatch = 500
	// reviewSweepStagger spreads the checks of one sweep out.
	reviewSweepStagger = 200 * time.Millisecond
)

// ReviewSweep schedules the checks that settle review findings nobody's
// action would: entries that came due, and entries whose open finding no
// longer holds because the period changed.
//
// It is a ticker, not a durable job, for the reasons docs.Cleaner gives: a
// missed tick costs an hour and the next one catches up; several instances
// sweeping at once schedule the same checks, which the trigger's debounce
// collapses into one.
type ReviewSweep struct {
	repo     ReviewSweepRepository
	schedule ReviewScheduler
	enabled  bool
	interval time.Duration
	now      func() time.Time

	running   sync.Mutex
	stop      chan struct{}
	done      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	started   bool
}

// NewReviewSweep returns the sweep. Disabled, it never schedules anything.
func NewReviewSweep(repo ReviewSweepRepository, schedule ReviewScheduler, enabled bool) *ReviewSweep {
	return &ReviewSweep{
		repo: repo, schedule: schedule, enabled: enabled, interval: ReviewSweepInterval, now: time.Now,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

// Start begins the loop and returns; Stop ends it.
func (s *ReviewSweep) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		if !s.enabled {
			close(s.done)
			return
		}
		s.started = true
		go s.loop(ctx)
	})
}

// Stop ends the loop and waits for a sweep in progress.
func (s *ReviewSweep) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		if s.started {
			<-s.done
		}
	})
}

func (s *ReviewSweep) loop(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce schedules one round of checks and reports how many it scheduled. A
// round already in progress makes it a no-op.
func (s *ReviewSweep) RunOnce(ctx context.Context) int {
	if !s.enabled || !s.running.TryLock() {
		return 0
	}
	defer s.running.Unlock()
	defer func() {
		// Background tidying must not take the server down.
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[Findings] review sweep panicked: %v", r)
		}
	}()
	due, err := s.repo.ListReviewChecksDue(ctx, s.now(), reviewSweepBatch)
	if err != nil {
		logger.Warnf(ctx, "[Findings] review sweep could not list the entries due: %v", err)
		return 0
	}
	scheduled := 0
	for i, p := range due {
		ok, err := s.schedule.ScheduleKnowledgeFindings(ctx, p.TenantID, p.KnowledgeBaseID, p.KnowledgeID,
			time.Duration(i)*reviewSweepStagger)
		if err != nil {
			logger.Warnf(ctx, "[Findings] review sweep stopped after %d of %d: %v", scheduled, len(due), err)
			break
		}
		if ok {
			scheduled++
		}
	}
	if scheduled > 0 {
		logger.Infof(ctx, "[Findings] review sweep scheduled %d check(s)", scheduled)
	}
	return scheduled
}
