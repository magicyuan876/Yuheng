package service

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// StorageBackendService governs storage backend rows: registering, editing,
// sharing and retiring them, and choosing the one a new binding points at.
// Reading and writing bytes is FileStore's job, not this service's.
type StorageBackendService struct {
	repo interfaces.StorageBackendRepository
	db   *gorm.DB
}

// NewStorageBackendService creates a storage backend service.
func NewStorageBackendService(repo interfaces.StorageBackendRepository, db *gorm.DB) *StorageBackendService {
	return &StorageBackendService{repo: repo, db: db}
}

func (s *StorageBackendService) Create(ctx context.Context, backend *types.StorageBackend) error {
	// Only the startup sync writes the environment backend; nothing that
	// arrives through the API may claim to be it.
	backend.Source = types.StorageBackendSourceUser
	backend.IsBuiltin = false
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
		return errEnvBackendReadOnly()
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
	incoming.Source = existing.Source
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
		// A disabled backend takes no writes, so nothing may still send it
		// any — in whichever workspace the binding is.
		bound, err := s.countBindings(ctx, s.db, existing, false)
		if err != nil {
			return err
		}
		if bound.total() > 0 {
			return apperrors.NewBadRequestError("a storage backend still in use cannot be disabled: " + bound.String())
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

// Delete retires a backend its workspace owns. It is refused while anything
// in any workspace still uses it: an active stored resource (whose bytes are
// there), or a knowledge base, docs space or workspace default bound to it.
// None of those columns has a foreign key — rows are soft-deleted — so this
// check is what keeps a binding from dangling.
func (s *StorageBackendService) Delete(ctx context.Context, tenantID uint64, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var backend types.StorageBackend
		query := tx.Where("tenant_id = ? AND id = ?", tenantID, id).
			Clauses(clause.Locking{Strength: "UPDATE"})
		if err := query.First(&backend).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// The environment backend has no owning workspace, so the
				// query above never finds it; say why rather than 404.
				if id == types.EnvStorageBackendID {
					return errEnvBackendReadOnly()
				}
				return apperrors.NewNotFoundError("storage backend not found")
			}
			return err
		}
		// Withdrawing a backend from every workspace is a platform decision,
		// so a shared backend is unshared (by a system administrator) before
		// its owner can delete it.
		if backend.IsBuiltin {
			return apperrors.NewBadRequestError(
				"stop sharing this storage backend platform-wide before deleting it")
		}
		bound, err := s.countBindings(ctx, tx, &backend, false)
		if err != nil {
			return err
		}
		if bound.total() > 0 {
			return apperrors.NewBadRequestError("storage backend is still in use: " + bound.String())
		}
		return tx.Delete(&backend).Error
	})
}

