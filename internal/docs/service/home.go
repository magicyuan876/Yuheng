package service

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
)

// What somebody sees when they arrive: what they starred, what has been
// written lately, what is filed under a label.
//
// Every list here is filtered by what the caller may actually open, one page
// at a time. That is the rule the whole module turns on, and it is worth
// saying plainly where lists are built: a page somebody may not read must not
// appear even as a title, because a title is information — "there is a page
// called Q3 Redundancies" is the leak, not its contents.
//
// ---- decision: "recently viewed" is not stored
//
// The design asks for recently-edited and recently-viewed lists. The first is
// free: pages carry content_updated_at, and reading them back in that order
// costs one indexed query. The second would need a write on every page view —
// the most frequent thing that happens in a documentation tool — to record
// something that is only ever shown back to the person who did it, on the
// device they did it on.
//
// So it is kept in the browser, where it belongs, and there is no table for
// it. The trade is that it does not follow somebody between devices; against
// that is not turning every read into a write. If it ever needs to be shared
// across devices, it becomes a table then, and the API below is where it
// would surface.

// FavouriteKind names what a favourite points at.
const (
	FavouritePage  = types.ResourceTypeDocPage
	FavouriteSpace = types.ResourceTypeDocSpace
)

// MaxRecentPages bounds a "recently edited" list.
const MaxRecentPages = 20

// SpaceHome is what a space's landing page shows.
type SpaceHome struct {
	// RecentlyEdited is the pages whose bodies changed most recently, and
	// which this caller may open.
	RecentlyEdited []*TreeNode `json:"recently_edited"`
	// Labels is the space's vocabulary, so the page can offer filters.
	Labels []*LabelView `json:"labels"`
	// Favourites is this caller's starred pages in this space.
	Favourites []*TreeNode `json:"favourites"`
}

// Home gathers a space's landing page in one request.
//
// One call rather than four, because a landing page that paints in four
// stages reads as a page that is broken.
func (s *PageService) Home(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole,
) (*SpaceHome, error) {
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	out := &SpaceHome{
		RecentlyEdited: []*TreeNode{}, Labels: []*LabelView{}, Favourites: []*TreeNode{},
	}

	recent, err := s.RecentlyEdited(ctx, actor, space, role, MaxRecentPages)
	if err != nil {
		return nil, err
	}
	out.RecentlyEdited = recent

	labels, err := s.Labels(ctx, actor, space, role)
	if err != nil {
		return nil, err
	}
	out.Labels = labels

	favourites, err := s.Favourites(ctx, actor, space)
	if err != nil {
		return nil, err
	}
	out.Favourites = favourites
	return out, nil
}

// RecentlyEdited lists the pages of a space whose bodies changed most
// recently and that this caller may open.
func (s *PageService) RecentlyEdited(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, limit int,
) ([]*TreeNode, error) {
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxRecentPages {
		limit = MaxRecentPages
	}
	// Asked for generously and then filtered: a reader who may open few of a
	// space's pages should still get a full list rather than a short one.
	pages, err := s.d.Repos.Pages.RecentlyEdited(ctx, space.TenantID, space.ID, limit*4)
	if err != nil {
		return nil, err
	}
	return s.visibleNodes(ctx, actor, pages, limit)
}

// Favourites lists this caller's starred pages, optionally within one space.
//
// Starring is per person, so the list is theirs; but a page they starred and
// have since lost access to is left out rather than shown as an unopenable
// row, for the same reason a backlink to an invisible page is left out.
func (s *PageService) Favourites(ctx context.Context, actor *acl.Identity, space *model.Space) (
	[]*TreeNode, error,
) {
	if s.d.Favourites == nil {
		return []*TreeNode{}, nil
	}
	rows, err := s.d.Favourites.List(ctx, actor.UserID, actor.TenantID, FavouritePage)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ResourceID)
	}
	if len(ids) == 0 {
		return []*TreeNode{}, nil
	}

	pages, err := s.d.Repos.Pages.GetSummaries(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	if space != nil {
		kept := pages[:0]
		for _, page := range pages {
			if page.SpaceID == space.ID {
				kept = append(kept, page)
			}
		}
		pages = kept
	}
	return s.visibleNodes(ctx, actor, pages, 0)
}

// SetFavourite stars a page or unstars it.
func (s *PageService) SetFavourite(ctx context.Context, actor *acl.Identity, d acl.Decision,
	favourite bool,
) error {
	// Reading a page is enough to star it: a favourite is a bookmark, not a
	// change to anything.
	if err := requireRole(d, model.RoleReader); err != nil {
		return err
	}
	if s.d.Favourites == nil {
		return nil
	}
	if favourite {
		return s.d.Favourites.Add(ctx, actor.UserID, actor.TenantID, FavouritePage, d.Page.ID)
	}
	return s.d.Favourites.Remove(ctx, actor.UserID, actor.TenantID, FavouritePage, d.Page.ID)
}

// IsFavourite reports whether this caller starred a page.
func (s *PageService) IsFavourite(ctx context.Context, actor *acl.Identity, pageID string) bool {
	if s.d.Favourites == nil {
		return false
	}
	rows, err := s.d.Favourites.List(ctx, actor.UserID, actor.TenantID, FavouritePage)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if row.ResourceID == pageID {
			return true
		}
	}
	return false
}

// PagesWithLabels lists a space's live pages carrying every one of the given
// labels, filtered by what the caller may open.
func (s *PageService) PagesWithLabels(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, labelIDs []string, limit int,
) ([]*TreeNode, error) {
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Labels == nil || len(labelIDs) == 0 {
		return []*TreeNode{}, nil
	}
	pages, err := s.d.Repos.Labels.PagesWith(ctx, space.TenantID, space.ID, labelIDs, 0)
	if err != nil {
		return nil, err
	}
	return s.visibleNodes(ctx, actor, pages, limit)
}

// visibleNodes keeps the pages this caller may open, in the order given,
// stopping at limit (0 for no limit).
//
// The permission check is per page rather than per space because a restricted
// subtree inside a space somebody can otherwise read is exactly the case
// these lists must not leak.
func (s *PageService) visibleNodes(ctx context.Context, actor *acl.Identity,
	pages []*model.Page, limit int,
) ([]*TreeNode, error) {
	kept := make([]*model.Page, 0, len(pages))
	roles := make(map[string]model.SpaceRole, len(pages))
	for _, page := range pages {
		decision, err := s.d.Resolver.Decide(ctx, actor, page)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		kept = append(kept, page)
		roles[page.ID] = decision.Role
		if limit > 0 && len(kept) >= limit {
			break
		}
	}
	if len(kept) == 0 {
		return []*TreeNode{}, nil
	}

	// Which of them cut inheritance, in one query rather than per row: the
	// flag is drawn as a badge, and a badge is not worth N round trips.
	ids := make([]string, 0, len(kept))
	for _, page := range kept {
		ids = append(ids, page.ID)
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}

	out := make([]*TreeNode, 0, len(kept))
	for _, page := range kept {
		out = append(out, &TreeNode{
			Page: page, CanEdit: canEdit(roles[page.ID], page),
			Restricted: restricted[page.ID],
		})
	}
	return out, nil
}
