package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/importer"
	"github.com/magicyuan876/yuheng/internal/docs/markdown"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// Importing a bundle of Markdown into a space.
//
// The inverse of exportjob.go, and asynchronous for the same reason: a bundle
// is a thousand files, and unpacking, converting and writing them is not work
// that belongs inside an HTTP timeout.
//
// ---- decision: two passes, always
//
// Every page is created first, empty, and only then is each one's content
// converted and written. It is twice the writes, and it is the only way a
// link between two imported pages can become a page link: converting the
// first file means knowing the id of a page the second file has not created
// yet. One pass would silently degrade every cross-reference in the bundle
// into a dead relative path, and nobody would notice until they clicked one.
//
// ---- decision: the import runs as the person who asked
//
// Not as a system actor. Every page is created through the same service the
// editor uses, with that person's identity, so an import cannot put a page
// anywhere they could not have put it by hand, and the audit trail names
// them rather than "the importer".
//
// ---- decision: a failed file does not fail the import
//
// A bundle is usually somebody's folder of notes, and one file with markup
// this cannot represent should not cost them the other four hundred. Failed
// files are counted and named in the job's report, and the job ends partial.

// ImportKindMarkdown is the only bundle format understood so far. It covers a
// single .md file and a .zip of them, because a zip of one file and a file
// are the same import with a different number of entries.
const ImportKindMarkdown = "markdown"

// MaxImportUploadBytes bounds the upload itself, before it is unpacked.
const MaxImportUploadBytes = 256 << 20

