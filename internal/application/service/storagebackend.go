package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	filesvc "github.com/magicyuan876/yuheng/internal/application/service/file"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StorageBackendService struct {
	repo            interfaces.StorageBackendRepository
	db              *gorm.DB
	resourceCatalog interfaces.ResourceCatalog
}

// NewStorageBackendService creates a storage backend service. The optional
// catalog keeps focused tests compatible while production uses the explicit
// constructor below.
func NewStorageBackendService(
	repo interfaces.StorageBackendRepository,
	db *gorm.DB,
	catalogs ...interfaces.ResourceCatalog,
) *StorageBackendService {
	service := &StorageBackendService{repo: repo, db: db}
	if len(catalogs) > 0 {
		service.resourceCatalog = catalogs[0]
	}
	return service
}

// NewStorageBackendServiceWithResources is the production DI constructor.
// The variadic constructor above remains convenient for focused tests that do
// not exercise resource registration.
func NewStorageBackendServiceWithResources(
	repo interfaces.StorageBackendRepository,
	db *gorm.DB,
	catalog interfaces.ResourceCatalog,
) *StorageBackendService {
	return NewStorageBackendService(repo, db, catalog)
}

func (s *StorageBackendService) Create(ctx context.Context, backend *types.StorageBackend) error {
	if err := backend.Validate(); err != nil {
		return err
	}
	if err := validateStorageBackendEndpoint(backend); err != nil {
		return err
	}
	if err := s.Test(ctx, backend); err != nil {
		return apperrors.NewBadRequestError("storage connection test failed").WithDetails(secutils.SanitizeStorageConnectivityError(err))
	}
	backend.CreatedAt, backend.UpdatedAt = time.Now(), time.Now()
	if err := s.repo.Create(ctx, backend); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return apperrors.NewConflictError("a storage backend with this name already exists")
		}
		return err
	}
	return nil
}

func (s *StorageBackendService) Update(ctx context.Context, incoming *types.StorageBackend) error {
	existing, err := s.repo.GetByID(ctx, incoming.TenantID, incoming.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperrors.NewNotFoundError("storage backend not found")
	}
	if existing.Source == types.StorageBackendSourceEnv {
		return apperrors.NewBadRequestError("environment storage backend is read-only")
	}
	// A platform-shared backend is visible to every workspace, so only a
	// system administrator may repoint it — and the write must be keyed on the
	// owning workspace, since the repository predicate is (tenant_id, id) and
	// the owner is rarely the caller's current workspace.
	owner, err := authorizePlatformSharedWrite(ctx, "storage backend", existing.IsBuiltin, existing.TenantID)
	if err != nil {
		return err
	}
	incoming.TenantID = owner
	incoming.IsBuiltin = existing.IsBuiltin
	incoming.Provider = existing.Provider
	incoming.Config = incoming.Config.MergeSecrets(existing.Config)
	if incoming.Config.LocationKey(existing.Provider) != existing.Config.LocationKey(existing.Provider) {
		return apperrors.NewBadRequestError("endpoint, region, bucket and path prefix are immutable; use storage migration instead")
	}
	if incoming.Status == "" {
		incoming.Status = existing.Status
	}
	if incoming.Status == types.StorageBackendStatusDisabled && existing.Status != types.StorageBackendStatusDisabled {
		var references int64
		if err := s.db.WithContext(ctx).Model(&types.Tenant{}).Where("id = ? AND default_storage_backend_id = ?", incoming.TenantID, incoming.ID).Count(&references).Error; err != nil {
			return err
		}
		if references == 0 {
			if err := s.db.WithContext(ctx).Model(&types.KnowledgeBase{}).Where("tenant_id = ? AND storage_backend_id = ?", incoming.TenantID, incoming.ID).Count(&references).Error; err != nil {
				return err
			}
		}
		if references == 0 {
			if err := s.db.WithContext(ctx).Model(&types.StoredResource{}).
				Where(
					"tenant_id = ? AND storage_backend_id = ? AND state = ?",
					incoming.TenantID,
					incoming.ID,
					types.ResourceStateActive,
				).
				Count(&references).Error; err != nil {
				return err
			}
		}
		if references > 0 {
			return apperrors.NewBadRequestError("a default or bound storage backend cannot be disabled")
		}
	}
	if err := incoming.Validate(); err != nil {
		return err
	}
	if err := validateStorageBackendEndpoint(incoming); err != nil {
		return err
	}
	if err := s.Test(ctx, incoming); err != nil {
		return apperrors.NewBadRequestError("storage connection test failed").WithDetails(secutils.SanitizeStorageConnectivityError(err))
	}
	incoming.UpdatedAt = time.Now()
	return s.repo.Update(ctx, incoming)
}

