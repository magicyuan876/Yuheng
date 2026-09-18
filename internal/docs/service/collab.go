package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// CollabService answers the three callbacks of the collaboration service.
//
// The division of labour is fixed: the service merges Yjs updates and knows
// nothing else; this side decides who may connect, hands out the state to
// load, and owns everything that happens when a document is persisted
// (validation, rendering, statistics, events). The Yjs state is the truth
// source and the ProseMirror JSON is its projection, so both are written in
// the same statement, guarded by the version the writer started from.
type CollabService struct{ *base }

// DefaultMaxYDocBytes bounds one page's Yjs state when no limit is configured.
const DefaultMaxYDocBytes int64 = 20 * 1024 * 1024

// CollabAccess is what a connection may do.
type CollabAccess string

// Access levels handed to the collaboration service.
const (
	AccessReadWrite CollabAccess = "readwrite"
	AccessReadOnly  CollabAccess = "readonly"
)

// AuthenticateInput identifies a connection attempt.
type AuthenticateInput struct {
	Token string
	// TenantID is the workspace the browser is working in; empty falls back
	// to the workspace encoded in the token.
	TenantID     uint64
	PageID       string
	ConnectionID string
}

// AuthenticateResult is the answer the collaboration service caches for the
// life of a connection.
type AuthenticateResult struct {
	UserID      string       `json:"user_id"`
	DisplayName string       `json:"display_name"`
	Avatar      string       `json:"avatar"`
	Access      CollabAccess `json:"access"`
	YDocVersion int64        `json:"ydoc_version"`
	TenantID    string       `json:"tenant_id"`
	SpaceID     string       `json:"space_id"`
}

// LoadResult carries either the stored Yjs state or, for a page that has
// never been opened collaboratively, its JSON body.
type LoadResult struct {
	YDoc        []byte
	Content     json.RawMessage
	YDocVersion int64
}

// PersistInput is one store call.
type PersistInput struct {
	TenantID       uint64
	PageID         string
	BaseVersion    int64
	YDoc           []byte
	Content        json.RawMessage
	EditorIDs      []string
	AwarenessCount int
}

// PersistResult reports the version the page now holds.
type PersistResult struct {
	YDocVersion int64 `json:"ydoc_version"`
}

// ErrCollabUnauthorized means the token is not valid, or the user cannot see
// the page at all. The caller answers 401 so the service closes the socket.
var ErrCollabUnauthorized = errors.New("docs: collaboration connection is not authorised")

// Authenticate resolves a WebSocket connection: it validates the user's
// token, checks that the user is an active member of the workspace, and
// resolves the effective role on the page. A user who cannot read the page
// is rejected; one who cannot write it gets a read-only connection.
func (s *CollabService) Authenticate(ctx context.Context, in AuthenticateInput) (*AuthenticateResult, error) {
	if s.d.Tokens == nil {
		return nil, fmt.Errorf("docs: no token validator is configured")
	}
	user, tokenTenant, err := s.d.Tokens.ValidateToken(ctx, in.Token)
	if err != nil || user == nil {
		return nil, ErrCollabUnauthorized
	}
	if !user.IsActive {
		return nil, ErrCollabUnauthorized
	}
	tenantID := in.TenantID
	if tenantID == 0 {
		tenantID = tokenTenant
	}
	if tenantID == 0 {
		return nil, ErrCollabUnauthorized
	}
	identity, err := s.d.Resolver.Identity(ctx, tenantID, user.ID)
	if err != nil {
		return nil, err
	}
	if !identity.Member {
		return nil, ErrCollabUnauthorized
	}
	decision, err := s.d.Resolver.Page(ctx, identity, in.PageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCollabUnauthorized
		}
		return nil, err
	}
	if decision.Role == model.RoleNone {
		return nil, ErrCollabUnauthorized
	}
	access := AccessReadOnly
	if canEdit(decision.Role, decision.Page) {
		access = AccessReadWrite
	}
	return &AuthenticateResult{
		UserID:      user.ID,
		DisplayName: displayName(user),
		Avatar:      user.Avatar,
		Access:      access,
		YDocVersion: decision.Page.YDocVersion,
		TenantID:    fmt.Sprint(tenantID),
		SpaceID:     decision.Page.SpaceID,
	}, nil
}

