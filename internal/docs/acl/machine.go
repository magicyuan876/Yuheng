package acl

import (
	"context"
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Machine identities.
//
// An API key is not a person. The rest of the platform keeps it apart from
// user accounts (a tenant key owns its chat sessions as
// "api_tenant_key:<tenant>:<key>"), and so does this module: a key acts under
// its own identity, never under whichever account happens to be the
// workspace's oldest. That keeps its reach independent of that person's
// memberships and grants, and its writes and audit rows attributable to the
// key that made them.
//
// What a key may see is what the workspace gives every member, bounded by
// what the key may do:
//
//   - It holds only the implicit "everyone" group, so it reads open and
//     public spaces, and spaces or pages granted to everyone. Private spaces
//     and restricted pages granted to named people stay out of reach: nobody
//     can grant anything to a key.
//   - Its capabilities cap its role like a tenant role would: docs_read makes
//     it a Viewer (reader at most), docs_write a Contributor, and docs_admin
//     or full access an Admin, which is admin in every space.
//   - A key limited to some knowledge bases sees only the spaces bound to one
//     of them, whatever its role.
//
// External users resolved through a key (the direct and signed-token API
// principal modes) get the same identity shape under their own id, so two
// external users of one key do not share watchers or authorship.

// machineIDPrefixes are the id shapes MachineIdentity produces.
var machineIDPrefixes = []string{
	types.SessionOwnerAPITenantKeyPrefix,
	types.SessionOwnerAPIExternalUserPrefix,
	types.PrincipalAPIPlatform + ":",
}

// IsMachineUserID reports whether an actor id belongs to an API key rather
// than a user account. Features that only make sense for people (watching a
// page, being notified) skip machine ids.
func IsMachineUserID(id string) bool {
	for _, prefix := range machineIDPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// machineUserID names the caller: the external user when the request
// resolved one, otherwise the key itself.
func machineUserID(tenantID uint64, scope types.TenantAPIKeyScope, principal types.Principal) (string, error) {
	switch principal.Type {
	case types.PrincipalAPIExternalUser:
		return principal.StorageID(), nil
	case types.PrincipalAPIPlatform:
		return principal.StorageID(), nil
	}
	if scope.KeyID == 0 {
		return "", fmt.Errorf("acl: API key scope without a key id")
	}
	return fmt.Sprintf("%s%d:%d", types.SessionOwnerAPITenantKeyPrefix, tenantID, scope.KeyID), nil
}

// machineTenantRole maps a key's authority onto the tenant-role ladder the
// resolver already understands.
func machineTenantRole(scope types.TenantAPIKeyScope) types.TenantRole {
	switch {
	case scope.FullAccess || scope.HasCapability(types.APIKeyCapabilityDocsAdmin):
		return types.TenantRoleAdmin
	case scope.HasCapability(types.APIKeyCapabilityDocsWrite):
		return types.TenantRoleContributor
	default:
		return types.TenantRoleViewer
	}
}

// MachineIdentity builds the identity an API-key request acts under. It is
// derived from the key on every request rather than cached: it costs one
// lookup of the default group, and a key's scope may change at any time.
func (r *Resolver) MachineIdentity(ctx context.Context, tenantID uint64, scope types.TenantAPIKeyScope,
	principal types.Principal,
) (*Identity, error) {
	scope = scope.Normalize()
	userID, err := machineUserID(tenantID, scope, principal)
	if err != nil {
		return nil, err
	}
	id := &Identity{
		TenantID: tenantID, UserID: userID, TenantRole: machineTenantRole(scope), Member: true, Machine: true,
	}
	if len(scope.KnowledgeBaseIDs) > 0 {
		id.KnowledgeBases = make(map[string]bool, len(scope.KnowledgeBaseIDs))
		for _, kb := range scope.KnowledgeBaseIDs {
			id.KnowledgeBases[kb] = true
		}
	}
	def, err := r.defaultGroup(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if def != "" {
		id.DefaultGroupID = def
		id.Principals = []model.Principal{model.GroupPrincipal(def)}
	}
	return id, nil
}

// reaches reports whether a knowledge-base limited identity may see a space
// at all: only spaces bound to one of its knowledge bases.
func (id *Identity) reaches(space *model.Space) bool {
	if id.KnowledgeBases == nil {
		return true
	}
	return space.KnowledgeBaseID != nil && id.KnowledgeBases[*space.KnowledgeBaseID]
}
