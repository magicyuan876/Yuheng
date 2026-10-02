package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"

	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// groupTestMembers is the workspace membership table ("user" -> role, all
// active in tenant 1) and the user directory. Only the methods the group
// service reaches are implemented; the rest stay on the embedded nil
// interfaces so that an unexpected call fails loudly.
type groupTestMembers struct {
	interfaces.TenantMemberRepository
	interfaces.UserRepository
	roles map[string]types.TenantRole
}

func newGroupTestMembers() *groupTestMembers {
	return &groupTestMembers{roles: map[string]types.TenantRole{
		"owner": types.TenantRoleOwner, "admin": types.TenantRoleAdmin,
		"alice": types.TenantRoleContributor, "bob": types.TenantRoleContributor,
		"carol": types.TenantRoleContributor, "viewer": types.TenantRoleViewer,
		"reviewer-01": types.TenantRoleContributor,
	}}
}

func (m *groupTestMembers) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	role, ok := m.roles[userID]
	if !ok || tenantID != 1 {
		return nil, nil
	}
	return &types.TenantMember{
		UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive,
	}, nil
}

// sorted lists the members matching search, in a stable order, the way
// the real repository pages them.
func (m *groupTestMembers) sorted(search string) []string {
	var out []string
	for user := range m.roles {
		if search == "" || strings.Contains(user, search) {
			out = append(out, user)
		}
	}
	sort.Strings(out)
	return out
}

func (m *groupTestMembers) ListPagedByTenant(_ context.Context, _ uint64, search string, offset,
	limit int,
) ([]*types.TenantMember, error) {
	users := m.sorted(search)
	if offset >= len(users) {
		return nil, nil
	}
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	out := make([]*types.TenantMember, 0, end-offset)
	for _, user := range users[offset:end] {
		out = append(out, &types.TenantMember{UserID: user, TenantID: 1, Role: m.roles[user]})
	}
	return out, nil
}

func (m *groupTestMembers) CountFilteredByTenant(_ context.Context, _ uint64, search string) (int64, error) {
	return int64(len(m.sorted(search))), nil
}

func (m *groupTestMembers) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	out := map[string]*types.User{}
	for _, id := range ids {
		if _, ok := m.roles[id]; ok {
			out[id] = &types.User{ID: id, Username: id, Email: id + "@example.test"}
		}
	}
	return out, nil
}

// groupTestAudit captures audit rows.
type groupTestAudit struct {
	interfaces.AuditLogService
	mu   sync.Mutex
	rows []*types.AuditLog
}

func (a *groupTestAudit) Log(_ context.Context, row *types.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, row)
	return nil
}

func (a *groupTestAudit) snapshot() []*types.AuditLog {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*types.AuditLog(nil), a.rows...)
}

// recordingDependent remembers what it was told, and whether its grant
// cleanup ran inside a transaction.
type recordingDependent struct {
	mu      sync.Mutex
	changes []types.TenantGroupChange
	removed []string
}

func (d *recordingDependent) RemoveGroupGrants(_ context.Context, tx *gorm.DB, _ uint64, groupID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if tx == nil {
		panic("RemoveGroupGrants must run inside the deleting transaction")
	}
	d.removed = append(d.removed, groupID)
	return nil
}

func (d *recordingDependent) GroupChanged(_ context.Context, change types.TenantGroupChange) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.changes = append(d.changes, change)
}

func (d *recordingDependent) actions() []types.AuditAction {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]types.AuditAction, 0, len(d.changes))
	for _, c := range d.changes {
		out = append(out, c.Action)
	}
	return out
}

func newGroupServiceForTest(t *testing.T) (interfaces.TenantGroupService, *groupTestAudit, *recordingDependent) {
	t.Helper()
	members := newGroupTestMembers()
	auditLog := &groupTestAudit{}
	dep := &recordingDependent{}
	svc := NewTenantGroupService(apprepo.NewTenantGroupRepository(pgtest.New(t)), members, members, auditLog)
	svc.RegisterDependent(dep)
	return svc, auditLog, dep
}

// asGroupAdmin is a request context as the auth middleware leaves it for a
// workspace administrator.
func asGroupAdmin() context.Context {
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "admin")
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
}

func groupHTTPCode(t *testing.T, err error) int {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr, "expected AppError, got %T: %v", err, err)
	return appErr.HTTPCode
}

func strp(s string) *string { return &s }

func TestTenantGroupDefaultGroupRules(t *testing.T) {
	svc, _, _ := newGroupServiceForTest(t)
	ctx := asGroupAdmin()

	groups, err := svc.List(ctx, 1)
	require.NoError(t, err)
	require.Len(t, groups, 1, "listing creates the default group on first use")
	def := groups[0]
	require.True(t, def.IsDefault)
	require.Equal(t, types.DefaultTenantGroupName, def.Name)
	require.EqualValues(t, 7, def.MemberCount, "every active workspace member is implicitly in it")

	_, err = svc.Update(ctx, 1, def.ID, types.UpdateTenantGroupInput{Name: strp("staff")})
	require.Equal(t, 400, groupHTTPCode(t, err), "the default group keeps its name")
	v, err := svc.Update(ctx, 1, def.ID, types.UpdateTenantGroupInput{Description: strp("all hands")})
	require.NoError(t, err, "but may be described")
	require.Equal(t, "all hands", v.Description)
	require.Equal(t, 400, groupHTTPCode(t, svc.Delete(ctx, 1, def.ID)))
	require.Equal(t, 400, groupHTTPCode(t, svc.AddMembers(ctx, 1, def.ID, []string{"bob"})))
	require.Equal(t, 400, groupHTTPCode(t, svc.RemoveMember(ctx, 1, def.ID, "bob")))

	page, err := svc.ListMembers(ctx, 1, def.ID, "", 1, 4)
	require.NoError(t, err)
	require.EqualValues(t, 7, page.Total)
	require.Len(t, page.Members, 4)
	page, err = svc.ListMembers(ctx, 1, def.ID, "bob", 1, 20)
	require.NoError(t, err)
	require.Len(t, page.Members, 1)
	require.Equal(t, "bob@example.test", page.Members[0].Email)
}

