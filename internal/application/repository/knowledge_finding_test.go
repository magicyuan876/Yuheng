package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

type findingFixture struct {
	db   *gorm.DB
	repo interfaces.KnowledgeFindingRepository
	kb   string
}

func newFindingFixture(t *testing.T) *findingFixture {
	t.Helper()
	db := pgtest.New(t)
	return &findingFixture{db: db, repo: NewKnowledgeFindingRepository(db), kb: uuid.NewString()}
}

// doc seeds a completed knowledge entry of the fixture's base.
func (f *findingFixture) doc(t *testing.T, title string) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, f.db.Exec(`
		INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source, parse_status)
		VALUES (?, 1, ?, 'manual', ?, 'manual', 'completed')`, id, f.kb, title).Error)
	return id
}

func duplicateOf(a, b, hash string, score float64) *types.KnowledgeFinding {
	fingerprint := "duplicate:" + a + "|" + b
	if b < a {
		fingerprint = "duplicate:" + b + "|" + a
	}
	return &types.KnowledgeFinding{
		Type: types.FindingTypeDuplicate, Detector: "duplicate", Severity: types.FindingSeverityWarning,
		Fingerprint: fingerprint, SubjectKnowledgeID: a, RelatedKnowledgeID: &b, Score: &score,
		Details: types.FindingDetails{
			OverlapRatio: 0.8, EvidenceHash: hash,
			Evidence: []types.FindingEvidence{{SubjectChunkID: "ca", RelatedChunkID: "cb", Score: score}},
		},
	}
}

func (f *findingFixture) run(t *testing.T, knowledgeID string, detectors []string,
	findings ...*types.KnowledgeFinding,
) {
	t.Helper()
	require.NoError(t, f.repo.Reconcile(context.Background(), types.FindingRun{
		TenantID: 1, KnowledgeBaseID: f.kb, KnowledgeID: knowledgeID, Detectors: detectors, Findings: findings,
	}))
}

func (f *findingFixture) list(t *testing.T, status string) []*types.KnowledgeFindingRow {
	t.Helper()
	rows, total, err := f.repo.List(context.Background(), 1, f.kb, types.KnowledgeFindingFilter{
		Status: status, Page: 1, PageSize: 100,
	})
	require.NoError(t, err)
	require.EqualValues(t, len(rows), total)
	return rows
}

// A re-run over unchanged content updates the one row; the titles come from
// the documents, and reruns from the other side land on the same fingerprint.
func TestFindingReconcileUpsertsByFingerprint(t *testing.T) {
	f := newFindingFixture(t)
	a, b := f.doc(t, "Travel policy"), f.doc(t, "Travel policy (copy)")

	f.run(t, a, []string{"duplicate"}, duplicateOf(a, b, "h1", 0.97))
	f.run(t, b, []string{"duplicate"}, duplicateOf(a, b, "h1", 0.99))

	rows := f.list(t, "open")
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, types.FindingStatusOpen, row.Status)
	assert.InDelta(t, 0.99, *row.Score, 1e-6, "the later run's evidence replaces the earlier")
	assert.Equal(t, "Travel policy", row.SubjectTitle)
	assert.Equal(t, "Travel policy (copy)", row.RelatedTitle)
	view := row.View()
	require.NotNil(t, view.Related)
	assert.Equal(t, b, view.Related.KnowledgeID)
	assert.Len(t, view.Evidence, 1)
	assert.InDelta(t, 0.8, view.OverlapRatio, 1e-9)

	at, err := f.repo.LastScanAt(context.Background(), 1, f.kb)
	require.NoError(t, err)
	assert.NotNil(t, at, "a run records that the entry was checked")
}

