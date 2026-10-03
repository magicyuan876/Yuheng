package container

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

// TestSystemAdministratorsManageWorkspaces drives the "one enterprise, one
// workspace" administration model through the real router: after bootstrap
// only the system administrator can see the workspace catalog, create a
// workspace (always with an Owner), create accounts, and put accounts into
// any workspace; the person placed somewhere finds their workspace at the
// next /auth/me without doing anything; and a workspace is deleted only
// once it is empty and not the last one.
func TestSystemAdministratorsManageWorkspaces(t *testing.T) {
	s := bootServer(t, bootOptions{})
	const (
		adminEmail = "admin@example.com"
		adminPass  = "correct horse battery"
		leadEmail  = "lead@example.com"
		leadPass   = "lead password 1"
	)

	// Bootstrap: the first account is the system administrator and owns the
	// default workspace.
	got := s.do(request{method: http.MethodPost, path: "/api/v1/auth/register", body: map[string]string{
		"username": "admin", "email": adminEmail, "password": adminPass, "workspace_name": "Acme",
	}, ip: "203.0.113.30"})
	if got.Code != http.StatusCreated {
		t.Fatalf("bootstrap = %d: %s", got.Code, got.Body)
	}
	var adminRow types.User
	must1(t, s.DB.Where("email = ?", adminEmail).First(&adminRow).Error)
	adminTenant := *adminRow.Preferences.LastActiveTenantID
	login := func(email, password, ip string) string {
		t.Helper()
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": email, "password": password}, ip: ip,
		})
		if got.Code != http.StatusOK {
			t.Fatalf("login %s = %d: %s", email, got.Code, got.Body)
		}
		tok, _ := got.JSON(t)["token"].(string)
		return tok
	}
	adminToken := login(adminEmail, adminPass, "203.0.113.31")
	me := func(token string) map[string]any {
		t.Helper()
		got := s.do(request{method: http.MethodGet, path: "/api/v1/auth/me", bearer: token})
		if got.Code != http.StatusOK {
			t.Fatalf("GET /auth/me = %d: %s", got.Code, got.Body)
		}
		data, _ := got.JSON(t)["data"].(map[string]any)
		return data
	}
	errorCode := func(r response) float64 {
		t.Helper()
		errObj, _ := r.JSON(t)["error"].(map[string]any)
		code, _ := errObj["code"].(float64)
		return code
	}

	var (
		leadID       string
		secondTenant uint64
	)

	t.Run("the administrator sees the catalog and may create workspaces", func(t *testing.T) {
		data := me(adminToken)
		caps, _ := data["capabilities"].(map[string]any)
		if caps["can_create_tenant"] != true {
			t.Errorf("a system administrator must be offered workspace creation: %v", caps)
		}
		all := s.do(request{method: http.MethodGet, path: "/api/v1/tenants/all", bearer: adminToken})
		if all.Code != http.StatusOK {
			t.Fatalf("GET /tenants/all = %d: %s", all.Code, all.Body)
		}
		allData, _ := all.JSON(t)["data"].(map[string]any)
		items, _ := allData["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("catalog lists %d workspaces, want the default one: %s", len(items), all.Body)
		}
		if first, _ := items[0].(map[string]any); first["member_count"] != float64(1) {
			t.Errorf("the default workspace must report its single member: %v", first)
		}
	})

	t.Run("an account created without a workspace belongs nowhere", func(t *testing.T) {
		created := s.do(request{
			method: http.MethodPost, path: "/api/v1/system/admin/users/create", bearer: adminToken,
			body: map[string]any{"username": "lead", "email": leadEmail, "password": leadPass},
		})
		if created.Code != http.StatusCreated {
			t.Fatalf("create user = %d: %s", created.Code, created.Body)
		}
		user, _ := created.JSON(t)["user"].(map[string]any)
		leadID, _ = user["id"].(string)
		if leadID == "" {
			t.Fatalf("no user id in %s", created.Body)
		}
		data := me(login(leadEmail, leadPass, "203.0.113.32"))
		if data["tenant_required"] != true || data["tenant"] != nil {
			t.Errorf("a fresh account must have no workspace: %v", data)
		}
		caps, _ := data["capabilities"].(map[string]any)
		if caps["can_create_tenant"] != false {
			t.Errorf("an ordinary user must not be offered workspace creation: %v", caps)
		}
	})

	t.Run("a workspace is created with its owner in one request", func(t *testing.T) {
		created := s.do(request{
			method: http.MethodPost, path: "/api/v1/tenants", bearer: adminToken,
			body: map[string]any{"name": "Field Office", "owner_email": leadEmail},
		})
		if created.Code != http.StatusCreated {
			t.Fatalf("POST /tenants = %d: %s", created.Code, created.Body)
		}
		data, _ := created.JSON(t)["data"].(map[string]any)
		secondTenant = uint64(data["id"].(float64))
		if quota, _ := data["storage_quota"].(float64); quota != 0 {
			t.Errorf("default quota = %v, want 0 (unlimited)", quota)
		}
		var member types.TenantMember
		must1(t, s.DB.Where("user_id = ? AND tenant_id = ?", leadID, secondTenant).First(&member).Error)
		if member.Role != types.TenantRoleOwner {
			t.Errorf("the named owner is %s, want owner", member.Role)
		}
		var adminMemberships int64
		s.DB.Model(&types.TenantMember{}).
			Where("user_id = ? AND tenant_id = ?", adminRow.ID, secondTenant).
			Count(&adminMemberships)
		if adminMemberships != 0 {
			t.Error("naming an owner must not also make the administrator a member")
		}
		// The owner, who had no workspace a moment ago, now lands in it.
		data = me(login(leadEmail, leadPass, "203.0.113.33"))
		tenant, _ := data["tenant"].(map[string]any)
		if tenant == nil || tenant["id"] != float64(secondTenant) {
			t.Errorf("the new owner's /auth/me must resolve the new workspace: %v", data)
		}
		var audit types.AuditLog
		must1(t, s.DB.Where("action = ?", types.AuditActionSystemTenantCreated).First(&audit).Error)
		if audit.TenantID != 0 || audit.ActorUserID != adminRow.ID ||
			audit.TargetID != strconv.FormatUint(secondTenant, 10) {
			t.Errorf("creation audit row = %+v, want system-scope, by the administrator, naming the workspace", audit)
		}
	})

	t.Run("a workspace owner is not a workspace administrator", func(t *testing.T) {
		leadToken := login(leadEmail, leadPass, "203.0.113.34")
		for _, tc := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/tenants/all"},
			{http.MethodPost, "/api/v1/tenants"},
			{http.MethodGet, "/api/v1/system/admin/tenants/" + strconv.FormatUint(secondTenant, 10) + "/members"},
		} {
			got := s.do(request{method: tc.method, path: tc.path, bearer: leadToken, body: map[string]any{"name": "x"}})
			if got.Code != http.StatusForbidden {
				t.Errorf("%s %s as a workspace owner = %d, want 403: %s", tc.method, tc.path, got.Code, got.Body)
			}
		}
	})

	t.Run("a platform key must name the owner", func(t *testing.T) {
		created := s.do(request{
			method: http.MethodPost, path: "/api/v1/system/admin/api-keys", bearer: adminToken,
			body: map[string]any{"name": "provisioner", "capabilities": []string{"system_tenants_manage"}},
		})
		if created.Code != http.StatusCreated {
			t.Fatalf("create platform key = %d: %s", created.Code, created.Body)
		}
		data, _ := created.JSON(t)["data"].(map[string]any)
		key, _ := data["token"].(string)
		without := s.do(request{
			method: http.MethodPost, path: "/api/v1/tenants", apiKey: key, body: map[string]any{"name": "Ownerless"},
		})
		if without.Code != http.StatusBadRequest || errorCode(without) != 2006 {
			t.Errorf("platform key without owner_email = %d %s, want 400 code 2006", without.Code, without.Body)
		}
		var n int64
		s.DB.Model(&types.Tenant{}).Where("name = ?", "Ownerless").Count(&n)
		if n != 0 {
			t.Error("the ownerless workspace must not exist")
		}
		with := s.do(request{method: http.MethodPost, path: "/api/v1/tenants", apiKey: key, body: map[string]any{
			"name": "Provisioned", "owner_email": adminEmail,
		}})
		if with.Code != http.StatusCreated {
			t.Fatalf("platform key with owner_email = %d: %s", with.Code, with.Body)
		}
		provisioned, _ := with.JSON(t)["data"].(map[string]any)
		id := uint64(provisioned["id"].(float64))
		var member types.TenantMember
		must1(t, s.DB.Where("user_id = ? AND tenant_id = ?", adminRow.ID, id).First(&member).Error)
		if member.Role != types.TenantRoleOwner {
			t.Errorf("the named owner is %s, want owner", member.Role)
		}
		// Clean up so the deletion steps below see exactly two workspaces.
		must1(t, s.DB.Where("tenant_id = ?", id).Delete(&types.TenantMember{}).Error)
		must1(t, s.DB.Delete(&types.Tenant{}, id).Error)
	})

	t.Run("the administrator places a workspace-less account into any workspace", func(t *testing.T) {
		created := s.do(request{
			method: http.MethodPost, path: "/api/v1/system/admin/users/create", bearer: adminToken,
			body: map[string]any{"username": "lonely", "email": "lonely@example.com", "password": "lonely password 1"},
		})
		if created.Code != http.StatusCreated {
			t.Fatalf("create user = %d: %s", created.Code, created.Body)
		}
		lonelyToken := login("lonely@example.com", "lonely password 1", "203.0.113.35")
		if data := me(lonelyToken); data["tenant_required"] != true {
			t.Fatalf("lonely must start without a workspace: %v", data)
		}

		// Into the second workspace, which the administrator is not a member
		// of: that is what the admin mount is for.
		membersPath := "/api/v1/system/admin/tenants/" + strconv.FormatUint(secondTenant, 10) + "/members"
		added := s.do(request{method: http.MethodPost, path: membersPath, bearer: adminToken, body: map[string]any{
			"email": "lonely@example.com", "role": "viewer",
		}})
		if added.Code != http.StatusCreated {
			t.Fatalf("admin add member = %d: %s", added.Code, added.Body)
		}
		// Same token, no new login: the session now resolves the workspace.
		data := me(lonelyToken)
		tenant, _ := data["tenant"].(map[string]any)
		if tenant == nil || tenant["id"] != float64(secondTenant) {
			t.Errorf("after being added, /auth/me must resolve the workspace: %v", data)
		}
		var audit types.AuditLog
		must1(t, s.DB.Where("action = ? AND tenant_id = ?", types.AuditActionMemberAdded, secondTenant).
			Order("id DESC").First(&audit).Error)
		if audit.ActorUserID != adminRow.ID || audit.ActorRole != "system_admin" {
			t.Errorf("audit actor = %s/%s, want the administrator recorded as system_admin",
				audit.ActorUserID, audit.ActorRole)
		}

		listed := s.do(request{method: http.MethodGet, path: membersPath, bearer: adminToken})
		if listed.Code != http.StatusOK {
			t.Fatalf("admin list members = %d: %s", listed.Code, listed.Body)
		}
		listData, _ := listed.JSON(t)["data"].(map[string]any)
		if listData["total"] != float64(2) {
			t.Errorf("the workspace must list its owner and the new viewer: %s", listed.Body)
		}

		var lonely types.User
		must1(t, s.DB.Where("email = ?", "lonely@example.com").First(&lonely).Error)
		memberPath := membersPath + "/" + lonely.ID
		changed := s.do(request{
			method: http.MethodPatch, path: memberPath, bearer: adminToken, body: map[string]any{"role": "admin"},
		})
		if changed.Code != http.StatusOK {
			t.Fatalf("admin change role = %d: %s", changed.Code, changed.Body)
		}
		var row types.TenantMember
		must1(t, s.DB.Where("user_id = ? AND tenant_id = ?", lonely.ID, secondTenant).First(&row).Error)
		if row.Role != types.TenantRoleAdmin {
			t.Errorf("role after PATCH = %s, want admin", row.Role)
		}
		// The last-owner rule holds on the admin mount too.
		demote := s.do(request{
			method: http.MethodPatch, path: membersPath + "/" + leadID, bearer: adminToken,
			body: map[string]any{"role": "viewer"},
		})
		if demote.Code != http.StatusConflict {
			t.Errorf("demoting the only owner = %d, want 409: %s", demote.Code, demote.Body)
		}
		removed := s.do(request{method: http.MethodDelete, path: memberPath, bearer: adminToken})
		if removed.Code != http.StatusOK {
			t.Fatalf("admin remove member = %d: %s", removed.Code, removed.Body)
		}
		data = me(login("lonely@example.com", "lonely password 1", "203.0.113.36"))
		if data["tenant_required"] != true {
			t.Errorf("after removal lonely must again have no workspace: %v", data)
		}
	})

	t.Run("an account can be created straight into a workspace", func(t *testing.T) {
		created := s.do(request{
			method: http.MethodPost, path: "/api/v1/system/admin/users/create", bearer: adminToken,
			body: map[string]any{
				"username": "newbie", "email": "newbie@example.com", "password": "newbie password 1",
				"tenant_id": adminTenant, "role": "contributor",
			},
		})
		if created.Code != http.StatusCreated {
			t.Fatalf("create user into workspace = %d: %s", created.Code, created.Body)
		}
		membership, _ := created.JSON(t)["membership"].(map[string]any)
		if membership == nil || membership["role"] != "contributor" {
			t.Errorf("response must carry the membership: %s", created.Body)
		}
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": "newbie@example.com", "password": "newbie password 1"}, ip: "203.0.113.37",
		})
		active, _ := got.JSON(t)["active_tenant"].(map[string]any)
		if active == nil || active["id"] != float64(adminTenant) {
			t.Errorf("login must land in the workspace the account was created into: %s", got.Body)
		}
		unknown := s.do(request{
			method: http.MethodPost, path: "/api/v1/system/admin/users/create", bearer: adminToken,
			body: map[string]any{"username": "typo", "email": "typo@example.com", "tenant_id": 999999},
		})
		if unknown.Code != http.StatusNotFound {
			t.Errorf("an unknown tenant_id = %d, want 404: %s", unknown.Code, unknown.Body)
		}
		var n int64
		s.DB.Model(&types.User{}).Where("email = ?", "typo@example.com").Count(&n)
		if n != 0 {
			t.Error("a refused create must not leave an account behind")
		}
	})

	t.Run("a workspace is deleted only when empty and not the last one", func(t *testing.T) {
		adminPath := "/api/v1/tenants/" + strconv.FormatUint(adminTenant, 10)
		// The default workspace still has newbie in it.
		got := s.do(request{method: http.MethodDelete, path: adminPath, bearer: adminToken})
		if got.Code != http.StatusConflict || errorCode(got) != 2007 {
			t.Errorf("deleting a workspace with other members = %d %s, want 409 code 2007", got.Code, got.Body)
		}
		// The second workspace has only its owner; the owner may delete it.
		leadToken := login(leadEmail, leadPass, "203.0.113.38")
		secondPath := "/api/v1/tenants/" + strconv.FormatUint(secondTenant, 10)
		got = s.do(request{method: http.MethodDelete, path: secondPath, bearer: leadToken})
		if got.Code != http.StatusOK {
			t.Fatalf("owner deleting an otherwise empty workspace = %d: %s", got.Code, got.Body)
		}
		var remaining int64
		s.DB.Model(&types.Tenant{}).Count(&remaining)
		if remaining != 1 {
			t.Fatalf("%d workspaces remain, want 1", remaining)
		}
		// Empty the default workspace of everyone else, then it is the last one.
		var newbie types.User
		must1(t, s.DB.Where("email = ?", "newbie@example.com").First(&newbie).Error)
		removed := s.do(request{
			method: http.MethodDelete, path: adminPath + "/members/" + newbie.ID, bearer: adminToken,
		})
		if removed.Code != http.StatusOK {
			t.Fatalf("remove member = %d: %s", removed.Code, removed.Body)
		}
		got = s.do(request{method: http.MethodDelete, path: adminPath, bearer: adminToken})
		if got.Code != http.StatusConflict || errorCode(got) != 2008 {
			t.Errorf("deleting the last workspace = %d %s, want 409 code 2008", got.Code, got.Body)
		}
		s.DB.Model(&types.Tenant{}).Count(&remaining)
		if remaining != 1 {
			t.Errorf("the last workspace must survive, %d remain", remaining)
		}
	})
}
