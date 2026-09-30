package service

import (
	"context"
	"errors"
	"fmt"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/models/asr"
	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/models/embedding"
	"github.com/magicyuan876/yuheng/internal/models/rerank"
	"github.com/magicyuan876/yuheng/internal/models/utils/ollama"
	"github.com/magicyuan876/yuheng/internal/models/vlm"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// ErrModelNotFound is returned when a model cannot be found in the repository
var ErrModelNotFound = errors.New("model not found")

// modelService implements the model service interface
type modelService struct {
	repo          interfaces.ModelRepository
	kbRepo        interfaces.KnowledgeBaseRepository
	ollamaService *ollama.OllamaService
	pooler        embedding.EmbedderPooler
	tenantService interfaces.TenantService
}

// NewModelService creates a new model service instance
func NewModelService(repo interfaces.ModelRepository,
	kbRepo interfaces.KnowledgeBaseRepository,
	ollamaService *ollama.OllamaService,
	pooler embedding.EmbedderPooler,
	tenantService interfaces.TenantService,
) interfaces.ModelService {
	return &modelService{
		repo:          repo,
		kbRepo:        kbRepo,
		ollamaService: ollamaService,
		pooler:        pooler,
		tenantService: tenantService,
	}
}

// decryptAppSecret 解密 AppSecret（如果为空或 cryptoSvc 为空则原样返回）
func (s *modelService) decryptAppSecret(encrypted string) string {
	if encrypted == "" {
		return encrypted
	}
	if key := utils.GetAESKey(); key != nil {
		if encrypted, err := utils.DecryptAESGCM(encrypted, key); err == nil {
			return encrypted
		}
	}
	return encrypted
}

// resolveModelAppCredentials returns the AppID / AppSecret stored on the model
// itself, decrypting the secret. Providers that authenticate with an app-id +
// signed-secret pair (rather than a bearer API key) read them from here.
func (s *modelService) resolveModelAppCredentials(
	_ context.Context, params *types.ModelParameters,
) (appID, appSecret string) {
	return params.AppID, s.decryptAppSecret(params.AppSecret)
}

// CreateModel creates a new model in the repository
// For local models, it initiates an asynchronous download process
// Remote models are immediately set to active status
func (s *modelService) CreateModel(ctx context.Context, model *types.Model) error {
	logger.Infof(ctx, "Creating model: %s, type: %s, source: %s", model.Name, model.Type, model.Source)

	// Handle remote models (e.g., OpenAI, Azure)
	if model.Source == types.ModelSourceRemote {
		logger.Info(ctx, "Remote model detected, setting status to active")
		model.Status = types.ModelStatusActive

		logger.Info(ctx, "Saving remote model to repository")
		err := s.repo.Create(ctx, model)
		if err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"model_name": model.Name,
				"model_type": model.Type,
			})
			return err
		}

		logger.Infof(ctx, "Remote model created successfully: %s", model.ID)
		return nil
	}

	// Handle local models (e.g., Ollama)
	logger.Info(ctx, "Local model detected, setting status to downloading")
	model.Status = types.ModelStatusDownloading

	logger.Info(ctx, "Saving local model to repository")
	err := s.repo.Create(ctx, model)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_name": model.Name,
			"model_type": model.Type,
		})
		return err
	}

	// Start asynchronous model download
	logger.Infof(ctx, "Starting background download for model: %s", model.Name)
	newCtx := logger.CloneContext(ctx)
	go func() {
		logger.Info(newCtx, "Background download started")
		err := s.ollamaService.PullModel(newCtx, model.Name)
		if err != nil {
			logger.ErrorWithFields(newCtx, err, map[string]interface{}{
				"model_name": model.Name,
			})
			model.Status = types.ModelStatusDownloadFailed
		} else {
			logger.Infof(newCtx, "Model download completed successfully: %s", model.Name)
			model.Status = types.ModelStatusActive
		}
		logger.Infof(newCtx, "Updating model status to: %s", model.Status)
		s.repo.Update(newCtx, model)
	}()

	logger.Infof(ctx, "Model creation initiated successfully: %s", model.ID)
	return nil
}

