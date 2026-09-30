// Package acl resolves what a caller may do with a docs space or page.
//
// The model (技术方案 §3) has three layers, applied in order:
//
//  1. Tenant role. Owner and Admin of the tenant are admin everywhere in the
//     tenant's spaces; a Viewer can never exceed reader; a non-member has
//     nothing.
//  2. Space role. The strongest of the caller's direct memberships (as a
//     user or through a group) and, for open/public spaces, the space's
//     default role.
//  3. Page restrictions. Walking from the root to the page, every restricted
//     ancestor must grant the caller something, and each one can only narrow
//     the role. Space admins skip this layer.
//
// Finally a locked page caps non-admins at reader. Results are cached per
// (tenant, user, page) and dropped for the whole tenant on any change.
package acl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/types"
)

// TenantRoleSource answers "what role does this user hold in this tenant".
// It is an interface so the resolver can be tested without the full tenant
// membership service; production wires it to the TenantMember repository.
type TenantRoleSource interface {
	// TenantRole returns the role of an ACTIVE member. ok is false for
	// non-members and for invited/suspended memberships.
	TenantRole(ctx context.Context, tenantID uint64, userID string) (role types.TenantRole, ok bool, err error)
}

// Identity is a caller expanded to every principal that can carry a grant.
type Identity struct {
	TenantID   uint64
	UserID     string
	TenantRole types.TenantRole
	// Member is false for callers with no active membership in the tenant.
	Member bool
	// Principals: the user itself, its explicit groups, and the tenant's
	// default group (implicit membership of every member).
	Principals []model.Principal
	// DefaultGroupID is the implicit "everyone" group, if one exists.
	DefaultGroupID string
	// Machine marks an API key's identity (see MachineIdentity); UserID is
	// then not a user account.
	Machine bool `json:",omitempty"`
	// KnowledgeBases, when set, limits the identity to spaces bound to one of
	// these knowledge bases. Only machine identities carry it.
	KnowledgeBases map[string]bool `json:",omitempty"`
}

// IsTenantAdmin reports whether the identity administers the whole tenant:
// every space, and the workspace-level settings of the module (groups,
// workspace templates, the space trash). An identity limited to some
// knowledge bases never does, whatever its role.
func (id *Identity) IsTenantAdmin() bool {
	return id.administers() && id.KnowledgeBases == nil
}

// administers reports a tenant Owner or Admin role, which makes the identity
// admin of every space it can reach.
func (id *Identity) administers() bool {
	return id.Member && (id.TenantRole == types.TenantRoleOwner || id.TenantRole == types.TenantRoleAdmin)
}

// Decision is the outcome of resolving a page.
type Decision struct {
	// UserID is who the decision was made for: a user, or an API key's
	// machine identity. Rules about "the caller" read it here rather than
	// from the request, whose user an API key does not act as.
	UserID string
	Role   model.SpaceRole
	Page   *model.Page
	Space  *model.Space
	// Chain lists ancestor IDs root-first followed by the page itself.
	Chain []string
	// RestrictedAt lists which chain members cut inheritance.
	RestrictedAt []string
}

// Allows reports whether the decision grants at least the given role.
func (d Decision) Allows(min model.SpaceRole) bool {
	return d.Role.AtLeast(min) && d.Role != model.RoleNone
}

// Resolver computes effective permissions.
type Resolver struct {
	repos *repository.Repositories
	roles TenantRoleSource
	cache Cache
	ttl   time.Duration
}

// Option configures a Resolver.
type Option func(*Resolver)

// WithCache sets the decision cache (default: in-process).
func WithCache(c Cache) Option { return func(r *Resolver) { r.cache = c } }

// WithTTL sets how long a cached decision may be served (default 60s).
func WithTTL(d time.Duration) Option { return func(r *Resolver) { r.ttl = d } }

