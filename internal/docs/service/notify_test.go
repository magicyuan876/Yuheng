package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/notify"
)

// inbox reads somebody's notifications.
func (p *pageEnv) inbox(t *testing.T, actor *acl.Identity) []*NotificationView {
	t.Helper()
	page, err := p.svc.Pages.Notifications(ctx(), actor, false, "", 0)
	require.NoError(t, err)
	return page.Items
}

// countOf tallies an inbox by kind.
func countOf(items []*NotificationView, kind notify.Kind) int {
	n := 0
	for _, item := range items {
		if item.Kind == kind {
			n++
		}
	}
	return n
}

// backdateNotifications moves somebody's notifications into the past, which
// is how a test reaches the far side of a merge window without waiting.
func (p *pageEnv) backdateNotifications(t *testing.T, userID string, by time.Duration) {
	t.Helper()
	require.NoError(t, p.repos.DB().Model(&model.Notification{}).
		Where("user_id = ?", userID).
		Update("created_at", time.Now().UTC().Add(-by)).Error)
}

func TestCreatingAPageMakesYouAWatcherOfIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	state, err := p.svc.Pages.WatchState(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)
	assert.True(t, state.Watched)
	assert.Equal(t, model.WatchAuthor, state.Reason)
}

func TestCommentingMakesYouAWatcher(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a point")})

	state, err := p.svc.Pages.WatchState(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID))
	require.NoError(t, err)
	assert.True(t, state.Watched)
	assert.Equal(t, model.WatchComment, state.Reason)
}

// Acceptance (T3.3): your own actions do not notify you.
func TestYourOwnCommentDoesNotNotifyYou(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("thinking aloud")})
	assert.Empty(t, p.inbox(t, p.alice))
}

// Commenting on somebody's page is addressed to them.
func TestCommentingTellsThePagesAuthor(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a question")})

	items := p.inbox(t, p.alice)
	require.Len(t, items, 1)
	assert.Equal(t, notify.Commented, items[0].Kind)
	assert.Equal(t, p.bob.UserID, items[0].ActorID)
	assert.Contains(t, string(items[0].Payload), "a question", "the excerpt says what it was about")
}

func TestAReplyTellsThePersonBeingAnswered(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	thread := p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a question")})

	p.comment(t, p.carol, page.Page.ID, CreateCommentInput{
		Body: say("an answer"), ParentID: thread.ID,
	})

	assert.Len(t, p.inbox(t, p.bob), 1, "the person answered hears about it")
}

// Acceptance (T3.3): several comments on one page within ten minutes are one
// notification.
func TestSeveralCommentsInTenMinutesAreOneNotification(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	for _, text := range []string{"first", "second", "third"} {
		p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say(text)})
	}

	items := p.inbox(t, p.alice)
	assert.Equal(t, 1, countOf(items, notify.Commented), "one conversation, one notice")
	assert.Contains(t, string(items[0].Payload), "third", "showing the most recent remark")
}

func TestPastTheWindowACommentIsNewsAgain(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("first")})
	p.backdateNotifications(t, p.alice.UserID, notify.CommentWindow+time.Minute)
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("second")})

	assert.Equal(t, 2, countOf(p.inbox(t, p.alice), notify.Commented))
}

// Folding a new event into a dismissed row would hide it.
func TestACommentAfterYouHaveReadTheLastOneIsANewNotification(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("first")})
	_, err := p.svc.Pages.MarkNotificationsRead(ctx(), p.alice, nil)
	require.NoError(t, err)

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("second")})
	assert.Equal(t, 2, countOf(p.inbox(t, p.alice), notify.Commented))
}

// Acceptance (T3.3): muting stops the notifications.
func TestMutingAPageStopsTellingYouAboutComments(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	_, err := p.svc.Pages.MutePage(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID), true)
	require.NoError(t, err)

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a question")})
	assert.Empty(t, p.inbox(t, p.alice), "including as the page's author")
}

func TestUnmutingStartsTellingYouAgain(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.MutePage(ctx(), p.alice, d, true)
	require.NoError(t, err)
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("while muted")})
	require.Empty(t, p.inbox(t, p.alice))

	_, err = p.svc.Pages.MutePage(ctx(), p.alice, d, false)
	require.NoError(t, err)
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("after unmuting")})
	assert.Len(t, p.inbox(t, p.alice), 1)
}

