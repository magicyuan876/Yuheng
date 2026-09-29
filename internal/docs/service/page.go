package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/fracindex"
	"github.com/magicyuan876/yuheng/internal/docs/markdown"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// PageService owns the page tree: creation, ordering, moves, duplication and
// the trash. Bodies are only touched here at creation and duplication; live
// editing goes through the collaboration path.
//
// Ordering uses fractional index keys (internal/docs/fracindex), so a move
// rewrites one row. Sibling appends are serialised per space inside the
// write transaction and jittered, so two users creating pages under the same
// parent at the same moment never end up with equal keys; client-supplied
// keys are accepted for API users who keep their own tree model.
type PageService struct {
	*base
	// files stores the attachments an imported bundle carries. May be nil in
	// a trimmed build, which importAssets reports rather than crashes on.
	files *AttachmentService
}

// Limits on page metadata.
const (
	MaxTitleRunes = 500
	MaxCoverBytes = 1024
	// MaxContentBytes bounds a body supplied at creation.
	MaxContentBytes = 4 << 20
	// MaxDuplicateNodes bounds how many pages one duplicate call may copy.
	MaxDuplicateNodes = 2000
	// RebalanceKeyLen: a computed position longer than this triggers a
	// renumbering of the whole sibling list with short keys.
	RebalanceKeyLen = 40
	// DefaultTreePage is the page size of tree listings when none is given.
	DefaultTreePage = 500
	// ShortIDLen is the length of the URL-stable page identifier.
	ShortIDLen = 10
)

const shortIDAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// EmptyDocument is the body of a page created without content.
var EmptyDocument = model.JSON(`{"type":"doc","content":[{"type":"paragraph"}]}`)

// newShortID returns a random URL identifier; the unique index on
// (tenant_id, short_id) catches the astronomically rare collision and the
// caller retries.
func newShortID() (string, error) {
	var buf [ShortIDLen]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("docs: short id: %w", err)
	}
	for i := range buf {
		buf[i] = shortIDAlphabet[int(buf[i])%len(shortIDAlphabet)]
	}
	return string(buf[:]), nil
}

// ---- views ----------------------------------------------------------------------

// PageView is a page as the API returns it: the row plus what the caller may
// do with it.
type PageView struct {
	*model.Page
	// Role is the caller's effective role on this page.
	Role model.SpaceRole `json:"role"`
	// CanEdit folds role and lock state into one flag for clients.
	CanEdit     bool `json:"can_edit"`
	HasChildren bool `json:"has_children"`
	// Restricted is true when this page cuts permission inheritance.
	Restricted bool `json:"restricted"`
	// Labels is what the page is filed under. Carried on the page read
	// rather than fetched separately: the chips are drawn with the title,
	// and a second round trip would make them appear late.
	Labels []*LabelView `json:"labels"`
	// Favourite is whether this caller starred the page.
	Favourite bool `json:"favourite"`
}

// TreeNode is one row of a tree listing.
type TreeNode struct {
	*model.Page
	HasChildren bool `json:"has_children"`
	CanEdit     bool `json:"can_edit"`
	Restricted  bool `json:"restricted"`
}

// TreePage is one page of children.
type TreePage struct {
	Items []*TreeNode `json:"items"`
	// NextCursor is empty when the listing is complete.
	NextCursor string `json:"next_cursor,omitempty"`
}

// TrashEntry is a trash root with who deleted it.
type TrashEntry struct {
	*model.Page
	DeletedByUser *UserView `json:"deleted_by_user,omitempty"`
	CanRestore    bool      `json:"can_restore"`
}

// PageContent is the body of a page.
type PageContent struct {
	PageID      string     `json:"page_id"`
	YDocVersion int64      `json:"ydoc_version"`
	Content     model.JSON `json:"content"`
	// HTML is filled when the caller asks for a rendered projection.
	HTML string `json:"html,omitempty"`
}

// ---- inputs -----------------------------------------------------------------------

// CreatePageInput is what a new page needs. The page is appended after its
// siblings.
type CreatePageInput struct {
	SpaceID  string
	ParentID *string
	Title    string
	Icon     *string
	// Content is a ProseMirror JSON document; Markdown is converted with the
	// module's converter. At most one may be set; neither yields an empty page.
	Content  json.RawMessage
	Markdown string
	// TemplateID starts the page from a saved body. Mutually exclusive with
	// Content and Markdown; the template must be one this caller may use in
	// this space.
	TemplateID string
}

// UpdatePageInput is a partial metadata update; nil leaves a field alone and
// an empty string clears icon/cover.
type UpdatePageInput struct {
	Title *string
	Icon  *string
	Cover *string
}

// PlacementMode says where among its siblings a page lands.
type PlacementMode int

// Placement modes.
const (
	// PlaceEnd appends after the last sibling.
	PlaceEnd PlacementMode = iota
	// PlaceFirst puts the page before the first sibling.
	PlaceFirst
	// PlaceAfter puts the page right after AfterID.
	PlaceAfter
	// PlaceAt uses the client-supplied Position verbatim.
	PlaceAt
)

// Placement is the target slot of a move.
type Placement struct {
	Mode     PlacementMode
	AfterID  string
	Position string
}

// MovePageInput describes a move. ParentID nil means the space root; an
// empty SpaceID keeps the page in its space.
type MovePageInput struct {
	ParentID  *string
	SpaceID   string
	Placement Placement
}

// MoveResult reports what a move did.
type MoveResult struct {
	Page *PageView `json:"page"`
	// Orphaned lists subtree pages the mover could not see; on a cross-space
	// move they stay behind at the source space root.
	Orphaned []string `json:"orphaned,omitempty"`
	// Rebalanced is true when the sibling list was renumbered; clients
	// should reload that parent's children.
	Rebalanced bool `json:"rebalanced"`
}

// DuplicatePageInput copies a page and its visible subtree.
type DuplicatePageInput struct {
	// SpaceID targets another space; empty duplicates next to the original.
	SpaceID string
	// ParentID places the copy under a page of the target space (other
	// space only); nil puts it at the root.
	ParentID *string
	// Title overrides the root copy's title.
	Title string
}

