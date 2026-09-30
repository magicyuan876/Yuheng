package types

import "time"

// Stewardship: who looks after a knowledge entry, and when a person last
// vouched for it. Knowledge health takes its problems to these people; a
// review period turns "nobody has looked at this in a year" into a problem of
// its own.
//
// None of it is a permission. Who may read or change an entry is decided where
// it always was; the owner is the person expected to act, and anyone allowed to
// edit the knowledge base may act in their place.

// KnowledgeOrigin says where the content of an entry is maintained. It decides
// how the entry can be retired from its knowledge base and who controls its
// stewardship.
type KnowledgeOrigin string

const (
	// KnowledgeOriginLocal is an entry maintained here: uploaded, fetched
	// from a URL once, or written in the knowledge base.
	KnowledgeOriginLocal KnowledgeOrigin = "local"
	// KnowledgeOriginDocs is the mirror of a docs page. The page is the
	// source: its owner is the entry's owner, and retiring the entry means
	// excluding the page from the knowledge base.
	KnowledgeOriginDocs KnowledgeOrigin = "docs"
	// KnowledgeOriginSynced is kept in step with an external source by a
	// data-source sync. Deleting it here does not last — the next sync
	// brings it back — so it is retired at the source.
	KnowledgeOriginSynced KnowledgeOrigin = "synced"
)

// KnowledgeOriginSQL computes KnowledgeOrigin in SQL over a knowledges row
// aliased k. It must agree with Knowledge.Origin; a repository test checks
// the two on the same rows. It has no "?" in it, which GORM would read as a
// placeholder.
const KnowledgeOriginSQL = `CASE WHEN k.channel = '` + ChannelDocs + `' THEN '` + string(KnowledgeOriginDocs) + `'
	WHEN k.metadata->>'datasource_id' IS NOT NULL THEN '` + string(KnowledgeOriginSynced) + `'
	ELSE '` + string(KnowledgeOriginLocal) + `' END`

// Origin reports where the entry's content is maintained.
func (k *Knowledge) Origin() KnowledgeOrigin {
	if k == nil {
		return KnowledgeOriginLocal
	}
	if k.Channel == ChannelDocs {
		return KnowledgeOriginDocs
	}
	if len(k.Metadata) > 0 {
		if m, err := k.Metadata.Map(); err == nil {
			if v, ok := m["datasource_id"]; ok && v != nil {
				return KnowledgeOriginSynced
			}
		}
	}
	return KnowledgeOriginLocal
}

// KnowledgeSteward is the stewardship of one entry as the routing of problems
// needs it: the people involved, whether each is still an active member of the
// entry's workspace, and the review clock.
type KnowledgeSteward struct {
	KnowledgeID     string          `gorm:"column:knowledge_id"`
	KnowledgeBaseID string          `gorm:"column:knowledge_base_id"`
	Title           string          `gorm:"column:title"`
	Origin          KnowledgeOrigin `gorm:"column:origin"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
	// OwnerID, ReviewedBy and KBCreatorID are empty when unset. The Active
	// flags say whether the person can still be asked: an active user who
	// is an active member of the workspace.
	OwnerID         string     `gorm:"column:owner_id"`
	OwnerActive     bool       `gorm:"column:owner_active"`
	ReviewedAt      *time.Time `gorm:"column:reviewed_at"`
	ReviewedBy      string     `gorm:"column:reviewed_by"`
	ReviewerActive  bool       `gorm:"column:reviewer_active"`
	KBCreatorID     string     `gorm:"column:kb_creator_id"`
	KBCreatorActive bool       `gorm:"column:kb_creator_active"`
	// ReviewIntervalDays is the knowledge base's review period; 0 is none.
	ReviewIntervalDays int `gorm:"column:review_interval_days"`
}

// LastVouchedAt is when a person last vouched for the entry: its last review,
// or, never reviewed, its creation.
func (s *KnowledgeSteward) LastVouchedAt() time.Time {
	if s.ReviewedAt != nil && s.ReviewedAt.After(s.CreatedAt) {
		return *s.ReviewedAt
	}
	return s.CreatedAt
}

// ReviewDueAt is when the entry is due for review, nil when its knowledge base
// has no review period.
func (s *KnowledgeSteward) ReviewDueAt() *time.Time {
	if s.ReviewIntervalDays <= 0 {
		return nil
	}
	due := s.LastVouchedAt().AddDate(0, 0, s.ReviewIntervalDays)
	return &due
}

// Overdue reports whether the entry is past its review date at now.
func (s *KnowledgeSteward) Overdue(now time.Time) bool {
	due := s.ReviewDueAt()
	return due != nil && !now.Before(*due)
}

// Responsible is the person answerable for the entry: its owner, else the
// last person who vouched for it, else the creator of its knowledge base —
// the first of them who can still be asked. Empty when none can.
func (s *KnowledgeSteward) Responsible() string {
	return firstActive(
		stewardCandidate{s.OwnerID, s.OwnerActive},
		stewardCandidate{s.ReviewedBy, s.ReviewerActive},
		stewardCandidate{s.KBCreatorID, s.KBCreatorActive},
	)
}

// LatestHand is the person who last worked on the entry: the last person who
// vouched for it, else its owner (who, for an entry nobody has reviewed, is
// the person who added it), else the creator of its knowledge base.
func (s *KnowledgeSteward) LatestHand() string {
	return firstActive(
		stewardCandidate{s.ReviewedBy, s.ReviewerActive},
		stewardCandidate{s.OwnerID, s.OwnerActive},
		stewardCandidate{s.KBCreatorID, s.KBCreatorActive},
	)
}

type stewardCandidate struct {
	id     string
	active bool
}

func firstActive(candidates ...stewardCandidate) string {
	for _, c := range candidates {
		if c.id != "" && c.active {
			return c.id
		}
	}
	return ""
}

// PersonRef names a person in an API response.
type PersonRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar,omitempty"`
	// Active is false for somebody who has left the workspace or whose
	// account is disabled: still shown, since it explains why nobody acts.
	Active bool `json:"active"`
}

// KnowledgeStewardshipView is an entry's stewardship as the API returns it.
type KnowledgeStewardshipView struct {
	KnowledgeID string          `json:"knowledge_id"`
	Origin      KnowledgeOrigin `json:"origin"`
	Owner       *PersonRef      `json:"owner"`
	// OwnerEditable is false for a docs mirror, whose owner is the page's
	// and is changed on the page.
	OwnerEditable      bool       `json:"owner_editable"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	ReviewedBy         *PersonRef `json:"reviewed_by"`
	ReviewIntervalDays int        `json:"review_interval_days"`
	ReviewDueAt        *time.Time `json:"review_due_at"`
	Overdue            bool       `json:"overdue"`
}

// SetKnowledgeOwnerRequest is the body of PUT /knowledge/{id}/owner.
type SetKnowledgeOwnerRequest struct {
	// OwnerID is the new owner, who must be an active member of the entry's
	// workspace. Empty removes the owner.
	OwnerID string `json:"owner_id"`
}
