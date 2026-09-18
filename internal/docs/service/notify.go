package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/notify"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Watching pages, and being told what happened on them.
//
// The rules — who hears about what, and when two notices are really one —
// are in internal/docs/notify, with no storage and no clock. What is here is
// the wiring: gather the audience, ask the rules, write or fold a row, push
// it to whoever is connected.
//
// Like every other derived thing on the save path, none of this is allowed to
// fail the action that caused it. Somebody's comment must not be refused
// because a notification could not be written.

// NotificationView is one entry in somebody's inbox.
type NotificationView struct {
	ID        string      `json:"id"`
	Kind      notify.Kind `json:"kind"`
	PageID    string      `json:"page_id,omitempty"`
	SpaceID   string      `json:"space_id,omitempty"`
	CommentID string      `json:"comment_id,omitempty"`
	ActorID   string      `json:"actor_id,omitempty"`
	Actor     UserView    `json:"actor,omitzero"`
	// Payload carries what the row is about: the page title, an excerpt, and
	// for a merged notice how many events it stands for.
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// NotificationPage is one page of an inbox.
type NotificationPage struct {
	Items      []*NotificationView `json:"items"`
	Unread     int64               `json:"unread"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// WatchView is somebody's relationship to a page.
type WatchView struct {
	PageID  string            `json:"page_id"`
	Reason  model.WatchReason `json:"reason"`
	Muted   bool              `json:"muted"`
	Watched bool              `json:"watched"`
}

// ---- watching ------------------------------------------------------------------

// WatchPage records that this caller wants to hear about a page.
func (s *PageService) WatchPage(ctx context.Context, actor *acl.Identity, d acl.Decision,
	watch bool,
) (*WatchView, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Watchers == nil {
		return nil, notFound("page")
	}
	if !watch {
		if err := s.d.Repos.Watchers.Unwatch(ctx, d.Page.TenantID, actorID(actor), d.Page.ID); err != nil {
			return nil, err
		}
		return &WatchView{PageID: d.Page.ID}, nil
	}

	pageID := d.Page.ID
	row, err := s.d.Repos.Watchers.Watch(ctx, &model.Watcher{
		TenantID: d.Page.TenantID, UserID: actorID(actor),
		SpaceID: d.Page.SpaceID, PageID: &pageID, Reason: model.WatchManual,
	}, notify.Upgrades)
	if err != nil {
		return nil, err
	}
	return watchView(row), nil
}

// MutePage silences a page without giving up watching it.
func (s *PageService) MutePage(ctx context.Context, actor *acl.Identity, d acl.Decision,
	muted bool,
) (*WatchView, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Watchers == nil {
		return nil, notFound("page")
	}

	err := s.d.Repos.Watchers.SetMuted(ctx, d.Page.TenantID, actorID(actor), d.Page.ID, muted)
	if errors.Is(err, repository.ErrNotFound) {
		// Muting a page nobody had chosen to watch is a reasonable thing to
		// ask for: it means "and do not start telling me about this either".
		pageID := d.Page.ID
		row, createErr := s.d.Repos.Watchers.Watch(ctx, &model.Watcher{
			TenantID: d.Page.TenantID, UserID: actorID(actor),
			SpaceID: d.Page.SpaceID, PageID: &pageID, Reason: model.WatchManual,
		}, notify.Upgrades)
		if createErr != nil {
			return nil, createErr
		}
		if muted {
			if err := s.d.Repos.Watchers.SetMuted(ctx, d.Page.TenantID, actorID(actor),
				d.Page.ID, true); err != nil {
				return nil, err
			}
			at := time.Now().UTC()
			row.MutedAt = &at
		}
		return watchView(row), nil
	}
	if err != nil {
		return nil, err
	}

	row, err := s.d.Repos.Watchers.Get(ctx, d.Page.TenantID, actorID(actor), d.Page.ID)
	if err != nil {
		return nil, err
	}
	return watchView(row), nil
}

// WatchState reports how this caller is related to a page.
func (s *PageService) WatchState(ctx context.Context, actor *acl.Identity, d acl.Decision) (
	*WatchView, error,
) {
	if s.d.Repos.Watchers == nil {
		return &WatchView{PageID: d.Page.ID}, nil
	}
	row, err := s.d.Repos.Watchers.Get(ctx, d.Page.TenantID, actorID(actor), d.Page.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return &WatchView{PageID: d.Page.ID}, nil
	}
	if err != nil {
		return nil, err
	}
	return watchView(row), nil
}

func watchView(row *model.Watcher) *WatchView {
	view := &WatchView{Reason: row.Reason, Muted: row.MutedAt != nil, Watched: true}
	if row.PageID != nil {
		view.PageID = *row.PageID
	}
	return view
}

// autoWatch enrols somebody as a watcher because of something they did.
//
// Best-effort throughout: failing to record a watch must not fail the comment
// that caused it, and the next thing they do enrols them again.
func (b *base) autoWatch(ctx context.Context, page *model.Page, userID, action string) {
	if b.d.Repos.Watchers == nil || page == nil || userID == "" {
		return
	}
	reason, ok := notify.AutoWatch(action)
	if !ok {
		return
	}
	pageID := page.ID
	if _, err := b.d.Repos.Watchers.Watch(ctx, &model.Watcher{
		TenantID: page.TenantID, UserID: userID, SpaceID: page.SpaceID,
		PageID: &pageID, Reason: reason,
	}, notify.Upgrades); err != nil {
		logger.Warnf(ctx, "[docs] recording a watcher of page %s failed: %v", page.ID, err)
	}
}

// ---- telling people ------------------------------------------------------------

// notification is one row about to be written, before the merge decision.
type notification struct {
	kind      notify.Kind
	page      *model.Page
	actorID   string
	commentID string
	payload   map[string]any
}

// deliver writes one notification per recipient, folding into an existing one
// where the rules say two events are really one.
func (b *base) deliver(ctx context.Context, n notification, recipients []string) {
	if b.d.Repos.Notices == nil || n.page == nil || len(recipients) == 0 {
		return
	}
	// The short id goes into every payload here rather than at each call
	// site: it is how a client turns a notification into a link, and one
	// forgotten copy would produce rows that cannot be opened.
	if n.payload == nil {
		n.payload = map[string]any{}
	}
	n.payload["short_id"] = n.page.ShortID
	// And the space's slug, because a page's URL is built from both. Looked
	// up here rather than carried through every call site; a notification is
	// written rarely enough for one read to be beside the point.
	if b.d.Repos.Spaces != nil {
		if space, err := b.d.Repos.Spaces.Get(ctx, n.page.TenantID, n.page.SpaceID); err == nil {
			n.payload["space_slug"] = space.Slug
		}
	}

	payload, err := json.Marshal(n.payload)
	if err != nil {
		payload = []byte("{}")
	}
	now := time.Now().UTC()
	window := notify.MergeWindow(n.kind)

	for _, userID := range recipients {
		if merged := b.mergeOrCreate(ctx, n, userID, payload, window, now); merged != "" {
			b.publish(ctx, events.New(events.NotificationCreated, n.page.TenantID).
				WithSpace(n.page.SpaceID).WithPage(n.page.ID).WithActor(n.actorID).
				With("notification_id", merged).With("user_id", userID).
				With("kind", string(n.kind)))
		}
	}
}

// mergeOrCreate returns the id of the notification the event landed in, or
// empty when nothing could be written.
func (b *base) mergeOrCreate(ctx context.Context, n notification, userID string,
	payload []byte, window time.Duration, now time.Time,
) string {
	if window > 0 {
		recent, err := b.d.Repos.Notices.RecentFor(ctx, n.page.TenantID, userID, n.page.ID,
			now.Add(-window))
		if err == nil && len(recent) > 0 {
			existing := make([]notify.Existing, 0, len(recent))
			for _, row := range recent {
				pageID := ""
				if row.PageID != nil {
					pageID = *row.PageID
				}
				existing = append(existing, notify.Existing{
					ID: row.ID, Kind: notify.Kind(row.Kind), PageID: pageID,
					CreatedAt: row.CreatedAt, ReadAt: row.ReadAt,
				})
			}
			candidate := notify.Candidate{
				Kind: n.kind, UserID: userID, PageID: n.page.ID, ActorID: n.actorID,
			}
			if into := notify.MergeInto(candidate, existing, now); into != nil {
				if err := b.d.Repos.Notices.Touch(ctx, n.page.TenantID, into.ID,
					model.JSON(payload)); err != nil {
					logger.Warnf(ctx, "[docs] folding a notification failed: %v", err)
					return ""
				}
				return into.ID
			}
		}
	}

	row := &model.Notification{
		TenantID: n.page.TenantID, UserID: userID, Kind: string(n.kind),
		Payload: model.JSON(payload),
	}
	if n.actorID != "" {
		actor := n.actorID
		row.ActorID = &actor
	}
	spaceID, pageID := n.page.SpaceID, n.page.ID
	row.SpaceID, row.PageID = &spaceID, &pageID
	if n.commentID != "" {
		commentID := n.commentID
		row.CommentID = &commentID
	}
	if err := b.d.Repos.Notices.Create(ctx, row); err != nil {
		logger.Warnf(ctx, "[docs] writing a notification failed: %v", err)
		return ""
	}
	return row.ID
}

// notifyComment tells the right people about a new comment or reply.
func (b *base) notifyComment(ctx context.Context, page *model.Page, comment *model.Comment,
	repliedToID, excerpt string, mentioned []string,
) {
	if b.d.Repos.Watchers == nil || page == nil {
		return
	}
	watchers := b.watcherRules(ctx, page)
	author := ""
	if page.CreatorID != nil {
		author = *page.CreatorID
	}

	// Being named comes first and is never merged, so somebody both watching
	// the page and named in the comment gets the more specific notice.
	named := notify.MentionAudience(mentioned, comment.CreatorID)
	b.deliver(ctx, notification{
		kind: notify.Mentioned, page: page, actorID: comment.CreatorID,
		commentID: comment.ID,
		payload:   map[string]any{"title": page.Title, "excerpt": excerpt},
	}, named)

	// Everybody else hears about the comment itself.
	audience := notify.CommentAudience(watchers, author, repliedToID, comment.CreatorID)
	b.deliver(ctx, notification{
		kind: notify.Commented, page: page, actorID: comment.CreatorID,
		commentID: comment.ID,
		payload:   map[string]any{"title": page.Title, "excerpt": excerpt},
	}, without(audience, named))
}

// notifyPageUpdated tells the people who chose to watch a page that its body
// changed.
func (b *base) notifyPageUpdated(ctx context.Context, page *model.Page, actorID string) {
	if b.d.Repos.Watchers == nil || page == nil || actorID == "" {
		return
	}
	audience := notify.UpdateAudience(b.watcherRules(ctx, page), actorID)
	b.deliver(ctx, notification{
		kind: notify.PageUpdated, page: page, actorID: actorID,
		payload: map[string]any{"title": page.Title},
	}, audience)
}

// notifyMentionedInPage tells people named in the body of a page.
//
// The same rule as a mention in a comment: a direct address, never merged,
// and not silenced by muting the page. Mentioning somebody also enrols them
// as a watcher, since they have been brought into it.
func (b *base) notifyMentionedInPage(ctx context.Context, page *model.Page,
	mentioned []string, actorID string,
) {
	if page == nil || len(mentioned) == 0 {
		return
	}
	audience := notify.MentionAudience(mentioned, actorID)
	for _, userID := range audience {
		b.autoWatch(ctx, page, userID, "mention")
	}
	b.deliver(ctx, notification{
		kind: notify.Mentioned, page: page, actorID: actorID,
		payload: map[string]any{"title": page.Title},
	}, audience)
}

// watcherRules reads a page's watchers into the shape the rules take.
func (b *base) watcherRules(ctx context.Context, page *model.Page) []notify.Watcher {
	rows, err := b.d.Repos.Watchers.ForPage(ctx, page.TenantID, page.ID)
	if err != nil {
		logger.Warnf(ctx, "[docs] reading the watchers of page %s failed: %v", page.ID, err)
		return nil
	}
	out := make([]notify.Watcher, 0, len(rows))
	for _, row := range rows {
		out = append(out, notify.Watcher{
			UserID: row.UserID,
			Muted:  row.MutedAt != nil,
			Manual: row.Reason == model.WatchManual,
		})
	}
	return out
}

// without removes the second list from the first, keeping order.
func without(all, remove []string) []string {
	if len(remove) == 0 {
		return all
	}
	drop := make(map[string]bool, len(remove))
	for _, id := range remove {
		drop[id] = true
	}
	out := make([]string, 0, len(all))
	for _, id := range all {
		if !drop[id] {
			out = append(out, id)
		}
	}
	return out
}

// ---- reading an inbox ----------------------------------------------------------

// Notifications returns one person's inbox. It is theirs, so no page
// permission is consulted: the row says what happened, not what a page says.
func (s *PageService) Notifications(ctx context.Context, actor *acl.Identity,
	unreadOnly bool, cursor string, limit int,
) (*NotificationPage, error) {
	out := &NotificationPage{Items: []*NotificationView{}}
	if s.d.Repos.Notices == nil {
		return out, nil
	}
	after, err := decodeNotificationCursor(cursor)
	if err != nil {
		return nil, invalid("cursor: %v", err)
	}

	rows, err := s.d.Repos.Notices.ListFor(ctx, actor.TenantID, actor.UserID, unreadOnly, after, limit)
	if err != nil {
		return nil, err
	}
	unread, err := s.d.Repos.Notices.CountUnread(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return nil, err
	}
	out.Unread = unread

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ActorID != nil {
			ids = append(ids, *row.ActorID)
		}
	}
	users := s.users(ctx, dedupe(ids))

	for _, row := range rows {
		view := &NotificationView{
			ID: row.ID, Kind: notify.Kind(row.Kind),
			Payload: json.RawMessage(row.Payload), ReadAt: row.ReadAt, CreatedAt: row.CreatedAt,
		}
		if row.PageID != nil {
			view.PageID = *row.PageID
		}
		if row.SpaceID != nil {
			view.SpaceID = *row.SpaceID
		}
		if row.CommentID != nil {
			view.CommentID = *row.CommentID
		}
		if row.ActorID != nil {
			view.ActorID = *row.ActorID
			view.Actor = userView(*row.ActorID, users)
		}
		out.Items = append(out.Items, view)
	}
	if len(rows) > 0 && (limit <= 0 || len(rows) >= limit) {
		last := rows[len(rows)-1]
		out.NextCursor = encodeNotificationCursor(repository.NotificationCursor{
			CreatedAt: last.CreatedAt, ID: last.ID,
		})
	}
	return out, nil
}

// MarkNotificationsRead marks some of somebody's notifications read, or all.
func (s *PageService) MarkNotificationsRead(ctx context.Context, actor *acl.Identity,
	ids []string,
) (int64, error) {
	if s.d.Repos.Notices == nil {
		return 0, nil
	}
	return s.d.Repos.Notices.MarkRead(ctx, actor.TenantID, actor.UserID, ids)
}

// ArchiveNotifications hides notifications from an inbox.
func (s *PageService) ArchiveNotifications(ctx context.Context, actor *acl.Identity,
	ids []string,
) (int64, error) {
	if s.d.Repos.Notices == nil {
		return 0, nil
	}
	return s.d.Repos.Notices.Archive(ctx, actor.TenantID, actor.UserID, ids)
}

func encodeNotificationCursor(c repository.NotificationCursor) string {
	raw, err := json.Marshal(map[string]any{
		"at": c.CreatedAt.UTC().Format(time.RFC3339Nano), "id": c.ID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeNotificationCursor(value string) (*repository.NotificationCursor, error) {
	if value == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, errors.New("malformed")
	}
	var fields struct {
		At string `json:"at"`
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, errors.New("malformed")
	}
	at, err := time.Parse(time.RFC3339Nano, fields.At)
	if err != nil || fields.ID == "" {
		return nil, errors.New("malformed")
	}
	return &repository.NotificationCursor{CreatedAt: at, ID: fields.ID}, nil
}
