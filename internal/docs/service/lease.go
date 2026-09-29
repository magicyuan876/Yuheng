package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// LeaseService is how a page is edited when no collaboration service is
// configured, that is in any deployment that leaves YUHENG_COLLAB_URL empty.
//
// The editor is the same one collaborative deployments run: the same Tiptap
// extensions over the same Yjs document. Only the transport changes. Instead
// of a WebSocket that merges everyone's updates, one editor at a time holds a
// short lease on the page and posts the whole Yjs state over REST; everybody
// else reads. That is a real limitation, and it is deliberate: merging CRDT
// updates needs a process such a deployment does not have, and silently letting two
// people write would lose one of them. A lease makes the limitation visible
// instead.
//
// The lease is short (five minutes) and renewed while the tab is open, so a
// browser that crashes or a laptop that sleeps frees the page on its own; no
// administrator ever has to unlock anything.
type LeaseService struct{ *base }

// Lease timings. They are constants rather than configuration: the numbers
// only matter relative to each other, and the client's renewal interval is
// derived from the TTL it is handed.
const (
	// LeaseTTL is how long a lease stays valid without a renewal.
	LeaseTTL = 5 * time.Minute
	// LeaseRenewInterval is the cadence the client is told to renew at. It
	// is well inside the TTL so a couple of failed requests are survivable.
	LeaseRenewInterval = 60 * time.Second
	// MaxSessionIDRunes bounds the opaque per-tab identifier a client sends.
	MaxSessionIDRunes = 64
)

// ErrExclusiveEditingDisabled is returned when a lease route is called on a
// deployment that runs a collaboration service. Such a deployment merges
// concurrent edits and must never hand out an exclusive lease: a client that
// asks has the wrong idea about the deployment and should re-read
// docs_collab_url from the capabilities endpoint.
var ErrExclusiveEditingDisabled = errors.New(
	"docs: this deployment edits pages through the collaboration service, not exclusive leases")

// LeaseView is what a client is told about a page's lease.
type LeaseView struct {
	PageID string `json:"page_id"`
	// Held is false when the page is free to take.
	Held bool `json:"held"`
	// HeldByMe distinguishes "you may type" from "somebody else is typing".
	HeldByMe bool `json:"held_by_me"`
	// Holder is the current editor; zero when the page is free.
	Holder UserView `json:"holder,omitzero"`
	// ExpiresAt is when the lease lapses unless renewed.
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	// RenewAfterSeconds is the cadence the holder should renew at.
	RenewAfterSeconds int `json:"renew_after_seconds"`
	// YDocVersion lets the holder start from the right base version without
	// a second round trip.
	YDocVersion int64 `json:"ydoc_version"`
}

// YDocView carries a page's editing state to the REST provider. Exactly one
// of YDoc and Content is set: a page that has never been edited has no Yjs
// state yet, and the client builds one from the stored JSON body.
type YDocView struct {
	PageID string `json:"page_id"`
	// YDoc is the Yjs state, base64 so it fits the JSON envelope every other
	// endpoint uses. Pages are small; a binary body is not worth a
	// second response shape.
	YDoc string `json:"ydoc,omitempty"`
	// Content is the ProseMirror body to materialise a Yjs document from.
	Content     json.RawMessage `json:"content,omitempty"`
	YDocVersion int64           `json:"ydoc_version"`
}

// SaveYDocInput is one save from the REST provider.
type SaveYDocInput struct {
	SessionID   string
	BaseVersion int64
	// YDoc is the full Yjs state, base64-encoded.
	YDoc    string
	Content json.RawMessage
}

// SaveYDocResult reports the stored version and the refreshed lease, so a
// save doubles as a renewal and the client needs no separate heartbeat while
// somebody is actually typing.
type SaveYDocResult struct {
	YDocVersion int64      `json:"ydoc_version"`
	Lease       *LeaseView `json:"lease"`
}

// exclusive returns an error unless this deployment edits exclusively.
func (s *LeaseService) exclusive() error {
	if strings.TrimSpace(s.d.CollabURL) != "" {
		return conflict("%v", ErrExclusiveEditingDisabled)
	}
	return nil
}

// cleanSessionID validates the opaque per-tab identifier. It is never shown to
// anyone and never trusted for authorisation — the user id in the lease row is
// what authorises — so any short printable string will do.
func cleanSessionID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", invalid("session_id is required")
	}
	if len([]rune(id)) > MaxSessionIDRunes {
		return "", invalid("session_id must be at most %d characters", MaxSessionIDRunes)
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return "", invalid("session_id must not contain control characters")
		}
	}
	return id, nil
}

// Current reports who is editing the page. Any reader may ask: knowing that a
// colleague is typing is exactly what stops two people from fighting over the
// page, and the answer carries no more than that colleague's display name,
// which every space member can already see in the member list.
func (s *LeaseService) Current(ctx context.Context, actor *acl.Identity, d acl.Decision) (*LeaseView, error) {
	if err := s.exclusive(); err != nil {
		return nil, err
	}
	return s.view(ctx, d.Page, actorID(actor), time.Now().UTC())
}

// Acquire takes the lease, or renews it when this session already holds it.
// One call serves both so the client has a single heartbeat: it posts every
// minute and learns from the answer whether it may still type.
//
// A caller who cannot write the page is refused rather than handed a lease it
// could not use, and a locked page is refused for everyone but a space admin,
// matching what the collaboration service's authenticate callback decides.
func (s *LeaseService) Acquire(ctx context.Context, actor *acl.Identity, d acl.Decision,
	sessionID string,
) (*LeaseView, error) {
	if err := s.exclusive(); err != nil {
		return nil, err
	}
	session, err := cleanSessionID(sessionID)
	if err != nil {
		return nil, err
	}
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	holder, acquired, err := s.d.Repos.Leases.Acquire(ctx, model.EditLease{
		PageID:    d.Page.ID,
		TenantID:  d.Page.TenantID,
		UserID:    actorID(actor),
		SessionID: session,
		ExpiresAt: at.Add(LeaseTTL),
	}, at)
	if err != nil {
		return nil, err
	}
	if acquired {
		s.publish(ctx, events.New(events.PageLease, d.Page.TenantID).
			WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
			With("held", true).With("expires_at", holder.ExpiresAt))
	}
	return s.leaseView(ctx, d.Page, holder, session, actorID(actor), at), nil
}

