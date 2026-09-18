package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/handler"
)

func allDeploymentFeaturesAvailable() handler.DeploymentFeatureAvailability {
	return handler.DeploymentFeatureAvailability{
		Organizations: true,
		WebSearch:     true,
		VectorStore:   true,
		Storage:       true,
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
	if !result.Capabilities["settings.storage"].Supported {
		t.Fatal("storage should remain supported in lite edition")
	}
}

func TestBuildDeploymentCapabilitiesReflectsMissingRoutes(t *testing.T) {
	available := allDeploymentFeaturesAvailable()
	available.WebSearch = false

	result := handler.BuildDeploymentCapabilities("standard", available)

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
	if !body.Data.Capabilities["settings.websearch"].Supported {
		t.Fatal("websearch capability should be returned")
	}
}
