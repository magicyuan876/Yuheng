package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// WatcherRepository records who wants to hear about a page or a space.
//
// A row is a relationship rather than a setting, which is why muting is a
// column on it rather than a deletion: somebody who mutes a page is saying
// "not now", and taking the row away would also take away the fact that they
// were involved — and sign them up again the next time they commented.
type WatcherRepository interface {
	// Watch records a watcher, or upgrades the reason on an existing one.
	// Returns the row as it now stands.
	Watch(ctx context.Context, w *model.Watcher, upgrade func(existing, incoming model.WatchReason) bool) (
		*model.Watcher, error)

	// Unwatch removes the relationship entirely.
	Unwatch(ctx context.Context, tenantID uint64, userID, pageID string) error

	// SetMuted silences a watched page, or unsilences it.
	SetMuted(ctx context.Context, tenantID uint64, userID, pageID string, muted bool) error

	// ForPage lists everybody watching a page, including the muted.
	//
	// Muted watchers come back rather than being filtered here because the
	// rules decide what muting means per event -- it silences comments and
	// edits but not being named -- and that decision belongs in one place.
	ForPage(ctx context.Context, tenantID uint64, pageID string) ([]*model.Watcher, error)

	// Get returns one person's relationship to one page, or ErrNotFound.
	Get(ctx context.Context, tenantID uint64, userID, pageID string) (*model.Watcher, error)

	// ListForUser lists the pages one person watches, newest first.
	ListForUser(ctx context.Context, tenantID uint64, userID string, limit int) ([]*model.Watcher, error)

	// DeleteForPage drops every watcher of a page; used when it is purged.
	DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error
}

type watcherRepository struct{ db *gorm.DB }

func (r *watcherRepository) Watch(ctx context.Context, w *model.Watcher,
	upgrade func(existing, incoming model.WatchReason) bool,
) (*model.Watcher, error) {
	if w.PageID == nil || *w.PageID == "" || w.UserID == "" {
		return nil, ErrNotFound
	}
	var out *model.Watcher
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.Watcher
		err := tx.Where("tenant_id = ? AND user_id = ? AND page_id = ?",
			w.TenantID, w.UserID, *w.PageID).Take(&existing).Error
		switch {
		case err == gorm.ErrRecordNotFound:
			if w.ID == "" {
				w.ID = NewID()
			}
			if err := tx.Create(w).Error; err != nil {
				return err
			}
			out = w
			return nil
		case err != nil:
			return err
		}

		// Already watching. The reason only changes when the caller says the
		// new one is stronger; see notify.Upgrades for why that is not a
		// simple overwrite.
		if upgrade != nil && upgrade(existing.Reason, w.Reason) {
			if err := tx.Model(&model.Watcher{}).
				Where("id = ?", existing.ID).
				Update("reason", w.Reason).Error; err != nil {
				return err
			}
			existing.Reason = w.Reason
		}
		out = &existing
		return nil
	})
	return out, err
}

func (r *watcherRepository) Unwatch(ctx context.Context, tenantID uint64, userID, pageID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND page_id = ?", tenantID, userID, pageID).
		Delete(&model.Watcher{}).Error
}

func (r *watcherRepository) SetMuted(ctx context.Context, tenantID uint64, userID, pageID string,
	muted bool,
) error {
	var at *time.Time
	if muted {
		moment := now()
		at = &moment
	}
	res := r.db.WithContext(ctx).Model(&model.Watcher{}).
		Where("tenant_id = ? AND user_id = ? AND page_id = ?", tenantID, userID, pageID).
		Update("muted_at", at)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *watcherRepository) ForPage(ctx context.Context, tenantID uint64, pageID string) (
	[]*model.Watcher, error,
) {
	var rows []*model.Watcher
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Find(&rows).Error
	return rows, err
}

func (r *watcherRepository) Get(ctx context.Context, tenantID uint64, userID, pageID string) (
	*model.Watcher, error,
) {
	var row model.Watcher
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND page_id = ?", tenantID, userID, pageID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *watcherRepository) ListForUser(ctx context.Context, tenantID uint64, userID string,
	limit int,
) ([]*model.Watcher, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []*model.Watcher
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND page_id IS NOT NULL", tenantID, userID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *watcherRepository) DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Delete(&model.Watcher{}).Error
}
