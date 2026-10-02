package service

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// activeTenantUserRepo records preference writes so the tests can tell
// whether ResolveActiveTenantID persisted its fallback choice.
type activeTenantUserRepo struct {
	interfaces.UserRepository
	users   map[string]*types.User
	updates []uint64 // LastActiveTenantID after each UpdateUser (0 = nil)
}

func (r *activeTenantUserRepo) GetUserByID(_ context.Context, id string) (*types.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (r *activeTenantUserRepo) UpdateUser(_ context.Context, user *types.User) error {
	pref := uint64(0)
	if user.Preferences.LastActiveTenantID != nil {
		pref = *user.Preferences.LastActiveTenantID
	}
	r.updates = append(r.updates, pref)
	if r.users != nil {
		cp := *user
		r.users[user.ID] = &cp
	}
	return nil
}

// activeTenantTenantService knows a fixed set of workspaces; anything else
// is "deleted".
type activeTenantTenantService struct {
	interfaces.TenantService
	existing map[uint64]string
}

func (s *activeTenantTenantService) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	name, ok := s.existing[id]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	return &types.Tenant{ID: id, Name: name}, nil
}

func (s *activeTenantTenantService) GetTenantsByIDs(_ context.Context, ids []uint64) (map[uint64]*types.Tenant, error) {
	out := map[uint64]*types.Tenant{}
	for _, id := range ids {
		if name, ok := s.existing[id]; ok {
			out[id] = &types.Tenant{ID: id, Name: name}
		}
	}
	return out, nil
}

// newActiveTenantService wires a userService over the real membership
// service (on its in-memory repository) so membership status, join order and
// soft deletion behave as in production.
func newActiveTenantService(
	user *types.User, existing map[uint64]string,
) (*userService, *activeTenantUserRepo, interfaces.TenantMemberService) {
	members, _ := newServiceWithRepo()
	repo := &activeTenantUserRepo{users: map[string]*types.User{user.ID: user}}
	svc := &userService{
		userRepo:      repo,
		tenantService: &activeTenantTenantService{existing: existing},
		memberService: members,
	}
	return svc, repo, members
}

func pref(id uint64) *uint64 { return &id }

