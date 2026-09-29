<template>
  <div class="text-foreground mr-auto box-border max-w-full rounded text-base">
    <div class="flex flex-col gap-2">
      <!-- 显示@的知识库和文件 -->
      <div v-if="mentionedItems && mentionedItems.length > 0" class="chat-mentioned-items">
        <span v-for="item in mentionedItems" :key="item.id" class="chat-mentioned-tag" :class="[mentionTagClass(item)]">
          <span class="tag_icon">
            <FolderIcon v-if="item.type === 'kb' && item.kb_type !== 'faq'" class="h-3.5 w-3.5" />
            <MessageCircleQuestionIcon v-else-if="item.type === 'kb'" class="h-3.5 w-3.5" />
            <TagIcon v-else-if="item.type === 'tag'" class="h-3.5 w-3.5" />
            <FileIcon v-else class="h-3.5 w-3.5" />
          </span>
          <span class="tag_name">{{ item.name }}</span>
        </span>
      </div>
      <!-- RAG 问答：检索/附件流水线时间线（含思考过程卡片） -->
      <RagPipelineProgress :session="session" />
      <deepThink :deepSession="session" v-if="session.showThink"></deepThink>
    </div>
    <!-- 回答正文 markdown 渲染 -->
    <div ref="parentMd">
      <!-- 直接渲染完整内容，避免切分导致的问题，样式与 thinking 一致 -->
      <!-- 只有当有实际内容时才显示包围框 -->
      <div v-if="hasActualContent" class="py-[2px]">
        <div
          class="ai-markdown-template markdown-content chat-markdown-typography chat-citation-pills"
          v-stable-html="renderedHTML"
        ></div>
      </div>
      <!-- 复制和添加到知识库按钮 -->
      <div v-if="answerFullyRendered && (content || session.content)" class="answer-toolbar">
        <!-- The toolbar's buttons wear the look the toolbar gave TDesign buttons: a 30px square,
             transparent, borderless, muted, with thin-stroke 16px glyphs. ChatRequestInfoButton
             beside them carries the same classes. -->
        <button
          type="button"
          data-slot="answer-toolbar-button"
          class="text-muted-foreground hover:bg-accent hover:text-foreground inline-flex size-[30px] shrink-0 items-center justify-center rounded-[8px] transition-[background-color,color] duration-150 ease-in-out active:bg-[var(--td-bg-color-container-active)]"
          :title="$t('agent.copy')"
          :aria-label="$t('agent.copy')"
          @click.stop="handleCopyAnswer"
        >
          <CopyIcon class="size-4 stroke-[1.2]" aria-hidden="true" />
        </button>
        <button
          type="button"
          data-slot="answer-toolbar-button"
          class="text-muted-foreground hover:bg-accent hover:text-foreground inline-flex size-[30px] shrink-0 items-center justify-center rounded-[8px] transition-[background-color,color] duration-150 ease-in-out active:bg-[var(--td-bg-color-container-active)]"
          :title="$t('agent.addToKnowledgeBase')"
          :aria-label="$t('agent.addToKnowledgeBase')"
          @click.stop="handleAddToKnowledge"
        >
          <BookmarkPlusIcon class="size-4 stroke-[1.2]" aria-hidden="true" />
        </button>
        <!-- Fallback 提示图标: dimmer than its neighbours, it only explains, it does nothing. -->
        <Tooltip v-if="session.is_fallback">
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="answer-toolbar-button"
              class="hover:bg-accent hover:text-placeholder inline-flex size-[30px] shrink-0 items-center justify-center rounded-[8px] text-[var(--td-text-color-disabled)] transition-[background-color,color] duration-150 ease-in-out active:bg-[var(--td-bg-color-container-active)]"
              :aria-label="$t('chat.fallbackHint')"
            >
              <InfoIcon class="size-4 stroke-[1.2]" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{{ $t("chat.fallbackHint") }}</TooltipContent>
        </Tooltip>
        <ChatRequestInfoButton v-if="showRequestInfo" :session="session" :session-id="sessionId" />
        <transition name="follow-up-toolbar-loading">
          <span v-if="followUpLoading" class="answer-toolbar__follow-up-loading" role="status" aria-live="polite">
            <LightbulbIcon class="h-3.5 w-3.5" />
            <span class="answer-toolbar__follow-up-label">{{ t("chat.followUpQuestionsLoading") }}</span>
          </span>
        </transition>
      </div>
      <div
        v-if="isImgLoading"
        class="bg-accent text-placeholder ml-4 flex h-[230px] w-[230px] flex-col items-center justify-center gap-1 rounded-lg text-xs"
      >
        <Loader2Icon class="h-4 w-4 animate-spin" /><span>{{ $t("common.loading") }}</span>
      </div>
    </div>
    <picturePreview :reviewImg="reviewImg" :reviewUrl="reviewUrl" @closePreImg="closePreImg"></picturePreview>
    <Teleport to="body">
      <ChatCitationFloat :float="citationFloat" :on-enter="cancelCitationClose" :on-leave="scheduleCitationClose" />
    </Teleport>
  </div>
