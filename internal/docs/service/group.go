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

// GroupService manages tenant-level user groups. Groups are a tenant
// concept (the docs module is their first consumer), so the routes sit
// under /groups and management needs the tenant Admin role, which the
// router enforces; reading is open to every member because pickers need it.
type GroupService struct{ *base }

// GroupView is a group with its member count.
type GroupView struct {
	*model.TenantGroup
	MemberCount int64 `json:"member_count"`
}

// CreateGroupInput is what a client may set when creating a group.
type CreateGroupInput struct {
	Name        string
	Description string
	// MemberIDs are added in the same transaction.
	MemberIDs []string
}

// UpdateGroupInput is a partial update.
type UpdateGroupInput struct {
	Name        *string
	Description *string
}

// List returns the tenant's groups, the default group first, then by name.
// It also makes sure the default group exists, so every tenant sees one
// without a separate bootstrap step.
func (s *GroupService) List(ctx context.Context, actor *acl.Identity) ([]*GroupView, error) {
	if _, err := s.d.Repos.Groups.EnsureDefault(ctx, actor.TenantID, actor.UserID); err != nil {
		return nil, err
	}
	groups, err := s.d.Repos.Groups.List(ctx, actor.TenantID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].IsDefault != groups[j].IsDefault {
			return groups[i].IsDefault
		}
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	out := make([]*GroupView, 0, len(groups))
	for _, g := range groups {
		v, err := s.view(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Get returns one group.
func (s *GroupService) Get(ctx context.Context, actor *acl.Identity, id string) (*GroupView, error) {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, g)
}

func (s *GroupService) view(ctx context.Context, g *model.TenantGroup) (*GroupView, error) {
	count, err := s.memberCount(ctx, g)
	if err != nil {
		return nil, err
	}
	return &GroupView{TenantGroup: g, MemberCount: count}, nil
}

func (s *GroupService) memberCount(ctx context.Context, g *model.TenantGroup) (int64, error) {
	if g.IsDefault {
		if s.d.Members == nil {
			return 0, nil
		}
		return s.d.Members.CountFilteredByTenant(ctx, g.TenantID, "")
	}
	ids, err := s.d.Repos.Groups.ListMemberIDs(ctx, g.TenantID, g.ID)
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// Create makes a group, optionally with initial members.
func (s *GroupService) Create(ctx context.Context, actor *acl.Identity, in CreateGroupInput) (*GroupView, error) {
	name, err := cleanName("name", in.Name)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(name, model.DefaultGroupName) {
		return nil, invalid("%q is reserved for the default group", model.DefaultGroupName)
	}
	desc, err := cleanDescription(in.Description)
	if err != nil {
		return nil, err
	}
	memberIDs, err := s.checkUsers(ctx, actor.TenantID, in.MemberIDs)
	if err != nil {
		return nil, err
	}
	if err := s.checkNameFree(ctx, actor.TenantID, name, ""); err != nil {
		return nil, err
	}
	g := &model.TenantGroup{TenantID: actor.TenantID, Name: name, Description: desc, Source: model.GroupSourceManual}
	if actor.UserID != "" {
		uid := actor.UserID
		g.CreatorID = &uid
	}
	err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		if err := tx.Groups.Create(ctx, g); err != nil {
			return err
		}
		return tx.Groups.AddMembers(ctx, actor.TenantID, g.ID, memberIDs, actor.UserID)
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, conflict("a group named %q already exists", name)
		}
		return nil, err
	}
	s.afterChange(ctx, actor, g.ID, audit.GroupCreated, "")
	return s.view(ctx, g)
}

// checkNameFree rejects a name another live group already uses, compared
// case-insensitively so "Backend" and "backend" cannot coexist.
func (s *GroupService) checkNameFree(ctx context.Context, tenantID uint64, name, exclude string) error {
	groups, err := s.d.Repos.Groups.List(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.ID != exclude && strings.EqualFold(g.Name, name) {
			return conflict("a group named %q already exists", name)
		}
	}
	return nil
}

// checkUsers validates a batch of user IDs: bounded, distinct, all active
// members of the tenant.
func (s *GroupService) checkUsers(ctx context.Context, tenantID uint64, ids []string) ([]string, error) {
	ids = dedupe(ids)
	if len(ids) > MaxBatch {
		return nil, invalid("at most %d users per request", MaxBatch)
	}
	for _, id := range ids {
		active, err := s.activeMember(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, invalid("user %q is not an active member of this workspace", id)
		}
	}
	return ids, nil
}

// Update renames or re-describes a group. The default group keeps its name.
func (s *GroupService) Update(ctx context.Context, actor *acl.Identity, id string,
	in UpdateGroupInput,
) (*GroupView, error) {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if in.Name != nil {
		name, err := cleanName("name", *in.Name)
		if err != nil {
			return nil, err
		}
		if name != g.Name {
			if g.IsDefault {
				return nil, invalid("the default group cannot be renamed")
			}
			if strings.EqualFold(name, model.DefaultGroupName) {
				return nil, invalid("%q is reserved for the default group", model.DefaultGroupName)
			}
			if err := s.checkNameFree(ctx, actor.TenantID, name, g.ID); err != nil {
				return nil, err
			}
			fields["name"] = name
			g.Name = name
		}
	}
	if in.Description != nil {
		desc, err := cleanDescription(*in.Description)
		if err != nil {
			return nil, err
		}
		if desc != g.Description {
			fields["description"] = desc
			g.Description = desc
		}
	}
	if len(fields) == 0 {
		return s.view(ctx, g)
	}
	if err := s.d.Repos.Groups.Update(ctx, actor.TenantID, g.ID, fields); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, conflict("a group named %q already exists", g.Name)
		}
		return nil, err
	}
	s.afterChange(ctx, actor, g.ID, audit.GroupUpdated, "")
	return s.view(ctx, g)
}

