package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

var now = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func watching(userID string) Watcher { return Watcher{UserID: userID} }
func manual(userID string) Watcher   { return Watcher{UserID: userID, Manual: true} }
func muted(userID string) Watcher    { return Watcher{UserID: userID, Muted: true} }
func mutedManual(id string) Watcher  { return Watcher{UserID: id, Manual: true, Muted: true} }

// ---- merging ------------------------------------------------------------------

func TestTheWindowsMatchTheDesign(t *testing.T) {
	assert.Equal(t, 10*time.Minute, MergeWindow(Commented))
	assert.Equal(t, time.Hour, MergeWindow(PageUpdated))
	assert.Equal(t, time.Duration(0), MergeWindow(Mentioned))
	assert.Equal(t, time.Duration(0), MergeWindow(AccessGranted))
}

// Acceptance (T3.3): several comments on one page within ten minutes produce
// one notification.
func TestSeveralCommentsOnOnePageAreOneNotification(t *testing.T) {
	first := Existing{ID: "n1", Kind: Commented, PageID: "p1", CreatedAt: now}
	candidate := Candidate{Kind: Commented, UserID: "bob", PageID: "p1", ActorID: "alice"}

	within := MergeInto(candidate, []Existing{first}, now.Add(9*time.Minute))
	require.NotNil(t, within)
	assert.Equal(t, "n1", within.ID)

	past := MergeInto(candidate, []Existing{first}, now.Add(11*time.Minute))
	assert.Nil(t, past, "past the window it is news again")
}

func TestAnEditIsMergedForAnHour(t *testing.T) {
	existing := Existing{ID: "n1", Kind: PageUpdated, PageID: "p1", CreatedAt: now}
	candidate := Candidate{Kind: PageUpdated, UserID: "bob", PageID: "p1", ActorID: "alice"}

	assert.NotNil(t, MergeInto(candidate, []Existing{existing}, now.Add(59*time.Minute)))
	assert.Nil(t, MergeInto(candidate, []Existing{existing}, now.Add(61*time.Minute)))
}

// Rolling these up would turn "you have been mentioned" into "something
// happened", which is the one thing a notification must not do.
func TestBeingNamedOrGrantedAccessIsNeverMerged(t *testing.T) {
	for _, kind := range []Kind{Mentioned, AccessGranted} {
		existing := Existing{ID: "n1", Kind: kind, PageID: "p1", CreatedAt: now}
		candidate := Candidate{Kind: kind, UserID: "bob", PageID: "p1", ActorID: "alice"}
		assert.Nil(t, MergeInto(candidate, []Existing{existing}, now.Add(time.Second)), string(kind))
	}
}

// It has done its job; folding a new event into it would hide that event
// behind a row the reader has dismissed.
func TestANotificationAlreadyReadIsNotMergedInto(t *testing.T) {
	read := now.Add(time.Minute)
	existing := Existing{ID: "n1", Kind: Commented, PageID: "p1", CreatedAt: now, ReadAt: &read}
	candidate := Candidate{Kind: Commented, UserID: "bob", PageID: "p1", ActorID: "alice"}

	assert.Nil(t, MergeInto(candidate, []Existing{existing}, now.Add(2*time.Minute)))
}

func TestOnlyTheSameKindAboutTheSamePageMerges(t *testing.T) {
	existing := []Existing{
		{ID: "other-page", Kind: Commented, PageID: "p2", CreatedAt: now},
		{ID: "other-kind", Kind: PageUpdated, PageID: "p1", CreatedAt: now},
	}
	candidate := Candidate{Kind: Commented, UserID: "bob", PageID: "p1", ActorID: "alice"}

	assert.Nil(t, MergeInto(candidate, existing, now.Add(time.Minute)))
}

func TestMergingWithNothingToMergeInto(t *testing.T) {
	candidate := Candidate{Kind: Commented, UserID: "bob", PageID: "p1", ActorID: "alice"}
	assert.Nil(t, MergeInto(candidate, nil, now))

	noPage := Candidate{Kind: Commented, UserID: "bob", ActorID: "alice"}
	assert.Nil(t, MergeInto(noPage, []Existing{{ID: "n1", Kind: Commented, CreatedAt: now}}, now))
}

// ---- who hears about it -------------------------------------------------------

// Acceptance (T3.3): your own actions do not notify you.
func TestNobodyIsToldAboutTheirOwnDoing(t *testing.T) {
	watchers := []Watcher{watching("alice"), watching("bob")}

	assert.Equal(t, []string{"bob"}, CommentAudience(watchers, "alice", "", "alice"))
	assert.Equal(t, []string{}, MentionAudience([]string{"alice"}, "alice"))
	assert.Equal(t, []string{}, UpdateAudience([]Watcher{manual("alice")}, "alice"))
}

