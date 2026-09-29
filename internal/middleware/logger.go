package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

const (
	maxBodySize = 1024 * 10 // 最大记录10KB的body内容
)

// loggerResponseBodyWriter 自定义ResponseWriter用于捕获响应内容（用于logger中间件）
type loggerResponseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 重写Write方法，同时写入buffer和原始writer
// 限制buffer大小，避免SSE等流式响应导致内存无限增长
func (r loggerResponseBodyWriter) Write(b []byte) (int, error) {
	if r.body.Len() < maxBodySize {
		remaining := maxBodySize - r.body.Len()
		if len(b) <= remaining {
			r.body.Write(b)
		} else {
			r.body.Write(b[:remaining])
		}
	}
	return r.ResponseWriter.Write(b)
}

// sensitiveFieldRegex 匹配 JSON 中的敏感字段（不区分大小写，兼容 snake_case / camelCase / PascalCase）。
// $1 捕获原始字段名（包括两侧引号），保持日志中的字段名不变，仅将值替换为 "***"。
var sensitiveFieldRegex = regexp.MustCompile(
	`(?i)("(?:new[_-]?password|old[_-]?password|password|passwd|token|access[_-]?token|` +
		`refresh[_-]?token|id[_-]?token|authorization|auth[_-]?token|api[_-]?key|` +
		`api[_-]?secret|secret[_-]?key|client[_-]?secret|private[_-]?key|secret|` +
		`authorization[_-]?url|authorization[_-]?attempt)")\s*:\s*"[^"]*"`,
)

// sanitizeBody 清理敏感信息
func sanitizeBody(body string) string {
	return sensitiveFieldRegex.ReplaceAllString(body, `$1:"***"`)
}

var sensitiveQueryFields = map[string]struct{}{
	"access_token":          {},
	"authorization_attempt": {},
	"code":                  {},
	"id_token":              {},
	"refresh_token":         {},
	"state":                 {},
	"token":                 {},
}

// sanitizeQuery prevents OAuth authorization codes and CSRF/attempt state from
// being copied into access logs. Parsing the query also covers repeated and
// percent-encoded parameters without relying on fragile string replacement.
func sanitizeQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "[invalid query omitted]"
	}
	for key := range values {
		if _, sensitive := sensitiveQueryFields[strings.ToLower(key)]; sensitive {
			values[key] = []string{"***"}
		}
	}
	return values.Encode()
}

// requestCaptureLimit is how much of a request body is kept for the access log.
// It is larger than maxBodySize (what is finally logged) on purpose: secrets are
// masked in the captured text first and the result is cut afterwards, so a
// credential that straddles the logged prefix's end is masked, not half-printed.
const requestCaptureLimit = 64 * 1024

// bodyCapture wraps a request body and remembers the first requestCaptureLimit
// bytes that pass through it. It is a tee, not a buffer: the handler reads the
// original stream at its own pace and sees every byte, whatever its size, and
// memory use is bounded by the limit no matter how large the upload is.
// Reads may happen on a goroutine the handler spawned, hence the mutex.
type bodyCapture struct {
	rc    io.ReadCloser
	limit int

	mu    sync.Mutex
	buf   []byte
	total int64
}

func (b *bodyCapture) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if n > 0 {
		b.mu.Lock()
		if room := b.limit - len(b.buf); room > 0 {
			b.buf = append(b.buf, p[:min(n, room)]...)
		}
		b.total += int64(n)
		b.mu.Unlock()
	}
	return n, err
}

func (b *bodyCapture) Close() error { return b.rc.Close() }

// captured returns what was read so far and whether more than that passed through.
func (b *bodyCapture) captured() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf), b.total > int64(len(b.buf))
}

// isLoggableBody reports whether a request body of this Content-Type is text
// worth logging. Everything else (multipart uploads, binary) is not captured at
// all, so those streams are not touched.
func isLoggableBody(contentType string) bool {
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/x-www-form-urlencoded") ||
		strings.Contains(contentType, "text/")
}

// captureRequestBody installs a tee on the request body when it is loggable and
// returns it, or nil when nothing is to be captured. The body is not read here:
// the log line is written after the handler ran, from whatever it consumed.
func captureRequestBody(c *gin.Context) *bodyCapture {
	if c.Request.Body == nil || c.Request.Body == http.NoBody || !isLoggableBody(c.GetHeader("Content-Type")) {
		return nil
	}
	capture := &bodyCapture{rc: c.Request.Body, limit: requestCaptureLimit}
	c.Request.Body = capture
	return capture
}

// unterminatedSecretRegex matches a sensitive field whose value was cut off by
// the capture limit, so it has no closing quote for sensitiveFieldRegex to find.
var unterminatedSecretRegex = regexp.MustCompile(
	`(?i)("(?:new[_-]?password|old[_-]?password|password|passwd|token|access[_-]?token|` +
		`refresh[_-]?token|id[_-]?token|authorization|auth[_-]?token|api[_-]?key|` +
		`api[_-]?secret|secret[_-]?key|client[_-]?secret|private[_-]?key|secret)")\s*:\s*"[^"]*$`,
)