// Being named is a direct address: it gets through a muted page.
func TestBeingNamedGetsThroughAMutedPage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	_, err := p.svc.Pages.MutePage(ctx(), p.named, p.decision(t, p.named, page.Page.ID), true)
	require.NoError(t, err)

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: mentioning(p.named.UserID)})

	items := p.inbox(t, p.named)
	require.Len(t, items, 1)
	assert.Equal(t, notify.Mentioned, items[0].Kind)
}

// Somebody both watching the page and named in the comment gets the more
// specific notice rather than two.
func TestBeingNamedInAWatchedPageGivesOneNoticeNotTwo(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: mentioning(p.named.UserID)})

	items := p.inbox(t, p.named)
	require.Len(t, items, 1)
	assert.Equal(t, notify.Mentioned, items[0].Kind)
}

func TestBeingNamedMakesYouAWatcher(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: mentioning(p.named.UserID)})

	state, err := p.svc.Pages.WatchState(ctx(), p.named, p.decision(t, p.named, page.Page.ID))
	require.NoError(t, err)
	assert.True(t, state.Watched)
	assert.Equal(t, model.WatchMention, state.Reason)
}

// Only people who chose to watch hear about edits.
func TestAnEditTellsOnlyThoseWhoChoseToWatch(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")

	// Carol chose to watch; bob was enrolled by commenting.
	_, err := p.svc.Pages.WatchPage(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID), true)
	require.NoError(t, err)
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a point")})

	current := p.mustPage(t, page.Page.ID)
	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.Page.ID, BaseVersion: current.YDocVersion,
		YDoc: []byte("state"), Content: textBody("blockaa", "edited body"),
		EditorIDs: []string{p.alice.UserID},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, countOf(p.inbox(t, p.carol), notify.PageUpdated))
	assert.Equal(t, 0, countOf(p.inbox(t, p.bob), notify.PageUpdated),
		"commenting once does not sign you up to every future edit")
}

func TestChoosingToWatchOverridesAnAutomaticReason(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	d := p.decision(t, p.bob, page.Page.ID)

	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a point")})
	_, err := p.svc.Pages.WatchPage(ctx(), p.bob, d, true)
	require.NoError(t, err)

	state, err := p.svc.Pages.WatchState(ctx(), p.bob, d)
	require.NoError(t, err)
	assert.Equal(t, model.WatchManual, state.Reason)
}

func TestUnwatchingStopsEverything(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	d := p.decision(t, p.alice, page.Page.ID)

	_, err := p.svc.Pages.WatchPage(ctx(), p.alice, d, false)
	require.NoError(t, err)

	state, err := p.svc.Pages.WatchState(ctx(), p.alice, d)
	require.NoError(t, err)
	assert.False(t, state.Watched)
}

// An inbox belongs to its reader.
func TestNamingSomebodyElsesNotificationDoesNothingToIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a question")})

	items := p.inbox(t, p.alice)
	require.Len(t, items, 1)

	n, err := p.svc.Pages.MarkNotificationsRead(ctx(), p.carol, []string{items[0].ID})
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Nil(t, p.inbox(t, p.alice)[0].ReadAt, "and it is still unread for its owner")
}

func TestAnInboxCanBeReadAndCleared(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Notes")
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("a question")})

	unread, err := p.svc.Pages.Notifications(ctx(), p.alice, true, "", 0)
	require.NoError(t, err)
	require.Len(t, unread.Items, 1)
	assert.Equal(t, int64(1), unread.Unread)

	_, err = p.svc.Pages.MarkNotificationsRead(ctx(), p.alice, nil)
	require.NoError(t, err)

	after, err := p.svc.Pages.Notifications(ctx(), p.alice, true, "", 0)
	require.NoError(t, err)
	assert.Empty(t, after.Items)
	assert.Equal(t, int64(0), after.Unread)

	_, err = p.svc.Pages.ArchiveNotifications(ctx(), p.alice, nil)
	require.NoError(t, err)
	assert.Empty(t, p.inbox(t, p.alice), "archiving takes it out of the list entirely")
}

func TestAMalformedInboxCursorIsRefused(t *testing.T) {
	p := newPageEnv(t)
	_, err := p.svc.Pages.Notifications(ctx(), p.alice, false, "not-a-cursor", 0)
	require.Error(t, err)
}
