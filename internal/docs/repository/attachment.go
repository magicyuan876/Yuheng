package repository

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
)

// AttachmentRepository persists uploaded files' metadata. The bytes live in
// whichever storage backend the space is bound to; these rows are what says
// who may read them, which page they belong to and how much space they take.
type AttachmentRepository interface {
	Create(ctx context.Context, a *model.Attachment) error
	// Get loads one live attachment.
	Get(ctx context.Context, tenantID uint64, id string) (*model.Attachment, error)
	// GetMany loads live attachments by id, for a page's bound set.
	GetMany(ctx context.Context, tenantID uint64, ids []string) ([]*model.Attachment, error)
	// FindByDigest returns a live attachment of the same space holding the
	// same bytes, so a second upload of one file can point at the object the
	// first one stored instead of writing it again.
	FindByDigest(ctx context.Context, tenantID uint64, spaceID, sha256 string) (*model.Attachment, error)
	// CountByPath reports how many live rows reference one stored object. It
	// is what makes deleting a deduplicated attachment safe: the object goes
	// only when the last row referencing it does.
	CountByPath(ctx context.Context, tenantID uint64, filePath string) (int64, error)
	// BindToPage claims the still-unbound attachments of a space for a page.
	// Attachments already bound elsewhere are left alone, so moving an image
	// between pages by copy-and-paste does not steal it from the original.
	BindToPage(ctx context.Context, tenantID uint64, spaceID, pageID string, ids []string) (int64, error)
	// ListByPage returns a page's live attachments, newest first.
	ListByPage(ctx context.Context, tenantID uint64, pageID string) ([]*model.Attachment, error)
	// SumBytesByPage totals the live attachments bound to each page.
	SumBytesByPage(ctx context.Context, tenantID uint64, pageIDs []string) (map[string]int64, error)
	// SoftDelete marks one attachment deleted; the object stays until a purge.
	SoftDelete(ctx context.Context, tenantID uint64, id string) error
	// ListForPages returns every attachment bound to the given pages, the
	// already-deleted ones included, so a purge can release their objects.
	ListForPages(ctx context.Context, tenantID uint64, pageIDs []string) ([]*model.Attachment, error)
	// DeleteRows removes attachment rows outright.
	DeleteRows(ctx context.Context, tenantID uint64, ids []string) error
	// ListOrphans returns attachments that no page claims and that are older
	// than the cutoff, for the maintenance pool to release.
	ListOrphans(ctx context.Context, before time.Time, limit int) ([]*model.Attachment, error)
}

type attachmentRepository struct{ db *gorm.DB }

func (r *attachmentRepository) Create(ctx context.Context, a *model.Attachment) error {
	if a.ID == "" {
		a.ID = NewID()
	}
	return translateWriteError(r.db.WithContext(ctx).Create(a).Error)
}

func (r *attachmentRepository) Get(ctx context.Context, tenantID uint64, id string) (*model.Attachment, error) {
	var row model.Attachment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Take(&row).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (r *attachmentRepository) GetMany(ctx context.Context, tenantID uint64,
	ids []string,
) ([]*model.Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*model.Attachment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).
		Find(&rows).Error
	return rows, err
}

func (r *attachmentRepository) FindByDigest(ctx context.Context, tenantID uint64,
	spaceID, sha256 string,
) (*model.Attachment, error) {
	if sha256 == "" {
		return nil, ErrNotFound
	}
	var row model.Attachment
	// Deduplication stays inside one space on purpose. Sharing an object
	// across spaces would let one space's cleanup break another space's page.
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ? AND sha256 = ? AND deleted_at IS NULL", tenantID, spaceID, sha256).
		Order("created_at ASC").
		Take(&row).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

func (r *attachmentRepository) CountByPath(ctx context.Context, tenantID uint64, filePath string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Attachment{}).
		Where("tenant_id = ? AND file_path = ? AND deleted_at IS NULL", tenantID, filePath).
		Count(&n).Error
	return n, err
}

func (r *attachmentRepository) BindToPage(ctx context.Context, tenantID uint64,
	spaceID, pageID string, ids []string,
) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&model.Attachment{}).
		Where("tenant_id = ? AND space_id = ? AND id IN ? AND page_id IS NULL AND deleted_at IS NULL",
			tenantID, spaceID, ids).
		Updates(map[string]any{"page_id": pageID, "updated_at": now()})
	return res.RowsAffected, res.Error
}

func (r *attachmentRepository) ListByPage(ctx context.Context, tenantID uint64,
	pageID string,
) ([]*model.Attachment, error) {
	var rows []*model.Attachment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND deleted_at IS NULL", tenantID, pageID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *attachmentRepository) SumBytesByPage(ctx context.Context, tenantID uint64,
	pageIDs []string,
) (map[string]int64, error) {
	out := map[string]int64{}
	if len(pageIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		PageID string
		Total  int64
	}
	err := r.db.WithContext(ctx).Model(&model.Attachment{}).
		Select("page_id", "SUM(size_bytes) AS total").
		Where("tenant_id = ? AND page_id IN ? AND deleted_at IS NULL", tenantID, pageIDs).
		Group("page_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PageID] = row.Total
	}
	return out, nil
}

func (r *attachmentRepository) SoftDelete(ctx context.Context, tenantID uint64, id string) error {
	res := r.db.WithContext(ctx).Model(&model.Attachment{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(map[string]any{"deleted_at": now(), "updated_at": now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *attachmentRepository) ListForPages(ctx context.Context, tenantID uint64,
	pageIDs []string,
) ([]*model.Attachment, error) {
	if len(pageIDs) == 0 {
		return nil, nil
	}
	var rows []*model.Attachment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id IN ?", tenantID, pageIDs).
		Find(&rows).Error
	return rows, err
}

func (r *attachmentRepository) DeleteRows(ctx context.Context, tenantID uint64, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Delete(&model.Attachment{}).Error
}

func (r *attachmentRepository) ListOrphans(ctx context.Context, before time.Time,
	limit int,
) ([]*model.Attachment, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows []*model.Attachment
	err := r.db.WithContext(ctx).
		Where("page_id IS NULL AND deleted_at IS NULL AND created_at < ?", before).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
