// Package events is the docs module's domain event bus.
//
// Services publish an Event after every committed change; subscribers are
// the SSE streams that push tree/comment/notification updates to browsers,
// the ACL invalidation hook, the knowledge-base ingestion bridge and the
// history snapshotter. Delivery is best-effort and at-most-once: an event is
// a hint to refresh, never the record of truth (the database is).
//
// Two implementations share one contract: MemoryBus for a single process
// (single-instance deployments, tests) and RedisBus, which fans events out to every instance through
// one pub/sub channel while still delivering locally without a round trip.
package events

import (
	"context"
	"time"
)

// Type names an event. The vocabulary is closed on purpose: SSE clients
// switch on it, so adding one is an API change and belongs in this file.
type Type string

// Event types.
const (
	SpaceCreated       Type = "docs.space.created"
	SpaceUpdated       Type = "docs.space.updated"
	SpaceDeleted       Type = "docs.space.deleted"
	SpaceMembersChange Type = "docs.space.members_changed"

	PageCreated     Type = "docs.page.created"
	PageContent     Type = "docs.page.content_updated" // body persisted
	PageMeta        Type = "docs.page.meta_updated"    // title/icon/status/lock/...
	PageMoved       Type = "docs.page.moved"
	PageDeleted     Type = "docs.page.deleted"
	PageRestored    Type = "docs.page.restored"
	PagePurged      Type = "docs.page.purged"
	PageAccess      Type = "docs.page.access_changed"
	PageReplaced    Type = "docs.page.content_replaced" // import / restore / AI write-back
	RevisionCreated Type = "docs.revision.created"
	// TreeRebalanced: a sibling list was renumbered; reload that parent.
	TreeRebalanced Type = "docs.page.tree_rebalanced"
	// PageLease: somebody took, renewed or released the exclusive-edit lease
	// on a page. Only a deployment without a collaboration service emits it;
	// readers use it to show who is editing and when the page frees up.
	PageLease Type = "docs.page.lease_changed"

	// AttachmentAdded: a file was uploaded against a page, so an open view
	// can show it without waiting for the next save.
	AttachmentAdded Type = "docs.attachment.added"

	CommentChanged      Type = "docs.comment.changed"
	NotificationCreated Type = "docs.notification.created"
	// LabelChanged: a space's label vocabulary changed -- one was created,
	// renamed, recoloured or removed. Carries the space, so a list open on it
	// reloads; what happened to which page is not part of it, because a label
	// removed touches every page carrying it.
	ShareChanged    Type = "docs.share.changed"
	TemplateChanged Type = "docs.template.changed"
	LabelChanged    Type = "docs.label.changed"
	GroupChanged    Type = "docs.group.changed"
	ACLInvalidated  Type = "docs.acl.invalidated"
)

// Event is what travels over the bus. Payload is small, JSON-friendly data
// (ids, titles, positions); never document bodies.
type Event struct {
	Type     Type           `json:"type"`
	TenantID uint64         `json:"tenant_id"`
	SpaceID  string         `json:"space_id,omitempty"`
	PageID   string         `json:"page_id,omitempty"`
	ActorID  string         `json:"actor_id,omitempty"`
	At       time.Time      `json:"at"`
	Payload  map[string]any `json:"payload,omitempty"`
	// Origin identifies the publishing instance so a Redis subscriber can
	// skip events it already delivered locally.
	Origin string `json:"origin,omitempty"`
}

// Handler receives events. Handlers must not block: the memory bus calls
// them synchronously on the publisher's goroutine.
type Handler func(Event)

// Bus publishes and subscribes to events.
type Bus interface {
	// Publish delivers to local subscribers (and to peers, for RedisBus).
	Publish(ctx context.Context, e Event) error
	// Subscribe registers a handler for one tenant (0 = every tenant) and
	// returns the function that removes it.
	Subscribe(tenantID uint64, h Handler) (unsubscribe func())
	// Close stops background work; the bus must not be used afterwards.
	Close() error
}

// New fills defaults and returns the event for chaining.
func New(t Type, tenantID uint64) Event {
	return Event{Type: t, TenantID: tenantID, At: time.Now().UTC()}
}

// WithSpace sets the space id.
func (e Event) WithSpace(id string) Event { e.SpaceID = id; return e }

// WithPage sets the page id.
func (e Event) WithPage(id string) Event { e.PageID = id; return e }

// WithActor sets the acting user.
func (e Event) WithActor(id string) Event { e.ActorID = id; return e }

// With adds one payload entry.
func (e Event) With(key string, value any) Event {
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	e.Payload[key] = value
	return e
}
