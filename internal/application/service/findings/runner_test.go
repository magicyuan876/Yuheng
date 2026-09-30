package findings

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/repository/retriever/postgres"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// stubDetector reports what it is told to.
type stubDetector struct {
	name  string
	cands []Candidate
	err   error
	calls int
}

func (s *stubDetector) Name() string { return s.name }

func (s *stubDetector) Detect(context.Context, Scope) ([]Candidate, error) {
	s.calls++
	return s.cands, s.err
}

// recordingRepo keeps the last run it was asked to record.
type recordingRepo struct {
	interfaces.KnowledgeFindingRepository
	runs []types.FindingRun
	err  error
}

func (r *recordingRepo) Reconcile(_ context.Context, run types.FindingRun) error {
	r.runs = append(r.runs, run)
	return r.err
}

func pair(subject, related string, score float64) Candidate {
	return Candidate{
		Type: types.FindingTypeDuplicate, Severity: types.FindingSeverityWarning,
		SubjectKnowledgeID: subject, RelatedKnowledgeID: related, Score: score,
	}
}

func TestRunnerParamsNameTheDetectorGroup(t *testing.T) {
	field, ok := reflect.TypeOf(RunnerParams{}).FieldByName("Detectors")
	require.True(t, ok)
	assert.Equal(t, DetectorGroup, field.Tag.Get("group"),
		"the struct tag must name the group detectors are provided into")
}

func TestRunnerRejectsAmbiguousDetectors(t *testing.T) {
	_, err := NewRunner(&recordingRepo{}, &stubDetector{name: "a"}, &stubDetector{name: "a"})
	assert.Error(t, err, "two detectors with one name would resolve each other's findings")
	_, err = NewRunner(&recordingRepo{}, &stubDetector{name: ""})
	assert.Error(t, err)

	r, err := NewRunner(&recordingRepo{}, &stubDetector{name: "zeta"}, nil, &stubDetector{name: "alpha"})
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "zeta"}, r.Detectors(), "detectors run in name order")
}

// Detectors that ran are named in the run — so only their findings can be
// resolved; one that does not apply or fails is not.
func TestRunnerRecordsOnlyTheDetectorsThatRan(t *testing.T) {
	repo := &recordingRepo{}
	ok := &stubDetector{name: "duplicate", cands: []Candidate{pair("a", "b", 0.97)}}
	skipped := &stubDetector{name: "needs-llm", err: ErrUnsupported}
	boom := errors.New("model down")
	failing := &stubDetector{name: "flaky", err: boom}
	r, err := NewRunner(repo, ok, skipped, failing)
	require.NoError(t, err)

	err = r.Run(context.Background(), scopeOf("a"))
	require.ErrorIs(t, err, boom, "a failing detector fails the check so it is retried")
	assert.NotErrorIs(t, err, ErrUnsupported)
	require.Len(t, repo.runs, 1, "what did run is still recorded")
	run := repo.runs[0]
	assert.Equal(t, []string{"duplicate"}, run.Detectors)
	assert.Equal(t, "a", run.KnowledgeID)
	assert.Equal(t, "kb-1", run.KnowledgeBaseID)
	require.Len(t, run.Findings, 1)
	f := run.Findings[0]
	assert.Equal(t, "duplicate", f.Detector)
	assert.Equal(t, PairFingerprint(types.FindingTypeDuplicate, "kb-1", "a", "b"), f.Fingerprint,
		"an empty fingerprint defaults to the pair's")
	require.NotNil(t, f.RelatedKnowledgeID)
	assert.Equal(t, "b", *f.RelatedKnowledgeID)
}

func TestRunnerRecordsNothingWhenNoDetectorApplies(t *testing.T) {
	repo := &recordingRepo{}
	r, err := NewRunner(repo, &stubDetector{name: "duplicate", err: ErrUnsupported})
	require.NoError(t, err)
	require.NoError(t, r.Run(context.Background(), scopeOf("a")))
	assert.Empty(t, repo.runs, "no scan is recorded for a base nothing can check")
}

