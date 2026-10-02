package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

// Limits on what a group request may carry.
const (
	// tenantGroupMaxNameRunes bounds a group name; the column holds 128.
	tenantGroupMaxNameRunes = 100
	// tenantGroupMaxDescriptionRunes bounds the free-text description.
	tenantGroupMaxDescriptionRunes = 4000
	// tenantGroupMaxBatch caps the users one membership request may name.
	tenantGroupMaxBatch = 200

	listGroupMembersDefaultPageSize = 20
	listGroupMembersMaxPageSize     = 100
)

// tenantGroupService implements interfaces.TenantGroupService.
//
// Every mutation follows one shape: validate, write in one transaction, then
// record the audit rows and tell the dependents, in that order, so a failed
// write leaves no trace and nothing is told about a change that did not
// happen.
type tenantGroupService struct {
	repo    interfaces.TenantGroupRepository
	members interfaces.TenantMemberRepository
	users   interfaces.UserRepository
	audit   interfaces.AuditLogService // optional; nil ⇒ no audit, the operations still succeed

	mu         sync.RWMutex
	dependents []interfaces.TenantGroupDependent
}

// NewTenantGroupService constructs the service. The audit service is
// optional, like everywhere else in the application layer: a group change
// must not fail because the audit table is unavailable.
func NewTenantGroupService(
	repo interfaces.TenantGroupRepository,
	members interfaces.TenantMemberRepository,
	users interfaces.UserRepository,
	audit interfaces.AuditLogService,
) interfaces.TenantGroupService {
	return &tenantGroupService{repo: repo, members: members, users: users, audit: audit}
}

func (s *tenantGroupService) RegisterDependent(d interfaces.TenantGroupDependent) {
	if d == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dependents = append(s.dependents, d)
}

func (s *tenantGroupService) snapshotDependents() []interfaces.TenantGroupDependent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]interfaces.TenantGroupDependent(nil), s.dependents...)
}

func (s *tenantGroupService) List(ctx context.Context, tenantID uint64) ([]*types.TenantGroupView, error) {
	if _, err := s.repo.EnsureDefault(ctx, tenantID, attributableUser(ctx)); err != nil {
		return nil, err
	}
	groups, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	// The repository orders by name as the database collates it; the
	// listing is for people, so order case-insensitively here.
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].IsDefault != groups[j].IsDefault {
			return groups[i].IsDefault
		}
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	out := make([]*types.TenantGroupView, 0, len(groups))
	for _, g := range groups {
		v, err := s.view(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *tenantGroupService) Get(ctx context.Context, tenantID uint64, id string) (*types.TenantGroupView, error) {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, g)
}

// get loads a live group, translating the repository's sentinel.
func (s *tenantGroupService) get(ctx context.Context, tenantID uint64, id string) (*types.TenantGroup, error) {
	g, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, translateGroupError(err, "")
	}
	return g, nil
}

func (s *tenantGroupService) view(ctx context.Context, g *types.TenantGroup) (*types.TenantGroupView, error) {
	count, err := s.memberCount(ctx, g)
	if err != nil {
		return nil, err
	}
	return &types.TenantGroupView{TenantGroup: g, MemberCount: count}, nil
}

// memberCount counts a group's members; the default group counts every
// active workspace member because its membership is implicit.
func (s *tenantGroupService) memberCount(ctx context.Context, g *types.TenantGroup) (int64, error) {
	if g.IsDefault {
		return s.members.CountFilteredByTenant(ctx, g.TenantID, "")
	}
	ids, err := s.repo.ListMemberIDs(ctx, g.TenantID, g.ID)
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (s *tenantGroupService) Create(ctx context.Context, tenantID uint64,
	in types.CreateTenantGroupInput,
) (*types.TenantGroupView, error) {
	name, err := cleanGroupName(in.Name)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(name, types.DefaultTenantGroupName) {
		return nil, apperrors.NewValidationError(
			fmt.Sprintf("%q is reserved for the default group", types.DefaultTenantGroupName))
	}
	desc, err := cleanGroupDescription(in.Description)
	if err != nil {
		return nil, err
	}
	memberIDs, err := s.checkUsers(ctx, tenantID, in.MemberIDs)
	if err != nil {
		return nil, err
	}
	if err := s.checkNameFree(ctx, tenantID, name, ""); err != nil {
		return nil, err
	}
	actor := attributableUser(ctx)
	g := &types.TenantGroup{TenantID: tenantID, Name: name, Description: desc, Source: types.TenantGroupSourceManual}
	if actor != "" {
		g.CreatorID = &actor
	}
	if err := s.repo.Create(ctx, g, memberIDs, actor); err != nil {
		return nil, translateGroupError(err, name)
	}
	s.afterChange(ctx, g, types.AuditActionGroupCreated, "")
	return s.view(ctx, g)
}

// checkNameFree rejects a name another live group already uses, compared
// case-insensitively so "Backend" and "backend" cannot coexist. The unique
// index behind it is case-sensitive, which is why this is a query and not
// only an error translation.
func (s *tenantGroupService) checkNameFree(ctx context.Context, tenantID uint64, name, exclude string) error {
	groups, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.ID != exclude && strings.EqualFold(g.Name, name) {
			return apperrors.NewConflictError(fmt.Sprintf("a group named %q already exists", name))
		}
	}
	return nil
}

