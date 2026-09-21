<template>
  <div
    class="chat"
    :class="{
      'is-sidebar-collapsed': uiStore.sidebarCollapsed,
      'has-references-panel': referencesDrawerVisible,
    }"
  >
    <ChatHeader :session="currentSession" :has-references-panel="referencesDrawerVisible" />
    <div ref="scrollContainer" class="chat_scroll_box" @scroll="handleScroll">
      <div class="msg_list">
        <!-- 消息列表骨架屏 -->
        <div v-if="historyLoading && messagesList.length === 0" class="msg-skeleton-list">
          <div class="msg-skeleton msg-skeleton-user">
            <t-skeleton animation="gradient" :row-col="[{ width: '45%', height: '36px', type: 'rect' }]" />
          </div>
          <div class="msg-skeleton msg-skeleton-bot">
            <t-skeleton
              animation="gradient"
              :row-col="[
                { width: '80%', height: '16px' },
                { width: '100%', height: '16px' },
                { width: '60%', height: '16px' },
              ]"
            />
          </div>
          <div class="msg-skeleton msg-skeleton-user">
            <t-skeleton animation="gradient" :row-col="[{ width: '35%', height: '36px', type: 'rect' }]" />
          </div>
          <div class="msg-skeleton msg-skeleton-bot">
            <t-skeleton
              animation="gradient"
              :row-col="[
                { width: '70%', height: '16px' },
                { width: '90%', height: '16px' },
              ]"
            />
          </div>
        </div>
        <!--
                  关键：必须用 session.id 作为 key，不能用 v-for 的索引。
                  向上滚动加载历史时会插入一批消息（push/unshift）到列表，
                  若用索引作 key 会让所有已渲染消息的 key 漂移，触发整个列表的销毁重建
                  （botmsg 全部重新挂载、markdown 重新渲染），
                  这是历史加载时白屏 + layout shift 蔓延到 session 列表的根因。
                  仅对极少数尚未拿到 id 的本地占位消息 fallback 到 role+created_at+index。
                -->
        <div
          v-for="(session, index) in messagesList"
          :key="session.id || `${session.role}-${session.created_at}-${index}`"
          class="msg-item-wrapper"
        >
          <MessageTimestamp v-if="shouldShowConversationTimestamp(messagesList, index)" :value="session.created_at" />

          <div v-if="session.role == 'user'" class="message-row">
            <usermsg
              :content="session.content"
              :mentioned_items="session.mentioned_items"
              :images="session.images"
              :attachments="session.attachments"
              :session-id="session_id"
            >
            </usermsg>
          </div>
          <div v-if="session.role == 'assistant'" class="message-row">
            <botmsg
              :content="session.content"
              :session="session"
              :session-id="session_id"
              :user-query="getUserQuery(index)"
              @scroll-bottom="scrollToBottom"
              :isFirstEnter="isFirstEnter"
              :follow-up-loading="Boolean(session.suggestionLoading && !session.suggestionSet?.questions?.length)"
              @render-complete-change="(ready) => handleAnswerRenderComplete(session, ready)"
            >
            </botmsg>
            <FollowUpSuggestions
              v-if="session.answerFullyRendered && !session.suggestionsDismissed"
              :suggestion-set="session.suggestionSet"
              :loading="session.suggestionLoading"
              :allow-regenerate="session.suggestionSet?.allow_regenerate"
              @select="(item) => handleFollowUpSelect(session, item)"
              @regenerate="loadFollowUpSuggestions(session, true, true)"
              @impression="(set) => recordSuggestionEvent(session, set, 'impression')"
              @dismiss="(set) => dismissSuggestions(session, set)"
            />
          </div>
        </div>
        <div
          v-if="showGlobalTypingIndicator"
          class="chat-global-wait"
          role="status"
          :aria-label="t('chat.thinkingAlt')"
        >
          <span class="chat-global-wait__spinner" aria-hidden="true"></span>
        </div>
      </div>
    </div>
    <transition name="scroll-btn-fade">
      <div v-show="userHasScrolledUp" class="scroll-to-bottom-btn" @click="onClickScrollToBottom">
        <t-icon name="chevron-down" size="20px" />
      </div>
    </transition>
    <div class="input-container">
      <InputField
        ref="inputFieldRef"
        @send-msg="
          (query, modelId, mentionedItems, imageFiles, attachmentFiles) =>
            sendMsg(query, modelId, mentionedItems, imageFiles, attachmentFiles)
        "
        @stop-generation="handleStopGeneration"
        :isReplying="isReplying"
        :sessionId="session_id"
        :assistantMessageId="currentAssistantMessageId"
      ></InputField>
    </div>
  </div>
  <KnowledgeBaseEditorModal
    :visible="uiStore.showKBEditorModal"
    :mode="uiStore.kbEditorMode"
    :kb-id="uiStore.currentKBId || undefined"
    :initial-type="uiStore.kbEditorType"
    @update:visible="(val) => (val ? null : uiStore.closeKBEditor())"
    @success="handleKBEditorSuccess"
  />
  <ChatReferencesDrawer />
  <ChatAttachmentPreviewDrawer />
