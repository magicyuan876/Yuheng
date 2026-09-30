package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// Knowledge health: the interfaces between the findings framework, the
// retrieval engines it reads vectors from, its storage and its API.

// SimilarChunkFinder is an optional capability of a retrieval engine: comparing
// the vectors it already stores, without embedding anything.
//
// An engine that implements it lets the duplicate detector run over a
// knowledge base; one that does not is reported as unsupported and nothing is
// detected there. It is deliberately a separate interface rather than a
// method of RetrieveEngineService, so an engine added by an extension does not
// have to implement it to be a retrieval engine.
type SimilarChunkFinder interface {
	// SimilarChunks returns, for every enabled chunk of the knowledge entry
	// that has a vector, up to perChunk of the nearest enabled chunks of
	// OTHER entries in the same knowledge base, compared only with vectors of
	// the same dimension, and keeps the pairs whose cosine similarity is at
	// least minScore. Only chunk vectors count: a generated question or any
	// other row that indexes a chunk under a different source ID is not a
	// passage of the document and is left out.
	SimilarChunks(ctx context.Context, kbID, knowledgeID string, minScore float64,
		perChunk int) ([]types.ChunkSimilarity, error)
}

// SimilarChunkFinderProvider is implemented by an engine that wraps a store:
// whether it can find similar chunks depends on the store beneath it, which a
// type assertion on the wrapper cannot tell.
type SimilarChunkFinderProvider interface {
	SimilarChunkFinder() (SimilarChunkFinder, bool)
}

// ErrFindingNotFound is returned for a finding that does not exist in the
// knowledge base named, which includes one whose documents were deleted.
var ErrFindingNotFound = errors.New("finding not found")

// KnowledgeFindingRepository stores findings.
//
// Every read joins the documents a finding names and returns nothing for a
// finding one of whose documents has been deleted or moved to another
// knowledge base. The database also deletes such findings (a trigger on
// knowledges), so the join is the guarantee for the moment between the two.
type KnowledgeFindingRepository interface {
	// Reconcile records one run of the detectors over a knowledge entry:
	// reported findings are inserted or updated by fingerprint, and open
	// findings of the detectors that ran which involve the entry and were not
	// reported again are resolved by the system. It also records when the
	// entry was checked. A finding naming a document that no longer exists is
	// not written.
	Reconcile(ctx context.Context, run types.FindingRun) error
	// List pages through the findings of a knowledge base.
	List(ctx context.Context, tenantID uint64, kbID string,
		filter types.KnowledgeFindingFilter) ([]*types.KnowledgeFindingRow, int64, error)
	// Get loads one finding of a knowledge base.
	Get(ctx context.Context, tenantID uint64, kbID, id string) (*types.KnowledgeFindingRow, error)
	// SetStatus changes a finding's status by a person's hand: dismissing
	// records who, when and why (resolution), reopening clears all three.
	SetStatus(ctx context.Context, tenantID uint64, kbID, id, status, actor,
		resolution string) (*types.KnowledgeFindingRow, error)
	// SetAssignee assigns a finding by hand to assigneeID on behalf of
	// assignedBy, which later checks then leave alone; an empty assignedBy
	// hands the finding back to automatic routing, which the next check
	// applies.
	SetAssignee(ctx context.Context, tenantID uint64, kbID, id, assigneeID,
		assignedBy string) (*types.KnowledgeFindingRow, error)
	// ListAssigned pages through the findings of a tenant's knowledge bases
	// assigned to a person, with the knowledge bases' names. status "all"
	// or empty lists every status.
	ListAssigned(ctx context.Context, tenantID uint64, assigneeID, status string,
		page, pageSize int) ([]*types.KnowledgeFindingRow, int64, error)
	// CountOpenAssigned counts the open findings assigned to a person in a
	// tenant.
	CountOpenAssigned(ctx context.Context, tenantID uint64, assigneeID string) (int64, error)
	// ListReviewChecksDue lists up to limit entries, across tenants, whose
	// periodic-review state changed with time or with their knowledge
	// base's review period, and so need a check (see the findings review
	// sweep).
	ListReviewChecksDue(ctx context.Context, now time.Time, limit int) ([]types.KnowledgeFindingsPayload, error)
	// CountOpenByType counts the open findings of a knowledge base by type.
	CountOpenByType(ctx context.Context, tenantID uint64, kbID string) (map[string]int64, error)
	// LastScanAt is when an entry of the knowledge base was last checked.
	LastScanAt(ctx context.Context, tenantID uint64, kbID string) (*time.Time, error)
	// ListOpenForKnowledge lists the open findings that name a knowledge
	// entry on either side.
	ListOpenForKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.KnowledgeFindingRow, error)
	// ListCheckableKnowledgeIDs lists up to limit entries of a knowledge base
	// whose indexing has finished, for a full re-check.
	ListCheckableKnowledgeIDs(ctx context.Context, tenantID uint64, kbID string, limit int) ([]string, error)
	// DeleteForKnowledge removes every finding naming the entry and its scan
	// record.
	DeleteForKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) error
}

