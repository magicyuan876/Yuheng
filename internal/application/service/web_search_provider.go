package service

import (
	"context"
	"fmt"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	infra_web_search "github.com/magicyuan876/yuheng/internal/infrastructure/web_search"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// webSearchProviderService implements interfaces.WebSearchProviderService
type webSearchProviderService struct {
	repo interfaces.WebSearchProviderRepository
}

// NewWebSearchProviderService creates a new web search provider service
func NewWebSearchProviderService(repo interfaces.WebSearchProviderRepository) interfaces.WebSearchProviderService {
	return &webSearchProviderService{repo: repo}
}

// CreateProvider creates a new web search provider configuration.
func (s *webSearchProviderService) CreateProvider(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	if provider.TenantID == 0 {
		return fmt.Errorf("tenant ID is required")
	}

	if !isValidProviderType(provider.Provider) {
		return fmt.Errorf("invalid provider type: %s", provider.Provider)
	}

	if err := validateProviderParameters(provider.Provider, provider.Parameters); err != nil {
		return err
	}

	if provider.IsDefault {
		if err := s.repo.ClearDefault(ctx, provider.TenantID, ""); err != nil {
			logger.Warnf(ctx, "Failed to clear default providers: %v", err)
		}
	}

	logger.Infof(ctx, "Creating web search provider: tenant=%d, name=%s, type=%s", provider.TenantID, provider.Name, provider.Provider)
	return s.repo.Create(ctx, provider)
}

// UpdateProvider updates an existing provider.
func (s *webSearchProviderService) UpdateProvider(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	if provider.TenantID == 0 {
		return fmt.Errorf("tenant ID is required")
	}

	// Validate provider type if set
	if provider.Provider != "" && !isValidProviderType(provider.Provider) {
		return fmt.Errorf("invalid provider type: %s", provider.Provider)
	}

	// A platform-shared provider is visible to every workspace, so only a
	// system administrator may repoint it, and the write has to be keyed on
	// the owning workspace — the repository predicate is (id, tenant_id).
	existing, err := s.repo.GetByID(ctx, provider.TenantID, provider.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("web search provider not found")
	}
	owner, err := authorizePlatformSharedWrite(
		ctx, "web search provider", existing.IsBuiltin, existing.TenantID,
	)
	if err != nil {
		return err
	}
	provider.TenantID = owner
	provider.IsBuiltin = existing.IsBuiltin

	if provider.IsDefault {
		if err := s.repo.ClearDefault(ctx, provider.TenantID, provider.ID); err != nil {
			logger.Warnf(ctx, "Failed to clear default providers: %v", err)
		}
	}

	if provider.Provider != "" {
		if err := validateProviderParameters(provider.Provider, provider.Parameters); err != nil {
			return err
		}
	}

	logger.Infof(ctx, "Updating web search provider: tenant=%d, id=%s", provider.TenantID, provider.ID)
	return s.repo.Update(ctx, provider)
}

// UpdateProviderCredentials writes the api_key credential field. Web search
// providers are stateless from our side — every search call rebuilds a
// transport from current Parameters — so no cache invalidation is required.
func (s *webSearchProviderService) UpdateProviderCredentials(
	ctx context.Context, tenantID uint64, id string, apiKey *string,
) (*types.WebSearchProviderEntity, error) {
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("web search provider not found")
	}
	if _, err := authorizePlatformSharedWrite(
		ctx, "web search provider", existing.IsBuiltin, existing.TenantID,
	); err != nil {
		return nil, err
	}

	if apiKey != nil && *apiKey != "" && *apiKey != existing.Parameters.APIKey {
		existing.Parameters.APIKey = *apiKey
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		logger.Infof(ctx, "WebSearch provider credentials updated: tenant=%d id=%s", tenantID, id)
	}
	return existing, nil
}

// ClearProviderCredential clears the api_key credential. Idempotent.
func (s *webSearchProviderService) ClearProviderCredential(
	ctx context.Context, tenantID uint64, id, field string,
) error {
	if field != "api_key" {
		return fmt.Errorf("unknown credential field: %s", field)
	}
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("web search provider not found")
	}
	if _, err := authorizePlatformSharedWrite(
		ctx, "web search provider", existing.IsBuiltin, existing.TenantID,
	); err != nil {
		return err
	}
	if existing.Parameters.APIKey == "" {
		return nil
	}
	existing.Parameters.APIKey = ""
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}
	logger.Infof(ctx, "WebSearch provider credential cleared by user: tenant=%d id=%s field=%s", tenantID, id, field)
	return nil
}

