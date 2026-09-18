package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/stretchr/testify/require"
)

// pageEnv is the service env plus one private space administered by alice
// with bob as writer and carol as reader.
type pageEnv struct {
	*env
	alice, bob, carol, viewer *acl.Identity
	// named is a reader whose id is long enough to appear in a mention;
	// docs-schema's `id` format wants 8-36 characters and the short names
	// above do not qualify. Real user ids are UUIDs.
	named *acl.Identity
	space *model.Space
}

func newPageEnv(t *testing.T) *pageEnv { return newPageEnvWith(t) }

// newPageEnvWith builds the fixture with extra dependencies (the
// collaboration client, a token validator, limits).
func newPageEnvWith(t *testing.T, opts ...func(*Deps)) *pageEnv {
	t.Helper()
	e := newEnv(t, opts...)
	p := &pageEnv{env: e, alice: e.identity("alice")}
	sp, err := e.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Handbook"})
	require.NoError(t, err)
	p.space = sp.Space
	_, err = e.svc.Spaces.SetMembers(ctx(), p.alice, sp.Space, []MemberInput{
		{Type: model.PrincipalUser, ID: "bob", Role: model.RoleWriter},
		{Type: model.PrincipalUser, ID: "carol", Role: model.RoleReader},
		{Type: model.PrincipalUser, ID: "reviewer-01", Role: model.RoleReader},
	})
	require.NoError(t, err)
	// Identities are cached; resolve them after the memberships exist.
	p.bob, p.carol, p.viewer = e.identity("bob"), e.identity("carol"), e.identity("viewer")
	p.named = e.identity("reviewer-01")
	return p
}

func (p *pageEnv) create(t *testing.T, actor *acl.Identity, parent *string, title string) *PageView {
	t.Helper()
	v, err := p.svc.Pages.Create(ctx(), actor, CreatePageInput{SpaceID: p.space.ID, ParentID: parent, Title: title})
	require.NoError(t, err)
	return v
}

func (p *pageEnv) decision(t *testing.T, actor *acl.Identity, pageID string) acl.Decision {
	t.Helper()
	d, err := p.resolver.Page(ctx(), actor, pageID)
	require.NoError(t, err)
	return d
}

func (p *pageEnv) childTitles(t *testing.T, actor *acl.Identity, parent *string) []string {
	t.Helper()
	role := p.mustSpaceRole(actor, p.space)
	page, err := p.svc.Pages.Children(ctx(), actor, p.space, role, parent, "", 0)
	require.NoError(t, err)
	out := make([]string, len(page.Items))
	for i, n := range page.Items {
		out[i] = n.Title
	}
	return out
}

// restrict cuts inheritance on a page and grants exactly the given users.
func (p *pageEnv) restrict(t *testing.T, pageID string, users ...string) {
	t.Helper()
	require.NoError(t, p.repos.Access.SetRestricted(ctx(), 1, p.space.ID, pageID, "alice"))
	for _, u := range users {
		require.NoError(t, p.repos.Access.UpsertGrant(ctx(), &model.PageGrant{
			PageID: pageID, TenantID: 1, PrincipalType: model.PrincipalUser, PrincipalID: u, Role: model.RoleWriter,
		}))
	}
	p.resolver.Invalidate(ctx(), 1)
}

func (p *pageEnv) hasEvent(typ events.Type, check func(events.Event) bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ev := range p.events {
		if ev.Type == typ && (check == nil || check(ev)) {
			return true
		}
	}
	return false
}