</template>
<script setup>
import { storeToRefs } from "pinia";
import { ref, onMounted, onBeforeMount, onUnmounted, nextTick, watch, reactive, computed } from "vue";
import { useRoute, onBeforeRouteLeave, onBeforeRouteUpdate } from "vue-router";
import InputField from "../../components/Input-field.vue";
import botmsg from "./components/botmsg.vue";
import usermsg from "./components/usermsg.vue";
import { getMessageList, getSession } from "@/api/chat/index";
import { deleteTemporaryAttachment, uploadTemporaryAttachment } from "@/api/chat/temporary-attachments";
import { useStream } from "../../api/chat/streame";
import { useMenuStore } from "@/stores/menu";
import { useSettingsStore } from "@/stores/settings";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { useUIStore } from "@/stores/ui";
import KnowledgeBaseEditorModal from "@/views/knowledge/KnowledgeBaseEditorModal.vue";
import { useKnowledgeBaseCreationNavigation } from "@/hooks/useKnowledgeBaseCreationNavigation";
import { useChatStreamHandler } from "@/composables/useChatStreamHandler";
import { useStickyBottomOnResize } from "@/composables/useStickyBottomOnResize";
import { clearCitationChunkCache } from "@/utils/citationChunkCache";
import ChatReferencesDrawer from "@/components/ChatReferencesDrawer.vue";
import ChatAttachmentPreviewDrawer from "@/components/ChatAttachmentPreviewDrawer.vue";
import FollowUpSuggestions from "@/components/chat/FollowUpSuggestions.vue";
import MessageTimestamp from "@/components/chat/MessageTimestamp.vue";
import { shouldShowConversationTimestamp } from "@/utils/messageTimestamp";
import ChatHeader from "@/components/ChatHeader.vue";
import { notifySessionMutation, SESSION_MUTATION_EVENT } from "@/components/sessionMutations";
import {
  ensureMessageSuggestions,
  getMessageSuggestions,
  recordMessageSuggestionEvent,
} from "@/api/message-suggestion";
import { provideChatReferencesDrawer } from "@/composables/useChatReferencesDrawer";
import { provideChatAttachmentPreviewDrawer } from "@/composables/useChatAttachmentPreviewDrawer";
const referencesDrawer = provideChatReferencesDrawer();
provideChatAttachmentPreviewDrawer();
const { visible: referencesDrawerVisible } = referencesDrawer;

const props = defineProps({
  session_id: { type: String, default: "" },
});

const usemenuStore = useMenuStore();
const useSettingsStoreInstance = useSettingsStore();

const uiStore = useUIStore();
const { navigateToKnowledgeBaseList } = useKnowledgeBaseCreationNavigation();
const { t } = useI18n();
const { firstQuery, firstMentionedItems, firstModelId, firstImageFiles, firstAttachmentFiles } =
  storeToRefs(usemenuStore);
const { onChunk, error, startStream, stopStream, lastStreamRequest } = useStream();
/** Snapshot of the in-flight HTTP request for attaching to the next assistant message. */
const pendingStreamDebug = ref(null);