// GetModelByID retrieves a model by its ID
// Returns an error if the model is not found or is in a non-active state
func (s *modelService) GetModelByID(ctx context.Context, id string) (*types.Model, error) {
	// Check if ID is empty
	if id == "" {
		logger.Error(ctx, "Model ID is empty")
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	// Fetch model from repository
	model, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  id,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	// Check if model exists
	if model == nil {
		logger.Error(ctx, "Model not found")
		return nil, ErrModelNotFound
	}

	logger.Infof(ctx, "Model found, name: %s, status: %s", model.Name, model.Status)

	// Check model status
	if model.Status == types.ModelStatusActive {
		return model, nil
	}

	if model.Status == types.ModelStatusDownloading {
		logger.Warn(ctx, "Model is currently downloading")
		return nil, errors.New("model is currently downloading")
	}

	if model.Status == types.ModelStatusDownloadFailed {
		logger.Error(ctx, "Model download failed")
		return nil, errors.New("model download failed")
	}

	logger.Error(ctx, "Model status is abnormal")
	return nil, errors.New("abnormal model status")
}

// ListModels returns all models belonging to the tenant
func (s *modelService) ListModels(ctx context.Context) ([]*types.Model, error) {
	logger.Info(ctx, "Start listing models")

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Listing models for tenant ID: %d", tenantID)

	// List models from repository with no additional filters
	models, err := s.repo.List(ctx, tenantID, "", "")
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Retrieved %d models successfully", len(models))
	return models, nil
}

// UpdateModel updates an existing model in the repository
func (s *modelService) UpdateModel(ctx context.Context, model *types.Model) error {
	logger.Info(ctx, "Start updating model")
	logger.Infof(ctx, "Updating model ID: %s, name: %s", model.ID, model.Name)

	// Built-in models are platform-wide. Tenant administrators may view them,
	// but only a system administrator may change their shared configuration.
	tenantID := types.MustTenantIDFromContext(ctx)
	existingModel, err := s.repo.GetByID(ctx, tenantID, model.ID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": model.ID,
		})
		return err
	}
	if existingModel != nil && existingModel.IsBuiltin {
		if !types.IsSystemAdminFromContext(ctx) {
			logger.Warnf(ctx, "Non-system-admin attempted to update builtin model: %s", model.ID)
			return apperrors.NewForbiddenError("only system administrators can update builtin models")
		}
		// A UI edit is an explicit runtime override. Clear YAML ownership so
		// the startup reconciler does not silently replace the saved values.
		model.TenantID = existingModel.TenantID
		// Sharing is deliberately NOT read from the incoming model here.
		// Callers routinely build a partial types.Model carrying only the
		// fields they mean to change, so honouring a zero IsBuiltin would let
		// an ordinary edit silently un-share a platform model — and that
		// strands every other workspace bound to it. Toggling sharing is a
		// separate, explicit operation: see SetModelSharing.
		model.IsBuiltin = true
		model.ManagedBy = ""
	}

	// Update model in repository
	err = s.repo.Update(ctx, model)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return err
	}

	logger.Infof(ctx, "Model updated successfully: %s", model.ID)
	return nil
}

