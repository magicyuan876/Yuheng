package repository

import (
	"context"
	"testing"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newSessionRepositoryForTest(t *testing.T) (interfaces.SessionRepository, *gorm.DB) {
	t.Helper()

	db := pgtest.New(t)

	return NewSessionRepository(db), db
}

func createSessionForTest(t *testing.T, db *gorm.DB, tenantID uint64, userID string) *types.Session {
	t.Helper()

	session := &types.Session{
		TenantID: tenantID,
		UserID:   userID,
		Title:    userID + " session",
	}
	require.NoError(t, db.Create(session).Error)

	return session
}

func countActiveSessionsForTest(t *testing.T, db *gorm.DB, id string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Model(&types.Session{}).Where("id = ?", id).Count(&count).Error)
	return count
}

func sessionIDsForTest(sessions []*types.Session) []string {
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	return ids
}

func TestSessionRepositoryGetAndListHonorUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")
	_ = createSessionForTest(t, db, 2, "bob")

	_, err := repo.Get(ctx, 1, "bob", aliceSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := repo.Get(ctx, 1, "bob", bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)

	sessions, err := repo.GetByTenantID(ctx, 1, "bob")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{bobSession.ID}, sessionIDsForTest(sessions))

	paged, total, err := repo.GetPagedByTenantID(ctx, 1, "bob", &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.ElementsMatch(t, []string{bobSession.ID}, sessionIDsForTest(paged))

	// The empty owner is the tenant-wide view admin listings use.
	all, err := repo.GetByTenantID(ctx, 1, "")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{aliceSession.ID, bobSession.ID}, sessionIDsForTest(all))
}

func TestSessionRepositoryUpdateHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")

	rows, err := repo.Update(ctx, &types.Session{
		ID:       aliceSession.ID,
		TenantID: aliceSession.TenantID,
		Title:    "bob update attempt",
	}, "bob")
	require.NoError(t, err)
	require.Zero(t, rows)

	var unchanged types.Session
	require.NoError(t, db.First(&unchanged, "id = ?", aliceSession.ID).Error)
	require.Equal(t, aliceSession.Title, unchanged.Title)

	rows, err = repo.Update(ctx, &types.Session{
		ID:       aliceSession.ID,
		TenantID: aliceSession.TenantID,
		Title:    "alice updated session",
	}, "alice")
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)

	var changed types.Session
	require.NoError(t, db.First(&changed, "id = ?", aliceSession.ID).Error)
	require.Equal(t, "alice updated session", changed.Title)
}

func TestSessionRepositoryDeleteHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")

	rows, err := repo.Delete(ctx, 1, "bob", aliceSession.ID)
	require.NoError(t, err)
	require.Zero(t, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))

	rows, err = repo.Delete(ctx, 1, "bob", bobSession.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
}

func TestSessionRepositoryBatchDeleteHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")

	rows, err := repo.BatchDelete(ctx, 1, "bob", []string{aliceSession.ID, bobSession.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
}

func TestSessionRepositoryDeleteAllHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")
	otherTenantSession := createSessionForTest(t, db, 2, "bob")

	rows, err := repo.DeleteAllByTenantID(ctx, 1, "bob")
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, otherTenantSession.ID))
}

func listItemIDsForTest(items []*types.SessionListItem) []string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

// A source filter this build does not know must never widen the listing: it
// falls back to the web filter, so API-key sessions stay out of it.
func TestSessionRepositoryQueryPagedUnknownSourceFallsBackToWeb(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()

	web := createSessionForTest(t, db, 1, "alice")
	_ = createSessionForTest(t, db, 1, types.SessionOwnerAPITenantKeyPrefix+"1:10")

	items, _, err := repo.QueryPaged(ctx, &types.SessionListQuery{
		TenantID: 1, Source: "wecom", Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	require.Equal(t, []string{web.ID}, listItemIDsForTest(items),
		"unknown sources must fall back to the web filter")
}

// The "web" source is user chats only; API-key sessions live in the
// admin-only "api" bucket and must never leak into a tenant-wide web listing.
func TestSessionRepositoryQueryPagedWebExcludesAPIKeySessions(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()

	web := createSessionForTest(t, db, 1, "alice")
	_ = createSessionForTest(t, db, 1, types.SessionOwnerAPITenantKeyPrefix+"1:10")
	_ = createSessionForTest(t, db, 1, types.SessionOwnerAPIExternalUserPrefix+"1:alice")

	items, _, err := repo.QueryPaged(ctx, &types.SessionListQuery{
		TenantID: 1, UserID: "", Source: "web", Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	require.Equal(t, []string{web.ID}, listItemIDsForTest(items),
		"web must list user chats but exclude API-key sessions")
}

func TestSessionRepositoryQueryPagedAPISourceReturnsAllTenantAPIKeySessions(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()

	// API requests without and with external-user identity, plus a web user and
	// cross-tenant API sessions.
	key1 := createSessionForTest(t, db, 1, types.SessionOwnerAPITenantKeyPrefix+"1:10")
	key2 := createSessionForTest(t, db, 1, types.SessionOwnerAPITenantKeyPrefix+"1:20")
	directHeader := createSessionForTest(t, db, 1, types.SessionOwnerAPIExternalUserPrefix+"1:alice")
	signedToken := createSessionForTest(t, db, 1, types.SessionOwnerAPIExternalUserPrefix+"1:bob")
	_ = createSessionForTest(t, db, 1, "alice")
	_ = createSessionForTest(t, db, 2, types.SessionOwnerAPITenantKeyPrefix+"2:30")
	_ = createSessionForTest(t, db, 2, types.SessionOwnerAPIExternalUserPrefix+"2:mallory")

	// The admin view clears UserID, so every API-key session in the tenant is
	// returned regardless of which key created it.
	items, total, err := repo.QueryPaged(ctx, &types.SessionListQuery{
		TenantID: 1, UserID: "", Source: types.SessionSourceAPI, Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.ElementsMatch(
		t,
		[]string{key1.ID, key2.ID, directHeader.ID, signedToken.ID},
		listItemIDsForTest(items),
	)
}
