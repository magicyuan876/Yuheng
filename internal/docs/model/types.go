// Package model holds the persistence models of the docs module. Every row is
// tenant-scoped; repositories are the only writers. Field-level documentation
// lives in migrations/versioned/000120_docs_module.up.sql.
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSON is a raw JSON column, stored as JSONB. It is handed to the driver as
// text, which Postgres parses into JSONB; nil stores as SQL NULL.
type JSON json.RawMessage

// Value implements driver.Valuer.
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("model.JSON: invalid JSON")
	}
	return string(j), nil
}

// Scan implements sql.Scanner.
func (j *JSON) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append(JSON(nil), v...)
	case string:
		*j = JSON(v)
	default:
		return fmt.Errorf("model.JSON: cannot scan %T", value)
	}
	return nil
}

// MarshalJSON emits the raw document (or null).
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON stores the raw bytes.
func (j *JSON) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*j = nil
		return nil
	}
	*j = append(JSON(nil), data...)
	return nil
}

// StringList is a JSON array of strings stored in a JSON/TEXT column. A nil
// list stores as "[]" so NOT NULL columns are satisfied.
type StringList []string

// Value implements driver.Valuer.
func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (l *StringList) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("model.StringList: cannot scan %T", value)
	}
	if len(raw) == 0 {
		*l = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("model.StringList: %w", err)
	}
	*l = out
	return nil
}

// Contains reports whether the list holds s.
func (l StringList) Contains(s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// Add appends s unless already present and returns the (possibly new) list.
func (l StringList) Add(s string) StringList {
	if s == "" || l.Contains(s) {
		return l
	}
	return append(l, s)
}

// ---- enumerations ------------------------------------------------------------

// SpaceVisibility controls who can discover a space.
type SpaceVisibility string

// Space visibility values.
const (
	VisibilityPrivate SpaceVisibility = "private" // explicit members only
	VisibilityOpen    SpaceVisibility = "open"    // every tenant member gets DefaultRole
	VisibilityPublic  SpaceVisibility = "public"  // readable without login
)

// Valid reports whether v is a known visibility.
func (v SpaceVisibility) Valid() bool {
	switch v {
	case VisibilityPrivate, VisibilityOpen, VisibilityPublic:
		return true
	}
	return false
}

// SpaceRole is the role a principal holds in a space (or on a restricted page).
type SpaceRole string

// Space roles, weakest to strongest. RoleNone means "no access" and is only
// used as a resolution result / open-space default.
const (
	RoleNone   SpaceRole = "none"
	RoleReader SpaceRole = "reader"
	RoleWriter SpaceRole = "writer"
	RoleAdmin  SpaceRole = "admin"
)

var roleLevel = map[SpaceRole]int{RoleNone: 0, RoleReader: 10, RoleWriter: 20, RoleAdmin: 30}

// Level orders roles so they can be compared; unknown roles rank as none.
func (r SpaceRole) Level() int { return roleLevel[r] }

// Valid reports whether r is a grantable role (none is not grantable).
func (r SpaceRole) Valid() bool {
	return r == RoleReader || r == RoleWriter || r == RoleAdmin
}

// AtLeast reports whether r grants at least what other grants.
func (r SpaceRole) AtLeast(other SpaceRole) bool { return r.Level() >= other.Level() }

// MaxRole returns the stronger of two roles.
func MaxRole(a, b SpaceRole) SpaceRole {
	if b.Level() > a.Level() {
		return b
	}
	return a
}

// MinRole returns the weaker of two roles.
func MinRole(a, b SpaceRole) SpaceRole {
	if b.Level() < a.Level() {
		return b
	}
	return a
}

// PrincipalType distinguishes users from groups in membership tables.
type PrincipalType string

// Principal types.
const (
	PrincipalUser  PrincipalType = "user"
	PrincipalGroup PrincipalType = "group"
)

// Principal identifies a user or a group.
type Principal struct {
	Type PrincipalType
	ID   string
}

// UserPrincipal builds a user principal.
func UserPrincipal(id string) Principal { return Principal{Type: PrincipalUser, ID: id} }

// GroupPrincipal builds a group principal.
func GroupPrincipal(id string) Principal { return Principal{Type: PrincipalGroup, ID: id} }

// RevisionReason records why a snapshot was taken.
type RevisionReason string

// Revision reasons.
const (
	RevisionInterval RevisionReason = "interval"
	RevisionPublish  RevisionReason = "publish"
	RevisionRestore  RevisionReason = "restore"
	RevisionImport   RevisionReason = "import"
	RevisionManual   RevisionReason = "manual"
)

// AttachmentKind classifies an attachment for listing and rendering.
type AttachmentKind string

// Attachment kinds.
const (
	AttachmentFile    AttachmentKind = "file"
	AttachmentImage   AttachmentKind = "image"
	AttachmentVideo   AttachmentKind = "video"
	AttachmentAudio   AttachmentKind = "audio"
	AttachmentDiagram AttachmentKind = "diagram"
)

// WatchReason records how a watcher relationship was created.
type WatchReason string

// Watch reasons.
const (
	WatchManual  WatchReason = "manual"
	WatchAuthor  WatchReason = "author"
	WatchComment WatchReason = "comment"
	WatchMention WatchReason = "mention"
)

// LinkKind distinguishes plain page links from block transclusions.
type LinkKind string

// Link kinds.
const (
	LinkPage         LinkKind = "link"
	LinkTransclusion LinkKind = "transclusion"
)

// ImportJobStatus is the lifecycle of an import.
type ImportJobStatus string

// Import job statuses.
const (
	ImportPending   ImportJobStatus = "pending"
	ImportRunning   ImportJobStatus = "running"
	ImportSucceeded ImportJobStatus = "succeeded"
	ImportPartial   ImportJobStatus = "partial"
	ImportFailed    ImportJobStatus = "failed"
)

// JobStatus is the lifecycle of an import or an export. The two share it
// because the states are genuinely the same; the rows are not.
type JobStatus = ImportJobStatus

// Job statuses, named without the Import prefix for the shared alias.
const (
	JobPending   = ImportPending
	JobRunning   = ImportRunning
	JobSucceeded = ImportSucceeded
	JobPartial   = ImportPartial
	JobFailed    = ImportFailed
)

// GroupSource records where a group's membership is managed.
type GroupSource string

// Group sources.
const (
	GroupSourceManual GroupSource = "manual"
	GroupSourceOIDC   GroupSource = "oidc"
	GroupSourceLDAP   GroupSource = "ldap"
)

// DefaultGroupName is the reserved name of the implicit "everyone" group.
const DefaultGroupName = "everyone"

// SupersededBy records that a page was superseded by another document: a
// snapshot of the replacement taken when it happened, so that it still reads
// after the replacement is renamed or gone. Stored as JSONB; nil when the page
// is not superseded.
type SupersededBy struct {
	// KnowledgeID and Title name the replacement's knowledge entry.
	KnowledgeID string `json:"knowledge_id"`
	Title       string `json:"title"`
	// PageID is set when the replacement is a page.
	PageID string    `json:"page_id,omitempty"`
	By     string    `json:"by,omitempty"`
	At     time.Time `json:"at"`
}

// Value implements driver.Valuer.
func (s SupersededBy) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (s *SupersededBy) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*s = SupersededBy{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("model.SupersededBy: cannot scan %T", value)
	}
	if len(raw) == 0 {
		*s = SupersededBy{}
		return nil
	}
	return json.Unmarshal(raw, s)
}
