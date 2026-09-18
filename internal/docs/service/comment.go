package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/comment"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Comments.
//
// What a comment may say and where it may point are decided in
// internal/docs/comment, free of storage. What is here is who may do what,
// which is the part with the surprising rule in it:
//
//	**A reader may comment.** Commenting is not editing. Somebody invited to
//	review a page can say what they think of it without being given the
//	ability to change it, and a product where the only way to raise a point is
//	to be handed write access is one where review does not happen. This is the
//	work package's own acceptance criterion, and it is why every write below
//	takes RoleReader rather than RoleWriter.
//
// The other rules follow from that one: a remark belongs to whoever made it,
// so only its author edits it; a thread belongs to the page, so a writer may
// resolve anybody's; and a space admin may remove a comment they did not
// write, because moderation has to be possible.

// CommentView is one comment as a client sees it.
type CommentView struct {
	ID       string            `json:"id"`
	PageID   string            `json:"page_id"`
	ParentID string            `json:"parent_id,omitempty"`
	Body     json.RawMessage   `json:"body"`
	Anchor   json.RawMessage   `json:"anchor,omitempty"`
	Quoted   string            `json:"quoted_text,omitempty"`
	Place    comment.Placement `json:"placement"`

	Creator   UserView   `json:"creator,omitzero"`
	CreatorID string     `json:"creator_id"`
	CreatedAt time.Time  `json:"created_at"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`

	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy   string     `json:"resolved_by,omitempty"`
	ResolvedUser UserView   `json:"resolved_user,omitzero"`

	// Replies are the comments answering this one, oldest first. Only a
	// top-level comment has them.
	Replies []*CommentView `json:"replies,omitempty"`

	// CanEdit and CanDelete say what this caller may do, so a client does not
	// have to reimplement the rules above and disagree with them.
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
	// CanResolve is set on a thread, never on a reply.
	CanResolve bool `json:"can_resolve"`
}

// CommentList is a page's threads plus the counts a badge needs.
type CommentList struct {
	Items []*CommentView `json:"items"`
	Open  int64          `json:"open"`
	Total int64          `json:"total"`
}

// CreateCommentInput is a new comment or reply.
type CreateCommentInput struct {
	Body json.RawMessage
	// Anchor is the editor's relative position; absent makes a page comment.
	Anchor json.RawMessage
	// QuotedText is what the anchor covered when it was made, kept so the
	// comment can still be placed when the position stops resolving.
	QuotedText string
	// ParentID makes this a reply. A reply may not have an anchor of its own
	// and may not answer another reply.
	ParentID string
}

// MaxExcerptRunes bounds the piece of a comment carried in a notification.
// Enough to know what it is about; short enough that an inbox is not a copy
// of the discussion.
const MaxExcerptRunes = 140

// excerptOf shortens a comment for a notification.
func excerptOf(text string) string {
	runes := []rune(text)
	if len(runes) <= MaxExcerptRunes {
		return text
	}
	return string(runes[:MaxExcerptRunes]) + "…"
}

// MaxCommentsPerPage bounds one page's threads. Generous for a review, and a
// bound rather than none, because every reader of the page loads all of them.
const MaxCommentsPerPage = 1000

// ---- writing -----------------------------------------------------------------

// CreateComment adds a comment or a reply to a page.
func (s *PageService) CreateComment(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in CreateCommentInput,
) (*CommentView, error) {
	// Reader, not writer: see the note at the top of this file.
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Comments == nil {
		return nil, notFound("comment")
	}

	body, text, err := comment.ParseBody(in.Body)
	if err != nil {
		return nil, invalid("%v", err)
	}

	row := &model.Comment{
		TenantID: d.Page.TenantID, SpaceID: d.Page.SpaceID, PageID: d.Page.ID,
		Body: model.JSON(in.Body), CreatorID: actorID(actor),
	}

	repliedToID := ""
	if in.ParentID != "" {
		parent, err := s.loadComment(ctx, d, in.ParentID)
		if err != nil {
			return nil, err
		}
		repliedToID = parent.CreatorID
		if parent.ParentID != nil {
			// One level, so a thread is always a remark and its answers. A
			// deeper tree has nowhere sensible to be drawn.
			return nil, invalid("a reply cannot be answered; reply to the comment it belongs to")
		}
		row.ParentID = &parent.ID
		// A reply is about the thread, not about a passage of its own.
	} else if len(in.Anchor) > 0 {
		if _, err := comment.ParseAnchor(in.Anchor); err != nil && !errors.Is(err, comment.ErrNoAnchor) {
			return nil, invalid("%v", err)
		} else if err == nil {
			row.Anchor = model.JSON(in.Anchor)
			if quoted := comment.CleanQuotedText(in.QuotedText); quoted != "" {
				row.QuotedText = &quoted
			}
		}
	}

	if _, total, err := s.d.Repos.Comments.CountForPage(ctx, d.Page.TenantID, d.Page.ID); err == nil {
		if total >= MaxCommentsPerPage {
			return nil, invalid("this page already has %d comment threads", MaxCommentsPerPage)
		}
	}

	if err := s.d.Repos.Comments.Create(ctx, row); err != nil {
		return nil, err
	}

	s.publishComment(ctx, d, row.ID, "created", actor)
	// Deliberately not audited, matching the action vocabulary T0.5 declared:
	// a comment is its own visible record, signed and dated on the page, so an
	// audit row would say what anybody can already read. Deleting one removes
	// that record, which is why only the deletion is audited.

	// Commenting enrols you as a watcher of the page, and so does being named
	// in one: both mean you are now part of the conversation.
	mentioned := comment.Mentions(body)
	s.autoWatch(ctx, d.Page, actorID(actor), "comment")
	for _, userID := range mentioned {
		s.autoWatch(ctx, d.Page, userID, "mention")
	}
	s.notifyComment(ctx, d.Page, row, repliedToID, excerptOf(text), mentioned)

	return s.commentView(ctx, actor, d, row, nil), nil
}

