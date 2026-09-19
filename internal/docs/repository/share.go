package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// ShareRepository persists public links to pages.
//
// Two things here differ from the other repositories in this package and are
// deliberate:
//
//   - GetByKey is the only lookup in the module that does not take a tenant
//     id, because an anonymous visitor has no tenant. The key is globally
//     unique and carries the tenant with it; every caller must then scope
//     what it does next by the tenant the row names.
//
//   - Revoking is a soft act (revoked_at) rather than a delete, so that an
//     audit trail of "this was shared, then it was not" survives, and so a
//     revoked key can never be reissued to a different page.
type ShareRepository interface {
	Create(ctx context.Context, s *model.Share) error
	// GetByKey finds a link by its key, revoked or not, across tenants.
	GetByKey(ctx context.Context, key string) (*model.Share, error)
	Get(ctx context.Context, tenantID uint64, id string) (*model.Share, error)
	ListForPage(ctx context.Context, tenantID uint64, pageID string) ([]*model.Share, error)
	ListForSpace(ctx context.Context, tenantID uint64, spaceID string) ([]*model.Share, error)
	Update(ctx context.Context, s *model.Share) error
	Revoke(ctx context.Context, tenantID uint64, id string, at time.Time) error
	// RevokeForPages turns off every live link to any of the given pages and
	// reports how many it touched. Used when pages leave (trash, purge).
	RevokeForPages(ctx context.Context, tenantID uint64, pageIDs []string, at time.Time) (int64, error)
	// CountViewed records a visit and returns the new count.
	CountViewed(ctx context.Context, id string) (int64, error)
}

type shareRepository struct{ db *gorm.DB }

func (r *shareRepository) Create(ctx context.Context, s *model.Share) error {
	if s.ID == "" {
		s.ID = NewID()
	}
	return translateWriteError(r.db.WithContext(ctx).Create(s).Error)
}

func (r *shareRepository) GetByKey(ctx context.Context, key string) (*model.Share, error) {
	var out model.Share
	err := r.db.WithContext(ctx).Where("key = ?", key).Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (r *shareRepository) Get(ctx context.Context, tenantID uint64, id string) (*model.Share, error) {
	var out model.Share
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}

func (r *shareRepository) ListForPage(ctx context.Context, tenantID uint64, pageID string) (
	[]*model.Share, error,
) {
	var out []*model.Share
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND revoked_at IS NULL", tenantID, pageID).
		Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *shareRepository) ListForSpace(ctx context.Context, tenantID uint64, spaceID string) (
	[]*model.Share, error,
) {
	var out []*model.Share
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ? AND revoked_at IS NULL", tenantID, spaceID).
		Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *shareRepository) Update(ctx context.Context, s *model.Share) error {
	res := r.db.WithContext(ctx).Model(&model.Share{}).
		Where("tenant_id = ? AND id = ?", s.TenantID, s.ID).
		Updates(map[string]any{
			"include_children":   s.IncludeChildren,
			"allow_search_index": s.AllowSearchIndex,
			"password_hash":      s.PasswordHash,
			"expires_at":         s.ExpiresAt,
			"updated_at":         now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *shareRepository) Revoke(ctx context.Context, tenantID uint64, id string, at time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.Share{}).
		Where("tenant_id = ? AND id = ? AND revoked_at IS NULL", tenantID, id).
		Updates(map[string]any{"revoked_at": at.UTC(), "updated_at": now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *shareRepository) RevokeForPages(ctx context.Context, tenantID uint64, pageIDs []string,
	at time.Time,
) (int64, error) {
	if len(pageIDs) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&model.Share{}).
		Where("tenant_id = ? AND page_id IN ? AND revoked_at IS NULL", tenantID, pageIDs).
		Updates(map[string]any{"revoked_at": at.UTC(), "updated_at": now()})
	return res.RowsAffected, res.Error
}

// CountViewed increments in the database rather than reading and writing, so
// concurrent visitors cannot lose each other's counts — and so the returned
// number is the one that visit actually produced, which is what decides
// whether a milestone was reached.
func (r *shareRepository) CountViewed(ctx context.Context, id string) (int64, error) {
	if err := r.db.WithContext(ctx).Model(&model.Share{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return 0, err
	}
	var out model.Share
	if err := r.db.WithContext(ctx).Select("view_count").Where("id = ?", id).Take(&out).Error; err != nil {
		return 0, mapNotFound(err)
	}
	return out.ViewCount, nil
}
