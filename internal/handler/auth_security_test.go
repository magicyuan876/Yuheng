package handler

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

func cfgWithMode(mode string) *config.Config {
	return &config.Config{Auth: &config.AuthConfig{RegistrationMode: mode}}
}

// TestRegister_AutoMode covers the default policy: open only while the
// deployment has no user, closed afterwards and on any doubt.
func TestRegister_AutoMode(t *testing.T) {
	const (
		auto   = config.AuthRegistrationModeAuto
		self   = config.AuthRegistrationModeSelfServe
		invite = config.AuthRegistrationModeInviteOnly
	)
	dbDown := stderrors.New("db down")
	closed := types.ErrRegistrationClosed
	cases := []struct {
		name          string
		mode          string
		hasUser       bool
		hasUserErr    error
		registerErr   error
		wantStatus    int
		wantBootstrap bool
		wantCalled    bool
	}{
		{"auto, empty deployment: first registrant bootstraps", auto, false, nil, nil, 201, true, true},
		{"unset mode behaves as auto (empty)", "", false, nil, nil, 201, true, true},
		{"auto, users exist: closed", auto, true, nil, nil, 403, false, false},
		{"unset mode, users exist: closed", "", true, nil, nil, 403, false, false},
		{"unknown mode never opens the door", "open_sesame", true, nil, nil, 403, false, false},
		{"auto, probe fails: closed", auto, false, dbDown, nil, 403, false, false},
		{"auto, lost the first-user race: closed", auto, false, nil, closed, 403, true, true},
		{"self_serve stays open with users", self, true, nil, nil, 201, false, true},
		{"invite_only stays closed on empty deployment", invite, false, nil, nil, 403, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var gotBootstrap bool
			us := &stubRegisterUserService{
				hasUser: tc.hasUser, hasUserErr: tc.hasUserErr,
				register: func(_ context.Context, req *types.RegisterRequest) (*types.User, error) {
					called = true
					gotBootstrap = req.BootstrapFirstUser
					if tc.registerErr != nil {
						return nil, tc.registerErr
					}
					return &types.User{ID: "u1", Email: "alice@example.com"}, nil
				},
			}
			h := NewAuthHandler(cfgWithMode(tc.mode), us, nil, nil, nil)
			w := doRegister(t, newRegisterTestRouter(h), validRegisterBody())
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if called != tc.wantCalled {
				t.Fatalf("Register called = %v, want %v", called, tc.wantCalled)
			}
			if called && gotBootstrap != tc.wantBootstrap {
				t.Fatalf("BootstrapFirstUser = %v, want %v", gotBootstrap, tc.wantBootstrap)
			}
			if tc.wantStatus == http.StatusForbidden && !strings.Contains(w.Body.String(), "administrator") {
				t.Fatalf("closed-registration message must say how to get an account, got %s", w.Body.String())
			}
		})
	}
}

// TestRegisterByInvite_WorksWhileRegistrationClosed pins the contract that
// closing public registration (any mode) leaves the invitation path open.
func TestRegisterByInvite_WorksWhileRegistrationClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{
		config.AuthRegistrationModeInviteOnly, config.AuthRegistrationModeAuto,
	} {
		t.Run(mode, func(t *testing.T) {
			users := &invitedRegistrationUserService{}
			h := NewAuthHandler(cfgWithMode(mode), users, &invitedRegistrationTenantService{}, nil,
				&invitedRegistrationInvitationService{})
			r := gin.New()
			r.Use(errorCapture())
			r.POST("/auth/register-by-invite", h.RegisterByInvite)
			r.POST("/auth/register", h.Register)

			body := []byte(`{"token":"invite-token","email":"alice@example.com",` +
				`"username":"alice","password":"supersecret"}`)
			req := httptest.NewRequest(http.MethodPost, "/auth/register-by-invite", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusCreated {
				t.Fatalf("invitation registration = %d body=%s", w.Code, w.Body.String())
			}
			if users.registered == nil || users.registered.BootstrapFirstUser {
				t.Fatalf("invitee must register as an ordinary account, got %+v", users.registered)
			}
		})
	}
}

func getAuthConfig(t *testing.T, h *AuthHandler) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/config", h.GetAuthConfig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGetAuthConfig_ReportsEffectiveRegistrationState(t *testing.T) {
	cases := []struct {
		name         string
		mode         string
		hasUser      bool
		wantOpen     bool
		wantFirst    bool
		wantEffMode  string
		wantConfMode string
	}{
		{"auto, fresh install", "auto", false, true, true, "self_serve", "auto"},
		{"auto, has users", "auto", true, false, false, "invite_only", "auto"},
		{"self_serve", "self_serve", true, true, false, "self_serve", "self_serve"},
		{"invite_only on fresh install", "invite_only", false, false, false, "invite_only", "invite_only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			us := &stubRegisterUserService{hasUser: tc.hasUser}
			h := NewAuthHandler(cfgWithMode(tc.mode), us, nil, nil, nil)
			got := getAuthConfig(t, h)
			if got["registration_open"] != tc.wantOpen {
				t.Errorf("registration_open = %v, want %v", got["registration_open"], tc.wantOpen)
			}
			if got["first_user"] != tc.wantFirst {
				t.Errorf("first_user = %v, want %v", got["first_user"], tc.wantFirst)
			}
			// Legacy clients hide the sign-up entry on "invite_only".
			if got["registration_mode"] != tc.wantEffMode {
				t.Errorf("registration_mode = %v, want %v", got["registration_mode"], tc.wantEffMode)
			}
			if got["configured_registration_mode"] != tc.wantConfMode {
				t.Errorf("configured_registration_mode = %v, want %v",
					got["configured_registration_mode"], tc.wantConfMode)
			}
		})
	}
}

