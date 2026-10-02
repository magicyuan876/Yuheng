package service

import (
	"context"
	"encoding/json"
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

// SpaceService implements spaces and their membership.
type SpaceService struct{ *base }

// SpaceView is a space as the API returns it: the row plus what the caller
// may do in it and two cheap counters for list cards.
type SpaceView struct {
	*model.Space
	// Role is the caller's effective role in the space.
	Role model.SpaceRole `json:"role"`
	// MemberCount counts direct memberships (users and groups), not the
	// users reachable through groups.
	MemberCount int   `json:"member_count"`
	PageCount   int64 `json:"page_count"`
}

// CreateSpaceInput is what a client may set when creating a space.
type CreateSpaceInput struct {
	Name        string
	Slug        string
	Description string
	Icon        *string
	Visibility  model.SpaceVisibility
	// DefaultRole applies to tenant members who are not members of the
	// space; it is meaningful for open spaces only.
	DefaultRole      model.SpaceRole
	KnowledgeBaseID  *string
	StorageBackendID *string
	Settings         json.RawMessage
}

// UpdateSpaceInput is a partial update; nil fields are left alone. An empty
// Icon clears the icon.
type UpdateSpaceInput struct {
	Name        *string
	Slug        *string
	Description *string
	Icon        *string
	Visibility  *model.SpaceVisibility
	DefaultRole *model.SpaceRole
	Settings    json.RawMessage
}

// normaliseVisibility applies the coupling between visibility and default
// role: a private space grants nothing by default; an open space must grant
// at least reader, else "open" would mean nothing.
//
// A public space is readable by anyone at all, with no login, so it is
// refused unless the deployment has switched public sharing on — the same
// gate a share link goes through, because both put content on the public
// internet and an installation that said no to one did not say yes to the
// other.
func (s *SpaceService) normaliseVisibility(v model.SpaceVisibility, role model.SpaceRole) (
	model.SpaceVisibility, model.SpaceRole, error,
) {
	if v == "" {
		v = model.VisibilityPrivate
	}
	switch v {
	case model.VisibilityPrivate:
		return v, model.RoleNone, nil
	case model.VisibilityOpen:
		if role == "" || role == model.RoleNone {
			role = model.RoleReader
		}
		if role != model.RoleReader && role != model.RoleWriter {
			return "", "", invalid("default_role of an open space must be reader or writer")
		}
		return v, role, nil
	case model.VisibilityPublic:
		if !s.d.PublicSharing {
			return "", "", forbidden("public sharing is switched off in this deployment")
		}
		// The default role governs signed-in members; anonymous visitors
		// always get reader and nothing more, which the resolver enforces.
		if role == "" || role == model.RoleNone {
			role = model.RoleReader
		}
		if role != model.RoleReader && role != model.RoleWriter {
			return "", "", invalid("default_role of a public space must be reader or writer")
		}
		return v, role, nil
	default:
		return "", "", invalid("visibility must be private, open or public")
	}
}

// Create makes a space and its first administrator (the creator).
func (s *SpaceService) Create(ctx context.Context, actor *acl.Identity, in CreateSpaceInput) (*SpaceView, error) {
	name, err := cleanName("name", in.Name)
	if err != nil {
		return nil, err
	}
	desc, err := cleanDescription(in.Description)
	if err != nil {
		return nil, err
	}
	vis, role, err := s.normaliseVisibility(in.Visibility, in.DefaultRole)
	if err != nil {
		return nil, err
	}
	settings, err := cleanSettings(in.Settings)
	if err != nil {
		return nil, err
	}
	icon, err := cleanIcon(in.Icon)
	if err != nil {
		return nil, err
	}
	kbID, err := s.checkKnowledgeBase(ctx, actor.TenantID, in.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	var requested string
	if in.StorageBackendID != nil {
		requested = *in.StorageBackendID
	}
	storageID, err := s.checkStorageBackend(ctx, actor.TenantID, requested)
	if err != nil {
		return nil, err
	}

	explicitSlug := strings.ToLower(strings.TrimSpace(in.Slug))
	if explicitSlug != "" && !ValidSlug(explicitSlug) {
		return nil, invalid("slug must be %d-%d lowercase letters, digits or hyphens", MinSlugLen, MaxSlugLen)
	}

	space := &model.Space{
		TenantID: actor.TenantID, Name: name, Description: desc, Icon: icon,
		Visibility: vis, DefaultRole: role, KnowledgeBaseID: kbID, StorageBackendID: storageID,
		Settings: model.JSON(settings),
	}
	if actor.UserID != "" {
		uid := actor.UserID
		space.CreatorID = &uid
	}

	// Slug collisions are rare but real (two people creating "Engineering");
	// a generated slug is retried, an explicit one is reported.
	for attempt := 0; attempt < 3; attempt++ {
		if explicitSlug != "" {
			space.Slug = explicitSlug
		} else {
			space.Slug, err = s.uniqueSlug(ctx, actor.TenantID, Slugify(name), "")
			if err != nil {
				return nil, err
			}
		}
		space.ID = ""
		err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
			if err := tx.Spaces.Create(ctx, space); err != nil {
				return err
			}
			return tx.Members.Upsert(ctx, &model.SpaceMember{
				SpaceID: space.ID, TenantID: actor.TenantID, PrincipalType: model.PrincipalUser,
				PrincipalID: actor.UserID, Role: model.RoleAdmin, AddedBy: space.CreatorID,
			})
		})
		if err == nil {
			break
		}
		if !errors.Is(err, repository.ErrDuplicate) {
			return nil, err
		}
		if explicitSlug != "" {
			return nil, conflict("slug %q is already in use", explicitSlug)
		}
	}
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceCreated, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("name", space.Name).With("slug", space.Slug))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.SpaceCreated, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
	})
	return &SpaceView{Space: space, Role: model.RoleAdmin, MemberCount: 1}, nil
}

