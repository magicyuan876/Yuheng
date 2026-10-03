package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

var (
	jwtSecretOnce sync.Once
	jwtSecret     string

	// ErrUserEmailExists is returned by Register when the target entity's
	// email already exists.
	ErrUserEmailExists = errors.New("user with this email already exists")

	// ErrUserUsernameExists is returned by Register when the target entity's
	// username already exists.
	ErrUserUsernameExists = errors.New("user with this username already exists")

	// ErrUserIdentityConflict is returned by AdminCreateUser when only part
	// of the requested identity (email or username) collides with an existing
	// user, so a blind idempotent retry would return the wrong account.
	ErrUserIdentityConflict = errors.New("email and username refer to conflicting existing identities")

	// ErrPasswordPolicy is returned when a newly chosen password does not
	// meet the product's public 8-32 character, letter-and-number contract.
	// It is exported so HTTP handlers can translate the failure to a 400
	// without exposing bcrypt or persistence errors.
	ErrPasswordPolicy = errors.New("password must be 8-32 characters and contain at least one letter and one number")

	// ErrInvalidOldPassword is returned by ChangePassword when the supplied
	// current password does not match the stored hash. Handlers map this to
	// a 400 so callers can prompt the user without treating it as a 500.
	ErrInvalidOldPassword = errors.New("invalid old password")

	// ErrSamePassword is returned when the new password equals the current
	// one so callers can reject no-op rotations that would still revoke
	// every session.
	ErrSamePassword = errors.New("new password must differ from current password")
)

// Machine-readable change-password failure reasons for HTTP details fields.
const (
	DetailInvalidOldPassword = "invalid_old_password"
	DetailPasswordPolicy     = "password_policy"
	DetailSamePassword       = "same_password"
)

// ValidatePasswordPolicy keeps administrative password resets aligned with
// the registration form's documented policy. Password bytes are never logged
// or included in the returned error.
func ValidatePasswordPolicy(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 8 || length > 32 {
		return ErrPasswordPolicy
	}
	hasLetter := false
	hasNumber := false
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return ErrPasswordPolicy
	}
	return nil
}

// getJwtSecret retrieves the JWT secret from the environment, falling back to a securely generated random secret.
func getJwtSecret() string {
	jwtSecretOnce.Do(func() {
		if envSecret := strings.TrimSpace(os.Getenv("JWT_SECRET")); envSecret != "" {
			jwtSecret = envSecret
			return
		}

		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			panic(fmt.Sprintf("failed to generate JWT secret: %v", err))
		}
		jwtSecret = base64.StdEncoding.EncodeToString(randomBytes)
	})

	return jwtSecret
}

var (
	dummyHashOnce  sync.Once
	dummyHashBytes []byte
)

// dummyPasswordHash returns a bcrypt hash (at the same cost real passwords
// use) of a random value, computed once. Login compares against it when the
// e-mail is unknown so that unknown and wrong-password attempts take the same
// time.
func dummyPasswordHash() []byte {
	dummyHashOnce.Do(func() {
		random := make([]byte, 16)
		_, _ = rand.Read(random)
		h, err := bcrypt.GenerateFromPassword(random, bcrypt.DefaultCost)
		if err != nil {
			// Cannot happen for a 16-byte input; a fixed valid hash keeps the
			// comparison expensive even so.
			h = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5BCSq4EFUR0jy7r3vJ8U4PzQwXW2K")
		}
		dummyHashBytes = h
	})
	return dummyHashBytes
}

// HasAnyUser reports whether the deployment has at least one user account.
func (s *userService) HasAnyUser(ctx context.Context) (bool, error) {
	return s.userRepo.HasAnyUser(ctx)
}

// userService implements the UserService interface
type userService struct {
	userRepo      interfaces.UserRepository
	tokenRepo     interfaces.AuthTokenRepository
	tenantService interfaces.TenantService
	memberService interfaces.TenantMemberService
	config        *config.Config
}

// NewUserService creates a new user service instance
func NewUserService(
	configInfo *config.Config,
	userRepo interfaces.UserRepository,
	tokenRepo interfaces.AuthTokenRepository,
	tenantService interfaces.TenantService,
	memberService interfaces.TenantMemberService,
) interfaces.UserService {
	return &userService{
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		tenantService: tenantService,
		memberService: memberService,
		config:        configInfo,
	}
}

// Register creates a new user account and nothing else: a user is a global
// identity, and membership in a workspace comes from an invitation or an
// administrator, never from registering. The bootstrap registration is the
// one exception: the deployment's first account also creates its default
// workspace and owns it, because somebody has to be able to invite everyone
// else. That is an installation step, done atomically by the repository.
func (s *userService) Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error) {
	logger.Info(ctx, "Start user registration")

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email and password are required")
	}

	// Check if user already exists
	existingUser, _ := s.userRepo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, ErrUserEmailExists
	}

	existingUser, _ = s.userRepo.GetUserByUsername(ctx, req.Username)
	if existingUser != nil {
		return nil, ErrUserUsernameExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Errorf(ctx, "Failed to hash password: %v", err)
		return nil, errors.New("failed to process password")
	}

	user := &types.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if req.BootstrapFirstUser {
		// User, workspace, Owner membership and the preference pointing at
		// the workspace commit together, or not at all; see
		// UserRepository.BootstrapFirstUser for why the "only if the table
		// is empty" check lives in the same transaction.
		err = s.userRepo.BootstrapFirstUser(ctx, user, newDefaultWorkspace(req.WorkspaceName))
	} else {
		err = s.userRepo.CreateUser(ctx, user)
	}
	if err != nil {
		if errors.Is(err, types.ErrRegistrationClosed) {
			// Lost the race for the first account: not a server fault.
			return nil, types.ErrRegistrationClosed
		}
		logger.Errorf(ctx, "Failed to create user: %v", err)
		return nil, errors.New("failed to create user")
	}

	logger.Info(ctx, "User registered successfully")
	return user, nil
}

