package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/event"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/storageurl"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

const (
	maxAttachmentUploadsPerRequest = 5
	maxAttachmentUploadTotalBytes  = int64(100 * 1024 * 1024)
)

// qaRequestContext holds all the common data needed for QA requests
type qaRequestContext struct {
	ctx                   context.Context
	c                     *gin.Context
	sessionID             string
	requestID             string
	receivedAt            time.Time // Wall-clock time the handler started processing the request
	query                 string
	session               *types.Session
	assistantMessage      *types.Message
	knowledgeBaseIDs      []string
	knowledgeIDs          []string
	tagScopes             []types.TagScope
	tagIDs                []string
	summaryModelID        string
	webSearchEnabled      bool
	mentionedItems        types.MentionedItems
	effectiveTenantID     uint64                   // retrieval/model tenant; 0 = use context tenant
	images                []ImageAttachment        // Uploaded images with analysis text
	userMessageID         string                   // Created user message ID (populated after createUserMessage)
	userCreatedAt         time.Time                // Persisted user message timestamp, echoed on agent_query
	channel               string                   // Source channel: "web", "api", etc.
	attachments           types.MessageAttachments // Processed base64 file attachments (legacy inline uploads)
	attachmentIDs         []string                 // Pre-uploaded session-scoped document IDs, resolved after SSE starts
	attachmentMetas       types.MessageAttachments // Metadata-only view of attachmentIDs for the persisted user message
	suggestionAttribution *types.SuggestionAttribution
	// resourceRewriter turns internal storage references in the outbound stream
	// into directly loadable URLs when the caller asks for `resource_urls=public`.
	// Disabled (a pass-through) in the default handle mode.
	resourceRewriter *storageurl.StreamRewriter
}

// buildQARequest converts the qaRequestContext into a types.QARequest for service invocation.
func (rc *qaRequestContext) buildQARequest() *types.QARequest {
	imageURLs, imageDescription := extractImageURLsAndOCRText(rc.images)
	return &types.QARequest{
		Session:            rc.session,
		Query:              rc.query,
		AssistantMessageID: rc.assistantMessage.ID,
		SummaryModelID:     rc.summaryModelID,
		KnowledgeBaseIDs:   rc.knowledgeBaseIDs,
		KnowledgeIDs:       rc.knowledgeIDs,
		TagScopes:          rc.tagScopes,
		ImageURLs:          imageURLs,
		ImageDescription:   imageDescription,
		UserMessageID:      rc.userMessageID,
		WebSearchEnabled:   rc.webSearchEnabled,
		Attachments:        rc.attachments,
	}
}

