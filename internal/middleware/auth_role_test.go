package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// fakeMemberService is a hand-rolled stand-in for
// interfaces.TenantMemberService. It backs Get/ListByUser/AddMember with an
// in-memory map and lets each test seed exactly the rows it cares about.
// Other interface methods are stubbed because the middleware never touches
// them.
type fakeMemberService struct {
	members map[string]*types.TenantMember // key = userID + "|" + tenantID
	// addCalls records every AddMember invocation so tests can assert that
	// the middleware never writes a membership row.
	addCalls []struct {
		UserID   string
		TenantID uint64
		Role     types.TenantRole
	}
	failGet    error
	failHasAny error
	failAdd    error
}

func newFakeMemberService() *fakeMemberService {
	return &fakeMemberService{members: map[string]*types.TenantMember{}}
}

func memberKey(u string, t uint64) string {
	return u + "|" + uintToStr(t)
}

func uintToStr(t uint64) string {
	// 简单数字转字符串，避免引入额外依赖。
	if t == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for t > 0 {
		i--
		buf[i] = byte('0' + t%10)
		t /= 10
	}
	return string(buf[i:])
}

func (f *fakeMemberService) seedActive(userID string, tenantID uint64, role types.TenantRole) {
	f.members[memberKey(userID, tenantID)] = &types.TenantMember{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		Status:   types.TenantMemberStatusActive,
	}
}

func (f *fakeMemberService) AddMember(
	ctx context.Context, userID string, tenantID uint64, role types.TenantRole, invitedBy *string,
) (*types.TenantMember, error) {
	f.addCalls = append(f.addCalls, struct {
		UserID   string
		TenantID uint64
		Role     types.TenantRole
	}{userID, tenantID, role})
	if f.failAdd != nil {
		return nil, f.failAdd
	}
	m := &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive}
	f.members[memberKey(userID, tenantID)] = m
	return m, nil
}

func (f *fakeMemberService) EnsureOwner(
	ctx context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	if existing, ok := f.members[memberKey(userID, tenantID)]; ok {
		return existing, nil
	}
	return f.AddMember(ctx, userID, tenantID, types.TenantRoleOwner, nil)
}

