package service

import (
	"context"
	"sort"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Page links, mentions and backlinks.
//
// A page link stores nothing but the target's id. Its text is resolved when it
// is shown, which is what makes renaming a page update every link to it
// without touching a single document — and what makes a link to a page the
// reader cannot see render as unavailable rather than leaking its title.
//
// The stored link rows exist only to answer the opposite question, "what
// points at this page", which no document can answer on its own.

// MaxSuggestions bounds a suggestion list. It is a menu under a cursor, not a
// search: past a dozen entries nobody reads further, and a longer list only
// gives a caller a cheaper way to enumerate a space.
const MaxSuggestions = 12

// MaxTitleLookup bounds one batch of title resolutions, which is one open
// page's worth of links.
const MaxTitleLookup = 200

// PageRefView is a page as a link, a suggestion or a backlink shows it.
type PageRefView struct {
	PageID  string `json:"page_id"`
	ShortID string `json:"short_id,omitempty"`
	SpaceID string `json:"space_id,omitempty"`
	Title   string `json:"title"`
	Icon    string `json:"icon,omitempty"`
	// Breadcrumb is the ancestor titles, root first, so two pages with the
	// same name can be told apart in a suggestion list.
	Breadcrumb []string `json:"breadcrumb,omitempty"`
	// Resolved is false when the page is gone or the reader may not see it.
	// The editor draws the link as broken; it never says which of the two.
	Resolved bool `json:"resolved"`
}

// MentionView is a person a mention can point at.
type MentionView struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// ---- recording what a document refers to ----------------------------------

// recordLinks rebuilds one page's outgoing links from the document just saved.
//
// It runs inside the shared persist path, so it covers every way a body is
// written: somebody typing, the collaboration service storing, the REST
// provider saving, and a replace from an import or a restore. Rebuilding from
// the document rather than tracking edits is the only correct approach —
// deleting the text around a link is how a link is removed, and that produces
// no event of its own.
func (b *base) recordLinks(ctx context.Context, page *model.Page, st render.Structure) {
	if b.d.Repos.Links == nil {
		return
	}
	links := make([]model.Link, 0, len(st.PageLinks)+len(st.Transclusions))
	for _, target := range st.PageLinks {
		links = append(links, model.Link{TargetPageID: target, Kind: model.LinkPage})
	}
	seen := map[string]bool{}
	for _, t := range st.Transclusions {
		if t.SourcePageID == "" || seen[t.SourcePageID] {
			continue
		}
		seen[t.SourcePageID] = true
		links = append(links, model.Link{TargetPageID: t.SourcePageID, Kind: model.LinkTransclusion})
	}
	if err := b.d.Repos.Links.ReplaceForSource(ctx, page.TenantID, page.ID, links); err != nil {
		// The page itself is already stored. Losing a backlink row degrades a
		// sidebar, and the next save of this page rebuilds it.
		logger.Warnf(ctx, "[docs] recording the links of page %s failed: %v", page.ID, err)
	}
}

// ---- reading ---------------------------------------------------------------

// Backlinks lists the pages that refer to this one and that the caller may
// see. A page the reader cannot open is left out entirely rather than shown as
// an unnamed entry: the fact that some page links here is itself information.
func (s *PageService) Backlinks(ctx context.Context, actor *acl.Identity,
	d acl.Decision,
) ([]*PageRefView, error) {
	if s.d.Repos.Links == nil {
		return []*PageRefView{}, nil
	}
	pages, err := s.d.Repos.Links.Backlinks(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*PageRefView, 0, len(pages))
	for _, p := range pages {
		decision, err := s.d.Resolver.Decide(ctx, actor, p)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		out = append(out, pageRef(p, nil))
	}
	return out, nil
}

// ResolveTitles answers what a set of page links should read as.
//
// Unresolved covers both "deleted" and "you may not see it", deliberately: the
// editor shows one broken-link state for both, so a link cannot be used to
// probe whether a page exists.
func (s *PageService) ResolveTitles(ctx context.Context, actor *acl.Identity,
	ids []string,
) ([]*PageRefView, error) {
	ids = dedupe(ids)
	if len(ids) > MaxTitleLookup {
		return nil, invalid("at most %d page ids may be resolved at once", MaxTitleLookup)
	}
	out := make([]*PageRefView, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	pages, err := s.d.Repos.Pages.GetSummaries(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Page, len(pages))
	for _, p := range pages {
		decision, err := s.d.Resolver.Decide(ctx, actor, p)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		byID[p.ID] = p
	}
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, pageRef(p, nil))
			continue
		}
		out = append(out, &PageRefView{PageID: id, Resolved: false})
	}
	return out, nil
}

