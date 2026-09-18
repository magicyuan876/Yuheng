package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/event"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// StreamHandler bridges the per-request EventBus to the StreamManager for one
// knowledge-chat turn. It subscribes to the chat SSE protocol events
// (thought / tool_call / tool_result / references / final_answer / complete)
// and appends them to the StreamManager, from which both the live SSE
// response and the continue-stream replay read.
type StreamHandler struct {
	ctx                context.Context
	sessionID          string
	assistantMessageID string
	requestID          string
	receivedAt         time.Time // Handler entry timestamp, used for TTFB logging
	ttfbLogged         bool      // Guards one-shot TTFB log on first answer chunk
	assistantMessage   *types.Message
	streamManager      interfaces.StreamManager

	eventBus *event.EventBus

	// State tracking
	knowledgeRefs   []*types.SearchResult
	finalAnswer     string
	answerSegments  []*answerSegment     // Per-answer-event-ID accumulation, so superseded preambles can be dropped
	eventStartTimes map[string]time.Time // Track start time for duration calculation
	mu              sync.Mutex
}

// answerSegment accumulates the streamed content of a single final-answer event
// ID. A non-terminal round may stream a preamble ("let me search…") under its
// own answer ID and then be marked superseded once the round turns out to call
// tools; tracking segments separately lets us exclude that preamble from the
// persisted assistant message instead of leaking it into the final answer.
type answerSegment struct {
	id         string
	content    string
	superseded bool
}

// findAnswerSegment returns the segment for an answer event ID, or nil.
// Callers must hold h.mu.
func (h *StreamHandler) findAnswerSegment(id string) *answerSegment {
	for _, seg := range h.answerSegments {
		if seg.id == id {
			return seg
		}
	}
	return nil
}

// composeFinalAnswer rebuilds the persisted answer from all non-superseded
// segments in arrival order. Callers must hold h.mu.
func (h *StreamHandler) composeFinalAnswer() string {
	var b strings.Builder
	for _, seg := range h.answerSegments {
		if !seg.superseded {
			b.WriteString(seg.content)
		}
	}
	return b.String()
}

// NewStreamHandler creates a new handler for chat SSE streaming.
func NewStreamHandler(
	ctx context.Context,
	sessionID, assistantMessageID, requestID string,
	receivedAt time.Time,
	assistantMessage *types.Message,
	streamManager interfaces.StreamManager,
	eventBus *event.EventBus,
) *StreamHandler {
	return &StreamHandler{
		ctx:                ctx,
		sessionID:          sessionID,
		assistantMessageID: assistantMessageID,
		requestID:          requestID,
		receivedAt:         receivedAt,
		assistantMessage:   assistantMessage,
		streamManager:      streamManager,
		eventBus:           eventBus,
		knowledgeRefs:      make([]*types.SearchResult, 0),
		eventStartTimes:    make(map[string]time.Time),
	}
}

// Subscribe subscribes to all chat streaming events on the dedicated EventBus.
// No SessionID filtering needed since we have a dedicated EventBus per request.
func (h *StreamHandler) Subscribe() {
	h.eventBus.On(event.EventAgentThought, h.handleThought)
	h.eventBus.On(event.EventAgentToolCall, h.handleToolCall)
	h.eventBus.On(event.EventAgentToolResult, h.handleToolResult)
	h.eventBus.On(event.EventAgentReferences, h.handleReferences)
	h.eventBus.On(event.EventAgentFinalAnswer, h.handleFinalAnswer)
	h.eventBus.On(event.EventError, h.handleError)
	h.eventBus.On(event.EventSessionTitle, h.handleSessionTitle)
	h.eventBus.On(event.EventAgentComplete, h.handleComplete)
}

