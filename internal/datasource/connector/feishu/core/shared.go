package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/magicyuan876/yuheng/internal/datasource"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

const FeishuWikiNodeResourceSeparator = ":"

// shared.go holds the helpers used by BOTH the wiki Connector (connector.go)
// and the Drive DriveConnector (connector.go): error classification,
// config parsing, stream-Checkpoint tuning, the fetch tally, filename/time
// utilities, attachment rules, and the docx blocks fetch path. Anything that
// is specific to one connector stays in that connector's own file.

// FeishuStreamCheckpointInterval is how many processed nodes pass between
// cursor checkpoints during a streaming fetch. Small enough that a timed-out
// sync loses little work on resume, large enough that Checkpoint persistence
// (a DB write) does not dominate. Overridable in tests. See FetchStream.
var FeishuStreamCheckpointInterval = 50

// FeishuStreamCheckpointMaxInterval bounds checkpointing by wall-clock time as
// well as node count. Without it, a sync of fewer than
// FeishuStreamCheckpointInterval very slow (rate-limited) exports could reach
// the 2h task timeout having never checkpointed, and resume from scratch every
// retry — the #2136 "never fully syncs" case. Overridable in tests.
var FeishuStreamCheckpointMaxInterval = 30 * time.Second

// fetchTally accumulates the outcome of fetching a wiki node subtree so the
// connector can Emit a single actionable summary. Without it, unsupported nodes
// (mindnote/slides/etc.) vanish with no item, no error and no log, leaving users
// unable to explain why "13 documents synced only 3" (upstream#2136).
type fetchTally struct {
	discovered    int
	fetched       int
	failed        int
	skippedByType map[string]int
}

func newFetchTally(discovered int) *fetchTally {
	return &fetchTally{discovered: discovered, skippedByType: map[string]int{}}
}

func (t *fetchTally) fetch()              { t.fetched++ }
func (t *fetchTally) fail()               { t.failed++ }
func (t *fetchTally) Skip(objType string) { t.skippedByType[objType]++ }

func (t *fetchTally) skipped() int {
	n := 0
	for _, c := range t.skippedByType {
		n += c
	}
	return n
}

func (t *fetchTally) summary() string {
	return fmt.Sprintf("discovered=%d fetched=%d failed=%d skipped_unsupported=%d by_type=%v",
		t.discovered, t.fetched, t.failed, t.skipped(), t.skippedByType)
}

var reFeishuErrorCode = regexp.MustCompile(`code["\s]*[:=]\s*(\d+)`)

// feishuErrorCode extracts the numeric Feishu error code from a raw error string
// (e.g. `body={"code":1663,...}` or `code=1663`), best-effort.
func feishuErrorCode(raw string) string {
	if m := reFeishuErrorCode.FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	return ""
}

// feishuFailure classifies a raw connector/API error into a stable i18n code
// (mapped to a localized string on the frontend), an optional numeric Feishu
// error code for interpolation, and an English fallback message for clients
// without the i18n key. The raw status/JSON body/log_id is never returned here —
// it stays in the server logs. Dumping it in the UI is the anti-pattern
// Airbyte/Fivetran/Onyx warn against. Transient errors are retried next sync
// (the cursor is retained); auth/permission errors point at the fix instead.
func feishuFailure(err error) (code, codeValue, fallback string) {
	if err == nil {
		return "sync_failed", "", "Sync failed; will retry on the next sync"
	}
	s := strings.ToLower(err.Error())

	switch {
	case strings.Contains(s, "auth error"),
		strings.Contains(s, "invalid access token"),
		strings.Contains(s, "permission"),
		strings.Contains(s, "forbidden"),
		strings.Contains(s, "status=403"):
		return "feishu_auth_or_permission", "", "Authentication or permission error; check credentials and app scopes"
	case strings.Contains(s, "rate limited"), strings.Contains(s, "status=429"):
		return "feishu_rate_limited", "", "Feishu API rate limited; will retry on the next sync"
	case strings.Contains(s, "timed out"),
		strings.Contains(s, "timeout"),
		strings.Contains(s, "deadline exceeded"):
		return "feishu_timeout", "", "Export or request timed out; will retry on the next sync"
	case strings.Contains(s, "server error"):
		return "feishu_server_unavailable", "", "Feishu service temporarily unavailable; will retry on the next sync"
	case strings.Contains(s, "api error"),
		strings.Contains(s, "export task failed"),
		strings.Contains(s, "download failed"):
		if v := feishuErrorCode(err.Error()); v != "" {
			return "feishu_api_error", v, fmt.Sprintf("Feishu API error (code=%s); will retry on the next sync", v)
		}
		return "feishu_api_error_generic", "", "Feishu API error; will retry on the next sync"
	default:
		return "sync_failed", "", "Sync failed; will retry on the next sync"
	}
}

