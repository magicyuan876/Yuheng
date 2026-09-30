package service

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testSessionScopeContext(tenantID uint64, userID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	if userID != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	}
	return ctx
}

func testAPISessionScopeContext(tenantID uint64, externalUserID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-7")
	return types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPIExternalUser,
		ID:   externalUserID,
	})
}

func testAPITenantKeyScopeContext(tenantID uint64, keyID uint64) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
	ctx = types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPITenant,
		ID:   "1",
	})
	return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KeyID: keyID})
}

func newTestSessionService(t *testing.T) (*sessionService, *gorm.DB) {
	t.Helper()

	db := pgtest.New(t)

	return &sessionService{
		sessionRepo: repository.NewSessionRepository(db),
	}, db
}

func TestGetSessionIsScopedToCurrentUser(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   "alice",
		Title:    "alice private session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(bobSession).Error)

	_, err := svc.GetSession(testSessionScopeContext(1, "bob"), aliceSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := svc.GetSession(testSessionScopeContext(1, "bob"), bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)
}

// The owner scope is what keeps sessions private, so a session nobody owns is
// refused at creation rather than stored for everyone or no one to see.
func TestCreateSessionRequiresAnOwner(t *testing.T) {
	svc, _ := newTestSessionService(t)

	_, err := svc.CreateSession(testSessionScopeContext(1, "alice"), &types.Session{TenantID: 1, Title: "ownerless"})
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrUnauthorized, appErr.Code)

	created, err := svc.CreateSession(testSessionScopeContext(1, "alice"),
		&types.Session{TenantID: 1, UserID: "alice", Title: "owned"})
	require.NoError(t, err)
	require.Equal(t, "alice", created.UserID)
}

func TestUpdateSessionIsScopedToCurrentUserAndAllowsNoOp(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "alice private session",
		Description: "original description",
	}
	require.NoError(t, db.Create(aliceSession).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "bob"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       "bob update attempt",
		Description: "should not be saved",
	})
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	var unchanged types.Session
	require.NoError(t, db.First(&unchanged, "id = ?", aliceSession.ID).Error)
	require.Equal(t, aliceSession.Title, unchanged.Title)
	require.Equal(t, aliceSession.Description, unchanged.Description)

	err = svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       aliceSession.Title,
		Description: aliceSession.Description,
	})
	require.NoError(t, err)
}

func TestGetSessionIsScopedToAPIExternalUser(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   "api_external_user:7:alice",
		Title:    "alice api session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   "api_external_user:7:bob",
		Title:    "bob api session",
	}
	require.NoError(t, db.Create(bobSession).Error)
	tenantSession := &types.Session{
		TenantID: 1,
		UserID:   "system-7",
		Title:    "tenant api session",
	}
	require.NoError(t, db.Create(tenantSession).Error)

	_, err := svc.GetSession(testAPISessionScopeContext(1, "7:alice"), bobSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := svc.GetSession(testAPISessionScopeContext(1, "7:alice"), aliceSession.ID)
	require.NoError(t, err)
	require.Equal(t, aliceSession.ID, got.ID)

	_, err = svc.GetSession(testAPISessionScopeContext(1, "7:alice"), tenantSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err = svc.GetSession(testSessionScopeContext(1, "system-7"), tenantSession.ID)
	require.NoError(t, err)
	require.Equal(t, tenantSession.ID, got.ID)
}

func TestGetSessionAllowsAdminToOpenAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)
	otherUserSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(otherUserSession).Error)

	// A non-admin web user cannot open the API-key session.
	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.GetSession(viewerCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	// An admin can open the API-key session.
	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// The admin fallback is limited to API-key sessions; another user's
	// personal session stays hidden.
	_, err = svc.GetSession(adminCtx, otherUserSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetOwnedSessionDeniesAdminOnAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	adminCtx := context.WithValue(
		testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin,
	)

	// The read path lets an admin open the API-key session (folder navigation)...
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// ...but the strict owner scope used by write/mutation endpoints (title
	// generation, attachments, stop, QA) denies it, so admins stay read-only.
	_, err = svc.GetOwnedSession(adminCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestListSessionsAPISourceRequiresAdminAndReturnsAllKeys(t *testing.T) {
	svc, db := newTestSessionService(t)

	key1 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:10", Title: "key1"}
	key2 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:20", Title: "key2"}
	externalUser := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	web := &types.Session{TenantID: 1, UserID: "alice", Title: "alice web"}
	require.NoError(t, db.Create(key1).Error)
	require.NoError(t, db.Create(key2).Error)
	require.NoError(t, db.Create(externalUser).Error)
	require.NoError(t, db.Create(web).Error)

	// A non-admin (viewer) web user is rejected.
	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.ListSessions(viewerCtx, &types.SessionListQuery{Source: types.SessionSourceAPI})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	// An admin sees every API-key session in the tenant, not just their own.
	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	result, err := svc.ListSessions(adminCtx, &types.SessionListQuery{Source: types.SessionSourceAPI})
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Total)
}

func TestGetSessionAllowsAdminToReadAPIExternalUserSession(t *testing.T) {
	svc, db := newTestSessionService(t)

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	require.NoError(t, db.Create(apiSession).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.GetSession(viewerCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	adminCtx := context.WithValue(viewerCtx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}

// A source filter the server does not know requires Admin+ like every non-web
// source, and the admin listing falls back to the web visibility filter.
func TestListSessionsUnknownSourceRequiresAdmin(t *testing.T) {
	svc, db := newTestSessionService(t)

	chat := &types.Session{TenantID: 1, UserID: "bob", Title: "bob chat"}
	require.NoError(t, db.Create(chat).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.ListSessions(viewerCtx, &types.SessionListQuery{Source: "feishu"})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	result, err := svc.ListSessions(adminCtx, &types.SessionListQuery{Source: "feishu"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
}

func TestGetSessionAllowsAPITenantRuntimeToReadOwnAPIKeySession(t *testing.T) {
	svc, db := newTestSessionService(t)

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	got, err := svc.GetSession(testAPITenantKeyScopeContext(1, 10), apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}