// Delete removes a group and every grant it carried: its space memberships
// and page grants go with it in the same transaction, so nobody keeps
// access through a group that no longer exists.
func (s *GroupService) Delete(ctx context.Context, actor *acl.Identity, id string) error {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return invalid("the default group cannot be deleted")
	}
	principal := model.GroupPrincipal(g.ID)
	err = s.d.Repos.Transaction(ctx, func(tx *repository.Repositories) error {
		if err := tx.Members.RemoveAllForPrincipal(ctx, actor.TenantID, principal); err != nil {
			return err
		}
		if err := tx.Access.RemoveAllGrantsForPrincipal(ctx, actor.TenantID, principal); err != nil {
			return err
		}
		return tx.Groups.SoftDelete(ctx, actor.TenantID, g.ID)
	})
	if err != nil {
		return err
	}
	s.afterChange(ctx, actor, g.ID, audit.GroupDeleted, "")
	return nil
}

// MemberPage is one page of a group's members.
type MemberPage struct {
	Members  []UserView `json:"members"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// ListMembers pages through a group's members. The default group lists the
// tenant's active members, since its membership is implicit.
func (s *GroupService) ListMembers(ctx context.Context, actor *acl.Identity, id, query string,
	page, pageSize int,
) (*MemberPage, error) {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query = strings.TrimSpace(query)
	out := &MemberPage{Members: []UserView{}, Page: page, PageSize: pageSize}
	if g.IsDefault {
		if s.d.Members == nil {
			return out, nil
		}
		total, err := s.d.Members.CountFilteredByTenant(ctx, actor.TenantID, query)
		if err != nil {
			return nil, err
		}
		rows, err := s.d.Members.ListPagedByTenant(ctx, actor.TenantID, query, (page-1)*pageSize, pageSize)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		users := s.users(ctx, ids)
		for _, id := range ids {
			out.Members = append(out.Members, userView(id, users))
		}
		out.Total = total
		return out, nil
	}
	ids, err := s.d.Repos.Groups.ListMemberIDs(ctx, actor.TenantID, g.ID)
	if err != nil {
		return nil, err
	}
	users := s.users(ctx, ids)
	views := make([]UserView, 0, len(ids))
	q := strings.ToLower(query)
	for _, id := range ids {
		v := userView(id, users)
		if q != "" && !strings.Contains(strings.ToLower(v.Username), q) &&
			!strings.Contains(strings.ToLower(v.Email), q) {
			continue
		}
		views = append(views, v)
	}
	sort.Slice(views, func(i, j int) bool {
		return strings.ToLower(views[i].Username+views[i].UserID) < strings.ToLower(views[j].Username+views[j].UserID)
	})
	out.Total = int64(len(views))
	start := (page - 1) * pageSize
	if start < len(views) {
		end := start + pageSize
		if end > len(views) {
			end = len(views)
		}
		out.Members = views[start:end]
	}
	return out, nil
}

// AddMembers adds users to a group; users already in it are skipped.
func (s *GroupService) AddMembers(ctx context.Context, actor *acl.Identity, id string, userIDs []string) error {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return invalid("every workspace member already belongs to the default group")
	}
	ids, err := s.checkUsers(ctx, actor.TenantID, userIDs)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return invalid("user_ids is required")
	}
	if err := s.d.Repos.Groups.AddMembers(ctx, actor.TenantID, g.ID, ids, actor.UserID); err != nil {
		return err
	}
	s.afterChange(ctx, actor, g.ID, audit.GroupMembersChanged, "")
	for _, uid := range ids {
		s.audit(ctx, audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: audit.GroupMemberAdded, TargetType: audit.TargetGroup, TargetID: g.ID, TargetUserID: uid,
		})
	}
	return nil
}

// RemoveMember removes one user from a group.
func (s *GroupService) RemoveMember(ctx context.Context, actor *acl.Identity, id, userID string) error {
	g, err := s.d.Repos.Groups.Get(ctx, actor.TenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return invalid("members cannot be removed from the default group")
	}
	ids, err := s.d.Repos.Groups.ListMemberIDs(ctx, actor.TenantID, g.ID)
	if err != nil {
		return err
	}
	found := false
	for _, existing := range ids {
		if existing == userID {
			found = true
			break
		}
	}
	if !found {
		return notFound("group member")
	}
	if err := s.d.Repos.Groups.RemoveMembers(ctx, actor.TenantID, g.ID, []string{userID}); err != nil {
		return err
	}
	s.afterChange(ctx, actor, g.ID, audit.GroupMembersChanged, userID)
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: audit.GroupMemberRemoved, TargetType: audit.TargetGroup, TargetID: g.ID, TargetUserID: userID,
	})
	return nil
}

// afterChange runs the post-commit trio for a group change. Group changes
// alter who holds which grants everywhere, so the whole tenant's cache goes.
func (s *GroupService) afterChange(ctx context.Context, actor *acl.Identity, groupID string,
	action types.AuditAction, targetUserID string,
) {
	s.invalidate(ctx, actor.TenantID)
	s.publish(ctx, events.New(events.GroupChanged, actor.TenantID).WithActor(actor.UserID).
		With("group_id", groupID).With("action", string(action)))
	if action == audit.GroupMembersChanged {
		return // per-user rows are written by the caller
	}
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
		Action: action, TargetType: audit.TargetGroup, TargetID: groupID, TargetUserID: targetUserID,
	})
}