// DuplicateResult is the new root and the ids of every copied page.
type DuplicateResult struct {
	Page     *PageView `json:"page"`
	ChildIDs []string  `json:"child_ids"`
	Count    int       `json:"count"`
}

// ---- helpers -------------------------------------------------------------------------

func cleanTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if utf8.RuneCountInString(title) > MaxTitleRunes {
		return "", invalid("title must be at most %d characters", MaxTitleRunes)
	}
	if strings.ContainsAny(title, "\n\r") {
		return "", invalid("title must be a single line")
	}
	return title, nil
}

func cleanCover(cover *string) (*string, error) {
	if cover == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*cover)
	if v == "" {
		return nil, nil
	}
	if len(v) > MaxCoverBytes {
		return nil, invalid("cover must be at most %d bytes", MaxCoverBytes)
	}
	return &v, nil
}

// body holds a validated document and its derived columns.
type body struct {
	content model.JSON
	text    string
	words   int
}

// parseBody validates the supplied content (or converts Markdown) and
// derives the search text and word count.
func parseBody(in CreatePageInput) (*body, error) {
	if len(in.Content) > 0 && strings.TrimSpace(in.Markdown) != "" {
		return nil, invalid("content and markdown are mutually exclusive")
	}
	var node *schema.Node
	switch {
	case len(in.Content) > 0:
		if len(in.Content) > MaxContentBytes {
			return nil, invalid("content must be at most %d bytes", MaxContentBytes)
		}
		n, _, err := schema.Default().Validate(in.Content)
		if err != nil {
			return nil, invalid("content: %v", err)
		}
		node = n
	case strings.TrimSpace(in.Markdown) != "":
		if len(in.Markdown) > MaxContentBytes {
			return nil, invalid("markdown must be at most %d bytes", MaxContentBytes)
		}
		n, _, err := markdown.ToDocument([]byte(in.Markdown), markdown.Options{})
		if err != nil {
			return nil, invalid("markdown: %v", err)
		}
		node = n
	default:
		return &body{content: EmptyDocument}, nil
	}
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}
	st := render.Extract(node)
	return &body{content: model.JSON(raw), text: render.Text(node), words: st.WordCount}, nil
}

func canEdit(role model.SpaceRole, page *model.Page) bool {
	if !role.AtLeast(model.RoleWriter) {
		return false
	}
	return !page.IsLocked || role == model.RoleAdmin
}

// requireRole turns an insufficient decision into the right API error:
// invisible pages are 404, visible ones 403.
func requireRole(d acl.Decision, min model.SpaceRole) error {
	if d.Role == model.RoleNone {
		return notFound("page")
	}
	if !d.Role.AtLeast(min) {
		return forbidden("this action needs the %s role on the page", min)
	}
	return nil
}

// requireSpaceRole is requireRole for something addressed by its space rather
// than by a page: a label, a space setting. Same shape, same distinction
// between "you cannot see this" and "you cannot do this".
func requireSpaceRole(role, min model.SpaceRole) error {
	if role == model.RoleNone {
		return notFound("space")
	}
	if !role.AtLeast(min) {
		return forbidden("this action needs the %s role on the space", min)
	}
	return nil
}

func (s *PageService) view(ctx context.Context, d acl.Decision) (*PageView, error) {
	counts, err := s.d.Repos.Pages.ChildCounts(ctx, d.Page.TenantID, []string{d.Page.ID})
	if err != nil {
		return nil, err
	}
	restricted := false
	for _, id := range d.RestrictedAt {
		if id == d.Page.ID {
			restricted = true
		}
	}
	return &PageView{
		Page: d.Page, Role: d.Role, CanEdit: canEdit(d.Role, d.Page),
		HasChildren: counts[d.Page.ID] > 0, Restricted: restricted,
	}, nil
}

// resolve re-reads a page and the caller's decision on it (after a write).
func (s *PageService) resolve(ctx context.Context, actor *acl.Identity, pageID string) (*PageView, error) {
	page, err := s.d.Repos.Pages.Get(ctx, actor.TenantID, pageID)
	if err != nil {
		return nil, err
	}
	d, err := s.d.Resolver.Decide(ctx, actor, page)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, d)
}

// ---- create ------------------------------------------------------------------------------

// Create adds a page at the end of its siblings. Root pages need the writer
// role in the space; child pages need it on the parent.
func (s *PageService) Create(ctx context.Context, actor *acl.Identity, in CreatePageInput) (*PageView, error) {
	title, err := cleanTitle(in.Title)
	if err != nil {
		return nil, err
	}
	icon, err := cleanIcon(in.Icon)
	if err != nil {
		return nil, err
	}
	// A template is resolved before the body is parsed, because it becomes
	// the body: everything after this point treats a templated page exactly
	// like one created with content, which is the point.
	if in.TemplateID != "" {
		if len(in.Content) > 0 || strings.TrimSpace(in.Markdown) != "" {
			return nil, invalid("a template and a body are mutually exclusive")
		}
		tpl, err := s.templateFor(ctx, actor, in.TemplateID, in.SpaceID)
		if err != nil {
			return nil, err
		}
		in.Content = json.RawMessage(tpl.Content)
	}
	b, err := parseBody(in)
	if err != nil {
		return nil, err
	}
	space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, in.SpaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("space")
		}
		return nil, err
	}
	role, err := s.d.Resolver.SpaceRole(ctx, actor, space)
	if err != nil {
		return nil, err
	}
	if role == model.RoleNone {
		return nil, notFound("space")
	}
	if in.ParentID != nil {
		parent, err := s.d.Resolver.Page(ctx, actor, *in.ParentID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("parent page")
			}
			return nil, err
		}
		if err := requireRole(parent, model.RoleWriter); err != nil {
			return nil, err
		}
		if parent.Page.SpaceID != space.ID {
			return nil, invalid("parent page belongs to another space")
		}
		if !canEdit(parent.Role, parent.Page) {
			return nil, forbidden("the parent page is locked")
		}
	} else if !role.AtLeast(model.RoleWriter) {
		return nil, forbidden("creating pages at the space root needs the writer role")
	}

	uid := actor.UserID
	page := &model.Page{
		TenantID: actor.TenantID, SpaceID: space.ID, ParentID: in.ParentID, Title: title, Icon: icon,
		Content: b.content, TextContent: b.text, WordCount: b.words,
		CreatorID: &uid, LastEditorID: &uid, ContributorIDs: model.StringList{uid},
		SourceRefs: model.StringList{},
	}
	// Recorded so that "which pages came from this template" is answerable,
	// and so a template's author can see whether anybody uses it.
	if in.TemplateID != "" {
		page.TemplateID = &in.TemplateID
	}
	for attempt := 0; attempt < 5; attempt++ {
		page.ShortID, err = newShortID()
		if err != nil {
			return nil, err
		}
		page.ID = ""
		err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
			if err := tx.Pages.LockSpace(ctx, actor.TenantID, space.ID); err != nil {
				return err
			}
			last, _, err := tx.Pages.LastPosition(ctx, actor.TenantID, space.ID, in.ParentID)
			if err != nil {
				return err
			}
			page.Position, err = fracindex.Jittered(last, "")
			if err != nil {
				return err
			}
			return tx.Pages.Create(ctx, page)
		})
		if err == nil {
			break
		}
		if !errors.Is(err, repository.ErrDuplicate) {
			return nil, err
		}
	}
	if err != nil {
		return nil, conflict("could not allocate a page identifier; retry")
	}

	s.publish(ctx, events.New(events.PageCreated, actor.TenantID).WithSpace(space.ID).WithPage(page.ID).
		WithActor(actor.UserID).With("parent_id", in.ParentID).With("position", page.Position).
		With("title", page.Title).With("icon", page.Icon).With("short_id", page.ShortID))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PageCreated, SpaceID: space.ID, TargetType: audit.TargetPage, TargetID: page.ID,
	})
	// Creating a page makes you a watcher of it; see internal/docs/notify.
	s.autoWatch(ctx, page, actor.UserID, "create")
	return s.resolve(ctx, actor, page.ID)
}

