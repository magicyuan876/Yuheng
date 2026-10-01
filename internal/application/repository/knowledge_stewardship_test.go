package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

const stewardTenant uint64 = 7

type stewardFixture struct {
	db   *gorm.DB
	repo interfaces.KnowledgeStewardshipRepository
	kb   string
}

func newStewardFixture(t *testing.T, kbCreator string) *stewardFixture {
	t.Helper()
	db := pgtest.New(t)
	f := &stewardFixture{db: db, repo: NewKnowledgeStewardshipRepository(db), kb: uuid.NewString()}
	require.NoError(t, db.Exec(`
		INSERT INTO knowledge_bases (id, name, tenant_id, embedding_model_id, summary_model_id, creator_id,
		                             review_interval_days, storage_backend_id)
		VALUES (?, 'Handbook', ?, '', '', ?, 30, 'env')`, f.kb, stewardTenant, kbCreator).Error)
	return f
}

// member adds a user with a membership of the fixture's tenant.
func (f *stewardFixture) member(t *testing.T, role types.TenantRole, status types.TenantMemberStatus,
	accountActive bool,
) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, f.db.Exec(`INSERT INTO users (id, username, email, password_hash, is_active)
		VALUES (?, ?, ?, 'x', ?)`, id, "u-"+id[:8], id[:8]+"@example.com", accountActive).Error)
	require.NoError(t, f.db.Exec(`INSERT INTO tenant_members (user_id, tenant_id, role, status)
		VALUES (?, ?, ?, ?)`, id, stewardTenant, role, status).Error)
	return id
}

func (f *stewardFixture) entry(t *testing.T, channel string, metadata string, owner string) string {
	t.Helper()
	id := uuid.NewString()
	var ownerArg any
	if owner != "" {
		ownerArg = owner
	}
	var metaArg any
	if metadata != "" {
		metaArg = metadata
	}
	require.NoError(t, f.db.Exec(`
		INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source, parse_status, channel,
		                        metadata, owner_id)
		VALUES (?, ?, ?, 'manual', 'Leave policy', 'manual', 'completed', ?, ?::jsonb, ?)`,
		id, stewardTenant, f.kb, channel, metaArg, ownerArg).Error)
	return id
}

func (f *stewardFixture) steward(t *testing.T, id string) *types.KnowledgeSteward {
	t.Helper()
	got, err := f.repo.Stewards(context.Background(), stewardTenant, []string{id})
	require.NoError(t, err)
	require.Contains(t, got, id)
	return got[id]
}

// The origin computed in SQL is the one Knowledge.Origin computes on the same
// rows; the review sweep and the API read the one, the services the other.
func TestKnowledgeOriginSQLAgreesWithGo(t *testing.T) {
	f := newStewardFixture(t, "")
	cases := map[string]struct {
		channel, metadata string
		want              types.KnowledgeOrigin
	}{
		"upload":      {types.ChannelWeb, "", types.KnowledgeOriginLocal},
		"docs mirror": {types.ChannelDocs, "", types.KnowledgeOriginDocs},
		"synced":      {types.ChannelNotion, `{"datasource_id":"ds1","external_id":"e1"}`, types.KnowledgeOriginSynced},
		"null source": {types.ChannelAPI, `{"datasource_id":null}`, types.KnowledgeOriginLocal},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			id := f.entry(t, c.channel, c.metadata, "")
			assert.Equal(t, c.want, f.steward(t, id).Origin, "SQL")
			var k types.Knowledge
			require.NoError(t, f.db.Where("id = ?", id).Take(&k).Error)
			assert.Equal(t, c.want, k.Origin(), "Go")
		})
	}
}

// Somebody who left, was suspended or had their account disabled is named but
// cannot be asked; the routing falls past them to the next person.
func TestStewardsMarkPeopleWhoCanNoLongerBeAsked(t *testing.T) {
	creator := uuid.NewString()
	f := newStewardFixture(t, creator)
	require.NoError(t, f.db.Exec(`INSERT INTO users (id, username, email, password_hash)
		VALUES (?, 'creator', 'creator@example.com', 'x')`, creator).Error)
	require.NoError(t, f.db.Exec(`INSERT INTO tenant_members (user_id, tenant_id, role) VALUES (?, ?, 'admin')`,
		creator, stewardTenant).Error)

	suspended := f.member(t, types.TenantRoleContributor, types.TenantMemberStatusSuspended, true)
	disabled := f.member(t, types.TenantRoleContributor, types.TenantMemberStatusActive, false)
	reviewer := f.member(t, types.TenantRoleContributor, types.TenantMemberStatusActive, true)

	for name, owner := range map[string]string{"suspended": suspended, "disabled": disabled} {
		t.Run(name, func(t *testing.T) {
			id := f.entry(t, types.ChannelWeb, "", owner)
			st := f.steward(t, id)
			assert.Equal(t, owner, st.OwnerID)
			assert.False(t, st.OwnerActive)
			assert.Equal(t, creator, st.Responsible(), "falls back to the knowledge base's creator")

			require.NoError(t, f.repo.MarkReviewed(context.Background(), stewardTenant, id, reviewer, time.Now()))
			st = f.steward(t, id)
			assert.True(t, st.ReviewerActive)
			assert.Equal(t, reviewer, st.Responsible(), "the last reviewer comes before the creator")
			assert.Equal(t, reviewer, st.LatestHand())
			assert.Equal(t, 30, st.ReviewIntervalDays)
		})
	}
}

