package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// LabelRepository stores a space's labels and what they are put on.
//
// Labels belong to a space rather than to a tenant, which is the decision
// worth stating: two teams both wanting a "draft" label should get two
// labels, not an argument about one. It also means a label cannot leak the
// existence of pages in a space somebody cannot see.
type LabelRepository interface {
	// Create adds a label to a space. A name already used there is a
	// conflict rather than a second label.
	Create(ctx context.Context, label *model.Label) error

	// Update renames a label or recolours it.
	Update(ctx context.Context, tenantID uint64, labelID string, name, color string) error

	// Delete removes a label; the rows attaching it to pages go with it.
	Delete(ctx context.Context, tenantID uint64, labelID string) error

	// Get returns one label, or ErrNotFound.
	Get(ctx context.Context, tenantID uint64, labelID string) (*model.Label, error)

	// ListForSpace returns a space's labels by name, with how many live pages
	// carry each — a label nobody uses is worth being able to see as such.
	ListForSpace(ctx context.Context, tenantID uint64, spaceID string) ([]*LabelCount, error)

	// SetForPage makes a page's labels exactly the given set.
	SetForPage(ctx context.Context, tenantID uint64, pageID string, labelIDs []string) error

	// ForPages returns the labels on each of the given pages, keyed by page
	// id, so a listing costs one query rather than one per row.
	ForPages(ctx context.Context, tenantID uint64, pageIDs []string) (map[string][]*model.Label, error)

	// PagesWith lists the live pages in a space carrying every one of the
	// given labels, newest first.
	PagesWith(ctx context.Context, tenantID uint64, spaceID string, labelIDs []string, limit int) (
		[]*model.Page, error)
}

// LabelCount is a label with how many pages carry it.
type LabelCount struct {
	*model.Label
	PageCount int64 `json:"page_count"`
}

type labelRepository struct{ db *gorm.DB }

func (r *labelRepository) Create(ctx context.Context, label *model.Label) error {
	if label.ID == "" {
		label.ID = NewID()
	}
	label.Name = strings.TrimSpace(label.Name)
	err := r.db.WithContext(ctx).Create(label).Error
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *labelRepository) Update(ctx context.Context, tenantID uint64, labelID string,
	name, color string,
) error {
	updates := map[string]any{}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		updates["name"] = trimmed
	}
	if color != "" {
		updates["color"] = color
	}
	if len(updates) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&model.Label{}).
		Where("tenant_id = ? AND id = ?", tenantID, labelID).
		Updates(updates)
	if isUniqueViolation(res.Error) {
		return ErrConflict
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *labelRepository) Delete(ctx context.Context, tenantID uint64, labelID string) error {
	// The label's attachments to pages go with it: docs_page_labels.label_id
	// cascades on delete, so no page is left pointing at a removed label.
	res := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, labelID).Delete(&model.Label{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *labelRepository) Get(ctx context.Context, tenantID uint64, labelID string) (
	*model.Label, error,
) {
	var label model.Label
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, labelID).Take(&label).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &label, nil
}

func (r *labelRepository) ListForSpace(ctx context.Context, tenantID uint64, spaceID string) (
	[]*LabelCount, error,
) {
	var labels []*model.Label
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ?", tenantID, spaceID).
		Order("name ASC").Find(&labels).Error; err != nil {
		return nil, err
	}
	if len(labels) == 0 {
		return []*LabelCount{}, nil
	}

	ids := make([]string, 0, len(labels))
	for _, label := range labels {
		ids = append(ids, label.ID)
	}
	// Counted over live pages only: a label's count should match what
	// clicking it shows, and clicking it does not show the trash.
	type row struct {
		LabelID string
		N       int64
	}
	var counts []row
	if err := r.db.WithContext(ctx).Model(&model.PageLabel{}).
		Select("docs_page_labels.label_id AS label_id, COUNT(*) AS n").
		Joins("JOIN docs_pages ON docs_pages.id = docs_page_labels.page_id AND docs_pages.deleted_at IS NULL").
		Where("docs_page_labels.label_id IN ?", ids).
		Group("docs_page_labels.label_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]int64, len(counts))
	for _, c := range counts {
		byID[c.LabelID] = c.N
	}

	out := make([]*LabelCount, 0, len(labels))
	for _, label := range labels {
		out = append(out, &LabelCount{Label: label, PageCount: byID[label.ID]})
	}
	return out, nil
}

func (r *labelRepository) SetForPage(ctx context.Context, tenantID uint64, pageID string,
	labelIDs []string,
) error {
	wanted := distinct(labelIDs)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("page_id = ?", pageID).Delete(&model.PageLabel{}).Error; err != nil {
			return err
		}
		if len(wanted) == 0 {
			return nil
		}
		// Only labels of this tenant, and only ones that exist: a caller
		// naming an id from somewhere else must not attach it.
		var valid []string
		if err := tx.Model(&model.Label{}).
			Where("tenant_id = ? AND id IN ?", tenantID, wanted).
			Pluck("id", &valid).Error; err != nil {
			return err
		}
		if len(valid) == 0 {
			return nil
		}
		rows := make([]model.PageLabel, 0, len(valid))
		for _, id := range valid {
			rows = append(rows, model.PageLabel{PageID: pageID, LabelID: id})
		}
		return tx.Create(&rows).Error
	})
}

func (r *labelRepository) ForPages(ctx context.Context, tenantID uint64, pageIDs []string) (
	map[string][]*model.Label, error,
) {
	out := map[string][]*model.Label{}
	ids := distinct(pageIDs)
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		PageID string
		model.Label
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.PageLabel{}).
		Select("docs_page_labels.page_id AS page_id, docs_labels.*").
		Joins("JOIN docs_labels ON docs_labels.id = docs_page_labels.label_id").
		Where("docs_labels.tenant_id = ? AND docs_page_labels.page_id IN ?", tenantID, ids).
		Order("docs_labels.name ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		label := rows[i].Label
		out[rows[i].PageID] = append(out[rows[i].PageID], &label)
	}
	return out, nil
}

func (r *labelRepository) PagesWith(ctx context.Context, tenantID uint64, spaceID string,
	labelIDs []string, limit int,
) ([]*model.Page, error) {
	wanted := distinct(labelIDs)
	if len(wanted) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	// Every label, not any: filtering by two labels means pages carrying
	// both, which is what somebody narrowing a list expects.
	var pageIDs []string
	err := r.db.WithContext(ctx).Model(&model.PageLabel{}).
		Select("page_id").
		Where("label_id IN ?", wanted).
		Group("page_id").
		Having("COUNT(DISTINCT label_id) = ?", len(wanted)).
		Pluck("page_id", &pageIDs).Error
	if err != nil {
		return nil, err
	}
	if len(pageIDs) == 0 {
		return []*model.Page{}, nil
	}

	var pages []*model.Page
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND space_id = ? AND id IN ? AND deleted_at IS NULL",
			tenantID, spaceID, pageIDs).
		Order("content_updated_at DESC, updated_at DESC").
		Limit(limit).Find(&pages).Error
	return pages, err
}
