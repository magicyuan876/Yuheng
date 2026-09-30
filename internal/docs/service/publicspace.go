package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// Public spaces: a whole space readable without logging in.
//
// The same idea as a share link and deliberately the same code path for
// everything that matters — a visitor gets SharedPage, restricted pages are
// invisible, trashed pages are invisible, and page links resolve only inside
// what the visitor can already see. The differences are only in how the
// audience is named:
//
//   - A share link is a capability: holding the URL is the permission, and
//     the link says which page and whether the subtree comes with it.
//   - A public space is a property of the space: everything live and
//     unrestricted in it is readable, and the space's slug is the address.
//
// Both are behind the same deployment switch, because both put content on the
// public internet and an installation that said no to one did not say yes to
// the other.

// MaxPublicSpacePages bounds how many page titles are resolved for a public
// space's internal links in one request. Past this, an internal link renders
// as plain text — the wrong way round is publishing a title by accident.
const MaxPublicSpacePages = 2000

// publicTitlePage is one batch of that walk; the repository caps a page at
// 1000 rows of its own accord.
const publicTitlePage = 500

// PublicSpaceView is a public space's landing information.
type PublicSpaceView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// Pages is the top level of the space, in tree order.
	Pages []SharedRef `json:"pages"`
}

// PublicSpace returns a public space's landing information.
func (s *PageService) PublicSpace(ctx context.Context, spaceID string) (*PublicSpaceView, error) {
	space, err := s.publicSpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	roots, err := s.publicChildren(ctx, space, nil)
	if err != nil {
		return nil, err
	}
	out := &PublicSpaceView{
		ID: space.ID, Name: space.Name, Slug: space.Slug, Description: space.Description,
		Icon: text(space.Icon), Pages: roots,
	}
	return out, nil
}

// PublicSpacePage renders one page of a public space.
func (s *PageService) PublicSpacePage(ctx context.Context, spaceID, shortID string) (*SharedPage, error) {
	space, err := s.publicSpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	page, err := s.d.Repos.Pages.GetByShortID(ctx, space.TenantID, shortID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("page")
		}
		return nil, err
	}
	// The page must be in THIS space. Without this, a short id from a private
	// space would render through a public space's address.
	if page.SpaceID != space.ID {
		return nil, notFound("page")
	}
	if s.anyRestricted(ctx, space.TenantID, page) {
		return nil, notFound("page")
	}

	titles, err := s.publicTitles(ctx, space)
	if err != nil {
		return nil, err
	}
	html, err := s.renderForVisitor(page, titles, publicSpaceAttachmentURL(space.ID))
	if err != nil {
		return nil, err
	}
	kids, err := s.publicChildren(ctx, space, &page.ID)
	if err != nil {
		return nil, err
	}
	breadcrumb, err := s.publicBreadcrumb(ctx, space, page)
	if err != nil {
		return nil, err
	}
	return &SharedPage{
		Title: page.Title, Icon: text(page.Icon), HTML: html, ShortID: page.ShortID,
		UpdatedAt: when(page.ContentUpdatedAt, page.UpdatedAt),
		Children:  kids, Breadcrumb: breadcrumb,
		// A public space is meant to be found, unlike a share link, which is
		// meant to be given to somebody.
		AllowSearchIndex: true,
		SpaceName:        space.Name,
	}, nil
}

// ---- helpers -------------------------------------------------------------------

// publicSpace loads a space for an anonymous visitor.
//
// Addressed by id rather than by slug, and this is the reason: a slug is
// unique within a tenant, and a visitor has no tenant, so two tenants could
// each have a "handbook". The id is globally unique, which makes the address
// unambiguous without a lookup table. The slug still travels in the response
// for display.
//
// "Not public" is reported as "not found": whether a private space exists
// under a given id is not a visitor's business.
func (s *PageService) publicSpace(ctx context.Context, spaceID string) (*model.Space, error) {
	if !s.d.PublicSharing {
		return nil, notFound("space")
	}
	space, err := s.d.Repos.Spaces.GetPublicByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("space")
		}
		return nil, err
	}
	return space, nil
}

// publicChildren lists the live, unrestricted children of a page (or the
// space's roots when parentID is nil).
func (s *PageService) publicChildren(ctx context.Context, space *model.Space, parentID *string) (
	[]SharedRef, error,
) {
	kids, err := s.d.Repos.Pages.ListChildren(ctx, space.TenantID, space.ID, parentID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(kids))
	for _, k := range kids {
		ids = append(ids, k.ID)
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, space.TenantID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]SharedRef, 0, len(kids))
	for _, k := range kids {
		if restricted[k.ID] {
			continue
		}
		out = append(out, SharedRef{ShortID: k.ShortID, Title: k.Title, Icon: text(k.Icon)})
	}
	return out, nil
}

