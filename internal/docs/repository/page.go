package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"gorm.io/gorm"
)

// PageRepository persists pages and the page tree.
type PageRepository interface {
	Create(ctx context.Context, page *model.Page) error
	// Get loads a live page including content and ydoc.
	Get(ctx context.Context, tenantID uint64, id string) (*model.Page, error)
	// GetAny loads a page whether or not it is in the trash.
	GetAny(ctx context.Context, tenantID uint64, id string) (*model.Page, error)
	GetByShortID(ctx context.Context, tenantID uint64, shortID string) (*model.Page, error)
	// GetSummaries loads summary columns for a set of live pages.
	GetSummaries(ctx context.Context, tenantID uint64, ids []string) ([]*model.Page, error)
	// GetByKnowledgeIDs loads the live pages mirrored into any of the given
	// knowledge entries, summary columns and knowledge_id only.
	GetByKnowledgeIDs(ctx context.Context, tenantID uint64, knowledgeIDs []string) ([]*model.Page, error)

	// RecentlyEdited lists a space's live pages by when their bodies last
	// changed, newest first. Draft pages are included: somebody coming back
	// to what they were working on is exactly who this list is for.
	RecentlyEdited(ctx context.Context, tenantID uint64, spaceID string, limit int) (
		[]*model.Page, error)
	// GetMany loads live pages with content (duplicate, export).
	GetMany(ctx context.Context, tenantID uint64, ids []string) ([]*model.Page, error)
	// ListChildren returns the live children of parentID (nil = space roots)
	// in reading order, summary columns only.
	ListChildren(ctx context.Context, tenantID uint64, spaceID string, parentID *string) ([]*model.Page, error)
	// ListAncestors returns the live ancestors of a page from the root down,
	// excluding the page itself.
	ListAncestors(ctx context.Context, tenantID uint64, id string) ([]*model.Page, error)
	// SubtreeIDs returns the page and every descendant, trash included.
	SubtreeIDs(ctx context.Context, tenantID uint64, id string) ([]string, error)
	// ListSpaceSummaries pages through every live page of a space by
	// (updated_at desc, id) for indexing and export.
	ListSpaceSummaries(ctx context.Context, tenantID uint64, spaceID string, limit int,
		afterID string) ([]*model.Page, error)
	CountLive(ctx context.Context, tenantID uint64, spaceID string) (int64, error)

	// GetAnyByShortID loads a page by short id whether or not it is trashed.
	GetAnyByShortID(ctx context.Context, tenantID uint64, shortID string) (*model.Page, error)
	// CreateBatch inserts many pages (a duplicated subtree); parents first.
	CreateBatch(ctx context.Context, pages []*model.Page) error
	// LockSpace serialises tree writers of a space until the transaction ends.
	LockSpace(ctx context.Context, tenantID uint64, spaceID string) error
	// ListChildrenAfter pages through the live children of parentID in
	// reading order starting after the cursor (nil = from the start).
	ListChildrenAfter(ctx context.Context, tenantID uint64, spaceID string, parentID *string, after *TreeCursor,
		limit int) ([]*model.Page, error)
	// ChildCounts returns the number of live children of each parent.
	ChildCounts(ctx context.Context, tenantID uint64, parentIDs []string) (map[string]int64, error)
	// LastPosition returns the position of the last live sibling.
	LastPosition(ctx context.Context, tenantID uint64, spaceID string, parentID *string) (string, bool, error)
	// NextPosition returns the position of the live sibling that follows
	// (afterPosition, afterID).
	NextPosition(ctx context.Context, tenantID uint64, spaceID string, parentID *string, afterPosition,
		afterID string) (string, bool, error)
	// SetPositions rewrites the positions of many pages (a rebalance).
	SetPositions(ctx context.Context, tenantID uint64, positions map[string]string) error
	// PurgeOne hard-deletes one trashed page with its subtree.
	PurgeOne(ctx context.Context, tenantID uint64, id string) ([]string, error)

	// Move re-parents a page (and, when the space changes, its subtree
	// together with every dependent row that carries a space id).
	Move(ctx context.Context, tenantID uint64, id string, target MoveTarget) error
	// ListExpiredTrashRoots returns trash roots deleted before the cutoff: a
	// page whose own deleted_at has passed the retention window and whose
	// parent is not itself in the trash, so a subtree is purged from its top
	// rather than one page at a time.
	ListExpiredTrashRoots(ctx context.Context, before time.Time, limit int) ([]*model.Page, error)
	// SetKnowledgeID points a page at its knowledge-base entry, or clears it.
	//
	// Separate from UpdateMeta because that method only touches live pages,
	// and this one must also work on a page in the trash: a trashed page has
	// its entry removed, and the pointer has to be cleared with it or the
	// page is treated as indexed for ever.
	SetKnowledgeID(ctx context.Context, tenantID uint64, pageID string, knowledgeID *string) error
	// UpdateMeta writes non-content columns (title, icon, cover, exclude_from_knowledge,
	// is_locked, template_id, source_refs, position).
	UpdateMeta(ctx context.Context, tenantID uint64, id string, fields map[string]any) error
	// SuggestByTitle returns live pages of the given spaces matching a title
	// substring, newest first, for the link-suggestion menu.
	SuggestByTitle(ctx context.Context, tenantID uint64, spaceIDs []string, query string,
		limit int) ([]*model.Page, error)
	// UpdateContent persists a new body if baseVersion still matches and
	// returns the new version. ErrConflict means someone persisted first.
	UpdateContent(ctx context.Context, tenantID uint64, id string, baseVersion int64, upd ContentUpdate) (int64, error)

	// SoftDeleteSubtree moves a page and its live descendants to the trash,
	// stamping them all with the same instant so Restore can tell them apart
	// from descendants that were already in the trash.
	SoftDeleteSubtree(ctx context.Context, tenantID uint64, id, deletedBy string) (int64, error)
	// RestoreSubtree brings back a trashed page and the descendants trashed
	// with it. If its parent is itself in the trash the page is re-attached at
	// the space root.
	RestoreSubtree(ctx context.Context, tenantID uint64, id string) (int64, error)
	// ListTrash returns trash roots (trashed pages whose parent is live or
	// was trashed separately), newest first.
	ListTrash(ctx context.Context, tenantID uint64, spaceID string) ([]*model.Page, error)
	// PurgeTrash hard-deletes trash roots trashed before the cutoff; children
	// follow through ON DELETE CASCADE. Returns the IDs removed so callers can
	// release attachments.
	PurgeTrash(ctx context.Context, tenantID uint64, before time.Time, limit int) ([]string, error)
}