// ---- read ----------------------------------------------------------------------------------

// SpaceRoleOf resolves the caller's role in a space (for handlers that hold
// a page decision but need the space-level role for its children).
func (s *PageService) SpaceRoleOf(ctx context.Context, actor *acl.Identity, space *model.Space) (model.SpaceRole,
	error,
) {
	return s.d.Resolver.SpaceRole(ctx, actor, space)
}

// Get returns the page the guard resolved.
func (s *PageService) Get(ctx context.Context, actor *acl.Identity, d acl.Decision) (*PageView, error) {
	view, err := s.view(ctx, d)
	if err != nil {
		return nil, err
	}
	// Only on the single-page read. A tree listing of a hundred rows does not
	// want a hundred label lookups for chips it does not draw.
	labels, err := s.PageLabels(ctx, actor, d)
	if err != nil {
		return nil, err
	}
	view.Labels = labels
	view.Favourite = s.IsFavourite(ctx, actor, d.Page.ID)
	return view, nil
}

// Content returns the body; withHTML adds the rendered projection.
func (s *PageService) Content(ctx context.Context, d acl.Decision, withHTML bool) (*PageContent, error) {
	out := &PageContent{PageID: d.Page.ID, YDocVersion: d.Page.YDocVersion, Content: d.Page.Content}
	if len(out.Content) == 0 {
		out.Content = EmptyDocument
	}
	if withHTML {
		node, _, err := schema.Default().Validate(out.Content)
		if err != nil {
			return nil, fmt.Errorf("docs: stored content of %s is invalid: %w", d.Page.ID, err)
		}
		titles := map[string]string{}
		st := render.Extract(node)
		if len(st.PageLinks) > 0 {
			pages, err := s.d.Repos.Pages.GetSummaries(ctx, d.Page.TenantID, st.PageLinks)
			if err != nil {
				return nil, err
			}
			for _, p := range pages {
				titles[p.ID] = p.Title
			}
		}
		out.HTML = render.HTML(node, render.Options{
			PageTitle: func(id string) (string, bool) { t, ok := titles[id]; return t, ok },
		})
	}
	return out, nil
}

// Ancestors returns the breadcrumb chain, root first, excluding the page.
// A caller who can read a page can read every ancestor (restrictions only
// narrow), so no filtering is needed.
func (s *PageService) Ancestors(ctx context.Context, d acl.Decision) ([]*model.Page, error) {
	return s.d.Repos.Pages.ListAncestors(ctx, d.Page.TenantID, d.Page.ID)
}

// Children lists one page of the live children of parentID (nil = roots)
// the caller may see. spaceRole is the caller's role in the space.
func (s *PageService) Children(ctx context.Context, actor *acl.Identity, space *model.Space,
	spaceRole model.SpaceRole, parentID *string, cursor string, limit int,
) (*TreePage, error) {
	if parentID != nil {
		parent, err := s.d.Resolver.Page(ctx, actor, *parentID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("parent page")
			}
			return nil, err
		}
		if parent.Role == model.RoleNone || parent.Page.SpaceID != space.ID {
			return nil, notFound("parent page")
		}
	}
	if limit <= 0 {
		limit = DefaultTreePage
	}
	if limit > repository.MaxTreePage {
		limit = repository.MaxTreePage
	}
	var after *repository.TreeCursor
	if cursor != "" {
		pos, id, ok := strings.Cut(cursor, ":")
		if !ok || fracindex.Validate(pos) != nil || id == "" {
			return nil, invalid("cursor is malformed")
		}
		after = &repository.TreeCursor{Position: pos, ID: id}
	}
	rows, err := s.d.Repos.Pages.ListChildrenAfter(ctx, actor.TenantID, space.ID, parentID, after, limit)
	if err != nil {
		return nil, err
	}
	out := &TreePage{Items: make([]*TreeNode, 0, len(rows))}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	counts, err := s.d.Repos.Pages.ChildCounts(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		role := spaceRole
		if restricted[r.ID] {
			// Only restricted rows need the full resolution; the rest inherit
			// the (already resolved) parent chain.
			d, err := s.d.Resolver.Page(ctx, actor, r.ID)
			if err != nil {
				return nil, err
			}
			if d.Role == model.RoleNone {
				continue
			}
			role = d.Role
		}
		out.Items = append(out.Items, &TreeNode{
			Page: r, HasChildren: counts[r.ID] > 0, CanEdit: canEdit(role, r), Restricted: restricted[r.ID],
		})
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		out.NextCursor = last.Position + ":" + last.ID
	}
	return out, nil
}

