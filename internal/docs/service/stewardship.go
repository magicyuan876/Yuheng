package service

import (
	"context"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Who maintains a page.
//
// A page has a maintainer — its creator until they hand it over — and it is the
// person knowledge health takes the problems of the page's mirror entry to: an
// overdue review, an answer people disputed, a copy of it elsewhere. It is not
// a permission; the page's role decides who may change it, and the maintainer
// must be one of those people.
//
// The maintainer is kept on the page rather than on the mirror entry because a
// page outlives its entry: excluding a page from the knowledge base drops the
// entry, and including it again makes a new one, which must come back with the
// maintainer the page had. The entry carries a copy, written when the page is
// synchronised (syncStewardship).

// canChangeOwner reports whether the caller of the request may hand the page
// over: its maintainer, or an administrator of it. Handing over somebody
// else's responsibility is not a writer's to do.
func canChangeOwner(ctx context.Context, d acl.Decision) bool {
	if d.Page == nil || !d.Role.AtLeast(model.RoleWriter) {
		return false
	}
	if d.Role == model.RoleAdmin {
		return true
	}
	uid, ok := types.UserIDFromContext(ctx)
	return ok && uid != "" && uid == d.Page.Steward()
}

// steward names the page's maintainer for its view. Nil when the page has
// none, or the directory is unavailable or fails: the name is a courtesy, and
// the ID is in the view regardless.
func (s *PageService) steward(ctx context.Context, page *model.Page) *UserView {
	id := page.Steward()
	if id == "" || s.d.Users == nil {
		return nil
	}
	users, err := s.d.Users.GetUsersByIDs(ctx, []string{id})
	if err != nil {
		logger.Warnf(ctx, "[docs] naming the maintainer of page %s failed: %v", page.ID, err)
		return nil
	}
	v := userView(id, users)
	return &v
}

// SetOwner hands a page to another maintainer, who must be able to write it.
// Allowed on a locked page: who maintains a page is not its content.
func (s *PageService) SetOwner(ctx context.Context, actor *acl.Identity, d acl.Decision, ownerID string,
) (*PageView, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if d.Role != model.RoleAdmin && actorID(actor) != d.Page.Steward() {
		return nil, forbidden("only the page's maintainer or an administrator of it can hand it over")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, invalid("owner_id is required: a page always has a maintainer")
	}
	if ownerID == d.Page.Steward() {
		return s.view(ctx, d)
	}
	target, err := s.d.Resolver.Identity(ctx, actor.TenantID, ownerID)
	if err != nil {
		return nil, err
	}
	if !target.Member {
		return nil, invalid("the new maintainer must be a member of the workspace")
	}
	targetDecision, err := s.d.Resolver.Decide(ctx, target, d.Page)
	if err != nil {
		return nil, err
	}
	if !targetDecision.Allows(model.RoleWriter) {
		return nil, invalid("the new maintainer must be able to edit the page")
	}
	if err := s.d.Repos.Pages.UpdateMeta(ctx, actor.TenantID, d.Page.ID,
		map[string]any{"owner_id": ownerID}); err != nil {
		return nil, err
	}
	s.recordPageMeta(ctx, actor, d.Page, audit.PageOwnerChanged, "owner_id", ownerID)
	// The mirror entry takes the new maintainer the next time the page is
	// synchronised; asking for that now makes it immediate, through the
	// queue that retries it should the knowledge base be unreachable.
	if err := s.QueueIndex(ctx, d.Page.TenantID, d.Page.ID, false, 0); err != nil {
		logger.Warnf(ctx, "[docs] queueing page %s after its hand-over failed: %v", d.Page.ID, err)
	}
	return s.resolve(ctx, actor, d.Page.ID)
}

// syncStewardship copies the page's stewardship onto its mirror entry: the
// maintainer, and the last person to change the content as the entry's last
// review. Whoever last rewrote a page has vouched for what it says.
func (s *PageService) syncStewardship(ctx context.Context, page *model.Page, knowledgeID string) error {
	reviewedBy := ""
	if page.LastEditorID != nil {
		reviewedBy = *page.LastEditorID
	} else if page.CreatorID != nil {
		reviewedBy = *page.CreatorID
	}
	reviewedAt := page.CreatedAt
	if page.ContentUpdatedAt != nil {
		reviewedAt = *page.ContentUpdatedAt
	}
	if reviewedBy == "" {
		reviewedAt = time.Time{}
	}
	return s.d.Knowledge.SyncStewardship(ctx, page.TenantID, knowledgeID, page.Steward(), reviewedBy, reviewedAt)
}

// ConfirmReviewed records that the caller, a writer of the page, vouches for
// it as it stands: its mirror entry's review clock restarts, and the problems
// that only asked for a look are settled at the check that follows. It is
// recorded on the entry, where the review clock lives; a page excluded from
// the knowledge base has no entry and nothing to confirm.
func (s *PageService) ConfirmReviewed(ctx context.Context, actor *acl.Identity, d acl.Decision) error {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return err
	}
	page := d.Page
	if s.d.Knowledge == nil || page.KnowledgeID == nil || *page.KnowledgeID == "" {
		return conflict("the page is not in a knowledge base, so there is nothing to confirm")
	}
	if err := s.d.Knowledge.SyncStewardship(ctx, page.TenantID, *page.KnowledgeID, page.Steward(), actorID(actor),
		time.Now().UTC()); err != nil {
		return err
	}
	s.audit(ctx, audit.Entry{
		TenantID: page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.PageReviewed, SpaceID: page.SpaceID, TargetType: audit.TargetPage, TargetID: page.ID,
	})
	return nil
}
