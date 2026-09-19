package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// orphan uploads a file and leaves it unbound, aged to look like it was
// uploaded `age` ago.
func (p *pageEnv) orphan(t *testing.T, age time.Duration) *model.Attachment {
	t.Helper()
	row := &model.Attachment{
		TenantID: 1, SpaceID: p.space.ID, FileName: "stray.png",
		Mime: "image/png", SizeBytes: 1024,
		FilePath: "docs/1/" + p.space.ID + "/" + randomWord(t),
	}
	require.NoError(t, p.repos.Files.Create(ctx(), row))
	require.NoError(t, p.ageAttachment(row.ID, time.Now().Add(-age)))
	return row
}

// ageAttachment backdates created_at, which is what the grace period reads.
func (p *pageEnv) ageAttachment(id string, at time.Time) error {
	return p.gorm.Exec("UPDATE docs_attachments SET created_at = ? WHERE id = ?", at, id).Error
}

// pageRowExists asks the database directly, because Pages.Get excludes the
// trash and these assertions are about whether the row survived a purge.
func (p *pageEnv) pageRowExists(t *testing.T, id string) bool {
	t.Helper()
	var n int64
	require.NoError(t, p.gorm.Raw(
		"SELECT COUNT(*) FROM docs_pages WHERE id = ?", id).Scan(&n).Error)
	return n > 0
}

func randomWord(t *testing.T) string {
	t.Helper()
	id, err := newShortID()
	require.NoError(t, err)
	return id
}

// The grace period is what tells "somebody mid-edit" from "rubbish".
func TestAFreshOrphanIsLeftAlone(t *testing.T) {
	p := newPageEnv(t)
	row := p.orphan(t, time.Hour)

	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Considered, "an hour old is somebody still working")

	_, err = p.repos.Files.Get(ctx(), 1, row.ID)
	require.NoError(t, err, "and it is still there")
}

func TestAnOldOrphanIsSweptAway(t *testing.T) {
	p := newPageEnv(t)
	row := p.orphan(t, OrphanGrace+time.Hour)

	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Considered)
	assert.Equal(t, 1, report.Deleted)
	assert.Equal(t, 0, report.Failed)
	assert.EqualValues(t, 1024, report.BytesReleased)

	_, err = p.repos.Files.Get(ctx(), 1, row.ID)
	require.Error(t, err, "the row is gone")
}

// An operator about to delete a year of orphans should see what that means
// before it happens.
func TestADryRunDeletesNothing(t *testing.T) {
	p := newPageEnv(t)
	row := p.orphan(t, OrphanGrace+time.Hour)

	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{DryRun: true})
	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Equal(t, 1, report.Deleted, "it reports what it would do")
	assert.EqualValues(t, 1024, report.BytesReleased)

	_, err = p.repos.Files.Get(ctx(), 1, row.ID)
	require.NoError(t, err, "and does none of it")
}

// A file the document actually uses is not an orphan, however old.
func TestAnAttachmentBoundToAPageIsNeverSwept(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	row := &model.Attachment{
		TenantID: 1, SpaceID: p.space.ID, PageID: &page.Page.ID,
		FileName: "kept.png", Mime: "image/png", SizeBytes: 2048,
		FilePath: "docs/1/" + p.space.ID + "/kept",
	}
	require.NoError(t, p.repos.Files.Create(ctx(), row))
	require.NoError(t, p.ageAttachment(row.ID, time.Now().Add(-10*OrphanGrace)))

	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Considered)

	_, err = p.repos.Files.Get(ctx(), 1, row.ID)
	require.NoError(t, err)
}

// The first run after a long outage must not become an unbounded delete.
func TestASweepIsBoundedAndSaysWhenMoreRemain(t *testing.T) {
	p := newPageEnv(t)
	for i := 0; i < 5; i++ {
		p.orphan(t, OrphanGrace+time.Hour)
	}

	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, report.Considered)
	assert.Equal(t, 2, report.Deleted)
	assert.True(t, report.More, "and it says there is more to do")

	rest, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 3, rest.Considered)
	assert.False(t, rest.More)
}

func TestASweepOfNothingIsNotAnError(t *testing.T) {
	p := newPageEnv(t)
	report, err := p.svc.Files.SweepOrphanAttachments(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Considered)
	assert.Equal(t, 0, report.Deleted)
	assert.False(t, report.More)
}

