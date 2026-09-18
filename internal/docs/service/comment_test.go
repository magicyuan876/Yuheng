package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/comment"
)

// say builds a one-paragraph comment body.
func say(text string) json.RawMessage {
	quoted, _ := json.Marshal(text)
	return json.RawMessage(
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":` +
			string(quoted) + `}]}]}`)
}

// mentioning builds a comment body that names somebody.
func mentioning(userID string) json.RawMessage {
	quoted, _ := json.Marshal(userID)
	return json.RawMessage(
		`{"type":"doc","content":[{"type":"paragraph","content":[` +
			`{"type":"text","text":"ask "},` +
			`{"type":"mention","attrs":{"userId":` + string(quoted) + `}}]}]}`)
}

// anchorJSON is a well-formed relative position, as the editor would send.
const anchorJSON = `{
	"start": {"type": {"client": 1, "clock": 2}, "item": {"client": 1, "clock": 7}, "assoc": 0},
	"end":   {"type": {"client": 1, "clock": 2}, "item": {"client": 1, "clock": 19}, "assoc": -1}
}`

func (p *pageEnv) comment(t *testing.T, actor *acl.Identity, pageID string,
	in CreateCommentInput,
) *CommentView {
	t.Helper()
	view, err := p.svc.Pages.CreateComment(ctx(), actor, p.decision(t, actor, pageID), in)
	require.NoError(t, err)
	return view
}

func (p *pageEnv) threads(t *testing.T, actor *acl.Identity, pageID string, includeResolved bool) *CommentList {
	t.Helper()
	list, err := p.svc.Pages.Comments(ctx(), actor, p.decision(t, actor, pageID), includeResolved)
	require.NoError(t, err)
	return list
}

// Acceptance (T3.2): a reader may comment but may not edit the page.
//
// Commenting is not editing. Somebody invited to review a page has to be able
// to say what they think of it without being handed the ability to change it.
func TestAReaderMayCommentButNotEditThePage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	view := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("I have a question")})
	assert.Equal(t, p.carol.UserID, view.CreatorID)

	// The same person cannot write the page.
	_, err := p.svc.Pages.ReplaceContent(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID),
		ReplaceInput{Content: textBody("blockaa", "rewritten by a reader")})
	require.Error(t, err, "a reader may not rewrite the page they are reviewing")
}

func TestACommentOnAPassageKeepsItsAnchorAndQuotation(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	view := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{
		Body:       say("is this still true?"),
		Anchor:     json.RawMessage(anchorJSON),
		QuotedText: "  the   passage \n being discussed ",
	})

	assert.Equal(t, comment.Inline, view.Place)
	assert.NotEmpty(t, view.Anchor)
	assert.Equal(t, "the passage being discussed", view.Quoted, "whitespace is normalised")
}

func TestACommentWithNoAnchorIsAboutThePage(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	view := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("overall, good")})
	assert.Equal(t, comment.Page, view.Place)
	assert.Empty(t, view.Anchor)
}

func TestARejectedAnchorRejectsTheComment(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("x"), Anchor: json.RawMessage(`{"payload": "arbitrary"}`)})
	require.Error(t, err, "an anchor that is not one is refused rather than stored")
}

func TestRepliesNestOneLevelAndNoFurther(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	thread := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("a point")})
	reply := p.comment(t, p.bob, page.Page.ID, CreateCommentInput{
		Body: say("an answer"), ParentID: thread.ID,
	})
	assert.Equal(t, thread.ID, reply.ParentID)

	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("answering the answer"), ParentID: reply.ID})
	require.Error(t, err, "a thread is a remark and its answers, not a tree")
}

func TestThreadsComeBackWithTheirRepliesNested(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	first := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("first point")})
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("answer one"), ParentID: first.ID})
	p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("answer two"), ParentID: first.ID})
	p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("second point")})

	list := p.threads(t, p.alice, page.Page.ID, false)
	require.Len(t, list.Items, 2, "two threads, not four comments")
	assert.Len(t, list.Items[0].Replies, 2)
	assert.Empty(t, list.Items[1].Replies)
	assert.Equal(t, int64(2), list.Total)
	assert.Equal(t, int64(2), list.Open)
}

// A remark belongs to whoever made it. An admin rewriting somebody's words
// while their name stays on them is not moderation.
func TestOnlyTheAuthorMayEditAComment(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	mine := p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("original wording")})

	updated, err := p.svc.Pages.UpdateComment(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID),
		mine.ID, say("better wording"))
	require.NoError(t, err)
	assert.NotNil(t, updated.EditedAt, "and it says it was edited")

	// Alice administers the space and still may not.
	_, err = p.svc.Pages.UpdateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		mine.ID, say("rewritten by somebody else"))
	require.Error(t, err)
}

// A thread belongs to the page rather than to whoever started it.
func TestAWriterMayResolveAnybodysThread(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	thread := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("a reader's point")})

	resolved, err := p.svc.Pages.ResolveComment(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID),
		thread.ID, true)
	require.NoError(t, err)
	require.NotNil(t, resolved.ResolvedAt)
	assert.Equal(t, p.bob.UserID, resolved.ResolvedBy)

	reopened, err := p.svc.Pages.ResolveComment(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID),
		thread.ID, false)
	require.NoError(t, err)
	assert.Nil(t, reopened.ResolvedAt)
	assert.Empty(t, reopened.ResolvedBy)
}

