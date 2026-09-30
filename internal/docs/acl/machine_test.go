package acl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

func keyScope(id uint64, fullAccess bool, caps ...types.APIKeyCapability) types.TenantAPIKeyScope {
	scope := types.TenantAPIKeyScope{KeyID: id, ScopeType: types.APIKeyScopeTenant, FullAccess: fullAccess}
	for _, c := range caps {
		scope.Capabilities = append(scope.Capabilities, string(c))
	}
	return scope
}

func (w *world) keyIdentity(scope types.TenantAPIKeyScope) *Identity {
	w.t.Helper()
	id, err := w.res.MachineIdentity(ctx(), 1, scope, types.Principal{Type: types.PrincipalAPITenant, ID: "1"})
	require.NoError(w.t, err)
	return id
}

func (w *world) keyRole(id *Identity, page string) model.SpaceRole {
	w.t.Helper()
	d, err := w.res.Page(ctx(), id, w.pages[page].ID)
	require.NoError(w.t, err)
	return d.Role
}

func TestMachineIdentityIsTheKeyNotAPerson(t *testing.T) {
	w := newWorld(t)
	id := w.keyIdentity(keyScope(7, false, types.APIKeyCapabilityDocsRead))
	require.Equal(t, "api_tenant_key:1:7", id.UserID)
	require.True(t, id.Machine)
	require.True(t, IsMachineUserID(id.UserID))
	require.False(t, IsMachineUserID("alice"))

	external, err := w.res.MachineIdentity(ctx(), 1, keyScope(7, false),
		types.Principal{Type: types.PrincipalAPIExternalUser, ID: "1:ext-42"})
	require.NoError(t, err)
	require.Equal(t, "api_external_user:1:ext-42", external.UserID, "each external user acts under its own id")

	_, err = w.res.MachineIdentity(ctx(), 1, types.TenantAPIKeyScope{}, types.Principal{})
	require.Error(t, err, "a scope without a key id cannot name anybody")
}

func TestMachineIdentityReachIsWhatEveryMemberGetsCappedByCapability(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.page("named", "")
	w.member(model.UserPrincipal("alice"), model.RoleAdmin)
	w.restrict("named", map[model.Principal]model.SpaceRole{model.UserPrincipal("alice"): model.RoleWriter})

	reader := w.keyIdentity(keyScope(1, false, types.APIKeyCapabilityDocsRead))
	writer := w.keyIdentity(keyScope(2, false, types.APIKeyCapabilityDocsRead, types.APIKeyCapabilityDocsWrite))
	full := w.keyIdentity(keyScope(3, true))

	require.Equal(t, model.RoleNone, w.keyRole(reader, "root"), "a private space is not every member's")
	require.Equal(t, model.RoleAdmin, w.keyRole(full, "root"), "full access administers the workspace")
	require.True(t, full.IsTenantAdmin())

	// Opening the space to members with writer by default reaches the keys,
	// capped by what each may do.
	require.NoError(t, w.repos.Spaces.Update(ctx(), 1, w.space.ID, map[string]any{
		"visibility": model.VisibilityOpen, "default_role": model.RoleWriter,
	}))
	w.res.Invalidate(ctx(), 1)
	require.Equal(t, model.RoleReader, w.keyRole(reader, "root"), "docs_read caps at reader")
	require.Equal(t, model.RoleWriter, w.keyRole(writer, "root"))
	require.Equal(t, model.RoleNone, w.keyRole(writer, "named"),
		"a page restricted to named people stays out of reach: nothing can be granted to a key")
	require.Equal(t, model.RoleAdmin, w.keyRole(full, "named"), "space admins are exempt from restrictions")
}

func TestMachineIdentityLimitedToKnowledgeBases(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	bound := &model.Space{TenantID: 1, Slug: "bound", Name: "Bound", Visibility: model.VisibilityPrivate}
	kb := "kb-1"
	bound.KnowledgeBaseID = &kb
	require.NoError(t, w.repos.Spaces.Create(ctx(), bound))

	scope := keyScope(4, false, types.APIKeyCapabilityDocsAdmin)
	scope.KnowledgeBaseIDs = types.StringArray{"kb-1"}
	id := w.keyIdentity(scope)

	require.False(t, id.IsTenantAdmin(), "a key limited to some knowledge bases never administers the workspace")
	visible, err := w.res.VisibleSpaces(ctx(), id)
	require.NoError(t, err)
	require.Equal(t, map[string]model.SpaceRole{bound.ID: model.RoleAdmin}, visible)
	require.Equal(t, model.RoleNone, w.keyRole(id, "root"), "the unbound space is out of reach")
}

func TestGuardResolvesAPIKeysAsThemselves(t *testing.T) {
	w := newWorld(t)
	w.page("root", "")
	w.member(model.UserPrincipal("owner"), model.RoleAdmin)
	guard := NewGuard(w.res)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		// The auth layer attaches the workspace's oldest account to key
		// requests; the guard must not act under it.
		reqCtx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		reqCtx = context.WithValue(reqCtx, types.UserIDContextKey, "owner")
		reqCtx = types.WithPrincipal(reqCtx, types.Principal{Type: types.PrincipalAPITenant, ID: "1"})
		reqCtx = types.WithTenantAPIKeyScope(reqCtx, keyScope(9, false, types.APIKeyCapabilityDocsRead))
		c.Request = c.Request.WithContext(reqCtx)
		c.Next()
	})
	r.GET("/spaces/:sid", guard.RequireSpace("sid", SpaceByID, model.RoleReader), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/me", guard.RequireMember(), func(c *gin.Context) {
		id, _ := IdentityFromGin(c)
		c.String(http.StatusOK, id.UserID)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/spaces/"+w.space.ID, nil))
	require.Equal(t, http.StatusNotFound, rec.Code, "the owner reads this private space; the key does not")

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me", nil))
	require.Equal(t, "api_tenant_key:1:9", rec.Body.String())
}