// newDefaultWorkspace describes the deployment's default workspace for the
// bootstrap registration. The name is the registrant's choice, falling back
// to types.DefaultWorkspaceName; everything else takes the same defaults
// TenantService.CreateTenant applies (RetrieverEngines is left empty so the
// deployment's RETRIEVE_DRIVER defaults apply; storage quota and backend
// take the column defaults).
func newDefaultWorkspace(name string) *types.Tenant {
	name = strings.TrimSpace(name)
	if name == "" {
		name = types.DefaultWorkspaceName
	}
	now := time.Now()
	return &types.Tenant{
		Name:                    secutils.SanitizeForLog(name),
		Description:             "Default workspace of this deployment",
		Status:                  "active",
		DefaultStorageBackendID: types.EnvStorageBackendID,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// Login authenticates a user and returns tokens
func (s *userService) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	logger.Info(ctx, "Start user login")
	// Get user by email
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil || user == nil {
		if err != nil {
			logger.Errorf(ctx, "Failed to get user by email: %v", err)
		} else {
			logger.Warn(ctx, "User not found for email")
		}
		// Spend the same bcrypt time an existing account would, so response
		// latency does not reveal whether the e-mail is registered.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(req.Password))
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		}, nil
	}

	// Verify the password before looking at account status: a disabled
	// account must not confirm its existence to someone who does not hold
	// the password.
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		logger.Warn(ctx, "Password verification failed")
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		}, nil
	}
	if !user.IsActive {
		logger.Warn(ctx, "User account is disabled")
		return &types.LoginResponse{
			Success: false,
			Message: "Account is disabled",
		}, nil
	}
	logger.Info(ctx, "Password verification successful")

	// Generate tokens. Resolve the target tenant once so the JWT claim
	// and the tenant we return below agree — otherwise an honoured
	// "last active tenant" preference would mint a token for tenant N
	// but tell the client they are somewhere else.
	logger.Info(ctx, "Generating tokens")
	resolvedTenantID := s.ResolveActiveTenantID(ctx, user)
	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, resolvedTenantID)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate tokens: %v", err)
		return &types.LoginResponse{
			Success: false,
			Message: "Login failed",
		}, nil
	}
	logger.Info(ctx, "Tokens generated successfully")

	// Get tenant information. A zero resolved ID is a valid tenantless
	// identity, not a failed tenant lookup.
	var tenant *types.Tenant
	if resolvedTenantID > 0 {
		tenant, err = s.tenantService.GetTenantByID(ctx, resolvedTenantID)
		if err != nil {
			logger.Warn(ctx, "Failed to get tenant info")
		} else {
			logger.Info(ctx, "Tenant information retrieved successfully")
		}
	}

	memberships := s.buildMembershipsForUser(ctx, user, tenant)

	logger.Info(ctx, "User logged in successfully")
	return &types.LoginResponse{
		Success:      true,
		Message:      "Login successful",
		User:         user,
		ActiveTenant: tenant,
		Memberships:  memberships,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// buildMembershipsForUser returns the user's tenant memberships projected
// into the login-response shape. activeTenant (if non-nil and matching one
// of the rows) is used to reuse its already-fetched name without a second
// DB lookup; other tenants are looked up in one batch. Errors are logged
// but never propagated — a missing memberships array degrades gracefully
// to length 0 rather than failing the whole login. tenant_members is the
// only source: an empty result is authoritative and nothing is invented
// from the user row (that is what kept removed workspaces visible in the
// workspace switcher, #2586).
func (s *userService) BuildLoginMemberships(
	ctx context.Context,
	user *types.User,
	activeTenant *types.Tenant,
) []types.Membership {
	return s.buildMembershipsForUser(ctx, user, activeTenant)
}

func (s *userService) buildMembershipsForUser(
	ctx context.Context,
	user *types.User,
	activeTenant *types.Tenant,
) []types.Membership {
	if user == nil || s.memberService == nil {
		return []types.Membership{}
	}
	rows, err := s.memberService.ListByUser(ctx, user.ID)
	if err != nil {
		logger.Warnf(ctx, "Failed to list memberships for user %s: %v", user.ID, err)
		return []types.Membership{}
	}
	if len(rows) == 0 {
		return []types.Membership{}
	}
	// 收集需要批量查询名称的 tenant id（跳过 activeTenant 因为它已经在手）。
	needsLookup := make([]uint64, 0, len(rows))
	for _, m := range rows {
		if m == nil || m.Status != types.TenantMemberStatusActive {
			continue
		}
		if activeTenant != nil && m.TenantID == activeTenant.ID {
			continue
		}
		needsLookup = append(needsLookup, m.TenantID)
	}
	tenantByID := map[uint64]*types.Tenant{}
	if len(needsLookup) > 0 {
		if found, terr := s.tenantService.GetTenantsByIDs(ctx, needsLookup); terr == nil {
			tenantByID = found
		} else {
			logger.Warnf(ctx, "Failed to batch-load tenants for memberships (user=%s): %v",
				user.ID, terr)
		}
	}

	out := make([]types.Membership, 0, len(rows))
	for _, m := range rows {
		if m == nil || m.Status != types.TenantMemberStatusActive {
			continue
		}
		name := ""
		if activeTenant != nil && m.TenantID == activeTenant.ID {
			name = activeTenant.Name
		} else if t, ok := tenantByID[m.TenantID]; ok && t != nil {
			name = t.Name
		}
		// Drop memberships whose tenant row is gone (deleted tenant or
		// stale tenant_members left over from before cascade delete).
		if strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, types.Membership{
			TenantID:   m.TenantID,
			TenantName: name,
			Role:       m.Role,
		})
	}
	return out
}

