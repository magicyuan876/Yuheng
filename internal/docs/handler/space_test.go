package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// tenantTable is the fake tenant membership; every listed user is active.
type tenantTable map[string]types.TenantRole

func (t tenantTable) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	role, ok := t[fmt.Sprintf("%d/%s", tenantID, userID)]
	if !ok {
		return nil, nil
	}
	return &types.TenantMember{
		UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive,
	}, nil
}

// ListPagedByTenant answers with the table's own rows, sorted so the result
// is stable. Mention suggestions are built from it, so a stub returning
// nothing would make that feature untestable here.
func (t tenantTable) ListPagedByTenant(_ context.Context, tenantID uint64, search string,
	offset, limit int,
) ([]*types.TenantMember, error) {
	keys := make([]string, 0, len(t))
	for key := range t {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]*types.TenantMember, 0, len(keys))
	for _, key := range keys {
		var tid uint64
		var user string
		if _, err := fmt.Sscanf(key, "%d/%s", &tid, &user); err != nil {
			continue
		}
		if tid != tenantID || (search != "" && !strings.Contains(user, search)) {
			continue
		}
		out = append(out, &types.TenantMember{
			UserID: user, TenantID: tid, Role: t[key], Status: types.TenantMemberStatusActive,
		})
	}
	if offset >= len(out) {
		return nil, nil
	}
	end := len(out)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return out[offset:end], nil
}

func (t tenantTable) CountFilteredByTenant(context.Context, uint64, string) (int64, error) {
	return 0, nil
}

func (t tenantTable) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	out := map[string]*types.User{}
	for _, id := range ids {
		out[id] = &types.User{ID: id, Username: id, Email: id + "@example.test"}
	}
	return out, nil
}

// openHandlerDB opens a private PostgreSQL database with the production
// schema applied.
func openHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	return pgtest.New(t)
}