// A finding the detector no longer reports is resolved by the system — but
// only findings of the detectors that actually ran, and never a dismissed one.
func TestFindingReconcileResolvesWhatIsNoLongerReported(t *testing.T) {
	f := newFindingFixture(t)
	a, b, c := f.doc(t, "A"), f.doc(t, "B"), f.doc(t, "C")
	other := duplicateOf(a, c, "hc", 0.96)
	other.Detector = "other-detector"
	f.run(t, a, []string{"duplicate", "other-detector"}, duplicateOf(a, b, "h1", 0.97), other)
	require.Len(t, f.list(t, "open"), 2)

	// Only the duplicate detector ran this time, and reported nothing.
	f.run(t, a, []string{"duplicate"})
	open := f.list(t, "open")
	require.Len(t, open, 1, "a detector that did not run says nothing about its findings")
	assert.Equal(t, "other-detector", open[0].Detector)
	resolved := f.list(t, "resolved")
	require.Len(t, resolved, 1)
	require.NotNil(t, resolved[0].ResolvedBy)
	assert.Equal(t, types.FindingResolvedBySystem, *resolved[0].ResolvedBy)
	assert.NotNil(t, resolved[0].ResolvedAt)

	// Reported again: the problem is back, so the finding is open again.
	f.run(t, b, []string{"duplicate"}, duplicateOf(a, b, "h1", 0.97))
	assert.Len(t, f.list(t, "open"), 2)
	reopened, err := f.repo.Get(context.Background(), 1, f.kb, resolved[0].ID)
	require.NoError(t, err)
	assert.Nil(t, reopened.ResolvedAt)
	assert.Nil(t, reopened.ResolvedBy)
}

// Dismissing is a decision about specific evidence: it survives re-runs that
// find the same thing and gives way when the evidence changes.
func TestDismissedFindingStaysUntilTheEvidenceChanges(t *testing.T) {
	f := newFindingFixture(t)
	ctx := context.Background()
	a, b := f.doc(t, "A"), f.doc(t, "B")
	f.run(t, a, []string{"duplicate"}, duplicateOf(a, b, "h1", 0.97))
	id := f.list(t, "open")[0].ID

	row, err := f.repo.SetStatus(ctx, 1, f.kb, id, types.FindingStatusDismissed, "user-1")
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusDismissed, row.Status)
	require.NotNil(t, row.ResolvedBy)
	assert.Equal(t, "user-1", *row.ResolvedBy)

	// Same evidence, from either side: stays dismissed, with who dismissed it.
	f.run(t, b, []string{"duplicate"}, duplicateOf(a, b, "h1", 0.98))
	row, err = f.repo.Get(ctx, 1, f.kb, id)
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusDismissed, row.Status)
	assert.Equal(t, "user-1", *row.ResolvedBy)

	// Not reported: a dismissed finding is not resolved by the system either.
	f.run(t, a, []string{"duplicate"})
	row, err = f.repo.Get(ctx, 1, f.kb, id)
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusDismissed, row.Status)

	// Different evidence: reopened, and the dismissal is forgotten.
	f.run(t, a, []string{"duplicate"}, duplicateOf(a, b, "h2", 0.98))
	row, err = f.repo.Get(ctx, 1, f.kb, id)
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusOpen, row.Status)
	assert.Nil(t, row.ResolvedBy)

	// Reopening by hand clears the resolution columns too.
	_, err = f.repo.SetStatus(ctx, 1, f.kb, id, types.FindingStatusDismissed, "user-1")
	require.NoError(t, err)
	row, err = f.repo.SetStatus(ctx, 1, f.kb, id, types.FindingStatusOpen, "user-2")
	require.NoError(t, err)
	assert.Equal(t, types.FindingStatusOpen, row.Status)
	assert.Nil(t, row.ResolvedAt)
	assert.Nil(t, row.ResolvedBy)
}

