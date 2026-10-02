package docs

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"gorm.io/gorm"
)

// groupBridge is the module's side of workspace groups
// (interfaces.TenantGroupDependent). The groups themselves are managed by
// the application layer; this module grants space membership and page access
// to them, and expands a caller into its groups when deciding permissions.
// Both of those are what the bridge keeps right when a group changes.
type groupBridge struct {
	repos    *repository.Repositories
	resolver *acl.Resolver
	bus      events.Bus
}

func newGroupBridge(repos *repository.Repositories, resolver *acl.Resolver, bus events.Bus) *groupBridge {
	return &groupBridge{repos: repos, resolver: resolver, bus: bus}
}

// RemoveGroupGrants drops the group from every space it is a member of and
// from every restricted page it was granted. It runs inside the transaction
// that deletes the group, over that transaction's handle, so nobody keeps
// access through a group that no longer exists -- the rows go with the
// group or not at all.
func (b *groupBridge) RemoveGroupGrants(ctx context.Context, tx *gorm.DB, tenantID uint64, groupID string) error {
	repos := repository.New(tx)
	principal := model.GroupPrincipal(groupID)
	if err := repos.Members.RemoveAllForPrincipal(ctx, tenantID, principal); err != nil {
		return err
	}
	return repos.Access.RemoveAllGrantsForPrincipal(ctx, tenantID, principal)
}

// GroupChanged drops the tenant's cached permission decisions and publishes
// the module's own event, which the live-update stream relays and which
// drops the same cache on every other instance (see the subscription in
// NewModule). Cached identities list the groups a user is in, so any group
// change -- a rename included, since the panel shows names -- is a reason.
func (b *groupBridge) GroupChanged(ctx context.Context, change types.TenantGroupChange) {
	b.resolver.Invalidate(ctx, change.TenantID)
	if b.bus == nil {
		return
	}
	e := events.New(events.GroupChanged, change.TenantID).WithActor(change.ActorUserID).
		With("group_id", change.GroupID).With("action", string(change.Action))
	if err := b.bus.Publish(ctx, e); err != nil {
		// The change has committed and the cache is already dropped here;
		// a failing bus is worth a line, not a failed request.
		logger.Warnf(ctx, "[docs] publish %s failed: %v", e.Type, err)
	}
}

// Compile-time check that the bridge satisfies the core's contract.
var _ interfaces.TenantGroupDependent = (*groupBridge)(nil)