// NewResolver builds a resolver over the module's repositories.
func NewResolver(repos *repository.Repositories, roles TenantRoleSource, opts ...Option) *Resolver {
	r := &Resolver{repos: repos, roles: roles, cache: NewMemoryCache(0), ttl: 60 * time.Second}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Invalidate drops every cached decision of a tenant. Call it after any
// change to space membership, groups, page grants, restrictions, locks, or
// the page tree.
func (r *Resolver) Invalidate(ctx context.Context, tenantID uint64) {
	r.cache.InvalidateTenant(ctx, tenantID)
}

// ---- identity ----------------------------------------------------------------

// Identity expands a user into its principals. Cached.
func (r *Resolver) Identity(ctx context.Context, tenantID uint64, userID string) (*Identity, error) {
	key := "id:" + userID
	if raw, ok := r.cache.Get(ctx, tenantID, key); ok {
		var id Identity
		if json.Unmarshal(raw, &id) == nil {
			return &id, nil
		}
	}
	id := &Identity{TenantID: tenantID, UserID: userID}
	role, ok, err := r.roles.TenantRole(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("acl: tenant role: %w", err)
	}
	if ok {
		id.Member = true
		id.TenantRole = role
		id.Principals = append(id.Principals, model.UserPrincipal(userID))
		groups, err := r.repos.Groups.GroupIDsForUser(ctx, tenantID, userID)
		if err != nil {
			return nil, fmt.Errorf("acl: groups: %w", err)
		}
		for _, g := range groups {
			id.Principals = append(id.Principals, model.GroupPrincipal(g))
		}
		if def, err := r.defaultGroup(ctx, tenantID); err != nil {
			return nil, err
		} else if def != "" {
			id.DefaultGroupID = def
			id.Principals = append(id.Principals, model.GroupPrincipal(def))
		}
	}
	if raw, err := json.Marshal(id); err == nil {
		r.cache.Set(ctx, tenantID, key, raw, r.ttl)
	}
	return id, nil
}

func (r *Resolver) defaultGroup(ctx context.Context, tenantID uint64) (string, error) {
	groups, err := r.repos.Groups.List(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("acl: default group: %w", err)
	}
	for _, g := range groups {
		if g.IsDefault {
			return g.ID, nil
		}
	}
	return "", nil
}

// ---- spaces ------------------------------------------------------------------

// SpaceRole resolves the caller's role in a space (layers 1 and 2).
func (r *Resolver) SpaceRole(ctx context.Context, id *Identity, space *model.Space) (model.SpaceRole, error) {
	if !id.Member || !id.reaches(space) {
		return model.RoleNone, nil
	}
	if id.administers() {
		return model.RoleAdmin, nil
	}
	direct, err := r.repos.Members.RolesFor(ctx, id.TenantID, space.ID, id.Principals)
	if err != nil {
		return model.RoleNone, fmt.Errorf("acl: space roles: %w", err)
	}
	return r.combineSpaceRole(id, space, direct), nil
}

func (r *Resolver) combineSpaceRole(id *Identity, space *model.Space, direct []model.SpaceRole) model.SpaceRole {
	role := model.RoleNone
	for _, d := range direct {
		role = model.MaxRole(role, d)
	}
	switch space.Visibility {
	case model.VisibilityOpen:
		role = model.MaxRole(role, space.DefaultRole)
	case model.VisibilityPublic:
		// Public spaces are readable by anyone; members get at least the
		// default role and never less than reader.
		role = model.MaxRole(role, model.MaxRole(space.DefaultRole, model.RoleReader))
	}
	if id.TenantRole == types.TenantRoleViewer {
		role = model.MinRole(role, model.RoleReader)
	}
	return role
}

// VisibleSpaces lists every live space of the tenant the caller can read,
// with the caller's role in each.
func (r *Resolver) VisibleSpaces(ctx context.Context, id *Identity) (map[string]model.SpaceRole, error) {
	out := map[string]model.SpaceRole{}
	if !id.Member {
		return out, nil
	}
	spaces, err := r.repos.Spaces.List(ctx, id.TenantID)
	if err != nil {
		return nil, fmt.Errorf("acl: spaces: %w", err)
	}
	if id.administers() {
		for _, s := range spaces {
			if id.reaches(s) {
				out[s.ID] = model.RoleAdmin
			}
		}
		return out, nil
	}
	direct, err := r.repos.Members.SpaceRolesFor(ctx, id.TenantID, id.Principals)
	if err != nil {
		return nil, fmt.Errorf("acl: space roles: %w", err)
	}
	for _, s := range spaces {
		if !id.reaches(s) {
			continue
		}
		var d []model.SpaceRole
		if role, ok := direct[s.ID]; ok {
			d = []model.SpaceRole{role}
		}
		if role := r.combineSpaceRole(id, s, d); role != model.RoleNone {
			out[s.ID] = role
		}
	}
	return out, nil
}

// ---- pages -------------------------------------------------------------------

// Page resolves the caller's effective role on a live page. Pages in the
// trash and pages of other tenants yield repository.ErrNotFound; callers
// should surface that as 404 regardless of role, which also avoids leaking
// whether a page exists.
func (r *Resolver) Page(ctx context.Context, id *Identity, pageID string) (Decision, error) {
	// The row is always read fresh: it proves the page is live and gives the
	// handler current metadata. Only the role computation is cached.
	page, err := r.repos.Pages.Get(ctx, id.TenantID, pageID)
	if err != nil {
		return Decision{}, err
	}
	key := "pg:" + id.UserID + ":" + pageID
	if raw, ok := r.cache.Get(ctx, id.TenantID, key); ok {
		var cached cachedDecision
		if json.Unmarshal(raw, &cached) == nil && cached.SpaceID == page.SpaceID {
			if space, err := r.repos.Spaces.Get(ctx, id.TenantID, page.SpaceID); err == nil {
				d := cached.toDecision()
				d.UserID, d.Page, d.Space = id.UserID, page, space
				return d, nil
			}
		}
	}
	d, err := r.Decide(ctx, id, page)
	if err != nil {
		return Decision{}, err
	}
	if raw, err := json.Marshal(fromDecision(d)); err == nil {
		r.cache.Set(ctx, id.TenantID, key, raw, r.ttl)
	}
	return d, nil
}

// cachedDecision is the serialisable form (Page/Space carry content-heavy
// fields the cache should not hold).
type cachedDecision struct {
	Role         model.SpaceRole `json:"role"`
	Chain        []string        `json:"chain"`
	RestrictedAt []string        `json:"restricted_at"`
	SpaceID      string          `json:"space_id"`
	PageID       string          `json:"page_id"`
}

func fromDecision(d Decision) cachedDecision {
	c := cachedDecision{Role: d.Role, Chain: d.Chain, RestrictedAt: d.RestrictedAt}
	if d.Space != nil {
		c.SpaceID = d.Space.ID
	}
	if d.Page != nil {
		c.PageID = d.Page.ID
	}
	return c
}

// toDecision restores the cached part; the caller attaches the freshly
// loaded page and space.
func (c cachedDecision) toDecision() Decision {
	return Decision{Role: c.Role, Chain: c.Chain, RestrictedAt: c.RestrictedAt}
}

// Decide resolves the caller's role on an already loaded page, which may be
// in the trash (its live ancestors still narrow; trashed ones are skipped).
// Trash listing and restore use it; live lookups go through Page, which
// caches.
func (r *Resolver) Decide(ctx context.Context, id *Identity, page *model.Page) (Decision, error) {
	space, err := r.repos.Spaces.Get(ctx, id.TenantID, page.SpaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Space in the trash: its pages are unreachable.
			return Decision{}, repository.ErrNotFound
		}
		return Decision{}, err
	}
	d := Decision{UserID: id.UserID, Page: page, Space: space}

	ancestors, err := r.repos.Pages.ListAncestors(ctx, id.TenantID, page.ID)
	if err != nil {
		return Decision{}, fmt.Errorf("acl: ancestors: %w", err)
	}
	for _, a := range ancestors {
		d.Chain = append(d.Chain, a.ID)
	}
	d.Chain = append(d.Chain, page.ID)

	restricted, err := r.repos.Access.Restricted(ctx, id.TenantID, d.Chain)
	if err != nil {
		return Decision{}, fmt.Errorf("acl: restrictions: %w", err)
	}
	for _, cid := range d.Chain {
		if restricted[cid] {
			d.RestrictedAt = append(d.RestrictedAt, cid)
		}
	}

	role, err := r.SpaceRole(ctx, id, space)
	if err != nil {
		return Decision{}, err
	}
	if role == model.RoleNone {
		d.Role = model.RoleNone
		return d, nil
	}

	// Layer 3: restricted ancestors narrow; space admins are exempt.
	if role != model.RoleAdmin && len(d.RestrictedAt) > 0 {
		grants, err := r.repos.Access.GrantsFor(ctx, id.TenantID, d.RestrictedAt, id.Principals)
		if err != nil {
			return Decision{}, fmt.Errorf("acl: grants: %w", err)
		}
		for _, rid := range d.RestrictedAt {
			granted := model.RoleNone
			for _, g := range grants[rid] {
				granted = model.MaxRole(granted, g)
			}
			if granted == model.RoleNone {
				d.Role = model.RoleNone
				return d, nil
			}
			role = model.MinRole(role, granted)
		}
		// A page-level grant can never exceed what the tenant role allows.
		if id.TenantRole == types.TenantRoleViewer {
			role = model.MinRole(role, model.RoleReader)
		}
	}

	if page.IsLocked && role != model.RoleAdmin {
		role = model.MinRole(role, model.RoleReader)
	}
	d.Role = role
	return d, nil
}

