package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/stretchr/testify/require"
)

// newLeaseEnv builds the Lite shape: docs enabled, no collaboration service.
func newLeaseEnv(t *testing.T) *pageEnv {
	t.Helper()
	return newPageEnvWith(t)
}

// newCollaborativeEnv builds the standard shape, where leases do not apply.
func newCollaborativeEnv(t *testing.T) *pageEnv {
	t.Helper()
	return newPageEnvWith(t, func(d *Deps) { d.CollabURL = "ws://collab:1234" })
}

// ydoc stands in for a real Yjs state. Nothing in Go parses it — the byte
// string is opaque to the server and only ever handed back to a browser.
func ydoc(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func docBody(text string) json.RawMessage {
	doc := map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": text},
		}},
	}}
	raw, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return raw
}

func TestLeaseRoutesRejectedWhenACollaborationServiceIsConfigured(t *testing.T) {
	p := newCollaborativeEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)
	ls := p.svc.Leases

	_, err := ls.Acquire(ctx(), p.alice, d, "s1")
	require.Equal(t, 409, httpCode(t, err))
	_, err = ls.Current(ctx(), p.alice, d)
	require.Equal(t, 409, httpCode(t, err))
	require.Equal(t, 409, httpCode(t, ls.Release(ctx(), p.alice, d, "s1")))
	_, err = ls.LoadYDoc(ctx(), d)
	require.Equal(t, 409, httpCode(t, err))
	_, err = ls.SaveYDoc(ctx(), p.alice, d, SaveYDocInput{SessionID: "s1", YDoc: ydoc("x"), Content: docBody("hi")})
	require.Equal(t, 409, httpCode(t, err))
}

func TestLeaseAcquireAndRenewByTheSameSession(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)

	first, err := p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)
	require.True(t, first.Held)
	require.True(t, first.HeldByMe)
	require.Equal(t, "alice", first.Holder.UserID)
	require.Equal(t, int(LeaseRenewInterval/time.Second), first.RenewAfterSeconds)
	require.True(t, first.ExpiresAt.After(time.Now().UTC()))

	second, err := p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)
	require.True(t, second.HeldByMe, "renewing must not lose the lease")
	require.False(t, second.ExpiresAt.Before(first.ExpiresAt))

	require.Contains(t, p.eventTypes(), events.PageLease)
}