// Commenting on somebody's page is addressed to them, and answering somebody
// is addressed to them — neither should need a button to have been pressed.
func TestACommentReachesTheAuthorAndThePersonAnswered(t *testing.T) {
	got := CommentAudience(nil, "author", "answered", "alice")
	assert.ElementsMatch(t, []string{"author", "answered"}, got)
}

// Muting has to silence the page's author too, or the control is a lie: they
// said they did not want to hear about this page, and writing the page is not
// consent to hear about it forever.
func TestMutingSilencesEvenTheAuthorAndThePersonAnswered(t *testing.T) {
	watchers := []Watcher{muted("author"), muted("answered"), watching("bob")}
	got := CommentAudience(watchers, "author", "answered", "alice")
	assert.Equal(t, []string{"bob"}, got)
}

// Acceptance (T3.3): muting stops the notifications.
func TestMutingAPageStopsItsComments(t *testing.T) {
	watchers := []Watcher{watching("bob"), muted("carol")}
	assert.Equal(t, []string{"bob"}, CommentAudience(watchers, "", "", "alice"))
}

func TestMutingStopsEditNoticesToo(t *testing.T) {
	assert.Equal(t, []string{}, UpdateAudience([]Watcher{mutedManual("bob")}, "alice"))
	assert.Equal(t, []string{"bob"}, UpdateAudience([]Watcher{manual("bob")}, "alice"))
}

// Being named is a direct address: somebody who muted a page still wants to
// know they were asked a question on it.
func TestMutingDoesNotSilenceBeingNamed(t *testing.T) {
	assert.Equal(t, []string{"carol"}, MentionAudience([]string{"carol"}, "alice"))
}

// Everybody who once answered a question on a busy page would otherwise be
// told about every edit to it forever.
func TestOnlyPeopleWhoChoseToWatchHearAboutEdits(t *testing.T) {
	watchers := []Watcher{manual("chose"), watching("enrolled-by-commenting")}
	assert.Equal(t, []string{"chose"}, UpdateAudience(watchers, "alice"))
}

func TestNobodyIsToldTwice(t *testing.T) {
	watchers := []Watcher{watching("bob"), watching("bob")}
	got := CommentAudience(watchers, "bob", "bob", "alice")
	assert.Equal(t, []string{"bob"}, got)
}

func TestAnAudienceWithNobodyInItIsEmptyRatherThanNil(t *testing.T) {
	// A caller ranging over the result should not have to check for nil.
	assert.NotNil(t, CommentAudience(nil, "", "", "alice"))
	assert.NotNil(t, MentionAudience(nil, "alice"))
	assert.NotNil(t, UpdateAudience(nil, "alice"))
}

func TestBlanksAreNotRecipients(t *testing.T) {
	assert.Equal(t, []string{}, CommentAudience([]Watcher{watching("")}, "", "", "alice"))
	assert.Equal(t, []string{}, MentionAudience([]string{"", ""}, "alice"))
}

// ---- watching -----------------------------------------------------------------

func TestTheActionsThatEnrolAWatcher(t *testing.T) {
	for action, want := range map[string]model.WatchReason{
		"create":  model.WatchAuthor,
		"comment": model.WatchComment,
		"mention": model.WatchMention,
	} {
		reason, ok := AutoWatch(action)
		require.True(t, ok, action)
		assert.Equal(t, want, reason, action)
	}

	_, ok := AutoWatch("read")
	assert.False(t, ok, "reading a page does not sign you up to it")
}

// Somebody chose a manual watch; being mentioned afterwards must not quietly
// downgrade them to hearing less.
func TestAManualWatchIsNeverDowngraded(t *testing.T) {
	assert.False(t, Upgrades(model.WatchManual, model.WatchComment))
	assert.False(t, Upgrades(model.WatchManual, model.WatchMention))
	assert.False(t, Upgrades(model.WatchManual, model.WatchManual))
}

func TestChoosingToWatchUpgradesAnAutomaticOne(t *testing.T) {
	assert.True(t, Upgrades(model.WatchComment, model.WatchManual))
	assert.True(t, Upgrades(model.WatchAuthor, model.WatchManual))
}

// You did become a watcher by commenting, whatever happened afterwards.
func TestOneAutomaticReasonDoesNotReplaceAnother(t *testing.T) {
	assert.False(t, Upgrades(model.WatchComment, model.WatchMention))
	assert.False(t, Upgrades(model.WatchAuthor, model.WatchComment))
}