// GetOIDCAuthorizationURL builds the OIDC authorization URL.
func (s *userService) GetOIDCAuthorizationURL(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error) {
	cfg, err := s.getOIDCConfig(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("redirect_uri is required")
	}

	nonce, err := generateRandomString(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	state, err := secutils.SignOIDCState(&secutils.OIDCStatePayload{
		Nonce:       nonce,
		RedirectURI: strings.TrimSpace(redirectURI),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode OIDC state: %w", err)
	}

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(cfg.Scopes, " "))
	query.Set("state", state)

	authURL := cfg.AuthorizationEndpoint
	if strings.Contains(authURL, "?") {
		authURL += "&" + query.Encode()
	} else {
		authURL += "?" + query.Encode()
	}

	return &types.OIDCAuthURLResponse{
		Success:             true,
		ProviderDisplayName: cfg.ProviderDisplayName,
		AuthorizationURL:    authURL,
		State:               state,
		Nonce:               nonce,
	}, nil
}

// LoginWithOIDC exchanges code for tokens, loads user info, provisions user if
// needed, and returns local login tokens. A first-time OIDC login creates the
// account only; the user joins workspaces the same way a password user does.
func (s *userService) LoginWithOIDC(
	ctx context.Context,
	code, redirectURI string,
) (*types.OIDCCallbackResponse, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("code is required")
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("redirect_uri is required")
	}

	cfg, err := s.getOIDCConfig(ctx)
	if err != nil {
		return nil, err
	}

	tokenResp, err := s.exchangeOIDCCode(ctx, cfg, code, redirectURI)
	if err != nil {
		return nil, err
	}

	userInfo, err := s.resolveOIDCUserInfo(ctx, cfg, tokenResp)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userInfo.Email) == "" {
		return nil, errors.New("OIDC provider did not return email")
	}

	user, err := s.userRepo.GetUserByEmail(ctx, userInfo.Email)
	if err != nil && !isUserLookupNotFound(err) {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}
	isNewUser := false
	if isUserLookupNotFound(err) || user == nil {
		user, err = s.provisionOIDCUser(ctx, userInfo)
		if err != nil {
			return nil, err
		}
		isNewUser = true
	}

	if !user.IsActive {
		return &types.OIDCCallbackResponse{Success: false, Message: "Account is disabled"}, nil
	}

	// Resolve target tenant once so the JWT claim and the tenant we
	// return below stay in sync; see Login for the rationale.
	resolvedTenantID := s.ResolveActiveTenantID(ctx, user)
	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, resolvedTenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate local tokens: %w", err)
	}

	// 拉取 tenant + memberships，让 OIDC 登录的返回结构与本地登录一致，
	// 前端无须为 OIDC 单独走一次 /auth/me 才能拿到角色。
	var tenant *types.Tenant
	if resolvedTenantID > 0 {
		if t, terr := s.tenantService.GetTenantByID(ctx, resolvedTenantID); terr == nil {
			tenant = t
		} else {
			logger.Warnf(ctx, "OIDC login: failed to load tenant %d for user %s: %v",
				resolvedTenantID, user.ID, terr)
		}
	}
	memberships := s.buildMembershipsForUser(ctx, user, tenant)

	return &types.OIDCCallbackResponse{
		Success:      true,
		Message:      "登录成功",
		User:         user,
		ActiveTenant: tenant,
		Memberships:  memberships,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsNewUser:    isNewUser,
	}, nil
}

// GetUserByID gets a user by ID
func (s *userService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

// GetUsersByIDs proxies to the repository batch fetch. Returns an empty
// map for an empty input; missing ids are absent from the result.
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	return s.userRepo.GetUsersByIDs(ctx, ids)
}

// GetUserByEmail gets a user by email
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// GetUserByUsername gets a user by username
func (s *userService) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	return s.userRepo.GetUserByUsername(ctx, username)
}