// SetSharing publishes a storage backend to every workspace, or withdraws it.
// System administrators only.
//
// Withdrawal is refused while a workspace other than the owner still uses the
// backend (see countBindings); the owner may keep using it after un-sharing.
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
			"the deployment storage backend is read-only; it is already available to every workspace")
	}
	if existing.IsBuiltin == shared {
		return existing, nil // idempotent
	}

	if !shared {
		bound, err := s.countBindings(ctx, s.db, existing, true)
		if err != nil {
			return nil, err
		}
		if bound.total() > 0 {
			return nil, apperrors.NewBadRequestError(
				"cannot stop sharing: other workspaces still use this storage backend: " + bound.String())
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

// backendBindings counts what still depends on one storage backend.
type backendBindings struct {
	defaults, knowledgeBases, spaces, resources int64
}

func (b backendBindings) total() int64 {
	return b.defaults + b.knowledgeBases + b.spaces + b.resources
}

// String names what is still bound, for the refusal message.
func (b backendBindings) String() string {
	var parts []string
	for _, part := range []struct {
		n    int64
		what string
	}{
		{b.defaults, "workspace default(s)"},
		{b.knowledgeBases, "knowledge base(s)"},
		{b.spaces, "docs space(s)"},
		{b.resources, "stored file(s)"},
	} {
		if part.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", part.n, part.what))
		}
	}
	return strings.Join(parts, ", ")
}

// countBindings counts everything that would break if the backend went away
// or stopped taking writes: workspaces that default to it, live knowledge
// bases and docs spaces bound to it, and active resources stored on it — in
// every workspace, because a shared backend is bound across workspaces. With
// exceptOwner the owner's own bindings are left out: un-sharing withdraws the
// backend from everyone else, not from its owner.
//
// db is the handle to count with, so Delete can count inside its locking
// transaction.
func (s *StorageBackendService) countBindings(
	ctx context.Context, db *gorm.DB, backend *types.StorageBackend, exceptOwner bool,
) (backendBindings, error) {
	db = db.WithContext(ctx)
	scope := func(q *gorm.DB, tenantColumn string) *gorm.DB {
		if exceptOwner {
			return q.Where(tenantColumn+" <> ?", backend.TenantID)
		}
		return q
	}
	var out backendBindings
	for _, count := range []struct {
		query *gorm.DB
		into  *int64
	}{
		{scope(db.Model(&types.Tenant{}).Where("default_storage_backend_id = ?", backend.ID), "id"), &out.defaults},
		{
			scope(db.Model(&types.KnowledgeBase{}).Where("storage_backend_id = ?", backend.ID), "tenant_id"),
			&out.knowledgeBases,
		},
		// The docs module owns docs_spaces; the table, not its model, is
		// named here so core storage governance does not import the module.
		{scope(db.Table("docs_spaces").
			Where("storage_backend_id = ? AND deleted_at IS NULL", backend.ID), "tenant_id"), &out.spaces},
		{
			scope(db.Model(&types.StoredResource{}).
				Where("storage_backend_id = ? AND state = ?", backend.ID, types.ResourceStateActive), "tenant_id"),
			&out.resources,
		},
	} {
		if err := count.query.Count(count.into).Error; err != nil {
			return backendBindings{}, err
		}
	}
	return out, nil
}

// SetDefault makes a backend the workspace default. Any backend the workspace
// can see qualifies — its own, a platform-shared one, the deployment backend —
// as long as it is active (B6: only owned rows used to be found, although the
// UI offers shared ones).
func (s *StorageBackendService) SetDefault(ctx context.Context, tenantID uint64, id string) error {
	backend, err := s.ResolveBackend(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&types.Tenant{}).Where("id = ?", tenantID).
		Update("default_storage_backend_id", backend.ID).Error
}

// ResolveBackend returns the backend a workspace binds new content to: the
// named one, or the workspace default when id is empty. The backend must be
// visible to the workspace (its own, shared, or the deployment backend) and
// active, because a binding decides where new bytes are written.
func (s *StorageBackendService) ResolveBackend(
	ctx context.Context, tenantID uint64, id string,
) (*types.StorageBackend, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		var tenant types.Tenant
		if err := s.db.WithContext(ctx).Select("default_storage_backend_id").
			Where("id = ?", tenantID).Take(&tenant).Error; err != nil {
			return nil, err
		}
		id = tenant.DefaultStorageBackendID
	}
	backend, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, apperrors.NewNotFoundError("storage backend not found")
	}
	if backend.Status != types.StorageBackendStatusActive {
		return nil, apperrors.NewBadRequestError("storage backend is not active")
	}
	return backend, nil
}

// Test checks that a backend's configuration reaches working storage. The
// deployment backend is tested with the credentials the environment holds,
// since its row never stores any.
func (s *StorageBackendService) Test(ctx context.Context, backend *types.StorageBackend) error {
	if backend.Source == types.StorageBackendSourceEnv {
		env := *backend
		env.Config.AccessKeyID, env.Config.SecretAccessKey = types.EnvStorageCredentials()
		backend = &env
	} else if err := backend.Validate(); err != nil {
		return err
	}
	if err := validateStorageBackendEndpoint(backend); err != nil {
		return err
	}
	c := backend.Config
	switch backend.Provider {
	case types.StorageProviderLocal:
		// A new local backend's directory is created here, so the
		// connectivity check below tests the directory it will really use.
		dir, err := filesvc.LocalBackendDir(c.PathPrefix)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create local storage directory: %w", err)
		}
		driver, err := filesvc.NewDriver(backend)
		if err != nil {
			return err
		}
		return driver.CheckConnectivity(ctx)
	case types.StorageProviderS3:
		return filesvc.CheckS3Connectivity(ctx, filesvc.S3Options{
			Endpoint: c.Endpoint, Region: c.Region, AccessKey: c.AccessKeyID, SecretKey: c.SecretAccessKey,
			BucketName: c.BucketName, UseSSL: c.UseSSL, AddressingStyle: c.AddressingStyle,
		})
	default:
		return fmt.Errorf("unsupported storage provider: %s", backend.Provider)
	}
}

// errEnvBackendReadOnly refuses any API change to the deployment backend: it
// is whatever the environment says, and the startup sync rewrites it.
func errEnvBackendReadOnly() error {
	return apperrors.NewBadRequestError(
		"the deployment storage backend is configured by the environment and is read-only")
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

var _ interfaces.StorageBackendService = (*StorageBackendService)(nil)
