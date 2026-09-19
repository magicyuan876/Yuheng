package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// ExportJobRepository persists asynchronous space exports.
type ExportJobRepository interface {
	Create(ctx context.Context, job *model.ExportJob) error
	Get(ctx context.Context, tenantID uint64, id string) (*model.ExportJob, error)
	Update(ctx context.Context, job *model.ExportJob) error
	// ListExpired returns finished jobs whose archive is due for deletion.
	ListExpired(ctx context.Context, before time.Time, limit int) ([]*model.ExportJob, error)
	// ListStale returns jobs still pending or running since before the given
	// time. A process restart mid-export leaves one behind, and a job that
	// says "running" for ever is worse than one that says it failed.
	ListStale(ctx context.Context, before time.Time, limit int) ([]*model.ExportJob, error)
	// Delete removes a job row once its archive is gone.
	Delete(ctx context.Context, tenantID uint64, id string) error
}

type exportJobRepository struct{ db *gorm.DB }

func (r *exportJobRepository) Create(ctx context.Context, job *model.ExportJob) error {
	if job.ID == "" {
		job.ID = NewID()
	}
	return translateWriteError(r.db.WithContext(ctx).Create(job).Error)
}

func (r *exportJobRepository) Get(ctx context.Context, tenantID uint64, id string) (
	*model.ExportJob, error,
) {
	var out model.ExportJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (r *exportJobRepository) Update(ctx context.Context, job *model.ExportJob) error {
	res := r.db.WithContext(ctx).Model(&model.ExportJob{}).
		Where("tenant_id = ? AND id = ?", job.TenantID, job.ID).
		Updates(map[string]any{
			"status":      job.Status,
			"result_path": job.ResultPath,
			"file_name":   job.FileName,
			"stats":       job.Stats,
			"error":       job.Error,
			"finished_at": job.FinishedAt,
			"expires_at":  job.ExpiresAt,
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

func (r *exportJobRepository) ListExpired(ctx context.Context, before time.Time, limit int) (
	[]*model.ExportJob, error,
) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []*model.ExportJob
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", before).
		Order("expires_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

func (r *exportJobRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.ExportJob{}).Error
}

func (r *exportJobRepository) ListStale(ctx context.Context, before time.Time, limit int) (
	[]*model.ExportJob, error,
) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []*model.ExportJob
	err := r.db.WithContext(ctx).
		Where("status IN ? AND created_at < ?",
			[]string{string(model.JobPending), string(model.JobRunning)}, before).
		Order("created_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
