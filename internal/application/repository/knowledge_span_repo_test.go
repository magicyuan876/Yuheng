package repository

import (
	"context"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSpanTestRepo(t *testing.T) (KnowledgeSpanRepository, *gorm.DB) {
	t.Helper()
	db := pgtest.New(t)
	return NewKnowledgeSpanRepository(db), db
}

// TestKnowledgeSpanRepo_UpsertAndList covers the round-trip: a Begin
// followed by an End for the same (kid, attempt, span_id) updates the
// existing row in place, leaving exactly one row queryable by
// ListByAttempt with the latest state.
func TestKnowledgeSpanRepo_UpsertAndList(t *testing.T) {
	repo, _ := setupSpanTestRepo(t)
	ctx := context.Background()
	kid := "kid-1"
	now := time.Now()
	row := &types.KnowledgeProcessingSpan{
		KnowledgeID: kid,
		Attempt:     1,
		SpanID:      "span-A",
		Name:        types.StageDocReader,
		Kind:        types.SpanKindStage,
		Status:      types.SpanStatusRunning,
		StartedAt:   &now,
	}
	require.NoError(t, repo.Upsert(ctx, row))

	// Second Upsert with same (kid, attempt, span_id) flips status and
	// sets finished_at — must overwrite, not insert a duplicate.
	finished := now.Add(2 * time.Second)
	row.Status = types.SpanStatusDone
	row.FinishedAt = &finished
	row.DurationMs = 2000
	require.NoError(t, repo.Upsert(ctx, row))

	rows, err := repo.ListByAttempt(ctx, kid, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1, "Upsert must replace, not append")
	assert.Equal(t, types.SpanStatusDone, rows[0].Status)
	assert.Equal(t, int64(2000), rows[0].DurationMs)
}

// TestKnowledgeSpanRepo_NextAttempt confirms that NextAttempt allocates
// a fresh number per knowledge, isolating reparse history. Critical
// because the API layer renders attempt history by this number.
func TestKnowledgeSpanRepo_NextAttempt(t *testing.T) {
	repo, _ := setupSpanTestRepo(t)
	ctx := context.Background()
	kid := "kid-2"

	a, err := repo.NextAttempt(ctx, kid)
	require.NoError(t, err)
	assert.Equal(t, 1, a, "first NextAttempt for fresh knowledge must be 1")

	now := time.Now()
	require.NoError(t, repo.Upsert(ctx, &types.KnowledgeProcessingSpan{
		KnowledgeID: kid, Attempt: 1, SpanID: "root-1",
		Name: "knowledge_processing", Kind: types.SpanKindRoot,
		Status: types.SpanStatusRunning, StartedAt: &now,
	}))
	a, err = repo.NextAttempt(ctx, kid)
	require.NoError(t, err)
	assert.Equal(t, 2, a, "after one attempt exists, NextAttempt must be 2")

	// Cross-knowledge isolation: a different kid stays at 1.
	other, err := repo.NextAttempt(ctx, "kid-other")
	require.NoError(t, err)
	assert.Equal(t, 1, other, "NextAttempt must scope to the knowledge_id")
}

// TestKnowledgeSpanRepo_CancelDescendants verifies the cascade walk:
// failing a stage cancels every pending/running descendant in its
// subtree, while terminal states (done/skipped/failed) are left intact.
func TestKnowledgeSpanRepo_CancelDescendants(t *testing.T) {
	repo, _ := setupSpanTestRepo(t)
	ctx := context.Background()
	kid := "kid-3"
	now := time.Now()

	// Tree: chunking → embedding (running) → batch[0] (running)
	//                → multimodal (running) → image[0] (done)
	for _, r := range []*types.KnowledgeProcessingSpan{
		{KnowledgeID: kid, Attempt: 1, SpanID: "chunking", Name: types.StageChunking, Kind: types.SpanKindStage, Status: types.SpanStatusRunning, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "embedding", ParentSpanID: "chunking", Name: types.StageEmbedding, Kind: types.SpanKindStage, Status: types.SpanStatusRunning, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "batch0", ParentSpanID: "embedding", Name: "embedding.batch[0]", Kind: types.SpanKindGeneration, Status: types.SpanStatusRunning, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "multimodal", ParentSpanID: "chunking", Name: types.StageMultimodal, Kind: types.SpanKindStage, Status: types.SpanStatusRunning, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "image0", ParentSpanID: "multimodal", Name: "multimodal.image[0]", Kind: types.SpanKindGeneration, Status: types.SpanStatusDone, StartedAt: &now},
	} {
		require.NoError(t, repo.Upsert(ctx, r))
	}

	affected, err := repo.CancelDescendants(ctx, kid, 1, "chunking", "test reason")
	require.NoError(t, err)
	// Expected cancellations: embedding, batch0, multimodal (3 rows).
	// The done image0 is terminal and left alone.
	assert.Equal(t, int64(3), affected, "must cancel exactly the 3 pending/running descendants")

	rows, err := repo.ListByAttempt(ctx, kid, 1)
	require.NoError(t, err)
	statusBy := map[string]string{}
	for _, r := range rows {
		statusBy[r.SpanID] = r.Status
	}
	assert.Equal(t, types.SpanStatusRunning, statusBy["chunking"], "the failed span itself stays untouched (FailSpan layer flips it)")
	assert.Equal(t, types.SpanStatusCancelled, statusBy["embedding"])
	assert.Equal(t, types.SpanStatusCancelled, statusBy["batch0"])
	assert.Equal(t, types.SpanStatusCancelled, statusBy["multimodal"])
	assert.Equal(t, types.SpanStatusDone, statusBy["image0"], "terminal states must not be touched")
}

func TestKnowledgeSpanRepo_CancelOpenSpansByName(t *testing.T) {
	repo, _ := setupSpanTestRepo(t)
	ctx := context.Background()
	kid := "kid-supersede"
	now := time.Now()

	for _, r := range []*types.KnowledgeProcessingSpan{
		{KnowledgeID: kid, Attempt: 1, SpanID: "sum-old", Name: "postprocess.summary", Kind: types.SpanKindSubSpan, Status: types.SpanStatusRunning, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "sum-done", Name: "postprocess.summary", Kind: types.SpanKindSubSpan, Status: types.SpanStatusDone, StartedAt: &now},
		{KnowledgeID: kid, Attempt: 1, SpanID: "q-old", Name: "postprocess.question", Kind: types.SpanKindSubSpan, Status: types.SpanStatusRunning, StartedAt: &now},
	} {
		require.NoError(t, repo.Upsert(ctx, r))
	}

	affected, err := repo.CancelOpenSpansByName(ctx, kid, 1, "postprocess.summary", "TASK_SUPERSEDED", "retry")
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	rows, err := repo.ListByAttempt(ctx, kid, 1)
	require.NoError(t, err)
	statusBy := map[string]string{}
	for _, r := range rows {
		statusBy[r.SpanID] = r.Status
	}
	assert.Equal(t, types.SpanStatusCancelled, statusBy["sum-old"])
	assert.Equal(t, types.SpanStatusDone, statusBy["sum-done"])
	assert.Equal(t, types.SpanStatusRunning, statusBy["q-old"])
}

// TestKnowledgeSpanRepo_ListAttemptIsolation guarantees that different
// attempts of the same knowledge stay queryable independently — the
// foundation for the "show history" UI navigation (?attempt=N).
func TestKnowledgeSpanRepo_ListAttemptIsolation(t *testing.T) {
	repo, _ := setupSpanTestRepo(t)
	ctx := context.Background()
	kid := "kid-history"
	now := time.Now()

	for _, attempt := range []int{1, 2} {
		require.NoError(t, repo.Upsert(ctx, &types.KnowledgeProcessingSpan{
			KnowledgeID: kid, Attempt: attempt, SpanID: "root",
			Name: "knowledge_processing", Kind: types.SpanKindRoot,
			Status: types.SpanStatusDone, StartedAt: &now,
		}))
	}
	a1, err := repo.ListByAttempt(ctx, kid, 1)
	require.NoError(t, err)
	require.Len(t, a1, 1)
	a2, err := repo.ListByAttempt(ctx, kid, 2)
	require.NoError(t, err)
	require.Len(t, a2, 1)

	all, err := repo.ListByAttempt(ctx, kid, 0)
	require.NoError(t, err)
	assert.Len(t, all, 2, "attempt=0 returns all attempts (used by housekeeping)")
}