// DeleteProvider deletes a provider by tenant + id.
//
// A platform-shared provider must be un-shared first. Withdrawing sharing is
// where the "is anyone else still using it" question gets asked; deleting
// straight through would remove a provider other workspaces are actively
// searching with.
func (s *webSearchProviderService) DeleteProvider(ctx context.Context, tenantID uint64, id string) error {
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("web search provider not found")
	}
	if existing.IsBuiltin {
		if !types.IsSystemAdminFromContext(ctx) {
			return apperrors.NewForbiddenError(
				"only system administrators can delete platform-shared web search providers")
		}
		return apperrors.NewBadRequestError(
			"stop sharing this web search provider platform-wide before deleting it")
	}
	logger.Infof(ctx, "Deleting web search provider: tenant=%d, id=%s", tenantID, id)
	return s.repo.Delete(ctx, tenantID, id)
}

// SetProviderSharing publishes a web search provider to every workspace, or
// withdraws it. System administrators only.
//
// Unlike models and vector stores there is no reference guard: a workspace
// selects a provider per search (or via its own tenant default), and losing
// access degrades to "web search unavailable" rather than corrupting stored
// state. Workspaces that had pinned the shared provider as their default fall
// back through WebSearchProviderRepository.GetDefault.
func (s *webSearchProviderService) SetProviderSharing(
	ctx context.Context, id string, shared bool,
) (*types.WebSearchProviderEntity, error) {
	if err := authorizePlatformSharingChange(ctx, "web search provider"); err != nil {
		return nil, err
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("web search provider not found")
	}
	if existing.IsBuiltin == shared {
		return existing, nil // idempotent
	}
	existing.IsBuiltin = shared
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Web search provider %s platform sharing set to %v", id, shared)
	return existing, nil
}

// isValidProviderType checks if the given provider type is supported
func isValidProviderType(provider types.WebSearchProviderType) bool {
	switch provider {
	case types.WebSearchProviderTypeBing,
		types.WebSearchProviderTypeGoogle,
		types.WebSearchProviderTypeDuckDuckGo,
		types.WebSearchProviderTypeTavily,
		types.WebSearchProviderTypeOllama,
		types.WebSearchProviderTypeBaidu,
		types.WebSearchProviderTypeSearxng,
		types.WebSearchProviderTypeKeenable,
		types.WebSearchProviderTypeMetaso,
		types.WebSearchProviderTypeZhipu,
		types.WebSearchProviderTypeExa,
		types.WebSearchProviderTypeFirecrawl:
		return true
	default:
		return false
	}
}

// validateProviderParameters validates required parameters for each provider type
func validateProviderParameters(provider types.WebSearchProviderType, params types.WebSearchProviderParameters) error {
	switch provider {
	case types.WebSearchProviderTypeBing:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Bing provider")
		}
	case types.WebSearchProviderTypeGoogle:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Google provider")
		}
		if params.EngineID == "" {
			return fmt.Errorf("engine ID is required for Google provider")
		}
	case types.WebSearchProviderTypeTavily:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Tavily provider")
		}
	case types.WebSearchProviderTypeOllama:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Ollama provider")
		}
	case types.WebSearchProviderTypeBaidu:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Baidu provider")
		}
	case types.WebSearchProviderTypeExa:
		if params.APIKey == "" {
			return fmt.Errorf("API key is required for Exa provider")
		}
	case types.WebSearchProviderTypeZhipu:
		if err := infra_web_search.ValidateZhipuParameters(params); err != nil {
			return err
		}
	case types.WebSearchProviderTypeMetaso:
		if err := infra_web_search.ValidateMetasoParameters(params); err != nil {
			return err
		}
	case types.WebSearchProviderTypeFirecrawl:
		if err := infra_web_search.ValidateFirecrawlParameters(params); err != nil {
			return err
		}
	case types.WebSearchProviderTypeDuckDuckGo:
		// No API key required
	case types.WebSearchProviderTypeKeenable:
		// No API key required (keyless by default; an optional key lifts the rate limit)
	case types.WebSearchProviderTypeSearxng:
		if err := infra_web_search.ValidateSearxngBaseURL(params.BaseURL); err != nil {
			return err
		}
	}
	if err := validateOptionalProxyURL(params.ProxyURL); err != nil {
		return err
	}
	return nil
}

func validateOptionalProxyURL(proxyURL string) error {
	return infra_web_search.ValidateProxyURL(proxyURL)
}
