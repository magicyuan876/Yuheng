package types

import "time"

// TenantGroup is a workspace group: a named set of the workspace's members
// that permissions can be granted to as one principal.
//
// Groups are a workspace concept, not a feature of any one module. The docs
// module was their first consumer (space membership and page grants name a
// group as `group:<id>`), the ACL work builds on them, and Source is where
// an SSO group mapping will hang its OIDC and LDAP groups. The table and
// type names predate the lift out of the docs module and are kept, because
// the ACL subject `group:<id>` and the stored rows are part of the contract.
type TenantGroup struct {
	ID          string `json:"id"           gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64 `json:"tenant_id"    gorm:"not null;index"`
	Name        string `json:"name"         gorm:"type:varchar(128);not null"`
	Description string `json:"description"  gorm:"type:text;not null;default:''"`
	// IsDefault marks the workspace's implicit "everyone" group. Its
	// membership is every active member and is never written to
	// tenant_group_members, so it cannot drift from the member list.
	IsDefault  bool              `json:"is_default"   gorm:"not null;default:false"`
	Source     TenantGroupSource `json:"source"       gorm:"type:varchar(16);not null;default:'manual'"`
	ExternalID *string           `json:"external_id,omitempty" gorm:"type:varchar(255)"`
	CreatorID  *string           `json:"creator_id,omitempty"  gorm:"type:varchar(36)"`
	CreatedAt  time.Time         `json:"created_at"   gorm:"autoCreateTime"`
	UpdatedAt  time.Time         `json:"updated_at"   gorm:"autoUpdateTime"`
	DeletedAt  *time.Time        `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (TenantGroup) TableName() string { return "tenant_groups" }

// TenantGroupMember links a user to a group.
type TenantGroupMember struct {
	GroupID   string    `json:"group_id"   gorm:"type:varchar(36);primaryKey"`
	UserID    string    `json:"user_id"    gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id"  gorm:"not null"`
	AddedBy   *string   `json:"added_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (TenantGroupMember) TableName() string { return "tenant_group_members" }

// TenantGroupSource records where a group's membership is managed: by hand
// in Yuheng, or mirrored from an identity provider. Only manual groups are
// edited through the API; the other two are the hook for SSO group mapping.
type TenantGroupSource string

// Group sources.
const (
	TenantGroupSourceManual TenantGroupSource = "manual"
	TenantGroupSourceOIDC   TenantGroupSource = "oidc"
	TenantGroupSourceLDAP   TenantGroupSource = "ldap"
)

// DefaultTenantGroupName is the reserved name of the implicit "everyone"
// group. No other group may take it, in any letter case.
const DefaultTenantGroupName = "everyone"

// AuditTargetTenantGroup is the target_type of audit rows about a group.
const AuditTargetTenantGroup = "tenant_group"

// TenantGroupView is a group as the API shows it: the row plus how many
// members it has, which for the default group is the workspace's active
// member count.
type TenantGroupView struct {
	*TenantGroup
	MemberCount int64 `json:"member_count"`
}

// TenantGroupMemberView is how a group lists one of its members. The
// display fields are filled from the user directory and are empty when the
// lookup fails, so a directory outage degrades to bare ids rather than
// failing the request.
type TenantGroupMemberView struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// TenantGroupMemberPage is one page of a group's members.
type TenantGroupMemberPage struct {
	Members  []TenantGroupMemberView `json:"members"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// CreateTenantGroupInput is what a client may set when creating a group.
type CreateTenantGroupInput struct {
	Name        string
	Description string
	// MemberIDs are added in the same transaction as the group.
	MemberIDs []string
}

// UpdateTenantGroupInput is a partial update; nil leaves a field alone.
type UpdateTenantGroupInput struct {
	Name        *string
	Description *string
}

// TenantGroupChange tells a dependent module what happened to a group, after
// the change has committed.
type TenantGroupChange struct {
	TenantID    uint64
	GroupID     string
	ActorUserID string
	// Action is the audit action the change was recorded under, one of the
	// AuditActionGroup* constants.
	Action AuditAction
}
