// Package history holds the two decisions behind page history: when a
// snapshot is worth taking, and which snapshots are worth keeping.
//
// Both are pure functions over plain values, with the clock passed in, so
// they are exercised directly rather than through a database and a queue.
// Everything else about history — reading rows, writing them, restoring one —
// is mechanical once these two are settled.
package history

import (
	"sort"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// How often an ordinary editing session leaves a snapshot behind.
const (
	// Interval is the gap between snapshots of a page being edited.
	Interval = 5 * time.Minute
	// FastInterval applies while a page is new, when edits are large and a
	// mistake is most likely to be worth undoing.
	FastInterval = time.Minute
	// FastWindow is how long after creation a page counts as new.
	FastWindow = 5 * time.Minute
)

// KeepRecent is how many of the most recent snapshots survive compaction
// untouched, whatever else the policy says.
const KeepRecent = 200

// Compaction thins what is older than the recent window: one snapshot per day
// for a while, then one per month.
const (
	DailyWindow = 30 * 24 * time.Hour
	// Beyond this, only one snapshot a month is kept.
	MonthlyAfter = DailyWindow
)

// Snapshot is what the policy needs to know about an existing revision.
//
// A subset of model.PageRevision rather than the row itself, so the policy has
// no opinion about storage and can be given rows from anywhere.
type Snapshot struct {
	ID        string
	CreatedAt time.Time
	Reason    model.RevisionReason
	// EditorIDs is who had edited the page when this was taken.
	EditorIDs []string
}

// Candidate describes the save that has just happened.
type Candidate struct {
	// ContentChanged is false when the save re-encoded an unchanged body.
	ContentChanged bool
	// Reason is why this save happened; anything but interval is deliberate.
	Reason model.RevisionReason
	// PageCreatedAt decides whether the fast interval applies.
	PageCreatedAt time.Time
	// EditorIDs is who has edited since the last snapshot.
	EditorIDs []string
	// Empty is true when the document holds nothing worth keeping, which is
	// what a page looks like between being created and being written.
	Empty bool
}

// ShouldSnapshot decides whether this save leaves a snapshot behind.
//
// The rules, in the order they are applied:
//
//  1. A save that did not change the body never snapshots. Re-encoding is not
//     an edit, and a history full of identical entries is worse than none.
//  2. A deliberate save — a publish, an import, a restore, an explicit "save
//     a version" — always snapshots. These are the entries somebody will
//     actually look for, and an interval must not swallow them.
//  3. The first snapshot of an empty page is skipped. A page is created
//     before it is written; the blank it starts as is not a version of
//     anything.
//  4. The first snapshot of a page that has content is always taken, so
//     there is something to go back to from the very first edit.
//  5. A different set of people has edited since the last snapshot: take one.
//     Whose work is in a version is most of what makes history readable, and
//     letting one person's changes be absorbed into another's entry loses
//     exactly that.
//  6. Otherwise, the interval — one minute while the page is new, five
//     minutes after that.
//
// Note how this differs from a debounce: the gap is measured from the last
// *snapshot*, not from the last edit. Somebody typing for an hour gets twelve
// entries rather than one entry an hour after they stop, which is what a
// person looking for "what it said before lunch" actually needs.
func ShouldSnapshot(c Candidate, last *Snapshot, now time.Time) bool {
	if !c.ContentChanged {
		return false
	}
	if c.Reason != "" && c.Reason != model.RevisionInterval {
		return true
	}
	if last == nil {
		return !c.Empty
	}
	if !sameEditors(c.EditorIDs, last.EditorIDs) {
		return true
	}
	return now.Sub(last.CreatedAt) >= intervalFor(c.PageCreatedAt, now)
}

func intervalFor(pageCreatedAt, now time.Time) time.Duration {
	if pageCreatedAt.IsZero() {
		return Interval
	}
	if now.Sub(pageCreatedAt) < FastWindow {
		return FastInterval
	}
	return Interval
}

// sameEditors compares two editor sets, ignoring order and duplicates.
//
// An empty candidate set means "nobody new is recorded", which must not count
// as a change: the collaboration service reports contributors per save, and a
// save with none of them attached should not force a snapshot.
func sameEditors(candidate, last []string) bool {
	if len(candidate) == 0 {
		return true
	}
	seen := make(map[string]bool, len(last))
	for _, id := range last {
		seen[id] = true
	}
	for _, id := range candidate {
		if !seen[id] {
			return false
		}
	}
	return true
}

// Compact decides which snapshots to delete.
//
// It is given every snapshot of one page and returns the ids to remove, so
// the caller does one delete and the decision stays testable. The rules:
//
//   - The most recent KeepRecent are kept whatever they are. This is the
//     acceptance criterion for this work package, and it comes first so no
//     other rule can eat into it.
//   - A snapshot that was taken deliberately is never deleted. A publish, an
//     import or a restore is a landmark somebody chose to create; thinning it
//     away to save a row would be deleting the only entries anybody named.
//   - Within the last DailyWindow, one snapshot a day survives: the last one
//     of each day, because that is the state the day ended in.
//   - Older than that, one a month, chosen the same way.
//
// The newest snapshot of the page is never deleted even if some rule would
// otherwise reach it — a page with history must have a most recent version.
func Compact(snapshots []Snapshot, now time.Time) []string {
	if len(snapshots) <= KeepRecent {
		return nil
	}
	// Newest first, so "the most recent KeepRecent" is a prefix.
	ordered := make([]Snapshot, len(snapshots))
	copy(ordered, snapshots)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID > ordered[j].ID
		}
		return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
	})

	var drop []string
	// Which bucket has already kept a snapshot. Walking newest first means
	// the one kept per bucket is the last of that day or month.
	kept := map[string]bool{}
	for i, snap := range ordered {
		if i < KeepRecent {
			continue
		}
		if snap.Reason != "" && snap.Reason != model.RevisionInterval {
			continue
		}
		key := bucketOf(snap.CreatedAt, now)
		if !kept[key] {
			kept[key] = true
			continue
		}
		drop = append(drop, snap.ID)
	}
	return drop
}

// bucketOf names the period a snapshot falls in: a day while it is recent
// enough to be worth that resolution, a month once it is not.
func bucketOf(at, now time.Time) string {
	if now.Sub(at) <= MonthlyAfter {
		return at.UTC().Format("2006-01-02")
	}
	return at.UTC().Format("2006-01")
}