// KnowledgeFindingsTrigger schedules a check of one knowledge entry. Calls
// for the same entry within a short window collapse into one check, so a
// caller may trigger on every change without worrying about bursts.
type KnowledgeFindingsTrigger interface {
	TriggerKnowledgeFindings(ctx context.Context, tenantID uint64, kbID, knowledgeID string) error
	// ScheduleKnowledgeFindings is TriggerKnowledgeFindings with extraDelay
	// added to the debounce window, for spreading a full re-check out. It
	// reports whether a check was newly scheduled: false when one for the
	// same entry was already pending, or when checks are switched off.
	ScheduleKnowledgeFindings(ctx context.Context, tenantID uint64, kbID, knowledgeID string,
		extraDelay time.Duration) (bool, error)
}

// KnowledgeFindingService is the API over findings. tenantID is the tenant
// that owns the knowledge base (for a shared base, the owner, not the caller).
type KnowledgeFindingService interface {
	List(ctx context.Context, tenantID uint64, kbID string,
		filter types.KnowledgeFindingFilter) (*types.KnowledgeFindingPage, error)
	Summary(ctx context.Context, tenantID uint64, kbID string) (*types.KnowledgeFindingSummary, error)
	// UpdateStatus dismisses (with a reason, types.ValidDismissReason) or
	// reopens a finding and records it in the knowledge base's activity.
	UpdateStatus(ctx context.Context, tenantID uint64, kbID, findingID,
		status, reason string) (*types.KnowledgeFindingView, error)
	// Assign takes a finding to a person of the caller's choosing, or, with
	// an empty assigneeID, back to the automatic routing.
	Assign(ctx context.Context, tenantID uint64, kbID, findingID,
		assigneeID string) (*types.KnowledgeFindingView, error)
	// ListAssigned pages through the findings of the tenant assigned to the
	// caller, across its knowledge bases.
	ListAssigned(ctx context.Context, tenantID uint64, status string, page,
		pageSize int) (*types.KnowledgeFindingPage, error)
	// CountAssigned counts the caller's open findings in the tenant.
	CountAssigned(ctx context.Context, tenantID uint64) (int64, error)
	// Supersede settles a finding about two documents by keeping one and
	// taking the other out of the knowledge base, the way its source
	// requires (see KnowledgeRetirer).
	Supersede(ctx context.Context, tenantID uint64, kbID, findingID,
		keepKnowledgeID string) (*types.SupersedeResult, error)
	// Scan schedules a check of every indexed entry of the knowledge base and
	// reports how many were scheduled.
	Scan(ctx context.Context, tenantID uint64, kbID string) (int, error)
	// OpenForKnowledge lists the open findings naming a knowledge entry, each
	// oriented so that the entry asked about is the subject.
	OpenForKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.KnowledgeFindingView, error)
}

// KnowledgeRetirer takes an entry out of its knowledge base in the way its
// source requires, because it has been superseded by another entry.
//
// An entry maintained in the knowledge base is simply deleted, which the
// findings service does itself. An entry that mirrors something else cannot
// be: deleting a docs page's mirror would bring it back at the page's next
// synchronisation. Such a source registers a retirer for its origin; the docs
// module does for pages, excluding the page from the knowledge base and
// marking it superseded.
type KnowledgeRetirer interface {
	// Retire takes the entry out on behalf of the person in ctx, who must be
	// allowed to change its source (an AppError says why not), and reports
	// how (types.Retired*). replacement is the entry that supersedes it.
	Retire(ctx context.Context, tenantID uint64, knowledgeID string, replacement types.KnowledgeRef) (string, error)
}

// KnowledgeRetirers is where sources register their retirers. Registration
// happens while the container is built; lookups after.
type KnowledgeRetirers interface {
	Register(origin types.KnowledgeOrigin, retirer KnowledgeRetirer)
	For(origin types.KnowledgeOrigin) (KnowledgeRetirer, bool)
}