func (s *StorageBackendService) Delete(ctx context.Context, tenantID uint64, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var backend types.StorageBackend
		query := tx.Where("tenant_id = ? AND id = ?", tenantID, id).
			Clauses(clause.Locking{Strength: "UPDATE"})
		if err := query.First(&backend).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.NewNotFoundError("storage backend not found")
			}
			return err
		}
		if backend.Source == types.StorageBackendSourceEnv {
			return apperrors.NewBadRequestError("environment storage backend is read-only")
		}
		// Deleting a shared backend has to be a two-step operation: stop
		// sharing it first, then delete. Every reference check below is
		// tenant-scoped (tenant_id = ? AND storage_backend_id = ?), so
		// running them here would clear the owner's workspace while other
		// workspaces still had knowledge bases and resources bound to the
		// backend — and those bindings are bare string columns with no
		// foreign key, so they would be left dangling silently. SetSharing
		// runs the cross-tenant check that this path cannot.
		if backend.IsBuiltin {
			return apperrors.NewBadRequestError(
				"stop sharing this storage backend platform-wide before deleting it")
		}
		var defaultCount int64
		if err := tx.Model(&types.Tenant{}).Where("id = ? AND default_storage_backend_id = ?", tenantID, id).Count(&defaultCount).Error; err != nil {
			return err
		}
		if defaultCount > 0 {
			return apperrors.NewBadRequestError("default storage backend cannot be deleted")
		}
		var kbCount int64
		if err := tx.Model(&types.KnowledgeBase{}).Where("tenant_id = ? AND storage_backend_id = ?", tenantID, id).Count(&kbCount).Error; err != nil {
			return err
		}
		if kbCount > 0 {
			return apperrors.NewBadRequestError(fmt.Sprintf("storage backend still has %d knowledge base(s) bound to it", kbCount))
		}
		var resourceCount int64
		if err := tx.Model(&types.StoredResource{}).
			Where("tenant_id = ? AND storage_backend_id = ? AND state = ?", tenantID, id, types.ResourceStateActive).
			Count(&resourceCount).Error; err != nil {
			return err
		}
		if resourceCount > 0 {
			return apperrors.NewBadRequestError(fmt.Sprintf("storage backend still has %d active resource(s)", resourceCount))
		}
		if backend.LegacyAlias {
			return apperrors.NewBadRequestError("legacy storage backend cannot be deleted while old file paths may reference it")
		}
		return tx.Delete(&backend).Error
	})
}

// SetSharing publishes a storage backend to every workspace, or withdraws it.
// System administrators only.
//
// Withdrawal is refused while a workspace other than the owner still has a
// knowledge base, an active stored resource, or its workspace default bound to
// the backend. Those bindings are bare string columns with no foreign key, so
// stranding them produces upload and download failures with no clear cause.
//
// Object keys already carry the tenant (prefix/{tenantID}/{knowledgeID}/uuid),
// so two workspaces sharing one bucket never collide — only the credential and
// the endpoint are shared.
func (s *StorageBackendService) SetSharing(
	ctx context.Context, id string, shared bool,
) (*types.StorageBackend, error) {
	if err := authorizePlatformSharingChange(ctx, "storage backend"); err != nil {
		return nil, err
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperrors.NewNotFoundError("storage backend not found")
	}
	if existing.Source == types.StorageBackendSourceEnv {
		return nil, apperrors.NewBadRequestError(
			"environment storage backend is read-only; it is already available to every workspace")
	}
	if existing.IsBuiltin == shared {
		return existing, nil // idempotent
	}

	if !shared {
		bound, err := s.countForeignBindings(ctx, existing.TenantID, id)
		if err != nil {
			return nil, err
		}
		if bound > 0 {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf(
				"cannot stop sharing: %d binding(s) in other workspaces still use this storage backend",
				bound,
			))
		}
	}

	if err := s.db.WithContext(ctx).Model(&types.StorageBackend{}).
		Where("tenant_id = ? AND id = ?", existing.TenantID, id).
		Update("is_builtin", shared).Error; err != nil {
		return nil, err
	}
	existing.IsBuiltin = shared
	logger.Infof(ctx, "Storage backend %s platform sharing set to %v", id, shared)
	return existing, nil
}

