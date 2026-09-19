package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// TemplateRepository persists reusable page bodies.
//
// A template belongs either to one space or to the whole tenant (space_id
// NULL). Listing for a space returns both, because that is the set somebody
// creating a page there can actually choose from.
type TemplateRepository interface {
	Create(ctx context.Context, t *model.Template) error
	Get(ctx context.Context, tenantID uint64, id string) (*model.Template, error)
	// ListFor returns the tenant-wide templates plus, when spaceID is not
	// empty, that space's own.
	ListFor(ctx context.Context, tenantID uint64, spaceID string) ([]*model.Template, error)
	Update(ctx context.Context, t *model.Template) error
	Delete(ctx context.Context, tenantID uint64, id string, at time.Time) error
}

type templateRepository struct{ db *gorm.DB }

func (r *templateRepository) Create(ctx context.Context, t *model.Template) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	return translateWriteError(r.db.WithContext(ctx).Create(t).Error)
}

func (r *templateRepository) Get(ctx context.Context, tenantID uint64, id string) (
	*model.Template, error,
) {
	var out model.Template
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (r *templateRepository) ListFor(ctx context.Context, tenantID uint64, spaceID string) (
	[]*model.Template, error,
) {
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID)
	if spaceID == "" {
		q = q.Where("space_id IS NULL")
	} else {
		q = q.Where("space_id IS NULL OR space_id = ?", spaceID)
	}
	var out []*model.Template
	// Tenant-wide first within a category, then by name: the shared ones are
	// the ones a new member is meant to find.
	err := q.Order("category ASC, space_id IS NULL DESC, name ASC").Find(&out).Error
	return out, err
}

func (r *templateRepository) Update(ctx context.Context, t *model.Template) error {
	res := r.db.WithContext(ctx).Model(&model.Template{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", t.TenantID, t.ID).
		Updates(map[string]any{
			"name":           t.Name,
			"description":    t.Description,
			"icon":           t.Icon,
			"category":       t.Category,
			"content":        t.Content,
			"text_content":   t.TextContent,
			"last_editor_id": t.LastEditorID,
			"updated_at":     now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *templateRepository) Delete(ctx context.Context, tenantID uint64, id string, at time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.Template{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(map[string]any{"deleted_at": at.UTC(), "updated_at": now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
