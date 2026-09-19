package service

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// waitForExport polls a job the way a client does, with a deadline so a
// deadlocked runner fails the test rather than hanging the suite.
func (p *pageEnv) waitForExport(t *testing.T, who *acl.Identity, jobID string) *ExportJobView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		view, err := p.svc.Pages.ExportJob(ctx(), who, jobID)
		require.NoError(t, err)
		if view.Status != string(model.JobPending) && view.Status != string(model.JobRunning) {
			return view
		}
		if time.Now().After(deadline) {
			t.Fatalf("export job %s is still %s after 10s", jobID, view.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// archiveOf downloads a finished export and reads its entries.
func (p *pageEnv) archiveOf(t *testing.T, who *acl.Identity, jobID string) map[string]string {
	t.Helper()
	result, err := p.svc.Pages.DownloadExport(ctx(), who, jobID)
	require.NoError(t, err)
	require.Equal(t, "application/zip", result.MediaType)

	r, err := zip.NewReader(bytes.NewReader(result.Content), int64(len(result.Content)))
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		_ = rc.Close()
		out[f.Name] = string(body)
	}
	return out
}

func (p *pageEnv) startExport(t *testing.T, who *acl.Identity, format string) *ExportJobView {
	t.Helper()
	job, err := p.svc.Pages.StartSpaceExport(ctx(), who, p.space,
		p.mustSpaceRole(who, p.space), format)
	require.NoError(t, err)
	return job
}

func TestASpaceExportContainsAFilePerPage(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	one := e.create(t, e.alice, nil, "Onboarding")
	two := e.create(t, e.alice, nil, "Expenses")
	e.write(t, e.alice, one.ID, "welcome aboard")
	e.write(t, e.alice, two.ID, "keep the receipt")

	job := e.startExport(t, e.alice, "markdown")
	assert.False(t, job.Ready, "a job is not ready the moment it is created")

	done := e.waitForExport(t, e.alice, job.ID)
	require.Equal(t, string(model.JobSucceeded), done.Status)
	assert.Equal(t, 2, done.Exported)
	assert.True(t, done.Ready)
	require.NotNil(t, done.ExpiresAt, "an archive that never expires is a leak")

	files := e.archiveOf(t, e.alice, job.ID)
	require.Len(t, files, 2)
	assert.Contains(t, files["Onboarding.md"], "welcome aboard")
	assert.Contains(t, files["Expenses.md"], "keep the receipt")
}

// A subtree becomes a folder tree, so the export is navigable on disk in the
// shape people already know from the sidebar.
func TestAChildPageIsExportedUnderItsParent(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	parent := e.create(t, e.alice, nil, "Handbook")
	child := e.create(t, e.alice, &parent.ID, "Leave")
	e.write(t, e.alice, child.ID, "ask first")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	files := e.archiveOf(t, e.alice, job.ID)
	assert.Contains(t, files, "Handbook.md")
	assert.Contains(t, files, "Handbook/Leave.md")
}

// A Chinese title stays Chinese: transliterating it would make the file
// unfindable by the person who wrote it.
func TestAChineseTitleSurvivesIntoTheArchive(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "存储配额说明")
	e.write(t, e.alice, page.ID, "每个空间都有配额")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	files := e.archiveOf(t, e.alice, job.ID)
	require.Contains(t, files, "存储配额说明.md")
	assert.Contains(t, files["存储配额说明.md"], "每个空间都有配额")
}

// The rule the whole feature rests on: an export contains what the person who
// asked could already read, and nothing else. Not the body of a restricted
// page, and not its title as a file name either.
func TestAnExportLeavesOutPagesTheRequesterCannotRead(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	open := e.create(t, e.alice, nil, "Open")
	secret := e.create(t, e.alice, nil, "Q3 Redundancies")
	e.write(t, e.alice, open.ID, "anyone may read this")
	e.write(t, e.alice, secret.ID, "the list of names")
	e.restrict(t, secret.ID, "alice")

	job := e.startExport(t, e.carol, "markdown")
	done := e.waitForExport(t, e.carol, job.ID)

	// Partial rather than succeeded: an archive missing pages says so.
	assert.Equal(t, string(model.JobPartial), done.Status)
	assert.Equal(t, 1, done.Exported)
	assert.Equal(t, 1, done.Skipped)

	files := e.archiveOf(t, e.carol, job.ID)
	require.Len(t, files, 1)
	assert.Contains(t, files, "Open.md")
	for name, body := range files {
		assert.NotContains(t, name, "Redundancies", "a title is information")
		assert.NotContains(t, body, "the list of names")
	}
}