// Findings do not outlive their documents, whichever way a document goes:
// soft delete, hard delete, a move to another base. The trigger removes the
// rows; a check that finishes after the delete does not write them back.
func TestFindingsDisappearWithTheirDocuments(t *testing.T) {
	f := newFindingFixture(t)
	ctx := context.Background()
	a, b, c, d := f.doc(t, "A"), f.doc(t, "B"), f.doc(t, "C"), f.doc(t, "D")
	f.run(t, a, []string{"duplicate"}, duplicateOf(a, b, "h", 0.97), duplicateOf(a, c, "h", 0.97))
	f.run(t, d, []string{"duplicate"}, duplicateOf(d, c, "h", 0.97))
	require.Len(t, f.list(t, "all"), 3)

	count := func(knowledgeID string) int64 {
		var n int64
		require.NoError(t, f.db.Raw(`SELECT COUNT(*) FROM knowledge_findings
			WHERE subject_knowledge_id = ? OR related_knowledge_id = ?`, knowledgeID, knowledgeID).Scan(&n).Error)
		return n
	}

	// Soft delete, the way the knowledge repository does it.
	require.NoError(t, NewKnowledgeRepository(f.db).DeleteKnowledge(ctx, 1, b))
	assert.Zero(t, count(b), "soft-deleting a document removes its findings")
	var scans int64
	require.NoError(t, f.db.Raw(`SELECT COUNT(*) FROM knowledge_finding_scans WHERE knowledge_id = ?`, a).
		Scan(&scans).Error)
	assert.EqualValues(t, 1, scans)

	// A move to another knowledge base.
	require.NoError(t, f.db.Exec(`UPDATE knowledges SET knowledge_base_id = ? WHERE id = ?`,
		uuid.NewString(), d).Error)
	assert.Zero(t, count(d), "moving a document away removes its findings")

	// A hard delete.
	require.NoError(t, f.db.Exec(`DELETE FROM knowledges WHERE id = ?`, a).Error)
	assert.Zero(t, count(a))
	require.NoError(t, f.db.Raw(`SELECT COUNT(*) FROM knowledge_finding_scans WHERE knowledge_id = ?`, a).
		Scan(&scans).Error)
	assert.Zero(t, scans)

	// A late write for the deleted document is refused.
	f.run(t, c, []string{"duplicate"}, duplicateOf(c, b, "h", 0.97))
	assert.Zero(t, count(b), "a check finishing after the delete does not resurrect the finding")

	// Status changes that are not deletes leave findings alone.
	e, g := f.doc(t, "E"), f.doc(t, "G")
	f.run(t, e, []string{"duplicate"}, duplicateOf(e, g, "h", 0.97))
	require.NoError(t, f.db.Exec(`UPDATE knowledges SET parse_status = 'processing' WHERE id = ?`, e).Error)
	assert.EqualValues(t, 1, count(e))

	require.NoError(t, f.repo.DeleteForKnowledge(ctx, 1, g))
	assert.Zero(t, count(e))
}

// Reads filter by the query's own terms and hide what they should.
func TestFindingListFiltersAndCounts(t *testing.T) {
	f := newFindingFixture(t)
	ctx := context.Background()
	a, b, c := f.doc(t, "A"), f.doc(t, "B"), f.doc(t, "C")
	weak := duplicateOf(b, c, "h", 0.95)
	weak.Severity = types.FindingSeverityInfo
	f.run(t, a, []string{"duplicate"}, duplicateOf(a, b, "h", 0.97))
	f.run(t, b, []string{"duplicate"}, weak, duplicateOf(a, b, "h", 0.97))

	rows := f.list(t, "open")
	require.Len(t, rows, 2)
	assert.Equal(t, types.FindingSeverityWarning, rows[0].Severity, "the more urgent finding comes first")

	byDoc, total, err := f.repo.List(ctx, 1, f.kb, types.KnowledgeFindingFilter{
		Status: "open", KnowledgeID: c, Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, byDoc, 1)

	paged, total, err := f.repo.List(ctx, 1, f.kb, types.KnowledgeFindingFilter{Status: "open", Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total, "the total counts every page")
	require.Len(t, paged, 1)
	assert.Equal(t, types.FindingSeverityInfo, paged[0].Severity)

	none, _, err := f.repo.List(ctx, 1, f.kb,
		types.KnowledgeFindingFilter{Type: "contradiction", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, none)

	other, _, err := f.repo.List(ctx, 2, f.kb, types.KnowledgeFindingFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, other, "another tenant sees nothing")

	counts, err := f.repo.CountOpenByType(ctx, 1, f.kb)
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{types.FindingTypeDuplicate: 2}, counts)

	forB, err := f.repo.ListOpenForKnowledge(ctx, 1, b)
	require.NoError(t, err)
	assert.Len(t, forB, 2)

	_, err = f.repo.Get(ctx, 1, f.kb, "no-such-finding")
	assert.ErrorIs(t, err, interfaces.ErrFindingNotFound)

	ids, err := f.repo.ListCheckableKnowledgeIDs(ctx, 1, f.kb, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{a, b, c}, ids)
	require.NoError(t, f.db.Exec(`UPDATE knowledges SET parse_status = 'processing' WHERE id = ?`, c).Error)
	ids, err = f.repo.ListCheckableKnowledgeIDs(ctx, 1, f.kb, 1)
	require.NoError(t, err)
	assert.Len(t, ids, 1, "the limit holds")
}