// UpdateModelCredentials writes one or more credential fields on the model's
// Parameters jsonb. Models are not pooled per-instance (each call to
// GetEmbeddingModel/GetChatModel rebuilds the client from
// the current Parameters), so no explicit cache invalidation is required —
// the next call will pick up the new credential automatically.
func (s *modelService) UpdateModelCredentials(
	ctx context.Context, id string, apiKey, appSecret *string,
) (*types.Model, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrModelNotFound
	}
	if existing.IsBuiltin && !types.IsSystemAdminFromContext(ctx) {
		return nil, apperrors.NewForbiddenError(
			"only system administrators can modify builtin model credentials")
	}

	changed := false
	if apiKey != nil && *apiKey != "" && *apiKey != existing.Parameters.APIKey {
		existing.Parameters.APIKey = *apiKey
		changed = true
	}
	if appSecret != nil && *appSecret != "" && *appSecret != existing.Parameters.AppSecret {
		existing.Parameters.AppSecret = *appSecret
		changed = true
	}
	if !changed {
		return existing, nil
	}
	if existing.IsBuiltin {
		// Credential changes are also runtime overrides of YAML-managed data.
		existing.ManagedBy = ""
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Model credentials updated: id=%s", id)
	return existing, nil
}

// ClearModelCredential removes a single credential field. Idempotent.
func (s *modelService) ClearModelCredential(ctx context.Context, id, field string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrModelNotFound
	}
	if existing.IsBuiltin && !types.IsSystemAdminFromContext(ctx) {
		return apperrors.NewForbiddenError(
			"only system administrators can modify builtin model credentials")
	}

	changed := false
	switch field {
	case "api_key":
		if existing.Parameters.APIKey != "" {
			existing.Parameters.APIKey = ""
			changed = true
		}
	case "app_secret":
		if existing.Parameters.AppSecret != "" {
			existing.Parameters.AppSecret = ""
			changed = true
		}
	default:
		return errors.New("unknown credential field: " + field)
	}
	if !changed {
		return nil
	}
	if existing.IsBuiltin {
		existing.ManagedBy = ""
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}
	logger.Infof(ctx, "Model credential cleared by user: id=%s field=%s", id, field)
	return nil
}

// SetModelSharing promotes a workspace model to a platform-shared ("built-in")
// model, or withdraws one from sharing. System administrators only.
//
// Deliberately a dedicated operation rather than a field on UpdateModel:
// sharing changes which workspaces can bind the model, and UpdateModel is
// called with partially-populated structs all over the codebase, so a plain
// bool there would make an unrelated edit capable of silently un-sharing a
// model that other workspaces depend on.
func (s *modelService) SetModelSharing(ctx context.Context, id string, shared bool) (*types.Model, error) {
	if !types.IsSystemAdminFromContext(ctx) {
		logger.Warnf(ctx, "Non-system-admin attempted to change model sharing: %s", id)
		return nil, apperrors.NewForbiddenError("only system administrators can change model sharing")
	}

	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"model_id": id})
		return nil, err
	}
	if existing == nil {
		return nil, ErrModelNotFound
	}
	if existing.IsBuiltin == shared {
		return existing, nil // idempotent
	}

	if !shared {
		// Withdrawing sharing strands every other workspace that selected
		// this model: knowledge_bases.embedding_model_id is a bare string with
		// no foreign key, so retrieval would degrade with nothing raised
		// anywhere. Refuse while any knowledge base binds it.
		if err := s.assertBuiltinModelUnreferenced(ctx, id); err != nil {
			return nil, err
		}
	}

	existing.IsBuiltin = shared
	// Runtime-owned from here on, either way: an un-shared row must not be
	// resurrected as built-in by the builtin_models.yaml reconciler, and a
	// newly shared row must not be swept away by its drift pass (which only
	// deletes rows still tagged managed_by="yaml").
	existing.ManagedBy = ""
	if err := s.repo.Update(ctx, existing); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"model_id": id})
		return nil, err
	}
	logger.Infof(ctx, "Model %s platform sharing set to %v", id, shared)
	return existing, nil
}

// assertBuiltinModelUnreferenced refuses the operation when any workspace
// still binds the given built-in model.
//
// The counts are deliberately cross-tenant: a built-in model is visible to
// every workspace, so the tenant-scoped counts used for ordinary models would
// report zero while other workspaces are actively using it.
func (s *modelService) assertBuiltinModelUnreferenced(ctx context.Context, id string) error {
	kbCount, err := s.kbRepo.CountByModelIDAllTenants(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"model_id": id})
		return err
	}
	if kbCount > 0 {
		logger.Warnf(ctx, "Builtin model %s still referenced by %d knowledge base(s)", id, kbCount)
		return apperrors.NewBadRequestError(formatModelInUseMessage(kbCount, false))
	}
	return nil
}