// Release hands the page back early. Only the holding session can, so a stale
// tab closing after somebody else took over cannot unlock the new holder.
func (s *LeaseService) Release(ctx context.Context, actor *acl.Identity, d acl.Decision,
	sessionID string,
) error {
	if err := s.exclusive(); err != nil {
		return err
	}
	session, err := cleanSessionID(sessionID)
	if err != nil {
		return err
	}
	released, err := s.d.Repos.Leases.Release(ctx, d.Page.TenantID, d.Page.ID, actorID(actor), session)
	if err != nil {
		return err
	}
	if released {
		s.publish(ctx, events.New(events.PageLease, d.Page.TenantID).
			WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).With("held", false))
	}
	return nil
}

// LoadYDoc returns the state to open the editor on. It needs only read
// permission: the read-only view runs the same editor, and a reader who can
// see the page can already see its body.
func (s *LeaseService) LoadYDoc(ctx context.Context, d acl.Decision) (*YDocView, error) {
	if err := s.exclusive(); err != nil {
		return nil, err
	}
	state, err := s.loadState(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	out := &YDocView{PageID: d.Page.ID, YDocVersion: state.YDocVersion, Content: state.Content}
	if len(state.YDoc) > 0 {
		out.YDoc = base64.StdEncoding.EncodeToString(state.YDoc)
	}
	return out, nil
}

// SaveYDoc stores a full Yjs state and its ProseMirror projection.
//
// The lease is checked first and the write is still guarded by the base
// version, which is not redundant: a lease can expire mid-edit and be taken
// over, and the version check is what turns that into a clean 409 the client
// recovers from by reloading, rather than a silent overwrite of the new
// holder's work.
func (s *LeaseService) SaveYDoc(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in SaveYDocInput,
) (*SaveYDocResult, error) {
	if err := s.exclusive(); err != nil {
		return nil, err
	}
	session, err := cleanSessionID(in.SessionID)
	if err != nil {
		return nil, err
	}
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	if len(in.Content) == 0 {
		return nil, invalid("content is required")
	}
	ydoc, err := base64.StdEncoding.DecodeString(in.YDoc)
	if err != nil {
		return nil, invalid("ydoc must be base64")
	}
	if len(ydoc) == 0 {
		return nil, invalid("ydoc is required")
	}

	at := time.Now().UTC().Truncate(time.Microsecond)
	holder, err := s.d.Repos.Leases.Get(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, conflict("the edit lease has lapsed; take it again before saving")
		}
		return nil, err
	}
	if holder.UserID != actorID(actor) || holder.SessionID != session || !holder.ExpiresAt.After(at) {
		return nil, conflict("somebody else is editing this page; reload to see their changes")
	}

	res, err := s.persist(ctx, PersistInput{
		TenantID:       d.Page.TenantID,
		PageID:         d.Page.ID,
		BaseVersion:    in.BaseVersion,
		YDoc:           ydoc,
		Content:        in.Content,
		EditorIDs:      []string{actorID(actor)},
		AwarenessCount: 1,
	})
	if err != nil {
		return nil, err
	}

	// A save proves the tab is alive, so it renews the lease too and the
	// client's heartbeat only has to run while nobody is typing.
	renewed, _, err := s.d.Repos.Leases.Acquire(ctx, model.EditLease{
		PageID:    d.Page.ID,
		TenantID:  d.Page.TenantID,
		UserID:    actorID(actor),
		SessionID: session,
		ExpiresAt: at.Add(LeaseTTL),
	}, at)
	if err != nil {
		return nil, err
	}
	return &SaveYDocResult{
		YDocVersion: res.YDocVersion,
		Lease:       s.leaseView(ctx, d.Page, renewed, session, actorID(actor), at),
	}, nil
}

// view loads the stored lease and shapes it for viewerID.
func (s *LeaseService) view(ctx context.Context, page *model.Page, viewerID string,
	at time.Time,
) (*LeaseView, error) {
	lease, err := s.d.Repos.Leases.Get(ctx, page.TenantID, page.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &LeaseView{
				PageID: page.ID, RenewAfterSeconds: int(LeaseRenewInterval / time.Second),
				YDocVersion: page.YDocVersion,
			}, nil
		}
		return nil, err
	}
	return s.leaseView(ctx, page, lease, "", viewerID, at), nil
}

// leaseView shapes a lease row. viewerID is who is asking, which is what
// decides held_by_me; an expired row is reported as free, because that is what
// the next Acquire will find.
func (s *LeaseService) leaseView(ctx context.Context, page *model.Page, lease *model.EditLease,
	session, viewerID string, at time.Time,
) *LeaseView {
	out := &LeaseView{
		PageID:            page.ID,
		RenewAfterSeconds: int(LeaseRenewInterval / time.Second),
		YDocVersion:       page.YDocVersion,
	}
	if lease == nil || !lease.ExpiresAt.After(at) {
		return out
	}
	out.Held = true
	out.ExpiresAt = lease.ExpiresAt
	out.HeldByMe = lease.UserID == viewerID && (session == "" || lease.SessionID == session)
	out.Holder = userView(lease.UserID, s.users(ctx, []string{lease.UserID}))
	return out
}