// MoveTarget describes where a page goes.
type MoveTarget struct {
	SpaceID  string
	ParentID *string // nil = space root
	Position string
}

// ContentUpdate carries everything the persist path writes for a body change.
type ContentUpdate struct {
	Content        model.JSON
	YDoc           []byte
	TextContent    string
	WordCount      int
	EditorID       string
	ContributorIDs model.StringList
	// ContentChanged controls content_updated_at; a persist that only
	// re-encodes an unchanged ydoc must not look like an edit.
	ContentChanged bool
}

type pageRepository struct{ db *gorm.DB }

func (r *pageRepository) Create(ctx context.Context, page *model.Page) error {
	if page.ID == "" {
		page.ID = NewID()
	}
	if page.SourceRefs == nil {
		page.SourceRefs = model.StringList{}
	}
	if page.ContributorIDs == nil {
		page.ContributorIDs = model.StringList{}
	}
	return translateWriteError(r.db.WithContext(ctx).Create(page).Error)
}

func (r *pageRepository) Get(ctx context.Context, tenantID uint64, id string) (*model.Page, error) {
	var p model.Page
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
		First(&p).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &p, nil
}

func (r *pageRepository) GetAny(ctx context.Context, tenantID uint64, id string) (*model.Page, error) {
	var p model.Page
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&p).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &p, nil
}

func (r *pageRepository) GetByShortID(ctx context.Context, tenantID uint64, shortID string) (*model.Page, error) {
	var p model.Page
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND short_id = ? AND deleted_at IS NULL", tenantID, shortID).
		First(&p).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &p, nil
}

func (r *pageRepository) RecentlyEdited(ctx context.Context, tenantID uint64, spaceID string,
	limit int,
) ([]*model.Page, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var pages []*model.Page
	err := r.db.WithContext(ctx).
		Select(model.PageSummaryColumns).
		Where("tenant_id = ? AND space_id = ? AND deleted_at IS NULL", tenantID, spaceID).
		// content_updated_at is only touched when the body changes, so this
		// is "what has been written lately" rather than "what has been
		// renamed or moved lately".
		Order("content_updated_at DESC, updated_at DESC").
		Limit(limit).Find(&pages).Error
	return pages, err
}

func (r *pageRepository) GetSummaries(ctx context.Context, tenantID uint64, ids []string) ([]*model.Page, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []*model.Page
	err := r.db.WithContext(ctx).Select(model.PageSummaryColumns).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).
		Find(&out).Error
	return out, err
}

