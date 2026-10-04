package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/handler/dto"
	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// storageTestErrorMessage returns a safe user-facing message for a storage
// connectivity test failure. Validation/AppError messages are already
// user-facing and are passed through, while raw driver/network errors are
// sanitized so they don't leak internal hostnames, IPs, ports or TLS details.
func storageTestErrorMessage(err error) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return secutils.SanitizeStorageConnectivityError(err)
}

type StorageBackendHandler struct {
	repo    interfaces.StorageBackendRepository
	service interfaces.StorageBackendService
}

func NewStorageBackendHandler(repo interfaces.StorageBackendRepository, service interfaces.StorageBackendService) *StorageBackendHandler {
	return &StorageBackendHandler{repo: repo, service: service}
}

type storageBackendRequest struct {
	Name     string                     `json:"name" binding:"required"`
	Provider string                     `json:"provider" binding:"required"`
	Config   types.StorageBackendConfig `json:"config"`
	Status   string                     `json:"status,omitempty"`
}

func storageTenantID(c *gin.Context) uint64 { return c.GetUint64(types.TenantIDContextKey.String()) }

// List godoc
// @Summary      List storage backends
// @Description  List all storage backend instances for the current workspace, with credentials masked. The workspace default backend id is returned alongside the list.
// @Tags         StorageBackend
// @Produce      json
// @Success      200  {object}  map[string]interface{}   "List of storage backends and default_storage_backend_id"
// @Failure      401  {object}  map[string]interface{}   "Unauthorized"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends [get]
func (h *StorageBackendHandler) List(c *gin.Context) {
	// Platform-shared backends hide their endpoint/bucket detail from everyone
	// but platform administrators; see dto.CanSeeSharedInfraDetail.
	sharedDetail := dto.CanSeeSharedInfraDetail(c.Request.Context())
	tenantID := storageTenantID(c)
	backends, err := h.repo.List(c.Request.Context(), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	result := make([]types.StorageBackend, 0, len(backends))
	for _, backend := range backends {
		result = append(result, types.NewStorageBackendResponseWithSharedDetail(backend, sharedDetail))
	}
	tenant, _ := types.TenantInfoFromContext(c.Request.Context())
	defaultID := ""
	if tenant != nil {
		defaultID = tenant.DefaultStorageBackendID
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result, "default_storage_backend_id": defaultID})
}

