package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Sentinel errors of the group repository. The service maps them to HTTP
// statuses; nothing else should need to compare against them.
var (
	// ErrTenantGroupNotFound: no live group with that id in the workspace.
	ErrTenantGroupNotFound = errors.New("tenant group not found")
	// ErrTenantGroupNameTaken: a live group of the workspace already has the
	// name (uniq_tenant_groups_name).
	ErrTenantGroupNameTaken = errors.New("tenant group name already taken")
)

// tenantGroupRepository implements interfaces.TenantGroupRepository.
type tenantGroupRepository struct{ db *gorm.DB }

// NewTenantGroupRepository creates the repository over db.
func NewTenantGroupRepository(db *gorm.DB) interfaces.TenantGroupRepository {
	return &tenantGroupRepository{db: db}
}

// now returns the wall clock truncated to microseconds, the finest precision
// a Postgres timestamp stores, so a value round-trips unchanged.
func (r *tenantGroupRepository) now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func (r *tenantGroupRepository) Create(ctx context.Context, g *types.TenantGroup, memberIDs []string,
	addedBy string,
) error {
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	if g.Source == "" {
		g.Source = types.TenantGroupSourceManual
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(g).Error; err != nil {
			return translateGroupWriteError(err)
		}
		return insertGroupMembers(tx, g.TenantID, g.ID, memberIDs, addedBy)
	})
}

func (r *tenantGroupRepository) Get(ctx context.Context, tenantID uint64, id string) (*types.TenantGroup, error) {
	var g types.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		First(&g).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantGroupNotFound
		}
		return nil, err
	}
	return &g, nil
}

func (r *tenantGroupRepository) List(ctx context.Context, tenantID uint64) ([]*types.TenantGroup, error) {
	var out []*types.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("is_default DESC, name ASC").
		Find(&out).Error
	return out, err
}

// groupUpdatableColumns are the columns Update may write. An allow-list,
// because the field names come from the service and a typo there must fail
// loudly rather than write an unexpected column or be ignored by GORM.
var groupUpdatableColumns = map[string]bool{"name": true, "description": true, "source": true, "external_id": true}

func (r *tenantGroupRepository) Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	for k := range fields {
		if !groupUpdatableColumns[k] {
			return fmt.Errorf("tenant group column %q is not updatable", k)
		}
	}
	fields["updated_at"] = r.now()
	res := r.db.WithContext(ctx).Model(&types.TenantGroup{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(fields)
	if res.Error != nil {
		return translateGroupWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrTenantGroupNotFound
	}
	return nil
}

func (r *tenantGroupRepository) SoftDelete(ctx context.Context, tenantID uint64, id string,
	inTx func(tx *gorm.DB) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Locked first: inTx is about to delete rows that name this group,
		// and a concurrent writer must not add more against a group that is
		// about to disappear, nor may two deletions interleave.
		var g types.TenantGroup
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL AND is_default = ?", tenantID, id, false).
			First(&g).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTenantGroupNotFound
			}
			return err
		}
		if inTx != nil {
			if err := inTx(tx); err != nil {
				return err
			}
		}
		ts := r.now()
		return tx.Model(&types.TenantGroup{}).Where("id = ?", g.ID).
			Updates(map[string]any{"deleted_at": ts, "updated_at": ts}).Error
	})
}

func (r *tenantGroupRepository) EnsureDefault(ctx context.Context, tenantID uint64,
	creatorID string,
) (*types.TenantGroup, error) {
	var g types.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_default = ? AND deleted_at IS NULL", tenantID, true).
		First(&g).Error
	if err == nil {
		return &g, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	g = types.TenantGroup{
		ID: uuid.NewString(), TenantID: tenantID, Name: types.DefaultTenantGroupName,
		Description: "Every member of the workspace", IsDefault: true, Source: types.TenantGroupSourceManual,
	}
	if creatorID != "" {
		g.CreatorID = &creatorID
	}
	if err := r.db.WithContext(ctx).Create(&g).Error; err != nil {
		if isUniqueViolation(err) {
			// Lost a race with a concurrent EnsureDefault; read the winner.
			var existing types.TenantGroup
			if err2 := r.db.WithContext(ctx).
				Where("tenant_id = ? AND is_default = ? AND deleted_at IS NULL", tenantID, true).
				First(&existing).Error; err2 == nil {
				return &existing, nil
			}
		}
		return nil, err
	}
	return &g, nil
}

func (r *tenantGroupRepository) AddMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string,
	addedBy string,
) error {
	if len(userIDs) == 0 {
		return nil
	}
	if _, err := r.Get(ctx, tenantID, groupID); err != nil {
		return err
	}
	return insertGroupMembers(r.db.WithContext(ctx), tenantID, groupID, userIDs, addedBy)
}

// insertGroupMembers writes membership rows for the distinct, non-empty ids,
// leaving existing rows alone. The caller has verified the group.
func insertGroupMembers(db *gorm.DB, tenantID uint64, groupID string, userIDs []string, addedBy string) error {
	rows := make([]types.TenantGroupMember, 0, len(userIDs))
	seen := map[string]bool{}
	for _, uid := range userIDs {
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		m := types.TenantGroupMember{GroupID: groupID, UserID: uid, TenantID: tenantID}
		if addedBy != "" {
			m.AddedBy = &addedBy
		}
		rows = append(rows, m)
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *tenantGroupRepository) RemoveMembers(ctx context.Context, tenantID uint64, groupID string,
	userIDs []string,
) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND group_id = ? AND user_id IN ?", tenantID, groupID, userIDs).
		Delete(&types.TenantGroupMember{}).Error
}

func (r *tenantGroupRepository) ListMemberIDs(ctx context.Context, tenantID uint64, groupID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&types.TenantGroupMember{}).
		Where("tenant_id = ? AND group_id = ?", tenantID, groupID).
		Order("created_at ASC").
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *tenantGroupRepository) GroupIDsForUser(ctx context.Context, tenantID uint64, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("tenant_group_members AS m").
		Joins("JOIN tenant_groups AS g ON g.id = m.group_id AND g.deleted_at IS NULL").
		Where("m.tenant_id = ? AND m.user_id = ?", tenantID, userID).
		Pluck("m.group_id", &ids).Error
	return ids, err
}

// isUniqueViolation recognises a unique-constraint failure (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// translateGroupWriteError turns the name index's violation into the
// sentinel; that index is the only unique constraint a group write can hit.
func translateGroupWriteError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %v", ErrTenantGroupNameTaken, err)
	}
	return err
}
