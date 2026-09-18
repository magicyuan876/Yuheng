package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// RevisionRepository stores the page snapshots history is read from.
//
// Rows are append-only in normal operation: a snapshot records what a page
// said at a moment, and rewriting one would make history a thing that can be
// edited. The only deletions are compaction, which thins old entries by a
// policy, and the cascade when a page is purged.
type RevisionRepository interface {
	// Create stores one snapshot, assigning it the next version number for
	// the page.
	Create(ctx context.Context, rev *model.PageRevision) error

	// Latest returns the newest snapshot of a page, or ErrNotFound.
	// `withContent` decides whether the body is loaded: the snapshot policy
	// needs only the metadata, and a page's body can be large.
	Latest(ctx context.Context, tenantID uint64, pageID string, withContent bool) (*model.PageRevision, error)

	// ListPage returns one page of snapshots, newest first, never with their
	// bodies — a listing shows who and when, and loading every body to draw
	// it would read the whole history of the page.
	ListPage(ctx context.Context, tenantID uint64, pageID string,
		after *RevisionCursor, limit int) ([]*model.PageRevision, error)

	// Get returns one snapshot with its body.
	Get(ctx context.Context, tenantID uint64, revisionID string) (*model.PageRevision, error)

	// All returns every snapshot of a page without bodies, for compaction.
	All(ctx context.Context, tenantID uint64, pageID string) ([]*model.PageRevision, error)

	// DeleteMany removes the given snapshots of one page.
	DeleteMany(ctx context.Context, tenantID uint64, pageID string, ids []string) (int64, error)

	// Retag renames why an existing snapshot was taken.
	//
	// The one case where a row is not append-only, and it exists because of
	// the order two writes arrive in: with a collaboration service, a replace
	// reaches the database through the store callback before the caller that
	// asked for it gets its answer back, so the snapshot of that very content
	// is written as an ordinary interval one moments before anybody can say
	// it was a restore. Renaming it is more honest than writing a second,
	// identical snapshot beside it.
	Retag(ctx context.Context, tenantID uint64, revisionID string,
		reason model.RevisionReason, actor string) error

	// DeleteForPage drops every snapshot of a page; used when it is purged.
	DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error
}

// RevisionCursor marks a position in a history listing: the last row's
// (created_at, id). The id breaks ties, because two snapshots of the same
// page can share a timestamp at the precision the database stores.
type RevisionCursor struct {
	CreatedAt time.Time
	ID        string
}

// MaxRevisionPage caps how many snapshots one listing returns.
const MaxRevisionPage = 100

type revisionRepository struct{ db *gorm.DB }

// revisionListColumns is every column but the body, which a listing never
// needs and which dominates the size of a row.
const revisionListColumns = `id, page_id, tenant_id, space_id, version, title, icon,
	text_content, editor_ids, reason, created_by, created_at`

func (r *revisionRepository) Create(ctx context.Context, rev *model.PageRevision) error {
	if rev.ID == "" {
		rev.ID = NewID()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if rev.Version == 0 {
			// The next version for this page. Inside the transaction, so two
			// concurrent saves cannot pick the same number; the unique index
			// on (page_id, version) is the backstop if they somehow do.
			var highest *int
			if err := tx.Model(&model.PageRevision{}).
				Where("tenant_id = ? AND page_id = ?", rev.TenantID, rev.PageID).
				Select("MAX(version)").Scan(&highest).Error; err != nil {
				return err
			}
			rev.Version = 1
			if highest != nil {
				rev.Version = *highest + 1
			}
		}
		return tx.Create(rev).Error
	})
}

func (r *revisionRepository) Latest(ctx context.Context, tenantID uint64, pageID string,
	withContent bool,
) (*model.PageRevision, error) {
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Order("created_at DESC, id DESC").
		Limit(1)
	if !withContent {
		q = q.Select(revisionListColumns)
	}
	var rev model.PageRevision
	if err := q.Take(&rev).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *revisionRepository) ListPage(ctx context.Context, tenantID uint64, pageID string,
	after *RevisionCursor, limit int,
) ([]*model.PageRevision, error) {
	if limit <= 0 || limit > MaxRevisionPage {
		limit = MaxRevisionPage
	}
	q := r.db.WithContext(ctx).
		Select(revisionListColumns).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID)
	if after != nil {
		// Strictly after the cursor row in (created_at DESC, id DESC) order.
		q = q.Where("created_at < ? OR (created_at = ? AND id < ?)",
			after.CreatedAt, after.CreatedAt, after.ID)
	}
	var rows []*model.PageRevision
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *revisionRepository) Get(ctx context.Context, tenantID uint64, revisionID string) (
	*model.PageRevision, error,
) {
	var rev model.PageRevision
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, revisionID).
		Take(&rev).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *revisionRepository) All(ctx context.Context, tenantID uint64, pageID string) (
	[]*model.PageRevision, error,
) {
	var rows []*model.PageRevision
	err := r.db.WithContext(ctx).
		Select(revisionListColumns).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Order("created_at DESC, id DESC").
		Find(&rows).Error
	return rows, err
}

func (r *revisionRepository) DeleteMany(ctx context.Context, tenantID uint64, pageID string,
	ids []string,
) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Scoped to the page as well as the tenant: an id from another page could
	// otherwise be deleted by naming it here.
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND id IN ?", tenantID, pageID, ids).
		Delete(&model.PageRevision{})
	return res.RowsAffected, res.Error
}

func (r *revisionRepository) Retag(ctx context.Context, tenantID uint64, revisionID string,
	reason model.RevisionReason, actor string,
) error {
	updates := map[string]any{"reason": reason}
	if actor != "" {
		updates["created_by"] = actor
	}
	return r.db.WithContext(ctx).Model(&model.PageRevision{}).
		Where("tenant_id = ? AND id = ?", tenantID, revisionID).
		Updates(updates).Error
}

func (r *revisionRepository) DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Delete(&model.PageRevision{}).Error
}