// UpdateUser updates user information
func (s *userService) UpdateUser(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// ListSystemAdmins lists users with IsSystemAdmin=true. Thin pass-through
// to the repository; the handler enforces SystemAdmin gating, so the
// service does not duplicate the role check here.
func (s *userService) ListSystemAdmins(
	ctx context.Context, offset, limit int,
) ([]*types.User, int64, error) {
	return s.userRepo.ListSystemAdmins(ctx, offset, limit)
}

// RevokeSystemAdmin removes system-admin privileges through the
// repository's transactional guard so concurrent revokes cannot remove
// the final administrator.
func (s *userService) RevokeSystemAdmin(ctx context.Context, userID, actorID string) (*types.User, error) {
	return s.userRepo.RevokeSystemAdmin(ctx, userID, actorID)
}

// UpdateUserPreferences applies a partial update over the user's
// preferences blob. PATCH semantics: only keys present in `patch`
// (non-nil pointer fields) replace the existing value; everything else
// is preserved. This lets the front-end PUT only the preference that
// changed without having to read-modify-write the whole struct, and
// also makes the endpoint forward-compatible — older clients that
// don't know about newer keys won't accidentally erase them.
func (s *userService) UpdateUserPreferences(
	ctx context.Context,
	userID string,
	patch types.UserPreferences,
) (types.UserPreferences, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return types.UserPreferences{}, err
	}

	merged := user.Preferences
	if patch.LastActiveTenantID != nil {
		// *0 = "forget my preference, resolve from memberships on the next
		// login"; any positive value = set/replace. We do not validate
		// membership here — an invalid value is culled on the next login by
		// ResolveActiveTenantID, keeping this endpoint cheap.
		if *patch.LastActiveTenantID == 0 {
			merged.LastActiveTenantID = nil
		} else {
			v := *patch.LastActiveTenantID
			merged.LastActiveTenantID = &v
		}
	}

	user.Preferences = merged
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return types.UserPreferences{}, err
	}
	return merged, nil
}

// DeleteUser deletes a user
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// ChangePassword changes user password after verifying the current
// credential. The new password must satisfy ValidatePasswordPolicy so
// self-service rotation cannot introduce weaker passwords than
// registration / admin reset allow. On success every outstanding session
// is revoked so a stolen token cannot survive the rotation.
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password before policy checks so callers with a wrong
	// current credential get a clear failure instead of a policy error.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidOldPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)); err == nil {
		return ErrSamePassword
	}

	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()
	if user.Preferences.OidcOnlyLogin != nil && *user.Preferences.OidcOnlyLogin {
		cleared := false
		user.Preferences.OidcOnlyLogin = &cleared
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// Invalidate every outstanding session so a stolen token cannot
	// survive a password rotation.
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// AdminResetPassword replaces a user's password without checking the previous
// credential. Authorization and the cannot-reset-self rule live at the system
// admin HTTP boundary; this service owns the security-critical persistence and
// session invalidation so no caller can accidentally update only one of them.
func (s *userService) AdminResetPassword(ctx context.Context, userID string, newPassword string) error {
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// AdminCreateUser provisions a new local user on behalf of a SystemAdmin.
//
// An absent password generates a random one, returned exactly once.
// Any provided password, must satisfy ValidatePasswordPolicy.
//
// Delegates to Register, so duplicate checks match public registration. The
// account belongs to no workspace until an administrator adds it to one.
func (s *userService) AdminCreateUser(
	ctx context.Context,
	req *types.AdminCreateUserRequest,
) (*types.User, string, error) {
	if req == nil || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Email) == "" {
		return nil, "", errors.New("username and email are required")
	}

	password := ""
	generated := false
	if req.Password == nil {
		randomPassword, err := generatePolicyCompliantPassword()
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate password: %w", err)
		}
		password = randomPassword
		generated = true
	} else {
		password = *req.Password
	}
	// Generation triggers only on an absent password. Any provided
	// value, empty or whitespace-only, is hashed byte-for-byte and must
	// satisfy the password policy.
	if err := ValidatePasswordPolicy(password); err != nil {
		return nil, "", err
	}

	user, err := s.Register(ctx, &types.RegisterRequest{
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		Password: password,
	})
	// WARN: idempotency is sequential only. Two concurrent creates of the
	// same identity can race past Register's check; the loser gets a 500,
	// and a retry resolves idempotently.
	if err != nil {
		// Register owns duplicate detection; on a duplicate we surface
		// the existing row so the caller can respond idempotently. The
		// sentinel names the colliding identity, so the lookup is
		// targeted at exactly that key.
		switch {
		case errors.Is(err, ErrUserEmailExists):
			return s.adminCreateUserOnDuplicate(
				ctx, req, err,
				func(ctx context.Context) (*types.User, error) {
					return s.userRepo.GetUserByEmail(ctx, strings.TrimSpace(req.Email))
				},
			)
		case errors.Is(err, ErrUserUsernameExists):
			return s.adminCreateUserOnDuplicate(
				ctx, req, err,
				func(ctx context.Context) (*types.User, error) {
					return s.userRepo.GetUserByUsername(ctx, strings.TrimSpace(req.Username))
				},
			)
		}
		return nil, "", err
	}
	if generated {
		return user, password, nil
	}
	return user, "", nil
}

func (s *userService) adminCreateUserOnDuplicate(
	ctx context.Context,
	req *types.AdminCreateUserRequest,
	dupErr error,
	lookup func(context.Context) (*types.User, error),
) (*types.User, string, error) {
	existing, lookupErr := lookup(ctx)
	if lookupErr != nil || existing == nil {
		return nil, "", dupErr
	}
	if adminCreateIdentityMatches(existing, req.Username, req.Email) {
		return existing, "", dupErr
	}
	return nil, "", ErrUserIdentityConflict
}

