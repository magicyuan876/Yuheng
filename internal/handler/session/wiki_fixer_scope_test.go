package session

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

type wikiFixerKBLookupStub struct {
	kb         *types.KnowledgeBase
	err        error
	calledWith string
}

func (s *wikiFixerKBLookupStub) GetKnowledgeBaseByIDOnly(_ context.Context, id string) (*types.KnowledgeBase, error) {
	s.calledWith = id
	return s.kb, s.err
}

type wikiFixerKBShareStub struct {
	permission        types.OrgMemberRole
	isShared          bool
	err               error
	checkedKBID       string
	checkedTenantID   uint64
	checkedTenantRole types.TenantRole
}

func (s *wikiFixerKBShareStub) CheckTenantKBPermission(
	_ context.Context,
	kbID string,
	callerTenantID uint64,
	callerTenantRole types.TenantRole,
) (types.OrgMemberRole, bool, error) {
	s.checkedKBID = kbID
	s.checkedTenantID = callerTenantID
	s.checkedTenantRole = callerTenantRole
	return s.permission, s.isShared, s.err
}

func TestResolveSingleSharedKBChatTenantScope_SharedEditorUsesSourceTenant(t *testing.T) {
	kbLookup := &wikiFixerKBLookupStub{
		kb: &types.KnowledgeBase{ID: "kb-shared", TenantID: 20, Name: "Shared KB"},
	}
	kbShare := &wikiFixerKBShareStub{
		permission: types.OrgRoleEditor,
		isShared:   true,
	}

	effectiveTenantID := resolveSingleSharedKBChatTenantScope(
		context.Background(),
		10,
		types.TenantRoleContributor,
		[]string{"kb-shared"},
		kbLookup,
		kbShare,
	)

	require.Equal(t, uint64(20), effectiveTenantID)
	require.Equal(t, "kb-shared", kbLookup.calledWith)
	require.Equal(t, "kb-shared", kbShare.checkedKBID)
	require.Equal(t, uint64(10), kbShare.checkedTenantID)
	require.Equal(t, types.TenantRoleContributor, kbShare.checkedTenantRole)
}

func TestResolveSingleSharedKBChatTenantScope_SharedViewerDoesNotSwitchTenant(t *testing.T) {
	kbLookup := &wikiFixerKBLookupStub{
		kb: &types.KnowledgeBase{ID: "kb-shared", TenantID: 20},
	}
	kbShare := &wikiFixerKBShareStub{
		permission: types.OrgRoleViewer,
		isShared:   true,
	}

	effectiveTenantID := resolveSingleSharedKBChatTenantScope(
		context.Background(),
		10,
		types.TenantRoleContributor,
		[]string{"kb-shared"},
		kbLookup,
		kbShare,
	)

	require.Zero(t, effectiveTenantID)
	require.Equal(t, uint64(20), kbLookup.kb.TenantID, "must not mutate the KB")
}

func TestResolveSingleSharedKBChatTenantScope_RequiresSingleKnowledgeBase(t *testing.T) {
	kbLookup := &wikiFixerKBLookupStub{
		kb: &types.KnowledgeBase{ID: "kb-shared", TenantID: 20},
	}

	effectiveTenantID := resolveSingleSharedKBChatTenantScope(
		context.Background(),
		10,
		types.TenantRoleContributor,
		[]string{"kb-a", "kb-b"},
		kbLookup,
		&wikiFixerKBShareStub{permission: types.OrgRoleEditor, isShared: true},
	)

	require.Zero(t, effectiveTenantID)
	require.Empty(t, kbLookup.calledWith)
}

func TestResolveSingleSharedKBChatTenantScope_FallsBackOnLookupOrPermissionErrors(t *testing.T) {
	t.Run("kb lookup error", func(t *testing.T) {
		effectiveTenantID := resolveSingleSharedKBChatTenantScope(
			context.Background(),
			10,
			types.TenantRoleContributor,
			[]string{"kb-shared"},
			&wikiFixerKBLookupStub{err: errors.New("lookup failed")},
			&wikiFixerKBShareStub{permission: types.OrgRoleEditor, isShared: true},
		)

		require.Zero(t, effectiveTenantID)
	})

	t.Run("permission check error", func(t *testing.T) {
		effectiveTenantID := resolveSingleSharedKBChatTenantScope(
			context.Background(),
			10,
			types.TenantRoleContributor,
			[]string{"kb-shared"},
			&wikiFixerKBLookupStub{kb: &types.KnowledgeBase{ID: "kb-shared", TenantID: 20}},
			&wikiFixerKBShareStub{err: errors.New("permission failed")},
		)

		require.Zero(t, effectiveTenantID)
	})
}
