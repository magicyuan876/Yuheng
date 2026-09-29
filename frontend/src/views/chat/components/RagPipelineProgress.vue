<template>
  <div v-if="visible" ref="rootElement" class="rag-pipeline-progress">
    <!-- Announcements need a region that outlives each wait row, otherwise screen
         readers miss a live region that appears together with its own text. -->
    <div class="sr-only" role="status" aria-live="polite">{{ liveStatusText }}</div>
    <div v-if="showPrePipelineWait" class="tree-children relative mt-0 ml-2.5 pl-0">
      <div class="tree-child tree-child-last streaming-loading-node relative mb-0 pl-[42px]">
        <div class="tree-branch hidden" />
        <div class="tree-child-content">
          <div class="tool-event">
            <div class="action-card action-pending relative">
              <div class="action-header no-results flex min-h-6 cursor-default items-center py-0 select-none">
                <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                  <LightbulbIcon
                    class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                  />
                  <span
                    class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                  >
                    {{ t("chat.preparingAnswer") }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="!showCollapsedRoot" class="tree-children relative mt-0 ml-2.5 pl-0">
      <div
        v-for="(step, index) in steps"
        :key="step.id"
        class="tree-child relative pl-[42px]"
        :class="
          !showDoneRow && !showWaitStep && !showThinkingStep && index === steps.length - 1
            ? 'tree-child-last mb-0'
            : 'mb-[18px]'
        "
      >
        <div class="tree-branch hidden" />
        <div class="tree-child-content">
          <div class="tool-event">
            <div
              class="action-card group relative"
              :class="{ 'has-reference-trigger cursor-pointer': step.canOpenReferences }"
              :role="step.canOpenReferences ? 'button' : undefined"
              :tabindex="step.canOpenReferences ? 0 : undefined"
              @click="handleStepClick(step)"
              @keydown.enter="handleStepClick(step)"
              @keydown.space.prevent="handleStepClick(step)"
            >
              <div
                class="action-header flex min-h-6 items-center py-0 select-none"
                :class="step.canOpenReferences ? 'cursor-pointer' : 'cursor-default'"
              >
                <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                  <component
                    :is="agentToolIcons[step.iconName] ?? ClipboardPasteIcon"
                    class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                  />
                  <span
                    class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                    :class="{ 'is-running': step.pending, 'group-hover:text-foreground': step.canOpenReferences }"
                  >
                    {{ step.title }}
                  </span>
                </div>
              </div>
              <div v-if="step.summaryHtml" class="search-results-summary-fixed pt-0.5 pr-0 pb-0 pl-0">
                <div
                  class="results-summary-text text-muted-foreground text-[length:var(--agent-step-summary-size)] leading-[1.5] font-normal"
                  :class="{ 'group-hover:text-foreground': step.canOpenReferences }"
                  v-html="step.summaryHtml"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="showWaitStep"
        class="tree-child tree-child-last streaming-loading-node rag-model-wait-step relative mb-0 pl-[42px]"
      >
        <div class="tree-branch hidden" />
        <div class="tree-child-content">
          <div class="tool-event">
            <div class="action-card relative" :class="{ 'action-pending': !waitStepStalled }">
              <div class="action-header no-results flex min-h-6 cursor-default items-center py-0 select-none">
                <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                  <LightbulbIcon
                    class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                  />
                  <span
                    class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                  >
                    {{ waitStepText }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="showThinkingStep"
        class="tree-child rag-thinking-step relative pl-[42px]"
        :class="!showDoneRow ? 'tree-child-last mb-0' : 'mb-[18px]'"
      >
        <div class="tree-branch hidden" />
        <div class="tree-child-content">
          <div class="tool-event">
            <div class="action-card relative" :class="{ 'action-pending': thinkingPending }">
              <div
                class="action-header flex min-h-6 items-center py-0 select-none"
                :class="thinkingContent ? 'cursor-pointer' : 'cursor-default'"
                @click="toggleThinking"
              >
                <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                  <LightbulbIcon
                    class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                  />
                  <span
                    class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                  >
                    {{ t("agent.think") }}
                  </span>
                </div>
              </div>
              <div
                v-if="thinkingContent && thinkingExpanded"
                class="thinking-detail-content text-placeholder mt-1 max-h-[200px] overflow-y-auto p-0 text-[length:var(--agent-step-summary-size)] leading-[1.55] font-normal break-words whitespace-pre-wrap"
              >
                {{ thinkingContent }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="showDoneRow" class="tree-child agent-step-done tree-child-last relative mb-0 pl-[42px]">
        <div class="tree-branch hidden" />
        <div class="tree-child-content">
          <div class="tool-event">
            <div class="action-card relative">
              <div class="action-header no-results flex min-h-6 cursor-default items-center py-0 select-none">
                <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                  <CircleCheckIcon
                    class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                  />
                  <span
                    class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                  >
                    {{ t("common.finish") }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="tree-container">
      <div class="tool-event">
        <div class="action-card tree-root relative mb-0">
          <div class="tree-root-toolbar flex w-full min-w-0 items-center justify-start max-[640px]:gap-2">
            <button
              type="button"
              data-slot="rag-tree-root-expand"
              class="tree-root-expand text-muted-foreground hover:text-foreground m-0 inline-flex max-w-full min-w-0 flex-[0_1_auto] cursor-pointer items-center gap-1.5 rounded border-0 bg-transparent p-0 text-[14px] leading-[22px] whitespace-nowrap"
              :aria-expanded="showExpandedTimeline"
              :aria-label="collapsedStatusText"
              @click="toggleExpanded"
            >
              <span class="tree-root-status min-w-0 flex-[0_1_auto] whitespace-nowrap">{{ collapsedStatusText }}</span>
              <span
                v-if="referenceSummaryText"
                class="tree-root-reference inline-flex min-w-0 flex-[0_1_auto] items-center gap-1.5 whitespace-nowrap"
              >
                {{ referenceSummaryText }}
              </span>
              <component
                :is="showExpandedTimeline ? ChevronDownIcon : ChevronRightIcon"
                class="tree-root-expand__icon h-3.5 w-3.5 shrink-0 text-current"
              />
            </button>
          </div>
        </div>
      </div>

      <div v-if="showExpandedTimeline" class="tree-children tree-children-expanded relative mt-3.5 ml-2.5 pl-0">
        <div
          v-for="(step, index) in steps"
          :key="step.id"
          class="tree-child relative pl-[42px]"
          :class="
            index === steps.length - 1 && !showDoneRow && !showThinkingStep ? 'tree-child-last mb-0' : 'mb-[18px]'
          "
        >
          <div class="tree-branch hidden" />
          <div class="tree-child-content">
            <div class="tool-event">
              <div
                class="action-card group relative"
                :class="{ 'has-reference-trigger cursor-pointer': step.canOpenReferences }"
                :role="step.canOpenReferences ? 'button' : undefined"
                :tabindex="step.canOpenReferences ? 0 : undefined"
                @click="handleStepClick(step)"
                @keydown.enter="handleStepClick(step)"
                @keydown.space.prevent="handleStepClick(step)"
              >
                <div
                  class="action-header flex min-h-6 items-center py-0 select-none"
                  :class="step.canOpenReferences ? 'cursor-pointer' : 'cursor-default'"
                >
                  <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                    <component
                      :is="agentToolIcons[step.iconName] ?? ClipboardPasteIcon"
                      class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                    />
                    <span
                      class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                      :class="{ 'is-running': step.pending, 'group-hover:text-foreground': step.canOpenReferences }"
                    >
                      {{ step.title }}
                    </span>
                  </div>
                </div>
                <div v-if="step.summaryHtml" class="search-results-summary-fixed pt-0.5 pr-0 pb-0 pl-0">
                  <div
                    class="results-summary-text text-muted-foreground text-[length:var(--agent-step-summary-size)] leading-[1.5] font-normal"
                    :class="{ 'group-hover:text-foreground': step.canOpenReferences }"
                    v-html="step.summaryHtml"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="showThinkingStep"
          class="tree-child rag-thinking-step relative pl-[42px]"
          :class="!showDoneRow ? 'tree-child-last mb-0' : 'mb-[18px]'"
        >
          <div class="tree-branch hidden" />
          <div class="tree-child-content">
            <div class="tool-event">
              <div class="action-card relative" :class="{ 'action-pending': thinkingPending }">
                <div
                  class="action-header flex min-h-6 items-center py-0 select-none"
                  :class="thinkingContent ? 'cursor-pointer' : 'cursor-default'"
                  @click="toggleThinking"
                >
                  <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                    <LightbulbIcon
                      class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                    />
                    <span
                      class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                    >
                      {{ t("agent.think") }}
                    </span>
                  </div>
                </div>
                <div
                  v-if="thinkingContent && thinkingExpanded"
                  class="thinking-detail-content text-placeholder mt-1 max-h-[200px] overflow-y-auto p-0 text-[length:var(--agent-step-summary-size)] leading-[1.55] font-normal break-words whitespace-pre-wrap"
                >
                  {{ thinkingContent }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="showDoneRow" class="tree-child agent-step-done tree-child-last relative mb-0 pl-[42px]">
          <div class="tree-branch hidden" />
          <div class="tree-child-content">
            <div class="tool-event">
              <div class="action-card relative">
                <div class="action-header no-results flex min-h-6 cursor-default items-center py-0 select-none">
                  <div class="action-title relative flex min-w-0 flex-[0_1_auto] items-center gap-3">
                    <CircleCheckIcon
                      class="action-title-icon text-placeholder absolute top-[3px] -left-[42px] h-[18px] w-[18px] shrink-0"
                    />
                    <span
                      class="action-name text-muted-foreground max-w-[min(820px,100%)] text-[length:var(--agent-step-text-size)] leading-[1.55] font-normal break-words"
                    >
                      {{ t("common.finish") }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";
import "@/components/css/chat-timeline-loading.css";
import {
  BrainIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleCheckIcon,
  ClipboardPasteIcon,
  CodeIcon,
  DatabaseIcon,
  FileSearchIcon,
  GlobeIcon,
  LightbulbIcon,
  ListTodoIcon,
  PaperclipIcon,
  SearchIcon,
  SquareTerminalIcon,
} from "@lucide/vue";
import { getAgentToolIconName } from "@/utils/agent-tool-icons";
import {
  getKnowledgeSearchSummaryHtml,
  getRagPipelineStepTitle,
  getRetrievalSearchSource,
} from "@/utils/agent-tool-display";
import { getAttachmentParsingSummaryHtml } from "@/utils/attachmentParsingDisplay";
import { RAG_RETRIEVAL_TOOL_NAMES, RAG_TIMELINE_TOOL_NAMES } from "@/utils/rag-pipeline-history";
import { useChatReferencesDrawer } from "@/composables/useChatReferencesDrawer";
import { buildReferenceSections } from "@/utils/referenceSources";
import { createRagWaitController, getRagPipelineWaitKind, type RagWaitView } from "@/utils/rag-pipeline-state";

/**
 * getAgentToolIconName still returns the TDesign icon names the agent
 * timeline was built with; this maps each one onto its lucide component.
 */
const agentToolIcons: Record<string, Component> = {
  "ai-search": BrainIcon,
  internet: GlobeIcon,
  "data-search": DatabaseIcon,
  search: SearchIcon,
  "file-search": FileSearchIcon,
  task: ListTodoIcon,
  attach: PaperclipIcon,
  terminal: SquareTerminalIcon,
  code: CodeIcon,
  "file-paste": ClipboardPasteIcon,
};

const props = defineProps<{
  session?: {
    id?: string | number;
    agentEventStream?: Array<Record<string, unknown>>;
    content?: string;
    knowledge_references?: Array<{ chunk_type?: string; knowledge_id?: string; knowledge_title?: string }>;
    is_completed?: boolean;
  };
  embeddedMode?: boolean;
}>();

const { t } = useI18n();
const referencesDrawer = useChatReferencesDrawer();
const userExpanded = ref(false);
const thinkingExpanded = ref(true);
const rootElement = ref<HTMLElement | null>(null);
const waitView = ref<RagWaitView>({ kind: "none", stalled: false });
const waitController = createRagWaitController((view) => {
  waitView.value = view;
});

const thinkingContent = computed(() => {
  const stream = props.session?.agentEventStream;
  if (!Array.isArray(stream)) return "";
  return stream
    .filter((event) => event.type === "thinking")
    .map((event) => String(event.content || ""))
    .join("");
});

const hasThinking = computed(() => thinkingContent.value.trim().length > 0);

const hasThinkingEvent = computed(() => {
  const stream = props.session?.agentEventStream;
  if (!Array.isArray(stream)) return false;
  return stream.some((event) => event.type === "thinking");
});

const hasAnswer = computed(() => {
  const sessionContent = props.session?.content;
  if (typeof sessionContent === "string" && sessionContent.trim().length > 0) return true;

  const stream = props.session?.agentEventStream;
  if (!stream?.length) return false;
  return stream.some((event) => {
    if (event.type !== "answer" || event.superseded) return false;
    const content = event.content;
    return typeof content === "string" && content.trim().length > 0;
  });
});

const hasReferences = computed(() => (props.session?.knowledge_references?.length ?? 0) > 0);

const referenceSections = computed(() => buildReferenceSections(props.session?.knowledge_references));

const steps = computed(() => {
  const stream = props.session?.agentEventStream;
  if (!stream?.length) return [];

  return stream
    .filter((event) => {
      return (
        event.type === "tool_call" &&
        typeof event.tool_name === "string" &&
        RAG_TIMELINE_TOOL_NAMES.has(event.tool_name)
      );
    })
    .map((event) => {
      const toolName = String(event.tool_name);
      const pending = event.pending === true;
      const toolData =
        event.tool_data && typeof event.tool_data === "object" ? (event.tool_data as Record<string, unknown>) : null;

      const isSearchTool = RAG_RETRIEVAL_TOOL_NAMES.has(toolName);
      const isAttachmentTool = toolName === "attachment_parsing" || toolName === "image_analysis";
      const searchSource = isSearchTool ? getRetrievalSearchSource(event.arguments, toolData) : undefined;
      let summaryHtml = "";
      if (!pending && isSearchTool && toolData) {
        summaryHtml = getKnowledgeSearchSummaryHtml(t, toolData);
      } else if (!pending && isAttachmentTool) {
        summaryHtml = getAttachmentParsingSummaryHtml(t, event);
      }
      const canOpenReferences = !pending && isSearchTool && hasReferences.value;

      return {
        id: String(event.tool_call_id || `${toolName}-${event.timestamp || 0}`),
        toolName,
        pending,
        iconName: getAgentToolIconName(toolName, searchSource),
        title: getRagPipelineStepTitle(t, {
          tool_name: toolName,
          pending,
          success: event.success as boolean | undefined,
          arguments: event.arguments,
          tool_data: toolData,
        }),
        summaryHtml,
        canOpenReferences,
      };
    });
});

const allStepsDone = computed(() => steps.value.length > 0 && steps.value.every((step) => !step.pending));

const hasCompletedRetrievalStep = computed(() =>
  steps.value.some((step) => RAG_RETRIEVAL_TOOL_NAMES.has(step.toolName) && !step.pending),
);

const waitKind = computed(() =>
  getRagPipelineWaitKind({
    isCompleted: Boolean(props.session?.is_completed),
    hasAnswer: hasAnswer.value,
    hasThinkingEvent: hasThinkingEvent.value,
    stepCount: steps.value.length,
    allStepsDone: allStepsDone.value,
    hasCompletedRetrievalStep: hasCompletedRetrievalStep.value,
  }),
);

const showWaitStep = computed(() => waitView.value.kind !== "none");

const waitStepStalled = computed(() => waitView.value.stalled);

const waitStepText = computed(() => {
  if (waitView.value.stalled) return t("chat.modelStillResponding");
  return waitView.value.kind === "model" ? t("chat.connectingModelAndGeneratingAnswer") : t("chat.preparingAnswer");
});

const showCollapsedRoot = computed(
  () => (hasAnswer.value || Boolean(props.session?.is_completed)) && (steps.value.length > 0 || hasThinking.value),
);

const showExpandedTimeline = computed(() => {
  if (!showCollapsedRoot.value) return true;
  return userExpanded.value;
});

const showDoneRow = computed(() => {
  const turnDone = hasAnswer.value || Boolean(props.session?.is_completed);
  if (!turnDone) return false;
  if (steps.value.length === 0 && !hasThinking.value) return false;
  if (steps.value.length > 0 && !allStepsDone.value) return false;
  return true;
});

const showPrePipelineWait = computed(() => {
  if (hasAnswer.value || props.session?.is_completed || steps.value.length > 0 || hasThinking.value) {
    return false;
  }
  return true;
});

// Only show the thinking row once the backend actually streams thinking events.
// Do not pre-empt during the model phase — that flashes "思考" even when thinking is disabled.
const showThinkingStep = computed(() => hasThinkingEvent.value);

const thinkingPending = computed(
  () => showThinkingStep.value && !hasThinking.value && !hasAnswer.value && !props.session?.is_completed,
);

const isThinkingStreaming = computed(
  () => showThinkingStep.value && thinkingExpanded.value && !hasAnswer.value && !props.session?.is_completed,
);

const visible = computed(() => {
  return steps.value.length > 0 || showPrePipelineWait.value || showThinkingStep.value;
});

const liveStatusText = computed(() => {
  if (showPrePipelineWait.value) return t("chat.preparingAnswer");
  if (showWaitStep.value) return waitStepText.value;
  return "";
});

const collapsedStatusText = computed(() => {
  if (steps.value.length === 0) {
    return hasThinking.value ? t("agentStream.toolStatus.thinkingDone") : "";
  }
  return t("agentStream.ragPipeline.searchDone");
});

const referenceSummaryText = computed(() => {
  const docCount = referenceSections.value.find((section) => section.id === "documents")?.items.length ?? 0;
  const webCount = referenceSections.value.find((section) => section.id === "web")?.items.length ?? 0;

  if (docCount > 0 && webCount > 0) {
    return t("chat.referencesDocAndWebCount", { docCount, webCount });
  }
  if (docCount > 0) {
    return t("chat.referencesDocCount", { count: docCount });
  }
  if (webCount > 0) {
    return t("chat.referencesWebCount", { count: webCount });
  }

  return "";
});

function toggleReferencesDrawer() {
  const refs = props.session?.knowledge_references;
  if (!referencesDrawer || !refs?.length) return;
  referencesDrawer.toggle({
    references: refs,
    highlight: null,
    messageId: props.session?.id ? String(props.session.id) : "",
    sourceKey: `rag:${props.session?.id || refs.map((item) => item.knowledge_id || item.knowledge_title).join("|")}`,
  });
}

function handleStepClick(step: { canOpenReferences?: boolean }) {
  if (!step.canOpenReferences) return;
  toggleReferencesDrawer();
}

function toggleExpanded() {
  userExpanded.value = !userExpanded.value;
}

function toggleThinking() {
  if (!showThinkingStep.value || !thinkingContent.value) return;
  thinkingExpanded.value = !thinkingExpanded.value;
}

function scrollThinkingDetailToBottom() {
  nextTick(() => {
    if (!rootElement.value) return;
    rootElement.value.querySelectorAll(".thinking-detail-content").forEach((el) => {
      const htmlEl = el as HTMLElement;
      htmlEl.scrollTop = htmlEl.scrollHeight;
    });
  });
}

watch(thinkingPending, (pending) => {
  if (pending) {
    thinkingExpanded.value = true;
  }
});

watch(waitKind, (kind) => waitController.update(kind), { immediate: true });

watch(hasAnswer, (answered) => {
  if (answered && hasThinking.value) {
    thinkingExpanded.value = false;
  }
});

watch(thinkingContent, () => {
  if (!isThinkingStreaming.value) return;
  scrollThinkingDetailToBottom();
});

watch(thinkingExpanded, (expanded) => {
  if (!expanded || !isThinkingStreaming.value) return;
  scrollThinkingDetailToBottom();
});

onBeforeUnmount(() => {
  waitController.dispose();
});
</script>

<style scoped>
/*
 * Stays CSS: the timeline's size and colour variables (read by the
 * text-[length:var(--agent-step-…)] utilities in the template), the tree's
 * connecting line and the dot before the reference summary (both
 * pseudo-elements), and the <strong> emphasis inside the v-html summaries,
 * which utilities on this template cannot reach. The shimmer on a pending
 * step's title comes from the shared chat-timeline-loading.css, imported in
 * the script.
 */
.rag-pipeline-progress {
  --agent-step-text-size: 14px;
  --agent-step-summary-size: 13px;
  --agent-step-line-color: color-mix(in srgb, var(--td-text-color-primary) 16%, transparent);
  --agent-step-icon-color: var(--td-text-color-placeholder);

  margin: 0;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.tree-container {
  margin: 0 0 8px;
  position: relative;
}

/* The vertical line joining one step's icon to the next; the last step has none. */
.tree-child::before {
  content: "";
  position: absolute;
  left: 9px;
  top: 22px;
  bottom: -18px;
  width: 0;
  border-left: 1px solid var(--agent-step-line-color);
}

.tree-child.tree-child-last::before {
  content: none;
}

/* The small dot separating the collapsed status from the reference count. */
.tree-root-reference::before {
  content: "";
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.65;
  flex-shrink: 0;
}

.results-summary-text :deep(strong) {
  color: var(--td-text-color-secondary);
  font-weight: 500;
}
</style>