// parseQARequest parses and validates a QA request, returns the request context
func (h *Handler) parseQARequest(c *gin.Context, logPrefix string) (*qaRequestContext, *CreateKnowledgeQARequest, error) {
	receivedAt := time.Now()
	ctx := logger.CloneContext(c.Request.Context())
	requestID := secutils.SanitizeForLog(c.GetString(types.RequestIDContextKey.String()))
	logger.Infof(ctx, "[%s] TTFB:start request_id=%s received_at=%d",
		logPrefix, requestID, receivedAt.UnixMilli())

	// Get session ID from URL parameter
	sessionID := secutils.SanitizeForLog(c.Param("session_id"))
	if sessionID == "" {
		logger.Error(ctx, "Session ID is empty")
		return nil, nil, errors.NewBadRequestError(errors.ErrInvalidSessionID.Error())
	}

	// Parse request body
	var request CreateKnowledgeQARequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to parse request data", err)
		return nil, nil, errors.NewBadRequestError(err.Error())
	}

	// Validate query content
	if request.Query == "" {
		logger.Error(ctx, "Query content is empty")
		return nil, nil, errors.NewBadRequestError("Query content cannot be empty")
	}

	// Resolve the storage-reference representation up front: once the SSE stream
	// has started an invalid value can no longer be reported as a 400.
	resourceRewriter, err := h.resolveStreamRewriter(c)
	if err != nil {
		logger.Warnf(ctx, "Rejected resource URL mode: %v", err)
		return nil, nil, err
	}
	if h.suggestionService != nil && request.SuggestionAttribution != nil {
		if err := h.suggestionService.ValidateAttribution(ctx, sessionID, request.Query, request.SuggestionAttribution); err != nil {
			return nil, nil, errors.NewBadRequestError("invalid suggestion attribution")
		}
	}

	// SSRF protection: strip client-supplied URL/Caption fields from image attachments.
	// The URL field must only be populated server-side by saveImageAttachments; an
	// attacker could inject internal network URLs to trigger SSRF via the LLM provider.
	for i := range request.Images {
		request.Images[i].URL = ""
		request.Images[i].Caption = ""
	}

	// Log request details
	if requestJSON, err := json.Marshal(request); err == nil {
		logger.Infof(ctx, "[%s] Request: session_id=%s, request=%s",
			logPrefix, sessionID, secutils.SanitizeForLog(secutils.CompactImageDataURLForLog(string(requestJSON))))
	}

	// Get session. QA writes new messages into the session, so use the strict
	// owner scope: a tenant admin may read an API-key session but must not be
	// able to post messages to it (which would otherwise fail later at message
	// creation with a 500 instead of a clean not-found).
	session, err := h.sessionService.GetOwnedSession(ctx, sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session, session ID: %s, error: %v", sessionID, err)
		return nil, nil, errors.NewNotFoundError("Session not found")
	}

	// Merge @mentioned items into knowledge_base_ids and knowledge_ids
	kbIDs, knowledgeIDs := mergeKnowledgeTargets(request.KnowledgeBaseIDs, request.KnowledgeIds, request.MentionedItems)
	if err := types.AuthorizeTenantAPIKeyKnowledgeTargets(ctx, kbIDs, knowledgeIDs); err != nil {
		return nil, nil, err
	}

	// The wiki fixer is invoked from the wiki editor with its dedicated agent
	// marker. It no longer resolves to an agent; the marker only triggers the
	// shared-KB tenant scope below so a fix run over an org-shared wiki
	// resolves models/retrieval in the source workspace.
	var effectiveTenantID uint64
	if secutils.SanitizeForLog(request.AgentID) == BuiltinWikiFixerAgentID {
		effectiveTenantID = h.resolveWikiFixerTenantScope(
			ctx,
			c.GetUint64(types.TenantIDContextKey.String()),
			types.TenantRoleFromContext(ctx),
			kbIDs,
		)
	}

	// Log merge results for debugging
	logger.Infof(ctx, "[%s] @mention merge: request.KnowledgeBaseIDs=%v, request.MentionedItems=%d, merged kbIDs=%v, merged knowledgeIDs=%v",
		logPrefix, request.KnowledgeBaseIDs, len(request.MentionedItems), kbIDs, knowledgeIDs)

	// Process inline base64 images: decode and save to storage.
	// VLM analysis for RAG paths is deferred to the pipeline rewrite step.
	// For pure chat paths with non-vision models, VLM analysis runs here as fallback.
	if len(request.Images) > 0 {
		tenantID := c.GetUint64(types.TenantIDContextKey.String())
		if err := h.saveImageAttachments(ctx, request.Images, tenantID, ""); err != nil {
			logger.Errorf(ctx, "[%s] Failed to save images: %v", logPrefix, err)
			return nil, nil, errors.NewBadRequestError(fmt.Sprintf("Image save failed: %v", err))
		}
	}

	// Process file attachments: decode and save to storage, extract content
	var processedAttachments types.MessageAttachments
	if len(request.AttachmentUploads) > 0 {
		logger.Infof(ctx, "[%s] processing %d attachment(s)", logPrefix, len(request.AttachmentUploads))

		// Decode first and validate actual bytes. Client-declared file_size is
		// metadata only and must not be trusted for memory limits.
		maxSize := secutils.GetMaxFileSize()
		decodedAttachments, decodeErr := decodeAndValidateAttachmentUploads(
			request.AttachmentUploads,
			maxAttachmentUploadsPerRequest,
			maxSize,
			maxAttachmentUploadTotalBytes,
		)
		if decodeErr != nil {
			return nil, nil, errors.NewBadRequestError(decodeErr.Error())
		}

		tenantID := c.GetUint64(types.TenantIDContextKey.String())

		// Process all attachments concurrently.
		processedAttachments = make(types.MessageAttachments, len(request.AttachmentUploads))
		var wg sync.WaitGroup
		errChan := make(chan error, len(request.AttachmentUploads))

		for i, upload := range request.AttachmentUploads {
			wg.Add(1)
			go func(idx int, att AttachmentUpload) {
				defer wg.Done()

				processed, err := h.attachmentProcessor.ProcessAttachment(
					ctx, decodedAttachments[idx], att.FileName, int64(len(decodedAttachments[idx])), tenantID, "",
				)
				if err != nil {
					errChan <- fmt.Errorf("attachment %d processing failed: %w", idx+1, err)
					return
				}

				processedAttachments[idx] = *processed
			}(i, upload)
		}

		wg.Wait()
		close(errChan)

		if len(errChan) > 0 {
			err := <-errChan
			logger.Errorf(ctx, "[%s] attachment processing failed: %v", logPrefix, err)
			return nil, nil, errors.NewBadRequestError(fmt.Sprintf("attachment processing failed: %v", err))
		}

		logger.Infof(ctx, "[%s] all attachments processed", logPrefix)
	}

	// Pre-uploaded documents may still be parsing. Only fetch their metadata
	// here (fast, available even while processing) to record the attachments on
	// the persisted user message. The heavy content selection happens after the
	// SSE stream is up, so the send is not blocked while parsing finishes (see
	// resolveTemporaryAttachments).
	var attachmentIDs []string
	var attachmentMetas types.MessageAttachments
	if len(request.AttachmentIDs) > 0 {
		normalizedIDs, normErr := normalizeTemporaryAttachmentIDs(request.AttachmentIDs)
		if normErr != nil {
			return nil, nil, errors.NewBadRequestError(normErr.Error())
		}
		tenantID := session.TenantID
		attachmentMetas = make(types.MessageAttachments, 0, len(normalizedIDs))
		for _, id := range normalizedIDs {
			doc, getErr := h.temporaryDocuments.Get(ctx, tenantID, sessionID, id)
			if getErr != nil || doc == nil {
				return nil, nil, errors.NewBadRequestError(
					fmt.Sprintf("attachment %s was not found in this session", secutils.SanitizeForLog(id)))
			}
			attachmentMetas = append(attachmentMetas, types.MessageAttachment{
				ID: doc.ID, URL: doc.ResourceRef, FileName: doc.FileName,
				FileType: doc.FileType, FileSize: doc.FileSize,
			})
		}
		attachmentIDs = normalizedIDs
	}

	mentionScopes := tagScopesFromMentionedItems(request.MentionedItems)
	requestTagIDs := dedupRequestStrings(request.TagIDs)
	if err := validateUnscopedTagIDs(orphanTagIDsForScope(requestTagIDs, mentionScopes), secutils.SanitizeForLogArray(kbIDs)); err != nil {
		return nil, nil, errors.NewBadRequestError(err.Error())
	}
	tagScopes := mergeTagScopesFromRequestIDs(mentionScopes, requestTagIDs, secutils.SanitizeForLogArray(kbIDs))
	tagIDs := dedupRequestStrings(request.TagIDs)
	executionContext := buildMessageExecutionContext(
		ctx,
		secutils.SanitizeForLogArray(kbIDs),
		secutils.SanitizeForLogArray(knowledgeIDs),
		secutils.SanitizeForLogArray(tagIDs),
		tagScopes,
		request.WebSearchEnabled,
	)

	// Build request context
	reqCtx := &qaRequestContext{
		ctx:        ctx,
		c:          c,
		sessionID:  sessionID,
		requestID:  requestID,
		receivedAt: receivedAt,
		query:      request.Query,
		session:    session,
		assistantMessage: &types.Message{
			SessionID:        sessionID,
			Role:             "assistant",
			RequestID:        c.GetString(types.RequestIDContextKey.String()),
			IsCompleted:      false,
			Channel:          request.Channel,
			ModelID:          request.SummaryModelID,
			ExecutionContext: executionContext,
		},
		knowledgeBaseIDs:      secutils.SanitizeForLogArray(kbIDs),
		knowledgeIDs:          secutils.SanitizeForLogArray(knowledgeIDs),
		tagScopes:             tagScopes,
		tagIDs:                secutils.SanitizeForLogArray(tagIDs),
		summaryModelID:        secutils.SanitizeForLog(request.SummaryModelID),
		webSearchEnabled:      request.WebSearchEnabled,
		mentionedItems:        convertMentionedItems(request.MentionedItems),
		effectiveTenantID:     effectiveTenantID,
		images:                request.Images,
		channel:               request.Channel,
		attachments:           processedAttachments,
		attachmentIDs:         attachmentIDs,
		attachmentMetas:       attachmentMetas,
		suggestionAttribution: request.SuggestionAttribution,
		resourceRewriter:      resourceRewriter,
	}

	return reqCtx, &request, nil
}

