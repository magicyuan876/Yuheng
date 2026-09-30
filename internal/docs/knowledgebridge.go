package docs

import (
	"context"
	"errors"
	"fmt"
	"time"

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
const DocsChannel = types.ChannelDocs

type knowledgeBridge struct {
	svc         interfaces.KnowledgeService
	tenants     interfaces.TenantRepository
	stewardship interfaces.KnowledgeStewardshipService
}

// NewKnowledgeBridge adapts Yuheng's knowledge service for the docs module.
// Without the knowledge service, or without the tenant repository the bridge
// needs to act on behalf of a tenant, it yields nil, which leaves every space
// unindexed. Without the stewardship service mirror entries are indexed and
// carry no maintainer.
func NewKnowledgeBridge(svc interfaces.KnowledgeService, tenants interfaces.TenantRepository,
	stewardship interfaces.KnowledgeStewardshipService,
) service.Knowledge {
	if svc == nil || tenants == nil {
		return nil
	}
	return &knowledgeBridge{svc: svc, tenants: tenants, stewardship: stewardship}
}

// asTenant returns a context the knowledge service will accept.
//
// The knowledge service reads the tenant, and the tenant's record, out of the
// context of a request and assumes it is there. The indexing workers call it
// from no request at all, and without this it would fail on a missing value
// instead of working. The record is loaded the way the data-source sync, the
// other background writer of knowledge, does.
func (b *knowledgeBridge) asTenant(ctx context.Context, tenantID uint64) (context.Context, error) {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	tenant, err := b.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("docs: loading tenant %d for the knowledge base: %w", tenantID, err)
	}
	return context.WithValue(ctx, types.TenantInfoContextKey, tenant), nil
}

// CreateKnowledgeFromText adds a page's Markdown as a knowledge document.
//
// Published, not saved as a draft: the knowledge service only chunks and embeds
// a manual document that is published, and a draft is stored and never becomes
// searchable, which would leave the mirror looking complete and answering
// nothing.
func (b *knowledgeBridge) CreateKnowledgeFromText(ctx context.Context, tenantID uint64, kbID, title,
	markdown string,
) (string, error) {
	ctx, err := b.asTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	created, err := b.svc.CreateKnowledgeFromManual(ctx, kbID, &types.ManualKnowledgePayload{
		Title:   title,
		Content: markdown,
		Status:  types.ManualKnowledgeStatusPublish,
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
func (b *knowledgeBridge) UpdateKnowledgeContent(ctx context.Context, tenantID uint64, knowledgeID, title,
	markdown string,
) (bool, error) {
	ctx, err := b.asTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	updated, err := b.svc.UpdateManualKnowledge(ctx, knowledgeID, &types.ManualKnowledgePayload{
		Title:   title,
		Content: markdown,
		Status:  types.ManualKnowledgeStatusPublish,
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
func (b *knowledgeBridge) DeleteKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) error {
	ctx, err := b.asTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	return b.svc.DeleteKnowledge(ctx, knowledgeID)
}

// SyncStewardship copies a page's stewardship onto its mirror entry.
func (b *knowledgeBridge) SyncStewardship(ctx context.Context, tenantID uint64, knowledgeID, ownerID,
	reviewedBy string, reviewedAt time.Time,
) error {
	if b.stewardship == nil {
		return nil
	}
	return b.stewardship.SyncFromSource(ctx, tenantID, knowledgeID, ownerID, reviewedBy, reviewedAt)
}

// pageRetirer is the docs module's interfaces.KnowledgeRetirer: a superseded
// page mirror leaves the knowledge base by its page being excluded, never by
// its entry being deleted, which the page's next synchronisation would undo.
type pageRetirer struct {
	pages *service.PageService
}

// Retire implements interfaces.KnowledgeRetirer.
func (r pageRetirer) Retire(ctx context.Context, tenantID uint64, knowledgeID string,
	replacement types.KnowledgeRef,
) (string, error) {
	if err := r.pages.RetireMirror(ctx, tenantID, knowledgeID, replacement); err != nil {
		return "", err
	}
	return types.RetiredExcluded, nil
}
