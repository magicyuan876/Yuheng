package service

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Superseding a page: when two documents say nearly the same thing, keeping
// one and taking the other out of the knowledge base.
//
// A page is never deleted for it. It is excluded from the knowledge base — the
// same flag its writers use to keep meeting notes out of AI answers — and
// marked with what replaced it, so that a reader who still opens it (from a
// bookmark, a link in an old chat) learns that it has been replaced, and by
// what. Letting the page back into the knowledge base clears the mark.
//
// The exclusion drops the page's mirror entry at its next synchronisation,
// which is queued at once, and with the entry go the findings that named it.
//
// Two ways in, one rule: whoever supersedes a page must be able to change it.
//
//   - From the page that stays (SupersedeFromPage): the person who just wrote
//     the new version sees that it nearly repeats an older page and says "this
//     replaces it", which is the moment they know best what they meant.
//   - From the knowledge base's health view, through the retirer the module
//     registers for page mirrors (RetireMirror).

// SupersedeFromPage settles a finding of the page d by keeping d and taking
// the other document out of the knowledge base. The other document must be a
// page the caller can edit; an uploaded file is the knowledge base's editors'
// to remove, from its health view.
func (s *PageService) SupersedeFromPage(ctx context.Context, actor *acl.Identity, d acl.Decision,
	findingID string,
) (*types.SupersedeResult, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	page := d.Page
	if s.d.Findings == nil || page.KnowledgeID == nil || *page.KnowledgeID == "" {
		return nil, notFound("finding")
	}
	findings, err := s.d.Findings.OpenForKnowledge(ctx, page.TenantID, *page.KnowledgeID)
	if err != nil {
		return nil, err
	}
	var finding *types.KnowledgeFindingView
	for _, f := range findings {
		if f.ID == findingID {
			finding = f
		}
	}
	if finding == nil || finding.Related == nil {
		return nil, notFound("finding")
	}
	others, err := s.d.Repos.Pages.GetByKnowledgeIDs(ctx, page.TenantID, []string{finding.Related.KnowledgeID})
	if err != nil {
		return nil, err
	}
	if len(others) == 0 {
		return nil, conflict("the other document is not a page; " +
			"the knowledge base's editors remove it from its health view")
	}
	other := others[0]
	od, err := s.d.Resolver.Decide(ctx, actor, other)
	if err != nil {
		return nil, err
	}
	replacement := model.SupersededBy{
		KnowledgeID: *page.KnowledgeID, Title: page.Title, PageID: page.ID, By: actorID(actor),
	}
	if err := s.retire(ctx, actor, od, replacement); err != nil {
		return nil, err
	}
	return &types.SupersedeResult{RetiredKnowledgeID: finding.Related.KnowledgeID, How: types.RetiredExcluded}, nil
}

// RetireMirror takes the page mirrored by knowledgeID out of the knowledge
// base on behalf of the person in ctx, marking it superseded by replacement.
// It is the page side of interfaces.KnowledgeRetirer.
func (s *PageService) RetireMirror(ctx context.Context, tenantID uint64, knowledgeID string,
	replacement types.KnowledgeRef,
) error {
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || types.IsSyntheticUserID(userID) {
		return forbidden("superseding a page needs a signed-in person")
	}
	actor, err := s.d.Resolver.Identity(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	pages, err := s.d.Repos.Pages.GetByKnowledgeIDs(ctx, tenantID, []string{knowledgeID, replacement.KnowledgeID})
	if err != nil {
		return err
	}
	var retired, kept *model.Page
	for _, p := range pages {
		switch *p.KnowledgeID {
		case knowledgeID:
			retired = p
		case replacement.KnowledgeID:
			kept = p
		}
	}
	if retired == nil {
		return notFound("page")
	}
	d, err := s.d.Resolver.Decide(ctx, actor, retired)
	if err != nil {
		return err
	}
	mark := model.SupersededBy{KnowledgeID: replacement.KnowledgeID, Title: replacement.Title, By: userID}
	if kept != nil {
		mark.PageID, mark.Title = kept.ID, kept.Title
	}
	return s.retire(ctx, actor, d, mark)
}

// retire excludes the page of d from the knowledge base and marks it
// superseded. The caller must be able to edit the page, and a locked page is
// its administrators' to change.
func (s *PageService) retire(ctx context.Context, actor *acl.Identity, d acl.Decision, mark model.SupersededBy) error {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return err
	}
	if !canEdit(d.Role, d.Page) {
		return forbidden("the page is locked")
	}
	mark.At = time.Now().UTC()
	if err := s.d.Repos.Pages.UpdateMeta(ctx, d.Page.TenantID, d.Page.ID,
		map[string]any{"exclude_from_knowledge": true, "superseded_by": mark}); err != nil {
		return err
	}
	s.recordPageMeta(ctx, actor, d.Page, audit.PageSuperseded, "exclude_from_knowledge", true)
	// Now, not after the debounce: the page is to stop answering questions,
	// and every second it stays in the knowledge base it still can.
	if err := s.QueueIndex(ctx, d.Page.TenantID, d.Page.ID, false, 0); err != nil {
		logger.Warnf(ctx, "[docs] queueing superseded page %s failed: %v", d.Page.ID, err)
	}
	return nil
}
