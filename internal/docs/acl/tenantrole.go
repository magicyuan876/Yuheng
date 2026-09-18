package acl

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/types"
	"gorm.io/gorm"
)

// TenantMemberGetter is the slice of the TenantMember repository the
// resolver needs. interfaces.TenantMemberRepository satisfies it.
type TenantMemberGetter interface {
	Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)
}

// memberRoleSource adapts tenant membership rows to TenantRoleSource.
type memberRoleSource struct{ members TenantMemberGetter }

// NewTenantMemberRoleSource builds the production TenantRoleSource. Only
// ACTIVE memberships count: invited and suspended members hold no role.
func NewTenantMemberRoleSource(members TenantMemberGetter) TenantRoleSource {
	return &memberRoleSource{members: members}
}

func (s *memberRoleSource) TenantRole(ctx context.Context, tenantID uint64,
	userID string,
) (types.TenantRole, bool, error) {
	m, err := s.members.Get(ctx, userID, tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if m == nil || m.Status != types.TenantMemberStatusActive {
		return "", false, nil
	}
	return m.Role, true, nil
}

// isNotFound recognises the repository's own not-found sentinel by message,
// since the tenant member repository predates a shared error type.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "tenant member not found" || msg == "record not found"
}
