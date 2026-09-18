package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// TransclusionRepository stores a snapshot of every block another page shows
// by reference.
//
// The table is not an index of every block in every document. It holds only
// the blocks somebody actually references, which is a far smaller and far more
// stable set: a page of two hundred paragraphs that nobody quotes stores
// nothing at all. Rows appear when a reference to a block is first saved, and
// are refreshed when the page the block lives on is saved.
//
// The snapshot exists so that showing a page with forty references costs forty
// reads of one small table rather than forty reads, parses and walks of forty
// other documents. It is derived data throughout: if a row is missing or
// stale, the worst outcome is one reference resolving a moment late, and the
// next save of either page repairs it.
type TransclusionRepository interface {
	// Wanted records that these blocks of this page are referenced by
	// somebody, creating a placeholder row for any that has none yet. It does
	// not fill in content: the source page's own save does that, and this
	// runs on the referring page's save, when the source document is not to
	// hand. Returns the block ids that were newly recorded.
	Wanted(ctx context.Context, tenantID uint64, pageID string, blockIDs []string) ([]string, error)

	// TrackedFor lists the block ids of one page that somebody references, so
	// a save of that page knows which blocks to snapshot.
	TrackedFor(ctx context.Context, tenantID uint64, pageID string) ([]string, error)

	// Store writes the current content of the given blocks of one page, and
	// removes the rows for blocks that no longer exist in it. `blocks` is the
	// complete set that could be found; anything tracked and absent from it
	// has been deleted from the document.
	Store(ctx context.Context, tenantID uint64, pageID string, blocks []model.TransclusionBlock) error

	// Load fetches the snapshots for a set of (page, block) pairs.
	Load(ctx context.Context, tenantID uint64, refs []BlockRef) ([]model.TransclusionBlock, error)

	// DeleteForPage drops every snapshot of one page; used when it is purged.
	DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error
}

// BlockRef names one block of one page.
type BlockRef struct {
	PageID  string
	BlockID string
}

type transclusionRepository struct{ db *gorm.DB }

func (r *transclusionRepository) Wanted(ctx context.Context, tenantID uint64, pageID string,
	blockIDs []string,
) ([]string, error) {
	if pageID == "" || len(blockIDs) == 0 {
		return nil, nil
	}
	wanted := distinct(blockIDs)
	if len(wanted) == 0 {
		return nil, nil
	}

	var have []string
	if err := r.db.WithContext(ctx).Model(&model.TransclusionBlock{}).
		Where("tenant_id = ? AND page_id = ? AND block_id IN ?", tenantID, pageID, wanted).
		Pluck("block_id", &have).Error; err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(have))
	for _, id := range have {
		known[id] = true
	}

	added := make([]string, 0, len(wanted))
	for _, id := range wanted {
		if known[id] {
			continue
		}
		row := model.TransclusionBlock{
			ID: uuid.NewString(), TenantID: tenantID, PageID: pageID, BlockID: id,
			// Empty until the source page is next saved. A reference whose
			// snapshot is empty renders as "not loaded yet" rather than as
			// "deleted", which is the honest thing to say about it.
			Content: model.JSON("null"),
		}
		err := r.db.WithContext(ctx).Create(&row).Error
		switch {
		case err == nil:
			added = append(added, id)
		case isUniqueViolation(err):
			// Another save got there first, which is the expected outcome of
			// two people referencing the same block at once.
		default:
			return nil, err
		}
	}
	return added, nil
}

func (r *transclusionRepository) TrackedFor(ctx context.Context, tenantID uint64, pageID string) ([]string, error) {
	if pageID == "" {
		return nil, nil
	}
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.TransclusionBlock{}).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Pluck("block_id", &ids).Error
	return ids, err
}

func (r *transclusionRepository) Store(ctx context.Context, tenantID uint64, pageID string,
	blocks []model.TransclusionBlock,
) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		present := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if block.BlockID == "" {
				continue
			}
			present = append(present, block.BlockID)
			updates := map[string]any{
				"content":      block.Content,
				"text_content": block.TextContent,
			}
			res := tx.Model(&model.TransclusionBlock{}).
				Where("tenant_id = ? AND page_id = ? AND block_id = ?", tenantID, pageID, block.BlockID).
				Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			// No row means nobody references this block; a snapshot of it
			// would be storage spent on a question nobody asked.
		}

		// A tracked block that is no longer in the document has been deleted.
		// Dropping the row is what makes the reference say so, and leaving it
		// would show text that is not on the page any more.
		q := tx.Where("tenant_id = ? AND page_id = ?", tenantID, pageID)
		if len(present) > 0 {
			q = q.Where("block_id NOT IN ?", present)
		}
		return q.Delete(&model.TransclusionBlock{}).Error
	})
}

func (r *transclusionRepository) Load(ctx context.Context, tenantID uint64, refs []BlockRef) (
	[]model.TransclusionBlock, error,
) {
	if len(refs) == 0 {
		return nil, nil
	}
	// Grouped by page, so this is one query per referenced page rather than
	// one per reference: a page quoted forty times is read once.
	byPage := map[string][]string{}
	order := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.PageID == "" || ref.BlockID == "" {
			continue
		}
		if _, seen := byPage[ref.PageID]; !seen {
			order = append(order, ref.PageID)
		}
		byPage[ref.PageID] = append(byPage[ref.PageID], ref.BlockID)
	}

	out := make([]model.TransclusionBlock, 0, len(refs))
	for _, pageID := range order {
		var rows []model.TransclusionBlock
		if err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND page_id = ? AND block_id IN ?",
				tenantID, pageID, distinct(byPage[pageID])).
			Find(&rows).Error; err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func (r *transclusionRepository) DeleteForPage(ctx context.Context, tenantID uint64, pageID string) error {
	if pageID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND page_id = ?", tenantID, pageID).
		Delete(&model.TransclusionBlock{}).Error
}

// distinct keeps the first occurrence of each non-empty id, in order.
func distinct(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
