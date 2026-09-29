package docs

import (
	"context"
	"errors"

	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The adapter between this module's narrow Knowledge interface and Yuheng's
// knowledge service.
//
// It lives here rather than in the service package so that the docs services
// depend on three methods they define themselves, and not on the forty-odd
// of interfaces.KnowledgeService. That keeps the test fake honest — it
// implements the whole interface the code uses — and it means a change to
// Yuheng's knowledge API touches this file rather than the module.

// DocsChannel labels knowledge entries that came from a document page, so an
// operator looking at a knowledge base can tell which rows are mirrors of
// documents and which somebody uploaded.
const DocsChannel = "docs"

type knowledgeBridge struct {
	svc interfaces.KnowledgeService
}

// NewKnowledgeBridge adapts Yuheng's knowledge service for the docs module.
// A nil service yields nil, which leaves every space unindexed.
func NewKnowledgeBridge(svc interfaces.KnowledgeService) service.Knowledge {
	if svc == nil {
		return nil
	}
	return &knowledgeBridge{svc: svc}
}

// CreateKnowledgeFromText adds a page's Markdown as a knowledge document.
func (b *knowledgeBridge) CreateKnowledgeFromText(ctx context.Context, kbID, title, markdown string) (
	string, error,
) {
	created, err := b.svc.CreateKnowledgeFromManual(ctx, kbID, &types.ManualKnowledgePayload{
		Title:   title,
		Content: markdown,
		Channel: DocsChannel,
	}, DocsChannel)
	if err != nil {
		return "", err
	}
	if created == nil {
		return "", errors.New("docs: the knowledge service returned no entry")
	}
	return created.ID, nil
}

// UpdateKnowledgeContent replaces a mirrored document.
//
// Reports false rather than an error when the entry has gone, so the caller
// can make a new one instead of leaving the page unindexed for ever — a
// knowledge base somebody emptied by hand is an ordinary situation, not a
// failure.
func (b *knowledgeBridge) UpdateKnowledgeContent(ctx context.Context, knowledgeID, title,
	markdown string,
) (bool, error) {
	updated, err := b.svc.UpdateManualKnowledge(ctx, knowledgeID, &types.ManualKnowledgePayload{
		Title:   title,
		Content: markdown,
		Channel: DocsChannel,
	})
	if err != nil || updated == nil {
		// Treated as "gone" rather than surfaced: the caller makes a new
		// entry, which is the right outcome whether the row was deleted or
		// the update genuinely failed. A page left permanently unindexed
		// because of one bad call would be the worse failure.
		return false, nil
	}
	return true, nil
}

// KnowledgeBaseOf reports the knowledge base holding an entry.
func (b *knowledgeBridge) KnowledgeBaseOf(ctx context.Context, knowledgeID string) (string, bool, error) {
	k, err := b.svc.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil {
		if errors.Is(err, interfaces.ErrKnowledgeNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return k.KnowledgeBaseID, true, nil
}

// DeleteKnowledge removes a mirrored document.
func (b *knowledgeBridge) DeleteKnowledge(ctx context.Context, knowledgeID string) error {
	return b.svc.DeleteKnowledge(ctx, knowledgeID)
}
