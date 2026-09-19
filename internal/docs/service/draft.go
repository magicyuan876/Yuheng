package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/draft"
	"github.com/magicyuan876/yuheng/internal/docs/markdown"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Writing a page from knowledge-base material.
//
// The other direction from T5.2: that mirrors documents into a knowledge
// base, this assembles a document out of one. It is what docs_pages.
// source_refs and the "ai" replace reason were reserved for in T0.2 and
// T1.7, both of which have sat unused until now.
//
// ---- decision: the caller names the sources
//
// This does not search the knowledge base and decide for itself what is
// relevant. It is given a list of entries and draws only on those. Two
// reasons, and the second is the important one:
//
// A draft assembled from material the person did not choose is one they
// cannot check. "Where did this paragraph come from" has to have an answer
// shorter than re-running a retrieval query.
//
// And an automatic search would have to decide which entries the caller may
// see. A knowledge base is shared by its own rules, not by this module's;
// resolving that correctly means duplicating a permission model that lives
// elsewhere and will drift. Asking for explicit ids moves that question to
// where it belongs — the caller already had to be able to see them to name
// them, and the check below confirms they are in the space's own bound base
// rather than somebody else's.
//
// ---- decision: the write goes through ReplaceContent
//
// Not straight into the page row. ReplaceContent is the one write path
// (T1.7): it takes a history snapshot first, so a draft that replaced
// somebody's work can be undone; it routes through the collaboration service
// where one is configured, so anybody with the page open sees the draft
// appear instead of losing it on their next save; and it records the reason,
// so the audit trail says a model wrote this.

// MaxDraftSources bounds how many entries one draft may draw on.
const MaxDraftSources = draft.MaxSources

// DraftInput is a request to write a page from knowledge material.
type DraftInput struct {
	// Instruction is what the draft should cover.
	Instruction string
	// KnowledgeIDs are the entries to draw on. They must belong to the
	// knowledge base this page's space is bound to.
	KnowledgeIDs []string
}

// DraftResult is what was written.
type DraftResult struct {
	PageID string `json:"page_id"`
	// SourceIDs are the knowledge entries the draft drew on, also recorded
	// on the page so a later reader can see where it came from.
	SourceIDs []string `json:"source_ids"`
	// Markdown is what the model produced, returned so a client can show it
	// before or beside the rendered page.
	Markdown string `json:"markdown"`
	// Warnings are the converter's complaints about the model's Markdown:
	// constructs it could not represent. Surfaced rather than swallowed,
	// because they are the difference between "the model wrote a table" and
	// "the page has no table in it".
	Warnings []string `json:"warnings,omitempty"`
}

// DraftPageFromKnowledge writes a page body from knowledge-base material.
func (s *PageService) DraftPageFromKnowledge(ctx context.Context, actor *acl.Identity,
	d acl.Decision, in DraftInput,
) (*DraftResult, error) {
	// Writing the page is the permission this needs; the material is checked
	// separately below.
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	if s.d.Drafter == nil {
		return nil, forbidden("drafting from the knowledge base is not available in this deployment")
	}

	instruction, err := draft.CleanInstruction(in.Instruction)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	if len(in.KnowledgeIDs) == 0 {
		return nil, invalid("name at least one knowledge entry to draw on")
	}
	if len(in.KnowledgeIDs) > MaxDraftSources {
		return nil, invalid("a draft may draw on at most %d entries", MaxDraftSources)
	}

	kbID, err := s.boundKnowledgeBase(ctx, d.Page)
	if err != nil {
		return nil, err
	}
	sources, err := s.draftSources(ctx, kbID, dedupe(in.KnowledgeIDs))
	if err != nil {
		return nil, err
	}

	body, err := s.d.Drafter.Draft(ctx, kbID, draft.Request{
		Instruction: instruction,
		PageTitle:   d.Page.Title,
		Sources:     sources,
	})
	if err != nil {
		return nil, fmt.Errorf("docs: drafting page %s: %w", d.Page.ID, err)
	}
	cleaned := draft.CleanOutput(body)
	if strings.TrimSpace(cleaned) == "" {
		return nil, fmt.Errorf("docs: the model returned an empty draft")
	}

	doc, report, err := markdown.ToDocument([]byte(cleaned), markdown.Options{})
	if err != nil {
		return nil, invalid("the draft could not be converted into a document: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	if _, err := s.ReplaceContent(ctx, actor, d, ReplaceInput{
		Content: encoded,
		Reason:  ReplaceReasonAI,
	}); err != nil {
		return nil, err
	}

	sourceIDs := draft.SourceIDs(sources)
	// Recorded after the write, and a failure here is logged rather than
	// returned: the page has the content, and losing the provenance is worse
	// than a failed request but not worth discarding the draft over.
	s.recordSourceRefs(ctx, d.Page, sourceIDs)

	return &DraftResult{
		PageID: d.Page.ID, SourceIDs: sourceIDs,
		Markdown: cleaned, Warnings: reportWarnings(report),
	}, nil
}