// countForeignBindings counts references to a storage backend from workspaces
// OTHER than its owner: workspace defaults, knowledge bases and active stored
// resources. The owner may keep using its own backend after un-sharing.
func (s *StorageBackendService) countForeignBindings(
	ctx context.Context, ownerTenantID uint64, id string,
) (int64, error) {
	db := s.db.WithContext(ctx)
	var total int64
	for _, q := range []*gorm.DB{
		db.Model(&types.Tenant{}).
			Where("default_storage_backend_id = ? AND id <> ?", id, ownerTenantID),
		db.Model(&types.KnowledgeBase{}).
			Where("storage_backend_id = ? AND tenant_id <> ?", id, ownerTenantID),
		db.Model(&types.StoredResource{}).
			Where("storage_backend_id = ? AND tenant_id <> ? AND state = ?",
				id, ownerTenantID, types.ResourceStateActive),
	} {
		var n int64
		if err := q.Count(&n).Error; err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func (s *StorageBackendService) SetDefault(ctx context.Context, tenantID uint64, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var backend types.StorageBackend
		query := tx.Where("tenant_id = ? AND id = ?", tenantID, id).
			Clauses(clause.Locking{Strength: "UPDATE"})
		if err := query.First(&backend).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.NewNotFoundError("storage backend not found")
			}
			return err
		}
		if backend.Status != types.StorageBackendStatusActive {
			return apperrors.NewBadRequestError("only an active storage backend can be the default")
		}
		return tx.Model(&types.Tenant{}).Where("id = ?", tenantID).Update("default_storage_backend_id", id).Error
	})
}

func (s *StorageBackendService) Test(ctx context.Context, backend *types.StorageBackend) error {
	if err := backend.Validate(); err != nil {
		return err
	}
	if err := validateStorageBackendEndpoint(backend); err != nil {
		return err
	}
	if backend.Provider == "local" {
		baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
		if baseDir == "" {
			baseDir = "/data/files"
		}
		candidate := filepath.Join(baseDir, strings.Trim(strings.TrimSpace(backend.Config.PathPrefix), "/\\"))
		safeDir, err := secutils.SafePathUnderBase(baseDir, candidate)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(safeDir, 0o755); err != nil {
			return fmt.Errorf("create local storage directory: %w", err)
		}
	}
	c := backend.Config
	switch backend.Provider {
	case "local":
		fileService, _, err := filesvc.NewFileServiceFromStorageConfig("local", backend.ToStorageEngineConfig(), "")
		if err != nil {
			return err
		}
		return fileService.CheckConnectivity(ctx)
	case types.StorageProviderS3:
		return filesvc.CheckS3Connectivity(ctx, filesvc.S3Options{
			Endpoint: c.Endpoint, Region: c.Region, AccessKey: c.AccessKeyID, SecretKey: c.SecretAccessKey,
			BucketName: c.BucketName, UseSSL: c.UseSSL, AddressingStyle: c.AddressingStyle,
		})
	default:
		return fmt.Errorf("unsupported storage provider: %s", backend.Provider)
	}
}

func storageBackendID(id *string) string {
	if id == nil {
		return ""
	}
	return strings.TrimSpace(*id)
}

func storageEngineDefaultProvider(sec *types.StorageEngineConfig) string {
	if sec == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(sec.DefaultProvider))
}

