package history

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// edited builds the candidate an ordinary save produces.
func edited(editors ...string) Candidate {
	return Candidate{
		ContentChanged: true,
		Reason:         model.RevisionInterval,
		PageCreatedAt:  base.Add(-time.Hour),
		EditorIDs:      editors,
	}
}

func snapshotAt(at time.Time, editors ...string) *Snapshot {
	return &Snapshot{ID: "r1", CreatedAt: at, Reason: model.RevisionInterval, EditorIDs: editors}
}

// Re-encoding is not an edit, and a history of identical entries is worse
// than no history at all.
func TestASaveThatChangedNothingLeavesNoSnapshot(t *testing.T) {
	c := edited("alice")
	c.ContentChanged = false

	assert.False(t, ShouldSnapshot(c, nil, base))
	assert.False(t, ShouldSnapshot(c, snapshotAt(base.Add(-time.Hour), "alice"), base))
}

func TestTheFirstEditOfAPageIsAlwaysKept(t *testing.T) {
	assert.True(t, ShouldSnapshot(edited("alice"), nil, base),
		"there must be something to go back to from the very first edit")
}

// A page is created before it is written; the blank it starts as is not a
// version of anything.
func TestAnEmptyPageDoesNotGetAFirstSnapshot(t *testing.T) {
	c := edited("alice")
	c.Empty = true
	assert.False(t, ShouldSnapshot(c, nil, base))

	// Once there is a snapshot, emptiness is somebody deleting everything,
	// which is very much a version worth keeping.
	c.PageCreatedAt = base.Add(-time.Hour)
	assert.True(t, ShouldSnapshot(c, snapshotAt(base.Add(-time.Hour), "alice"), base))
}

// These are the entries somebody will actually look for by name.
func TestADeliberateSaveAlwaysSnapshots(t *testing.T) {
	for _, reason := range []model.RevisionReason{
		model.RevisionPublish, model.RevisionRestore, model.RevisionImport, model.RevisionManual,
	} {
		c := edited("alice")
		c.Reason = reason
		// Even one second after the last snapshot, and by the same person.
		assert.True(t, ShouldSnapshot(c, snapshotAt(base.Add(-time.Second), "alice"), base), string(reason))
	}
}

func TestAnOrdinarySaveWaitsForTheInterval(t *testing.T) {
	last := snapshotAt(base.Add(-Interval+time.Second), "alice")
	assert.False(t, ShouldSnapshot(edited("alice"), last, base), "too soon")

	last = snapshotAt(base.Add(-Interval), "alice")
	assert.True(t, ShouldSnapshot(edited("alice"), last, base), "exactly the interval counts")
}

// A new page is where edits are largest and a mistake is most likely to be
// worth undoing.
func TestANewPageSnapshotsMoreOften(t *testing.T) {
	c := edited("alice")
	c.PageCreatedAt = base.Add(-time.Minute) // created a minute ago

	assert.False(t, ShouldSnapshot(c, snapshotAt(base.Add(-30*time.Second), "alice"), base))
	assert.True(t, ShouldSnapshot(c, snapshotAt(base.Add(-FastInterval), "alice"), base))

	// And once it is no longer new, the ordinary interval applies again.
	c.PageCreatedAt = base.Add(-time.Hour)
	assert.False(t, ShouldSnapshot(c, snapshotAt(base.Add(-FastInterval), "alice"), base))
}

// Whose work is in a version is most of what makes history readable; letting
// one person's changes be absorbed into another's entry loses exactly that.
func TestANewEditorForcesASnapshot(t *testing.T) {
	last := snapshotAt(base.Add(-time.Second), "alice")

	assert.True(t, ShouldSnapshot(edited("bob"), last, base), "somebody else has started editing")
	assert.False(t, ShouldSnapshot(edited("alice"), last, base), "the same person has not")
	assert.False(t, ShouldSnapshot(edited(), last, base),
		"a save with nobody attached is not a new editor")
}

func TestEditorComparisonIgnoresOrderAndRepeats(t *testing.T) {
	last := snapshotAt(base.Add(-time.Second), "alice", "bob")

	assert.False(t, ShouldSnapshot(edited("bob", "alice"), last, base))
	assert.False(t, ShouldSnapshot(edited("alice", "alice"), last, base))
	assert.True(t, ShouldSnapshot(edited("alice", "carol"), last, base))
}

