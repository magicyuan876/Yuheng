package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// zipOf builds an archive in memory from path -> content.
func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func (p *pageEnv) startImport(t *testing.T, who *acl.Identity, name string, data []byte,
	parentID string,
) *ImportJobView {
	t.Helper()
	job, err := p.svc.Pages.StartImport(ctx(), who, p.space, p.mustSpaceRole(who, p.space),
		ImportInput{File: upload(t, name, data), ParentID: parentID})
	require.NoError(t, err)
	return job
}

func (p *pageEnv) waitForImport(t *testing.T, who *acl.Identity, jobID string) *ImportJobView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		view, err := p.svc.Pages.ImportJob(ctx(), who, jobID)
		require.NoError(t, err)
		if view.Done {
			return view
		}
		if time.Now().After(deadline) {
			t.Fatalf("import job %s is still %s after 10s", jobID, view.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// importedTitles lists the space's pages as "parent/child" title paths.
func (p *pageEnv) importedTitles(t *testing.T, who *acl.Identity) []string {
	t.Helper()
	summaries, err := p.repos.Pages.ListSpaceSummaries(ctx(), 1, p.space.ID, 500, "")
	require.NoError(t, err)

	titles := map[string]string{}
	parents := map[string]*string{}
	for _, s := range summaries {
		page, err := p.repos.Pages.Get(ctx(), 1, s.ID)
		require.NoError(t, err)
		titles[page.ID] = page.Title
		parents[page.ID] = page.ParentID
	}
	var out []string
	for id := range titles {
		path := titles[id]
		for parent := parents[id]; parent != nil; parent = parents[*parent] {
			path = titles[*parent] + "/" + path
		}
		out = append(out, path)
	}
	return out
}

func (p *pageEnv) contentOf(t *testing.T, title string) string {
	t.Helper()
	summaries, err := p.repos.Pages.ListSpaceSummaries(ctx(), 1, p.space.ID, 500, "")
	require.NoError(t, err)
	for _, s := range summaries {
		page, err := p.repos.Pages.Get(ctx(), 1, s.ID)
		require.NoError(t, err)
		if page.Title == title {
			return string(page.Content)
		}
	}
	t.Fatalf("no page called %q", title)
	return ""
}

func TestASingleMarkdownFileBecomesOnePage(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "Onboarding.md",
		[]byte("# Welcome aboard\n\nRead this first.\n"), "")
	done := e.waitForImport(t, e.alice, job.ID)

	assert.Equal(t, string(model.JobSucceeded), done.Status)
	assert.Equal(t, 1, done.Created)
	assert.Empty(t, done.Skipped)

	// The leading heading became the title and was consumed, so it does not
	// appear twice.
	assert.Contains(t, e.importedTitles(t, e.alice), "Welcome aboard")
	body := e.contentOf(t, "Welcome aboard")
	assert.Contains(t, body, "Read this first.")
	assert.NotContains(t, body, "Welcome aboard")
}

func TestAZipBecomesAPageTree(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "handbook.zip", zipOf(t, map[string]string{
		"Handbook.md":       "The handbook.\n",
		"Handbook/Leave.md": "Ask first.\n",
		"Notes/One.md":      "One.\n",
	}), "")
	done := e.waitForImport(t, e.alice, job.ID)

	require.Equal(t, string(model.JobSucceeded), done.Status, "skipped: %v", done.Skipped)
	titles := e.importedTitles(t, e.alice)
	assert.Contains(t, titles, "Handbook")
	assert.Contains(t, titles, "Handbook/Leave")
	// A folder with no file of its own still gets a parent page.
	assert.Contains(t, titles, "Notes/One")
}