func TestPageCreateRulesAndOrdering(t *testing.T) {
	p := newPageEnv(t)

	a := p.create(t, p.alice, nil, "A")
	b := p.create(t, p.bob, nil, "B")
	c := p.create(t, p.bob, nil, "C")
	require.Less(t, a.Position, b.Position)
	require.Less(t, b.Position, c.Position)
	require.Len(t, a.ShortID, ShortIDLen)
	require.Equal(t, model.RoleAdmin, a.Role)
	require.True(t, a.CanEdit)
	require.Equal(t, []string{"A", "B", "C"}, p.childTitles(t, p.carol, nil))

	child := p.create(t, p.bob, &a.ID, "A.1")
	require.Equal(t, a.ID, *child.ParentID)
	got, err := p.svc.Pages.Get(ctx(), p.decision(t, p.carol, a.ID))
	require.NoError(t, err)
	require.True(t, got.HasChildren)
	require.False(t, got.CanEdit, "readers cannot edit")

	// Readers cannot create; strangers do not learn the space exists.
	_, err = p.svc.Pages.Create(ctx(), p.carol, CreatePageInput{SpaceID: p.space.ID, Title: "x"})
	require.Equal(t, 403, httpCode(t, err))
	_, err = p.svc.Pages.Create(ctx(), p.carol, CreatePageInput{SpaceID: p.space.ID, ParentID: &a.ID, Title: "x"})
	require.Equal(t, 403, httpCode(t, err))
	_, err = p.svc.Pages.Create(ctx(), p.viewer, CreatePageInput{SpaceID: p.space.ID, Title: "x"})
	require.Equal(t, 404, httpCode(t, err))

	// Validation.
	_, err = p.svc.Pages.Create(ctx(), p.bob, CreatePageInput{SpaceID: p.space.ID, Title: strings.Repeat("x", 501)})
	require.Equal(t, 400, httpCode(t, err))
	_, err = p.svc.Pages.Create(ctx(), p.bob, CreatePageInput{
		SpaceID: p.space.ID, Content: json.RawMessage(`{"type":"doc","content":[{"type":"nope"}]}`),
	})
	require.Equal(t, 400, httpCode(t, err))
	_, err = p.svc.Pages.Create(ctx(), p.bob, CreatePageInput{
		SpaceID: p.space.ID, Content: json.RawMessage(`{"type":"doc"}`), Markdown: "# x",
	})
	require.Equal(t, 400, httpCode(t, err))
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Other"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{SpaceID: other.ID, ParentID: &a.ID, Title: "x"})
	require.Equal(t, 400, httpCode(t, err), "parent must belong to the space")

	// Initial content: Markdown is converted and indexed.
	md, err := p.svc.Pages.Create(ctx(), p.bob, CreatePageInput{
		SpaceID: p.space.ID, Title: "Guide", Markdown: "# Intro\n\nHello **world** and more words here.",
	})
	require.NoError(t, err)
	full, err := p.repos.Pages.Get(ctx(), 1, md.ID)
	require.NoError(t, err)
	require.Contains(t, full.TextContent, "Hello world")
	require.Greater(t, full.WordCount, 3)
	content, err := p.svc.Pages.Content(ctx(), p.decision(t, p.carol, md.ID), true)
	require.NoError(t, err)
	require.Contains(t, content.HTML, "<h1")
	require.Contains(t, content.HTML, "<strong>world</strong>")

	require.True(t, p.audit.has(audit.PageCreated))
	require.True(t, p.hasEvent(events.PageCreated, func(ev events.Event) bool {
		parent, _ := ev.Payload["parent_id"].(*string)
		return ev.PageID == child.ID && parent != nil && *parent == a.ID
	}))
}

// Acceptance (T1.2): concurrent inserts under one parent never collide on
// position and the resulting order is total.
func TestPageConcurrentCreatesKeepDistinctPositions(t *testing.T) {
	p := newPageEnv(t)
	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := p.svc.Pages.Create(ctx(), p.bob, CreatePageInput{SpaceID: p.space.ID, Title: fmt.Sprint(i)})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	roots, err := p.repos.Pages.ListChildren(ctx(), 1, p.space.ID, nil)
	require.NoError(t, err)
	require.Len(t, roots, n)
	seen := map[string]bool{}
	positions := make([]string, 0, n)
	for _, r := range roots {
		require.False(t, seen[r.Position], "duplicate position %q", r.Position)
		seen[r.Position] = true
		positions = append(positions, r.Position)
	}
	require.True(t, sort.StringsAreSorted(positions))
}

