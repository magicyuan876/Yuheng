package interfaces

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/types"
	"gorm.io/gorm"
)

// TenantGroupRepository persists workspace groups and their members.
//
// Every method takes the tenant id and pins its statements to it, so a
// group cannot be reached from another workspace by guessing its id. Soft
// deletion is explicit (deleted_at) rather than GORM's automatic scope:
// a deleted group's name is free again, and its membership rows stay until
// the group is purged but no longer count anywhere.
//
// Missing rows are reported as repository.ErrTenantGroupNotFound and a name
// another live group holds as repository.ErrTenantGroupNameTaken
// (internal/application/repository), which the service maps to HTTP.
type TenantGroupRepository interface {
	// Create inserts the group and its initial members in one transaction.
	// An empty ID is assigned; an empty Source becomes manual.
	Create(ctx context.Context, g *types.TenantGroup, memberIDs []string, addedBy string) error
	Get(ctx context.Context, tenantID uint64, id string) (*types.TenantGroup, error)
	// List returns the workspace's live groups, the default group first,
	// then by name.
	List(ctx context.Context, tenantID uint64) ([]*types.TenantGroup, error)
	// Update writes the given columns; only name, description, source and
	// external_id may be named.
	Update(ctx context.Context, tenantID uint64, id string, fields map[string]any) error
	// SoftDelete hides a non-default group. inTx, when not nil, runs inside
	// the same transaction before the row is hidden; it is how the modules
	// that hold grants keyed by the group id (see TenantGroupDependent)
	// remove them with the group or not at all, and it receives the
	// transaction because those grants live in tables this repository
	// knows nothing about.
	SoftDelete(ctx context.Context, tenantID uint64, id string, inTx func(tx *gorm.DB) error) error
	// EnsureDefault returns the workspace's default ("everyone") group,
	// creating it on first use. Membership of the default group is
	// implicit: every active member belongs to it, so it never has rows in
	// tenant_group_members.
	EnsureDefault(ctx context.Context, tenantID uint64, creatorID string) (*types.TenantGroup, error)
	// AddMembers adds users to a group; users already in it are left alone.
	AddMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string, addedBy string) error
	RemoveMembers(ctx context.Context, tenantID uint64, groupID string, userIDs []string) error
	// ListMemberIDs returns the explicit members of a group, oldest first.
	ListMemberIDs(ctx context.Context, tenantID uint64, groupID string) ([]string, error)
	// GroupIDsForUser returns the live groups a user explicitly belongs to.
	// The default group is not among them (see EnsureDefault).
	GroupIDsForUser(ctx context.Context, tenantID uint64, userID string) ([]string, error)
}

// TenantGroupService manages a workspace's groups.
//
// The caller's identity comes from the request context, as the auth
// middleware left it: the user id attributes creations and audit rows, the
// role is recorded with them. Whether the caller may act at all is the
// router's decision (workspace Admin and above change groups, every member
// may read them), not this service's.
type TenantGroupService interface {
	// List returns the workspace's groups, the default group first, each
	// with its member count. It creates the default group when the
	// workspace has none yet, so there is no separate bootstrap step.
	List(ctx context.Context, tenantID uint64) ([]*types.TenantGroupView, error)
	Get(ctx context.Context, tenantID uint64, id string) (*types.TenantGroupView, error)
	Create(ctx context.Context, tenantID uint64, in types.CreateTenantGroupInput) (*types.TenantGroupView, error)
	// Update renames or re-describes a group. The default group keeps its
	// name.
	Update(ctx context.Context, tenantID uint64, id string,
		in types.UpdateTenantGroupInput) (*types.TenantGroupView, error)
	// Delete removes a group together with every grant the registered
	// dependents hold for it, in one transaction. The default group cannot
	// be deleted.
	Delete(ctx context.Context, tenantID uint64, id string) error
	// ListMembers pages through a group's members, optionally filtered by
	// a case-insensitive match on username or email. The default group
	// lists the workspace's active members, since its membership is
	// implicit.
	ListMembers(ctx context.Context, tenantID uint64, id, query string,
		page, pageSize int) (*types.TenantGroupMemberPage, error)
	// AddMembers adds active workspace members to a group; users already
	// in it are skipped. The default group does not take members.
	AddMembers(ctx context.Context, tenantID uint64, id string, userIDs []string) error
	RemoveMember(ctx context.Context, tenantID uint64, id, userID string) error
	// RegisterDependent enrols a module that holds grants keyed by group id.
	// Called once per module at start-up, before any request is served.
	RegisterDependent(d TenantGroupDependent)
}

// TenantGroupDependent is a module that grants permissions to groups and so
// has state keyed by group id -- the docs module's space memberships and
// page grants, for one. The group service tells it when a group goes away
// and when one changes, so that no grant outlives its group and no cached
// permission decision outlives a membership change.
type TenantGroupDependent interface {
	// RemoveGroupGrants deletes every grant the module holds for the group.
	// It runs inside the transaction that deletes the group and must use tx
	// for its writes, so the grants leave with the group or not at all.
	RemoveGroupGrants(ctx context.Context, tx *gorm.DB, tenantID uint64, groupID string) error
	// GroupChanged runs after a change to a group or its membership has
	// committed, deletion included. Cached decisions about who may do what
	// are stale from here on.
	GroupChanged(ctx context.Context, change types.TenantGroupChange)
}