func TestRunnerKeepsTheStrongestReportOfOneProblemAndRejectsMalformedOnes(t *testing.T) {
	repo := &recordingRepo{}
	d := &stubDetector{name: "duplicate", cands: []Candidate{
		pair("a", "b", 0.96), pair("b", "a", 0.99),
		{Type: types.FindingTypeDuplicate, Severity: "critical", SubjectKnowledgeID: "a"},
		{Type: types.FindingTypeDuplicate, Severity: types.FindingSeverityInfo},
	}}
	r, err := NewRunner(repo, d)
	require.NoError(t, err)
	err = r.Run(context.Background(), scopeOf("a"))
	assert.Error(t, err, "malformed candidates are reported")
	require.Len(t, repo.runs, 1)
	require.Len(t, repo.runs[0].Findings, 1)
	assert.InDelta(t, 0.99, *repo.runs[0].Findings[0].Score, 1e-9)
}

// recordingEnqueuer captures what the trigger sends.
type recordingEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (e *recordingEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.tasks = append(e.tasks, task)
	e.opts = append(e.opts, opts)
	return &asynq.TaskInfo{ID: "t"}, nil
}

func optionValue(opts []asynq.Option, kind asynq.OptionType) any {
	for _, o := range opts {
		if o.Type() == kind {
			return o.Value()
		}
	}
	return nil
}

