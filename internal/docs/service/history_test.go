package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/history"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// textBody builds a one-paragraph document with a stable block id, so a
// structural diff has something to pair on.
func textBody(blockID, text string) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type":    "paragraph",
			"attrs":   map[string]any{"id": blockID},
			"content": []any{map[string]any{"type": "text", "text": text}},
		}},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

// revisions lists a page's history as the service returns it.
func (p *pageEnv) revisions(t *testing.T, pageID string) []*RevisionView {
	t.Helper()
	page, err := p.svc.Pages.History(ctx(), p.alice, p.decision(t, p.alice, pageID), "", 0)
	require.NoError(t, err)
	return page.Items
}

// backdate moves a page's newest snapshot into the past, which is how a test
// reaches the far side of an interval without waiting.
func (p *pageEnv) backdate(t *testing.T, pageID string, by time.Duration) {
	t.Helper()
	last, err := p.repos.History.Latest(ctx(), 1, pageID, false)
	require.NoError(t, err)
	require.NoError(t, p.repos.DB().Model(&model.PageRevision{}).
		Where("id = ?", last.ID).
		Update("created_at", last.CreatedAt.Add(-by)).Error)
}

func TestTheFirstEditOfAPageIsRecorded(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "first words"))

	rows := p.revisions(t, page.Page.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Version)
	assert.Equal(t, model.RevisionInterval, rows[0].Reason)
	assert.Equal(t, "Notes", rows[0].Title)
}

// A history full of identical entries is worse than no history at all.
func TestASaveThatChangedNothingAddsNoVersion(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	body := textBody("blockaa", "first words")

	p.save(t, page.Page.ID, page.Page.YDocVersion, body)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, body)

	assert.Len(t, p.revisions(t, page.Page.ID), 1)
}

func TestFurtherEditsWaitForTheInterval(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "one"))
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "two"))
	assert.Len(t, p.revisions(t, page.Page.ID), 1, "too soon for a second version")

	// Once the interval has passed, the next edit is kept.
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current = p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "three"))

	rows := p.revisions(t, page.Page.ID)
	require.Len(t, rows, 2)
	assert.Equal(t, 2, rows[0].Version, "newest first")
}

// Acceptance (T3.1): restoring goes through the same entry point every other
// non-typed write uses, so anybody with the page open sees it happen.
func TestRestoringPutsTheOlderTextBack(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "the original"))
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "the replacement"))

	rows := p.revisions(t, page.Page.ID)
	require.Len(t, rows, 2)
	first := rows[len(rows)-1]

	_, err := p.svc.Pages.RestoreRevision(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), first.ID)
	require.NoError(t, err)

	restored := p.mustPage(t, page.Page.ID)
	assert.Contains(t, restored.TextContent, "the original")
	assert.NotContains(t, restored.TextContent, "the replacement")
}

// And it can be undone: the state before a restore is kept, so going back is
// not a way to lose what was there.
func TestRestoringIsItselfUndoable(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "version one"))
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "version two"))

	rows := p.revisions(t, page.Page.ID)
	oldest := rows[len(rows)-1]
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.RestoreRevision(ctx(), p.alice, d, oldest.ID)
	require.NoError(t, err)
	assert.Contains(t, p.mustPage(t, page.Page.ID).TextContent, "version one")

	// The state before the restore is in the history, so the restore can be
	// restored away.
	after := p.revisions(t, page.Page.ID)
	var twoID string
	for _, row := range after {
		detail, err := p.svc.Pages.Revision(ctx(), p.alice, d, row.ID)
		require.NoError(t, err)
		if string(detail.Content) != "" && contains(string(detail.Content), "version two") {
			twoID = row.ID
			break
		}
	}
	require.NotEmpty(t, twoID, "what the page said before the restore is still in history")

	_, err = p.svc.Pages.RestoreRevision(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), twoID)
	require.NoError(t, err)
	assert.Contains(t, p.mustPage(t, page.Page.ID).TextContent, "version two")
}

