package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// fakeStewardRepo keeps stewardship in memory, with the rules of the real one
// that the service relies on: reviews only move forward, maintainers are a set.
type fakeStewardRepo struct {
	stewards    map[string]*types.KnowledgeSteward
	maintainers map[string]bool
	// members can be asked but cannot edit the knowledge base.
	members map[string]bool
}

func (r *fakeStewardRepo) Stewards(_ context.Context, _ uint64, ids []string,
) (map[string]*types.KnowledgeSteward, error) {
	out := map[string]*types.KnowledgeSteward{}
	for _, id := range ids {
		if st, ok := r.stewards[id]; ok {
			copied := *st
			out[id] = &copied
		}
	}
	return out, nil
}

func (r *fakeStewardRepo) SetOwner(_ context.Context, _ uint64, id, owner string) error {
	st, ok := r.stewards[id]
	if !ok {
		return interfaces.ErrKnowledgeNotFound
	}
	st.OwnerID, st.OwnerActive = owner, owner != ""
	return nil
}

func (r *fakeStewardRepo) MarkReviewed(_ context.Context, _ uint64, id, user string, at time.Time) error {
	st := r.stewards[id]
	if st.ReviewedAt == nil || !st.ReviewedAt.After(at) {
		st.ReviewedAt, st.ReviewedBy, st.ReviewerActive = &at, user, true
	}
	return nil
}

func (r *fakeStewardRepo) CanMaintain(_ context.Context, _ uint64, _, user string) (bool, error) {
	return r.maintainers[user], nil
}

func (r *fakeStewardRepo) IsActiveMember(_ context.Context, _ uint64, user string) (bool, error) {
	return r.maintainers[user] || r.members[user], nil
}

type fakeUserRepo struct {
	interfaces.UserRepository
	users map[string]*types.User
}

func (r *fakeUserRepo) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	out := map[string]*types.User{}
	for _, id := range ids {
		if u, ok := r.users[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

type stewardFixture struct {
	svc     interfaces.KnowledgeStewardshipService
	repo    *fakeStewardRepo
	trigger *recordingFindingsTrigger
	audit   *capturedAudit
}

func newStewardshipFixture(origin types.KnowledgeOrigin) *stewardFixture {
	repo := &fakeStewardRepo{
		stewards: map[string]*types.KnowledgeSteward{"k1": {
			KnowledgeID: "k1", KnowledgeBaseID: "kb1", Title: "Leave policy", Origin: origin,
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), OwnerID: "alice", OwnerActive: true,
			ReviewIntervalDays: 90,
		}},
		maintainers: map[string]bool{"alice": true, "bob": true},
	}
	trigger := &recordingFindingsTrigger{}
	audit := &capturedAudit{}
	users := &fakeUserRepo{users: map[string]*types.User{
		"alice": {ID: "alice", Username: "Alice"}, "bob": {ID: "bob", Username: "Bob"},
	}}
	return &stewardFixture{
		svc: NewKnowledgeStewardshipService(repo, users, trigger, audit), repo: repo, trigger: trigger, audit: audit,
	}
}

// A transfer is recorded, and re-checks the entry so its open problems follow
// the new owner.
func TestStewardshipTransferReroutes(t *testing.T) {
	f := newStewardshipFixture(types.KnowledgeOriginLocal)
	view, err := f.svc.SetOwner(asUser("alice"), 1, "k1", "bob")
	require.NoError(t, err)
	require.NotNil(t, view.Owner)
	assert.Equal(t, "Bob", view.Owner.Username)
	assert.True(t, view.OwnerEditable)
	assert.Len(t, f.trigger.calls, 1)
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, types.AuditActionKnowledgeOwnerChanged, f.audit.rows[0].Action)

	// The same owner again changes nothing and schedules nothing.
	_, err = f.svc.SetOwner(asUser("alice"), 1, "k1", "bob")
	require.NoError(t, err)
	assert.Len(t, f.trigger.calls, 1)
}

func TestStewardshipTransferIsRefused(t *testing.T) {
	f := newStewardshipFixture(types.KnowledgeOriginLocal)
	_, err := f.svc.SetOwner(asUser("alice"), 1, "k1", "mallory")
	assert.Equal(t, 400, httpCodeOf(t, err), "somebody who cannot edit the base cannot own its entries")

	_, err = f.svc.SetOwner(asUser("alice"), 1, "missing", "bob")
	assert.Equal(t, 404, httpCodeOf(t, err))

	mirror := newStewardshipFixture(types.KnowledgeOriginDocs)
	_, err = mirror.svc.SetOwner(asUser("alice"), 1, "k1", "bob")
	assert.Equal(t, 409, httpCodeOf(t, err), "a docs mirror's owner is its page's")
	view, err := mirror.svc.Get(asUser("alice"), 1, "k1")
	require.NoError(t, err)
	assert.False(t, view.OwnerEditable)
	assert.Empty(t, mirror.trigger.calls)
}

// A confirmation is a person's word: it restarts the review clock and
// re-checks the entry; an API key cannot give it.
func TestStewardshipConfirmReviewed(t *testing.T) {
	f := newStewardshipFixture(types.KnowledgeOriginLocal)
	before, err := f.svc.Get(asUser("alice"), 1, "k1")
	require.NoError(t, err)
	require.NotNil(t, before.ReviewDueAt)

	view, err := f.svc.ConfirmReviewed(asUser("bob"), 1, "k1")
	require.NoError(t, err)
	require.NotNil(t, view.ReviewedBy)
	assert.Equal(t, "bob", view.ReviewedBy.ID)
	assert.True(t, view.ReviewDueAt.After(*before.ReviewDueAt))
	assert.False(t, view.Overdue)
	assert.Len(t, f.trigger.calls, 1)

	_, err = f.svc.ConfirmReviewed(asUser("system-1"), 1, "k1")
	assert.Equal(t, 403, httpCodeOf(t, err))
	_, err = f.svc.ConfirmReviewed(context.Background(), 1, "k1")
	assert.Equal(t, 403, httpCodeOf(t, err))
}

// A page's stewardship is copied as it is — no membership check, since the
// page decided — and only a change re-checks the entry.
func TestStewardshipSyncFromSource(t *testing.T) {
	f := newStewardshipFixture(types.KnowledgeOriginDocs)
	edited := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, f.svc.SyncFromSource(context.Background(), 1, "k1", "carol", "dave", edited))
	st := f.repo.stewards["k1"]
	assert.Equal(t, "carol", st.OwnerID)
	assert.Equal(t, "dave", st.ReviewedBy)
	assert.Len(t, f.trigger.calls, 1)

	require.NoError(t, f.svc.SyncFromSource(context.Background(), 1, "k1", "carol", "dave", edited))
	assert.Len(t, f.trigger.calls, 1, "nothing changed, nothing to re-check")
	assert.Empty(t, f.audit.rows, "the page recorded the decision where it was made")
}