func TestTheSweepLimitIsClamped(t *testing.T) {
	assert.Equal(t, DefaultSweepLimit, SweepOptions{}.limit())
	assert.Equal(t, 50, SweepOptions{Limit: 50}.limit())
	assert.Equal(t, MaxSweepLimit, SweepOptions{Limit: MaxSweepLimit * 10}.limit())
	assert.Equal(t, DefaultSweepLimit, SweepOptions{Limit: -1}.limit())
}

// ---- expired trash -------------------------------------------------------------

// trashAged deletes a page and backdates its deleted_at.
func (p *pageEnv) trashAged(t *testing.T, pageID string, age time.Duration) {
	t.Helper()
	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, pageID))
	require.NoError(t, err)
	require.NoError(t, p.gorm.Exec(
		"UPDATE docs_pages SET deleted_at = ? WHERE id = ? OR parent_id = ?",
		time.Now().Add(-age), pageID, pageID).Error)
}

func TestTrashInsideTheWindowIsKept(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Recently binned")
	p.trashAged(t, page.Page.ID, 24*time.Hour)

	report, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour, SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Considered, "the promise was thirty days")
}

// The window is a promise; keeping it on day thirty-one is the surprise.
func TestExpiredTrashIsPurged(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Long gone")
	p.trashAged(t, page.Page.ID, 31*24*time.Hour)

	report, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour, SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Considered)
	assert.Equal(t, 1, report.Deleted)

	assert.False(t, p.pageRowExists(t, page.Page.ID), "it is really gone")
}

func TestExpiredTrashDryRunPurgesNothing(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Long gone")
	p.trashAged(t, page.Page.ID, 31*24*time.Hour)

	report, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour,
		SweepOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Deleted)

	assert.True(t, p.pageRowExists(t, page.Page.ID), "and it is still restorable")
}

// A subtree is purged from its top, so the sweep must not also offer the
// children as separate candidates.
func TestOnlyTrashRootsAreOffered(t *testing.T) {
	p := newPageEnv(t)
	parent := p.create(t, p.alice, nil, "Parent")
	child := p.create(t, p.alice, &parent.Page.ID, "Child")
	p.trashAged(t, parent.Page.ID, 31*24*time.Hour)

	report, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour,
		SweepOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Considered, "the parent only")

	// And purging it really takes the child.
	done, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour, SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, done.Deleted)
	assert.False(t, p.pageRowExists(t, child.Page.ID))
}

// The retention sweep goes through the same purge as a person emptying the
// trash, so everything hanging off the page goes with it.
func TestAnExpiredPurgeReleasesEverythingAManualOneDoes(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "With baggage")

	file := &model.Attachment{
		TenantID: 1, SpaceID: p.space.ID, PageID: &page.Page.ID,
		FileName: "a.png", Mime: "image/png", SizeBytes: 512,
		FilePath: "docs/1/" + p.space.ID + "/a",
	}
	require.NoError(t, p.repos.Files.Create(ctx(), file))

	p.trashAged(t, page.Page.ID, 31*24*time.Hour)
	_, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour, SweepOptions{})
	require.NoError(t, err)

	_, err = p.repos.Files.Get(ctx(), 1, file.ID)
	require.Error(t, err, "the attachment row went with the page")
}

// "Who deleted this" deserves an answer, and "the retention policy" is it.
func TestAnExpiredPurgeIsAuditedAsTheSystem(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Long gone")
	p.trashAged(t, page.Page.ID, 31*24*time.Hour)

	_, err := p.svc.Pages.SweepExpiredTrash(ctx(), 30*24*time.Hour, SweepOptions{})
	require.NoError(t, err)

	found := false
	for _, row := range p.audit.snapshot() {
		if row.Action == "docs.page.purged" && row.ActorUserID == RetentionActor {
			found = true
		}
	}
	assert.True(t, found, "the sweep names itself in the audit trail")
}

// A retention of zero switches the sweep off rather than deleting everything,
// which is the direction a misconfiguration should fail in.
func TestARetentionOfZeroSweepsNothing(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Long gone")
	p.trashAged(t, page.Page.ID, 365*24*time.Hour)

	report, err := p.svc.Pages.SweepExpiredTrash(ctx(), 0, SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Considered)

	assert.True(t, p.pageRowExists(t, page.Page.ID), "nothing was touched")
}