// newSpaceRouter wires the module's routes on top of a private PostgreSQL database.
// opts adjust the service dependencies, which is how a test switches the
// deployment between the collaborative and the exclusive-edit shape.
func newSpaceRouter(t *testing.T, opts ...func(*service.Deps)) (*gin.Engine, *repository.Repositories) {
	t.Helper()
	repos := repository.New(openHandlerDB(t))

	members := tenantTable{
		"1/owner": types.TenantRoleOwner, "1/alice": types.TenantRoleContributor,
		"1/bob": types.TenantRoleContributor, "1/viewer": types.TenantRoleViewer,
		"1/carol": types.TenantRoleContributor,
	}
	resolver := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(members), acl.WithCache(acl.NewMemoryCache(0)))
	guard := acl.NewGuard(resolver)
	deps := service.Deps{
		Repos: repos, Resolver: resolver, Bus: events.NewMemoryBus(), Audit: audit.NewRecorder(nil),
		Users: members, Members: members,
	}
	for _, o := range opts {
		o(&deps)
	}
	services := service.New(deps)
	h := New(Deps{Repos: repos, Resolver: resolver, Guard: guard, Services: services})

	r := gin.New()
	// The app's error middleware renders errors attached with c.Error.
	r.Use(middleware.ErrorHandler())
	// The caller is named by header so one engine serves every test user.
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-User"))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	docs := r.Group("/docs")
	docs.GET("/spaces", guard.RequireMember(), h.Spaces.List)
	docs.POST("/spaces", guard.RequireMember(), h.Spaces.Create)
	docs.GET("/spaces/:sid", guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), h.Spaces.Get)
	docs.GET("/spaces/by-slug/:slug", guard.RequireSpace("slug", acl.SpaceBySlug, model.RoleReader), h.Spaces.GetBySlug)
	docs.PATCH("/spaces/:sid", guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Spaces.Update)
	docs.DELETE("/spaces/:sid", guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Spaces.Delete)
	docs.GET("/spaces/:sid/members", guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), h.Spaces.ListMembers)
	docs.PUT("/spaces/:sid/members", guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Spaces.SetMembers)
	docs.DELETE("/spaces/:sid/members/:ptype/:pid",
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Spaces.RemoveMember)
	docs.GET("/spaces/:sid/tree", guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), h.Pages.Tree)
	docs.GET("/spaces/:sid/trash", guard.RequireSpace("sid", acl.SpaceByID, model.RoleReader), h.Pages.Trash)
	docs.DELETE("/spaces/:sid/trash", guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Pages.EmptyTrash)
	docs.DELETE("/spaces/:sid/trash/:pid", guard.RequireSpace("sid", acl.SpaceByID, model.RoleAdmin), h.Pages.Purge)
	docs.POST("/pages", guard.RequireMember(), h.Pages.Create)
	docs.GET("/pages/:pid", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Get)
	docs.GET("/pages/by-short-id/:short", guard.RequirePage("short", acl.PageByShortID, model.RoleReader),
		h.Pages.GetByShortID)
	docs.GET("/pages/:pid/content", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Content)
	docs.PUT("/pages/:pid/content",
		guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Pages.ReplaceContent)
	docs.PATCH("/pages/:pid", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Pages.Update)
	docs.POST("/pages/:pid/move", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Pages.Move)
	docs.POST("/pages/:pid/duplicate", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Duplicate)
	docs.DELETE("/pages/:pid", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Pages.Delete)
	docs.POST("/pages/:pid/restore", guard.RequireMember(), h.Pages.Restore)
	docs.GET("/pages/:pid/ancestors", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Ancestors)
	docs.GET("/pages/:pid/children", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Children)
	docs.POST("/spaces/:sid/attachments",
		guard.RequireSpace("sid", acl.SpaceByID, model.RoleWriter), h.Files.Upload)
	docs.GET("/attachments/:aid", guard.RequireMember(), h.Files.Download)
	docs.DELETE("/attachments/:aid", guard.RequireMember(), h.Files.Delete)
	docs.GET("/pages/:pid/backlinks",
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.Backlinks)
	docs.GET("/pages/:pid/mention-candidates",
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Pages.SuggestMentions)
	docs.GET("/page-links/suggest", guard.RequireMember(), h.Pages.SuggestPages)
	docs.POST("/page-links/titles", guard.RequireMember(), h.Pages.ResolveTitles)
	docs.GET("/pages/:pid/attachments",
		guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Files.ListForPage)
	docs.GET("/pages/:pid/lease", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Leases.Get)
	docs.POST("/pages/:pid/lease", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Leases.Acquire)
	docs.DELETE("/pages/:pid/lease", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Leases.Release)
	docs.GET("/pages/:pid/ydoc", guard.RequirePage("pid", acl.PageByID, model.RoleReader), h.Leases.LoadYDoc)
	docs.PUT("/pages/:pid/ydoc", guard.RequirePage("pid", acl.PageByID, model.RoleWriter), h.Leases.SaveYDoc)
	return r, repos
}

type resp struct {
	Code int
	Body map[string]any
}

func call(t *testing.T, r http.Handler, user, method, path string, body any) resp {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", user)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	out := resp{Code: rec.Code}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
	}
	return out
}

func data(r resp) map[string]any {
	d, _ := r.Body["data"].(map[string]any)
	return d
}

