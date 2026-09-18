package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/stretchr/testify/require"
)

// newReplaceEnv is the exclusive-edit shape: no collaboration service, so a
// replace writes the body directly.
func newReplaceEnv(t *testing.T) *pageEnv {
	t.Helper()
	return newPageEnvWith(t)
}

// newReplaceCollabEnv is the standard shape, where a replace is applied to the
// live document by the collaboration service.
func newReplaceCollabEnv(t *testing.T) (*pageEnv, *recordingCollab) {
	t.Helper()
	client := newRecordingCollab()
	p := newPageEnvWith(t, func(d *Deps) {
		d.Collab = client
		d.CollabURL = "ws://collab:1234"
	})
	return p, client
}

// Acceptance (T1.7): with a collaboration service, a replace goes through it
// so everyone with the page open sees the change and can undo it; the server
// never writes the body behind its back.
func TestReplaceGoesThroughTheCollaborationServiceWhenThereIsOne(t *testing.T) {
	p, client := newReplaceCollabEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)

	res, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, d, ReplaceInput{
		Content: docBody("imported text"), Reason: ReplaceReasonImport,
	})
	require.NoError(t, err)
	require.Equal(t, AppliedCollab, res.Applied)
	require.Equal(t, int64(1), res.YDocVersion)

	calls := client.calls()
	require.Len(t, calls, 1)
	require.Equal(t, page.ID, calls[0].pageID)
	require.Equal(t, ReplaceReasonImport, calls[0].reason)
	require.Contains(t, calls[0].content, "imported text")

	// The database is written by the service's own store callback, not here,
	// which is what keeps the Yjs state and the JSON from diverging.
	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.NotContains(t, string(stored.Content), "imported text")

	require.Contains(t, p.eventTypes(), events.PageReplaced)
	require.True(t, p.audit.has(audit.PageReplaced))
}

// Acceptance (T1.7): a replace must not fork the page. If the collaboration
// service cannot apply it, nothing is written at all.
func TestReplaceFailsRatherThanWritingBehindTheCollaborationService(t *testing.T) {
	p, client := newReplaceCollabEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	before, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)

	client.failReplace = fmt.Errorf("connection refused")
	_, err = p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Content: docBody("would fork the page"),
	})
	require.Error(t, err)

	after, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, before.YDocVersion, after.YDocVersion, "a failed replace must change nothing")
	require.Equal(t, string(before.Content), string(after.Content))
	require.NotContains(t, p.eventTypes(), events.PageReplaced)
}

// Without a collaboration service the body is written directly and the Yjs
// state is cleared, so the next client rebuilds a document from it.
func TestReplaceWritesDirectlyAndClearsTheYjsStateWhenThereIsNoService(t *testing.T) {
	p := newReplaceEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	// Give the page a Yjs state first, the way editing it would.
	_, err := p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0,
		YDoc: []byte("old-state"), Content: docBody("typed by hand"), EditorIDs: []string{"alice"},
	})
	require.NoError(t, err)

	res, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Content: docBody("restored text"), Reason: ReplaceReasonRestore,
	})
	require.NoError(t, err)
	require.Equal(t, AppliedDirect, res.Applied)
	require.Equal(t, int64(2), res.YDocVersion)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Contains(t, string(stored.Content), "restored text")
	require.Equal(t, "restored text", stored.TextContent, "the search text follows the body")
	require.Empty(t, stored.YDoc, "the stale Yjs state must not survive the replace")

	// And that is exactly what makes the next editor pick up the new body.
	state, err := p.svc.Leases.LoadYDoc(ctx(), p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	require.Empty(t, state.YDoc)
	require.Contains(t, string(state.Content), "restored text")
	require.Equal(t, int64(2), state.YDocVersion)
}

func TestReplaceAcceptsMarkdown(t *testing.T) {
	p := newReplaceEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	_, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Markdown: "# Heading\n\nSome words.\n",
	})
	require.NoError(t, err)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Contains(t, string(stored.Content), "heading")
	require.Contains(t, stored.TextContent, "Some words.")

	// The two forms are mutually exclusive, as everywhere else in the module.
	_, err = p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Content: docBody("x"), Markdown: "# y",
	})
	require.Equal(t, 400, httpCode(t, err))
}

