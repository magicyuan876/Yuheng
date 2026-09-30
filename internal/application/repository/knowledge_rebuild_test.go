package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertRebuildKnowledge(t *testing.T, db *gorm.DB, tenantID uint64, kbID, status string, deleted bool) string {
	t.Helper()
	id := uuid.New().String()
	var deletedAt any
	if deleted {
		deletedAt = "2026-06-16 12:00:00"
	}
	require.NoError(t, db.Exec(`
		INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source, parse_status, deleted_at)
		VALUES (?, ?, ?, 'file', 'rebuild-test', 'upload', ?, ?)
	`, id, tenantID, kbID, status, deletedAt).Error)
	return id
}

// An index rebuild covers every live entry of the base in any state but a
// draft (never indexed) or one being deleted, and nothing of another base,
// another tenant, or a deleted row.
func TestRebuildableKnowledgeScope(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db)
	ctx := context.Background()
	kbID := uuid.New().String()

	var want []string
	for _, status := range []string{
		types.ParseStatusCompleted, types.ParseStatusFailed, types.ParseStatusPending,
		types.ParseStatusProcessing, types.ParseStatusFinalizing, types.ParseStatusCancelled,
	} {
		want = append(want, insertRebuildKnowledge(t, db, 1, kbID, status, false))
	}
	insertRebuildKnowledge(t, db, 1, kbID, types.ManualKnowledgeStatusDraft, false)
	insertRebuildKnowledge(t, db, 1, kbID, types.ParseStatusDeleting, false)
	insertRebuildKnowledge(t, db, 1, kbID, types.ParseStatusCompleted, true)
	insertRebuildKnowledge(t, db, 1, uuid.New().String(), types.ParseStatusCompleted, false)
	insertRebuildKnowledge(t, db, 2, kbID, types.ParseStatusCompleted, false)

	count, err := repo.CountRebuildableKnowledge(ctx, 1, kbID)
	require.NoError(t, err)
	require.EqualValues(t, len(want), count)

	// Page through two at a time; the pages must add up to the scope, in id
	// order, with no repeats.
	var got []string
	after := ""
	for {
		page, err := repo.ListRebuildableKnowledgeIDs(ctx, 1, kbID, after, 2)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		require.LessOrEqual(t, len(page), 2)
		got = append(got, page...)
		after = page[len(page)-1]
	}
	require.ElementsMatch(t, want, got)
	require.IsIncreasing(t, got)
}