func TestSpaceRoutesVisibilityAndRoles(t *testing.T) {
	r, _ := newSpaceRouter(t)

	created := call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"name": "Engineering"})
	require.Equal(t, http.StatusCreated, created.Code, created.Body)
	sid := data(created)["id"].(string)
	require.Equal(t, "engineering", data(created)["slug"])
	require.Equal(t, "admin", data(created)["role"])

	// A stranger to a private space gets 404, never 403, and no listing.
	require.Equal(t, http.StatusNotFound, call(t, r, "bob", http.MethodGet, "/docs/spaces/"+sid, nil).Code)
	require.Equal(t, http.StatusNotFound,
		call(t, r, "bob", http.MethodGet, "/docs/spaces/by-slug/engineering", nil).Code)
	list := call(t, r, "bob", http.MethodGet, "/docs/spaces", nil)
	require.Equal(t, http.StatusOK, list.Code)
	require.Empty(t, list.Body["data"])
	// Not a tenant member at all: 403 from the guard.
	require.Equal(t, http.StatusForbidden, call(t, r, "stranger", http.MethodGet, "/docs/spaces", nil).Code)

	// Open the space: bob can read, cannot administer.
	patched := call(t, r, "alice", http.MethodPatch, "/docs/spaces/"+sid,
		map[string]any{"visibility": "open", "default_role": "reader"})
	require.Equal(t, http.StatusOK, patched.Code, patched.Body)
	got := call(t, r, "bob", http.MethodGet, "/docs/spaces/"+sid, nil)
	require.Equal(t, http.StatusOK, got.Code)
	require.Equal(t, "reader", data(got)["role"])
	require.Equal(t, http.StatusForbidden,
		call(t, r, "bob", http.MethodPatch, "/docs/spaces/"+sid, map[string]any{"name": "x"}).Code)
	require.Equal(t, http.StatusForbidden,
		call(t, r, "bob", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{"members": []any{}}).Code)

	// Alice grants bob writer; bob still cannot administer; tenant owner can.
	set := call(t, r, "alice", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{
		"members": []map[string]any{{"principal_type": "user", "principal_id": "bob", "role": "writer"}},
	})
	require.Equal(t, http.StatusOK, set.Code, set.Body)
	require.Len(t, set.Body["data"], 2)
	require.Equal(t, http.StatusForbidden,
		call(t, r, "bob", http.MethodDelete, "/docs/spaces/"+sid+"/members/user/alice", nil).Code)
	require.Equal(t, http.StatusOK,
		call(t, r, "owner", http.MethodPatch, "/docs/spaces/"+sid, map[string]any{"description": "owned"}).Code)

	// Validation and conflict codes surface through the error envelope.
	bad := call(t, r, "alice", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{
		"members": []map[string]any{{"principal_type": "user", "principal_id": "alice", "role": "reader"}},
	})
	require.Equal(t, http.StatusBadRequest, bad.Code, "last admin cannot demote themselves")
	dup := call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"name": "Other", "slug": "engineering"})
	require.Equal(t, http.StatusConflict, dup.Code)
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"description": "no name"}).Code)

	// Delete hides the space from everyone who is not a tenant admin.
	require.Equal(t, http.StatusNoContent, call(t, r, "alice", http.MethodDelete, "/docs/spaces/"+sid, nil).Code)
	require.Equal(t, http.StatusNotFound, call(t, r, "alice", http.MethodGet, "/docs/spaces/"+sid, nil).Code)
}

// A space grant to a workspace group reaches the group's members over HTTP,
// and nobody else. The group routes themselves live in the application layer
// (internal/handler/tenant_group.go) and are tested there; this is the docs
// side, so the group is written straight through the repository. That a
// membership change reaches a cached identity at once is the group bridge's
// job and is tested with it (internal/docs/groupbridge_test.go).
func TestSpaceRoutesGrantThroughGroups(t *testing.T) {
	r, repos := newSpaceRouter(t)
	created := call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"name": "Team"})
	require.Equal(t, http.StatusCreated, created.Code)
	sid := data(created)["id"].(string)

	group := &types.TenantGroup{TenantID: 1, Name: "Backend"}
	require.NoError(t, repos.Groups.Create(context.Background(), group, []string{"bob"}, "owner"))

	set := call(t, r, "alice", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{
		"members": []map[string]any{{"principal_type": "group", "principal_id": group.ID, "role": "writer"}},
	})
	require.Equal(t, http.StatusOK, set.Code, set.Body)

	got := call(t, r, "bob", http.MethodGet, "/docs/spaces/"+sid, nil)
	require.Equal(t, http.StatusOK, got.Code, "bob is in the space through the group")
	require.Equal(t, "writer", data(got)["role"])
	require.Equal(t, http.StatusNotFound, call(t, r, "carol", http.MethodGet, "/docs/spaces/"+sid, nil).Code,
		"carol is not in the group and the space is private")
}
