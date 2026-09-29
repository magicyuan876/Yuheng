package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/index"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Mirroring pages into their space's knowledge base.
//
// A space has been able to name a knowledge base since T1.1, and until now
// that binding did nothing: it was stored and no page ever arrived. This is
// the bridge, and the rules about WHAT crosses it are in internal/docs/index
// — chiefly that a restricted page never does, because Yuheng's retrieval
// pipeline has no per-entry permission filter and an indexed restricted page
// would be answerable to everybody who may query the knowledge base.
//
// ---- what is sent
//
// The page as Markdown, which is the form the knowledge pipeline already
// understands and the form a retrieved passage reads best in. Not the
// ProseMirror JSON, which would index structure keywords, and not the HTML,
// which would index tag names.
//
// ---- keeping the two in step
//
// A page owns its entry: docs_pages.knowledge_id points at it, so an edit
// updates rather than duplicates, and a page that stops being eligible has
// its entry removed. That last part matters most — restricting a page that
// was already indexed has to REMOVE it, or the restriction would be
// cosmetic.

// IndexResult is what happened to one page.
type IndexResult struct {
	PageID string `json:"page_id"`
	// Indexed is true when the page now has an entry.
	Indexed bool `json:"indexed"`
	// Removed is true when an entry was taken away.
	Removed bool `json:"removed"`
	// Reason names the rule that applied when the page was not indexed.
	Reason string `json:"reason,omitempty"`
}

// SyncPageToKnowledge brings one page's knowledge entry in step with the page.
//
// Idempotent: running it twice on an unchanged page is one update and no
// duplicate. Safe to call for any page, including ones in spaces with no
// knowledge base — the policy answers "no binding" and nothing happens.
func (s *PageService) SyncPageToKnowledge(ctx context.Context, tenantID uint64, pageID string) (
	*IndexResult, error,
) {
	if s.d.Knowledge == nil {
		return &IndexResult{PageID: pageID, Reason: index.ReasonNoBinding}, nil
	}
	page, err := s.d.Repos.Pages.GetAny(ctx, tenantID, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// The page is gone entirely; so must its entry be.
			return &IndexResult{PageID: pageID, Reason: index.ReasonTrashed}, nil
		}
		return nil, err
	}

	return s.syncLoaded(ctx, page, false)
}

// syncLoaded is SyncPageToKnowledge for a page already read.
//
// With reconcile set, an entry that is already where it belongs is left as it
// is instead of being rewritten. Rewriting means chunking and embedding, and a
// subtree sync visits every page under a moved or restricted page, nearly all
// of which have not changed.
func (s *PageService) syncLoaded(ctx context.Context, page *model.Page, reconcile bool) (*IndexResult, error) {
	decision, boundKB, err := s.indexDecision(ctx, page)
	if err != nil {
		return nil, err
	}
	result := &IndexResult{PageID: page.ID, Reason: decision.Reason}

	if !decision.Index {
		// Whatever the reason, a page that should not be indexed must not
		// have an entry left behind. Restricting an already-indexed page is
		// the case that makes this a removal rather than a skip.
		removed, err := s.dropKnowledgeEntry(ctx, page)
		if err != nil {
			return nil, err
		}
		result.Removed = removed
		return result, nil
	}

	if err := s.writeKnowledgeEntry(ctx, page, boundKB, reconcile); err != nil {
		return nil, err
	}
	result.Indexed = true
	return result, nil
}

