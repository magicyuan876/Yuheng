package service

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Knowledge health, as a page reader sees it.
//
// A page mirrored into its space's knowledge base is a knowledge entry, and
// the knowledge-health checks may find it duplicated by another entry. The
// reader of the page is told about the other side only when the other side is
// a page they may read themselves. Anything else — an uploaded file of the
// knowledge base, a page of a space they are not in, a restricted page — is
// only counted: the finding's evidence quotes the other document, and even its
// title is more than the page's permissions give away.

// PageFindingPage names the page on the other side of a finding.
type PageFindingPage struct {
	ID        string `json:"id"`
	ShortID   string `json:"short_id"`
	Title     string `json:"title"`
	SpaceSlug string `json:"space_slug"`
}

// PageFindingView is one finding of a page. RelatedPage is null for a finding
// about the page alone, such as a review that is due.
type PageFindingView struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Severity     string                  `json:"severity"`
	Score        float64                 `json:"score"`
	OverlapRatio float64                 `json:"overlap_ratio"`
	RelatedPage  *PageFindingPage        `json:"related_page"`
	Evidence     []types.FindingEvidence `json:"evidence"`
	// Assignee is who the finding is taken to, nil when nobody could be
	// found.
	Assignee *types.PersonRef `json:"assignee"`
}

// PageFindingsView answers GET /docs/pages/{pid}/findings.
type PageFindingsView struct {
	Items []*PageFindingView `json:"items"`
	// OtherCount is how many further open findings involve the page whose
	// other side the reader may not see.
	OtherCount int `json:"other_count"`
}

// Findings lists the open knowledge-health findings of the page for the
// caller, who has already been checked to read it.
func (s *PageService) Findings(ctx context.Context, actor *acl.Identity, d acl.Decision) (*PageFindingsView, error) {
	out := &PageFindingsView{Items: []*PageFindingView{}}
	page := d.Page
	if s.d.Findings == nil || page == nil || page.KnowledgeID == nil || *page.KnowledgeID == "" {
		return out, nil
	}
	findings, err := s.d.Findings.OpenForKnowledge(ctx, page.TenantID, *page.KnowledgeID)
	if err != nil {
		return nil, err
	}
	if len(findings) == 0 {
		return out, nil
	}

	relatedIDs := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.Related != nil {
			relatedIDs = append(relatedIDs, f.Related.KnowledgeID)
		}
	}
	pages, err := s.d.Repos.Pages.GetByKnowledgeIDs(ctx, page.TenantID, dedupe(relatedIDs))
	if err != nil {
		return nil, err
	}
	byKnowledge := make(map[string]*model.Page, len(pages))
	for _, p := range pages {
		if p.KnowledgeID != nil && p.ID != page.ID {
			byKnowledge[*p.KnowledgeID] = p
		}
	}
	// Each related page is decided once, however many findings name it.
	readable := map[string]*PageFindingPage{}
	decided := map[string]bool{}
	visible := func(p *model.Page) (*PageFindingPage, error) {
		if decided[p.ID] {
			return readable[p.ID], nil
		}
		decided[p.ID] = true
		decision, err := s.d.Resolver.Decide(ctx, actor, p)
		if err != nil {
			return nil, err
		}
		if !decision.Allows(model.RoleReader) || decision.Space == nil {
			return nil, nil
		}
		ref := &PageFindingPage{ID: p.ID, ShortID: p.ShortID, Title: p.Title, SpaceSlug: decision.Space.Slug}
		readable[p.ID] = ref
		return ref, nil
	}

	for _, f := range findings {
		view := &PageFindingView{
			ID: f.ID, Type: f.Type, Severity: f.Severity, Score: f.Score, OverlapRatio: f.OverlapRatio,
			Evidence: append([]types.FindingEvidence{}, f.Evidence...), Assignee: f.Assignee,
		}
		if f.Related != nil {
			other, ok := byKnowledge[f.Related.KnowledgeID]
			if !ok {
				out.OtherCount++
				continue
			}
			ref, err := visible(other)
			if err != nil {
				return nil, err
			}
			if ref == nil {
				out.OtherCount++
				continue
			}
			view.RelatedPage = ref
		}
		out.Items = append(out.Items, view)
	}
	return out, nil
}
