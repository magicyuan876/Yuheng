package index

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	c.Excluded = true
	assert.Equal(t, ReasonTrashed, Decide(c).Reason)
}

func TestAnExcludedPageIsNotIndexed(t *testing.T) {
	c := indexable()
	c.Excluded = true
	assert.Equal(t, ReasonExcluded, Decide(c).Reason)
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