// ---- update -------------------------------------------------------------------------------

// Update changes title, icon or cover.
func (s *PageService) Update(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in UpdatePageInput,
) (*PageView, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	fields := map[string]any{}
	payload := events.New(events.PageMeta, actor.TenantID).WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).
		WithActor(actor.UserID)
	if in.Title != nil {
		title, err := cleanTitle(*in.Title)
		if err != nil {
			return nil, err
		}
		fields["title"] = title
		payload = payload.With("title", title)
	}
	if in.Icon != nil {
		icon, err := cleanIcon(in.Icon)
		if err != nil {
			return nil, err
		}
		fields["icon"] = icon
		payload = payload.With("icon", icon)
	}
	if in.Cover != nil {
		cover, err := cleanCover(in.Cover)
		if err != nil {
			return nil, err
		}
		fields["cover"] = cover
		payload = payload.With("cover", cover)
	}
	if len(fields) == 0 {
		return s.view(ctx, d)
	}
	if err := s.d.Repos.Pages.UpdateMeta(ctx, actor.TenantID, d.Page.ID, fields); err != nil {
		return nil, err
	}
	s.publish(ctx, payload)
	return s.resolve(ctx, actor, d.Page.ID)
}

// ---- move ----------------------------------------------------------------------------------

// Move re-parents and/or re-orders a page. Cross-space moves carry the
// subtree, except pages the mover cannot see: those stay behind at the
// source space root so a move never widens who can read them.
func (s *PageService) Move(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in MovePageInput,
) (*MoveResult, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	page := d.Page
	targetSpaceID := page.SpaceID
	if in.SpaceID != "" && in.SpaceID != page.SpaceID {
		targetSpaceID = in.SpaceID
	}
	crossSpace := targetSpaceID != page.SpaceID
	if crossSpace {
		target, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, targetSpaceID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("target space")
			}
			return nil, err
		}
		role, err := s.d.Resolver.SpaceRole(ctx, actor, target)
		if err != nil {
			return nil, err
		}
		if role == model.RoleNone {
			return nil, notFound("target space")
		}
		if !role.AtLeast(model.RoleWriter) {
			return nil, forbidden("moving into this space needs the writer role there")
		}
	}
	if in.ParentID != nil {
		if *in.ParentID == page.ID {
			return nil, invalid("a page cannot be its own parent")
		}
		parent, err := s.d.Resolver.Page(ctx, actor, *in.ParentID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("target parent")
			}
			return nil, err
		}
		if err := requireRole(parent, model.RoleWriter); err != nil {
			return nil, err
		}
		if parent.Page.SpaceID != targetSpaceID {
			return nil, invalid("target parent belongs to another space")
		}
		if !canEdit(parent.Role, parent.Page) {
			return nil, forbidden("the target parent is locked")
		}
	}
	if in.Placement.Mode == PlaceAfter && in.Placement.AfterID == page.ID {
		return nil, invalid("a page cannot be placed after itself")
	}
	if in.Placement.Mode == PlaceAt {
		if err := fracindex.Validate(in.Placement.Position); err != nil {
			return nil, invalid("position: %v", err)
		}
	}

	// Permission checks read outside the write transaction (the resolver has
	// its own connection); the transaction only applies what was decided.
	var orphaned []string
	if crossSpace {
		var err error
		orphaned, err = s.invisibleInSubtree(ctx, actor, page)
		if err != nil {
			return nil, err
		}
	}
	result := &MoveResult{}
	oldParent := page.ParentID
	err := s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		// Lock both spaces in a fixed order so two opposite moves cannot
		// deadlock.
		locks := []string{page.SpaceID}
		if crossSpace {
			locks = append(locks, targetSpaceID)
			sort.Strings(locks)
		}
		for _, sid := range locks {
			if err := tx.Pages.LockSpace(ctx, actor.TenantID, sid); err != nil {
				return err
			}
		}
		pos, err := s.slot(ctx, tx, actor.TenantID, targetSpaceID, in.ParentID, page, in.Placement)
		if err != nil {
			return err
		}
		if len(pos) > RebalanceKeyLen {
			pos, err = s.rebalance(ctx, tx, actor.TenantID, targetSpaceID, in.ParentID, page.ID, pos)
			if err != nil {
				return err
			}
			result.Rebalanced = true
		}
		if err := s.detach(ctx, tx, actor.TenantID, page.SpaceID, orphaned); err != nil {
			return err
		}
		return tx.Pages.Move(ctx, actor.TenantID, page.ID, repository.MoveTarget{
			SpaceID: targetSpaceID, ParentID: in.ParentID, Position: pos,
		})
	})
	if err != nil {
		if errors.Is(err, repository.ErrInvalidMove) {
			return nil, invalid("%v", err)
		}
		return nil, err
	}
	s.invalidate(ctx, actor.TenantID)

	view, err := s.resolve(ctx, actor, page.ID)
	if err != nil {
		return nil, err
	}
	result.Page = view
	result.Orphaned = orphaned

	ev := events.New(events.PageMoved, actor.TenantID).WithSpace(targetSpaceID).WithPage(page.ID).
		WithActor(actor.UserID).With("from_space_id", page.SpaceID).With("old_parent_id", oldParent).
		With("parent_id", in.ParentID).With("position", view.Position).With("cross_space", crossSpace).
		With("rebalanced", result.Rebalanced)
	if crossSpace {
		ev = ev.With("orphaned", orphaned)
	}
	s.publish(ctx, ev)
	if crossSpace {
		// The source space's tree changed too (the page left, orphans arrived).
		s.publish(ctx, events.New(events.PageMoved, actor.TenantID).WithSpace(page.SpaceID).WithPage(page.ID).
			WithActor(actor.UserID).With("from_space_id", page.SpaceID).With("to_space_id", targetSpaceID).
			With("old_parent_id", oldParent).With("cross_space", true).With("orphaned", orphaned).With("left", true))
	}
	if result.Rebalanced {
		s.publish(ctx, events.New(events.TreeRebalanced, actor.TenantID).WithSpace(targetSpaceID).
			WithActor(actor.UserID).With("parent_id", in.ParentID))
	}
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PageMoved, SpaceID: page.SpaceID, TargetType: audit.TargetPage, TargetID: page.ID,
	})
	return result, nil
}

