package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// ImportJobRepository persists asynchronous imports into a space.
type ImportJobRepository interface {
	Create(ctx context.Context, job *model.ImportJob) error
	Get(ctx context.Context, tenantID uint64, id string) (*model.ImportJob, error)
	Update(ctx context.Context, job *model.ImportJob) error
	// ListStale returns jobs still pending or running since before the given
	// time: a process restart leaves one behind, and a job that says
	// "running" for ever is worse than one that says it failed.
	ListStale(ctx context.Context, before time.Time, limit int) ([]*model.ImportJob, error)
}

type importJobRepository struct{ db *gorm.DB }

func (r *importJobRepository) Create(ctx context.Context, job *model.ImportJob) error {
	if job.ID == "" {
		job.ID = NewID()
	}
	return translateWriteError(r.db.WithContext(ctx).Create(job).Error)
}

func (r *importJobRepository) Get(ctx context.Context, tenantID uint64, id string) (
	*model.ImportJob, error,
) {
	var out model.ImportJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (r *importJobRepository) Update(ctx context.Context, job *model.ImportJob) error {
	res := r.db.WithContext(ctx).Model(&model.ImportJob{}).
		Where("tenant_id = ? AND id = ?", job.TenantID, job.ID).
		Updates(map[string]any{
			"status":      job.Status,
			"stats":       job.Stats,
			"error":       job.Error,
			"finished_at": job.FinishedAt,
			"updated_at":  now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *importJobRepository) ListStale(ctx context.Context, before time.Time, limit int) (
	[]*model.ImportJob, error,
) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []*model.ImportJob
	err := r.db.WithContext(ctx).
		Where("status IN ? AND created_at < ?",
			[]string{string(model.JobPending), string(model.JobRunning)}, before).
		Order("created_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