// SyncSubtreeToKnowledge brings a page and every page beneath it in step with
// the knowledge base.
//
// Whether a page may be indexed depends on its ancestors: a restriction is
// inherited down the tree, a trashed page takes its children with it, and a
// page moved elsewhere arrives under different ancestors and possibly in a
// different space. An event names only the page it happened to, so re-syncing
// just that page left the ones below it exactly as they were — searchable
// after their parent had been restricted or deleted.
//
// The page itself is synced in full. The pages below are only reconciled
// (see syncLoaded): removed if they should no longer be there, added if they
// should now be, moved if their space is bound to another knowledge base, and
// otherwise left alone.
//
// One page failing does not stop the rest, for the reason SyncSpaceToKnowledge
// gives; the failures come back joined so the caller can log them.
func (s *PageService) SyncSubtreeToKnowledge(ctx context.Context, tenantID uint64, pageID string) (
	[]*IndexResult, error,
) {
	if s.d.Knowledge == nil {
		return []*IndexResult{{PageID: pageID, Reason: index.ReasonNoBinding}}, nil
	}
	ids, err := s.d.Repos.Pages.SubtreeIDs(ctx, tenantID, pageID)
	if err != nil {
		return nil, err
	}

	results := make([]*IndexResult, 0, len(ids)+1)
	var failures []error

	root, err := s.SyncPageToKnowledge(ctx, tenantID, pageID)
	if err != nil {
		failures = append(failures, err)
	} else {
		results = append(results, root)
	}

	for _, id := range ids {
		if id == pageID {
			continue
		}
		page, err := s.d.Repos.Pages.GetAny(ctx, tenantID, id)
		if err != nil {
			failures = append(failures, fmt.Errorf("docs: reading page %s: %w", id, err))
			continue
		}
		result, err := s.syncLoaded(ctx, page, true)
		if err != nil {
			failures = append(failures, fmt.Errorf("docs: reconciling page %s: %w", id, err))
			continue
		}
		results = append(results, result)
	}
	return results, errors.Join(failures...)
}

// indexDecision gathers what the policy needs and applies it.
func (s *PageService) indexDecision(ctx context.Context, page *model.Page) (
	index.Decision, string, error,
) {
	boundKB := ""
	space, err := s.d.Repos.Spaces.Get(ctx, page.TenantID, page.SpaceID)
	if err == nil && space != nil && space.KnowledgeBaseID != nil {
		boundKB = *space.KnowledgeBaseID
	}

	candidate := index.Candidate{
		BoundKB: boundKB,
		Trashed: page.DeletedAt != nil,
		Draft:   page.Status == model.PageDraft,
		Text:    page.TextContent,
	}
	// Only asked when it could change the answer: the restriction lookup is
	// two queries and most pages fail an earlier rule.
	if boundKB != "" && !candidate.Trashed && !candidate.Draft {
		candidate.Restricted = s.anyRestricted(ctx, page.TenantID, page)
	}
	return index.Decide(candidate), boundKB, nil
}

// writeKnowledgeEntry creates or updates the entry mirroring a page, in the
// knowledge base its space is bound to.
//
// An entry lives in one knowledge base, and the binding can change under it: a
// page moved to a space bound to another knowledge base, or its space rebound.
// Updating the entry in place would then keep the page in the old knowledge
// base, answerable to that base's audience and not to the audience of the space
// the page now belongs to. So an entry in the wrong knowledge base is removed
// and a new one made in the right one.
func (s *PageService) writeKnowledgeEntry(ctx context.Context, page *model.Page, kbID string, reconcile bool) error {
	body, err := s.pageMarkdown(page)
	if err != nil {
		return err
	}
	title := page.Title
	if strings.TrimSpace(title) == "" {
		title = "Untitled"
	}

	if page.KnowledgeID != nil && *page.KnowledgeID != "" {
		entryKB, found, err := s.d.Knowledge.KnowledgeBaseOf(ctx, *page.KnowledgeID)
		if err != nil {
			// Not knowing where the entry is, it can be neither updated nor
			// replaced without risking a second copy in the wrong place. The
			// next event tries again.
			return fmt.Errorf("docs: locating the entry of page %s: %w", page.ID, err)
		}
		if found && entryKB != kbID {
			if err := s.d.Knowledge.DeleteKnowledge(ctx, *page.KnowledgeID); err != nil {
				// Making a new entry while this one stays would leave the page
				// searchable in both places.
				return fmt.Errorf("docs: removing the entry of page %s from knowledge base %s: %w",
					page.ID, entryKB, err)
			}
			found = false
		}
		if found && reconcile {
			return nil
		}
		if found {
			updated, err := s.d.Knowledge.UpdateKnowledgeContent(ctx, *page.KnowledgeID, title, body)
			if err == nil && updated {
				return nil
			}
			if err != nil {
				logger.Warnf(ctx, "[docs] updating knowledge entry %s for page %s failed: %v",
					*page.KnowledgeID, page.ID, err)
			}
		}
	}

	return s.createKnowledgeEntry(ctx, page, kbID, title, body)
}