func TestReplaceValidatesTheBodyLikeEveryOtherWrite(t *testing.T) {
	p := newReplaceEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)

	_, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, d, ReplaceInput{
		Content: json.RawMessage(`{"type":"doc","content":[{"type":"evilNode"}]}`),
	})
	require.Equal(t, 400, httpCode(t, err))

	// An empty request empties the page, which is a legitimate outcome.
	res, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, d, ReplaceInput{})
	require.NoError(t, err)
	require.Positive(t, res.YDocVersion)
	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, 0, stored.WordCount)
}

func TestReplaceNeedsWriteAccessAndRefusesALockedPage(t *testing.T) {
	p := newReplaceEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	_, err := p.svc.Pages.ReplaceContent(ctx(), p.carol, p.decision(t, p.carol, page.ID),
		ReplaceInput{Content: docBody("nope")})
	require.Equal(t, 403, httpCode(t, err))

	_, err = p.svc.Pages.ReplaceContent(ctx(), p.viewer, p.decision(t, p.viewer, page.ID),
		ReplaceInput{Content: docBody("nope")})
	require.Equal(t, 404, httpCode(t, err))

	require.NoError(t, p.repos.Pages.UpdateMeta(ctx(), 1, page.ID, map[string]any{"is_locked": true}))
	_, err = p.svc.Pages.ReplaceContent(ctx(), p.bob, p.decision(t, p.bob, page.ID),
		ReplaceInput{Content: docBody("nope")})
	require.Equal(t, 403, httpCode(t, err))

	// A space admin still may, as everywhere else.
	_, err = p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID),
		ReplaceInput{Content: docBody("admin override")})
	require.NoError(t, err)
}

// The provenance recorded for a write is the server's to decide; a caller
// cannot claim its REST edit was an import or a history restore.
func TestReplaceReasonVocabularyIsClosed(t *testing.T) {
	require.Equal(t, ReplaceReasonREST, cleanReason(""))
	require.Equal(t, ReplaceReasonREST, cleanReason("something-else"))
	require.Equal(t, ReplaceReasonImport, cleanReason(ReplaceReasonImport))
	require.Equal(t, ReplaceReasonRestore, cleanReason(" restore "))
	require.Equal(t, ReplaceReasonAI, cleanReason(ReplaceReasonAI))
}

// A replace claims the attachments its new body references, the same way a
// save does, so an imported page's images belong to it straight away.
func TestReplaceBindsTheAttachmentsItsBodyReferences(t *testing.T) {
	e := newAttachEnv(t, 0)
	page := e.create(t, e.alice, nil, "Page")
	view, err := e.put(t, e.alice, "figure.png", testPNG(t, 8, 8, 200), "")
	require.NoError(t, err)
	require.Empty(t, view.PageID)

	body := json.RawMessage(fmt.Sprintf(
		`{"type":"doc","content":[{"type":"image","attrs":{"attachmentId":%q,"src":null}}]}`, view.ID))
	_, err = e.svc.Pages.ReplaceContent(ctx(), e.alice, e.decision(t, e.alice, page.ID), ReplaceInput{
		Content: body, Reason: ReplaceReasonImport,
	})
	require.NoError(t, err)

	rows, err := e.repos.Files.ListByPage(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, view.ID, rows[0].ID)
}

// Replacing twice in a row must keep working: the second write starts from the
// version the first one produced, not from the one the caller first read.
func TestReplaceSucceedsAgainstAPageThatMovedOn(t *testing.T) {
	p := newReplaceEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	stale := p.decision(t, p.alice, page.ID)

	first, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, stale, ReplaceInput{Content: docBody("one")})
	require.NoError(t, err)

	// The same, now-stale decision is reused on purpose: an importer holds one
	// for a whole batch, and a bounded retry is what makes that work.
	second, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, stale, ReplaceInput{Content: docBody("two")})
	require.NoError(t, err)
	require.Equal(t, first.YDocVersion+1, second.YDocVersion)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Contains(t, string(stored.Content), "two")
}
