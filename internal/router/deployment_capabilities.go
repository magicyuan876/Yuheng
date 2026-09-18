package router

import (
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
)

func deploymentCapabilitiesFromRouter(params RouterParams) handler.DeploymentCapabilitiesData {
	mcp := params.MCPServiceHandler != nil &&
		params.MCPCredentialsHandler != nil &&
		params.MCPOAuthHandler != nil
	webSearch := params.WebSearchHandler != nil &&
		params.WebSearchProviderHandler != nil &&
		params.WebSearchCredentialsHandler != nil
	return handler.BuildDeploymentCapabilities(handler.Edition, handler.DeploymentFeatureAvailability{
		Organizations: params.OrganizationHandler != nil,
		Agents:        params.CustomAgentHandler != nil,
		IM:            params.IMHandler != nil,
		// Match RegisterEmbedChannelRoutes: management routes depend on handler only.
		Embed:         params.EmbedChannelHandler != nil,
		API:           params.TenantHandler != nil && params.TenantAPIKeyService != nil,
		MCP:           mcp,
		WebSearch:     webSearch,
		VectorStore:   params.VectorStoreHandler != nil,
		Storage:       params.StorageBackendHandler != nil,
		Sandbox:       params.SandboxConfigHandler != nil,
		Docs:          params.DocsModule != nil && params.DocsModule.Enabled,
		DocsCollabURL: docsCollabURL(params.DocsModule),
	})
}

// docsCollabURL exposes the browser-facing collaboration WebSocket address
// only while the module is on and a collaboration service is actually
// configured; an exclusive-edit deployment (T1.5) reports none.
func docsCollabURL(m *docs.Module) string {
	if m == nil || !m.Enabled || m.Config == nil || !m.Config.CollabEnabled() {
		return ""
	}
	return m.Config.CollabURL
}