func displayName(u *types.User) string {
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// Load hands the collaboration service the state to open a document with.
// Permission was already established by Authenticate; this call only reads.
func (s *CollabService) Load(ctx context.Context, tenantID uint64, pageID string) (*LoadResult, error) {
	return s.loadState(ctx, tenantID, pageID)
}

// loadState reads a page's editing state. Both editing transports use it: the
// collaboration service over its internal callback, and the exclusive-edit
// REST provider over the public API.
func (b *base) loadState(ctx context.Context, tenantID uint64, pageID string) (*LoadResult, error) {
	page, err := b.d.Repos.Pages.Get(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	out := &LoadResult{YDocVersion: page.YDocVersion}
	if len(page.YDoc) > 0 {
		out.YDoc = page.YDoc
		return out, nil
	}
	content := page.Content
	if len(content) == 0 {
		content = EmptyDocument
	}
	out.Content = json.RawMessage(content)
	return out, nil
}

// Persist writes a merged document. It validates the body against the
// document model, derives the search text and statistics, and writes both
// representations under the caller's base version; a version that has moved
// on yields repository.ErrConflict so the collaboration service can merge
// the newer state and try again.
func (s *CollabService) Persist(ctx context.Context, in PersistInput) (*PersistResult, error) {
	return s.persist(ctx, in)
}

// persist is the single write path for a page body. It is shared verbatim by
// the collaboration service's store callback and by the exclusive-edit REST
// provider, so a page saved either way goes through the same validation,
// rendering, statistics and event publication.
func (s *base) persist(ctx context.Context, in PersistInput) (*PersistResult, error) {
	limit := s.d.MaxYDocBytes
	if limit <= 0 {
		limit = DefaultMaxYDocBytes
	}
	if int64(len(in.YDoc)) > limit {
		return nil, invalid("the page state is %d bytes, over the %d byte limit; split the page", len(in.YDoc), limit)
	}
	page, err := s.d.Repos.Pages.Get(ctx, in.TenantID, in.PageID)
	if err != nil {
		return nil, err
	}
	if page.IsLocked {
		return nil, forbidden("the page is locked")
	}
	node, _, err := schema.Default().Validate(in.Content)
	if err != nil {
		return nil, invalid("content: %v", err)
	}
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}
	structure := render.Extract(node)
	text := render.Text(node)
	// A document may only ever come to hold an iframe this deployment allows.
	// Checking here rather than only at render time is what keeps a refused
	// embed out of the stored document altogether.
	if err := s.checkEmbeds(structure); err != nil {
		return nil, err
	}

	editors := dedupe(in.EditorIDs)
	contributors := append(model.StringList{}, page.ContributorIDs...)
	for _, e := range editors {
		contributors = contributors.Add(e)
	}
	if page.CreatorID != nil {
		contributors = contributors.Add(*page.CreatorID)
	}
	upd := repository.ContentUpdate{
		Content:        model.JSON(raw),
		YDoc:           in.YDoc,
		TextContent:    text,
		WordCount:      structure.WordCount,
		ContributorIDs: contributors,
		ContentChanged: !sameDocument(page.Content, raw),
	}
	if len(editors) > 0 {
		upd.EditorID = editors[len(editors)-1]
	}
	version, err := s.d.Repos.Pages.UpdateContent(ctx, in.TenantID, in.PageID, in.BaseVersion, upd)
	if err != nil {
		return nil, err
	}
	// A file pasted into the page becomes that page's attachment here, in the
	// one place both editing transports pass through, rather than at upload
	// time: until the document that references it is saved, nothing says the
	// paste was kept.
	s.bindAttachments(ctx, page, structure.AttachmentIDs)
	// Backlinks are derived from the document, so they are rebuilt here rather
	// than tracked edit by edit: removing a link means deleting the text around
	// it, which produces no event of its own.
	s.recordLinks(ctx, page, structure)
	// Which blocks of other pages this document now watches, and which of
	// this page's own blocks other documents watch. Two different jobs; see
	// transclusion.go.
	s.recordTransclusions(ctx, page, structure)
	s.refreshTransclusionSnapshots(ctx, page, node)
	if upd.ContentChanged {
		s.publish(ctx, events.New(events.PageContent, in.TenantID).WithSpace(page.SpaceID).WithPage(in.PageID).
			WithActor(upd.EditorID).With("version", version).With("word_count", structure.WordCount).
			With("editors", editors).With("awareness_count", in.AwarenessCount))
	}
	return &PersistResult{YDocVersion: version}, nil
}

// sameDocument reports whether two encodings hold the same document, so a
// persist that only re-encodes an unchanged body does not look like an edit.
func sameDocument(stored model.JSON, fresh []byte) bool {
	if len(stored) == 0 {
		return false
	}
	var a, b any
	if json.Unmarshal(stored, &a) != nil || json.Unmarshal(fresh, &b) != nil {
		return false
	}
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}

// Evict asks the collaboration service to drop a page's connections. It is
// best effort: the caller's write has already committed, and a missing or
// unreachable service must not fail the request.
func (b *base) evict(ctx context.Context, pageID string) {
	if b.d.Collab == nil || !b.d.Collab.Configured() {
		return
	}
	// The request is short-lived and must not be cancelled by the caller
	// returning its response.
	go func() {
		detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := b.d.Collab.Evict(detached, pageID); err != nil {
			logger.Warnf(detached, "[docs] evicting page %s from the collaboration service failed: %v", pageID, err)
		}
	}()
}

// CollabURL is the browser-facing WebSocket address, or empty in exclusive
// edit mode. Handlers expose it so the editor knows which provider to use.
func (s *CollabService) CollabURL() string { return s.d.CollabURL }