const buildStreamDebugPayload = () => {
  const meta = lastStreamRequest.value;
  if (!meta) return null;
  return {
    requestId: meta.requestId,
    url: meta.url,
    method: meta.method,
    body: meta.body,
    sentAt: meta.sentAt,
    sessionId: session_id.value,
  };
};

const attachStreamDebugToMessage = (message) => {
  if (!message) return;
  const payload = pendingStreamDebug.value || buildStreamDebugPayload();
  if (!payload) return;
  if (payload.requestId && !message.request_id) {
    message.request_id = payload.requestId;
  }
  message.debugRequest = payload;
};
const route = useRoute();
const session_id = ref(props.session_id || route.params.chatid);
const currentSession = ref(null);

// 拉 session 详情，并按其 last_request_state 把输入栏状态恢复到当时的发起态。
const loadSessionAndHydrate = async (sid) => {
  if (!sid) return;
  try {
    const sessionRes = await getSession(sid);
    if (sessionRes?.data && sid === session_id.value) {
      currentSession.value = sessionRes.data;
      const lastState = sessionRes.data.last_request_state;
      if (lastState) {
        // 先把当前的"全局默认"快照下来，再用 session 状态覆盖；
        // 离开会话时会从快照还原，避免本会话的状态污染新建对话。
        useSettingsStoreInstance.snapshotAsDefaultsIfNeeded();
        useSettingsStoreInstance.applyLastRequestState(lastState);
      }
    }
  } catch (error) {
    console.error("Failed to load session data:", error);
  }
};
const inputFieldRef = ref();
const created_at = ref("");
const limit = ref(20);
const messagesList = reactive([]);
const isReplying = ref(false);
const currentAssistantMessageId = ref(""); // 当前正在生成的 assistant message ID
const scrollLock = ref(false);
const isFirstEnter = ref(true);
const loading = ref(false);
const historyLoading = ref(true);
const historyLoadingMore = ref(false);
const hasMoreHistory = ref(true);
const fullContent = ref("");
const scrollContainer = ref(null);
const userHasScrolledUp = ref(false);
const SCROLL_BOTTOM_THRESHOLD = 80;

const isNearBottom = () => {
  if (!scrollContainer.value) return true;
  const { scrollTop, scrollHeight, clientHeight } = scrollContainer.value;
  return scrollHeight - scrollTop - clientHeight < SCROLL_BOTTOM_THRESHOLD;
};

const handleKBEditorSuccess = (kbId) => {
  navigateToKnowledgeBaseList(kbId);
};

const resolveAssistantMessageId = (message) => message?.assistant_message_id || message?.id;

const handleAnswerRenderComplete = (message, ready) => {
  message.answerFullyRendered = Boolean(ready);
};

const loadFollowUpSuggestions = async (message, ensure = false, regenerate = false) => {
  const messageId = resolveAssistantMessageId(message);
  const targetSessionId = session_id.value;
  if (!messageId || !targetSessionId || message.suggestionsDismissed) return;
  message.suggestionLoading = true;
  try {
    let response = ensure
      ? await ensureMessageSuggestions(targetSessionId, messageId, regenerate)
      : await getMessageSuggestions(targetSessionId, messageId);
    let set = response?.data;
    for (let attempt = 0; set?.status === "generating" && attempt < 120; attempt++) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      if (session_id.value !== targetSessionId || message.suggestionsDismissed) return;
      response = await getMessageSuggestions(targetSessionId, messageId);
      set = response?.data;
    }
    message.suggestionSet = set?.status === "ready" ? set : null;
  } catch (error) {
    if (ensure) console.warn("[FollowUpSuggestions] Failed to generate:", error);
    message.suggestionSet = null;
  } finally {
    message.suggestionLoading = false;
  }
};

const recordSuggestionEvent = (message, set, eventType, questionId = "") => {
  if (!set?.id) return;
  void recordMessageSuggestionEvent(session_id.value, set.id, eventType, questionId).catch(() => undefined);
};

const handleFollowUpSelect = (message, item) => {
  recordSuggestionEvent(message, message.suggestionSet, "click", item.id);
  pendingSuggestionAttribution = {
    suggestion_set_id: message.suggestionSet.id,
    question_id: item.id,
  };
  // Knowledge-backed follow-ups are generated from a specific KB. Keep that
  // authorized retrieval anchor for the immediate next request.
  pendingSuggestionKnowledgeBaseIds = [...new Set(item.knowledge_base_ids || [])];
  if (inputFieldRef.value?.triggerSend) inputFieldRef.value.triggerSend(item.text);
  else sendMsg(item.text);
};

