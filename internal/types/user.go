package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserPreferences holds per-user preferences persisted server-side
// so they sync across devices/browsers. Fields are pointers so we can
// distinguish "client didn't send this key" (leave existing value alone)
// from "client explicitly set false" — the partial-update merge in
// UpdateUserPreferences relies on this.
//
// Adding a new preference key:
//  1. Add a *T field below + JSON tag (snake_case, must match the front-end key).
//  2. Extend the merge logic in service.UserService.UpdateUserPreferences.
//  3. Surface the new knob in the frontend settings store.
//
// No DB DDL is required — preferences is a single jsonb column.
type UserPreferences struct {
	// LastActiveTenantID is the user's current workspace: the one a fresh
	// login (new device, cleared browser, new refresh token) lands in and the
	// one a request without an X-Tenant-ID header and without a tenant in its
	// JWT is scoped to. A user is a global identity with no "home" workspace;
	// this preference is the only pointer from a user to a workspace, and
	// tenant_members decides whether it may be honoured. UserService.
	// ResolveActiveTenantID validates it (the workspace must exist and the
	// user must still have an active membership, or CanAccessAllTenants)
	// and otherwise falls back to the earliest active membership, rewriting
	// the preference so later logins do not repeat the lookup.
	//
	// nil  = no preference (resolve from memberships)
	// *0   = "clear preference" sentinel for the partial-update endpoint
	//        (UpdateUserPreferences turns this into nil). Otherwise treat
	//        a stored *0 the same as nil.
	// *N   = preferred workspace id.
	LastActiveTenantID *uint64 `json:"last_active_tenant_id,omitempty"`

	// OidcOnlyLogin is set server-side when an account is auto-provisioned
	// via OIDC with a random password the user never received. The profile
	// UI hides self-service password rotation until the user sets a known
	// password via ChangePassword (which clears this flag).
	OidcOnlyLogin *bool `json:"oidc_only_login,omitempty"`
}

// Value implements driver.Valuer so GORM persists UserPreferences as
// JSON in the Postgres jsonb column. Empty struct serialises
// to "{}", matching the NOT NULL DEFAULT '{}' column constraint.
func (p UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements sql.Scanner so GORM can hydrate UserPreferences back
// from the underlying column. Accept []byte (how the Postgres driver hands
// back jsonb) and string (a driver may hand text as string), so the type
// does not depend on one driver's choice.
func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*p = UserPreferences{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("UserPreferences.Scan: unsupported type")
	}
	if len(data) == 0 {
		*p = UserPreferences{}
		return nil
	}
	return json.Unmarshal(data, p)
}

