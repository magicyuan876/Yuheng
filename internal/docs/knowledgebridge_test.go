package docs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// stubKnowledgeService records what the bridge asks of the knowledge service.
// It reads the tenant out of the context the way the real one does: with an
// unchecked type assertion, which panics when the value is missing.
type stubKnowledgeService struct {
	interfaces.KnowledgeService // every method the bridge does not use

	payloads []types.ManualKnowledgePayload
	tenants  []uint64
	infos    []*types.Tenant
	deleted  []string
	found    *types.Knowledge
	findErr  error
}

func (s *stubKnowledgeService) record(ctx context.Context) {
	s.tenants = append(s.tenants, ctx.Value(types.TenantIDContextKey).(uint64))
	s.infos = append(s.infos, ctx.Value(types.TenantInfoContextKey).(*types.Tenant))
}

func (s *stubKnowledgeService) CreateKnowledgeFromManual(ctx context.Context, _ string,
	p *types.ManualKnowledgePayload, _ string,
) (*types.Knowledge, error) {
	s.record(ctx)
	s.payloads = append(s.payloads, *p)
	return &types.Knowledge{ID: "k-1"}, nil
}

func (s *stubKnowledgeService) UpdateManualKnowledge(ctx context.Context, _ string,
	p *types.ManualKnowledgePayload,
) (*types.Knowledge, error) {
	s.record(ctx)
	s.payloads = append(s.payloads, *p)
	return &types.Knowledge{ID: "k-1"}, nil
}

func (s *stubKnowledgeService) DeleteKnowledge(ctx context.Context, id string) error {
	s.record(ctx)
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *stubKnowledgeService) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	return s.found, s.findErr
}

type stubTenants struct {
	interfaces.TenantRepository
	err error
}

func (s stubTenants) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &types.Tenant{ID: id}, nil
}

func newBridge(svc *stubKnowledgeService, tenants interfaces.TenantRepository) *knowledgeBridge {
	return NewKnowledgeBridge(svc, tenants).(*knowledgeBridge)
}

// A draft is stored and never chunked or embedded: the mirror would look
// complete and answer nothing.
func TestMirroredPagesArePublishedNotSavedAsDrafts(t *testing.T) {
	svc := &stubKnowledgeService{}
	b := newBridge(svc, stubTenants{})

	_, err := b.CreateKnowledgeFromText(context.Background(), 7, "kb-1", "标题", "正文")
	require.NoError(t, err)
	updated, err := b.UpdateKnowledgeContent(context.Background(), 7, "k-1", "标题", "新正文")
	require.NoError(t, err)
	require.True(t, updated)

	require.Len(t, svc.payloads, 2)
	for _, p := range svc.payloads {
		assert.Equal(t, types.ManualKnowledgeStatusPublish, p.Status)
		assert.Equal(t, DocsChannel, p.Channel)
	}
}

// The workers run outside any request, so nothing puts the tenant in the
// context; the bridge has to, or the knowledge service fails on a missing value.
func TestTheKnowledgeServiceIsCalledOnBehalfOfTheTenant(t *testing.T) {
	svc := &stubKnowledgeService{}
	b := newBridge(svc, stubTenants{})

	_, err := b.CreateKnowledgeFromText(context.Background(), 7, "kb-1", "标题", "正文")
	require.NoError(t, err)
	_, err = b.UpdateKnowledgeContent(context.Background(), 7, "k-1", "标题", "正文")
	require.NoError(t, err)
	require.NoError(t, b.DeleteKnowledge(context.Background(), 7, "k-1"))

	assert.Equal(t, []uint64{7, 7, 7}, svc.tenants)
	for _, info := range svc.infos {
		assert.EqualValues(t, 7, info.ID)
	}
}

func TestAMissingTenantIsAnErrorNotAPanic(t *testing.T) {
	svc := &stubKnowledgeService{}
	b := newBridge(svc, stubTenants{err: errors.New("no such tenant")})

	_, err := b.CreateKnowledgeFromText(context.Background(), 7, "kb-1", "标题", "正文")
	assert.Error(t, err)
	_, err = b.UpdateKnowledgeContent(context.Background(), 7, "k-1", "标题", "正文")
	assert.Error(t, err)
	assert.Error(t, b.DeleteKnowledge(context.Background(), 7, "k-1"))
	assert.Empty(t, svc.payloads)
}

func TestKnowledgeBaseOfTellsAMissingEntryFromAFailedLookup(t *testing.T) {
	svc := &stubKnowledgeService{findErr: interfaces.ErrKnowledgeNotFound}
	b := newBridge(svc, stubTenants{})
	_, found, err := b.KnowledgeBaseOf(context.Background(), "k-1")
	require.NoError(t, err)
	assert.False(t, found)

	svc.findErr = errors.New("database down")
	_, _, err = b.KnowledgeBaseOf(context.Background(), "k-1")
	assert.Error(t, err)

	svc.findErr = nil
	svc.found = &types.Knowledge{KnowledgeBaseID: "kb-9"}
	kb, found, err := b.KnowledgeBaseOf(context.Background(), "k-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "kb-9", kb)
}

func TestTheBridgeNeedsBothServices(t *testing.T) {
	assert.Nil(t, NewKnowledgeBridge(nil, stubTenants{}))
	assert.Nil(t, NewKnowledgeBridge(&stubKnowledgeService{}, nil))
}