// handleThought handles thought (reasoning_content) events
func (h *StreamHandler) handleThought(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentThoughtData)
	if !ok {
		return nil
	}

	h.mu.Lock()

	// Track start time on first chunk
	if _, exists := h.eventStartTimes[evt.ID]; !exists {
		h.eventStartTimes[evt.ID] = time.Now()
	}

	// Calculate duration if done
	var metadata map[string]interface{}
	if data.Done {
		startTime := h.eventStartTimes[evt.ID]
		duration := time.Since(startTime)
		metadata = map[string]interface{}{
			"event_id":     evt.ID,
			"duration_ms":  duration.Milliseconds(),
			"completed_at": time.Now().Unix(),
		}
		delete(h.eventStartTimes, evt.ID)
	} else {
		metadata = map[string]interface{}{
			"event_id": evt.ID,
		}
	}

	h.mu.Unlock()

	// Append this chunk to stream (no accumulation - frontend will accumulate)
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeThinking,
		Content:   data.Content, // Just this chunk
		Done:      data.Done,
		Timestamp: time.Now(),
		Data:      metadata,
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append thought event to stream failed", "error", err)
	}

	return nil
}

// handleToolCall handles tool call events
func (h *StreamHandler) handleToolCall(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentToolCallData)
	if !ok {
		return nil
	}

	h.mu.Lock()
	// Track start time for this tool call (use tool_call_id as key)
	h.eventStartTimes[data.ToolCallID] = time.Now()
	// Any answer text streamed before this tool call was a non-terminal round's
	// preamble, not the final answer. Drop those segments from the persisted
	// answer so the preamble never leaks into Message.Content.
	supersededAny := false
	for _, seg := range h.answerSegments {
		if !seg.superseded && seg.content != "" {
			seg.superseded = true
			supersededAny = true
		}
	}
	if supersededAny {
		h.finalAnswer = h.composeFinalAnswer()
	}
	h.mu.Unlock()

	metadata := map[string]interface{}{
		"tool_name":    data.ToolName,
		"arguments":    data.Arguments,
		"tool_call_id": data.ToolCallID,
	}

	// Append event to stream
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeToolCall,
		Content:   fmt.Sprintf("Calling tool: %s", data.ToolName),
		Done:      false,
		Timestamp: time.Now(),
		Data:      metadata,
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append tool call event to stream failed", "error", err)
	}

	return nil
}

// handleToolResult handles tool result events
func (h *StreamHandler) handleToolResult(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentToolResultData)
	if !ok {
		return nil
	}

	h.mu.Lock()
	// Calculate duration from start time if available, otherwise use provided duration
	var durationMs int64
	if startTime, exists := h.eventStartTimes[data.ToolCallID]; exists {
		durationMs = time.Since(startTime).Milliseconds()
		delete(h.eventStartTimes, data.ToolCallID)
	} else if data.Duration > 0 {
		// Fallback to provided duration if start time not tracked
		durationMs = data.Duration
	}
	h.mu.Unlock()

	// Send SSE response (both success and failure)
	responseType := types.ResponseTypeToolResult
	content := StreamContentForToolResult(data.ToolName, data.Success, data.Error, data.Data)
	if !data.Success {
		responseType = types.ResponseTypeError
		if content == "" && data.Error != "" {
			content = data.Error
		}
	}

	// Build metadata including tool result data for rich frontend rendering
	metadata := map[string]interface{}{
		"tool_name":    data.ToolName,
		"success":      data.Success,
		"error":        data.Error,
		"duration_ms":  durationMs,
		"tool_call_id": data.ToolCallID,
	}

	clientData := SanitizeToolResultForClient(data.ToolName, &types.ToolResult{
		Success: data.Success,
		Output:  data.Output,
		Error:   data.Error,
		Data:    data.Data,
	})
	for k, v := range clientData {
		metadata[k] = v
	}

	// Append event to stream
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      responseType,
		Content:   content,
		Done:      false,
		Timestamp: time.Now(),
		Data:      metadata,
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append tool result event to stream failed", "error", err)
	}

	return nil
}

// handleReferences handles knowledge references events
func (h *StreamHandler) handleReferences(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentReferencesData)
	if !ok {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Extract knowledge references
	// Try to cast directly to []*types.SearchResult first
	if searchResults, ok := data.References.([]*types.SearchResult); ok {
		h.knowledgeRefs = append(h.knowledgeRefs, searchResults...)
	} else if refs, ok := data.References.([]interface{}); ok {
		// Fallback: convert from []interface{}
		for _, ref := range refs {
			if sr, ok := ref.(*types.SearchResult); ok {
				h.knowledgeRefs = append(h.knowledgeRefs, sr)
			} else if refMap, ok := ref.(map[string]interface{}); ok {
				// Parse from map if needed
				h.knowledgeRefs = append(h.knowledgeRefs, searchResultFromMap(refMap))
			}
		}
	}

	// Update assistant message references
	h.assistantMessage.KnowledgeReferences = h.knowledgeRefs

	// Append references event to stream
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeReferences,
		Content:   "",
		Done:      false,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"references": types.References(h.knowledgeRefs),
		},
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append references event to stream failed", "error", err)
	}

	return nil
}

// handleFinalAnswer handles final answer events
func (h *StreamHandler) handleFinalAnswer(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentFinalAnswerData)
	if !ok {
		return nil
	}

	h.mu.Lock()

	// Track start time on first chunk
	if _, exists := h.eventStartTimes[evt.ID]; !exists {
		h.eventStartTimes[evt.ID] = time.Now()
	}

	// Emit a one-shot TTFB log the first time *any* answer chunk reaches
	// the stream handler. This lets us compare the backend's "request in →
	// first token out" timing against the frontend-observed TTFB and pin
	// down where latency lives (network vs server vs LLM).
	if !h.ttfbLogged && !h.receivedAt.IsZero() {
		h.ttfbLogged = true
		ttfb := time.Since(h.receivedAt)
		logger.GetLogger(h.ctx).Infof("TTFB:first_answer_chunk request_id=%s, session_id=%s, ttfb_ms=%d",
			h.requestID, h.sessionID, ttfb.Milliseconds())
	}

	// Accumulate final answer locally for assistant message (database). Track
	// per event ID so a later supersede can subtract this segment's content.
	if data.Content != "" {
		seg := h.findAnswerSegment(evt.ID)
		if seg == nil {
			seg = &answerSegment{id: evt.ID}
			h.answerSegments = append(h.answerSegments, seg)
		}
		seg.content += data.Content
		h.finalAnswer = h.composeFinalAnswer()
	}
	if data.IsFallback {
		h.assistantMessage.IsFallback = true
	}

	// Calculate duration if done
	var metadata map[string]interface{}
	if data.Done {
		startTime := h.eventStartTimes[evt.ID]
		duration := time.Since(startTime)
		metadata = map[string]interface{}{
			"event_id":     evt.ID,
			"duration_ms":  duration.Milliseconds(),
			"completed_at": time.Now().Unix(),
		}
		delete(h.eventStartTimes, evt.ID)
	} else {
		metadata = map[string]interface{}{
			"event_id": evt.ID,
		}
	}
	if data.IsFallback {
		metadata["is_fallback"] = true
	}
	h.mu.Unlock()

	// Append this chunk to stream (frontend will accumulate by event ID)
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeAnswer,
		Content:   data.Content, // Just this chunk
		Done:      data.Done,
		Timestamp: time.Now(),
		Data:      metadata,
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append answer event to stream failed", "error", err)
	}

	return nil
}

// handleError handles error events
func (h *StreamHandler) handleError(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.ErrorData)
	if !ok {
		return nil
	}

	// Build error metadata
	metadata := map[string]interface{}{
		"stage": data.Stage,
		"error": data.Error,
	}

	// Append error event to stream
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeError,
		Content:   data.Error,
		Done:      true,
		Timestamp: time.Now(),
		Data:      metadata,
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append error event to stream failed", "error", err)
	}

	return nil
}