func decodeAndValidateAttachmentUploads(
	uploads []AttachmentUpload,
	maxCount int,
	maxFileBytes, maxTotalBytes int64,
) ([][]byte, error) {
	if len(uploads) > maxCount {
		return nil, fmt.Errorf("at most %d attachments are allowed per request", maxCount)
	}
	decoded := make([][]byte, len(uploads))
	var total int64
	for i, upload := range uploads {
		data, err := DecodeBase64Attachment(upload.Data)
		if err != nil {
			return nil, fmt.Errorf("attachment %d decode failed: %w", i+1, err)
		}
		actualSize := int64(len(data))
		if actualSize > maxFileBytes {
			return nil, fmt.Errorf("attachment %d exceeds size limit of %d bytes", i+1, maxFileBytes)
		}
		total += actualSize
		if total > maxTotalBytes {
			return nil, fmt.Errorf("attachments exceed total request limit of %d bytes", maxTotalBytes)
		}
		decoded[i] = data
	}
	return decoded, nil
}

// buildMessageExecutionContext snapshots the per-turn request state used by
// derived experiences such as follow-up suggestions. The returned hash
// invalidates cached suggestion sets when the request scope changes.
func buildMessageExecutionContext(
	ctx context.Context,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
	tagIDs []string,
	tagScopes []types.TagScope,
	webSearchEnabled bool,
) types.MessageExecutionContext {
	locale := types.LanguageFromContextOrDefault(ctx)

	snapshot := types.MessageExecutionContext{
		KnowledgeBaseIDs: knowledgeBaseIDs,
		KnowledgeIDs:     knowledgeIDs,
		TagIDs:           tagIDs,
		TagScopes:        cloneTagScopes(tagScopes),
		WebSearchEnabled: webSearchEnabled,
		Locale:           locale,
	}
	hashInput := struct {
		KnowledgeBaseIDs []string         `json:"knowledge_base_ids,omitempty"`
		KnowledgeIDs     []string         `json:"knowledge_ids,omitempty"`
		TagIDs           []string         `json:"tag_ids,omitempty"`
		TagScopes        []types.TagScope `json:"tag_scopes,omitempty"`
	}{
		KnowledgeBaseIDs: knowledgeBaseIDs,
		KnowledgeIDs:     knowledgeIDs,
		TagIDs:           tagIDs,
		TagScopes:        snapshot.TagScopes,
	}
	if encoded, err := json.Marshal(hashInput); err == nil {
		hash := sha256.Sum256(encoded)
		snapshot.ConfigHash = fmt.Sprintf("%x", hash[:])
	}

	return snapshot
}

func cloneTagScopes(scopes []types.TagScope) []types.TagScope {
	if len(scopes) == 0 {
		return nil
	}
	cloned := make([]types.TagScope, 0, len(scopes))
	for _, scope := range scopes {
		if scope.KnowledgeBaseID == "" || len(scope.TagIDs) == 0 {
			continue
		}
		cloned = append(cloned, types.TagScope{
			KnowledgeBaseID: scope.KnowledgeBaseID,
			TagIDs:          append([]string(nil), scope.TagIDs...),
		})
	}
	return cloned
}

// mergeKnowledgeTargets merges request KB/knowledge IDs with @mentioned items into deduplicated slices.
func mergeKnowledgeTargets(requestKBIDs []string, requestKnowledgeIDs []string, mentionedItems []MentionedItemRequest) (kbIDs []string, knowledgeIDs []string) {
	kbIDSet := make(map[string]bool)
	kbIDs = make([]string, 0, len(requestKBIDs)+len(mentionedItems))
	for _, id := range requestKBIDs {
		if id != "" && !kbIDSet[id] {
			kbIDs = append(kbIDs, id)
			kbIDSet[id] = true
		}
	}

	knowledgeIDSet := make(map[string]bool)
	knowledgeIDs = make([]string, 0, len(requestKnowledgeIDs)+len(mentionedItems))
	for _, id := range requestKnowledgeIDs {
		if id != "" && !knowledgeIDSet[id] {
			knowledgeIDs = append(knowledgeIDs, id)
			knowledgeIDSet[id] = true
		}
	}

	for _, item := range mentionedItems {
		if item.ID == "" {
			continue
		}
		switch item.Type {
		case "kb":
			if !kbIDSet[item.ID] {
				kbIDs = append(kbIDs, item.ID)
				kbIDSet[item.ID] = true
			}
		case "file":
			if !knowledgeIDSet[item.ID] {
				knowledgeIDs = append(knowledgeIDs, item.ID)
				knowledgeIDSet[item.ID] = true
			}
		}
	}
	return kbIDs, knowledgeIDs
}

// sseStreamContext holds the context for SSE streaming
type sseStreamContext struct {
	eventBus         *event.EventBus
	asyncCtx         context.Context
	cancel           context.CancelFunc
	assistantMessage *types.Message
}

// setupSSEStream sets up the SSE streaming context
func (h *Handler) setupSSEStream(reqCtx *qaRequestContext, generateTitle bool) *sseStreamContext {
	// Set SSE headers
	setSSEHeaders(reqCtx.c)

	// Write initial agent_query event
	h.writeAgentQueryEvent(
		reqCtx.ctx,
		reqCtx.sessionID,
		reqCtx.userMessageID,
		reqCtx.userCreatedAt,
		reqCtx.assistantMessage,
	)

	// Base context for async work. The wiki fixer borrows the source
	// workspace of a shared KB so KB-scoped models resolve there; everyone
	// else runs in the caller's tenant.
	baseCtx := reqCtx.ctx
	if reqCtx.effectiveTenantID != 0 && h.tenantService != nil {
		if tenant, err := h.tenantService.GetTenantByID(reqCtx.ctx, reqCtx.effectiveTenantID); err == nil && tenant != nil {
			baseCtx = context.WithValue(context.WithValue(reqCtx.ctx, types.TenantIDContextKey, reqCtx.effectiveTenantID), types.TenantInfoContextKey, tenant)
			logger.Infof(reqCtx.ctx, "Using effective tenant %d for shared-KB chat (model/KB resolution)", reqCtx.effectiveTenantID)
		}
	}

	// Create EventBus and cancellable context
	eventBus := event.NewEventBus()
	asyncCtx, cancel := context.WithCancel(logger.CloneContext(baseCtx))

	streamCtx := &sseStreamContext{
		eventBus:         eventBus,
		asyncCtx:         asyncCtx,
		cancel:           cancel,
		assistantMessage: reqCtx.assistantMessage,
	}

	// Setup stop event handler
	h.setupStopEventHandler(eventBus, reqCtx.sessionID, reqCtx.session.TenantID, reqCtx.assistantMessage, cancel)

	// Watch for stop events independently of the client SSE connection so a
	// user-requested stop reliably cancels generation even when the client
	// has already disconnected (e.g. API-Key callers that close the stream
	// before POSTing /stop). The watcher self-terminates on a terminal stream
	// event, so its lifetime is decoupled from when the QA service call
	// returns (KnowledgeQA returns immediately while streaming continues in a
	// background goroutine). Use a connection-independent context derived from
	// baseCtx so it survives the client disconnect.
	h.startStopWatcher(logger.CloneContext(baseCtx), reqCtx.sessionID, reqCtx.assistantMessage.ID, eventBus)

	// Setup stream handler
	h.setupStreamHandler(asyncCtx, reqCtx.sessionID, reqCtx.assistantMessage.ID,
		reqCtx.requestID, reqCtx.session.TenantID, reqCtx.receivedAt, reqCtx.assistantMessage, eventBus)

	// Generate title if needed
	if generateTitle && reqCtx.session.Title == "" {
		logger.Infof(reqCtx.ctx, "Session has no title, starting async title generation, session ID: %s", reqCtx.sessionID)
		h.sessionService.GenerateTitleAsync(asyncCtx, reqCtx.session, reqCtx.query, reqCtx.summaryModelID, eventBus)
	}

	return streamCtx
}

// SearchKnowledge godoc
// @Summary      知识搜索
// @Description  在知识库中搜索（不使用LLM总结）
// @Tags         问答
// @Accept       json
// @Produce      json
// @Param        request  body      SearchKnowledgeRequest  true  "搜索请求"
// @Param        resource_urls  query     string  false  "文件引用形式，public 返回可加载直链"  Enums(handle, public)  default(handle)
// @Success      200      {object}  map[string]interface{}  "搜索结果"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-search [post]
func (h *Handler) SearchKnowledge(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	logger.Info(ctx, "Start processing knowledge search request")

	// Parse request body
	var request SearchKnowledgeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to parse request data", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Validate request parameters
	if request.Query == "" {
		logger.Error(ctx, "Query content is empty")
		c.Error(errors.NewBadRequestError("Query content cannot be empty"))
		return
	}

	// Resolve the storage-reference representation before retrieving, so a typo
	// or a rejected scope costs nothing.
	rewriter, err := h.resolveResourceRewriter(c)
	if err != nil {
		logger.Warnf(ctx, "Rejected resource URL mode: %v", err)
		_ = c.Error(err)
		return
	}

	// Merge single knowledge_base_id into knowledge_base_ids for backward compatibility
	knowledgeBaseIDs := request.KnowledgeBaseIDs
	if request.KnowledgeBaseID != "" {
		// Check if it's already in the list to avoid duplicates
		found := false
		for _, id := range knowledgeBaseIDs {
			if id == request.KnowledgeBaseID {
				found = true
				break
			}
		}
		if !found {
			knowledgeBaseIDs = append(knowledgeBaseIDs, request.KnowledgeBaseID)
		}
	}

	mentionScopes := tagScopesFromMentionedItems(request.MentionedItems)
	requestTagIDs := dedupRequestStrings(request.TagIDs)
	if err := validateUnscopedTagIDs(orphanTagIDsForScope(requestTagIDs, mentionScopes), secutils.SanitizeForLogArray(knowledgeBaseIDs)); err != nil {
		logger.Error(ctx, err.Error())
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	tagScopes := mergeTagScopesFromRequestIDs(mentionScopes, requestTagIDs, secutils.SanitizeForLogArray(knowledgeBaseIDs))

	if len(knowledgeBaseIDs) == 0 && len(request.KnowledgeIDs) == 0 && len(tagScopes) == 0 {
		logger.Error(ctx, "No knowledge base IDs, knowledge IDs, or tag scopes provided")
		c.Error(errors.NewBadRequestError("At least one knowledge_base_id, knowledge_base_ids, knowledge_ids, or scoped tag must be provided"))
		return
	}
	if err := types.AuthorizeTenantAPIKeyKnowledgeTargets(ctx, knowledgeBaseIDs, request.KnowledgeIDs); err != nil {
		c.Error(err)
		return
	}

	logger.Infof(
		ctx,
		"Knowledge search request, knowledge base IDs: %v, knowledge IDs: %v, tag scopes: %d, query: %s",
		secutils.SanitizeForLogArray(knowledgeBaseIDs),
		secutils.SanitizeForLogArray(request.KnowledgeIDs),
		len(tagScopes),
		secutils.SanitizeForLog(request.Query),
	)

	// Directly call knowledge retrieval service without LLM summarization
	searchResults, err := h.sessionService.SearchKnowledge(ctx, knowledgeBaseIDs, request.KnowledgeIDs, tagScopes, request.Query)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge search completed, found %d results", len(searchResults))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rewriter.CopyReferences(ctx, searchResults),
	})
}

// KnowledgeQA godoc
// @Summary      知识问答
// @Description  基于知识库的问答（使用LLM总结），支持SSE流式响应
// @Tags         问答
// @Accept       json
// @Produce      text/event-stream
// @Param        session_id  path      string                   true  "会话ID"
// @Param        request     body      CreateKnowledgeQARequest true  "问答请求"
// @Param        resource_urls  query     string  false  "文件引用形式，public 返回可加载直链"  Enums(handle, public)  default(handle)
// @Success      200         {object}  map[string]interface{}   "问答结果（SSE流）"
// @Failure      400         {object}  errors.AppError          "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-chat/{session_id} [post]
func (h *Handler) KnowledgeQA(c *gin.Context) {
	// Parse and validate request
	reqCtx, request, err := h.parseQARequest(c, "KnowledgeQA")
	if err != nil {
		c.Error(err)
		return
	}

	// Execute QA, generate title unless disabled
	h.executeQA(reqCtx, !request.DisableTitle)
}