// createKnowledgeEntry makes the entry for a page and records it.
func (s *PageService) createKnowledgeEntry(ctx context.Context, page *model.Page, kbID, title, body string) error {
	created, err := s.d.Knowledge.CreateKnowledgeFromText(ctx, kbID, title, body)
	if err != nil {
		return fmt.Errorf("docs: indexing page %s: %w", page.ID, err)
	}
	return s.d.Repos.Pages.SetKnowledgeID(ctx, page.TenantID, page.ID, &created)
}

// dropKnowledgeEntry removes a page's entry, reporting whether there was one.
func (s *PageService) dropKnowledgeEntry(ctx context.Context, page *model.Page) (bool, error) {
	if page.KnowledgeID == nil || *page.KnowledgeID == "" {
		return false, nil
	}
	if err := s.d.Knowledge.DeleteKnowledge(ctx, *page.KnowledgeID); err != nil {
		// A failed delete is harmless when the entry is already gone, and
		// clearing the pointer is then the right thing. When it is still there
		// the page has been dropped for a reason (restricted, trashed) and is
		// still searchable, so the failure must surface and the pointer must
		// stay, or nothing would ever come back to remove it.
		_, stillThere, lookupErr := s.d.Knowledge.KnowledgeBaseOf(ctx, *page.KnowledgeID)
		if lookupErr != nil || stillThere {
			return false, fmt.Errorf("docs: removing the entry of page %s: %w", page.ID, err)
		}
	}
	if err := s.d.Repos.Pages.SetKnowledgeID(ctx, page.TenantID, page.ID, nil); err != nil {
		return false, err
	}
	return true, nil
}

// pageMarkdown renders a page for the knowledge pipeline.
func (s *PageService) pageMarkdown(page *model.Page) (string, error) {
	content := page.Content
	if len(content) == 0 {
		content = EmptyDocument
	}
	node, _, err := schema.Default().Validate(content)
	if err != nil {
		return "", fmt.Errorf("docs: stored content of %s is invalid: %w", page.ID, err)
	}
	return render.Markdown(node, render.Options{}), nil
}

// SyncSpaceToKnowledge re-indexes a whole space, for a newly bound knowledge
// base or after somebody changed a lot of permissions.
//
// Bounded per call and resumable by the cursor it returns, for the same
// reason the maintenance sweeps are: a space with ten thousand pages must
// not become one transaction.
func (s *PageService) SyncSpaceToKnowledge(ctx context.Context, actor *acl.Identity,
	space *model.Space, role model.SpaceRole, after string, limit int,
) ([]*IndexResult, string, error) {
	// Rebuilding a space's index is an administrator's act: it costs
	// embedding calls and changes what everybody's retrieval returns.
	if err := requireSpaceRole(role, model.RoleAdmin); err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	pages, err := s.d.Repos.Pages.ListSpaceSummaries(ctx, space.TenantID, space.ID, limit, after)
	if err != nil {
		return nil, "", err
	}

	out := make([]*IndexResult, 0, len(pages))
	next := ""
	for _, page := range pages {
		result, err := s.SyncPageToKnowledge(ctx, space.TenantID, page.ID)
		if err != nil {
			// One page that cannot be indexed must not stop the rest: a
			// rebuild that gives up halfway leaves a space half-indexed,
			// which is worse than one page missing.
			logger.Warnf(ctx, "[docs] indexing page %s failed: %v", page.ID, err)
			result = &IndexResult{PageID: page.ID, Reason: "error"}
		}
		out = append(out, result)
		next = page.ID
	}
	if len(pages) < limit {
		next = ""
	}
	return out, next, nil
}

// Knowledge is the slice of Yuheng's knowledge service this module needs.
//
// Narrow on purpose: the module writes documents into a knowledge base and
// removes them, and nothing here should be able to do more than that.
type Knowledge interface {
	// CreateKnowledgeFromText adds a Markdown document and returns its id.
	CreateKnowledgeFromText(ctx context.Context, kbID, title, markdown string) (string, error)
	// UpdateKnowledgeContent replaces a document, reporting false when the
	// entry no longer exists.
	UpdateKnowledgeContent(ctx context.Context, knowledgeID, title, markdown string) (bool, error)
	// KnowledgeBaseOf says which knowledge base holds a document, and false when
	// there is no such document.
	KnowledgeBaseOf(ctx context.Context, knowledgeID string) (kbID string, found bool, err error)
	// DeleteKnowledge removes a document.
	DeleteKnowledge(ctx context.Context, knowledgeID string) error
}
