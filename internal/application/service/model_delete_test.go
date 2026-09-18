package service

import (
	"context"
	"testing"
	"time"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type stubKBRepoForModelDelete struct {
	count int64
}

func (s *stubKBRepoForModelDelete) CreateKnowledgeBase(context.Context, *types.KnowledgeBase) error {
	return nil
}
func (s *stubKBRepoForModelDelete) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) GetKnowledgeBaseByIDAndTenant(context.Context, string, uint64) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) GetKnowledgeBaseByIDs(context.Context, []string) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) ListKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) ListKnowledgeBasesByTenantID(context.Context, uint64) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) UpdateKnowledgeBase(context.Context, *types.KnowledgeBase) error {
	return nil
}
func (s *stubKBRepoForModelDelete) DeleteKnowledgeBase(context.Context, string) error { return nil }
func (s *stubKBRepoForModelDelete) CountByVectorStoreIDAllTenants(
	_ context.Context, _ *gorm.DB, _ uint64, _ string,
) (int64, error) {
	return 0, nil
}
func (s *stubKBRepoForModelDelete) CountByVectorStoreID(context.Context, *gorm.DB, uint64, string) (int64, error) {
	return 0, nil
}
func (s *stubKBRepoForModelDelete) CountByModelID(context.Context, uint64, string) (int64, error) {
	return s.count, nil
}
func (s *stubKBRepoForModelDelete) CountByModelIDAllTenants(context.Context, string) (int64, error) {
	return s.count, nil
}
func (s *stubKBRepoForModelDelete) SetUserKBPin(context.Context, uint64, string, string, bool) (*time.Time, error) {
	return nil, nil
}
func (s *stubKBRepoForModelDelete) ListUserKBPinIDs(context.Context, uint64, string) (map[string]time.Time, error) {
	return nil, nil
}

type stubModelRepoForDelete struct {
	model  *types.Model
	delete func(id string) error
	update func(model *types.Model) error
}

func (s *stubModelRepoForDelete) Create(context.Context, *types.Model) error { return nil }
func (s *stubModelRepoForDelete) GetByID(_ context.Context, _ uint64, id string) (*types.Model, error) {
	if s.model != nil && s.model.ID == id {
		return s.model, nil
	}
	return nil, nil
}
func (s *stubModelRepoForDelete) List(context.Context, uint64, types.ModelType, types.ModelSource) ([]*types.Model, error) {
	return nil, nil
}
func (s *stubModelRepoForDelete) Update(_ context.Context, model *types.Model) error {
	if s.update != nil {
		return s.update(model)
	}
	return nil
}
func (s *stubModelRepoForDelete) Delete(_ context.Context, _ uint64, id string) error {
	if s.delete != nil {
		return s.delete(id)
	}
	return nil
}
func (s *stubModelRepoForDelete) ClearDefaultByType(context.Context, uint, types.ModelType, string) error {
	return nil
}

func TestDeleteModel_RejectsWhenReferenced(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	modelID := "model-in-use"

	svc := NewModelService(
		&stubModelRepoForDelete{model: &types.Model{ID: modelID, TenantID: 1}},
		&stubKBRepoForModelDelete{count: 1},
		nil, nil, nil,
	)

	err := svc.DeleteModel(ctx, modelID)
	require.Error(t, err)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.ErrBadRequest, appErr.Code)
	assert.Contains(t, appErr.Message, "knowledge base")
}

func TestDeleteModel_SucceedsWhenUnreferenced(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	modelID := "free-model"
	deleted := false

	svc := NewModelService(
		&stubModelRepoForDelete{
			model: &types.Model{ID: modelID, TenantID: 1},
			delete: func(id string) error {
				assert.Equal(t, modelID, id)
				deleted = true
				return nil
			},
		},
		&stubKBRepoForModelDelete{},
		nil, nil, nil,
	)

	require.NoError(t, svc.DeleteModel(ctx, modelID))
	assert.True(t, deleted)
}

type stubTenantServiceForModelDelete struct {
	tenant *types.Tenant
}

func (s *stubTenantServiceForModelDelete) CreateTenant(context.Context, *types.Tenant) (*types.Tenant, error) {
	return nil, nil
}
func (s *stubTenantServiceForModelDelete) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return s.tenant, nil
}
func (s *stubTenantServiceForModelDelete) GetTenantsByIDs(context.Context, []uint64) (map[uint64]*types.Tenant, error) {
	return nil, nil
}
func (s *stubTenantServiceForModelDelete) ListTenants(context.Context) ([]*types.Tenant, error) {
	return nil, nil
}
func (s *stubTenantServiceForModelDelete) UpdateTenant(context.Context, *types.Tenant) (*types.Tenant, error) {
	return nil, nil
}
func (s *stubTenantServiceForModelDelete) DeleteTenant(context.Context, uint64) error { return nil }
func (s *stubTenantServiceForModelDelete) ListAllTenants(context.Context) ([]*types.Tenant, error) {
	return nil, nil
}
func (s *stubTenantServiceForModelDelete) BulkSetStorageQuota(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *stubTenantServiceForModelDelete) SearchTenants(context.Context, string, uint64, int, int) ([]*types.Tenant, int64, error) {
	return nil, 0, nil
}
func (s *stubTenantServiceForModelDelete) GetTenantByIDForUser(context.Context, uint64, string) (*types.Tenant, error) {
	return s.tenant, nil
}

func TestFormatModelInUseMessage(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		"model is used by 1 knowledge base(s); reconfigure or remove those references before deleting",
		formatModelInUseMessage(1),
	)
	assert.Equal(t,
		"model is used by 3 knowledge base(s); reconfigure or remove those references before deleting",
		formatModelInUseMessage(3),
	)
}
