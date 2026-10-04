package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/handler/dto"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// TenantHandler implements HTTP request handlers for tenant management
// Provides functionality for creating, retrieving, updating, and deleting tenants
// through the REST API endpoints
type TenantHandler struct {
	service       interfaces.TenantService
	apiKeyService interfaces.TenantAPIKeyService
	userService   interfaces.UserService
	memberService interfaces.TenantMemberService
	kbService     interfaces.KnowledgeBaseService
	config        *config.Config
	// systemSettingSvc resolves runtime tenant policies and limits.
	// Reading goes DB > ENV >
	// in-code default, so a SystemAdmin's UI override applies on the
	// very next CreateTenant call.
	systemSettingSvc interfaces.SystemSettingService
	// auditSvc records workspace creation and deletion in the platform
	// audit feed. Optional: nil in partially wired tests, in which case
	// the lifecycle events are simply not recorded.
	auditSvc interfaces.AuditLogService
}

// NewTenantHandler creates a new tenant handler instance with the provided service
// Parameters:
//   - service: An implementation of the TenantService interface for business logic
//   - userService: An implementation of the UserService interface for user operations
//   - memberService: An implementation of TenantMemberService used to make the
//     Owner named in a create request a member of the new workspace.
//   - config: Application configuration
//
// # Returns a pointer to the newly created TenantHandler
//
// Note on RBAC: cross-tenant gating (CanAccessAllTenants /
// EnableCrossTenantAccess) and per-tenant path matching (URL :id ==
// active tenant) used to live in `authorizeTenantAccess` and the if
// blocks at the top of ListAllTenants / SearchTenants. Both moved to
// `middleware/access.go` (RequireCrossTenantAccess /
// RequirePathTenantMatch) and are wired in `router.go` so the handler
// stays focused on business logic.
func NewTenantHandler(
	service interfaces.TenantService,
	apiKeyService interfaces.TenantAPIKeyService,
	userService interfaces.UserService,
	memberService interfaces.TenantMemberService,
	kbService interfaces.KnowledgeBaseService,
	config *config.Config,
	systemSettingSvc interfaces.SystemSettingService,
	auditSvc interfaces.AuditLogService,
) *TenantHandler {
	return &TenantHandler{
		service:          service,
		apiKeyService:    apiKeyService,
		userService:      userService,
		memberService:    memberService,
		kbService:        kbService,
		config:           config,
		systemSettingSvc: systemSettingSvc,
		auditSvc:         auditSvc,
	}
}

// createTenantRequest is the JSON body for POST /tenants. The route is
// open to system administrators, cross-tenant superusers and platform API
// keys only, so the full Tenant payload (status, storage_quota, retriever
// engines, configs...) is accepted: provisioning workspaces is their job.
// Anything the caller leaves out is filled in server-side by
// TenantService.CreateTenant and the default-quota setting below.
//
// OwnerEmail names the workspace's first Owner. A workspace with no
// members is unreachable — the auth middleware never promotes anyone into
// a memberless workspace — so every create carries an Owner: the named
// user when owner_email is set, otherwise the caller when the caller is a
// human. A platform API key has no human behind it and must name one.
type createTenantRequest struct {
	types.Tenant
	OwnerEmail string `json:"owner_email" binding:"omitempty,email"`
}

// updateTenantRequest is the JSON body for PUT /tenants/:id. Only the
// fields an Owner is permitted to mutate via the public API are bound;
// everything else (storage_quota, status, business, api_key,
// retrieval / storage configs, ...) is intentionally NOT writable here
// — those go through dedicated endpoints (PUT /tenants/kv/:key, ...)
// that have their own validation.
//
// Pointers so we can distinguish "not sent" from "explicit empty
// string"; when nil we leave the existing column untouched.
type updateTenantRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=1,max=128"`
	Description *string `json:"description" binding:"omitempty,max=512"`
}

type apiPrincipalConfigRequest struct {
	Mode                  types.APIPrincipalMode `json:"mode"`
	DirectHeaderName      string                 `json:"direct_header_name"`
	SignedTokenHeaderName string                 `json:"signed_token_header_name"`
	RequireDirectHeader   bool                   `json:"require_direct_header"`
	HMACSecret            *string                `json:"hmac_secret"`
}

type apiPrincipalConfigResponse struct {
	Mode                  types.APIPrincipalMode `json:"mode"`
	DirectHeaderName      string                 `json:"direct_header_name"`
	SignedTokenHeaderName string                 `json:"signed_token_header_name"`
	RequireDirectHeader   bool                   `json:"require_direct_header"`
	// HasHMACSecret reports whether a signing secret is configured. The
	// plaintext secret is NEVER returned — clients only learn presence.
	HasHMACSecret bool `json:"has_hmac_secret"`
}

type apiPrincipalTestTokenRequest struct {
	ExternalUserID   string `json:"external_user_id"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type apiPrincipalTestTokenResponse struct {
	Token            string `json:"token"`
	HeaderName       string `json:"header_name"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	ExpiresAtUnix    int64  `json:"expires_at_unix"`
	ExternalUserID   string `json:"external_user_id"`
}

type tenantAPIKeyCreateRequest struct {
	Name             string   `json:"name"`
	FullAccess       bool     `json:"full_access"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Capabilities     []string `json:"capabilities"`
	ExpiresAt        *int64   `json:"expires_at_unix"`
}

// tenantAPIKeyUpdateRequest 修改已创建 API Key 的配置，字段语义与创建接口一致。
type tenantAPIKeyUpdateRequest struct {
	Name             string   `json:"name"`
	FullAccess       bool     `json:"full_access"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Capabilities     []string `json:"capabilities"`
	ExpiresAt        *int64   `json:"expires_at_unix"`
}

