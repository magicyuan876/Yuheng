package service

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/search"
)

// Searching the module.
//
// Three sources, one result list:
//
//   - PAGES, by title and body. The obvious one.
//   - COMMENTS, because a decision recorded in a comment thread is as much
//     part of the record as one recorded in the document, and somebody
//     looking for "why did we drop the Redis dependency" is as likely to
//     find the answer in a thread as in a paragraph.
//   - TEXT SHOWN BY REFERENCE, which is the debt T2.5 recorded. A page that
//     transcludes a block reads, to its reader, as though that text is part
//     of it; searching for that text should therefore find that page.
//
// ---- the permission rule, which is the whole difficulty
//
// Every result is filtered twice. The SQL filters by the spaces the caller
// can see, which keeps the scan proportional. The service then filters page
// by page through the resolver, because a restricted page inside a visible
// space is exactly what the first filter cannot catch.
//
// The third source is filtered against the SOURCE page rather than the page
// being returned. A page A that shows text from page B is only a result for
// somebody who may read B — otherwise search would be a way to read B's
// contents through A, which is precisely the side channel T2.5 refused to
// open by keeping this text out of A's own text_content. The check has to be
// here because this is the only place that knows both pages.

// MaxSearchResults bounds a response.
const MaxSearchResults = 50

// searchFanout is how many candidates each source fetches before permission
// filtering. Generous, because a caller who may open few of a space's pages
// should still get a full page of results rather than a short one.
const searchFanout = 4

// SearchHit is one result.
type SearchHit struct {
	// Kind is "page", "comment" or "transclusion".
	Kind string `json:"kind"`
	// PageID and ShortID identify the page to open.
	PageID  string `json:"page_id"`
	ShortID string `json:"short_id"`
	SpaceID string `json:"space_id"`
	// SpaceSlug is what a URL is built from. Carried on the hit rather than
	// looked up by the client, because results come from any space the
	// caller can read and a client that guessed the current one would
	// navigate into the wrong space.
	SpaceSlug string `json:"space_slug"`
	Title     string `json:"title"`
	// Excerpt is the matching text with surrounding context.
	Excerpt string `json:"excerpt"`
	// CommentID is set on a comment hit, so a client can scroll to it.
	CommentID string `json:"comment_id,omitempty"`
	// SourcePageID is set on a transclusion hit: the page the text lives on,
	// which is also the page whose permissions allowed this result.
	SourcePageID string  `json:"source_page_id,omitempty"`
	Score        float64 `json:"score"`
}

// SortKey implements search.Sortable. The page id is the tie-break, so two
// equally-scored untitled pages keep a stable order between requests.
func (h *SearchHit) SortKey() (float64, string, string) {
	return h.Score, h.Title, h.PageID + h.CommentID
}

// SearchResults is a response.
type SearchResults struct {
	Query string       `json:"query"`
	Hits  []*SearchHit `json:"hits"`
	// Truncated is true when more matched than were returned.
	Truncated bool `json:"truncated"`
}

// Search finds pages, comments and referenced text the caller may read.
func (s *PageService) Search(ctx context.Context, actor *acl.Identity, raw string,
	spaceID string, limit int,
) (*SearchResults, error) {
	q := search.Parse(raw)
	out := &SearchResults{Query: q.Text, Hits: []*SearchHit{}}
	if q.Empty() || s.d.Repos.Search == nil {
		return out, nil
	}
	if limit <= 0 || limit > MaxSearchResults {
		limit = MaxSearchResults
	}

	spaceIDs, err := s.searchableSpaces(ctx, actor, spaceID)
	if err != nil {
		return nil, err
	}
	if len(spaceIDs) == 0 {
		return out, nil
	}

	fanout := limit * searchFanout
	hits, err := s.gatherHits(ctx, actor, q, spaceIDs, fanout)
	if err != nil {
		return nil, err
	}

	if err := s.attachSpaceSlugs(ctx, actor.TenantID, hits); err != nil {
		return nil, err
	}
	search.Sort(hits)
	if len(hits) > limit {
		hits = hits[:limit]
		out.Truncated = true
	}
	for _, hit := range hits {
		out.Hits = append(out.Hits, hit)
	}
	return out, nil
}

// searchableSpaces is the set the SQL may look in: every space the caller can
// read, or the one they asked for if they may read it.
func (s *PageService) searchableSpaces(ctx context.Context, actor *acl.Identity, spaceID string) (
	[]string, error,
) {
	visible, err := s.d.Resolver.VisibleSpaces(ctx, actor)
	if err != nil {
		return nil, err
	}
	if spaceID != "" {
		// Naming a space you cannot see returns nothing rather than an
		// error: whether a space exists under a given id is not something
		// search should answer.
		if _, ok := visible[spaceID]; !ok {
			return nil, nil
		}
		return []string{spaceID}, nil
	}
	out := make([]string, 0, len(visible))
	for id := range visible {
		out = append(out, id)
	}
	return out, nil
}