const dismissSuggestions = (message, set) => {
  message.suggestionsDismissed = true;
  recordSuggestionEvent(message, set, "dismiss");
};

let pendingSuggestionAttribution = null;
let pendingSuggestionKnowledgeBaseIds = [];

watch([() => route.params], async (newvalue) => {
  isFirstEnter.value = true;
  if (newvalue[0].chatid) {
    if (!firstQuery.value) {
      scrollLock.value = false;
    }
    messagesList.splice(0);
    session_id.value = newvalue[0].chatid;
    currentSession.value = null;
    clearCitationChunkCache();

    // 切换会话时，重置状态
    historyLoading.value = true;
    historyLoadingMore.value = false;
    hasMoreHistory.value = true;
    created_at.value = "";
    loading.value = false;
    isReplying.value = false;
    currentAssistantMessageId.value = "";
    userHasScrolledUp.value = false;

    // 跨会话切换：先把旧会话覆盖前的全局默认还原，再让新会话重新拍快照
    // 并应用自己的 last_request_state（在 loadSessionAndHydrate 内部完成）。
    useSettingsStoreInstance.restoreDefaultsIfSnapshotted();

    await loadSessionAndHydrate(session_id.value);
    const data = {
      session_id: session_id.value,
      created_at: "",
      limit: limit.value,
    };
    getmsgList(data);
  }
});
const scrollToBottom = (force = false) => {
  if (!force && userHasScrolledUp.value) return;
  nextTick(() => {
    if (scrollContainer.value) {
      scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight;
    }
  });
};
const onClickScrollToBottom = () => {
  userHasScrolledUp.value = false;
  scrollToBottom(true);
};

// Images and other rich Markdown content can grow after the SSE chunk that
// introduced them. Follow those delayed height changes while the user remains
// at the live edge; preserve position when they intentionally scroll upward.
useStickyBottomOnResize(scrollContainer, userHasScrolledUp, scrollToBottom);

const debounce = (fn, delay) => {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args));
  };
};
const onChatScrollTop = () => {
  if (scrollLock.value || historyLoadingMore.value || !hasMoreHistory.value) return;
  if (!scrollContainer.value) return;
  const { scrollTop, scrollHeight } = scrollContainer.value;
  isFirstEnter.value = false;
  if (scrollTop <= 0) {
    const data = {
      session_id: session_id.value,
      created_at: created_at.value,
      limit: limit.value,
    };
    getmsgList(data, true, scrollHeight);
  }
};
const debouncedScrollTop = debounce(onChatScrollTop, 500);
let lastScrollTop = 0;
const handleScroll = () => {
  const el = scrollContainer.value;
  if (el) {
    const currentTop = el.scrollTop;
    // Only an actual upward scroll detaches from the live edge. Content that
    // grows after a chunk (images, diagrams) keeps scrollTop fixed and would
    // otherwise fire a stale scroll event that falsely marks the user as
    // scrolled up, killing the auto-follow during streaming.
    if (currentTop < lastScrollTop - 1) {
      userHasScrolledUp.value = !isNearBottom();
    } else if (isNearBottom()) {
      userHasScrolledUp.value = false;
    }
    lastScrollTop = currentTop;
  }
  debouncedScrollTop();
};

const fetchMessageList = (data) => getMessageList(data);