// executeQA is the unified execution flow for knowledge-chat.
// It handles message creation, SSE setup, VLM analysis, service invocation, and error handling.
func (h *Handler) executeQA(reqCtx *qaRequestContext, generateTitle bool) {
	ctx := reqCtx.ctx
	sessionID := reqCtx.sessionID

	// Persist the input-bar state used for this request so reopening the
	// session can rehydrate model / KB / web-search selections. This is a pure
	// UI memo (no behavioural effect) and runs in a goroutine to avoid adding
	// a DB round-trip to TTFB. Use WithoutCancel so a fast client disconnect
	// doesn't drop the write.
	go h.persistLastRequestState(ctx, reqCtx)

	// Create user message. Include pre-uploaded document metadata so history
	// reload shows the attachments even though their content is selected later.
	userMessageAttachments := reqCtx.attachments
	if len(reqCtx.attachmentMetas) > 0 {
		userMessageAttachments = append(append(types.MessageAttachments{}, reqCtx.attachments...), reqCtx.attachmentMetas...)
	}
	userMsg, err := h.createUserMessage(ctx, sessionID, reqCtx.query, reqCtx.requestID, reqCtx.mentionedItems, convertImageAttachments(reqCtx.images), userMessageAttachments, reqCtx.channel, reqCtx.suggestionAttribution)
	if err != nil {
		reqCtx.c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	reqCtx.userMessageID = userMsg.ID
	reqCtx.userCreatedAt = userMsg.CreatedAt

	// Create assistant message
	assistantMessagePtr, err := h.createAssistantMessage(ctx, reqCtx.assistantMessage)
	if err != nil {
		reqCtx.c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	reqCtx.assistantMessage = assistantMessagePtr

	logger.Infof(ctx, "Using knowledge bases: %v", reqCtx.knowledgeBaseIDs)

	// Setup SSE stream
	streamCtx := h.setupSSEStream(reqCtx, generateTitle)

	// Register the completion handler on EventAgentFinalAnswer.
	var completionHandled bool

	// Persist the pipeline's retrieval/attachment stages so a reloaded
	// conversation redraws the timeline it showed while streaming, including
	// turns that searched and cited nothing.
	registerQuickAnswerTimelineRecorder(streamCtx.eventBus, streamCtx.assistantMessage)

	// Persist reasoning_content into agent_steps so historical reload can
	// reconstruct the thinking card. Accumulate on assistantMessage directly so
	// user-initiated stop also keeps whatever reasoning had streamed before the
	// cancel.
	streamCtx.eventBus.On(event.EventAgentThought, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentThoughtData)
		if !ok || data.Content == "" {
			return nil
		}
		appendQuickAnswerReasoning(streamCtx.assistantMessage, data.Content)
		return nil
	})

	streamCtx.eventBus.On(event.EventAgentFinalAnswer, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		streamCtx.assistantMessage.Content += data.Content
		if data.IsFallback {
			streamCtx.assistantMessage.IsFallback = true
		}
		if data.Done {
			if completionHandled {
				return nil
			}
			completionHandled = true

			logger.Infof(streamCtx.asyncCtx, "Knowledge QA service completed for session: %s", sessionID)
			updateCtx := context.WithValue(streamCtx.asyncCtx, types.TenantIDContextKey, reqCtx.session.TenantID)
			h.completeAssistantMessage(updateCtx, streamCtx.assistantMessage, reqCtx.query, reqCtx.userMessageID)
			streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
				Type:      event.EventAgentComplete,
				SessionID: sessionID,
				Data:      event.AgentCompleteData{FinalAnswer: streamCtx.assistantMessage.Content},
			})
		}
		return nil
	})

	// Execute QA asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 10240)
				runtime.Stack(buf, true)
				logger.ErrorWithFields(streamCtx.asyncCtx,
					errors.NewInternalServerError(fmt.Sprintf("Knowledge QA service panicked: %v\n%s", r, string(buf))),
					map[string]interface{}{"session_id": sessionID})
			}
		}()

		// Resolve pre-uploaded attachments (may still be parsing): waits with a
		// timeline step so the send is not blocked, then injects content/images.
		h.resolveTemporaryAttachments(streamCtx, reqCtx)

		// Run VLM image analysis if applicable
		h.runVLMAnalysisIfNeeded(streamCtx, reqCtx)

		// Build QA request and invoke the service
		qaReq := reqCtx.buildQARequest()
		serviceErr := h.sessionService.KnowledgeQA(streamCtx.asyncCtx, qaReq, streamCtx.eventBus)

		if serviceErr != nil {
			// A user-requested stop cancels asyncCtx, which surfaces here as a
			// context cancellation. That is an expected outcome, not a failure:
			// the stop event already notifies the client, so don't emit a
			// spurious error event (which would otherwise show an error toast).
			if streamCtx.asyncCtx.Err() != nil {
				logger.Infof(streamCtx.asyncCtx, "QA cancelled by user stop for session: %s", sessionID)
			} else {
				logger.ErrorWithFields(streamCtx.asyncCtx, serviceErr, nil)
				streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
					Type:      event.EventError,
					SessionID: sessionID,
					Data: event.ErrorData{
						Error:     serviceErr.Error(),
						Stage:     "knowledge_qa_execution",
						SessionID: sessionID,
					},
				})
			}
		}
	}()

	// Handle SSE events (blocking)
	shouldWaitForTitle := generateTitle && reqCtx.session.Title == ""
	h.handleEventsForSSE(ctx, reqCtx.c, sessionID, reqCtx.assistantMessage.ID,
		reqCtx.requestID, streamCtx.eventBus, shouldWaitForTitle, reqCtx.resourceRewriter)
}

// runVLMAnalysisIfNeeded runs VLM image analysis within the async goroutine,
// emitting tool_call/tool_result events so the user can see progress.
// VLM only runs on the pure-chat path (no KB, no web search); RAG paths defer
// VLM to the pipeline rewrite step.
func (h *Handler) runVLMAnalysisIfNeeded(streamCtx *sseStreamContext, reqCtx *qaRequestContext) {
	if len(reqCtx.images) == 0 {
		return
	}
	hasRequestKBs := len(reqCtx.knowledgeBaseIDs) > 0 || len(reqCtx.knowledgeIDs) > 0
	if hasRequestKBs || reqCtx.webSearchEnabled {
		return // VLM will be handled by the pipeline rewrite step
	}

	// Resolve a VLM model: explicit request override first, then the first
	// available VLLM model in the workspace.
	vlmModelID := reqCtx.summaryModelID
	if vlmModelID == "" {
		vlmModelID = firstAvailableVLMModelID(streamCtx.asyncCtx, h.modelService)
	}
	if vlmModelID == "" {
		return
	}

	sessionID := reqCtx.sessionID

	// Emit VLM tool call/result events
	toolCallID := uuid.New().String()

	streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
		Type:      event.EventAgentToolCall,
		SessionID: sessionID,
		Data: event.AgentToolCallData{
			ToolCallID: toolCallID,
			ToolName:   "image_analysis",
			Iteration:  0,
		},
	})

	vlmStart := time.Now()
	h.analyzeImageAttachments(streamCtx.asyncCtx, reqCtx.images, vlmModelID, reqCtx.query)

	streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
		Type:      event.EventAgentToolResult,
		SessionID: sessionID,
		Data: event.AgentToolResultData{
			ToolCallID: toolCallID,
			ToolName:   "image_analysis",
			Output:     "已分析图片内容",
			Success:    true,
			Duration:   time.Since(vlmStart).Milliseconds(),
			Iteration:  0,
		},
	})
}