func (r *pageRepository) GetByKnowledgeIDs(ctx context.Context, tenantID uint64, knowledgeIDs []string,
) ([]*model.Page, error) {
	if len(knowledgeIDs) == 0 {
		return nil, nil
	}
	var out []*model.Page
	err := r.db.WithContext(ctx).Select(append(append([]string{}, model.PageSummaryColumns...), "knowledge_id")).
		Where("tenant_id = ? AND knowledge_id IN ? AND deleted_at IS NULL", tenantID, knowledgeIDs).
		Find(&out).Error
	return out, err
}

func (r *pageRepository) GetMany(ctx context.Context, tenantID uint64, ids []string) ([]*model.Page, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []*model.Page
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).
		Find(&out).Error
	return out, err
}

func (r *pageRepository) ListChildren(ctx context.Context, tenantID uint64, spaceID string,
	parentID *string,
) ([]*model.Page, error) {
	q := r.db.WithContext(ctx).Select(model.PageSummaryColumns).
		Where("tenant_id = ? AND space_id = ? AND deleted_at IS NULL", tenantID, spaceID)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	var out []*model.Page
	err := q.Order("position ASC, id ASC").Find(&out).Error
	return out, err
}

// ancestorsCTE walks parent links upward. depth increases as we climb, so
// ordering by depth DESC yields root first.
const ancestorsCTE = `
WITH RECURSIVE up AS (
    SELECT p.id, p.parent_id, 0 AS depth
    FROM docs_pages p
    WHERE p.id = ? AND p.tenant_id = ?
  UNION ALL
    SELECT q.id, q.parent_id, up.depth + 1
    FROM docs_pages q
    JOIN up ON q.id = up.parent_id
    WHERE up.depth < 200
)
SELECT id, depth FROM up WHERE depth > 0`

func (r *pageRepository) ListAncestors(ctx context.Context, tenantID uint64, id string) ([]*model.Page, error) {
	var rows []struct {
		ID    string
		Depth int
	}
	if err := r.db.WithContext(ctx).Raw(ancestorsCTE, id, tenantID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]string, len(rows))
	order := make(map[string]int, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		order[row.ID] = row.Depth
	}
	pages, err := r.GetSummaries(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	// Deepest depth = root; sort root first.
	sortPages(pages, func(a, b *model.Page) bool { return order[a.ID] > order[b.ID] })
	return pages, nil
}

const subtreeCTE = `
WITH RECURSIVE down AS (
    SELECT p.id
    FROM docs_pages p
    WHERE p.id = ? AND p.tenant_id = ?
  UNION ALL
    SELECT c.id
    FROM docs_pages c
    JOIN down ON c.parent_id = down.id
)
SELECT id FROM down`

func (r *pageRepository) SubtreeIDs(ctx context.Context, tenantID uint64, id string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw(subtreeCTE, id, tenantID).Scan(&ids).Error
	return ids, err
}

func (r *pageRepository) ListSpaceSummaries(ctx context.Context, tenantID uint64, spaceID string, limit int,
	afterID string,
) ([]*model.Page, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	q := r.db.WithContext(ctx).Select(model.PageSummaryColumns).
		Where("tenant_id = ? AND space_id = ? AND deleted_at IS NULL", tenantID, spaceID)
	if afterID != "" {
		q = q.Where("id > ?", afterID)
	}
	var out []*model.Page
	err := q.Order("id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *pageRepository) CountLive(ctx context.Context, tenantID uint64, spaceID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Page{}).
		Where("tenant_id = ? AND space_id = ? AND deleted_at IS NULL", tenantID, spaceID).
		Count(&n).Error
	return n, err
}

func (r *pageRepository) Move(ctx context.Context, tenantID uint64, id string, target MoveTarget) error {
	if target.SpaceID == "" {
		return fmt.Errorf("%w: target space is required", ErrInvalidMove)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txr := &pageRepository{db: tx}
		page, err := txr.Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if target.ParentID != nil {
			if *target.ParentID == id {
				return fmt.Errorf("%w: a page cannot be its own parent", ErrInvalidMove)
			}
			parent, err := txr.Get(ctx, tenantID, *target.ParentID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return fmt.Errorf("%w: target parent not found", ErrInvalidMove)
				}
				return err
			}
			if parent.SpaceID != target.SpaceID {
				return fmt.Errorf("%w: target parent is in another space", ErrInvalidMove)
			}
			subtree, err := txr.SubtreeIDs(ctx, tenantID, id)
			if err != nil {
				return err
			}
			for _, sid := range subtree {
				if sid == *target.ParentID {
					return fmt.Errorf("%w: target parent is inside the moved subtree", ErrInvalidMove)
				}
			}
		}
		ts := now()
		fields := map[string]any{
			"parent_id": target.ParentID, "position": target.Position, "space_id": target.SpaceID, "updated_at": ts,
		}
		res := tx.Model(&model.Page{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(fields)
		if res.Error != nil {
			return res.Error
		}
		if page.SpaceID != target.SpaceID {
			subtree, err := txr.SubtreeIDs(ctx, tenantID, id)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Page{}).
				Where("tenant_id = ? AND id IN ?", tenantID, subtree).
				Updates(map[string]any{"space_id": target.SpaceID, "updated_at": ts}).Error; err != nil {
				return err
			}
			if err := reSpaceRelated(tx, tenantID, subtree, target.SpaceID); err != nil {
				return err
			}
		}
		return nil
	})
}

