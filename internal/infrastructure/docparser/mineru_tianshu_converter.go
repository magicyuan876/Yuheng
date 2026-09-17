package docparser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// MinerU Tianshu (天枢) is an async task-queue front for MinerU
// (submit → poll → markdown), so this reader is a two-phase HTTP client
// rather than mineru_converter.go's single /file_parse call:
//
//	POST {endpoint}/api/v1/tasks/submit   multipart: file + backend/lang/method/…
//	  → {"success": true, "task_id": "…", "status": "pending"}
//	GET  {endpoint}/api/v1/tasks/{id}
//	  → {"success": true, "status": "pending|processing|completed|failed|cancelled",
//	     "error_message": …, "data": {"content": "<markdown>"}}
//
// The result is markdown text only: Tianshu keeps page images on its own
// storage (upload_images would rewrite refs to its MinIO), so no ImageRefs
// are produced here — image links in the markdown stay as inert relative
// paths, same as the builtin parser's placeholders.
const (
	// mineruTianshuPollTimeout bounds one document's submit-to-completed wait.
	// Large scanned decks take a long time on a busy GPU queue; the asynq parse
	// task allows more, but a still-pending Tianshu task keeps running server-side
	// and a retry will resubmit rather than hang this worker forever.
	mineruTianshuPollTimeout = 30 * time.Minute
	// mineruTianshuPollMaxInterval caps the poll backoff (0.5s → 5s, ×2).
	mineruTianshuPollMaxInterval = 5 * time.Second

	// mineruTianshuUploadStallTimeout aborts an upload that stops making
	// progress. A wall-clock cap is the wrong tool for the upload: a genuinely
	// slow link (a remote Tianshu behind a high-latency tunnel can sustain
	// only ~100KB/s, so 50MB legitimately takes minutes) is indistinguishable
	// from a hang until it finishes. Failing on *no bytes moved* instead lets
	// slow-but-healthy uploads run to completion while a truly stuck socket is
	// reported in two minutes rather than tying up a worker for a quarter hour.
	mineruTianshuUploadStallTimeout = 2 * time.Minute
	// mineruTianshuUploadMaxDuration is the outer bound on one upload, high
	// enough that only a pathological transfer reaches it.
	mineruTianshuUploadMaxDuration = 60 * time.Minute
	// mineruTianshuProgressInterval is how often an in-flight upload reports
	// bytes/rate/ETA. Silence during a long upload is what made a stalled
	// transfer look identical to a dead service.
	mineruTianshuProgressInterval = 15 * time.Second
	// mineruTianshuPollHTTPTimeout bounds a single status poll (a tiny JSON
	// GET); unlike the upload it has no reason to take long.
	mineruTianshuPollHTTPTimeout = 60 * time.Second

	// mineruTianshuDefaultAuthHeader carries the optional API key. Some
	// deployments require it on the task endpoints while leaving /health open.
	mineruTianshuDefaultAuthHeader = "X-API-Key"
)

// errUploadStalled is the cancellation cause attached when the stall watchdog
// fires, so the failure reads as "stalled" instead of a bare "context canceled".
var errUploadStalled = errors.New("upload stalled: no bytes accepted by the server")

// MinerUTianshuReader calls a self-hosted MinerU Tianshu service.
type MinerUTianshuReader struct {
	endpoint      string
	apiKey        string
	authHeader    string
	backend       string // empty = server default (pipeline; newer builds accept auto)
	language      string // empty = server default
	parseMethod   string // empty = server default (auto)
	formulaEnable bool
	tableEnable   bool

	// Timeouts are fields so tests can shrink them.
	pollTimeout      time.Duration
	pollMaxInterval  time.Duration
	uploadStall      time.Duration
	uploadMax        time.Duration
	progressInterval time.Duration
}

