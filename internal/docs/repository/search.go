package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// Search queries.
//
// All three match on a substring rather than on the tsvector column, for the
// reason set out at the top of internal/docs/search: `simple` tokenisation
// turns a Chinese sentence into one token, so a full-text query silently
// finds nothing in the language most of these documents are written in. A
// deployment with pg_trgm gets an index behind these LIKEs; one without gets
// a scan, bounded by the space filter and the limit.
//
// Every query takes the spaces the caller can see and filters by them in
// SQL. That is not the whole permission check — restricted pages inside a
// visible space are excluded afterwards by the service, which has the
// resolver — but doing the space filter here keeps the scan proportional to
// what the caller could possibly be shown.

// SearchRepository runs the three text queries search is built from.
type SearchRepository interface {
	// Pages matches page titles and bodies.
	Pages(ctx context.Context, tenantID uint64, spaceIDs []string, pattern string,
		limit int) ([]*SearchPageRow, error)
	// Comments matches comment text, returning the page each is on.
	Comments(ctx context.Context, tenantID uint64, spaceIDs []string, pattern string,
		limit int) ([]*SearchCommentRow, error)
	// Transclusions matches text a page shows by reference, returning both
	// the referring page and the page the text came from.
	Transclusions(ctx context.Context, tenantID uint64, spaceIDs []string, pattern string,
		limit int) ([]*SearchTransclusionRow, error)
}

// SearchPageRow is a page whose title or body matched.
type SearchPageRow struct {
	ID          string
	ShortID     string
	SpaceID     string
	Title       string
	TextContent string
}

// SearchCommentRow is a comment whose text matched, with its page.
type SearchCommentRow struct {
	ID          string
	PageID      string
	ShortID     string
	SpaceID     string
	PageTitle   string
	TextContent string
}

// SearchTransclusionRow is referenced text that matched.
//
// ReferringPageID is the page a reader would open — the one showing the text.
// SourcePageID is where the text lives, and is what the caller's permission
// is checked against: showing somebody text from a page they may not read
// would be a leak however it reached the screen.
type SearchTransclusionRow struct {
	ReferringPageID string
	ReferringShort  string
	ReferringTitle  string
	ReferringSpace  string
	SourcePageID    string
	TextContent     string
}

type searchRepository struct{ db *gorm.DB }

// like is the dialect's case-insensitive substring operator.
//
// Postgres ILIKE folds case for Latin text; SQLite's LIKE already does for
// ASCII. Neither folds case for Chinese, which has none, so the two behave
// identically on the text this mostly runs against.
func (r *searchRepository) like() string {
	if r.db.Dialector.Name() == "postgres" {
		return "ILIKE"
	}
	return "LIKE"
}

func searchLimit(limit int) int {
	if limit <= 0 || limit > 500 {
		return 100
	}
	return limit
}

func (r *searchRepository) Pages(ctx context.Context, tenantID uint64, spaceIDs []string,
	pattern string, limit int,
) ([]*SearchPageRow, error) {
	if len(spaceIDs) == 0 || pattern == "" {
		return nil, nil
	}
	op := r.like()
	wild := "%" + pattern + "%"
	var rows []*SearchPageRow
	err := r.db.WithContext(ctx).
		Table("docs_pages").
		Select("id, short_id, space_id, title, text_content").
		Where("tenant_id = ? AND space_id IN ? AND deleted_at IS NULL", tenantID, spaceIDs).
		// ESCAPE is explicit: SQLite has no escape character unless one is
		// declared, so a query containing % would otherwise be a wildcard.
		Where("title "+op+" ? ESCAPE '\\' OR text_content "+op+" ? ESCAPE '\\'", wild, wild).
		Order("content_updated_at DESC NULLS LAST, updated_at DESC").
		Limit(searchLimit(limit)).
		Find(&rows).Error
	return rows, err
}

func (r *searchRepository) Comments(ctx context.Context, tenantID uint64, spaceIDs []string,
	pattern string, limit int,
) ([]*SearchCommentRow, error) {
	if len(spaceIDs) == 0 || pattern == "" {
		return nil, nil
	}
	op := r.like()
	wild := "%" + pattern + "%"
	var rows []*SearchCommentRow
	err := r.db.WithContext(ctx).
		Table("docs_comments AS c").
		Select("c.id AS id, c.page_id AS page_id, p.short_id AS short_id, "+
			"c.space_id AS space_id, p.title AS page_title, c.text_content AS text_content").
		Joins("JOIN docs_pages p ON p.id = c.page_id").
		Where("c.tenant_id = ? AND c.space_id IN ?", tenantID, spaceIDs).
		// A comment on a trashed page is not a result: the page it belongs to
		// cannot be opened.
		Where("c.deleted_at IS NULL AND p.deleted_at IS NULL").
		Where("c.text_content "+op+" ? ESCAPE '\\'", wild).
		Order("c.created_at DESC").
		Limit(searchLimit(limit)).
		Find(&rows).Error
	return rows, err
}

// Transclusions finds text one page shows from another.
//
// The join is page-level rather than block-level, because docs_links records
// "A references B" without saying which block. A page that references one
// block of B can therefore match on the text of another block of B that it
// does not show. That is a precision loss and not a leak: the result is
// filtered by permission on B, and anybody who may read B could have found
// the same text by searching B directly. Making it exact would need a
// block-level reference table, which is not worth a column for a wrong
// result somebody clicks once.
func (r *searchRepository) Transclusions(ctx context.Context, tenantID uint64, spaceIDs []string,
	pattern string, limit int,
) ([]*SearchTransclusionRow, error) {
	if len(spaceIDs) == 0 || pattern == "" {
		return nil, nil
	}
	op := r.like()
	wild := "%" + pattern + "%"
	var rows []*SearchTransclusionRow
	err := r.db.WithContext(ctx).
		Table("docs_transclusion_blocks AS tb").
		Select("DISTINCT l.source_page_id AS referring_page_id, rp.short_id AS referring_short, "+
			"rp.title AS referring_title, rp.space_id AS referring_space, "+
			"tb.page_id AS source_page_id, tb.text_content AS text_content").
		Joins("JOIN docs_links l ON l.target_page_id = tb.page_id AND l.kind = ?",
			string(model.LinkTransclusion)).
		Joins("JOIN docs_pages rp ON rp.id = l.source_page_id").
		Where("tb.tenant_id = ? AND rp.space_id IN ? AND rp.deleted_at IS NULL", tenantID, spaceIDs).
		Where("tb.text_content "+op+" ? ESCAPE '\\'", wild).
		Limit(searchLimit(limit)).
		Find(&rows).Error
	return rows, err
}