// SuggestPagesInput narrows a page suggestion list.
type SuggestPagesInput struct {
	Query string
	// SpaceID restricts the suggestions to one space; empty searches every
	// space the caller can read, which is what a link typed from scratch
	// wants.
	SpaceID string
	Limit   int
}

// SuggestPages answers the `[[` menu.
//
// Every candidate is resolved against the caller before it is offered, so a
// page they may not open never appears — not even as a title. That is the
// difference between a convenience and a directory of everything in the
// workspace.
func (s *PageService) SuggestPages(ctx context.Context, actor *acl.Identity,
	in SuggestPagesInput,
) ([]*PageRefView, error) {
	limit := in.Limit
	if limit <= 0 || limit > MaxSuggestions {
		limit = MaxSuggestions
	}
	spaces, err := s.readableSpaces(ctx, actor, in.SpaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*PageRefView, 0, limit)
	if len(spaces) == 0 {
		return out, nil
	}

	query := strings.ToLower(strings.TrimSpace(in.Query))
	// Over-fetch: candidates are filtered by page permission afterwards, and
	// a restricted subtree could otherwise empty the menu.
	matches, err := s.d.Repos.Pages.SuggestByTitle(ctx, actor.TenantID, spaces, query, limit*4)
	if err != nil {
		return nil, err
	}
	for _, p := range matches {
		if len(out) >= limit {
			break
		}
		decision, err := s.d.Resolver.Decide(ctx, actor, p)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		out = append(out, pageRef(p, nil))
	}
	return out, nil
}

// SuggestMentions answers the `@` menu with the people who can already read
// the page being edited. Mentioning somebody who cannot open the page would
// send them a notification about something they cannot look at.
func (s *PageService) SuggestMentions(ctx context.Context, actor *acl.Identity, d acl.Decision,
	query string, limit int,
) ([]*MentionView, error) {
	if limit <= 0 || limit > MaxSuggestions {
		limit = MaxSuggestions
	}
	if s.d.Members == nil {
		return []*MentionView{}, nil
	}
	members, err := s.d.Members.ListPagedByTenant(ctx, actor.TenantID, "", 0, 500)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		if m != nil && m.UserID != "" {
			ids = append(ids, m.UserID)
		}
	}
	users := s.users(ctx, ids)

	needle := strings.ToLower(strings.TrimSpace(query))
	out := make([]*MentionView, 0, limit)
	for _, id := range ids {
		view := userView(id, users)
		if needle != "" && !strings.Contains(strings.ToLower(view.Username+" "+view.Email), needle) {
			continue
		}
		// Each candidate is checked against this page, not against the space:
		// a restricted page narrows who may be mentioned on it.
		identity, err := s.d.Resolver.Identity(ctx, actor.TenantID, id)
		if err != nil {
			return nil, err
		}
		if !identity.Member {
			continue
		}
		decision, err := s.d.Resolver.Decide(ctx, identity, d.Page)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		out = append(out, &MentionView{
			UserID: view.UserID, Username: view.Username, Email: view.Email, Avatar: view.Avatar,
		})
		if len(out) >= limit {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

// readableSpaces resolves which spaces a suggestion may draw from.
func (s *PageService) readableSpaces(ctx context.Context, actor *acl.Identity,
	spaceID string,
) ([]string, error) {
	if spaceID != "" {
		space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, spaceID)
		if err != nil {
			return nil, notFound("space")
		}
		role, err := s.d.Resolver.SpaceRole(ctx, actor, space)
		if err != nil {
			return nil, err
		}
		if role == model.RoleNone {
			return nil, notFound("space")
		}
		return []string{space.ID}, nil
	}
	roles, err := s.d.Resolver.VisibleSpaces(ctx, actor)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(roles))
	for id, role := range roles {
		if role != model.RoleNone {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}

func pageRef(p *model.Page, breadcrumb []string) *PageRefView {
	out := &PageRefView{
		PageID: p.ID, ShortID: p.ShortID, SpaceID: p.SpaceID,
		Title: p.Title, Breadcrumb: breadcrumb, Resolved: true,
	}
	if p.Icon != nil {
		out.Icon = *p.Icon
	}
	return out
}