func TestResolveActiveTenantIDHonoursAValidPreference(t *testing.T) {
	ctx := context.Background()
	user := &types.User{ID: "u1", Preferences: types.UserPreferences{LastActiveTenantID: pref(20)}}
	svc, repo, members := newActiveTenantService(user, map[uint64]string{10: "First", 20: "Second"})
	if _, err := members.AddMember(ctx, "u1", 10, types.TenantRoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := members.AddMember(ctx, "u1", 20, types.TenantRoleContributor, nil); err != nil {
		t.Fatal(err)
	}

	if got := svc.ResolveActiveTenantID(ctx, user); got != 20 {
		t.Fatalf("resolved %d, want the preferred workspace 20", got)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("a valid preference must not be rewritten, got updates %v", repo.updates)
	}
}

func TestResolveActiveTenantIDFallsBackToEarliestMembershipAndRewritesThePreference(t *testing.T) {
	ctx := context.Background()
	// The preference points at workspace 99, where the membership is gone.
	user := &types.User{ID: "u1", Preferences: types.UserPreferences{LastActiveTenantID: pref(99)}}
	svc, repo, members := newActiveTenantService(user, map[uint64]string{10: "First", 20: "Second", 99: "Left"})
	for _, id := range []uint64{10, 20} {
		if _, err := members.AddMember(ctx, "u1", id, types.TenantRoleViewer, nil); err != nil {
			t.Fatal(err)
		}
	}

	if got := svc.ResolveActiveTenantID(ctx, user); got != 10 {
		t.Fatalf("resolved %d, want the earliest membership 10", got)
	}
	if len(repo.updates) != 1 || repo.updates[0] != 10 {
		t.Fatalf("preference writes = %v, want exactly one write of 10", repo.updates)
	}
	if user.Preferences.LastActiveTenantID == nil || *user.Preferences.LastActiveTenantID != 10 {
		t.Fatal("the in-memory user must carry the rewritten preference for the rest of the request")
	}
	// The next resolution is served by the preference alone.
	if got := svc.ResolveActiveTenantID(ctx, user); got != 10 || len(repo.updates) != 1 {
		t.Fatalf("second resolution = %d with writes %v; want 10 and no new write", got, repo.updates)
	}
}

func TestResolveActiveTenantIDSkipsMembershipsOfDeletedWorkspaces(t *testing.T) {
	ctx := context.Background()
	user := &types.User{ID: "u1"}
	// Workspace 10 has a membership row but no tenant row any more.
	svc, _, members := newActiveTenantService(user, map[uint64]string{20: "Second"})
	for _, id := range []uint64{10, 20} {
		if _, err := members.AddMember(ctx, "u1", id, types.TenantRoleViewer, nil); err != nil {
			t.Fatal(err)
		}
	}
	if got := svc.ResolveActiveTenantID(ctx, user); got != 20 {
		t.Fatalf("resolved %d, want 20 (the only membership whose workspace exists)", got)
	}
}

func TestResolveActiveTenantIDIsZeroForAUserWithoutWorkspaces(t *testing.T) {
	ctx := context.Background()
	user := &types.User{ID: "u1", Preferences: types.UserPreferences{LastActiveTenantID: pref(7)}}
	svc, repo, _ := newActiveTenantService(user, map[uint64]string{7: "Gone for me"})
	if got := svc.ResolveActiveTenantID(ctx, user); got != 0 {
		t.Fatalf("resolved %d, want 0 for a user with no membership anywhere", got)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("nothing to persist for a tenantless user, got %v", repo.updates)
	}
}

func TestResolveActiveTenantIDLetsASuperuserKeepAPreferenceWithoutMembership(t *testing.T) {
	ctx := context.Background()
	user := &types.User{
		ID: "root", CanAccessAllTenants: true, Preferences: types.UserPreferences{LastActiveTenantID: pref(20)},
	}
	svc, _, _ := newActiveTenantService(user, map[uint64]string{20: "Visited"})
	if got := svc.ResolveActiveTenantID(ctx, user); got != 20 {
		t.Fatalf("resolved %d, want 20: SwitchTenant let the superuser in, so login lands there too", got)
	}
}

func TestRememberFirstWorkspaceSetsOnlyAnEmptyPreference(t *testing.T) {
	ctx := context.Background()
	fresh := &types.User{ID: "fresh"}
	settled := &types.User{ID: "settled", Preferences: types.UserPreferences{LastActiveTenantID: pref(3)}}
	repo := &activeTenantUserRepo{users: map[string]*types.User{"fresh": fresh, "settled": settled}}
	svc := &userService{userRepo: repo}

	if err := svc.RememberFirstWorkspace(ctx, "fresh", 42); err != nil {
		t.Fatal(err)
	}
	if got := repo.users["fresh"].Preferences.LastActiveTenantID; got == nil || *got != 42 {
		t.Fatalf("fresh user's preference = %v, want 42", got)
	}
	if err := svc.RememberFirstWorkspace(ctx, "settled", 42); err != nil {
		t.Fatal(err)
	}
	if got := repo.users["settled"].Preferences.LastActiveTenantID; got == nil || *got != 3 {
		t.Fatalf("settled user's preference = %v, want 3 left alone", got)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("writes = %v, want exactly one (for the fresh user)", repo.updates)
	}
	if err := svc.RememberFirstWorkspace(ctx, "fresh", 0); err == nil {
		t.Fatal("workspace 0 must be rejected")
	}
}

func TestSwitchTenantRequiresMembershipUnlessSuperuser(t *testing.T) {
	ctx := context.Background()
	user := &types.User{ID: "u1"}
	svc, _, members := newActiveTenantService(user, map[uint64]string{10: "Mine", 20: "Theirs"})
	svc.tokenRepo = &stubAuthTokenRepo{tokens: map[string]*types.AuthToken{}}
	if _, err := members.AddMember(ctx, "u1", 10, types.TenantRoleViewer, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SwitchTenant(ctx, user, 20, ""); !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("switch without membership: err = %v, want ErrMembershipNotFound", err)
	}
	resp, err := svc.SwitchTenant(ctx, user, 10, "")
	if err != nil || resp.ActiveTenant == nil || resp.ActiveTenant.ID != 10 {
		t.Fatalf("switch into own membership: resp=%+v err=%v", resp, err)
	}

	root := &types.User{ID: "root", CanAccessAllTenants: true}
	resp, err = svc.SwitchTenant(ctx, root, 20, "")
	if err != nil || resp.ActiveTenant == nil || resp.ActiveTenant.ID != 20 {
		t.Fatalf("superuser switch without membership: resp=%+v err=%v", resp, err)
	}
}
