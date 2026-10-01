package model

import "time"

// Space is a document container inside a tenant (workspace).
type Space struct {
	ID              string          `json:"id"                 gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64          `json:"tenant_id"          gorm:"not null;index"`
	Slug            string          `json:"slug"               gorm:"type:varchar(64);not null"`
	Name            string          `json:"name"               gorm:"type:varchar(255);not null"`
	Description     string          `json:"description"        gorm:"type:text;not null;default:''"`
	Icon            *string         `json:"icon,omitempty"     gorm:"type:varchar(64)"`
	Visibility      SpaceVisibility `json:"visibility"         gorm:"type:varchar(16);not null;default:'private'"`
	DefaultRole     SpaceRole       `json:"default_role"       gorm:"type:varchar(16);not null;default:'none'"`
	KnowledgeBaseID *string         `json:"knowledge_base_id,omitempty"  gorm:"type:varchar(36)"`
	// StorageBackendID is where the space's new attachments, imports and
	// exports go. Required; existing files are read through their own
	// resource rows, so rebinding never strands one.
	StorageBackendID string `json:"storage_backend_id" gorm:"type:varchar(36);not null"`
	Settings         JSON   `json:"settings"           gorm:"type:json;not null;default:'{}'"`
	// QuotaBytes caps the attachment bytes this space may hold; 0 is
	// unlimited, as everywhere else in Yuheng. A column rather than a key in
	// Settings, because Settings is writable by a space administrator and a
	// quota somebody can raise for themselves is not a quota.
	QuotaBytes int64      `json:"quota_bytes" gorm:"not null;default:0"`
	CreatorID  *string    `json:"creator_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt  time.Time  `json:"created_at"         gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at"         gorm:"autoUpdateTime"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (Space) TableName() string { return "docs_spaces" }

// SpaceMember grants a role in a space to a user or a group.
type SpaceMember struct {
	ID            string        `json:"id"             gorm:"type:varchar(36);primaryKey"`
	SpaceID       string        `json:"space_id"       gorm:"type:varchar(36);not null"`
	TenantID      uint64        `json:"tenant_id"      gorm:"not null"`
	PrincipalType PrincipalType `json:"principal_type" gorm:"type:varchar(8);not null"`
	PrincipalID   string        `json:"principal_id"   gorm:"type:varchar(36);not null"`
	Role          SpaceRole     `json:"role"           gorm:"type:varchar(16);not null"`
	AddedBy       *string       `json:"added_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt     time.Time     `json:"created_at"     gorm:"autoCreateTime"`
	UpdatedAt     time.Time     `json:"updated_at"     gorm:"autoUpdateTime"`
}

// TableName pins the table name.
func (SpaceMember) TableName() string { return "docs_space_members" }

// Principal returns the member's principal.
func (m SpaceMember) Principal() Principal {
	return Principal{Type: m.PrincipalType, ID: m.PrincipalID}
}

// TenantGroup is a tenant-level user group.
type TenantGroup struct {
	ID          string      `json:"id"           gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64      `json:"tenant_id"    gorm:"not null;index"`
	Name        string      `json:"name"         gorm:"type:varchar(128);not null"`
	Description string      `json:"description"  gorm:"type:text;not null;default:''"`
	IsDefault   bool        `json:"is_default"   gorm:"not null;default:false"`
	Source      GroupSource `json:"source"       gorm:"type:varchar(16);not null;default:'manual'"`
	ExternalID  *string     `json:"external_id,omitempty" gorm:"type:varchar(255)"`
	CreatorID   *string     `json:"creator_id,omitempty"  gorm:"type:varchar(36)"`
	CreatedAt   time.Time   `json:"created_at"   gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updated_at"   gorm:"autoUpdateTime"`
	DeletedAt   *time.Time  `json:"deleted_at,omitempty"`
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
