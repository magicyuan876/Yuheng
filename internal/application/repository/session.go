package repository

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

// sessionRepository implements the SessionRepository interface
type sessionRepository struct {
	db *gorm.DB
}

func applySessionUserScope(db *gorm.DB, userID string) *gorm.DB {
	if userID == "" {
		return db
	}
	// Empty user_id rows are legacy/API-created tenant-level sessions.
	return db.Where("(user_id = ? OR user_id IS NULL OR user_id = '')", userID)
}

// NewSessionRepository creates a new session repository instance
func NewSessionRepository(db *gorm.DB) interfaces.SessionRepository {
	return &sessionRepository{db: db}
}

// Create creates a new session
func (r *sessionRepository) Create(ctx context.Context, session *types.Session) (*types.Session, error) {
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, err
	}
	// Return the session with generated ID
	return session, nil
}

// Get retrieves a session by ID
func (r *sessionRepository) Get(ctx context.Context, tenantID uint64, userID string, id string) (*types.Session, error) {
	var session types.Session
	err := applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id),
		userID,
	).First(&session).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

// GetByID retrieves a session by tenant and id without user scoping.
func (r *sessionRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.Session, error) {
	var session types.Session
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&session).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

// GetByTenantID retrieves all sessions for a tenant
func (r *sessionRepository) GetByTenantID(ctx context.Context, tenantID uint64, userID string) ([]*types.Session, error) {
	var sessions []*types.Session
	err := applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ?", tenantID),
		userID,
	).Order("updated_at DESC").Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetPagedByTenantID retrieves sessions for a tenant with pagination
