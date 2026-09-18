package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/handler"
)

func allDeploymentFeaturesAvailable() handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		Organizations: true,
		Agents:        true,
		IM:            true,
		Embed:         true,
		API:           true,
		MCP:           true,
		WebSearch:     true,
		VectorStore:   true,
		Storage:       true,
		Sandbox:       true,
		Docs:          true,
	}
}

func TestBuildDeploymentCapabilitiesHidesOrganizationsInLite(t *testing.T) {
	result := handler.BuildDeploymentCapabilities("lite", allDeploymentFeaturesAvailable())

	organization := result.Capabilities["organizations"]
	if organization.Supported {
		t.Fatal("organizations should be unsupported in lite edition")
	}
	if organization.Reason != "not_supported_in_lite" {
		t.Fatalf("organization reason = %q, want not_supported_in_lite", organization.Reason)
	}
	if !result.Capabilities["agents"].Supported {
		t.Fatal("agents should remain supported in lite edition")
	}
}

func TestBuildDeploymentCapabilitiesReflectsMissingRoutes(t *testing.T) {
	available := allDeploymentFeaturesAvailable()
	available.Embed = false
	available.MCP = false

	result := handler.BuildDeploymentCapabilities("standard", available)

	for _, key := range []string{"integrations.embed", "settings.mcp"} {
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
	if !live.Capabilities["docs"].Supported {
		t.Fatal("docs should be supported when the module is enabled")
	}
}

func TestGetDeploymentCapabilitiesHandlerReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	want := handler.BuildDeploymentCapabilities("standard", allDeploymentFeaturesAvailable())
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
	if body.Code != 0 || body.Data.Edition != "standard" {
		t.Fatalf("response = %#v", body)
	}
	if !body.Data.Capabilities["integrations.embed"].Supported {
		t.Fatal("embed capability should be returned")
	}
}
