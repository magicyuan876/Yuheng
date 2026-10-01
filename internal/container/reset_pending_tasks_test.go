package container

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/magicyuan876/yuheng/internal/application/service"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupResetPendingDB returns an empty database with the production schema.
func setupResetPendingDB(t *testing.T) *gorm.DB {
	t.Helper()
	return pgtest.New(t)
}

// insertKnowledge inserts a knowledges row. The production table requires a
// tenant, a knowledge base, a type, a title and a source; none of them matter
// to the startup reset, so they are filled with fixed values and each test
// passes only the columns it is about.
func insertKnowledge(t *testing.T, db *gorm.DB, fields map[string]interface{}) {
	t.Helper()
	row := map[string]interface{}{
		"tenant_id":         7,
		"knowledge_base_id": "kb-reset",
		"type":              "file",
		"title":             fields["id"],
		"source":            "test",
	}
	for k, v := range fields {
		row[k] = v
	}
	require.NoError(t, db.Table("knowledges").Create(row).Error)
}

// insertRunningSyncLog inserts a running sync_logs row. sync_logs references
// data_sources, so the data source it belongs to is created first.
func insertRunningSyncLog(t *testing.T, db *gorm.DB, id string, startedAt time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO data_sources (id, tenant_id, knowledge_base_id, name, type)
		 VALUES ('ds-reset', 7, 'kb-reset', 'reset source', 'feishu')
		 ON CONFLICT (id) DO NOTHING`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sync_logs (id, data_source_id, tenant_id, status, started_at) VALUES (?, 'ds-reset', 7, ?, ?)`,
		id, types.SyncLogStatusRunning, startedAt,
	).Error)
}

func TestResetPendingTasks_KnowledgeFindThenUpdate(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-stuck", "parse_status": types.ParseStatusProcessing, "updated_at": stale,
	})

	os.Unsetenv("REDIS_ADDR")
	resetPendingTasks(db)

	var status, errMsg string
	require.NoError(t, db.Raw(
		`SELECT parse_status, error_message FROM knowledges WHERE id = ?`, "k-stuck",
	).Row().Scan(&status, &errMsg))
	assert.Equal(t, types.ParseStatusFailed, status)
	assert.Contains(t, errMsg, "application restart")
}

func TestResetPendingTasks_KnowledgeFreshInDistributedMode(t *testing.T) {
	db := setupResetPendingDB(t)
	fresh := time.Now().Add(-5 * time.Minute)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-fresh", "parse_status": types.ParseStatusProcessing, "updated_at": fresh,
	})

	t.Setenv("REDIS_ADDR", "redis:6379")
	resetPendingTasks(db)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "k-fresh",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusProcessing, status)
}

func TestResetPendingTasks_DistributedModePreservesEveryStage(t *testing.T) {
	cases := []struct {
		name        string
		parseStatus string
	}{
		{"pending", types.ParseStatusPending},
		{"docreader_chunking_embedding", types.ParseStatusProcessing},
		{"multimodal", types.ParseStatusProcessing},
		{"postprocess", types.ParseStatusFinalizing},
		{"wiki", types.ParseStatusFinalizing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupResetPendingDB(t)
			stale := time.Now().Add(-2 * time.Hour)
			insertKnowledge(t, db, map[string]interface{}{
				"id": "k-active-span", "parse_status": tc.parseStatus, "updated_at": stale,
			})

			t.Setenv("REDIS_ADDR", "redis:6379")
			resetPendingTasks(db)

			var status string
			require.NoError(t, db.Raw(
				`SELECT parse_status FROM knowledges WHERE id = ?`, "k-active-span",
			).Row().Scan(&status))
			assert.Equal(t, tc.parseStatus, status,
				"distributed tasks belong to Asynq/housekeeping, not startup reset")
		})
	}
}

func TestResetPendingTasks_DistributedSummaryTaskSurvivesRestart(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-summary", "parse_status": types.ParseStatusCompleted,
		"summary_status": types.SummaryStatusProcessing, "updated_at": stale,
	})

	t.Setenv("REDIS_ADDR", "redis:6379")
	resetPendingTasks(db)

	var status string
	require.NoError(t, db.Raw(
		`SELECT summary_status FROM knowledges WHERE id = ?`, "k-summary",
	).Row().Scan(&status))
	assert.Equal(t, types.SummaryStatusProcessing, status)
}

func TestResetPendingTasks_DurableWikiOpSurvivesLiteRestart(t *testing.T) {
	db := setupResetPendingDB(t)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-wiki", "parse_status": types.ParseStatusFinalizing, "pending_subtasks_count": 1,
	})
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload)
		 VALUES (7, ?, ?, 'kb-wiki', 'ingest', 'k-wiki', '{}')`,
		types.TypeWikiIngest, types.TaskScopeKnowledgeBase,
	).Error)

	os.Unsetenv("REDIS_ADDR")
	resetPendingTasks(db)

	var status string
	var pending int
	require.NoError(t, db.Raw(
		`SELECT parse_status, pending_subtasks_count FROM knowledges WHERE id = ?`, "k-wiki",
	).Row().Scan(&status, &pending))
	assert.Equal(t, types.ParseStatusFinalizing, status)
	assert.Equal(t, 1, pending, "resumed wiki worker still owns its finalizing slot")
}

func TestResetPendingTasks_LiteWikiDoesNotHideOtherLostSubtasks(t *testing.T) {
	db := setupResetPendingDB(t)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-wiki-plus-summary", "parse_status": types.ParseStatusFinalizing, "pending_subtasks_count": 2,
	})
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload)
		 VALUES (7, ?, ?, 'kb-wiki', 'ingest', 'k-wiki-plus-summary', '{}')`,
		types.TypeWikiIngest, types.TaskScopeKnowledgeBase,
	).Error)

	os.Unsetenv("REDIS_ADDR")
	resetPendingTasks(db)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "k-wiki-plus-summary",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"a durable wiki op cannot recover another lost in-memory subtask")
}