func TestPageMoveWithinSpace(t *testing.T) {
	p := newPageEnv(t)
	a := p.create(t, p.alice, nil, "A")
	b := p.create(t, p.alice, nil, "B")
	c := p.create(t, p.alice, nil, "C")

	move := func(actor *acl.Identity, id string, in MovePageInput) (*MoveResult, error) {
		return p.svc.Pages.Move(ctx(), actor, p.decision(t, actor, id), in)
	}

	// C first, then B right after C, then A becomes a child of B.
	res, err := move(p.bob, c.ID, MovePageInput{Placement: Placement{Mode: PlaceFirst}})
	require.NoError(t, err)
	require.False(t, res.Rebalanced)
	require.Equal(t, []string{"C", "A", "B"}, p.childTitles(t, p.bob, nil))
	_, err = move(p.bob, b.ID, MovePageInput{Placement: Placement{Mode: PlaceAfter, AfterID: c.ID}})
	require.NoError(t, err)
	require.Equal(t, []string{"C", "B", "A"}, p.childTitles(t, p.bob, nil))
	_, err = move(p.bob, a.ID, MovePageInput{ParentID: &b.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"C", "B"}, p.childTitles(t, p.bob, nil))
	require.Equal(t, []string{"A"}, p.childTitles(t, p.bob, &b.ID))
	anc, err := p.svc.Pages.Ancestors(ctx(), p.decision(t, p.carol, a.ID))
	require.NoError(t, err)
	require.Len(t, anc, 1)
	require.Equal(t, "B", anc[0].Title)

	// Moving a page after itself, under itself, or under its descendant fails.
	_, err = move(p.bob, b.ID, MovePageInput{Placement: Placement{Mode: PlaceAfter, AfterID: b.ID}})
	require.Equal(t, 400, httpCode(t, err))
	_, err = move(p.bob, b.ID, MovePageInput{ParentID: &b.ID})
	require.Equal(t, 400, httpCode(t, err))
	_, err = move(p.bob, b.ID, MovePageInput{ParentID: &a.ID})
	require.Equal(t, 400, httpCode(t, err))
	// after_id must be a sibling at the target location.
	_, err = move(p.bob, c.ID, MovePageInput{Placement: Placement{Mode: PlaceAfter, AfterID: a.ID}})
	require.Equal(t, 400, httpCode(t, err))
	// Explicit positions are validated.
	_, err = move(p.bob, c.ID, MovePageInput{Placement: Placement{Mode: PlaceAt, Position: "not a key"}})
	require.Equal(t, 400, httpCode(t, err))
	_, err = move(p.bob, c.ID, MovePageInput{Placement: Placement{Mode: PlaceAt, Position: "b00"}})
	require.NoError(t, err)
	require.Equal(t, []string{"B", "C"}, p.childTitles(t, p.bob, nil))
	// Readers cannot move.
	_, err = move(p.carol, c.ID, MovePageInput{Placement: Placement{Mode: PlaceFirst}})
	require.Equal(t, 403, httpCode(t, err))

	require.True(t, p.hasEvent(events.PageMoved, func(ev events.Event) bool {
		return ev.PageID == a.ID && ev.Payload["cross_space"] == false
	}))
	require.True(t, p.audit.has(audit.PageMoved))
}

func TestPageMoveRebalancesLongKeys(t *testing.T) {
	p := newPageEnv(t)
	parent := p.create(t, p.alice, nil, "P")
	var kids []*PageView
	for i := 0; i < 4; i++ {
		kids = append(kids, p.create(t, p.alice, &parent.ID, fmt.Sprint("K", i)))
	}
	// Simulate a sibling list whose keys have grown past the threshold.
	long := map[string]string{}
	for i, k := range kids {
		long[k.ID] = "a0" + strings.Repeat("V", RebalanceKeyLen-2) + string(rune('1'+i))
	}
	require.NoError(t, p.repos.Pages.SetPositions(ctx(), 1, long))

	res, err := p.svc.Pages.Move(ctx(), p.bob, p.decision(t, p.bob, kids[3].ID),
		MovePageInput{ParentID: &parent.ID, Placement: Placement{Mode: PlaceAfter, AfterID: kids[0].ID}})
	require.NoError(t, err)
	require.True(t, res.Rebalanced)
	require.Equal(t, []string{"K0", "K3", "K1", "K2"}, p.childTitles(t, p.bob, &parent.ID))
	rows, err := p.repos.Pages.ListChildren(ctx(), 1, p.space.ID, &parent.ID)
	require.NoError(t, err)
	for _, r := range rows {
		require.LessOrEqual(t, len(r.Position), 3, "rebalanced keys are short: %q", r.Position)
	}
	require.True(t, p.hasEvent(events.TreeRebalanced, nil))
}

