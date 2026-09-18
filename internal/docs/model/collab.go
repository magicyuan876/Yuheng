package model

import "time"

// Comment is a page-level or inline comment thread entry.
type Comment struct {
	ID         string     `json:"id"           gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64     `json:"tenant_id"    gorm:"not null"`
	SpaceID    string     `json:"space_id"     gorm:"type:varchar(36);not null"`
	PageID     string     `json:"page_id"      gorm:"type:varchar(36);not null"`
	ParentID   *string    `json:"parent_id,omitempty" gorm:"type:varchar(36)"`
	Body       JSON       `json:"body"         gorm:"type:json;not null"`
	Anchor     JSON       `json:"anchor,omitempty" gorm:"type:json"`
	QuotedText *string    `json:"quoted_text,omitempty" gorm:"type:text"`
	CreatorID  string     `json:"creator_id"   gorm:"type:varchar(36);not null"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy *string    `json:"resolved_by,omitempty" gorm:"type:varchar(36)"`
	EditedAt   *time.Time `json:"edited_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"   gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at"   gorm:"autoUpdateTime"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (Comment) TableName() string { return "docs_comments" }

// Attachment is the metadata row for a stored file.
type Attachment struct {
	ID         string         `json:"id"          gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64         `json:"tenant_id"   gorm:"not null"`
	SpaceID    string         `json:"space_id"    gorm:"type:varchar(36);not null"`
	PageID     *string        `json:"page_id,omitempty" gorm:"type:varchar(36)"`
	FilePath   string         `json:"-"           gorm:"type:varchar(1024);not null"`
	FileName   string         `json:"file_name"   gorm:"type:varchar(512);not null"`
	FileExt    string         `json:"file_ext"    gorm:"type:varchar(32);not null;default:''"`
	Mime       string         `json:"mime"        gorm:"type:varchar(255);not null;default:'application/octet-stream'"`
	SizeBytes  int64          `json:"size_bytes"  gorm:"not null;default:0"`
	SHA256     *string        `json:"sha256,omitempty" gorm:"column:sha256;type:char(64)"`
	Width      *int           `json:"width,omitempty"`
	Height     *int           `json:"height,omitempty"`
	Kind       AttachmentKind `json:"kind"        gorm:"type:varchar(16);not null;default:'file'"`
	UploaderID *string        `json:"uploader_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt  time.Time      `json:"created_at"  gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"updated_at"  gorm:"autoUpdateTime"`
	DeletedAt  *time.Time     `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (Attachment) TableName() string { return "docs_attachments" }

// Share is a public link to a page (optionally with its subtree).
type Share struct {
	ID               string     `json:"id"                 gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64     `json:"tenant_id"          gorm:"not null"`
	SpaceID          string     `json:"space_id"           gorm:"type:varchar(36);not null"`
	PageID           string     `json:"page_id"            gorm:"type:varchar(36);not null"`
	Key              string     `json:"key"                gorm:"type:varchar(64);not null;uniqueIndex"`
	IncludeChildren  bool       `json:"include_children"   gorm:"not null;default:false"`
	AllowSearchIndex bool       `json:"allow_search_index" gorm:"not null;default:false"`
	PasswordHash     *string    `json:"-"                  gorm:"type:varchar(255)"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ViewCount        int64      `json:"view_count"         gorm:"not null;default:0"`
	CreatorID        *string    `json:"creator_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt        time.Time  `json:"created_at"         gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at"         gorm:"autoUpdateTime"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
}

// TableName pins the table name.
func (Share) TableName() string { return "docs_shares" }

// Watcher subscribes a user to a page or a whole space.
type Watcher struct {
	ID        string      `json:"id"         gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64      `json:"tenant_id"  gorm:"not null"`
	UserID    string      `json:"user_id"    gorm:"type:varchar(36);not null"`
	SpaceID   string      `json:"space_id"   gorm:"type:varchar(36);not null"`
	PageID    *string     `json:"page_id,omitempty" gorm:"type:varchar(36)"`
	Reason    WatchReason `json:"reason"     gorm:"type:varchar(16);not null;default:'manual'"`
	MutedAt   *time.Time  `json:"muted_at,omitempty"`
	CreatedBy *string     `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time   `json:"created_at" gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (Watcher) TableName() string { return "docs_watchers" }

// Notification is one in-app notification for one user.
type Notification struct {
	ID         string     `json:"id"          gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64     `json:"tenant_id"   gorm:"not null"`
	UserID     string     `json:"user_id"     gorm:"type:varchar(36);not null"`
	Kind       string     `json:"kind"        gorm:"type:varchar(32);not null"`
	ActorID    *string    `json:"actor_id,omitempty"   gorm:"type:varchar(36)"`
	SpaceID    *string    `json:"space_id,omitempty"   gorm:"type:varchar(36)"`
	PageID     *string    `json:"page_id,omitempty"    gorm:"type:varchar(36)"`
	CommentID  *string    `json:"comment_id,omitempty" gorm:"type:varchar(36)"`
	Payload    JSON       `json:"payload"     gorm:"type:json;not null;default:'{}'"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	EmailedAt  *time.Time `json:"emailed_at,omitempty"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"  gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (Notification) TableName() string { return "docs_notifications" }

// Template is a reusable page body, tenant-wide (SpaceID nil) or per space.
type Template struct {
	ID           string     `json:"id"            gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64     `json:"tenant_id"     gorm:"not null"`
	SpaceID      *string    `json:"space_id,omitempty" gorm:"type:varchar(36)"`
	Name         string     `json:"name"          gorm:"type:varchar(255);not null"`
	Description  string     `json:"description"   gorm:"type:text;not null;default:''"`
	Icon         *string    `json:"icon,omitempty" gorm:"type:varchar(64)"`
	Content      JSON       `json:"content"       gorm:"type:json;not null"`
	TextContent  string     `json:"-"             gorm:"type:text;not null;default:''"`
	Category     string     `json:"category"      gorm:"type:varchar(64);not null;default:''"`
	CreatorID    *string    `json:"creator_id,omitempty"     gorm:"type:varchar(36)"`
	LastEditorID *string    `json:"last_editor_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt    time.Time  `json:"created_at"    gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at"    gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (Template) TableName() string { return "docs_templates" }

// Label is a space-scoped tag.
type Label struct {
	ID        string    `json:"id"         gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id"  gorm:"not null"`
	SpaceID   string    `json:"space_id"   gorm:"type:varchar(36);not null"`
	Name      string    `json:"name"       gorm:"type:varchar(64);not null"`
	Color     string    `json:"color"      gorm:"type:varchar(16);not null;default:'gray'"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName pins the table name.
func (Label) TableName() string { return "docs_labels" }

// PageLabel attaches a label to a page.
type PageLabel struct {
	PageID    string    `json:"page_id"    gorm:"type:varchar(36);primaryKey"`
	LabelID   string    `json:"label_id"   gorm:"type:varchar(36);primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (PageLabel) TableName() string { return "docs_page_labels" }

// ImportJob tracks an asynchronous import into a space.
type ImportJob struct {
	ID             string          `json:"id"               gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64          `json:"tenant_id"        gorm:"not null"`
	SpaceID        string          `json:"space_id"         gorm:"type:varchar(36);not null"`
	Kind           string          `json:"kind"             gorm:"type:varchar(32);not null"`
	Status         ImportJobStatus `json:"status"           gorm:"type:varchar(16);not null;default:'pending'"`
	SourcePath     string          `json:"-"                gorm:"type:varchar(1024);not null"`
	FileName       string          `json:"file_name"        gorm:"type:varchar(512);not null;default:''"`
	TargetParentID *string         `json:"target_parent_id,omitempty" gorm:"type:varchar(36)"`
	Stats          JSON            `json:"stats"            gorm:"type:json;not null;default:'{}'"`
	Error          string          `json:"error"            gorm:"type:text;not null;default:''"`
	CreatedBy      *string         `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt      time.Time       `json:"created_at"       gorm:"autoCreateTime"`
	UpdatedAt      time.Time       `json:"updated_at"       gorm:"autoUpdateTime"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty"`
}

// TableName pins the table name.
func (ImportJob) TableName() string { return "docs_import_jobs" }
