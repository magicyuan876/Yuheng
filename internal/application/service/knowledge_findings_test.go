package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/application/service/findings"
	werrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFindingRepo keeps findings in memory.
type fakeFindingRepo struct {
	interfaces.KnowledgeFindingRepository
	rows      map[string]*types.KnowledgeFindingRow
	checkable []string
	lastScan  *time.Time
	lastSet   struct{ status, actor, resolution string }
}

func (f *fakeFindingRepo) Get(_ context.Context, _ uint64, kbID, id string) (*types.KnowledgeFindingRow, error) {
	row, ok := f.rows[id]
	if !ok || row.KnowledgeBaseID != kbID {
		return nil, interfaces.ErrFindingNotFound
	}
	copied := *row
	return &copied, nil
}

func (f *fakeFindingRepo) SetStatus(ctx context.Context, tenantID uint64, kbID, id, status,
	actor, resolution string,
) (*types.KnowledgeFindingRow, error) {
	f.lastSet.status, f.lastSet.actor, f.lastSet.resolution = status, actor, resolution
	f.rows[id].Status = status
	return f.Get(ctx, tenantID, kbID, id)
}

func (f *fakeFindingRepo) SetAssignee(ctx context.Context, tenantID uint64, kbID, id, assignee,
	assignedBy string,
) (*types.KnowledgeFindingRow, error) {
	row := f.rows[id]
	row.AssignedBy = nil
	if assignedBy != "" {
		row.AssigneeID, row.AssignedBy = &assignee, &assignedBy
	}
	return f.Get(ctx, tenantID, kbID, id)
}