// Acceptance (T1.2): a cross-space move carries the subtree, leaves behind
// what the mover cannot see, updates dependent rows and announces itself so
// the retrieval ACL snapshots can be recomputed.
func TestPageCrossSpaceMoveOrphansInvisiblePages(t *testing.T) {
	p := newPageEnv(t)
	target, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Target"})
	require.NoError(t, err)
	_, err = p.svc.Spaces.SetMembers(ctx(), p.alice, target.Space, []MemberInput{
		{Type: model.PrincipalUser, ID: "bob", Role: model.RoleWriter},
		{Type: model.PrincipalUser, ID: "carol", Role: model.RoleReader},
	})
	require.NoError(t, err)
	hidden, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Hidden"})
	require.NoError(t, err)

	root := p.create(t, p.alice, nil, "Root")
	secret := p.create(t, p.alice, &root.ID, "Secret")
	deeper := p.create(t, p.alice, &secret.ID, "Deeper")
	public := p.create(t, p.alice, &root.ID, "Public")
	p.restrict(t, secret.ID, "alice")
	bob := p.identity("bob")

	// Bob cannot move into a space where he only reads, nor one he cannot see.
	_, err = p.svc.Pages.Move(ctx(), bob, p.decision(t, bob, root.ID), MovePageInput{SpaceID: hidden.ID})
	require.Equal(t, 404, httpCode(t, err))
	carol := p.identity("carol")
	_, err = p.svc.Pages.Move(ctx(), p.identity("alice"), p.decision(t, p.identity("alice"), root.ID),
		MovePageInput{SpaceID: target.ID, ParentID: &public.ID})
	require.Equal(t, 400, httpCode(t, err), "parent must be in the target space")
	_ = carol

	res, err := p.svc.Pages.Move(ctx(), bob, p.decision(t, bob, root.ID), MovePageInput{SpaceID: target.ID})
	require.NoError(t, err)
	require.Equal(t, []string{secret.ID}, res.Orphaned)
	require.Equal(t, target.ID, res.Page.SpaceID)

	for id, want := range map[string]string{
		root.ID: target.ID, public.ID: target.ID, secret.ID: p.space.ID,
		deeper.ID: p.space.ID,
	} {
		got, err := p.repos.Pages.Get(ctx(), 1, id)
		require.NoError(t, err)
		require.Equal(t, want, got.SpaceID, "space of %s", got.Title)
	}
	secretNow, err := p.repos.Pages.Get(ctx(), 1, secret.ID)
	require.NoError(t, err)
	require.Nil(t, secretNow.ParentID, "the orphan sits at the source space root")
	deeperNow, err := p.repos.Pages.Get(ctx(), 1, deeper.ID)
	require.NoError(t, err)
	require.Equal(t, secret.ID, *deeperNow.ParentID, "its subtree travels with it")
	require.Equal(t, []string{"Secret"}, p.childTitles(t, p.identity("alice"), nil))

	// The ACL snapshot event carries what the bridge needs.
	require.True(t, p.hasEvent(events.PageMoved, func(ev events.Event) bool {
		return ev.PageID == root.ID && ev.SpaceID == target.ID && ev.Payload["cross_space"] == true &&
			ev.Payload["from_space_id"] == p.space.ID
	}))
	// Permission cache reflects the new space: carol reads Public in Target.
	d, err := p.resolver.Page(ctx(), p.identity("carol"), public.ID)
	require.NoError(t, err)
	require.Equal(t, model.RoleReader, d.Role)
	subjects, err := p.resolver.PageSubjects(ctx(), 1, public.ID)
	require.NoError(t, err)
	require.Contains(t, subjects, acl.SubjectSpace+target.ID)
}