// DeleteModel removes a model from the repository
func (s *modelService) DeleteModel(ctx context.Context, id string) error {
	logger.Info(ctx, "Start deleting model")
	logger.Infof(ctx, "Deleting model ID: %s", id)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Tenant ID: %d", tenantID)

	// Check if the model is builtin - builtin models cannot be deleted
	existingModel, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": id,
		})
		return err
	}
	if existingModel == nil {
		return ErrModelNotFound
	}
	if existingModel.IsBuiltin {
		// A built-in model created or promoted through the UI must be
		// removable through the UI, otherwise the only way back out is
		// hand-written SQL. Two conditions guard it:
		//   1. only a system administrator may delete a platform resource;
		//   2. rows owned by config/builtin_models.yaml stay undeletable —
		//      the startup reconciler would UPSERT them straight back, so
		//      "deleting" one would look like it worked until the next
		//      restart. Those are removed by editing the YAML.
		if !types.IsSystemAdminFromContext(ctx) {
			logger.Warnf(ctx, "Non-system-admin attempted to delete builtin model: %s", id)
			return apperrors.NewForbiddenError("only system administrators can delete builtin models")
		}
		if existingModel.ManagedBy == types.BuiltinModelManagedBy {
			logger.Warnf(ctx, "Attempted to delete YAML-managed builtin model: %s", id)
			return apperrors.NewBadRequestError(
				"this built-in model is declared in builtin_models.yaml; remove it there instead")
		}
		if err := s.assertBuiltinModelUnreferenced(ctx, id); err != nil {
			return err
		}
		// Delete against the owning workspace: the repository keys on
		// (id, tenant_id) and a built-in row belongs to whichever workspace
		// created it, which is usually not the admin's current one. Passing
		// the caller's tenant here would silently delete nothing.
		if err := s.repo.Delete(ctx, existingModel.TenantID, id); err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{"model_id": id})
			return err
		}
		logger.Infof(ctx, "Builtin model deleted successfully: %s", id)
		return nil
	}

	kbCount, err := s.kbRepo.CountByModelID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": id,
		})
		return err
	}
	if kbCount > 0 {
		logger.Warnf(ctx, "Model %s is in use by %d knowledge base(s)", id, kbCount)
		return apperrors.NewBadRequestError(formatModelInUseMessage(kbCount, false))
	}

	// Delete model from repository
	err = s.repo.Delete(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  id,
			"tenant_id": tenantID,
		})
		return err
	}

	logger.Infof(ctx, "Model deleted successfully: %s", id)
	return nil
}

// GetEmbeddingModel retrieves and initializes an embedding model instance
// Takes a model ID and returns an Embedder interface implementation
func (s *modelService) GetEmbeddingModel(ctx context.Context, modelId string) (embedding.Embedder, error) {
	// Get the model details
	model, err := s.GetModelByID(ctx, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": modelId,
		})
		return nil, err
	}

	logger.Infof(ctx, "Getting embedding model: %s, source: %s", model.Name, model.Source)

	appID, appSecret := s.resolveModelAppCredentials(ctx, &model.Parameters)

	embedder, err := embedding.NewEmbedder(embedding.ConfigFromModel(model, appID, appSecret), s.pooler, s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	logger.Info(ctx, "Embedding model initialized successfully")
	return embedder, nil
}

