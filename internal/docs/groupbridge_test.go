package docs

import (
	"context"
	"fmt"
	"sync"
	"testing"

	appservice "github.com/magicyuan876/yuheng/internal/application/service"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// bridgeMembers is the workspace membership for the bridge test: every
// listed user is an active member of tenant 1. It serves the permission
// resolver (TenantMemberGetter), the docs services (TenantMembers) and the
// group service (TenantMemberRepository); the methods the group service
// never reaches stay on the embedded nil interface.
type bridgeMembers struct {
	interfaces.TenantMemberRepository
	roles map[string]types.TenantRole
}

func newBridgeMembers() *bridgeMembers {
	return &bridgeMembers{roles: map[string]types.TenantRole{
		"owner": types.TenantRoleOwner, "alice": types.TenantRoleContributor,
		"bob": types.TenantRoleContributor, "carol": types.TenantRoleContributor,
	}}
}

func (m *bridgeMembers) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	role, ok := m.roles[userID]
	if !ok || tenantID != 1 {
		return nil, nil
	}
	return &types.TenantMember{
		UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive,
	}, nil
}

func (m *bridgeMembers) CountFilteredByTenant(context.Context, uint64, string) (int64, error) {
	return int64(len(m.roles)), nil
}

func (m *bridgeMembers) ListPagedByTenant(context.Context, uint64, string, int, int) ([]*types.TenantMember, error) {
	return nil, nil
}

// bridgeUsers resolves ids to display rows; the test does not look at them.
type bridgeUsers struct{ interfaces.UserRepository }

func (bridgeUsers) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	out := map[string]*types.User{}
	for _, id := range ids {
		out[id] = &types.User{ID: id, Username: id, Email: id + "@example.test"}
	}
	return out, nil
}

// bridgeAudit captures the audit rows the group service writes.
type bridgeAudit struct {
	interfaces.AuditLogService
	mu      sync.Mutex
	actions []types.AuditAction
}

func (a *bridgeAudit) Log(_ context.Context, row *types.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, row.Action)
	return nil
}

func (a *bridgeAudit) recorded() []types.AuditAction {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]types.AuditAction(nil), a.actions...)
}

// asOwner is a request context as the auth middleware would leave it for
// the workspace owner, which is who changes groups.
func asOwner() context.Context {
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
}

func httpCode(t *testing.T, err error) int {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr, "expected AppError, got %T: %v", err, err)
	return appErr.HTTPCode
}