// adminCreateIdentityMatches reports whether an existing row is the exact
// identity an admin create is idempotently retrying (both email and username).
func adminCreateIdentityMatches(existing *types.User, username, email string) bool {
	if existing == nil {
		return false
	}
	return existing.Username == strings.TrimSpace(username) &&
		existing.Email == strings.TrimSpace(email)
}

// ValidatePassword validates user password
func (s *userService) ValidatePassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// GenerateTokens generates access and refresh tokens for user. The access
// token's tenant_id claim is the workspace ResolveActiveTenantID picks, so
// login (and the refresh-token rotation path that also calls into here)
// lands the user back where they left off across devices. SwitchTenant
// remains the explicit tool for switching to an arbitrary membership.
func (s *userService) GenerateTokens(
	ctx context.Context,
	user *types.User,
) (accessToken, refreshToken string, err error) {
	return s.generateTokensForTenant(ctx, user, s.ResolveActiveTenantID(ctx, user))
}

// ResolveActiveTenantID picks the workspace whose ID a session is scoped to
// when nothing names one explicitly. It is the single implementation behind
// a fresh login token, a refreshed token and the auth middleware's fallback
// for a JWT without a tenant claim, so the three can never disagree. The
// order:
//
//  1. The LastActiveTenantID preference, if it points at a workspace that
//     still exists and the user may still act in (an active membership, or
//     CanAccessAllTenants, mirroring SwitchTenant's gate).
//  2. Otherwise the earliest active membership whose workspace still exists;
//     ListByUser is ordered by join time, so the choice is deterministic.
//     That workspace is written back as the preference, best effort, so the
//     next login does not repeat the walk; a stale preference is thereby
//     overwritten rather than cleared and left empty.
//  3. Otherwise 0: a user who belongs to no workspace. The middleware then
//     admits identity-level routes only and answers TENANT_REQUIRED elsewhere.
//
// Lookup errors never fail a login; they are logged and the resolution
// narrows (an unverifiable preference is skipped, an unlistable membership
// table yields 0).
func (s *userService) ResolveActiveTenantID(ctx context.Context, user *types.User) uint64 {
	if user == nil {
		return 0
	}
	if pref := user.Preferences.LastActiveTenantID; pref != nil && *pref != 0 {
		if s.workspaceExists(ctx, *pref) && s.mayActInTenant(ctx, user, *pref) {
			return *pref
		}
		logger.Warnf(ctx,
			"ResolveActiveTenantID: preferred workspace %d is no longer usable by user %s; "+
				"falling back to the earliest membership",
			*pref, user.ID)
	}
	tenantID := s.earliestMembershipTenant(ctx, user)
	if tenantID != 0 {
		s.persistActiveTenantPreference(ctx, user, tenantID)
	}
	return tenantID
}

// mayActInTenant reports whether user may be scoped to tenantID: an active
// membership row, or the cross-workspace attribute. It is the one rule
// SwitchTenant and the preference check share, so a workspace the user can
// switch into is also one their login may land in.
func (s *userService) mayActInTenant(ctx context.Context, user *types.User, tenantID uint64) bool {
	if user.CanAccessAllTenants {
		return true
	}
	if s.memberService == nil {
		return false
	}
	member, err := s.memberService.GetMembership(ctx, user.ID, tenantID)
	if err != nil {
		logger.Warnf(ctx, "membership lookup failed for user %s tenant %d: %v", user.ID, tenantID, err)
		return false
	}
	return member != nil && member.Status == types.TenantMemberStatusActive
}

// workspaceExists reports whether the workspace row is still there. A nil
// tenant service (partial dependency graphs in tests) is treated as "yes" so
// the membership check alone decides.
func (s *userService) workspaceExists(ctx context.Context, tenantID uint64) bool {
	if s.tenantService == nil {
		return true
	}
	if _, err := s.tenantService.GetTenantByID(ctx, tenantID); err != nil {
		logger.Warnf(ctx, "workspace %d not loadable: %v", tenantID, err)
		return false
	}
	return true
}

// earliestMembershipTenant returns the first active membership (by join
// time) whose workspace still exists, or 0. Memberships whose workspace row
// is gone are skipped rather than reported: a token scoped to a deleted
// workspace would fail every request.
func (s *userService) earliestMembershipTenant(ctx context.Context, user *types.User) uint64 {
	if s.memberService == nil {
		return 0
	}
	members, err := s.memberService.ListByUser(ctx, user.ID)
	if err != nil {
		logger.Warnf(ctx, "ResolveActiveTenantID: failed to list memberships for user %s: %v", user.ID, err)
		return 0
	}
	for _, member := range members {
		if member == nil || member.TenantID == 0 || member.Status != types.TenantMemberStatusActive {
			continue
		}
		if s.workspaceExists(ctx, member.TenantID) {
			return member.TenantID
		}
	}
	return 0
}

// persistActiveTenantPreference writes tenantID into the user's
// LastActiveTenantID, in memory for this request and in the database, best
// effort: a failed write is logged, the token being minted is still scoped to
// tenantID, and the next login redoes the resolution.
func (s *userService) persistActiveTenantPreference(ctx context.Context, user *types.User, tenantID uint64) {
	if pref := user.Preferences.LastActiveTenantID; pref != nil && *pref == tenantID {
		return
	}
	user.Preferences.LastActiveTenantID = &tenantID
	user.UpdatedAt = time.Now()
	if s.userRepo == nil {
		return
	}
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		logger.Warnf(ctx,
			"failed to persist workspace %d as the active workspace of user %s: %v",
			tenantID, user.ID, err)
	}
}

