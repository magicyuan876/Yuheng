package service

import (
	"testing"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sharing a model exposes it to every workspace on the deployment, so the
// decision belongs to a platform operator and not to whoever happens to own
// the workspace the model was created in. Every self-registered user is Owner
// of their own workspace, so a tenant-role check could not express this.
func TestSetModelSharing_RequiresSystemAdmin(t *testing.T) {
	stored := &types.Model{ID: "chat-1", TenantID: 7}
	updated := false
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(*types.Model) error {
			updated = true
			return nil
		},
	}, nil, nil, nil, nil)

	_, err := svc.SetModelSharing(builtinModelContext(false), stored.ID, true)
	require.Error(t, err)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.ErrForbidden, appErr.Code)
	assert.False(t, updated, "sharing must not be persisted for a non-system-admin")
	assert.False(t, stored.IsBuiltin)
}

// Promotion must clear managed_by. The builtin_models.yaml reconciler sweeps
// rows still tagged managed_by="yaml" that the file no longer declares, so a
// UI-shared model left carrying that tag would be soft-deleted on the next
// restart.
func TestSetModelSharing_PromoteClearsYAMLOwnership(t *testing.T) {
	stored := &types.Model{
		ID: "chat-1", TenantID: 7,
		ManagedBy: types.BuiltinModelManagedBy,
	}
	var saved *types.Model
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(model *types.Model) error {
			copied := *model
			saved = &copied
			return nil
		},
	}, nil, nil, nil, nil)

	got, err := svc.SetModelSharing(builtinModelContext(true), stored.ID, true)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsBuiltin)
	assert.Empty(t, saved.ManagedBy, "UI sharing must take the row out of YAML ownership")
	assert.True(t, got.IsBuiltin)
}

// Withdrawing sharing while another workspace still binds the model would
// strand it: knowledge_bases.embedding_model_id is a bare string with no
// foreign key, so retrieval degrades with nothing raised anywhere. The counts
// consulted must be cross-tenant — the tenant-scoped ones report zero for a
// model the caller's own workspace does not use.
func TestSetModelSharing_WithdrawRefusedWhileReferenced(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kbCount int64
	}{
		{"knowledge base in another workspace", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := &types.Model{ID: "builtin-embed", TenantID: 10000, IsBuiltin: true}
			updated := false
			svc := NewModelService(
				&stubModelRepoForDelete{
					model: stored,
					update: func(*types.Model) error {
						updated = true
						return nil
					},
				},
				&stubKBRepoForModelDelete{count: tc.kbCount},
				nil, nil, nil,
			)

			_, err := svc.SetModelSharing(builtinModelContext(true), stored.ID, false)
			require.Error(t, err)
			appErr, ok := apperrors.IsAppError(err)
			require.True(t, ok)
			assert.Equal(t, apperrors.ErrBadRequest, appErr.Code)
			assert.False(t, updated)
			assert.True(t, stored.IsBuiltin, "refused withdrawal must leave the row shared")
		})
	}
}

func TestSetModelSharing_WithdrawAllowedWhenUnreferenced(t *testing.T) {
	stored := &types.Model{ID: "builtin-embed", TenantID: 10000, IsBuiltin: true}
	var saved *types.Model
	svc := NewModelService(
		&stubModelRepoForDelete{
			model: stored,
			update: func(model *types.Model) error {
				copied := *model
				saved = &copied
				return nil
			},
		},
		&stubKBRepoForModelDelete{count: 0},
		nil, nil, nil,
	)

	_, err := svc.SetModelSharing(builtinModelContext(true), stored.ID, false)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.False(t, saved.IsBuiltin)
	assert.Empty(t, saved.ManagedBy, "an un-shared row must not be resurrected by the YAML reconciler")
}

// Re-applying the current state is a no-op success rather than an error, so a
// client that retries a request or double-clicks a toggle does not see a
// spurious failure. Crucially it must also skip the reference check, which
// would otherwise reject "un-share an already-unshared model".
func TestSetModelSharing_Idempotent(t *testing.T) {
	stored := &types.Model{ID: "builtin-chat", TenantID: 10000, IsBuiltin: true}
	updated := false
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(*types.Model) error {
			updated = true
			return nil
		},
	}, nil, nil, nil, nil)

	got, err := svc.SetModelSharing(builtinModelContext(true), stored.ID, true)
	require.NoError(t, err)
	assert.True(t, got.IsBuiltin)
	assert.False(t, updated, "no write for an unchanged sharing state")
}

func TestSetModelSharing_UnknownModel(t *testing.T) {
	svc := NewModelService(&stubModelRepoForDelete{}, nil, nil, nil, nil)

	_, err := svc.SetModelSharing(builtinModelContext(true), "missing", true)
	assert.ErrorIs(t, err, ErrModelNotFound)
}

// UpdateModel must never move a model in or out of platform sharing: callers
// across the codebase pass partially-populated structs, so honouring a zero
// IsBuiltin there would let an unrelated edit silently strand every workspace
// bound to a shared model.
func TestUpdateModel_CannotWithdrawSharing(t *testing.T) {
	stored := &types.Model{ID: "builtin-chat", TenantID: 10000, IsBuiltin: true}
	var saved *types.Model
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(model *types.Model) error {
			copied := *model
			saved = &copied
			return nil
		},
	}, nil, nil, nil, nil)

	// IsBuiltin left at its zero value, as a partial update would leave it.
	err := svc.UpdateModel(builtinModelContext(true), &types.Model{ID: stored.ID, Name: "edited"})
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsBuiltin, "a plain edit must not un-share a platform model")
}
