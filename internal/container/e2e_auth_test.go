package container

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// mintToken signs an access token the way userService.generateTokensForTenant
// does, but with a secret and expiry of the test's choosing, and stores it in
// auth_tokens like the server does. Storing it matters: ValidateToken also
// requires a live row, so without one every "bad" token would be refused for
// being unknown and the test would prove nothing about signature or expiry.
func mintToken(
	t *testing.T, s *server, secret, userID, email string, tenantID uint64, expires time.Time, typ string,
) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": userID, "email": email, "tenant_id": tenantID,
		"exp": expires.Unix(), "iat": time.Now().Add(-2 * time.Hour).Unix(), "type": typ,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	row := &types.AuthToken{
		ID: uuid.NewString(), UserID: userID, Token: tok, TokenType: "access_token",
		ExpiresAt: expires,
	}
	if err := s.DB.Create(row).Error; err != nil {
		t.Fatalf("store token: %v", err)
	}
	return tok
}

// TestAuthenticationThroughTheRealRouter drives registration, login, token
// validation, API keys and workspace switching through the real router with the
// real Auth middleware, the real handlers and a real database.
//
// The middleware was only ever tested against stub services, so nothing pinned
// what a client actually experiences: which status a bad token gets, that a
// refresh token cannot stand in for an access token, that a scoped API key
// stops at its scope. The steps share one booted server and one database, and
// run in order, because registration is a one-time act on an empty database.
func TestAuthenticationThroughTheRealRouter(t *testing.T) {
	s := bootServer(t, bootOptions{})
	const (
		adminEmail = "admin@example.com"
		adminPass  = "correct horse battery"
	)
	var (
		accessToken, refreshToken string
		adminID                   string
		adminTenant               uint64
	)

	t.Run("first registration on an empty database succeeds and is the administrator", func(t *testing.T) {
		got := s.do(request{method: http.MethodPost, path: "/api/v1/auth/register", body: map[string]string{
			"username": "admin", "email": adminEmail, "password": adminPass,
		}, ip: "203.0.113.10"})
		if got.Code != http.StatusCreated {
			t.Fatalf("register = %d, want 201: %s", got.Code, got.Body)
		}
		user, _ := got.JSON(t)["user"].(map[string]any)
		if user == nil || user["email"] != adminEmail {
			t.Fatalf("response does not carry the new user: %s", got.Body)
		}
		if _, leaked := user["password_hash"]; leaked {
			t.Errorf("registration response leaks the password hash: %s", got.Body)
		}
		var row types.User
		must1(t, s.DB.Where("email = ?", adminEmail).First(&row).Error)
		adminID, adminTenant = row.ID, row.TenantID
		if !row.IsSystemAdmin {
			t.Error("the first account of a deployment must be the system administrator")
		}
		if adminTenant == 0 {
			t.Error("the first account has no workspace")
		}
	})

	t.Run("a second public registration is refused", func(t *testing.T) {
		got := s.do(request{method: http.MethodPost, path: "/api/v1/auth/register", body: map[string]string{
			"username": "intruder", "email": "intruder@example.com", "password": "another password",
		}, ip: "203.0.113.11"})
		if got.Code != http.StatusForbidden {
			t.Fatalf("second register = %d, want 403: %s", got.Code, got.Body)
		}
		var n int64
		s.DB.Model(&types.User{}).Where("email = ?", "intruder@example.com").Count(&n)
		if n != 0 {
			t.Error("the refused registration created an account anyway")
		}
	})

	t.Run("login returns tokens for the right password", func(t *testing.T) {
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": adminEmail, "password": adminPass}, ip: "203.0.113.12",
		})
		if got.Code != http.StatusOK {
			t.Fatalf("login = %d, want 200: %s", got.Code, got.Body)
		}
		body := got.JSON(t)
		accessToken, _ = body["token"].(string)
		refreshToken, _ = body["refresh_token"].(string)
		if accessToken == "" || refreshToken == "" || accessToken == refreshToken {
			t.Fatalf("login must return distinct access and refresh tokens: %s", got.Body)
		}
	})

	t.Run("wrong password and unknown user are indistinguishable", func(t *testing.T) {
		wrong := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": adminEmail, "password": "not the password"}, ip: "203.0.113.13",
		})
		unknown := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": "nobody@example.com", "password": "not the password"}, ip: "203.0.113.13",
		})
		if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password = %d, unknown user = %d, both want 401", wrong.Code, unknown.Code)
		}
		// Same status and same body, so the response cannot be used to find out
		// which addresses are registered.
		if string(wrong.Body) != string(unknown.Body) {
			t.Errorf("responses differ:\nwrong password: %s\nunknown user:   %s", wrong.Body, unknown.Body)
		}
		if wrong.JSON(t)["token"] != nil && wrong.JSON(t)["token"] != "" {
			t.Errorf("a failed login returned a token: %s", wrong.Body)
		}
	})

	// Every case below calls GET /api/v1/auth/me, an authenticated route that
	// needs no particular role.
	t.Run("bearer tokens", func(t *testing.T) {
		mint := func(secret string, expires time.Time, typ string) string {
			return mintToken(t, s, secret, adminID, adminEmail, adminTenant, expires, typ)
		}
		expired := mint(testJWTSecret, time.Now().Add(-time.Hour), "access")
		foreign := mintToken(t, s, "a-completely-different-signing-secret-0000", adminID, adminEmail, adminTenant,
			time.Now().Add(time.Hour), "access")
		// A well-formed, stored, unexpired token whose type says refresh: what a
		// client would send if it mixed the two up.
		refreshLike := mint(testJWTSecret, time.Now().Add(time.Hour), "refresh")

		cases := []struct {
			name   string
			bearer string
			want   int
		}{
			{"valid access token", accessToken, http.StatusOK},
			{"no token", "", http.StatusUnauthorized},
			{"garbage", "not-a-jwt", http.StatusUnauthorized},
			{"expired token", expired, http.StatusUnauthorized},
			{"signed with another secret", foreign, http.StatusUnauthorized},
			{"the refresh token from login", refreshToken, http.StatusUnauthorized},
			{"a refresh-typed token", refreshLike, http.StatusUnauthorized},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := s.do(request{method: http.MethodGet, path: "/api/v1/auth/me", bearer: tc.bearer})
				if got.Code != tc.want {
					t.Errorf("GET /auth/me = %d, want %d: %s", got.Code, tc.want, got.Body)
				}
			})
		}

		// The same token must open a tenant-scoped route as well.
		got := s.do(request{method: http.MethodGet, path: "/api/v1/knowledge-bases", bearer: accessToken})
		if got.Code != http.StatusOK {
			t.Errorf("GET /knowledge-bases with a valid token = %d, want 200: %s", got.Code, got.Body)
		}
		got = s.do(request{method: http.MethodGet, path: "/api/v1/knowledge-bases"})
		if got.Code != http.StatusUnauthorized {
			t.Errorf("GET /knowledge-bases without a token = %d, want 401", got.Code)
		}
	})

	t.Run("API keys are held to their scope", func(t *testing.T) {
		tenantPath := "/api/v1/tenants/" + strconv.FormatUint(adminTenant, 10) + "/api-keys"
		create := func(body map[string]any) string {
			t.Helper()
			got := s.do(request{method: http.MethodPost, path: tenantPath, body: body, bearer: accessToken})
			if got.Code != http.StatusCreated {
				t.Fatalf("create API key = %d, want 201: %s", got.Code, got.Body)
			}
			data, _ := got.JSON(t)["data"].(map[string]any)
			tok, _ := data["token"].(string)
			if tok == "" {
				t.Fatalf("no token in the response: %s", got.Body)
			}
			return tok
		}
		limited := create(map[string]any{"name": "reader", "capabilities": []string{"retrieve"}})
		full := create(map[string]any{"name": "everything", "full_access": true})

		cases := []struct {
			name, key, method, path string
			want                    int
		}{
			{"scoped key on a route in its scope", limited, http.MethodGet, "/api/v1/knowledge-bases", http.StatusOK},
			{"scoped key on identity route", limited, http.MethodGet, "/api/v1/auth/me", http.StatusOK},
			// Creating a knowledge base needs manage_kbs, which this key lacks.
			{"scoped key beyond its scope", limited, http.MethodPost, "/api/v1/knowledge-bases", http.StatusForbidden},
			// Key management is not open to keys at all, not even full-access ones.
			{"scoped key on key management", limited, http.MethodGet, tenantPath, http.StatusForbidden},
			{"full-access key on a read route", full, http.MethodGet, "/api/v1/knowledge-bases", http.StatusOK},
			{"garbage key", "sk-not-a-key", http.MethodGet, "/api/v1/knowledge-bases", http.StatusUnauthorized},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := s.do(request{method: tc.method, path: tc.path, apiKey: tc.key})
				if got.Code != tc.want {
					t.Errorf("%s %s = %d, want %d: %s", tc.method, tc.path, got.Code, tc.want, got.Body)
				}
			})
		}
	})

	t.Run("X-Tenant-ID is honoured only for members", func(t *testing.T) {
		// A second, ordinary user with a workspace of their own. Public
		// registration is closed by now, so create them through the service, as
		// an administrator or an invitation would.
		var other *types.User
		must1(t, s.DI.Invoke(func(users interfaces.UserService) {
			u, err := users.Register(context.Background(), &types.RegisterRequest{
				Username: "other", Email: "other@example.com", Password: "other password",
				TenantProvisioning: types.TenantProvisioningCreatePersonal,
			})
			if err != nil {
				t.Fatalf("create second user: %v", err)
			}
			other = u
		}))
		login := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": "other@example.com", "password": "other password"}, ip: "203.0.113.15",
		})
		otherToken, _ := login.JSON(t)["token"].(string)
		if otherToken == "" {
			t.Fatalf("second user cannot log in: %s", login.Body)
		}
		if other.TenantID == 0 || other.TenantID == adminTenant {
			t.Fatalf("second user needs a workspace of their own, got %d (admin: %d)", other.TenantID, adminTenant)
		}
		adminWS := strconv.FormatUint(adminTenant, 10)
		ownWS := strconv.FormatUint(other.TenantID, 10)

		call := func(header string) int {
			return s.do(request{
				method: http.MethodGet, path: "/api/v1/knowledge-bases",
				bearer: otherToken, tenantHeader: header,
			}).Code
		}
		if got := call(""); got != http.StatusOK {
			t.Errorf("own workspace by default = %d, want 200", got)
		}
		if got := call(ownWS); got != http.StatusOK {
			t.Errorf("own workspace by header = %d, want 200", got)
		}
		if got := call(adminWS); got != http.StatusForbidden {
			t.Errorf("someone else's workspace, not a member = %d, want 403", got)
		}
		if got := call("999999"); got != http.StatusForbidden {
			t.Errorf("a workspace that does not exist = %d, want 403 (must not reveal which ids exist)", got)
		}
		if got := call("abc"); got != http.StatusBadRequest {
			t.Errorf("malformed header = %d, want 400", got)
		}

		// Membership is what opens the door.
		must1(t, s.DI.Invoke(func(members interfaces.TenantMemberService) {
			_, err := members.AddMember(context.Background(), other.ID, adminTenant, types.TenantRoleViewer, &adminID)
			if err != nil {
				t.Fatalf("add member: %v", err)
			}
		}))
		if got := call(adminWS); got != http.StatusOK {
			t.Errorf("workspace after joining as a viewer = %d, want 200", got)
		}
		// ... and only at the role granted: a viewer cannot create knowledge bases.
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/knowledge-bases", bearer: otherToken,
			tenantHeader: adminWS, body: map[string]any{"name": "sneaky"},
		})
		if got.Code != http.StatusForbidden {
			t.Errorf("a viewer creating a knowledge base = %d, want 403: %s", got.Code, got.Body)
		}
	})

	t.Run("five wrong passwords lock the account", func(t *testing.T) {
		const email = "locked@example.com"
		must1(t, s.DI.Invoke(func(users interfaces.UserService) {
			if _, err := users.Register(context.Background(), &types.RegisterRequest{
				Username: "locked", Email: email, Password: "the right password",
				TenantProvisioning: types.TenantProvisioningCreatePersonal,
			}); err != nil {
				t.Fatalf("create user: %v", err)
			}
		}))
		attempt := func(password string) response {
			return s.do(request{
				method: http.MethodPost, path: "/api/v1/auth/login",
				body: map[string]string{"email": email, "password": password}, ip: "203.0.113.20",
			})
		}
		for i := 1; i <= 5; i++ {
			if got := attempt("wrong password"); got.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d = %d, want 401", i, got.Code)
			}
		}
		// Locked: even the right password is now refused, so a guesser cannot
		// tell when they hit it, and the client is told when to come back.
		for _, password := range []string{"wrong password", "the right password"} {
			got := attempt(password)
			if got.Code != http.StatusTooManyRequests {
				t.Errorf("attempt with %q while locked = %d, want 429: %s", password, got.Code, got.Body)
			}
			secs, err := strconv.Atoi(got.Header.Get("Retry-After"))
			if err != nil || secs <= 0 {
				t.Errorf("429 lacks a usable Retry-After header: %q", got.Header.Get("Retry-After"))
			}
		}
		// Another account is unaffected by this one's lock.
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": adminEmail, "password": adminPass}, ip: "203.0.113.21",
		})
		if got.Code != http.StatusOK {
			t.Errorf("a different account's login = %d, want 200", got.Code)
		}
	})

	// Last: tokens are deterministic for one user within a second (no jti), so
	// revoking one login's token can revoke another's. Nothing runs after this.
	t.Run("a revoked token stops working", func(t *testing.T) {
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": adminEmail, "password": adminPass}, ip: "203.0.113.14",
		})
		tok, _ := got.JSON(t)["token"].(string)
		if s.do(request{method: http.MethodGet, path: "/api/v1/auth/me", bearer: tok}).Code != http.StatusOK {
			t.Fatal("fresh token was refused")
		}
		must1(t, s.DB.Model(&types.AuthToken{}).Where("token = ?", tok).Update("is_revoked", true).Error)
		code := s.do(request{method: http.MethodGet, path: "/api/v1/auth/me", bearer: tok}).Code
		if code != http.StatusUnauthorized {
			t.Errorf("revoked token = %d, want 401", code)
		}
	})
}