// GetEmbeddingModelForTenant retrieves and initializes an embedding model for a specific tenant
// This is used for cross-tenant knowledge base sharing where the embedding model from
// the source tenant must be used to ensure vector compatibility
func (s *modelService) GetEmbeddingModelForTenant(ctx context.Context, modelId string, tenantID uint64) (embedding.Embedder, error) {
	// Check if model ID is empty
	if modelId == "" {
		logger.Error(ctx, "Model ID is empty")
		return nil, errors.New("model ID cannot be empty")
	}

	// Fetch model from repository using the specified tenant ID
	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		logger.Error(ctx, "Model not found for specified tenant")
		return nil, ErrModelNotFound
	}

	if model.Status != types.ModelStatusActive {
		logger.Errorf(ctx, "Model is not active, status: %s", model.Status)
		return nil, errors.New("model is not active")
	}

	logger.Infof(ctx, "Getting cross-tenant embedding model: %s, source: %s, tenant: %d", model.Name, model.Source, tenantID)

	appID, appSecret := s.resolveModelAppCredentials(ctx, &model.Parameters)

	embedder, err := embedding.NewEmbedder(embedding.ConfigFromModel(model, appID, appSecret), s.pooler, s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
			"tenant_id":  tenantID,
		})
		return nil, err
	}

	logger.Info(ctx, "Cross-tenant embedding model initialized successfully")
	return embedder, nil
}

// GetRerankModel retrieves and initializes a reranking model instance
// Takes a model ID and returns a Reranker interface implementation
func (s *modelService) GetRerankModel(ctx context.Context, modelId string) (rerank.Reranker, error) {
	// Get the model details
	model, err := s.GetModelByID(ctx, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": modelId,
		})
		return nil, err
	}

	logger.Infof(ctx, "Getting rerank model: %s, source: %s", model.Name, model.Source)

	appID, appSecret := s.resolveModelAppCredentials(ctx, &model.Parameters)

	reranker, err := rerank.NewReranker(rerank.ConfigFromModel(model, appID, appSecret))
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	logger.Info(ctx, "Rerank model initialized successfully")
	return reranker, nil
}

// GetChatModel retrieves and initializes a chat model instance
// Takes a model ID and returns a Chat interface implementation
func (s *modelService) GetChatModel(ctx context.Context, modelId string) (chat.Chat, error) {
	// Check if model ID is empty
	if modelId == "" {
		logger.Error(ctx, "Model ID is empty")
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	// Get the model directly from repository to avoid status checks
	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		logger.Error(ctx, "Chat model not found")
		return nil, ErrModelNotFound
	}

	logger.Infof(ctx, "Getting chat model: %s, source: %s", model.Name, model.Source)

	appID, appSecret := s.resolveModelAppCredentials(ctx, &model.Parameters)

	chatModel, err := chat.NewChat(chat.ConfigFromModel(model, appID, appSecret), s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	return chatModel, nil
}

// GetVLMModel retrieves and initializes a vision language model instance.
func (s *modelService) GetVLMModel(ctx context.Context, modelId string) (vlm.VLM, error) {
	if modelId == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		return nil, ErrModelNotFound
	}

	logger.Infof(ctx, "Getting VLM model: %s, source: %s", model.Name, model.Source)

	appID, appSecret := s.resolveModelAppCredentials(ctx, &model.Parameters)

	vlmModel, err := vlm.NewVLM(vlm.ConfigFromModel(model, appID, appSecret), s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	return vlmModel, nil
}

// Note: default model selection logic has been removed; models no longer
// maintain a per-type default flag at the service layer.

// GetASRModel retrieves and initializes an automatic speech recognition model instance.
func (s *modelService) GetASRModel(ctx context.Context, modelId string) (asr.ASR, error) {
	if modelId == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		return nil, ErrModelNotFound
	}

	logger.Infof(ctx, "Getting ASR model: %s, source: %s", model.Name, model.Source)

	sttModel, err := asr.NewASR(asr.ConfigFromModel(model))
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	return sttModel, nil
}

func formatModelInUseMessage(kbCount int64, _ ...bool) string {
	return fmt.Sprintf(
		"model is used by %d knowledge base(s); reconfigure or remove those references before deleting",
		kbCount,
	)
}