// slot computes the position for a page placed among the children of
// parentID. The page itself may already be one of those siblings.
func (s *PageService) slot(ctx context.Context, tx *repository.Repositories, tenantID uint64, spaceID string,
	parentID *string, page *model.Page, p Placement,
) (string, error) {
	switch p.Mode {
	case PlaceAt:
		return p.Position, nil
	case PlaceFirst:
		first, err := tx.Pages.ListChildrenAfter(ctx, tenantID, spaceID, parentID, nil, 2)
		if err != nil {
			return "", err
		}
		upper := ""
		for _, f := range first {
			if f.ID != page.ID {
				upper = f.Position
				break
			}
		}
		return fracindex.Jittered("", upper)
	case PlaceAfter:
		after, err := tx.Pages.Get(ctx, tenantID, p.AfterID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return "", invalid("after_id names no live page")
			}
			return "", err
		}
		if after.SpaceID != spaceID || !sameParent(after.ParentID, parentID) {
			return "", invalid("after_id is not a sibling at the target location")
		}
		next, ok, err := tx.Pages.NextPosition(ctx, tenantID, spaceID, parentID, after.Position, after.ID)
		if err != nil {
			return "", err
		}
		if ok && next == page.Position && sameParent(page.ParentID, parentID) && page.SpaceID == spaceID {
			// The page already follows after_id: keep its key.
			return page.Position, nil
		}
		upper := ""
		if ok {
			upper = next
		}
		return fracindex.Jittered(after.Position, upper)
	default:
		last, ok, err := tx.Pages.LastPosition(ctx, tenantID, spaceID, parentID)
		if err != nil {
			return "", err
		}
		if ok && last == page.Position && sameParent(page.ParentID, parentID) && page.SpaceID == spaceID {
			return page.Position, nil
		}
		return fracindex.Jittered(last, "")
	}
}

func sameParent(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// rebalance renumbers every sibling with short keys, slotting the moved page
// where its long key would have sorted, and returns the page's new key.
func (s *PageService) rebalance(ctx context.Context, tx *repository.Repositories, tenantID uint64, spaceID string,
	parentID *string, pageID, pos string,
) (string, error) {
	var order []string
	var cursor *repository.TreeCursor
	inserted := false
	for {
		batch, err := tx.Pages.ListChildrenAfter(ctx, tenantID, spaceID, parentID, cursor, repository.MaxTreePage)
		if err != nil {
			return "", err
		}
		for _, p := range batch {
			if p.ID == pageID {
				continue
			}
			if !inserted && pos < p.Position {
				order = append(order, pageID)
				inserted = true
			}
			order = append(order, p.ID)
		}
		if len(batch) < repository.MaxTreePage {
			break
		}
		last := batch[len(batch)-1]
		cursor = &repository.TreeCursor{Position: last.Position, ID: last.ID}
	}
	if !inserted {
		order = append(order, pageID)
	}
	keys, err := fracindex.NBetween("", "", len(order))
	if err != nil {
		return "", err
	}
	positions := make(map[string]string, len(order))
	newPos := ""
	for i, id := range order {
		if id == pageID {
			newPos = keys[i]
			continue // the moved page's row is written by Move itself
		}
		positions[id] = keys[i]
	}
	if err := tx.Pages.SetPositions(ctx, tenantID, positions); err != nil {
		return "", err
	}
	return newPos, nil
}

// invisibleInSubtree finds, inside the subtree about to leave the space, the
// topmost pages the mover cannot read. Only restricted pages can be
// invisible to someone who can write the subtree root, so only those are
// resolved.
func (s *PageService) invisibleInSubtree(ctx context.Context, actor *acl.Identity,
	root *model.Page,
) ([]string, error) {
	repos := s.d.Repos
	restricted, err := repos.Access.RestrictedInSpace(ctx, actor.TenantID, root.SpaceID)
	if err != nil {
		return nil, err
	}
	if len(restricted) == 0 {
		return nil, nil
	}
	subtree, err := repos.Pages.SubtreeIDs(ctx, actor.TenantID, root.ID)
	if err != nil {
		return nil, err
	}
	inSubtree := make(map[string]bool, len(subtree))
	for _, id := range subtree {
		inSubtree[id] = true
	}
	invisible := map[string]bool{}
	for _, rid := range restricted {
		if rid == root.ID || !inSubtree[rid] {
			continue
		}
		page, err := repos.Pages.Get(ctx, actor.TenantID, rid)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue // trashed: it stays where it is
			}
			return nil, err
		}
		d, err := s.d.Resolver.Decide(ctx, actor, page)
		if err != nil {
			return nil, err
		}
		if d.Role == model.RoleNone {
			invisible[rid] = true
		}
	}
	if len(invisible) == 0 {
		return nil, nil
	}
	// Keep only the topmost invisible pages: a nested one travels with its
	// invisible ancestor anyway.
	var orphans []string
	for id := range invisible {
		anc, err := repos.Pages.ListAncestors(ctx, actor.TenantID, id)
		if err != nil {
			return nil, err
		}
		nested := false
		for _, a := range anc {
			if invisible[a.ID] {
				nested = true
				break
			}
		}
		if !nested {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}

// detach re-attaches the given pages (with their subtrees) at the end of the
// source space root, so they stay behind when their ancestor leaves.
func (s *PageService) detach(ctx context.Context, tx *repository.Repositories, tenantID uint64, spaceID string,
	ids []string,
) error {
	for _, id := range ids {
		last, _, err := tx.Pages.LastPosition(ctx, tenantID, spaceID, nil)
		if err != nil {
			return err
		}
		pos, err := fracindex.Jittered(last, "")
		if err != nil {
			return err
		}
		if err := tx.Pages.Move(ctx, tenantID, id, repository.MoveTarget{
			SpaceID: spaceID, ParentID: nil, Position: pos,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ---- trash -----------------------------------------------------------------------------------

// Delete moves a page and its live descendants to the trash.
func (s *PageService) Delete(ctx context.Context, actor *acl.Identity, d acl.Decision) (int64, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return 0, err
	}
	if !canEdit(d.Role, d.Page) {
		return 0, forbidden("the page is locked")
	}
	n, err := s.d.Repos.Pages.SoftDeleteSubtree(ctx, actor.TenantID, d.Page.ID, actor.UserID)
	if err != nil {
		return 0, err
	}
	s.invalidate(ctx, actor.TenantID)
	// Anyone still editing the page (or one of its descendants) must be
	// disconnected; their client keeps its copy but can no longer save.
	ids, err := s.d.Repos.Pages.SubtreeIDs(ctx, actor.TenantID, d.Page.ID)
	if err != nil {
		ids = []string{d.Page.ID}
	}
	for _, id := range ids {
		s.evict(ctx, id)
	}
	// And any public link to the subtree stops working. A trashed page
	// already fails the share layer's own checks, so this is not what makes
	// it safe; it is what keeps the link list honest and what stops a later
	// restore from silently republishing something.
	s.RevokeSharesForPages(ctx, actor.TenantID, ids)
	s.publish(ctx, events.New(events.PageDeleted, actor.TenantID).WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).
		WithActor(actor.UserID).With("parent_id", d.Page.ParentID).With("count", n))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PageDeleted, SpaceID: d.Page.SpaceID, TargetType: audit.TargetPage, TargetID: d.Page.ID,
	})
	return n, nil
}

// Restore brings a trashed page (and what was trashed with it) back. If its
// parent is still in the trash it is re-attached at the end of the space
// root.
func (s *PageService) Restore(ctx context.Context, actor *acl.Identity, pageID string) (*PageView, error) {
	page, err := s.d.Repos.Pages.GetAny(ctx, actor.TenantID, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("page")
		}
		return nil, err
	}
	d, err := s.d.Resolver.Decide(ctx, actor, page)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("page")
		}
		return nil, err
	}
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if page.DeletedAt == nil {
		return nil, conflict("page is not in the trash")
	}
	var n int64
	reattached := false
	err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		if err := tx.Pages.LockSpace(ctx, actor.TenantID, page.SpaceID); err != nil {
			return err
		}
		n, err = tx.Pages.RestoreSubtree(ctx, actor.TenantID, page.ID)
		if err != nil {
			return err
		}
		restored, err := tx.Pages.Get(ctx, actor.TenantID, page.ID)
		if err != nil {
			return err
		}
		if !sameParent(restored.ParentID, page.ParentID) {
			// Re-attached at the root: give it a fresh slot at the end.
			reattached = true
			last, _, err := tx.Pages.LastPosition(ctx, actor.TenantID, page.SpaceID, nil)
			if err != nil {
				return err
			}
			pos, err := fracindex.Jittered(last, "")
			if err != nil {
				return err
			}
			return tx.Pages.UpdateMeta(ctx, actor.TenantID, page.ID, map[string]any{"position": pos})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, actor.TenantID)
	view, err := s.resolve(ctx, actor, page.ID)
	if err != nil {
		return nil, err
	}
	s.publish(ctx, events.New(events.PageRestored, actor.TenantID).WithSpace(page.SpaceID).WithPage(page.ID).
		WithActor(actor.UserID).With("parent_id", view.ParentID).With("position", view.Position).
		With("title", view.Title).With("icon", view.Icon).With("short_id", view.ShortID).With("count", n).
		With("reattached", reattached))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PageRestored, SpaceID: page.SpaceID, TargetType: audit.TargetPage, TargetID: page.ID,
	})
	return view, nil
}

