// Package notify holds the decisions behind notifications: who hears about
// something, and when two of them are really one.
//
// Both come straight from the design's §8.3 table, and both are the sort of
// rule that is easy to state and easy to get subtly wrong, so they are pure
// functions over values with the clock passed in.
//
// The table:
//
//	event                  recipients                              merging
//	------------------------------------------------------------------
//	comment on a page      watchers, the author, the person        one per page
//	                       being replied to                        per 10 minutes
//	mentioned by name      the person named                        never
//	page content changed   people who chose to watch it            one per page
//	                                                               per hour
//	granted access         the person granted it                   never
//
// A fifth row — a share link passing a view threshold — belongs to T4.2,
// which introduces shares; it is deliberately absent rather than stubbed.
package notify

import (
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// Kind names what happened. Stored in docs_notifications.kind.
type Kind string

const (
	// Commented is a new comment or reply on a page.
	Commented Kind = "comment"
	// Mentioned is somebody naming you, in a page or in a comment.
	Mentioned Kind = "mention"
	// PageUpdated is the body of a watched page changing.
	PageUpdated Kind = "page_updated"
	// AccessGranted is being given access to a page or a space.
	AccessGranted Kind = "access_granted"
)

// Merge windows, from the table above.
const (
	// CommentWindow: a conversation is one event, not six.
	CommentWindow = 10 * time.Minute
	// PageUpdateWindow: somebody editing all afternoon is one event.
	PageUpdateWindow = time.Hour
)

// MergeWindow says how long a notification of this kind absorbs later ones
// about the same page.
//
// Zero means never merge, and the two that return it are the two that are
// *about* you rather than about a page: being named, and being given access.
// Rolling those up would turn "you have been mentioned" into "something
// happened", which is the one thing a notification must not do.
func MergeWindow(kind Kind) time.Duration {
	switch kind {
	case Commented:
		return CommentWindow
	case PageUpdated:
		return PageUpdateWindow
	default:
		return 0
	}
}

// Existing is what is already in somebody's inbox, as merging needs to see it.
type Existing struct {
	ID        string
	Kind      Kind
	PageID    string
	CreatedAt time.Time
	// ReadAt marks a notification somebody has already looked at.
	ReadAt *time.Time
}

// Candidate is a notification about to be written.
type Candidate struct {
	Kind   Kind
	UserID string
	PageID string
	// ActorID is who did the thing.
	ActorID string
}

// MergeInto finds the notification a candidate should be folded into, or nil
// when it deserves one of its own.
//
// Two rules beyond the window, and both matter:
//
//   - A notification that has already been read is never merged into. It has
//     done its job; folding a new event into it would hide that event behind
//     a row the reader has dismissed, and they would never learn of it.
//   - Only the same kind about the same page merges. A comment and an edit on
//     one page are two different things to be told about.
func MergeInto(candidate Candidate, existing []Existing, now time.Time) *Existing {
	window := MergeWindow(candidate.Kind)
	if window == 0 || candidate.PageID == "" {
		return nil
	}
	for i := range existing {
		row := &existing[i]
		if row.Kind != candidate.Kind || row.PageID != candidate.PageID {
			continue
		}
		if row.ReadAt != nil {
			continue
		}
		if now.Sub(row.CreatedAt) < window {
			return row
		}
	}
	return nil
}

// Watcher is what recipient selection needs to know about one.
type Watcher struct {
	UserID string
	// Muted silences a page without giving up watching it, so somebody can
	// stop hearing about a page they still want in their list.
	Muted bool
	// Manual is true when the person chose to watch rather than being
	// enrolled by commenting or being mentioned.
	Manual bool
}

// CommentAudience is who hears about a new comment.
//
// The author of the page and the person being replied to are included whether
// or not they watch it: commenting on somebody's page is addressed to them,
// and answering somebody is addressed to them, and neither should require
// having remembered to press a button.
func CommentAudience(watchers []Watcher, pageAuthorID, repliedToID, actorID string) []string {
	out := newAudience(actorID)
	for _, watcher := range watchers {
		if !watcher.Muted {
			out.add(watcher.UserID)
		}
	}
	// Not subject to muting: these two are being spoken to, not subscribed.
	out.add(pageAuthorID)
	out.add(repliedToID)
	return out.list
}

// UpdateAudience is who hears that a page's body changed.
//
// Only people who chose to watch it. Everybody who has ever commented on a
// busy page would otherwise be told about every edit to it, which is how a
// notification list becomes something people stop reading.
func UpdateAudience(watchers []Watcher, actorID string) []string {
	out := newAudience(actorID)
	for _, watcher := range watchers {
		if watcher.Manual && !watcher.Muted {
			out.add(watcher.UserID)
		}
	}
	return out.list
}

// MentionAudience is who hears about being named.
//
// Muting does not apply: being named is a direct address, and somebody who
// muted a page still wants to know they were asked a question on it.
func MentionAudience(mentioned []string, actorID string) []string {
	out := newAudience(actorID)
	for _, id := range mentioned {
		out.add(id)
	}
	return out.list
}

// audience collects recipients, dropping repeats, blanks, and the person who
// caused the event.
//
// That last one is the third acceptance criterion of this work package and
// the easiest to lose: every path adds people from a different source, so the
// exclusion lives here rather than at each call site.
type audience struct {
	actorID string
	seen    map[string]bool
	list    []string
}

func newAudience(actorID string) *audience {
	return &audience{actorID: actorID, seen: map[string]bool{}, list: []string{}}
}

func (a *audience) add(userID string) {
	if userID == "" || userID == a.actorID || a.seen[userID] {
		return
	}
	a.seen[userID] = true
	a.list = append(a.list, userID)
}

// AutoWatch says whether an action should enrol somebody as a watcher, and
// why.
//
// From §8.3: creating a page, commenting on one, or being mentioned in one
// all make you a watcher. The reason is kept because it decides what you hear
// about afterwards — only a manual watcher is told about every edit, so
// somebody who once answered a question on a busy page is not signed up to
// its entire future.
//
// Returns false for an action that enrols nobody.
func AutoWatch(action string) (model.WatchReason, bool) {
	switch action {
	case "create":
		return model.WatchAuthor, true
	case "comment":
		return model.WatchComment, true
	case "mention":
		return model.WatchMention, true
	default:
		return "", false
	}
}

// Upgrades reports whether a new reason should replace an existing one.
//
// A manual watch is the strongest: somebody chose it, and being mentioned
// afterwards must not quietly downgrade them to hearing less. Otherwise the
// first reason stands, because it is the true one — you did become a watcher
// by commenting, whatever happened next.
func Upgrades(existing, incoming model.WatchReason) bool {
	if existing == model.WatchManual {
		return false
	}
	return incoming == model.WatchManual
}
