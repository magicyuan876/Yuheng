import { markRaw, nextTick, type Ref } from "vue";
import { useI18n } from "vue-i18n";
import { ensureRagPipelineHistoryStream } from "@/utils/rag-pipeline-history";
import { applyMessageCreatedAt, bindServerTurnTimestamps, ensureMessageCreatedAt } from "@/utils/messageTimestamp";

export type ChatMessage = Record<string, unknown>;

export interface UseChatStreamHandlerOptions {
  messagesList: ChatMessage[];
  loading: Ref<boolean>;
  isReplying: Ref<boolean>;
  currentAssistantMessageId: Ref<string>;
  fullContent: Ref<string>;
  scrollToBottom: (force?: boolean) => void;
  onReplyComplete?: (content: string) => void;
  onTurnComplete?: (message: ChatMessage) => void;
  onError?: (message: string) => void;
  /** Main chat: keep the last incomplete message reactive for continue-stream. */
  preserveIncompleteStreamReactive?: boolean;
  isFirstEnter?: Ref<boolean>;
  scrollContainer?: Ref<HTMLElement | null>;
  onAfterMsgList?: () => void | Promise<void>;
  /** knowledge-chat 的 agent_query 关联帧：绑定 request/assistant message id。 */
  onQueryEvent?: (data: ChatMessage, existingMessage: ChatMessage | undefined, created: boolean) => void;
  onMessageCreated?: (message: ChatMessage) => void;
  onMessageUpdated?: (message: ChatMessage, payload?: ChatMessage) => void;
  onAnswerDone?: (message: ChatMessage) => void;
  onChunkBound?: (message: ChatMessage, created: boolean) => void;
  debug?: boolean;
}

