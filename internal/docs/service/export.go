package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/export"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
)

// Exporting a page.
//
// The rules about what an exported file may contain are in
// internal/docs/export; this is where they meet the permission system, and
// the meeting point is one rule: an export contains exactly what the person
// asking could already read, rendered differently.
//
// That sounds obvious and is the thing most easily got wrong, because a
// document is full of references to other documents. Three of them:
//
//   - A PAGE LINK to something the caller cannot open must not render as
//     that page's title. "There is a page called Q3 Redundancies" is the
//     leak, and an export is exactly where somebody would forget it, because
//     the title is right there in the resolver.
//   - A TRANSCLUDED block is flattened into the file, so it has to be
//     resolved with the same source-page check the reader gets on screen.
//   - An ATTACHMENT stays a URL back into this installation. A single-page
//     export cannot carry bytes, and a link that needs a login is honest
//     about that in a way an inlined data URI of somebody else's image
//     would not be.

// ExportResult is a rendered file.
type ExportResult struct {
	// FileName is what a browser should save it as.
	FileName string `json:"file_name"`
	// MediaType is the Content-Type to serve it with.
	MediaType string `json:"media_type"`
	// Content is the file.
	Content []byte `json:"-"`
}

// ExportPage renders one page as a file.
func (s *PageService) ExportPage(ctx context.Context, actor *acl.Identity, d acl.Decision,
	rawFormat string,
) (*ExportResult, error) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	format, err := export.ParseFormat(rawFormat)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}

	content := d.Page.Content
	if len(content) == 0 {
		content = EmptyDocument
	}
	node, _, err := schema.Default().Validate(content)
	if err != nil {
		return nil, fmt.Errorf("docs: stored content of %s is invalid: %w", d.Page.ID, err)
	}

	opts, err := s.exportOptions(ctx, actor, d.Page, node)
	if err != nil {
		return nil, err
	}

	var body string
	if format == export.FormatHTML {
		body = render.HTML(node, opts)
	} else {
		body = render.Markdown(node, opts)
	}

	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.Exported, SpaceID: d.Page.SpaceID,
		TargetType: audit.TargetPage, TargetID: d.Page.ID,
	})

	return &ExportResult{
		FileName:  export.FileName(d.Page.Title, d.Page.ShortID) + format.Extension(),
		MediaType: format.MediaType(),
		Content:   []byte(body),
	}, nil
}

// exportOptions builds the render hooks for an export, applying the
// permission rules above.
func (s *PageService) exportOptions(ctx context.Context, actor *acl.Identity, page *model.Page,
	node *schema.Node,
) (render.Options, error) {
	structure := render.Extract(node)

	// Page links: resolve a title only for pages this caller may open.
	// Anything else renders as a neutral placeholder, because a title is
	// information.
	titles := map[string]string{}
	for _, targetID := range structure.PageLinks {
		target, err := s.d.Resolver.Page(ctx, actor, targetID)
		if err != nil || target.Role == model.RoleNone {
			continue
		}
		titles[targetID] = target.Page.Title
	}

	// Transclusions: flattened into the file, resolved through the same path
	// the reader's screen uses. Reusing ResolveTransclusions rather than
	// repeating its checks is the point — it already refuses a block whose
	// source page this caller may not open, and a second copy of that rule
	// would be the one that drifts.
	blocks := map[string]*schema.Node{}
	if len(structure.Transclusions) > 0 {
		refs := make([]repository.BlockRef, 0, len(structure.Transclusions))
		for _, ref := range structure.Transclusions {
			refs = append(refs, repository.BlockRef{
				PageID: ref.SourcePageID, BlockID: ref.SourceBlockID,
			})
		}
		views, err := s.ResolveTransclusions(ctx, actor, refs)
		if err != nil {
			return render.Options{}, err
		}
		for _, view := range views {
			if view.State != TransclusionOK || len(view.Content) == 0 {
				continue
			}
			node, err := parseBlockSnapshot(view.Content)
			if err != nil {
				// A snapshot that no longer validates is left out rather
				// than exported half-formed; the page still exports, with a
				// placeholder where the block was.
				continue
			}
			blocks[view.SourcePageID+"\x00"+view.SourceBlockID] = node
		}
	}

	return render.Options{
		PageTitle: func(id string) (string, bool) {
			title, ok := titles[id]
			return title, ok
		},
		TransclusionContent: func(pageID, blockID string) (*schema.Node, bool) {
			body, ok := blocks[pageID+"\x00"+blockID]
			return body, ok
		},
		EmbedURL: s.embedURL,
	}, nil
}

// parseBlockSnapshot validates one stored transclusion snapshot.
//
// A snapshot is a single block, and which block varies — a paragraph, a
// heading, a table. So its own type is read first and handed to the
// validator as the allowed root, which validates the content properly
// without hard-coding the list of blocks that may be referenced.
func parseBlockSnapshot(raw []byte) (*schema.Node, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	if probe.Type == "" {
		return nil, fmt.Errorf("docs: a block snapshot has no type")
	}
	node, _, err := schema.Default().Validate(raw,
		schema.ValidateOptions{AllowedRoot: probe.Type})
	if err != nil {
		return nil, err
	}
	return node, nil
}