func (r *sessionRepository) GetPagedByTenantID(
	ctx context.Context, tenantID uint64, userID string, page *types.Pagination,
) ([]*types.Session, int64, error) {
	var sessions []*types.Session
	var total int64

	// First query the total count
	baseQ := applySessionUserScope(
		r.db.WithContext(ctx).Model(&types.Session{}).Where("tenant_id = ?", tenantID),
		userID,
	)
	err := baseQ.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Then query the paginated data
	err = applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ?", tenantID),
		userID,
	).
		Order("updated_at DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&sessions).Error
	if err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// QueryPaged lists sessions for tenant/user with keyword/source filters and
// pin-aware ordering. Source filtering is description/owner based only: the
// im_channel_sessions table was dropped with the agent infrastructure, so
// legacy IM rows are indistinguishable from ordinary chats and list as web.
func (r *sessionRepository) QueryPaged(
	ctx context.Context, q *types.SessionListQuery,
) ([]*types.SessionListItem, int64, error) {
	titleLikeExpr := "s.title ILIKE ?"
	// Postgres sorts NULLs first under DESC; NULLS LAST keeps a pinned row
	// with no pinned_at from jumping ahead of the ones that have one.
	orderClause := "s.is_pinned DESC, s.pinned_at DESC NULLS LAST, s.updated_at DESC"

	// Base filter shared by count and list queries.
	applyBase := func(db *gorm.DB) *gorm.DB {
		db = db.Where("s.tenant_id = ? AND s.deleted_at IS NULL", q.TenantID)
		if q.UserID != "" {
			db = db.Where("(s.user_id = ? OR s.user_id IS NULL OR s.user_id = '')", q.UserID)
		}
		if kw := strings.TrimSpace(q.Keyword); kw != "" {
			db = db.Where(titleLikeExpr, "%"+escapeLikeKeyword(kw)+"%")
		}
		return db
	}

	// webVisible keeps only ordinary user chats: embed-widget sessions and
	// API-key sessions are excluded (the latter surface only in the admin-only
	// "api" bucket). The user_id NULL check keeps legacy tenant-level web rows
	// visible, since "col NOT LIKE ?" is unknown (not true) for NULL.
	webVisible := func(db *gorm.DB) *gorm.DB {
		return db.Where(
			"(s.description = '' OR s.description NOT LIKE ?) "+
				"AND (s.user_id IS NULL OR (s.user_id NOT LIKE ? AND s.user_id NOT LIKE ?))",
			types.EmbedSessionMarkerPrefix+"%",
			types.SessionOwnerAPITenantKeyPrefix+"%",
			types.SessionOwnerAPIExternalUserPrefix+"%",
		)
	}

	applySource := func(db *gorm.DB) *gorm.DB {
		src := strings.TrimSpace(q.Source)
		lower := strings.ToLower(src)
		embedPrefix := types.EmbedSessionMarkerPrefix
		switch lower {
		case "":
			return db
		case types.SessionSourceAPI:
			// Tenant-wide view of API-key sessions. Requests without an
			// external identity use an api_tenant_key owner; requests with a
			// direct-header or signed-token identity use api_external_user.
			// The service layer already enforced Admin+ and cleared the
			// per-user scope for this source.
			return db.Where(
				"(s.user_id LIKE ? OR s.user_id LIKE ?)",
				types.SessionOwnerAPITenantKeyPrefix+"%",
				types.SessionOwnerAPIExternalUserPrefix+"%",
			)
		case "web":
			return webVisible(db)
		case "embed":
			return db.Where("s.description LIKE ?", embedPrefix+"%")
		default:
			if strings.HasPrefix(lower, "embed:") {
				channelID := strings.TrimSpace(src[len("embed:"):])
				if channelID != "" {
					return db.Where("s.description = ?", embedPrefix+channelID)
				}
			}
			// Unknown sources (including legacy IM platform names) fall back to
			// the web visibility filter: with im_channel_sessions gone there is
			// no way to recognize IM-origin rows, so they list as ordinary chats.
			return webVisible(db)
		}
	}

	var total int64
	countQ := applySource(applyBase(
		r.db.WithContext(ctx).Table("sessions AS s"),
	))
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size < 1 {
		size = 20
	}

	items := make([]*types.SessionListItem, 0)
	rowsQ := applySource(applyBase(
		r.db.WithContext(ctx).Table("sessions AS s"),
	)).
		Select("s.*").
		Order(orderClause).
		Offset((page - 1) * size).
		Limit(size)
	if err := rowsQ.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// SetPinned toggles is_pinned/pinned_at for a single session.
// Scope: must match tenant, and user_id (when provided) to prevent pinning
// other users' sessions. Legacy rows with user_id NULL/” remain mutable
// at the tenant level (same visibility rule as QueryPaged).
//
// Returns the number of rows affected so callers can distinguish "session
// doesn't exist / not visible to this user" (0) from a real DB error.
func (r *sessionRepository) SetPinned(
	ctx context.Context, tenantID uint64, userID string, id string, pinned bool,
) (int64, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"is_pinned":  pinned,
		"updated_at": now,
	}
	if pinned {
		updates["pinned_at"] = now
	} else {
		updates["pinned_at"] = nil
	}

	q := r.db.WithContext(ctx).
		Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", tenantID, id)
	if userID != "" {
		q = q.Where("(user_id = ? OR user_id IS NULL OR user_id = '')", userID)
	}
	res := q.Updates(updates)
	return res.RowsAffected, res.Error
}

// Update updates a session
func (r *sessionRepository) Update(ctx context.Context, session *types.Session, userID string) (int64, error) {
	session.UpdatedAt = time.Now()
	res := applySessionUserScope(r.db.WithContext(ctx).
		Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", session.TenantID, session.ID), userID).
		Updates(map[string]interface{}{
			"title":       session.Title,
			"description": session.Description,
			"updated_at":  session.UpdatedAt,
		})
	return res.RowsAffected, res.Error
}

// SetOwnerID assigns sessions.user_id for a tenant-scoped row.
func (r *sessionRepository) SetOwnerID(ctx context.Context, tenantID uint64, id, ownerID string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"user_id":    ownerID,
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// UpdateLastRequestState writes only the last_request_state column and bumps
// updated_at. We deliberately bypass the regular Update path so the call
// doesn't perturb title/description and stays cheap (single-row UPDATE by PK).
func (r *sessionRepository) UpdateLastRequestState(
	ctx context.Context, tenantID uint64, userID string, sessionID string,
	state *types.SessionLastRequestState,
) (int64, error) {
	now := time.Now()
	var stateValue interface{}
	if state != nil {
		v, err := state.Value()
		if err != nil {
			return 0, err
		}
		stateValue = v
	}
	res := applySessionUserScope(r.db.WithContext(ctx).
		Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", tenantID, sessionID), userID).
		Updates(map[string]interface{}{
			"last_request_state": stateValue,
			"updated_at":         now,
		})
	return res.RowsAffected, res.Error
}

// Delete deletes a session
func (r *sessionRepository) Delete(ctx context.Context, tenantID uint64, userID string, id string) (int64, error) {
	res := applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id),
		userID,
	).Delete(&types.Session{})
	return res.RowsAffected, res.Error
}

// BatchDelete deletes multiple sessions by IDs
func (r *sessionRepository) BatchDelete(ctx context.Context, tenantID uint64, userID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids),
		userID,
	).Delete(&types.Session{})
	return res.RowsAffected, res.Error
}

// DeleteAllByTenantID deletes all sessions for a tenant
func (r *sessionRepository) DeleteAllByTenantID(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	res := applySessionUserScope(
		r.db.WithContext(ctx).Where("tenant_id = ?", tenantID),
		userID,
	).Delete(&types.Session{})
	return res.RowsAffected, res.Error
}
