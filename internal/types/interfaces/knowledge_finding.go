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
	// records who and when, reopening clears both.
	SetStatus(ctx context.Context, tenantID uint64, kbID, id, status, actor string) (*types.KnowledgeFindingRow, error)
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
	// UpdateStatus dismisses or reopens a finding and records it in the
	// knowledge base's activity.
	UpdateStatus(ctx context.Context, tenantID uint64, kbID, findingID,
		status string) (*types.KnowledgeFindingView, error)
	// Scan schedules a check of every indexed entry of the knowledge base and
	// reports how many were scheduled.
	Scan(ctx context.Context, tenantID uint64, kbID string) (int, error)
	// OpenForKnowledge lists the open findings naming a knowledge entry, each
	// oriented so that the entry asked about is the subject.
	OpenForKnowledge(ctx context.Context, tenantID uint64, knowledgeID string) ([]*types.KnowledgeFindingView, error)
}
