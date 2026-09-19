// Package service holds the business rules of the docs module. Handlers
// parse requests and call in here; repositories persist what is decided
// here. Every mutation follows the same shape: validate, write in one
// transaction, then publish a domain event, record an audit row and drop the
// permission cache, in that order, so a failed write never leaves a trace.
//
// The rules were written after reading how established team wikis behave
// (space membership through users and groups, an implicit "everyone" group,
// a last-administrator invariant) and are an independent implementation.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// Directory resolves user IDs to display data. interfaces.UserRepository
// satisfies it.
type Directory interface {
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
}

// TenantMembers answers membership questions about the tenant.
// interfaces.TenantMemberRepository satisfies it.
type TenantMembers interface {
	Get(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error)
	ListPagedByTenant(ctx context.Context, tenantID uint64, search string, offset,
		limit int) ([]*types.TenantMember, error)
	CountFilteredByTenant(ctx context.Context, tenantID uint64, search string) (int64, error)
}

// KnowledgeBases checks that a knowledge base belongs to the tenant.
// interfaces.KnowledgeBaseRepository satisfies it.
type KnowledgeBases interface {
	GetKnowledgeBaseByIDAndTenant(ctx context.Context, id string, tenantID uint64) (*types.KnowledgeBase, error)
}

// StorageBackends checks that a storage backend is usable by the tenant.
// interfaces.StorageBackendRepository satisfies it.
type StorageBackends interface {
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.StorageBackend, error)
}

// Tokens validates a user's access token for the collaboration callbacks,
// which arrive without a session. interfaces.UserService satisfies it.
type Tokens interface {
	ValidateToken(ctx context.Context, token string) (*types.User, uint64, error)
}

// Storage resolves the FileService an attachment's bytes are written to and
// read back from. interfaces.StorageBackendResolver satisfies it.
type Storage interface {
	ResolveFileService(ctx context.Context, tenant *types.Tenant, backendID, provider,
		localBaseDir string) (interfaces.FileService, string, error)
}

// Tenants supplies the workspace storage accounting attachments are charged
// against. interfaces.TenantRepository satisfies it.
type Tenants interface {
	GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error)
	AdjustStorageUsed(ctx context.Context, tenantID uint64, delta int64) error
}

// CollabClient is the collaboration service as this package uses it:
// replacing a live document and dropping its connections.
type CollabClient interface {
	Configured() bool
	Replace(ctx context.Context, tenantID uint64, pageID string, content json.RawMessage,
		reason string) (*collab.ReplaceResult, error)
	Evict(ctx context.Context, pageID string) error
}

// Deps are the collaborators shared by every service.
// Favourites is the part of Yuheng's starred-resources service this module
// uses. Narrow on purpose: depending on the interface rather than the
// concrete service is the repository's own convention.
type Favourites interface {
	List(ctx context.Context, userID string, tenantID uint64, resourceType string) (
		[]*types.UserResourceFavorite, error)
	Add(ctx context.Context, userID string, tenantID uint64, resourceType, resourceID string) error
	Remove(ctx context.Context, userID string, tenantID uint64, resourceType, resourceID string) error
}

type Deps struct {
	Repos    *repository.Repositories
	Resolver *acl.Resolver
	Bus      events.Bus
	Audit    *audit.Recorder
	Users    Directory
	Members  TenantMembers
	// KnowledgeBases and StorageBackends may be nil (tests, trimmed builds);
	// binding a space to either is then rejected.
	KnowledgeBases  KnowledgeBases
	StorageBackends StorageBackends
	// Tokens is required by the collaboration callbacks only.
	Tokens Tokens
	// Collab is nil in the Lite edition (exclusive editing instead).
	Collab CollabClient
	// CollabURL is the browser-facing WebSocket address, empty when pages
	// are edited exclusively.
	CollabURL string
	// MaxYDocBytes caps one page's Yjs state; 0 uses DefaultMaxYDocBytes.
	MaxYDocBytes int64
	// Favourites is Yuheng's own starred-resources service, reused rather
	// than reimplemented: `doc_page` and `doc_space` are resource types in
	// it. Nil in trimmed builds, and starring is then simply unavailable.
	Favourites Favourites
	// Storage and Tenants are required by attachments only; without them
	// uploading is refused and the rest of the module still works.
	Storage Storage
	Tenants Tenants
	// MaxAttachmentBytes caps one upload; 0 uses DefaultMaxAttachmentBytes.
	MaxAttachmentBytes int64
	// Drafter writes pages from knowledge-base material. Nil makes the
	// feature unavailable, which is the state of a build without a model.
	Drafter Drafter
	// Knowledge mirrors pages into a space's knowledge base. Nil leaves
	// every space unindexed, which is the state of a build without it.
	Knowledge Knowledge
	// DefaultSpaceQuotaBytes is the attachment limit for spaces that have
	// none of their own; 0 leaves those spaces unlimited. It lets a careful
	// deployment be careful without anybody visiting every space.
	DefaultSpaceQuotaBytes int64
	// VariantCacheBytes bounds the in-memory cache of rendered image sizes;
	// 0 uses a 64 MiB default.
	VariantCacheBytes int
	// Embeds is the allow-list of external pages a document may frame. nil
	// refuses every embed, which is the safe direction for a build that
	// forgot to wire it.
	Embeds Embeds
	// DrawioURL is the self-hosted draw.io editor, reported to the client.
	DrawioURL string
	// PublicSharing allows pages to be published to anonymous URLs. Off by
	// default and off in every build that does not set it: an installation
	// that never wanted anything on the public internet should not acquire
	// the ability by upgrading.
	PublicSharing bool
	// HTTPClient makes the optional oEmbed metadata request; nil uses a
	// short-timeout client of its own.
	HTTPClient *http.Client
}

