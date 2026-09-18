package repository

import (
	"context"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PageAccessRepository persists page-level permission overrides: which pages
// cut inheritance ("restricted") and who is granted what on them.
type PageAccessRepository interface {
	// Restricted reports which of the given pages are restricted.
	Restricted(ctx context.Context, tenantID uint64, pageIDs []string) (map[string]bool, error)
	// RestrictedInSpace lists every restricted page of a space.
	RestrictedInSpace(ctx context.Context, tenantID uint64, spaceID string) ([]string, error)
	SetRestricted(ctx context.Context, tenantID uint64, spaceID, pageID, createdBy string) error
	// ClearRestricted removes the restriction and every grant on the page.
	ClearRestricted(ctx context.Context, tenantID uint64, pageID string) error

	UpsertGrant(ctx context.Context, g *model.PageGrant) error
	RemoveGrant(ctx context.Context, tenantID uint64, pageID string, principal model.Principal) error
	ListGrants(ctx context.Context, tenantID uint64, pageID string) ([]*model.PageGrant, error)
	// GrantsFor returns, per page, the roles the principals hold directly.
	GrantsFor(ctx context.Context, tenantID uint64, pageIDs []string,
		principals []model.Principal) (map[string][]model.SpaceRole, error)
	// GrantsForPages returns every grant on the given pages.
	GrantsForPages(ctx context.Context, tenantID uint64, pageIDs []string) (map[string][]*model.PageGrant, error)
	RemoveAllGrantsForPrincipal(ctx context.Context, tenantID uint64, principal model.Principal) error
}

type pageAccessRepository struct{ db *gorm.DB }

func (r *pageAccessRepository) Restricted(ctx context.Context, tenantID uint64,
	pageIDs []string,
) (map[string]bool, error) {
	out := map[string]bool{}
	if len(pageIDs) == 0 {
		return out, nil
	}
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.PageAccess{}).
		Where("tenant_id = ? AND page_id IN ?", tenantID, pageIDs).
		Pluck("page_id", &ids).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (r *pageAccessRepository) RestrictedInSpace(ctx context.Context, tenantID uint64,
	spaceID string,
) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.PageAccess{}).
		Where("tenant_id = ? AND space_id = ?", tenantID, spaceID).
		Pluck("page_id", &ids).Error
	return ids, err
}

func (r *pageAccessRepository) SetRestricted(ctx context.Context, tenantID uint64, spaceID, pageID,
	createdBy string,
) error {
	row := model.PageAccess{PageID: pageID, TenantID: tenantID, SpaceID: spaceID, Mode: "restricted"}
	if createdBy != "" {
		row.CreatedBy = &createdBy
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}},
		DoUpdates: clause.Assignments(map[string]any{"updated_at": now()}),
	}).Create(&row).Error
}

func (r *pageAccessRepository) ClearRestricted(ctx context.Context, tenantID uint64, pageID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
			Delete(&model.PageGrant{}).Error; err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
			Delete(&model.PageAccess{}).Error
	})
}

func (r *pageAccessRepository) UpsertGrant(ctx context.Context, g *model.PageGrant) error {
	if g.ID == "" {
		g.ID = NewID()
	}
	if !g.Role.Valid() {
		return fmt.Errorf("docs: role %q is not grantable", g.Role)
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "principal_type"}, {Name: "principal_id"}},
		DoUpdates: clause.Assignments(map[string]any{"role": g.Role, "added_by": g.AddedBy}),
	}).Create(g).Error
}

func (r *pageAccessRepository) RemoveGrant(ctx context.Context, tenantID uint64, pageID string,
	p model.Principal,
) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ? AND principal_type = ? AND principal_id = ?",
			tenantID, pageID, p.Type, p.ID).
		Delete(&model.PageGrant{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pageAccessRepository) ListGrants(ctx context.Context, tenantID uint64,
	pageID string,
) ([]*model.PageGrant, error) {
	var out []*model.PageGrant
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Order("principal_type ASC, created_at ASC").
		Find(&out).Error
	return out, err
}

func (r *pageAccessRepository) GrantsFor(ctx context.Context, tenantID uint64, pageIDs []string,
	principals []model.Principal,
) (map[string][]model.SpaceRole, error) {
	out := map[string][]model.SpaceRole{}
	if len(pageIDs) == 0 || len(principals) == 0 {
		return out, nil
	}
	where, args := principalClause(principals)
	var rows []struct {
		PageID string
		Role   model.SpaceRole
	}
	err := r.db.WithContext(ctx).Model(&model.PageGrant{}).
		Select("page_id", "role").
		Where("tenant_id = ? AND page_id IN ?", tenantID, pageIDs).
		Where(where, args...).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PageID] = append(out[row.PageID], row.Role)
	}
	return out, nil
}

func (r *pageAccessRepository) GrantsForPages(ctx context.Context, tenantID uint64,
	pageIDs []string,
) (map[string][]*model.PageGrant, error) {
	out := map[string][]*model.PageGrant{}
	if len(pageIDs) == 0 {
		return out, nil
	}
	var rows []*model.PageGrant
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id IN ?", tenantID, pageIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, g := range rows {
		out[g.PageID] = append(out[g.PageID], g)
	}
	return out, nil
}

func (r *pageAccessRepository) RemoveAllGrantsForPrincipal(ctx context.Context, tenantID uint64,
	p model.Principal,
) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND principal_type = ? AND principal_id = ?", tenantID, p.Type, p.ID).
		Delete(&model.PageGrant{}).Error
}