// formatRequestBody turns captured bytes into the text that is logged.
func formatRequestBody(captured *bodyCapture) string {
	if captured == nil {
		return "[非文本类型，已跳过]"
	}
	body, more := captured.captured()
	if body == "" {
		return ""
	}
	body = sanitizeBody(body)
	if more {
		body = unterminatedSecretRegex.ReplaceAllString(body, `$1:"***"`)
	}
	if len(body) > maxBodySize {
		body = body[:maxBodySize]
		more = true
	}
	if more {
		body += "... [内容过长，已截断]"
	}
	return body
}

// RequestID middleware adds a unique request ID to the context
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get request ID from header or generate a new one
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		safeRequestID := secutils.SanitizeForLog(requestID)
		// Set request ID in header
		c.Header("X-Request-ID", requestID)

		// Set request ID in context
		c.Set(types.RequestIDContextKey.String(), requestID)

		// Set logger in context
		requestLogger := logger.GetLogger(c)
		requestLogger = requestLogger.WithField("request_id", safeRequestID)
		c.Set(types.LoggerContextKey.String(), requestLogger)

		// Set request ID in the global context for logging
		c.Request = c.Request.WithContext(
			context.WithValue(
				context.WithValue(c.Request.Context(), types.RequestIDContextKey, requestID),
				types.LoggerContextKey, requestLogger,
			),
		)

		c.Next()
	}
}

// Logger middleware logs request details with request ID, input and output
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		isWikiStats := strings.HasPrefix(path, "/api/v1/knowledgebase/") && strings.HasSuffix(path, "/wiki/stats")
		if strings.HasPrefix(path, "/assets/") || isWikiStats {
			c.Next()
			return
		}

		// 请求体只做旁路捕获（tee）：handler 照常流式读取完整内容，日志只留前
		// requestCaptureLimit 字节，不再在鉴权之前把整个 body 读进内存。
		var requestCapture *bodyCapture
		hasBody := c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH"
		if hasBody {
			requestCapture = captureRequestBody(c)
		}

		// 创建响应体捕获器
		responseBody := &bytes.Buffer{}
		responseWriter := &loggerResponseBodyWriter{
			ResponseWriter: c.Writer,
			body:           responseBody,
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Get request ID from context
		requestID, exists := c.Get(types.RequestIDContextKey.String())
		requestIDStr := "unknown"
		if exists {
			if idStr, ok := requestID.(string); ok && idStr != "" {
				requestIDStr = idStr
			}
		}
		safeRequestID := secutils.SanitizeForLog(requestIDStr)

		// Calculate latency
		latency := time.Since(start)

		// Get client IP and status code
		clientIP := c.ClientIP()
		statusCode := c.Writer.Status()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + sanitizeQuery(raw)
		}

		// 读取响应体
		responseBodyStr := ""
		if responseBody.Len() > 0 {
			contentType := c.Writer.Header().Get("Content-Type")
			if strings.Contains(contentType, "text/event-stream") {
				responseBodyStr = "[SSE流式响应，已跳过]"
			} else if strings.Contains(contentType, "application/json") ||
				strings.Contains(contentType, "text/") {
				bodyBytes := responseBody.Bytes()
				if len(bodyBytes) >= maxBodySize {
					responseBodyStr = string(bodyBytes[:maxBodySize]) + "... [内容过长，已截断]"
				} else {
					responseBodyStr = string(bodyBytes)
				}
				responseBodyStr = sanitizeBody(responseBodyStr)
			} else {
				responseBodyStr = "[非文本类型，已跳过]"
			}
		}

		// 构建日志消息
		logMsg := logger.GetLogger(c)
		logMsg = logMsg.WithFields(map[string]interface{}{
			"request_id":  safeRequestID,
			"method":      method,
			"path":        secutils.SanitizeForLog(path),
			"status_code": statusCode,
			"size":        c.Writer.Size(),
			"latency":     latency.String(),
			"client_ip":   secutils.SanitizeForLog(clientIP),
		})

		// 添加请求体（如果有）；非文本请求体没有被捕获，直接标注跳过
		var requestBody string
		if hasBody && c.Request.ContentLength != 0 {
			requestBody = formatRequestBody(requestCapture)
		}
		if requestBody != "" {
			logMsg = logMsg.WithField("request_body", secutils.SanitizeForLog(requestBody))
		}

		// 添加响应体（如果有）
		if responseBodyStr != "" {
			logMsg = logMsg.WithField("response_body", secutils.SanitizeForLog(responseBodyStr))
		}
		if last := c.Errors.Last(); last != nil && last.Err != nil {
			logMsg = logMsg.WithField("error", secutils.SanitizeForLog(last.Err.Error()))
		}
		switch {
		case statusCode >= 500:
			logMsg.Error()
		case statusCode >= 400:
			logMsg.Warn()
		default:
			logMsg.Info()
		}
	}
}