// NewMinerUTianshuReader creates a reader from ParserEngineOverrides.
func NewMinerUTianshuReader(overrides map[string]string) *MinerUTianshuReader {
	return &MinerUTianshuReader{
		endpoint:         strings.TrimRight(strings.TrimSpace(overrides["mineru_tianshu_endpoint"]), "/"),
		apiKey:           strings.TrimSpace(overrides["mineru_tianshu_api_key"]),
		authHeader:       stringOr(strings.TrimSpace(overrides["mineru_tianshu_auth_header"]), mineruTianshuDefaultAuthHeader),
		backend:          strings.TrimSpace(overrides["mineru_tianshu_backend"]),
		language:         strings.TrimSpace(overrides["mineru_tianshu_language"]),
		parseMethod:      strings.TrimSpace(overrides["mineru_tianshu_parse_method"]),
		formulaEnable:    parseBoolOr(overrides["mineru_tianshu_enable_formula"], true),
		tableEnable:      parseBoolOr(overrides["mineru_tianshu_enable_table"], true),
		pollTimeout:      mineruTianshuPollTimeout,
		pollMaxInterval:  mineruTianshuPollMaxInterval,
		uploadStall:      mineruTianshuUploadStallTimeout,
		uploadMax:        mineruTianshuUploadMaxDuration,
		progressInterval: mineruTianshuProgressInterval,
	}
}