// A review never turns the clock back: a late write of an older review, from a
// page synchronised out of order, leaves the newer one.
func TestMarkReviewedOnlyMovesForward(t *testing.T) {
	f := newStewardFixture(t, "")
	a := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	b := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	id := f.entry(t, types.ChannelWeb, "", "")
	now := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, f.repo.MarkReviewed(context.Background(), stewardTenant, id, a, now))
	require.NoError(t, f.repo.MarkReviewed(context.Background(), stewardTenant, id, b, now.Add(-time.Hour)))
	st := f.steward(t, id)
	assert.Equal(t, a, st.ReviewedBy)
	require.NotNil(t, st.ReviewedAt)
	assert.True(t, st.ReviewedAt.Equal(now))
}

// Ownership survives a full-row update made from a copy loaded before the
// transfer — the way an ingestion step saves the row it read minutes ago.
func TestFullRowUpdateLeavesStewardshipAlone(t *testing.T) {
	f := newStewardFixture(t, "")
	before := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	after := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	id := f.entry(t, types.ChannelWeb, "", before)

	var stale types.Knowledge
	require.NoError(t, f.db.Where("id = ?", id).Take(&stale).Error)
	require.NoError(t, f.repo.SetOwner(context.Background(), stewardTenant, id, after))
	require.NoError(t, f.repo.MarkReviewed(context.Background(), stewardTenant, id, after, time.Now()))

	stale.Title = "Leave policy (2026)"
	require.NoError(t, NewKnowledgeRepository(f.db).UpdateKnowledge(context.Background(), &stale))

	st := f.steward(t, id)
	assert.Equal(t, "Leave policy (2026)", st.Title)
	assert.Equal(t, after, st.OwnerID, "the stale copy must not write the old owner back")
	assert.Equal(t, after, st.ReviewedBy)

	require.NoError(t, f.repo.SetOwner(context.Background(), stewardTenant, id, ""))
	assert.Empty(t, f.steward(t, id).OwnerID)
	assert.ErrorIs(t, f.repo.SetOwner(context.Background(), stewardTenant, uuid.NewString(), after),
		interfaces.ErrKnowledgeNotFound)
}

// Whoever creates an entry owns it; an API key, which speaks for nobody, does
// not become an owner, and an owner the caller set is kept.
func TestCreatingAnEntryMakesTheCreatorItsOwner(t *testing.T) {
	f := newStewardFixture(t, "")
	person := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	chosen := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	repo := NewKnowledgeRepository(f.db)
	create := func(ctx context.Context, owner *string) *types.KnowledgeSteward {
		k := &types.Knowledge{
			TenantID: stewardTenant, KnowledgeBaseID: f.kb, Type: "manual", Title: "Entry", Source: "manual",
			ParseStatus: types.ParseStatusCompleted, OwnerID: owner,
		}
		require.NoError(t, repo.CreateKnowledge(ctx, k))
		return f.steward(t, k.ID)
	}
	asPerson := context.WithValue(context.Background(), types.UserIDContextKey, person)
	asKey := context.WithValue(context.Background(), types.UserIDContextKey, "system-7")

	assert.Equal(t, person, create(asPerson, nil).OwnerID)
	assert.Empty(t, create(asKey, nil).OwnerID)
	assert.Empty(t, create(context.Background(), nil).OwnerID)
	assert.Equal(t, chosen, create(asPerson, &chosen).OwnerID)
}

// An owner must be able to change what they own: an Admin+ of the workspace,
// or the creator of the knowledge base; an active membership alone is not
// enough, and nor is a role on a membership that is no longer active.
func TestCanMaintain(t *testing.T) {
	creator := uuid.NewString()
	f := newStewardFixture(t, creator)
	require.NoError(t, f.db.Exec(`INSERT INTO users (id, username, email, password_hash)
		VALUES (?, 'creator', 'creator@example.com', 'x')`, creator).Error)
	require.NoError(t, f.db.Exec(`INSERT INTO tenant_members (user_id, tenant_id, role) VALUES (?, ?, 'contributor')`,
		creator, stewardTenant).Error)
	admin := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusActive, true)
	contributor := f.member(t, types.TenantRoleContributor, types.TenantMemberStatusActive, true)
	suspendedAdmin := f.member(t, types.TenantRoleAdmin, types.TenantMemberStatusSuspended, true)

	for user, want := range map[string]bool{
		creator: true, admin: true, contributor: false, suspendedAdmin: false, uuid.NewString(): false, "": false,
	} {
		got, err := f.repo.CanMaintain(context.Background(), stewardTenant, f.kb, user)
		require.NoError(t, err)
		assert.Equal(t, want, got, "user %q", user)
	}
}
