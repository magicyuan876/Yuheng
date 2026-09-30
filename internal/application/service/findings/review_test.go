package findings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

var reviewNow = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

func reviewDetector(st *types.KnowledgeSteward) *ReviewDetector {
	d := NewReviewDetector(fakeStewards{byID: map[string]*types.KnowledgeSteward{"a": st}})
	d.now = func() time.Time { return reviewNow }
	return d
}

func steward(daysSinceVouched, interval int) *types.KnowledgeSteward {
	at := reviewNow.AddDate(0, 0, -daysSinceVouched)
	return &types.KnowledgeSteward{
		KnowledgeID: "a", CreatedAt: at.AddDate(-1, 0, 0), ReviewedAt: &at, ReviewIntervalDays: interval,
		OwnerID: "owner", OwnerActive: true,
	}
}

// An entry past its review date is taken to its owner, as a note, and as a
// warning once a whole further period has gone by; the finding is one per
// review cycle.
func TestReviewDetectorReportsWhatIsDue(t *testing.T) {
	out, err := reviewDetector(steward(100, 90)).Detect(context.Background(), scopeOf("a"))
	require.NoError(t, err)
	require.Len(t, out, 1)
	c := out[0]
	assert.Equal(t, types.FindingTypeStale, c.Type)
	assert.Equal(t, types.FindingSeverityInfo, c.Severity)
	assert.Equal(t, AssignSubjectOwner, c.Assign)
	assert.Empty(t, c.RelatedKnowledgeID)
	due := reviewNow.AddDate(0, 0, -10).UTC().Format(time.RFC3339)
	assert.Equal(t, due, c.Details.EvidenceHash, "one finding per cycle")
	assert.Equal(t, due, c.Details.Extra["due_at"])

	out, err = reviewDetector(steward(200, 90)).Detect(context.Background(), scopeOf("a"))
	require.NoError(t, err)
	assert.Equal(t, types.FindingSeverityWarning, out[0].Severity, "a whole period overdue")
}

// Nothing due, no period, or no such entry: nothing reported — which, the
// detector having run, resolves a stale finding the entry had.
func TestReviewDetectorReportsNothingOtherwise(t *testing.T) {
	for name, st := range map[string]*types.KnowledgeSteward{
		"within the period": steward(30, 90),
		"no review period":  steward(900, 0),
	} {
		out, err := reviewDetector(st).Detect(context.Background(), scopeOf("a"))
		require.NoError(t, err, name)
		assert.Empty(t, out, name)
	}
	d := NewReviewDetector(fakeStewards{byID: map[string]*types.KnowledgeSteward{}})
	out, err := d.Detect(context.Background(), scopeOf("gone"))
	require.NoError(t, err)
	assert.Empty(t, out)

	ok, err := d.Supports(context.Background(), &types.KnowledgeBase{ReviewIntervalDays: 30})
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = d.Supports(context.Background(), &types.KnowledgeBase{})
	assert.False(t, ok)
}

type fakeReviewRepo struct {
	due []types.KnowledgeFindingsPayload
	err error
}

func (f fakeReviewRepo) ListReviewChecksDue(context.Context, time.Time, int) ([]types.KnowledgeFindingsPayload, error) {
	return f.due, f.err
}

type countingScheduler struct {
	ids    []string
	delays []time.Duration
	failAt int
}

func (c *countingScheduler) ScheduleKnowledgeFindings(_ context.Context, _ uint64, _, id string, d time.Duration,
) (bool, error) {
	if c.failAt > 0 && len(c.ids) == c.failAt {
		return false, errors.New("queue unavailable")
	}
	c.ids, c.delays = append(c.ids, id), append(c.delays, d)
	return true, nil
}

// A sweep schedules a staggered check per entry due, stops at the first
// failure to schedule, and does nothing while knowledge health is off.
func TestReviewSweepSchedulesChecks(t *testing.T) {
	due := []types.KnowledgeFindingsPayload{
		{TenantID: 1, KnowledgeBaseID: "kb", KnowledgeID: "a"},
		{TenantID: 1, KnowledgeBaseID: "kb", KnowledgeID: "b"},
		{TenantID: 1, KnowledgeBaseID: "kb", KnowledgeID: "c"},
	}
	sched := &countingScheduler{}
	assert.Equal(t, 3, NewReviewSweep(fakeReviewRepo{due: due}, sched, true).RunOnce(context.Background()))
	assert.Equal(t, []string{"a", "b", "c"}, sched.ids)
	assert.Less(t, sched.delays[0], sched.delays[2], "spread out")

	failing := &countingScheduler{failAt: 1}
	assert.Equal(t, 1, NewReviewSweep(fakeReviewRepo{due: due}, failing, true).RunOnce(context.Background()))

	down := NewReviewSweep(fakeReviewRepo{err: errors.New("db down")}, sched, true)
	assert.Zero(t, down.RunOnce(context.Background()))
	off := &countingScheduler{}
	sweep := NewReviewSweep(fakeReviewRepo{due: due}, off, false)
	sweep.Start(context.Background())
	assert.Zero(t, sweep.RunOnce(context.Background()))
	sweep.Stop()
	assert.Empty(t, off.ids)
}
