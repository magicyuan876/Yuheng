package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// IndexClaim is one page a worker has taken to synchronise.
type IndexClaim struct {
	TenantID uint64
	PageID   string
	// Seq is the request counter at the time of the claim; Complete and Fail use
	// it to tell whether another request arrived meanwhile.
	Seq      int64
	Attempts int
}

// IndexStateRepository is the queue of pages waiting to be mirrored into their
// knowledge base, and the record of what was last sent.
//
// Every time is the database's own clock, not the caller's: the workers run on
// several application instances whose clocks need not agree, and a lease is
// only meaningful against one clock.
type IndexStateRepository interface {
	// Get returns a page's state, or ErrNotFound if it has none yet.
	Get(ctx context.Context, tenantID uint64, pageID string) (*model.IndexState, error)
	// Mark requests that pages be synchronised. A positive delay debounces: the
	// page becomes due that long from now, unless it is already due, in which
	// case it stays so. A zero delay makes it due at once and keeps it so
	// however many later requests follow, for changes that take content away
	// from readers.
	Mark(ctx context.Context, tenantID uint64, pageIDs []string, delay time.Duration) error
	// MarkSubtree marks a page and everything beneath it, trash included.
	MarkSubtree(ctx context.Context, tenantID uint64, rootID string, delay time.Duration) error
	// MarkSpace marks every page of a space, trash included: what changes the
	// answer for all of them at once, such as the space's knowledge base.
	MarkSpace(ctx context.Context, tenantID uint64, spaceID string, delay time.Duration) error
	// Claim takes up to limit due pages that no worker holds, for the lease.
	// Concurrent callers, on this instance or others, get different pages.
	Claim(ctx context.Context, limit int, lease time.Duration) ([]IndexClaim, error)
	// Complete records that the claim's page is in step with the knowledge base,
	// with the hash of what is there. The page stays due if a request arrived
	// after the claim.
	Complete(ctx context.Context, claim IndexClaim, hash string) error
	// Fail releases the claim and schedules a retry. A request that arrived
	// after the claim keeps its own, earlier due time.
	Fail(ctx context.Context, claim IndexClaim, cause error, retryIn time.Duration) error
	// ClaimPage takes one particular page, due or not, for the lease, so that an
	// explicit synchronisation and the workers never work on a page at once.
	// It reports false when a worker already holds the page, or the page is gone.
	ClaimPage(ctx context.Context, tenantID uint64, pageID string, lease time.Duration) (IndexClaim, bool, error)
	// Requeue finds pages whose knowledge-base entry may be out of date without
	// anyone having said so — a lost event, a knowledge base bound to a space
	// after its pages were written — and marks them due. It examines at most
	// limit pages. reverifyAfter is how long an indexed page may go without being
	// looked at again; the look is cheap when nothing changed.
	Requeue(ctx context.Context, limit int, reverifyAfter time.Duration) (int64, error)
}

type indexStateRepository struct{ db *gorm.DB }

func (r *indexStateRepository) Get(ctx context.Context, tenantID uint64, pageID string) (*model.IndexState, error) {
	var row model.IndexState
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND page_id = ?", tenantID, pageID).Take(&row).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

// markInsertSQL and markUpsertSQL make the state of the pages selected
// between them due. The two cases of due_at: an urgent request pulls the page
// forward and nothing pushes it back; a debounced one pushes the deadline out,
// so a page saved every ten seconds is synchronised once the writing stops,
// unless it is already overdue, when waiting longer would only delay it
// further. Mark and MarkSpace differ only in which pages they select.
const (
	markInsertSQL = `
INSERT INTO docs_index_state (page_id, tenant_id, due_at, seq)
SELECT p.id, p.tenant_id, NOW() + make_interval(secs => @delay), 1
FROM docs_pages p
`
	markUpsertSQL = `
ON CONFLICT (page_id) DO UPDATE SET
    seq = docs_index_state.seq + 1,
    due_at = CASE
        WHEN CAST(@delay AS float8) = 0 THEN LEAST(COALESCE(docs_index_state.due_at, NOW()), NOW())
        WHEN docs_index_state.due_at IS NOT NULL AND docs_index_state.due_at <= NOW() THEN docs_index_state.due_at
        ELSE NOW() + make_interval(secs => @delay)
    END`
	markSQL      = markInsertSQL + `WHERE p.tenant_id = @tenant AND p.id IN @ids` + markUpsertSQL
	markSpaceSQL = markInsertSQL + `WHERE p.tenant_id = @tenant AND p.space_id = @space` + markUpsertSQL
)

func (r *indexStateRepository) Mark(ctx context.Context, tenantID uint64, pageIDs []string,
	delay time.Duration,
) error {
	if len(pageIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Exec(markSQL, map[string]any{
		"tenant": tenantID, "ids": pageIDs, "delay": delay.Seconds(),
	}).Error
}

// MarkSpace is one statement however large the space: a page list passed as
// parameters would hit the protocol's parameter limit on a big space, which is
// exactly the space whose rebinding most needs to reach every page.
func (r *indexStateRepository) MarkSpace(ctx context.Context, tenantID uint64, spaceID string,
	delay time.Duration,
) error {
	return r.db.WithContext(ctx).Exec(markSpaceSQL, map[string]any{
		"tenant": tenantID, "space": spaceID, "delay": delay.Seconds(),
	}).Error
}

func (r *indexStateRepository) MarkSubtree(ctx context.Context, tenantID uint64, rootID string,
	delay time.Duration,
) error {
	var ids []string
	if err := r.db.WithContext(ctx).Raw(subtreeCTE, rootID, tenantID).Scan(&ids).Error; err != nil {
		return err
	}
	return r.Mark(ctx, tenantID, ids, delay)
}

func (r *indexStateRepository) Claim(ctx context.Context, limit int, lease time.Duration) ([]IndexClaim, error) {
	var claims []IndexClaim
	err := r.db.WithContext(ctx).Raw(`
WITH due AS (
    SELECT page_id FROM docs_index_state
    WHERE due_at <= NOW() AND (claimed_until IS NULL OR claimed_until < NOW())
    ORDER BY due_at
    LIMIT @limit
    FOR UPDATE SKIP LOCKED
)
UPDATE docs_index_state s
SET claimed_until = NOW() + make_interval(secs => @lease)
FROM due
WHERE s.page_id = due.page_id
RETURNING s.tenant_id, s.page_id, s.seq, s.attempts`,
		map[string]any{"limit": limit, "lease": lease.Seconds()}).Scan(&claims).Error
	return claims, err
}

func (r *indexStateRepository) Complete(ctx context.Context, claim IndexClaim, hash string) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE docs_index_state
SET claimed_until = NULL, attempts = 0, last_error = '',
    indexed_hash = @hash, indexed_at = NOW(),
    due_at = CASE WHEN seq = @seq THEN NULL ELSE due_at END
WHERE page_id = @page`,
		map[string]any{"hash": hash, "seq": claim.Seq, "page": claim.PageID}).Error
}

func (r *indexStateRepository) Fail(ctx context.Context, claim IndexClaim, cause error,
	retryIn time.Duration,
) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE docs_index_state
SET claimed_until = NULL, attempts = attempts + 1, last_error = @cause,
    due_at = CASE WHEN seq = @seq THEN NOW() + make_interval(secs => @retry) ELSE due_at END
WHERE page_id = @page`,
		map[string]any{
			"cause": truncate(cause.Error(), 2000), "seq": claim.Seq, "page": claim.PageID,
			"retry": retryIn.Seconds(),
		}).Error
}