const {
  shouldShowGlobalTypingIndicator,
  handleMsgList,
  processStreamChunk,
  prepareForNewOutgoingMessage,
  markInFlightAssistantStopped,
} = useChatStreamHandler({
  messagesList,
  loading,
  isReplying,
  currentAssistantMessageId,
  fullContent,
  scrollToBottom,
  onError: (msg) => MessagePlugin.error(msg),
  preserveIncompleteStreamReactive: true,
  isFirstEnter,
  scrollContainer,
  debug: import.meta.env.DEV,
  onAfterMsgList: async () => {
    for (const message of messagesList) {
      if (message.role === "assistant" && message.is_completed && message.suggestionSet === undefined) {
        void loadFollowUpSuggestions(message, false);
      }
    }
    const lastMessage = messagesList[messagesList.length - 1];
    if (lastMessage && !lastMessage.is_completed) {
      isReplying.value = true;
      if (lastMessage.role === "assistant") {
        currentAssistantMessageId.value = lastMessage.id;
        console.log("[Continue Stream] Set assistant message ID:", lastMessage.id);
      }
      await startStream({
        session_id: session_id.value,
        query: lastMessage.id,
        method: "GET",
        url: "/api/v1/sessions/continue-stream",
      });
    }
  },
  onQueryEvent: (data, existingMessage) => {
    pendingStreamDebug.value = buildStreamDebugPayload();
    if (existingMessage) attachStreamDebugToMessage(existingMessage);
  },
  onMessageCreated: (message) => attachStreamDebugToMessage(message),
  onMessageUpdated: (message, payload) => {
    attachStreamDebugToMessage(message);
    if (payload?.is_completed) pendingStreamDebug.value = null;
  },
  onAnswerDone: (message) => {
    attachStreamDebugToMessage(message);
    pendingStreamDebug.value = null;
  },
  onChunkBound: (message) => {
    attachStreamDebugToMessage(message);
    pendingStreamDebug.value = null;
  },
  onTurnComplete: (message) => {
    void loadFollowUpSuggestions(message, true);
  },
});

const showGlobalTypingIndicator = computed(() => shouldShowGlobalTypingIndicator(messagesList, loading.value));

const getmsgList = (data, isScrollType = false, scrollHeight) => {
  if (isScrollType) {
    if (historyLoadingMore.value || !hasMoreHistory.value) return;
    historyLoadingMore.value = true;
  }
  fetchMessageList(data)
    .then(async (res) => {
      const batch = res?.data;
      if (!batch?.length) {
        if (isScrollType) {
          hasMoreHistory.value = false;
        }
        return;
      }
      const nextCursor = batch[0].created_at;
      if (isScrollType && created_at.value && nextCursor === created_at.value) {
        hasMoreHistory.value = false;
        return;
      }
      if (batch.length < limit.value) {
        hasMoreHistory.value = false;
      }
      created_at.value = nextCursor;
      await handleMsgList(batch, isScrollType, scrollHeight);
    })
    .catch((err) => {
      console.error("Failed to load messages:", err);
      if (isScrollType) {
        hasMoreHistory.value = false;
      }
    })
    .finally(() => {
      historyLoading.value = false;
      historyLoadingMore.value = false;
    });
};

// 发送消息
// 处理停止生成事件 - 立即清除 loading 状态
const handleStopGeneration = () => {
  console.log("[Stop Generation] Immediately clearing loading state");
  stopStream();
  loading.value = false;
  isReplying.value = false;
  // 标记当前 assistant 为已结束，避免下一条 query 复用该消息行
  markInFlightAssistantStopped(currentAssistantMessageId.value);
  // 保留 currentAssistantMessageId，Input-field 仍需用它调用 stop API
};