</template>
<script setup>
import { onMounted, onBeforeUnmount, watch, computed, ref, nextTick, onUpdated } from "vue";
import "katex/dist/katex.min.css";
import {
  BookmarkPlusIcon,
  CopyIcon,
  FileIcon,
  FolderIcon,
  InfoIcon,
  Loader2Icon,
  LightbulbIcon,
  MessageCircleQuestionIcon,
  TagIcon,
} from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import deepThink from "./deepThink.vue";
import RagPipelineProgress from "./RagPipelineProgress.vue";
import ChatRequestInfoButton from "@/components/ChatRequestInfoButton.vue";
import ChatCitationFloat from "@/components/ChatCitationFloat.vue";
import picturePreview from "@/components/picture-preview.vue";
import {
  sanitizeMarkdownHTML,
  safeMarkdownToHTML,
  createSafeImage,
  isValidImageURL,
  hydrateProtectedFileImages,
} from "@/utils/security";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { useUIStore } from "@/stores/ui";
import { buildManualMarkdown, formatManualTitle } from "@/utils/chatMessageShared";
import { copyWithToast } from "@/utils/clipboard";
import { createChatMarkdownRenderer, renderChatMarkdown } from "@/utils/chatMarkdownRenderer";
import {
  createMermaidCodeRenderer,
  ensureMermaidInitialized,
  renderMermaidInContainer,
  enhanceMarkdownContainer,
} from "@/utils/mermaidShared";
import { refreshMarkdownEnhancements } from "@/utils/markdownEnhancements";
import { useChatCitationPopover } from "@/composables/useChatCitationPopover";
import { useTypewriter } from "@/composables/useTypewriter";
import { vStableHtml } from "@/directives/stableHtml";
import "@/components/css/chat-markdown.css";
import "@/components/css/chat-message-shared.css";
import "@/components/css/chat-citations.css";
import "@/components/css/chat-resource-chips.css";

ensureMermaidInitialized();

const mentionTagClass = (item) => {
  if (item.type === "kb") return item.kb_type === "faq" ? "faq-tag" : "kb-tag";
  return `${item.type || "file"}-tag`;
};

const emit = defineEmits(["scroll-bottom", "render-complete-change"]);
const { t } = useI18n();
const uiStore = useUIStore();
const parentMd = ref();
const {
  float: citationFloat,
  rebind: rebindCitations,
  cancelClose: cancelCitationClose,
  scheduleClose: scheduleCitationClose,
} = useChatCitationPopover(parentMd, {
  getKnowledgeReferences: () => props.session?.knowledge_references,
  sessionId: () => props.sessionId,
});
const reviewUrl = ref("");
const reviewImg = ref(false);
const isImgLoading = ref(false);
const props = defineProps({
  // 必填项
  content: {
    type: String,
    required: false,
  },
  session: {
    type: Object,
    required: false,
  },
  userQuery: {
    type: String,
    required: false,
    default: "",
  },
  isFirstEnter: {
    type: Boolean,
    required: false,
  },
  sessionId: {
    type: String,
    default: "",
  },
  followUpLoading: {
    type: Boolean,
    default: false,
  },
});

const showRequestInfo = computed(() => !!(props.session?.request_id || props.session?.id));

const preview = (url) => {
  nextTick(() => {
    reviewUrl.value = url;
    reviewImg.value = true;
  });
};

const closePreImg = () => {
  reviewImg.value = false;
  reviewUrl.value = "";
};