// UpdateComment replaces what a comment says.
//
// Only its author, whatever else they can do to the page: a remark belongs to
// whoever made it, and an admin rewriting somebody's words while their name
// stays on them is not moderation.
func (s *PageService) UpdateComment(ctx context.Context, actor *acl.Identity, d acl.Decision,
	commentID string, body json.RawMessage,
) (*CommentView, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	row, err := s.loadComment(ctx, d, commentID)
	if err != nil {
		return nil, err
	}
	if row.CreatorID != actorID(actor) {
		return nil, forbidden("only the author of a comment may edit it")
	}
	if _, _, err := comment.ParseBody(body); err != nil {
		return nil, invalid("%v", err)
	}

	if err := s.d.Repos.Comments.UpdateBody(ctx, d.Page.TenantID, commentID, model.JSON(body)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("comment")
		}
		return nil, err
	}
	s.publishComment(ctx, d, commentID, "updated", actor)

	fresh, err := s.loadComment(ctx, d, commentID)
	if err != nil {
		return nil, err
	}
	return s.commentView(ctx, actor, d, fresh, nil), nil
}

// ResolveComment marks a thread settled, or reopens it.
//
// A thread belongs to the page rather than to whoever started it, so anybody
// who may write the page can settle one — and its author can too, since
// withdrawing your own point should not need permission you do not have.
func (s *PageService) ResolveComment(ctx context.Context, actor *acl.Identity, d acl.Decision,
	commentID string, resolved bool,
) (*CommentView, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	row, err := s.loadComment(ctx, d, commentID)
	if err != nil {
		return nil, err
	}
	if row.ParentID != nil {
		return nil, invalid("a reply is resolved with the thread it belongs to")
	}
	if !canResolveComment(d, row, actorID(actor)) {
		return nil, forbidden("you may not resolve this thread")
	}

	var at *time.Time
	by := ""
	if resolved {
		moment := time.Now().UTC()
		at, by = &moment, actorID(actor)
	}
	if err := s.d.Repos.Comments.SetResolved(ctx, d.Page.TenantID, commentID, at, by); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("comment")
		}
		return nil, err
	}
	s.publishComment(ctx, d, commentID, map[bool]string{true: "resolved", false: "reopened"}[resolved], actor)

	fresh, err := s.loadComment(ctx, d, commentID)
	if err != nil {
		return nil, err
	}
	return s.commentView(ctx, actor, d, fresh, nil), nil
}

// DeleteComment removes a comment and, with it, its replies.
//
// Its author may, and so may a space admin — moderation has to be possible —
// but nobody in between.
func (s *PageService) DeleteComment(ctx context.Context, actor *acl.Identity, d acl.Decision,
	commentID string,
) error {
	if err := requireRole(d, model.RoleReader); err != nil {
		return err
	}
	row, err := s.loadComment(ctx, d, commentID)
	if err != nil {
		return err
	}
	if !canDeleteComment(d, row, actorID(actor)) {
		return forbidden("you may not delete this comment")
	}

	if _, err := s.d.Repos.Comments.SoftDelete(ctx, d.Page.TenantID, commentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound("comment")
		}
		return err
	}
	s.publishComment(ctx, d, commentID, "deleted", actor)
	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.CommentDeleted, SpaceID: d.Page.SpaceID,
		TargetType: audit.TargetComment, TargetID: commentID,
	})
	return nil
}

// ---- reading -----------------------------------------------------------------

