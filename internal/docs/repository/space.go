package repository

import (
	"context"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SpaceRepository persists spaces.
type SpaceRepository interface {
	Create(ctx context.Context, space *model.Space) error
	Get(ctx context.Context, tenantID uint64, id string) (*model.Space, error)
	GetBySlug(ctx context.Context, tenantID uint64, slug string) (*model.Space, error)
	// GetPublicByID finds a PUBLIC space by id, without a tenant scope, for
	// anonymous visitors. It can never return a private space.
	GetPublicByID(ctx context.Context, id string) (*model.Space, error)
	// List returns the tenant's live spaces ordered by name.
	List(ctx context.Context, tenantID uint64) ([]*model.Space, error)
	// ListByIDs returns live spaces among the given IDs (any order).
	ListByIDs(ctx context.Context, tenantID uint64, ids []string) ([]*model.Space, error)
	// Update writes the given columns (snake_case keys) and bumps updated_at.
	Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error
	SoftDelete(ctx context.Context, tenantID uint64, id string) error
	Restore(ctx context.Context, tenantID uint64, id string) error
}

// SpaceMemberRepository persists space membership.
type SpaceMemberRepository interface {
	// Upsert inserts the membership or updates the role of an existing one.
	Upsert(ctx context.Context, m *model.SpaceMember) error
	Remove(ctx context.Context, tenantID uint64, spaceID string, principal model.Principal) error
	ListBySpace(ctx context.Context, tenantID uint64, spaceID string) ([]*model.SpaceMember, error)
	// RolesFor returns the roles the principals hold directly in one space.
	RolesFor(ctx context.Context, tenantID uint64, spaceID string,
		principals []model.Principal) ([]model.SpaceRole, error)
	// SpaceRolesFor returns, per space, the strongest role any of the
	// principals holds directly. Open-space default roles are not included;
	// the ACL resolver layers those on.
	SpaceRolesFor(ctx context.Context, tenantID uint64,
		principals []model.Principal) (map[string]model.SpaceRole, error)
	// RemoveAllForPrincipal drops a principal from every space (user left the
	// tenant, group deleted).
	RemoveAllForPrincipal(ctx context.Context, tenantID uint64, principal model.Principal) error
}

type spaceRepository struct{ db *gorm.DB }

func (r *spaceRepository) Create(ctx context.Context, space *model.Space) error {
	if space.ID == "" {
		space.ID = NewID()
	}
	if space.Visibility == "" {
		space.Visibility = model.VisibilityPrivate
	}
	if space.DefaultRole == "" {
		space.DefaultRole = model.RoleNone
	}
	if len(space.Settings) == 0 {
		space.Settings = model.JSON("{}")
	}
	return translateWriteError(r.db.WithContext(ctx).Create(space).Error)
}

func (r *spaceRepository) Get(ctx context.Context, tenantID uint64, id string) (*model.Space, error) {
	var s model.Space
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		First(&s).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &s, nil
}

func (r *spaceRepository) GetBySlug(ctx context.Context, tenantID uint64, slug string) (*model.Space, error) {
	var s model.Space
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND slug = ? AND deleted_at IS NULL", tenantID, slug).
		First(&s).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &s, nil
}

func (r *spaceRepository) List(ctx context.Context, tenantID uint64) ([]*model.Space, error) {
	var out []*model.Space
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("name ASC, id ASC").
		Find(&out).Error
	return out, err
}

func (r *spaceRepository) ListByIDs(ctx context.Context, tenantID uint64, ids []string) ([]*model.Space, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []*model.Space
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).
		Order("name ASC, id ASC").
		Find(&out).Error
	return out, err
}

// spaceUpdatableColumns is the allow-list for Update; anything else is a
// programming error, not a client error, so it fails loudly.
var spaceUpdatableColumns = map[string]bool{
	"slug": true, "name": true, "description": true, "icon": true, "visibility": true,
	"default_role": true, "knowledge_base_id": true, "storage_backend_id": true, "settings": true,
}

func (r *spaceRepository) Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	for k := range fields {
		if !spaceUpdatableColumns[k] {
			return fmt.Errorf("docs: space column %q is not updatable", k)
		}
	}
	fields["updated_at"] = now()
	res := r.db.WithContext(ctx).Model(&model.Space{}).
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