func (f *fakeFindingRepo) List(_ context.Context, _ uint64, _ string,
	filter types.KnowledgeFindingFilter,
) ([]*types.KnowledgeFindingRow, int64, error) {
	var out []*types.KnowledgeFindingRow
	for _, row := range f.rows {
		if filter.Status == "all" || row.Status == filter.Status {
			out = append(out, row)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeFindingRepo) CountOpenByType(context.Context, uint64, string) (map[string]int64, error) {
	out := map[string]int64{}
	for _, row := range f.rows {
		if row.Status == types.FindingStatusOpen {
			out[row.Type]++
		}
	}
	return out, nil
}

func (f *fakeFindingRepo) LastScanAt(context.Context, uint64, string) (*time.Time, error) {
	return f.lastScan, nil
}

func (f *fakeFindingRepo) ListOpenForKnowledge(context.Context, uint64, string) ([]*types.KnowledgeFindingRow, error) {
	var out []*types.KnowledgeFindingRow
	for _, row := range f.rows {
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeFindingRepo) ListCheckableKnowledgeIDs(context.Context, uint64, string, int) ([]string, error) {
	return f.checkable, nil
}

type fakeFindingKBs struct {
	interfaces.KnowledgeBaseRepository
	kb *types.KnowledgeBase
}

func (f fakeFindingKBs) GetKnowledgeBaseByIDAndTenant(_ context.Context, id string, tenantID uint64,
) (*types.KnowledgeBase, error) {
	if f.kb == nil || f.kb.ID != id || f.kb.TenantID != tenantID {
		return nil, repository.ErrKnowledgeBaseNotFound
	}
	return f.kb, nil
}

type scheduleCall struct {
	knowledgeID string
	delay       time.Duration
}

type fakeScheduler struct {
	calls   []scheduleCall
	pending map[string]bool
}

func (f *fakeScheduler) TriggerKnowledgeFindings(context.Context, uint64, string, string) error {
	return nil
}

func (f *fakeScheduler) ScheduleKnowledgeFindings(_ context.Context, _ uint64, _ string, id string,
	extra time.Duration,
) (bool, error) {
	f.calls = append(f.calls, scheduleCall{id, extra})
	return !f.pending[id], nil
}

type capturedAudit struct {
	interfaces.AuditLogService
	mu   sync.Mutex
	rows []*types.AuditLog
}

func (a *capturedAudit) Log(_ context.Context, e *types.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, e)
	return nil
}

type supportDetector struct{ supported bool }

func (supportDetector) Name() string { return "duplicate" }

func (supportDetector) Detect(context.Context, findings.Scope) ([]findings.Candidate, error) {
	return nil, nil
}

func (d supportDetector) Supports(context.Context, *types.KnowledgeBase) (bool, error) {
	return d.supported, nil
}

type findingServiceFixture struct {
	svc      interfaces.KnowledgeFindingService
	repo     *fakeFindingRepo
	sched    *fakeScheduler
	audit    *capturedAudit
	stewards *fakeStewardRepo
}

func newFindingServiceFixture(t *testing.T, enabled, supported bool) *findingServiceFixture {
	t.Helper()
	related := "doc-b"
	score := 0.97
	repo := &fakeFindingRepo{rows: map[string]*types.KnowledgeFindingRow{
		"f1": {
			KnowledgeFinding: types.KnowledgeFinding{
				ID: "f1", KnowledgeBaseID: "kb-1", Type: types.FindingTypeDuplicate, Detector: "duplicate",
				Severity: types.FindingSeverityWarning, Status: types.FindingStatusOpen,
				SubjectKnowledgeID: "doc-a", RelatedKnowledgeID: &related, Score: &score,
				Details: types.FindingDetails{OverlapRatio: 0.9, Evidence: []types.FindingEvidence{{
					SubjectChunkID: "ca", SubjectExcerpt: "from a", RelatedChunkID: "cb", RelatedExcerpt: "from b",
					Score: score,
				}}},
			},
			SubjectTitle: "Doc A", RelatedTitle: "Doc B",
		},
	}}
	sched := &fakeScheduler{pending: map[string]bool{}}
	audit := &capturedAudit{}
	runner, err := findings.NewRunner(repo, supportDetector{supported: supported})
	require.NoError(t, err)
	kbs := fakeFindingKBs{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 1}}
	stewards := &fakeStewardRepo{
		stewards: map[string]*types.KnowledgeSteward{
			"doc-a": {KnowledgeID: "doc-a", Origin: types.KnowledgeOriginLocal},
			"doc-b": {KnowledgeID: "doc-b", Origin: types.KnowledgeOriginLocal},
		},
		maintainers: map[string]bool{"editor": true},
		members:     map[string]bool{"writer": true},
	}
	users := &fakeUserRepo{users: map[string]*types.User{"editor": {ID: "editor", Username: "Editor"}}}
	svc := &knowledgeFindingService{
		repo: repo, kbs: kbs, trigger: sched, runner: runner, audit: audit, enabled: enabled,
		stewards: stewards, users: users,
	}
	return &findingServiceFixture{svc: svc, repo: repo, sched: sched, audit: audit, stewards: stewards}
}

func asUser(user string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	return context.WithValue(ctx, types.UserIDContextKey, user)
}

func httpCodeOf(t *testing.T, err error) int {
	t.Helper()
	var appErr *werrors.AppError
	require.True(t, errors.As(err, &appErr), "want an AppError, got %v", err)
	return appErr.HTTPCode
}

func TestFindingListDefaultsToOpenAndValidatesStatus(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	page, err := f.svc.List(asUser("u"), 1, "kb-1", types.KnowledgeFindingFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.EqualValues(t, 1, page.Total)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 20, page.PageSize)
	assert.Equal(t, "Doc A", page.Items[0].Subject.Title)

	_, err = f.svc.List(asUser("u"), 1, "kb-1", types.KnowledgeFindingFilter{Status: "closed"})
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err))
}

// Dismissing and reopening are recorded in the base's activity, with who did
// it; moves the detectors own are refused.
func TestFindingStatusChangesAreAuditedAndConstrained(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	ctx := asUser("user-9")
	const intentional = types.FindingResolutionIntentional

	view, err := f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusDismissed, intentional)
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusDismissed, view.Status)
	assert.Equal(t, "user-9", f.repo.lastSet.actor)
	assert.Equal(t, types.FindingResolutionIntentional, f.repo.lastSet.resolution)
	require.Len(t, f.audit.rows, 1)
	row := f.audit.rows[0]
	assert.Equal(t, types.AuditActionFindingStatusChanged, row.Action)
	assert.Equal(t, "knowledge_base", row.ScopeType)
	assert.Equal(t, "kb-1", row.ScopeID)
	assert.Equal(t, "f1", row.TargetID)
	assert.Contains(t, string(row.Details), `"to":"dismissed"`)
	assert.Contains(t, string(row.Details), `"reason":"intentional"`)
	assert.Contains(t, string(row.Details), "Doc A")

	// Setting the status it already has changes and records nothing.
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusDismissed, intentional)
	require.NoError(t, err)
	assert.Len(t, f.audit.rows, 1)

	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusOpen, "")
	require.NoError(t, err)
	assert.Len(t, f.audit.rows, 2)

	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusDismissed, "")
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err), "a dismissal says why")
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusDismissed, "because")
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err))

	f.repo.rows["f1"].Status = types.FindingStatusResolved
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusOpen, "")
	assert.Equal(t, http.StatusConflict, httpCodeOf(t, err), "resolved is the detectors' to change")
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "f1", types.FindingStatusResolved, "")
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err))
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-1", "missing", types.FindingStatusDismissed, intentional)
	assert.Equal(t, http.StatusNotFound, httpCodeOf(t, err))
	_, err = f.svc.UpdateStatus(ctx, 1, "kb-other", "f1", types.FindingStatusDismissed, intentional)
	assert.Equal(t, http.StatusNotFound, httpCodeOf(t, err), "a finding is addressed through its own base")
}