func TestLeaseAcquireReportsTheOtherHolder(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	held, err := p.svc.Leases.Acquire(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-1")
	require.NoError(t, err)
	require.True(t, held.HeldByMe)

	// Bob is a writer too, but Alice got there first.
	view, err := p.svc.Leases.Acquire(ctx(), p.bob, p.decision(t, p.bob, page.ID), "tab-2")
	require.NoError(t, err)
	require.True(t, view.Held)
	require.False(t, view.HeldByMe)
	require.Equal(t, "alice", view.Holder.UserID)
	require.Equal(t, "alice", view.Holder.Username, "the read-only banner needs a name, not an id")
}

func TestLeaseAcquireTakesOverAfterTheHolderStopsRenewing(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	// Alice's tab died five minutes ago: write the row the way it would look.
	expired := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	_, acquired, err := p.repos.Leases.Acquire(ctx(), model.EditLease{
		PageID: page.ID, TenantID: 1, UserID: "alice", SessionID: "tab-1", ExpiresAt: expired,
	}, expired.Add(-LeaseTTL))
	require.NoError(t, err)
	require.True(t, acquired)

	view, err := p.svc.Leases.Acquire(ctx(), p.bob, p.decision(t, p.bob, page.ID), "tab-2")
	require.NoError(t, err)
	require.True(t, view.HeldByMe, "an abandoned page must be takeable without an administrator")
	require.Equal(t, "bob", view.Holder.UserID)
}

func TestLeaseAcquireNeedsWriteAccess(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	// Carol reads the space.
	_, err := p.svc.Leases.Acquire(ctx(), p.carol, p.decision(t, p.carol, page.ID), "tab-1")
	require.Equal(t, 403, httpCode(t, err))

	// Somebody with no membership at all must not even learn the page exists.
	_, err = p.svc.Leases.Acquire(ctx(), p.viewer, p.decision(t, p.viewer, page.ID), "tab-1")
	require.Equal(t, 404, httpCode(t, err))
}

func TestLeaseAcquireRefusesALockedPage(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	require.NoError(t, p.repos.Pages.UpdateMeta(ctx(), 1, page.ID, map[string]any{"is_locked": true}))

	_, err := p.svc.Leases.Acquire(ctx(), p.bob, p.decision(t, p.bob, page.ID), "tab-1")
	require.Equal(t, 403, httpCode(t, err))

	// A space admin may still edit a locked page, as everywhere else.
	view, err := p.svc.Leases.Acquire(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-2")
	require.NoError(t, err)
	require.True(t, view.HeldByMe)
}

func TestLeaseAcquireRejectsAnUnusableSessionID(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)

	for _, bad := range []string{"", "   ", "tab\n1", string(make([]rune, MaxSessionIDRunes+1))} {
		_, err := p.svc.Leases.Acquire(ctx(), p.alice, d, bad)
		require.Equal(t, 400, httpCode(t, err), "session id %q", bad)
	}
}

func TestLeaseCurrentTellsReadersWhoIsEditing(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	free, err := p.svc.Leases.Current(ctx(), p.carol, p.decision(t, p.carol, page.ID))
	require.NoError(t, err)
	require.False(t, free.Held)

	_, err = p.svc.Leases.Acquire(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-1")
	require.NoError(t, err)

	view, err := p.svc.Leases.Current(ctx(), p.carol, p.decision(t, p.carol, page.ID))
	require.NoError(t, err)
	require.True(t, view.Held)
	require.False(t, view.HeldByMe)
	require.Equal(t, "alice", view.Holder.UserID)

	mine, err := p.svc.Leases.Current(ctx(), p.alice, p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	require.True(t, mine.HeldByMe, "the holder must recognise its own lease across a reload")
}

func TestLeaseReleaseOnlyFreesTheHoldersOwnLease(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	_, err := p.svc.Leases.Acquire(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-1")
	require.NoError(t, err)

	// Bob closing his read-only tab must not unlock Alice's page.
	require.NoError(t, p.svc.Leases.Release(ctx(), p.bob, p.decision(t, p.bob, page.ID), "tab-2"))
	view, err := p.svc.Leases.Current(ctx(), p.carol, p.decision(t, p.carol, page.ID))
	require.NoError(t, err)
	require.True(t, view.Held)

	require.NoError(t, p.svc.Leases.Release(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-1"))
	view, err = p.svc.Leases.Current(ctx(), p.carol, p.decision(t, p.carol, page.ID))
	require.NoError(t, err)
	require.False(t, view.Held, "releasing must let the next person in immediately")
}

func TestLeaseLoadYDocMaterialisesFromTheStoredBody(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	// A page nobody has edited has no Yjs state; the client builds one.
	first, err := p.svc.Leases.LoadYDoc(ctx(), p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	require.Empty(t, first.YDoc)
	require.NotEmpty(t, first.Content)
	require.Equal(t, int64(0), first.YDocVersion)

	_, err = p.svc.Leases.Acquire(ctx(), p.alice, p.decision(t, p.alice, page.ID), "tab-1")
	require.NoError(t, err)
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, p.decision(t, p.alice, page.ID), SaveYDocInput{
		SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("yjs-state"), Content: docBody("hello"),
	})
	require.NoError(t, err)

	after, err := p.svc.Leases.LoadYDoc(ctx(), p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	require.Empty(t, after.Content, "once a Yjs state exists it is the only truth handed out")
	decoded, err := base64.StdEncoding.DecodeString(after.YDoc)
	require.NoError(t, err)
	require.Equal(t, "yjs-state", string(decoded))
	require.Equal(t, int64(1), after.YDocVersion)
}

func TestLeaseSaveYDocGoesThroughTheSharedPersistPath(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)
	_, err := p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)

	res, err := p.svc.Leases.SaveYDoc(ctx(), p.alice, d, SaveYDocInput{
		SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("state-1"), Content: docBody("first draft"),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.YDocVersion)
	require.True(t, res.Lease.HeldByMe, "saving renews the lease so typing alone keeps the page")

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, "first draft", stored.TextContent, "the REST save must render like the collaboration one")
	require.Equal(t, 2, stored.WordCount)
	require.Contains(t, stored.ContributorIDs, "alice")
	require.Contains(t, p.eventTypes(), events.PageContent)
}

func TestLeaseSaveYDocNeedsALiveLease(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)
	in := SaveYDocInput{SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("state"), Content: docBody("text")}

	_, err := p.svc.Leases.SaveYDoc(ctx(), p.alice, d, in)
	require.Equal(t, 409, httpCode(t, err), "saving without ever taking the lease must be refused")

	_, err = p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)

	// Another tab of the same person is a different session.
	other := in
	other.SessionID = "tab-2"
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, other)
	require.Equal(t, 409, httpCode(t, err))

	// And so is another person.
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.bob, p.decision(t, p.bob, page.ID), in)
	require.Equal(t, 409, httpCode(t, err))
}

func TestLeaseSaveYDocRefusesAStaleBaseVersion(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)
	_, err := p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, SaveYDocInput{
		SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("state-1"), Content: docBody("one"),
	})
	require.NoError(t, err)

	// A second save still claiming version 0 would silently drop the first.
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, SaveYDocInput{
		SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("state-2"), Content: docBody("two"),
	})
	require.ErrorIs(t, err, repository.ErrConflict)
}