// A page whose parent is not in the archive is filed at the top level rather
// than under a folder named after something that is not there.
//
// This is reachable without any permission trick: MaxExportPages bounds a
// run, so a very large space can be truncated between a parent and its child.
// Tested directly because arranging five thousand pages in a fixture to prove
// one branch would be a slow way to learn nothing extra.
func TestAPageWhoseParentIsMissingIsFiledAtTheTop(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	missing := "some-parent-that-was-not-exported"
	page := &model.Page{ID: "p1", Title: "Process", ParentID: &missing}

	dir := e.svc.Pages.exportDir(page, map[string]string{})
	assert.Equal(t, "", dir)
}

// And when the parent IS there, the child is filed under it — which is what
// makes the ordering in buildSpaceArchive load-bearing.
func TestParentsAreOrderedBeforeTheirChildren(t *testing.T) {
	root := &model.Page{ID: "root"}
	rootID := "root"
	child := &model.Page{ID: "child", ParentID: &rootID}
	childID := "child"
	grandchild := &model.Page{ID: "grandchild", ParentID: &childID}

	// Deliberately the wrong way round on the way in.
	got := parentsFirst([]*model.Page{grandchild, child, root})
	require.Len(t, got, 3)
	assert.Equal(t, []string{"root", "child", "grandchild"},
		[]string{got[0].ID, got[1].ID, got[2].ID})
}

// Two pages with the same title must both survive; on macOS and Windows a
// case-only difference is not a difference either.
func TestTwoPagesWithTheSameTitleBothSurvive(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	first := e.create(t, e.alice, nil, "Notes")
	second := e.create(t, e.alice, nil, "notes")
	e.write(t, e.alice, first.ID, "the first one")
	e.write(t, e.alice, second.ID, "the second one")

	job := e.startExport(t, e.alice, "markdown")
	done := e.waitForExport(t, e.alice, job.ID)
	assert.Equal(t, 2, done.Exported)

	files := e.archiveOf(t, e.alice, job.ID)
	require.Len(t, files, 2, "a collision must rename, not overwrite")
	joined := strings.Join(mapKeys(files), " ")
	assert.Contains(t, strings.ToLower(joined), "notes-2.md")
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestHTMLIsAnAcceptedArchiveFormat(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "Onboarding")
	e.write(t, e.alice, page.ID, "welcome aboard")

	job := e.startExport(t, e.alice, "html")
	e.waitForExport(t, e.alice, job.ID)

	files := e.archiveOf(t, e.alice, job.ID)
	require.Contains(t, files, "Onboarding.html")
	assert.Contains(t, files["Onboarding.html"], "<p>")
}

func TestAFormatThisCannotProduceIsRefusedBeforeAJobExists(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	_, err := e.svc.Pages.StartSpaceExport(ctx(), e.alice, e.space,
		e.mustSpaceRole(e.alice, e.space), "pdf")
	require.Error(t, err)
}

// The archive holds one person's view of a space, so it is theirs alone.
func TestSomebodyElsesExportIsNotDownloadable(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "Onboarding")
	e.write(t, e.alice, page.ID, "welcome aboard")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	_, err := e.svc.Pages.DownloadExport(ctx(), e.bob, job.ID)
	require.Error(t, err, "bob may read the space but this archive is alice's")
	// Not found rather than forbidden: whether somebody else's export exists
	// is not this endpoint's to confirm.
	assert.Contains(t, strings.ToLower(err.Error()), "not found")

	_, err = e.svc.Pages.ExportJob(ctx(), e.bob, job.ID)
	require.Error(t, err)
}

func TestAnUnfinishedExportIsNotDownloadable(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := &model.ExportJob{
		TenantID: 1, SpaceID: e.space.ID, Format: "markdown",
		Status: model.JobRunning, Stats: model.JSON("{}"),
		CreatedBy: strPtr("alice"),
	}
	require.NoError(t, e.repos.Exports.Create(ctx(), job))

	_, err := e.svc.Pages.DownloadExport(ctx(), e.alice, job.ID)
	require.Error(t, err)
}