// RememberFirstWorkspace sets the preference to tenantID for a user who has
// none, typically right after an invitation made tenantID their first
// workspace. It reads the user row rather than trusting a caller's copy
// because the invitation handlers hold stale snapshots from the start of the
// request.
func (s *userService) RememberFirstWorkspace(ctx context.Context, userID string, tenantID uint64) error {
	if tenantID == 0 {
		return errors.New("workspace ID is required")
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if pref := user.Preferences.LastActiveTenantID; pref != nil && *pref != 0 {
		return nil
	}
	user.Preferences.LastActiveTenantID = &tenantID
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// generateTokensForTenant is the shared implementation behind
// GenerateTokens and SwitchTenant. It encodes activeTenantID into the
// access token's tenant_id claim so the auth middleware scopes future
// requests there.
func (s *userService) generateTokensForTenant(
	ctx context.Context,
	user *types.User,
	activeTenantID uint64,
) (accessToken, refreshToken string, err error) {
	// Every token carries a unique id (jti). Without it two logins by the same
	// user in the same second, scoped to the same workspace, produce the same
	// string; the token table is keyed by that string, so revoking the first
	// session silently revoked the second one too.
	// Generate access token (expires in 24 hours)
	accessClaims := jwt.MapClaims{
		"jti":       uuid.New().String(),
		"user_id":   user.ID,
		"email":     user.Email,
		"tenant_id": activeTenantID,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"type":      "access",
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", err
	}

	// Generate refresh token (expires in 7 days)
	refreshClaims := jwt.MapClaims{
		"jti":     uuid.New().String(),
		"user_id": user.ID,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
		"type":    "refresh",
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", err
	}

	// Store tokens in database
	accessTokenRecord := &types.AuthToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     accessToken,
		TokenType: "access_token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	refreshTokenRecord := &types.AuthToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     refreshToken,
		TokenType: "refresh_token",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = s.tokenRepo.CreateToken(ctx, accessTokenRecord)
	_ = s.tokenRepo.CreateToken(ctx, refreshTokenRecord)

	return accessToken, refreshToken, nil
}

// SwitchTenant verifies that user may act in targetTenantID and issues a
// new token pair scoped to that tenant. The previous refresh token (if
// provided) is revoked so the old session can no longer roll forward into
// the source tenant.
//
// Returns ErrMembershipNotFound when the user has no active membership in
// the target tenant. Cross-tenant superuser access (CanAccessAllTenants)
// is allowed without a membership row, mirroring the auth middleware's
// resolveTenantRole behaviour.
func (s *userService) SwitchTenant(
	ctx context.Context,
	user *types.User,
	targetTenantID uint64,
	currentRefreshToken string,
) (*types.LoginResponse, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	if targetTenantID == 0 {
		return nil, errors.New("target workspace ID is required")
	}

	// Verify membership unless the caller is a cross-tenant superuser.
	if !user.CanAccessAllTenants {
		if s.memberService == nil {
			return nil, errors.New("workspace membership service unavailable")
		}
		member, err := s.memberService.GetMembership(ctx, user.ID, targetTenantID)
		if err != nil {
			return nil, fmt.Errorf("lookup membership: %w", err)
		}
		if member == nil || member.Status != types.TenantMemberStatusActive {
			return nil, ErrMembershipNotFound
		}
	}

	tenant, err := s.tenantService.GetTenantByID(ctx, targetTenantID)
	if err != nil {
		return nil, fmt.Errorf("load target workspace: %w", err)
	}

	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, targetTenantID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	// Best-effort revoke of the previous refresh token. Failure is
	// logged but not fatal — the new tokens are already issued and the
	// old refresh token will expire naturally.
	if strings.TrimSpace(currentRefreshToken) != "" {
		if err := s.RevokeToken(ctx, currentRefreshToken); err != nil {
			logger.Warnf(ctx, "Failed to revoke previous refresh token during tenant switch: %v", err)
		}
	}

	memberships := s.buildMembershipsForUser(ctx, user, tenant)

	return &types.LoginResponse{
		Success:      true,
		Message:      "Workspace switched",
		User:         user,
		ActiveTenant: tenant,
		Memberships:  memberships,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateToken validates an access token. The second return value is
// the JWT's `tenant_id` claim — the tenant the token was minted for — or 0
// for a token minted without one (a tenantless session, or a login that
// found no usable workspace at the time). The caller decides what 0 means;
// the auth middleware falls back to ResolveActiveTenantID.
func (s *userService) ValidateToken(ctx context.Context, tokenString string) (*types.User, uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	})

	if err != nil || !token.Valid {
		return nil, 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, 0, errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, 0, errors.New("invalid user ID in token")
	}

	if isRefreshTokenClaims(claims) {
		return nil, 0, errors.New("refresh token cannot be used as access token")
	}

	// Check if token is revoked
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return nil, 0, errors.New("token is revoked")
	}
	if tokenRecord.TokenType == "refresh_token" {
		return nil, 0, errors.New("refresh token cannot be used as access token")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	return user, tenantIDFromClaims(claims), nil
}

func isRefreshTokenClaims(claims jwt.MapClaims) bool {
	tokenType, ok := claims["type"].(string)
	return ok && tokenType == "refresh"
}

func userIDFromSignedToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil || token == nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", errors.New("invalid user ID in token")
	}
	return userID, nil
}

