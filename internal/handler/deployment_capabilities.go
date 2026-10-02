package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/extension"
)

// DeploymentCapabilityKeys is the canonical capability key list shared with
// frontend/src/config/deploymentCapabilities.ts — keep both in sync.
var DeploymentCapabilityKeys = []string{
	"settings.websearch",
	"settings.vectorstore",
	"settings.storage",
	"docs",
	"docs.public_sharing",
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
	// Extensions reports the features that extensions have added, keyed by the
	// names the extensions chose. Unlike Capabilities, a key that is absent
	// means the feature does not exist here, so clients must treat absence as
	// "not available". Empty (and omitted) when no extension is installed.
	Extensions map[string]DeploymentCapability `json:"extensions,omitempty"`
}

// DeploymentFeatureAvailability mirrors injected backend handlers/services.
type DeploymentFeatureAvailability struct {
	WebSearch   bool
	VectorStore bool
	Storage     bool
	Docs        bool
	// DocsCollabURL is passed through verbatim into DeploymentCapabilitiesData.
	DocsCollabURL string
	// DocsPublicSharing reports whether pages may be published to anonymous
	// URLs. Clients use it to decide whether to offer the control at all,
	// rather than offering one whose only outcome is a refusal.
	DocsPublicSharing bool
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
			"settings.websearch":   supportedDeploymentCapability(available.WebSearch),
			"settings.vectorstore": supportedDeploymentCapability(available.VectorStore),
			"settings.storage":     supportedDeploymentCapability(available.Storage),
			"docs":                 supportedDeploymentCapability(available.Docs),
			"docs.public_sharing":  supportedDeploymentCapability(available.DocsPublicSharing),
		},
		DocsCollabURL: available.DocsCollabURL,
	}
}

// BindDeploymentCapabilities stores the startup snapshot used by GetDeploymentCapabilities.
func (h *SystemHandler) BindDeploymentCapabilities(data DeploymentCapabilitiesData) {
	h.deploymentCapabilities = data
}

// BindExtensionFeatures sets the registry GetDeploymentCapabilities consults for
// extension features. Optional: without it the response carries none.
func (h *SystemHandler) BindExtensionFeatures(features extension.Features) {
	h.extensionFeatures = features
}

// extensionCapabilities projects the extension registry onto the response.
// Unlike the rest of the capability list it is read on every request: what an
// extension reports can change while the server runs, and a snapshot taken at
// start-up would go on showing a feature that has since been switched off.
func extensionCapabilities(features extension.Features) map[string]DeploymentCapability {
	if features == nil {
		return nil
	}
	all := features.All()
	if len(all) == 0 {
		return nil
	}
	out := make(map[string]DeploymentCapability, len(all))
	for name, status := range all {
		out[string(name)] = DeploymentCapability{Supported: status.Enabled, Reason: status.Reason}
	}
	return out
}

// GetDeploymentCapabilities godoc
// @Summary      获取部署能力清单
// @Description  返回当前部署版本及实际注册的后端路由所对应的功能能力；仅 supported=false 表示入口应隐藏
// @Tags         系统
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "标准 code/msg/data 包装，data 为 DeploymentCapabilitiesData"
// @Router       /system/capabilities [get]
func (h *SystemHandler) GetDeploymentCapabilities(c *gin.Context) {
	data := h.deploymentCapabilities
	data.Extensions = extensionCapabilities(h.extensionFeatures)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}