// The rule that makes an export/import round trip stable: a file beside a
// folder of the same name is one page, not two.
func TestAnExportedBundleImportsBackToTheSameShape(t *testing.T) {
	source := newAttachEnv(t, 1<<30)
	parent := source.create(t, source.alice, nil, "Handbook")
	child := source.create(t, source.alice, &parent.ID, "Leave")
	source.write(t, source.alice, parent.ID, "the handbook")
	source.write(t, source.alice, child.ID, "ask first")

	job := source.startExport(t, source.alice, "markdown")
	source.waitForExport(t, source.alice, job.ID)
	archive, err := source.svc.Pages.DownloadExport(ctx(), source.alice, job.ID)
	require.NoError(t, err)

	target := newAttachEnv(t, 1<<30)
	imported := target.startImport(t, target.alice, "space.zip", archive.Content, "")
	done := target.waitForImport(t, target.alice, imported.ID)

	require.Equal(t, string(model.JobSucceeded), done.Status, "skipped: %v", done.Skipped)
	assert.Equal(t, 2, done.Created, "a file beside its own folder is one page")
	titles := target.importedTitles(t, target.alice)
	assert.Contains(t, titles, "Handbook")
	assert.Contains(t, titles, "Handbook/Leave")
}

// Converting the first file means knowing the id of a page the second file
// has not created yet, which is why the import runs in two passes.
func TestALinkBetweenTwoImportedFilesBecomesAPageLink(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "bundle.zip", zipOf(t, map[string]string{
		"First.md":  "See [the second](Second.md) for more.\n",
		"Second.md": "Here it is.\n",
	}), "")
	done := e.waitForImport(t, e.alice, job.ID)
	require.Equal(t, string(model.JobSucceeded), done.Status, "skipped: %v", done.Skipped)

	body := e.contentOf(t, "First")
	assert.Contains(t, body, "pageLink", "a link inside the bundle must resolve to a page")
	assert.NotContains(t, body, "Second.md", "the relative path must not survive as a dead link")
}

func TestALinkOutOfTheBundleStaysAPlainLink(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "Notes.md",
		[]byte("See [the site](https://example.test/x).\n"), "")
	e.waitForImport(t, e.alice, job.ID)

	body := e.contentOf(t, "Notes")
	assert.Contains(t, body, "https://example.test/x")
	assert.NotContains(t, body, "pageLink")
}

func TestAnImageInTheBundleBecomesAnAttachment(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "bundle.zip", zipOf(t, map[string]string{
		"Notes.md":           "![a diagram](images/diagram.png)\n",
		"images/diagram.png": string(testPNG(t, 8, 8, 30)),
	}), "")
	done := e.waitForImport(t, e.alice, job.ID)

	require.Equal(t, 1, done.Attachments, "skipped: %v", done.Skipped)
	body := e.contentOf(t, "Notes")
	assert.Contains(t, body, "image")
	assert.NotContains(t, body, "images/diagram.png", "the bundle path must be replaced by the id")
}

func TestABundleCanBeImportedUnderAnExistingPage(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	parent := e.create(t, e.alice, nil, "Archive")

	job := e.startImport(t, e.alice, "Notes.md", []byte("Some notes.\n"), parent.ID)
	e.waitForImport(t, e.alice, job.ID)

	assert.Contains(t, e.importedTitles(t, e.alice), "Archive/Notes")
}

func TestImportingNeedsTheWriterRole(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	_, err := e.svc.Pages.StartImport(ctx(), e.carol, e.space,
		e.mustSpaceRole(e.carol, e.space),
		ImportInput{File: upload(t, "Notes.md", []byte("hello"))})
	require.Error(t, err, "carol is a reader in this space")
}

func TestImportingIntoSomebodyElsesPageIsRefused(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	secret := e.create(t, e.alice, nil, "Private")
	e.restrict(t, secret.ID, "alice")

	_, err := e.svc.Pages.StartImport(ctx(), e.bob, e.space, e.mustSpaceRole(e.bob, e.space),
		ImportInput{File: upload(t, "Notes.md", []byte("hello")), ParentID: secret.ID})
	require.Error(t, err)
}

// A zip entry can say anything; a path that climbs out of the bundle must not
// be able to masquerade as another entry.
func TestAnEscapingPathIsReportedNotImported(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "bundle.zip", zipOf(t, map[string]string{
		"../../escape.md": "nope\n",
		"Fine.md":         "yes\n",
	}), "")
	done := e.waitForImport(t, e.alice, job.ID)

	assert.Equal(t, 1, done.Created)
	assert.Equal(t, string(model.JobPartial), done.Status)
	require.NotEmpty(t, done.Skipped)
	assert.Contains(t, strings.Join(done.Skipped, " "), "escape.md")
}