func cleanIcon(icon *string) (*string, error) {
	if icon == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*icon)
	if v == "" {
		return nil, nil
	}
	if len(v) > 64 {
		return nil, invalid("icon must be at most 64 bytes")
	}
	return &v, nil
}

// checkKnowledgeBase validates an optional knowledge base reference.
// A pointer to "" means "none".
func (s *SpaceService) checkKnowledgeBase(ctx context.Context, tenantID uint64, id *string) (*string, error) {
	if id == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*id)
	if v == "" {
		return nil, nil
	}
	if s.d.KnowledgeBases == nil {
		return nil, invalid("knowledge base binding is not available in this deployment")
	}
	kb, err := s.d.KnowledgeBases.GetKnowledgeBaseByIDAndTenant(ctx, v, tenantID)
	if err != nil || kb == nil {
		return nil, invalid("knowledge base %q was not found in this workspace", v)
	}
	return &v, nil
}

// checkStorageBackend returns the backend a space binds to: id, or the
// workspace default when id is empty. Every space is bound — its attachments,
// imports and exports have to land somewhere. Without the storage dependency
// (a trimmed build) a space is left unbound and uploads to it are refused;
// naming a backend is then an error rather than silently ignored.
func (s *SpaceService) checkStorageBackend(ctx context.Context, tenantID uint64, id string) (string, error) {
	id = strings.TrimSpace(id)
	if s.d.StorageBackends == nil {
		if id != "" {
			return "", invalid("storage backend binding is not available in this deployment")
		}
		return "", nil
	}
	sb, err := s.d.StorageBackends.ResolveBackend(ctx, tenantID, id)
	if err != nil {
		if id == "" {
			return "", err
		}
		return "", invalid("storage backend %q cannot be used by this workspace: %v", id, err)
	}
	return sb.ID, nil
}

