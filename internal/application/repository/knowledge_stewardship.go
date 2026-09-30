package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// knowledgeStewardshipRepository stores who looks after knowledge entries, in
// the stewardship columns of knowledges.
type knowledgeStewardshipRepository struct {
	db *gorm.DB
}

// NewKnowledgeStewardshipRepository returns the PostgreSQL stewardship store.
func NewKnowledgeStewardshipRepository(db *gorm.DB) interfaces.KnowledgeStewardshipRepository {
	return &knowledgeStewardshipRepository{db: db}
}

// activeMemberSQL is true when the user named by the column expression is an
// active account with an active membership of the tenant named by the other.
// Somebody who left the workspace, was suspended or had their account
// disabled cannot be asked to act, however long their name stays on an entry.
func activeMemberSQL(userCol, tenantCol string) string {
	return `EXISTS (SELECT 1 FROM tenant_members m JOIN users u ON u.id = m.user_id
		WHERE m.user_id = ` + userCol + ` AND m.tenant_id = ` + tenantCol + `
		  AND m.status = '` + string(types.TenantMemberStatusActive) + `' AND m.deleted_at IS NULL
		  AND u.is_active AND u.deleted_at IS NULL)`
}

// stewardSelectSQL selects a types.KnowledgeSteward from knowledges k joined
// to its knowledge base kb. Shared with the findings repository, whose review
// sweep reads the same shape.
var stewardSelectSQL = `SELECT k.id AS knowledge_id, k.knowledge_base_id, k.title,
	` + types.KnowledgeOriginSQL + ` AS origin, k.created_at,
	COALESCE(k.owner_id, '') AS owner_id,
	` + activeMemberSQL("k.owner_id", "k.tenant_id") + ` AS owner_active,
	k.reviewed_at, COALESCE(k.reviewed_by, '') AS reviewed_by,
	` + activeMemberSQL("k.reviewed_by", "k.tenant_id") + ` AS reviewer_active,
	COALESCE(kb.creator_id, '') AS kb_creator_id,
	` + activeMemberSQL("kb.creator_id", "k.tenant_id") + ` AS kb_creator_active,
	kb.review_interval_days
	FROM knowledges k
	JOIN knowledge_bases kb ON kb.id = k.knowledge_base_id AND kb.deleted_at IS NULL`

// Stewards implements interfaces.KnowledgeStewardshipRepository.
func (r *knowledgeStewardshipRepository) Stewards(ctx context.Context, tenantID uint64, knowledgeIDs []string,
) (map[string]*types.KnowledgeSteward, error) {
	out := make(map[string]*types.KnowledgeSteward, len(knowledgeIDs))
	if len(knowledgeIDs) == 0 {
		return out, nil
	}
	var rows []*types.KnowledgeSteward
	err := r.db.WithContext(ctx).
		Raw(stewardSelectSQL+` WHERE k.tenant_id = ? AND k.id IN ? AND k.deleted_at IS NULL`, tenantID, knowledgeIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.KnowledgeID] = row
	}
	return out, nil
}

// SetOwner implements interfaces.KnowledgeStewardshipRepository.
func (r *knowledgeStewardshipRepository) SetOwner(ctx context.Context, tenantID uint64, knowledgeID,
	ownerID string,
) error {
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	// UpdateColumn: no updated_at bump. Ownership is not content, and
	// updated_at is what the knowledge list sorts and the pipeline reasons by.
	res := r.db.WithContext(ctx).Model(&types.Knowledge{}).
		Where("tenant_id = ? AND id = ?", tenantID, knowledgeID).
		UpdateColumn("owner_id", owner)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return interfaces.ErrKnowledgeNotFound
	}
	return nil
}

// MarkReviewed implements interfaces.KnowledgeStewardshipRepository.
func (r *knowledgeStewardshipRepository) MarkReviewed(ctx context.Context, tenantID uint64, knowledgeID,
	userID string, at time.Time,
) error {
	if userID == "" || at.IsZero() {
		return errors.New("a review needs a person and a time")
	}
	return r.db.WithContext(ctx).Exec(`
		UPDATE knowledges SET reviewed_at = ?, reviewed_by = ?
		 WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
		   AND (reviewed_at IS NULL OR reviewed_at <= ?)`,
		at, userID, tenantID, knowledgeID, at,
	).Error
}

// CanMaintain implements interfaces.KnowledgeStewardshipRepository.
func (r *knowledgeStewardshipRepository) CanMaintain(ctx context.Context, tenantID uint64, kbID, userID string,
) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var ok bool
	err := r.db.WithContext(ctx).Raw(`SELECT `+activeMemberSQL("@user", "@tenant")+` AND (
		EXISTS (SELECT 1 FROM tenant_members m WHERE m.user_id = @user AND m.tenant_id = @tenant
		         AND m.deleted_at IS NULL AND m.role IN @roles)
		OR EXISTS (SELECT 1 FROM knowledge_bases kb WHERE kb.id = @kb AND kb.tenant_id = @tenant
		            AND kb.deleted_at IS NULL AND kb.creator_id = @user))`,
		map[string]any{
			"user": userID, "tenant": tenantID, "kb": kbID,
			"roles": []string{string(types.TenantRoleOwner), string(types.TenantRoleAdmin)},
		}).Scan(&ok).Error
	return ok, err
}

// IsActiveMember implements interfaces.KnowledgeStewardshipRepository.
func (r *knowledgeStewardshipRepository) IsActiveMember(ctx context.Context, tenantID uint64, userID string,
) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var ok bool
	err := r.db.WithContext(ctx).Raw(`SELECT `+activeMemberSQL("@user", "@tenant"),
		map[string]any{"user": userID, "tenant": tenantID}).Scan(&ok).Error
	return ok, err
}