// FeishuErrorItemMeta builds the metadata for a failed item: the raw error (for
// server logs) plus the classified i18n code / param / fallback (for a
// localisable SyncItemError in the UI), merged with any caller-supplied extras.
func FeishuErrorItemMeta(err error, extra map[string]string) map[string]string {
	code, codeValue, fallback := feishuFailure(err)
	m := map[string]string{
		"error":             err.Error(),
		"error_reason_code": code,
		"error_reason":      fallback,
	}
	if codeValue != "" {
		m["error_reason_code_value"] = codeValue
	}
	maps.Copy(m, extra)
	return m
}

// parseableAttachmentExts are attachment extensions worth ingesting as their
// own knowledge entries; other files (icons, tiny decor) are skipped.
var parseableAttachmentExts = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true, ".md": true, ".csv": true,
}

// videoAttachmentExts are embedded/drive video files ingested as standalone
// video knowledge entries (transcribed timeline + keyframe captions). Gated
// by the data source's sync_video_attachments setting.
var videoAttachmentExts = map[string]bool{
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
	".webm": true, ".wmv": true, ".flv": true, ".m4v": true,
}

// IsVideoAttachmentExt reports whether a (dot-prefixed, lowercase) extension
// is a supported video attachment type.
func IsVideoAttachmentExt(ext string) bool { return videoAttachmentExts[strings.ToLower(ext)] }

// MinAttachmentBytes filters out decorative micro-files.
const MinAttachmentBytes = 2 * 1024

// maxLinkedPagesPerDoc caps how many hyperlinks of a single document are
// fanned out as web-page knowledge items, so a "link collection" document
// cannot flood one sync with hundreds of crawls. Deterministic (document
// order), so the same doc keeps the same pages across syncs.
const maxLinkedPagesPerDoc = 50

// feishuInternalHostSuffixes are the Feishu/Lark product domains. A link to
// these hosts is an in-tenant document or resource, not a public web page:
// the URL crawler has no session there (it would fetch a login page at best),
// and in-tenant documents are synced through the connector's own scope rather
// than smuggled in via hyperlinks.
var feishuInternalHostSuffixes = []string{
	"feishu.cn",
	"feishucdn.com",
	"larksuite.com",
	"larksuitecdn.com",
	"larkoffice.com",
	"larkenterprise.com",
}

// IsCrawlableLinkURL reports whether a document hyperlink points at a public
// http(s) web page that the URL ingestion pipeline may crawl. Feishu/Lark
// hosts and the document's own web host (docHost, covering custom
// web_base_url domains) are excluded; non-http(s) schemes (mailto:, tel:,
// javascript:) are rejected. Deeper SSRF validation (private/loopback IPs)
// happens again in CreateKnowledgeFromURL — this is the connector-side gate.
func IsCrawlableLinkURL(raw string, docHost string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if docHost != "" && host == strings.ToLower(docHost) {
		return false
	}
	for _, s := range feishuInternalHostSuffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return false
		}
	}
	return true
}

