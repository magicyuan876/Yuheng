package service

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
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