// User represents a user in the system
type User struct {
	// Unique identifier of the user
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// Username of the user
	Username string `json:"username"   gorm:"type:varchar(100);uniqueIndex;not null"`
	// Email address of the user
	Email string `json:"email"      gorm:"type:varchar(255);uniqueIndex;not null"`
	// Hashed password of the user
	PasswordHash string `json:"-"          gorm:"type:varchar(255);not null"`
	// Avatar URL of the user
	Avatar string `json:"avatar"     gorm:"type:varchar(500)"`
	// Whether the user is active
	IsActive bool `json:"is_active"  gorm:"default:true"`
	// Whether the user can access all workspaces (cross-workspace access)
	CanAccessAllTenants bool `json:"can_access_all_tenants" gorm:"default:false"`
	// Whether the user is a system administrator (independent of workspace roles)
	IsSystemAdmin bool `json:"is_system_admin" gorm:"default:false;index"`
	// Per-user UI/feature preferences.
	// Stored as JSON in a jsonb column via the
	// driver.Valuer / sql.Scanner methods on UserPreferences.
	Preferences UserPreferences `json:"preferences" gorm:"type:jsonb;not null;default:'{}'"`
	// Creation time of the user
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the user
	UpdatedAt time.Time `json:"updated_at"`
	// Deletion time of the user
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// AuthToken represents an authentication token
type AuthToken struct {
	// Unique identifier of the token
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// User ID that owns this token
	UserID string `json:"user_id"    gorm:"type:varchar(36);index;not null"`
	// Token value (JWT or other format)
	Token string `json:"token"      gorm:"type:text;not null"`
	// Token type (access_token, refresh_token)
	TokenType string `json:"token_type" gorm:"type:varchar(50);not null"`
	// Token expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// Whether the token is revoked
	IsRevoked bool `json:"is_revoked" gorm:"default:false"`
	// Creation time of the token
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the token
	UpdatedAt time.Time `json:"updated_at"`

	// Association relationship
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type OIDCAuthURLResponse struct {
	Success             bool   `json:"success"`
	ProviderDisplayName string `json:"provider_display_name,omitempty"`
	AuthorizationURL    string `json:"authorization_url,omitempty"`
	State               string `json:"state,omitempty"`
	// Nonce is bound to an HttpOnly cookie on /auth/oidc/url and verified
	// on callback; omitted from JSON so clients cannot replay it alone.
	Nonce string `json:"-"`
}

type OIDCConfigResponse struct {
	Success             bool   `json:"success"`
	Enabled             bool   `json:"enabled"`
	ProviderDisplayName string `json:"provider_display_name,omitempty"`
}

type OIDCCallbackResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
	// ActiveTenant carries the active tenant for the issued token, named as
	// in LoginResponse so both login flows share one response shape.
	ActiveTenant *Tenant `json:"active_tenant,omitempty"`
	// Memberships mirrors LoginResponse.Memberships so the OIDC flow
	// produces the same role information available to password logins.
	// Always populated (length >= 1 for an authenticated user).
	Memberships  []Membership `json:"memberships"`
	Token        string       `json:"token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	IsNewUser    bool         `json:"is_new_user,omitempty"`
}

type OIDCUserInfo struct {
	Subject  string                 `json:"subject,omitempty"`
	Username string                 `json:"username,omitempty"`
	Email    string                 `json:"email,omitempty"`
	Claims   map[string]interface{} `json:"claims,omitempty"`
}

// ErrRegistrationClosed is returned when public registration is not (or is no
// longer) open: in "auto" mode once the first user exists, or in "invite_only"
// mode. Handlers translate it to 403.
var ErrRegistrationClosed = errors.New("registration is closed")

// DefaultWorkspaceName names the workspace the bootstrap registration creates
// when the registrant leaves the name empty. English, like every other default
// the backend writes; the registration page localises its placeholder.
const DefaultWorkspaceName = "Default Workspace"

// RegisterRequest represents a registration request. Registration creates an
// account and nothing else: a user is a global identity that enters workspaces
// through invitations or an administrator. The one exception is the bootstrap
// registration below, which is an installation step rather than a policy.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=2,max=50"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`

	// WorkspaceName names the deployment's default workspace. It is honoured
	// only when BootstrapFirstUser is set and is ignored otherwise; empty
	// means DefaultWorkspaceName. The bound is the one POST /tenants applies.
	WorkspaceName string `json:"workspace_name" binding:"omitempty,max=128"`

	// BootstrapFirstUser is server-controlled (never bound from JSON): the
	// account is created as system administrator together with the
	// deployment's default workspace, atomically, and only if no user exists
	// yet. Set by the handler in "auto" registration mode.
	BootstrapFirstUser bool `json:"-"`
}

// AdminCreateUserRequest is the payload for a SystemAdmin provisioning a
// new local user via POST /api/v1/system/admin/users/create.
//
// Password is optional: when absent (or null), the service generates a
// random one and returns it exactly once. Any provided value, the
// empty string included, is subject to the password policy.
//
// TenantID and Role are optional and go together: when TenantID is set
// the new account is added to that workspace in the same request, with
// Role (default viewer). Without them the account belongs to no
// workspace until an administrator adds it to one.
type AdminCreateUserRequest struct {
	Username string     `json:"username" binding:"required,min=2,max=50"`
	Email    string     `json:"email"    binding:"required,email"`
	Password *string    `json:"password"`
	TenantID uint64     `json:"tenant_id"`
	Role     TenantRole `json:"role"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
	// ActiveTenant is the workspace whose ID is encoded in the issued JWT;
	// future requests are scoped to it until the client calls /auth/switch-tenant.
	// Nil for a user who belongs to no workspace yet (see
	// UserService.ResolveActiveTenantID for how it is chosen).
	ActiveTenant *Tenant `json:"active_tenant,omitempty"`
	// Memberships lists every workspace the user can authenticate into,
	// along with their role in each. Always populated (possibly empty for a
	// user who belongs to no workspace yet) so frontends can render a
	// workspace switcher without a follow-up request. Serialised without
	// omitempty so the field is always present as a JSON array (possibly
	// empty) — the "always populated" contract relies on the server side
	// guaranteeing a non-nil slice.
	Memberships  []Membership `json:"memberships"`
	Token        string       `json:"token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	Success bool    `json:"success"`
	Message string  `json:"message,omitempty"`
	User    *User   `json:"user,omitempty"`
	Tenant  *Tenant `json:"tenant,omitempty"`
}

// UserInfo represents user information for API responses. It carries no
// workspace: the current workspace is the `tenant` object next to it in the
// login and /auth/me responses, resolved per request from the token and the
// user's memberships.
type UserInfo struct {
	ID                  string          `json:"id"`
	Username            string          `json:"username"`
	Email               string          `json:"email"`
	Avatar              string          `json:"avatar"`
	IsActive            bool            `json:"is_active"`
	CanAccessAllTenants bool            `json:"can_access_all_tenants"`
	IsSystemAdmin       bool            `json:"is_system_admin"`
	Preferences         UserPreferences `json:"preferences"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// ToUserInfo converts User to UserInfo (without sensitive data)
func (u *User) ToUserInfo() *UserInfo {
	return &UserInfo{
		ID:                  u.ID,
		Username:            u.Username,
		Email:               u.Email,
		Avatar:              u.Avatar,
		IsActive:            u.IsActive,
		CanAccessAllTenants: u.CanAccessAllTenants,
		IsSystemAdmin:       u.IsSystemAdmin,
		Preferences:         u.Preferences,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
}
