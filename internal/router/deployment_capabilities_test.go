package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allDeploymentFeaturesAvailable() handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		WebSearch:   true,
		VectorStore: true,
		Storage:     true,
		Docs:        true,
	}
}

func TestBuildDeploymentCapabilitiesReflectsMissingRoutes(t *testing.T) {
	available := allDeploymentFeaturesAvailable()
	available.WebSearch = false

	result := handler.BuildDeploymentCapabilities(available)

	for _, key := range []string{"settings.websearch"} {
		capability := result.Capabilities[key]
		if capability.Supported {
			t.Fatalf("%s should be unsupported", key)
		}
		if capability.Reason != "route_not_registered" {
			t.Fatalf("%s reason = %q, want route_not_registered", key, capability.Reason)
		}
	}
	if !result.Capabilities["settings.storage"].Supported {
		t.Fatal("an available route should remain supported")
	}
}

func TestDocsCollabURLIsGatedOnTheModuleAndConfig(t *testing.T) {
	off := deploymentCapabilitiesFromRouter(RouterParams{})
	if off.DocsCollabURL != "" {
		t.Fatalf("DocsCollabURL = %q, want empty when the module is absent", off.DocsCollabURL)
	}

	disabled := deploymentCapabilitiesFromRouter(RouterParams{DocsModule: &docs.Module{Enabled: false}})
	if disabled.DocsCollabURL != "" {
		t.Fatalf("DocsCollabURL = %q, want empty when the module is disabled", disabled.DocsCollabURL)
	}

	exclusive := deploymentCapabilitiesFromRouter(RouterParams{
		DocsModule: &docs.Module{Enabled: true, Config: &config.DocsConfig{Enabled: true}},
	})
	if exclusive.DocsCollabURL != "" {
		t.Fatalf("DocsCollabURL = %q, want empty when no collaboration service is configured", exclusive.DocsCollabURL)
	}

	live := deploymentCapabilitiesFromRouter(RouterParams{
		DocsModule: &docs.Module{
			Enabled: true,
			Config:  &config.DocsConfig{Enabled: true, CollabURL: "ws://collab:1234"},
		},
	})
	if live.DocsCollabURL != "ws://collab:1234" {
		t.Fatalf("DocsCollabURL = %q, want ws://collab:1234", live.DocsCollabURL)
	}
	// The function now reports raw availability; the capability map is built
	// from it one layer up.
	if !handler.BuildDeploymentCapabilities(live).Capabilities["docs"].Supported {
		t.Fatal("docs should be supported when the module is enabled")
	}
}

func TestGetDeploymentCapabilitiesHandlerReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	want := handler.BuildDeploymentCapabilities(allDeploymentFeaturesAvailable())
	systemHandler := &handler.SystemHandler{}
	systemHandler.BindDeploymentCapabilities(want)
	engine.GET("/capabilities", systemHandler.GetDeploymentCapabilities)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body struct {
		Code int                                `json:"code"`
		Data handler.DeploymentCapabilitiesData `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("response = %#v", body)
	}
	if !body.Data.Capabilities["settings.websearch"].Supported {
		t.Fatal("websearch capability should be returned")
	}
}

// Registering a vector store needs an engine to register. The community
// edition ships none, so it must not advertise the screen.
func TestVectorStoreCapabilityFollowsRegistrableEngines(t *testing.T) {
	registrable := retriever.EngineDescriptor{
		Type: "registrable", Driver: "registrable", DisplayName: "Registrable",
		Capabilities:   retriever.EngineCapabilities{Retrievers: []types.RetrieverType{types.VectorRetrieverType}},
		Registrable:    true,
		EnvStore:       func(types.EnvLookupFunc) *types.VectorStore { return nil },
		DialAddresses:  func(types.ConnectionConfig) []string { return nil },
		TestConnection: func(context.Context, types.ConnectionConfig) (string, error) { return "", nil },
		New: func(context.Context, types.VectorStore, retriever.EngineDeps) (interfaces.RetrieveEngineService, error) {
			return nil, nil
		},
	}
	builtInOnly, err := retriever.NewCatalog(retriever.PostgresDescriptor())
	require.NoError(t, err)
	withEngine, err := retriever.NewCatalog(retriever.PostgresDescriptor(), registrable)
	require.NoError(t, err)

	base := RouterParams{VectorStoreHandler: &handler.VectorStoreHandler{}}

	assert.False(t, deploymentCapabilitiesFromRouter(base).VectorStore, "no catalog")

	base.EngineCatalog = builtInOnly
	assert.False(t, deploymentCapabilitiesFromRouter(base).VectorStore, "only the built-in engine")

	base.EngineCatalog = withEngine
	assert.True(t, deploymentCapabilitiesFromRouter(base).VectorStore, "an engine can be registered")

	base.VectorStoreHandler = nil
	assert.False(t, deploymentCapabilitiesFromRouter(base).VectorStore, "no handler, no screen")
}