// Services groups the module's services.
type Services struct {
	Spaces *SpaceService
	Groups *GroupService
	Pages  *PageService
	Collab *CollabService
	// Leases serves the exclusive-edit transport used when no collaboration
	// service is configured.
	Leases *LeaseService
	Files  *AttachmentService
}

// New wires the services.
func New(d Deps) *Services {
	base := &base{d: d}
	files := &AttachmentService{base: base, variants: newVariantCache(d.VariantCacheBytes)}
	return &Services{
		Spaces: &SpaceService{base: base},
		Groups: &GroupService{base: base},
		// Pages holds the attachment service because importing a bundle
		// stores its images, and an import is a page operation that happens
		// to carry files rather than a file operation.
		Pages:  &PageService{base: base, files: files},
		Collab: &CollabService{base: base},
		Leases: &LeaseService{base: base},
		Files:  files,
	}
}

// base carries the shared collaborators and the post-commit side effects.
type base struct{ d Deps }

// publish sends a domain event; a failing bus is logged, never surfaced,
// because the write has already committed.
func (b *base) publish(ctx context.Context, e events.Event) {
	if b.d.Bus == nil {
		return
	}
	if err := b.d.Bus.Publish(ctx, e); err != nil {
		logger.Warnf(ctx, "[docs] publish %s failed: %v", e.Type, err)
	}
}

// audit records one row (nil-safe).
func (b *base) audit(ctx context.Context, e audit.Entry) {
	b.d.Audit.Record(ctx, e)
}

// invalidate drops the tenant's cached permission decisions on this
// instance immediately; the event published alongside reaches the others.
func (b *base) invalidate(ctx context.Context, tenantID uint64) {
	if b.d.Resolver != nil {
		b.d.Resolver.Invalidate(ctx, tenantID)
	}
}

// ---- validation helpers ---------------------------------------------------------

// Limits on user-supplied text, in runes.
const (
	MaxNameRunes        = 100
	MaxDescriptionRunes = 4000
	MaxSettingsBytes    = 16 * 1024
	// MaxBatch caps the principals accepted by one membership request.
	MaxBatch = 200
)

func invalid(format string, args ...any) error {
	return apperrors.NewValidationError(fmt.Sprintf(format, args...))
}

func conflict(format string, args ...any) error {
	return apperrors.NewConflictError(fmt.Sprintf(format, args...))
}

func forbidden(format string, args ...any) error {
	return apperrors.NewForbiddenError(fmt.Sprintf(format, args...))
}

func notFound(what string) error {
	return apperrors.NewNotFoundError(what + " not found")
}

// cleanName trims and validates a display name.
func cleanName(field, raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", invalid("%s is required", field)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return "", invalid("%s must be at most %d characters", field, MaxNameRunes)
	}
	if strings.ContainsAny(name, "\n\r\t") {
		return "", invalid("%s must be a single line", field)
	}
	return name, nil
}

func cleanDescription(raw string) (string, error) {
	desc := strings.TrimSpace(raw)
	if utf8.RuneCountInString(desc) > MaxDescriptionRunes {
		return "", invalid("description must be at most %d characters", MaxDescriptionRunes)
	}
	return desc, nil
}

// cleanSettings accepts a JSON object of bounded size.
func cleanSettings(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("{}"), nil
	}
	if len(raw) > MaxSettingsBytes {
		return nil, invalid("settings must be at most %d bytes", MaxSettingsBytes)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, invalid("settings must be a JSON object")
	}
	compact, err := json.Marshal(obj)
	if err != nil {
		return nil, invalid("settings must be a JSON object")
	}
	return compact, nil
}

// activeMember reports whether the user is an active member of the tenant.
func (b *base) activeMember(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	if b.d.Members == nil {
		return false, fmt.Errorf("docs: tenant member source not configured")
	}
	m, err := b.d.Members.Get(ctx, userID, tenantID)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return m != nil && m.Status == types.TenantMemberStatusActive, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "tenant member not found" || msg == "record not found"
}

// userViews hydrates user IDs into display rows; a directory failure
// degrades to bare IDs rather than failing the request.
func (b *base) users(ctx context.Context, ids []string) map[string]*types.User {
	if b.d.Users == nil || len(ids) == 0 {
		return map[string]*types.User{}
	}
	users, err := b.d.Users.GetUsersByIDs(ctx, ids)
	if err != nil {
		logger.Warnf(ctx, "[docs] user lookup failed: %v", err)
		return map[string]*types.User{}
	}
	return users
}

// UserView is how the module shows a user.
type UserView struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

func userView(id string, users map[string]*types.User) UserView {
	v := UserView{UserID: id}
	if u, ok := users[id]; ok && u != nil {
		v.Username, v.Email, v.Avatar = u.Username, u.Email, u.Avatar
	}
	return v
}

func actorID(actor *acl.Identity) string {
	if actor == nil {
		return ""
	}
	return actor.UserID
}

func actorRole(actor *acl.Identity) string {
	if actor == nil {
		return ""
	}
	return string(actor.TenantRole)
}

// dedupe returns the distinct non-empty strings in order of first appearance.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