func TestAPageWithNoCreationTimeUsesTheOrdinaryInterval(t *testing.T) {
	c := edited("alice")
	c.PageCreatedAt = time.Time{}
	assert.False(t, ShouldSnapshot(c, snapshotAt(base.Add(-FastInterval), "alice"), base))
	assert.True(t, ShouldSnapshot(c, snapshotAt(base.Add(-Interval), "alice"), base))
}

// ---- compaction -------------------------------------------------------------

// history builds n interval snapshots, one per interval, newest last.
func history(n int, from time.Time, step time.Duration) []Snapshot {
	out := make([]Snapshot, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Snapshot{
			ID:        fmt.Sprintf("r%04d", i),
			CreatedAt: from.Add(time.Duration(i) * step),
			Reason:    model.RevisionInterval,
		})
	}
	return out
}

func contains(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

// The acceptance criterion for this work package.
func TestCompactionKeepsTheMostRecentTwoHundredWhole(t *testing.T) {
	// 400 snapshots, one an hour, ending now.
	snaps := history(400, base.Add(-400*time.Hour), time.Hour)
	drop := Compact(snaps, base)

	require.NotEmpty(t, drop, "there is something to thin")
	for _, snap := range snaps[len(snaps)-KeepRecent:] {
		assert.False(t, contains(drop, snap.ID), "%s is within the recent window", snap.ID)
	}
}

func TestNothingIsDroppedWhileThereIsLittleHistory(t *testing.T) {
	assert.Nil(t, Compact(history(KeepRecent, base.Add(-time.Hour), time.Minute), base))
	assert.Nil(t, Compact(nil, base))
}

// A publish, an import or a restore is a landmark somebody chose to create.
func TestADeliberateSnapshotIsNeverThinnedAway(t *testing.T) {
	snaps := history(400, base.Add(-400*time.Hour), time.Hour)
	snaps[10].Reason = model.RevisionPublish
	snaps[11].Reason = model.RevisionRestore
	snaps[12].Reason = model.RevisionImport

	drop := Compact(snaps, base)
	for _, id := range []string{snaps[10].ID, snaps[11].ID, snaps[12].ID} {
		assert.False(t, contains(drop, id), "%s was deliberate", id)
	}
	assert.True(t, contains(drop, snaps[13].ID), "while the ordinary ones around it are thinned")
}

func TestOlderHistoryIsThinnedToOnePerDayThenOnePerMonth(t *testing.T) {
	// Four a day for a year, which is far more than any window keeps.
	snaps := history(4*365, base.Add(-365*24*time.Hour), 6*time.Hour)
	drop := Compact(snaps, base)
	dropped := map[string]bool{}
	for _, id := range drop {
		dropped[id] = true
	}

	// Counted only outside the recent window, which is exempt by design and
	// would otherwise put many same-day entries in one bucket.
	cutoff := snaps[len(snaps)-KeepRecent].CreatedAt
	kept := map[string]int{}
	for _, snap := range snaps {
		if dropped[snap.ID] || !snap.CreatedAt.Before(cutoff) {
			continue
		}
		kept[bucketOf(snap.CreatedAt, base)]++
	}

	require.NotEmpty(t, kept, "there is thinned history to look at")
	for bucket, n := range kept {
		assert.Equal(t, 1, n, "bucket %s keeps exactly one", bucket)
	}
	// And the whole thing is a great deal smaller than it was.
	assert.Less(t, len(snaps)-len(drop), len(snaps)/2)
}

func TestCompactionDoesNotDependOnTheOrderItIsGiven(t *testing.T) {
	snaps := history(400, base.Add(-400*time.Hour), time.Hour)
	forwards := Compact(snaps, base)

	reversed := make([]Snapshot, len(snaps))
	for i, snap := range snaps {
		reversed[len(snaps)-1-i] = snap
	}
	backwards := Compact(reversed, base)

	assert.ElementsMatch(t, forwards, backwards)
}

func TestTheNewestSnapshotSurvivesEveryRule(t *testing.T) {
	snaps := history(400, base.Add(-400*time.Hour), time.Hour)
	drop := Compact(snaps, base)
	assert.False(t, contains(drop, snaps[len(snaps)-1].ID))
}