func TestAFileThatIsNeitherMarkdownNorAZipIsRefused(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "photo.png", testPNG(t, 8, 8, 30), "")
	done := e.waitForImport(t, e.alice, job.ID)

	assert.Equal(t, string(model.JobFailed), done.Status)
	assert.NotEmpty(t, done.Error)
}

func TestAnEmptyUploadIsRefusedBeforeAJobExists(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	_, err := e.svc.Pages.StartImport(ctx(), e.alice, e.space,
		e.mustSpaceRole(e.alice, e.space),
		ImportInput{File: &multipart.FileHeader{Filename: "empty.md"}})
	require.Error(t, err)
}

// A job reports on a space; somebody who cannot read that space cannot see it.
func TestAnImportJobIsInvisibleOutsideItsSpace(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "Notes.md", []byte("hello"), "")
	e.waitForImport(t, e.alice, job.ID)

	_, err := e.svc.Pages.ImportJob(ctx(), e.viewer, job.ID)
	require.Error(t, err)
}

// Front matter is the other place a title can come from.
func TestFrontMatterTitleIsUsedAndStripped(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "x.md",
		[]byte("---\ntitle: Expenses Policy\n---\n\nKeep the receipt.\n"), "")
	e.waitForImport(t, e.alice, job.ID)

	assert.Contains(t, e.importedTitles(t, e.alice), "Expenses Policy")
	body := e.contentOf(t, "Expenses Policy")
	assert.Contains(t, body, "Keep the receipt.")
	assert.NotContains(t, body, "title:")
}

// The imported document must be a valid document, not just bytes that were
// written into a column.
func TestImportedContentIsAValidDocument(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := e.startImport(t, e.alice, "Notes.md",
		[]byte("# T\n\n- one\n- two\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"), "")
	e.waitForImport(t, e.alice, job.ID)

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.contentOf(t, "T")), &doc))
	assert.Equal(t, "doc", doc["type"])
	assert.NotEmpty(t, doc["content"])
}

// A restart leaves a job mid-flight. Nothing resumes it, so the sweep closes
// it out — and leaves the pages it had already created alone.
func TestAnInterruptedImportIsMarkedFailedButKeepsItsPages(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	kept := e.create(t, e.alice, nil, "Half imported")

	job := &model.ImportJob{
		TenantID: 1, SpaceID: e.space.ID, Kind: ImportKindMarkdown,
		Status: model.JobRunning, Stats: model.JSON("{}"),
		SourcePath: "mem://x", FileName: "bundle.zip",
	}
	require.NoError(t, e.repos.Imports.Create(ctx(), job))
	require.NoError(t, e.gorm.Exec(
		"UPDATE docs_import_jobs SET created_at = ? WHERE id = ?",
		time.Now().Add(-3*time.Hour), job.ID).Error)

	report, err := e.svc.Pages.SweepStaleImports(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Deleted)

	view, err := e.svc.Pages.ImportJob(ctx(), e.alice, job.ID)
	require.NoError(t, err)
	assert.Equal(t, string(model.JobFailed), view.Status)

	_, err = e.repos.Pages.Get(ctx(), 1, kept.ID)
	require.NoError(t, err, "an interrupted import must not delete what it made")
}

func TestAFreshRunningImportIsNotTouchedByTheSweep(t *testing.T) {
	e := newAttachEnv(t, 1<<30)
	job := &model.ImportJob{
		TenantID: 1, SpaceID: e.space.ID, Kind: ImportKindMarkdown,
		Status: model.JobRunning, Stats: model.JSON("{}"),
		SourcePath: "mem://x", FileName: "bundle.zip",
	}
	require.NoError(t, e.repos.Imports.Create(ctx(), job))

	report, err := e.svc.Pages.SweepStaleImports(ctx(), SweepOptions{})
	require.NoError(t, err)
	assert.Zero(t, report.Deleted)
}
