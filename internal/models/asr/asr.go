package asr

import (
	"context"
	"strings"

	"github.com/magicyuan876/yuheng/internal/models/provider"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Segment represents a transcribed segment with timestamps.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptionResult holds the full text and its segments.
type TranscriptionResult struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments,omitempty"`
}

// ASR defines the interface for Automatic Speech Recognition model operations.
type ASR interface {
	// Transcribe sends audio bytes to the ASR model and returns the transcribed text and segments.
	Transcribe(ctx context.Context, audioBytes []byte, fileName string) (*TranscriptionResult, error)

	GetModelName() string
	GetModelID() string
}

// Config holds the configuration needed to create an ASR instance.
type Config struct {
	Source    types.ModelSource
	BaseURL   string
	ModelName string
	APIKey    string
	ModelID   string
	Language  string // optional: specify language for transcription
	// Provider is the vendor identifier from Parameters.Provider (e.g.
	// "aliyun") — the UI stores the vendor here while Source stays
	// "remote"/"local", so adapter dispatch must consider both.
	Provider string
	// CustomHeaders 允许在调用远程 API 时附加自定义 HTTP 请求头（类似 OpenAI Python SDK 的 extra_headers）。
	CustomHeaders map[string]string
}

// ConfigFromModel 根据 types.Model 构造 asr.Config。
// 生产路径（从 DB 拉起）和测试连接路径（临时表单）共享这份映射。
// 当前 ASR 不涉及 app-id/app-secret 类凭证，所以签名不含 appID/appSecret。
func ConfigFromModel(m *types.Model) *Config {
	if m == nil {
		return nil
	}
	return &Config{
		ModelID:       m.ID,
		APIKey:        m.Parameters.APIKey,
		BaseURL:       m.Parameters.BaseURL,
		ModelName:     m.Name,
		Source:        m.Source,
		Provider:      m.Parameters.Provider,
		CustomHeaders: m.Parameters.CustomHeaders,
	}
}

// NewASR creates an ASR instance based on the provided configuration.
// Alibaba Cloud Model Studio (百炼/DashScope) has no OpenAI-compatible
// /audio/transcriptions endpoint, so its Qwen-ASR models go through the
// native DashScope adapter; every other vendor uses the OpenAI-compatible
// /v1/audio/transcriptions API.
//
// The vendor is resolved from (in order): the explicit Provider parameter,
// the model Source, and finally the BaseURL domain — the UI stores vendor
// choice in Parameters.Provider while Source stays "remote", and older
// hand-configured models may carry only a dashscope BaseURL.
func NewASR(config *Config) (ASR, error) {
	if isAliyunASRConfig(config) {
		a, err := NewAliyunASR(config)
		return wrapASRLangfuse(a, err)
	}
	a, err := NewOpenAIASR(config)
	return wrapASRLangfuse(a, err)
}

func isAliyunASRConfig(config *Config) bool {
	if provider.ProviderName(strings.ToLower(config.Provider)) == provider.ProviderAliyun {
		return true
	}
	if config.Source == types.ModelSourceAliyun {
		return true
	}
	return config.Provider == "" && provider.DetectProvider(config.BaseURL) == provider.ProviderAliyun
}