const sendMsg = async (value, modelId = "", mentionedItems = [], imageFiles = [], attachmentFiles = []) => {
  stopStream();
  prepareForNewOutgoingMessage();
  isReplying.value = true;
  loading.value = true;

  // Images are unified with the attachment pipeline: they upload as temporary
  // documents (understood in the background by the VLM) and are sent as
  // attachment_ids. A base64 fallback is used per-image if the async upload
  // fails.
  const imageAttachments = [];
  const userImages = [];
  const imageAttachmentIds = [];
  if (imageFiles && imageFiles.length > 0) {
    for (const file of imageFiles) {
      let dataURI;
      try {
        dataURI = await fileToBase64(file);
      } catch (e) {
        console.error("[Image] Failed to read images:", e);
        loading.value = false;
        isReplying.value = false;
        return;
      }
      userImages.push({ url: dataURI });
      try {
        const upload = await uploadTemporaryAttachment(session_id.value, file, "auto");
        imageAttachmentIds.push(upload.data.id);
      } catch (e) {
        console.error("[Image] Temporary image upload failed, falling back to inline:", e);
        imageAttachments.push({ data: dataURI });
      }
    }
  }

  // The create-chat page cannot upload before its session exists. Once it
  // navigates here, move those local files through the same asynchronous
  // upload/parse flow before starting the first stream.
  const localAttachments = (attachmentFiles || []).filter((attachment) => !attachment.documentId);
  if (localAttachments.length > 0) {
    try {
      // Only upload to obtain a document ID; parsing continues in the
      // background and is awaited by the backend (shown on the timeline).
      await Promise.all(
        localAttachments.map(async (attachment) => {
          attachment.status = "uploading";
          const upload = await uploadTemporaryAttachment(session_id.value, attachment.file, "auto");
          attachment.documentId = upload.data.id;
          attachment.status = upload.data.status;
        }),
      );
    } catch (error) {
      console.error("[Attachment] Temporary document upload failed:", error);
      await Promise.all(
        localAttachments
          .filter((attachment) => attachment.documentId)
          .map((attachment) =>
            deleteTemporaryAttachment(session_id.value, attachment.documentId).catch(() => undefined),
          ),
      );
      MessagePlugin.error(error?.message || t("chat.attachmentParseFailed"));
      loading.value = false;
      isReplying.value = false;
      return;
    }
  }

  // Send any successfully uploaded attachment (parsing may still be running);
  // the backend waits for readiness and reports progress on the timeline.
  const attachmentIds = (attachmentFiles || [])
    .filter((attachment) => attachment.documentId && attachment.status !== "failed")
    .map((attachment) => attachment.documentId);
  attachmentIds.push(...imageAttachmentIds);

  // 将@提及的知识库和文件信息存入用户消息
  messagesList.push({
    content: value,
    role: "user",
    mentioned_items: mentionedItems,
    images: userImages,
    attachments: attachmentFiles.map((a) => ({
      id: a.documentId,
      file_name: a.name,
      file_size: a.size,
      file_type: "." + a.name.split(".").pop()?.toLowerCase(),
    })),
    channel: "web",
    created_at: new Date().toISOString(),
  });
  userHasScrolledUp.value = false;
  scrollToBottom(true);

  // Get knowledge_base_ids from settings store (selected by user via @mention)
  // Merge @mentioned KB/file IDs so retrieval uses the same targets user @mentioned (including shared KBs)
  const sidebarKbIds = useSettingsStoreInstance.settings.selectedKnowledgeBases || [];
  const sidebarFileIds = useSettingsStoreInstance.settings.selectedFiles || [];
  const kbIdSet = new Set(sidebarKbIds);
  const fileIdSet = new Set(sidebarFileIds);
  for (const kbId of pendingSuggestionKnowledgeBaseIds) {
    if (kbId) kbIdSet.add(kbId);
  }
  for (const item of mentionedItems || []) {
    if (!item?.id) continue;
    if (item.type === "kb" && !kbIdSet.has(item.id)) {
      kbIdSet.add(item.id);
    } else if (item.type === "file" && !fileIdSet.has(item.id)) {
      fileIdSet.add(item.id);
    }
  }
  const kbIds = [...kbIdSet];
  const knowledgeIds = [...fileIdSet];
  const tagIds = [
    ...new Set((mentionedItems || []).filter((item) => item.type === "tag" && item.id).map((item) => item.id)),
  ];

  const suggestionAttribution = pendingSuggestionAttribution;
  pendingSuggestionAttribution = null;
  pendingSuggestionKnowledgeBaseIds = [];
  await startStream({
    session_id: session_id.value,
    knowledge_base_ids: kbIds,
    knowledge_ids: knowledgeIds,
    tag_ids: tagIds,
    summary_model_id: modelId,
    mentioned_items: mentionedItems,
    images: imageAttachments.length > 0 ? imageAttachments : undefined,
    attachment_ids: attachmentIds.length > 0 ? attachmentIds : undefined,
    query: value,
    suggestion_attribution: suggestionAttribution || undefined,
    method: "POST",
    url: "/api/v1/knowledge-chat",
  });
};