var pageMetaColumns = map[string]bool{
	"title": true, "icon": true, "cover": true, "exclude_from_knowledge": true, "is_locked": true, "template_id": true,
	"source_refs": true, "position": true, "attachment_bytes": true, "knowledge_id": true,
}

func (r *pageRepository) UpdateMeta(ctx context.Context, tenantID uint64, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	for k := range fields {
		if !pageMetaColumns[k] {
			return fmt.Errorf("docs: page column %q is not updatable through UpdateMeta", k)
		}
	}
	fields["updated_at"] = now()
	res := r.db.WithContext(ctx).Model(&model.Page{}).
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

func (r *pageRepository) UpdateContent(ctx context.Context, tenantID uint64, id string, baseVersion int64,
	upd ContentUpdate,
) (int64, error) {
	ts := now()
	fields := map[string]any{
		"content":         upd.Content,
		"ydoc":            upd.YDoc,
		"ydoc_version":    gorm.Expr("ydoc_version + 1"),
		"text_content":    upd.TextContent,
		"word_count":      upd.WordCount,
		"contributor_ids": upd.ContributorIDs,
		"updated_at":      ts,
	}
	if upd.EditorID != "" {
		fields["last_editor_id"] = upd.EditorID
	}
	if upd.ContentChanged {
		fields["content_updated_at"] = ts
	}
	res := r.db.WithContext(ctx).Model(&model.Page{}).
		Where("tenant_id = ? AND id = ? AND ydoc_version = ? AND deleted_at IS NULL", tenantID, id, baseVersion).
		Updates(fields)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		// Distinguish "someone else persisted" from "no such page".
		if _, err := r.Get(ctx, tenantID, id); err != nil {
			return 0, err
		}
		return 0, ErrConflict
	}
	return baseVersion + 1, nil
}

func (r *pageRepository) SoftDeleteSubtree(ctx context.Context, tenantID uint64, id, deletedBy string) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txr := &pageRepository{db: tx}
		if _, err := txr.Get(ctx, tenantID, id); err != nil {
			return err
		}
		ids, err := txr.SubtreeIDs(ctx, tenantID, id)
		if err != nil {
			return err
		}
		ts := now()
		fields := map[string]any{"deleted_at": ts, "updated_at": ts}
		if deletedBy != "" {
			fields["deleted_by"] = deletedBy
		}
		res := tx.Model(&model.Page{}).
			Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, ids).
			Updates(fields)
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

func (r *pageRepository) RestoreSubtree(ctx context.Context, tenantID uint64, id string) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txr := &pageRepository{db: tx}
		root, err := txr.GetAny(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if root.DeletedAt == nil {
			return ErrNotFound
		}
		ids, err := txr.SubtreeIDs(ctx, tenantID, id)
		if err != nil {
			return err
		}
		ts := now()
		// Only rows trashed in the same operation come back; a descendant that
		// was already in the trash stays there.
		res := tx.Model(&model.Page{}).
			Where("tenant_id = ? AND id IN ? AND deleted_at = ?", tenantID, ids, *root.DeletedAt).
			Updates(map[string]any{"deleted_at": nil, "deleted_by": nil, "updated_at": ts})
		if res.Error != nil {
			return res.Error
		}
		affected = res.RowsAffected
		if root.ParentID != nil {
			parent, err := txr.GetAny(ctx, tenantID, *root.ParentID)
			if err != nil || parent.DeletedAt != nil {
				// Parent is gone or still in the trash: re-attach at the root.
				return tx.Model(&model.Page{}).Where("tenant_id = ? AND id = ?", tenantID, id).
					Updates(map[string]any{"parent_id": nil, "updated_at": ts}).Error
			}
		}
		return nil
	})
	return affected, err
}