func TestAnExpiredArchiveIsNoLongerDownloadable(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "Onboarding")
	e.write(t, e.alice, page.ID, "welcome aboard")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	row, err := e.repos.Exports.Get(ctx(), 1, job.ID)
	require.NoError(t, err)
	past := time.Now().Add(-time.Minute)
	row.ExpiresAt = &past
	require.NoError(t, e.repos.Exports.Update(ctx(), row))

	_, err = e.svc.Pages.DownloadExport(ctx(), e.alice, job.ID)
	require.Error(t, err)
}

// The second half of the feature: the archive does not outlive its window.
func TestTheSweepDeletesExpiredArchivesAndTheirRows(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "Onboarding")
	e.write(t, e.alice, page.ID, "welcome aboard")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	row, err := e.repos.Exports.Get(ctx(), 1, job.ID)
	require.NoError(t, err)
	require.NotEmpty(t, row.ResultPath)
	storedPath := row.ResultPath
	past := time.Now().Add(-time.Minute)
	row.ExpiresAt = &past
	require.NoError(t, e.repos.Exports.Update(ctx(), row))

	// A dry run reports and destroys nothing, which is what an operator
	// about to delete a day of archives needs to be able to check.
	dry, err := e.svc.Pages.SweepExpiredExports(ctx(), SweepOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, dry.Deleted)
	_, err = e.repos.Exports.Get(ctx(), 1, job.ID)
	require.NoError(t, err, "a dry run must not delete the row")

	report, err := e.svc.Pages.SweepExpiredExports(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Deleted)
	assert.Zero(t, report.Failed)

	assert.Contains(t, e.files.deleted, storedPath, "the archive itself must go")
	_, err = e.repos.Exports.Get(ctx(), 1, job.ID)
	require.Error(t, err, "the row goes with the archive")
}

func TestAnArchiveStillWithinItsWindowIsLeftAlone(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	page := e.create(t, e.alice, nil, "Onboarding")
	e.write(t, e.alice, page.ID, "welcome aboard")

	job := e.startExport(t, e.alice, "markdown")
	e.waitForExport(t, e.alice, job.ID)

	report, err := e.svc.Pages.SweepExpiredExports(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Zero(t, report.Deleted)
	_, err = e.svc.Pages.DownloadExport(ctx(), e.alice, job.ID)
	require.NoError(t, err)
}

// A process restart leaves a job mid-flight; nothing resumes it, so the sweep
// closes it out rather than leaving "running" on the screen for ever.
func TestAnInterruptedExportIsEventuallyMarkedFailed(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := &model.ExportJob{
		TenantID: 1, SpaceID: e.space.ID, Format: "markdown",
		Status: model.JobRunning, Stats: model.JSON("{}"),
		CreatedBy: strPtr("alice"),
	}
	require.NoError(t, e.repos.Exports.Create(ctx(), job))
	// Backdate it past the patience window.
	require.NoError(t, e.gorm.Exec(
		"UPDATE docs_export_jobs SET created_at = ? WHERE id = ?",
		time.Now().Add(-3*time.Hour), job.ID).Error)

	_, err := e.svc.Pages.SweepExpiredExports(ctx(), SweepOptions{})
	require.NoError(t, err)

	view, err := e.svc.Pages.ExportJob(ctx(), e.alice, job.ID)
	require.NoError(t, err)
	assert.Equal(t, string(model.JobFailed), view.Status)
	assert.NotEmpty(t, view.Error)
}

// A job that has only just started is left alone.
func TestAFreshRunningExportIsNotTouchedBytheSweep(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := &model.ExportJob{
		TenantID: 1, SpaceID: e.space.ID, Format: "markdown",
		Status: model.JobRunning, Stats: model.JSON("{}"),
		CreatedBy: strPtr("alice"),
	}
	require.NoError(t, e.repos.Exports.Create(ctx(), job))

	_, err := e.svc.Pages.SweepExpiredExports(ctx(), SweepOptions{})
	require.NoError(t, err)

	view, err := e.svc.Pages.ExportJob(ctx(), e.alice, job.ID)
	require.NoError(t, err)
	assert.Equal(t, string(model.JobRunning), view.Status)
}
