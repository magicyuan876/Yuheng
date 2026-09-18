package repository

import (
	"context"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GroupRepository persists tenant groups and their members.
type GroupRepository interface {
	Create(ctx context.Context, g *model.TenantGroup) error
	Get(ctx context.Context, tenantID uint64, id string) (*model.TenantGroup, error)
	List(ctx context.Context, tenantID uint64) ([]*model.TenantGroup, error)
	Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error
	SoftDelete(ctx context.Context, tenantID uint64, id string) error
	// EnsureDefault returns the tenant's default ("everyone") group, creating
	// it on first use. Membership of the default group is implicit: the ACL
	// layer treats every active tenant member as a member, so it never needs
	// rows in tenant_group_members.
	EnsureDefault(ctx context.Context, tenantID uint64, creatorID string) (*model.TenantGroup, error)
	AddMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string, addedBy string) error
	RemoveMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string) error
	ListMemberIDs(ctx context.Context, tenantID uint64, groupID string) ([]string, error)
	// GroupIDsForUser returns the live, explicitly-joined groups of a user
	// (the default group is not included; see EnsureDefault).
	GroupIDsForUser(ctx context.Context, tenantID uint64, userID string) ([]string, error)
}

type groupRepository struct{ db *gorm.DB }

func (r *groupRepository) Create(ctx context.Context, g *model.TenantGroup) error {
	if g.ID == "" {
		g.ID = NewID()
	}
	if g.Source == "" {
		g.Source = model.GroupSourceManual
	}
	return translateWriteError(r.db.WithContext(ctx).Create(g).Error)
}

func (r *groupRepository) Get(ctx context.Context, tenantID uint64, id string) (*model.TenantGroup, error) {
	var g model.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		First(&g).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &g, nil
}

func (r *groupRepository) List(ctx context.Context, tenantID uint64) ([]*model.TenantGroup, error) {
	var out []*model.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("is_default DESC, name ASC").
		Find(&out).Error
	return out, err
}

var groupUpdatableColumns = map[string]bool{"name": true, "description": true, "source": true, "external_id": true}

func (r *groupRepository) Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	for k := range fields {
		if !groupUpdatableColumns[k] {
			return fmt.Errorf("docs: group column %q is not updatable", k)
		}
	}
	fields["updated_at"] = now()
	res := r.db.WithContext(ctx).Model(&model.TenantGroup{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(fields)
	if res.Error != nil {
		return translateWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *groupRepository) SoftDelete(ctx context.Context, tenantID uint64, id string) error {
	ts := now()
	res := r.db.WithContext(ctx).Model(&model.TenantGroup{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL AND is_default = ?", tenantID, id, false).
		Updates(map[string]any{"deleted_at": ts, "updated_at": ts})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *groupRepository) EnsureDefault(ctx context.Context, tenantID uint64,
	creatorID string,
) (*model.TenantGroup, error) {
	var g model.TenantGroup
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_default = ? AND deleted_at IS NULL", tenantID, true).
		First(&g).Error
	if err == nil {
		return &g, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	g = model.TenantGroup{
		ID: NewID(), TenantID: tenantID, Name: model.DefaultGroupName,
		Description: "Every member of the workspace", IsDefault: true, Source: model.GroupSourceManual,
	}
	if creatorID != "" {
		g.CreatorID = &creatorID
	}
	if err := r.db.WithContext(ctx).Create(&g).Error; err != nil {
		if isUniqueViolation(err) {
			// Lost a race with a concurrent EnsureDefault; read the winner.
			var existing model.TenantGroup
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

func (r *groupRepository) AddMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string,
	addedBy string,
) error {
	if len(userIDs) == 0 {
		return nil
	}
	if _, err := r.Get(ctx, tenantID, groupID); err != nil {
		return err
	}
	rows := make([]model.TenantGroupMember, 0, len(userIDs))
	seen := map[string]bool{}
	for _, uid := range userIDs {
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		m := model.TenantGroupMember{GroupID: groupID, UserID: uid, TenantID: tenantID}
		if addedBy != "" {
			m.AddedBy = &addedBy
		}
		rows = append(rows, m)
	}
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *groupRepository) RemoveMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND group_id = ? AND user_id IN ?", tenantID, groupID, userIDs).
		Delete(&model.TenantGroupMember{}).Error
}

func (r *groupRepository) ListMemberIDs(ctx context.Context, tenantID uint64, groupID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.TenantGroupMember{}).
		Where("tenant_id = ? AND group_id = ?", tenantID, groupID).
		Order("created_at ASC").
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *groupRepository) GroupIDsForUser(ctx context.Context, tenantID uint64, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("tenant_group_members AS m").
		Joins("JOIN tenant_groups AS g ON g.id = m.group_id AND g.deleted_at IS NULL").
		Where("m.tenant_id = ? AND m.user_id = ?", tenantID, userID).
		Pluck("m.group_id", &ids).Error
	return ids, err
}