// ---- subjects (ACL snapshots for retrieval) ------------------------------------

// Subject prefixes used in retrieval metadata and in query-time expansion.
const (
	SubjectSpace       = "space:"        // every reader of the space
	SubjectSpaceAdmin  = "space-admin:"  // admins of the space
	SubjectTenantAdmin = "tenant-admin:" // tenant owners/admins
	SubjectUser        = "user:"
	SubjectGroup       = "group:"
)

// PageSubjects computes the set of subjects allowed to see a page, for
// storage next to its indexed chunks. An unrestricted page is visible to the
// whole space; a restricted page to the intersection of the principals
// granted at every restricted level. Space and tenant admins are always
// included so their queries match without a special case.
func (r *Resolver) PageSubjects(ctx context.Context, tenantID uint64, pageID string) ([]string, error) {
	page, err := r.repos.Pages.Get(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	ancestors, err := r.repos.Pages.ListAncestors(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	chain := make([]string, 0, len(ancestors)+1)
	for _, a := range ancestors {
		chain = append(chain, a.ID)
	}
	chain = append(chain, page.ID)

	restricted, err := r.repos.Access.Restricted(ctx, tenantID, chain)
	if err != nil {
		return nil, err
	}
	subjects := map[string]bool{
		SubjectSpaceAdmin + page.SpaceID:          true,
		SubjectTenantAdmin + fmt.Sprint(tenantID): true,
	}
	var restrictedIDs []string
	for _, cid := range chain {
		if restricted[cid] {
			restrictedIDs = append(restrictedIDs, cid)
		}
	}
	if len(restrictedIDs) == 0 {
		subjects[SubjectSpace+page.SpaceID] = true
		return sortedKeys(subjects), nil
	}
	grants, err := r.repos.Access.GrantsForPages(ctx, tenantID, restrictedIDs)
	if err != nil {
		return nil, err
	}
	var allowed map[string]bool
	for _, rid := range restrictedIDs {
		level := map[string]bool{}
		for _, g := range grants[rid] {
			level[subjectOf(g.Principal())] = true
		}
		if allowed == nil {
			allowed = level
			continue
		}
		for s := range allowed {
			if !level[s] {
				delete(allowed, s)
			}
		}
	}
	for s := range allowed {
		subjects[s] = true
	}
	return sortedKeys(subjects), nil
}

// QuerySubjects expands a caller into the subjects it may match at query
// time: itself, its groups, the spaces it can read, the spaces it
// administers, and the tenant-admin marker when applicable.
func (r *Resolver) QuerySubjects(ctx context.Context, id *Identity) ([]string, error) {
	if !id.Member {
		return nil, nil
	}
	subjects := map[string]bool{}
	for _, p := range id.Principals {
		subjects[subjectOf(p)] = true
	}
	if id.IsTenantAdmin() {
		subjects[SubjectTenantAdmin+fmt.Sprint(id.TenantID)] = true
	}
	spaces, err := r.VisibleSpaces(ctx, id)
	if err != nil {
		return nil, err
	}
	for sid, role := range spaces {
		subjects[SubjectSpace+sid] = true
		if role == model.RoleAdmin {
			subjects[SubjectSpaceAdmin+sid] = true
		}
	}
	return sortedKeys(subjects), nil
}

func subjectOf(p model.Principal) string {
	if p.Type == model.PrincipalGroup {
		return SubjectGroup + p.ID
	}
	return SubjectUser + p.ID
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
