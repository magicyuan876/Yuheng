package repository

import (
	"context"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
)

// TreeCursor marks a position in a sibling list for paging: the last row's
// (position, id). Positions are unique per sibling list in practice; the id
// makes the cursor total even when they are not.
type TreeCursor struct {
	Position string
	ID       string
}

// MaxTreePage caps how many children one tree request returns.
const MaxTreePage = 2000

// LockSpace serialises writers of one space's tree for the rest of the
// current transaction: Postgres takes a row lock on the space; SQLite has a
// single writer anyway so nothing is needed. Callers must be inside
// Repositories.Transaction.
func (r *pageRepository) LockSpace(ctx context.Context, tenantID uint64, spaceID string) error {
	if r.db.Dialector.Name() != "postgres" {
		return nil
	}
	var id string
	err := r.db.WithContext(ctx).Raw(
		"SELECT id FROM docs_spaces WHERE tenant_id = ? AND id = ? FOR UPDATE", tenantID, spaceID).Scan(&id).Error
	if err != nil {
		return err
	}
	if id == "" {
		return ErrNotFound
	}
	return nil
}

func (r *pageRepository) GetAnyByShortID(ctx context.Context, tenantID uint64, shortID string) (*model.Page, error) {
	var p model.Page
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND short_id = ?", tenantID, shortID).First(&p).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &p, nil
}

// CreateBatch inserts pages in chunks (duplicate a subtree). Parents must
// precede children in the slice because of the self-referencing foreign key.
func (r *pageRepository) CreateBatch(ctx context.Context, pages []*model.Page) error {
	if len(pages) == 0 {
		return nil
	}
	for _, p := range pages {
		if p.ID == "" {
			p.ID = NewID()
		}
		if p.Status == "" {
			p.Status = model.PagePublished
		}
		if p.SourceRefs == nil {
			p.SourceRefs = model.StringList{}
		}
		if p.ContributorIDs == nil {
			p.ContributorIDs = model.StringList{}
		}
	}
	return translateWriteError(r.db.WithContext(ctx).CreateInBatches(pages, 100).Error)
}

func (r *pageRepository) siblings(ctx context.Context, tenantID uint64, spaceID string, parentID *string) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Page{}).
		Where("tenant_id = ? AND space_id = ? AND deleted_at IS NULL", tenantID, spaceID)
	if parentID == nil {
		return q.Where("parent_id IS NULL")
	}
	return q.Where("parent_id = ?", *parentID)
}

func (r *pageRepository) ListChildrenAfter(ctx context.Context, tenantID uint64, spaceID string, parentID *string,
	after *TreeCursor, limit int,
) ([]*model.Page, error) {
	if limit <= 0 || limit > MaxTreePage {
		limit = MaxTreePage
	}
	q := r.siblings(ctx, tenantID, spaceID, parentID).Select(model.PageSummaryColumns)
	if after != nil {
		q = q.Where("(position > ? OR (position = ? AND id > ?))", after.Position, after.Position, after.ID)
	}
	var out []*model.Page
	err := q.Order("position ASC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *pageRepository) ChildCounts(ctx context.Context, tenantID uint64, parentIDs []string) (map[string]int64,
	error,
) {
	out := make(map[string]int64, len(parentIDs))
	if len(parentIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ParentID string
		N        int64
	}
	err := r.db.WithContext(ctx).Model(&model.Page{}).
		Select("parent_id, COUNT(*) AS n").
		Where("tenant_id = ? AND parent_id IN ? AND deleted_at IS NULL", tenantID, parentIDs).
		Group("parent_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ParentID] = row.N
	}
	return out, nil
}

func (r *pageRepository) LastPosition(ctx context.Context, tenantID uint64, spaceID string,
	parentID *string,
) (string, bool, error) {
	var rows []string
	err := r.siblings(ctx, tenantID, spaceID, parentID).
		Order("position DESC, id DESC").Limit(1).Pluck("position", &rows).Error
	if err != nil || len(rows) == 0 {
		return "", false, err
	}
	return rows[0], true, nil
}

func (r *pageRepository) NextPosition(ctx context.Context, tenantID uint64, spaceID string, parentID *string,
	afterPosition, afterID string,
) (string, bool, error) {
	var rows []string
	err := r.siblings(ctx, tenantID, spaceID, parentID).
		Where("(position > ? OR (position = ? AND id > ?))", afterPosition, afterPosition, afterID).
		Order("position ASC, id ASC").Limit(1).Pluck("position", &rows).Error
	if err != nil || len(rows) == 0 {
		return "", false, err
	}
	return rows[0], true, nil
}

func (r *pageRepository) SetPositions(ctx context.Context, tenantID uint64, positions map[string]string) error {
	if len(positions) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ts := now()
		for id, pos := range positions {
			res := tx.Model(&model.Page{}).Where("tenant_id = ? AND id = ?", tenantID, id).
				Updates(map[string]any{"position": pos, "updated_at": ts})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("%w: page %s", ErrNotFound, id)
			}
		}
		return nil
	})
}

// PurgeOne hard-deletes one trashed page and, through the foreign key
// cascade, everything under it. Returns the IDs removed.
func (r *pageRepository) PurgeOne(ctx context.Context, tenantID uint64, id string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txr := &pageRepository{db: tx}
		p, err := txr.GetAny(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if p.DeletedAt == nil {
			return fmt.Errorf("%w: only trashed pages can be purged", ErrInvalidMove)
		}
		ids, err = txr.SubtreeIDs(ctx, tenantID, id)
		if err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Page{}).Error
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// spaceScopedTables carry a denormalised space_id next to their page_id and
// must follow a page when it changes space.
var spaceScopedTables = []string{
	"docs_page_revisions", "docs_page_access", "docs_comments", "docs_attachments",
	"docs_shares", "docs_watchers", "docs_notifications",
}

// reSpaceRelated points every dependent row of the pages at their new space.
func reSpaceRelated(tx *gorm.DB, tenantID uint64, pageIDs []string, spaceID string) error {
	for _, table := range spaceScopedTables {
		if err := tx.Table(table).Where("tenant_id = ? AND page_id IN ?", tenantID, pageIDs).
			Update("space_id", spaceID).Error; err != nil {
			return fmt.Errorf("docs: re-space %s: %w", table, err)
		}
	}
	return nil
}