// publicBreadcrumb is the ancestor chain, stopping at the first restricted
// level: a visitor is shown the path they could have walked, not one that
// runs through a page they cannot open.
func (s *PageService) publicBreadcrumb(ctx context.Context, space *model.Space, page *model.Page) (
	[]SharedRef, error,
) {
	ancestors, err := s.d.Repos.Pages.ListAncestors(ctx, space.TenantID, page.ID)
	if err != nil {
		return nil, err
	}
	if len(ancestors) == 0 {
		return []SharedRef{}, nil
	}
	ids := make([]string, 0, len(ancestors))
	for _, a := range ancestors {
		ids = append(ids, a.ID)
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, space.TenantID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]SharedRef, 0, len(ancestors))
	for _, a := range ancestors {
		if restricted[a.ID] {
			// Everything above this is behind it too; a page under a
			// restricted ancestor is unreachable anyway and anyRestricted
			// will already have refused it.
			return []SharedRef{}, nil
		}
		out = append(out, SharedRef{ShortID: a.ShortID, Title: a.Title, Icon: text(a.Icon)})
	}
	return out, nil
}

// anyRestricted reports whether a page or any ancestor cuts inheritance.
func (s *PageService) anyRestricted(ctx context.Context, tenantID uint64, page *model.Page) bool {
	ancestors, err := s.d.Repos.Pages.ListAncestors(ctx, tenantID, page.ID)
	if err != nil {
		// Not being able to prove a page is unrestricted is not permission to
		// publish it.
		return true
	}
	chain := make([]string, 0, len(ancestors)+1)
	for _, a := range ancestors {
		chain = append(chain, a.ID)
	}
	chain = append(chain, page.ID)
	restricted, err := s.d.Repos.Access.Restricted(ctx, tenantID, chain)
	if err != nil {
		return true
	}
	return len(restricted) > 0
}

// publicTitles is every page of the space a visitor may see, so an internal
// page link renders as a link only when its destination is public too.
//
// Built from the space's own pages rather than by walking the tree, because
// a public space publishes all of them; the restricted ones are removed.
func (s *PageService) publicTitles(ctx context.Context, space *model.Space) (map[string]string, error) {
	out := map[string]string{}
	after := ""
	for len(out) < MaxPublicSpacePages {
		pages, err := s.d.Repos.Pages.ListSpaceSummaries(ctx, space.TenantID, space.ID,
			publicTitlePage, after)
		if err != nil {
			return nil, err
		}
		if len(pages) == 0 {
			break
		}
		ids := make([]string, 0, len(pages))
		for _, p := range pages {
			ids = append(ids, p.ID)
		}
		restricted, err := s.d.Repos.Access.Restricted(ctx, space.TenantID, ids)
		if err != nil {
			return nil, err
		}
		for _, p := range pages {
			if !restricted[p.ID] {
				out[p.ID] = p.Title
			}
		}
		if len(pages) < publicTitlePage {
			break
		}
		after = pages[len(pages)-1].ID
	}
	return out, nil
}

// renderForVisitor renders a page for somebody with no session, through a
// share link or a public space; both get the same treatment of page links,
// embeds and attachments.
//
// Page links are the subtle part. A published document may link to pages that
// are not published, and resolving those titles would leak them — "there is a
// page called Q3 Redundancies" is the leak, not its contents. So titles
// resolves only the pages the visitor may reach, and every other link renders
// as plain text with no destination.
//
// Attachments are addressed through attachmentURL, a route of the same
// anonymous surface: the member route behind a normal page needs a session,
// which a visitor's <img> does not carry.
func (s *PageService) renderForVisitor(page *model.Page, titles map[string]string,
	attachmentURL func(attachmentID string) string,
) (string, error) {
	content := page.Content
	if len(content) == 0 {
		content = EmptyDocument
	}
	node, _, err := schema.Default().Validate(content)
	if err != nil {
		return "", fmt.Errorf("docs: stored content of %s is invalid: %w", page.ID, err)
	}
	return render.HTML(node, render.Options{
		PageTitle: func(id string) (string, bool) {
			title, ok := titles[id]
			return title, ok
		},
		EmbedURL:      s.embedURL,
		AttachmentURL: attachmentURL,
	}), nil
}