// tianshuSubmitResponse is the POST /api/v1/tasks/submit payload.
type tianshuSubmitResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// tianshuStatusResponse is the GET /api/v1/tasks/{id} payload. Data is only
// present on completed tasks (and may be null when results were cleaned up).
type tianshuStatusResponse struct {
	Success      bool   `json:"success"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
	Data         *struct {
		Content string `json:"content"`
	} `json:"data"`
}

func (c *MinerUTianshuReader) Read(ctx context.Context, req *types.ReadRequest) (*types.ReadResult, error) {
	if c.endpoint == "" {
		return &types.ReadResult{Error: "MinerU Tianshu endpoint is not configured"}, nil
	}
	if err := validateMinerUOutboundURL(c.endpoint); err != nil {
		return &types.ReadResult{Error: err.Error()}, nil
	}
	if len(req.FileContent) == 0 {
		return &types.ReadResult{Error: "no file content provided"}, nil
	}

	logger.Infof(ctx, "[Tianshu] Parsing file=%s size=%s via %s",
		req.FileName, humanBytes(int64(len(req.FileContent))), c.endpoint)

	started := time.Now()
	taskID, err := c.submitTask(ctx, req.FileContent, req.FileName, req.FileType)
	if err != nil {
		// Log here rather than relying on the caller: a failed upload is the
		// one outcome an operator needs the byte/rate detail for, and the
		// error string alone cannot carry it.
		logger.Errorf(ctx, "[Tianshu] Submit failed for %s after %s: %v", req.FileName, time.Since(started).Round(time.Second), err)
		return nil, fmt.Errorf("tianshu submit: %w", err)
	}
	logger.Infof(ctx, "[Tianshu] Task submitted: %s (file=%s, upload took %s)",
		taskID, req.FileName, time.Since(started).Round(time.Second))

	mdContent, err := c.pollTask(ctx, taskID)
	if err != nil {
		logger.Errorf(ctx, "[Tianshu] Task %s failed for %s: %v", taskID, req.FileName, err)
		return nil, fmt.Errorf("tianshu task %s: %w", taskID, err)
	}

	// Tianshu returns MinerU markdown; apply the same narrow compatibility
	// fixes as the direct MinerU path (over-escaped image syntax / headings).
	mdContent = normalizeMinerUMarkdown(mdContent)

	logger.Infof(ctx, "[Tianshu] Task %s parsed successfully, markdown=%d chars (total %s)",
		taskID, len(mdContent), time.Since(started).Round(time.Second))
	return &types.ReadResult{MarkdownContent: mdContent}, nil
}

// uploadProgress tracks how much of the request body the transport has handed
// to the socket. Read runs on the transport's goroutine while the watchdog
// samples from another, hence the atomics.
type uploadProgress struct {
	total     int64
	sent      atomic.Int64
	lastMoved atomic.Int64 // UnixNano of the most recent non-empty read
}

// progressReader counts bytes as the transport drains the body.
type progressReader struct {
	inner io.Reader
	p     *uploadProgress
}

func (r *progressReader) Read(b []byte) (int, error) {
	n, err := r.inner.Read(b)
	if n > 0 {
		r.p.sent.Add(int64(n))
		r.p.lastMoved.Store(time.Now().UnixNano())
	}
	return n, err
}

// watchUpload logs throughput while the body is being sent and cancels the
// request when no bytes move for stall. It returns when ctx ends, which the
// caller guarantees by cancelling once the request completes.
func (c *MinerUTianshuReader) watchUpload(
	ctx context.Context, cancel context.CancelCauseFunc, p *uploadProgress, fileName string,
) {
	ticker := time.NewTicker(c.progressInterval)
	defer ticker.Stop()
	started := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		sent := p.sent.Load()
		elapsed := time.Since(started)
		idle := time.Since(time.Unix(0, p.lastMoved.Load()))

		if idle >= c.uploadStall {
			logger.Errorf(ctx, "[Tianshu] Upload of %s stalled: %s of %s sent, no progress for %s",
				fileName, humanBytes(sent), humanBytes(p.total), idle.Round(time.Second))
			cancel(errUploadStalled)
			return
		}

		// rate over the whole upload, which is what an ETA should be based on.
		rate := float64(sent) / elapsed.Seconds()
		eta := "unknown"
		if rate > 0 && p.total > sent {
			eta = (time.Duration(float64(p.total-sent)/rate) * time.Second).Round(time.Second).String()
		}
		logger.Infof(ctx, "[Tianshu] Uploading %s: %s/%s (%.1f%%) at %s/s, elapsed %s, ETA %s",
			fileName, humanBytes(sent), humanBytes(p.total),
			float64(sent)*100/float64(max64(p.total, 1)), humanBytes(int64(rate)),
			elapsed.Round(time.Second), eta)
	}
}

// buildSubmitBody assembles the multipart body as prefix + file + suffix.
//
// The file bytes are referenced, not copied: an earlier version wrote the whole
// upload into a bytes.Buffer, so a 50MB document occupied ~100MB (the caller's
// copy plus the multipart copy) for the duration of the request. Content-Length
// stays known, so the request does not fall back to chunked encoding.
func (c *MinerUTianshuReader) buildSubmitBody(
	content []byte, fileName, fileType string,
) (body func() io.Reader, contentType string, length int64, err error) {
	var prefix bytes.Buffer
	writer := multipart.NewWriter(&prefix)

	// Optional fields are omitted when unset so the server's own defaults
	// apply — keeps the client compatible across Tianshu versions whose
	// defaults differ (e.g. backend pipeline vs auto).
	fields := map[string]string{
		"formula_enable": fmt.Sprintf("%v", c.formulaEnable),
		"table_enable":   fmt.Sprintf("%v", c.tableEnable),
		"priority":       "0",
	}
	if c.backend != "" {
		fields["backend"] = c.backend
	}
	if c.language != "" {
		fields["lang"] = c.language
	}
	if c.parseMethod != "" {
		fields["method"] = c.parseMethod
	}
	for k, v := range fields {
		if werr := writer.WriteField(k, v); werr != nil {
			return nil, "", 0, fmt.Errorf("write field %s: %w", k, werr)
		}
	}
	if _, werr := writer.CreateFormFile("file", minerUUploadFileName(fileName, fileType)); werr != nil {
		return nil, "", 0, fmt.Errorf("create form file: %w", werr)
	}
	// CreateFormFile wrote the part header into prefix; the content follows it
	// unbuffered and the closing boundary comes from a second writer.
	head := append([]byte(nil), prefix.Bytes()...)

	var tail bytes.Buffer
	tailWriter := multipart.NewWriter(&tail)
	if serr := tailWriter.SetBoundary(writer.Boundary()); serr != nil {
		return nil, "", 0, fmt.Errorf("set boundary: %w", serr)
	}
	if cerr := tailWriter.Close(); cerr != nil {
		return nil, "", 0, fmt.Errorf("close writer: %w", cerr)
	}
	foot := tail.Bytes()

	newBody := func() io.Reader {
		return io.MultiReader(bytes.NewReader(head), bytes.NewReader(content), bytes.NewReader(foot))
	}
	return newBody, writer.FormDataContentType(), int64(len(head) + len(content) + len(foot)), nil
}

// submitTask uploads the file and returns the Tianshu task id.
func (c *MinerUTianshuReader) submitTask(ctx context.Context, content []byte, fileName, fileType string) (string, error) {
	newBody, contentType, length, err := c.buildSubmitBody(content, fileName, fileType)
	if err != nil {
		return "", err
	}

	// The upload gets its own cancellable context: the stall watchdog cancels
	// it with a cause that names the real problem, and uploadMax bounds the
	// pathological case.
	upCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(c.uploadMax, func() {
		cancel(fmt.Errorf("upload exceeded %s", c.uploadMax))
	})
	defer timer.Stop()

	progress := &uploadProgress{total: length}
	progress.lastMoved.Store(time.Now().UnixNano())
	go c.watchUpload(upCtx, cancel, progress, fileName)

	httpReq, err := http.NewRequestWithContext(upCtx, http.MethodPost, c.endpoint+"/api/v1/tasks/submit",
		&progressReader{inner: newBody(), p: progress})
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.ContentLength = length
	// Rewindable body so a redirect does not fail the upload outright.
	httpReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(&progressReader{inner: newBody(), p: progress}), nil
	}
	c.applyAuth(httpReq)

	respBody, err := c.doRequest(httpReq, 0)
	if err != nil {
		return "", c.describeUploadError(err, upCtx, progress)
	}

	var submit tianshuSubmitResponse
	if err := json.Unmarshal(respBody, &submit); err != nil {
		return "", fmt.Errorf("decode submit response: %w", err)
	}
	if !submit.Success {
		return "", fmt.Errorf("submit rejected: %s", stringOr(submit.Message, "unknown error"))
	}
	if submit.TaskID == "" {
		return "", fmt.Errorf("submit response missing task_id")
	}
	return submit.TaskID, nil
}

// describeUploadError turns a transport failure into a message that says how
// far the upload got, so a slow link is not mistaken for a dead service.
func (c *MinerUTianshuReader) describeUploadError(err error, upCtx context.Context, p *uploadProgress) error {
	sent, total := p.sent.Load(), p.total
	detail := fmt.Sprintf("%s of %s sent", humanBytes(sent), humanBytes(total))
	if cause := context.Cause(upCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		if errors.Is(cause, errUploadStalled) {
			return fmt.Errorf("%w (%s); the service accepted no data for %s — check connectivity to %s",
				errUploadStalled, detail, c.uploadStall, c.endpoint)
		}
		return fmt.Errorf("upload aborted (%s): %w", detail, cause)
	}
	return fmt.Errorf("upload failed (%s): %w", detail, err)
}

// pollTask polls task status with backoff until completed/failed/timeout and
// returns the parsed markdown content.
func (c *MinerUTianshuReader) pollTask(ctx context.Context, taskID string) (string, error) {
	statusURL := c.endpoint + "/api/v1/tasks/" + url.PathEscape(taskID)
	started := time.Now()
	deadline := started.Add(c.pollTimeout)
	interval := 500 * time.Millisecond
	lastLogged := time.Now()

	for {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			return "", fmt.Errorf("create status request: %w", err)
		}
		c.applyAuth(httpReq)

		respBody, err := c.doRequest(httpReq, mineruTianshuPollHTTPTimeout)
		if err != nil {
			return "", err
		}

		var status tianshuStatusResponse
		if err := json.Unmarshal(respBody, &status); err != nil {
			return "", fmt.Errorf("decode status response: %w", err)
		}
		if !status.Success {
			return "", fmt.Errorf("status query rejected: %s", stringOr(status.Message, "unknown error"))
		}

		switch status.Status {
		case "completed":
			if status.Data == nil || status.Data.Content == "" {
				// data==null covers both "results cleaned up" and "markdown
				// missing" — the server's message says which.
				return "", fmt.Errorf("task completed but no content returned: %s",
					stringOr(status.Message, "data field is empty"))
			}
			return status.Data.Content, nil
		case "failed":
			return "", fmt.Errorf("parsing failed: %s", stringOr(status.ErrorMessage, "unknown error"))
		case "cancelled":
			return "", fmt.Errorf("task was cancelled on the server")
		case "pending", "processing":
			// A long queue wait is normal on a busy GPU; report it rather than
			// leaving the stage silent for half an hour.
			if time.Since(lastLogged) >= c.progressInterval {
				logger.Infof(ctx, "[Tianshu] Task %s still %s after %s",
					taskID, status.Status, time.Since(started).Round(time.Second))
				lastLogged = time.Now()
			}
		default:
			logger.Warnf(ctx, "[Tianshu] task %s: unknown status %q, keep polling", taskID, status.Status)
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out after %s waiting for completion (last status: %s)",
				c.pollTimeout, status.Status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		if interval *= 2; interval > c.pollMaxInterval {
			interval = c.pollMaxInterval
		}
	}
}

// applyAuth attaches the optional API key header.
func (c *MinerUTianshuReader) applyAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set(c.authHeader, c.apiKey)
	}
}

// doRequest executes one HTTP call with SSRF protection and returns the body
// of a 200 response. timeout==0 leaves the deadline to the request context,
// which the upload path manages itself (a wall-clock cap there would abort
// healthy slow transfers).
func (c *MinerUTianshuReader) doRequest(req *http.Request, timeout time.Duration) ([]byte, error) {
	client := utils.NewSSRFSafeHTTPClient(utils.SSRFSafeHTTPClientConfig{
		Timeout:      timeout,
		MaxRedirects: 5,
	})
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tianshu API status %d: %s", resp.StatusCode, truncateForLog(string(respBody), 500))
	}
	return respBody, nil
}

// truncateForLog bounds an error payload echoed into messages/logs.
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// humanBytes renders a byte count for logs and error messages.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2fGB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// PingMinerUTianshu reports whether a Tianshu service is reachable AND, when an
// API key is configured, whether that key is actually accepted.
//
// The health endpoint alone is not enough: deployments that require auth on the
// task endpoints commonly leave /health open, so a wrong or missing key still
// produced a green "available" badge while every parse failed with 401. The
// second probe hits an authenticated read-only endpoint and distinguishes
// "rejected" (401/403) from "reachable" (404 for an unknown task id is the
// expected answer and proves the credential passed).
func PingMinerUTianshu(endpoint, apiKey, authHeader string) (bool, string) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return false, "未配置 MinerU 天枢端点"
	}
	if err := validateMinerUOutboundURL(endpoint); err != nil {
		return false, err.Error()
	}
	client := utils.NewSSRFSafeHTTPClient(utils.SSRFSafeHTTPClientConfig{
		Timeout:      5 * time.Second,
		MaxRedirects: 5,
	})

	resp, err := client.Get(endpoint + "/api/v1/health")
	if err != nil {
		return false, fmt.Sprintf("MinerU 天枢服务不可达: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("MinerU 天枢服务返回状态 %d", resp.StatusCode)
	}

	// Probe a task-scoped read with a well-formed but non-existent id. Auth is
	// evaluated before the lookup, so 401/403 means the credential is the
	// problem while 404 means it was accepted.
	probeURL := endpoint + "/api/v1/tasks/00000000-0000-0000-0000-000000000000"
	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		return true, "" // cannot probe; health passed, don't block the engine
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set(stringOr(strings.TrimSpace(authHeader), mineruTianshuDefaultAuthHeader), strings.TrimSpace(apiKey))
	}
	authResp, err := client.Do(req)
	if err != nil {
		return true, "" // health already proved reachability
	}
	authResp.Body.Close()
	switch authResp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if strings.TrimSpace(apiKey) == "" {
			return false, "MinerU 天枢需要鉴权，但未配置 API Key"
		}
		return false, "MinerU 天枢拒绝了配置的 API Key（请检查 Key 与请求头名称）"
	default:
		return true, ""
	}
}
