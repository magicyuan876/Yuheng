package interfaces

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/types"
)

// UserService defines the user service interface
type UserService interface {
	// Register creates a new user account. It creates no workspace, except
	// for the bootstrap registration (req.BootstrapFirstUser), which creates
	// the deployment's default workspace with the registrant as its Owner.
	Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error)
	// HasAnyUser reports whether the deployment already has a user; drives the
	// "auto" registration mode (open only until the first account exists).
	HasAnyUser(ctx context.Context) (bool, error)
	// Login authenticates a user and returns tokens
	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	// GetOIDCAuthorizationURL builds the third-party OIDC authorization URL
	GetOIDCAuthorizationURL(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error)
	// LoginWithOIDC exchanges the callback code, auto-provisions users if needed, and completes login.
	LoginWithOIDC(ctx context.Context, code, redirectURI string) (*types.OIDCCallbackResponse, error)
	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs batch-fetches users by id, returning a map keyed by
	// user id. Missing ids are simply absent from the result; the call
	// is not an error when some ids resolve to no row. Used on hot list
	// endpoints (tenant members, audit logs) to avoid N+1 queries.
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// GetUserByUsername gets a user by username
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	// UpdateUser updates user information
	UpdateUser(ctx context.Context, user *types.User) error
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, id string) error
	// ChangePassword changes user password
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	// AdminResetPassword replaces a user's password without requiring the old
	// password and revokes all of that user's existing sessions. Callers must
	// enforce the system-admin and cannot-reset-self guards before invoking it.
	AdminResetPassword(ctx context.Context, userID string, newPassword string) error
	// ValidatePassword validates user password
	ValidatePassword(ctx context.Context, userID string, password string) error
	// GenerateTokens generates access and refresh tokens for user, scoped to
	// the workspace ResolveActiveTenantID picks.
	GenerateTokens(ctx context.Context, user *types.User) (accessToken, refreshToken string, err error)
	// ResolveActiveTenantID picks the workspace a session lands in when the
	// request names none (no X-Tenant-ID header, no tenant in the JWT): the
	// user's last_active_tenant_id preference if the workspace still exists
	// and the user may still act there (active membership, or
	// CanAccessAllTenants), else the earliest active membership, which is
	// then written back as the preference, best effort. Returns 0 for a user
	// who belongs to no workspace. Never fails: a lookup error only narrows
	// the result.
	ResolveActiveTenantID(ctx context.Context, user *types.User) uint64
	// RememberFirstWorkspace records tenantID as the user's
	// last_active_tenant_id when they have no preference yet, so a user whose
	// first workspace came through an invitation lands there on their next
	// login instead of going through the membership fallback. A user who
	// already has a preference keeps it.
	RememberFirstWorkspace(ctx context.Context, userID string, tenantID uint64) error
	// BuildLoginMemberships projects the user's tenant memberships into
	// the login-response shape. activeTenant is reused (without an extra
	// lookup) for the matching row's TenantName. The slice is guaranteed
	// non-nil so callers can serialise it as an empty JSON array when the
	// membership table is unavailable.
	BuildLoginMemberships(ctx context.Context, user *types.User, activeTenant *types.Tenant) []types.Membership
	// SwitchTenant issues a new token pair scoped to targetTenantID and
	// returns the corresponding LoginResponse. The caller's previous
	// refresh token (passed in for revocation) is invalidated. The user
	// must have an active membership in the target workspace unless they
	// CanAccessAllTenants.
	SwitchTenant(ctx context.Context, user *types.User, targetTenantID uint64, currentRefreshToken string) (*types.LoginResponse, error)
	// ValidateToken validates an access token. It returns the user
	// referenced by the token plus the workspace ID encoded in the JWT's
	// `tenant_id` claim, or 0 when the token carries none (a tenantless
	// session); the auth middleware then falls back to
	// ResolveActiveTenantID.
	ValidateToken(ctx context.Context, token string) (*types.User, uint64, error)
	// RefreshToken refreshes access token using refresh token
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token string) error
	// Logout revokes every outstanding access/refresh token for the user
	// identified by the presented JWT.
	Logout(ctx context.Context, token string) error
	// GetCurrentUser gets current user from context
	GetCurrentUser(ctx context.Context) (*types.User, error)
	// ListSystemAdmins lists users with IsSystemAdmin=true.
	// Returns the page of admins plus the total count (for pagination UI);
	// callers pass offset/limit to page through results. Used by the
	// /api/v1/system/admin/list endpoint, gated to SystemAdmin callers.
	ListSystemAdmins(ctx context.Context, offset, limit int) ([]*types.User, int64, error)
	// AdminCreateUser provisions a new local user on behalf of a
	// SystemAdmin. When req.Password is nil, a random password is generated
	// and returned exactly once as the second result. The account belongs to
	// no workspace until it is added to one.
	AdminCreateUser(ctx context.Context, req *types.AdminCreateUserRequest) (*types.User, string, error)
	// RevokeSystemAdmin removes system-admin privileges with the
	// last-admin/self-revoke checks performed atomically.
	RevokeSystemAdmin(ctx context.Context, userID, actorID string) (*types.User, error)
	// UpdateUserPreferences partially updates the calling user's
	// preferences blob (PATCH semantics: only keys present in `patch`
	// overwrite existing values). Returns the updated, persisted prefs.
	UpdateUserPreferences(ctx context.Context, userID string, patch types.UserPreferences) (types.UserPreferences, error)
}