// hostOfURL returns the lowercase hostname of a URL, or "" if unparseable.
func hostOfURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// DefaultVideoMaxMB caps a single synced video download (UI-configurable via
// the data source's video_max_mb setting).
const DefaultVideoMaxMB = 2048

// FetchOptions carries per-sync-run behaviour derived from the data source
// config: the KB's multimodal capability plus video attachment settings.
type FetchOptions struct {
	Multimodal    bool
	SyncVideos    bool
	VideoMaxBytes int64

	// SyncLinkedPages fans hyperlinks found in document text out as URL-only
	// sub-items that the ingest layer crawls as web pages. Off by default —
	// link-heavy documents would otherwise trigger mass crawling the operator
	// never asked for.
	SyncLinkedPages bool

	// EmitEarly, when non-nil, hands a finished sub-item to the sync engine
	// immediately instead of accumulating it in the returned slice. Used for
	// multi-GB video attachments so each one is ingested (and its memory
	// released) before the next download starts, rather than holding every
	// video of a document in RAM until the whole node finishes. A non-nil
	// error aborts the fetch (the engine's Emit failed, e.g. canceled context).
	EmitEarly func(item *types.FetchedItem) error `json:"-"`
}

// FetchOptionsFromConfig reads connector settings with defaults: video
// attachment sync is on unless disabled, and the per-video size cap defaults
// to DefaultVideoMaxMB.
func FetchOptionsFromConfig(config *types.DataSourceConfig) FetchOptions {
	opts := FetchOptions{
		SyncVideos:    true,
		VideoMaxBytes: int64(DefaultVideoMaxMB) * 1024 * 1024,
	}
	if config == nil {
		return opts
	}
	opts.Multimodal = config.MultimodalEnabled
	if v, ok := config.Settings["sync_video_attachments"]; ok {
		if b, isBool := v.(bool); isBool {
			opts.SyncVideos = b
		}
	}
	if v, ok := config.Settings["sync_linked_pages"]; ok {
		if b, isBool := v.(bool); isBool {
			opts.SyncLinkedPages = b
		}
	}
	if v, ok := config.Settings["video_max_mb"]; ok {
		switch n := v.(type) {
		case float64:
			if n > 0 {
				opts.VideoMaxBytes = int64(n) * 1024 * 1024
			}
		case int:
			if n > 0 {
				opts.VideoMaxBytes = int64(n) * 1024 * 1024
			}
		}
	}
	return opts
}

// SupportedImageExt sniffs image bytes and returns the filename extension and
// content type Yuheng accepts for a standalone image knowledge item (png/jpg/
// gif/webp — the image set isValidFileType admits). ok is false for non-image
// or unsupported formats (e.g. bmp), which the caller skips rather than
// mislabel — a wrong extension would fail parsing. The detected content type is
// returned even when ok is false so the caller can log it without re-sniffing.
func SupportedImageExt(data []byte) (ext, contentType string, ok bool) {
	switch ct := http.DetectContentType(data); ct {
	case "image/png":
		return ".png", ct, true
	case "image/jpeg":
		return ".jpg", ct, true
	case "image/gif":
		return ".gif", ct, true
	case "image/webp":
		return ".webp", ct, true
	default:
		return "", ct, false
	}
}

// ParseFeishuConfig extracts and validates Feishu/Lark-specific configuration.
//
// base_url stays an explicit override so existing data sources that pointed a
// "feishu" connector at open.larksuite.com keep working; when it is unset the
// region's own host is filled in, making the resolved Config.BaseURL concrete
// for everything downstream.
func ParseFeishuConfig(config *types.DataSourceConfig, region Region) (*Config, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	credBytes, err := json.Marshal(config.Credentials)
	if err != nil {
		return nil, fmt.Errorf("marshal credentials: %w", err)
	}

	var feishuConfig Config
	if err := json.Unmarshal(credBytes, &feishuConfig); err != nil {
		return nil, fmt.Errorf("parse %s credentials: %w", region.ConnectorType, err)
	}

	if feishuConfig.AppID == "" || feishuConfig.AppSecret == "" {
		return nil, fmt.Errorf("%s app_id and app_secret are required", region.ConnectorType)
	}

	if feishuConfig.BaseURL == "" {
		feishuConfig.BaseURL = region.OpenBaseURL
	}

	// web_base_url is optional and only used to render user-facing links; it is
	// never fetched, but validate it anyway so a typo surfaces at save time.
	if feishuConfig.WebBaseURL != "" {
		if !strings.Contains(feishuConfig.WebBaseURL, "://") {
			feishuConfig.WebBaseURL = "https://" + feishuConfig.WebBaseURL
		}
		if err := datasource.ValidateConnectorBaseURL(feishuConfig.WebBaseURL); err != nil {
			return nil, fmt.Errorf("web_base_url: %w", err)
		}
	}

	// Timezone is a display setting (bitable date rendering), not a credential, so
	// it lives in Settings. Empty falls back to GMT+8 in resolveLocation.
	if feishuConfig.Timezone == "" && config.Settings != nil {
		if tz, ok := config.Settings["timezone"].(string); ok {
			feishuConfig.Timezone = strings.TrimSpace(tz)
		}
	}

	if err := datasource.ValidateConnectorBaseURL(feishuConfig.GetBaseURL()); err != nil {
		return nil, err
	}

	return &feishuConfig, nil
}

// IsSupportedDocType checks if a Feishu document type can be synced.
// mindnote and slides have no content read API and are skipped.
func IsSupportedDocType(objType string) bool {
	switch objType {
	case "docx", "doc", "sheet", "bitable", "file":
		return true
	default:
		// mindnote, slides — no content retrieval API available
		return false
	}
}

// ParseFeishuTimestamp parses a Feishu unix timestamp string (seconds) into time.Time.
func ParseFeishuTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// SanitizeFileName removes characters that are invalid in filenames and
// truncates at a UTF-8 rune boundary. Raw byte truncation would split a
// multi-byte codepoint (Chinese chars are 3 bytes) and produce invalid UTF-8
// that downstream validation (utf8.ValidString) rejects.
//
// The extension is preserved across truncation: only the base name is trimmed,
// so a long attachment name like "很长的名字….pdf" keeps its ".pdf" suffix that
// downstream file-type classification depends on.
func SanitizeFileName(name string) string {
	if name == "" {
		return "untitled"
	}
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	result := replacer.Replace(name)
	const maxBytes = 200
	if len(result) <= maxBytes {
		return result
	}
	ext := filepath.Ext(result)
	if len(ext) >= maxBytes {
		// pathological: extension alone overflows the budget → drop it
		ext = ""
	}
	base := truncateUTF8(result[:len(result)-len(ext)], maxBytes-len(ext))
	return base + ext
}

// truncateUTF8 shortens s to at most maxBytes bytes without splitting a
// multi-byte rune: after a hard byte cut it trims any trailing partial codepoint.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r != utf8.RuneError || size != 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// DocxFetchInput is the unified description of one docx document from either
// source (wiki node or Drive file) that FetchDocxWithBlocks needs.
type DocxFetchInput struct {
	// Yuheng external_id: wiki=node.NodeToken, drive=file.Token
	DocToken string
	// Feishu docx document token
	ObjToken          string
	Title             string
	URL               string
	ResourceID        string
	EditTime          time.Time
	BaseMeta          map[string]string
	MultimodalEnabled bool
	// SyncVideos enables downloading embedded video attachments as standalone
	// video knowledge entries; VideoMaxBytes caps one video's download size.
	SyncVideos    bool
	VideoMaxBytes int64
	// SyncLinkedPages fans document hyperlinks out as crawlable web-page
	// sub-items (see FetchOptions.SyncLinkedPages).
	SyncLinkedPages bool
	// EmitEarly mirrors FetchOptions.EmitEarly for the docx fetch path.
	EmitEarly func(item *types.FetchedItem) error
}