// checkUsers validates a batch of user ids: bounded, distinct, all active
// members of the workspace. A group may only ever contain members, or a
// grant to it would reach somebody the workspace never admitted.
func (s *tenantGroupService) checkUsers(ctx context.Context, tenantID uint64, ids []string) ([]string, error) {
	ids = distinctTrimmed(ids)
	if len(ids) > tenantGroupMaxBatch {
		return nil, apperrors.NewValidationError(fmt.Sprintf("at most %d users per request", tenantGroupMaxBatch))
	}
	for _, id := range ids {
		m, err := s.members.Get(ctx, id, tenantID)
		if err != nil {
			return nil, err
		}
		if m == nil || m.Status != types.TenantMemberStatusActive {
			return nil, apperrors.NewValidationError(
				fmt.Sprintf("user %q is not an active member of this workspace", id))
		}
	}
	return ids, nil
}

func (s *tenantGroupService) Update(ctx context.Context, tenantID uint64, id string,
	in types.UpdateTenantGroupInput,
) (*types.TenantGroupView, error) {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if in.Name != nil {
		name, err := cleanGroupName(*in.Name)
		if err != nil {
			return nil, err
		}
		if name != g.Name {
			if g.IsDefault {
				return nil, apperrors.NewValidationError("the default group cannot be renamed")
			}
			if strings.EqualFold(name, types.DefaultTenantGroupName) {
				return nil, apperrors.NewValidationError(
					fmt.Sprintf("%q is reserved for the default group", types.DefaultTenantGroupName))
			}
			if err := s.checkNameFree(ctx, tenantID, name, g.ID); err != nil {
				return nil, err
			}
			fields["name"] = name
			g.Name = name
		}
	}
	if in.Description != nil {
		desc, err := cleanGroupDescription(*in.Description)
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
	if err := s.repo.Update(ctx, tenantID, g.ID, fields); err != nil {
		return nil, translateGroupError(err, g.Name)
	}
	s.afterChange(ctx, g, types.AuditActionGroupUpdated, "")
	return s.view(ctx, g)
}

func (s *tenantGroupService) Delete(ctx context.Context, tenantID uint64, id string) error {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return apperrors.NewValidationError("the default group cannot be deleted")
	}
	// The dependents' grants go in the deleting transaction, so nobody
	// keeps access through a group that no longer exists -- not even for
	// the instant between two statements.
	dependents := s.snapshotDependents()
	err = s.repo.SoftDelete(ctx, tenantID, g.ID, func(tx *gorm.DB) error {
		for _, d := range dependents {
			if err := d.RemoveGroupGrants(ctx, tx, tenantID, g.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return translateGroupError(err, "")
	}
	s.afterChange(ctx, g, types.AuditActionGroupDeleted, "")
	return nil
}

func (s *tenantGroupService) ListMembers(ctx context.Context, tenantID uint64, id, query string,
	page, pageSize int,
) (*types.TenantGroupMemberPage, error) {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > listGroupMembersMaxPageSize {
		pageSize = listGroupMembersDefaultPageSize
	}
	query = strings.TrimSpace(query)
	out := &types.TenantGroupMemberPage{Members: []types.TenantGroupMemberView{}, Page: page, PageSize: pageSize}
	if g.IsDefault {
		// Implicit membership: page through the member list itself, whose
		// repository already filters and pages.
		total, err := s.members.CountFilteredByTenant(ctx, tenantID, query)
		if err != nil {
			return nil, err
		}
		rows, err := s.members.ListPagedByTenant(ctx, tenantID, query, (page-1)*pageSize, pageSize)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		users := s.lookupUsers(ctx, ids)
		for _, uid := range ids {
			out.Members = append(out.Members, memberView(uid, users))
		}
		out.Total = total
		return out, nil
	}
	// Explicit membership is bounded (tenantGroupMaxBatch per request), so
	// filtering and paging in memory over the hydrated rows is fine and
	// lets the filter see usernames and emails the membership table lacks.
	ids, err := s.repo.ListMemberIDs(ctx, tenantID, g.ID)
	if err != nil {
		return nil, err
	}
	users := s.lookupUsers(ctx, ids)
	views := make([]types.TenantGroupMemberView, 0, len(ids))
	q := strings.ToLower(query)
	for _, uid := range ids {
		v := memberView(uid, users)
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

func (s *tenantGroupService) AddMembers(ctx context.Context, tenantID uint64, id string, userIDs []string) error {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return apperrors.NewValidationError("every workspace member already belongs to the default group")
	}
	ids, err := s.checkUsers(ctx, tenantID, userIDs)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return apperrors.NewValidationError("user_ids is required")
	}
	if err := s.repo.AddMembers(ctx, tenantID, g.ID, ids, attributableUser(ctx)); err != nil {
		return translateGroupError(err, "")
	}
	// One audit row per user, so the log answers "who was put in which
	// group"; the dependents hear once, since what they drop is per group.
	for _, uid := range ids {
		s.recordAudit(ctx, g, types.AuditActionGroupMemberAdded, uid)
	}
	s.notifyDependents(ctx, g, types.AuditActionGroupMemberAdded)
	return nil
}

func (s *tenantGroupService) RemoveMember(ctx context.Context, tenantID uint64, id, userID string) error {
	g, err := s.get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if g.IsDefault {
		return apperrors.NewValidationError("members cannot be removed from the default group")
	}
	ids, err := s.repo.ListMemberIDs(ctx, tenantID, g.ID)
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
		return apperrors.NewNotFoundError("group member not found")
	}
	if err := s.repo.RemoveMembers(ctx, tenantID, g.ID, []string{userID}); err != nil {
		return err
	}
	s.afterChange(ctx, g, types.AuditActionGroupMemberRemoved, userID)
	return nil
}

// afterChange is the post-commit pair for a change recorded as one audit
// row: the row, then the dependents.
func (s *tenantGroupService) afterChange(ctx context.Context, g *types.TenantGroup, action types.AuditAction,
	targetUserID string,
) {
	s.recordAudit(ctx, g, action, targetUserID)
	s.notifyDependents(ctx, g, action)
}

// recordAudit writes one audit row. Best-effort, like the member service's:
// the change has committed, and an audit outage must not fail the request
// -- the audit service logs its own failures.
func (s *tenantGroupService) recordAudit(ctx context.Context, g *types.TenantGroup, action types.AuditAction,
	targetUserID string,
) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(ctx, &types.AuditLog{
		TenantID:     g.TenantID,
		ActorUserID:  auditActor(ctx),
		ActorRole:    auditActorRole(ctx, g.TenantID),
		Action:       action,
		TargetType:   types.AuditTargetTenantGroup,
		TargetID:     g.ID,
		TargetUserID: targetUserID,
	})
}

func (s *tenantGroupService) notifyDependents(ctx context.Context, g *types.TenantGroup, action types.AuditAction) {
	change := types.TenantGroupChange{TenantID: g.TenantID, GroupID: g.ID, ActorUserID: auditActor(ctx), Action: action}
	for _, d := range s.snapshotDependents() {
		d.GroupChanged(ctx, change)
	}
}

// lookupUsers hydrates user ids into display rows; a directory failure
// degrades to bare ids rather than failing the request.
func (s *tenantGroupService) lookupUsers(ctx context.Context, ids []string) map[string]*types.User {
	if len(ids) == 0 {
		return map[string]*types.User{}
	}
	users, err := s.users.GetUsersByIDs(ctx, ids)
	if err != nil {
		logger.Warnf(ctx, "[tenant_group] user lookup failed: %v", err)
		return map[string]*types.User{}
	}
	return users
}

func memberView(id string, users map[string]*types.User) types.TenantGroupMemberView {
	v := types.TenantGroupMemberView{UserID: id}
	if u, ok := users[id]; ok && u != nil {
		v.Username, v.Email, v.Avatar = u.Username, u.Email, u.Avatar
	}
	return v
}

// attributableUser is the caller to record as a group's creator or a
// membership's adder: the human user, or nobody. An API key acts as the
// synthetic "system-<tenant>" user (see types.IsSyntheticUserID), and a
// column that joins users must not point at an account that does not
// exist -- the same rule tenant invitations follow for invited_by.
func attributableUser(ctx context.Context) string {
	uid := auditActor(ctx)
	if types.IsSyntheticUserID(uid) {
		return ""
	}
	return uid
}

// translateGroupError maps the repository's sentinels to API errors. name
// is the name a duplicate complaint should quote; empty when the operation
// did not set one.
func translateGroupError(err error, name string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, apprepo.ErrTenantGroupNotFound):
		return apperrors.NewNotFoundError("group not found")
	case errors.Is(err, apprepo.ErrTenantGroupNameTaken):
		if name == "" {
			return apperrors.NewConflictError("a group with that name already exists")
		}
		return apperrors.NewConflictError(fmt.Sprintf("a group named %q already exists", name))
	}
	return err
}

// cleanGroupName trims and validates a group name: present, bounded, one
// line.
func cleanGroupName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", apperrors.NewValidationError("name is required")
	}
	if utf8.RuneCountInString(name) > tenantGroupMaxNameRunes {
		return "", apperrors.NewValidationError(
			fmt.Sprintf("name must be at most %d characters", tenantGroupMaxNameRunes))
	}
	if strings.ContainsAny(name, "\n\r\t") {
		return "", apperrors.NewValidationError("name must be a single line")
	}
	return name, nil
}

func cleanGroupDescription(raw string) (string, error) {
	desc := strings.TrimSpace(raw)
	if utf8.RuneCountInString(desc) > tenantGroupMaxDescriptionRunes {
		return "", apperrors.NewValidationError(
			fmt.Sprintf("description must be at most %d characters", tenantGroupMaxDescriptionRunes))
	}
	return desc, nil
}

// distinctTrimmed returns the distinct non-empty strings in order of first
// appearance.
func distinctTrimmed(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
