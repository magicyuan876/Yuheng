package router

import "github.com/magicyuan876/yuheng/internal/handler"

func deploymentCapabilitiesFromRouter(params RouterParams) handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		Organizations: params.OrganizationHandler != nil,
		WebSearch:     params.WebSearchHandler != nil && params.WebSearchProviderHandler != nil && params.WebSearchCredentialsHandler != nil,
		VectorStore:   params.VectorStoreHandler != nil,
		Storage:       params.StorageBackendHandler != nil,
	}
}