// List returns the spaces the caller can read, ordered by name.
func (s *SpaceService) List(ctx context.Context, actor *acl.Identity) ([]*SpaceView, error) {
	if s.d.Resolver == nil {
		return nil, errors.New("docs: resolver not configured")
	}
	roles, err := s.d.Resolver.VisibleSpaces(ctx, actor)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	spaces, err := s.d.Repos.Spaces.ListByIDs(ctx, actor.TenantID, ids)
	if err != nil {
		return nil, err
	}
	sort.Slice(spaces, func(i, j int) bool {
		if spaces[i].Name != spaces[j].Name {
			return strings.ToLower(spaces[i].Name) < strings.ToLower(spaces[j].Name)
		}
		return spaces[i].ID < spaces[j].ID
	})
	out := make([]*SpaceView, 0, len(spaces))
	for _, sp := range spaces {
		v, err := s.view(ctx, sp, roles[sp.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Get returns one space the guard has already authorised.
func (s *SpaceService) Get(ctx context.Context, space *model.Space, role model.SpaceRole) (*SpaceView, error) {
	return s.view(ctx, space, role)
}

func (s *SpaceService) view(ctx context.Context, space *model.Space, role model.SpaceRole) (*SpaceView, error) {
	members, err := s.d.Repos.Members.ListBySpace(ctx, space.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	pages, err := s.d.Repos.Pages.CountLive(ctx, space.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	return &SpaceView{Space: space, Role: role, MemberCount: len(members), PageCount: pages}, nil
}

// Update applies a partial update. The guard has verified the caller is a
// space admin.
func (s *SpaceService) Update(ctx context.Context, actor *acl.Identity, space *model.Space,
	in UpdateSpaceInput,
) (*SpaceView, error) {
	fields := map[string]any{}
	changed := []string{}
	if in.Name != nil {
		name, err := cleanName("name", *in.Name)
		if err != nil {
			return nil, err
		}
		if name != space.Name {
			fields["name"] = name
			space.Name = name
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		desc, err := cleanDescription(*in.Description)
		if err != nil {
			return nil, err
		}
		if desc != space.Description {
			fields["description"] = desc
			space.Description = desc
			changed = append(changed, "description")
		}
	}
	if in.Icon != nil {
		icon, err := cleanIcon(in.Icon)
		if err != nil {
			return nil, err
		}
		fields["icon"] = icon
		space.Icon = icon
		changed = append(changed, "icon")
	}
	if in.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.Slug))
		if !ValidSlug(slug) {
			return nil, invalid("slug must be %d-%d lowercase letters, digits or hyphens", MinSlugLen, MaxSlugLen)
		}
		if slug != space.Slug {
			taken, err := s.slugTaken(ctx, actor.TenantID, slug, space.ID)
			if err != nil {
				return nil, err
			}
			if taken {
				return nil, conflict("slug %q is already in use", slug)
			}
			fields["slug"] = slug
			space.Slug = slug
			changed = append(changed, "slug")
		}
	}
	if in.Visibility != nil || in.DefaultRole != nil {
		vis, role := space.Visibility, space.DefaultRole
		if in.Visibility != nil {
			vis = *in.Visibility
		}
		if in.DefaultRole != nil {
			role = *in.DefaultRole
		}
		vis, role, err := s.normaliseVisibility(vis, role)
		if err != nil {
			return nil, err
		}
		if vis != space.Visibility || role != space.DefaultRole {
			fields["visibility"] = vis
			fields["default_role"] = role
			space.Visibility, space.DefaultRole = vis, role
			changed = append(changed, "visibility")
		}
	}
	if len(in.Settings) > 0 {
		settings, err := cleanSettings(in.Settings)
		if err != nil {
			return nil, err
		}
		fields["settings"] = string(settings)
		space.Settings = model.JSON(settings)
		changed = append(changed, "settings")
	}
	if len(fields) == 0 {
		return s.view(ctx, space, model.RoleAdmin)
	}
	if err := s.d.Repos.Spaces.Update(ctx, actor.TenantID, space.ID, fields); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, conflict("slug %q is already in use", space.Slug)
		}
		return nil, err
	}
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceUpdated, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("changed", changed))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.SpaceUpdated, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
	})
	fresh, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, fresh, model.RoleAdmin)
}