// UserRepository defines the user repository interface
type UserRepository interface {
	// CreateUser creates a user
	CreateUser(ctx context.Context, user *types.User) error
	// HasAnyUser reports whether at least one user exists (cheap EXISTS probe).
	HasAnyUser(ctx context.Context) (bool, error)
	// BootstrapFirstUser creates the deployment's first account and its default
	// workspace in one transaction: the user as system administrator, the
	// workspace, an Owner membership and the user's last_active_tenant_id
	// preference pointing at it. Only if no user exists yet; otherwise it
	// returns types.ErrRegistrationClosed and writes nothing.
	BootstrapFirstUser(ctx context.Context, user *types.User, workspace *types.Tenant) error
	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs batch-fetches users by id, returning a map keyed by
	// user id. Missing ids are simply absent from the result.
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// GetUserByUsername gets a user by username
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *types.User) error
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, id string) error
	// ListUsers lists users with pagination
	ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error)
	// ListSystemAdmins lists users where is_system_admin = true.
	// Walks the partial-friendly idx_users_is_system_admin index. Returns
	// the slice plus the total count for pagination metadata. Used by
	// the system-admin management endpoint.
	ListSystemAdmins(ctx context.Context, offset, limit int) ([]*types.User, int64, error)
	// RevokeSystemAdmin removes system-admin privileges with the
	// last-admin/self-revoke checks performed atomically.
	RevokeSystemAdmin(ctx context.Context, userID, actorID string) (*types.User, error)
}

// AuthTokenRepository defines the auth token repository interface
type AuthTokenRepository interface {
	// CreateToken creates an auth token
	CreateToken(ctx context.Context, token *types.AuthToken) error
	// GetTokenByValue gets a token by its value
	GetTokenByValue(ctx context.Context, tokenValue string) (*types.AuthToken, error)
	// GetTokensByUserID gets all tokens for a user
	GetTokensByUserID(ctx context.Context, userID string) ([]*types.AuthToken, error)
	// UpdateToken updates a token
	UpdateToken(ctx context.Context, token *types.AuthToken) error
	// DeleteToken deletes a token
	DeleteToken(ctx context.Context, id string) error
	// DeleteExpiredTokens deletes all expired tokens
	DeleteExpiredTokens(ctx context.Context) error
	// RevokeTokensByUserID revokes all tokens for a user
	RevokeTokensByUserID(ctx context.Context, userID string) error
}
