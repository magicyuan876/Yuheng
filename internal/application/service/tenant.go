package service

import (
	"context"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// ListTenantsParams defines parameters for listing tenants with filtering and pagination
type ListTenantsParams struct {
	Page     int    // Page number for pagination
	PageSize int    // Number of items per page
	Status   string // Filter by tenant status
	Name     string // Filter by tenant name
}

// tenantService implements the TenantService interface
type tenantService struct {
	repo interfaces.TenantRepository // Repository for tenant data operations
}

// NewTenantService creates a new tenant service instance
func NewTenantService(repo interfaces.TenantRepository) interfaces.TenantService {
	return &tenantService{repo: repo}
}

// CreateTenant creates a new tenant
func (s *tenantService) CreateTenant(ctx context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	logger.Info(ctx, "Start creating tenant")

	if tenant.Name == "" {
		logger.Error(ctx, "Workspace name cannot be empty")
		return nil, errors.New("workspace name cannot be empty")
	}

	logger.Infof(ctx, "Creating tenant, name: %s", tenant.Name)

	// New tenants do not receive an API key by default. Integrations create
	// keys explicitly through tenant_api_keys.
	tenant.Status = "active"
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()
	// Every workspace writes somewhere from the start: the deployment
	// backend, until it registers or picks another default.
	if tenant.DefaultStorageBackendID == "" {
		tenant.DefaultStorageBackendID = types.EnvStorageBackendID
	}

	logger.Info(ctx, "Saving tenant information to database")
	if err := s.repo.CreateTenant(ctx, tenant); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_name": tenant.Name,
		})
		return nil, err
	}

	logger.Infof(ctx, "Tenant created successfully, ID: %d, name: %s", tenant.ID, tenant.Name)
	return tenant, nil
}

// GetTenantByID retrieves a tenant by their ID
func (s *tenantService) GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error) {
	if id == 0 {
		logger.Error(ctx, "Workspace ID cannot be 0")
		return nil, errors.New("tenant ID cannot be 0")
	}

	tenant, err := s.repo.GetTenantByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": id,
		})
		return nil, err
	}

	return tenant, nil
}

// GetTenantsByIDs batches GetTenantByID; returns a map keyed by tenant ID.
func (s *tenantService) GetTenantsByIDs(ctx context.Context, ids []uint64) (map[uint64]*types.Tenant, error) {
	return s.repo.GetTenantsByIDs(ctx, ids)
}

// ListTenants retrieves a list of all tenants
func (s *tenantService) ListTenants(ctx context.Context) ([]*types.Tenant, error) {
	tenants, err := s.repo.ListTenants(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return nil, err
	}

	logger.Infof(ctx, "Tenant list retrieved successfully, total: %d", len(tenants))
	return tenants, nil
}

// UpdateTenant updates an existing tenant's information
func (s *tenantService) UpdateTenant(ctx context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	if tenant.ID == 0 {
		logger.Error(ctx, "Workspace ID cannot be 0")
		return nil, errors.New("tenant ID cannot be 0")
	}

	logger.Infof(ctx, "Updating tenant, ID: %d, name: %s", tenant.ID, tenant.Name)

	tenant.UpdatedAt = time.Now()
	logger.Info(ctx, "Saving tenant information to database")

	if err := s.repo.UpdateTenant(ctx, tenant); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenant.ID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Tenant updated successfully, ID: %d", tenant.ID)
	return tenant, nil
}

// DeleteTenant removes a tenant by their ID
func (s *tenantService) DeleteTenant(ctx context.Context, id uint64) error {
	logger.Info(ctx, "Start deleting tenant")

	if id == 0 {
		logger.Error(ctx, "Workspace ID cannot be 0")
		return errors.New("tenant ID cannot be 0")
	}

	logger.Infof(ctx, "Deleting tenant, ID: %d", id)

	// Get tenant information for logging
	tenant, err := s.repo.GetTenantByID(ctx, id)
	if err != nil {
		if err.Error() == "record not found" {
			logger.Warnf(ctx, "Tenant to be deleted does not exist, ID: %d", id)
		} else {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"tenant_id": id,
			})
			return err
		}
	} else {
		logger.Infof(ctx, "Deleting tenant, ID: %d, name: %s", id, tenant.Name)
	}

	err = s.repo.DeleteTenant(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": id,
		})
		return err
	}

	logger.Infof(ctx, "Workspace deleted successfully, ID: %d", id)
	return nil
}

// ListAllTenants lists all tenants (for users with cross-tenant access permission)
// This method returns all tenants without filtering, intended for admin users
func (s *tenantService) ListAllTenants(ctx context.Context) ([]*types.Tenant, error) {
	tenants, err := s.repo.ListTenants(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return nil, err
	}

	logger.Infof(ctx, "All tenants list retrieved successfully, total: %d", len(tenants))
	return tenants, nil
}

// CountTenants proxies to the repository.
func (s *tenantService) CountTenants(ctx context.Context) (int64, error) {
	return s.repo.CountTenants(ctx)
}

// BulkSetStorageQuota delegates to the repository. Zero is a legitimate
// value — the storage-quota checks treat StorageQuota <= 0 as "unlimited"
// and the default setting is 0 — so only a negative quota, which can only
// come from a bug, is rejected.
func (s *tenantService) BulkSetStorageQuota(ctx context.Context, quotaBytes int64) (int64, error) {
	if quotaBytes < 0 {
		return 0, errors.New("quota must not be negative")
	}
	affected, err := s.repo.BulkSetStorageQuota(ctx, quotaBytes)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"quota_bytes": quotaBytes})
		return 0, err
	}
	logger.Infof(ctx, "Bulk set storage_quota=%d on %d tenants", quotaBytes, affected)
	return affected, nil
}

// SearchTenants searches tenants with pagination and filters
func (s *tenantService) SearchTenants(ctx context.Context, keyword string, tenantID uint64, page, pageSize int) ([]*types.Tenant, int64, error) {
	tenants, total, err := s.repo.SearchTenants(ctx, keyword, tenantID, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"keyword":  keyword,
			"tenantID": tenantID,
			"page":     page,
			"pageSize": pageSize,
		})
		return nil, 0, err
	}

	logger.Infof(ctx, "Tenants search completed, keyword: %s, tenantID: %d, page: %d, pageSize: %d, total: %d, found: %d",
		keyword, tenantID, page, pageSize, total, len(tenants))
	return tenants, total, nil
}

// GetTenantByIDForUser gets a tenant by ID with permission check
// This method verifies that the user has permission to access the tenant
func (s *tenantService) GetTenantByIDForUser(ctx context.Context, tenantID uint64, userID string) (*types.Tenant, error) {
	tenant, err := s.repo.GetTenantByID(ctx, tenantID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
			"user_id":   userID,
		})
		return nil, err
	}

	return tenant, nil
}
