package interfaces

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// KnowledgeStewardshipRepository reads and writes who looks after knowledge
// entries. It is the only writer of the stewardship columns, which a full-row
// knowledge update leaves alone.
type KnowledgeStewardshipRepository interface {
	// Stewards loads the stewardship of the live entries among knowledgeIDs
	// in the tenant, keyed by entry ID. Entries that do not exist are left
	// out.
	Stewards(ctx context.Context, tenantID uint64, knowledgeIDs []string) (map[string]*types.KnowledgeSteward, error)
	// SetOwner sets or, with an empty ownerID, clears the owner of an entry.
	// It returns ErrKnowledgeNotFound for an entry that does not exist.
	SetOwner(ctx context.Context, tenantID uint64, knowledgeID, ownerID string) error
	// MarkReviewed records that a person vouched for an entry at the given
	// time. A review older than the one recorded is ignored, so a late write
	// cannot turn the clock back.
	MarkReviewed(ctx context.Context, tenantID uint64, knowledgeID, userID string, at time.Time) error
	// CanMaintain reports whether a user can be made the owner of entries of
	// a knowledge base: an active account, an active member of the tenant,
	// and able to edit the base's content — its creator or an Admin+. An
	// owner who could not change the entry would be sent problems they
	// cannot fix.
	CanMaintain(ctx context.Context, tenantID uint64, kbID, userID string) (bool, error)
	// IsActiveMember reports whether a user is an active account with an
	// active membership of the tenant.
	IsActiveMember(ctx context.Context, tenantID uint64, userID string) (bool, error)
}

// KnowledgeStewardshipService is the API over stewardship. tenantID is the
// tenant that owns the entry.
type KnowledgeStewardshipService interface {
	// Get returns the stewardship of an entry.
	Get(ctx context.Context, tenantID uint64, knowledgeID string) (*types.KnowledgeStewardshipView, error)
	// SetOwner transfers an entry to another member, or leaves it without an
	// owner. A docs mirror is refused: its owner is its page's.
	SetOwner(ctx context.Context, tenantID uint64, knowledgeID, ownerID string) (*types.KnowledgeStewardshipView, error)
	// ConfirmReviewed records that the caller vouches for the entry as it
	// stands, which restarts its review clock and settles the problems that
	// asked for a review.
	ConfirmReviewed(ctx context.Context, tenantID uint64, knowledgeID string) (*types.KnowledgeStewardshipView, error)
	// SyncFromSource copies the stewardship of an entry's source onto it:
	// the docs module calls it for a page's mirror. ownerID may be empty;
	// reviewedBy empty or a zero reviewedAt records no review.
	SyncFromSource(ctx context.Context, tenantID uint64, knowledgeID, ownerID, reviewedBy string,
		reviewedAt time.Time) error
}
