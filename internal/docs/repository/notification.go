package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// NotificationRepository is one person's inbox.
//
// Rows belong to their recipient, not to the page they are about, so every
// query here is scoped by user id as well as tenant. A notification about a
// page somebody has since lost access to is still theirs to see and dismiss —
// it says what happened, not what the page says.
type NotificationRepository interface {
	// Create stores one notification.
	Create(ctx context.Context, n *model.Notification) error

	// RecentFor returns the unarchived notifications one person has about one
	// page, newest first, for the merge decision. Bounded: merging only ever
	// looks at what is recent.
	RecentFor(ctx context.Context, tenantID uint64, userID, pageID string, since time.Time) (
		[]*model.Notification, error)

	// Touch folds a new event into an existing notification: it moves back to
	// unread and its timestamp and payload are refreshed, so the inbox shows
	// the conversation as still going.
	Touch(ctx context.Context, tenantID uint64, notificationID string, payload model.JSON) error

	// ListFor returns one person's inbox, newest first.
	ListFor(ctx context.Context, tenantID uint64, userID string, unreadOnly bool,
		after *NotificationCursor, limit int) ([]*model.Notification, error)

	// CountUnread counts what is waiting, for a badge.
	CountUnread(ctx context.Context, tenantID uint64, userID string) (int64, error)

	// MarkRead marks some notifications read; an empty list marks all of them.
	MarkRead(ctx context.Context, tenantID uint64, userID string, ids []string) (int64, error)

	// Archive hides notifications from the inbox without deleting them.
	Archive(ctx context.Context, tenantID uint64, userID string, ids []string) (int64, error)

	// DeleteForPage drops every notification about a page; used when it is
	// purged, because there would be nothing left to open.
	DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error
}

// NotificationCursor marks a position in an inbox listing.
type NotificationCursor struct {
	CreatedAt time.Time
	ID        string
}

// MaxNotificationPage caps one page of an inbox.
const MaxNotificationPage = 50

// MaxMergeCandidates bounds how many recent notifications the merge decision
// looks at. Merging only cares about the last few minutes of one page.
const MaxMergeCandidates = 20

type notificationRepository struct{ db *gorm.DB }

func (r *notificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if n.ID == "" {
		n.ID = NewID()
	}
	if len(n.Payload) == 0 {
		n.Payload = model.JSON("{}")
	}
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *notificationRepository) RecentFor(ctx context.Context, tenantID uint64,
	userID, pageID string, since time.Time,
) ([]*model.Notification, error) {
	if userID == "" || pageID == "" {
		return nil, nil
	}
	var rows []*model.Notification
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND page_id = ? AND archived_at IS NULL AND created_at >= ?",
			tenantID, userID, pageID, since).
		Order("created_at DESC").Limit(MaxMergeCandidates).Find(&rows).Error
	return rows, err
}

func (r *notificationRepository) Touch(ctx context.Context, tenantID uint64, notificationID string,
	payload model.JSON,
) error {
	updates := map[string]any{
		"created_at": now(),
		// Back to unread: the conversation moved on, and an inbox that leaves
		// it marked read would hide the fact.
		"read_at": nil,
	}
	if len(payload) > 0 {
		updates["payload"] = payload
	}
	res := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("tenant_id = ? AND id = ?", tenantID, notificationID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *notificationRepository) ListFor(ctx context.Context, tenantID uint64, userID string,
	unreadOnly bool, after *NotificationCursor, limit int,
) ([]*model.Notification, error) {
	if limit <= 0 || limit > MaxNotificationPage {
		limit = MaxNotificationPage
	}
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND archived_at IS NULL", tenantID, userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	if after != nil {
		q = q.Where("created_at < ? OR (created_at = ? AND id < ?)",
			after.CreatedAt, after.CreatedAt, after.ID)
	}
	var rows []*model.Notification
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *notificationRepository) CountUnread(ctx context.Context, tenantID uint64, userID string) (
	int64, error,
) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("tenant_id = ? AND user_id = ? AND read_at IS NULL AND archived_at IS NULL",
			tenantID, userID).
		Count(&count).Error
	return count, err
}

func (r *notificationRepository) MarkRead(ctx context.Context, tenantID uint64, userID string,
	ids []string,
) (int64, error) {
	// Scoped by user as well as tenant: naming somebody else's notification
	// must not mark it read.
	q := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("tenant_id = ? AND user_id = ? AND read_at IS NULL", tenantID, userID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	res := q.Update("read_at", now())
	return res.RowsAffected, res.Error
}

func (r *notificationRepository) Archive(ctx context.Context, tenantID uint64, userID string,
	ids []string,
) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("tenant_id = ? AND user_id = ? AND archived_at IS NULL", tenantID, userID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	res := q.Update("archived_at", now())
	return res.RowsAffected, res.Error
}

func (r *notificationRepository) DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Delete(&model.Notification{}).Error
}