func TestTriggerSchedulesADebouncedMaintenanceTask(t *testing.T) {
	enq := &recordingEnqueuer{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	trigger := &Trigger{enqueuer: enq, enabled: true, now: func() time.Time { return now }}

	require.NoError(t, trigger.TriggerKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1"))
	require.Len(t, enq.tasks, 1)
	task := enq.tasks[0]
	assert.Equal(t, types.TypeKnowledgeFindings, task.Type())
	var payload types.KnowledgeFindingsPayload
	require.NoError(t, json.Unmarshal(task.Payload(), &payload))
	assert.Equal(t, types.KnowledgeFindingsPayload{TenantID: 7, KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1"}, payload)

	opts := enq.opts[0]
	assert.Equal(t, types.QueueMaintenance, optionValue(opts, asynq.QueueOpt))
	assert.Equal(t, DebounceWindow, optionValue(opts, asynq.ProcessInOpt))
	queue, declared := types.QueueForTaskType(types.TypeKnowledgeFindings)
	require.True(t, declared)
	assert.Equal(t, queue, optionValue(opts, asynq.QueueOpt), "the queue the topology declares")

	// A change a few seconds later lands in the same slot: same task ID.
	firstID := optionValue(opts, asynq.TaskIDOpt)
	now = now.Add(5 * time.Second)
	require.NoError(t, trigger.TriggerKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1"))
	assert.Equal(t, firstID, optionValue(enq.opts[1], asynq.TaskIDOpt))
	// One after the window names a new one; so does another document.
	now = now.Add(DebounceWindow)
	require.NoError(t, trigger.TriggerKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1"))
	assert.NotEqual(t, firstID, optionValue(enq.opts[2], asynq.TaskIDOpt))
	require.NoError(t, trigger.TriggerKnowledgeFindings(context.Background(), 7, "kb-1", "doc-2"))
	assert.NotEqual(t, optionValue(enq.opts[2], asynq.TaskIDOpt), optionValue(enq.opts[3], asynq.TaskIDOpt))
}

func TestTriggerTreatsAPendingCheckAsDone(t *testing.T) {
	trigger := &Trigger{enqueuer: &recordingEnqueuer{err: asynq.ErrTaskIDConflict}, enabled: true, now: time.Now}
	queued, err := trigger.ScheduleKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1", 0)
	require.NoError(t, err, "a conflict means a check is already pending")
	assert.False(t, queued)

	down := errors.New("redis down")
	trigger = &Trigger{enqueuer: &recordingEnqueuer{err: down}, enabled: true, now: time.Now}
	assert.ErrorIs(t, trigger.TriggerKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1"), down)
}

func TestDisabledTriggerSchedulesNothing(t *testing.T) {
	enq := &recordingEnqueuer{}
	trigger := &Trigger{enqueuer: enq, enabled: false, now: time.Now}
	queued, err := trigger.ScheduleKnowledgeFindings(context.Background(), 7, "kb-1", "doc-1", time.Minute)
	require.NoError(t, err)
	assert.False(t, queued)
	assert.Empty(t, enq.tasks)
}

// fakeLookups serve the task's reads.
type fakeLookups struct {
	knowledge *types.Knowledge
	kb        *types.KnowledgeBase
	kbErr     error
}

func (f fakeLookups) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	if f.knowledge == nil {
		return nil, interfaces.ErrKnowledgeNotFound
	}
	return f.knowledge, nil
}

func (f fakeLookups) GetKnowledgeBaseByIDAndTenant(context.Context, string, uint64) (*types.KnowledgeBase, error) {
	if f.kbErr != nil {
		return nil, f.kbErr
	}
	return f.kb, nil
}

func findingsTask(t *testing.T) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(types.KnowledgeFindingsPayload{TenantID: 1, KnowledgeBaseID: "kb-1", KnowledgeID: "a"})
	require.NoError(t, err)
	return asynq.NewTask(types.TypeKnowledgeFindings, payload)
}

func TestTaskHandlerChecksOnlyWhatCanBeChecked(t *testing.T) {
	ready := &types.Knowledge{ID: "a", TenantID: 1, KnowledgeBaseID: "kb-1", ParseStatus: types.ParseStatusCompleted}
	cases := []struct {
		name    string
		lookups fakeLookups
		enabled bool
		runs    int
		wantErr bool
	}{
		{"checked", fakeLookups{knowledge: ready, kb: docBase}, true, 1, false},
		{"switched off", fakeLookups{knowledge: ready, kb: docBase}, false, 0, false},
		{"deleted", fakeLookups{kb: docBase}, true, 0, false},
		{"moved", fakeLookups{knowledge: &types.Knowledge{
			ID: "a", TenantID: 1, KnowledgeBaseID: "kb-2", ParseStatus: types.ParseStatusCompleted,
		}, kb: docBase}, true, 0, false},
		{"being re-indexed", fakeLookups{knowledge: &types.Knowledge{
			ID: "a", TenantID: 1, KnowledgeBaseID: "kb-1", ParseStatus: types.ParseStatusProcessing,
		}, kb: docBase}, true, 0, false},
		{"base deleted", fakeLookups{knowledge: ready, kbErr: repository.ErrKnowledgeBaseNotFound}, true, 0, false},
		{"database down", fakeLookups{knowledge: ready, kbErr: errors.New("connection refused")}, true, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &stubDetector{name: "duplicate"}
			r, err := NewRunner(&recordingRepo{}, d)
			require.NoError(t, err)
			h := newTaskHandler(tc.lookups, tc.lookups, r, tc.enabled)
			err = h.Handle(context.Background(), findingsTask(t))
			if tc.wantErr {
				assert.Error(t, err, "a transient failure is retried")
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.runs, d.calls)
		})
	}
}

// End to end on a real database: stored vectors in, a finding out; the
// document edited so it no longer matches, the finding resolved.
func TestDuplicateCheckEndToEnd(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()
	kbID := uuid.NewString()
	engine := postgres.NewPostgresRetrieveEngineRepository(db)
	chunkRepo := repository.NewChunkRepository(db)
	findingRepo := repository.NewKnowledgeFindingRepository(db)
	finder, ok := engine.(interfaces.SimilarChunkFinder)
	require.True(t, ok)

	newDoc := func(title string) string {
		id := uuid.NewString()
		require.NoError(t, db.Exec(`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source,
			parse_status) VALUES (?, 1, ?, 'manual', ?, 'manual', 'completed')`, id, kbID, title).Error)
		return id
	}
	addPassage := func(doc, content string, vec []float32) string {
		c := &types.Chunk{
			ID: uuid.NewString(), TenantID: 1, KnowledgeBaseID: kbID, KnowledgeID: doc,
			Content: content, ChunkType: types.ChunkTypeText, IsEnabled: true,
		}
		require.NoError(t, chunkRepo.CreateChunks(ctx, []*types.Chunk{c}))
		require.NoError(t, engine.BatchSave(ctx, []*types.IndexInfo{{
			SourceID: c.ID, SourceType: types.ChunkSourceType, ChunkID: c.ID, KnowledgeID: doc,
			KnowledgeBaseID: kbID, Content: content, IsEnabled: true,
		}}, map[string]any{"embedding": map[string][]float32{c.ID: vec}}))
		return c.ID
	}

	original, copied := newDoc("Expense policy"), newDoc("Expense policy (old copy)")
	addPassage(original, long("Expenses are reimbursed monthly"), []float32{1, 0, 0, 0})
	addPassage(original, long("Taxis need a receipt"), []float32{0, 1, 0, 0})
	copyChunk := addPassage(copied, long("Expenses are reimbursed monthly"), []float32{1, 0, 0, 0})

	kb := &types.KnowledgeBase{
		ID: kbID, TenantID: 1, Type: types.KnowledgeBaseTypeDocument,
		IndexingStrategy: types.IndexingStrategy{VectorEnabled: true},
	}
	detector := NewDuplicateDetector(chunkRepo, fixedResolver{finder: finder, ok: true}, 0.95)
	runner, err := NewRunner(findingRepo, detector)
	require.NoError(t, err)
	scope := func(id string) Scope {
		return Scope{TenantID: 1, KnowledgeBase: kb, Knowledge: &types.Knowledge{ID: id, KnowledgeBaseID: kbID}}
	}

	require.NoError(t, runner.Run(ctx, scope(original)))
	list := func(status string) types.KnowledgeFindingFilter {
		return types.KnowledgeFindingFilter{Status: status, Page: 1, PageSize: 10}
	}
	rows, total, err := findingRepo.List(ctx, 1, kbID, list("open"))
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	view := rows[0].View()
	assert.Equal(t, copied, view.Subject.KnowledgeID, "the copy is wholly inside the original")
	assert.Equal(t, "Expense policy (old copy)", view.Subject.Title)
	require.NotNil(t, view.Related)
	assert.Equal(t, original, view.Related.KnowledgeID)
	assert.Equal(t, types.FindingSeverityWarning, view.Severity)
	assert.InDelta(t, 1.0, view.OverlapRatio, 1e-9)
	require.Len(t, view.Evidence, 1)
	assert.Equal(t, copyChunk, view.Evidence[0].SubjectChunkID)

	// Checked from the copy's side: the same single finding.
	require.NoError(t, runner.Run(ctx, scope(copied)))
	_, total, err = findingRepo.List(ctx, 1, kbID, list("all"))
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)

	// The copy is rewritten: its passage no longer matches, the finding is
	// resolved by the system.
	require.NoError(t, engine.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{copyChunk: false}))
	require.NoError(t, db.Exec(`UPDATE chunks SET is_enabled = FALSE WHERE id = ?`, copyChunk).Error)
	addPassage(copied, long("Entirely new guidance on per diems"), []float32{0, 0, 1, 0})
	require.NoError(t, runner.Run(ctx, scope(copied)))
	rows, _, err = findingRepo.List(ctx, 1, kbID, list("resolved"))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, types.FindingResolvedBySystem, *rows[0].ResolvedBy)
}