// Trash lists the trash roots of a space the caller may see, newest first.
func (s *PageService) Trash(ctx context.Context, actor *acl.Identity, space *model.Space) ([]*TrashEntry, error) {
	rows, err := s.d.Repos.Pages.ListTrash(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	restricted, err := s.d.Repos.Access.RestrictedInSpace(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	spaceRole, err := s.d.Resolver.SpaceRole(ctx, actor, space)
	if err != nil {
		return nil, err
	}
	out := make([]*TrashEntry, 0, len(rows))
	var deleters []string
	for _, p := range rows {
		role := spaceRole
		if len(restricted) > 0 {
			// Restrictions anywhere in the space may sit on the trashed page
			// or on one of its live ancestors; resolve each root.
			d, err := s.d.Resolver.Decide(ctx, actor, p)
			if err != nil {
				return nil, err
			}
			role = d.Role
		}
		if role == model.RoleNone {
			continue
		}
		e := &TrashEntry{Page: p, CanRestore: role.AtLeast(model.RoleWriter)}
		if p.DeletedBy != nil {
			deleters = append(deleters, *p.DeletedBy)
		}
		out = append(out, e)
	}
	users := s.users(ctx, dedupe(deleters))
	for _, e := range out {
		if e.DeletedBy != nil {
			v := userView(*e.DeletedBy, users)
			e.DeletedByUser = &v
		}
	}
	return out, nil
}

// Purge permanently deletes one trash root and its subtree. Space admins only
// (enforced by the route); returns the removed ids.
func (s *PageService) Purge(ctx context.Context, actor *acl.Identity, space *model.Space,
	pageID string,
) ([]string, error) {
	page, err := s.d.Repos.Pages.GetAny(ctx, actor.TenantID, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("page")
		}
		return nil, err
	}
	if page.SpaceID != space.ID {
		return nil, notFound("page")
	}
	if page.DeletedAt == nil {
		return nil, conflict("page is not in the trash")
	}
	return s.purgeSubtree(ctx, actor.TenantID, space.ID, pageID, actor.UserID, actorRole(actor))
}

// purgeSubtree is the one path by which pages are permanently removed.
//
// Shared by the manual purge above and by the retention sweep in
// maintenance.go. It exists as one function rather than two similar ones
// because everything it releases — attachments, block snapshots, history,
// comments, watchers, notifications, share links — is a thing that would be
// left behind by whichever copy somebody forgot to update, and debris from a
// purge is invisible until a quota or a stale link makes it somebody's
// problem months later.
func (s *PageService) purgeSubtree(ctx context.Context, tenantID uint64, spaceID, pageID,
	actorUserID, actorRoleName string,
) ([]string, error) {
	// Collected before the purge: the foreign key nulls page_id as soon as the
	// page rows are gone, and there would be nothing left to release.
	subtree, err := s.d.Repos.Pages.SubtreeIDs(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	doomed := s.collectPageAttachments(ctx, tenantID, subtree)
	ids, err := s.d.Repos.Pages.PurgeOne(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	s.releaseAttachments(ctx, tenantID, doomed)
	s.invalidate(ctx, tenantID)
	for _, id := range ids {
		s.evict(ctx, id)
		// The foreign keys on these tables cascade from docs_pages, so the
		// purge above has normally removed the rows already; clearing them
		// explicitly keeps the purge complete even for a table that is added
		// without the cascade.
		if s.d.Repos.Blocks != nil {
			if err := s.d.Repos.Blocks.DeleteForPage(ctx, tenantID, id); err != nil {
				logger.Warnf(ctx, "[docs] clearing the block snapshots of page %s failed: %v", id, err)
			}
		}
		if s.d.Repos.History != nil {
			if err := s.d.Repos.History.DeleteForPage(ctx, tenantID, id); err != nil {
				logger.Warnf(ctx, "[docs] clearing the history of page %s failed: %v", id, err)
			}
		}
		if s.d.Repos.Comments != nil {
			if err := s.d.Repos.Comments.DeleteForPage(ctx, tenantID, id); err != nil {
				logger.Warnf(ctx, "[docs] clearing the comments of page %s failed: %v", id, err)
			}
		}
		if s.d.Repos.Watchers != nil {
			if err := s.d.Repos.Watchers.DeleteForPage(ctx, tenantID, id); err != nil {
				logger.Warnf(ctx, "[docs] clearing the watchers of page %s failed: %v", id, err)
			}
		}
		if s.d.Repos.Notices != nil {
			// The notifications go too: there would be nothing left to open.
			if err := s.d.Repos.Notices.DeleteForPage(ctx, tenantID, id); err != nil {
				logger.Warnf(ctx, "[docs] clearing the notifications of page %s failed: %v", id, err)
			}
		}
	}
	// A share link to a purged page already fails the share layer's own
	// checks; revoking keeps the owner's link list from being a list of dead
	// addresses.
	s.RevokeSharesForPages(ctx, tenantID, ids)

	s.publish(ctx, events.New(events.PagePurged, tenantID).WithSpace(spaceID).WithPage(pageID).
		WithActor(actorUserID).With("ids", ids))
	s.audit(ctx, audit.Entry{
		TenantID: tenantID, ActorUserID: actorUserID, ActorRole: actorRoleName,
		Action: audit.PagePurged, SpaceID: spaceID, TargetType: audit.TargetPage, TargetID: pageID,
	})
	return ids, nil
}

// EmptyTrash purges every trash root of the space. Returns how many pages
// (roots and descendants) were removed.
func (s *PageService) EmptyTrash(ctx context.Context, actor *acl.Identity, space *model.Space) (int, error) {
	roots, err := s.d.Repos.Pages.ListTrash(ctx, actor.TenantID, space.ID)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, r := range roots {
		subtree, err := s.d.Repos.Pages.SubtreeIDs(ctx, actor.TenantID, r.ID)
		if err != nil {
			return total, err
		}
		doomed := s.collectPageAttachments(ctx, actor.TenantID, subtree)
		ids, err := s.d.Repos.Pages.PurgeOne(ctx, actor.TenantID, r.ID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue // purged concurrently
			}
			return total, err
		}
		s.releaseAttachments(ctx, actor.TenantID, doomed)
		total += len(ids)
	}
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.PagePurged, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("count", total).With("all", true))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PagePurged, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
	})
	return total, nil
}

