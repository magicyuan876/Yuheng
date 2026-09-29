package router

import (
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
)

func deploymentCapabilitiesFromRouter(params RouterParams) handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		Organizations: params.OrganizationHandler != nil,
		WebSearch: params.WebSearchHandler != nil &&
			params.WebSearchProviderHandler != nil &&
			params.WebSearchCredentialsHandler != nil,
		// Registering a store only makes sense when some engine can be registered;
		// the community edition ships none (its engine is the application's own
		// database), and an extension adds them.
		VectorStore:       params.VectorStoreHandler != nil && len(params.EngineCatalog.Registrable()) > 0,
		Storage:           params.StorageBackendHandler != nil,
		Docs:              params.DocsModule != nil && params.DocsModule.Enabled,
		DocsCollabURL:     docsCollabURL(params.DocsModule),
		DocsPublicSharing: docsPublicSharing(params.DocsModule),
	}
}

// docsPublicSharing reports whether this deployment allows pages to be
// published to anonymous URLs. Off unless YUHENG_DOCS_PUBLIC_SHARING is set;
// clients use it to hide the control rather than offer one that can only be
// refused.
func docsPublicSharing(m *docs.Module) bool {
	return m != nil && m.Enabled && m.Config != nil && m.Config.PublicSharing
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
