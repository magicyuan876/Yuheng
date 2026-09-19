package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Page-level permissions: the panel behind "who can see this page".
//
// The rules themselves live in package acl and were settled long before this
// file; what is here is only the management surface. Three of them govern
// every decision below and are worth restating where somebody will change
// them:
//
//  1. A restriction only ever NARROWS. A grant is a ceiling applied on top of
//     the space role, never a promotion: granting somebody "admin" on a page
//     in a space where they are a reader leaves them a reader. This is what
//     makes page permissions safe to hand to page authors — the worst they
//     can do is lock people out of one subtree, never widen the blast radius
//     of the space itself.
//
//  2. Restrictions are INHERITED down the tree, so restricting a page
//     restricts its whole subtree without writing a row per descendant. There
//     is deliberately no cascade here: a cascade would have to be re-run on
//     every move, and a moved page would silently carry somebody else's
//     permissions into its new home.
//
//  3. Space admins are EXEMPT from this layer entirely. That is the reason
//     the panel can never be used to lock a space out of its own content, and
//     the reason a few of the guards below can be as simple as they are.
//
// ---- decision: restricting seeds the page with its own people
//
// Cutting inheritance on a page with no grants would leave it visible to
// space admins alone — taking it away from the person who wrote it. That is
// confiscation, not narrowing, so the first restriction seeds one grant: the
// page's author, at admin, which under rule 1 means "not narrowed" rather
// than "promoted".
//
// ---- why there is no "you are locking yourself out" guard
//
// Every write below needs admin on the page, and rule 1 says a grant cannot
// lift anybody to admin — so the only people who reach these methods are
// space admins, and rule 3 says they are exempt from the whole layer. There
// is no sequence of calls here that can take away the caller's own way back
// in, which is why no guard checks for one. If management is ever opened to
// page authors, that stops being true and the guard has to appear.