// ---- login -----------------------------------------------------------

// loginUsers is a UserService whose Login knows one account.
type loginUsers struct {
	interfaces.UserService
	email, password string
	calls           int
	err             error
}

func (u *loginUsers) Login(_ context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	u.calls++
	if u.err != nil {
		return nil, u.err
	}
	if req.Email == u.email && req.Password == u.password {
		return &types.LoginResponse{
			Success: true, Message: "Login successful",
			User: &types.User{ID: "u1", Email: u.email}, Token: "t", RefreshToken: "r",
		}, nil
	}
	// Same message and shape for an unknown address and a wrong password.
	return &types.LoginResponse{Success: false, Message: "Invalid email or password"}, nil
}

func doLogin(r *gin.Engine, email, password string) *httptest.ResponseRecorder {
	buf, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newLoginRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/auth/login", h.Login)
	return r
}

func newLoginHandler(us interfaces.UserService, clk func() time.Time) *AuthHandler {
	h := NewAuthHandler(cfgWithMode(config.AuthRegistrationModeSelfServe), us, nil, nil, nil)
	h.loginLockout.SetClock(clk)
	return h
}

func TestLogin_UnknownUserAndWrongPasswordLookIdentical(t *testing.T) {
	us := &loginUsers{email: "alice@example.com", password: "right-password1"}
	h := newLoginHandler(us, time.Now)
	r := newLoginRouter(h)

	unknown := doLogin(r, "nobody@example.com", "whatever123")
	wrong := doLogin(r, "alice@example.com", "wrong-password1")
	if unknown.Code != wrong.Code || unknown.Body.String() != wrong.Body.String() {
		t.Fatalf("responses differ:\n unknown: %d %s\n wrong:   %d %s",
			unknown.Code, unknown.Body.String(), wrong.Code, wrong.Body.String())
	}
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", unknown.Code)
	}
}

func TestLogin_LocksAfterFiveFailuresForKnownAndUnknownAlike(t *testing.T) {
	clk := &testClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	us := &loginUsers{email: "alice@example.com", password: "right-password1"}
	h := newLoginHandler(us, clk.now)
	r := newLoginRouter(h)

	for _, email := range []string{"alice@example.com", "ghost@example.com"} {
		for i := 0; i < 5; i++ {
			if w := doLogin(r, email, "wrong-password1"); w.Code != http.StatusUnauthorized {
				t.Fatalf("%s attempt %d = %d, want 401", email, i+1, w.Code)
			}
		}
	}
	callsBefore := us.calls
	locked := doLogin(r, "alice@example.com", "right-password1") // even the RIGHT password
	ghost := doLogin(r, "ghost@example.com", "right-password1")
	if locked.Code != http.StatusTooManyRequests || ghost.Code != http.StatusTooManyRequests {
		t.Fatalf("locked accounts must answer 429, got %d and %d", locked.Code, ghost.Code)
	}
	if locked.Body.String() != ghost.Body.String() {
		t.Fatalf("locked known/unknown answers differ:\n%s\n%s", locked.Body.String(), ghost.Body.String())
	}
	if locked.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
	if us.calls != callsBefore {
		t.Fatal("a locked account must not reach the user service (no bcrypt work, no oracle)")
	}

	clk.advance(16 * time.Minute)
	if w := doLogin(r, "alice@example.com", "right-password1"); w.Code != http.StatusOK {
		t.Fatalf("after the lock expires the right password works, got %d", w.Code)
	}
}

func TestLogin_SuccessClearsFailureCounter(t *testing.T) {
	clk := &testClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	us := &loginUsers{email: "alice@example.com", password: "right-password1"}
	r := newLoginRouter(newLoginHandler(us, clk.now))

	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			doLogin(r, "alice@example.com", "wrong-password1")
		}
		if w := doLogin(r, "alice@example.com", "right-password1"); w.Code != http.StatusOK {
			t.Fatalf("round %d: success after 4 failures = %d", round, w.Code)
		}
	}
}

func TestLogin_ServiceErrorIsGenericAndDetailless(t *testing.T) {
	us := &loginUsers{err: stderrors.New("pq: password authentication failed for user \"yuheng\" at 10.1.2.3")}
	r := newLoginRouter(newLoginHandler(us, time.Now))
	w := doLogin(r, "alice@example.com", "whatever123")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
	for _, leak := range []string{"pq:", "10.1.2.3", "yuheng"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Fatalf("login error leaks internals (%q): %s", leak, w.Body.String())
		}
	}
	// An internal fault is not a wrong password, so it must not count.
	for i := 0; i < 10; i++ {
		if w := doLogin(r, "alice@example.com", "whatever123"); w.Code == http.StatusTooManyRequests {
			t.Fatal("service errors must not lock the account")
		}
	}
}

func TestLogin_NilLockoutHandlerStillWorks(t *testing.T) {
	us := &loginUsers{email: "alice@example.com", password: "right-password1"}
	h := &AuthHandler{userService: us}
	if w := doLogin(newLoginRouter(h), "alice@example.com", "right-password1"); w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

// testClock is a manually advanced clock.
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }
