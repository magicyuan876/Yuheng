package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/extension"
)

func TestDeploymentCapabilityKeysMatchFrontend(t *testing.T) {
	frontendKeys, err := readFrontendDeploymentCapabilityKeys()
	if err != nil {
		t.Fatalf("read frontend capability keys: %v", err)
	}

	if !slices.Equal(DeploymentCapabilityKeys, frontendKeys) {
		t.Fatalf("backend keys = %#v, frontend keys = %#v", DeploymentCapabilityKeys, frontendKeys)
	}
}

func TestBuildDeploymentCapabilitiesIncludesAllKeys(t *testing.T) {
	result := BuildDeploymentCapabilities(DeploymentFeatureAvailability{
		WebSearch:   true,
		VectorStore: true,
		Storage:     true,
		Docs:        true,
	})

	for _, key := range DeploymentCapabilityKeys {
		if _, ok := result.Capabilities[key]; !ok {
			t.Fatalf("missing capability key %q", key)
		}
	}
}

func readFrontendDeploymentCapabilityKeys() ([]string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, os.ErrInvalid
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	frontendPath := filepath.Join(repoRoot, "frontend", "src", "config", "deploymentCapabilities.ts")
	content, err := os.ReadFile(frontendPath)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?s)export const DEPLOYMENT_CAPABILITY_KEYS = \[(.*?)\]`)
	match := re.FindSubmatch(content)
	if len(match) < 2 {
		return nil, os.ErrInvalid
	}

	// Take the string literals themselves rather than trimming lines. Trimming
	// broke twice on formatting alone: a Windows checkout's "\r" line endings
	// kept the trailing comma, and when Prettier moved the file from single to
	// double quotes every key came back still wrapped in quotes. A literal is
	// either quote style; line breaks, commas and the `as const` around it
	// no longer matter.
	literal := regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
	var keys []string
	for _, m := range literal.FindAllSubmatch(match[1], -1) {
		keys = append(keys, string(m[1])+string(m[2]))
	}
	return keys, nil
}

// getCapabilities calls the endpoint and decodes the data envelope.
func getCapabilities(t *testing.T, h *SystemHandler) map[string]json.RawMessage {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/system/capabilities", nil)

	h.GetDeploymentCapabilities(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func TestCapabilitiesOmitExtensionsWhenThereAreNone(t *testing.T) {
	h := &SystemHandler{}
	h.BindDeploymentCapabilities(BuildDeploymentCapabilities(DeploymentFeatureAvailability{Docs: true}))

	defaultFeatures, err := extension.NewFeatures()
	require.NoError(t, err)
	for name, features := range map[string]extension.Features{
		"never bound":      nil,
		"default registry": defaultFeatures,
	} {
		t.Run(name, func(t *testing.T) {
			h.BindExtensionFeatures(features)

			data := getCapabilities(t, h)

			assert.Contains(t, data, "capabilities")
			assert.NotContains(t, data, "extensions", "a build without extensions reports none")
		})
	}
}

func TestCapabilitiesReportExtensionFeatures(t *testing.T) {
	h := &SystemHandler{}
	h.BindDeploymentCapabilities(BuildDeploymentCapabilities(DeploymentFeatureAvailability{}))
	h.BindExtensionFeatures(extension.StaticFeatures{
		"acme.on":  {Enabled: true},
		"acme.off": {Reason: "needs_something"},
	})

	data := getCapabilities(t, h)

	var extensions map[string]DeploymentCapability
	require.NoError(t, json.Unmarshal(data["extensions"], &extensions))
	assert.Equal(t, map[string]DeploymentCapability{
		"acme.on":  {Supported: true},
		"acme.off": {Supported: false, Reason: "needs_something"},
	}, extensions)
	assert.NotContains(t, string(data["capabilities"]), "acme",
		"extension features stay out of the fixed key list the frontend is tested against")
}

// The snapshot of the built-in capabilities is taken once; an extension's
// status is read again on every request, because it can change at runtime.
func TestExtensionStatusIsReadOnEveryRequest(t *testing.T) {
	h := &SystemHandler{}
	h.BindDeploymentCapabilities(BuildDeploymentCapabilities(DeploymentFeatureAvailability{}))
	live := &switchableFeatures{}
	h.BindExtensionFeatures(live)

	live.on.Store(false)
	before := getCapabilities(t, h)
	live.on.Store(true)
	after := getCapabilities(t, h)

	assert.JSONEq(t, `{"acme.widget":{"supported":false,"reason":"switched_off"}}`, string(before["extensions"]))
	assert.JSONEq(t, `{"acme.widget":{"supported":true}}`, string(after["extensions"]))
}

type switchableFeatures struct{ on atomic.Bool }

func (s *switchableFeatures) Status(f extension.Feature) extension.FeatureStatus {
	return s.All()[f]
}

func (s *switchableFeatures) All() map[extension.Feature]extension.FeatureStatus {
	if s.on.Load() {
		return map[extension.Feature]extension.FeatureStatus{"acme.widget": {Enabled: true}}
	}
	return map[extension.Feature]extension.FeatureStatus{"acme.widget": {Reason: "switched_off"}}
}
