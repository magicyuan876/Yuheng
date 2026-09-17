package asr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/magicyuan876/yuheng/internal/logger"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// AliyunASR implements ASR via Alibaba Cloud Model Studio (百炼 / DashScope).
//
// DashScope exposes no OpenAI-compatible /audio/transcriptions endpoint —
// Qwen-ASR models (qwen3-asr-flash, qwen-audio-asr) are called through the
// native multimodal-generation API with the audio inlined as a base64 data
// URI. The synchronous API returns plain text without timestamps; timeline
// alignment for videos comes from segment-wise transcription driven by the
// caller (audio track segments carry their own start/end offsets).
type AliyunASR struct {
	modelName  string
	modelID    string
	apiKey     string
	endpoint   string
	language   string
	httpClient *http.Client
}

const dashScopeMultimodalPath = "/api/v1/services/aigc/multimodal-generation/generation"

// dashScopeEndpointFromBaseURL derives the multimodal-generation endpoint
// from whatever base URL the user configured. Users commonly paste the
// OpenAI compatible-mode URL (https://dashscope.aliyuncs.com/compatible-mode/v1),
// the bare host, or nothing at all — all of them resolve to the same native
// endpoint on that host.
func dashScopeEndpointFromBaseURL(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "https://dashscope.aliyuncs.com" + dashScopeMultimodalPath, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid DashScope base URL %q: %w", baseURL, err)
	}
	return parsed.Scheme + "://" + parsed.Host + dashScopeMultimodalPath, nil
}

// NewAliyunASR creates a DashScope-backed ASR instance.
func NewAliyunASR(config *Config) (*AliyunASR, error) {
	if err := validateASRBaseURL(config.BaseURL); err != nil {
		return nil, err
	}
	endpoint, err := dashScopeEndpointFromBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := newASRHTTPClient(asrDefaultTimeout)
	if len(config.CustomHeaders) > 0 {
		httpClient = secutils.WrapHTTPClientWithHeaders(httpClient, config.CustomHeaders)
	}

	return &AliyunASR{
		modelName:  config.ModelName,
		modelID:    config.ModelID,
		apiKey:     config.APIKey,
		endpoint:   endpoint,
		language:   config.Language,
		httpClient: httpClient,
	}, nil
}

type dashScopeASRRequest struct {
	Model      string                 `json:"model"`
	Input      dashScopeASRInput      `json:"input"`
	Parameters dashScopeASRParameters `json:"parameters,omitempty"`
}

type dashScopeASRInput struct {
	Messages []dashScopeASRMessage `json:"messages"`
}

type dashScopeASRMessage struct {
	Role    string          `json:"role"`
	Content []dashScopeItem `json:"content"`
}

type dashScopeItem struct {
	Audio string `json:"audio,omitempty"`
	Text  string `json:"text,omitempty"`
}

type dashScopeASRParameters struct {
	ASROptions *dashScopeASROptions `json:"asr_options,omitempty"`
}

type dashScopeASROptions struct {
	Language  string `json:"language,omitempty"`
	EnableITN bool   `json:"enable_itn"`
}

type dashScopeASRResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// audioMIMEFromFileName picks the data-URI MIME for the inlined audio.
func audioMIMEFromFileName(fileName string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(fileName), ".wav"):
		return "audio/wav"
	case strings.HasSuffix(strings.ToLower(fileName), ".m4a"):
		return "audio/mp4"
	case strings.HasSuffix(strings.ToLower(fileName), ".flac"):
		return "audio/flac"
	case strings.HasSuffix(strings.ToLower(fileName), ".ogg"):
		return "audio/ogg"
	default:
		return "audio/mpeg"
	}
}

// Transcribe sends audio bytes to the DashScope multimodal-generation API.
// The synchronous Qwen-ASR API returns no timestamps, so Segments is nil.
func (s *AliyunASR) Transcribe(ctx context.Context, audioBytes []byte, fileName string) (*TranscriptionResult, error) {
	if len(audioBytes) == 0 {
		return nil, fmt.Errorf("audio bytes are empty")
	}

	dataURI := "data:" + audioMIMEFromFileName(fileName) + ";base64," +
		base64.StdEncoding.EncodeToString(audioBytes)

	reqBody := dashScopeASRRequest{
		Model: s.modelName,
		Input: dashScopeASRInput{
			Messages: []dashScopeASRMessage{
				{Role: "user", Content: []dashScopeItem{{Audio: dataURI}}},
			},
		},
	}
	if s.language != "" {
		reqBody.Parameters = dashScopeASRParameters{
			ASROptions: &dashScopeASROptions{Language: s.language, EnableITN: true},
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal DashScope ASR request: %w", err)
	}

	logger.Infof(ctx, "[ASR] Calling DashScope multimodal API, model=%s, endpoint=%s, audioSize=%d, file=%s",
		s.modelName, s.endpoint, len(audioBytes), fileName)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build DashScope ASR request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("DashScope ASR request failed: %w", err)
	}
	defer resp.Body.Close()

	// The error body carries the DashScope code/message — surface it so a
	// misconfigured model name or workspace is diagnosable from the UI.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read DashScope ASR response: %w", err)
	}

	var parsed dashScopeASRResponse
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil && resp.StatusCode == http.StatusOK {
		return nil, fmt.Errorf("decode DashScope ASR response: %w", jsonErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DashScope ASR request failed: status %d, code=%s, message=%s",
			resp.StatusCode, parsed.Code, firstNonEmpty(parsed.Message, string(body)))
	}
	if parsed.Code != "" {
		return nil, fmt.Errorf("DashScope ASR error: code=%s, message=%s", parsed.Code, parsed.Message)
	}

	var sb strings.Builder
	for _, choice := range parsed.Output.Choices {
		for _, item := range choice.Message.Content {
			sb.WriteString(item.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	logger.Infof(ctx, "[ASR] DashScope transcription completed, text length=%d", len(text))

	return &TranscriptionResult{Text: text}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *AliyunASR) GetModelName() string { return s.modelName }
func (s *AliyunASR) GetModelID() string   { return s.modelID }