// firstAvailableVLMModelID returns the ID of the first active VLLM model in
// the workspace, or "" when none is configured.
func firstAvailableVLMModelID(ctx context.Context, modelService interfaces.ModelService) string {
	if modelService == nil {
		return ""
	}
	models, err := modelService.ListModels(ctx)
	if err != nil {
		return ""
	}
	for _, m := range models {
		if m != nil && m.Type == types.ModelTypeVLLM && m.Status == types.ModelStatusActive {
			return m.ID
		}
	}
	return ""
}

// defaultAttachmentParseWaitTimeout bounds how long a QA turn waits for
// still-parsing attachments before proceeding with only the finished ones.
// Large or scanned documents can exceed this; raise it via
// YUHENG_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC when needed.
const defaultAttachmentParseWaitTimeout = 60 * time.Second

// attachmentParseWaitTimeout returns the configured wait timeout, honoring the
// YUHENG_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC override (in seconds) and falling
// back to the default when unset or invalid.
func attachmentParseWaitTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("YUHENG_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return defaultAttachmentParseWaitTimeout
}

// resolveTemporaryAttachments selects prompt content for pre-uploaded documents
// after the SSE stream is live. When any attachment is still parsing it emits a
// "attachment_parsing" timeline step and waits (bounded); unfinished attachments
// are skipped rather than blocking or failing the whole turn.
func (h *Handler) resolveTemporaryAttachments(streamCtx *sseStreamContext, reqCtx *qaRequestContext) {
	if len(reqCtx.attachmentIDs) == 0 {
		return
	}
	ctx := streamCtx.asyncCtx
	sessionID := reqCtx.sessionID
	// Prefer the session's tenant over gin.Context: this runs in an async
	// goroutine after the HTTP handler may have returned.
	tenantID := reqCtx.session.TenantID

	start := time.Now()
	var toolCallID string
	if h.hasPendingAttachments(ctx, tenantID, sessionID, reqCtx.attachmentIDs) {
		toolCallID = uuid.New().String()
		streamCtx.eventBus.Emit(ctx, event.Event{
			Type:      event.EventAgentToolCall,
			SessionID: sessionID,
			Data: event.AgentToolCallData{
				ToolCallID: toolCallID,
				ToolName:   "attachment_parsing",
				Iteration:  0,
			},
		})
		h.waitForAttachments(ctx, tenantID, sessionID, reqCtx.attachmentIDs, attachmentParseWaitTimeout())
	}

	readyIDs, skipped := h.partitionReadyAttachments(ctx, tenantID, sessionID, reqCtx.attachmentIDs)

	var temporaryResult *types.TemporaryDocumentPromptResult
	var resolveErr error
	if len(readyIDs) > 0 {
		temporaryResult, resolveErr = h.temporaryDocuments.ResolveForPrompt(ctx, tenantID, sessionID, readyIDs, reqCtx.query)
	}

	if toolCallID != "" {
		output := fmt.Sprintf("已解析 %d 个附件", len(readyIDs))
		if skipped > 0 {
			output += fmt.Sprintf("，%d 个未完成已跳过", skipped)
		}
		success := resolveErr == nil
		if resolveErr != nil {
			output = fmt.Sprintf("附件解析失败: %v", resolveErr)
		}
		streamCtx.eventBus.Emit(ctx, event.Event{
			Type:      event.EventAgentToolResult,
			SessionID: sessionID,
			Data: event.AgentToolResultData{
				ToolCallID: toolCallID,
				ToolName:   "attachment_parsing",
				Output:     output,
				Success:    success,
				Duration:   time.Since(start).Milliseconds(),
				Iteration:  0,
				Data: map[string]interface{}{
					"display_type":  "attachment_parsing",
					"parsed_count":  len(readyIDs),
					"skipped_count": skipped,
				},
			},
		})
	}
	if resolveErr != nil || temporaryResult == nil {
		if resolveErr != nil {
			logger.Warnf(ctx, "temporary attachment resolution failed for session %s: %v", sessionID, resolveErr)
		}
		return
	}

	reqCtx.attachments = append(reqCtx.attachments, temporaryResult.Attachments...)
	// Persist the freshly selected content back onto the stored user message.
	// The message was created with metadata-only attachment entries (content is
	// selected here, after the SSE stream is live), so without this write a
	// later turn rebuilds history from the Attachments column and sees empty
	// attachments.
	h.persistResolvedAttachmentContent(ctx, reqCtx, temporaryResult.Attachments)
	for _, imageURL := range temporaryResult.ImageURLs {
		reqCtx.images = append(reqCtx.images, ImageAttachment{URL: imageURL})
	}
}