// A finding is assigned by hand to somebody who can act on it, named in the
// response and recorded; handing it back to the routing re-checks at once.
func TestFindingAssignment(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	ctx := asUser("admin-1")

	view, err := f.svc.Assign(ctx, 1, "kb-1", "f1", "editor")
	require.NoError(t, err)
	require.NotNil(t, view.Assignee)
	assert.Equal(t, "Editor", view.Assignee.Username)
	assert.True(t, view.AssignedManually)
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, types.AuditActionFindingAssigned, f.audit.rows[0].Action)

	_, err = f.svc.Assign(ctx, 1, "kb-1", "f1", "writer")
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err),
		"a member who cannot edit the base cannot act on two uploaded files")
	f.stewards.stewards["doc-b"].Origin = types.KnowledgeOriginDocs
	_, err = f.svc.Assign(ctx, 1, "kb-1", "f1", "writer")
	require.NoError(t, err, "a page's writers are members the knowledge base does not know as editors")

	view, err = f.svc.Assign(ctx, 1, "kb-1", "f1", "")
	require.NoError(t, err)
	assert.False(t, view.AssignedManually)

	_, err = f.svc.Assign(asUser("system-1"), 1, "kb-1", "f1", "editor")
	assert.Equal(t, http.StatusForbidden, httpCodeOf(t, err))
	_, err = f.svc.Assign(ctx, 1, "kb-1", "missing", "editor")
	assert.Equal(t, http.StatusNotFound, httpCodeOf(t, err))
}

type deletingKnowledge struct {
	interfaces.KnowledgeService
	deleted []string
}

func (k *deletingKnowledge) DeleteKnowledge(_ context.Context, id string) error {
	k.deleted = append(k.deleted, id)
	return nil
}

type recordingRetirer struct {
	retired     []string
	replacement types.KnowledgeRef
	err         error
}

func (r *recordingRetirer) Retire(_ context.Context, _ uint64, id string, replacement types.KnowledgeRef,
) (string, error) {
	r.retired, r.replacement = append(r.retired, id), replacement
	return types.RetiredExcluded, r.err
}

// Superseding keeps one document and takes the other out the way its source
// requires: an upload is deleted, a page is left to its module, a synced
// entry is refused.
func TestFindingSupersedeRetiresTheOtherDocumentByItsOrigin(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	svc := f.svc.(*knowledgeFindingService)
	knowledge := &deletingKnowledge{}
	retirer := &recordingRetirer{}
	svc.knowledge = knowledge
	svc.retirers = NewKnowledgeRetirers()
	svc.retirers.Register(types.KnowledgeOriginDocs, retirer)
	ctx := asUser("editor")

	res, err := f.svc.Supersede(ctx, 1, "kb-1", "f1", "doc-a")
	require.NoError(t, err)
	assert.Equal(t, &types.SupersedeResult{RetiredKnowledgeID: "doc-b", How: types.RetiredDeleted}, res)
	assert.Equal(t, []string{"doc-b"}, knowledge.deleted, "an upload is deleted")
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, types.AuditActionFindingSuperseded, f.audit.rows[0].Action)
	assert.Contains(t, string(f.audit.rows[0].Details), `"kept_title":"Doc A"`)

	f.stewards.stewards["doc-a"].Origin = types.KnowledgeOriginDocs
	res, err = f.svc.Supersede(ctx, 1, "kb-1", "f1", "doc-b")
	require.NoError(t, err)
	assert.Equal(t, types.RetiredExcluded, res.How)
	assert.Equal(t, []string{"doc-a"}, retirer.retired, "a page is taken out by its module")
	assert.Equal(t, types.KnowledgeRef{KnowledgeID: "doc-b", Title: "Doc B"}, retirer.replacement)

	f.stewards.stewards["doc-b"].Origin = types.KnowledgeOriginSynced
	_, err = f.svc.Supersede(ctx, 1, "kb-1", "f1", "doc-a")
	assert.Equal(t, http.StatusConflict, httpCodeOf(t, err), "the next sync would bring it back")

	_, err = f.svc.Supersede(ctx, 1, "kb-1", "f1", "doc-z")
	assert.Equal(t, http.StatusBadRequest, httpCodeOf(t, err), "the kept document is one of the two")
	f.repo.rows["f1"].Status = types.FindingStatusDismissed
	_, err = f.svc.Supersede(ctx, 1, "kb-1", "f1", "doc-a")
	assert.Equal(t, http.StatusConflict, httpCodeOf(t, err))
}

