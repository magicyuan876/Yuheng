package acl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func init() { gin.SetMode(gin.TestMode) }

// withAuth simulates the auth middleware: it puts tenant and user into the
// request context the way Yuheng's auth layer does.
func withAuth(tenantID uint64, userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if tenantID != 0 {
			ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
		}
		if userID != "" {
			ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func TestGuardStatusCodes(t *testing.T) {
	w := newWorld(t)
	root := w.page("root", "")
	hidden := w.page("hidden", "")
	w.member(model.UserPrincipal("alice"), model.RoleReader)
	w.restrict("hidden", map[model.Principal]model.SpaceRole{model.UserPrincipal("bob"): model.RoleReader})
	guard := NewGuard(w.res)

	run := func(user string, tenant uint64, mw gin.HandlerFunc, path string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(withAuth(tenant, user))
		r.GET("/spaces/:sid", mw, func(c *gin.Context) {
			space, role, ok := SpaceFromGin(c)
			require.True(t, ok)
			c.JSON(200, gin.H{"space": space.ID, "role": role})
		})
		r.GET("/pages/:pid", mw, func(c *gin.Context) {
			d, ok := DecisionFromGin(c)
			require.True(t, ok)
			c.JSON(200, gin.H{"role": d.Role})
		})
		r.GET("/slug/:slug", mw, func(c *gin.Context) { c.Status(200) })
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(rec, req)
		return rec
	}

	// Space by id: reader passes reader, fails writer.
	spacePath := "/spaces/" + w.space.ID
	spaceReader := guard.RequireSpace("sid", SpaceByID, model.RoleReader)
	spaceWriter := guard.RequireSpace("sid", SpaceByID, model.RoleWriter)
	require.Equal(t, 200, run("alice", 1, spaceReader, spacePath).Code)
	require.Equal(t, 403, run("alice", 1, spaceWriter, spacePath).Code)
	// Non-member of the space: 404, not 403.
	require.Equal(t, 404, run("carol", 1, spaceReader, spacePath).Code)
	// Unknown space: 404.
	require.Equal(t, 404, run("owner", 1, guard.RequireSpace("sid", SpaceByID, model.RoleReader), "/spaces/nope").Code)
	// By slug.
	require.Equal(t, 200, run("alice", 1, guard.RequireSpace("slug", SpaceBySlug, model.RoleReader), "/slug/eng").Code)

	// Pages.
	require.Equal(t, 200, run("alice", 1, guard.RequirePage("pid", PageByID, model.RoleReader), "/pages/"+root.ID).Code)
	require.Equal(t, 403, run("alice", 1, guard.RequirePage("pid", PageByID, model.RoleWriter), "/pages/"+root.ID).Code)
	require.Equal(t, 404, run("alice", 1, guard.RequirePage("pid", PageByID, model.RoleReader), "/pages/"+hidden.ID).Code,
		"restricted page the caller cannot see reads as missing")
	require.Equal(t, 200, run("owner", 1, guard.RequirePage("pid", PageByID, model.RoleAdmin), "/pages/"+hidden.ID).Code)
	pageByShort := guard.RequirePage("pid", PageByShortID, model.RoleReader)
	require.Equal(t, 200, run("alice", 1, pageByShort, "/pages/"+root.ShortID).Code)

	// Tenant / session problems.
	require.Equal(t, 401, run("", 1, guard.RequirePage("pid", PageByID, model.RoleReader), "/pages/"+root.ID).Code)
	require.Equal(t, 401, run("alice", 0, guard.RequirePage("pid", PageByID, model.RoleReader), "/pages/"+root.ID).Code)
	require.Equal(t, 403, run("stranger", 1, guard.RequireMember(), "/slug/eng").Code, "not a tenant member")
	require.Equal(t, 404, run("alice", 2, guard.RequirePage("pid", PageByID, model.RoleReader), "/pages/"+root.ID).Code,
		"alice owns tenant 2 but the page is in tenant 1")
}

type fakeMembers map[string]*types.TenantMember

func (f fakeMembers) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	m, ok := f[userID]
	if !ok {
		return nil, errors.New("tenant member not found")
	}
	if m == nil {
		return nil, gorm.ErrRecordNotFound
	}
	_ = tenantID
	return m, nil
}

func TestTenantMemberRoleSource(t *testing.T) {
	src := NewTenantMemberRoleSource(fakeMembers{
		"active":    {Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive},
		"invited":   {Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusInvited},
		"suspended": {Role: types.TenantRoleOwner, Status: types.TenantMemberStatusSuspended},
		"gormnil":   nil,
	})
	role, ok, err := src.TenantRole(ctx(), 1, "active")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, types.TenantRoleAdmin, role)
	for _, u := range []string{"invited", "suspended", "missing", "gormnil"} {
		_, ok, err := src.TenantRole(ctx(), 1, u)
		require.NoError(t, err, u)
		require.False(t, ok, u)
	}
}