func TestResetPendingTasks_SyncLogStaleRunning(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	insertRunningSyncLog(t, db, "sync-1", stale)

	t.Setenv("REDIS_ADDR", "redis:6379")
	resetPendingTasks(db)

	var status string
	var finishedAt *time.Time
	require.NoError(t, db.Raw(
		`SELECT status, finished_at FROM sync_logs WHERE id = ?`, "sync-1",
	).Row().Scan(&status, &finishedAt))
	assert.Equal(t, types.SyncLogStatusFailed, status)
	require.NotNil(t, finishedAt)
}

func TestResetPendingTasks_SyncLogLiteMode(t *testing.T) {
	db := setupResetPendingDB(t)
	os.Unsetenv("REDIS_ADDR")
	insertRunningSyncLog(t, db, "sync-lite", time.Now())

	resetPendingTasks(db)

	var status string
	require.NoError(t, db.Raw(
		`SELECT status FROM sync_logs WHERE id = ?`, "sync-lite",
	).Row().Scan(&status))
	assert.Equal(t, types.SyncLogStatusFailed, status)
}

func TestStuckKnowledgeParseQuery_ReuseAfterFindDoesNotBreakUpdate(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	insertKnowledge(t, db, map[string]interface{}{
		"id": "k-reuse", "parse_status": types.ParseStatusProcessing, "updated_at": stale,
	})

	var rows []types.Knowledge
	q := stuckKnowledgeParseQuery(db)
	require.NoError(t, q.Select("id").Find(&rows).Error)
	require.Len(t, rows, 1)

	result := stuckKnowledgeParseQuery(db).Updates(map[string]interface{}{
		"parse_status": types.ParseStatusFailed,
	})
	require.NoError(t, result.Error)
	assert.Equal(t, int64(1), result.RowsAffected)
}

type recordingTaskEnqueuer struct {
	tasks []*asynq.Task
}

func (r *recordingTaskEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	r.tasks = append(r.tasks, task)
	return &asynq.TaskInfo{ID: "test", Type: task.Type()}, nil
}

func TestRecoverPendingWikiTasks_RecreatesOneTriggerPerLaneAndKB(t *testing.T) {
	db := setupResetPendingDB(t)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledge_bases
		 (id, tenant_id, name, embedding_model_id, summary_model_id, storage_backend_id, deleted_at)
		 VALUES (?, ?, 'a', '', '', 'env', NULL), (?, ?, 'b', '', '', 'env', NULL),
		        (?, ?, 'deleted', '', '', 'env', ?)`,
		"kb-a", 7, "kb-b", 8, "kb-deleted", 9, time.Now(),
	).Error)
	rows := []struct {
		tenantID uint64
		taskType string
		kbID     string
		dedup    string
	}{
		{7, types.TypeWikiIngest, "kb-a", "k-1"},
		{7, types.TypeWikiIngest, "kb-a", "k-2"}, // same lane: one trigger
		{7, types.TypeWikiFinalize, "kb-a", "slug-a"},
		{8, types.TypeWikiIngest, "kb-b", "k-3"},
		{9, types.TypeWikiIngest, "kb-deleted", "k-deleted"},
		{10, types.TypeWikiFinalize, "kb-missing", "k-missing"},
	}
	for _, row := range rows {
		require.NoError(t, db.Exec(
			`INSERT INTO task_pending_ops
			 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload)
			 VALUES (?, ?, ?, ?, 'ingest', ?, '{}')`,
			row.tenantID, row.taskType, types.TaskScopeKnowledgeBase, row.kbID, row.dedup,
		).Error)
	}

	recorder := &recordingTaskEnqueuer{}
	recoverPendingWikiTasks(db, recorder)
	require.Len(t, recorder.tasks, 3)

	seen := map[string]service.WikiIngestPayload{}
	for _, task := range recorder.tasks {
		var payload service.WikiIngestPayload
		require.NoError(t, json.Unmarshal(task.Payload(), &payload))
		seen[task.Type()+":"+payload.KnowledgeBaseID] = payload
	}
	assert.Equal(t, uint64(7), seen[types.TypeWikiIngest+":kb-a"].TenantID)
	assert.Equal(t, uint64(7), seen[types.TypeWikiFinalize+":kb-a"].TenantID)
	assert.Equal(t, uint64(8), seen[types.TypeWikiIngest+":kb-b"].TenantID)

	var orphaned int64
	require.NoError(t, db.Model(&types.TaskPendingOp{}).
		Where("scope_id IN ?", []string{"kb-deleted", "kb-missing"}).
		Count(&orphaned).Error)
	assert.Zero(t, orphaned)
}