func (r *pageRepository) ListTrash(ctx context.Context, tenantID uint64, spaceID string) ([]*model.Page, error) {
	var out []*model.Page
	err := r.db.WithContext(ctx).Table("docs_pages AS p").
		Select(prefixColumns("p", model.PageSummaryColumns)).
		Joins("LEFT JOIN docs_pages AS parent ON parent.id = p.parent_id").
		Where("p.tenant_id = ? AND p.space_id = ? AND p.deleted_at IS NOT NULL", tenantID, spaceID).
		Where("p.parent_id IS NULL OR parent.deleted_at IS NULL OR parent.deleted_at <> p.deleted_at").
		Order("p.deleted_at DESC, p.id ASC").
		Find(&out).Error
	return out, err
}

func (r *pageRepository) PurgeTrash(ctx context.Context, tenantID uint64, before time.Time,
	limit int,
) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	var ids []string
	err := r.db.WithContext(ctx).Table("docs_pages AS p").
		Joins("LEFT JOIN docs_pages AS parent ON parent.id = p.parent_id").
		Where("p.tenant_id = ? AND p.deleted_at IS NOT NULL AND p.deleted_at < ?", tenantID, before).
		Where("p.parent_id IS NULL OR parent.deleted_at IS NULL OR parent.deleted_at <> p.deleted_at").
		Order("p.deleted_at ASC").Limit(limit).
		Pluck("p.id", &ids).Error
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	// Collect the full subtrees first so callers can clean up attachments of
	// descendants that the FK cascade removes.
	var all []string
	for _, id := range ids {
		sub, err := r.SubtreeIDs(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		all = append(all, sub...)
	}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Delete(&model.Page{}).Error; err != nil {
		return nil, err
	}
	return all, nil
}

func prefixColumns(alias string, cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + "." + c
	}
	return out
}

// sortPages is an insertion sort; ancestor chains are short.
func sortPages(pages []*model.Page, less func(a, b *model.Page) bool) {
	for i := 1; i < len(pages); i++ {
		for j := i; j > 0 && less(pages[j], pages[j-1]); j-- {
			pages[j], pages[j-1] = pages[j-1], pages[j]
		}
	}
}

// SuggestByTitle returns live pages of the given spaces whose title matches,
// for the link-suggestion menu.
//
// Matching is a case-insensitive substring, and an empty query returns the
// most recently touched pages, which is what somebody who has just typed the
// trigger and nothing else is most likely reaching for. It is deliberately not
// full-text search: that arrives with its own work package and its own index,
// and a menu under the cursor wants the answer in a millisecond.
func (r *pageRepository) SuggestByTitle(ctx context.Context, tenantID uint64, spaceIDs []string,
	query string, limit int,
) ([]*model.Page, error) {
	if len(spaceIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Select(model.PageSummaryColumns).
		Where("tenant_id = ? AND space_id IN ? AND deleted_at IS NULL", tenantID, spaceIDs)
	if query != "" {
		q = q.Where(`LOWER(title) LIKE ? ESCAPE '\'`, "%"+escapeLike(strings.ToLower(query))+"%")
	}
	var out []*model.Page
	err := q.Order("updated_at DESC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

// escapeLike neutralises the wildcards in a user-supplied pattern, so a query
// of "%" matches that character rather than every page in the space. The
// escape character is named in the query itself, because relying on
// PostgreSQL's default staying the same would be a trap.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	return r.Replace(s)
}

// ListExpiredTrashRoots finds subtrees whose retention window has passed.
//
// Only roots: a page is a trash root when its parent is not also deleted, so
// purging it takes its descendants with it. Without that condition a sweep
// would try to purge children whose parents it had already removed, and the
// count of what it did would be meaningless.
func (r *pageRepository) ListExpiredTrashRoots(ctx context.Context, before time.Time,
	limit int,
) ([]*model.Page, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var rows []*model.Page
	err := r.db.WithContext(ctx).
		Select(model.PageSummaryColumns).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", before).
		Where("parent_id IS NULL OR parent_id NOT IN (?)",
			r.db.Model(&model.Page{}).Select("id").Where("deleted_at IS NOT NULL")).
		Order("deleted_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// SetKnowledgeID points a page at its knowledge entry, or clears it. Works on
// trashed pages too; see the interface comment.
func (r *pageRepository) SetKnowledgeID(ctx context.Context, tenantID uint64, pageID string,
	knowledgeID *string,
) error {
	res := r.db.WithContext(ctx).Model(&model.Page{}).
		Where("tenant_id = ? AND id = ?", tenantID, pageID).
		UpdateColumn("knowledge_id", knowledgeID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
