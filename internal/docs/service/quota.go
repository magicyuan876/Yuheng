package service

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// Space storage quotas.
//
// Yuheng already meters storage per workspace (tenant.StorageQuota, checked
// on every upload since T1.6). This adds the level below it, because a
// workspace quota answers "is the installation safe" and not "is one team
// filling the disk". Both apply: an upload has to fit under each.
//
// ---- decision: the quota lives in a column, not in space settings
//
// A space's settings blob is writable by a space administrator. A quota that
// the person it constrains can raise is not a quota, so quota_bytes is its
// own column and only a workspace administrator may set it. Space
// administrators can read it — being told why an upload was refused is not a
// privilege — but not change it.
//
// Zero means unlimited, as everywhere else in Yuheng. A deployment-wide
// default fills in for spaces nobody has set one on, so an installation can
// be careful by default without anybody visiting every space.

// SpaceUsage is what a space holds and what it may hold.
type SpaceUsage struct {
	SpaceID string `json:"space_id"`
	// UsedBytes is the live attachment bytes in this space.
	UsedBytes int64 `json:"used_bytes"`
	// QuotaBytes is the effective limit: the space's own, or the
	// deployment default where the space has none. 0 means unlimited.
	QuotaBytes int64 `json:"quota_bytes"`
	// FromDefault is true when the limit came from the deployment rather
	// than from this space, which is what an administrator needs to know
	// before changing it.
	FromDefault bool `json:"from_default"`
	// CanManage is whether the caller may change the quota.
	CanManage bool `json:"can_manage"`
}

// Remaining reports the bytes still available, or -1 for unlimited.
func (u SpaceUsage) Remaining() int64 {
	if u.QuotaBytes <= 0 {
		return -1
	}
	if u.UsedBytes >= u.QuotaBytes {
		return 0
	}
	return u.QuotaBytes - u.UsedBytes
}

// effectiveQuota is a space's limit, falling back to the deployment default.
func effectiveQuota(space *model.Space, deploymentDefault int64) (limit int64, fromDefault bool) {
	if space != nil && space.QuotaBytes > 0 {
		return space.QuotaBytes, false
	}
	if deploymentDefault > 0 {
		return deploymentDefault, true
	}
	return 0, false
}

// SpaceUsage reports what a space holds and what it may hold.
//
// Any reader of the space may ask. A member who cannot see why their upload
// was refused will try again, and then ask somebody — which costs more than
// telling them.
func (s *SpaceService) SpaceUsage(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole,
) (*SpaceUsage, error) {
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	used, err := s.spaceBytes(ctx, space)
	if err != nil {
		return nil, err
	}
	limit, fromDefault := effectiveQuota(space, s.d.DefaultSpaceQuotaBytes)
	return &SpaceUsage{
		SpaceID: space.ID, UsedBytes: used, QuotaBytes: limit,
		FromDefault: fromDefault, CanManage: actor.IsTenantAdmin(),
	}, nil
}

// SetSpaceQuota changes a space's limit. Workspace administrators only.
//
// Setting a quota below what the space already holds is allowed and does not
// delete anything: it stops the space growing, which is what somebody
// reacting to a space that has got out of hand actually wants. Deleting data
// is never a side effect of changing a number.
func (s *SpaceService) SetSpaceQuota(ctx context.Context, actor *acl.Identity, space *model.Space,
	bytes int64,
) (*SpaceUsage, error) {
	if !actor.IsTenantAdmin() {
		return nil, forbidden("changing a space's quota needs a workspace administrator")
	}
	if bytes < 0 {
		return nil, invalid("a quota may not be negative; use 0 for unlimited")
	}
	if err := s.d.Repos.Spaces.SetQuota(ctx, space.TenantID, space.ID, bytes); err != nil {
		return nil, err
	}
	space.QuotaBytes = bytes
	return s.SpaceUsage(ctx, actor, space, model.RoleAdmin)
}

// spaceBytes totals a space's live attachments.
func (s *SpaceService) spaceBytes(ctx context.Context, space *model.Space) (int64, error) {
	if s.d.Repos.Files == nil {
		return 0, nil
	}
	return s.d.Repos.Files.SumBytesBySpace(ctx, space.TenantID, space.ID)
}

// checkSpaceQuota refuses an upload that would take a space over its limit.
//
// Called from the attachment service alongside the workspace check, not
// instead of it: the two answer different questions and an upload must fit
// under both.
func (b *base) checkSpaceQuota(ctx context.Context, space *model.Space, size int64) error {
	if space == nil || size <= 0 || b.d.Repos.Files == nil {
		return nil
	}
	limit, _ := effectiveQuota(space, b.d.DefaultSpaceQuotaBytes)
	if limit <= 0 {
		return nil
	}
	used, err := b.d.Repos.Files.SumBytesBySpace(ctx, space.TenantID, space.ID)
	if err != nil {
		// Not being able to prove there is room is not permission to use it:
		// a quota that fails open is one that does nothing on the day the
		// database is slow, which is exactly the day it matters.
		return err
	}
	if used+size > limit {
		return conflict("this space's storage quota of %d bytes would be exceeded "+
			"(%d bytes in use)", limit, used)
	}
	return nil
}
