package service

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Locking a page, and its publication state.
//
// Both of these were half-built before T4.4's contract audit found them: the
// page row has carried is_locked since T0.2, the permission
// resolver has capped a locked page at reader since T0.3, and the audit
// vocabulary has had the four action names since T0.5 — but nothing could
// ever set either field. A permission rule that no code path can trigger is
// worse than a missing feature, because it reads as working.
//
// ---- decision: locking is an administrator's act, unlocking too
//
// A lock says "this is settled, stop editing it". If any writer could set it,
// a disagreement about whether a page is finished would be settled by
// whoever clicked first; and if the person who locked it were the only one
// who could unlock it, a lock would outlive them leaving the space. Space
// admins can do both, which matches the resolver: they are the ones a lock
// does not apply to, so they are the only ones who can still act on the page
// afterwards anyway.

// SetLocked locks a page against editing, or unlocks it.
//
// The resolver caps everybody except space admins at reader on a locked page,
// so this does not need to evict editors by itself — but it does, because a
// collaborative session already open would otherwise keep writing until its
// next permission refresh.
func (s *PageService) SetLocked(ctx context.Context, actor *acl.Identity, d acl.Decision,
	locked bool,
) (*PageView, error) {
	if err := requireRole(d, model.RoleAdmin); err != nil {
		return nil, err
	}
	if d.Page.IsLocked == locked {
		return s.view(ctx, d)
	}
	if err := s.d.Repos.Pages.UpdateMeta(ctx, actor.TenantID, d.Page.ID,
		map[string]any{"is_locked": locked}); err != nil {
		return nil, err
	}
	// Every open client is holding a permission answer that has just become
	// wrong, and an editor mid-sentence is the case this is for.
	s.invalidate(ctx, actor.TenantID)
	if locked {
		s.evict(ctx, d.Page.ID)
	}

	action := audit.PageUnlocked
	if locked {
		action = audit.PageLocked
	}
	s.recordPageMeta(ctx, actor, d.Page, action, "locked", locked)
	return s.resolve(ctx, actor, d.Page.ID)
}

// SetKnowledgeExcluded keeps a page out of its space's knowledge base, or lets it
// take part again.
//
// This is not a permission: everybody who could read the page can still read it,
// and this is deliberate. It decides only whether the page may be quoted in AI
// answers, which is a different question from who may read it: meeting notes and
// scratch pages are readable and still should not answer somebody's question.
// The restriction of a page is the other way to keep it out, and stronger: a
// restricted page never reaches the knowledge base whatever this says.
func (s *PageService) SetKnowledgeExcluded(ctx context.Context, actor *acl.Identity, d acl.Decision,
	excluded bool,
) (*PageView, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	if d.Page.ExcludeFromKnowledge == excluded {
		return s.view(ctx, d)
	}
	if err := s.d.Repos.Pages.UpdateMeta(ctx, actor.TenantID, d.Page.ID,
		map[string]any{"exclude_from_knowledge": excluded}); err != nil {
		return nil, err
	}
	action := audit.PageKnowledgeIncluded
	if excluded {
		action = audit.PageKnowledgeExcluded
	}
	s.recordPageMeta(ctx, actor, d.Page, action, "exclude_from_knowledge", excluded)
	return s.resolve(ctx, actor, d.Page.ID)
}

// recordPageMeta audits a metadata change and announces it.
func (s *PageService) recordPageMeta(ctx context.Context, actor *acl.Identity, page *model.Page,
	action types.AuditAction, key string, value any,
) {
	s.audit(ctx, audit.Entry{
		TenantID: page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: action, SpaceID: page.SpaceID,
		TargetType: audit.TargetPage, TargetID: page.ID,
	})
	s.publishPageMeta(ctx, actor, page, key, value)
}

func (s *PageService) publishPageMeta(ctx context.Context, actor *acl.Identity, page *model.Page,
	key string, value any,
) {
	s.publish(ctx, events.New(events.PageMeta, page.TenantID).
		WithSpace(page.SpaceID).WithPage(page.ID).WithActor(actorID(actor)).
		With(key, value))
}
