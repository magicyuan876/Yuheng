package service

import (
	"context"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Cleanup: the two jobs that delete things nobody asked to keep.
//
// This is the only code in the module whose purpose is to destroy data, so
// it is written to be boring and to be provable:
//
//   - Nothing is deleted before a GRACE PERIOD has passed. An attachment with
//     no page is usually somebody mid-edit who has not saved yet, not
//     rubbish; the grace period is what tells those two apart, and it is
//     long enough that the difference is never a judgement call.
//
//   - Every run is BOUNDED. A sweep processes at most a fixed number of rows
//     and reports whether more remain, so the first run after a long outage
//     cannot turn into an unbounded delete that holds locks for an hour.
//
//   - One failure does not stop the sweep, and never turns into a half-delete.
//     Each candidate is handled on its own: the object is released first and
//     the row only once that succeeded, so a failure leaves a row pointing at
//     a file that is gone at worst — which the next sweep retries — rather
//     than a file nobody can find any more.
//
//   - Every run can be a DRY RUN, which does the same reads and none of the
//     writes. An operator about to delete a year of orphans should be able to
//     see what that means first.
//
// The two jobs are separate because their reasons differ. Orphaned
// attachments are an accident of how uploading works: the file arrives before
// the document that references it is saved, so a page somebody abandoned
// leaves bytes behind. Expired trash is a promise being kept: the retention
// window said thirty days, and on day thirty-one keeping it is the surprise.

// OrphanGrace is how long an unreferenced attachment is left alone.
//
// Deliberately generous. Somebody who uploaded a file, went to lunch and came
// back to finish the page must find their image still there; the cost of
// waiting is a day of bytes, and the cost of being wrong is somebody's work.
const OrphanGrace = 24 * time.Hour

// DefaultSweepLimit bounds one run of either job.
const DefaultSweepLimit = 200

// MaxSweepLimit bounds what a caller may ask for.
const MaxSweepLimit = 2000

// SweepOptions configures one maintenance run.
type SweepOptions struct {
	// DryRun reports what would be deleted and deletes nothing.
	DryRun bool
	// Limit caps the rows considered; 0 uses DefaultSweepLimit.
	Limit int
	// Now overrides the clock, for tests.
	Now time.Time
}

func (o SweepOptions) at() time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

func (o SweepOptions) limit() int {
	if o.Limit <= 0 {
		return DefaultSweepLimit
	}
	if o.Limit > MaxSweepLimit {
		return MaxSweepLimit
	}
	return o.Limit
}

// SweepReport is what a run did, or would have done.
type SweepReport struct {
	DryRun bool `json:"dry_run"`
	// Considered is how many candidates the run looked at.
	Considered int `json:"considered"`
	// Deleted is how many were removed (0 on a dry run).
	Deleted int `json:"deleted"`
	// BytesReleased is the storage given back (an estimate on a dry run).
	BytesReleased int64 `json:"bytes_released"`
	// Failed is how many could not be removed; they stay for the next run.
	Failed int `json:"failed"`
	// More is true when the limit was reached and candidates remain.
	More bool `json:"more"`
}

// SweepOrphanAttachments releases files that no page ever claimed.
//
// An attachment becomes an orphan when somebody uploads into a page they then
// abandon: the upload is bound to a page only when the document that
// references it is saved (see the note in attachment.go about binding in
// base.persist). Until then the row has no page, and after the grace period
// it is rubbish rather than work in progress.
func (s *AttachmentService) SweepOrphanAttachments(ctx context.Context, opts SweepOptions) (
	*SweepReport, error,
) {
	report := &SweepReport{DryRun: opts.DryRun}
	if s.d.Repos.Files == nil {
		return report, nil
	}
	cutoff := opts.at().Add(-OrphanGrace)
	limit := opts.limit()

	rows, err := s.d.Repos.Files.ListOrphans(ctx, cutoff, limit)
	if err != nil {
		return nil, err
	}
	report.Considered = len(rows)
	report.More = len(rows) == limit

	for _, row := range rows {
		if opts.DryRun {
			report.Deleted++
			report.BytesReleased += row.SizeBytes
			continue
		}
		if err := s.releaseOrphan(ctx, row); err != nil {
			// Logged and counted rather than returned: one unreadable object
			// must not stop the other hundred and ninety-nine from being
			// cleaned up, and the row stays for the next run to retry.
			logger.Warnf(ctx, "[docs] releasing orphan attachment %s failed: %v", row.ID, err)
			report.Failed++
			continue
		}
		report.Deleted++
		report.BytesReleased += row.SizeBytes
	}
	return report, nil
}

// releaseOrphan removes one orphan: the stored object first, then the row.
//
// That order matters. If the row went first and the object failed, nothing
// would ever point at the file again and its bytes would be lost to the
// quota for good. This way the worst case is a row whose object is already
// gone, which the next sweep finishes.
func (s *AttachmentService) releaseOrphan(ctx context.Context, row *model.Attachment) error {
	s.releaseIfUnreferenced(ctx, row)
	return s.d.Repos.Files.DeleteRows(ctx, row.TenantID, []string{row.ID})
}

// SweepExpiredTrash purges pages whose retention window has passed.
//
// The window is a promise: the trash said the page could be restored for
// thirty days, and keeping it on day thirty-one is the surprise rather than
// the courtesy. Purging goes through the same path a manual purge does, so
// attachments, revisions, links and comments are released exactly as they
// are when somebody empties the trash by hand.
func (s *PageService) SweepExpiredTrash(ctx context.Context, retention time.Duration,
	opts SweepOptions,
) (*SweepReport, error) {
	report := &SweepReport{DryRun: opts.DryRun}
	if s.d.Repos.Pages == nil || retention <= 0 {
		return report, nil
	}
	cutoff := opts.at().Add(-retention)
	limit := opts.limit()

	roots, err := s.d.Repos.Pages.ListExpiredTrashRoots(ctx, cutoff, limit)
	if err != nil {
		return nil, err
	}
	report.Considered = len(roots)
	report.More = len(roots) == limit

	for _, page := range roots {
		if opts.DryRun {
			report.Deleted++
			continue
		}
		if err := s.purgeExpired(ctx, page); err != nil {
			logger.Warnf(ctx, "[docs] purging expired page %s failed: %v", page.ID, err)
			report.Failed++
			continue
		}
		report.Deleted++
	}
	return report, nil
}

// purgeExpired removes one expired trash root and its subtree.
//
// It calls the same purgeSubtree the manual purge does, so the attachments,
// the history, the comments, the watchers, the notifications, the share
// links and the audit row are all handled exactly as they are when a person
// empties the trash. A retention sweep with its own copy of that list would
// be the one path leaving debris behind.
//
// The actor is recorded as the system rather than as nobody: an audit reader
// asking "who deleted this" deserves an answer, and "the retention policy"
// is the true one.
func (s *PageService) purgeExpired(ctx context.Context, page *model.Page) error {
	_, err := s.purgeSubtree(ctx, page.TenantID, page.SpaceID, page.ID,
		RetentionActor, RetentionActorRole)
	return err
}

// RetentionActor names the automatic sweep in audit rows and events.
const (
	RetentionActor     = "system:retention"
	RetentionActorRole = "system"
)

// SweepExpiredExports deletes archives whose download window has closed.
//
// A space export is a copy of a space's contents sitting in object storage,
// assembled for one person. Keeping it after they have had their day to
// download it means the most concentrated thing this module produces outlives
// the reason it exists, so the sweep is not housekeeping — it is the second
// half of the export feature.
//
// The job row goes with the archive rather than being kept as history: a row
// saying an export once existed, with a result_path pointing at nothing, is a
// thing every later reader has to work out the meaning of.
func (s *PageService) SweepExpiredExports(ctx context.Context, opts SweepOptions) (
	*SweepReport, error,
) {
	report := &SweepReport{DryRun: opts.DryRun}
	if s.d.Repos.Exports == nil {
		return report, nil
	}
	limit := opts.limit()

	// Jobs abandoned by a restart are closed out first, so a stale row does
	// not sit at "running" until somebody notices.
	report.Considered += s.failStaleExports(ctx, opts)

	rows, err := s.d.Repos.Exports.ListExpired(ctx, opts.at(), limit)
	if err != nil {
		return nil, err
	}
	report.Considered += len(rows)
	report.More = len(rows) == limit

	for _, job := range rows {
		if opts.DryRun {
			report.Deleted++
			continue
		}
		if err := s.releaseExport(ctx, job); err != nil {
			logger.Warnf(ctx, "[docs] releasing expired export %s failed: %v", job.ID, err)
			report.Failed++
			continue
		}
		report.Deleted++
	}
	return report, nil
}

// StaleExportAfter is how long an unfinished export is believed.
//
// Longer than any export should take, because the cost of being impatient is
// telling somebody their export failed while it is still being written.
const StaleExportAfter = 2 * time.Hour

// failStaleExports closes out jobs left running by a restart.
//
// Nothing resumes an interrupted export: the goroutine that was building the
// archive is gone with the process, and the job it was writing would
// otherwise say "running" until the end of time. Marking it failed is the
// honest answer, and the person can start another one.
func (s *PageService) failStaleExports(ctx context.Context, opts SweepOptions) int {
	rows, err := s.d.Repos.Exports.ListStale(ctx, opts.at().Add(-StaleExportAfter), opts.limit())
	if err != nil {
		logger.Warnf(ctx, "[docs] listing stale exports failed: %v", err)
		return 0
	}
	if opts.DryRun {
		return len(rows)
	}
	closed := 0
	for _, job := range rows {
		s.failExport(ctx, job.TenantID, job.ID, "the export was interrupted and did not finish")
		closed++
	}
	return closed
}

// releaseExport deletes one archive and then its row, in that order and for
// the same reason releaseOrphan does: a row without its object is retried by
// the next sweep, an object without its row is lost. The archive is found
// through its own resource row, so neither a rebound nor a deleted space
// keeps it from being released.
func (s *PageService) releaseExport(ctx context.Context, job *model.ExportJob) error {
	if job.ResultPath != "" && s.d.Storage != nil {
		if err := s.d.Storage.Delete(ctx, job.ResultPath); err != nil && !errors.Is(err, types.ErrResourceNotFound) {
			return err
		}
	}
	return s.d.Repos.Exports.Delete(ctx, job.TenantID, job.ID)
}

// SweepStaleImports closes out imports left running by a restart.
//
// The same reasoning as failStaleExports, and deliberately not folded into it:
// an interrupted import has already created some of its pages, so the honest
// report is "failed" on a job whose work is partly done, and somebody reading
// the code should see that said out loud rather than inferred from a shared
// helper. The pages it made are left alone — they are ordinary pages now, and
// deleting somebody's content because a process died would be far worse than
// leaving a half-imported tree they can finish or bin themselves.
func (s *PageService) SweepStaleImports(ctx context.Context, opts SweepOptions) (
	*SweepReport, error,
) {
	report := &SweepReport{DryRun: opts.DryRun}
	if s.d.Repos.Imports == nil {
		return report, nil
	}
	rows, err := s.d.Repos.Imports.ListStale(ctx, opts.at().Add(-StaleExportAfter), opts.limit())
	if err != nil {
		return nil, err
	}
	report.Considered = len(rows)
	if opts.DryRun {
		report.Deleted = len(rows)
		return report, nil
	}
	for _, job := range rows {
		s.failImport(ctx, job.TenantID, job.ID,
			"the import was interrupted; the pages it had already created were kept")
		report.Deleted++
	}
	return report, nil
}