func (f *fakeMemberService) GetMembership(
	ctx context.Context, userID string, tenantID uint64,
) (*types.TenantMember, error) {
	if f.failGet != nil {
		return nil, f.failGet
	}
	m, ok := f.members[memberKey(userID, tenantID)]
	if !ok {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}

func (f *fakeMemberService) ListByUser(ctx context.Context, userID string) ([]*types.TenantMember, error) {
	var out []*types.TenantMember
	for _, member := range f.members {
		if member.UserID == userID && member.Status == types.TenantMemberStatusActive {
			copy := *member
			out = append(out, &copy)
		}
	}
	return out, nil
}

func (f *fakeMemberService) ListByTenant(ctx context.Context, tenantID uint64) ([]*types.TenantMember, error) {
	return nil, nil
}

func (f *fakeMemberService) ListMembersPage(
	ctx context.Context, tenantID uint64, query string, page, pageSize int,
) ([]*types.TenantMember, int64, error) {
	return nil, 0, nil
}

func (f *fakeMemberService) HasAnyMembers(ctx context.Context, tenantID uint64) (bool, error) {
	if f.failHasAny != nil {
		return false, f.failHasAny
	}
	for _, m := range f.members {
		if m.TenantID == tenantID && m.Status == types.TenantMemberStatusActive {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeMemberService) UpdateRole(
	ctx context.Context, userID string, tenantID uint64, newRole types.TenantRole,
) error {
	return nil
}

func (f *fakeMemberService) RemoveMember(ctx context.Context, userID string, tenantID uint64) error {
	return nil
}

var _ interfaces.TenantMemberService = (*fakeMemberService)(nil)

func cfgWithRBAC(enabled bool) *config.Config {
	return &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}
}

func TestResolveTenantRole_ActiveMembershipWins(t *testing.T) {
	svc := newFakeMemberService()
	svc.seedActive("u1", 10, types.TenantRoleContributor)

	got, ok := resolveTenantRole(context.Background(), svc, &types.User{ID: "u1"}, 10, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleContributor {
		t.Fatalf("got (%v, %v), want (contributor, true)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("must not write memberships, got %d AddMember calls", len(svc.addCalls))
	}
}

func TestResolveTenantRole_SuperuserWithoutMembershipGetsAdmin_NoWrite(t *testing.T) {
	// 回归 H1：跨空间超管进入没有成员关系的空间时，绝对不能写入 tenant_members。
	svc := newFakeMemberService()
	user := &types.User{ID: "super", CanAccessAllTenants: true}

	got, ok := resolveTenantRole(context.Background(), svc, user, 99, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("got (%v, %v), want (admin, true)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("cross-tenant superuser must not be written into tenant_members, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_SuperuserMembershipStillWins(t *testing.T) {
	// A superuser who is a real Owner somewhere keeps that role there; the
	// visitor Admin applies only where no membership exists.
	svc := newFakeMemberService()
	svc.seedActive("super", 10, types.TenantRoleOwner)
	user := &types.User{ID: "super", CanAccessAllTenants: true}

	got, ok := resolveTenantRole(context.Background(), svc, user, 10, cfgWithRBAC(true))
	if !ok || got != types.TenantRoleOwner {
		t.Fatalf("got (%v, %v), want (owner, true)", got, ok)
	}
}

func TestResolveTenantRole_MemberlessWorkspaceNeverPromotes(t *testing.T) {
	// A workspace with no members used to make its first visitor Owner (the
	// "orphan workspace" self-heal keyed on users.tenant_id). That path is
	// gone: nobody is promoted, and under RBAC the request is rejected.
	svc := newFakeMemberService() // 空 — 任何空间都没有成员
	user := &types.User{ID: "u1"}

	got, ok := resolveTenantRole(context.Background(), svc, user, 7, cfgWithRBAC(true))
	if ok {
		t.Fatalf("no membership must be rejected under RBAC, got role=%v", got)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("a memberless workspace must not promote its visitor, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_FailOpenAdminWhenRBACDisabled(t *testing.T) {
	svc := newFakeMemberService()
	user := &types.User{ID: "u1"}
	got, ok := resolveTenantRole(context.Background(), svc, user, 8, cfgWithRBAC(false))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("EnableRBAC=false should fail open Admin, got (%v, %v)", got, ok)
	}
	if len(svc.addCalls) != 0 {
		t.Fatalf("fail-open must not write memberships either, got %+v", svc.addCalls)
	}
}

func TestResolveTenantRole_FailClosedWhenRBACEnabled(t *testing.T) {
	svc := newFakeMemberService()
	svc.seedActive("other", 8, types.TenantRoleOwner)
	user := &types.User{ID: "u1"}
	if _, ok := resolveTenantRole(context.Background(), svc, user, 8, cfgWithRBAC(true)); ok {
		t.Fatalf("EnableRBAC=true + no membership should be rejected")
	}
}

func TestResolveTenantRole_LookupErrorFailsOpenWhenRBACDisabled(t *testing.T) {
	// 短暂 DB 错误时，fail-open 模式不应锁死现有用户。
	svc := newFakeMemberService()
	svc.failGet = errors.New("transient db failure")
	user := &types.User{ID: "u1"}

	got, ok := resolveTenantRole(context.Background(), svc, user, 8, cfgWithRBAC(false))
	if !ok || got != types.TenantRoleAdmin {
		t.Fatalf("transient lookup error under RBAC=false should fail open Admin, got (%v, %v)", got, ok)
	}
}

func TestResolveTenantRole_LookupErrorFailsClosedWhenRBACEnabled(t *testing.T) {
	svc := newFakeMemberService()
	svc.failGet = errors.New("transient db failure")
	if _, ok := resolveTenantRole(context.Background(), svc, &types.User{ID: "u1"}, 8, cfgWithRBAC(true)); ok {
		t.Fatal("a lookup error must not grant access under RBAC")
	}
}
