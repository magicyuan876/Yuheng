package docs

import (
	"context"
	"errors"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/draft"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The adapter between this module's Drafter interface and Yuheng's knowledge
// and model services.
//
// ---- decision: the model is the knowledge base's summary model
//
// There is no new setting for which model writes drafts. A space that can
// draft is by definition bound to a knowledge base, and a knowledge base
// already names a model for summarising its documents. Using that one means
// an administrator who configured retrieval has configured this too, and
// that a deployment whose knowledge base has no model simply cannot draft —
// which is the right answer, not an error to work around.

type draftBridge struct {
	knowledge interfaces.KnowledgeService
	bases     interfaces.KnowledgeBaseService
	models    interfaces.ModelService
}

// NewDraftBridge adapts Yuheng's services for the docs drafting feature.
// Any missing dependency yields nil, which makes the feature unavailable
// rather than half-working.
func NewDraftBridge(knowledge interfaces.KnowledgeService, bases interfaces.KnowledgeBaseService,
	models interfaces.ModelService,
) service.Drafter {
	if knowledge == nil || bases == nil || models == nil {
		return nil
	}
	return &draftBridge{knowledge: knowledge, bases: bases, models: models}
}

// Entry returns a knowledge entry's title, text and owning knowledge base.
//
// The knowledge base id is returned rather than checked here on purpose: the
// caller compares it against the space's own binding, and a check split
// across two files is one somebody can satisfy in the wrong place.
func (b *draftBridge) Entry(ctx context.Context, knowledgeID string) (string, string, string, error) {
	entry, err := b.knowledge.GetKnowledgeByID(ctx, knowledgeID)
	if err != nil {
		return "", "", "", err
	}
	if entry == nil {
		return "", "", "", errors.New("docs: knowledge entry not found")
	}
	text := entryText(entry)
	if strings.TrimSpace(text) == "" {
		// An entry still being parsed has no text yet. Reported as an error
		// rather than drafted from silently, because a draft assembled from
		// an empty source is one whose gaps nobody can explain.
		return "", "", "", errors.New("docs: that knowledge entry has no text yet")
	}
	return entry.Title, text, entry.KnowledgeBaseID, nil
}

// entryText finds the entry's plain text.
//
// Manual entries keep their Markdown in metadata; parsed documents carry a
// description and their chunks live elsewhere. The description is what is
// available synchronously and is what a summary is for.
func entryText(entry *types.Knowledge) string {
	if entry.IsManual() {
		if meta, err := entry.ManualMetadata(); err == nil && meta != nil &&
			strings.TrimSpace(meta.Content) != "" {
			return meta.Content
		}
	}
	return entry.Description
}

// Draft asks the knowledge base's summary model to write the page.
func (b *draftBridge) Draft(ctx context.Context, kbID string, req draft.Request) (string, error) {
	base, err := b.bases.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return "", err
	}
	if base == nil || strings.TrimSpace(base.SummaryModelID) == "" {
		return "", errors.New("docs: this knowledge base has no model configured for writing")
	}
	model, err := b.models.GetChatModel(ctx, base.SummaryModelID)
	if err != nil {
		return "", err
	}

	system, user := draft.BuildPrompt(req)
	answer, err := model.Chat(ctx, []chat.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, &chat.ChatOptions{})
	if err != nil {
		return "", err
	}
	if answer == nil {
		return "", errors.New("docs: the model returned nothing")
	}
	return answer.Content, nil
}
