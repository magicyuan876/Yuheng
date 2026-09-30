package acl

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// ErrNoLongerMember reports that a subscriber lost its active membership of
// the tenant while subscribed. The stream serving it should end: nothing the
// tenant publishes is theirs to see any more.
var ErrNoLongerMember = errors.New("acl: subscriber is no longer an active member")

// rememberedPages bounds how many page ids one subscriber remembers having
// been shown. The memory only serves the "you just lost access" hint, so
// forgetting the oldest entries on a very long connection costs a missed
// repaint, never a leak.
const rememberedPages = 2048

// Audience decides which bus events one subscriber may receive.
//
// The bus fans every event of a tenant out to every subscriber of that
// tenant, and events carry page titles, notification recipients and the
// shape of the tree. Filtering on what the client asked for (?space=) is a
// convenience, not a boundary; this is the boundary. Each event is checked
// against the subscriber's current permissions, resolved the same way a
// request would be, so a grant revoked mid-connection takes effect on the
// next event instead of at reconnect.
//
// Rules, in order:
//   - A notification reaches only its recipient.
//   - A page event reaches whoever can read that page. When the page has left
//     the event's space (a cross-space move, as seen from the space it left)
//     or no longer exists (purged), the event describes the space's tree and
//     carries only ids, so it reaches the space's readers instead.
//   - A space event reaches the space's readers.
//   - An event with neither (groups, workspace templates, cache invalidation)
//     carries only ids and reaches every member.
//
// A subscriber who could see a page or space earlier on this connection
// still receives the one event that takes it away, so the open view repaints
// instead of keeping a stale title on screen. Those events are the only
// exception and carry nothing the subscriber had not already been shown.
//
// An Audience belongs to one connection and is not safe for concurrent use.
type Audience struct {
	res      *Resolver
	tenantID uint64
	userID   string
	// machine is the fixed identity of an API-key subscriber. A key's
	// identity comes from its scope, not from stored memberships, so there
	// is nothing to re-resolve; revoking the key ends its future requests,
	// and a stream is one of them only until it reconnects.
	machine *Identity

	spaces map[string]bool
	// pages maps a remembered page to the sequence number it was remembered
	// under; order holds the same pairs oldest first. Eviction removes a page
	// only while its number still matches, so a page forgotten and shown
	// again is not dropped by its own older entry.
	pages map[string]uint64
	order []rememberedPage
	seq   uint64
}

type rememberedPage struct {
	id  string
	seq uint64
}

// Audience starts the event filter for one subscriber. The spaces they can
// see now are remembered up front, so the event that deletes one of them
// (after which it can no longer be resolved) still reaches them.
func (r *Resolver) Audience(ctx context.Context, id *Identity) (*Audience, error) {
	visible, err := r.VisibleSpaces(ctx, id)
	if err != nil {
		return nil, err
	}
	a := &Audience{
		res: r, tenantID: id.TenantID, userID: id.UserID,
		spaces: make(map[string]bool, len(visible)), pages: map[string]uint64{},
	}
	if id.Machine {
		a.machine = id
	}
	for spaceID := range visible {
		a.spaces[spaceID] = true
	}
	return a, nil
}

// Admits reports whether the subscriber may receive e. It returns
// ErrNoLongerMember once the subscriber has left the tenant, and any other
// error when permissions could not be resolved; the caller must then drop
// the event (fail closed), never deliver it.
func (a *Audience) Admits(ctx context.Context, e events.Event) (bool, error) {
	if e.TenantID != a.tenantID {
		return false, nil
	}
	id, err := a.identity(ctx)
	if err != nil {
		return false, err
	}
	if !id.Member {
		return false, ErrNoLongerMember
	}
	switch {
	case e.Type == events.NotificationCreated:
		recipient, _ := e.Payload["user_id"].(string)
		return recipient == a.userID, nil
	case e.PageID != "":
		return a.admitsPage(ctx, id, e)
	case e.SpaceID != "":
		return a.admitsSpace(ctx, id, e.SpaceID)
	default:
		return true, nil
	}
}

// identity re-resolves a user subscriber on every event (the resolver caches
// it), so a membership ended mid-connection is noticed.
func (a *Audience) identity(ctx context.Context) (*Identity, error) {
	if a.machine != nil {
		return a.machine, nil
	}
	return a.res.Identity(ctx, a.tenantID, a.userID)
}

func (a *Audience) admitsPage(ctx context.Context, id *Identity, e events.Event) (bool, error) {
	d, err := a.res.Page(ctx, id, e.PageID)
	if errors.Is(err, repository.ErrNotFound) {
		// Not live: in the trash, in a trashed space, or purged. A trashed
		// page is still judged by its own chain, which Decide can walk.
		page, getErr := a.res.repos.Pages.GetAny(ctx, a.tenantID, e.PageID)
		if errors.Is(getErr, repository.ErrNotFound) {
			return a.admitsSpace(ctx, id, e.SpaceID)
		}
		if getErr != nil {
			return false, getErr
		}
		d, err = a.res.Decide(ctx, id, page)
	}
	if errors.Is(err, repository.ErrNotFound) {
		// The page's space is in the trash; only those who had it open hear
		// about it.
		return a.forgetPage(e.PageID), nil
	}
	if err != nil {
		return false, err
	}
	if e.SpaceID != "" && d.Page.SpaceID != e.SpaceID {
		return a.admitsSpace(ctx, id, e.SpaceID)
	}
	if d.Allows(model.RoleReader) {
		a.rememberPage(e.PageID)
		return true, nil
	}
	return a.forgetPage(e.PageID), nil
}

func (a *Audience) admitsSpace(ctx context.Context, id *Identity, spaceID string) (bool, error) {
	if spaceID == "" {
		return true, nil
	}
	space, err := a.res.repos.Spaces.Get(ctx, a.tenantID, spaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return a.forgetSpace(spaceID), nil
	}
	if err != nil {
		return false, err
	}
	role, err := a.res.SpaceRole(ctx, id, space)
	if err != nil {
		return false, err
	}
	if role != model.RoleNone {
		a.spaces[spaceID] = true
		return true, nil
	}
	return a.forgetSpace(spaceID), nil
}

// forgetSpace and forgetPage drop a remembered space or page and report
// whether it was remembered: the subscriber had been shown it, so this last
// event is theirs to repaint on.
func (a *Audience) forgetSpace(spaceID string) bool {
	if !a.spaces[spaceID] {
		return false
	}
	delete(a.spaces, spaceID)
	return true
}

func (a *Audience) forgetPage(pageID string) bool {
	if _, ok := a.pages[pageID]; !ok {
		return false
	}
	delete(a.pages, pageID)
	return true
}

func (a *Audience) rememberPage(pageID string) {
	if _, ok := a.pages[pageID]; ok {
		return
	}
	a.seq++
	a.pages[pageID] = a.seq
	a.order = append(a.order, rememberedPage{id: pageID, seq: a.seq})
	for len(a.pages) > rememberedPages && len(a.order) > 0 {
		oldest := a.order[0]
		a.order = a.order[1:]
		if a.pages[oldest.id] == oldest.seq {
			delete(a.pages, oldest.id)
		}
	}
	// Entries of forgotten pages linger in order until they reach the front;
	// compact once they outnumber the live ones so the slice stays bounded.
	if len(a.order) > 2*rememberedPages {
		live := make([]rememberedPage, 0, len(a.pages))
		for _, r := range a.order {
			if a.pages[r.id] == r.seq {
				live = append(live, r)
			}
		}
		a.order = live
	}
}