// gatherHits runs the three queries and keeps what the caller may read.
func (s *PageService) gatherHits(ctx context.Context, actor *acl.Identity, q search.Query,
	spaceIDs []string, fanout int,
) ([]*SearchHit, error) {
	// readable caches the per-page decision across all three sources: a page
	// that holds a match and three commented threads is resolved once.
	readable := map[string]bool{}
	allow := func(pageID string) bool {
		if seen, ok := readable[pageID]; ok {
			return seen
		}
		d, err := s.d.Resolver.Page(ctx, actor, pageID)
		ok := err == nil && d.Role != model.RoleNone
		readable[pageID] = ok
		return ok
	}

	var hits []*SearchHit

	pages, err := s.d.Repos.Search.Pages(ctx, actor.TenantID, spaceIDs, q.Pattern, fanout)
	if err != nil {
		return nil, err
	}
	for _, row := range pages {
		if !allow(row.ID) {
			continue
		}
		hits = append(hits, &SearchHit{
			Kind: string(search.KindPage), PageID: row.ID, ShortID: row.ShortID,
			SpaceID: row.SpaceID, Title: row.Title,
			Excerpt: search.Excerpt(q, row.TextContent),
			Score:   search.Score(q, search.KindPage, row.Title, row.TextContent),
		})
	}

	comments, err := s.d.Repos.Search.Comments(ctx, actor.TenantID, spaceIDs, q.Pattern, fanout)
	if err != nil {
		return nil, err
	}
	for _, row := range comments {
		// A comment is readable exactly when its page is: commenting needs
		// only read access, so the page's permission is the comment's.
		if !allow(row.PageID) {
			continue
		}
		hits = append(hits, &SearchHit{
			Kind: string(search.KindComment), PageID: row.PageID, ShortID: row.ShortID,
			SpaceID: row.SpaceID, Title: row.PageTitle, CommentID: row.ID,
			Excerpt: search.Excerpt(q, row.TextContent),
			Score:   search.Score(q, search.KindComment, row.PageTitle, row.TextContent),
		})
	}

	referenced, err := s.d.Repos.Search.Transclusions(ctx, actor.TenantID, spaceIDs, q.Pattern, fanout)
	if err != nil {
		return nil, err
	}
	for _, row := range referenced {
		// BOTH pages. The source decides whether the text may be shown at
		// all — this is the check T2.5 deferred to here — and the referring
		// page is the one being returned, so it has to be readable too.
		if !allow(row.SourcePageID) || !allow(row.ReferringPageID) {
			continue
		}
		hits = append(hits, &SearchHit{
			Kind: string(search.KindTransclusion), PageID: row.ReferringPageID,
			ShortID: row.ReferringShort, SpaceID: row.ReferringSpace,
			Title: row.ReferringTitle, SourcePageID: row.SourcePageID,
			Excerpt: search.Excerpt(q, row.TextContent),
			Score:   search.Score(q, search.KindTransclusion, row.ReferringTitle, row.TextContent),
		})
	}

	return dedupeHits(hits), nil
}

// dedupeHits keeps the strongest hit per (page, kind, comment).
//
// A page whose title and body both match is one result, not two; a page that
// references three blocks of the same source is one result rather than
// three. Without this a single well-matched page can fill the whole list.
func dedupeHits(hits []*SearchHit) []*SearchHit {
	type key struct{ page, kind, extra string }
	best := map[key]*SearchHit{}
	order := make([]key, 0, len(hits))

	for _, hit := range hits {
		extra := hit.CommentID
		if hit.Kind == string(search.KindTransclusion) {
			extra = hit.SourcePageID
		}
		k := key{page: hit.PageID, kind: hit.Kind, extra: extra}
		if existing, ok := best[k]; ok {
			if hit.Score > existing.Score {
				best[k] = hit
			}
			continue
		}
		best[k] = hit
		order = append(order, k)
	}

	out := make([]*SearchHit, 0, len(order))
	for _, k := range order {
		if hit := best[k]; hit != nil && hit.Score > 0 {
			out = append(out, hit)
		}
	}
	return out
}

// attachSpaceSlugs fills in the slug each hit's URL is built from.
//
// One lookup per distinct space rather than per hit: a page of results
// usually comes from two or three spaces.
func (s *PageService) attachSpaceSlugs(ctx context.Context, tenantID uint64,
	hits []*SearchHit,
) error {
	slugs := map[string]string{}
	for _, hit := range hits {
		if hit.SpaceID == "" || slugs[hit.SpaceID] != "" {
			continue
		}
		space, err := s.d.Repos.Spaces.Get(ctx, tenantID, hit.SpaceID)
		if err != nil {
			// A space that has gone between the query and now: the hit keeps
			// an empty slug and the client falls back to opening by id.
			continue
		}
		slugs[hit.SpaceID] = space.Slug
	}
	for _, hit := range hits {
		hit.SpaceSlug = slugs[hit.SpaceID]
	}
	return nil
}
