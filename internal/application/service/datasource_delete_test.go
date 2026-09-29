package service

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/datasource"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// dataSourceDeleteFixture wires the real data source and sync log
// repositories to a Postgres database with the production schema, so the
// delete paths below run the same soft-delete and sync-log UPDATEs the server
// runs, against the real columns and the sync_logs → data_sources foreign key.
type dataSourceDeleteFixture struct {
	db          *gorm.DB
	dsRepo      interfaces.DataSourceRepository
	syncLogRepo interfaces.SyncLogRepository
	scheduler   *datasource.Scheduler
	ds          *types.DataSource
	pendingLog  *types.SyncLog
	runningLog  *types.SyncLog
}

func newDataSourceDeleteFixture(t *testing.T) *dataSourceDeleteFixture {
	t.Helper()
	db := pgtest.New(t)

	dsRepo := repository.NewDataSourceRepository(db)
	syncLogRepo := repository.NewSyncLogRepository(db)
	ds := &types.DataSource{
		ID:              "ds-delete",
		TenantID:        1,
		KnowledgeBaseID: "kb-delete",
		Name:            "Delete fixture",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
		SyncSchedule:    "0 0 * * * *",
	}
	pendingLog := &types.SyncLog{
		ID:           "log-pending",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       "pending",
	}
	runningLog := &types.SyncLog{
		ID:           "log-running",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
	}
	require.NoError(t, dsRepo.Create(context.Background(), ds))
	require.NoError(t, syncLogRepo.Create(context.Background(), pendingLog))
	require.NoError(t, syncLogRepo.Create(context.Background(), runningLog))

	scheduler := datasource.NewScheduler(dsRepo, syncLogRepo, kbDeleteTaskEnqueuer{})
	require.NoError(t, scheduler.AddOrUpdate(ds))
	require.Equal(t, 1, scheduler.EntryCount())

	return &dataSourceDeleteFixture{
		db:          db,
		dsRepo:      dsRepo,
		syncLogRepo: syncLogRepo,
		scheduler:   scheduler,
		ds:          ds,
		pendingLog:  pendingLog,
		runningLog:  runningLog,
	}
}

func TestDataSourceServiceDeleteCleansUpAfterSoftDelete(t *testing.T) {
	fixture := newDataSourceDeleteFixture(t)
	svc := &DataSourceService{
		dsRepo:      fixture.dsRepo,
		syncLogRepo: fixture.syncLogRepo,
		scheduler:   fixture.scheduler,
	}

	require.NoError(t, svc.DeleteDataSource(context.Background(), fixture.ds.ID))

	_, err := fixture.dsRepo.FindByID(context.Background(), fixture.ds.ID)
	require.EqualError(t, err, "data source not found")
	assert.Equal(t, 0, fixture.scheduler.EntryCount())

	for _, logID := range []string{fixture.pendingLog.ID, fixture.runningLog.ID} {
		log, err := fixture.syncLogRepo.FindByID(context.Background(), logID)
		require.NoError(t, err)
		assert.Equal(t, types.SyncLogStatusCanceled, log.Status)
		require.NotNil(t, log.FinishedAt)
		assert.Equal(t, "data source deleted", log.ErrorMessage)
	}
}

func TestDataSourceServiceDeleteKeepsCleanupStateWhenSoftDeleteFails(t *testing.T) {
	fixture := newDataSourceDeleteFixture(t)
	// A trigger that rejects the soft-delete UPDATE of this one row makes the
	// database, not a mock, fail the delete, so the test proves the service
	// leaves the scheduler entry and the sync logs alone when the row stays.
	require.NoError(t, fixture.db.Exec(`
		CREATE FUNCTION fail_datasource_soft_delete() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced soft delete failure';
		END;
		$$ LANGUAGE plpgsql;
	`).Error)
	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER fail_datasource_soft_delete
		BEFORE UPDATE OF deleted_at ON data_sources
		FOR EACH ROW
		WHEN (NEW.id = 'ds-delete')
		EXECUTE FUNCTION fail_datasource_soft_delete();
	`).Error)
	svc := &DataSourceService{
		dsRepo:      fixture.dsRepo,
		syncLogRepo: fixture.syncLogRepo,
		scheduler:   fixture.scheduler,
	}

	err := svc.DeleteDataSource(context.Background(), fixture.ds.ID)
	require.ErrorContains(t, err, "forced soft delete failure")

	found, err := fixture.dsRepo.FindByID(context.Background(), fixture.ds.ID)
	require.NoError(t, err)
	assert.Equal(t, fixture.ds.ID, found.ID)
	assert.Equal(t, 1, fixture.scheduler.EntryCount())

	pending, err := fixture.syncLogRepo.FindByID(context.Background(), fixture.pendingLog.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", pending.Status)
	running, err := fixture.syncLogRepo.FindByID(context.Background(), fixture.runningLog.ID)
	require.NoError(t, err)
	assert.Equal(t, types.SyncLogStatusRunning, running.Status)
}

func TestDeleteKnowledgeBaseCleansUpPersistedDataSources(t *testing.T) {
	fixture := newDataSourceDeleteFixture(t)
	kbRepo := &kbDeleteKBRepo{fakeKBRepo: *newFakeKBRepo()}
	kbRepo.rows[fixture.ds.KnowledgeBaseID] = &types.KnowledgeBase{
		ID:       fixture.ds.KnowledgeBaseID,
		TenantID: fixture.ds.TenantID,
		Name:     "Delete fixture",
	}
	svc := &knowledgeBaseService{
		repo:        kbRepo,
		asynqClient: kbDeleteTaskEnqueuer{},
		dsRepo:      fixture.dsRepo,
		syncLogRepo: fixture.syncLogRepo,
		dsScheduler: fixture.scheduler,
	}

	err := svc.DeleteKnowledgeBase(
		ctxWithTenantStorage(fixture.ds.TenantID, "local"),
		fixture.ds.KnowledgeBaseID,
	)
	require.NoError(t, err)
	assert.Equal(t, fixture.ds.KnowledgeBaseID, kbRepo.deletedID)

	_, err = fixture.dsRepo.FindByID(context.Background(), fixture.ds.ID)
	require.EqualError(t, err, "data source not found")
	var deleted types.DataSource
	require.NoError(t, fixture.db.Unscoped().First(&deleted, "id = ?", fixture.ds.ID).Error)
	assert.True(t, deleted.DeletedAt.Valid)
	assert.Equal(t, 0, fixture.scheduler.EntryCount())

	for _, logID := range []string{fixture.pendingLog.ID, fixture.runningLog.ID} {
		log, err := fixture.syncLogRepo.FindByID(context.Background(), logID)
		require.NoError(t, err)
		assert.Equal(t, types.SyncLogStatusCanceled, log.Status)
	}
}