const markdownRenderer = createChatMarkdownRenderer({
  codeRenderer: createMermaidCodeRenderer("mermaid-botmsg"),
  imageRenderer: ({ href, title, text }) => createSafeImage(href, text || "", title || ""),
  invalidImageHtml: () => `<p>${t("error.invalidImageLink")}</p>`,
  isValidImageUrl: isValidImageURL,
});

// 计算属性：将 Markdown 文本转换为 tokens
const mentionedItems = computed(() => {
  return props.session?.mentioned_items || [];
});

// Smooth the streamed answer into a steady typewriter cadence.
// Copy/toolbar still read the full content; only display is paced.
const answerText = computed(() => {
  const text = props.content || props.session?.content || "";
  return typeof text === "string" ? text : "";
});
const { displayed: typedAnswer } = useTypewriter(
  () => answerText.value,
  () => Boolean(props.session?.is_completed),
);

// The backend completion event can arrive while the local typewriter still has
// buffered text to reveal. Treat the answer as visually complete only after
// the displayed text has caught up, so actions never appear beside a moving answer.
const answerFullyRendered = computed(
  () => Boolean(props.session?.is_completed) && typedAnswer.value.length >= answerText.value.length,
);

watch(
  answerFullyRendered,
  (ready) => {
    emit("render-complete-change", ready);
  },
  { immediate: true },
);

// 单次渲染整个 Markdown 内容（替代 token-by-token，修复 KaTeX 公式在 streaming 时闪烁消失的问题）
const renderedHTML = computed(() => {
  const text = typedAnswer.value;
  if (!text || typeof text !== "string") return "";
  return renderChatMarkdown(text, {
    renderer: markdownRenderer,
    escapeMarkdown: safeMarkdownToHTML,
    sanitizeHtml: sanitizeMarkdownHTML,
    streaming: !props.session?.is_completed,
    knowledgeReferences: props.session?.knowledge_references,
  });
});

// 计算属性：判断是否有实际内容（非空且不只是空白）
const hasActualContent = computed(() => {
  const text = props.content || props.session?.content || "";
  return text && text.trim().length > 0;
});

// 获取实际内容
const getActualContent = () => {
  return (props.content || props.session?.content || "").trim();
};

// 复制回答内容
const handleCopyAnswer = async () => {
  const content = getActualContent();
  if (!content) {
    MessagePlugin.warning(t("chat.emptyContentWarning"));
    return;
  }

  await copyWithToast(content, "chat.copySuccess", "chat.copyFailed");
};

// 添加到知识库
const handleAddToKnowledge = () => {
  const content = getActualContent();
  if (!content) {
    MessagePlugin.warning(t("chat.emptyContentWarning"));
    return;
  }

  const question = (props.userQuery || "").trim();
  const manualContent = buildManualMarkdown(question, content);
  const manualTitle = formatManualTitle(question);
  uiStore.openManualEditor({
    mode: "create",
    title: manualTitle,
    content: manualContent,
    status: "draft",
  });

  MessagePlugin.info(t("chat.editorOpened"));
};

// 处理 markdown-content 中图片的点击事件
const handleMarkdownImageClick = (e) => {
  const target = e.target;
  if (target && target.tagName === "IMG") {
    const src = target.getAttribute("src");
    if (src) {
      e.preventDefault();
      e.stopPropagation();
      preview(src);
    }
  }
};

watch(renderedHTML, () => {
  nextTick(() => {
    rebindCitations();
  });
});

// 渲染 Mermaid 图表的函数
onUpdated(() => {
  nextTick(async () => {
    await hydrateProtectedFileImages(parentMd.value);
    refreshMarkdownEnhancements(parentMd.value);
    if (props.session?.is_completed) {
      await renderMermaidInContainer(parentMd.value);
    }
  });
});

onMounted(async () => {
  // 为 markdown-content 中的图片添加点击事件
  nextTick(async () => {
    if (parentMd.value) {
      parentMd.value.addEventListener("click", handleMarkdownImageClick, true);
    }
    rebindCitations();
    await hydrateProtectedFileImages(parentMd.value);
    await enhanceMarkdownContainer(parentMd.value);
  });
});

onBeforeUnmount(() => {
  if (parentMd.value) {
    parentMd.value.removeEventListener("click", handleMarkdownImageClick, true);
  }
});
</script>
