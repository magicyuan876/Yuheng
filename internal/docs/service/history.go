package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/history"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Page history: taking snapshots, reading them, comparing them, going back.
//
// The two decisions — when a save is worth keeping and which snapshots are
// worth keeping afterwards — live in internal/docs/history, free of storage
// and of the clock. What is here is the wiring: read the last snapshot, ask
// the policy, write a row, and the three read paths a history page needs.
//
// Restoring is deliberately not a special kind of write. It goes through
// ReplaceContent like an import or an AI write-back does, which is what makes
// the people with the page open see it happen rather than find out when they
// next reload, and what puts the restored state on the undo stack so it can
// be undone like any other edit.

// RevisionView is one entry in a history listing. It never carries the body;
// a listing shows who and when, and loading every body to draw it would read
// the whole history of the page.
type RevisionView struct {
	ID        string               `json:"id"`
	PageID    string               `json:"page_id"`
	Version   int                  `json:"version"`
	Title     string               `json:"title"`
	Icon      string               `json:"icon,omitempty"`
	Reason    model.RevisionReason `json:"reason"`
	EditorIDs []string             `json:"editor_ids"`
	Editors   []UserView           `json:"editors,omitempty"`
	CreatedBy string               `json:"created_by,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
	// WordCount is the snapshot's own size, so a listing can show a page
	// growing or shrinking without loading anything.
	WordCount int `json:"word_count"`
}

// RevisionPage is one page of history.
type RevisionPage struct {
	Items []*RevisionView `json:"items"`
	// NextCursor is empty when the listing is complete.
	NextCursor string `json:"next_cursor,omitempty"`
}

// RevisionDetail is one snapshot with its body.
type RevisionDetail struct {
	*RevisionView
	Content json.RawMessage `json:"content"`
}

// DiffView is a comparison of two versions.
type DiffView struct {
	// From and To name what was compared. To is empty when the newer side is
	// the page as it stands now.
	From *RevisionView `json:"from,omitempty"`
	To   *RevisionView `json:"to,omitempty"`
	// Lines is the text comparison; Blocks the structural one.
	Lines  []history.LineChange  `json:"lines"`
	Blocks []history.BlockChange `json:"blocks"`
	// Summaries count each, so a caller can show "+12 −3" without walking.
	LineSummary  history.DiffSummary `json:"line_summary"`
	BlockSummary history.DiffSummary `json:"block_summary"`
}

// ---- taking snapshots --------------------------------------------------------

// snapshotInput is what the save path knows at the moment it has written.
type snapshotInput struct {
	page   *model.Page
	node   *schema.Node
	raw    json.RawMessage
	text   string
	words  int
	reason model.RevisionReason
	// editors is who has edited since the last save, as the transport
	// reported them.
	editors []string
	// actor is who caused a deliberate snapshot; empty for an interval one,
	// which has no single author.
	actor string
	// changed is false when the save re-encoded an unchanged body.
	changed bool
}

// snapshot records a version of the page if the policy says this save is one.
//
// Called from the shared persist path, so it covers every way a body is
// written. Like the other derived work there, it is never allowed to fail a
// save: a missing snapshot costs one entry in a list, and the next save takes
// one.
func (b *base) snapshot(ctx context.Context, in snapshotInput) {
	if b.d.Repos.History == nil || in.page == nil {
		return
	}
	last, err := b.d.Repos.History.Latest(ctx, in.page.TenantID, in.page.ID, false)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		logger.Warnf(ctx, "[docs] reading the last revision of page %s failed: %v", in.page.ID, err)
		return
	}

	var previous *history.Snapshot
	if last != nil {
		previous = &history.Snapshot{
			ID: last.ID, CreatedAt: last.CreatedAt, Reason: last.Reason,
			EditorIDs: []string(last.EditorIDs),
		}
	}
	candidate := history.Candidate{
		ContentChanged: in.changed,
		Reason:         in.reason,
		PageCreatedAt:  in.page.CreatedAt,
		EditorIDs:      in.editors,
		Empty:          strings.TrimSpace(in.text) == "",
	}
	if !history.ShouldSnapshot(candidate, previous, time.Now().UTC()) {
		return
	}
	// The policy said yes; now check the one thing it cannot see, which is
	// whether this body is already the newest snapshot.
	//
	// It usually is not, and this read costs nothing in the common case
	// because it only happens when a snapshot was going to be written anyway.
	// It matters on the replace path with a collaboration service, where the
	// store callback writes the body before the caller that asked for the
	// replace can say what it was for: without this, that content would be
	// snapshotted twice, once anonymously and once by name.
	if previous != nil {
		if full, err := b.d.Repos.History.Latest(ctx, in.page.TenantID, in.page.ID, true); err == nil {
			if sameDocument(full.Content, in.raw) {
				b.retagIfMoreSpecific(ctx, in, full)
				return
			}
		}
	}

	rev := &model.PageRevision{
		PageID: in.page.ID, TenantID: in.page.TenantID, SpaceID: in.page.SpaceID,
		Title: in.page.Title, Icon: in.page.Icon,
		Content: model.JSON(in.raw), TextContent: in.text,
		EditorIDs: model.StringList(dedupe(in.editors)),
		Reason:    in.reason,
	}
	if in.actor != "" {
		actor := in.actor
		rev.CreatedBy = &actor
	}
	if err := b.d.Repos.History.Create(ctx, rev); err != nil {
		logger.Warnf(ctx, "[docs] writing a revision of page %s failed: %v", in.page.ID, err)
		return
	}
	b.publish(ctx, events.New(events.RevisionCreated, in.page.TenantID).
		WithSpace(in.page.SpaceID).WithPage(in.page.ID).WithActor(in.actor).
		With("revision_id", rev.ID).With("version", rev.Version).With("reason", string(rev.Reason)))

	b.compact(ctx, in.page)
}

// retagIfMoreSpecific renames an existing snapshot when this save knows
// better why it happened.
//
// An interval snapshot is what a save is called when nothing said otherwise.
// If the same content arrives again moments later as a restore or an import,
// that is what the entry should say — and renaming it is more honest than
// leaving the anonymous one and adding an identical entry beside it.
func (b *base) retagIfMoreSpecific(ctx context.Context, in snapshotInput, existing *model.PageRevision) {
	deliberate := in.reason != "" && in.reason != model.RevisionInterval
	if !deliberate || existing.Reason != model.RevisionInterval {
		return
	}
	if err := b.d.Repos.History.Retag(ctx, in.page.TenantID, existing.ID, in.reason, in.actor); err != nil {
		logger.Warnf(ctx, "[docs] retagging revision %s failed: %v", existing.ID, err)
	}
}

// compact thins a page's older history once it has grown past the recent
// window. Cheap to call on every snapshot: below the threshold the policy
// returns nothing and no delete is issued.
func (b *base) compact(ctx context.Context, page *model.Page) {
	rows, err := b.d.Repos.History.All(ctx, page.TenantID, page.ID)
	if err != nil || len(rows) <= history.KeepRecent {
		return
	}
	snaps := make([]history.Snapshot, 0, len(rows))
	for _, row := range rows {
		snaps = append(snaps, history.Snapshot{
			ID: row.ID, CreatedAt: row.CreatedAt, Reason: row.Reason,
		})
	}
	drop := history.Compact(snaps, time.Now().UTC())
	if len(drop) == 0 {
		return
	}
	if _, err := b.d.Repos.History.DeleteMany(ctx, page.TenantID, page.ID, drop); err != nil {
		logger.Warnf(ctx, "[docs] compacting the history of page %s failed: %v", page.ID, err)
	}
}

// revisionReasonFor maps a replace reason onto a snapshot reason.
func revisionReasonFor(replaceReason string) model.RevisionReason {
	switch replaceReason {
	case ReplaceReasonImport:
		return model.RevisionImport
	case ReplaceReasonRestore:
		return model.RevisionRestore
	case ReplaceReasonAI:
		// An AI write-back is a machine's draft, not a landmark somebody
		// chose; it is kept like any other deliberate write so it can be
		// undone, and named for what it was.
		return model.RevisionManual
	default:
		return model.RevisionInterval
	}
}

// ---- reading -----------------------------------------------------------------

// History lists a page's snapshots, newest first.
func (s *PageService) History(ctx context.Context, actor *acl.Identity, d acl.Decision,
	cursor string, limit int,
) (*RevisionPage, error) {
	if s.d.Repos.History == nil {
		return &RevisionPage{Items: []*RevisionView{}}, nil
	}
	after, err := decodeRevisionCursor(cursor)
	if err != nil {
		return nil, invalid("cursor: %v", err)
	}
	rows, err := s.d.Repos.History.ListPage(ctx, d.Page.TenantID, d.Page.ID, after, limit)
	if err != nil {
		return nil, err
	}

	out := &RevisionPage{Items: make([]*RevisionView, 0, len(rows))}
	for _, row := range rows {
		out.Items = append(out.Items, s.revisionView(row))
	}
	s.attachEditors(ctx, out.Items)
	if len(rows) > 0 && (limit <= 0 || len(rows) >= limit) {
		last := rows[len(rows)-1]
		out.NextCursor = encodeRevisionCursor(repository.RevisionCursor{
			CreatedAt: last.CreatedAt, ID: last.ID,
		})
	}
	return out, nil
}

// Revision returns one snapshot with its body.
//
// The caller's access to the *page* governs, which is the only sensible rule:
// a snapshot is that page at an earlier moment, and somebody who may read the
// page may read what it used to say.
func (s *PageService) Revision(ctx context.Context, actor *acl.Identity, d acl.Decision,
	revisionID string,
) (*RevisionDetail, error) {
	row, err := s.loadRevision(ctx, d, revisionID)
	if err != nil {
		return nil, err
	}
	return &RevisionDetail{
		RevisionView: s.revisionView(row),
		Content:      json.RawMessage(row.Content),
	}, nil
}

// Diff compares two versions of a page.
//
// `to` may be empty, which compares the snapshot against the page as it
// stands now — the comparison somebody actually wants when looking at an old
// version and wondering what has happened since.
func (s *PageService) Diff(ctx context.Context, actor *acl.Identity, d acl.Decision,
	fromID, toID string,
) (*DiffView, error) {
	from, err := s.loadRevision(ctx, d, fromID)
	if err != nil {
		return nil, err
	}

	var (
		toView *RevisionView
		toRaw  json.RawMessage
		toText string
	)
	if toID == "" {
		page, err := s.d.Repos.Pages.Get(ctx, d.Page.TenantID, d.Page.ID)
		if err != nil {
			return nil, err
		}
		toRaw, toText = json.RawMessage(page.Content), page.TextContent
	} else {
		to, err := s.loadRevision(ctx, d, toID)
		if err != nil {
			return nil, err
		}
		toView = s.revisionView(to)
		toRaw, toText = json.RawMessage(to.Content), to.TextContent
	}

	// A body that will not parse is compared as an empty document rather than
	// failing the request: history holds what was stored, including whatever
	// an older version of this software stored.
	beforeNode, _, _ := schema.Default().Validate(json.RawMessage(from.Content))
	afterNode, _, _ := schema.Default().Validate(toRaw)

	lines := history.DiffText(from.TextContent, toText)
	blocks := history.DiffBlocks(beforeNode, afterNode)
	return &DiffView{
		From:         s.revisionView(from),
		To:           toView,
		Lines:        lines,
		Blocks:       blocks,
		LineSummary:  history.SummariseLines(lines),
		BlockSummary: history.SummariseBlocks(blocks),
	}, nil
}

// ---- restoring ---------------------------------------------------------------

// RestoreRevision puts a page back to what a snapshot says.
//
// Named apart from Restore, which brings a page back from the trash: one
// restores a page, the other restores its contents.
//
// Two things happen in order, and the order is the point. The page as it
// stands is snapshotted first, so restoring is itself undoable and nobody can
// lose the current text by going back to an old one. Then the body is written
// through ReplaceContent, the same entry point an import uses — so anybody
// with the page open watches it change rather than finding out on reload, and
// the restored state joins their undo stack like any other edit.
func (s *PageService) RestoreRevision(ctx context.Context, actor *acl.Identity, d acl.Decision,
	revisionID string,
) (*ReplaceResult, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	row, err := s.loadRevision(ctx, d, revisionID)
	if err != nil {
		return nil, err
	}

	// Keep what is about to be replaced. Marked manual rather than restore:
	// this entry is the state before the restore, not the restore itself.
	current, err := s.d.Repos.Pages.Get(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	if node, _, err := schema.Default().Validate(json.RawMessage(current.Content)); err == nil {
		s.snapshot(ctx, snapshotInput{
			page: current, node: node, raw: json.RawMessage(current.Content),
			text: current.TextContent, words: current.WordCount,
			reason: model.RevisionManual, actor: actorID(actor), changed: true,
		})
	}

	res, err := s.ReplaceContent(ctx, actor, d, ReplaceInput{
		Content: json.RawMessage(row.Content), Reason: ReplaceReasonRestore,
	})
	if err != nil {
		return nil, err
	}

	s.publish(ctx, events.New(events.PageRestored, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("revision_id", row.ID).With("version", row.Version))
	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.PageRestoredTo, SpaceID: d.Page.SpaceID,
		TargetType: "docs_page", TargetID: d.Page.ID,
	})
	return res, nil
}

// ---- helpers -----------------------------------------------------------------

// loadRevision reads a snapshot and checks it belongs to the page the caller
// was granted access to. Naming another page's revision id must not read it.
func (s *PageService) loadRevision(ctx context.Context, d acl.Decision, revisionID string) (
	*model.PageRevision, error,
) {
	if s.d.Repos.History == nil {
		return nil, notFound("revision")
	}
	if strings.TrimSpace(revisionID) == "" {
		return nil, invalid("a revision id is required")
	}
	row, err := s.d.Repos.History.Get(ctx, d.Page.TenantID, revisionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("revision")
		}
		return nil, err
	}
	if row.PageID != d.Page.ID {
		return nil, notFound("revision")
	}
	return row, nil
}

func (s *PageService) revisionView(row *model.PageRevision) *RevisionView {
	view := &RevisionView{
		ID: row.ID, PageID: row.PageID, Version: row.Version,
		Title: row.Title, Reason: row.Reason,
		EditorIDs: []string(row.EditorIDs), CreatedAt: row.CreatedAt,
		WordCount: len(strings.Fields(row.TextContent)),
	}
	if row.Icon != nil {
		view.Icon = *row.Icon
	}
	if row.CreatedBy != nil {
		view.CreatedBy = *row.CreatedBy
	}
	return view
}

// attachEditors resolves the people named across a listing in one lookup,
// rather than once per entry: a page's history is mostly the same few names.
func (s *PageService) attachEditors(ctx context.Context, views []*RevisionView) {
	ids := make([]string, 0, len(views)*2)
	for _, view := range views {
		ids = append(ids, view.EditorIDs...)
		if view.CreatedBy != "" {
			ids = append(ids, view.CreatedBy)
		}
	}
	users := s.users(ctx, dedupe(ids))
	if len(users) == 0 {
		return
	}
	for _, view := range views {
		for _, id := range view.EditorIDs {
			view.Editors = append(view.Editors, userView(id, users))
		}
	}
}

// encodeRevisionCursor renders a cursor opaque, so a caller cannot page from
// an arbitrary timestamp of its own choosing.
func encodeRevisionCursor(c repository.RevisionCursor) string {
	raw, err := json.Marshal(map[string]any{
		"at": c.CreatedAt.UTC().Format(time.RFC3339Nano), "id": c.ID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeRevisionCursor(value string) (*repository.RevisionCursor, error) {
	if value == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, errors.New("malformed")
	}
	var fields struct {
		At string `json:"at"`
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, errors.New("malformed")
	}
	at, err := time.Parse(time.RFC3339Nano, fields.At)
	if err != nil || fields.ID == "" {
		return nil, errors.New("malformed")
	}
	return &repository.RevisionCursor{CreatedAt: at, ID: fields.ID}, nil
}
