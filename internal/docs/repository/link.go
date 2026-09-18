package repository

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
)

// LinkRepository records which page refers to which, so a page can show what
// points at it.
//
// The rows are derived data, rebuilt from the document every time it is saved
// rather than maintained edit by edit. That is the only way they can be right:
// a link is deleted by deleting the text around it, and there is no event for
// that — only the new document, which either still mentions the target or does
// not.
type LinkRepository interface {
	// ReplaceForSource makes the stored links of one page exactly the given
	// set, adding what is new and removing what the document no longer says.
	ReplaceForSource(ctx context.Context, tenantID uint64, sourcePageID string, links []model.Link) error
	// Backlinks lists the live pages that refer to the target, newest first.
	// The caller filters them by what the reader may see.
	Backlinks(ctx context.Context, tenantID uint64, targetPageID string) ([]*model.Page, error)
	// Outgoing lists the target ids one page refers to.
	Outgoing(ctx context.Context, tenantID uint64, sourcePageID string) ([]model.Link, error)
	// DeleteForSource drops every link a page makes; used when it is purged.
	DeleteForSource(ctx context.Context, tenantID uint64, sourcePageID string) error
}

type linkRepository struct{ db *gorm.DB }

func (r *linkRepository) ReplaceForSource(ctx context.Context, tenantID uint64, sourcePageID string,
	links []model.Link,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wanted := make(map[string]model.Link, len(links))
		for _, l := range links {
			// A page linking to itself is not a backlink anybody wants to see,
			// and a missing target would fail the foreign key.
			if l.TargetPageID == "" || l.TargetPageID == sourcePageID {
				continue
			}
			l.TenantID = tenantID
			l.SourcePageID = sourcePageID
			if l.Kind == "" {
				l.Kind = model.LinkPage
			}
			wanted[string(l.Kind)+"\x00"+l.TargetPageID] = l
		}

		var existing []model.Link
		if err := tx.Where("tenant_id = ? AND source_page_id = ?", tenantID, sourcePageID).
			Find(&existing).Error; err != nil {
			return err
		}
		for _, have := range existing {
			key := string(have.Kind) + "\x00" + have.TargetPageID
			if _, keep := wanted[key]; keep {
				// Already recorded; leaving it alone keeps created_at, which
				// is what orders a page's backlinks.
				delete(wanted, key)
				continue
			}
			if err := tx.Where("tenant_id = ? AND source_page_id = ? AND target_page_id = ? AND kind = ?",
				tenantID, sourcePageID, have.TargetPageID, have.Kind).Delete(&model.Link{}).Error; err != nil {
				return err
			}
		}
		for _, add := range wanted {
			// A target deleted between extracting the document and writing the
			// row fails the foreign key; that link simply does not exist, and
			// failing the whole save over it would be wrong.
			if err := tx.Create(&add).Error; err != nil && !isForeignKeyViolation(err) {
				return translateWriteError(err)
			}
		}
		return nil
	})
}

func (r *linkRepository) Backlinks(ctx context.Context, tenantID uint64,
	targetPageID string,
) ([]*model.Page, error) {
	var rows []*model.Page
	// DISTINCT because one page may refer to another both as a link and as a
	// transclusion; it is still one page in the list.
	err := r.db.WithContext(ctx).Table("docs_pages AS p").
		Distinct(prefixColumns("p", model.PageSummaryColumns)).
		Joins("JOIN docs_links AS l ON l.source_page_id = p.id").
		Where("l.tenant_id = ? AND l.target_page_id = ?", tenantID, targetPageID).
		Where("p.deleted_at IS NULL").
		Order("p.updated_at DESC, p.id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *linkRepository) Outgoing(ctx context.Context, tenantID uint64,
	sourcePageID string,
) ([]model.Link, error) {
	var rows []model.Link
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source_page_id = ?", tenantID, sourcePageID).
		Find(&rows).Error
	return rows, err
}

func (r *linkRepository) DeleteForSource(ctx context.Context, tenantID uint64, sourcePageID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND source_page_id = ?", tenantID, sourcePageID).
		Delete(&model.Link{}).Error
}