// Watch for stream errors and show message
watch(error, (newError) => {
  if (!newError) return;
  MessagePlugin.error(newError);
  isReplying.value = false;
  loading.value = false;
  // 清空当前 assistant message ID
  currentAssistantMessageId.value = "";
});

onChunk((data) => {
  if (data.response_type === "session_title") {
    const title = data.content || data.data?.title;
    if (title && data.data?.session_id) {
      console.log("[Session Title Update]", {
        session_id: data.data.session_id,
        title: title,
      });
      usemenuStore.updatasessionTitle(data.data.session_id, title);
      usemenuStore.changeIsFirstSession(false);
      notifySessionMutation({
        sessionId: data.data.session_id,
        patch: { title },
      });
    }
    return;
  }
  processStreamChunk(data);
});

const handleSessionMutation = (event) => {
  const detail = event.detail;
  if (detail?.sessionId !== session_id.value) return;

  if (detail.patch) {
    currentSession.value = {
      ...(currentSession.value || { id: session_id.value }),
      ...detail.patch,
    };
  }
  if (detail.messagesCleared) {
    messagesList.splice(0);
    created_at.value = "";
    hasMoreHistory.value = true;
    historyLoadingMore.value = false;
  }
};

onBeforeMount(async () => {
  // 必须在 Input-field onMounted 之前完成：按 session.last_request_state 恢复输入栏
  await loadSessionAndHydrate(session_id.value);
});

onMounted(async () => {
  window.addEventListener(SESSION_MUTATION_EVENT, handleSessionMutation);
  messagesList.splice(0);

  // 初始化状态：加载历史消息时不应显示loading
  loading.value = false;
  isReplying.value = false;

  if (firstQuery.value) {
    scrollLock.value = true;
    historyLoading.value = false;
    if (firstModelId.value) {
      useSettingsStoreInstance.updateConversationModels({
        summaryModelId: firstModelId.value,
        selectedChatModelId: firstModelId.value,
        rerankModelId: "",
      });
    }
    sendMsg(
      firstQuery.value,
      firstModelId.value || "",
      firstMentionedItems.value || [],
      firstImageFiles.value || [],
      firstAttachmentFiles.value || [],
    );
    usemenuStore.changeFirstQuery("", [], "", [], []);
  } else {
    scrollLock.value = false;
    hasMoreHistory.value = true;
    historyLoadingMore.value = false;
    const data = {
      session_id: session_id.value,
      created_at: "",
      limit: limit.value,
    };
    getmsgList(data);
  }
});
function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

const getUserQuery = (index) => {
  if (index <= 0) {
    return "";
  }
  const previous = messagesList[index - 1];
  if (previous && previous.role === "user") {
    return previous.content || "";
  }
  return "";
};

const clearData = () => {
  stopStream();
  referencesDrawer.close();
  isReplying.value = false;
  fullContent.value = "";
};
onUnmounted(() => {
  window.removeEventListener(SESSION_MUTATION_EVENT, handleSessionMutation);
});
onBeforeRouteLeave((to, from, next) => {
  clearData();
  // 离开聊天会话 → 还原"用户全局默认"，避免旧会话的请求态泄漏到新建对话。
  useSettingsStoreInstance.restoreDefaultsIfSnapshotted();
  next();
});
onBeforeRouteUpdate((to, from, next) => {
  clearData();
  // 仅"会话 → 会话"会落到这里；跨会话覆盖的还原放到 route.params 的 watch 里，
  // 因为新会话的 getSession 也在那边触发，便于保证 restore→snapshot→apply 顺序。
  next();
});
</script>
<style lang="less" scoped>
.chat {
  font-size: 20px;
  // 右侧不留 padding，滚动条贴到内容区最右缘
  padding: 0 0 20px 20px;
  box-sizing: border-box;
  flex: 1;
  // The parent .platform-route-outlet is a flex column with min-height:0
  // and overflow:hidden — we also need min-height:0 here so that our
  // own flex:1 child (.chat_scroll_box) can shrink below its content
  // height and scroll instead of pushing .input-container out of view.
  min-height: 0;
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  max-width: calc(100vw - 260px);
  min-width: 400px;

  &.is-sidebar-collapsed {
    max-width: calc(100vw - 60px);
  }

  &.has-references-panel {
    @media (min-width: 960px) {
      padding-right: 420px;
      box-sizing: border-box;

      .chat_scroll_box {
        padding-top: 0;
      }
    }
  }

  :deep(.answers-input) {
    position: static;
    transform: translateX(0);

    .t-textarea__inner {
      width: 100% !important;
    }

    @media (min-width: 960px) {
      transition: padding-right 0.3s cubic-bezier(0.22, 0.61, 0.36, 1);
    }
  }
}