// hydrateTenantStorage fills DefaultStorageBackendID / StorageEngineConfig from
// the workspace row when the caller only passed a stub Tenant{ID: ...} (the
// docs attachment store falls back to one when it cannot load the tenant row),
// and without the default the factory returns "empty provider".
func (s *StorageBackendService) hydrateTenantStorage(ctx context.Context, tenant *types.Tenant) *types.Tenant {
	if tenant == nil || tenant.ID == 0 || s.db == nil {
		return tenant
	}
	if storageBackendID(tenant.DefaultStorageBackendID) != "" {
		return tenant
	}
	var stored types.Tenant
	if err := s.db.WithContext(ctx).
		Select("id", "default_storage_backend_id", "storage_engine_config").
		Where("id = ?", tenant.ID).
		Take(&stored).Error; err != nil {
		return tenant
	}
	out := *tenant
	if storageBackendID(out.DefaultStorageBackendID) == "" {
		out.DefaultStorageBackendID = stored.DefaultStorageBackendID
	}
	if out.StorageEngineConfig == nil {
		out.StorageEngineConfig = stored.StorageEngineConfig
	}
	return &out
}

func (s *StorageBackendService) ResolveBackend(ctx context.Context, tenant *types.Tenant, backendID, provider string) (*types.StorageBackend, error) {
	if tenant == nil {
		return nil, fmt.Errorf("workspace context missing")
	}
	tenant = s.hydrateTenantStorage(ctx, tenant)
	backendID = strings.TrimSpace(backendID)
	provider = strings.ToLower(strings.TrimSpace(provider))
	if backendID == "" && provider != "" {
		backend, err := s.repo.FindLegacyAlias(ctx, tenant.ID, provider)
		if err != nil || backend != nil {
			return backend, err
		}
	}
	if backendID == "" {
		backendID = storageBackendID(tenant.DefaultStorageBackendID)
	}
	if backendID != "" {
		backend, err := s.repo.GetByID(ctx, tenant.ID, backendID)
		if err != nil {
			return nil, err
		}
		if backend == nil {
			return nil, fmt.Errorf("storage backend not found")
		}
		if backend.Status != types.StorageBackendStatusActive {
			return nil, fmt.Errorf("storage backend is not active")
		}
		return backend, nil
	}
	return nil, nil
}

func (s *StorageBackendService) ResolveFileService(ctx context.Context, tenant *types.Tenant, backendID, provider, localBaseDir string) (interfaces.FileService, string, error) {
	if tenant == nil {
		return nil, "", fmt.Errorf("workspace context missing")
	}
	tenant = s.hydrateTenantStorage(ctx, tenant)
	backend, err := s.ResolveBackend(ctx, tenant, backendID, provider)
	if err != nil {
		return nil, "", err
	}
	if backend != nil {
		inner, provider, err := filesvc.NewFileServiceFromStorageConfig(backend.Provider, backend.ToStorageEngineConfig(), localBaseDir)
		if err != nil {
			return nil, provider, err
		}
		scoped := filesvc.NewBackendScopedFileService(backend.ID, inner)
		return filesvc.NewResourceCatalogFileService(scoped, s.resourceCatalog), provider, nil
	}
	sec := tenant.StorageEngineConfig
	if strings.TrimSpace(provider) == "" && storageEngineDefaultProvider(sec) == "" {
		if env := types.StorageBackendFromEnvironment(tenant.ID); env != nil {
			provider = env.Provider
			if sec == nil {
				sec = env.ToStorageEngineConfig()
			}
		}
	}
	inner, resolvedProvider, err := filesvc.NewFileServiceFromStorageConfig(
		provider,
		sec,
		localBaseDir,
	)
	if err != nil {
		return nil, resolvedProvider, err
	}
	return filesvc.NewResourceCatalogFileService(inner, s.resourceCatalog), resolvedProvider, nil
}

func validateStorageBackendEndpoint(backend *types.StorageBackend) error {
	if backend.Provider == types.StorageProviderLocal {
		return nil
	}
	endpoint := filesvc.S3EndpointURL(backend.Config.Endpoint, backend.Config.UseSSL)
	if endpoint == "" {
		return nil
	}
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		return apperrors.NewBadRequestError("storage endpoint failed SSRF validation").WithDetails(err.Error())
	}
	return nil
}

var (
	_ interfaces.StorageBackendService  = (*StorageBackendService)(nil)
	_ interfaces.StorageBackendResolver = (*StorageBackendService)(nil)
)