export function useChatStreamHandler(options: UseChatStreamHandlerOptions) {
  const { t } = useI18n();
  const {
    messagesList,
    loading,
    isReplying,
    currentAssistantMessageId,
    fullContent,
    scrollToBottom,
    onReplyComplete,
    onTurnComplete,
    onError,
    preserveIncompleteStreamReactive = false,
    isFirstEnter,
    scrollContainer,
    onAfterMsgList,
    onQueryEvent,
    onMessageCreated,
    onMessageUpdated,
    onAnswerDone,
    onChunkBound,
    debug = false,
  } = options;

  const emitMessageCreated = (message: ChatMessage) => {
    ensureMessageCreatedAt(message);
    onMessageCreated?.(message);
  };

  const emitMessageUpdated = (message: ChatMessage, payload?: ChatMessage) => {
    if (payload) applyMessageCreatedAt(message, payload.created_at);
    onMessageUpdated?.(message, payload);
  };

  const log = (...args: unknown[]) => {
    if (debug) console.log(...args);
  };

  const findLastMessage = (predicate: (item: ChatMessage) => boolean) => {
    for (let i = messagesList.length - 1; i >= 0; i--) {
      const item = messagesList[i];
      if (predicate(item)) return item;
    }
    return undefined;
  };

  /** Incomplete assistant row for the current turn (must be the list tail). */
  const getTrailingIncompleteAssistant = () => {
    const last = messagesList[messagesList.length - 1];
    if (last?.role === "assistant" && !last.is_completed) return last;
    return undefined;
  };

  const markAssistantStopped = (message: ChatMessage) => {
    if (!message || message.is_completed) return;
    message.is_completed = true;
  };

  /** Finalize any in-flight assistant rows before a new user query is sent. */
  const prepareForNewOutgoingMessage = () => {
    for (const msg of messagesList) {
      if (msg.role === "assistant" && !msg.is_completed) {
        markAssistantStopped(msg);
      }
    }
    fullContent.value = "";
    currentAssistantMessageId.value = "";
  };

  /** Mark the assistant row being stopped without clearing its id (stop API still needs it). */
  const markInFlightAssistantStopped = (messageId?: string) => {
    let target: ChatMessage | undefined;
    if (messageId) {
      target = messagesList.find((m) => m.id === messageId || m.request_id === messageId);
    }
    if (!target) target = getTrailingIncompleteAssistant();
    if (target) markAssistantStopped(target);
    fullContent.value = "";
  };

  const extractKnowledgeReferences = (data: ChatMessage) => {
    const dataPayload = data.data as ChatMessage | undefined;
    const refs = data.knowledge_references || dataPayload?.references || dataPayload?.knowledge_references || [];
    return Array.isArray(refs) ? refs : [];
  };

  /** Match the in-flight assistant row by request id or assistant message id. */
  const resolveActiveAssistantMessage = (data: ChatMessage) => {
    const dataId = data.id as string | undefined;
    const assistantId =
      (data.assistant_message_id as string | undefined) || currentAssistantMessageId.value || undefined;

    const matched = findLastMessage((item) => {
      if (item.role !== "assistant") return false;
      if (dataId && (item.request_id === dataId || item.id === dataId)) return true;
      if (assistantId && (item.id === assistantId || item.request_id === assistantId)) return true;
      return false;
    });
    if (matched) return matched;

    return getTrailingIncompleteAssistant();
  };

  const applyKnowledgeReferences = (data: ChatMessage) => {
    const refs = extractKnowledgeReferences(data);
    if (!refs.length) return undefined;

    let message = resolveActiveAssistantMessage(data);
    const created = !message;
    if (!message) {
      const rowId = (data.id as string | undefined) || currentAssistantMessageId.value;
      message = {
        id: rowId,
        request_id: rowId,
        role: "assistant",
        content: "",
        showThink: false,
        thinkContent: "",
        thinking: false,
        is_completed: false,
        isRagMode: true,
        knowledge_references: [],
      };
      ensureStreamMessageShell(message, data.id as string | undefined);
      messagesList.push(message);
      emitMessageCreated(message);
      loading.value = false;
    } else {
      ensureStreamMessageShell(message, data.id as string | undefined);
    }

    message.knowledge_references = refs.slice();
    if (created) onChunkBound?.(message, true);
    emitMessageUpdated(message, data);
    log("[References] Saved to message, count:", refs.length);
    return message;
  };

  const ensureStreamMessageShell = (message: ChatMessage, requestId?: string) => {
    message.isRagMode = true;
    if (!message.agentEventStream) message.agentEventStream = [];
    if (!message._eventMap) message._eventMap = new Map();
    if (requestId) {
      if (!message.id) message.id = requestId;
      if (!message.request_id) message.request_id = requestId;
    }
  };

  const shouldShowGlobalTypingIndicator = (messages: ChatMessage[], isLoading: boolean, isRecovering = false) => {
    if (!isLoading && !isRecovering) return false;
    return true;
  };

  /** History reload: restore RAG flags lost after history reload. */
  const restoreRagFlags = (item: ChatMessage) => {
    if (item.role !== "assistant") return;
    item.isRagMode = true;
    ensureRagPipelineHistoryStream(item as Parameters<typeof ensureRagPipelineHistoryStream>[0]);
    if (item.agentEventStream) {
      item.agentEventStream = markRaw(item.agentEventStream as object);
    }
  };

  const recomposeAgentAnswer = (message: ChatMessage) => {
    const stream = message.agentEventStream as
      | Array<{
          type?: string;
          superseded?: boolean;
          content?: string;
        }>
      | undefined;
    if (!stream) return "";
    let out = "";
    for (const e of stream) {
      if (e.type === "answer" && !e.superseded && e.content) {
        out += e.content;
      }
    }
    return out;
  };

  // History replay: rebuild the RAG timeline (thinking card + retrieval /
  // attachment steps) from the persisted agent_steps JSON. knowledge-chat
  // records reasoning plus its pipeline tool calls (query_understand /
  // knowledge_search / attachment_parsing / image_analysis) there so a
  // reloaded conversation redraws what streamed live.
  const reconstructEventStreamFromSteps = (
    agentSteps: unknown[],
    messageContent: string,
    isCompleted = false,
    isFallback = false,
    agentDurationMs = 0,
  ) => {
    const events: ChatMessage[] = [];

    if (agentSteps && Array.isArray(agentSteps) && agentSteps.length > 0) {
      agentSteps.forEach((rawStep) => {
        const step = rawStep as ChatMessage;
        const stepTimestamp = step.timestamp ? new Date(String(step.timestamp)).getTime() : 0;
        const toolCalls = step.tool_calls;
        const hasToolCalls = toolCalls && Array.isArray(toolCalls) && toolCalls.length > 0;

        const reasoningText =
          step.reasoning_content && String(step.reasoning_content).trim() ? String(step.reasoning_content) : "";
        if (reasoningText) {
          events.push({
            type: "thinking",
            event_id: `step-${step.iteration}-thought`,
            content: reasoningText,
            done: true,
            thinking: false,
            timestamp: stepTimestamp || undefined,
            duration_ms: step.duration || undefined,
          });
        }
        const preambleText = step.thought && String(step.thought).trim() ? String(step.thought) : "";
        if (preambleText && hasToolCalls) {
          events.push({
            type: "answer",
            event_id: `step-${step.iteration}-preamble`,
            content: preambleText,
            done: true,
            superseded: true,
            timestamp: stepTimestamp || undefined,
          });
        }

        if (toolCalls && Array.isArray(toolCalls)) {
          toolCalls.forEach((toolCall: ChatMessage) => {
            if (toolCall.name === "final_answer") return;
            const result = toolCall.result as ChatMessage | undefined;
            const resultData = result?.data as ChatMessage | undefined;
            events.push({
              type: "tool_call",
              tool_call_id: toolCall.id,
              tool_name: toolCall.name,
              arguments: toolCall.args,
              pending: false,
              success: result?.success !== false,
              output: result?.output || "",
              error: result?.error || undefined,
              timestamp: stepTimestamp || undefined,
              duration: toolCall.duration,
              duration_ms: toolCall.duration,
              display_type: resultData?.display_type,
              tool_data: result?.data,
            });
          });
        }
      });
    }

    if (agentDurationMs > 0) {
      events.push({
        type: "agent_complete",
        total_duration_ms: agentDurationMs,
      });
    }

    if (messageContent && messageContent.trim()) {
      const answerEvent: ChatMessage = {
        type: "answer",
        content: messageContent,
        done: true,
      };
      if (isFallback) answerEvent.is_fallback = true;
      events.push(answerEvent);
    } else if (isCompleted) {
      events.push({
        type: "stop",
        timestamp: Date.now(),
        reason: "user_requested",
      });
    }

    return events;
  };

  const handleMsgList = async (data: ChatMessage[], isScrollType = false, newScrollHeight?: number) => {
    const chatlist = [...data];
    const existingIds = new Set(messagesList.map((m) => m.id).filter(Boolean));
    const processed: ChatMessage[] = [];

    for (const raw of chatlist) {
      const item = preserveIncompleteStreamReactive ? raw : { ...raw };
      if (item.id && existingIds.has(item.id)) continue;
      if (item.id) existingIds.add(item.id);

      const willContinueStream = preserveIncompleteStreamReactive && !item.is_completed;
      if (willContinueStream) {
        item.agentEventStream = item.agentEventStream || [];
        item._eventMap = new Map();
      } else {
        item.agent_steps = item.agent_steps ? markRaw(item.agent_steps as object) : item.agent_steps;
        item.agentEventStream = markRaw((item.agentEventStream as unknown[]) || []);
        item._eventMap = markRaw(new Map());
      }

      if (item.agent_steps && Array.isArray(item.agent_steps) && item.agent_steps.length > 0) {
        item.agentEventStream = markRaw(
          reconstructEventStreamFromSteps(
            item.agent_steps as unknown[],
            String(item.content || ""),
            Boolean(item.is_completed),
            Boolean(item.is_fallback),
            Number(item.agent_duration_ms) || 0,
          ),
        );
      }

      restoreRagFlags(item);

      if (item.content) {
        const content = String(item.content);
        const thinkCloseTag = "</think>";
        if (!content.includes("<think>") && !content.includes(thinkCloseTag)) {
          item.thinkContent = "";
          item.showThink = false;
          item.thinking = false;
        } else if (content.includes(thinkCloseTag)) {
          item.showThink = true;
          item.thinking = false;
          const index = content.trim().lastIndexOf(thinkCloseTag);
          item.thinkContent = content.trim().substring(0, index).replace("<think>", "").trim();
          item.content = content.trim().substring(index + thinkCloseTag.length);
        } else if (content.includes("<think>")) {
          item.showThink = true;
          item.thinking = true;
          item.thinkContent = content.replace("<think>", "").trim();
          item.content = "";
        }
      }

      processed.push(item);
    }

    if (processed.length > 0) {
      if (isScrollType) {
        for (let i = processed.length - 1; i >= 0; i--) {
          messagesList.unshift(processed[i]);
        }
      } else {
        messagesList.push(...processed);
      }
    }

    if (isFirstEnter?.value) {
      scrollToBottom(true);
    } else if (isScrollType && scrollContainer?.value && typeof newScrollHeight === "number") {
      nextTick(() => {
        if (!scrollContainer.value) return;
        const { scrollHeight } = scrollContainer.value;
        scrollContainer.value.scrollTop = scrollHeight - newScrollHeight;
      });
    }

    if (onAfterMsgList) {
      await onAfterMsgList();
    }
  };

  const updateAssistantSession = (payload: ChatMessage) => {
    const message = findLastMessage((item) => {
      if (item.request_id === payload.id) return true;
      return item.id === payload.id;
    });
    if (message) {
      if (payload.id && !message.request_id) message.request_id = payload.id;
      message.content = payload.content;
      message.thinking = payload.thinking;
      message.thinkContent = payload.thinkContent;
      message.showThink = payload.showThink;
      if (!message.knowledge_references) {
        message.knowledge_references = payload.knowledge_references;
      }
      if (payload.is_fallback) message.is_fallback = true;
      if (payload.is_completed) message.is_completed = true;
      emitMessageUpdated(message, payload);
    } else {
      const entry = { ...payload };
      if (entry.id && !entry.request_id) entry.request_id = entry.id;
      messagesList.push(entry);
      emitMessageCreated(entry);
      emitMessageUpdated(entry, payload);
    }
    scrollToBottom();
  };

  const reportError = (errorMsg: string) => {
    if (onError) {
      onError(errorMsg);
    }
  };

  // knowledge-chat 事件帧处理：thinking / answer / complete / stop / error。
  // answer 事件按 event_id 累积进 agentEventStream，再重组为 message.content
  // 供 markdown 渲染；thinking 事件同样累积，供 RAG 时间线卡片展示。
  const handleStreamChunk = (data: ChatMessage) => {
    const dataId = data.id as string | undefined;
    let message = findLastMessage((item) => item.request_id === dataId || item.id === dataId);
    let created = false;

    if (!message) {
      const newMsg: ChatMessage = {
        id: dataId,
        request_id: dataId,
        role: "assistant",
        content: "",
        isRagMode: true,
        agentEventStream: [],
        _eventMap: new Map(),
        knowledge_references: [],
      };
      messagesList.push(newMsg);
      emitMessageCreated(newMsg);
      loading.value = false;
      scrollToBottom(true);
      message = newMsg;
      created = true;
    } else {
      onChunkBound?.(message, false);
    }

    if (created) {
      onChunkBound?.(message, true);
    }

    ensureStreamMessageShell(message, dataId);
    applyMessageCreatedAt(message, data.created_at);

    if (
      loading.value &&
      (data.response_type === "thinking" || data.response_type === "answer" || data.response_type === "error")
    ) {
      log("[Stream Chunk] Closing loading for continued stream");
      loading.value = false;
    }

    const responseType = data.response_type as string;
    const dataPayload = data.data as ChatMessage | undefined;

    switch (responseType) {
      case "thinking": {
        const eventId = dataPayload?.event_id as string | undefined;
        log("[Thinking Event]", {
          event_id: eventId,
          done: data.done,
          content_length: (data.content as string | undefined)?.length || 0,
        });
        if (!message.agentEventStream) message.agentEventStream = [];
        if (!message._eventMap) message._eventMap = new Map();
        const eventMap = message._eventMap as Map<string, ChatMessage>;
        const stream = message.agentEventStream as ChatMessage[];

        if (!data.done) {
          let thinkingEvent = eventMap.get(eventId || "");
          if (!thinkingEvent) {
            log("[Thinking] Creating new thinking event, event_id:", eventId);
            thinkingEvent = {
              type: "thinking",
              event_id: eventId,
              content: "",
              done: false,
              startTime: Date.now(),
              thinking: true,
            };
            stream.push(thinkingEvent);
            if (eventId) eventMap.set(eventId, thinkingEvent);
          }
          if (data.content) {
            thinkingEvent.content = String(thinkingEvent.content || "") + String(data.content);
            log("[Thinking] Event", eventId, "accumulated:", String(thinkingEvent.content).length, "chars");
          }
        } else {
          const thinkingEvent = eventMap.get(eventId || "");
          if (thinkingEvent) {
            thinkingEvent.done = true;
            thinkingEvent.thinking = false;
            thinkingEvent.duration_ms =
              dataPayload?.duration_ms || Date.now() - Number(thinkingEvent.startTime || Date.now());
            thinkingEvent.completed_at = dataPayload?.completed_at || Date.now();
            log("[Thinking] Event completed, duration:", thinkingEvent.duration_ms, "ms");
          } else {
            console.warn("[Thinking] Received done for unknown event_id:", eventId);
          }
        }
        break;
      }
      case "error": {
        const errorMsg = String(data.content || (dataPayload ? dataPayload.error : "") || t("chat.processError"));
        message.content = errorMsg;
        message.is_completed = true;
        isReplying.value = false;
        loading.value = false;
        fullContent.value = "";
        currentAssistantMessageId.value = "";
        reportError(errorMsg);
        console.error("[Chat Error]", errorMsg);
        break;
      }
      case "answer": {
        message.thinking = false;
        const eventId = dataPayload?.event_id as string | undefined;
        if (!message.agentEventStream) message.agentEventStream = [];
        if (!message._eventMap) message._eventMap = new Map();
        const eventMap = message._eventMap as Map<string, ChatMessage>;
        const stream = message.agentEventStream as ChatMessage[];

        let answerEvent = eventId ? eventMap.get(eventId) : stream.find((e) => e.type === "answer" && !e.event_id);
        if (!answerEvent) {
          answerEvent = { type: "answer", event_id: eventId, content: "", done: false };
          stream.push(answerEvent);
          if (eventId) eventMap.set(eventId, answerEvent);
        }
        if (!answerEvent.content && message.content && String(message.content).trim()) {
          answerEvent.content = message.content;
        }
        if (data.content) {
          answerEvent.content = String(answerEvent.content || "") + String(data.content);
          message.content = recomposeAgentAnswer(message);
          fullContent.value = String(message.content || "");
        }
        if (dataPayload?.is_fallback) {
          answerEvent.is_fallback = true;
          message.is_fallback = true;
        }
        if (data.done && !answerEvent.done) {
          answerEvent.done = true;
          onAnswerDone?.(message);
          loading.value = false;
          isReplying.value = false;
          fullContent.value = "";
          currentAssistantMessageId.value = "";
        }
        break;
      }
      case "complete": {
        log("[Stream] Complete event received");
        loading.value = false;
        isReplying.value = false;
        message.is_completed = true;
        onReplyComplete?.(String(message.content || ""));
        onTurnComplete?.(message);
        fullContent.value = "";
        currentAssistantMessageId.value = "";
        if (message.agentEventStream) {
          (message.agentEventStream as ChatMessage[]).push({
            type: "agent_complete",
            total_duration_ms: dataPayload?.total_duration_ms || 0,
            total_steps: dataPayload?.total_steps || 0,
          });
        }
        break;
      }
      case "stop": {
        log("[Stream] Stop event received");
        if (!message.agentEventStream) message.agentEventStream = [];
        (message.agentEventStream as ChatMessage[]).push({
          type: "stop",
          timestamp: Date.now(),
          reason: dataPayload?.reason || "user_requested",
        });
        message.is_completed = true;
        loading.value = false;
        isReplying.value = false;
        fullContent.value = "";
        currentAssistantMessageId.value = "";
        break;
      }
    }

    scrollToBottom();
  };

  // Agent 管线事件帧（tool_call / tool_result / 工具审批 / MCP OAuth / 记忆召回 /
  // reflection 等）：随 agent 能力一并移除，knowledge-chat 不会再发送，收到时安全忽略。
  const IGNORED_LEGACY_RESPONSE_TYPES = new Set([
    "tool_call",
    "tool_result",
    "tool_approval_required",
    "tool_approval_resolved",
    "mcp_oauth_required",
    "mcp_oauth_resolved",
    "memory_recalled",
    "reflection",
  ]);

  const processStreamChunk = (data: ChatMessage) => {
    log("[Stream Event Received]", {
      response_type: data.response_type,
      id: data.id,
      done: data.done,
      content_length: (data.content as string | undefined)?.length || 0,
      content_preview: data.content ? String(data.content).substring(0, 50) : "",
      data: data.data,
      session_id: data.session_id,
      assistant_message_id: data.assistant_message_id,
    });

    if (data.response_type === "agent_query") {
      if (data.id) {
        const earlyMsg = getTrailingIncompleteAssistant();
        if (earlyMsg) earlyMsg.request_id = data.id;
      }
      if (data.assistant_message_id) {
        currentAssistantMessageId.value = data.assistant_message_id as string;
        log("[Agent Query] Saved assistant message ID:", data.assistant_message_id);
      }
      log("[Agent Query Event]", {
        session_id: data.session_id || (data.data as ChatMessage | undefined)?.session_id,
        assistant_message_id: data.assistant_message_id,
        query: (data.data as ChatMessage | undefined)?.query,
        request_id: (data.data as ChatMessage | undefined)?.request_id,
      });

      let existingMessage = findLastMessage((item) => item.id === data.id || item.request_id === data.id);
      const created = !existingMessage;
      if (!existingMessage) {
        const assistantId = data.assistant_message_id as string | undefined;
        existingMessage = {
          id: assistantId || data.id,
          assistant_message_id: assistantId,
          request_id: data.id,
          role: "assistant",
          content: "",
          isRagMode: true,
          is_completed: false,
          agentEventStream: [],
          _eventMap: new Map(),
          knowledge_references: [],
        };
        messagesList.push(existingMessage);
        emitMessageCreated(existingMessage);
        loading.value = false;
        scrollToBottom(true);
        log("[Agent Query] Created assistant placeholder message");
      } else {
        ensureStreamMessageShell(existingMessage, data.id as string | undefined);
        if (data.assistant_message_id) {
          existingMessage.id = data.assistant_message_id as string;
          existingMessage.assistant_message_id = data.assistant_message_id;
        }
        log("[Agent Query] Continuing stream for existing message");
      }
      bindServerTurnTimestamps(
        messagesList,
        (data.data as Record<string, unknown> | undefined) || data,
        existingMessage,
      );
      onQueryEvent?.(data, existingMessage, created);
      return;
    }

    if (data.response_type === "references") {
      applyKnowledgeReferences(data);
      scrollToBottom();
      return;
    }

    if (IGNORED_LEGACY_RESPONSE_TYPES.has(data.response_type as string)) {
      return;
    }

    if (
      data.response_type === "thinking" ||
      data.response_type === "answer" ||
      data.response_type === "error" ||
      data.response_type === "complete" ||
      data.response_type === "stop"
    ) {
      handleStreamChunk(data);
      return;
    }

    // 旧版非事件流兜底：按 content 累积的经典路径（history / continue-stream
    // 之外的异常帧仍可走这里渲染）。
    const existingMessage = findLastMessage((item) => {
      if (item.request_id === data.id) return true;
      return item.id === data.id;
    });
    if (existingMessage?.is_completed && data.done && !data.content) {
      log("[Legacy] Ignoring duplicate completion event for completed message");
      return;
    }

    fullContent.value += (data.content as string) || "";
    const obj: ChatMessage = {
      ...data,
      content: "",
      role: "assistant",
      showThink: false,
      is_completed: false,
    };

    if ((data.data as ChatMessage | undefined)?.is_fallback) obj.is_fallback = true;

    const thinkCloseTag = "</think>";
    if (fullContent.value.includes("<think>") && !fullContent.value.includes(thinkCloseTag)) {
      obj.thinking = true;
      obj.showThink = true;
      obj.content = "";
      obj.thinkContent = fullContent.value.replace("<think>", "").trim();
    } else if (fullContent.value.includes("<think>") && fullContent.value.includes(thinkCloseTag)) {
      obj.thinking = false;
      obj.showThink = true;
      const index = fullContent.value.lastIndexOf(thinkCloseTag);
      obj.thinkContent = fullContent.value.substring(0, index).replace("<think>", "").trim();
      obj.content = fullContent.value.substring(index + thinkCloseTag.length).trim();
    } else {
      obj.content = fullContent.value;
    }

    if (!existingMessage) loading.value = false;

    if (data.done) {
      obj.is_completed = true;
      onReplyComplete?.(String(obj.content || ""));
      isReplying.value = false;
      fullContent.value = "";
      currentAssistantMessageId.value = "";
    }
    updateAssistantSession(obj);
    if (data.done) {
      const completed = resolveActiveAssistantMessage(data) || obj;
      onTurnComplete?.(completed);
    }
  };

  return {
    findLastMessage,
    shouldShowGlobalTypingIndicator,
    handleMsgList,
    processStreamChunk,
    prepareForNewOutgoingMessage,
    markInFlightAssistantStopped,
  };
}