func TestTenantGroupNamesMembersAndPaging(t *testing.T) {
	svc, auditLog, dep := newGroupServiceForTest(t)
	ctx := asGroupAdmin()

	// Names: reserved, unique (case-insensitive), bounded.
	_, err := svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: "Everyone"})
	require.Equal(t, 400, groupHTTPCode(t, err))
	_, err = svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: strings.Repeat("x", tenantGroupMaxNameRunes+1)})
	require.Equal(t, 400, groupHTTPCode(t, err))
	g, err := svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: " Backend ", Description: "svc"})
	require.NoError(t, err)
	require.Equal(t, "Backend", g.Name, "names are trimmed")
	require.Equal(t, "admin", *g.CreatorID, "the human caller is recorded as the creator")
	_, err = svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: "backend"})
	require.Equal(t, 409, groupHTTPCode(t, err))
	_, err = svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: "Frontend", MemberIDs: []string{"ghost"}})
	require.Equal(t, 400, groupHTTPCode(t, err), "initial members must be workspace members")
	renamed, err := svc.Update(ctx, 1, g.ID, types.UpdateTenantGroupInput{Name: strp("Platform")})
	require.NoError(t, err)
	require.Equal(t, "Platform", renamed.Name)

	// Member paging of an explicit group, with search.
	require.NoError(t, svc.AddMembers(ctx, 1, g.ID, []string{"alice", "bob", "carol", "carol"}))
	view, err := svc.Get(ctx, 1, g.ID)
	require.NoError(t, err)
	require.EqualValues(t, 3, view.MemberCount, "duplicates in one request collapse")
	page, err := svc.ListMembers(ctx, 1, g.ID, "", 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, page.Members, 1)
	require.Equal(t, "carol", page.Members[0].UserID)
	page, err = svc.ListMembers(ctx, 1, g.ID, "ALICE@", 1, 20)
	require.NoError(t, err)
	require.Len(t, page.Members, 1)

	groups, err := svc.List(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []string{types.DefaultTenantGroupName, "Platform"}, []string{groups[0].Name, groups[1].Name})
	require.EqualValues(t, 3, groups[1].MemberCount)

	require.NoError(t, svc.RemoveMember(ctx, 1, g.ID, "carol"))
	require.Equal(t, 404, groupHTTPCode(t, svc.RemoveMember(ctx, 1, g.ID, "carol")))
	require.Equal(t, 404, groupHTTPCode(t, svc.RemoveMember(ctx, 1, "no-such-group", "carol")))

	require.NoError(t, svc.Delete(ctx, 1, g.ID))
	_, err = svc.Get(ctx, 1, g.ID)
	require.Equal(t, 404, groupHTTPCode(t, err))
	require.Equal(t, []string{g.ID}, dep.removed, "the dependent's grants were removed in the deleting transaction")

	// The audit trail: one row per member added, then the removal and the
	// deletion; the dependents heard about each committed change.
	var actions []types.AuditAction
	var memberTargets []string
	for _, row := range auditLog.snapshot() {
		actions = append(actions, row.Action)
		require.Equal(t, types.AuditTargetTenantGroup, row.TargetType)
		require.Equal(t, "admin", row.ActorUserID)
		require.Equal(t, string(types.TenantRoleAdmin), row.ActorRole)
		if row.Action == types.AuditActionGroupMemberAdded {
			memberTargets = append(memberTargets, row.TargetUserID)
		}
	}
	require.Equal(t, []types.AuditAction{
		types.AuditActionGroupCreated, types.AuditActionGroupUpdated,
		types.AuditActionGroupMemberAdded, types.AuditActionGroupMemberAdded, types.AuditActionGroupMemberAdded,
		types.AuditActionGroupMemberRemoved, types.AuditActionGroupDeleted,
	}, actions)
	require.Equal(t, []string{"alice", "bob", "carol"}, memberTargets)
	require.Equal(t, []types.AuditAction{
		types.AuditActionGroupCreated, types.AuditActionGroupUpdated, types.AuditActionGroupMemberAdded,
		types.AuditActionGroupMemberRemoved, types.AuditActionGroupDeleted,
	}, dep.actions(), "dependents hear once per change, not once per member")
}

// An API key acts as the synthetic "system-<tenant>" user. It may manage
// groups, but it is not an account and must not be recorded as one.
func TestTenantGroupDoesNotAttributeToSyntheticUsers(t *testing.T) {
	svc, auditLog, _ := newGroupServiceForTest(t)
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "system-1")

	g, err := svc.Create(ctx, 1, types.CreateTenantGroupInput{Name: "Automation", MemberIDs: []string{"bob"}})
	require.NoError(t, err)
	require.Nil(t, g.CreatorID, "no creator rather than a user that does not exist")
	rows := auditLog.snapshot()
	require.NotEmpty(t, rows)
	require.Equal(t, "system-1", rows[0].ActorUserID, "the audit row still says who acted")
}