// tenantIDFromClaims pulls the active tenant ID out of a parsed JWT
// claim map. Returns 0 when the claim is missing, non-positive or has an
// unrecognised type. Extracted as a free function so it can be unit
// tested without standing up the full userService dependency graph.
//
// JSON numbers come back as float64 from jwt.MapClaims; the int64 /
// uint64 branches cover claims built in memory (tests) rather than parsed.
func tenantIDFromClaims(claims jwt.MapClaims) uint64 {
	switch v := claims["tenant_id"].(type) {
	case float64:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case uint64:
		return v
	}
	return 0
}

// RefreshToken refreshes access token using refresh token
func (s *userService) RefreshToken(
	ctx context.Context,
	refreshTokenString string,
) (accessToken, newRefreshToken string, err error) {
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	})

	if err != nil || !token.Valid {
		return "", "", errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid token claims")
	}

	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", "", errors.New("not a refresh token")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", "", errors.New("invalid user ID in token")
	}

	// Check if token is revoked
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, refreshTokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return "", "", errors.New("refresh token is revoked")
	}
	if tokenRecord.TokenType != "refresh_token" {
		return "", "", errors.New("not a refresh token")
	}

	// Get user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// Revoke old refresh token
	tokenRecord.IsRevoked = true
	_ = s.tokenRepo.UpdateToken(ctx, tokenRecord)

	// Generate new tokens
	return s.GenerateTokens(ctx, user)
}

// Logout invalidates every outstanding session for the user identified by
// the presented JWT. Access and refresh tokens are both accepted so clients
// can end the session without refreshing first; expired tokens are allowed
// so logout still works after the access token TTL.
func (s *userService) Logout(ctx context.Context, tokenString string) error {
	userID, err := userIDFromSignedToken(tokenString)
	if err != nil {
		return err
	}
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// RevokeToken revokes a token
func (s *userService) RevokeToken(ctx context.Context, tokenString string) error {
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil {
		return err
	}

	tokenRecord.IsRevoked = true
	tokenRecord.UpdatedAt = time.Now()

	return s.tokenRepo.UpdateToken(ctx, tokenRecord)
}

// GetCurrentUser gets current user from context
func (s *userService) GetCurrentUser(ctx context.Context) (*types.User, error) {
	user, ok := ctx.Value(types.UserContextKey).(*types.User)
	if !ok {
		return nil, errors.New("user not found in context")
	}

	return user, nil
}

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

func newOIDCHTTPClient() *http.Client {
	cfg := secutils.DefaultSSRFSafeHTTPClientConfig()
	cfg.Timeout = 30 * time.Second
	return secutils.NewSSRFSafeHTTPClient(cfg)
}

func validateOIDCEndpoint(label, endpoint string, required bool) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		if required {
			return fmt.Errorf("OIDC %s endpoint is required", label)
		}
		return nil
	}
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		return fmt.Errorf("OIDC %s endpoint failed SSRF validation: %w", label, err)
	}
	return nil
}

func validateOIDCEndpoints(cfg *config.OIDCAuthConfig) error {
	if err := validateOIDCEndpoint("authorization", cfg.AuthorizationEndpoint, true); err != nil {
		return err
	}
	if err := validateOIDCEndpoint("token", cfg.TokenEndpoint, true); err != nil {
		return err
	}
	if err := validateOIDCEndpoint("userinfo", cfg.UserInfoEndpoint, false); err != nil {
		return err
	}
	return nil
}

func (s *userService) getOIDCConfig(ctx context.Context) (*config.OIDCAuthConfig, error) {
	if s.config == nil || s.config.OIDCAuth == nil || !s.config.OIDCAuth.Enable {
		return nil, errors.New("OIDC login is disabled")
	}
	cfg := *s.config.OIDCAuth
	if cfg.UserInfoMapping == nil {
		cfg.UserInfoMapping = &config.OIDCUserInfoMapping{Username: "name", Email: "email"}
	}
	if err := s.populateOIDCEndpoints(ctx, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *userService) populateOIDCEndpoints(ctx context.Context, cfg *config.OIDCAuthConfig) error {
	if strings.TrimSpace(cfg.AuthorizationEndpoint) != "" && strings.TrimSpace(cfg.TokenEndpoint) != "" {
		return validateOIDCEndpoints(cfg)
	}
	if strings.TrimSpace(cfg.DiscoveryURL) == "" {
		return errors.New("OIDC discovery_url or explicit endpoints are required")
	}
	if err := validateOIDCEndpoint("discovery", cfg.DiscoveryURL, true); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.DiscoveryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create OIDC discovery request: %w", err)
	}

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to load OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("OIDC discovery request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var doc oidcDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("failed to decode OIDC discovery document: %w", err)
	}
	if cfg.AuthorizationEndpoint == "" {
		cfg.AuthorizationEndpoint = doc.AuthorizationEndpoint
	}
	if cfg.TokenEndpoint == "" {
		cfg.TokenEndpoint = doc.TokenEndpoint
	}
	if cfg.UserInfoEndpoint == "" {
		cfg.UserInfoEndpoint = doc.UserInfoEndpoint
	}
	if cfg.AuthorizationEndpoint == "" || cfg.TokenEndpoint == "" {
		return errors.New("OIDC discovery document missing required endpoints")
	}
	return validateOIDCEndpoints(cfg)
}