// ---- duplicate ---------------------------------------------------------------------------------

// Duplicate copies a page and the part of its subtree the caller can read.
// Copies get fresh identifiers, inherit the destination's permissions, and
// internal links between copied pages are rewritten to the copies. Bodies
// are copied as JSON; the collaboration service materialises the Yjs state
// on first open.
func (s *PageService) Duplicate(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in DuplicatePageInput,
) (*DuplicateResult, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	source := d.Page
	targetSpace := d.Space
	sameSpace := in.SpaceID == "" || in.SpaceID == source.SpaceID
	if !sameSpace {
		sp, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, in.SpaceID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("target space")
			}
			return nil, err
		}
		targetSpace = sp
	}
	role, err := s.d.Resolver.SpaceRole(ctx, actor, targetSpace)
	if err != nil {
		return nil, err
	}
	if role == model.RoleNone {
		return nil, notFound("target space")
	}
	if !role.AtLeast(model.RoleWriter) {
		return nil, forbidden("duplicating into this space needs the writer role")
	}
	parentID := source.ParentID
	if !sameSpace {
		parentID = in.ParentID
		if parentID != nil {
			parent, err := s.d.Resolver.Page(ctx, actor, *parentID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return nil, notFound("target parent")
				}
				return nil, err
			}
			if err := requireRole(parent, model.RoleWriter); err != nil {
				return nil, err
			}
			if parent.Page.SpaceID != targetSpace.ID {
				return nil, invalid("target parent belongs to another space")
			}
		}
	} else if parentID != nil {
		// Copying next to the original needs write access to the parent.
		parent, err := s.d.Resolver.Page(ctx, actor, *parentID)
		if err != nil {
			return nil, err
		}
		if !parent.Role.AtLeast(model.RoleWriter) {
			return nil, forbidden("duplicating here needs the writer role on the parent page")
		}
	}
	title := source.Title
	if strings.TrimSpace(in.Title) != "" {
		title, err = cleanTitle(in.Title)
		if err != nil {
			return nil, err
		}
	} else if sameSpace {
		title = strings.TrimSpace(source.Title + " (copy)")
	}

	// Collect the visible subtree, parents before children.
	ids, err := s.d.Repos.Pages.SubtreeIDs(ctx, actor.TenantID, source.ID)
	if err != nil {
		return nil, err
	}
	if len(ids) > MaxDuplicateNodes {
		return nil, invalid("the subtree has %d pages; at most %d can be duplicated at once", len(ids),
			MaxDuplicateNodes)
	}
	pages, err := s.d.Repos.Pages.GetMany(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	visible, err := s.visibleSubtree(ctx, actor, source, pages)
	if err != nil {
		return nil, err
	}

	uid := actor.UserID
	idMap := make(map[string]string, len(visible))
	for _, p := range visible {
		idMap[p.ID] = repository.NewID()
	}
	copies := make([]*model.Page, 0, len(visible))
	for _, p := range visible {
		shortID, err := newShortID()
		if err != nil {
			return nil, err
		}
		content, err := remapContent(p.Content, idMap)
		if err != nil {
			return nil, fmt.Errorf("docs: content of %s: %w", p.ID, err)
		}
		c := &model.Page{
			ID: idMap[p.ID], ShortID: shortID, TenantID: p.TenantID, SpaceID: targetSpace.ID,
			Position: p.Position, Title: p.Title, Icon: p.Icon, Cover: p.Cover, Content: content,
			TextContent: p.TextContent, ExcludeFromKnowledge: p.ExcludeFromKnowledge, TemplateID: p.TemplateID,
			SourceRefs: append(model.StringList{}, p.SourceRefs...), ContributorIDs: model.StringList{uid},
			CreatorID: &uid, LastEditorID: &uid, WordCount: p.WordCount,
		}
		if p.ID == source.ID {
			c.Title = title
			c.ParentID = parentID
		} else if p.ParentID != nil {
			np := idMap[*p.ParentID]
			c.ParentID = &np
		}
		copies = append(copies, c)
	}

	err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		if err := tx.Pages.LockSpace(ctx, actor.TenantID, targetSpace.ID); err != nil {
			return err
		}
		var pos string
		if sameSpace {
			next, ok, err := tx.Pages.NextPosition(ctx, actor.TenantID, source.SpaceID, source.ParentID,
				source.Position, source.ID)
			if err != nil {
				return err
			}
			upper := ""
			if ok {
				upper = next
			}
			pos, err = fracindex.Jittered(source.Position, upper)
			if err != nil {
				return err
			}
		} else {
			last, _, err := tx.Pages.LastPosition(ctx, actor.TenantID, targetSpace.ID, parentID)
			if err != nil {
				return err
			}
			pos, err = fracindex.Jittered(last, "")
			if err != nil {
				return err
			}
		}
		copies[0].Position = pos
		return tx.Pages.CreateBatch(ctx, copies)
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, conflict("could not allocate page identifiers; retry")
		}
		return nil, err
	}

	newRoot := copies[0]
	childIDs := make([]string, 0, len(copies)-1)
	for _, c := range copies[1:] {
		childIDs = append(childIDs, c.ID)
	}
	s.publish(ctx, events.New(events.PageCreated, actor.TenantID).WithSpace(targetSpace.ID).WithPage(newRoot.ID).
		WithActor(actor.UserID).With("parent_id", newRoot.ParentID).With("position", newRoot.Position).
		With("title", newRoot.Title).With("icon", newRoot.Icon).With("short_id", newRoot.ShortID).
		With("duplicated_from", source.ID).With("count", len(copies)))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.PageDuplicated, SpaceID: targetSpace.ID, TargetType: audit.TargetPage, TargetID: newRoot.ID,
	})
	view, err := s.resolve(ctx, actor, newRoot.ID)
	if err != nil {
		return nil, err
	}
	return &DuplicateResult{Page: view, ChildIDs: childIDs, Count: len(copies)}, nil
}

