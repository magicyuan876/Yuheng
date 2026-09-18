package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DeploymentCapabilityKeys is the canonical capability key list shared with
// frontend/src/config/deploymentCapabilities.ts — keep both in sync.
var DeploymentCapabilityKeys = []string{
	"organizations",
	"settings.websearch",
	"settings.vectorstore",
	"settings.storage",
	"docs",
}

// DeploymentCapability describes whether a deployment exposes a feature route.
type DeploymentCapability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

// DeploymentCapabilitiesData is returned by GET /system/capabilities.
type DeploymentCapabilitiesData struct {
	Capabilities map[string]DeploymentCapability `json:"capabilities"`
	// DocsCollabURL is the browser-facing WebSocket address of the docs
	// collaboration service (e.g. "ws://collab:1234"); empty when the docs
	// module is disabled or no collaboration service is configured. The
	// editor then has no realtime provider and falls back to exclusive
	// editing under a lease (see internal/docs/service/lease.go).
	DocsCollabURL string `json:"docs_collab_url,omitempty"`
}

// DeploymentFeatureAvailability mirrors injected backend handlers/services.
type DeploymentFeatureAvailability struct {
	Organizations bool
	WebSearch     bool
	VectorStore   bool
	Storage       bool
	Docs          bool
	// DocsCollabURL is passed through verbatim into DeploymentCapabilitiesData.
	DocsCollabURL string
}

func supportedDeploymentCapability(supported bool) DeploymentCapability {
	if supported {
		return DeploymentCapability{Supported: true}
	}
	return DeploymentCapability{Supported: false, Reason: "route_not_registered"}
}

// BuildDeploymentCapabilities derives the deployment capability snapshot.
func BuildDeploymentCapabilities(
	available DeploymentFeatureAvailability,
) DeploymentCapabilitiesData {
	return DeploymentCapabilitiesData{
		Capabilities: map[string]DeploymentCapability{
			"organizations":        supportedDeploymentCapability(available.Organizations),
			"settings.websearch":   supportedDeploymentCapability(available.WebSearch),
			"settings.vectorstore": supportedDeploymentCapability(available.VectorStore),
			"settings.storage":     supportedDeploymentCapability(available.Storage),
			"docs":                 supportedDeploymentCapability(available.Docs),
		},
		DocsCollabURL: available.DocsCollabURL,
	}
}

// BindDeploymentCapabilities stores the startup snapshot used by GetDeploymentCapabilities.
func (h *SystemHandler) BindDeploymentCapabilities(data DeploymentCapabilitiesData) {
	h.deploymentCapabilities = data
}

// GetDeploymentCapabilities godoc
// @Summary      获取部署能力清单
// @Description  返回当前部署版本及实际注册的后端路由所对应的功能能力；仅 supported=false 表示入口应隐藏
// @Tags         系统
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "标准 code/msg/data 包装，data 为 DeploymentCapabilitiesData"
// @Router       /system/capabilities [get]
func (h *SystemHandler) GetDeploymentCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": h.deploymentCapabilities,
	})
}