func (s *userService) exchangeOIDCCode(ctx context.Context, cfg *config.OIDCAuthConfig, code, redirectURI string) (*oidcTokenResponse, error) {
	if err := validateOIDCEndpoint("token", cfg.TokenEndpoint, true); err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange OIDC code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("OIDC token exchange failed: status=%d", resp.StatusCode)
	}

	var tokenResp oidcTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode OIDC token response: %w", err)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" && strings.TrimSpace(tokenResp.IDToken) == "" {
		return nil, errors.New("OIDC token response missing access_token and id_token")
	}
	return &tokenResp, nil
}

func (s *userService) resolveOIDCUserInfo(ctx context.Context, cfg *config.OIDCAuthConfig, tokenResp *oidcTokenResponse) (*types.OIDCUserInfo, error) {
	claims := map[string]interface{}{}

	if strings.TrimSpace(tokenResp.IDToken) != "" {
		idTokenClaims, err := decodeJWTClaims(tokenResp.IDToken)
		if err != nil {
			logger.Warnf(ctx, "Failed to decode OIDC id_token claims: %v", err)
		} else {
			for k, v := range idTokenClaims {
				claims[k] = v
			}
		}
	}

	if strings.TrimSpace(cfg.UserInfoEndpoint) != "" && strings.TrimSpace(tokenResp.AccessToken) != "" {
		userInfoClaims, err := s.fetchOIDCUserInfo(ctx, cfg.UserInfoEndpoint, tokenResp.AccessToken)
		if err != nil {
			logger.Warnf(ctx, "Failed to fetch OIDC userinfo, fallback to id_token claims: %v", err)
		} else {
			for k, v := range userInfoClaims {
				claims[k] = v
			}
		}
	}

	info := &types.OIDCUserInfo{Claims: claims}
	if sub, _ := claims["sub"].(string); sub != "" {
		info.Subject = sub
	}
	info.Username = extractClaimAsString(claims, cfg.UserInfoMapping.Username)
	info.Email = extractClaimAsString(claims, cfg.UserInfoMapping.Email)
	if info.Username == "" {
		info.Username = extractClaimAsString(claims, "preferred_username")
	}
	if info.Username == "" {
		info.Username = extractClaimAsString(claims, "name")
	}
	if info.Username == "" && info.Email != "" {
		info.Username = strings.Split(info.Email, "@")[0]
	}
	return info, nil
}

func (s *userService) fetchOIDCUserInfo(ctx context.Context, endpoint, accessToken string) (map[string]interface{}, error) {
	if err := validateOIDCEndpoint("userinfo", endpoint, true); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := newOIDCHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("userinfo request failed: status=%d", resp.StatusCode)
	}

	var claims map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// provisionOIDCUser auto-creates a local account for a first-time OIDC
// login. Like password registration it creates the account only; the user
// is tenantless until invited or added by an administrator.
func (s *userService) provisionOIDCUser(ctx context.Context, info *types.OIDCUserInfo) (*types.User, error) {
	username := s.generateOIDCUsername(ctx, info)
	randomPassword, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password for OIDC user: %w", err)
	}

	user, err := s.Register(ctx, &types.RegisterRequest{
		Username: username,
		Email:    info.Email,
		Password: randomPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-provision OIDC user: %w", err)
	}

	oidcOnly := true
	user.Preferences.OidcOnlyLogin = &oidcOnly
	user.UpdatedAt = time.Now()
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to mark OIDC-only login preference: %w", err)
	}
	return user, nil
}

func (s *userService) generateOIDCUsername(ctx context.Context, info *types.OIDCUserInfo) string {
	base := sanitizeUsernameCandidate(info.Username)
	if base == "" {
		base = sanitizeUsernameCandidate(strings.Split(info.Email, "@")[0])
	}
	if base == "" {
		base = "oidc-user"
	}

	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := s.userRepo.GetUserByUsername(ctx, candidate)
		if isUserLookupNotFound(err) || (err == nil && existing == nil) {
			return candidate
		}
		if err != nil && !isUserLookupNotFound(err) {
			logger.Warnf(ctx, "Failed to check existing OIDC username %q: %v", candidate, err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i+1)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func generateRandomString(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// generatePolicyCompliantPassword returns a cryptographically random
// password that satisfies ValidatePasswordPolicy, regenerating until
// it does (a single 32-char base64url draw misses digits ~0.4% of the
// time).
func generatePolicyCompliantPassword() (string, error) {
	for {
		password, err := generateRandomString(24)
		if err != nil {
			return "", err
		}
		if ValidatePasswordPolicy(password) == nil {
			return password, nil
		}
	}
}

func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func extractClaimAsString(claims map[string]interface{}, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func sanitizeUsernameCandidate(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-._")
	if len(result) > 50 {
		result = strings.Trim(result[:50], "-._")
	}
	return result
}

func isUserLookupNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, apprepo.ErrUserNotFound) || strings.Contains(strings.ToLower(err.Error()), "user not found")
}
