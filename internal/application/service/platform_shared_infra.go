package service

import (
	"context"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Shared helpers for infrastructure rows that can be marked is_builtin and
// become visible to every workspace: models, MCP services, web-search
// providers, vector stores and storage backends.
//
// The shape is always the same, and getting any part of it wrong is a
// cross-tenant bug rather than a cosmetic one:
//
//   - READS are widened at the repository layer to
//     "(tenant_id = ? OR is_builtin = true)" so a workspace can select a
//     platform-provided instance when building a knowledge base or agent.
//   - WRITES must stay keyed on the OWNING tenant, not the caller's. Every
//     repository update/delete is keyed on (id, tenant_id); a shared row
//     belongs to whichever workspace created it, which is rarely the admin's
//     current one, so passing the caller's tenant matches no row and reports
//     success having changed nothing.
//   - Only system administrators may write a shared row. A workspace admin
//     can see it and use it, never repoint it — otherwise one workspace could
//     redirect an endpoint every other workspace depends on.

// authorizePlatformSharedWrite decides whether the caller may mutate a row and
// which tenant the write must be keyed on.
//
// For an ordinary workspace row it returns the caller's own tenant, leaving
// existing behaviour untouched. For a platform-shared row it demands system
// administrator and returns the owning tenant so the repository predicate
// actually matches.
// ownerTenantID is the tenant_id of the STORED row, which the caller has
// already loaded. For an ordinary row that is by construction the caller's own
// tenant (the read that found it matched on tenant_id), so returning it needs
// no context lookup — and notably does not call MustTenantIDFromContext, which
// panics on a tenantless context.
func authorizePlatformSharedWrite(
	ctx context.Context, resource string, isBuiltin bool, ownerTenantID uint64,
) (uint64, error) {
	if !isBuiltin {
		return ownerTenantID, nil
	}
	if !types.IsSystemAdminFromContext(ctx) {
		logger.Warnf(ctx, "Non-system-admin attempted to modify shared %s owned by tenant %d",
			resource, ownerTenantID)
		return 0, apperrors.NewForbiddenError(
			"only system administrators can modify platform-shared " + resource)
	}
	return ownerTenantID, nil
}

// authorizePlatformSharingChange gates the sharing toggle itself.
//
// Sharing exposes a row to every workspace on the deployment, so it is a
// platform decision. It cannot be expressed as a tenant-role threshold: every
// self-registered user is Owner of their own personal workspace, so even
// TenantRoleOwner is trivially satisfied there.
func authorizePlatformSharingChange(ctx context.Context, resource string) error {
	if types.IsSystemAdminFromContext(ctx) {
		return nil
	}
	logger.Warnf(ctx, "Non-system-admin attempted to change %s sharing", resource)
	return apperrors.NewForbiddenError(
		"only system administrators can change " + resource + " sharing")
}