// A source that registered no retirer cannot be superseded from here.
func TestFindingSupersedeWithoutARetirerIsRefused(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	svc := f.svc.(*knowledgeFindingService)
	svc.retirers = NewKnowledgeRetirers()
	f.stewards.stewards["doc-b"].Origin = types.KnowledgeOriginDocs
	_, err := f.svc.Supersede(asUser("editor"), 1, "kb-1", "f1", "doc-a")
	assert.Equal(t, http.StatusConflict, httpCodeOf(t, err))
}

func TestFindingSummaryReportsCountsAndCapability(t *testing.T) {
	scanned := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	f := newFindingServiceFixture(t, true, true)
	f.repo.lastScan = &scanned
	sum, err := f.svc.Summary(asUser("u"), 1, "kb-1")
	require.NoError(t, err)
	assert.EqualValues(t, 1, sum.OpenTotal)
	assert.Equal(t, map[string]int64{types.FindingTypeDuplicate: 1}, sum.OpenByType)
	assert.Equal(t, &scanned, sum.LastScanAt)
	assert.True(t, sum.Enabled)
	assert.True(t, sum.Supported)

	f = newFindingServiceFixture(t, false, false)
	sum, err = f.svc.Summary(asUser("u"), 1, "kb-1")
	require.NoError(t, err)
	assert.False(t, sum.Enabled)
	assert.False(t, sum.Supported)
	assert.Nil(t, sum.LastScanAt)

	_, err = f.svc.Summary(asUser("u"), 2, "kb-1")
	assert.Equal(t, http.StatusNotFound, httpCodeOf(t, err), "the base is looked up in its owner's tenant")
}

// A full re-check schedules every indexed document, spread out, and counts
// only what was newly scheduled.
func TestFindingScanSchedulesEveryDocumentOnce(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	f.repo.checkable = []string{"doc-a", "doc-b", "doc-c"}
	f.sched.pending["doc-b"] = true

	queued, err := f.svc.Scan(asUser("admin"), 1, "kb-1")
	require.NoError(t, err)
	assert.Equal(t, 2, queued, "a document with a check already pending is not counted twice")
	require.Len(t, f.sched.calls, 3)
	assert.Zero(t, f.sched.calls[0].delay)
	assert.Greater(t, f.sched.calls[2].delay, f.sched.calls[1].delay, "the checks are staggered")
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, types.AuditActionFindingScanRequested, f.audit.rows[0].Action)

	for _, tc := range []struct {
		name               string
		enabled, supported bool
	}{{"switched off", false, true}, {"unsupported base", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFindingServiceFixture(t, tc.enabled, tc.supported)
			f.repo.checkable = []string{"doc-a"}
			queued, err := f.svc.Scan(asUser("admin"), 1, "kb-1")
			require.NoError(t, err)
			assert.Zero(t, queued)
			assert.Empty(t, f.sched.calls)
		})
	}
}

// Asked about a document, every finding names it as the subject, with the
// evidence turned round to match.
func TestOpenFindingsAreOrientedTowardsTheDocumentAskedAbout(t *testing.T) {
	f := newFindingServiceFixture(t, true, true)
	views, err := f.svc.OpenForKnowledge(asUser("u"), 1, "doc-b")
	require.NoError(t, err)
	require.Len(t, views, 1)
	v := views[0]
	assert.Equal(t, "doc-b", v.Subject.KnowledgeID)
	assert.Equal(t, "Doc B", v.Subject.Title)
	require.NotNil(t, v.Related)
	assert.Equal(t, "doc-a", v.Related.KnowledgeID)
	assert.Equal(t, "cb", v.Evidence[0].SubjectChunkID)
	assert.Equal(t, "from b", v.Evidence[0].SubjectExcerpt)

	views, err = f.svc.OpenForKnowledge(asUser("u"), 1, "doc-a")
	require.NoError(t, err)
	assert.Equal(t, "doc-a", views[0].Subject.KnowledgeID)
	assert.Equal(t, "ca", views[0].Evidence[0].SubjectChunkID)
}