func (r *spaceRepository) SoftDelete(ctx context.Context, tenantID uint64, id string) error {
	ts := now()
	res := r.db.WithContext(ctx).Model(&model.Space{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		Updates(map[string]any{"deleted_at": ts, "updated_at": ts})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *spaceRepository) Restore(ctx context.Context, tenantID uint64, id string) error {
	res := r.db.WithContext(ctx).Model(&model.Space{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", tenantID, id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": now()})
	if res.Error != nil {
		return translateWriteError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- members ------------------------------------------------------------------

type spaceMemberRepository struct{ db *gorm.DB }

func (r *spaceMemberRepository) Upsert(ctx context.Context, m *model.SpaceMember) error {
	if m.ID == "" {
		m.ID = NewID()
	}
	if !m.Role.Valid() {
		return fmt.Errorf("docs: role %q is not grantable", m.Role)
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "space_id"}, {Name: "principal_type"}, {Name: "principal_id"}},
		DoUpdates: clause.Assignments(map[string]any{"role": m.Role, "added_by": m.AddedBy, "updated_at": now()}),
	}).Create(m).Error
}

func (r *spaceMemberRepository) Remove(ctx context.Context, tenantID uint64, spaceID string, p model.Principal) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ? AND principal_type = ? AND principal_id = ?",
			tenantID, spaceID, p.Type, p.ID).
		Delete(&model.SpaceMember{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *spaceMemberRepository) ListBySpace(ctx context.Context, tenantID uint64,
	spaceID string,
) ([]*model.SpaceMember, error) {
	var out []*model.SpaceMember
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ?", tenantID, spaceID).
		Order("principal_type ASC, created_at ASC").
		Find(&out).Error
	return out, err
}

// principalClause builds "(principal_type = ? AND principal_id = ?) OR ..." for
// a set of principals. Callers must check for an empty set first.
func principalClause(principals []model.Principal) (string, []any) {
	parts := make([]string, 0, len(principals))
	args := make([]any, 0, len(principals)*2)
	for _, p := range principals {
		parts = append(parts, "(principal_type = ? AND principal_id = ?)")
		args = append(args, p.Type, p.ID)
	}
	return "(" + joinOr(parts) + ")", args
}

func joinOr(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " OR "
		}
		out += p
	}
	return out
}

func (r *spaceMemberRepository) RolesFor(ctx context.Context, tenantID uint64, spaceID string,
	principals []model.Principal,
) ([]model.SpaceRole, error) {
	if len(principals) == 0 {
		return nil, nil
	}
	where, args := principalClause(principals)
	var roles []model.SpaceRole
	err := r.db.WithContext(ctx).Model(&model.SpaceMember{}).
		Where("tenant_id = ? AND space_id = ?", tenantID, spaceID).
		Where(where, args...).
		Pluck("role", &roles).Error
	return roles, err
}

func (r *spaceMemberRepository) SpaceRolesFor(ctx context.Context, tenantID uint64,
	principals []model.Principal,
) (map[string]model.SpaceRole, error) {
	out := map[string]model.SpaceRole{}
	if len(principals) == 0 {
		return out, nil
	}
	where, args := principalClause(principals)
	var rows []struct {
		SpaceID string
		Role    model.SpaceRole
	}
	err := r.db.WithContext(ctx).Model(&model.SpaceMember{}).
		Select("space_id", "role").
		Where("tenant_id = ?", tenantID).
		Where(where, args...).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SpaceID] = model.MaxRole(out[row.SpaceID], row.Role)
	}
	return out, nil
}

func (r *spaceMemberRepository) RemoveAllForPrincipal(ctx context.Context, tenantID uint64, p model.Principal) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND principal_type = ? AND principal_id = ?", tenantID, p.Type, p.ID).
		Delete(&model.SpaceMember{}).Error
}

// GetPublicByID finds a space by id with no tenant scope.
//
// The only query in this package that is not scoped to a tenant, because an
// anonymous visitor has no tenant to scope it by. The visibility condition is
// part of the statement rather than a check the caller is trusted to make
// afterwards: this method cannot return a private space, so no future caller
// can misuse it into one.
func (r *spaceRepository) GetPublicByID(ctx context.Context, id string) (*model.Space, error) {
	var out model.Space
	err := r.db.WithContext(ctx).
		Where("id = ? AND visibility = ? AND deleted_at IS NULL", id, model.VisibilityPublic).
		Take(&out).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &out, nil
}