func (r *indexStateRepository) ClaimPage(ctx context.Context, tenantID uint64, pageID string,
	lease time.Duration,
) (IndexClaim, bool, error) {
	var claims []IndexClaim
	err := r.db.WithContext(ctx).Raw(`
INSERT INTO docs_index_state (page_id, tenant_id, claimed_until)
SELECT p.id, p.tenant_id, NOW() + make_interval(secs => @lease)
FROM docs_pages p WHERE p.tenant_id = @tenant AND p.id = @page
ON CONFLICT (page_id) DO UPDATE SET claimed_until = NOW() + make_interval(secs => @lease)
WHERE docs_index_state.claimed_until IS NULL OR docs_index_state.claimed_until < NOW()
RETURNING tenant_id, page_id, seq, attempts`,
		map[string]any{"tenant": tenantID, "page": pageID, "lease": lease.Seconds()}).Scan(&claims).Error
	if err != nil || len(claims) == 0 {
		return IndexClaim{}, false, err
	}
	return claims[0], true, nil
}

// requeueMissingSQL gives a state row to pages that matter to the knowledge
// base and have none: live pages of a space bound to one, and pages that
// already own an entry (a trashed page whose entry must go).
const requeueMissingSQL = `
INSERT INTO docs_index_state (page_id, tenant_id, due_at, seq)
SELECT p.id, p.tenant_id, NOW(), 1
FROM docs_pages p
JOIN docs_spaces s ON s.id = p.space_id
LEFT JOIN docs_index_state st ON st.page_id = p.id
WHERE st.page_id IS NULL
  AND ((p.deleted_at IS NULL AND s.deleted_at IS NULL AND s.knowledge_base_id IS NOT NULL)
       OR p.knowledge_id IS NOT NULL)
LIMIT @limit
ON CONFLICT (page_id) DO NOTHING`

// requeueStaleSQL marks pages that nothing is waiting on but that changed
// since they were last looked at: the page was edited, the space's settings
// (its knowledge-base binding among them) changed, or an entry has gone a long
// while unverified, which is the net under a permission change whose event was
// lost.
const requeueStaleSQL = `
UPDATE docs_index_state st
SET due_at = NOW(), seq = st.seq + 1
WHERE st.page_id IN (
    SELECT st2.page_id
    FROM docs_index_state st2
    JOIN docs_pages p ON p.id = st2.page_id
    JOIN docs_spaces s ON s.id = p.space_id
    WHERE st2.due_at IS NULL
      AND (st2.claimed_until IS NULL OR st2.claimed_until < NOW())
      AND (st2.indexed_at IS NULL
           OR p.updated_at > st2.indexed_at
           OR s.updated_at > st2.indexed_at
           OR (p.knowledge_id IS NOT NULL
               AND st2.indexed_at < NOW() - make_interval(secs => @reverify)))
    ORDER BY st2.indexed_at NULLS FIRST
    LIMIT @limit
    FOR UPDATE OF st2 SKIP LOCKED
)`

func (r *indexStateRepository) Requeue(ctx context.Context, limit int, reverifyAfter time.Duration) (int64, error) {
	args := map[string]any{"limit": limit, "reverify": reverifyAfter.Seconds()}
	missing := r.db.WithContext(ctx).Exec(requeueMissingSQL, args)
	if missing.Error != nil {
		return 0, missing.Error
	}
	stale := r.db.WithContext(ctx).Exec(requeueStaleSQL, args)
	if stale.Error != nil {
		return 0, stale.Error
	}
	return missing.RowsAffected + stale.RowsAffected, nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