func TestPageTrashLifecycle(t *testing.T) {
	p := newPageEnv(t)
	root := p.create(t, p.alice, nil, "Root")
	child := p.create(t, p.alice, &root.ID, "Child")
	grand := p.create(t, p.alice, &child.ID, "Grand")
	sibling := p.create(t, p.alice, nil, "Sibling")
	secret := p.create(t, p.alice, nil, "Secret")
	p.restrict(t, secret.ID, "alice")
	alice, bob, carol := p.identity("alice"), p.identity("bob"), p.identity("carol")

	// Readers cannot delete; writers can. The whole subtree goes.
	_, err := p.svc.Pages.Delete(ctx(), carol, p.decision(t, carol, root.ID))
	require.Equal(t, 403, httpCode(t, err))
	n, err := p.svc.Pages.Delete(ctx(), bob, p.decision(t, bob, root.ID))
	require.NoError(t, err)
	require.Equal(t, int64(3), n)
	_, err = p.resolver.Page(ctx(), bob, grand.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
	require.Equal(t, []string{"Sibling", "Secret"}, p.childTitles(t, alice, nil))
	require.Equal(t, []string{"Sibling"}, p.childTitles(t, bob, nil), "restricted page hidden from bob")

	// A trashed restricted page is invisible to those without a grant.
	_, err = p.svc.Pages.Delete(ctx(), alice, p.decision(t, alice, secret.ID))
	require.NoError(t, err)
	trash, err := p.svc.Pages.Trash(ctx(), bob, p.space)
	require.NoError(t, err)
	require.Len(t, trash, 1)
	require.Equal(t, "Root", trash[0].Title)
	require.True(t, trash[0].CanRestore)
	require.NotNil(t, trash[0].DeletedByUser)
	require.Equal(t, "bob", trash[0].DeletedByUser.Username)
	trash, err = p.svc.Pages.Trash(ctx(), alice, p.space)
	require.NoError(t, err)
	require.Len(t, trash, 2)
	trash, err = p.svc.Pages.Trash(ctx(), carol, p.space)
	require.NoError(t, err)
	require.Len(t, trash, 1)
	require.False(t, trash[0].CanRestore)

	// The 410 helper: a trashed page still resolves for those who could read it.
	trashed, err := p.repos.Pages.GetAny(ctx(), 1, root.ID)
	require.NoError(t, err)
	d, err := p.resolver.Decide(ctx(), carol, trashed)
	require.NoError(t, err)
	require.Equal(t, model.RoleReader, d.Role)

	// Restore rules.
	_, err = p.svc.Pages.Restore(ctx(), carol, root.ID)
	require.Equal(t, 403, httpCode(t, err))
	_, err = p.svc.Pages.Restore(ctx(), bob, sibling.ID)
	require.Equal(t, 409, httpCode(t, err), "live pages are not in the trash")
	_, err = p.svc.Pages.Restore(ctx(), bob, secret.ID)
	require.Equal(t, 404, httpCode(t, err), "invisible trashed pages stay invisible")
	_, err = p.svc.Pages.Restore(ctx(), bob, "missing")
	require.Equal(t, 404, httpCode(t, err))

	// Restoring the child alone re-attaches it at the root with a fresh slot.
	view, err := p.svc.Pages.Restore(ctx(), bob, child.ID)
	require.NoError(t, err)
	require.Nil(t, view.ParentID)
	require.Equal(t, []string{"Sibling", "Child"}, p.childTitles(t, bob, nil))
	require.True(t, view.HasChildren, "Grand came back with Child")
	require.True(t, p.hasEvent(events.PageRestored, func(ev events.Event) bool {
		return ev.PageID == child.ID && ev.Payload["reattached"] == true
	}))

	// Purge needs the trash entry to belong to the space and to be trashed.
	_, err = p.svc.Pages.Purge(ctx(), alice, p.space, sibling.ID)
	require.Equal(t, 409, httpCode(t, err))
	ids, err := p.svc.Pages.Purge(ctx(), alice, p.space, root.ID)
	require.NoError(t, err)
	require.Equal(t, []string{root.ID}, ids, "child and grand were restored, only root remains under it")
	count, err := p.svc.Pages.EmptyTrash(ctx(), alice, p.space)
	require.NoError(t, err)
	require.Equal(t, 1, count, "secret")
	trash, err = p.svc.Pages.Trash(ctx(), alice, p.space)
	require.NoError(t, err)
	require.Empty(t, trash)
	require.True(t, p.audit.has(audit.PageDeleted))
	require.True(t, p.audit.has(audit.PageRestored))
	require.True(t, p.audit.has(audit.PagePurged))
}

func TestPageDuplicateCopiesVisibleSubtree(t *testing.T) {
	p := newPageEnv(t)
	outside := p.create(t, p.alice, nil, "Outside")
	root := p.create(t, p.alice, nil, "Spec")
	secret := p.create(t, p.alice, &root.ID, "Secret")
	p.restrict(t, secret.ID, "alice")
	linkDoc := fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","content":[`+
		`{"type":"pageLink","attrs":{"pageId":%q}},{"type":"text","text":" and "},`+
		`{"type":"pageLink","attrs":{"pageId":%q}}]}]}`, root.ID, outside.ID)
	links, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: p.space.ID, ParentID: &root.ID, Title: "Links", Content: json.RawMessage(linkDoc),
	})
	require.NoError(t, err)
	after := p.create(t, p.alice, nil, "After")
	bob, carol := p.identity("bob"), p.identity("carol")

	_, err = p.svc.Pages.Duplicate(ctx(), carol, p.decision(t, carol, root.ID), DuplicatePageInput{})
	require.Equal(t, 403, httpCode(t, err), "readers cannot duplicate")

	res, err := p.svc.Pages.Duplicate(ctx(), bob, p.decision(t, bob, root.ID), DuplicatePageInput{})
	require.NoError(t, err)
	require.Equal(t, "Spec (copy)", res.Page.Title)
	require.Equal(t, 2, res.Count, "the restricted child is not copied")
	require.NotEqual(t, root.ShortID, res.Page.ShortID)
	require.Equal(t, []string{"Outside", "Spec", "Spec (copy)", "After"}, p.childTitles(t, bob, nil))
	_ = after

	kids, err := p.repos.Pages.ListChildren(ctx(), 1, p.space.ID, &res.Page.ID)
	require.NoError(t, err)
	require.Len(t, kids, 1)
	copied, err := p.repos.Pages.Get(ctx(), 1, kids[0].ID)
	require.NoError(t, err)
	require.Equal(t, "Links", copied.Title)
	require.Contains(t, string(copied.Content), res.Page.ID, "links to copied pages point at the copies")
	require.NotContains(t, string(copied.Content), root.ID)
	require.Contains(t, string(copied.Content), outside.ID, "links to other pages are kept")
	require.Equal(t, model.StringList{"bob"}, copied.ContributorIDs)
	require.Equal(t, "bob", *copied.CreatorID)
	require.Nil(t, copied.YDoc)
	require.Equal(t, links.WordCount, copied.WordCount)

	// Copy into another space under a parent with an explicit title.
	target, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Target"})
	require.NoError(t, err)
	tp, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{SpaceID: target.ID, Title: "Home"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Duplicate(ctx(), bob, p.decision(t, bob, root.ID), DuplicatePageInput{SpaceID: target.ID})
	require.Equal(t, 404, httpCode(t, err), "bob cannot see the target space")
	alice := p.identity("alice")
	res2, err := p.svc.Pages.Duplicate(ctx(), alice, p.decision(t, alice, root.ID),
		DuplicatePageInput{SpaceID: target.ID, ParentID: &tp.ID, Title: "Imported spec"})
	require.NoError(t, err)
	require.Equal(t, "Imported spec", res2.Page.Title)
	require.Equal(t, target.ID, res2.Page.SpaceID)
	require.Equal(t, tp.ID, *res2.Page.ParentID)
	require.Equal(t, 3, res2.Count, "alice sees the restricted child too")
	require.True(t, p.audit.has(audit.PageDuplicated))
}

