package model

import "time"

// Page is a node in a space's page tree. Content is the ProseMirror JSON
// projection; YDoc is the Yjs state and the collaboration truth source.
type Page struct {
	ID          string  `json:"id"                gorm:"type:varchar(36);primaryKey"`
	ShortID     string  `json:"short_id"          gorm:"type:varchar(12);not null"`
	TenantID    uint64  `json:"tenant_id"         gorm:"not null"`
	SpaceID     string  `json:"space_id"          gorm:"type:varchar(36);not null"`
	ParentID    *string `json:"parent_id"         gorm:"type:varchar(36)"`
	Position    string  `json:"position"          gorm:"type:varchar(64);not null;default:''"`
	Title       string  `json:"title"             gorm:"type:varchar(512);not null;default:''"`
	Icon        *string `json:"icon,omitempty"    gorm:"type:varchar(64)"`
	Cover       *string `json:"cover,omitempty"   gorm:"type:varchar(1024)"`
	Content     JSON    `json:"content,omitempty" gorm:"type:json"`
	YDoc        []byte  `json:"-"                 gorm:"column:ydoc;type:bytes"`
	YDocVersion int64   `json:"ydoc_version"      gorm:"column:ydoc_version;not null;default:0"`
	TextContent string  `json:"-"                 gorm:"type:text;not null;default:''"`
	// ExcludeFromKnowledge keeps the page out of its space's knowledge base, and so
	// out of AI answers. It is not a permission: everybody who may read the page
	// still can.
	ExcludeFromKnowledge bool `json:"exclude_from_knowledge" gorm:"not null;default:false"`
	// SupersededBy is set while the page is excluded because another
	// document superseded it, and cleared when it is let back in.
	SupersededBy *SupersededBy `json:"superseded_by,omitempty" gorm:"type:jsonb"`
	IsLocked     bool          `json:"is_locked"         gorm:"not null;default:false"`
	TemplateID   *string       `json:"template_id,omitempty" gorm:"type:varchar(36)"`
	SourceRefs   StringList    `json:"source_refs"       gorm:"type:json;not null;default:'[]'"`
	// KnowledgeID is the knowledge-base entry mirroring this page, when its
	// space is bound to one and the page is eligible. nil means not indexed.
	KnowledgeID    *string    `json:"knowledge_id,omitempty" gorm:"type:varchar(36)"`
	ContributorIDs StringList `json:"contributor_ids"   gorm:"type:json;not null;default:'[]'"`
	CreatorID      *string    `json:"creator_id,omitempty"     gorm:"type:varchar(36)"`
	// OwnerID is the page's maintainer when somebody handed it over; nil
	// means its creator. See Steward.
	OwnerID          *string    `json:"owner_id,omitempty"       gorm:"type:varchar(36)"`
	LastEditorID     *string    `json:"last_editor_id,omitempty" gorm:"type:varchar(36)"`
	DeletedBy        *string    `json:"deleted_by,omitempty"     gorm:"type:varchar(36)"`
	WordCount        int        `json:"word_count"        gorm:"not null;default:0"`
	AttachmentBytes  int64      `json:"attachment_bytes"  gorm:"not null;default:0"`
	CreatedAt        time.Time  `json:"created_at"        gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at"        gorm:"autoUpdateTime"`
	ContentUpdatedAt *time.Time `json:"content_updated_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// TableName pins the table name.
func (Page) TableName() string { return "docs_pages" }

// Steward is the page's maintainer: the person it was handed to, else its
// creator. Empty for a page with neither (an import by a background job).
// Knowledge health takes the problems of the page's mirror entry to them.
func (p *Page) Steward() string {
	if p.OwnerID != nil && *p.OwnerID != "" {
		return *p.OwnerID
	}
	if p.CreatorID != nil {
		return *p.CreatorID
	}
	return ""
}

// IsDeleted reports whether the page is in the trash.
func (p *Page) IsDeleted() bool { return p.DeletedAt != nil }

// PageSummaryColumns are the columns loaded for tree and list views; the two
// large columns (content, ydoc) are only loaded when a page is opened.
var PageSummaryColumns = []string{
	"id", "short_id", "tenant_id", "space_id", "parent_id", "position", "title", "icon", "cover",
	"ydoc_version", "exclude_from_knowledge", "superseded_by", "is_locked", "template_id", "source_refs",
	"contributor_ids",
	"creator_id", "owner_id", "last_editor_id", "deleted_by", "word_count", "attachment_bytes",
	"created_at", "updated_at", "content_updated_at", "deleted_at",
}

// PageRevision is a content snapshot used for history, diff and restore.
type PageRevision struct {
	ID          string         `json:"id"           gorm:"type:varchar(36);primaryKey"`
	PageID      string         `json:"page_id"      gorm:"type:varchar(36);not null"`
	TenantID    uint64         `json:"tenant_id"    gorm:"not null"`
	SpaceID     string         `json:"space_id"     gorm:"type:varchar(36);not null"`
	Version     int            `json:"version"      gorm:"not null"`
	Title       string         `json:"title"        gorm:"type:varchar(512);not null;default:''"`
	Icon        *string        `json:"icon,omitempty" gorm:"type:varchar(64)"`
	Content     JSON           `json:"content"      gorm:"type:json;not null"`
	TextContent string         `json:"-"            gorm:"type:text;not null;default:''"`
	EditorIDs   StringList     `json:"editor_ids"   gorm:"type:json;not null;default:'[]'"`
	Reason      RevisionReason `json:"reason"       gorm:"type:varchar(16);not null;default:'interval'"`
	CreatedBy   *string        `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt   time.Time      `json:"created_at"   gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (PageRevision) TableName() string { return "docs_page_revisions" }

// PageAccess marks a page as cutting permission inheritance. Its absence
// means the page inherits from its space (or its nearest restricted
// ancestor).
type PageAccess struct {
	PageID    string    `json:"page_id"    gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id"  gorm:"not null"`
	SpaceID   string    `json:"space_id"   gorm:"type:varchar(36);not null"`
	Mode      string    `json:"mode"       gorm:"type:varchar(16);not null;default:'restricted'"`
	CreatedBy *string   `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName pins the table name.
func (PageAccess) TableName() string { return "docs_page_access" }

// PageGrant is an explicit role on a restricted page.
type PageGrant struct {
	ID            string        `json:"id"             gorm:"type:varchar(36);primaryKey"`
	PageID        string        `json:"page_id"        gorm:"type:varchar(36);not null"`
	TenantID      uint64        `json:"tenant_id"      gorm:"not null"`
	PrincipalType PrincipalType `json:"principal_type" gorm:"type:varchar(8);not null"`
	PrincipalID   string        `json:"principal_id"   gorm:"type:varchar(36);not null"`
	Role          SpaceRole     `json:"role"           gorm:"type:varchar(16);not null"`
	AddedBy       *string       `json:"added_by,omitempty" gorm:"type:varchar(36)"`
	CreatedAt     time.Time     `json:"created_at"     gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (PageGrant) TableName() string { return "docs_page_grants" }

// Principal returns the grant's principal.
func (g PageGrant) Principal() Principal { return Principal{Type: g.PrincipalType, ID: g.PrincipalID} }

// Link records that one page references another (for backlinks).
type Link struct {
	TenantID     uint64    `json:"tenant_id"      gorm:"not null"`
	SourcePageID string    `json:"source_page_id" gorm:"type:varchar(36);primaryKey"`
	TargetPageID string    `json:"target_page_id" gorm:"type:varchar(36);primaryKey"`
	Kind         LinkKind  `json:"kind"           gorm:"type:varchar(16);primaryKey;default:'link'"`
	CreatedAt    time.Time `json:"created_at"     gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (Link) TableName() string { return "docs_links" }

// TransclusionBlock caches the latest content of a block that other pages
// embed by reference.
type TransclusionBlock struct {
	ID          string    `json:"id"           gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64    `json:"tenant_id"    gorm:"not null"`
	PageID      string    `json:"page_id"      gorm:"type:varchar(36);not null"`
	BlockID     string    `json:"block_id"     gorm:"type:varchar(40);not null"`
	Content     JSON      `json:"content"      gorm:"type:json;not null"`
	TextContent string    `json:"-"            gorm:"type:text;not null;default:''"`
	UpdatedAt   time.Time `json:"updated_at"   gorm:"autoUpdateTime"`
}

// TableName pins the table name.
func (TransclusionBlock) TableName() string { return "docs_transclusion_blocks" }

// EditLease is the exclusive-edit lock used when no collaboration service is
// configured.
type EditLease struct {
	PageID    string    `json:"page_id"    gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64    `json:"tenant_id"  gorm:"not null"`
	UserID    string    `json:"user_id"    gorm:"type:varchar(36);not null"`
	SessionID string    `json:"session_id" gorm:"type:varchar(64);not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName pins the table name.
func (EditLease) TableName() string { return "docs_edit_leases" }

// IndexState is the bookkeeping for mirroring one page into its space's
// knowledge base: what is pending, who is working on it, and what was last sent.
type IndexState struct {
	PageID   string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"not null"`
	// DueAt is when the page is next to be synchronised; nil when nothing is
	// pending.
	DueAt *time.Time
	// Seq increases with every request to synchronise the page.
	Seq          int64 `gorm:"not null;default:0"`
	ClaimedUntil *time.Time
	Attempts     int    `gorm:"not null;default:0"`
	LastError    string `gorm:"type:text;not null;default:''"`
	// IndexedHash is the hash of the title and Markdown last sent.
	IndexedHash string `gorm:"type:varchar(64);not null;default:''"`
	IndexedAt   *time.Time
}

// TableName pins the table name.
func (IndexState) TableName() string { return "docs_index_state" }