// emitVideoItem routes one fetched video sub-item: through EmitEarly when the
// engine provided it (ingest now, free the bytes), otherwise into items.
func emitVideoItem(in DocxFetchInput, items []*types.FetchedItem, vi *types.FetchedItem) ([]*types.FetchedItem, error) {
	if in.EmitEarly != nil {
		return items, in.EmitEarly(vi)
	}
	return append(items, vi), nil
}

// videoAttachmentChildMeta labels a fanned-out embedded video sub-item.
func videoAttachmentChildMeta(in DocxFetchInput) map[string]string {
	m := maps.Clone(in.BaseMeta)
	m["parent_node_token"] = in.DocToken
	m["attachment"] = "true"
	m["embedded_video"] = "true"
	return m
}

// fetchVideoAttachmentItem downloads one embedded video block and wraps it as
// a standalone FetchedItem that flows into the video ingestion pipeline
// (timeline transcription + keyframe captions). Returns nil when the download
// fails permanently in a way worth recording (an error item is returned
// instead) or when the video should be skipped.
func fetchVideoAttachmentItem(
	ctx context.Context, client *Client, in DocxFetchInput, a pendingAttachment, childID string,
) *types.FetchedItem {
	maxBytes := in.VideoMaxBytes
	if maxBytes <= 0 {
		maxBytes = int64(DefaultVideoMaxMB) * 1024 * 1024
	}
	// Surface the long download in the sync log's live progress — a multi-GB
	// video is the one stage where the sync sits silent for minutes otherwise.
	datasource.ReportProgress(ctx, "download_video", a.Name)
	data, err := client.downloadMediaFileLimit(ctx, a.FileToken, maxBytes)
	if err != nil {
		logger.Warnf(ctx, "[Feishu] doc %s: video attachment %q (token=%s) download failed: %v",
			in.ObjToken, a.Name, a.FileToken, err)
		return &types.FetchedItem{
			ExternalID:       childID,
			Title:            a.Name,
			SourceResourceID: in.ResourceID,
			Metadata:         FeishuErrorItemMeta(err, videoAttachmentChildMeta(in)),
		}
	}
	logger.Infof(ctx, "[Feishu] doc %s: embedded video %q downloaded (%d bytes)",
		in.ObjToken, a.Name, len(data))
	return &types.FetchedItem{
		ExternalID:       childID,
		Title:            a.Name,
		Content:          data,
		ContentType:      "application/octet-stream",
		FileName:         SanitizeFileName(a.Name),
		URL:              in.URL,
		UpdatedAt:        in.EditTime,
		SourceResourceID: in.ResourceID,
		Metadata:         videoAttachmentChildMeta(in),
	}
}

// linkedPageChildMeta labels a fanned-out linked web page sub-item.
func linkedPageChildMeta(in DocxFetchInput, pageURL string) map[string]string {
	m := maps.Clone(in.BaseMeta)
	m["parent_node_token"] = in.DocToken
	m["linked_page"] = "true"
	m["linked_page_url"] = pageURL
	return m
}

// linkChildToken derives a stable child token from a link URL. The raw URL
// cannot be embedded in the external_id: it can be arbitrarily long and may
// itself contain '#', the SubtreeChildID separator. A short digest keeps the
// ID stable across syncs so unchanged links are recognized as updates.
func linkChildToken(u string) string {
	sum := sha256.Sum256([]byte(u))
	return hex.EncodeToString(sum[:8])
}

// appendLinkedPageItems fans a document's hyperlinks out as URL-only
// sub-items. The ingest layer routes an item that has a URL but no Content to
// CreateKnowledgeFromURL, which crawls and parses the page — the connector
// itself never fetches external sites. Returns the extended items and keep
// slices; keep entries make the links immune to the stale-subtree sweep.
func appendLinkedPageItems(
	ctx context.Context, in DocxFetchInput, blocks []DocxBlock,
	items []*types.FetchedItem, keep []string,
) ([]*types.FetchedItem, []string) {
	docHost := hostOfURL(in.URL)
	added := 0
	skipped := 0
	for _, l := range collectDocLinks(blocks) {
		if !IsCrawlableLinkURL(l.URL, docHost) {
			continue
		}
		if added >= maxLinkedPagesPerDoc {
			skipped++
			continue
		}
		added++
		childID := types.SubtreeChildID(in.DocToken, "link", linkChildToken(l.URL))
		keep = append(keep, childID)
		items = append(items, &types.FetchedItem{
			ExternalID:       childID,
			Title:            l.Title,
			URL:              l.URL,
			UpdatedAt:        in.EditTime,
			SourceResourceID: in.ResourceID,
			Metadata:         linkedPageChildMeta(in, l.URL),
		})
	}
	if skipped > 0 {
		logger.Warnf(ctx, "[Feishu] doc %s: %d linked page(s) beyond the per-document cap of %d were skipped",
			in.ObjToken, skipped, maxLinkedPagesPerDoc)
	}
	if added > 0 {
		logger.Infof(ctx, "[Feishu] doc %s: fanned out %d linked page(s) for crawling", in.ObjToken, added)
	}
	return items, keep
}

// collectVideoAttachments extracts embedded video File blocks from a block
// list (used by the export parsing mode, whose .docx output carries no
// videos).
func collectVideoAttachments(blocks []DocxBlock) []pendingAttachment {
	var vids []pendingAttachment
	for _, b := range blocks {
		if b.BlockType != BlockTypeFile || b.File == nil || b.File.Token == "" {
			continue
		}
		if IsVideoAttachmentExt(filepath.Ext(b.File.Name)) {
			vids = append(vids, pendingAttachment{FileToken: b.File.Token, Name: b.File.Name})
		}
	}
	return vids
}

