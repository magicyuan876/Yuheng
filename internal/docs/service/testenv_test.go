package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dbSeq atomic.Int64

func openRepos(t *testing.T) (*repository.Repositories, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:docs-svc-%d?mode=memory&cache=shared&_foreign_keys=1", dbSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	ddl, err := os.ReadFile(filepath.FromSlash("../testdata/schema_sqlite.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(ddl)).Error)
	return repository.New(db), db
}

// fakeMembers is the tenant membership table: "tenant/user" -> role. It
// serves both the ACL resolver (TenantMemberGetter) and the services
// (TenantMembers).
type fakeMembers struct {
	mu    sync.Mutex
	roles map[string]types.TenantRole
	users map[string]*types.User
}

func newFakeMembers() *fakeMembers {
	f := &fakeMembers{roles: map[string]types.TenantRole{}, users: map[string]*types.User{}}
	for user, role := range map[string]types.TenantRole{
		"owner": types.TenantRoleOwner, "admin": types.TenantRoleAdmin,
		"alice": types.TenantRoleContributor, "bob": types.TenantRoleContributor,
		"carol": types.TenantRoleContributor, "viewer": types.TenantRoleViewer,
		// Long enough to satisfy docs-schema's `id` format (8-36 chars), which
		// the short names above do not. Real user ids are UUIDs; a mention
		// carries one, so anything testing mentions needs a realistic id.
		"reviewer-01": types.TenantRoleContributor,
	} {
		f.roles["1/"+user] = role
		f.users[user] = &types.User{ID: user, Username: user, Email: user + "@example.test"}
	}
	f.roles["2/alice"] = types.TenantRoleOwner
	return f
}

func (f *fakeMembers) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	role, ok := f.roles[fmt.Sprintf("%d/%s", tenantID, userID)]
	if !ok {
		return nil, nil
	}
	return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive}, nil
}

func (f *fakeMembers) ListPagedByTenant(_ context.Context, tenantID uint64, search string, offset,
	limit int,
) ([]*types.TenantMember, error) {
	var out []*types.TenantMember
	for key, role := range f.roles {
		var tid uint64
		var user string
		_, _ = fmt.Sscanf(key, "%d/%s", &tid, &user)
		if tid == tenantID && (search == "" || user == search) {
			out = append(out, &types.TenantMember{UserID: user, TenantID: tid, Role: role})
		}
	}
	if offset >= len(out) {
		return nil, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (f *fakeMembers) CountFilteredByTenant(ctx context.Context, tenantID uint64, search string) (int64, error) {
	rows, err := f.ListPagedByTenant(ctx, tenantID, search, 0, 1000)
	return int64(len(rows)), err
}

func (f *fakeMembers) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	out := map[string]*types.User{}
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

// auditSink captures audit rows.
type auditSink struct {
	mu   sync.Mutex
	rows []*types.AuditLog
}

func (a *auditSink) Log(_ context.Context, row *types.AuditLog) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, row)
	return nil
}

// rows returns a copy of what was captured.
func (a *auditSink) snapshot() []*types.AuditLog {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*types.AuditLog{}, a.rows...)
}

func (a *auditSink) actions() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.rows))
	for _, r := range a.rows {
		out = append(out, string(r.Action))
	}
	return out
}

func (a *auditSink) has(action types.AuditAction) bool {
	for _, x := range a.actions() {
		if x == string(action) {
			return true
		}
	}
	return false
}

// fakeKBs knows one knowledge base per tenant.
type fakeKBs map[string]uint64

func (f fakeKBs) GetKnowledgeBaseByIDAndTenant(_ context.Context, id string, tenantID uint64) (*types.KnowledgeBase, error) {
	if t, ok := f[id]; ok && t == tenantID {
		return &types.KnowledgeBase{ID: id, TenantID: tenantID, Name: "kb"}, nil
	}
	return nil, fmt.Errorf("knowledge base not found")
}

type env struct {
	t          *testing.T
	repos      *repository.Repositories
	members    *fakeMembers
	resolver   *acl.Resolver
	bus        *events.MemoryBus
	audit      *auditSink
	gorm       *gorm.DB
	favourites *fakeFavourites
	svc        *Services
	events     []events.Event
	mu         sync.Mutex
}

func newEnv(t *testing.T, opts ...func(*Deps)) *env {
	t.Helper()
	e := &env{
		t: t, members: newFakeMembers(),
		bus: events.NewMemoryBus(), audit: &auditSink{}, favourites: newFakeFavourites(),
	}
	e.repos, e.gorm = openRepos(t)
	e.resolver = acl.NewResolver(e.repos, acl.NewTenantMemberRoleSource(e.members), acl.WithCache(acl.NewMemoryCache(0)))
	e.bus.Subscribe(0, func(ev events.Event) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.events = append(e.events, ev)
	})
	deps := Deps{
		Repos: e.repos, Resolver: e.resolver, Bus: e.bus, Audit: audit.NewRecorder(e.audit),
		Users: e.members, Members: e.members, Favourites: e.favourites,
	}
	for _, o := range opts {
		o(&deps)
	}
	e.svc = New(deps)
	return e
}

func (e *env) identity(user string) *acl.Identity {
	id, err := e.resolver.Identity(context.Background(), 1, user)
	require.NoError(e.t, err)
	return id
}

func (e *env) eventTypes() []events.Type {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]events.Type, 0, len(e.events))
	for _, ev := range e.events {
		out = append(out, ev.Type)
	}
	return out
}

// mustSpaceRole resolves the user's effective role from scratch (identity
// included), so a stale cache would show up as a wrong answer.
func (e *env) mustSpaceRole(id *acl.Identity, sp *model.Space) model.SpaceRole {
	fresh, err := e.resolver.Identity(ctx(), 1, id.UserID)
	require.NoError(e.t, err)
	role, err := e.resolver.SpaceRole(ctx(), fresh, sp)
	require.NoError(e.t, err)
	return role
}

func ctx() context.Context { return context.Background() }

func strp(s string) *string { return &s }

// fakeFavourites stands in for the platform's favourites service, which the
// docs module borrows rather than duplicating.
type fakeFavourites struct {
	mu   sync.Mutex
	rows map[string][]*types.UserResourceFavorite
}

func newFakeFavourites() *fakeFavourites {
	return &fakeFavourites{rows: map[string][]*types.UserResourceFavorite{}}
}

func (f *fakeFavourites) key(userID string, tenantID uint64, resourceType string) string {
	return fmt.Sprintf("%s/%d/%s", userID, tenantID, resourceType)
}

func (f *fakeFavourites) List(_ context.Context, userID string, tenantID uint64, resourceType string) (
	[]*types.UserResourceFavorite, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*types.UserResourceFavorite{}, f.rows[f.key(userID, tenantID, resourceType)]...), nil
}

func (f *fakeFavourites) Add(_ context.Context, userID string, tenantID uint64, resourceType, resourceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(userID, tenantID, resourceType)
	for _, row := range f.rows[key] {
		if row.ResourceID == resourceID {
			return nil
		}
	}
	f.rows[key] = append(f.rows[key], &types.UserResourceFavorite{
		UserID: userID, TenantID: tenantID, ResourceType: resourceType, ResourceID: resourceID,
	})
	return nil
}

func (f *fakeFavourites) Remove(_ context.Context, userID string, tenantID uint64, resourceType, resourceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(userID, tenantID, resourceType)
	kept := f.rows[key][:0]
	for _, row := range f.rows[key] {
		if row.ResourceID != resourceID {
			kept = append(kept, row)
		}
	}
	f.rows[key] = kept
	return nil
}