type tenantAPIKeyResponse struct {
	ID               uint64                `json:"id"`
	ScopeType        types.APIKeyScopeType `json:"scope_type"`
	Name             string                `json:"name"`
	APIKey           string                `json:"api_key"`
	FullAccess       bool                  `json:"full_access"`
	KnowledgeBaseIDs types.StringArray     `json:"knowledge_base_ids"`
	Capabilities     types.StringArray     `json:"capabilities"`
	LastUsedAt       *time.Time            `json:"last_used_at,omitempty"`
	ExpiresAt        *time.Time            `json:"expires_at,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
}

type tenantAPIKeyCreateResponse struct {
	tenantAPIKeyResponse
	Token string `json:"token"`
}

const (
	defaultAPIPrincipalDirectHeader  = "X-External-User-ID"
	defaultAPIPrincipalTokenHeader   = "X-External-User-Token"
	defaultAPIPrincipalTestTokenTTL  = 15 * time.Minute
	maxAPIPrincipalTestTokenTTL      = time.Hour
	maxAPIPrincipalExternalUserIDLen = 128
	// apiPrincipalSecretRedacted is the placeholder an update request may
	// send in place of the HMAC secret to signal "leave the stored secret
	// unchanged". The plaintext secret is never returned by GET, so the
	// client cannot echo the real value back.
	apiPrincipalSecretRedacted = "***"
)

// gib is the unit the default-quota setting is expressed in.
const gib = int64(1024 * 1024 * 1024)

// defaultStorageQuotaBytes resolves tenant.default_storage_quota_gb through
// the 3-tier resolver (DB > ENV > built-in) and converts it to bytes. Zero
// (the built-in default) and negative values mean "unlimited": the quota
// checks treat StorageQuota <= 0 as no limit, and a company-wide workspace
// has no reason to stop at an arbitrary number.
func (h *TenantHandler) defaultStorageQuotaBytes(ctx context.Context) int64 {
	gb := h.systemSettingSvc.GetInt(
		ctx,
		"tenant.default_storage_quota_gb",
		"YUHENG_TENANT_DEFAULT_STORAGE_QUOTA_GB",
		0,
	)
	if gb <= 0 {
		return 0
	}
	return gb * gib
}

// resolveWorkspaceOwner decides who owns a workspace being created. The
// named user wins when owner_email is set; otherwise the caller, unless
// the caller is a platform API key, which cannot own anything. The
// returned *errors.AppError is ready to hand to c.Error.
func (h *TenantHandler) resolveWorkspaceOwner(
	ctx context.Context,
	caller *types.User,
	platformCaller bool,
	ownerEmail string,
) (*types.User, *errors.AppError) {
	ownerEmail = strings.TrimSpace(ownerEmail)
	if ownerEmail == "" {
		if platformCaller {
			return nil, errors.NewTenantOwnerRequiredError()
		}
		return caller, nil
	}
	owner, err := h.userService.GetUserByEmail(ctx, ownerEmail)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			// The same 404 the member endpoints use for an unknown email,
			// so the UI can offer "create the account first" in one place.
			return nil, errors.NewNotFoundError(
				"user with this email is not registered; create the account first")
		}
		logger.Errorf(ctx, "GetUserByEmail failed for workspace owner %s: %v",
			secutils.SanitizeForLog(ownerEmail), err)
		return nil, errors.NewInternalServerError("Failed to look up the workspace owner").WithDetails(err.Error())
	}
	return owner, nil
}

// emitTenantAudit records a workspace lifecycle event in the platform
// audit feed (tenant_id=0, the convention for system-scope rows).
// Best-effort, like every other audit hook.
func (h *TenantHandler) emitTenantAudit(
	ctx context.Context,
	action types.AuditAction,
	tenantID uint64,
	details map[string]any,
) {
	if h.auditSvc == nil {
		return
	}
	actorID, _ := types.UserIDFromContext(ctx)
	detailsJSON, _ := json.Marshal(details)
	_ = h.auditSvc.Log(ctx, &types.AuditLog{
		TenantID:    0,
		ActorUserID: actorID,
		ActorRole:   systemAuditActorRole(ctx),
		Action:      action,
		TargetType:  "tenant",
		TargetID:    strconv.FormatUint(tenantID, 10),
		Outcome:     types.AuditOutcomeSuccess,
		Details:     types.JSON(detailsJSON),
	})
}

// CreateTenant godoc
// @Summary      创建工作区
// @Description  创建新的工作区。仅系统管理员、跨工作区超管与平台 API Key 可调用。
// @Description  每个工作区在创建时就必须有 Owner：owner_email 指定一位已注册用户，
// @Description  省略时调用者本人成为 Owner；平台 API Key 没有"本人"，必须指定 owner_email。
// @Description  不会随工作区发放 API Key，需要时通过 API Key 管理接口显式创建。
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        request  body      handler.createTenantRequest  true  "工作区信息（可含 owner_email）"
// @Success      201      {object}  map[string]interface{}  "创建的工作区"
// @Failure      400      {object}  errors.AppError         "请求参数错误 / 平台 Key 未指定 owner_email（code 2006）"
// @Failure      403      {object}  errors.AppError         "不是系统管理员"
// @Failure      404      {object}  errors.AppError         "owner_email 对应的用户不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants [post]
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating tenant")

	// The route guard (RequireTenantCatalogAccess) has already decided the
	// caller may create workspaces. The caller is still needed here as the
	// default Owner.
	caller, err := h.userService.GetCurrentUser(ctx)
	if err != nil || caller == nil {
		logger.Error(ctx, "Failed to resolve current user from context", err)
		c.Error(errors.NewUnauthorizedError("authentication required"))
		return
	}
	apiKeyScope, hasAPIKeyScope := types.TenantAPIKeyScopeFromContext(ctx)
	platformCaller := hasAPIKeyScope && apiKeyScope.IsPlatform()

	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		_ = c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	tenantData := req.Tenant
	tenantData.Name = strings.TrimSpace(tenantData.Name)
	tenantData.Description = strings.TrimSpace(tenantData.Description)
	if tenantData.Name == "" {
		_ = c.Error(errors.NewValidationError("workspace name is required"))
		return
	}
	// Reset client-supplied primary key so we don't accidentally insert
	// with a chosen ID that collides with a future auto-increment value.
	// Tenant IDs must always be DB-generated.
	tenantData.ID = 0

	// Resolve the Owner before touching the database: an unknown email
	// must not leave an orphan workspace behind.
	owner, ownerErr := h.resolveWorkspaceOwner(ctx, caller, platformCaller, req.OwnerEmail)
	if ownerErr != nil {
		_ = c.Error(ownerErr)
		return
	}

	// Apply the system-setting-driven default storage quota when the
	// request didn't specify one. We resolve at create time on purpose —
	// the on-disk row should carry an explicit value, so changing the
	// setting later doesn't silently shrink/grow established tenants.
	if tenantData.StorageQuota <= 0 {
		tenantData.StorageQuota = h.defaultStorageQuotaBytes(ctx)
	}

	logger.Infof(ctx, "Creating tenant, name: %s", secutils.SanitizeForLog(tenantData.Name))

	createdTenant, err := h.service.CreateTenant(ctx, &tenantData)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to create workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create workspace").WithDetails(err.Error()))
		}
		return
	}

	// Make the Owner a member in the same request. We MUST roll the tenant
	// back if this fails: without a membership row the new tenant is
	// unreachable, yet still occupies storage_bucket / name uniqueness
	// slots. EnsureOwner rather than AddMember because the Owner role may
	// be assigned by a platform API key here — the key is not assigning a
	// role inside a workspace it holds, it is provisioning the workspace.
	if _, err := h.memberService.EnsureOwner(ctx, owner.ID, createdTenant.ID); err != nil {
		logger.Errorf(ctx,
			"Failed to make user %s owner of tenant %d: %v — rolling back tenant",
			owner.ID, createdTenant.ID, err)
		if delErr := h.service.DeleteTenant(ctx, createdTenant.ID); delErr != nil {
			logger.Errorf(ctx,
				"Rollback DeleteTenant failed for orphan tenant %d: %v",
				createdTenant.ID, delErr,
			)
		}
		_ = c.Error(errors.NewInternalServerError("Failed to finalise workspace ownership").WithDetails(err.Error()))
		return
	}
	// An Owner who had no workspace until now should land in this one at
	// their next login. Membership is what authorises them; the preference
	// only saves a lookup, so a failure is logged and not surfaced.
	if err := h.userService.RememberFirstWorkspace(ctx, owner.ID, createdTenant.ID); err != nil {
		logger.Warnf(ctx, "failed to remember workspace %d as the active workspace of user %s: %v",
			createdTenant.ID, owner.ID, err)
	}

	h.emitTenantAudit(ctx, types.AuditActionSystemTenantCreated, createdTenant.ID, map[string]any{
		"name":          createdTenant.Name,
		"owner_user_id": owner.ID,
		"owner_email":   owner.Email,
	})

	logger.Infof(
		ctx,
		"Tenant created successfully, ID: %d, name: %s, owner: %s",
		createdTenant.ID,
		secutils.SanitizeForLog(createdTenant.Name),
		owner.ID,
	)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    createdTenant,
	})
}

// GetTenant godoc
// @Summary      获取工作区详情
// @Description  根据ID获取工作区详情
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "工作区ID"
// @Success      200  {object}  map[string]interface{}  "工作区详情"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "工作区不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id} [get]
func (h *TenantHandler) GetTenant(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to retrieve workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to retrieve workspace").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    dto.NewTenantResponse(ctx, tenant),
	})
}

// UpdateTenant godoc
// @Summary      更新工作区
// @Description  更新工作区信息
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id       path      int           true  "工作区ID"
// @Param        request  body      types.Tenant  true  "工作区信息"
// @Success      200      {object}  map[string]interface{}  "更新后的工作区"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /tenants/{id} [put]
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start updating tenant")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	// Strict whitelist: only Name / Description are mutable through the
	// public PUT. Storage quota, status, business, configs, api_key and
	// every other privileged column live behind dedicated endpoints
	// (PUT /tenants/kv/:key, ...). Without this, an
	// Owner — including any user who just self-served a tenant — could
	// flip status / bump storage_quota by simply crafting an extended
	// JSON body. Pointers distinguish "field omitted" from "explicit
	// empty string" so we can leave untouched columns alone.
	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	// Load the persisted tenant so any column the request omits keeps
	// its current value through the GORM `Updates(struct)` zero-skip
	// behaviour (we always pass back the full struct).
	existing, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			c.Error(errors.NewValidationError("name cannot be blank"))
			return
		}
		existing.Name = trimmed
	}
	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}

	logger.Infof(ctx, "Updating tenant, ID: %d, Name: %s", id, secutils.SanitizeForLog(existing.Name))

	updatedTenant, err := h.service.UpdateTenant(ctx, existing)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to update workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace").WithDetails(err.Error()))
		}
		return
	}

	logger.Infof(
		ctx,
		"Tenant updated successfully, ID: %d, Name: %s",
		updatedTenant.ID,
		secutils.SanitizeForLog(updatedTenant.Name),
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    dto.NewTenantResponse(ctx, updatedTenant),
	})
}

func (h *TenantHandler) ListAPIKeys(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keys, err := h.apiKeyService.ListAPIKeys(ctx, id)
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to list API keys").WithDetails(err.Error()))
		return
	}
	resp := make([]tenantAPIKeyResponse, 0, len(keys))
	for _, key := range keys {
		resp = append(resp, tenantAPIKeyForResponse(key))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *TenantHandler) CreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	var req tenantAPIKeyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if err := validateTenantAPIKeyRequest(ctx, h.kbService, id, req); err != nil {
		c.Error(err)
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		if !t.After(time.Now().UTC()) {
			c.Error(errors.NewValidationError("expires_at_unix must be in the future"))
			return
		}
		expiresAt = &t
	}
	result, err := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID:         id,
		Name:             req.Name,
		FullAccess:       req.FullAccess,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		Capabilities:     req.Capabilities,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		_ = c.Error(apiKeyWriteError(err, "Failed to create API key"))
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": tenantAPIKeyCreateResponse{
			tenantAPIKeyResponse: tenantAPIKeyForResponse(result.APIKey),
			Token:                result.Token,
		},
	})
}

// UpdateAPIKey 修改已创建租户 API Key 的授权范围和其他可配置属性。
// 路由层要求当前租户 Owner；字段校验与创建接口保持一致。
func (h *TenantHandler) UpdateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || tenantID == 0 {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keyID, err := strconv.ParseUint(c.Param("key_id"), 10, 64)
	if err != nil || keyID == 0 {
		c.Error(errors.NewBadRequestError("Invalid API key ID"))
		return
	}
	var req tenantAPIKeyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if appErr := validateTenantAPIKeyRequest(ctx, h.kbService, tenantID, tenantAPIKeyCreateRequest(req)); appErr != nil {
		c.Error(appErr)
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		expiresAt = &t
	}

	updated, err := h.apiKeyService.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
		TenantID: tenantID, APIKeyID: keyID, Name: req.Name, FullAccess: req.FullAccess,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs, Capabilities: req.Capabilities, ExpiresAt: expiresAt,
	})
	if err != nil {
		_ = c.Error(apiKeyWriteError(err, "Failed to update API key"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tenantAPIKeyForResponse(updated)})
}

func (h *TenantHandler) DeleteAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keyID, err := strconv.ParseUint(c.Param("key_id"), 10, 64)
	if err != nil || keyID == 0 {
		c.Error(errors.NewBadRequestError("Invalid API key ID"))
		return
	}
	if err := h.apiKeyService.RevokeAPIKey(ctx, tenantID, keyID); err != nil {
		_ = c.Error(apiKeyWriteError(err, "Failed to revoke API key"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// apiKeyWriteError says what went wrong with a key write: a key that is not
// there (or not this workspace's) is 404, a request the service refuses is
// 400, and only a failure of the service itself is 500. Reporting every
// error as "not found" hid a rejected capability behind a missing key.
func apiKeyWriteError(err error, what string) *errors.AppError {
	switch {
	case stderrors.Is(err, repository.ErrTenantAPIKeyNotFound):
		return errors.NewNotFoundError("API key not found")
	case stderrors.Is(err, types.ErrPlatformOnlyCapability):
		return errors.NewValidationError(err.Error())
	default:
		return errors.NewInternalServerError(what).WithDetails(err.Error())
	}
}

func tenantAPIKeyForResponse(key *types.TenantAPIKey) tenantAPIKeyResponse {
	if key == nil {
		return tenantAPIKeyResponse{}
	}
	return tenantAPIKeyResponse{
		ID:               key.ID,
		ScopeType:        types.NormalizeAPIKeyScopeType(key.ScopeType),
		Name:             key.Name,
		APIKey:           key.KeyHint,
		FullAccess:       key.FullAccess,
		KnowledgeBaseIDs: key.KnowledgeBaseIDs,
		Capabilities:     types.NormalizeAPIKeyCapabilities(key.Capabilities),
		LastUsedAt:       key.LastUsedAt,
		ExpiresAt:        key.ExpiresAt,
		CreatedAt:        key.CreatedAt,
	}
}

func validateTenantAPIKeyRequest(
	ctx context.Context,
	kbService interfaces.KnowledgeBaseService,
	tenantID uint64,
	req tenantAPIKeyCreateRequest,
) *errors.AppError {
	if strings.TrimSpace(req.Name) == "" {
		return errors.NewValidationError("name is required")
	}
	if req.FullAccess {
		return nil
	}
	caps := types.NormalizeAPIKeyCapabilities(types.StringArray(req.Capabilities))
	if len(caps) == 0 {
		return errors.NewValidationError("capabilities are required for scoped API keys")
	}
	for _, cap := range req.Capabilities {
		if strings.TrimSpace(cap) == "" {
			continue
		}
		normalized := types.NormalizeAPIKeyCapability(types.APIKeyCapability(cap))
		if normalized == "" {
			return errors.NewValidationError("capabilities contains an unknown capability")
		}
		if normalized.PlatformOnly() {
			return errors.NewValidationError(types.ErrPlatformOnlyCapability.Error())
		}
	}
	return validateTenantAPIKeyKnowledgeBaseIDs(ctx, kbService, tenantID, req.KnowledgeBaseIDs)
}

// validateTenantAPIKeyKnowledgeBaseIDs 校验白名单中的知识库真实存在且属于目标租户。
// 入参是请求上下文、知识库服务、租户 ID 和待授权 ID；成功无返回值，失败返回可直接响应的应用错误。
func validateTenantAPIKeyKnowledgeBaseIDs(
	ctx context.Context,
	kbService interfaces.KnowledgeBaseService,
	tenantID uint64,
	knowledgeBaseIDs []string,
) *errors.AppError {
	if len(knowledgeBaseIDs) == 0 {
		return nil
	}
	return validateTenantAPIKeyKnowledgeBaseIDsWithLookup(
		ctx, tenantID, knowledgeBaseIDs, kbService.GetKnowledgeBaseByID,
	)
}

// validateTenantAPIKeyKnowledgeBaseIDsWithLookup 将归属校验与大型知识库服务接口解耦，便于覆盖边界测试。
// lookup 输入知识库 ID 并返回真实知识库；函数输出 nil 或可直接响应的校验错误。
func validateTenantAPIKeyKnowledgeBaseIDsWithLookup(
	ctx context.Context,
	tenantID uint64,
	knowledgeBaseIDs []string,
	lookup func(context.Context, string) (*types.KnowledgeBase, error),
) *errors.AppError {
	for _, kbID := range knowledgeBaseIDs {
		kbID = strings.TrimSpace(kbID)
		if kbID == "" {
			continue
		}
		kb, err := lookup(ctx, kbID)
		if err != nil || kb == nil {
			return errors.NewValidationError("knowledge_base_ids contains an unknown knowledge base")
		}
		if kb.TenantID != tenantID {
			return errors.NewForbiddenError("knowledge_base_ids contains a knowledge base outside this workspace")
		}
	}
	return nil
}

func apiPrincipalConfigForResponse(cfg *types.APIPrincipalConfig) apiPrincipalConfigResponse {
	if cfg == nil {
		cfg = &types.APIPrincipalConfig{}
	}
	mode := cfg.Mode
	if mode == "" {
		mode = types.APIPrincipalModeTenant
	}
	return apiPrincipalConfigResponse{
		Mode:                  mode,
		DirectHeaderName:      defaultAPIPrincipalDirectHeader,
		SignedTokenHeaderName: defaultAPIPrincipalTokenHeader,
		RequireDirectHeader:   cfg.RequireDirectHeader,
		HasHMACSecret:         strings.TrimSpace(cfg.HMACSecret) != "",
	}
}

// GetAPIPrincipalConfig godoc
// @Summary      获取工作区 API Key 用户身份配置
// @Description  返回 X-API-Key 请求如何映射为终端 Principal 的配置（Owner）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "工作区ID"
// @Success      200  {object}  map[string]interface{}  "API principal 配置"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      403  {object}  errors.AppError         "权限不足"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-config [get]
func (h *TenantHandler) GetAPIPrincipalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiPrincipalConfigForResponse(tenant.APIPrincipalConfig),
	})
}

// UpdateAPIPrincipalConfig godoc
// @Summary      更新工作区 API Key 用户身份配置
// @Description  配置 X-API-Key 请求如何映射为终端 Principal（Owner）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id       path      int                           true  "工作区ID"
// @Param        request  body      handler.apiPrincipalConfigRequest  true  "API principal 配置"
// @Success      200      {object}  map[string]interface{}        "更新后的配置"
// @Failure      400      {object}  errors.AppError               "请求参数错误"
// @Failure      403      {object}  errors.AppError               "权限不足"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-config [put]
func (h *TenantHandler) UpdateAPIPrincipalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	var req apiPrincipalConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if req.Mode == "" {
		req.Mode = types.APIPrincipalModeTenant
	}
	switch req.Mode {
	case types.APIPrincipalModeTenant, types.APIPrincipalModeDirect, types.APIPrincipalModeSignedToken:
	default:
		c.Error(errors.NewValidationError("mode must be tenant, direct_header, or signed_token"))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	existingSecret := ""
	if tenant.APIPrincipalConfig != nil {
		existingSecret = tenant.APIPrincipalConfig.HMACSecret
	}
	hmacSecret := existingSecret
	if req.HMACSecret != nil {
		provided := strings.TrimSpace(*req.HMACSecret)
		// GET no longer discloses the plaintext secret, so a client that
		// edits the config re-submits the redaction placeholder to mean
		// "keep the existing secret". Treat it as a no-op instead of
		// overwriting the real secret with "***".
		if provided != apiPrincipalSecretRedacted {
			hmacSecret = provided
		}
	}
	cfg := &types.APIPrincipalConfig{
		Mode:                  req.Mode,
		DirectHeaderName:      defaultAPIPrincipalDirectHeader,
		SignedTokenHeaderName: defaultAPIPrincipalTokenHeader,
		RequireDirectHeader:   req.RequireDirectHeader,
		HMACSecret:            hmacSecret,
	}
	if cfg.Mode == types.APIPrincipalModeSignedToken && strings.TrimSpace(cfg.HMACSecret) == "" {
		c.Error(errors.NewValidationError("hmac_secret is required for signed_token mode"))
		return
	}
	tenant.APIPrincipalConfig = cfg

	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to update API principal config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiPrincipalConfigForResponse(updatedTenant.APIPrincipalConfig),
	})
}

// CreateAPIPrincipalTestToken godoc
// @Summary      生成 API Playground 测试 JWT
// @Description  使用工作区已保存的 HMAC 密钥签发短期外部用户 JWT（Owner）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id       path      int                                  true  "工作区ID"
// @Param        request  body      handler.apiPrincipalTestTokenRequest true  "测试 Token 参数"
// @Success      200      {object}  map[string]interface{}               "短期 JWT"
// @Failure      400      {object}  errors.AppError                      "请求参数错误"
// @Failure      403      {object}  errors.AppError                      "权限不足"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-test-token [post]
func (h *TenantHandler) CreateAPIPrincipalTestToken(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	var req apiPrincipalTestTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	externalUserID := strings.TrimSpace(req.ExternalUserID)
	if err := validateAPIPrincipalExternalUserID(externalUserID); err != nil {
		c.Error(errors.NewValidationError("external_user_id is invalid").WithDetails(err.Error()))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	cfg := tenant.APIPrincipalConfig
	if cfg == nil || cfg.Mode != types.APIPrincipalModeSignedToken {
		c.Error(errors.NewValidationError("signed_token mode is required"))
		return
	}
	secret := strings.TrimSpace(cfg.HMACSecret)
	if secret == "" {
		c.Error(errors.NewValidationError("hmac_secret is required for signed_token mode"))
		return
	}

	ttl := defaultAPIPrincipalTestTokenTTL
	if req.ExpiresInSeconds > 0 {
		ttl = time.Duration(req.ExpiresInSeconds) * time.Second
	}
	if ttl <= 0 || ttl > maxAPIPrincipalTestTokenTTL {
		c.Error(errors.NewValidationError("expires_in_seconds must be between 1 and 3600"))
		return
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       externalUserID,
		"tenant_id": strconv.FormatUint(id, 10),
		"aud":       middleware.ExternalUserTokenAudience,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to create API principal test token").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": apiPrincipalTestTokenResponse{
			Token:            token,
			HeaderName:       defaultAPIPrincipalTokenHeader,
			ExpiresInSeconds: int(ttl.Seconds()),
			ExpiresAtUnix:    expiresAt.Unix(),
			ExternalUserID:   externalUserID,
		},
	})
}

func validateAPIPrincipalExternalUserID(id string) error {
	if id == "" {
		return errors.NewValidationError("external_user_id is required")
	}
	if len(id) > maxAPIPrincipalExternalUserIDLen {
		return errors.NewValidationError("external_user_id is too long")
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return errors.NewValidationError("external_user_id contains invalid characters")
		}
	}
	return nil
}

// DeleteTenant godoc
// @Summary      删除工作区
// @Description  删除工作区。两种情况下拒绝：工作区里还有调用者以外的成员（code 2007），
// @Description  或这是部署中最后一个工作区（code 2008）。先把其他成员移出，再删除。
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "工作区ID"
// @Success      200  {object}  map[string]interface{}  "删除成功"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      409  {object}  errors.AppError         "工作区仍有其他成员 / 最后一个工作区"
// @Security     Bearer
// @Router       /tenants/{id} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start deleting tenant")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	// Deleting a workspace deletes its memberships with it. Other members
	// would silently lose access, and their sessions would keep pointing at
	// a workspace that is gone, so the Owner has to move or remove them
	// first; only their own membership may go down with the workspace.
	callerID, _ := types.UserIDFromContext(ctx)
	members, err := h.memberService.ListByTenant(ctx, id)
	if err != nil {
		logger.Errorf(ctx, "ListByTenant failed before deleting tenant %d: %v", id, err)
		_ = c.Error(errors.NewInternalServerError("Failed to check workspace members").WithDetails(err.Error()))
		return
	}
	others := 0
	for _, m := range members {
		if m != nil && m.Status == types.TenantMemberStatusActive && m.UserID != callerID {
			others++
		}
	}
	if others > 0 {
		_ = c.Error(errors.NewTenantHasMembersError(others))
		return
	}

	// A deployment always keeps at least one workspace: there is no other
	// way to put a user anywhere, and bootstrap only runs once.
	total, err := h.service.CountTenants(ctx)
	if err != nil {
		logger.Errorf(ctx, "CountTenants failed before deleting tenant %d: %v", id, err)
		_ = c.Error(errors.NewInternalServerError("Failed to count workspaces").WithDetails(err.Error()))
		return
	}
	if total <= 1 {
		_ = c.Error(errors.NewTenantLastWorkspaceError())
		return
	}

	logger.Infof(ctx, "Deleting tenant, ID: %d", id)

	if err := h.service.DeleteTenant(ctx, id); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to delete workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to delete workspace").WithDetails(err.Error()))
		}
		return
	}

	h.emitTenantAudit(ctx, types.AuditActionSystemTenantDeleted, id, nil)

	logger.Infof(ctx, "Workspace deleted successfully, ID: %d", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Workspace deleted successfully",
	})
}

// ListTenants godoc
// @Summary      获取工作区列表
// @Description  获取当前用户可访问的工作区列表
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "工作区列表"
// @Failure      500  {object}  errors.AppError         "服务器错误"
// @Security     Bearer
// @Router       /tenants [get]
func (h *TenantHandler) ListTenants(c *gin.Context) {
	ctx := c.Request.Context()

	tenant, ok := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if !ok || tenant == nil {
		c.Error(errors.NewUnauthorizedError("Authentication required"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": []*dto.TenantResponse{dto.NewTenantResponse(ctx, tenant)},
		},
	})
}

// attachMemberCounts fills TenantResponse.MemberCount for the catalog
// endpoints in one batched query. Best-effort: the list is still useful
// without the counts, so a failed count is logged and the field stays
// omitted rather than failing the whole response.
func (h *TenantHandler) attachMemberCounts(ctx context.Context, items []*dto.TenantResponse) {
	if h.memberService == nil || len(items) == 0 {
		return
	}
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	counts, err := h.memberService.CountMembersByTenants(ctx, ids)
	if err != nil {
		logger.Warnf(ctx, "failed to count workspace members for the catalog list: %v", err)
		return
	}
	for _, item := range items {
		n := counts[item.ID]
		item.MemberCount = &n
	}
}

// ListAllTenants godoc
// @Summary      获取所有工作区列表
// @Description  获取部署中的所有工作区，附带每个工作区的成员数（系统管理员 / 跨工作区超管 / 平台 API Key）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "所有工作区列表"
// @Failure      403  {object}  errors.AppError         "权限不足"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/all [get]
func (h *TenantHandler) ListAllTenants(c *gin.Context) {
	ctx := c.Request.Context()

	// Catalog gating (system administrator / cross-tenant superuser /
	// platform key) is enforced at the route layer via
	// middleware.RequireTenantCatalogAccess. The handler stays focused on
	// listing.
	tenants, err := h.service.ListAllTenants(ctx)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to retrieve all workspaces list: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to retrieve all workspaces list").WithDetails(err.Error()))
		}
		return
	}

	items := dto.NewTenantResponsesCrossTenant(tenants)
	h.attachMemberCounts(ctx, items)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": items,
		},
	})
}

// SearchTenants godoc
// @Summary      搜索工作区
// @Description  分页搜索工作区，附带成员数（系统管理员 / 跨工作区超管 / 平台 API Key）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        keyword    query     string  false  "搜索关键词"
// @Param        tenant_id  query     int     false  "工作区ID筛选"
// @Param        page       query     int     false  "页码"  default(1)
// @Param        page_size  query     int     false  "每页数量"  default(20)
// @Success      200        {object}  map[string]interface{}  "搜索结果"
// @Failure      403        {object}  errors.AppError         "权限不足"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/search [get]
func (h *TenantHandler) SearchTenants(c *gin.Context) {
	ctx := c.Request.Context()

	// Catalog gating is enforced at the route layer via
	// middleware.RequireTenantCatalogAccess; the handler only parses query
	// params and delegates to the service.

	// Parse query parameters
	keyword := c.Query("keyword")
	tenantIDStr := c.Query("tenant_id")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	var tenantID uint64
	if tenantIDStr != "" {
		parsedID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err == nil {
			tenantID = parsedID
		}
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Limit max page size
	}

	tenants, total, err := h.service.SearchTenants(ctx, keyword, tenantID, page, pageSize)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to search workspaces: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to search workspaces").WithDetails(err.Error()))
		}
		return
	}

	items := dto.NewTenantResponsesCrossTenant(tenants)
	h.attachMemberCounts(ctx, items)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetTenantKV godoc
// @Summary      获取工作区KV配置
// @Description  获取工作区级别的KV配置（支持web-search-config、prompt-templates、parser-engine-config、
// @Description  chat-history-config、retrieval-config）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        key  path      string  true  "配置键名"
// @Success      200  {object}  map[string]interface{}  "配置值"
// @Failure      400  {object}  errors.AppError         "不支持的键"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/{key} [get]
func (h *TenantHandler) GetTenantKV(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "web-search-config", "parser-engine-config":
		if !dto.CanViewIntegrationSecrets(ctx) {
			c.Error(errors.NewForbiddenError("integration configuration requires admin access"))
			return
		}
	}

	switch key {
	case "web-search-config":
		h.GetTenantWebSearchConfig(c)
		return
	case "prompt-templates":
		h.GetPromptTemplates(c)
		return
	case "parser-engine-config":
		h.GetTenantParserEngineConfig(c)
		return
	case "chat-history-config":
		h.GetTenantChatHistoryConfig(c)
		return
	case "retrieval-config":
		h.GetTenantRetrievalConfig(c)
		return
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
		return
	}
}

// UpdateTenantKV godoc
// @Summary      更新工作区KV配置
// @Description  更新工作区级别的KV配置（支持web-search-config、parser-engine-config、chat-history-config、retrieval-config）
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Param        key      path      string  true  "配置键名"
// @Param        request  body      object  true  "配置值"
// @Success      200      {object}  map[string]interface{}  "更新成功"
// @Failure      400      {object}  errors.AppError         "不支持的键"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/{key} [put]
func (h *TenantHandler) UpdateTenantKV(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "web-search-config", "parser-engine-config":
		if !dto.CanViewIntegrationSecrets(ctx) {
			c.Error(errors.NewForbiddenError("integration configuration requires admin access"))
			return
		}
		// These keys carry infrastructure configuration, so under
		// centralised mode they belong to the platform. The route itself is
		// one PUT /tenants/kv/:key dispatching on the key, so the guard
		// cannot be split at the router layer — it has to happen here,
		// alongside the workspace keys (chat-history / retrieval / memory /
		// prompt-templates) which stay with the workspace admin in both modes.
		if !h.canWritePlatformManagedKV(ctx) {
			c.Error(errors.NewForbiddenError(
				"this configuration is managed by the platform administrator"))
			return
		}
	}

	switch key {
	case "web-search-config":
		h.updateTenantWebSearchConfigInternal(c)
		return
	case "parser-engine-config":
		h.updateTenantParserEngineConfigInternal(c)
		return
	case "chat-history-config":
		h.updateTenantChatHistoryConfigInternal(c)
		return
	case "retrieval-config":
		h.updateTenantRetrievalConfigInternal(c)
		return
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
		return
	}
}

// canWritePlatformManagedKV reports whether the caller may write an
// infrastructure KV key (web-search / parser-engine config).
//
// It mirrors middleware.RequirePlatformManaged: SystemAdmins always pass,
// API-key principals are authorised by the APIKeyGate rather than here, and
// everyone else passes only while centralised-infrastructure mode is off. A
// missing system-setting service is treated as "off" so a wiring gap degrades
// to the historical behaviour instead of locking workspace admins out.
func (h *TenantHandler) canWritePlatformManagedKV(ctx context.Context) bool {
	if types.IsSystemAdminFromContext(ctx) {
		return true
	}
	if _, ok := types.TenantAPIKeyScopeFromContext(ctx); ok {
		return true
	}
	if h.systemSettingSvc == nil {
		return true
	}
	return !h.systemSettingSvc.GetBool(
		ctx, types.SettingKeyCentralizedInfra, types.SettingEnvCentralizedInfra, false,
	)
}

// updateTenantWebSearchConfigInternal updates tenant's web search config
func (h *TenantHandler) updateTenantWebSearchConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	// Bind directly into the strong typed struct
	var cfg types.WebSearchConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	cfg = *types.MergeWebSearchConfigForUpdate(&cfg, tenant.WebSearchConfig)

	// Validate configuration
	if cfg.MaxResults < 1 || cfg.MaxResults > 50 {
		c.Error(errors.NewBadRequestError("max_results must be between 1 and 50"))
		return
	}

	tenant.WebSearchConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to update workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace web search config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.WebSearchConfigForResponse(updatedTenant.WebSearchConfig, true),
		"message": "Web search configuration updated successfully",
	})
}

// GetTenantWebSearchConfig godoc
// @Summary      获取工作区网络搜索配置
// @Description  获取工作区的网络搜索配置
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "网络搜索配置"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/web-search-config [get]
func (h *TenantHandler) GetTenantWebSearchConfig(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start getting tenant web search config")
	// Get tenant
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	logger.Infof(ctx, "Tenant web search config retrieved successfully, Tenant ID: %d", tenant.ID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.WebSearchConfigForResponse(tenant.WebSearchConfig, true),
	})
}

// GetTenantParserEngineConfig returns the tenant's parser engine config (MinerU endpoint, API key, etc.).
func (h *TenantHandler) GetTenantParserEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := types.ParserEngineConfigForResponse(tenant.ParserEngineConfig, true)
	if data == nil {
		data = &types.ParserEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantParserEngineConfigInternal updates the tenant's parser engine config.
func (h *TenantHandler) updateTenantParserEngineConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.ParserEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	merged := types.MergeParserEngineConfigForUpdate(&cfg, tenant.ParserEngineConfig)
	if err := validateParserEngineOutboundURLs(merged); err != nil {
		c.Error(errors.NewValidationError(err.Error()))
		return
	}
	tenant.ParserEngineConfig = merged
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace parser engine config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.ParserEngineConfigForResponse(updatedTenant.ParserEngineConfig, true),
		"message": "解析引擎配置已更新",
	})
}

// GetPromptTemplates godoc
// @Summary      获取提示词模板
// @Description  获取系统配置的提示词模板列表
// @Tags         工作区管理
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "提示词模板配置"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/prompt-templates [get]
func (h *TenantHandler) GetPromptTemplates(c *gin.Context) {
	// Return prompt templates from config.yaml
	templates := h.config.PromptTemplates
	if templates == nil {
		templates = &config.PromptTemplatesConfig{}
	}

	// Determine user language from context (set by Language middleware)
	lang := types.LanguageFromContextOrDefault(c.Request.Context())

	// Build a localized copy so the original config is never mutated
	localized := &config.PromptTemplatesConfig{
		SystemPrompt:         config.LocalizeTemplates(templates.SystemPrompt, lang),
		ContextTemplate:      config.LocalizeTemplates(templates.ContextTemplate, lang),
		Rewrite:              config.LocalizeTemplates(templates.Rewrite, lang),
		Fallback:             config.LocalizeTemplates(templates.Fallback, lang),
		GenerateSessionTitle: templates.GenerateSessionTitle,
		GenerateSummary:      templates.GenerateSummary,
		KeywordsExtraction:   templates.KeywordsExtraction,
		IntentPrompts:        config.LocalizeTemplates(templates.IntentPrompts, lang),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    localized,
	})
}

// GetTenantChatHistoryConfig returns the tenant's chat history KB configuration.
func (h *TenantHandler) GetTenantChatHistoryConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.ChatHistoryConfig
	if data == nil {
		data = &types.ChatHistoryConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantChatHistoryConfigInternal updates the tenant's chat history KB configuration.
// When enabled with an embedding model and no KB exists yet, it auto-creates a hidden KB.
func (h *TenantHandler) updateTenantChatHistoryConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	// The frontend sends: enabled, embedding_model_id
	// knowledge_base_id is managed internally.
	var req types.ChatHistoryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	existing := tenant.ChatHistoryConfig

	// Build the new config, preserving the internally-managed knowledge_base_id
	cfg := &types.ChatHistoryConfig{
		Enabled:          req.Enabled,
		EmbeddingModelID: req.EmbeddingModelID,
		KnowledgeBaseID:  "", // will be set below
	}

	// Carry over existing KB ID if the embedding model hasn't changed
	if existing != nil && existing.KnowledgeBaseID != "" {
		if existing.EmbeddingModelID == req.EmbeddingModelID {
			cfg.KnowledgeBaseID = existing.KnowledgeBaseID
		} else {
			// Embedding model changed — the old KB is incompatible.
			// We'll create a new one below. The old KB remains but is orphaned (can be cleaned up later).
			logger.Infof(ctx, "Embedding model changed from %s to %s, will create new chat history KB", existing.EmbeddingModelID, req.EmbeddingModelID)
		}
	}

	// Auto-create hidden KB if enabled + model set + no KB yet
	if cfg.Enabled && cfg.EmbeddingModelID != "" && cfg.KnowledgeBaseID == "" {
		kb := &types.KnowledgeBase{
			Name:             "__chat_history__",
			Type:             types.KnowledgeBaseTypeDocument,
			IsTemporary:      true,
			Description:      "Auto-managed knowledge base for chat history message indexing",
			EmbeddingModelID: cfg.EmbeddingModelID,
		}
		createdKB, err := h.kbService.CreateKnowledgeBase(ctx, kb)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create chat history knowledge base").WithDetails(err.Error()))
			return
		}
		cfg.KnowledgeBaseID = createdKB.ID
		logger.Infof(ctx, "Auto-created chat history KB: id=%s, embedding_model=%s", createdKB.ID, cfg.EmbeddingModelID)
	}

	tenant.ChatHistoryConfig = cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update chat history config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.ChatHistoryConfig,
		"message": "Chat history configuration updated successfully",
	})
}

// GetTenantRetrievalConfig returns the tenant's global retrieval configuration.
func (h *TenantHandler) GetTenantRetrievalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.RetrievalConfig
	if data == nil {
		data = &types.RetrievalConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantRetrievalConfigInternal updates the tenant's global retrieval configuration.
func (h *TenantHandler) updateTenantRetrievalConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.RetrievalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	// Validate thresholds
	if cfg.VectorThreshold < 0 || cfg.VectorThreshold > 1 {
		c.Error(errors.NewBadRequestError("vector_threshold must be between 0 and 1"))
		return
	}
	if cfg.KeywordThreshold < 0 || cfg.KeywordThreshold > 1 {
		c.Error(errors.NewBadRequestError("keyword_threshold must be between 0 and 1"))
		return
	}
	if cfg.RerankThreshold < -10 || cfg.RerankThreshold > 10 {
		c.Error(errors.NewBadRequestError("rerank_threshold must be between -10 and 10"))
		return
	}
	if cfg.EmbeddingTopK < 0 || cfg.EmbeddingTopK > 200 {
		c.Error(errors.NewBadRequestError("embedding_top_k must be between 0 and 200"))
		return
	}
	if cfg.RerankTopK < 0 || cfg.RerankTopK > 200 {
		c.Error(errors.NewBadRequestError("rerank_top_k must be between 0 and 200"))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	tenant.RetrievalConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update retrieval config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.RetrievalConfig,
		"message": "Retrieval configuration updated successfully",
	})
}

func validateParserEngineOutboundURLs(cfg *types.ParserEngineConfig) error {
	if cfg == nil {
		return nil
	}
	if endpoint := strings.TrimSpace(cfg.MinerUEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("mineru_endpoint failed SSRF validation: %v", err)
		}
	}
	if vlmURL := strings.TrimSpace(cfg.MinerUVLMServerURL); vlmURL != "" {
		if err := secutils.ValidateURLForSSRF(vlmURL); err != nil {
			return fmt.Errorf("mineru_vlm_server_url failed SSRF validation: %v", err)
		}
	}
	if odlURL := strings.TrimSpace(cfg.ODLHybridURL); odlURL != "" {
		if err := secutils.ValidateURLForSSRF(odlURL); err != nil {
			return fmt.Errorf("odl_hybrid_url failed SSRF validation: %v", err)
		}
	}
	if endpoint := strings.TrimSpace(cfg.PaddleOCRVLEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("paddleocr_vl_endpoint failed SSRF validation: %v", err)
		}
	}
	return nil
}