// FetchDocxWithBlocks retrieves a docx document via the blocks API, converts it
// to Markdown, and returns a main item plus any parseable attachment/image
// sub-items. Falls back to the export API if the blocks API errors or renders
// empty. Shared by the wiki Connector and the Drive DriveConnector.
func FetchDocxWithBlocks(ctx context.Context, client *Client, in DocxFetchInput) ([]*types.FetchedItem, error) {
	// FEISHU_DOCX_PARSE_MODE selects the docx parsing path. The blocks path
	// renders image blocks as empty `![图片]()` placeholders and fans images out
	// into separate knowledge items, which breaks image↔document association in
	// retrieval/wiki/agent. The export path yields a .docx that docreader parses
	// inline, so images are bound to the parent document via parent_chunk_id
	// (same as a regular docx upload). Default (unset / "export") uses export so
	// images associate with the document; set "blocks" for the blocks-first
	// behaviour (faster, keeps docx attachments, but images are detached).

	// This is a temporary solution. If a better parsing solution is available later, this environment variable will be removed and replaced with a better one.
	parsingMode := strings.TrimSpace(os.Getenv("FEISHU_DOCX_PARSE_MODE"))
	if parsingMode == "" {
		parsingMode = "export"
	}

	if strings.EqualFold(parsingMode, "export") {
		item, err := exportDocxFallback(ctx, client, in)
		if err != nil {
			return nil, err
		}
		items := []*types.FetchedItem{item}
		// Exported .docx output carries no embedded videos and loses hyperlink
		// URLs — scan the block list separately so document videos still become
		// video knowledge and linked pages still get crawled.
		if in.SyncVideos || in.SyncLinkedPages {
			blocks, berr := client.listDocumentBlocks(ctx, in.ObjToken)
			if berr != nil {
				logger.Warnf(ctx, "[Feishu] doc %s: block scan for videos/linked pages failed (skipped this sync): %v",
					in.ObjToken, berr)
				return items, nil
			}
			keep := make([]string, 0, 4)
			if in.SyncVideos {
				for _, a := range collectVideoAttachments(blocks) {
					childID := types.SubtreeChildID(in.DocToken, "file", a.FileToken)
					keep = append(keep, childID)
					if vi := fetchVideoAttachmentItem(ctx, client, in, a, childID); vi != nil {
						var eerr error
						if items, eerr = emitVideoItem(in, items, vi); eerr != nil {
							return nil, eerr
						}
					}
				}
			}
			if in.SyncLinkedPages {
				items, keep = appendLinkedPageItems(ctx, in, blocks, items, keep)
			}
			// Sweep video/link sub-items that disappeared from the doc. Safe here:
			// the block list was fetched successfully, so keep is authoritative.
			item.ReplacesSubtree = true
			item.SubtreeKeep = keep
		}
		return items, nil
	}

	blocks, err := client.listDocumentBlocks(ctx, in.ObjToken)
	if err != nil {
		logger.Warnf(ctx, "[Feishu] blocks API failed for %s (%s), falling back to export: %v",
			in.Title, in.ObjToken, err)
		item, ferr := exportDocxFallback(ctx, client, in)
		if ferr != nil {
			return nil, ferr
		}
		// Do NOT set ReplacesSubtree here (see the wiki history: a transient
		// blocks failure must not sweep good attachment children from the prior
		// blocks-path sync with nothing to replace them).
		return []*types.FetchedItem{item}, nil
	}

	md, atts, err := blocksToMarkdown(ctx, client, blocks)
	if err != nil {
		return nil, fmt.Errorf("convert blocks %s: %w", in.Title, err)
	}

	if len(strings.TrimSpace(string(md))) == 0 {
		logger.Infof(ctx, "[Feishu] doc %s (%s): blocks rendered empty Markdown, falling back to export",
			in.Title, in.ObjToken)
		item, ferr := exportDocxFallback(ctx, client, in)
		if ferr != nil {
			return nil, ferr
		}
		return []*types.FetchedItem{item}, nil
	}

	main := &types.FetchedItem{
		ExternalID:       in.DocToken,
		Title:            in.Title,
		Content:          md,
		ContentType:      "text/markdown",
		FileName:         SanitizeFileName(in.Title) + ".md",
		URL:              in.URL,
		UpdatedAt:        in.EditTime,
		SourceResourceID: in.ResourceID,
		Metadata:         in.BaseMeta,
		ReplacesSubtree:  true, // sweep stale attachment sub-items on re-sync
	}
	items := []*types.FetchedItem{main}

	keep := make([]string, 0, len(atts))
	childMeta := func() map[string]string {
		m := maps.Clone(in.BaseMeta)
		m["parent_node_token"] = in.DocToken
		m["attachment"] = "true"
		return m
	}
	for _, a := range atts {
		childID := types.SubtreeChildID(in.DocToken, "file", a.FileToken)
		keep = append(keep, childID) // present in the doc → never sweep as stale
		ext := strings.ToLower(filepath.Ext(a.Name))
		if ext == "" {
			logger.Warnf(ctx, "[Feishu] doc %s: skipping attachment with no usable filename (token=%s name=%q)",
				in.ObjToken, a.FileToken, a.Name)
			continue
		}
		if IsVideoAttachmentExt(ext) {
			if !in.SyncVideos {
				continue
			}
			if vi := fetchVideoAttachmentItem(ctx, client, in, a, childID); vi != nil {
				var eerr error
				if items, eerr = emitVideoItem(in, items, vi); eerr != nil {
					return nil, eerr
				}
			}
			continue
		}
		if !parseableAttachmentExts[ext] {
			continue
		}
		data, derr := client.downloadMediaFile(ctx, a.FileToken)
		if derr != nil {
			logger.Warnf(ctx, "[Feishu] doc %s: attachment %q (token=%s) download failed: %v",
				in.ObjToken, a.Name, a.FileToken, derr)
			items = append(items, &types.FetchedItem{
				ExternalID:       childID,
				Title:            a.Name,
				SourceResourceID: in.ResourceID,
				Metadata:         FeishuErrorItemMeta(derr, childMeta()),
			})
			continue
		}
		if len(data) < MinAttachmentBytes {
			logger.Infof(ctx, "[Feishu] doc %s: skipping tiny attachment %q (token=%s, %d bytes < %d)",
				in.ObjToken, a.Name, a.FileToken, len(data), MinAttachmentBytes)
			continue
		}
		items = append(items, &types.FetchedItem{
			ExternalID:       childID,
			Title:            a.Name,
			Content:          data,
			ContentType:      "application/octet-stream",
			FileName:         SanitizeFileName(a.Name),
			URL:              in.URL,
			UpdatedAt:        in.EditTime,
			SourceResourceID: in.ResourceID,
			Metadata:         childMeta(),
		})
	}

	imgMeta := func() map[string]string {
		m := maps.Clone(in.BaseMeta)
		m["parent_node_token"] = in.DocToken
		m["embedded_image"] = "true"
		return m
	}
	for _, b := range blocks {
		if b.BlockType != BlockTypeImage || b.Image == nil || b.Image.Token == "" {
			continue
		}
		childID := types.SubtreeChildID(in.DocToken, "image", b.Image.Token)
		keep = append(keep, childID) // present in the doc → never sweep as stale
		if !in.MultimodalEnabled {
			continue // KB can't OCR images; the inline placeholder is all we keep
		}
		data, derr := client.downloadMediaFile(ctx, b.Image.Token)
		if derr != nil {
			logger.Warnf(ctx, "[Feishu] doc %s: image (token=%s) download failed: %v",
				in.ObjToken, b.Image.Token, derr)
			items = append(items, &types.FetchedItem{
				ExternalID:       childID,
				Title:            fmt.Sprintf("%s（内嵌图片）", in.Title),
				SourceResourceID: in.ResourceID,
				Metadata:         FeishuErrorItemMeta(derr, imgMeta()),
			})
			continue
		}
		if len(data) < MinAttachmentBytes {
			continue // decorative micro-image (icon/spacer)
		}
		ext, contentType, ok := SupportedImageExt(data)
		if !ok {
			logger.Warnf(ctx, "[Feishu] doc %s: skipping image (token=%s) of unsupported type %q",
				in.ObjToken, b.Image.Token, contentType)
			continue
		}
		items = append(items, &types.FetchedItem{
			ExternalID:       childID,
			Title:            fmt.Sprintf("%s（内嵌图片）", in.Title),
			Content:          data,
			ContentType:      contentType,
			FileName:         "image-" + b.Image.Token + ext,
			URL:              in.URL,
			UpdatedAt:        in.EditTime,
			SourceResourceID: in.ResourceID,
			Metadata:         imgMeta(),
		})
	}

	if in.SyncLinkedPages {
		items, keep = appendLinkedPageItems(ctx, in, blocks, items, keep)
	}

	main.SubtreeKeep = keep
	return items, nil
}

// exportDocxFallback exports a docx document via the async export API and
// returns a single FetchedItem containing the exported .docx binary. Used by
// FetchDocxWithBlocks when the blocks API is unavailable or renders empty.
func exportDocxFallback(ctx context.Context, client *Client, in DocxFetchInput) (*types.FetchedItem, error) {
	data, fileName, err := client.ExportAndDownload(ctx, in.ObjToken, "docx")
	if err != nil {
		return nil, fmt.Errorf("export %s (docx): %w", in.Title, err)
	}

	ext := ExportFileExtToSuffix[ObjTypeToExportFileExtension["docx"]]
	if fileName == "" {
		fileName = SanitizeFileName(in.Title) + ext
	} else if !strings.HasSuffix(strings.ToLower(fileName), ext) {
		fileName = SanitizeFileName(fileName) + ext
	}

	return &types.FetchedItem{
		ExternalID:       in.DocToken,
		Title:            in.Title,
		Content:          data,
		ContentType:      "application/octet-stream",
		FileName:         fileName,
		URL:              in.URL,
		UpdatedAt:        in.EditTime,
		SourceResourceID: in.ResourceID,
		Metadata:         in.BaseMeta,
	}, nil
}