// visibleSubtree orders the subtree parents-first and drops pages the caller
// cannot read together with everything under them.
func (s *PageService) visibleSubtree(ctx context.Context, actor *acl.Identity, root *model.Page,
	pages []*model.Page,
) ([]*model.Page, error) {
	byID := make(map[string]*model.Page, len(pages))
	children := map[string][]*model.Page{}
	for _, p := range pages {
		byID[p.ID] = p
		if p.ID != root.ID && p.ParentID != nil {
			children[*p.ParentID] = append(children[*p.ParentID], p)
		}
	}
	for _, kids := range children {
		sort.Slice(kids, func(i, j int) bool {
			if kids[i].Position == kids[j].Position {
				return kids[i].ID < kids[j].ID
			}
			return kids[i].Position < kids[j].Position
		})
	}
	ids := make([]string, 0, len(pages))
	for _, p := range pages {
		ids = append(ids, p.ID)
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	rootPage, ok := byID[root.ID]
	if !ok {
		return nil, notFound("page")
	}
	var out []*model.Page
	var walk func(p *model.Page) error
	walk = func(p *model.Page) error {
		if p.ID != root.ID && restricted[p.ID] {
			d, err := s.d.Resolver.Page(ctx, actor, p.ID)
			if err != nil {
				return err
			}
			if d.Role == model.RoleNone {
				return nil // skip the whole branch
			}
		}
		out = append(out, p)
		for _, c := range children[p.ID] {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(rootPage); err != nil {
		return nil, err
	}
	return out, nil
}

// remapContent rewrites references between copied pages so the copies link
// to each other rather than back to the originals.
func remapContent(content model.JSON, idMap map[string]string) (model.JSON, error) {
	if len(content) == 0 {
		return EmptyDocument, nil
	}
	node, err := schema.ParseDocument(content)
	if err != nil {
		return nil, err
	}
	changed := false
	var walk func(n *schema.Node)
	walk = func(n *schema.Node) {
		switch n.Type {
		case schema.NodePageLink:
			if id, ok := n.Attrs["pageId"].(string); ok {
				if nid, ok := idMap[id]; ok {
					n.Attrs["pageId"] = nid
					changed = true
				}
			}
		case schema.NodeTransclusion:
			if id, ok := n.Attrs["sourcePageId"].(string); ok {
				if nid, ok := idMap[id]; ok {
					n.Attrs["sourcePageId"] = nid
					changed = true
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(node)
	if !changed {
		return append(model.JSON(nil), content...), nil
	}
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}
	return model.JSON(raw), nil
}
