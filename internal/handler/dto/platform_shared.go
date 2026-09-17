package dto

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/types"
)

// CanSeeSharedInfraDetail reports whether the caller may see the configuration
// detail (endpoints, proxies, extra config) of a PLATFORM-SHARED infrastructure
// row — one carrying is_builtin = true.
//
// Shared rows are visible to every workspace so people can select them at
// point of use, but their configuration is not the viewing workspace's to
// know: a base URL or bucket endpoint names internal infrastructure, and in a
// multi-tenant deployment it is the platform operator's, not the tenant's.
// Only system administrators (and cross-tenant superusers, who administer
// workspaces they are not members of) see it.
//
// Credentials are never returned for any row by any caller — the response DTOs
// omit those fields by construction, and presence is reported through the
// separate /credentials subresource. This function governs the non-secret but
// still operator-owned fields.
//
// Deliberately NOT gated on the caller's tenant role: a workspace Admin is
// Admin of their own workspace, which says nothing about whether they should
// see how the platform configured a shared endpoint. Compare
// CanViewIntegrationSecrets, which is the right check for the workspace's own
// rows.
func CanSeeSharedInfraDetail(ctx context.Context) bool {
	if types.IsSystemAdminFromContext(ctx) {
		return true
	}
	user, ok := ctx.Value(types.UserContextKey).(*types.User)
	return ok && user != nil && user.CanAccessAllTenants
}