// Delete moves a space to the trash. Its pages stay in place and become
// unreachable; purge and retention are the maintenance job's business.
func (s *SpaceService) Delete(ctx context.Context, actor *acl.Identity, space *model.Space) error {
	if err := s.d.Repos.Spaces.SoftDelete(ctx, actor.TenantID, space.ID); err != nil {
		return err
	}
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceDeleted, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.SpaceDeleted, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
	})
	return nil
}

// Restore brings a space back from the trash. Only tenant administrators
// may: a trashed space has no readable role to guard on.
func (s *SpaceService) Restore(ctx context.Context, actor *acl.Identity, spaceID string) (*SpaceView, error) {
	if !actor.IsTenantAdmin() {
		return nil, forbidden("only workspace administrators can restore spaces")
	}
	if err := s.d.Repos.Spaces.Restore(ctx, actor.TenantID, spaceID); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, conflict("another space now uses this slug; rename it first")
		}
		return nil, err
	}
	space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, spaceID)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceUpdated, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("restored", true))
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.SpaceRestored, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
	})
	return s.view(ctx, space, model.RoleAdmin)
}

// BindKnowledgeBase sets or clears the knowledge base and storage backend a
// space uses. Pointers: nil leaves the field alone; "" clears the knowledge
// base and rebinds storage to the workspace default. The
// ingestion side effects of binding belong to the knowledge bridge (T5.1);
// here only the reference is validated and stored.
func (s *SpaceService) BindKnowledgeBase(ctx context.Context, actor *acl.Identity, space *model.Space,
	knowledgeBaseID, storageBackendID *string,
) (*SpaceView, error) {
	fields := map[string]any{}
	var kbAction audit.Entry
	if knowledgeBaseID != nil {
		kb, err := s.checkKnowledgeBase(ctx, actor.TenantID, knowledgeBaseID)
		if err != nil {
			return nil, err
		}
		fields["knowledge_base_id"] = kb
		action := audit.SpaceKBBound
		target := ""
		if kb == nil {
			action = audit.SpaceKBUnbound
			if space.KnowledgeBaseID != nil {
				target = *space.KnowledgeBaseID
			}
		} else {
			target = *kb
		}
		kbAction = audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: action, SpaceID: space.ID, TargetType: "knowledge_base", TargetID: target,
		}
		space.KnowledgeBaseID = kb
	}
	if storageBackendID != nil {
		// A space is never unbound: an empty id rebinds it to the workspace
		// default. Existing files stay where they are and keep resolving.
		sb, err := s.checkStorageBackend(ctx, actor.TenantID, *storageBackendID)
		if err != nil {
			return nil, err
		}
		fields["storage_backend_id"] = sb
		space.StorageBackendID = sb
	}
	if len(fields) == 0 {
		return s.view(ctx, space, model.RoleAdmin)
	}
	if err := s.d.Repos.Spaces.Update(ctx, actor.TenantID, space.ID, fields); err != nil {
		return nil, err
	}
	s.publish(ctx, events.New(events.SpaceUpdated, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("changed", []string{"bindings"}))
	if kbAction.Action != "" {
		s.audit(ctx, kbAction)
	} else {
		s.audit(ctx, audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: audit.SpaceUpdated, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
		})
	}
	fresh, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, fresh, model.RoleAdmin)
}

// ---- members --------------------------------------------------------------------

// MemberInput names one principal and the role to grant.
type MemberInput struct {
	Type model.PrincipalType
	ID   string
	Role model.SpaceRole
}