func TestLeaseSaveYDocValidatesItsPayload(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	d := p.decision(t, p.alice, page.ID)
	_, err := p.svc.Leases.Acquire(ctx(), p.alice, d, "tab-1")
	require.NoError(t, err)

	base := SaveYDocInput{SessionID: "tab-1", BaseVersion: 0, YDoc: ydoc("state"), Content: docBody("text")}

	noContent := base
	noContent.Content = nil
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, noContent)
	require.Equal(t, 400, httpCode(t, err))

	notBase64 := base
	notBase64.YDoc = "!!! not base64 !!!"
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, notBase64)
	require.Equal(t, 400, httpCode(t, err))

	emptyYDoc := base
	emptyYDoc.YDoc = ""
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, emptyYDoc)
	require.Equal(t, 400, httpCode(t, err))

	unknownNode := base
	unknownNode.Content = json.RawMessage(`{"type":"doc","content":[{"type":"evilNode"}]}`)
	_, err = p.svc.Leases.SaveYDoc(ctx(), p.alice, d, unknownNode)
	require.Equal(t, 400, httpCode(t, err), "the REST path must validate exactly like the collaboration path")
}

// TestLeaseAlternatingEditorsKeepEveryChange is the work package's headline
// acceptance criterion: two people editing a page in turn, in a deployment
// with no collaboration service, must not lose each other's work.
func TestLeaseAlternatingEditorsKeepEveryChange(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	// Alice takes the page and writes.
	aliceDecision := p.decision(t, p.alice, page.ID)
	_, err := p.svc.Leases.Acquire(ctx(), p.alice, aliceDecision, "alice-tab")
	require.NoError(t, err)
	saved, err := p.svc.Leases.SaveYDoc(ctx(), p.alice, aliceDecision, SaveYDocInput{
		SessionID: "alice-tab", BaseVersion: 0, YDoc: ydoc("state-alice"), Content: docBody("alice wrote this"),
	})
	require.NoError(t, err)

	// Bob cannot take the page while she holds it.
	blocked, err := p.svc.Leases.Acquire(ctx(), p.bob, p.decision(t, p.bob, page.ID), "bob-tab")
	require.NoError(t, err)
	require.False(t, blocked.HeldByMe)

	// She hands it back; he picks up her text, not an empty page.
	require.NoError(t, p.svc.Leases.Release(ctx(), p.alice, aliceDecision, "alice-tab"))
	bobDecision := p.decision(t, p.bob, page.ID)
	taken, err := p.svc.Leases.Acquire(ctx(), p.bob, bobDecision, "bob-tab")
	require.NoError(t, err)
	require.True(t, taken.HeldByMe)
	require.Equal(t, saved.YDocVersion, taken.YDocVersion, "the new editor starts from the stored version")

	state, err := p.svc.Leases.LoadYDoc(ctx(), bobDecision)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(state.YDoc)
	require.NoError(t, err)
	require.Equal(t, "state-alice", string(decoded))

	_, err = p.svc.Leases.SaveYDoc(ctx(), p.bob, bobDecision, SaveYDocInput{
		SessionID: "bob-tab", BaseVersion: state.YDocVersion,
		YDoc: ydoc("state-bob"), Content: docBody("alice wrote this and bob added more"),
	})
	require.NoError(t, err)

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, "alice wrote this and bob added more", stored.TextContent)
	require.Equal(t, int64(2), stored.YDocVersion)
	require.Subset(t, []string(stored.ContributorIDs), []string{"alice", "bob"})
}

// A page in the trash is never leasable: the guard that fronts the lease
// routes resolves live pages only, so the request is answered 410/404 before
// the service is reached.
func TestLeaseRoutesAreUnreachableForATrashedPage(t *testing.T) {
	p := newLeaseEnv(t)
	page := p.create(t, p.alice, nil, "Page")
	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.ID))
	require.NoError(t, err)

	_, err = p.resolver.Page(ctx(), p.alice, page.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
}