// handleSessionTitle handles session title update events
func (h *StreamHandler) handleSessionTitle(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.SessionTitleData)
	if !ok {
		return nil
	}

	// Use background context for title event since it may arrive after stream completion
	bgCtx := context.Background()

	// Append title event to stream
	if err := h.streamManager.AppendEvent(bgCtx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeSessionTitle,
		Content:   data.Title,
		Done:      true,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"session_id": data.SessionID,
			"title":      data.Title,
		},
	}); err != nil {
		logger.GetLogger(h.ctx).Warn("Append session title event to stream failed (stream may have ended)", "error", err)
	}

	return nil
}

// handleComplete handles chat complete events
func (h *StreamHandler) handleComplete(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentCompleteData)
	if !ok {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update assistant message with final data
	if data.MessageID == h.assistantMessageID {
		h.assistantMessage.IsCompleted = true

		// Update knowledge references if provided
		if len(data.KnowledgeRefs) > 0 {
			knowledgeRefs := make([]*types.SearchResult, 0, len(data.KnowledgeRefs))
			for _, ref := range data.KnowledgeRefs {
				if sr, ok := ref.(*types.SearchResult); ok {
					knowledgeRefs = append(knowledgeRefs, sr)
				}
			}
			h.assistantMessage.KnowledgeReferences = knowledgeRefs
		}

		h.assistantMessage.Content += data.FinalAnswer

		// Persist the turn's aggregated LLM usage with the message so history
		// reads still carry it after the live stream is gone.
		if usage, ok := data.Usage.(*types.TokenUsage); ok && usage != nil {
			h.assistantMessage.Usage = usage
		}
	}

	// Fallback: if no answer events were streamed but we have a final answer,
	// emit it as answer events so the frontend can render it properly.
	// This guards against edge cases where the LLM stops without calling final_answer.
	if h.finalAnswer == "" && data.FinalAnswer != "" {
		logger.GetLogger(h.ctx).Warnf(
			"No answer events were streamed, emitting fallback answer (len=%d). "+
				"This typically happens when the model stopped naturally and content was sent as thought events. "+
				"total_duration_ms=%d",
			len(data.FinalAnswer), data.TotalDurationMs,
		)
		fallbackID := fmt.Sprintf("answer-fallback-%d", time.Now().UnixMilli())
		if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
			ID:        fallbackID,
			Type:      types.ResponseTypeAnswer,
			Content:   data.FinalAnswer,
			Done:      false,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"event_id":    fallbackID,
				"is_fallback": true,
			},
		}); err != nil {
			logger.GetLogger(h.ctx).Errorf("Append fallback answer event failed: %v", err)
		}
		if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
			ID:        fallbackID,
			Type:      types.ResponseTypeAnswer,
			Content:   "",
			Done:      true,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"event_id":    fallbackID,
				"is_fallback": true,
			},
		}); err != nil {
			logger.GetLogger(h.ctx).Errorf("Append fallback answer done event failed: %v", err)
		}
	}

	// Send completion event to stream manager so SSE can detect completion
	completeData := map[string]interface{}{
		"total_steps":       data.TotalSteps,
		"total_duration_ms": data.TotalDurationMs,
	}
	// Carry the turn's aggregated LLM usage both inside data (map consumers)
	// and on the typed event field, which buildStreamResponse promotes to the
	// response's top-level usage.
	turnUsage, _ := data.Usage.(*types.TokenUsage)
	if turnUsage != nil {
		completeData["usage"] = turnUsage
	}
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeComplete,
		Content:   "",
		Done:      true,
		Timestamp: time.Now(),
		Data:      completeData,
		Usage:     turnUsage,
	}); err != nil {
		logger.GetLogger(h.ctx).Errorf("Append complete event to stream failed: %v", err)
	}

	return nil
}
