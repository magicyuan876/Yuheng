package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// CommentRepository stores a page's comment threads.
//
// A thread is a top-level comment plus its replies, one level deep. That
// bound is in the data rather than only in the service: a reply whose parent
// is itself a reply has no sensible place to be drawn, and a tree that can
// nest arbitrarily becomes one that eventually does.
//
// Deletion is soft throughout. A deleted comment leaves a tombstone so a
// thread does not silently lose the remark a reply was answering, and so the
// search index (T5.1) can drop it rather than having to notice its absence.
type CommentRepository interface {
	// Create stores one comment.
	Create(ctx context.Context, c *model.Comment) error

	// Get returns one live comment, or ErrNotFound.
	Get(ctx context.Context, tenantID uint64, commentID string) (*model.Comment, error)

	// ListForPage returns every live comment of a page, oldest first, so a
	// caller can assemble threads in the order they were written.
	ListForPage(ctx context.Context, tenantID uint64, pageID string, includeResolved bool) (
		[]*model.Comment, error)

	// UpdateBody replaces a comment's body and marks it edited.
	//
	// No plain-text column goes with it: the text is derived from the body
	// wherever it is wanted (comment.ParseBody), so there is one answer rather
	// than a stored copy that can fall behind the body it summarises.
	UpdateBody(ctx context.Context, tenantID uint64, commentID string, body model.JSON) error

	// SetResolved marks a thread resolved or reopens it. `by` is empty when
	// reopening.
	SetResolved(ctx context.Context, tenantID uint64, commentID string, at *time.Time, by string) error

	// SoftDelete marks a comment deleted, and its replies with it: a reply to
	// a remark that is gone has nothing left to answer.
	SoftDelete(ctx context.Context, tenantID uint64, commentID string) (int64, error)

	// CountForPage counts a page's live threads, for a badge.
	CountForPage(ctx context.Context, tenantID uint64, pageID string) (open int64, total int64, err error)

	// DeleteForPage drops every comment of a page; used when it is purged.
	DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error
}

type commentRepository struct{ db *gorm.DB }

func (r *commentRepository) Create(ctx context.Context, c *model.Comment) error {
	if c.ID == "" {
		c.ID = NewID()
	}
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *commentRepository) Get(ctx context.Context, tenantID uint64, commentID string) (
	*model.Comment, error,
) {
	var c model.Comment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, commentID).
		Take(&c).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *commentRepository) ListForPage(ctx context.Context, tenantID uint64, pageID string,
	includeResolved bool,
) ([]*model.Comment, error) {
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND deleted_at IS NULL", tenantID, pageID)
	if !includeResolved {
		// A resolved thread's replies are hidden with it, which is what makes
		// this one condition rather than a pass over the assembled threads.
		q = q.Where(`resolved_at IS NULL AND (parent_id IS NULL OR parent_id IN (?))`,
			r.db.Model(&model.Comment{}).
				Select("id").
				Where("tenant_id = ? AND page_id = ? AND deleted_at IS NULL AND resolved_at IS NULL",
					tenantID, pageID))
	}
	var rows []*model.Comment
	err := q.Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *commentRepository) UpdateBody(ctx context.Context, tenantID uint64, commentID string,
	body model.JSON,
) error {
	res := r.db.WithContext(ctx).Model(&model.Comment{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, commentID).
		Updates(map[string]any{
			"body":      body,
			"edited_at": now(),
			// The anchor is deliberately not touched: editing what a remark
			// says does not move what it is about.
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *commentRepository) SetResolved(ctx context.Context, tenantID uint64, commentID string,
	at *time.Time, by string,
) error {
	updates := map[string]any{"resolved_at": at, "resolved_by": nil}
	if at != nil && by != "" {
		updates["resolved_by"] = by
	}
	res := r.db.WithContext(ctx).Model(&model.Comment{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, commentID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *commentRepository) SoftDelete(ctx context.Context, tenantID uint64, commentID string) (
	int64, error,
) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		at := now()
		res := tx.Model(&model.Comment{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, commentID).
			Update("deleted_at", at)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		affected = res.RowsAffected

		// Replies go with the remark they were answering.
		replies := tx.Model(&model.Comment{}).
			Where("tenant_id = ? AND parent_id = ? AND deleted_at IS NULL", tenantID, commentID).
			Update("deleted_at", at)
		if replies.Error != nil {
			return replies.Error
		}
		affected += replies.RowsAffected
		return nil
	})
	return affected, err
}

func (r *commentRepository) CountForPage(ctx context.Context, tenantID uint64, pageID string) (
	int64, int64, error,
) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.Comment{}).
			Where("tenant_id = ? AND page_id = ? AND deleted_at IS NULL AND parent_id IS NULL",
				tenantID, pageID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var open int64
	if err := base().Where("resolved_at IS NULL").Count(&open).Error; err != nil {
		return 0, 0, err
	}
	return open, total, nil
}

func (r *commentRepository) DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Delete(&model.Comment{}).Error
}