// boundKnowledgeBase is the knowledge base this page's space is bound to.
func (s *PageService) boundKnowledgeBase(ctx context.Context, page *model.Page) (string, error) {
	space, err := s.d.Repos.Spaces.Get(ctx, page.TenantID, page.SpaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", notFound("space")
		}
		return "", err
	}
	if space.KnowledgeBaseID == nil || *space.KnowledgeBaseID == "" {
		return "", invalid("this space is not bound to a knowledge base")
	}
	return *space.KnowledgeBaseID, nil
}

// draftSources fetches the named entries, refusing any that are not in this
// space's own knowledge base.
//
// This is the check that keeps the feature from being a way to read other
// knowledge bases: without it, naming any entry id would pull its text into
// a document in a space that has nothing to do with it.
func (s *PageService) draftSources(ctx context.Context, kbID string, ids []string) (
	[]draft.Source, error,
) {
	out := make([]draft.Source, 0, len(ids))
	for _, id := range ids {
		title, text, entryKB, err := s.d.Drafter.Entry(ctx, id)
		if err != nil {
			return nil, notFound("knowledge entry")
		}
		if entryKB != kbID {
			// Reported as not found rather than forbidden: whether an entry
			// exists in a knowledge base this space cannot see is not
			// something this endpoint should confirm.
			return nil, notFound("knowledge entry")
		}
		out = append(out, draft.Source{ID: id, Title: title, Text: text})
	}
	return out, nil
}

// recordSourceRefs notes which entries a page was distilled from.
func (s *PageService) recordSourceRefs(ctx context.Context, page *model.Page, ids []string) {
	if len(ids) == 0 {
		return
	}
	refs := model.StringList(ids)
	if err := s.d.Repos.Pages.UpdateMeta(ctx, page.TenantID, page.ID,
		map[string]any{"source_refs": refs}); err != nil {
		logger.Warnf(ctx, "[docs] recording the sources of page %s failed: %v", page.ID, err)
	}
}

// reportWarnings flattens the converter's report.
func reportWarnings(report *markdown.Report) []string {
	if report == nil {
		return nil
	}
	return report.Warnings
}

// Drafter is the slice of Yuheng's model and knowledge services this feature
// needs.
//
// Narrow on purpose, and separate from the Knowledge interface the indexer
// uses: that one writes into a knowledge base, this one reads from it and
// calls a model. A build can have either, both or neither.
type Drafter interface {
	// Entry returns a knowledge entry's title, plain text and the knowledge
	// base it belongs to.
	Entry(ctx context.Context, knowledgeID string) (title, text, kbID string, err error)
	// Draft asks the knowledge base's configured model to write a page.
	Draft(ctx context.Context, kbID string, req draft.Request) (string, error)
}
