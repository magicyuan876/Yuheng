package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// ReplaceContent is the single way a page's body is written by anything other
// than somebody typing into it.
//
// Import, history restore, the AI write-back and the REST content endpoint all
// come through here, and none of them touches the `content` or `ydoc` columns
// directly. That rule exists because the Yjs state is the truth and the JSON
// is its projection: a write that set the JSON without going through the Yjs
// document would leave the two telling different stories, and the next person
// to open the page would silently lose whatever was written.
//
// The two deployment shapes reach the same end by different means:
//
//   - With a collaboration service, the new body is handed to it and applied
//     as one Yjs transaction. Everyone with the page open sees it appear, and
//     it lands on their undo stack as a single step they can take back. The
//     service then persists through the usual store callback, so the write is
//     validated, rendered and indexed exactly like any other edit.
//   - Without one, the JSON is written and the stored Yjs state is cleared, so
//     the next client to open the page builds a fresh document from it. That
//     is the honest equivalent: there is no process able to merge a change
//     into a live document, and pretending otherwise would fork the page.
type ReplaceInput struct {
	// Content is a ProseMirror document; Markdown is converted first. Exactly
	// one of them may be set.
	Content  json.RawMessage
	Markdown string
	// Reason is recorded in the audit row and in the collaboration service's
	// log: "rest", "import", "restore" or "ai".
	Reason string
}

// Replace reasons. The public REST endpoint always records "rest"; the others
// belong to callers inside the server.
const (
	ReplaceReasonREST    = "rest"
	ReplaceReasonImport  = "import"
	ReplaceReasonRestore = "restore"
	ReplaceReasonAI      = "ai"
)

// ReplaceResult reports the version the page now holds.
type ReplaceResult struct {
	YDocVersion int64 `json:"ydoc_version"`
	// Applied says which path took the write: "collab" when the live document
	// was updated in place, "direct" when the stored body was replaced and the
	// Yjs state left to be rebuilt on the next load.
	Applied string `json:"applied"`
}

// Applied values.
const (
	AppliedCollab = "collab"
	AppliedDirect = "direct"
)

// maxDirectRetries bounds the optimistic-concurrency retry of the direct path.
// Contention there means somebody is saving over REST at the same instant,
// which is rare and self-resolving; a bounded retry keeps a burst of imports
// from failing for no reason without ever looping.
const maxDirectRetries = 3

// ReplaceContent writes a page body through whichever path this deployment
// has. The caller must already hold the page decision; writer is required, and
// a locked page is refused for everyone but a space admin.
func (s *PageService) ReplaceContent(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in ReplaceInput,
) (*ReplaceResult, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if !canEdit(d.Role, d.Page) {
		return nil, forbidden("the page is locked")
	}
	reason := cleanReason(in.Reason)

	parsed, err := parseBody(CreatePageInput{Content: in.Content, Markdown: in.Markdown})
	if err != nil {
		return nil, err
	}

	if s.d.Collab != nil && s.d.Collab.Configured() {
		return s.replaceThroughCollab(ctx, actor, d.Page, parsed, reason)
	}
	return s.replaceDirect(ctx, actor, d.Page, parsed, reason)
}

// cleanReason keeps the vocabulary closed; anything unrecognised is recorded
// as a plain REST write rather than allowed to claim a provenance it does not
// have.
func cleanReason(raw string) string {
	switch strings.TrimSpace(raw) {
	case ReplaceReasonImport:
		return ReplaceReasonImport
	case ReplaceReasonRestore:
		return ReplaceReasonRestore
	case ReplaceReasonAI:
		return ReplaceReasonAI
	default:
		return ReplaceReasonREST
	}
}

// replaceThroughCollab hands the body to the collaboration service.
//
// Nothing is written here: the service applies the transaction and calls back
// into the store path, which is what persists. A failure is surfaced rather
// than worked around, because writing the JSON behind the service's back would
// leave every client that has the page open holding a document the database no
// longer agrees with — the exact fork this whole path exists to prevent.
func (s *PageService) replaceThroughCollab(ctx context.Context, actor *acl.Identity, page *model.Page,
	parsed *body, reason string,
) (*ReplaceResult, error) {
	res, err := s.d.Collab.Replace(ctx, page.TenantID, page.ID, json.RawMessage(parsed.content), reason)
	if err != nil {
		if errors.Is(err, collab.ErrNotConfigured) {
			// Configured() said otherwise a moment ago; treat it as absent.
			return s.replaceDirect(ctx, actor, page, parsed, reason)
		}
		return nil, fmt.Errorf("docs: the collaboration service could not apply the new body: %w", err)
	}
	s.afterReplace(ctx, actor, page, reason, res.YDocVersion)
	return &ReplaceResult{YDocVersion: res.YDocVersion, Applied: AppliedCollab}, nil
}

// replaceDirect writes the body and clears the Yjs state.
//
// Clearing is the point rather than a shortcut: an empty state is what makes
// the next client build a fresh document from this JSON. Leaving the old state
// in place and writing only the JSON would give the next reader the previous
// body, since the state is what the editor actually loads.
func (s *PageService) replaceDirect(ctx context.Context, actor *acl.Identity, page *model.Page,
	parsed *body, reason string,
) (*ReplaceResult, error) {
	node, _, err := schema.Default().Validate(parsed.content)
	if err != nil {
		// parseBody validated this a moment ago; disagreeing with itself now
		// is a bug worth surfacing rather than writing around.
		return nil, fmt.Errorf("docs: the validated body no longer parses: %w", err)
	}
	attachments := render.Extract(node).AttachmentIDs

	current := page
	for attempt := 0; ; attempt++ {
		contributors := append(model.StringList{}, current.ContributorIDs...)
		if id := actorID(actor); id != "" {
			contributors = contributors.Add(id)
		}
		version, err := s.d.Repos.Pages.UpdateContent(ctx, current.TenantID, current.ID, current.YDocVersion,
			repository.ContentUpdate{
				Content:        parsed.content,
				YDoc:           nil,
				TextContent:    parsed.text,
				WordCount:      parsed.words,
				EditorID:       actorID(actor),
				ContributorIDs: contributors,
				ContentChanged: !sameDocument(current.Content, parsed.content),
			})
		if err == nil {
			s.bindAttachments(ctx, current, attachments)
			s.afterReplace(ctx, actor, current, reason, version)
			return &ReplaceResult{YDocVersion: version, Applied: AppliedDirect}, nil
		}
		if !errors.Is(err, repository.ErrConflict) || attempt >= maxDirectRetries {
			return nil, err
		}
		// Somebody persisted between reading the page and writing it. The new
		// body does not depend on the old one, so re-reading the version and
		// trying again is safe and is exactly what the caller would do.
		current, err = s.d.Repos.Pages.Get(ctx, current.TenantID, current.ID)
		if err != nil {
			return nil, err
		}
	}
}

// afterReplace records the write and tells everyone watching.
func (s *PageService) afterReplace(ctx context.Context, actor *acl.Identity, page *model.Page,
	reason string, version int64,
) {
	s.publish(ctx, events.New(events.PageReplaced, page.TenantID).
		WithSpace(page.SpaceID).WithPage(page.ID).WithActor(actorID(actor)).
		With("reason", reason).With("version", version))
	s.audit(ctx, audit.Entry{
		TenantID: page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.PageReplaced, SpaceID: page.SpaceID,
		TargetType: audit.TargetPage, TargetID: page.ID,
	})
}