// ImportJobView is a job as a client sees it.
type ImportJobView struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	// FileName is the bundle that was uploaded.
	FileName string `json:"file_name,omitempty"`
	// TargetParentID is the page the bundle was imported under, if any.
	TargetParentID *string `json:"target_parent_id,omitempty"`
	// Created counts the pages that now exist.
	Created int `json:"created"`
	// Attachments counts the files stored alongside them.
	Attachments int `json:"attachments"`
	// Skipped names what was left out and why, so the result is checkable
	// rather than a number to be taken on trust.
	Skipped    []string   `json:"skipped,omitempty"`
	Error      string     `json:"error,omitempty"`
	Done       bool       `json:"done"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// importStats is what goes in the job's stats column.
type importStats struct {
	Created     int      `json:"created"`
	Attachments int      `json:"attachments"`
	Skipped     []string `json:"skipped,omitempty"`
}

// ImportInput is an upload to import.
type ImportInput struct {
	File *multipart.FileHeader
	// ParentID optionally puts the bundle under an existing page rather than
	// at the top of the space.
	ParentID string
}

// StartImport accepts a bundle and begins importing it.
func (s *PageService) StartImport(ctx context.Context, actor *acl.Identity,
	space *model.Space, role model.SpaceRole, in ImportInput,
) (*ImportJobView, error) {
	if err := requireSpaceRole(role, model.RoleWriter); err != nil {
		return nil, err
	}
	if s.d.Repos.Imports == nil || s.d.Storage == nil {
		return nil, forbidden("importing is not available in this deployment")
	}
	if in.File == nil || in.File.Size <= 0 {
		return nil, invalid("a file is required")
	}
	if in.File.Size > MaxImportUploadBytes {
		return nil, invalid("the bundle is %d bytes, over the %d byte limit",
			in.File.Size, MaxImportUploadBytes)
	}

	// The target is checked here rather than in the runner: being told
	// immediately that you cannot write to that page is worth more than a job
	// that fails a minute later.
	var parentID *string
	if in.ParentID != "" {
		d, err := s.d.Resolver.Page(ctx, actor, in.ParentID)
		if err != nil {
			return nil, err
		}
		if err := requireRole(d, model.RoleWriter); err != nil {
			return nil, err
		}
		if d.Page.SpaceID != space.ID {
			return nil, invalid("the page belongs to another space")
		}
		id := d.Page.ID
		parentID = &id
	}

	// The upload is read once and kept in memory for the runner, which is
	// what it works from: the bundle had to be buffered to be stored at all,
	// and re-reading it back out of object storage would add a way for the
	// import to fail that has nothing to do with the bundle.
	//
	// It is stored anyway, and the job points at it. That copy is the record
	// of what was actually imported — the one thing that can settle "the
	// import made this page, was that what I uploaded?" — and it expires with
	// the temporary bucket rather than being kept for ever.
	data, err := readUpload(in.File)
	if err != nil {
		return nil, err
	}
	files, err := s.writerFor(ctx, space)
	if err != nil {
		return nil, forbidden("no storage is configured for this space")
	}
	storedPath, err := files.SaveBytes(ctx, data, space.TenantID,
		"docs-import-"+in.File.Filename, true)
	if err != nil {
		return nil, err
	}

	job := &model.ImportJob{
		TenantID: space.TenantID, SpaceID: space.ID,
		Kind: ImportKindMarkdown, Status: model.JobPending,
		SourcePath: storedPath, FileName: in.File.Filename,
		TargetParentID: parentID, Stats: model.JSON("{}"),
	}
	if id := actorID(actor); id != "" {
		job.CreatedBy = &id
	}
	if err := s.d.Repos.Imports.Create(ctx, job); err != nil {
		return nil, err
	}

	s.audit(ctx, audit.Entry{
		TenantID: space.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.Imported, SpaceID: space.ID,
		TargetType: audit.TargetSpace, TargetID: space.ID,
	})

	// Detached from the request, for the same reason an export is: the caller
	// has their job id and has gone.
	go s.runImport(context.WithoutCancel(ctx), actor, space, role, job.ID, data, parentID)

	return importJobView(job), nil
}

// ImportJob reports on a job, for polling.
func (s *PageService) ImportJob(ctx context.Context, actor *acl.Identity, jobID string) (
	*ImportJobView, error,
) {
	if s.d.Repos.Imports == nil {
		return nil, notFound("import")
	}
	job, err := s.d.Repos.Imports.Get(ctx, actor.TenantID, jobID)
	if err != nil {
		return nil, notFound("import")
	}
	// The job says what was created in a space; anybody who can read that
	// space may see it, which is not true of an export, whose archive holds
	// one person's view. Here the pages themselves are the product and they
	// carry their own permissions.
	space, err := s.d.Repos.Spaces.Get(ctx, job.TenantID, job.SpaceID)
	if err != nil {
		return nil, notFound("import")
	}
	if role, err := s.d.Resolver.SpaceRole(ctx, actor, space); err != nil || role == model.RoleNone {
		return nil, notFound("import")
	}
	return importJobView(job), nil
}

// readUpload reads a multipart part into memory, bounded.
func readUpload(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, invalid("the upload could not be read: %v", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxImportUploadBytes+1))
	if err != nil {
		return nil, invalid("the upload could not be read: %v", err)
	}
	if int64(len(data)) > MaxImportUploadBytes {
		return nil, invalid("the bundle is over the %d byte limit", MaxImportUploadBytes)
	}
	return data, nil
}

// runImport unpacks the bundle and creates the pages.
func (s *PageService) runImport(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, jobID string, data []byte, parentID *string,
) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[docs] importing into space %s panicked: %v", space.ID, r)
			s.failImport(ctx, space.TenantID, jobID, "the import failed unexpectedly")
		}
	}()

	s.markImport(ctx, space.TenantID, jobID, model.JobRunning)

	job, err := s.d.Repos.Imports.Get(ctx, space.TenantID, jobID)
	if err != nil {
		return
	}
	bundle, err := openBundle(job.FileName, data)
	if err != nil {
		s.failImport(ctx, space.TenantID, jobID, err.Error())
		return
	}

	stats := s.importBundle(ctx, actor, space, role, bundle, parentID)

	finished := time.Now()
	job.Status = model.JobSucceeded
	if len(stats.Skipped) > 0 {
		job.Status = model.JobPartial
	}
	if stats.Created == 0 {
		job.Status = model.JobFailed
		job.Error = "the bundle contained no documents that could be imported"
	}
	job.FinishedAt = &finished
	if encoded, err := json.Marshal(stats); err == nil {
		job.Stats = model.JSON(encoded)
	}
	if err := s.d.Repos.Imports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] recording the import into space %s failed: %v", space.ID, err)
	}
}

// bundle is an unpacked upload: every file, by path.
type bundle struct {
	entries []importer.Entry
	bodies  map[string][]byte
}

// openBundle unpacks an upload, accepting either a zip or a single document.
func openBundle(fileName string, data []byte) (*bundle, error) {
	out := &bundle{bodies: map[string][]byte{}}

	// A zip is recognised by its own bytes rather than by the file name: an
	// archive renamed to .md is still an archive, and a .md file is never a
	// zip by accident.
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		name := path.Base(strings.ReplaceAll(fileName, "\\", "/"))
		if name == "" || name == "." || name == "/" {
			name = "Imported.md"
		}
		if !importer.IsMarkdown(name) {
			return nil, fmt.Errorf("only Markdown files and zip archives can be imported")
		}
		out.entries = append(out.entries, importer.Entry{Path: name, Size: int64(len(data))})
		out.bodies[name] = data
		return out, nil
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("the archive could not be opened")
	}
	var total int64
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		size := int64(f.UncompressedSize64)
		// Checked before reading, not after: this is what stops a small
		// archive from expanding into a very large one in memory.
		if total+size > importer.MaxTotalBytes || size > importer.MaxFileBytes {
			out.entries = append(out.entries, importer.Entry{Path: f.Name, Size: size})
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(rc, importer.MaxFileBytes+1))
		_ = rc.Close()
		if err != nil {
			continue
		}
		total += int64(len(body))
		out.entries = append(out.entries, importer.Entry{Path: f.Name, Size: int64(len(body))})
		// The plan normalises paths, so the bodies are keyed the same way or
		// the two would not line up.
		out.bodies[normalizeBundlePath(f.Name)] = body
	}
	if len(out.entries) == 0 {
		return nil, fmt.Errorf("the archive is empty")
	}
	return out, nil
}

func normalizeBundlePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return path.Clean(p)
}

// importBundle creates the pages and fills them in.
func (s *PageService) importBundle(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, b *bundle, parentID *string,
) importStats {
	plan := importer.BuildPlan(b.entries)
	stats := importStats{Skipped: plan.Skipped}

	// Pass one: every page, empty, so that pass two can turn a link between
	// two of them into a page link. See the note at the top of this file.
	pageIDs := map[string]string{}
	nodeParent := map[*importer.Node]string{}
	titles := map[*importer.Node]string{}
	bodies := map[*importer.Node][]byte{}

	plan.Walk(func(node, parent *importer.Node) {
		title := node.Title
		body := b.bodies[node.Path]
		if node.Path != "" {
			title, body = importer.Title(node.Path, body)
		}
		titles[node] = title
		bodies[node] = body

		target := parentID
		if parent != nil {
			if id, ok := nodeParent[parent]; ok && id != "" {
				target = &id
			} else {
				// The parent could not be created, so this page would be
				// orphaned; skipping keeps the tree honest rather than
				// scattering children across the target.
				stats.Skipped = append(stats.Skipped, nodeLabel(node)+": its parent could not be created")
				return
			}
		}
		view, err := s.Create(ctx, actor, CreatePageInput{
			SpaceID: space.ID, ParentID: target, Title: title,
		})
		if err != nil {
			stats.Skipped = append(stats.Skipped, nodeLabel(node)+": "+err.Error())
			return
		}
		nodeParent[node] = view.ID
		if node.Path != "" {
			pageIDs[node.Path] = view.ID
		}
		stats.Created++
	})

	// Assets before pass two, so that an image's attachment id exists by the
	// time the document referencing it is converted.
	assets := s.importAssets(ctx, actor, space, role, b, plan, &stats)

	// Pass two: convert and write each document.
	plan.Walk(func(node, _ *importer.Node) {
		pageID, ok := nodeParent[node]
		if !ok || node.Path == "" {
			return
		}
		if err := s.fillImportedPage(ctx, actor, pageID, node.Path, bodies[node], pageIDs, assets); err != nil {
			stats.Skipped = append(stats.Skipped, nodeLabel(node)+": "+err.Error())
		}
	})
	return stats
}

// nodeLabel names a node in a report: its file, or the folder it stands for.
func nodeLabel(node *importer.Node) string {
	if node.Path != "" {
		return node.Path
	}
	return node.Dir + "/"
}

// importAssets stores the bundle's non-Markdown files as attachments.
func (s *PageService) importAssets(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, b *bundle, plan *importer.Plan, stats *importStats,
) map[string]string {
	out := map[string]string{}
	if s.files == nil {
		if len(plan.Assets) > 0 {
			stats.Skipped = append(stats.Skipped,
				"attachments were not imported: this deployment stores no files")
		}
		return out
	}
	for _, asset := range plan.Assets {
		body := b.bodies[asset]
		if len(body) == 0 {
			continue
		}
		view, err := s.files.UploadBytes(ctx, actor, space, role, path.Base(asset), body)
		if err != nil {
			stats.Skipped = append(stats.Skipped, asset+": "+err.Error())
			continue
		}
		out[asset] = view.ID
		stats.Attachments++
	}
	return out
}

// fillImportedPage converts one document and writes it into its page.
func (s *PageService) fillImportedPage(ctx context.Context, actor *acl.Identity,
	pageID, fromPath string, body []byte, pageIDs, assets map[string]string,
) error {
	doc, _, err := markdown.ToDocument(body, markdown.Options{
		ResolvePage: func(dest string) (string, bool) {
			id, ok := pageIDs[importer.ResolveTarget(fromPath, dest)]
			return id, ok
		},
		ResolveAttachment: func(dest string) (string, bool) {
			id, ok := assets[importer.ResolveTarget(fromPath, dest)]
			return id, ok
		},
	})
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	d, err := s.d.Resolver.Page(ctx, actor, pageID)
	if err != nil {
		return err
	}
	_, err = s.ReplaceContent(ctx, actor, d, ReplaceInput{
		Content: encoded, Reason: "import",
	})
	return err
}

func (s *PageService) markImport(ctx context.Context, tenantID uint64, jobID string,
	status model.JobStatus,
) {
	job, err := s.d.Repos.Imports.Get(ctx, tenantID, jobID)
	if err != nil {
		return
	}
	job.Status = status
	if err := s.d.Repos.Imports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] updating import job %s failed: %v", jobID, err)
	}
}

func (s *PageService) failImport(ctx context.Context, tenantID uint64, jobID, reason string) {
	job, err := s.d.Repos.Imports.Get(ctx, tenantID, jobID)
	if err != nil {
		return
	}
	finished := time.Now()
	job.Status = model.JobFailed
	job.Error = reason
	job.FinishedAt = &finished
	if err := s.d.Repos.Imports.Update(ctx, job); err != nil {
		logger.Warnf(ctx, "[docs] failing import job %s failed: %v", jobID, err)
	}
}

func importJobView(job *model.ImportJob) *ImportJobView {
	view := &ImportJobView{
		ID: job.ID, SpaceID: job.SpaceID, Kind: job.Kind,
		Status: string(job.Status), FileName: job.FileName,
		TargetParentID: job.TargetParentID, Error: job.Error,
		CreatedAt: job.CreatedAt, FinishedAt: job.FinishedAt,
		Done: job.Status != model.JobPending && job.Status != model.JobRunning,
	}
	var stats importStats
	if len(job.Stats) > 0 && json.Unmarshal(job.Stats, &stats) == nil {
		view.Created, view.Attachments, view.Skipped = stats.Created, stats.Attachments, stats.Skipped
	}
	return view
}
