package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"path"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/export"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Exporting a whole space.
//
// A single page renders in milliseconds and is returned from the request
// that asked for it (export.go). A space cannot be: it is a thousand pages,
// a zip, and a storage write. So this is a job — started by one request,
// polled by another, and finished in the background.
//
// ---- decision: the export belongs to the person who asked for it
//
// A space export contains exactly the pages ITS REQUESTER could read, which
// is not the same set another member would get. That makes the finished
// archive personal: only its creator may download it. Anything else would
// turn "export the space" into a way to obtain somebody else's view of it,
// and the admin who exported a space with three restricted subtrees would be
// handing them out without knowing.
//
// ---- decision: the archive expires
//
// It is a copy of a space's contents sitting in object storage. Keeping it
// for ever means the most sensitive thing this module produces outlives the
// reason it was made. The maintenance sweep deletes them after ExportTTL.

// ExportTTL is how long a finished archive stays downloadable.
const ExportTTL = 24 * time.Hour

// MaxExportPages bounds one export. Past this, a zip is not the right tool
// and the person wants a database dump.
const MaxExportPages = 5000

// ExportJobView is a job as a client sees it.
type ExportJobView struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
	Format  string `json:"format"`
	Status  string `json:"status"`
	// FileName is what the archive is called once it is ready.
	FileName string `json:"file_name,omitempty"`
	// Exported and Skipped count what happened; Skipped is the pages the
	// requester could not read, which is information they already have.
	Exported int    `json:"exported"`
	Skipped  int    `json:"skipped"`
	Error    string `json:"error,omitempty"`
	// Ready is true when the archive can be downloaded.
	Ready      bool       `json:"ready"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// exportStats is what goes in the job's stats column.
type exportStats struct {
	Exported int `json:"exported"`
	Skipped  int `json:"skipped"`
}

// StartSpaceExport begins an export and returns the job to poll.
func (s *PageService) StartSpaceExport(ctx context.Context, actor *acl.Identity,
	space *model.Space, role model.SpaceRole, rawFormat string,
) (*ExportJobView, error) {
	// Reading the space is enough to export it: the archive contains only
	// what this person could already open, page by page.
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Exports == nil || s.d.Storage == nil {
		return nil, forbidden("exporting a space is not available in this deployment")
	}
	format, err := export.ParseFormat(rawFormat)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}

	job := &model.ExportJob{
		TenantID: space.TenantID, SpaceID: space.ID,
		Format: string(format), Status: model.JobPending,
		Stats: model.JSON("{}"),
	}
	if id := actorID(actor); id != "" {
		job.CreatedBy = &id
	}
	if err := s.d.Repos.Exports.Create(ctx, job); err != nil {
		return nil, err
	}

	s.audit(ctx, audit.Entry{
		TenantID: space.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.Exported, SpaceID: space.ID,
		TargetType: audit.TargetSpace, TargetID: space.ID,
	})

	// Run it detached from the request. The context is deliberately NOT the
	// request's: the caller gets their job id immediately and closes the
	// connection, and a cancelled request must not cancel an export halfway
	// through writing an archive.
	go s.runSpaceExport(context.WithoutCancel(ctx), actor, space, format, job.ID)

	return exportJobView(job), nil
}

// ExportJob reports on a job, for polling.
func (s *PageService) ExportJob(ctx context.Context, actor *acl.Identity, jobID string) (
	*ExportJobView, error,
) {
	job, err := s.ownedExportJob(ctx, actor, jobID)
	if err != nil {
		return nil, err
	}
	return exportJobView(job), nil
}

// DownloadExport returns a finished archive.
func (s *PageService) DownloadExport(ctx context.Context, actor *acl.Identity, jobID string) (
	*ExportResult, error,
) {
	job, err := s.ownedExportJob(ctx, actor, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status != model.JobSucceeded && job.Status != model.JobPartial {
		return nil, conflict("this export is not ready")
	}
	if job.ExpiresAt != nil && !time.Now().Before(*job.ExpiresAt) {
		return nil, notFound("export")
	}

	if s.d.Storage == nil {
		return nil, notFound("export")
	}
	reader, _, err := s.d.Storage.Open(ctx, job.ResultPath)
	if err != nil {
		return nil, notFound("export")
	}
	defer func() { _ = reader.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	return &ExportResult{
		FileName: job.FileName, MediaType: "application/zip", Content: buf.Bytes(),
	}, nil
}

// ownedExportJob loads a job and refuses anybody but its creator.
//
// The archive holds one person's view of a space; see the decision note.
func (s *PageService) ownedExportJob(ctx context.Context, actor *acl.Identity, jobID string) (
	*model.ExportJob, error,
) {
	if s.d.Repos.Exports == nil {
		return nil, notFound("export")
	}
	job, err := s.d.Repos.Exports.Get(ctx, actor.TenantID, jobID)
	if err != nil {
		return nil, notFound("export")
	}
	if job.CreatedBy == nil || *job.CreatedBy != actor.UserID {
		// Not found rather than forbidden: whether somebody else's export
		// exists is not this endpoint's to confirm.
		return nil, notFound("export")
	}
	return job, nil
}

// runSpaceExport builds the archive.
func (s *PageService) runSpaceExport(ctx context.Context, actor *acl.Identity,
	space *model.Space, format export.Format, jobID string,
) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] exporting space %s panicked: %v", space.ID, r)
			s.failExport(ctx, space.TenantID, jobID, "the export failed unexpectedly")
		}
	}()

	s.markExport(ctx, space.TenantID, jobID, model.JobRunning, nil)

	archive, stats, err := s.buildSpaceArchive(ctx, actor, space, format)
	if err != nil {
		logger.Warnf(ctx, "[docs] exporting space %s failed: %v", space.ID, err)
		s.failExport(ctx, space.TenantID, jobID, "the export could not be built")
		return
	}

	files, err := s.writerFor(ctx, space)
	if err != nil {
		s.failExport(ctx, space.TenantID, jobID, "no storage is configured for this space")
		return
	}
	fileName := export.FileName(space.Name, space.Slug) + ".zip"
	// The stored name carries the job id so that two exports of the same
	// space never land on the same object; the name a browser sees is the
	// plain one, set on the download response.
	storedPath, err := files.SaveBytes(ctx, archive, space.TenantID, jobID+"-"+fileName, false)
	if err != nil {
		logger.Warnf(ctx, "[docs] storing the export of space %s failed: %v", space.ID, err)
		s.failExport(ctx, space.TenantID, jobID, "the export could not be stored")
		return
	}

	job, err := s.d.Repos.Exports.Get(ctx, space.TenantID, jobID)
	if err != nil {
		return
	}
	finished := time.Now()
	expires := finished.Add(ExportTTL)
	job.Status = model.JobSucceeded
	if stats.Skipped > 0 {
		// Partial rather than succeeded: an archive missing pages should say
		// so, even though the missing ones are exactly the ones this person
		// could not read anyway.
		job.Status = model.JobPartial
	}
	job.ResultPath = storedPath
	job.FileName = fileName
	job.FinishedAt = &finished
	job.ExpiresAt = &expires
	if encoded, err := json.Marshal(stats); err == nil {
		job.Stats = model.JSON(encoded)
	}
	if err := s.d.Repos.Exports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] recording the export of space %s failed: %v", space.ID, err)
	}
}

// buildSpaceArchive walks the space and writes a zip.
func (s *PageService) buildSpaceArchive(ctx context.Context, actor *acl.Identity,
	space *model.Space, format export.Format,
) ([]byte, exportStats, error) {
	var stats exportStats
	pages, err := s.allSpacePages(ctx, space)
	if err != nil {
		return nil, stats, err
	}

	// Two passes. The first decides every file's path, so that the second
	// can turn a link between two pages into a relative path — impossible in
	// one pass, because a page may link to one that has not been placed yet.
	namer := export.NewNamer()
	paths := map[string]string{}
	readable := make([]*model.Page, 0, len(pages))
	for _, page := range pages {
		d, err := s.d.Resolver.Decide(ctx, actor, page)
		if err != nil || d.Role == model.RoleNone {
			stats.Skipped++
			continue
		}
		readable = append(readable, page)
	}
	// Parents before children: a child's folder is named after its parent's
	// file, so allocating in the order the repository happened to return
	// would file some children at the top level for no reason.
	readable = parentsFirst(readable)
	for _, page := range readable {
		dir := s.exportDir(page, paths)
		paths[page.ID] = namer.Allocate(dir, page.Title, page.ShortID, format.Extension())
	}

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, page := range readable {
		body, err := s.renderForArchive(ctx, actor, page, format, paths)
		if err != nil {
			logger.Warnf(ctx, "[docs] rendering page %s for export failed: %v", page.ID, err)
			stats.Skipped++
			continue
		}
		writer, err := archive.Create(paths[page.ID])
		if err != nil {
			return nil, stats, err
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			return nil, stats, err
		}
		stats.Exported++
	}
	if err := archive.Close(); err != nil {
		return nil, stats, err
	}
	return buf.Bytes(), stats, nil
}

// parentsFirst orders pages so that every page follows its parent.
//
// Pages whose parent is not in the list at all (the roots, and the children
// of pages this requester cannot read) come first: they are the roots of the
// forest that is actually being exported.
func parentsFirst(pages []*model.Page) []*model.Page {
	present := make(map[string]bool, len(pages))
	for _, page := range pages {
		present[page.ID] = true
	}
	children := map[string][]*model.Page{}
	var queue []*model.Page
	for _, page := range pages {
		if page.ParentID == nil || !present[*page.ParentID] {
			queue = append(queue, page)
			continue
		}
		children[*page.ParentID] = append(children[*page.ParentID], page)
	}

	out := make([]*model.Page, 0, len(pages))
	for len(queue) > 0 {
		page := queue[0]
		queue = queue[1:]
		out = append(out, page)
		queue = append(queue, children[page.ID]...)
		delete(children, page.ID)
	}
	// A cycle would strand pages here. It cannot happen — the move check
	// refuses one — but dropping pages from an export on the strength of
	// that would be the wrong way to be wrong.
	for _, rest := range children {
		out = append(out, rest...)
	}
	return out
}

// exportDir is the folder a page's file goes in: its parent's file name
// without the extension, so a subtree becomes a folder tree.
func (s *PageService) exportDir(page *model.Page, paths map[string]string) string {
	if page.ParentID == nil {
		return ""
	}
	parentPath, ok := paths[*page.ParentID]
	if !ok {
		// The parent is not in this archive — a very large space truncated at
		// MaxExportPages between the two. The page goes at the top level
		// rather than into a folder named after a file that is not there.
		return ""
	}
	return trimExtension(parentPath)
}

func trimExtension(p string) string {
	ext := path.Ext(p)
	if ext == "" {
		return p
	}
	return p[:len(p)-len(ext)]
}

// renderForArchive renders one page with links rewritten to relative paths.
func (s *PageService) renderForArchive(ctx context.Context, actor *acl.Identity,
	page *model.Page, format export.Format, paths map[string]string,
) (string, error) {
	content := page.Content
	if len(content) == 0 {
		content = EmptyDocument
	}
	node, _, err := schema.Default().Validate(content)
	if err != nil {
		return "", err
	}
	opts, err := s.exportOptions(ctx, actor, page, node)
	if err != nil {
		return "", err
	}

	// Inside an archive a page link points at the other file, when that file
	// is in the archive. Otherwise it falls back to the title-or-placeholder
	// rule exportOptions already applied.
	self := paths[page.ID]
	opts.PageURL = func(targetID string) string {
		if target, ok := paths[targetID]; ok {
			return export.RelativeLink(self, target)
		}
		return ""
	}

	if format == export.FormatHTML {
		return render.HTML(node, opts), nil
	}
	return render.Markdown(node, opts), nil
}

// allSpacePages reads every live page of a space, bounded.
func (s *PageService) allSpacePages(ctx context.Context, space *model.Space) ([]*model.Page, error) {
	var out []*model.Page
	after := ""
	for len(out) < MaxExportPages {
		batch, err := s.d.Repos.Pages.ListSpaceSummaries(ctx, space.TenantID, space.ID, 500, after)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		// The summary columns exclude the body, which an export needs.
		for _, summary := range batch {
			full, err := s.d.Repos.Pages.Get(ctx, space.TenantID, summary.ID)
			if err != nil {
				continue
			}
			out = append(out, full)
		}
		if len(batch) < 500 {
			break
		}
		after = batch[len(batch)-1].ID
	}
	return out, nil
}

func (s *PageService) markExport(ctx context.Context, tenantID uint64, jobID string,
	status model.JobStatus, finished *time.Time,
) {
	job, err := s.d.Repos.Exports.Get(ctx, tenantID, jobID)
	if err != nil {
		return
	}
	job.Status = status
	job.FinishedAt = finished
	if err := s.d.Repos.Exports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] updating export job %s failed: %v", jobID, err)
	}
}

func (s *PageService) failExport(ctx context.Context, tenantID uint64, jobID, reason string) {
	job, err := s.d.Repos.Exports.Get(ctx, tenantID, jobID)
	if err != nil {
		return
	}
	finished := time.Now()
	job.Status = model.JobFailed
	// The stored reason is deliberately general: an export failure message
	// is shown to somebody who cannot act on a storage error, and the detail
	// is in the log where an operator will look for it.
	job.Error = reason
	job.FinishedAt = &finished
	if err := s.d.Repos.Exports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] failing export job %s failed: %v", jobID, err)
	}
}

func exportJobView(job *model.ExportJob) *ExportJobView {
	view := &ExportJobView{
		ID: job.ID, SpaceID: job.SpaceID, Format: job.Format,
		Status: string(job.Status), FileName: job.FileName, Error: job.Error,
		CreatedAt: job.CreatedAt, FinishedAt: job.FinishedAt, ExpiresAt: job.ExpiresAt,
		Ready: job.Status == model.JobSucceeded || job.Status == model.JobPartial,
	}
	var stats exportStats
	if len(job.Stats) > 0 && json.Unmarshal(job.Stats, &stats) == nil {
		view.Exported, view.Skipped = stats.Exported, stats.Skipped
	}
	return view
}
