package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/im"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// ImgUploader publishes reply images to WeCom's permanent image CDN via the
// self-built-app uploadimg API. The returned https://wework.qpic.cn/... URLs
// are Tencent-hosted and reachable from every WeCom client regardless of
// where this deployment sits, so markdown replies can embed them inline.
//
// Requires a self-built app's CorpID + app Secret (NOT the intelligent bot's
// credentials). When unconfigured, callers fall back to standalone image
// messages via the bot media upload.
//
// Note the security trade-off: uploadimg URLs are unauthenticated public CDN
// links — anyone holding the link can view the image.
type ImgUploader struct {
	corpID     string
	corpSecret string
	apiBaseURL string
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	// urlCache dedupes uploads by image MD5 (uploadimg URLs are permanent).
	urlCache map[string]string
}

const defaultWeComAPIBase = "https://qyapi.weixin.qq.com"

// NewImgUploaderFromConfig builds an uploader from channel credentials with
// environment fallbacks (WECOM_CORP_ID / WECOM_CORP_SECRET). Returns nil when
// no credentials are configured — inline URL mode is simply unavailable.
func NewImgUploaderFromConfig(creds map[string]any) *ImgUploader {
	corpID := strings.TrimSpace(im.GetString(creds, "corp_id"))
	corpSecret := strings.TrimSpace(im.GetString(creds, "corp_secret"))
	if corpID == "" {
		corpID = strings.TrimSpace(os.Getenv("WECOM_CORP_ID"))
	}
	if corpSecret == "" {
		corpSecret = strings.TrimSpace(os.Getenv("WECOM_CORP_SECRET"))
	}
	if corpID == "" || corpSecret == "" {
		return nil
	}
	apiBase := strings.TrimSpace(im.GetString(creds, "api_base_url"))
	if apiBase == "" {
		apiBase = defaultWeComAPIBase
	}
	return &ImgUploader{
		corpID:     corpID,
		corpSecret: corpSecret,
		apiBaseURL: strings.TrimRight(apiBase, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		urlCache:   make(map[string]string),
	}
}

// token returns a cached access token, refreshing via gettoken when expired.
// Callers must hold u.mu.
func (u *ImgUploader) token(ctx context.Context) (string, error) {
	if u.accessToken != "" && time.Now().Before(u.tokenExpiry) {
		return u.accessToken, nil
	}

	tokenURL := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		u.apiBaseURL, u.corpID, u.corpSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("build gettoken request: %w", err)
	}
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gettoken: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode gettoken response: %w", err)
	}
	if result.ErrCode != 0 || result.AccessToken == "" {
		return "", fmt.Errorf("gettoken failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	u.accessToken = result.AccessToken
	// Refresh five minutes early to ride out clock skew and long uploads.
	u.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return u.accessToken, nil
}

// invalidateToken forces a refresh on the next call (401/40014 handling).
// Callers must hold u.mu.
func (u *ImgUploader) invalidateToken() {
	u.accessToken = ""
	u.tokenExpiry = time.Time{}
}

// UploadForURL uploads one JPEG/PNG image (≤2MB per API contract) and returns
// its permanent CDN URL. Results are cached by content MD5.
func (u *ImgUploader) UploadForURL(ctx context.Context, img im.ReplyImage) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if cached, ok := u.urlCache[img.MD5]; ok {
		return cached, nil
	}

	url, err := u.uploadOnce(ctx, img)
	if isWeComTokenError(err) {
		u.invalidateToken()
		url, err = u.uploadOnce(ctx, img)
	}
	if err != nil {
		return "", err
	}
	u.urlCache[img.MD5] = url
	return url, nil
}

func (u *ImgUploader) uploadOnce(ctx context.Context, img im.ReplyImage) (string, error) {
	token, err := u.token(ctx)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "reply.jpg")
	if err != nil {
		return "", fmt.Errorf("create multipart: %w", err)
	}
	if _, err := part.Write(img.Data); err != nil {
		return "", fmt.Errorf("write multipart: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart: %w", err)
	}

	uploadURL := fmt.Sprintf("%s/cgi-bin/media/uploadimg?access_token=%s", u.apiBaseURL, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("build uploadimg request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("uploadimg: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read uploadimg response: %w", err)
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode uploadimg response: %w", err)
	}
	if result.ErrCode != 0 || result.URL == "" {
		return "", fmt.Errorf("uploadimg failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	logger.Debugf(ctx, "[WeCom] uploadimg ok: md5=%s bytes=%d", img.MD5, len(img.Data))
	return result.URL, nil
}

// isWeComTokenError reports whether err carries a WeCom invalid/expired
// access-token errcode (40014 invalid, 42001 expired, 40001 invalid secret
// after rotation).
func isWeComTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "errcode=40014") ||
		strings.Contains(msg, "errcode=42001") ||
		strings.Contains(msg, "errcode=40001")
}