func TestPageChildrenListingRespectsAccessAndPaging(t *testing.T) {
	p := newPageEnv(t)
	var roots []*PageView
	for i := 0; i < 5; i++ {
		roots = append(roots, p.create(t, p.alice, nil, fmt.Sprint("R", i)))
	}
	p.create(t, p.alice, &roots[1].ID, "R1.1")
	p.restrict(t, roots[3].ID, "alice")
	require.NoError(t, p.repos.Pages.UpdateMeta(ctx(), 1, roots[4].ID, map[string]any{"is_locked": true}))
	bob, carol, alice := p.identity("bob"), p.identity("carol"), p.identity("alice")

	require.Equal(t, []string{"R0", "R1", "R2", "R4"}, p.childTitles(t, bob, nil))
	require.Equal(t, []string{"R0", "R1", "R2", "R3", "R4"}, p.childTitles(t, alice, nil))

	// Paging with a cursor, and per-row flags.
	page, err := p.svc.Pages.Children(ctx(), bob, p.space, model.RoleWriter, nil, "", 2)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotEmpty(t, page.NextCursor)
	require.False(t, page.Items[0].HasChildren)
	require.True(t, page.Items[1].HasChildren)
	require.True(t, page.Items[1].CanEdit)
	rest, err := p.svc.Pages.Children(ctx(), bob, p.space, model.RoleWriter, nil, page.NextCursor, 10)
	require.NoError(t, err)
	require.Len(t, rest.Items, 2)
	require.Empty(t, rest.NextCursor)
	require.Equal(t, "R4", rest.Items[1].Title)
	require.False(t, rest.Items[1].CanEdit, "locked pages are read-only for writers")
	adminView, err := p.svc.Pages.Children(ctx(), alice, p.space, model.RoleAdmin, nil, "", 0)
	require.NoError(t, err)
	require.True(t, adminView.Items[4].CanEdit, "admins may edit locked pages")
	require.True(t, adminView.Items[3].Restricted)

	_, err = p.svc.Pages.Children(ctx(), carol, p.space, model.RoleReader, nil, "garbage", 0)
	require.Equal(t, 400, httpCode(t, err))
	_, err = p.svc.Pages.Children(ctx(), bob, p.space, model.RoleWriter, &roots[3].ID, "", 0)
	require.Equal(t, 404, httpCode(t, err), "children of an invisible parent are not listed")
}

func TestPageUpdateMetadata(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Draft")
	bob, carol := p.identity("bob"), p.identity("carol")

	v, err := p.svc.Pages.Update(ctx(), bob, p.decision(t, bob, page.ID),
		UpdatePageInput{Title: strp("  Final  "), Icon: strp("📘")})
	require.NoError(t, err)
	require.Equal(t, "Final", v.Title)
	require.Equal(t, "📘", *v.Icon)
	v, err = p.svc.Pages.Update(ctx(), bob, p.decision(t, bob, page.ID), UpdatePageInput{Icon: strp("")})
	require.NoError(t, err)
	require.Nil(t, v.Icon)
	_, err = p.svc.Pages.Update(ctx(), carol, p.decision(t, carol, page.ID), UpdatePageInput{Title: strp("x")})
	require.Equal(t, 403, httpCode(t, err))
	_, err = p.svc.Pages.Update(ctx(), bob, p.decision(t, bob, page.ID), UpdatePageInput{Title: strp("a\nb")})
	require.Equal(t, 400, httpCode(t, err))
	require.True(t, p.hasEvent(events.PageMeta, func(ev events.Event) bool {
		return ev.PageID == page.ID && ev.Payload["title"] == "Final"
	}))
}
