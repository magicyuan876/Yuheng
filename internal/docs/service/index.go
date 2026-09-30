package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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

// indexLease is how long a claim on a page holds before another worker may take
// it over. It has to outlast the slowest realistic synchronisation (a long page
// through a slow embedding service); a worker that dies simply lets it expire.
const indexLease = 10 * time.Minute

// SyncPageToKnowledge brings one page's knowledge entry in step with the page,
// now, for the caller that asked (an administrator rebuilding a space, a test).
// The background workers do the same for the pages queued by edits.
//
// Idempotent: running it twice on an unchanged page sends nothing the second
// time. Safe to call for any page, including ones in spaces with no knowledge
// base — the policy answers "no binding" and nothing happens. A page a worker
// is synchronising at that moment is left to it (Reason "busy"): two writers
// on one page are how a duplicate entry would be made.
func (s *PageService) SyncPageToKnowledge(ctx context.Context, tenantID uint64, pageID string) (
	*IndexResult, error,
) {
	if s.d.Knowledge == nil {
		return &IndexResult{PageID: pageID, Reason: index.ReasonNoBinding}, nil
	}
	claim, ok, err := s.d.Repos.IndexQueue.ClaimPage(ctx, tenantID, pageID, indexLease)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Either a worker holds the page, or the page is gone entirely; a
		// missing page has nothing to mirror, and the entry of a purged page
		// was removed when it was trashed.
		if _, getErr := s.d.Repos.Pages.GetAny(ctx, tenantID, pageID); getErr != nil {
			if errors.Is(getErr, repository.ErrNotFound) {
				return &IndexResult{PageID: pageID, Reason: index.ReasonTrashed}, nil
			}
			return nil, getErr
		}
		return &IndexResult{PageID: pageID, Reason: ReasonBusy}, nil
	}
	return s.syncClaimed(ctx, claim)
}

// ReasonBusy is the IndexResult reason for a page another worker is
// synchronising.
const ReasonBusy = "busy"

// syncClaimed synchronises a page the caller holds a claim on, and settles the
// claim: completed with the hash of what is now in the knowledge base, or
// failed with a retry scheduled. The returned error is the synchronisation's.
func (s *PageService) syncClaimed(ctx context.Context, claim repository.IndexClaim) (*IndexResult, error) {
	result, hash, err := s.syncPage(ctx, claim.TenantID, claim.PageID)
	if err != nil {
		if failErr := s.d.Repos.IndexQueue.Fail(ctx, claim, err, indexRetryDelay(claim.Attempts)); failErr != nil {
			logger.Warnf(ctx, "[docs] releasing the claim on page %s failed: %v", claim.PageID, failErr)
		}
		return nil, err
	}
	if err := s.d.Repos.IndexQueue.Complete(ctx, claim, hash); err != nil {
		// The entry is right and only the bookkeeping failed: the claim expires
		// and the page is looked at again, which costs nothing while the hash
		// stored last time still matches.
		logger.Warnf(ctx, "[docs] recording the synchronisation of page %s failed: %v", claim.PageID, err)
	}
	return result, nil
}

// syncPage does the synchronisation itself and reports the hash of what the
// knowledge base holds for the page afterwards ("" when it holds nothing).
func (s *PageService) syncPage(ctx context.Context, tenantID uint64, pageID string) (
	*IndexResult, string, error,
) {
	page, err := s.d.Repos.Pages.GetAny(ctx, tenantID, pageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &IndexResult{PageID: pageID, Reason: index.ReasonTrashed}, "", nil
		}
		return nil, "", err
	}

	decision, boundKB, err := s.indexDecision(ctx, page)
	if err != nil {
		return nil, "", err
	}
	result := &IndexResult{PageID: page.ID, Reason: decision.Reason}

	if !decision.Index {
		// Whatever the reason, a page that should not be indexed must not
		// have an entry left behind. Restricting an already-indexed page is
		// the case that makes this a removal rather than a skip.
		removed, err := s.dropKnowledgeEntry(ctx, page)
		if err != nil {
			return nil, "", err
		}
		result.Removed = removed
		return result, "", nil
	}

	hash, err := s.writeKnowledgeEntry(ctx, page, boundKB)
	if err != nil {
		return nil, "", err
	}
	result.Indexed = true
	return result, hash, nil
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
		BoundKB:  boundKB,
		Trashed:  page.DeletedAt != nil,
		Excluded: page.ExcludeFromKnowledge,
		Text:     page.TextContent,
	}
	// Only asked when it could change the answer: the restriction lookup is
	// two queries and most pages fail an earlier rule.
	if boundKB != "" && !candidate.Trashed && !candidate.Excluded {
		candidate.Restricted = s.anyRestricted(ctx, page.TenantID, page)
	}
	return index.Decide(candidate), boundKB, nil
}

