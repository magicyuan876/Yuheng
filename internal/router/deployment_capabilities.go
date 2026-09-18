package router

import (
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
)

func deploymentCapabilitiesFromRouter(params RouterParams) handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		Organizations: params.OrganizationHandler != nil,
		WebSearch:     params.WebSearchHandler != nil && params.WebSearchProviderHandler != nil && params.WebSearchCredentialsHandler != nil,
		VectorStore:   params.VectorStoreHandler != nil,
		Storage:       params.StorageBackendHandler != nil,
		Docs:          params.DocsModule != nil && params.DocsModule.Enabled,
		DocsCollabURL: docsCollabURL(params.DocsModule),
	}
}

// docsCollabURL exposes the browser-facing collaboration WebSocket address
// only while the module is on and a collaboration service is actually
// configured. A deployment that runs without one reports none, and the
// editor falls back to exclusive editing under a lease.
func docsCollabURL(m *docs.Module) string {
	if m == nil || !m.Enabled || m.Config == nil || !m.Config.CollabEnabled() {
		return ""
	}
	return m.Config.CollabURL
}