// Comments lists a page's threads, oldest first, replies nested one level.
func (s *PageService) Comments(ctx context.Context, actor *acl.Identity, d acl.Decision,
	includeResolved bool,
) (*CommentList, error) {
	out := &CommentList{Items: []*CommentView{}}
	if s.d.Repos.Comments == nil {
		return out, nil
	}
	rows, err := s.d.Repos.Comments.ListForPage(ctx, d.Page.TenantID, d.Page.ID, includeResolved)
	if err != nil {
		return nil, err
	}
	open, total, err := s.d.Repos.Comments.CountForPage(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	out.Open, out.Total = open, total

	// Everybody named across the page in one lookup: a thread is mostly the
	// same few people, and one request per comment would be absurd.
	ids := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		ids = append(ids, row.CreatorID)
		if row.ResolvedBy != nil {
			ids = append(ids, *row.ResolvedBy)
		}
	}
	users := s.users(ctx, dedupe(ids))

	byID := make(map[string]*CommentView, len(rows))
	for _, row := range rows {
		byID[row.ID] = s.commentView(ctx, actor, d, row, users)
	}
	for _, row := range rows {
		view := byID[row.ID]
		if row.ParentID == nil {
			out.Items = append(out.Items, view)
			continue
		}
		// A reply whose parent is not in this listing — because the thread is
		// resolved and resolved threads were asked to be left out — is left
		// out with it rather than floated to the top as an orphan.
		if parent, ok := byID[*row.ParentID]; ok {
			parent.Replies = append(parent.Replies, view)
		}
	}
	return out, nil
}

// ---- rules -------------------------------------------------------------------

// canResolveComment: anybody who may write the page, or the author.
func canResolveComment(d acl.Decision, row *model.Comment, actorID string) bool {
	if row.CreatorID == actorID {
		return true
	}
	return d.Role.AtLeast(model.RoleWriter)
}

// canDeleteComment: the author, or a space admin moderating.
func canDeleteComment(d acl.Decision, row *model.Comment, actorID string) bool {
	if row.CreatorID == actorID {
		return true
	}
	return d.Role.AtLeast(model.RoleAdmin)
}

// ---- helpers -----------------------------------------------------------------

// loadComment reads a comment and checks it belongs to the page the caller
// was granted access to. Naming another page's comment must not reach it.
func (s *PageService) loadComment(ctx context.Context, d acl.Decision, commentID string) (
	*model.Comment, error,
) {
	if s.d.Repos.Comments == nil || commentID == "" {
		return nil, notFound("comment")
	}
	row, err := s.d.Repos.Comments.Get(ctx, d.Page.TenantID, commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("comment")
		}
		return nil, err
	}
	if row.PageID != d.Page.ID {
		return nil, notFound("comment")
	}
	return row, nil
}

func (s *PageService) commentView(ctx context.Context, actor *acl.Identity, d acl.Decision,
	row *model.Comment, users map[string]*types.User,
) *CommentView {
	if users == nil {
		ids := []string{row.CreatorID}
		if row.ResolvedBy != nil {
			ids = append(ids, *row.ResolvedBy)
		}
		users = s.users(ctx, dedupe(ids))
	}
	me := actorID(actor)

	view := &CommentView{
		ID: row.ID, PageID: row.PageID,
		Body:      json.RawMessage(row.Body),
		Place:     comment.PlacementOf(row.Anchor),
		CreatorID: row.CreatorID, Creator: userView(row.CreatorID, users),
		CreatedAt: row.CreatedAt, EditedAt: row.EditedAt,
		ResolvedAt: row.ResolvedAt,
		CanEdit:    row.CreatorID == me,
		CanDelete:  canDeleteComment(d, row, me),
	}
	if row.ParentID != nil {
		view.ParentID = *row.ParentID
	} else {
		view.CanResolve = canResolveComment(d, row, me)
	}
	if len(row.Anchor) > 0 {
		view.Anchor = json.RawMessage(row.Anchor)
	}
	if row.QuotedText != nil {
		view.Quoted = *row.QuotedText
	}
	if row.ResolvedBy != nil {
		view.ResolvedBy = *row.ResolvedBy
		view.ResolvedUser = userView(*row.ResolvedBy, users)
	}
	return view
}

// publishComment tells the page's readers something changed.
//
// One event type for every change, carrying the comment's id and what
// happened: a client reloads the thread list either way, and a stream of
// finely-named events nobody switches on is a contract to maintain for
// nothing.
func (s *PageService) publishComment(ctx context.Context, d acl.Decision, commentID, action string,
	actor *acl.Identity,
) {
	s.publish(ctx, events.New(events.CommentChanged, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("comment_id", commentID).With("action", action))
}
