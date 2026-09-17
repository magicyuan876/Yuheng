package dto

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// WebSearchProviderResponse mirrors types.WebSearchProviderEntity for
// response bodies, with the APIKey field removed by construction. Credential
// presence is exposed via the /credentials subresource.
type WebSearchProviderResponse struct {
	ID          string                         `json:"id"`
	TenantID    uint64                         `json:"tenant_id"`
	Name        string                         `json:"name"`
	Provider    types.WebSearchProviderType    `json:"provider"`
	Description string                         `json:"description"`
	Parameters  WebSearchProviderParametersDTO `json:"parameters"`
	IsDefault   bool                           `json:"is_default"`
	// IsBuiltin marks a platform-shared provider: available to every
	// workspace, configurable only by system administrators. The UI uses it to
	// render the shared badge and to hide the edit affordances.
	IsBuiltin bool      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Per-field "configured?" map. See MCPServiceResponse.Credentials.
	// Omitted for platform-shared rows unless the caller administers the
	// platform — whether the operator configured a key is their business.
	Credentials map[string]CredentialFieldMetadata `json:"credentials,omitempty"`
}

// WebSearchProviderParametersDTO holds every parameter field except APIKey.
// EngineID, BaseURL, ProxyURL and ExtraConfig are not secrets (they describe
// where to send the request, not how to authenticate) and remain visible.
type WebSearchProviderParametersDTO struct {
	EngineID    string            `json:"engine_id,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	ProxyURL    string            `json:"proxy_url,omitempty"`
	ExtraConfig map[string]string `json:"extra_config,omitempty"`
}

// NewWebSearchProviderResponse converts a stored entity into its response shape.
func NewWebSearchProviderResponse(ctx context.Context, e *types.WebSearchProviderEntity) *WebSearchProviderResponse {
	if e == nil {
		return nil
	}
	params := WebSearchProviderParametersDTO{
		EngineID:    e.Parameters.EngineID,
		BaseURL:     e.Parameters.BaseURL,
		ProxyURL:    e.Parameters.ProxyURL,
		ExtraConfig: e.Parameters.ExtraConfig,
	}
	if !CanViewIntegrationSecrets(ctx) {
		params.ProxyURL = ""
		params.ExtraConfig = nil
	}
	sharedDetail := CanSeeSharedInfraDetail(ctx)
	if e.IsBuiltin && !sharedDetail {
		// Platform-shared: strip everything describing how the operator
		// configured the upstream. Provider and name stay — they are the
		// capability surface a workspace picks from.
		params.BaseURL = ""
		params.ProxyURL = ""
		params.ExtraConfig = nil
		params.EngineID = ""
	}
	credentials := map[string]CredentialFieldMetadata{
		"api_key": {Configured: e.Parameters.APIKey != ""},
	}
	if e.IsBuiltin && !sharedDetail {
		credentials = nil
	}
	return &WebSearchProviderResponse{
		ID:          e.ID,
		TenantID:    e.TenantID,
		Name:        e.Name,
		Provider:    e.Provider,
		Description: e.Description,
		Parameters:  params,
		IsDefault:   e.IsDefault,
		IsBuiltin:   e.IsBuiltin,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Credentials: credentials,
	}
}

func NewWebSearchProviderResponses(ctx context.Context, es []*types.WebSearchProviderEntity) []*WebSearchProviderResponse {
	out := make([]*WebSearchProviderResponse, 0, len(es))
	for _, e := range es {
		out = append(out, NewWebSearchProviderResponse(ctx, e))
	}
	return out
}
