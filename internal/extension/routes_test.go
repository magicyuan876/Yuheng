package extension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

// toggleFeatures lets a test flip a feature between requests.
type toggleFeatures struct{ status FeatureStatus }

func (t *toggleFeatures) Status(Feature) FeatureStatus { return t.status }
func (t *toggleFeatures) All() map[Feature]FeatureStatus {
	return map[Feature]FeatureStatus{"acme": t.status}
}

func serveGated(features Features) (*httptest.ResponseRecorder, *bool) {
	gin.SetMode(gin.TestMode)
	called := false
	engine := gin.New()
	engine.GET("/x", RequireFeature(features, "acme"), func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "ok")
	})
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w, &called
}

func TestRequireFeatureAllowsAnEnabledFeature(t *testing.T) {
	w, called := serveGated(StaticFeatures{"acme": {Enabled: true}})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, *called)
}

func TestRequireFeatureBlocksWithAStableBody(t *testing.T) {
	w, called := serveGated(StaticFeatures{"acme": {Enabled: false, Reason: "license_expired"}})
	require.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, *called, "the handler must not run")

	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details struct {
				Feature string `json:"feature"`
				Reason  string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, "feature_disabled", body.Error.Code)
	assert.Equal(t, FeatureDisabledCode, body.Error.Code)
	assert.Equal(t, "acme", body.Error.Details.Feature)
	assert.Equal(t, "license_expired", body.Error.Details.Reason)
	assert.NotEmpty(t, body.Error.Message)
}

func TestRequireFeatureTreatsAnUnregisteredFeatureAndNilRegistryAsDisabled(t *testing.T) {
	w, called := serveGated(StaticFeatures{})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, *called)

	w, called = serveGated(nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, *called)
}

func TestRequireFeatureReadsTheStatusOnEveryRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tf := &toggleFeatures{}
	engine := gin.New()
	engine.GET("/x", RequireFeature(tf, "acme"), func(c *gin.Context) { c.Status(http.StatusOK) })
	get := func() int {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, get())
	tf.status = FeatureStatus{Enabled: true}
	assert.Equal(t, http.StatusOK, get())
	tf.status = FeatureStatus{Reason: "license_expired"}
	assert.Equal(t, http.StatusForbidden, get())
}

func TestAPIKeyPolicyConstructors(t *testing.T) {
	assert.False(t, APIKeyPolicy{}.Declared(), "the zero policy admits no key")
	assert.True(t, APIKeyAnyKey().Any())
	assert.True(t, APIKeyFullAccess().Declared())
	assert.False(t, APIKeyFullAccess().Any())

	caps := []types.APIKeyCapability{types.APIKeyCapabilityIngest}
	p := APIKeyCapabilities(caps...)
	caps[0] = types.APIKeyCapabilityChat
	assert.Equal(t, []types.APIKeyCapability{types.APIKeyCapabilityIngest}, p.Capabilities(),
		"the policy keeps its own copy of the capabilities")
}
