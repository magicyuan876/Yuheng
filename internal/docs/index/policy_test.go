package index

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexable() Candidate {
	return Candidate{BoundKB: "kb-1", Text: "这是一段足够长的正文内容。"}
}

func TestAnOrdinaryPageIsIndexed(t *testing.T) {
	d := Decide(indexable())
	assert.True(t, d.Index)
	assert.Empty(t, d.Reason)
}

// A space nobody bound to a knowledge base sends nothing anywhere.
func TestAPageInAnUnboundSpaceIsNotIndexed(t *testing.T) {
	c := indexable()
	c.BoundKB = ""
	d := Decide(c)
	assert.False(t, d.Index)
	assert.Equal(t, ReasonNoBinding, d.Reason)

	c.BoundKB = "   "
	assert.Equal(t, ReasonNoBinding, Decide(c).Reason, "whitespace is not a binding")
}

// The rule this package exists for: retrieval has no per-entry permission
// filter, so an indexed restricted page is readable by everybody who may
// query the knowledge base.
func TestARestrictedPageIsNeverIndexed(t *testing.T) {
	c := indexable()
	c.Restricted = true
	d := Decide(c)
	assert.False(t, d.Index)
	assert.Equal(t, ReasonRestricted, d.Reason)
}

func TestATrashedPageIsNotIndexed(t *testing.T) {
	c := indexable()
	c.Trashed = true
	assert.Equal(t, ReasonTrashed, Decide(c).Reason)
}

// Being in the bin outranks everything: a trashed page is gone whatever else
// is true of it.
func TestTrashOutranksTheOtherRules(t *testing.T) {
	c := indexable()
	c.Trashed = true
	c.Restricted = true
	c.Draft = true
	assert.Equal(t, ReasonTrashed, Decide(c).Reason)
}

func TestADraftIsNotIndexed(t *testing.T) {
	c := indexable()
	c.Draft = true
	assert.Equal(t, ReasonDraft, Decide(c).Reason)
}

func TestAnEmptyPageIsNotIndexed(t *testing.T) {
	c := indexable()
	for _, text := range []string{"", "   ", "\n\t", "四个字"} {
		c.Text = text
		d := Decide(c)
		assert.False(t, d.Index, "text %q", text)
		assert.Equal(t, ReasonEmpty, d.Reason)
	}
}

func TestAPageJustOverTheThresholdIsIndexed(t *testing.T) {
	c := indexable()
	c.Text = "一二三四五"
	assert.True(t, Decide(c).Index, "five runes is enough")
}

// ---- the debounce queue ---------------------------------------------------------

func TestAQueueHoldsAPageUntilItsDebounceExpires(t *testing.T) {
	now := time.Now()
	q := NewQueue(time.Minute)
	q.Touch("page-1", now)

	assert.Empty(t, q.Due(now), "not yet")
	assert.Empty(t, q.Due(now.Add(59*time.Second)))
	assert.Equal(t, []string{"page-1"}, q.Due(now.Add(time.Minute)))
}

// Somebody saving every ten seconds for half an hour should produce one
// rebuild, not a hundred and eighty.
func TestRepeatEditsPushTheDeadlineOut(t *testing.T) {
	now := time.Now()
	q := NewQueue(time.Minute)

	for i := 0; i < 5; i++ {
		q.Touch("page-1", now.Add(time.Duration(i*10)*time.Second))
	}
	assert.Equal(t, 1, q.Len(), "one entry, however many edits")
	assert.Empty(t, q.Due(now.Add(time.Minute)), "the last edit moved it")
	assert.Len(t, q.Due(now.Add(100*time.Second)), 1)
}

func TestATakenPageIsRemoved(t *testing.T) {
	now := time.Now()
	q := NewQueue(time.Minute)
	q.Touch("page-1", now)

	assert.Len(t, q.Due(now.Add(time.Minute)), 1)
	assert.Empty(t, q.Due(now.Add(time.Hour)), "taken once, not twice")
	assert.Equal(t, 0, q.Len())
}

func TestADeletedPageIsForgotten(t *testing.T) {
	now := time.Now()
	q := NewQueue(time.Minute)
	q.Touch("page-1", now)
	q.Forget("page-1")

	assert.Empty(t, q.Due(now.Add(time.Hour)))
	assert.Equal(t, 0, q.Len())
}

func TestSeveralPagesAreTrackedSeparately(t *testing.T) {
	now := time.Now()
	q := NewQueue(time.Minute)
	q.Touch("early", now)
	q.Touch("late", now.Add(30*time.Second))

	assert.Equal(t, []string{"early"}, q.Due(now.Add(time.Minute)))
	assert.Equal(t, []string{"late"}, q.Due(now.Add(91*time.Second)))
}

// A negative debounce means "now", which is what a manual rebuild wants.
func TestANegativeDebounceIsImmediate(t *testing.T) {
	now := time.Now()
	q := NewQueue(-1)
	q.Touch("page-1", now)
	assert.Len(t, q.Due(now), 1)
}

func TestAZeroDebounceUsesTheDefault(t *testing.T) {
	assert.Equal(t, DefaultDebounce, NewQueue(0).debounce)
}

func TestANilQueueIsInert(t *testing.T) {
	var q *Queue
	q.Touch("page-1", time.Now())
	q.Forget("page-1")
	assert.Empty(t, q.Due(time.Now()))
	assert.Equal(t, 0, q.Len())
}

func TestAnEmptyPageIDIsIgnored(t *testing.T) {
	q := NewQueue(time.Minute)
	q.Touch("", time.Now())
	require.Equal(t, 0, q.Len())
}

// A page taken away from readers is not held back by the debounce.
func TestAnUrgentPageIsDueAtOnce(t *testing.T) {
	q := NewQueue(time.Minute)
	now := time.Now()
	q.Urgent("page-1", now)
	assert.Equal(t, []string{"page-1"}, q.Due(now))
}

// An edit arriving after a restriction must not postpone the restriction.
func TestALaterEditDoesNotPostponeAnUrgentPage(t *testing.T) {
	q := NewQueue(time.Minute)
	now := time.Now()
	q.Urgent("page-1", now)
	q.Touch("page-1", now.Add(time.Second))
	assert.Equal(t, []string{"page-1"}, q.Due(now.Add(time.Second)))
}

// Once drained, a page is ordinary again.
func TestAnUrgentPageIsOrdinaryOnceDrained(t *testing.T) {
	q := NewQueue(time.Minute)
	now := time.Now()
	q.Urgent("page-1", now)
	q.Due(now)
	q.Touch("page-1", now)
	assert.Empty(t, q.Due(now))
	assert.Equal(t, 1, q.Len())
}

func TestForgettingAPageForgetsItsUrgency(t *testing.T) {
	q := NewQueue(time.Minute)
	now := time.Now()
	q.Urgent("page-1", now)
	q.Forget("page-1")
	q.Touch("page-1", now)
	assert.Empty(t, q.Due(now))
}