// The acceptance this module was built against (T1.1): a group membership
// change is reflected in permission decisions immediately, and a deleted
// group takes its grants with it. Groups are now managed by the application
// layer, so the test wires the real group service to the bridge and checks
// the effect through the resolver, exactly as the router does.
func TestGroupBridgeKeepsGrantsAndCacheInStepWithGroups(t *testing.T) {
	db := pgtest.New(t)
	repos := repository.New(db)
	members := newBridgeMembers()
	resolver := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(members), acl.WithCache(acl.NewMemoryCache(0)))
	bus := events.NewMemoryBus()
	var mu sync.Mutex
	var seen []events.Type
	bus.Subscribe(0, func(e events.Event) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, e.Type)
	})
	auditLog := &bridgeAudit{}
	groups := appservice.NewTenantGroupService(repos.Groups, members, bridgeUsers{}, auditLog)
	groups.RegisterDependent(newGroupBridge(repos, resolver, bus))
	svc := service.New(service.Deps{
		Repos: repos, Resolver: resolver, Bus: bus, Audit: audit.NewRecorder(nil),
		Users: bridgeUsers{}, Members: members,
	})

	identity := func(user string) *acl.Identity {
		id, err := resolver.Identity(context.Background(), 1, user)
		require.NoError(t, err)
		return id
	}
	// spaceRole resolves the user's effective role from scratch, identity
	// included, so a stale cache would show up as a wrong answer.
	spaceRole := func(user string, sp *model.Space) model.SpaceRole {
		role, err := resolver.SpaceRole(context.Background(), identity(user), sp)
		require.NoError(t, err)
		return role
	}

	alice := identity("alice")
	sp, err := svc.Spaces.Create(context.Background(), alice, service.CreateSpaceInput{Name: "Team"})
	require.NoError(t, err)
	g, err := groups.Create(asOwner(), 1, types.CreateTenantGroupInput{Name: "Backend"})
	require.NoError(t, err)
	_, err = svc.Spaces.SetMembers(context.Background(), alice, sp.Space, []service.MemberInput{
		{Type: model.PrincipalGroup, ID: g.ID, Role: model.RoleWriter},
	})
	require.NoError(t, err)

	require.Equal(t, model.RoleNone, spaceRole("carol", sp.Space), "warm the cache with a negative answer")
	require.NoError(t, groups.AddMembers(asOwner(), 1, g.ID, []string{"carol", "carol"}))
	require.Equal(t, model.RoleWriter, spaceRole("carol", sp.Space), "the grant reaches carol at once")
	view, err := groups.Get(asOwner(), 1, g.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, view.MemberCount, "duplicates in one request collapse")

	require.NoError(t, groups.RemoveMember(asOwner(), 1, g.ID, "carol"))
	require.Equal(t, model.RoleNone, spaceRole("carol", sp.Space), "removal reaches carol at once")
	require.Equal(t, 404, httpCode(t, groups.RemoveMember(asOwner(), 1, g.ID, "carol")))

	// Deleting the group removes the space membership it carried, in the
	// same transaction, so bob loses the access he had through it.
	require.NoError(t, groups.AddMembers(asOwner(), 1, g.ID, []string{"bob"}))
	require.Equal(t, model.RoleWriter, spaceRole("bob", sp.Space))
	require.NoError(t, groups.Delete(asOwner(), 1, g.ID))
	require.Equal(t, model.RoleNone, spaceRole("bob", sp.Space))
	rows, err := svc.Spaces.ListMembers(context.Background(), sp.Space)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the group's membership row is gone; alice's stays")

	mu.Lock()
	require.Contains(t, seen, events.GroupChanged, "the live-update stream hears about group changes")
	mu.Unlock()
	for _, a := range []types.AuditAction{
		types.AuditActionGroupCreated, types.AuditActionGroupMemberAdded,
		types.AuditActionGroupMemberRemoved, types.AuditActionGroupDeleted,
	} {
		require.Contains(t, auditLog.recorded(), a, fmt.Sprintf("audit row %s", a))
	}
}

// A deletion that fails inside a dependent leaves the group in place: the
// grants and the row go together or not at all.
func TestGroupBridgeDeletionIsAtomicWithTheGrants(t *testing.T) {
	db := pgtest.New(t)
	repos := repository.New(db)
	members := newBridgeMembers()
	resolver := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(members), acl.WithCache(acl.NewMemoryCache(0)))
	groups := appservice.NewTenantGroupService(repos.Groups, members, bridgeUsers{}, nil)
	groups.RegisterDependent(newGroupBridge(repos, resolver, nil))
	groups.RegisterDependent(failingDependent{})

	g, err := groups.Create(asOwner(), 1, types.CreateTenantGroupInput{Name: "Backend", MemberIDs: []string{"bob"}})
	require.NoError(t, err)
	require.Error(t, groups.Delete(asOwner(), 1, g.ID))
	again, err := groups.Get(asOwner(), 1, g.ID)
	require.NoError(t, err, "the group survived the failed deletion")
	require.EqualValues(t, 1, again.MemberCount)
}

// failingDependent refuses every deletion, standing in for a module whose
// grant cleanup fails half-way.
type failingDependent struct{}

func (failingDependent) RemoveGroupGrants(context.Context, *gorm.DB, uint64, string) error {
	return fmt.Errorf("dependent refused")
}

func (failingDependent) GroupChanged(context.Context, types.TenantGroupChange) {}