// persistResolvedAttachmentContent writes the parsed content of pre-uploaded
// attachments back onto the stored user message so multi-turn history can
// replay it. Entries are matched by their temporary-document ID; only those
// present on the message are enriched. Failures are logged but never bubble up
// — losing the write only degrades follow-up context, it must not fail the turn.
func (h *Handler) persistResolvedAttachmentContent(
	ctx context.Context, reqCtx *qaRequestContext, resolved types.MessageAttachments,
) {
	if reqCtx.userMessageID == "" || len(resolved) == 0 {
		return
	}
	// Detach from the request/stream lifetime and pin the session tenant so a
	// user-triggered stop (which cancels asyncCtx) does not drop the write.
	updateCtx := context.WithValue(
		context.WithoutCancel(ctx), types.TenantIDContextKey, reqCtx.session.TenantID,
	)
	msg, err := h.messageService.GetMessage(updateCtx, reqCtx.sessionID, reqCtx.userMessageID)
	if err != nil || msg == nil {
		logger.Warnf(updateCtx, "persist attachment content: load user message %s failed: %v",
			reqCtx.userMessageID, err)
		return
	}
	byID := make(map[string]types.MessageAttachment, len(resolved))
	for _, att := range resolved {
		if att.ID != "" {
			byID[att.ID] = att
		}
	}
	changed := false
	for i := range msg.Attachments {
		if msg.Attachments[i].ID == "" {
			continue
		}
		if enriched, ok := byID[msg.Attachments[i].ID]; ok {
			msg.Attachments[i] = enriched
			changed = true
		}
	}
	if !changed {
		return
	}
	if err := h.messageService.UpdateMessage(updateCtx, msg); err != nil {
		logger.Warnf(updateCtx, "persist attachment content: update user message %s failed: %v",
			reqCtx.userMessageID, err)
	}
}

// normalizeTemporaryAttachmentIDs rejects oversized ID lists before any DB
// lookup, then returns a deduplicated, order-preserving list of non-empty IDs.
func normalizeTemporaryAttachmentIDs(ids []string) ([]string, error) {
	if len(ids) > types.MaxTemporaryAttachmentsPerMessage {
		return nil, fmt.Errorf("a message can use at most %d attachments", types.MaxTemporaryAttachmentsPerMessage)
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// hasPendingAttachments reports whether any of the given documents is still
// uploading or being parsed.
func (h *Handler) hasPendingAttachments(ctx context.Context, tenantID uint64, sessionID string, ids []string) bool {
	for _, id := range ids {
		doc, err := h.temporaryDocuments.Get(ctx, tenantID, sessionID, id)
		if err != nil || doc == nil {
			continue
		}
		if doc.Status == types.TemporaryDocumentStatusUploaded ||
			doc.Status == types.TemporaryDocumentStatusProcessing {
			return true
		}
	}
	return false
}

// waitForAttachments polls until no attachment is pending or the timeout / ctx
// cancellation fires.
func (h *Handler) waitForAttachments(ctx context.Context, tenantID uint64, sessionID string, ids []string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !h.hasPendingAttachments(ctx, tenantID, sessionID, ids) || time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// partitionReadyAttachments splits the ids into ready ones and a count of those
// skipped (missing, failed, or still parsing after the wait).
func (h *Handler) partitionReadyAttachments(ctx context.Context, tenantID uint64, sessionID string, ids []string) (ready []string, skipped int) {
	for _, id := range ids {
		doc, err := h.temporaryDocuments.Get(ctx, tenantID, sessionID, id)
		if err != nil || doc == nil || doc.Status != types.TemporaryDocumentStatusReady {
			skipped++
			continue
		}
		ready = append(ready, id)
	}
	return ready, skipped
}

// persistLastRequestState records the input-bar state the user just sent so
// that reopening this session restores model/KB/web-search picks. Pure UI
// memo — failures are logged but never bubble up; the caller runs this in a
// goroutine and is safe to discard the returned context.
func (h *Handler) persistLastRequestState(parentCtx context.Context, reqCtx *qaRequestContext) {
	// Detach from the HTTP request lifetime: this write must survive both
	// SSE disconnects and the parent gin context being released after the
	// handler returns.
	ctx := logger.CloneContext(context.WithoutCancel(parentCtx))

	state := &types.SessionLastRequestState{
		ModelID:          reqCtx.summaryModelID,
		KnowledgeBaseIDs: reqCtx.knowledgeBaseIDs,
		KnowledgeIDs:     reqCtx.knowledgeIDs,
		TagIDs:           reqCtx.tagIDs,
		MentionedItems:   reqCtx.mentionedItems,
		WebSearchEnabled: reqCtx.webSearchEnabled,
	}

	if err := h.sessionService.UpdateSessionLastRequestState(ctx, reqCtx.sessionID, state); err != nil {
		logger.Warnf(ctx, "persist last_request_state failed for session %s: %v", reqCtx.sessionID, err)
	}
}

// appendQuickAnswerReasoning accumulates streamed reasoning_content from
// KnowledgeQA (fast answer) into a single AgentStep for history replay.
func appendQuickAnswerReasoning(msg *types.Message, content string) {
	if content == "" {
		return
	}
	ensureQuickAnswerStep(msg).ReasoningContent += content
}

// completeAssistantMessage marks an assistant message as complete, updates it,
// and asynchronously indexes the Q&A pair into the chat history knowledge base.
func (h *Handler) completeAssistantMessage(
	ctx context.Context, assistantMessage *types.Message, userQuery, userMessageID string,
) {
	assistantMessage.UpdatedAt = time.Now()
	assistantMessage.IsCompleted = true
	_ = h.messageService.UpdateMessage(ctx, assistantMessage)

	// Asynchronously index the Q&A pair into the chat history knowledge base for vector search.
	// Use WithoutCancel so the goroutine survives after the HTTP request context is done.
	bgCtx := context.WithoutCancel(ctx)
	go h.messageService.IndexMessageToKB(bgCtx, userQuery, assistantMessage.Content, assistantMessage.ID, assistantMessage.SessionID)
	if userQuery != "" && h.suggestionService != nil {
		go func() {
			if _, err := h.suggestionService.EnsureFollowUps(
				bgCtx, assistantMessage.SessionID, assistantMessage.ID, false,
			); err != nil {
				logger.Warnf(bgCtx, "follow-up suggestion generation failed for message %s: %v", assistantMessage.ID, err)
			}
		}()
	}
}