.chat_scroll_box {
  flex: 1;
  // Without min-height: 0, a flex-column child defaults to min-height: auto
  // and expands to fit all inner content. When there are many messages,
  // that pushes .input-container out of the viewport. Clamping min-height
  // to 0 lets overflow-y: auto take effect so the messages scroll inside
  // this box instead of stretching it.
  min-height: 0;
  width: 100%;
  padding-top: 8px;
  box-sizing: border-box;
  overflow-y: auto;
  // 使用系统原生滚动条（macOS 滚动时自动显示 overlay 滚动条，类似 ChatGPT）
  scrollbar-width: auto;
  scrollbar-color: auto;
}

// 深色模式下 theme.css 对 * 做了 webkit 滚动条着色，这里恢复为系统默认
:global(:root[theme-mode="dark"]) .chat_scroll_box {
  &::-webkit-scrollbar-thumb {
    background-color: initial !important;
  }

  &::-webkit-scrollbar-thumb:hover {
    background-color: initial !important;
  }

  &::-webkit-scrollbar-track {
    background-color: initial !important;
  }
}

.scroll-to-bottom-btn {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  bottom: 140px;
  z-index: 10;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--td-text-color-secondary);
  transition: all 0.2s ease;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  }

  &:active {
    transform: translateX(-50%) scale(0.92);
  }
}

.scroll-btn-fade-enter-active,
.scroll-btn-fade-leave-active {
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}

.scroll-btn-fade-enter-from,
.scroll-btn-fade-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(8px);
}

@keyframes contentFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.msg-skeleton-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 960px;
  padding: 16px 0;
  animation: contentFadeIn 0.3s ease-out;
}

.msg-skeleton-user {
  display: flex;
  justify-content: flex-end;
}

.msg-skeleton-bot {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-left: 4px;
}

.input-container {
  min-height: 115px;
  flex-shrink: 0;
  margin: 0 auto;
  width: 100%;
  max-width: 960px;
  box-sizing: border-box;
  position: relative;
}

.msg_list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 960px;
  flex: 1;
  margin: 0 auto;
  width: 100%;

  /*
      给每条消息加 layout/style containment：
      - 一条消息的内部布局变化不再让浏览器去 invalidate 整个文档，
        这是修掉"hover 到 session 列表也变白"那个问题的关键。
      - 不要再用 content-visibility: auto / contain-intrinsic-size：
        消息真实高度差异巨大（几百 ~ 数千 px），估的占位高度会让消息进入视口时
        反复发生"占位 -> 真实高度"的大幅 layout shift + 首次 paint 滞后，
        反而在向上滚动时制造"未画完"的白屏闪烁。
        当前 handleMsgList 全流程 ~50ms，根本无需跳过渲染，老老实实正常渲染最稳。
      - 不开 contain: paint：消息里有 tooltip / popover 等会溢出的浮层，
        paint containment 会把它们裁掉。
    */
  .msg-item-wrapper {
    contain: layout style;
  }

  .message-row {
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  .botanswer_laoding_gif {
    width: 24px;
    height: 18px;
    margin-left: 16px;
  }

  .chat-global-wait {
    display: flex;
    align-items: center;
    min-height: 28px;
    padding-left: 4px;
  }

  .chat-global-wait__spinner {
    width: 12px;
    height: 12px;
    box-sizing: border-box;
    border: 1.5px solid var(--td-component-stroke);
    border-top-color: var(--td-text-color-secondary);
    border-radius: 50%;
    animation: chatGlobalWaitSpin 0.8s linear infinite;
  }
}

@keyframes chatGlobalWaitSpin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .chat-global-wait__spinner {
    animation: none;
  }
}
</style>