// Withdrawing your own point should not need permission you do not have.
func TestAReaderMayResolveTheirOwnThread(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	mine := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("never mind")})
	theirs := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("somebody else's")})

	_, err := p.svc.Pages.ResolveComment(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID),
		mine.ID, true)
	require.NoError(t, err)

	_, err = p.svc.Pages.ResolveComment(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID),
		theirs.ID, true)
	require.Error(t, err, "but not anybody else's")
}

func TestAResolvedThreadIsHiddenWithItsReplies(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	settled := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("settled point")})
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("an answer"), ParentID: settled.ID})
	p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("open point")})

	_, err := p.svc.Pages.ResolveComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		settled.ID, true)
	require.NoError(t, err)

	open := p.threads(t, p.alice, page.Page.ID, false)
	require.Len(t, open.Items, 1, "the resolved thread is out of the way")
	assert.Equal(t, int64(1), open.Open)
	assert.Equal(t, int64(2), open.Total, "but still counted")

	all := p.threads(t, p.alice, page.Page.ID, true)
	require.Len(t, all.Items, 2)
	for _, item := range all.Items {
		if item.ID == settled.ID {
			assert.Len(t, item.Replies, 1, "and its answers come back with it")
		}
	}
}

func TestAReplyIsResolvedWithItsThreadRatherThanOnItsOwn(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	thread := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("a point")})
	reply := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("an answer"), ParentID: thread.ID})

	_, err := p.svc.Pages.ResolveComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		reply.ID, true)
	require.Error(t, err)
}

func TestTheAuthorAndAnAdminMayDeleteAComment(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	own := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("mine")})
	require.NoError(t, p.svc.Pages.DeleteComment(ctx(), p.carol,
		p.decision(t, p.carol, page.Page.ID), own.ID))

	// Moderation: alice administers the space.
	theirs := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("also mine")})
	require.NoError(t, p.svc.Pages.DeleteComment(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), theirs.ID))

	// Bob writes the page but neither wrote the comment nor administers.
	third := p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("still mine")})
	err := p.svc.Pages.DeleteComment(ctx(), p.bob, p.decision(t, p.bob, page.Page.ID), third.ID)
	require.Error(t, err)
}

// A reply to a remark that is gone has nothing left to answer.
func TestDeletingAThreadTakesItsRepliesWithIt(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	thread := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("a point")})
	p.comment(t, p.bob, page.Page.ID, CreateCommentInput{Body: say("an answer"), ParentID: thread.ID})

	require.NoError(t, p.svc.Pages.DeleteComment(ctx(), p.alice,
		p.decision(t, p.alice, page.Page.ID), thread.ID))
	assert.Empty(t, p.threads(t, p.alice, page.Page.ID, true).Items)
}

// Naming another page's comment must not reach it.
func TestACommentOfAnotherPageIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	mine := p.create(t, p.alice, nil, "Mine")
	theirs := p.create(t, p.alice, nil, "Theirs")
	other := p.comment(t, p.alice, theirs.Page.ID, CreateCommentInput{Body: say("over there")})

	d := p.decision(t, p.alice, mine.Page.ID)
	_, err := p.svc.Pages.UpdateComment(ctx(), p.alice, d, other.ID, say("moved"))
	require.Error(t, err)

	err = p.svc.Pages.DeleteComment(ctx(), p.alice, d, other.ID)
	require.Error(t, err)

	_, err = p.svc.Pages.ResolveComment(ctx(), p.alice, d, other.ID, true)
	require.Error(t, err)
}

// The client should not have to reimplement the rules and disagree with them.
func TestEachViewSaysWhatThisCallerMayDo(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	p.comment(t, p.carol, page.Page.ID, CreateCommentInput{Body: say("a reader's point")})

	asCarol := p.threads(t, p.carol, page.Page.ID, false).Items[0]
	assert.True(t, asCarol.CanEdit)
	assert.True(t, asCarol.CanDelete)
	assert.True(t, asCarol.CanResolve, "their own thread")

	asBob := p.threads(t, p.bob, page.Page.ID, false).Items[0]
	assert.False(t, asBob.CanEdit, "not their words")
	assert.False(t, asBob.CanDelete, "a writer is not a moderator")
	assert.True(t, asBob.CanResolve, "but a writer may settle the thread")

	asAlice := p.threads(t, p.alice, page.Page.ID, false).Items[0]
	assert.False(t, asAlice.CanEdit)
	assert.True(t, asAlice.CanDelete, "an admin may moderate")
}

func TestARepliesViewNeverOffersResolve(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")
	thread := p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("a point")})
	p.comment(t, p.alice, page.Page.ID, CreateCommentInput{Body: say("an answer"), ParentID: thread.ID})

	reply := p.threads(t, p.alice, page.Page.ID, false).Items[0].Replies[0]
	assert.False(t, reply.CanResolve)
}

func TestAnEmptyCommentIsRefused(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Proposal")

	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("   ")})
	require.Error(t, err)
}

func TestSomebodyWithNoAccessToThePageCannotComment(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Private")
	p.restrict(t, page.Page.ID, "alice")

	_, err := p.svc.Pages.CreateComment(ctx(), p.carol, p.decision(t, p.carol, page.Page.ID),
		CreateCommentInput{Body: say("let me in")})
	require.Error(t, err)
}
