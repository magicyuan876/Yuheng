package service

import (
	"context"
	"encoding/json"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Block references.
//
// A reference stores a page id and a block id and nothing else. What it shows
// is the source block as that page has it now, which is the whole point: a
// definition quoted on six pages is written once and corrected once.
//
// Two saves keep it working, and they are deliberately different jobs:
//
//   - Saving a page that *contains* references records which blocks of which
//     pages somebody is now watching. It cannot fill in their content, because
//     the source documents are not to hand.
//   - Saving a page that *is referenced* refreshes the snapshots of its own
//     blocks that somebody watches, and drops the ones that no longer exist.
//
// Everything is derived data. A missing or stale snapshot costs one reference
// one render, and the next save of either page repairs it, so none of it is
// allowed to fail a save.
//
// ---- decision: quoted text is searchable, but not through the quoting page
//
// The plan for this work package asks for the quoted content to count towards
// the quoting page's text when searching. Implemented the obvious way -- by
// appending the snapshot text to docs_pages.text_content -- that would be a
// side channel straight past the rule this whole file exists to enforce.
// Rendering already refuses to show a quoted block to somebody who may not
// read its source page; a search index that carries the same words under the
// quoting page's id would let exactly that reader find them, and a snippet
// would show them.
//
// So the text is stored where it can be filtered instead of where it cannot:
// docs_transclusion_blocks already holds each snapshot's text alongside the
// id of the page it came from. Search (T5.4) joins that table and applies the
// reader's own access to the *source* page before a quoted match counts,
// which satisfies the requirement without the leak. A test below pins the
// absence: the quoting page's stored text must not contain the quoted words.

// MaxTransclusionLookup bounds one batch of resolutions, which is one open
// page's worth of references.
const MaxTransclusionLookup = 200

// TransclusionState says why a reference shows what it shows.
type TransclusionState string

const (
	// TransclusionOK means the block was found and its content is below.
	TransclusionOK TransclusionState = "ok"
	// TransclusionMissing covers both "that block is gone" and "you may not
	// read that page", deliberately: one state for both, so a reference
	// cannot be used to find out whether a page exists.
	TransclusionMissing TransclusionState = "missing"
	// TransclusionPending means the reference is recorded but the source page
	// has not been saved since, so there is nothing to show yet. It is an
	// honest "not loaded" rather than a wrong "deleted".
	TransclusionPending TransclusionState = "pending"
)

// TransclusionView is one resolved reference.
type TransclusionView struct {
	SourcePageID  string            `json:"source_page_id"`
	SourceBlockID string            `json:"source_block_id"`
	State         TransclusionState `json:"state"`
	// Content is the block, as a document node, when State is ok.
	Content json.RawMessage `json:"content,omitempty"`
	// Title and Icon name the page the block lives on, so a reference can be
	// attributed and followed.
	Title string `json:"title,omitempty"`
	Icon  string `json:"icon,omitempty"`
	// SourceShortID addresses the source page in a URL.
	SourceShortID string `json:"source_short_id,omitempty"`
}

// ---- recording what a document watches -------------------------------------

// recordTransclusions notes the blocks this page now shows by reference.
//
// Runs on every save of the referring page, beside recordLinks and for the
// same reason: a reference is removed by deleting the block that holds it,
// which produces no event of its own. Rows for references that have gone are
// not removed here — the source page's own save does that when it finds
// nobody watching — so a reference cut and pasted within one editing session
// does not lose its snapshot.
func (b *base) recordTransclusions(ctx context.Context, page *model.Page, st render.Structure) {
	if b.d.Repos.Blocks == nil || len(st.Transclusions) == 0 {
		return
	}
	byPage := map[string][]string{}
	for _, t := range st.Transclusions {
		// A page referencing its own block is answered from the document in
		// front of the reader; storing a snapshot of it would be a copy of
		// something already on screen.
		if t.SourcePageID == "" || t.SourceBlockID == "" || t.SourcePageID == page.ID {
			continue
		}
		byPage[t.SourcePageID] = append(byPage[t.SourcePageID], t.SourceBlockID)
	}
	for sourcePageID, blockIDs := range byPage {
		if _, err := b.d.Repos.Blocks.Wanted(ctx, page.TenantID, sourcePageID, blockIDs); err != nil {
			logger.Warnf(ctx, "[docs] recording the block references of page %s failed: %v", page.ID, err)
			return
		}
	}
}

// refreshTransclusionSnapshots stores the current content of this page's
// blocks that somebody else references.
//
// Nothing happens for the overwhelming majority of pages, which nobody
// references: one indexed query returns nothing and the walk never runs.
func (b *base) refreshTransclusionSnapshots(ctx context.Context, page *model.Page, node *schema.Node) {
	if b.d.Repos.Blocks == nil {
		return
	}
	tracked, err := b.d.Repos.Blocks.TrackedFor(ctx, page.TenantID, page.ID)
	if err != nil {
		logger.Warnf(ctx, "[docs] reading the tracked blocks of page %s failed: %v", page.ID, err)
		return
	}
	if len(tracked) == 0 {
		return
	}

	found := render.FindBlocks(node, tracked)
	blocks := make([]model.TransclusionBlock, 0, len(found))
	for blockID, block := range found {
		// Clipped, so a snapshot never contains a reference of its own: see
		// render.ClipTransclusions for why that is a rule rather than a limit.
		clipped := render.ClipTransclusions(block)
		raw, err := json.Marshal(clipped)
		if err != nil {
			continue
		}
		blocks = append(blocks, model.TransclusionBlock{
			BlockID:     blockID,
			Content:     model.JSON(raw),
			TextContent: render.Text(clipped),
		})
	}
	// Store also removes the rows for tracked blocks that are not in `blocks`,
	// which is how a reference comes to say the block was deleted.
	if err := b.d.Repos.Blocks.Store(ctx, page.TenantID, page.ID, blocks); err != nil {
		logger.Warnf(ctx, "[docs] storing the block snapshots of page %s failed: %v", page.ID, err)
	}
}

// ---- reading ---------------------------------------------------------------

// ResolveTransclusions answers what a page's block references should show.
//
// Access is decided per source page, once, however many of its blocks are
// referenced. A page the reader may not open resolves exactly as a deleted one
// does — see TransclusionMissing.
func (s *PageService) ResolveTransclusions(ctx context.Context, actor *acl.Identity,
	refs []repository.BlockRef,
) ([]*TransclusionView, error) {
	refs = dedupeRefs(refs)
	if len(refs) > MaxTransclusionLookup {
		return nil, invalid("at most %d block references may be resolved at once", MaxTransclusionLookup)
	}
	out := make([]*TransclusionView, 0, len(refs))
	if len(refs) == 0 || s.d.Repos.Blocks == nil {
		return out, nil
	}

	pageIDs := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		if !seen[ref.PageID] {
			seen[ref.PageID] = true
			pageIDs = append(pageIDs, ref.PageID)
		}
	}
	pages, err := s.d.Repos.Pages.GetSummaries(ctx, actor.TenantID, pageIDs)
	if err != nil {
		return nil, err
	}
	readable := make(map[string]*model.Page, len(pages))
	for _, p := range pages {
		decision, err := s.d.Resolver.Decide(ctx, actor, p)
		if err != nil {
			return nil, err
		}
		if decision.Role == model.RoleNone {
			continue
		}
		readable[p.ID] = p
	}

	// Only the readable pages are asked for, so an unreadable one costs no
	// query and leaks nothing through timing either.
	allowed := make([]repository.BlockRef, 0, len(refs))
	for _, ref := range refs {
		if readable[ref.PageID] != nil {
			allowed = append(allowed, ref)
		}
	}
	rows, err := s.d.Repos.Blocks.Load(ctx, actor.TenantID, allowed)
	if err != nil {
		return nil, err
	}
	snapshots := make(map[string]model.TransclusionBlock, len(rows))
	for _, row := range rows {
		snapshots[row.PageID+"\x00"+row.BlockID] = row
	}

	for _, ref := range refs {
		view := &TransclusionView{
			SourcePageID: ref.PageID, SourceBlockID: ref.BlockID, State: TransclusionMissing,
		}
		page := readable[ref.PageID]
		if page == nil {
			out = append(out, view)
			continue
		}
		view.Title = page.Title
		view.SourceShortID = page.ShortID
		if page.Icon != nil {
			view.Icon = *page.Icon
		}

		row, ok := snapshots[ref.PageID+"\x00"+ref.BlockID]
		switch {
		case !ok:
			// No row at all. Saving the page that holds the reference creates
			// one, and a save of the source page removes it again when the
			// block is no longer in that document -- so the absence of a row
			// means the block is gone, not that it is on its way.
			view.State = TransclusionMissing
		case len(row.Content) == 0 || string(row.Content) == "null":
			// The row exists but has never been filled in: the reference was
			// recorded and the source page has not been saved since.
			view.State = TransclusionPending
		default:
			view.State = TransclusionOK
			view.Content = json.RawMessage(row.Content)
		}
		out = append(out, view)
	}
	return out, nil
}

// dedupeRefs keeps the first occurrence of each (page, block) pair, in order.
func dedupeRefs(refs []repository.BlockRef) []repository.BlockRef {
	seen := make(map[string]bool, len(refs))
	out := make([]repository.BlockRef, 0, len(refs))
	for _, ref := range refs {
		if ref.PageID == "" || ref.BlockID == "" {
			continue
		}
		key := ref.PageID + "\x00" + ref.BlockID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	return out
}
