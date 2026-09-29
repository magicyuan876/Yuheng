package asr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

// unwrapASRForTest strips the optional Langfuse decorator so tests can
// assert on the concrete adapter type.
func unwrapASRForTest(a ASR) ASR {
	if wrapped, ok := a.(*langfuseASR); ok {
		return wrapped.inner
	}
	return a
}

func TestDashScopeEndpointFromBaseURL(t *testing.T) {
	cases := map[string]string{
		"": "https://dashscope.aliyuncs.com" + dashScopeMultimodalPath,
		"https://dashscope.aliyuncs.com/compatible-mode/v1": "https://dashscope.aliyuncs.com" + dashScopeMultimodalPath,
		"https://dashscope.aliyuncs.com":                    "https://dashscope.aliyuncs.com" + dashScopeMultimodalPath,
		"https://dashscope.aliyuncs.com/api/v1":             "https://dashscope.aliyuncs.com" + dashScopeMultimodalPath,
	}
	for in, want := range cases {
		got, err := dashScopeEndpointFromBaseURL(in)
		require.NoErrorf(t, err, "input=%q", in)
		assert.Equalf(t, want, got, "input=%q", in)
	}

	_, err := dashScopeEndpointFromBaseURL("not a url")
	assert.Error(t, err)
}

func TestNewASRRoutesAliyunToDashScope(t *testing.T) {
	// Only routing is under test, so the SSRF check must not depend on what the
	// network's resolver says. Validating a public hostname resolves it, and a
	// resolver that answers with a tunnelling or private address (some networks
	// rewrite DNS answers) makes an unrelated test fail.
	withASRSSRFWhitelist(t, "dashscope.aliyuncs.com,api.openai.com")

	// The UI stores the vendor in Parameters.Provider while Source stays
	// "remote"; older hand-configured models may carry only the BaseURL.
	// All three shapes must route to the DashScope adapter.
	cases := []Config{
		{
			Provider: "aliyun", Source: types.ModelSourceRemote,
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
		{
			Source:  types.ModelSourceAliyun,
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
		{
			Source:  types.ModelSourceRemote,
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		},
	}
	for i, cfg := range cases {
		cfg.ModelName = "qwen3-asr-flash"
		cfg.APIKey = "sk-test"
		a, err := NewASR(&cfg)
		require.NoErrorf(t, err, "case %d", i)
		_, isAliyun := unwrapASRForTest(a).(*AliyunASR)
		assert.Truef(t, isAliyun, "case %d should route to AliyunASR", i)
	}

	// An explicit non-aliyun provider on a custom endpoint stays on the
	// OpenAI-compatible path even if the URL looks unrelated.
	a, err := NewASR(&Config{
		Provider:  "openai",
		ModelName: "whisper-1",
		APIKey:    "sk-test",
		BaseURL:   "https://api.openai.com/v1",
	})
	require.NoError(t, err)
	_, isAliyun := unwrapASRForTest(a).(*AliyunASR)
	assert.False(t, isAliyun)
}

func TestAliyunASRTranscribe(t *testing.T) {
	withASRSSRFWhitelist(t, "127.0.0.1")
	var gotPath, gotAuth string
	var gotBody dashScopeASRRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"content": []map[string]any{{"text": "大家好，"}, {"text": "这是转写结果。"}},
					},
				}},
			},
			"request_id": "req-1",
		})
	}))
	defer server.Close()

	asrModel, err := NewAliyunASR(&Config{
		ModelName: "qwen3-asr-flash",
		APIKey:    "sk-test",
		BaseURL:   server.URL,
		Language:  "zh",
	})
	require.NoError(t, err)

	result, err := asrModel.Transcribe(context.Background(), []byte("fake-mp3"), "track.mp3")
	require.NoError(t, err)
	assert.Equal(t, "大家好，这是转写结果。", result.Text)
	assert.Empty(t, result.Segments, "DashScope sync API returns no timestamps")

	assert.Equal(t, dashScopeMultimodalPath, gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
	assert.Equal(t, "qwen3-asr-flash", gotBody.Model)
	require.Len(t, gotBody.Input.Messages, 1)
	assert.Contains(t, gotBody.Input.Messages[0].Content[0].Audio, "data:audio/mpeg;base64,")
	require.NotNil(t, gotBody.Parameters.ASROptions)
	assert.Equal(t, "zh", gotBody.Parameters.ASROptions.Language)
}

func TestAliyunASRTranscribeSurfacesAPIError(t *testing.T) {
	withASRSSRFWhitelist(t, "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    "InvalidParameter",
			"message": "model not found",
		})
	}))
	defer server.Close()

	asrModel, err := NewAliyunASR(&Config{
		ModelName: "bad-model",
		APIKey:    "sk-test",
		BaseURL:   server.URL,
	})
	require.NoError(t, err)

	_, err = asrModel.Transcribe(context.Background(), []byte("x"), "a.mp3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidParameter")
	assert.Contains(t, err.Error(), "model not found")
}