// GrantView is one row of a page's permission list.
type GrantView struct {
	PrincipalType model.PrincipalType `json:"principal_type"`
	PrincipalID   string              `json:"principal_id"`
	// Role is what was granted here: a ceiling, not a promotion.
	Role model.SpaceRole `json:"role"`
	// Effective is what this principal actually ends up with on the page,
	// once its space role has been applied. It differs from Role whenever
	// somebody is granted more than the space gives them, which is worth
	// showing rather than letting them believe the grant did something.
	Effective model.SpaceRole `json:"effective"`
	// InSpace is false when the principal has no role in the space at all.
	// The grant is then inert: the panel says so instead of listing a name
	// that cannot open the page.
	InSpace bool `json:"in_space"`

	Name             string `json:"name"`
	Email            string `json:"email,omitempty"`
	Avatar           string `json:"avatar,omitempty"`
	IsDefaultGroup   bool   `json:"is_default_group,omitempty"`
	GroupMemberCount *int64 `json:"group_member_count,omitempty"`
	AddedBy          string `json:"added_by,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// AncestorRef names a page in the permission chain.
type AncestorRef struct {
	ID      string `json:"id"`
	ShortID string `json:"short_id"`
	Title   string `json:"title"`
	// Visible is false when the caller may not open that ancestor. Its title
	// is then omitted: which pages exist above is itself information.
	Visible bool `json:"visible"`
}

// PageAccessView is the page's permission panel.
type PageAccessView struct {
	PageID string `json:"page_id"`
	// Restricted is whether this page itself cuts inheritance.
	Restricted bool `json:"restricted"`
	// InheritedFrom lists restricted ANCESTORS, nearest last. A page can be
	// narrowed by a level above it that the panel does not manage, and not
	// saying so makes the panel look broken.
	InheritedFrom []AncestorRef `json:"inherited_from"`
	// Grants is empty when the page is not restricted.
	Grants []*GrantView `json:"grants"`
	// CanManage is whether the caller may change any of this.
	CanManage bool `json:"can_manage"`
	// SpaceDefault is the role the space gives its members, shown so the
	// panel can say what "inherited" actually means here.
	SpaceDefault model.SpaceRole `json:"space_default"`
}

// GrantInput is one principal's page-level ceiling.
type GrantInput struct {
	PrincipalType model.PrincipalType
	PrincipalID   string
	Role          model.SpaceRole
}

// MaxPageGrants bounds a page's permission list. Past this the panel is not
// a permission list any more, it is a membership system built in the wrong
// place — the answer there is a group.
const MaxPageGrants = 100

// PageAccess returns the page's permission panel.
//
// Reading it needs only read access on the page: somebody who can open a page
// is entitled to know why, and hiding the reason makes "why can't my
// colleague see this" unanswerable without an administrator.
func (s *PageService) PageAccess(ctx context.Context, actor *acl.Identity, d acl.Decision) (
	*PageAccessView, error,
) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	out := &PageAccessView{
		PageID: d.Page.ID, Grants: []*GrantView{}, InheritedFrom: []AncestorRef{},
		CanManage:    d.Allows(model.RoleAdmin),
		SpaceDefault: d.Space.DefaultRole,
	}
	for _, id := range d.RestrictedAt {
		if id == d.Page.ID {
			out.Restricted = true
			continue
		}
		ref, err := s.ancestorRef(ctx, actor, d, id)
		if err != nil {
			return nil, err
		}
		out.InheritedFrom = append(out.InheritedFrom, ref)
	}

	if out.Restricted {
		grants, err := s.d.Repos.Access.ListGrants(ctx, d.Page.TenantID, d.Page.ID)
		if err != nil {
			return nil, err
		}
		views, err := s.grantViews(ctx, d.Space, grants)
		if err != nil {
			return nil, err
		}
		out.Grants = views
	}
	return out, nil
}

// SetPageRestricted cuts a page's inheritance or restores it.
func (s *PageService) SetPageRestricted(ctx context.Context, actor *acl.Identity, d acl.Decision,
	restricted bool,
) (*PageAccessView, error) {
	if err := requireRole(d, model.RoleAdmin); err != nil {
		return nil, err
	}
	already := false
	for _, id := range d.RestrictedAt {
		if id == d.Page.ID {
			already = true
		}
	}
	if restricted == already {
		return s.PageAccess(ctx, actor, d)
	}

	if restricted {
		if err := s.d.Repos.Access.SetRestricted(ctx, d.Page.TenantID, d.Page.SpaceID,
			d.Page.ID, actorID(actor)); err != nil {
			return nil, err
		}
		if err := s.seedGrants(ctx, actor, d.Page); err != nil {
			return nil, err
		}
		s.recordAccessChange(ctx, actor, d.Page, audit.PageAccessRestricted, "restricted", model.Principal{})
	} else {
		// Clearing takes the grants with it: they only have meaning while the
		// page is restricted, and leaving them would resurrect a permission
		// set somebody thought they had removed the next time it is cut again.
		if err := s.d.Repos.Access.ClearRestricted(ctx, d.Page.TenantID, d.Page.ID); err != nil {
			return nil, err
		}
		s.recordAccessChange(ctx, actor, d.Page, audit.PageAccessInherited, "inherited", model.Principal{})
	}

	// Re-resolved rather than reusing the decision we were handed: the caller
	// has just changed what that decision says.
	fresh, err := s.d.Resolver.Page(ctx, actor, d.Page.ID)
	if err != nil {
		return nil, err
	}
	return s.PageAccess(ctx, actor, fresh)
}

// SetPageGrant adds a principal to a restricted page or changes its ceiling.
func (s *PageService) SetPageGrant(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in GrantInput,
) (*PageAccessView, error) {
	if err := requireRole(d, model.RoleAdmin); err != nil {
		return nil, err
	}
	principal, err := cleanPrincipal(in.PrincipalType, in.PrincipalID)
	if err != nil {
		return nil, err
	}
	if !in.Role.Valid() || in.Role == model.RoleNone {
		return nil, invalid("%q is not a role that can be granted", in.Role)
	}
	if !s.pageIsRestricted(d) {
		return nil, invalid("this page inherits its permissions; restrict it before granting access")
	}
	if err := s.checkPrincipalExists(ctx, d.Page.TenantID, principal); err != nil {
		return nil, err
	}

	existing, err := s.d.Repos.Access.ListGrants(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, g := range existing {
		if g.Principal() == principal {
			found = true
		}
	}
	if !found && len(existing) >= MaxPageGrants {
		return nil, invalid("a page may list at most %d principals; use a group instead", MaxPageGrants)
	}

	by := actorID(actor)
	grant := &model.PageGrant{
		PageID: d.Page.ID, TenantID: d.Page.TenantID,
		PrincipalType: principal.Type, PrincipalID: principal.ID, Role: in.Role,
	}
	if by != "" {
		grant.AddedBy = &by
	}
	if err := s.d.Repos.Access.UpsertGrant(ctx, grant); err != nil {
		return nil, err
	}
	s.recordAccessChange(ctx, actor, d.Page, audit.PageGrantAdded,
		string(principal.Type)+":"+principal.ID+"="+string(in.Role), principal)

	fresh, err := s.d.Resolver.Page(ctx, actor, d.Page.ID)
	if err != nil {
		return nil, err
	}
	return s.PageAccess(ctx, actor, fresh)
}

// RemovePageGrant takes a principal off a restricted page.
func (s *PageService) RemovePageGrant(ctx context.Context, actor *acl.Identity, d acl.Decision,
	principalType model.PrincipalType, principalID string,
) (*PageAccessView, error) {
	if err := requireRole(d, model.RoleAdmin); err != nil {
		return nil, err
	}
	principal, err := cleanPrincipal(principalType, principalID)
	if err != nil {
		return nil, err
	}
	if err := s.d.Repos.Access.RemoveGrant(ctx, d.Page.TenantID, d.Page.ID, principal); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("grant")
		}
		return nil, err
	}
	s.recordAccessChange(ctx, actor, d.Page, audit.PageGrantRemoved,
		string(principal.Type)+":"+principal.ID, principal)

	fresh, err := s.d.Resolver.Page(ctx, actor, d.Page.ID)
	if err != nil {
		return nil, err
	}
	return s.PageAccess(ctx, actor, fresh)
}

// ---- helpers -------------------------------------------------------------------

func (s *PageService) pageIsRestricted(d acl.Decision) bool {
	for _, id := range d.RestrictedAt {
		if id == d.Page.ID {
			return true
		}
	}
	return false
}

// seedGrants gives a newly restricted page the one person who would
// otherwise lose it: whoever wrote it. See the decision note.
//
// The caller is not seeded. They are a space admin — they are exempt from
// this layer and a row for them would do nothing except suggest, on a panel
// meant to explain, that their access comes from somewhere it does not.
func (s *PageService) seedGrants(ctx context.Context, actor *acl.Identity, page *model.Page) error {
	if page.CreatorID == nil || *page.CreatorID == "" {
		return nil
	}
	by := actorID(actor)
	grant := &model.PageGrant{
		PageID: page.ID, TenantID: page.TenantID,
		PrincipalType: model.PrincipalUser, PrincipalID: *page.CreatorID, Role: model.RoleAdmin,
	}
	if by != "" {
		grant.AddedBy = &by
	}
	return s.d.Repos.Access.UpsertGrant(ctx, grant)
}

func (s *PageService) checkPrincipalExists(ctx context.Context, tenantID uint64,
	p model.Principal,
) error {
	switch p.Type {
	case model.PrincipalGroup:
		groups, err := s.groupIndex(ctx, tenantID)
		if err != nil {
			return err
		}
		if _, ok := groups[p.ID]; !ok {
			return notFound("group")
		}
	case model.PrincipalUser:
		ok, err := s.activeMember(ctx, tenantID, p.ID)
		if err != nil {
			return err
		}
		if !ok {
			return invalid("that person is not an active member of this workspace")
		}
	}
	return nil
}

func (s *PageService) ancestorRef(ctx context.Context, actor *acl.Identity, d acl.Decision,
	id string,
) (AncestorRef, error) {
	ref := AncestorRef{ID: id}
	page, err := s.d.Repos.Pages.Get(ctx, d.Page.TenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ref, nil
		}
		return ref, err
	}
	above, err := s.d.Resolver.Decide(ctx, actor, page)
	if err != nil {
		return ref, err
	}
	if above.Role == model.RoleNone {
		// "There is a level above you cannot see" is the honest answer; its
		// title is not part of it.
		return ref, nil
	}
	ref.Visible = true
	ref.ShortID = page.ShortID
	ref.Title = page.Title
	return ref, nil
}

// grantViews resolves each grant for display and works out what it actually
// does, which is not the same as what it says.
func (s *PageService) grantViews(ctx context.Context, space *model.Space,
	grants []*model.PageGrant,
) ([]*GrantView, error) {
	var userIDs []string
	principals := make([]model.Principal, 0, len(grants))
	for _, g := range grants {
		principals = append(principals, g.Principal())
		if g.PrincipalType == model.PrincipalUser {
			userIDs = append(userIDs, g.PrincipalID)
		}
	}
	users := s.users(ctx, userIDs)
	groups, err := s.groupIndex(ctx, space.TenantID)
	if err != nil {
		return nil, err
	}
	spaceRoles, err := s.spaceRolesOf(ctx, space, principals)
	if err != nil {
		return nil, err
	}

	out := make([]*GrantView, 0, len(grants))
	for _, g := range grants {
		p := g.Principal()
		v := &GrantView{
			PrincipalType: g.PrincipalType, PrincipalID: g.PrincipalID, Role: g.Role,
			CreatedAt: g.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		if g.AddedBy != nil {
			v.AddedBy = *g.AddedBy
		}
		inSpace := spaceRoles[p]
		v.InSpace = inSpace != model.RoleNone
		// Rule 1, made visible: the grant is a ceiling on the space role.
		v.Effective = model.MinRole(inSpace, g.Role)

		switch g.PrincipalType {
		case model.PrincipalUser:
			u := userView(g.PrincipalID, users)
			v.Name, v.Email, v.Avatar = u.Username, u.Email, u.Avatar
			if v.Name == "" {
				v.Name = g.PrincipalID
			}
		case model.PrincipalGroup:
			if grp, ok := groups[g.PrincipalID]; ok {
				v.Name = grp.Name
				v.IsDefaultGroup = grp.IsDefault
				count, err := s.groupMemberCount(ctx, space.TenantID, grp)
				if err != nil {
					return nil, err
				}
				v.GroupMemberCount = &count
			} else {
				v.Name = g.PrincipalID
			}
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Role.Level() != b.Role.Level() {
			return a.Role.Level() > b.Role.Level()
		}
		if a.PrincipalType != b.PrincipalType {
			return a.PrincipalType == model.PrincipalGroup
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return out, nil
}

// recordAccessChange audits the change, invalidates the tenant's cached
// decisions and tells the tree to repaint.
//
// The invalidation is the important half: every open client is holding a
// permission answer that may have just become wrong.
func (s *PageService) recordAccessChange(ctx context.Context, actor *acl.Identity, page *model.Page,
	action types.AuditAction, detail string, target model.Principal,
) {
	s.invalidate(ctx, page.TenantID)
	entry := audit.Entry{
		TenantID: page.TenantID, ActorUserID: actorID(actor), Action: action,
		SpaceID: page.SpaceID, TargetType: audit.TargetPage, TargetID: page.ID,
	}
	// Who the grant was about, where the audit vocabulary has a place for it.
	// A group has no user id, so that column stays empty rather than being
	// filled with something that is not one.
	if target.Type == model.PrincipalUser {
		entry.TargetUserID = target.ID
	}
	s.audit(ctx, entry)
	s.publish(ctx, events.New(events.PageAccess, page.TenantID).
		WithSpace(page.SpaceID).WithPage(page.ID).WithActor(actorID(actor)).
		With("change", detail))
}

func cleanPrincipal(t model.PrincipalType, id string) (model.Principal, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Principal{}, invalid("a principal id is required")
	}
	switch t {
	case model.PrincipalUser, model.PrincipalGroup:
		return model.Principal{Type: t, ID: id}, nil
	default:
		return model.Principal{}, invalid("%q is not a principal type", t)
	}
}

// spaceRolesOf works out what each granted principal holds in the space,
// which is what decides whether a grant does anything at all.
//
// A person is resolved through the full identity path rather than by looking
// for a membership row: most people reach a space through a group, and a
// panel that called those people "not in this space" would be wrong about
// the common case. Both lookups the identity path makes are cached.
//
// A group is looked up directly. Groups do not belong to other groups, so a
// membership row is the whole answer for one — with the space's own default
// layered on where an open or public space gives everybody a role.
func (s *PageService) spaceRolesOf(ctx context.Context, space *model.Space,
	principals []model.Principal,
) (map[model.Principal]model.SpaceRole, error) {
	out := make(map[model.Principal]model.SpaceRole, len(principals))

	var groupRows map[string]model.SpaceRole
	for _, p := range principals {
		switch p.Type {
		case model.PrincipalUser:
			id, err := s.d.Resolver.Identity(ctx, space.TenantID, p.ID)
			if err != nil {
				return nil, err
			}
			role, err := s.d.Resolver.SpaceRole(ctx, id, space)
			if err != nil {
				return nil, err
			}
			out[p] = role
		case model.PrincipalGroup:
			if groupRows == nil {
				rows, err := s.d.Repos.Members.ListBySpace(ctx, space.TenantID, space.ID)
				if err != nil {
					return nil, err
				}
				groupRows = map[string]model.SpaceRole{}
				for _, row := range rows {
					if row.PrincipalType == model.PrincipalGroup {
						groupRows[row.PrincipalID] = row.Role
					}
				}
			}
			role := groupRows[p.ID]
			switch space.Visibility {
			case model.VisibilityOpen:
				role = model.MaxRole(role, space.DefaultRole)
			case model.VisibilityPublic:
				role = model.MaxRole(role, model.MaxRole(space.DefaultRole, model.RoleReader))
			}
			out[p] = role
		}
	}
	return out, nil
}

// EffectivePermission is the short answer to "what may I do here", for
// clients deciding which controls to draw.
//
// Separate from the panel because it is asked on every page open and the
// panel is asked when somebody clicks "share": this one costs nothing beyond
// the decision the guard already made.
type EffectivePermission struct {
	PageID string          `json:"page_id"`
	Role   model.SpaceRole `json:"role"`
	// CanEdit folds the lock state in, as the page view does.
	CanEdit bool `json:"can_edit"`
	// CanComment is true for readers too: commenting is not editing.
	CanComment bool `json:"can_comment"`
	// CanManageAccess is whether the permission panel is actionable.
	CanManageAccess bool `json:"can_manage_access"`
	// Restricted is whether this page or an ancestor cuts inheritance, which
	// is what a client needs to draw the badge.
	Restricted bool `json:"restricted"`
	// RestrictedHere distinguishes "narrowed by a level above" from "narrowed
	// right here", which are different things to offer somebody.
	RestrictedHere bool `json:"restricted_here"`
}

// EffectivePermission resolves what the caller may do with a page.
func (s *PageService) EffectivePermission(_ context.Context, d acl.Decision) (*EffectivePermission, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	out := &EffectivePermission{
		PageID: d.Page.ID, Role: d.Role,
		CanEdit:         canEdit(d.Role, d.Page),
		CanComment:      d.Allows(model.RoleReader),
		CanManageAccess: d.Allows(model.RoleAdmin),
		Restricted:      len(d.RestrictedAt) > 0,
	}
	for _, id := range d.RestrictedAt {
		if id == d.Page.ID {
			out.RestrictedHere = true
		}
	}
	return out, nil
}
