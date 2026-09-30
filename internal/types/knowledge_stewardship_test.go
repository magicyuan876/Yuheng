package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStewardChains(t *testing.T) {
	all := &KnowledgeSteward{
		OwnerID: "owner", OwnerActive: true, ReviewedBy: "editor", ReviewerActive: true,
		KBCreatorID: "creator", KBCreatorActive: true,
	}
	assert.Equal(t, "owner", all.Responsible(), "the owner answers for the entry")
	assert.Equal(t, "editor", all.LatestHand(), "the last reviewer is the latest hand")

	ownerLeft := *all
	ownerLeft.OwnerActive = false
	assert.Equal(t, "editor", ownerLeft.Responsible())

	nobody := &KnowledgeSteward{OwnerID: "gone", KBCreatorID: "also-gone"}
	assert.Empty(t, nobody.Responsible())
	assert.Empty(t, nobody.LatestHand())

	neverReviewed := &KnowledgeSteward{OwnerID: "owner", OwnerActive: true}
	assert.Equal(t, "owner", neverReviewed.LatestHand(), "unreviewed, the owner is who added it")
}

func TestStewardReviewClock(t *testing.T) {
	created := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	st := &KnowledgeSteward{CreatedAt: created}
	assert.Nil(t, st.ReviewDueAt(), "no review period, no due date")
	assert.False(t, st.Overdue(created.AddDate(10, 0, 0)))

	st.ReviewIntervalDays = 30
	assert.Equal(t, created.AddDate(0, 0, 30), *st.ReviewDueAt(), "never reviewed: the clock runs from creation")
	assert.False(t, st.Overdue(created.AddDate(0, 0, 29)))
	assert.True(t, st.Overdue(created.AddDate(0, 0, 30)))

	reviewed := created.AddDate(0, 0, 20)
	st.ReviewedAt = &reviewed
	assert.Equal(t, reviewed.AddDate(0, 0, 30), *st.ReviewDueAt())

	// A review stamped before the entry existed (a page's content older than
	// its re-created mirror) does not move the clock before creation.
	early := created.AddDate(0, 0, -5)
	st.ReviewedAt = &early
	assert.Equal(t, created, st.LastVouchedAt())
}
