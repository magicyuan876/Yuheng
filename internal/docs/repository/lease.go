package repository

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
)

// LeaseRepository persists the exclusive-edit leases a deployment without a
// collaboration service uses in its place: one row per page naming who may currently
// write it and until when.
//
// The table is only ever written by a deployment with no collaboration
// service; a standard deployment merges concurrent edits instead and leaves
// it empty.
type LeaseRepository interface {
	// Get returns the page's lease, expired ones included, so the caller can
	// report who held it last. ErrNotFound when the page was never leased.
	Get(ctx context.Context, tenantID uint64, pageID string) (*model.EditLease, error)
	// Acquire takes or extends a lease in one statement. It succeeds when the
	// page is unleased, when the stored lease has expired, or when the same
	// (user, session) already holds it; otherwise it leaves the row untouched
	// and returns the current holder with held=false.
	Acquire(ctx context.Context, lease model.EditLease, now time.Time) (held *model.EditLease,
		acquired bool, err error)
	// Release drops the lease if this (user, session) holds it. Releasing a
	// lease held by somebody else is a no-op and reports false, so a late
	// release from a superseded session cannot unlock the new holder's page.
	Release(ctx context.Context, tenantID uint64, pageID, userID, sessionID string) (bool, error)
	// PurgeExpired removes leases that expired before the cutoff. Nothing
	// depends on it (an expired row is already ignored); it keeps the table
	// from growing in a long-lived deployment without collab.
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}

type leaseRepository struct{ db *gorm.DB }

func (r *leaseRepository) Get(ctx context.Context, tenantID uint64, pageID string) (*model.EditLease, error) {
	var row model.EditLease
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Take(&row).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

// acquireSQL is one atomic upsert. The conflict target is the primary key, so
// two callers racing on an unleased page serialise on the unique index and
// exactly one of them inserts. The WHERE on the DO UPDATE branch is what makes
// a takeover legal only when the stored lease has expired or belongs to the
// same session; when it does not hold, no row changes and RowsAffected is 0.
//
// PostgreSQL supports the upsert-with-predicate form since 9.5, and the
// statement is written by hand rather than through GORM's clause builder so
// the predicate is unambiguous.
const acquireSQL = `
INSERT INTO docs_edit_leases (page_id, tenant_id, user_id, session_id, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (page_id) DO UPDATE SET
    user_id    = excluded.user_id,
    session_id = excluded.session_id,
    expires_at = excluded.expires_at
WHERE docs_edit_leases.expires_at <= ?
   OR (docs_edit_leases.user_id = excluded.user_id AND docs_edit_leases.session_id = excluded.session_id)`

func (r *leaseRepository) Acquire(ctx context.Context, lease model.EditLease,
	at time.Time,
) (*model.EditLease, bool, error) {
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = at
	}
	res := r.db.WithContext(ctx).Exec(acquireSQL,
		lease.PageID, lease.TenantID, lease.UserID, lease.SessionID, lease.ExpiresAt, lease.CreatedAt, at)
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected > 0 {
		return &lease, true, nil
	}
	// Somebody else holds a live lease; report who so the caller can say so.
	holder, err := r.Get(ctx, lease.TenantID, lease.PageID)
	if err != nil {
		return nil, false, err
	}
	return holder, false, nil
}

func (r *leaseRepository) Release(ctx context.Context, tenantID uint64, pageID, userID,
	sessionID string,
) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND user_id = ? AND session_id = ?",
			tenantID, pageID, userID, sessionID).
		Delete(&model.EditLease{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *leaseRepository) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("expires_at < ?", before).Delete(&model.EditLease{})
	return res.RowsAffected, res.Error
}