func TestARestoreIsNamedInTheHistory(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "one"))
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "two"))

	rows := p.revisions(t, page.Page.ID)
	_, err := p.svc.Pages.RestoreRevision(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), rows[len(rows)-1].ID)
	require.NoError(t, err)

	var reasons []model.RevisionReason
	for _, row := range p.revisions(t, page.Page.ID) {
		reasons = append(reasons, row.Reason)
	}
	assert.Contains(t, reasons, model.RevisionRestore, "the restore is a landmark somebody can find")
}

// Naming another page's revision must not read it.
func TestARevisionOfAnotherPageIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	mine := p.create(t, p.alice, nil, "Mine")
	theirs := p.create(t, p.alice, nil, "Theirs")

	p.save(t, theirs.Page.ID, theirs.Page.YDocVersion, textBody("blockaa", "secret"))
	other := p.revisions(t, theirs.Page.ID)[0]

	_, err := p.svc.Pages.Revision(ctx(), p.alice, p.decision(t, p.alice, mine.Page.ID), other.ID)
	require.Error(t, err)

	_, err = p.svc.Pages.RestoreRevision(ctx(), p.alice, p.decision(t, p.alice, mine.Page.ID), other.ID)
	require.Error(t, err, "nor write it into a page it does not belong to")
}

func TestReadersMayReadHistoryButNotRestoreIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "words"))

	d := p.decision(t, p.carol, page.Page.ID)
	rows, err := p.svc.Pages.History(ctx(), p.carol, d, "", 0)
	require.NoError(t, err)
	require.Len(t, rows.Items, 1, "a reader can see what the page used to say")

	_, err = p.svc.Pages.RestoreRevision(ctx(), p.carol, d, rows.Items[0].ID)
	require.Error(t, err, "but cannot write it back")
}

func TestHistoryPagesThroughACursor(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	for i := 0; i < 5; i++ {
		current := p.mustPage(t, page.Page.ID)
		p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", fmt.Sprintf("edit %d", i)))
		p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	}
	d := p.decision(t, p.alice, page.Page.ID)

	first, err := p.svc.Pages.History(ctx(), p.alice, d, "", 2)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.NotEmpty(t, first.NextCursor)

	second, err := p.svc.Pages.History(ctx(), p.alice, d, first.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, second.Items, 2)

	// No overlap, and still newest first.
	assert.NotEqual(t, first.Items[0].ID, second.Items[0].ID)
	assert.Greater(t, first.Items[1].Version, second.Items[0].Version)
}

func TestAMalformedCursorIsRefusedRatherThanIgnored(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.History(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), "not-a-cursor", 0)
	require.Error(t, err)
}

func TestDiffingAVersionAgainstThePageAsItStands(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "the original line"))
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "the rewritten line"))

	rows := p.revisions(t, page.Page.ID)
	d := p.decision(t, p.alice, page.Page.ID)

	diff, err := p.svc.Pages.Diff(ctx(), p.alice, d, rows[len(rows)-1].ID, "")
	require.NoError(t, err)
	assert.Nil(t, diff.To, "the newer side is the page itself")
	assert.Equal(t, 1, diff.LineSummary.Added)
	assert.Equal(t, 1, diff.LineSummary.Removed)
	assert.Equal(t, 1, diff.BlockSummary.Changed, "one paragraph, rewritten rather than replaced")
}

func TestDiffingTwoVersions(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.save(t, page.Page.ID, page.Page.YDocVersion, textBody("blockaa", "one"))
	p.backdate(t, page.Page.ID, history.Interval+time.Minute)
	current := p.mustPage(t, page.Page.ID)
	p.save(t, page.Page.ID, current.YDocVersion, textBody("blockaa", "two"))

	rows := p.revisions(t, page.Page.ID)
	require.Len(t, rows, 2)
	d := p.decision(t, p.alice, page.Page.ID)

	diff, err := p.svc.Pages.Diff(ctx(), p.alice, d, rows[1].ID, rows[0].ID)
	require.NoError(t, err)
	require.NotNil(t, diff.To)
	assert.Equal(t, rows[0].ID, diff.To.ID)
	assert.Equal(t, 1, diff.BlockSummary.Changed)
}

// contains is strings.Contains, spelled out so the import list stays short.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