// Get godoc
// @Summary      Get storage backend
// @Description  Retrieve a single storage backend by ID for the current workspace. Credentials are masked.
// @Tags         StorageBackend
// @Produce      json
// @Param        id   path      string  true  "Storage backend ID"
// @Success      200  {object}  map[string]interface{}   "Storage backend details"
// @Failure      401  {object}  map[string]interface{}   "Unauthorized"
// @Failure      404  {object}  apperrors.AppError          "Storage backend not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/{id} [get]
func (h *StorageBackendHandler) Get(c *gin.Context) {
	// Platform-shared backends hide their endpoint/bucket detail from everyone
	// but platform administrators; see dto.CanSeeSharedInfraDetail.
	sharedDetail := dto.CanSeeSharedInfraDetail(c.Request.Context())
	backend, err := h.repo.GetByID(c.Request.Context(), storageTenantID(c), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	if backend == nil {
		c.Error(apperrors.NewNotFoundError("storage backend not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": types.NewStorageBackendResponseWithSharedDetail(backend, sharedDetail)})
}

// Create godoc
// @Summary      Create storage backend
// @Description  Register a new object/file storage instance for the current workspace. The configuration is validated and a connectivity test is run before the backend is persisted.
// @Tags         StorageBackend
// @Accept       json
// @Produce      json
// @Param        request  body      storageBackendRequest    true  "Storage backend configuration"
// @Success      201      {object}  map[string]interface{}   "Created storage backend"
// @Failure      400      {object}  apperrors.AppError          "Invalid request, validation, or connectivity test failure"
// @Failure      401      {object}  map[string]interface{}   "Unauthorized"
// @Failure      409      {object}  apperrors.AppError          "A storage backend with this name already exists"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends [post]
func (h *StorageBackendHandler) Create(c *gin.Context) {
	// Platform-shared backends hide their endpoint/bucket detail from everyone
	// but platform administrators; see dto.CanSeeSharedInfraDetail.
	sharedDetail := dto.CanSeeSharedInfraDetail(c.Request.Context())
	var req storageBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	backend := &types.StorageBackend{TenantID: storageTenantID(c), Name: req.Name, Provider: req.Provider, Config: req.Config, Status: req.Status}
	if err := h.service.Create(c.Request.Context(), backend); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": types.NewStorageBackendResponseWithSharedDetail(backend, sharedDetail)})
}

// Update godoc
// @Summary      Update storage backend
// @Description  Update a storage backend's mutable fields (name, credentials, status). Provider and physical location (endpoint, region, bucket, path prefix) are immutable; use storage migration to move data. Environment-sourced backends are read-only. Redacted secret placeholders preserve the stored credentials.
// @Tags         StorageBackend
// @Accept       json
// @Produce      json
// @Param        id       path      string                   true  "Storage backend ID"
// @Param        request  body      storageBackendRequest    true  "Updated storage backend fields"
// @Success      200      {object}  map[string]interface{}   "Updated storage backend"
// @Failure      400      {object}  apperrors.AppError          "Immutable field change, read-only backend, validation, or connectivity failure"
// @Failure      401      {object}  map[string]interface{}   "Unauthorized"
// @Failure      404      {object}  apperrors.AppError          "Storage backend not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/{id} [put]
func (h *StorageBackendHandler) Update(c *gin.Context) {
	// Platform-shared backends hide their endpoint/bucket detail from everyone
	// but platform administrators; see dto.CanSeeSharedInfraDetail.
	sharedDetail := dto.CanSeeSharedInfraDetail(c.Request.Context())
	var req storageBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	backend := &types.StorageBackend{ID: c.Param("id"), TenantID: storageTenantID(c), Name: req.Name, Provider: req.Provider, Config: req.Config, Status: req.Status}
	if err := h.service.Update(c.Request.Context(), backend); err != nil {
		c.Error(err)
		return
	}
	updated, err := h.repo.GetByID(c.Request.Context(), backend.TenantID, backend.ID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": types.NewStorageBackendResponseWithSharedDetail(updated, sharedDetail)})
}

// Delete godoc
// @Summary      Delete storage backend
// @Description  Soft-delete a storage backend the workspace owns. Refused while any workspace still uses it
// @Description  (a workspace default, a knowledge base or docs space bound to it, or a stored file on it),
// @Description  while it is platform-shared, and always for the deployment backend.
// @Tags         StorageBackend
// @Produce      json
// @Param        id   path      string  true  "Storage backend ID"
// @Success      200  {object}  map[string]interface{}   "Deletion success"
// @Failure      400  {object}  apperrors.AppError          "Backend still in use, shared, or the deployment backend"
// @Failure      401  {object}  map[string]interface{}   "Unauthorized"
// @Failure      404  {object}  apperrors.AppError          "Storage backend not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/{id} [delete]
func (h *StorageBackendHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), storageTenantID(c), c.Param("id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SetDefault godoc
// @Summary      Set default storage backend
// @Description  Mark a storage backend as the workspace default: any active backend the workspace can see
// @Description  (its own, a platform-shared one, or the deployment backend). New knowledge bases and docs
// @Description  spaces without an explicit binding, and the workspace's own uploads (chat images,
// @Description  attachments), use the default.
// @Tags         StorageBackend
// @Produce      json
// @Param        id   path      string  true  "Storage backend ID"
// @Success      200  {object}  map[string]interface{}   "Default set successfully"
// @Failure      400  {object}  apperrors.AppError          "Backend is not active"
// @Failure      401  {object}  map[string]interface{}   "Unauthorized"
// @Failure      404  {object}  apperrors.AppError          "Storage backend not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/{id}/default [put]
func (h *StorageBackendHandler) SetDefault(c *gin.Context) {
	if err := h.service.SetDefault(c.Request.Context(), storageTenantID(c), c.Param("id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestRaw godoc
// @Summary      Test storage backend with raw config
// @Description  Test connectivity for the provided storage configuration without persisting it. Returns success=false with a sanitized error message on failure (the HTTP status stays 200).
// @Tags         StorageBackend
// @Accept       json
// @Produce      json
// @Param        request  body      storageBackendRequest    true  "Storage backend configuration to test"
// @Success      200      {object}  map[string]interface{}   "Connectivity test result (success, error)"
// @Failure      400      {object}  apperrors.AppError          "Invalid request or validation error"
// @Failure      401      {object}  map[string]interface{}   "Unauthorized"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/test [post]
func (h *StorageBackendHandler) TestRaw(c *gin.Context) {
	var req storageBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	backend := &types.StorageBackend{TenantID: storageTenantID(c), Name: req.Name, Provider: req.Provider, Config: req.Config}
	if err := backend.Validate(); err != nil {
		c.Error(err)
		return
	}
	if err := h.service.Test(c.Request.Context(), backend); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": storageTestErrorMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestByID godoc
// @Summary      Test storage backend by ID
// @Description  Test connectivity of an existing saved storage backend using its stored credentials. Returns success=false with a sanitized error message on failure (the HTTP status stays 200).
// @Tags         StorageBackend
// @Produce      json
// @Param        id   path      string  true  "Storage backend ID"
// @Success      200  {object}  map[string]interface{}   "Connectivity test result (success, error)"
// @Failure      401  {object}  map[string]interface{}   "Unauthorized"
// @Failure      404  {object}  apperrors.AppError          "Storage backend not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/{id}/test [post]
func (h *StorageBackendHandler) TestByID(c *gin.Context) {
	backend, err := h.repo.GetByID(c.Request.Context(), storageTenantID(c), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	if backend == nil {
		c.Error(apperrors.NewNotFoundError("storage backend not found"))
		return
	}
	if err := h.service.Test(c.Request.Context(), backend); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": storageTestErrorMessage(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Types godoc
// @Summary      List allowed storage provider types
// @Description  Return the storage provider types allowed by STORAGE_ALLOW_LIST for UI form generation (local, s3).
// @Tags         StorageBackend
// @Produce      json
// @Success      200  {object}  map[string]interface{}   "List of allowed storage provider types"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /storage-backends/types [get]
func (h *StorageBackendHandler) Types(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": storageallowlist.AllowedList()})
}

// UpdateSharing godoc
// @Summary      设置存储实例平台共享
// @Description  将存储实例设为平台共享（所有工作区可见可选用，端点与凭据对非系统管理员隐藏）或取消共享。
// @Description  仅系统管理员可调用。取消共享时，若 owner 之外的工作区仍有默认存储、知识库或活跃资源绑定，请求会被拒绝。
// @Tags         StorageBackend
// @Accept       json
// @Produce      json
// @Param        id       path      string                 true  "存储实例 ID"
// @Param        request  body      SetSharingRequest      true  "共享状态"
// @Success      200      {object}  map[string]interface{} "更新后的存储实例"
// @Failure      400      {object}  apperrors.AppError     "仍被其他工作区引用"
// @Failure      403      {object}  apperrors.AppError     "非系统管理员"
// @Security     Bearer
// @Router       /storage-backends/{id}/sharing [put]
func (h *StorageBackendHandler) UpdateSharing(c *gin.Context) {
	var req SetSharingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}

	backend, err := h.service.SetSharing(c.Request.Context(), c.Param("id"), *req.Shared)
	if err != nil {
		c.Error(err)
		return
	}

	// Necessarily a platform administrator here (the service enforces it), so
	// the unredacted projection is the right one.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.NewStorageBackendResponseWithSharedDetail(backend, true),
	})
}