// writeKnowledgeEntry creates or updates the entry mirroring a page, in the
// knowledge base its space is bound to, and returns the hash of what it sent.
//
// An entry lives in one knowledge base, and the binding can change under it: a
// page moved to a space bound to another knowledge base, or its space rebound.
// Updating the entry in place would then keep the page in the old knowledge
// base, answerable to that base's audience and not to the audience of the space
// the page now belongs to. So an entry in the wrong knowledge base is removed
// and a new one made in the right one.
//
// Sending a page means chunking and embedding it, which costs money on a hosted
// model, and most looks at a page find it unchanged (a permission event on its
// parent, a periodic re-check). An entry already in the right place holding the
// hash of what would be sent is left alone.
func (s *PageService) writeKnowledgeEntry(ctx context.Context, page *model.Page, kbID string) (string, error) {
	body, err := s.pageMarkdown(page)
	if err != nil {
		return "", err
	}
	title := page.Title
	if strings.TrimSpace(title) == "" {
		title = "Untitled"
	}
	hash := contentHash(title, body)

	if page.KnowledgeID != nil && *page.KnowledgeID != "" {
		entryKB, found, err := s.d.Knowledge.KnowledgeBaseOf(ctx, *page.KnowledgeID)
		if err != nil {
			// Not knowing where the entry is, it can be neither updated nor
			// replaced without risking a second copy in the wrong place. The
			// retry looks again.
			return "", fmt.Errorf("docs: locating the entry of page %s: %w", page.ID, err)
		}
		if found && entryKB != kbID {
			if err := s.d.Knowledge.DeleteKnowledge(ctx, page.TenantID, *page.KnowledgeID); err != nil {
				// Making a new entry while this one stays would leave the page
				// searchable in both places.
				return "", fmt.Errorf("docs: removing the entry of page %s from knowledge base %s: %w",
					page.ID, entryKB, err)
			}
			found = false
		}
		if found && s.sentHash(ctx, page) == hash {
			// The content is in place; the stewardship may not be, after a
			// hand-over, which queues the page for exactly this.
			return hash, s.syncStewardship(ctx, page, *page.KnowledgeID)
		}
		if found {
			updated, err := s.d.Knowledge.UpdateKnowledgeContent(ctx, page.TenantID, *page.KnowledgeID, title, body)
			if err == nil && updated {
				return hash, s.syncStewardship(ctx, page, *page.KnowledgeID)
			}
			if err != nil {
				logger.Warnf(ctx, "[docs] updating knowledge entry %s for page %s failed: %v",
					*page.KnowledgeID, page.ID, err)
			}
		}
	}

	if err := s.createKnowledgeEntry(ctx, page, kbID, title, body); err != nil {
		return "", err
	}
	return hash, nil
}

// sentHash is the hash recorded when the page was last synchronised, or "".
func (s *PageService) sentHash(ctx context.Context, page *model.Page) string {
	st, err := s.d.Repos.IndexQueue.Get(ctx, page.TenantID, page.ID)
	if err != nil {
		return ""
	}
	return st.IndexedHash
}

// contentHash identifies what is sent for a page. The separator keeps a title
// that ends where a body begins from colliding with a different split.
func contentHash(title, body string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + body))
	return hex.EncodeToString(sum[:])
}

// createKnowledgeEntry makes the entry for a page and records it.
func (s *PageService) createKnowledgeEntry(ctx context.Context, page *model.Page, kbID, title, body string) error {
	created, err := s.d.Knowledge.CreateKnowledgeFromText(ctx, page.TenantID, kbID, title, body)
	if err != nil {
		return fmt.Errorf("docs: indexing page %s: %w", page.ID, err)
	}
	if err := s.d.Repos.Pages.SetKnowledgeID(ctx, page.TenantID, page.ID, &created); err != nil {
		return err
	}
	return s.syncStewardship(ctx, page, created)
}

// dropKnowledgeEntry removes a page's entry, reporting whether there was one.
func (s *PageService) dropKnowledgeEntry(ctx context.Context, page *model.Page) (bool, error) {
	if page.KnowledgeID == nil || *page.KnowledgeID == "" {
		return false, nil
	}
	if err := s.d.Knowledge.DeleteKnowledge(ctx, page.TenantID, *page.KnowledgeID); err != nil {
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
// Every call that writes says whose knowledge base it is: the workers that make
// these calls run outside any request, so there is no caller to take the tenant
// from.
//
// Narrow on purpose: the module writes documents into a knowledge base and
// removes them, and nothing here should be able to do more than that.
type Knowledge interface {
	// CreateKnowledgeFromText adds a Markdown document and returns its id.
	CreateKnowledgeFromText(ctx context.Context, tenantID uint64, kbID, title, markdown string) (string, error)
	// UpdateKnowledgeContent replaces a document, reporting false when the
	// entry no longer exists.
	UpdateKnowledgeContent(ctx context.Context, tenantID uint64, knowledgeID, title, markdown string) (bool, error)
	// KnowledgeBaseOf says which knowledge base holds a document, and false when
	// there is no such document.
	KnowledgeBaseOf(ctx context.Context, knowledgeID string) (kbID string, found bool, err error)
	// DeleteKnowledge removes a document.
	DeleteKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) error
	// SyncStewardship copies a page's maintainer and its last review onto
	// its document. ownerID may be empty; an empty reviewedBy or a zero
	// reviewedAt records no review.
	SyncStewardship(ctx context.Context, tenantID uint64, knowledgeID, ownerID, reviewedBy string,
		reviewedAt time.Time) error
}