// MemberView is a membership row with its principal resolved for display.
type MemberView struct {
	PrincipalType model.PrincipalType `json:"principal_type"`
	PrincipalID   string              `json:"principal_id"`
	Role          model.SpaceRole     `json:"role"`
	// Name is the username or the group name.
	Name   string `json:"name"`
	Email  string `json:"email,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	// Group-only fields.
	IsDefaultGroup   bool   `json:"is_default_group,omitempty"`
	GroupMemberCount *int64 `json:"group_member_count,omitempty"`
	AddedBy          string `json:"added_by,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// ListMembers returns the space's direct members, administrators first,
// groups before users within a role, then by name.
func (s *SpaceService) ListMembers(ctx context.Context, space *model.Space) ([]*MemberView, error) {
	rows, err := s.d.Repos.Members.ListBySpace(ctx, space.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	return s.memberViews(ctx, space.TenantID, rows)
}

func (s *SpaceService) memberViews(ctx context.Context, tenantID uint64, rows []*model.SpaceMember) ([]*MemberView, error) {
	var userIDs []string
	for _, r := range rows {
		if r.PrincipalType == model.PrincipalUser {
			userIDs = append(userIDs, r.PrincipalID)
		}
	}
	users := s.users(ctx, userIDs)
	groups, err := s.groupIndex(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]*MemberView, 0, len(rows))
	for _, r := range rows {
		v := &MemberView{
			PrincipalType: r.PrincipalType, PrincipalID: r.PrincipalID, Role: r.Role,
			CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		if r.AddedBy != nil {
			v.AddedBy = *r.AddedBy
		}
		switch r.PrincipalType {
		case model.PrincipalUser:
			u := userView(r.PrincipalID, users)
			v.Name, v.Email, v.Avatar = u.Username, u.Email, u.Avatar
			if v.Name == "" {
				v.Name = r.PrincipalID
			}
		case model.PrincipalGroup:
			if g, ok := groups[r.PrincipalID]; ok {
				v.Name = g.Name
				v.IsDefaultGroup = g.IsDefault
				count, err := s.groupMemberCount(ctx, tenantID, g)
				if err != nil {
					return nil, err
				}
				v.GroupMemberCount = &count
			} else {
				v.Name = r.PrincipalID
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

func (b *base) groupIndex(ctx context.Context, tenantID uint64) (map[string]*types.TenantGroup, error) {
	groups, err := b.d.Repos.Groups.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]*types.TenantGroup, len(groups))
	for _, g := range groups {
		idx[g.ID] = g
	}
	return idx, nil
}

// groupMemberCount counts a group's members; the default group counts every
// active tenant member because its membership is implicit.
func (b *base) groupMemberCount(ctx context.Context, tenantID uint64, g *types.TenantGroup) (int64, error) {
	if g.IsDefault {
		if b.d.Members == nil {
			return 0, nil
		}
		return b.d.Members.CountFilteredByTenant(ctx, tenantID, "")
	}
	ids, err := b.d.Repos.Groups.ListMemberIDs(ctx, tenantID, g.ID)
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// SetMembers adds principals or changes their roles. Re-adding an existing
// member is an update of its role, never an error, so clients can retry.
// The space must keep at least one administrator membership.
func (s *SpaceService) SetMembers(ctx context.Context, actor *acl.Identity, space *model.Space,
	in []MemberInput,
) ([]*MemberView, error) {
	if len(in) == 0 {
		return nil, invalid("members is required")
	}
	if len(in) > MaxBatch {
		return nil, invalid("at most %d members per request", MaxBatch)
	}
	// Last write wins for a principal named twice.
	wanted := map[model.Principal]model.SpaceRole{}
	order := []model.Principal{}
	for _, m := range in {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			return nil, invalid("member id is required")
		}
		if m.Type != model.PrincipalUser && m.Type != model.PrincipalGroup {
			return nil, invalid("member type must be user or group")
		}
		if !m.Role.Valid() || m.Role == model.RoleNone {
			return nil, invalid("role must be reader, writer or admin")
		}
		p := model.Principal{Type: m.Type, ID: id}
		if _, seen := wanted[p]; !seen {
			order = append(order, p)
		}
		wanted[p] = m.Role
	}
	groups, err := s.groupIndex(ctx, actor.TenantID)
	if err != nil {
		return nil, err
	}
	for _, p := range order {
		switch p.Type {
		case model.PrincipalUser:
			active, err := s.activeMember(ctx, actor.TenantID, p.ID)
			if err != nil {
				return nil, err
			}
			if !active {
				return nil, invalid("user %q is not an active member of this workspace", p.ID)
			}
		case model.PrincipalGroup:
			if _, ok := groups[p.ID]; !ok {
				return nil, invalid("group %q was not found in this workspace", p.ID)
			}
		}
	}

	current, err := s.d.Repos.Members.ListBySpace(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	if err := s.checkKeepsAnAdmin(current, wanted, nil); err != nil {
		return nil, err
	}
	existing := map[model.Principal]model.SpaceRole{}
	for _, r := range current {
		existing[r.Principal()] = r.Role
	}

	addedBy := actor.UserID
	err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		for _, p := range order {
			m := &model.SpaceMember{
				SpaceID: space.ID, TenantID: actor.TenantID, PrincipalType: p.Type, PrincipalID: p.ID,
				Role: wanted[p],
			}
			if addedBy != "" {
				m.AddedBy = &addedBy
			}
			if err := tx.Members.Upsert(ctx, m); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceMembersChange, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("count", len(order)))
	for _, p := range order {
		entry := audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: audit.SpaceMemberAdded, SpaceID: space.ID, TargetType: targetTypeFor(p), TargetID: p.ID,
		}
		if old, was := existing[p]; was {
			if old == wanted[p] {
				continue // nothing changed for this principal
			}
			entry.Action = audit.SpaceMemberRoleChanged
		}
		if p.Type == model.PrincipalUser {
			entry.TargetUserID = p.ID
		}
		s.audit(ctx, entry)
	}
	return s.ListMembers(ctx, space)
}

func targetTypeFor(p model.Principal) string {
	if p.Type == model.PrincipalGroup {
		return types.AuditTargetTenantGroup
	}
	return "user"
}

// checkKeepsAnAdmin simulates the change and rejects it if no administrator
// membership would remain. Tenant administrators are implicitly admins of
// every space, but a space with no explicit admin would still be an orphan
// the moment the tenant role changes hands, so the invariant holds for
// everyone.
func (s *SpaceService) checkKeepsAnAdmin(current []*model.SpaceMember, changes map[model.Principal]model.SpaceRole,
	remove *model.Principal,
) error {
	admins := 0
	for _, r := range current {
		p := r.Principal()
		role := r.Role
		if newRole, ok := changes[p]; ok {
			role = newRole
		}
		if remove != nil && p == *remove {
			continue
		}
		if role == model.RoleAdmin {
			admins++
		}
	}
	for p, role := range changes {
		found := false
		for _, r := range current {
			if r.Principal() == p {
				found = true
				break
			}
		}
		if !found && role == model.RoleAdmin {
			admins++
		}
	}
	if admins == 0 {
		return invalid("a space must keep at least one administrator")
	}
	return nil
}

// RemoveMember drops one direct membership.
func (s *SpaceService) RemoveMember(ctx context.Context, actor *acl.Identity, space *model.Space,
	p model.Principal,
) error {
	if p.Type != model.PrincipalUser && p.Type != model.PrincipalGroup {
		return invalid("member type must be user or group")
	}
	current, err := s.d.Repos.Members.ListBySpace(ctx, actor.TenantID, space.ID)
	if err != nil {
		return err
	}
	found := false
	for _, r := range current {
		if r.Principal() == p {
			found = true
			break
		}
	}
	if !found {
		return notFound("member")
	}
	if err := s.checkKeepsAnAdmin(current, nil, &p); err != nil {
		return err
	}
	if err := s.d.Repos.Members.Remove(ctx, actor.TenantID, space.ID, p); err != nil {
		return err
	}
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.SpaceMembersChange, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("removed", string(p.Type)+":"+p.ID))
	entry := audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.SpaceMemberRemoved, SpaceID: space.ID, TargetType: targetTypeFor(p), TargetID: p.ID,
	}
	if p.Type == model.PrincipalUser {
		entry.TargetUserID = p.ID
	}
	s.audit(ctx, entry)
	return nil
}
