<script setup lang="ts">
/* eslint-disable vue/no-mutating-props -- inherited from upstream: the component writes
   summary state back onto its `details` prop in three places. The right fix is an emit
   the parent applies, and that belongs to this component's rewrite on the new stack,
   not to a patch here. */
// @ts-nocheck -- inherited from upstream and typed loosely throughout (66 explicit anys);
// it gets checked properly when it is rewritten, not repaired a line at a time.
import { marked } from "marked";
import markedKatex from "marked-katex-extension";
import "katex/dist/katex.min.css";

import hljs from "highlight.js";
import "highlight.js/styles/github.css";
import mermaid from "mermaid";
import { onMounted, ref, nextTick, onUnmounted, watch, computed } from "vue";
import {
  downKnowledgeDetails,
  deleteGeneratedQuestion,
  getChunkByIdOnly,
  previewKnowledgeFile,
  updateDocumentChunk,
  listChunkRevisions,
  revertDocumentChunk,
  updateKnowledgeMetadata,
  updateKnowledgeSummary,
  regenerateKnowledgeSummary,
  upsertGeneratedQuestion,
  regenerateGeneratedQuestions,
  getKnowledgeDetails,
  KNOWLEDGE_CHUNK_PAGE_SIZE,
} from "@/api/knowledge-base/index";
import { MessagePlugin } from "tdesign-vue-next";
import {
  sanitizeHTML,
  safeMarkdownToHTML,
  createSafeImage,
  isValidImageURL,
  hydrateProtectedFileImages,
  isValidURL,
} from "@/utils/security";
import { normalizeSpuriousTablePrefixes } from "@/utils/markdownTableNormalize";
import { openMermaidFullscreen } from "@/utils/mermaidViewer";
import { diffWikiLines, type WikiDiffLine } from "@/utils/wikiLineDiff";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import DocumentPreview from "@/components/document-preview.vue";
import {
  ChartLineIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  CircleAlertIcon,
  CircleHelpIcon,
  CirclePlayIcon,
  CircleStopIcon,
  DownloadIcon,
  EllipsisIcon,
  ExternalLinkIcon,
  FileIcon,
  FileQuestionMarkIcon,
  GitBranchIcon,
  HistoryIcon,
  InfoIcon,
  LinkIcon,
  Loader2Icon,
  MessageCircleQuestionMarkIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  Trash2Icon,
  Undo2Icon,
  XIcon,
} from "@lucide/vue";
import {
  PaginationEllipsis,
  PaginationList,
  PaginationListItem,
  PaginationNext,
  PaginationPrev,
  PaginationRoot,
} from "reka-ui";
import { Button, buttonVariants } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
// The `.md-content` Markdown styles. The sheet is global (it was imported
// into this component's scoped Less before), so it is loaded from here.
import "./css/markdown.css";
import KnowledgeProcessingTimeline from "@/components/knowledge-processing-timeline.vue";
import KnowledgeStewardship from "@/components/findings/KnowledgeStewardship.vue";
import { resolveKnowledgeDownloadFileName } from "@/views/knowledge/knowledgeDownloadFileName";

const { t } = useI18n();
const authStore = useAuthStore();

// canDeleteGeneratedQuestion 对应后端 DELETE /chunks/by-id/:id/questions
// 的 OwnedChunkKBOrAdminFromChunkID 守卫——KB 创建者或空间 Admin+
// 才允许删除。父组件 KnowledgeBase.vue 通过 :canEditKB 把 KB 级权限
// 传下来（包含 KB creator / Admin / 组织分享 editor 三种来源），未
// 传时按更严格的 Admin 兜底，避免 Viewer 看到一个会 403 的入口。
const canDeleteGeneratedQuestion = computed(() => {
  if (props.canEditKB === true) return true;
  return authStore.hasRole("admin");
});
const canEditContent = canDeleteGeneratedQuestion;

type MetadataValueType = "text" | "number" | "boolean" | "null";
interface MetadataDraftRow {
  id: number;
  key: string;
  value: string;
  type: MetadataValueType;
}

let metadataRowSeed = 0;
const metadataEditing = ref(false);
const metadataDraft = ref<MetadataDraftRow[]>([]);
const metadataSaving = ref(false);
const summaryRefreshing = ref(false);
const summaryEditing = ref(false);
const summaryDraft = ref("");
const summarySaving = ref(false);

const startSummaryEdit = () => {
  summaryDraft.value = props.details?.description || "";
  summaryEditing.value = true;
};

const cancelSummaryEdit = () => {
  summaryEditing.value = false;
  summaryDraft.value = "";
};

const saveSummary = async () => {
  summarySaving.value = true;
  try {
    const description = summaryDraft.value.trim();
    const result: any = await updateKnowledgeSummary(props.details.id, description);
    applySummaryState(result?.data?.summary_status, result?.data?.description ?? description);
    summaryEditing.value = false;
    MessagePlugin.success(t("common.saveSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  } finally {
    summarySaving.value = false;
  }
};

const metadataTypeOptions = computed(() => [
  { label: t("knowledgeBase.metadataTypeText"), value: "text" },
  { label: t("knowledgeBase.metadataTypeNumber"), value: "number" },
  { label: t("knowledgeBase.metadataTypeBoolean"), value: "boolean" },
  { label: t("knowledgeBase.metadataTypeNull"), value: "null" },
]);

const makeMetadataRow = (key = "", value: unknown = ""): MetadataDraftRow => {
  let type: MetadataValueType = "text";
  if (value === null) type = "null";
  else if (typeof value === "number") type = "number";
  else if (typeof value === "boolean") type = "boolean";
  return {
    id: ++metadataRowSeed,
    key,
    value: value === null ? "" : String(value),
    type,
  };
};

const syncMetadataDraft = () => {
  metadataDraft.value = Object.entries(props.details?.custom_metadata || {}).map(([key, value]) =>
    makeMetadataRow(key, value),
  );
};

const startMetadataEdit = () => {
  syncMetadataDraft();
  if (!metadataDraft.value.length) metadataDraft.value.push(makeMetadataRow());
  metadataEditing.value = true;
};

const addMetadataRow = () => {
  if (metadataDraft.value.length >= 20) return;
  metadataDraft.value.push(makeMetadataRow());
};

const removeMetadataRow = (id: number) => {
  metadataDraft.value = metadataDraft.value.filter((row) => row.id !== id);
};

const parseMetadataValue = (row: MetadataDraftRow): unknown => {
  if (row.type === "null") return null;
  if (row.type === "boolean") return row.value === "true";
  if (row.type === "number") {
    const value = Number(row.value);
    if (!row.value.trim() || !Number.isFinite(value)) {
      throw new Error(t("knowledgeBase.metadataNumberRequired", { key: row.key }));
    }
    return value;
  }
  return row.value;
};

const formatMetadataValue = (value: unknown) => {
  if (value === null) return "null";
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
};

const saveMetadata = async () => {
  try {
    const value: Record<string, unknown> = {};
    for (const row of metadataDraft.value) {
      const key = row.key.trim();
      if (!key) throw new Error(t("knowledgeBase.metadataKeyRequired"));
      if (Object.prototype.hasOwnProperty.call(value, key)) {
        throw new Error(t("knowledgeBase.metadataKeyDuplicate", { key }));
      }
      value[key] = parseMetadataValue(row);
    }
    metadataSaving.value = true;
    const result: any = await updateKnowledgeMetadata(props.details.id, value);
    props.details.custom_metadata = value;
    metadataEditing.value = false;
    if (result?.data) {
      applySummaryState(result.data.summary_status, result.data.description);
    }
    MessagePlugin.success(t("common.saveSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  } finally {
    metadataSaving.value = false;
  }
};

const refreshSummary = async () => {
  summaryRefreshing.value = true;
  try {
    const result: any = await regenerateKnowledgeSummary(props.details.id);
    if (result?.data) {
      applySummaryState(result.data.summary_status, result.data.description);
    }
    const status = result?.data?.summary_status;
    if (status === "pending" || status === "processing") {
      MessagePlugin.success(t("knowledgeBase.summaryRefreshQueued"));
    } else {
      MessagePlugin.success(t("knowledgeBase.summaryRefreshed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.error"));
  } finally {
    summaryRefreshing.value = false;
  }
};

const detailTags = computed(() => {
  const tags = props.details?.tags;
  return Array.isArray(tags) ? tags : [];
});

const headerIconName = computed(() => {
  switch (props.details?.type) {
    case "url":
      return "link";
    case "manual":
      return "edit";
    default:
      return "file";
  }
});

const showSummarySection = computed(
  () =>
    Boolean(props.details?.description) ||
    props.details?.summary_status === "pending" ||
    props.details?.summary_status === "processing" ||
    Boolean(props.details?.id && canEditContent.value),
);

// Mermaid 初始化计数器，用于生成唯一ID
let mermaidRenderCount = 0;

// 初始化 Mermaid
mermaid.initialize({
  startOnLoad: false,
  theme: "default",
  securityLevel: "strict",
  fontFamily: "PingFang SC, Microsoft YaHei, sans-serif",
  flowchart: {
    useMaxWidth: true,
    htmlLabels: true,
    curve: "basis",
  },
  sequence: {
    useMaxWidth: true,
    diagramMarginX: 8,
    diagramMarginY: 8,
    actorMargin: 50,
    width: 150,
    height: 65,
  },
  gantt: {
    useMaxWidth: true,
    leftPadding: 75,
    gridLineStartPadding: 35,
    barHeight: 20,
    barGap: 4,
    topPadding: 50,
  },
});
const props = defineProps([
  "visible",
  "details",
  "knowledgeType",
  "sourceInfo",
  "canEditKB",
  "canDownloadKB",
  "parse_status",
  "kbId",
  "seekMs",
]);
const emit = defineEmits(["closeDoc", "getDoc", "questionDeleted", "summaryStateChange"]);

const applySummaryState = (summaryStatus?: string, description?: string) => {
  if (typeof summaryStatus === "string" && summaryStatus) {
    props.details.summary_status = summaryStatus;
  }
  if (typeof description === "string") {
    props.details.description = description;
  }
  if (props.details?.id) {
    emit("summaryStateChange", {
      id: props.details.id,
      summary_status: props.details.summary_status,
      description: props.details.description,
    });
  }
};

const isSummaryStatusInFlight = (status?: string) => status === "pending" || status === "processing";
const summaryStatusRefreshing = computed(() => isSummaryStatusInFlight(props.details?.summary_status));
const canEditSummary = computed(() => canEditContent.value && !summaryStatusRefreshing.value);
let summaryStatusPollTimer: ReturnType<typeof setTimeout> | null = null;
let summaryStatusPollGeneration = 0;

const stopSummaryStatusPolling = () => {
  summaryStatusPollGeneration++;
  if (summaryStatusPollTimer !== null) {
    clearTimeout(summaryStatusPollTimer);
    summaryStatusPollTimer = null;
  }
};

const scheduleSummaryStatusPoll = () => {
  if (
    summaryStatusPollTimer !== null ||
    !props.visible ||
    !props.details?.id ||
    !isSummaryStatusInFlight(props.details?.summary_status)
  )
    return;

  const knowledgeID = props.details.id;
  const generation = summaryStatusPollGeneration;
  summaryStatusPollTimer = setTimeout(async () => {
    summaryStatusPollTimer = null;
    if (generation !== summaryStatusPollGeneration || !props.visible || props.details?.id !== knowledgeID) return;
    try {
      const result: any = await getKnowledgeDetails(knowledgeID);
      if (generation !== summaryStatusPollGeneration || props.details?.id !== knowledgeID) return;
      if (result?.success && result.data) {
        applySummaryState(result.data.summary_status, result.data.description);
      }
    } catch {
      // Keep the current status visible and retry while the drawer remains open.
    }
    if (
      generation === summaryStatusPollGeneration &&
      props.visible &&
      props.details?.id === knowledgeID &&
      isSummaryStatusInFlight(props.details?.summary_status)
    ) {
      scheduleSummaryStatusPoll();
    }
  }, 1500);
};

watch(
  () => [props.visible, props.details?.id, props.details?.summary_status],
  ([visible, knowledgeID, summaryStatus]) => {
    if (visible && knowledgeID && isSummaryStatusInFlight(summaryStatus as string)) {
      scheduleSummaryStatusPoll();
    } else {
      stopSummaryStatusPolling();
    }
  },
  { immediate: true },
);
watch(() => props.details?.id, syncMetadataDraft, { immediate: true });

const hasTimelineSpans = ref(false);
const timelineDrawerVisible = ref(false);
const timelineSummary = ref<{
  totalMs: number;
  status: string;
  stageIndex: number;
  stageTotal: number;
  stageLabel: string;
}>({
  totalMs: 0,
  status: "",
  stageIndex: 0,
  stageTotal: 0,
  stageLabel: "",
});

watch(
  () => props.details?.id,
  () => {
    hasTimelineSpans.value = false;
    timelineDrawerVisible.value = false;
    timelineSummary.value = { totalMs: 0, status: "", stageIndex: 0, stageTotal: 0, stageLabel: "" };
  },
);

function formatTimelineDuration(ms: number): string {
  if (!ms || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
  const mins = Math.floor(ms / 60000);
  const rem = ((ms % 60000) / 1000).toFixed(1);
  return `${mins}m${rem}s`;
}

function openTimeline() {
  timelineDrawerVisible.value = true;
}

function closeTimeline() {
  timelineDrawerVisible.value = false;
}

const TRACE_DRAWER_WIDTH_KEY = "yuheng-trace-drawer-width";
const TRACE_DRAWER_DEFAULT_WIDTH = 820;
const TRACE_DRAWER_MIN_WIDTH = 560;

const timelineDrawerWidth = ref(TRACE_DRAWER_DEFAULT_WIDTH);
const timelineDrawerResizing = ref(false);

let traceResizeStartX = 0;
let traceResizeStartWidth = 0;

function traceDrawerMaxWidth() {
  return Math.min(1400, Math.max(TRACE_DRAWER_MIN_WIDTH, Math.floor(window.innerWidth * 0.92)));
}

function clampTraceDrawerWidth(width: number) {
  return Math.max(TRACE_DRAWER_MIN_WIDTH, Math.min(traceDrawerMaxWidth(), width));
}

function loadTraceDrawerWidth() {
  try {
    const raw = localStorage.getItem(TRACE_DRAWER_WIDTH_KEY);
    const parsed = raw ? parseInt(raw, 10) : NaN;
    if (!Number.isNaN(parsed)) {
      timelineDrawerWidth.value = clampTraceDrawerWidth(parsed);
    }
  } catch {
    /* ignore quota / private mode */
  }
}

function onTraceDrawerResizeStart(e: MouseEvent) {
  timelineDrawerResizing.value = true;
  traceResizeStartX = e.clientX;
  traceResizeStartWidth = timelineDrawerWidth.value;
  document.addEventListener("mousemove", onTraceDrawerResizeMove);
  document.addEventListener("mouseup", onTraceDrawerResizeEnd);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function onTraceDrawerResizeMove(e: MouseEvent) {
  const delta = traceResizeStartX - e.clientX;
  timelineDrawerWidth.value = clampTraceDrawerWidth(traceResizeStartWidth + delta);
}

function onTraceDrawerResizeEnd() {
  document.removeEventListener("mousemove", onTraceDrawerResizeMove);
  document.removeEventListener("mouseup", onTraceDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  timelineDrawerResizing.value = false;
  try {
    localStorage.setItem(TRACE_DRAWER_WIDTH_KEY, String(timelineDrawerWidth.value));
  } catch {
    /* ignore */
  }
}

function onTraceDrawerWindowResize() {
  timelineDrawerWidth.value = clampTraceDrawerWidth(timelineDrawerWidth.value);
  mainDrawerWidth.value = clampMainDrawerWidth(mainDrawerWidth.value);
}

function cleanupTraceDrawerResize() {
  document.removeEventListener("mousemove", onTraceDrawerResizeMove);
  document.removeEventListener("mouseup", onTraceDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  timelineDrawerResizing.value = false;
}

// ============== 主抽屉（文档详情）宽度可调 ==============
const MAIN_DRAWER_WIDTH_KEY = "yuheng-doc-drawer-width";
const MAIN_DRAWER_DEFAULT_WIDTH = 654;
const MAIN_DRAWER_MIN_WIDTH = 480;

const mainDrawerWidth = ref(MAIN_DRAWER_DEFAULT_WIDTH);
const mainDrawerResizing = ref(false);

let mainResizeStartX = 0;
let mainResizeStartWidth = 0;

function mainDrawerMaxWidth() {
  return Math.min(1600, Math.max(MAIN_DRAWER_MIN_WIDTH, Math.floor(window.innerWidth * 0.95)));
}

function clampMainDrawerWidth(width: number) {
  return Math.max(MAIN_DRAWER_MIN_WIDTH, Math.min(mainDrawerMaxWidth(), width));
}

function loadMainDrawerWidth() {
  try {
    const raw = localStorage.getItem(MAIN_DRAWER_WIDTH_KEY);
    const parsed = raw ? parseInt(raw, 10) : NaN;
    if (!Number.isNaN(parsed)) {
      mainDrawerWidth.value = clampMainDrawerWidth(parsed);
    }
  } catch {
    /* ignore quota / private mode */
  }
}

function onMainDrawerResizeStart(e: MouseEvent) {
  mainDrawerResizing.value = true;
  mainResizeStartX = e.clientX;
  mainResizeStartWidth = mainDrawerWidth.value;
  document.addEventListener("mousemove", onMainDrawerResizeMove);
  document.addEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function onMainDrawerResizeMove(e: MouseEvent) {
  // 抽屉在右侧，向左拖动变宽
  const delta = mainResizeStartX - e.clientX;
  mainDrawerWidth.value = clampMainDrawerWidth(mainResizeStartWidth + delta);
}

function onMainDrawerResizeEnd() {
  document.removeEventListener("mousemove", onMainDrawerResizeMove);
  document.removeEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  mainDrawerResizing.value = false;
  try {
    localStorage.setItem(MAIN_DRAWER_WIDTH_KEY, String(mainDrawerWidth.value));
  } catch {
    /* ignore */
  }
}

function cleanupMainDrawerResize() {
  document.removeEventListener("mousemove", onMainDrawerResizeMove);
  document.removeEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  mainDrawerResizing.value = false;
}

const traceEntryTheme = computed(() => {
  const s = timelineSummary.value.status || "";
  switch (s) {
    case "done":
    case "completed":
      return "success";
    case "failed":
      return "danger";
    case "running":
    case "processing":
    case "pending":
      return "warning";
    default:
      return "default";
  }
});

const traceEntryTitle = computed(() => {
  let tip = t("knowledgeStages.viewTrace");
  if (timelineSummary.value.totalMs > 0) {
    tip += ` · ${formatTimelineDuration(timelineSummary.value.totalMs)}`;
  } else if (timelineSummary.value.stageTotal > 0) {
    tip += ` · ${timelineSummary.value.stageIndex}/${timelineSummary.value.stageTotal}`;
  }
  return tip;
});

// Exposed so the parent's three-dot menu can jump straight into the
// trace drawer for a card without forcing the user to click the
// detail drawer header link manually.
defineExpose({ openTimeline });

marked.use({
  breaks: true, // 启用单行换行转 <br>
  gfm: true, // 启用 GitHub Flavored Markdown
});
marked.use(markedKatex({ throwOnError: false, nonStandard: true }));

const preprocessMathDelimiters = (rawText: string): string => {
  if (!rawText || typeof rawText !== "string") {
    return "";
  }
  return rawText.replace(/\\\[([\s\S]*?)\\\]/g, "$$$$$1$$$$").replace(/\\\(([\s\S]*?)\\\)/g, "$$$1$$");
};
const renderer = new marked.Renderer();
const CHUNK_PAGE_SIZE = KNOWLEDGE_CHUNK_PAGE_SIZE;
const chunkPage = ref(1);
const loadedChunkPage = ref(1);
let pendingChunkPage: number | null = null;
const isChunkPageTransition = computed(
  () => Boolean(props.details?.chunkLoading) && chunkPage.value !== loadedChunkPage.value,
);
const mdContentWrap = ref();
// The drawer is portalled to <body>, so markdown nodes live outside mdContentWrap in the DOM.
// docMarkdownRoot is the drawer's scrolling body, which holds all of them.
const docMarkdownRoot = ref<HTMLElement | null>(null);

const getMarkdownRenderRoot = (): ParentNode | null =>
  docMarkdownRoot.value ?? (mdContentWrap.value as ParentNode | null) ?? null;
const url = ref("");
// 视图模式：chunks / merged / preview
// file 类型默认「预览」，URL / 手动创建 默认「全文」
const viewMode = ref<"chunks" | "merged" | "preview">("merged");

// 合并后的文档内容（在下方通过 computed 定义）

/**
 * 把已合并文本 acc 和下一个 chunk 内容 next 拼接，并去除两者的重叠部分。
 *
 * 不再依赖 start_at / end_at 做位置裁剪，而是用「文本重叠匹配」：在 next 的
 * 开头窗口里找 acc 后缀首次出现的位置，从该位置之后接上。这样能同时兼容：
 *  1. chunker 给拆分表格补写的表头（零宽 start/end，位置上不可见）——表头出现
 *     在重叠行之前，会被自然跳过；
 *  2. HTML 实体编码（&#34; 等）导致的 content 长度与原文区间不一致——比对的是
 *     文本本身，不受长度偏差影响。
 *
 * positionOverlap <= 0 时两段位置上严格相邻或不相交，无可去重重叠；此时进入
 * 文本匹配会因 headSlack 下限 320 在 next 开头窗口内误命中 acc 后缀的真实
 * 内容重复（如同一句话在文档多次出现），把 next 开头整段误判为补写表头删掉，
 * 造成不可逆的内容丢失。直接拼接，补写表头重复交给调用方后处理。
 *
 * @param positionOverlap 由 start/end 估算的重叠量，仅用于界定搜索窗口大小。
 */
const appendChunkContent = (acc: string, next: string, positionOverlap: number): string => {
  if (!acc) return next;
  if (!next) return acc;
  if (positionOverlap <= 0) return acc + next;

  const MIN_OVERLAP = 12; // 过短的后缀容易误匹配（如分隔行），忽略
  const span = Math.max(positionOverlap, 0);
  // 搜索的后缀最大长度；按位置重叠量放大几倍兜底，并设下限
  const maxK = Math.min(acc.length, next.length, Math.max(span * 3, 400));
  // 重叠行之前最多允许多少前缀（补写的表头）被跳过
  const headSlack = Math.max(span * 2, 320);

  for (let k = maxK; k >= MIN_OVERLAP; k--) {
    const suffix = acc.slice(acc.length - k);
    const pos = next.indexOf(suffix);
    if (pos !== -1 && pos <= headSlack) {
      return acc + next.slice(pos + k);
    }
  }
  return acc + next;
};

/**
 * 合并分块内容，还原完整文档。chunks 按 start_at 排序后逐段用文本重叠匹配拼接。
 */
const mergeChunks = (chunks: any[]): string => {
  if (!chunks || chunks.length === 0) return "";

  // 按 start_at 排序
  const sortedChunks = [...chunks].sort((a, b) => {
    const startA = a.start_at ?? a.chunk_index ?? 0;
    const startB = b.start_at ?? b.chunk_index ?? 0;
    return startA - startB;
  });

  let merged = sortedChunks[0].content || "";
  let mergedEnd = sortedChunks[0].end_at ?? 0;

  for (let i = 1; i < sortedChunks.length; i++) {
    const currentChunk = sortedChunks[i];
    const currentStartAt = currentChunk.start_at ?? 0;
    const currentEndAt = currentChunk.end_at ?? 0;
    const currentContent = currentChunk.content || "";

    if (!currentContent) continue;

    // 与上一段有明显间隙（位置不相邻），用空行分隔后整段拼接
    if (currentStartAt > mergedEnd && mergedEnd > 0) {
      merged = merged + "\n\n" + currentContent;
    } else {
      const positionOverlap = mergedEnd - currentStartAt;
      merged = appendChunkContent(merged, currentContent, positionOverlap);
    }

    if (currentEndAt > mergedEnd) {
      mergedEnd = currentEndAt;
    }
  }

  return merged;
};

onMounted(() => {
  loadTraceDrawerWidth();
  loadMainDrawerWidth();
  window.addEventListener("resize", onTraceDrawerWindowResize, { passive: true });
});

watch(
  () => props.details?.id,
  () => {
    cancelSummaryEdit();
    chunkPage.value = 1;
    loadedChunkPage.value = 1;
    pendingChunkPage = null;
  },
);
watch(
  () => props.details?.chunkLoading,
  (val) => {
    if (val === false && pendingChunkPage !== null) {
      if (props.details?.chunkLoadError) {
        chunkPage.value = loadedChunkPage.value;
        MessagePlugin.warning(props.details.chunkLoadError);
      } else {
        loadedChunkPage.value = pendingChunkPage;
      }
      pendingChunkPage = null;
    }
  },
);
onUnmounted(() => {
  stopSummaryStatusPolling();
  window.removeEventListener("resize", onTraceDrawerWindowResize);
  cleanupTraceDrawerResize();
  cleanupMainDrawerResize();
  if (audioBlobUrl.value) {
    URL.revokeObjectURL(audioBlobUrl.value);
  }
  if (videoBlobUrl.value) {
    URL.revokeObjectURL(videoBlobUrl.value);
  }
});
const checkImage = (url) => {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => resolve(true);
    img.onerror = () => resolve(false);
    img.src = url;
  });
};
renderer.image = function ({ href, title, text }) {
  if (!isValidImageURL(href)) {
    return `<p>${t("error.invalidImageLink")}</p>`;
  }

  const safeImage = createSafeImage(href, text || "", title || "");
  return `<figure>
                ${safeImage}
                <figcaption style="text-align: left;">${text || ""}</figcaption>
            </figure>`;
};

// 自定义代码块渲染器，只显示语言标签
renderer.code = function ({ text, lang }) {
  // 空值校验：防止 text 为 undefined 或 null
  if (!text || typeof text !== "string") {
    text = "";
  }

  // Mermaid 图表处理
  if (lang === "mermaid") {
    // 生成唯一ID
    const id = `mermaid-${++mermaidRenderCount}`;
    // 返回带有 mermaid 类的 div，后续由 mermaid.run() 处理
    return `<div class="mermaid" id="${id}">${text}</div>`;
  }

  let detectedLang = lang;
  let highlighted = "";
  if (lang && hljs.getLanguage(lang)) {
    try {
      highlighted = hljs.highlight(text, { language: lang }).value;
    } catch (e) {
      highlighted = hljs.highlightAuto(text).value;
      detectedLang = hljs.highlightAuto(text).language || lang;
    }
  } else {
    const auto = hljs.highlightAuto(text);
    highlighted = auto.value;
    detectedLang = auto.language || lang;
  }
  const displayLang = detectedLang || "Code";
  return `
    <div class="code-block-wrapper">
      <div class="code-block-header">
        <span class="code-block-lang">${displayLang}</span>
      </div>
      <pre class="code-block-pre"><code class="hljs language-${detectedLang || ""}">${highlighted}</code></pre>
    </div>
  `;
};
// 监听 chunks 变化，自动更新合并内容（已改为 computed 属性）
const mergedContent = computed(() => {
  const newChunks = props.details?.md;
  if (newChunks && newChunks.length > 0) {
    return mergeChunks(newChunks);
  }
  return "";
});

// 计算处理后的分块数据，避免在模板中频繁调用方法和 JSON.parse
const processedChunks = computed(() => {
  return (props.details?.md || []).map((item: any, index: number) => {
    return {
      original: item,
      processedContent: processMarkdown(item.content),
      questions: getGeneratedQuestions(item),
      meta: getChunkMeta(item),
      videoTime: getChunkVideoTime(item),
      hasParent: hasParentChunk(item),
      chunkClass: getChunkClass(index),
    };
  });
});

const previewSupportedTypes = new Set([
  "pdf",
  "docx",
  "pptx",
  "ppt",
  "xlsx",
  "xls",
  "csv",
  "jpg",
  "jpeg",
  "png",
  "gif",
  "bmp",
  "webp",
  "tiff",
  "svg",
  "txt",
  "md",
  "markdown",
  "json",
  "xml",
  "html",
  "css",
  "js",
  "ts",
  "py",
  "java",
  "go",
  "cpp",
  "c",
  "h",
  "sh",
  "yaml",
  "yml",
  "ini",
  "conf",
  "log",
  "sql",
  "rs",
  "rb",
  "php",
  "swift",
  "kt",
  "scala",
  "r",
  "lua",
  "pl",
  "toml",
  "mp3",
  "wav",
  "m4a",
  "flac",
  "ogg",
]);

const canPreview = (): boolean => {
  if (props.details?.type !== "file") return false;
  const ft = props.details?.file_type?.toLowerCase();
  if (!ft) return false;
  if (audioExtensions.has(ft)) return false; // 音频不走预览tab，播放器已内嵌
  if (videoExtensions.has(ft)) return false; // 视频不走预览tab，播放器已内嵌
  return previewSupportedTypes.has(ft);
};

// 当文档详情加载完成时，file 类型自动切换到「预览」；音/视频类型内嵌播放器
watch(
  () => props.details?.id,
  (newId) => {
    // 清理旧音视频
    if (audioBlobUrl.value) {
      URL.revokeObjectURL(audioBlobUrl.value);
      audioBlobUrl.value = "";
    }
    if (videoBlobUrl.value) {
      URL.revokeObjectURL(videoBlobUrl.value);
      videoBlobUrl.value = "";
    }
    if (!newId) return;
    if (isAudioFile(props.details?.file_type)) {
      viewMode.value = "merged"; // 音频默认全文视图，播放器已内嵌
      loadAudioPreview();
    } else if (isVideoFile(props.details?.file_type)) {
      viewMode.value = "chunks"; // 视频默认分块视图，时间轴 chunk 可点击跳转
      loadVideoPreview();
    } else if (props.details?.type === "file" && canPreview()) {
      viewMode.value = "preview";
    } else {
      viewMode.value = "merged";
    }
  },
);

// 音频文件判断与播放器状态
const audioExtensions = new Set(["mp3", "wav", "m4a", "flac", "ogg"]);
const isAudioFile = (fileType?: string): boolean => {
  if (!fileType) return false;
  return audioExtensions.has(fileType.toLowerCase());
};
const audioBlobUrl = ref("");
const audioLoading = ref(false);

const loadAudioPreview = async () => {
  if (!props.details?.id || audioBlobUrl.value) return;
  audioLoading.value = true;
  try {
    const blob = await previewKnowledgeFile(props.details.id);
    audioBlobUrl.value = URL.createObjectURL(blob);
  } catch (err) {
    console.error("Audio preview load failed:", err);
  } finally {
    audioLoading.value = false;
  }
};

// 视频文件判断与播放器状态（时间轴 chunk 可点击跳转到对应播放位置）
const videoExtensions = new Set(["mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v"]);
const isVideoFile = (fileType?: string): boolean => {
  if (!fileType) return false;
  return videoExtensions.has(fileType.toLowerCase());
};
const videoBlobUrl = ref("");
const videoLoading = ref(false);
const videoPlayerRef = ref<HTMLVideoElement | null>(null);

const loadVideoPreview = async () => {
  if (!props.details?.id || videoBlobUrl.value) return;
  videoLoading.value = true;
  try {
    const blob = await previewKnowledgeFile(props.details.id);
    videoBlobUrl.value = URL.createObjectURL(blob);
  } catch (err) {
    console.error("Video preview load failed:", err);
  } finally {
    videoLoading.value = false;
  }
};

const seekVideoTo = (ms: number) => {
  const player = videoPlayerRef.value;
  if (!player) return;
  player.currentTime = Math.max(0, ms / 1000);
  player.play?.()?.catch?.(() => {});
  player.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
};

// Deep-link seek (?t=<ms> from chat reference timestamp badges): applied once
// the player element has its metadata, then cleared so later opens don't
// replay a stale position.
let pendingDeepLinkSeekMs: number | null = null;
watch(
  () => props.seekMs,
  (ms) => {
    if (typeof ms === "number" && ms >= 0) {
      pendingDeepLinkSeekMs = ms;
      if (videoPlayerRef.value?.readyState) applyPendingDeepLinkSeek();
    }
  },
  { immediate: true },
);

const applyPendingDeepLinkSeek = () => {
  if (pendingDeepLinkSeekMs == null) return;
  seekVideoTo(pendingDeepLinkSeekMs);
  pendingDeepLinkSeekMs = null;
};

// 从 chunk metadata 中提取视频时间信息（转写窗口或关键帧）
const getChunkVideoTime = (item: any): { startMs: number; endMs?: number } | null => {
  try {
    const metadata = getChunkMetadata(item);
    const segment = metadata?.video_segment;
    if (segment && typeof segment.start_ms === "number") {
      return { startMs: segment.start_ms, endMs: typeof segment.end_ms === "number" ? segment.end_ms : undefined };
    }
    const frame = metadata?.video_frame;
    if (frame && typeof frame.timestamp_ms === "number") {
      return { startMs: frame.timestamp_ms };
    }
  } catch {
    // metadata 解析失败时静默降级为无时间标注
  }
  return null;
};

const formatVideoClock = (ms: number): string => {
  const totalSec = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
};
const runMarkdownPostRenderPipeline = async () => {
  await nextTick();
  const renderRoot = getMarkdownRenderRoot();
  if (!renderRoot) {
    return;
  }
  await hydrateProtectedFileImages(renderRoot, props.kbId ? { mode: "knowledgeBase", kbId: props.kbId } : undefined);
  const images = renderRoot?.querySelectorAll?.("img.markdown-image") as NodeListOf<HTMLImageElement> | undefined;
  if (images) {
    images.forEach(async (item) => {
      const isValid = await checkImage(item.src);
      if (!isValid) {
        item.remove();
      }
    });
  }
  // 渲染 Mermaid 图表
  await renderMermaidDiagrams();
};

watch(
  () => props.details.md,
  () => {
    runMarkdownPostRenderPipeline();
  },
  { immediate: true, deep: true, flush: "post" },
);

watch(
  () => viewMode.value,
  (mode) => {
    if ((mode === "chunks" || mode === "merged") && props.visible) {
      runMarkdownPostRenderPipeline();
    }
  },
  { flush: "post" },
);

watch(
  () => props.visible,
  (visible) => {
    if (!visible) {
      cancelSummaryEdit();
    } else if (viewMode.value === "chunks" || viewMode.value === "merged") {
      runMarkdownPostRenderPipeline();
    }
  },
  { flush: "post" },
);

watch(summaryStatusRefreshing, (refreshing) => {
  if (refreshing) {
    cancelSummaryEdit();
  }
});

// 渲染 Mermaid 图表的函数
const renderMermaidDiagrams = async () => {
  try {
    const mermaidElements = getMarkdownRenderRoot()?.querySelectorAll(".mermaid");
    console.log("[Mermaid] Found mermaid elements:", mermaidElements?.length);
    if (mermaidElements && mermaidElements.length > 0) {
      await mermaid.run({
        nodes: mermaidElements,
      });
      console.log("[Mermaid] Rendering complete");
      // 渲染完成后绑定点击事件
      nextTick(() => {
        bindMermaidClickEvents();
      });
    }
  } catch (error) {
    console.error("Mermaid rendering error:", error);
  }
};

// Mermaid 点击处理函数 - 必须在 bindMermaidClickEvents 之前定义
const handleMermaidClick = (e: Event) => {
  e.stopPropagation();
  const target = e.currentTarget as HTMLElement;
  const svg = target.querySelector("svg");
  if (svg) {
    openMermaidFullscreen(svg.outerHTML);
  }
};

// 为 Mermaid 容器绑定点击全屏事件（绑定在 div 上，不是 SVG 上）
const bindMermaidClickEvents = () => {
  const renderRoot = getMarkdownRenderRoot();
  if (!renderRoot) {
    console.log("[Mermaid] markdown render root is null");
    return;
  }
  // 绑定在 .mermaid div 上，而不是 SVG 上
  const mermaidDivs = renderRoot.querySelectorAll(".mermaid");
  console.log("[Mermaid] Found mermaid divs:", mermaidDivs.length);
  mermaidDivs.forEach((div, index) => {
    const divEl = div as HTMLElement;
    divEl.style.cursor = "pointer";
    // 移除旧的事件监听器（避免重复绑定）
    divEl.removeEventListener("click", handleMermaidClick);
    divEl.addEventListener("click", handleMermaidClick);
    console.log(`[Mermaid] Bound click event to div ${index}`);
  });
};

// 安全地处理 Markdown 内容（使用 marked）
const processMarkdown = (markdownText) => {
  if (!markdownText || typeof markdownText !== "string") return "";

  // 去除 Markdown 头部的 YAML Frontmatter（例如 --- title: xxx ---）
  let processedText = markdownText.replace(/^\s*---\r?\n[\s\S]*?\r?\n---\r?\n/, "");

  // 先还原原始文本中的 HTML 实体，让它们作为普通字符参与渲染
  processedText = processedText
    .replace(/&#39;/g, "'")
    .replace(/&#x27;/gi, "'")
    .replace(/&apos;/g, "'")
    .replace(/&#34;/g, '"')
    .replace(/&#x22;/gi, '"')
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&");

  // 处理被 <p> 包裹的表格行，转换为正常的表格行，并在前后补空行
  processedText = processedText.replace(/<p>\s*(\|[\s\S]*?\|)\s*<\/p>/gi, "\n$1\n");

  // MarkItDown 常在表格前插入空行 + 分隔行，渲染会出现多余空行
  processedText = normalizeSpuriousTablePrefixes(processedText);

  // 保留表格单元格中的 <br>，不转成换行，避免打散表格；其他区域原样交给 marked 处理

  // 先预处理数学定界符，再做安全预处理
  const mathSafeText = preprocessMathDelimiters(processedText);
  const safeMarkdown = safeMarkdownToHTML(mathSafeText);

  // 使用标记渲染
  // Do not register this renderer globally. DocumentPreview uses the same
  // `marked` module; registering here made its later Markdown preview reuse
  // this image validator after the user had opened the chunk view.
  let html = marked.parse(safeMarkdown, { renderer }) as string;

  // 还原被转义的 <br>
  html = html.replace(/&lt;br\s*\/?&gt;/gi, "<br>");

  // 最终安全清理
  const result = sanitizeHTML(html);

  return result;
};
const handleClose = () => {
  emit("closeDoc", false);
  // The drawer body is the scroll container; reopen the next document at its top.
  if (docMarkdownRoot.value) docMarkdownRoot.value.scrollTop = 0;
  viewMode.value = "merged";
};

// 获取显示标题
const getDisplayTitle = () => {
  if (!props.details.title) return "";
  if (props.details.type === "file") {
    // 文件类型去掉扩展名
    const lastDotIndex = props.details.title.lastIndexOf(".");
    return lastDotIndex > 0 ? props.details.title.substring(0, lastDotIndex) : props.details.title;
  }
  // URL和手动创建直接返回标题
  return props.details.title;
};

const channelLabelMap: Record<string, string> = {
  web: "knowledgeBase.channelWeb",
  api: "knowledgeBase.channelApi",
  browser_extension: "knowledgeBase.channelBrowserExtension",
  wechat: "knowledgeBase.channelWechat",
  wecom: "knowledgeBase.channelWecom",
  feishu: "knowledgeBase.channelFeishu",
  gitlab: "knowledgeBase.channelGitLab",
  // Drive (云盘) connectors get their own channel so Drive docs show
  // "飞书云盘" / "Lark 云盘", distinct from the wiki connector's "飞书".
  feishu_drive: "knowledgeBase.channelFeishuDrive",
  lark_drive: "knowledgeBase.channelLarkDrive",
  dingtalk: "knowledgeBase.channelDingtalk",
  slack: "knowledgeBase.channelSlack",
  im: "knowledgeBase.channelIm",
  ima: "knowledgeBase.channelIma",
};

const getChannelLabel = (channel: string) => {
  const key = channelLabelMap[channel];
  return key ? t(key) : t("knowledgeBase.channelUnknown");
};

// 获取类型标签
const getTypeLabel = () => {
  switch (props.details.type) {
    case "url":
      return t("knowledgeBase.typeURL");
    case "manual":
      return t("knowledgeBase.typeManual");
    case "file":
      return props.details.file_type ? props.details.file_type.toUpperCase() : t("knowledgeBase.typeFile");
    default:
      return "";
  }
};

// 获取类型主题色
const getTypeTheme = () => {
  switch (props.details.type) {
    case "url":
      return "primary";
    case "manual":
      return "success";
    case "file":
      return "default";
    default:
      return "default";
  }
};

// 获取内容标签
const getContentLabel = () => {
  switch (props.details.type) {
    case "url":
      return t("knowledgeBase.webContent");
    case "manual":
      return t("knowledgeBase.documentContent");
    case "file":
    default:
      return t("knowledgeBase.fileContent");
  }
};

// 获取时间标签
const getTimeLabel = () => {
  switch (props.details.type) {
    case "url":
      return t("knowledgeBase.importTime");
    case "manual":
      return t("knowledgeBase.createTime");
    case "file":
    default:
      return t("knowledgeBase.uploadTime");
  }
};

// 获取Chunk样式类
const getChunkClass = (index: number) => {
  return index % 2 !== 0 ? "chunk-odd" : "chunk-even";
};

// 获取Chunk元数据
const getChunkMeta = (item: any) => {
  if (!item) return "";
  const parts = [];
  if (item.char_count) {
    parts.push(`${item.char_count} ${t("knowledgeBase.characters")}`);
  }
  if (item.token_count) {
    parts.push(`${item.token_count} tokens`);
  }
  return parts.join(" · ");
};

// 生成的问题类型
interface GeneratedQuestion {
  id: string;
  question: string;
  content_revision?: number;
}

// 解析生成的问题
const getGeneratedQuestions = (item: any): GeneratedQuestion[] => {
  if (!item || !item.metadata) return [];
  try {
    const metadata = typeof item.metadata === "string" ? JSON.parse(item.metadata) : item.metadata;
    const questions = metadata.generated_questions || [];
    // 兼容旧格式（字符串数组）和新格式（对象数组）
    return questions.map((q: string | GeneratedQuestion, index: number) => {
      if (typeof q === "string") {
        // 旧格式：字符串，生成临时ID
        return { id: `legacy-${index}`, question: q };
      }
      return q;
    });
  } catch {
    return [];
  }
};

const hasStaleGeneratedQuestions = (item: any) => {
  const questions = getGeneratedQuestions(item);
  if (!questions.length) return false;
  try {
    const metadata = typeof item.metadata === "string" ? JSON.parse(item.metadata || "{}") : item.metadata || {};
    const fallbackRevision = metadata.generated_questions_revision || 0;
    const currentRevision = item.content_revision || 0;
    return questions.some((question) => (question.content_revision ?? fallbackRevision) !== currentRevision);
  } catch {
    return false;
  }
};

const editingChunkId = ref("");
const chunkDraft = ref("");
const savingChunkId = ref("");
const chunkStatusLoading = ref("");

const getChunkMetadata = (item: any): Record<string, any> =>
  typeof item.metadata === "string" ? JSON.parse(item.metadata || "{}") : { ...(item.metadata || {}) };

const setChunkMetadata = (item: any, metadata: Record<string, any>) => {
  item.metadata = metadata;
};

const upsertChunkGeneratedQuestion = (item: any, questionData: GeneratedQuestion) => {
  const metadata = getChunkMetadata(item);
  const questions: GeneratedQuestion[] = [...(metadata.generated_questions || [])];
  const index = questions.findIndex((q) => q.id === questionData.id);
  if (index >= 0) {
    questions[index] = questionData;
  } else {
    questions.push(questionData);
  }
  metadata.generated_questions = questions;
  setChunkMetadata(item, metadata);
};

const notifyChunkMutationOutcome = (item: any, result: any, successMessage?: string) => {
  Object.assign(item, result.data);
  applySummaryState(result.summary_status, result.description);
  if (item.index_status === "failed") {
    MessagePlugin.warning(t("knowledgeBase.chunkSavedIndexFailed"));
    return;
  }
  if (successMessage) {
    MessagePlugin.success(successMessage);
  }
};

const reloadChunksFromStart = () => {
  chunkPage.value = 1;
  pendingChunkPage = 1;
  editingChunkId.value = "";
  chunkDraft.value = "";
  emit("getDoc", 1);
};

const handleChunkEditError = (error: any, fallbackMessage: string) => {
  if (error?.status === 409) {
    MessagePlugin.warning(error?.message || t("knowledgeBase.chunkEditConflict"));
    reloadChunksFromStart();
    return;
  }
  MessagePlugin.error(error?.message || fallbackMessage);
};

const startChunkEdit = (item: any) => {
  if (editingChunkId.value === item.id) {
    editingChunkId.value = "";
    chunkDraft.value = "";
    return;
  }
  editingChunkId.value = item.id;
  chunkDraft.value = item.content || "";
};

const saveChunkEdit = async (item: any) => {
  if (!chunkDraft.value.trim()) {
    MessagePlugin.warning(t("knowledgeBase.chunkContentRequired"));
    return;
  }
  savingChunkId.value = item.id;
  try {
    const result: any = await updateDocumentChunk(props.details.id, item.id, {
      content: chunkDraft.value,
      expected_revision: item.content_revision || 0,
    });
    editingChunkId.value = "";
    notifyChunkMutationOutcome(item, result, t("common.saveSuccess"));
    void refreshChunkHistoryAfterMutation(item);
  } catch (error: any) {
    handleChunkEditError(error, t("common.saveFailed"));
  } finally {
    savingChunkId.value = "";
  }
};

const toggleChunkEnabled = async (item: any, isEnabled: boolean) => {
  chunkStatusLoading.value = item.id;
  try {
    const result: any = await updateDocumentChunk(props.details.id, item.id, {
      is_enabled: isEnabled,
      expected_revision: item.content_revision || 0,
    });
    notifyChunkMutationOutcome(item, result);
    void refreshChunkHistoryAfterMutation(item);
  } catch (error: any) {
    handleChunkEditError(error, t("common.error"));
  } finally {
    chunkStatusLoading.value = "";
  }
};

const retryChunkIndex = async (item: any) => {
  try {
    const result: any = await updateDocumentChunk(props.details.id, item.id, {
      expected_revision: item.content_revision || 0,
    });
    Object.assign(item, result.data);
    if (item.index_status === "failed") throw new Error(t("knowledgeBase.indexFailed"));
    MessagePlugin.success(t("knowledgeBase.indexRetrySuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.error"));
  }
};

type ChunkDiffLine = WikiDiffLine | { type: "skip"; text: string };

const chunkHistoryPopup = ref("");
const chunkHistoryLoading = ref("");
const chunkHistories = ref<Record<string, any[]>>({});
const selectedChunkRevision = ref<Record<string, number | null>>({});
const chunkHistoryRequestSequence = ref<Record<string, number>>({});
const revertingRevision = ref("");

const loadChunkHistory = async (item: any, force = false, silent = false) => {
  if (!force && Object.prototype.hasOwnProperty.call(chunkHistories.value, item.id)) return;

  const requestSequence = (chunkHistoryRequestSequence.value[item.id] || 0) + 1;
  chunkHistoryRequestSequence.value[item.id] = requestSequence;
  chunkHistoryLoading.value = item.id;
  try {
    const result: any = await listChunkRevisions(props.details.id, item.id);
    if (chunkHistoryRequestSequence.value[item.id] === requestSequence) {
      chunkHistories.value[item.id] = result?.data || [];
    }
  } catch (error: any) {
    if (!silent) MessagePlugin.error(error?.message || t("common.error"));
  } finally {
    if (chunkHistoryRequestSequence.value[item.id] === requestSequence) {
      chunkHistoryLoading.value = "";
    }
  }
};

const showChunkHistory = async (item: any) => loadChunkHistory(item);

const refreshChunkHistoryAfterMutation = async (item: any) => {
  const wasLoaded = Object.prototype.hasOwnProperty.call(chunkHistories.value, item.id);
  const shouldReload = wasLoaded || chunkHistoryPopup.value === item.id;
  // Invalidate both the cached list and any older request still in flight.
  chunkHistoryRequestSequence.value[item.id] = (chunkHistoryRequestSequence.value[item.id] || 0) + 1;
  delete chunkHistories.value[item.id];
  selectedChunkRevision.value[item.id] = null;
  if (shouldReload) {
    await loadChunkHistory(item, true, true);
  } else if (chunkHistoryLoading.value === item.id) {
    chunkHistoryLoading.value = "";
  }
};

const setChunkHistoryPopupVisible = (item: any, visible: boolean) => {
  chunkHistoryPopup.value = visible ? item.id : "";
  if (visible) {
    parentContextPopup.value = "";
    questionPopupChunk.value = "";
    showChunkHistory(item);
  }
};

const selectChunkRevision = (item: any, revision: number) => {
  selectedChunkRevision.value[item.id] = selectedChunkRevision.value[item.id] === revision ? null : revision;
};

const getSelectedChunkRevision = (item: any) => {
  const revision = selectedChunkRevision.value[item.id];
  return (chunkHistories.value[item.id] || []).find((entry: any) => entry.revision === revision) || null;
};

const compactChunkDiff = (item: any): ChunkDiffLine[] => {
  const revision = getSelectedChunkRevision(item);
  if (!revision) return [];
  const lines = diffWikiLines(revision.content || "", item.content || "");
  const changed = new Set<number>();
  lines.forEach((line, index) => {
    if (line.type !== "same") {
      for (let i = Math.max(0, index - 2); i <= Math.min(lines.length - 1, index + 2); i++) changed.add(i);
    }
  });
  if (!changed.size) return [];

  const compact: ChunkDiffLine[] = [];
  let lastIndex = -2;
  [...changed]
    .sort((a, b) => a - b)
    .forEach((index) => {
      if (index > lastIndex + 1) compact.push({ type: "skip", text: "…" });
      compact.push(lines[index]);
      lastIndex = index;
    });
  return compact;
};

const diffLinePrefix = (type: ChunkDiffLine["type"]) => {
  if (type === "add") return "+ ";
  if (type === "del") return "- ";
  return type === "same" ? "  " : "";
};

const revisionStatusChanged = (item: any, revisionIndex: number) => {
  const revisions = chunkHistories.value[item.id] || [];
  const newerEnabled = revisionIndex === 0 ? item.is_enabled : revisions[revisionIndex - 1]?.is_enabled;
  return revisions[revisionIndex]?.is_enabled !== newerEnabled;
};

const revertChunk = async (item: any, revision: number) => {
  revertingRevision.value = `${item.id}:${revision}`;
  try {
    const result: any = await revertDocumentChunk(props.details.id, item.id, revision, item.content_revision || 0);
    notifyChunkMutationOutcome(item, result, t("knowledgeBase.chunkReverted"));
    await refreshChunkHistoryAfterMutation(item);
  } catch (error: any) {
    handleChunkEditError(error, t("common.error"));
  } finally {
    revertingRevision.value = "";
  }
};

const questionDrafts = ref<Record<string, string>>({});
const savingQuestionChunk = ref("");
const regeneratingQuestionChunk = ref("");
const questionPopupChunk = ref("");
const questionComposerChunk = ref("");
const editingQuestionKey = ref("");
const questionEditDraft = ref("");
const savingQuestionKey = ref("");

watch(
  () => props.details?.id,
  () => {
    metadataEditing.value = false;
    editingChunkId.value = "";
    chunkHistoryPopup.value = "";
    chunkHistoryLoading.value = "";
    chunkHistories.value = {};
    selectedChunkRevision.value = {};
    chunkHistoryRequestSequence.value = {};
    parentContextPopup.value = "";
    questionPopupChunk.value = "";
    questionComposerChunk.value = "";
    editingQuestionKey.value = "";
  },
);

const setQuestionPopupVisible = (item: any, visible: boolean) => {
  questionPopupChunk.value = visible ? item.id : "";
  if (visible) {
    parentContextPopup.value = "";
    chunkHistoryPopup.value = "";
  } else {
    closeQuestionComposer(item);
    cancelQuestionEdit();
  }
};

const openQuestionComposer = (item: any) => {
  questionPopupChunk.value = item.id;
  questionComposerChunk.value = item.id;
};

const closeQuestionComposer = (item: any) => {
  questionComposerChunk.value = "";
  questionDrafts.value[item.id] = "";
};

const addQuestion = async (item: any) => {
  const question = (questionDrafts.value[item.id] || "").trim();
  if (!question) return;
  savingQuestionChunk.value = item.id;
  try {
    const result: any = await upsertGeneratedQuestion(item.id, question);
    questionDrafts.value[item.id] = "";
    questionComposerChunk.value = "";
    upsertChunkGeneratedQuestion(item, result.data);
    MessagePlugin.success(t("common.saveSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.error"));
  } finally {
    savingQuestionChunk.value = "";
  }
};

const startQuestionEdit = (item: any, question: GeneratedQuestion) => {
  editingQuestionKey.value = `${item.id}:${question.id}`;
  questionEditDraft.value = question.question;
};

const cancelQuestionEdit = () => {
  editingQuestionKey.value = "";
  questionEditDraft.value = "";
};

const saveQuestionEdit = async (item: any, question: GeneratedQuestion) => {
  const value = questionEditDraft.value.trim();
  if (!value) return;
  if (value === question.question) {
    cancelQuestionEdit();
    return;
  }
  savingQuestionKey.value = `${item.id}:${question.id}`;
  try {
    const result: any = await upsertGeneratedQuestion(item.id, value, question.id);
    if (result?.data) {
      upsertChunkGeneratedQuestion(item, result.data);
    }
    cancelQuestionEdit();
    MessagePlugin.success(t("common.saveSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.error"));
  } finally {
    savingQuestionKey.value = "";
  }
};

const regenerateQuestions = async (item: any) => {
  regeneratingQuestionChunk.value = item.id;
  try {
    const result: any = await regenerateGeneratedQuestions(item.id);
    const metadata = typeof item.metadata === "string" ? JSON.parse(item.metadata || "{}") : item.metadata || {};
    metadata.generated_questions = result?.data || [];
    metadata.generated_questions_revision = item.content_revision || 0;
    item.metadata = metadata;
    questionComposerChunk.value = "";
    MessagePlugin.success(t("knowledgeBase.questionsRegenerated"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.error"));
  } finally {
    regeneratingQuestionChunk.value = "";
  }
};

// 删除中的状态
const deletingQuestion = ref<{ chunkIndex: number; questionId: string } | null>(null);

// 删除生成的问题
const handleDeleteQuestion = async (item: any, chunkIndex: number, question: GeneratedQuestion) => {
  if (!item || !item.id) {
    MessagePlugin.error(t("common.error"));
    return;
  }

  // 检查是否是旧格式数据（无法删除）
  if (question.id.startsWith("legacy-")) {
    MessagePlugin.warning(t("knowledgeBase.legacyQuestionCannotDelete"));
    return;
  }

  deletingQuestion.value = { chunkIndex, questionId: question.id };
  try {
    await deleteGeneratedQuestion(item.id, question.id);
    MessagePlugin.success(t("common.deleteSuccess"));

    // 更新本地数据
    const metadata = typeof item.metadata === "string" ? JSON.parse(item.metadata) : item.metadata;
    if (metadata && metadata.generated_questions) {
      const idx = metadata.generated_questions.findIndex((q: GeneratedQuestion) => q.id === question.id);
      if (idx > -1) {
        metadata.generated_questions.splice(idx, 1);
      }
      item.metadata = typeof item.metadata === "string" ? JSON.stringify(metadata) : metadata;
    }

    // 通知父组件刷新数据
    emit("questionDeleted", { chunkId: item.id, questionId: question.id });
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.deleteFailed"));
  } finally {
    deletingQuestion.value = null;
  }
};

// 检查是否正在删除某个问题
const isDeleting = (chunkIndex: number, questionId: string) => {
  return deletingQuestion.value?.chunkIndex === chunkIndex && deletingQuestion.value?.questionId === questionId;
};

// 父 Chunk 上下文通过 Header Popup 展示，避免在每个分块下方占用高度。
const parentContextPopup = ref("");
const parentContextCache = ref<Map<string, string>>(new Map());
const parentContextLoading = ref<Set<number>>(new Set());

const hasParentChunk = (item: any) => !!item?.parent_chunk_id;

const loadParentContext = async (item: any, index: number) => {
  const parentId = item.parent_chunk_id;
  if (!parentContextCache.value.has(parentId)) {
    parentContextLoading.value.add(index);
    parentContextLoading.value = new Set(parentContextLoading.value);
    try {
      const result: any = await getChunkByIdOnly(parentId);
      if (result.success && result.data) {
        parentContextCache.value.set(parentId, result.data.content || "");
        parentContextCache.value = new Map(parentContextCache.value);
      }
    } catch (err) {
      MessagePlugin.error(t("knowledgeBase.parentContextLoadFailed"));
      parentContextPopup.value = "";
      return;
    } finally {
      parentContextLoading.value.delete(index);
      parentContextLoading.value = new Set(parentContextLoading.value);
    }
  }

  await nextTick();
  await runMarkdownPostRenderPipeline();
};

const setParentContextPopupVisible = (item: any, index: number, visible: boolean) => {
  parentContextPopup.value = visible ? item.id : "";
  if (visible) {
    questionPopupChunk.value = "";
    chunkHistoryPopup.value = "";
    loadParentContext(item, index);
  }
};

const getParentContent = (item: any) => {
  return parentContextCache.value.get(item.parent_chunk_id) || "";
};

const summaryExpanded = ref(false);
const summaryRef = ref<HTMLElement>();
const summaryOverflow = ref(false);

const checkSummaryOverflow = () => {
  nextTick(() => {
    const el = summaryRef.value;
    if (!el) {
      summaryOverflow.value = false;
      return;
    }
    summaryOverflow.value = el.scrollHeight > el.clientHeight + 1;
  });
};

watch(
  () => props.details?.description,
  () => {
    summaryExpanded.value = false;
    checkSummaryOverflow();
  },
);
watch(summaryRef, () => checkSummaryOverflow());

const downloadFile = () => {
  downKnowledgeDetails(props.details.id)
    .then((result) => {
      if (result) {
        if (url.value) {
          URL.revokeObjectURL(url.value);
        }
        url.value = URL.createObjectURL(result);
        const link = document.createElement("a");
        link.style.display = "none";
        link.setAttribute("href", url.value);
        link.setAttribute("download", resolveKnowledgeDownloadFileName(props.details));
        document.body.appendChild(link);
        link.click();
        nextTick(() => {
          document.body.removeChild(link);
          URL.revokeObjectURL(url.value);
        });
      }
    })
    .catch((err) => {
      MessagePlugin.error(t("file.downloadFailed"));
    });
};
const handleChunkPageChange = (pageInfo: { current: number }) => {
  if (props.details?.chunkLoading || pageInfo.current === loadedChunkPage.value) return;
  pendingChunkPage = pageInfo.current;
  emit("getDoc", pageInfo.current);
};

// ── Chunk pager ──
// Stands in for t-pagination with show-jumper and show-page-number: a
// total, numbered pages and a "go to page" box. Like the TDesign pager's
// v-model + @change pair, the page ref moves first and the change handler
// runs after it, so handleChunkPageChange sees the same sequence as before.
const chunkPageCount = computed(() => Math.max(1, Math.ceil((props.details?.total || 0) / CHUNK_PAGE_SIZE)));
const chunkJumpValue = ref<string | number>("");

const goToChunkPage = (next: number) => {
  if (props.details?.chunkLoading || next === chunkPage.value) return;
  chunkPage.value = next;
  handleChunkPageChange({ current: next });
};

const commitChunkJump = () => {
  const raw = Number(chunkJumpValue.value);
  chunkJumpValue.value = "";
  if (!Number.isFinite(raw) || raw <= 0) return;
  goToChunkPage(Math.min(chunkPageCount.value, Math.max(1, Math.floor(raw))));
};

// Enter commits the inline editors, as t-input's @enter did. An Enter that
// confirms an IME composition is not a submit.
const isSubmitEnter = (e: KeyboardEvent) => !e.isComposing && e.keyCode !== 229;

// Focuses the element it is put on when it mounts. The native autofocus
// attribute is honoured only once per page load, and these editors mount
// long after that, when the user opens them.
const vFocus = {
  mounted: (el: HTMLElement) => nextTick(() => el.focus()),
};

// ── Template class strings ──
// Kept here rather than repeated on every element: the drawer has a dozen
// icon buttons and six sections that must look the same.

// The square 28px icon button of the section and chunk toolbars. It turns
// brand-coloured on hover and while the popover it opens is showing
// (aria-expanded, which Reka's PopoverTrigger sets). The dark: variants
// override the ghost variant's own dark hover background.
const ICON_BTN =
  "size-7 min-w-7 rounded-[4px] p-0 text-muted-foreground hover:bg-[color:var(--td-brand-color-light)] hover:text-primary dark:hover:bg-[color:var(--td-brand-color-light)] aria-expanded:bg-[color:var(--td-brand-color-light)] aria-expanded:text-primary";
const ICON_BTN_ACTIVE = "bg-[color:var(--td-brand-color-light)] text-primary";
// The destructive flavour: remove a metadata row, delete a question.
const ICON_BTN_DANGER_HOVER =
  "hover:bg-[color:var(--td-error-color-light)] hover:text-destructive dark:hover:bg-[color:var(--td-error-color-light)]";

const SECTION =
  "flex flex-col border-b border-solid border-[color:var(--td-component-stroke)] pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0";
// The section heading, with the short brand-coloured bar in front of it.
const SECTION_TITLE =
  "m-0 flex items-center gap-2 text-[13px] font-semibold text-foreground select-none before:h-[14px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:bg-primary before:content-['']";

// A small light tag, the look of the TDesign small light tag it replaces.
const TAG = "inline-flex h-5 items-center rounded-[3px] px-1.5 text-xs leading-5 whitespace-nowrap";
const TAG_THEME: Record<string, string> = {
  primary: "bg-[color:var(--td-brand-color-light)] text-primary",
  success: "bg-[color:var(--td-success-color-light)] text-success",
  warning: "bg-[color:var(--td-warning-color-light)] text-warning",
  default: "bg-muted text-foreground",
};

// The header's type badge icon, by headerIconName.
const HEADER_ICONS = { link: LinkIcon, edit: PencilIcon, file: FileIcon };

// The trace button takes the colour of the processing status. The old
// t-button asked for the same through its theme, but the scoped
// `.header-action-btn` colour outranked it once TDesign moved into a
// cascade layer, so the status colour had stopped showing.
const TRACE_ENTRY_CLASS: Record<string, string> = {
  success: "text-success hover:text-success",
  danger: "text-destructive hover:text-destructive",
  warning: "text-warning hover:text-warning",
  default: "text-muted-foreground hover:text-foreground",
};

// The value column of a detail row; each use adds its own gap.
const DETAIL_VALUE =
  "inline-flex min-w-0 flex-1 flex-wrap items-center text-[13px] [word-break:break-word] text-foreground";
// The bare "add field" buttons under the metadata display and editor.
const METADATA_LINK_BTN =
  "flex min-h-7 w-fit items-center justify-start gap-[5px] rounded-[4px] bg-transparent px-1 text-xs text-muted-foreground hover:bg-[color:var(--td-brand-color-light)] hover:text-primary";
const PLAYER_LOADING = "flex items-center gap-2 py-1 text-[13px] text-placeholder";
const PAGE_LOADING = "flex min-h-[120px] items-center justify-center gap-2 text-muted-foreground";
const NO_CONTENT = "mt-3 p-4 text-center text-[13px] text-[color:var(--td-text-color-disabled)]";
// `md-content` is the hook css/markdown.css styles the rendered Markdown by.
const MD_CONTENT = "md-content leading-[1.6] [word-break:break-word] text-foreground";

// The chunk toolbar popovers: the panel draws its own box, so the popover
// content is only a frame with no padding of its own.
const POPUP_CONTENT = "w-auto gap-0 overflow-hidden rounded-[6px] p-0";
const POPUP_PANEL = "overflow-hidden rounded-[6px] bg-card";
const POPUP_HEAD =
  "flex min-h-[38px] items-center justify-between border-b border-solid border-[color:var(--td-component-stroke)] py-1 pr-2 pl-3";
const POPUP_TITLE = "flex items-center text-[13px] font-semibold text-foreground";
const POPUP_STATE = "flex min-h-[120px] items-center justify-center gap-2 text-xs text-placeholder";
const HISTORY_STATE = "flex min-h-[100px] items-center justify-center gap-2 text-xs text-placeholder";
// The small confirm popovers that replace t-popconfirm.
const CONFIRM_CONTENT = "w-auto max-w-[280px] gap-3 p-3 text-[13px]";
const CONFIRM_MESSAGE = "m-0 flex items-start gap-2 text-foreground";

const DIFF_LINE_CLASS: Record<ChunkDiffLine["type"], string> = {
  add: "bg-[color:var(--td-success-color-light)] text-[color:var(--td-success-color-active)]",
  del: "bg-[color:var(--td-error-color-light)] text-[color:var(--td-error-color-active)]",
  same: "text-muted-foreground",
  skip: "text-placeholder",
};

// A press on a resize handle is not a press outside the drawer: the handles
// are teleported next to the panels, not into them, and a modal drawer would
// otherwise close the moment the user grabbed its edge.
const onDrawerPointerDownOutside = (event: CustomEvent<{ originalEvent: PointerEvent }>) => {
  const target = event.detail?.originalEvent?.target;
  if (target instanceof Element && target.closest(".doc-drawer-resize-handle, .trace-drawer-resize-handle")) {
    event.preventDefault();
  }
};
</script>
<template>
  <div ref="mdContentWrap">
    <teleport to="body">
      <!--
        The resize handles sit outside the drawer panels, and a modal Reka
        drawer turns pointer events off on everything outside its panel, so
        they opt back in with pointer-events-auto; each drawer also treats a
        press on its handle as inside (onDrawerPointerDownOutside). The main
        handle stands down while the trace drawer is open: it would
        otherwise float over that panel, which the old z-order hid it under.
      -->
      <div
        v-if="visible && !timelineDrawerVisible"
        class="doc-drawer-resize-handle group pointer-events-auto fixed top-0 bottom-0 z-[2001] -ml-1.5 flex w-3 cursor-col-resize items-center justify-center"
        :style="{ right: `${mainDrawerWidth}px` }"
        role="separator"
        aria-orientation="vertical"
        @mousedown.prevent="onMainDrawerResizeStart"
      >
        <div
          class="bg-border group-hover:bg-primary h-12 w-0.5 rounded-[1px] opacity-55 transition-[opacity,background-color] duration-150 group-hover:opacity-100"
          :class="{ 'bg-primary opacity-100': mainDrawerResizing }"
        />
      </div>
    </teleport>
    <Drawer :open="visible" swipe-direction="right" @update:open="(open: boolean) => !open && handleClose()">
      <DrawerContent
        class="max-w-none gap-0 rounded-none border-0 sm:max-w-none"
        :style="{ width: `${mainDrawerWidth}px`, transition: mainDrawerResizing ? 'none' : undefined }"
        @pointer-down-outside="onDrawerPointerDownOutside"
      >
        <header
          class="flex w-full min-w-0 shrink-0 items-center gap-2.5 border-b border-solid border-[color:var(--td-component-stroke)] px-[18px] py-3.5 font-normal"
        >
          <div
            class="text-primary flex size-8 shrink-0 items-center justify-center rounded-[9px] bg-[rgba(7,192,95,0.1)] text-base"
          >
            <component :is="HEADER_ICONS[headerIconName]" class="size-4" />
          </div>
          <div class="min-w-0 flex-auto">
            <DrawerTitle class="text-foreground truncate text-[15px] leading-[1.4] font-semibold">
              {{ getDisplayTitle() }}
            </DrawerTitle>
          </div>
          <div class="flex shrink-0 grow-0 items-center gap-0.5">
            <Button
              v-if="canDownloadKB && (details.type === 'file' || details.type === 'manual')"
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground hover:bg-accent hover:text-foreground dark:hover:bg-accent size-7 min-w-7 shrink-0 rounded-[4px] p-0 transition-colors duration-150"
              :title="$t('common.download') || 'Download'"
              @click="downloadFile()"
            >
              <DownloadIcon class="size-4" />
            </Button>
            <Button
              v-if="details.id && hasTimelineSpans"
              variant="ghost"
              size="icon-sm"
              class="hover:bg-accent dark:hover:bg-accent size-7 min-w-7 shrink-0 rounded-[4px] p-0 transition-colors duration-150"
              :class="TRACE_ENTRY_CLASS[traceEntryTheme]"
              :title="traceEntryTitle"
              @click="openTimeline"
            >
              <ChartLineIcon class="size-4" />
            </Button>
            <!-- The close button t-drawer drew in its header. -->
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground hover:bg-accent hover:text-foreground dark:hover:bg-accent ml-1 size-7 min-w-7 shrink-0 rounded-[4px] p-0"
              :aria-label="$t('common.close')"
              @click="handleClose"
            >
              <XIcon class="size-4" />
            </Button>
          </div>
        </header>

        <!-- Hidden mount: keeps the timeline fetching data so the header
             link's status dot / duration stays live even before the user
             opens the secondary drawer. -->
        <div class="hidden" aria-hidden="true">
          <KnowledgeProcessingTimeline
            v-if="details.id"
            :knowledge-id="details.id"
            :parse-status="details.parse_status"
            :compact="true"
            :grace-poll="false"
            @update:has-spans="hasTimelineSpans = $event"
            @update:summary="timelineSummary = $event"
          />
        </div>

        <!-- 二级抽屉：完整 Langfuse-style waterfall -->
        <teleport to="body">
          <div
            v-if="timelineDrawerVisible"
            class="trace-drawer-resize-handle group pointer-events-auto fixed top-0 bottom-0 z-[2101] -ml-1.5 flex w-3 cursor-col-resize items-center justify-center"
            :style="{ right: `${timelineDrawerWidth}px` }"
            role="separator"
            aria-orientation="vertical"
            :aria-label="$t('knowledgeStages.resizeDrawer')"
            :title="$t('knowledgeStages.resizeDrawer')"
            @mousedown.prevent="onTraceDrawerResizeStart"
          >
            <div
              class="bg-border group-hover:bg-primary h-12 w-0.5 rounded-[1px] opacity-55 transition-[opacity,background-color] duration-150 group-hover:opacity-100"
              :class="{ 'bg-primary opacity-100': timelineDrawerResizing }"
            />
          </div>
        </teleport>
        <Drawer
          :open="timelineDrawerVisible"
          swipe-direction="right"
          @update:open="(open: boolean) => !open && closeTimeline()"
        >
          <DrawerContent
            class="bg-card max-w-none gap-0 rounded-none border-0 p-0 sm:max-w-none"
            :style="{ width: `${timelineDrawerWidth}px`, transition: timelineDrawerResizing ? 'none' : undefined }"
            @pointer-down-outside="onDrawerPointerDownOutside"
          >
            <DrawerTitle class="sr-only">{{ $t("knowledgeStages.viewTrace") }}</DrawerTitle>
            <div class="kp-drawer-shell bg-card relative flex size-full min-w-0 flex-col overflow-hidden">
              <KnowledgeProcessingTimeline
                v-if="details.id && timelineDrawerVisible"
                :knowledge-id="details.id"
                :parse-status="details.parse_status"
                :doc-title="details.title"
                show-close
                @close="closeTimeline"
              />
            </div>
          </DrawerContent>
        </Drawer>

        <div ref="docMarkdownRoot" class="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-[18px] py-4">
          <section v-if="details.id" :class="[SECTION, 'gap-[14px]']">
            <h4 :class="[SECTION_TITLE, 'mb-1']">{{ $t("knowledgeBase.detailSectionMeta") }}</h4>
            <div class="flex flex-col gap-2.5">
              <div v-if="details.time" class="flex items-start gap-3 leading-[1.6]">
                <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{ getTimeLabel() }}</span>
                <span :class="[DETAIL_VALUE, 'gap-1.5']">{{ details.time }}</span>
              </div>
              <div v-if="details.type" class="flex items-start gap-3 leading-[1.6]">
                <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{
                  $t("knowledgeBase.infoCard.type")
                }}</span>
                <span :class="[DETAIL_VALUE, 'gap-1.5']">
                  <span :class="[TAG, TAG_THEME[getTypeTheme()]]">{{ getTypeLabel() }}</span>
                </span>
              </div>
              <div v-if="details.channel && details.channel !== 'web'" class="flex items-start gap-3 leading-[1.6]">
                <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{
                  $t("knowledgeBase.infoCard.source")
                }}</span>
                <span :class="[DETAIL_VALUE, 'gap-1.5']">
                  <span :class="[TAG, TAG_THEME.warning]">{{ getChannelLabel(details.channel) }}</span>
                </span>
              </div>
              <div v-if="detailTags.length > 0" class="flex items-start gap-3 leading-[1.6]">
                <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{ $t("knowledgeBase.tagLabel") }}</span>
                <span :class="[DETAIL_VALUE, 'gap-1']">
                  <span
                    v-for="tag in detailTags"
                    :key="tag.id"
                    class="text-muted-foreground inline-flex h-5 max-w-[140px] items-center rounded-full border border-solid border-[color:var(--td-component-stroke)] bg-transparent px-2 leading-5 whitespace-nowrap"
                  >
                    <span class="inline-block max-w-[100px] truncate align-middle text-[11px]">{{ tag.name }}</span>
                  </span>
                </span>
              </div>
              <KnowledgeStewardship :knowledge-id="details.id" :can-edit="!!canEditKB" />
            </div>
          </section>

          <section v-if="details.id" :class="[SECTION, 'gap-[14px]']">
            <div class="flex items-center justify-between gap-2">
              <h4 :class="[SECTION_TITLE, 'mb-1']">
                <span>{{ $t("knowledgeBase.customMetadata") }}</span>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <InfoIcon class="text-placeholder size-3.5 shrink-0 cursor-help" />
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeBase.metadataCapabilityHint") }}</TooltipContent>
                </Tooltip>
                <span
                  v-if="Object.keys(details.custom_metadata || {}).length"
                  class="text-placeholder text-[11px] font-normal"
                >
                  {{ Object.keys(details.custom_metadata || {}).length }}/20
                </span>
              </h4>
              <Tooltip v-if="canEditContent && !metadataEditing">
                <TooltipTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    :class="ICON_BTN"
                    :aria-label="$t('common.edit')"
                    @click="startMetadataEdit"
                  >
                    <PencilIcon class="size-[15px]" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("common.edit") }}</TooltipContent>
              </Tooltip>
            </div>

            <div v-if="!metadataEditing" class="min-w-0">
              <div
                v-if="Object.keys(details.custom_metadata || {}).length"
                class="flex flex-wrap items-center gap-x-[18px] gap-y-[7px]"
              >
                <div
                  v-for="(value, key) in details.custom_metadata || {}"
                  :key="key"
                  class="inline-flex min-w-0 items-baseline gap-[5px]"
                >
                  <span class="text-placeholder block truncate text-xs after:content-[':']">{{ key }}</span>
                  <span class="text-foreground block truncate text-[13px]">{{ formatMetadataValue(value) }}</span>
                </div>
              </div>
              <button
                v-else-if="canEditContent"
                type="button"
                data-slot="metadata-empty-action"
                :class="METADATA_LINK_BTN"
                @click="startMetadataEdit"
              >
                <PlusIcon class="size-[15px]" />
                <span>{{ $t("knowledgeBase.addMetadataField") }}</span>
              </button>
              <span v-else class="text-placeholder">{{ $t("knowledgeBase.noCustomMetadata") }}</span>
            </div>

            <div v-else class="flex flex-col gap-2">
              <div v-for="row in metadataDraft" :key="row.id" class="flex items-center gap-1.5 max-[720px]:flex-wrap">
                <Input
                  v-model="row.key"
                  class="min-w-[100px] flex-[0_1_30%] max-[720px]:flex-[1_1_calc(50%-50px)]"
                  :placeholder="$t('knowledgeBase.metadataKeyPlaceholder')"
                />
                <Select v-model="row.type">
                  <SelectTrigger class="w-auto flex-[0_0_92px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="option in metadataTypeOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Select v-if="row.type === 'boolean'" v-model="row.value">
                  <SelectTrigger class="w-auto min-w-[110px] flex-1 max-[720px]:flex-[1_1_calc(50%-50px)]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">true</SelectItem>
                    <SelectItem value="false">false</SelectItem>
                  </SelectContent>
                </Select>
                <Input
                  v-else-if="row.type !== 'null'"
                  v-model="row.value"
                  class="min-w-[110px] flex-1 max-[720px]:flex-[1_1_calc(50%-50px)]"
                  :placeholder="$t('knowledgeBase.metadataValuePlaceholder')"
                />
                <div
                  v-else
                  class="border-border text-placeholder h-8 min-w-[110px] flex-1 rounded-[3px] border border-solid bg-[color:var(--td-bg-color-component-disabled)] px-2.5 text-[13px] leading-[30px] max-[720px]:flex-[1_1_calc(50%-50px)]"
                >
                  null
                </div>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="[ICON_BTN, ICON_BTN_DANGER_HOVER]"
                      :aria-label="$t('common.delete')"
                      @click="removeMetadataRow(row.id)"
                    >
                      <Trash2Icon class="size-[15px]" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("common.delete") }}</TooltipContent>
                </Tooltip>
              </div>
              <button
                v-if="metadataDraft.length < 20"
                type="button"
                data-slot="metadata-add-row"
                :class="METADATA_LINK_BTN"
                @click="addMetadataRow"
              >
                <PlusIcon class="size-[15px]" />
                <span>{{ $t("knowledgeBase.addMetadataField") }}</span>
              </button>
              <div class="mt-2 flex items-center justify-end gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  :disabled="metadataSaving"
                  @click="
                    metadataEditing = false;
                    syncMetadataDraft();
                  "
                >
                  {{ $t("common.cancel") }}
                </Button>
                <Button size="sm" :disabled="metadataSaving" @click="saveMetadata">
                  <Loader2Icon v-if="metadataSaving" class="animate-spin" />
                  {{ $t("common.save") }}
                </Button>
              </div>
            </div>
          </section>

          <section v-if="details.type === 'url'" :class="[SECTION, 'gap-[14px]']">
            <h4 :class="[SECTION_TITLE, 'mb-1']">{{ $t("knowledgeBase.urlSource") }}</h4>
            <div class="bg-accent rounded-[4px] px-3 py-2">
              <a
                :href="isValidURL(details.source) ? details.source : 'javascript:void(0)'"
                :target="isValidURL(details.source) ? '_blank' : undefined"
                class="text-primary flex items-center gap-2 no-underline"
              >
                <LinkIcon class="size-3.5 shrink-0" />
                <span class="flex-1 text-[13px] break-all">{{ details.source }}</span>
                <ExternalLinkIcon class="text-primary size-3.5 shrink-0" />
              </a>
            </div>
          </section>

          <section v-if="showSummarySection" :class="[SECTION, 'gap-[14px]']">
            <div class="flex items-center justify-between gap-2">
              <div class="flex min-w-0 items-center gap-2.5">
                <h4 :class="[SECTION_TITLE, 'mb-1']">{{ $t("knowledgeBase.documentSummary") }}</h4>
                <span
                  v-if="details.description && summaryStatusRefreshing"
                  class="text-placeholder flex items-center gap-[5px] text-[11px] whitespace-nowrap"
                >
                  <Loader2Icon class="text-primary size-3.5 animate-spin" />
                  <span>{{ $t("knowledgeBase.generatingSummary") }}</span>
                </span>
              </div>
              <div v-if="canEditContent && !summaryEditing" class="flex items-center gap-1.5">
                <Tooltip v-if="canEditSummary">
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="ICON_BTN"
                      :aria-label="$t('common.edit')"
                      @click="startSummaryEdit"
                    >
                      <PencilIcon class="size-[15px]" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("common.edit") }}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="ICON_BTN"
                      :disabled="summaryRefreshing"
                      :aria-label="$t('knowledgeBase.regenerateSummary')"
                      @click="refreshSummary"
                    >
                      <Loader2Icon v-if="summaryRefreshing" class="size-[15px] animate-spin" />
                      <RefreshCwIcon v-else class="size-[15px]" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeBase.regenerateSummary") }}</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <div v-if="summaryEditing" class="flex flex-col gap-2.5">
              <Textarea
                v-model="summaryDraft"
                class="max-h-[calc(10lh+18px)] min-h-[calc(4lh+18px)]"
                :placeholder="$t('knowledgeBase.noDocumentSummary')"
              />
              <div class="flex items-center justify-end gap-1.5">
                <Button size="sm" variant="outline" :disabled="summarySaving" @click="cancelSummaryEdit">
                  {{ $t("common.cancel") }}
                </Button>
                <Button size="sm" :disabled="summarySaving" @click="saveSummary">
                  <Loader2Icon v-if="summarySaving" class="animate-spin" />
                  {{ $t("common.save") }}
                </Button>
              </div>
            </div>
            <div
              v-else-if="details.description"
              class="border-border bg-card relative rounded-[6px] border border-solid"
              :class="{ 'cursor-pointer': summaryOverflow || summaryExpanded }"
              @click="(summaryOverflow || summaryExpanded) && (summaryExpanded = !summaryExpanded)"
            >
              <div
                ref="summaryRef"
                class="text-foreground p-3 text-[13px] leading-[1.5] [word-break:break-word] whitespace-pre-wrap"
                :class="{ 'max-h-[4.5em] overflow-hidden': !summaryExpanded }"
              >
                {{ details.description }}
              </div>
              <div
                v-if="(summaryOverflow && !summaryExpanded) || summaryExpanded"
                class="pointer-events-none flex justify-center pb-1"
                :class="{
                  'absolute inset-x-0 bottom-0 h-7 items-end rounded-b-[6px] bg-[linear-gradient(transparent,var(--td-bg-color-container)_80%)]':
                    !summaryExpanded,
                }"
              >
                <ChevronUpIcon v-if="summaryExpanded" class="text-placeholder size-3.5" />
                <ChevronDownIcon v-else class="text-placeholder size-3.5" />
              </div>
            </div>
            <div
              v-else
              class="border-border bg-card text-placeholder flex min-h-[42px] items-center gap-2 rounded-[6px] border border-dashed p-3 text-[13px]"
            >
              <template v-if="details.summary_status === 'pending' || details.summary_status === 'processing'">
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("knowledgeBase.generatingSummary") }}</span>
              </template>
              <template v-else>
                <FileQuestionMarkIcon class="size-[18px]" />
                <span>{{ $t("knowledgeBase.noDocumentSummary") }}</span>
                <Button
                  v-if="canEditContent"
                  size="sm"
                  variant="ghost"
                  class="text-foreground"
                  :disabled="summaryRefreshing"
                  @click="refreshSummary"
                >
                  <Loader2Icon v-if="summaryRefreshing" class="size-3.5 animate-spin" />
                  <RefreshCwIcon v-else class="size-3.5" />
                  {{ $t("knowledgeBase.generateSummary") }}
                </Button>
              </template>
            </div>
          </section>

          <section :class="[SECTION, 'gap-3']">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="flex min-w-0 flex-1 items-center gap-2">
                <h4 :class="[SECTION_TITLE, 'mb-0']">{{ getContentLabel() }}</h4>
                <span
                  v-if="details.total > 0"
                  class="bg-accent text-muted-foreground shrink-0 rounded-[4px] px-2 py-0.5 text-xs"
                >
                  {{ $t("knowledgeBase.chunkCount", { count: details.total }) }}
                </span>
              </div>
              <div class="flex shrink-0 gap-1">
                <Button
                  v-if="canPreview()"
                  size="sm"
                  :variant="viewMode === 'preview' ? 'default' : 'outline'"
                  class="h-7 min-w-[60px]"
                  @click="viewMode = 'preview'"
                >
                  {{ $t("preview.tab") }}
                </Button>
                <Button
                  v-if="!canPreview()"
                  size="sm"
                  :variant="viewMode === 'merged' ? 'default' : 'outline'"
                  class="h-7 min-w-[60px]"
                  @click="viewMode = 'merged'"
                >
                  {{ $t("knowledgeBase.viewMerged") }}
                </Button>
                <Button
                  size="sm"
                  :variant="viewMode === 'chunks' ? 'default' : 'outline'"
                  class="h-7 min-w-[60px]"
                  @click="viewMode = 'chunks'"
                >
                  {{ $t("knowledgeBase.viewChunks") }}
                </Button>
              </div>
            </div>

            <!-- 音频播放器（音频文件时固定显示在内容区顶部） -->
            <div
              v-if="isAudioFile(details.file_type)"
              class="border-border bg-accent mb-4 rounded-[6px] border border-solid px-4 py-3"
            >
              <div v-if="audioLoading" :class="PLAYER_LOADING">
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("preview.audioLoading") }}</span>
              </div>
              <audio v-else-if="audioBlobUrl" controls class="h-10 w-full" :src="audioBlobUrl">
                {{ $t("preview.audioNotSupported") }}
              </audio>
            </div>

            <!-- 视频播放器（视频文件时固定显示在内容区顶部，时间轴 chunk 点击跳转） -->
            <!-- Sticky, so the player stays in view while the timeline chunks below it scroll. -->
            <div
              v-if="isVideoFile(details.file_type)"
              class="border-border bg-accent sticky top-0 z-[5] mb-4 rounded-[6px] border border-solid px-4 py-3"
            >
              <div v-if="videoLoading" :class="PLAYER_LOADING">
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("preview.videoLoading") }}</span>
              </div>
              <video
                v-else-if="videoBlobUrl"
                ref="videoPlayerRef"
                controls
                class="max-h-[320px] w-full rounded-[4px] bg-black"
                :src="videoBlobUrl"
                @loadedmetadata="applyPendingDeepLinkSeek"
              >
                {{ $t("preview.videoNotSupported") }}
              </video>
            </div>

            <!-- 合并视图 -->
            <div v-if="viewMode === 'merged'">
              <div v-if="isChunkPageTransition" :class="PAGE_LOADING">
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("common.loading") }}</span>
              </div>
              <template v-else>
                <div v-if="!mergedContent" :class="NO_CONTENT">{{ $t("common.noData") }}</div>
                <div v-else :class="MD_CONTENT" v-html="processMarkdown(mergedContent)"></div>
              </template>
            </div>

            <!-- 分块视图 -->
            <div v-else-if="viewMode === 'chunks'">
              <div v-if="isChunkPageTransition" :class="PAGE_LOADING">
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("common.loading") }}</span>
              </div>
              <template v-else>
                <div v-if="!processedChunks.length" :class="NO_CONTENT">{{ $t("common.noData") }}</div>
                <div v-else class="flex flex-col gap-3">
                  <div
                    v-for="(chunk, index) in processedChunks"
                    :key="chunk.original.id || index"
                    class="border-border rounded-[6px] border border-solid px-3.5 py-3"
                    :class="chunk.original.is_enabled ? 'bg-card' : 'bg-muted'"
                  >
                    <div
                      class="mb-2.5 flex min-h-7 flex-wrap items-center justify-between gap-2 border-b border-solid border-[color:var(--td-component-stroke)] pb-2"
                    >
                      <div class="flex min-w-0 flex-wrap items-center gap-2">
                        <span class="text-muted-foreground text-xs font-semibold"
                          >{{ $t("knowledgeBase.segment") }}
                          {{ (loadedChunkPage - 1) * CHUNK_PAGE_SIZE + index + 1 }}</span
                        >
                        <span
                          v-if="chunk.videoTime"
                          class="text-primary inline-flex cursor-pointer items-center gap-1 rounded-[10px] bg-[color:var(--td-brand-color-light)] px-2 py-px text-xs select-none hover:bg-[color:var(--td-brand-color-focus)]"
                          role="button"
                          :title="$t('knowledgeBase.jumpToVideoTime')"
                          @click="seekVideoTo(chunk.videoTime.startMs)"
                        >
                          <CirclePlayIcon class="size-3.5" />
                          {{ formatVideoClock(chunk.videoTime.startMs)
                          }}<template v-if="chunk.videoTime.endMs != null">
                            - {{ formatVideoClock(chunk.videoTime.endMs) }}</template
                          >
                        </span>
                        <span class="text-placeholder text-[11px]">{{ chunk.meta }}</span>
                      </div>
                      <div class="flex shrink-0 items-center gap-0.5">
                        <Tooltip v-if="chunk.original.index_status === 'failed' && canEditContent">
                          <TooltipTrigger as-child>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              :class="[ICON_BTN, 'text-destructive', ICON_BTN_DANGER_HOVER]"
                              :aria-label="$t('knowledgeBase.retryIndex')"
                              @click="retryChunkIndex(chunk.original)"
                            >
                              <RefreshCwIcon class="size-[15px]" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{{ $t("knowledgeBase.retryIndex") }}</TooltipContent>
                        </Tooltip>
                        <Popover
                          v-if="chunk.hasParent"
                          :open="parentContextPopup === chunk.original.id"
                          @update:open="(open: boolean) => setParentContextPopupVisible(chunk.original, index, open)"
                        >
                          <PopoverTrigger as-child>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              :class="[ICON_BTN, parentContextPopup === chunk.original.id ? ICON_BTN_ACTIVE : '']"
                              :title="$t('knowledgeBase.viewParentContext')"
                            >
                              <GitBranchIcon class="size-[15px]" />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="end" :class="POPUP_CONTENT">
                            <div
                              :class="[
                                POPUP_PANEL,
                                'max-h-[min(560px,calc(100vh-96px))] w-[min(520px,calc(100vw-32px))]',
                              ]"
                              @click.stop
                            >
                              <div :class="POPUP_HEAD">
                                <div :class="[POPUP_TITLE, 'gap-[7px]']">
                                  <GitBranchIcon class="size-[15px]" />
                                  <span>{{ $t("knowledgeBase.viewParentContext") }}</span>
                                </div>
                              </div>
                              <div v-if="parentContextLoading.has(index)" :class="POPUP_STATE">
                                <Loader2Icon class="text-primary size-4 animate-spin" />
                                <span>{{ $t("common.loading") }}</span>
                              </div>
                              <div
                                v-else
                                :class="[
                                  MD_CONTENT,
                                  'max-h-[min(480px,calc(100vh-170px))] overflow-auto px-4 py-3.5 text-[13px]',
                                ]"
                                v-html="processMarkdown(getParentContent(chunk.original))"
                              ></div>
                            </div>
                          </PopoverContent>
                        </Popover>
                        <Popover
                          v-if="chunk.questions.length > 0 || canEditContent"
                          :open="questionPopupChunk === chunk.original.id"
                          @update:open="(open: boolean) => setQuestionPopupVisible(chunk.original, open)"
                        >
                          <PopoverTrigger as-child>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              class="chunk-question-entry"
                              :class="[ICON_BTN, questionPopupChunk === chunk.original.id ? ICON_BTN_ACTIVE : '']"
                              :title="$t('knowledgeBase.generatedQuestions')"
                            >
                              <CircleHelpIcon class="size-[15px]" />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="end" :class="POPUP_CONTENT">
                            <div
                              :class="[
                                POPUP_PANEL,
                                'max-h-[min(560px,calc(100vh-96px))] w-[min(520px,calc(100vw-32px))]',
                              ]"
                              @click.stop
                            >
                              <div :class="POPUP_HEAD">
                                <div :class="[POPUP_TITLE, 'gap-[7px]']">
                                  <CircleHelpIcon class="size-[15px]" />
                                  <span>{{ $t("knowledgeBase.generatedQuestions") }}</span>
                                  <span class="text-placeholder text-[11px] font-normal">{{
                                    chunk.questions.length
                                  }}</span>
                                  <span
                                    v-if="hasStaleGeneratedQuestions(chunk.original)"
                                    class="text-warning text-[11px] font-normal"
                                  >
                                    {{ $t("knowledgeBase.staleGeneratedQuestions") }}
                                  </span>
                                </div>
                                <div v-if="canEditContent" class="flex items-center gap-0.5">
                                  <Tooltip>
                                    <TooltipTrigger as-child>
                                      <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        :class="ICON_BTN"
                                        :aria-label="$t('knowledgeBase.addGeneratedQuestion')"
                                        @click.stop="openQuestionComposer(chunk.original)"
                                      >
                                        <PlusIcon class="size-[15px]" />
                                      </Button>
                                    </TooltipTrigger>
                                    <TooltipContent side="top">{{
                                      $t("knowledgeBase.addGeneratedQuestion")
                                    }}</TooltipContent>
                                  </Tooltip>
                                  <Tooltip>
                                    <TooltipTrigger as-child>
                                      <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        :class="ICON_BTN"
                                        :disabled="regeneratingQuestionChunk === chunk.original.id"
                                        :aria-label="$t('knowledgeBase.regenerateQuestions')"
                                        @click.stop="regenerateQuestions(chunk.original)"
                                      >
                                        <Loader2Icon
                                          v-if="regeneratingQuestionChunk === chunk.original.id"
                                          class="size-[15px] animate-spin"
                                        />
                                        <RefreshCwIcon v-else class="size-[15px]" />
                                      </Button>
                                    </TooltipTrigger>
                                    <TooltipContent side="top">{{
                                      $t("knowledgeBase.regenerateQuestions")
                                    }}</TooltipContent>
                                  </Tooltip>
                                </div>
                              </div>
                              <div class="max-h-[min(480px,calc(100vh-170px))] overflow-y-auto px-2.5 pb-2">
                                <div
                                  v-if="canEditContent && questionComposerChunk === chunk.original.id"
                                  class="flex items-center gap-1.5 border-b border-solid border-[color:var(--td-component-stroke)] px-1 py-2"
                                >
                                  <Input
                                    v-model="questionDrafts[chunk.original.id]"
                                    v-focus
                                    class="flex-1"
                                    :placeholder="$t('knowledgeBase.addGeneratedQuestion')"
                                    @keydown.enter="
                                      (e: KeyboardEvent) => isSubmitEnter(e) && addQuestion(chunk.original)
                                    "
                                  />
                                  <Tooltip>
                                    <TooltipTrigger as-child>
                                      <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        :class="ICON_BTN"
                                        :disabled="savingQuestionChunk === chunk.original.id"
                                        :aria-label="$t('common.cancel')"
                                        @click="closeQuestionComposer(chunk.original)"
                                      >
                                        <XIcon class="size-3.5" />
                                      </Button>
                                    </TooltipTrigger>
                                    <TooltipContent side="top">{{ $t("common.cancel") }}</TooltipContent>
                                  </Tooltip>
                                  <Button
                                    size="sm"
                                    :disabled="
                                      savingQuestionChunk === chunk.original.id ||
                                      !(questionDrafts[chunk.original.id] || '').trim()
                                    "
                                    @click="addQuestion(chunk.original)"
                                  >
                                    <Loader2Icon
                                      v-if="savingQuestionChunk === chunk.original.id"
                                      class="animate-spin"
                                    />
                                    {{ $t("common.add") }}
                                  </Button>
                                </div>
                                <div v-if="chunk.questions.length">
                                  <div
                                    v-for="question in chunk.questions"
                                    :key="question.id"
                                    class="group/question text-foreground hover:bg-accent flex min-h-[34px] items-center gap-2 rounded-[4px] border-b border-solid border-[color:var(--td-component-stroke)] bg-transparent px-1.5 py-[5px] text-[13px] leading-5"
                                  >
                                    <span
                                      class="text-muted-foreground flex h-5 w-[18px] shrink-0 items-center justify-center"
                                    >
                                      <CircleHelpIcon class="size-3.5" />
                                    </span>
                                    <div
                                      v-if="editingQuestionKey === `${chunk.original.id}:${question.id}`"
                                      class="flex flex-1 items-center gap-1.5"
                                    >
                                      <Input
                                        v-model="questionEditDraft"
                                        v-focus
                                        class="flex-1"
                                        @keydown.enter="
                                          (e: KeyboardEvent) =>
                                            isSubmitEnter(e) && saveQuestionEdit(chunk.original, question)
                                        "
                                      />
                                      <Button size="sm" variant="ghost" @click="cancelQuestionEdit">{{
                                        $t("common.cancel")
                                      }}</Button>
                                      <Button
                                        size="sm"
                                        :disabled="savingQuestionKey === `${chunk.original.id}:${question.id}`"
                                        @click="saveQuestionEdit(chunk.original, question)"
                                      >
                                        <Loader2Icon
                                          v-if="savingQuestionKey === `${chunk.original.id}:${question.id}`"
                                          class="animate-spin"
                                        />
                                        {{ $t("common.save") }}
                                      </Button>
                                    </div>
                                    <template v-else>
                                      <span class="flex-1 leading-5 [word-break:break-word]">{{
                                        question.question
                                      }}</span>
                                      <div
                                        class="flex items-center gap-0.5 opacity-0 transition-opacity duration-150 group-hover/question:opacity-100 focus-within:opacity-100"
                                      >
                                        <Tooltip v-if="canEditContent && !question.id.startsWith('legacy-')">
                                          <TooltipTrigger as-child>
                                            <Button
                                              variant="ghost"
                                              size="icon-sm"
                                              :class="ICON_BTN"
                                              :aria-label="$t('common.edit')"
                                              @click.stop="startQuestionEdit(chunk.original, question)"
                                            >
                                              <PencilIcon class="size-3.5" />
                                            </Button>
                                          </TooltipTrigger>
                                          <TooltipContent side="top">{{ $t("common.edit") }}</TooltipContent>
                                        </Tooltip>
                                        <Popover
                                          v-if="canDeleteGeneratedQuestion && !question.id.startsWith('legacy-')"
                                        >
                                          <PopoverTrigger as-child>
                                            <Button
                                              variant="ghost"
                                              size="icon-sm"
                                              :class="[ICON_BTN, 'text-placeholder', ICON_BTN_DANGER_HOVER]"
                                              :disabled="isDeleting(index, question.id)"
                                              :aria-label="$t('common.delete')"
                                            >
                                              <Loader2Icon
                                                v-if="isDeleting(index, question.id)"
                                                class="size-3.5 animate-spin"
                                              />
                                              <Trash2Icon v-else class="size-3.5" />
                                            </Button>
                                          </PopoverTrigger>
                                          <PopoverContent side="top" :class="CONFIRM_CONTENT">
                                            <p :class="CONFIRM_MESSAGE">
                                              <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
                                              <span>{{ $t("knowledgeBase.confirmDeleteQuestion") }}</span>
                                            </p>
                                            <div class="flex justify-end gap-2">
                                              <PopoverClose as-child>
                                                <Button size="xs" variant="outline">{{ $t("common.cancel") }}</Button>
                                              </PopoverClose>
                                              <PopoverClose as-child>
                                                <Button
                                                  size="xs"
                                                  @click="handleDeleteQuestion(chunk.original, index, question)"
                                                >
                                                  {{ $t("common.confirm") }}
                                                </Button>
                                              </PopoverClose>
                                            </div>
                                          </PopoverContent>
                                        </Popover>
                                      </div>
                                    </template>
                                  </div>
                                </div>
                                <div
                                  v-else-if="questionComposerChunk !== chunk.original.id"
                                  class="text-placeholder flex items-center justify-center gap-2 px-2 pt-5 pb-3 text-xs"
                                >
                                  <MessageCircleQuestionMarkIcon class="size-5" />
                                  <span>{{ $t("knowledgeBase.noGeneratedQuestions") }}</span>
                                </div>
                              </div>
                            </div>
                          </PopoverContent>
                        </Popover>
                        <template v-if="canEditContent">
                          <Tooltip>
                            <TooltipTrigger as-child>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                :class="[ICON_BTN, editingChunkId === chunk.original.id ? ICON_BTN_ACTIVE : '']"
                                :aria-label="$t('common.edit')"
                                @click="startChunkEdit(chunk.original)"
                              >
                                <PencilIcon class="size-[15px]" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top">{{ $t("common.edit") }}</TooltipContent>
                          </Tooltip>
                          <Popover
                            :open="chunkHistoryPopup === chunk.original.id"
                            @update:open="(open: boolean) => setChunkHistoryPopupVisible(chunk.original, open)"
                          >
                            <PopoverTrigger as-child>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                :class="[ICON_BTN, chunkHistoryPopup === chunk.original.id ? ICON_BTN_ACTIVE : '']"
                                :title="$t('knowledgeBase.chunkHistory')"
                              >
                                <HistoryIcon class="size-[15px]" />
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent align="end" :class="POPUP_CONTENT">
                              <div
                                :class="[
                                  POPUP_PANEL,
                                  'max-h-[min(620px,calc(100vh-96px))] w-[min(560px,calc(100vw-32px))]',
                                ]"
                                @click.stop
                              >
                                <div
                                  class="flex items-center justify-between gap-4 border-b border-solid border-[color:var(--td-component-stroke)] px-3 pt-2 pb-[7px]"
                                >
                                  <div>
                                    <div :class="[POPUP_TITLE, 'gap-1.5']">
                                      <HistoryIcon class="size-[15px]" />
                                      <span>{{ $t("knowledgeBase.chunkHistory") }}</span>
                                    </div>
                                    <div class="text-placeholder mt-0.5 text-[11px]">
                                      v{{ chunk.original.content_revision || 0 }} ·
                                      {{ $t("knowledgeBase.currentVersion") }} ·
                                      {{
                                        chunk.original.is_enabled
                                          ? $t("knowledgeBase.enabledStatus")
                                          : $t("knowledgeBase.disabledStatus")
                                      }}
                                    </div>
                                  </div>
                                  <div class="text-placeholder m-0 flex shrink-0 items-center gap-3 text-[10px]">
                                    <span class="inline-flex items-center gap-1">
                                      <i class="bg-success size-[7px] rounded-[2px]" />{{
                                        $t("knowledgeBase.diffAddedInCurrent")
                                      }}
                                    </span>
                                    <span class="inline-flex items-center gap-1">
                                      <i class="bg-destructive size-[7px] rounded-[2px]" />{{
                                        $t("knowledgeBase.diffRemovedFromCurrent")
                                      }}
                                    </span>
                                  </div>
                                </div>
                                <div v-if="chunkHistoryLoading === chunk.original.id" :class="HISTORY_STATE">
                                  <Loader2Icon class="text-primary size-4 animate-spin" />
                                  <span>{{ $t("common.loading") }}</span>
                                </div>
                                <div
                                  v-else-if="!(chunkHistories[chunk.original.id] || []).length"
                                  :class="HISTORY_STATE"
                                >
                                  {{ $t("knowledgeBase.noChunkHistory") }}
                                </div>
                                <div v-else class="max-h-[min(520px,calc(100vh-180px))] overflow-y-auto">
                                  <div
                                    v-for="(revision, revisionIndex) in chunkHistories[chunk.original.id]"
                                    :key="revision.id"
                                    class="border-b border-solid border-[color:var(--td-component-stroke)] last:border-b-0"
                                    :class="{
                                      'bg-muted': selectedChunkRevision[chunk.original.id] === revision.revision,
                                    }"
                                  >
                                    <button
                                      type="button"
                                      data-slot="chunk-history-version-row"
                                      class="text-muted-foreground hover:bg-accent flex min-h-10 w-full items-center gap-2 bg-transparent px-3.5 py-2 text-left"
                                      @click="selectChunkRevision(chunk.original, revision.revision)"
                                    >
                                      <span class="text-foreground min-w-8 text-xs font-semibold"
                                        >v{{ revision.revision }}</span
                                      >
                                      <span class="text-placeholder text-[11px]">{{
                                        new Date(revision.edited_at).toLocaleString()
                                      }}</span>
                                      <span
                                        v-if="revisionStatusChanged(chunk.original, revisionIndex)"
                                        class="text-muted-foreground ml-auto inline-flex items-center gap-1 text-[11px]"
                                      >
                                        <CirclePlayIcon v-if="revision.is_enabled" class="size-[13px]" />
                                        <CircleStopIcon v-else class="size-[13px]" />
                                        {{
                                          revision.is_enabled
                                            ? $t("knowledgeBase.enabledStatus")
                                            : $t("knowledgeBase.disabledStatus")
                                        }}
                                      </span>
                                      <!-- The status label, when shown, already pushes the chevron to the end. -->
                                      <component
                                        :is="
                                          selectedChunkRevision[chunk.original.id] === revision.revision
                                            ? ChevronUpIcon
                                            : ChevronDownIcon
                                        "
                                        class="text-placeholder size-3.5"
                                        :class="
                                          revisionStatusChanged(chunk.original, revisionIndex) ? 'ml-0' : 'ml-auto'
                                        "
                                      />
                                    </button>
                                    <div
                                      v-if="selectedChunkRevision[chunk.original.id] === revision.revision"
                                      class="px-3 pb-2.5"
                                    >
                                      <div
                                        class="text-placeholder flex min-h-[30px] items-center justify-between gap-2.5 text-[11px]"
                                      >
                                        <span>{{
                                          $t("knowledgeBase.compareRevisionWithCurrent", {
                                            revision: revision.revision,
                                            current: chunk.original.content_revision || 0,
                                          })
                                        }}</span>
                                        <Popover>
                                          <PopoverTrigger as-child>
                                            <Button
                                              variant="ghost"
                                              size="icon-sm"
                                              :class="ICON_BTN"
                                              :title="$t('knowledgeBase.revertRevision')"
                                              :disabled="
                                                revertingRevision === `${chunk.original.id}:${revision.revision}`
                                              "
                                            >
                                              <Loader2Icon
                                                v-if="revertingRevision === `${chunk.original.id}:${revision.revision}`"
                                                class="size-3.5 animate-spin"
                                              />
                                              <Undo2Icon v-else class="size-3.5" />
                                            </Button>
                                          </PopoverTrigger>
                                          <PopoverContent side="top" :class="CONFIRM_CONTENT">
                                            <p :class="CONFIRM_MESSAGE">
                                              <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
                                              <span>{{
                                                $t("knowledgeBase.revertRevisionConfirm", {
                                                  revision: revision.revision,
                                                })
                                              }}</span>
                                            </p>
                                            <div class="flex justify-end gap-2">
                                              <PopoverClose as-child>
                                                <Button size="xs" variant="outline">{{ $t("common.cancel") }}</Button>
                                              </PopoverClose>
                                              <PopoverClose as-child>
                                                <Button
                                                  size="xs"
                                                  @click="revertChunk(chunk.original, revision.revision)"
                                                >
                                                  {{ $t("common.confirm") }}
                                                </Button>
                                              </PopoverClose>
                                            </div>
                                          </PopoverContent>
                                        </Popover>
                                      </div>
                                      <div
                                        v-if="!compactChunkDiff(chunk.original).length"
                                        class="bg-card text-placeholder rounded-[4px] p-3 text-center text-xs"
                                      >
                                        {{ $t("knowledgeBase.noContentChanges") }}
                                      </div>
                                      <pre
                                        v-else
                                        class="border-border bg-card m-0 max-h-60 overflow-auto rounded-[4px] border border-solid py-2 font-[family-name:var(--app-font-family-mono)] text-[11px] leading-[1.55] [word-break:break-word] whitespace-pre-wrap"
                                      ><span
                                      v-for="(line, lineIndex) in compactChunkDiff(chunk.original)" :key="lineIndex"
                                      class="block min-h-[17px] px-2.5" :class="DIFF_LINE_CLASS[line.type]">{{ diffLinePrefix(line.type) }}{{ line.text }}
</span></pre>
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </PopoverContent>
                          </Popover>
                          <span class="mx-1.5 h-4 w-px bg-[color:var(--td-component-stroke)]" />
                          <!--
                            The span is the trigger rather than the switch: the trigger writes its
                            own data-state onto its child, which would overwrite the checked state
                            the switch is styled by. It also keeps the tooltip working while the
                            switch is disabled.
                          -->
                          <Tooltip>
                            <TooltipTrigger as-child>
                              <span class="inline-flex">
                                <Switch
                                  :key="`${chunk.original.id}-${chunk.original.is_enabled}`"
                                  size="sm"
                                  :model-value="chunk.original.is_enabled"
                                  :disabled="chunkStatusLoading === chunk.original.id"
                                  :aria-label="
                                    chunk.original.is_enabled
                                      ? $t('knowledgeBase.disableChunk')
                                      : $t('knowledgeBase.enableChunk')
                                  "
                                  @update:model-value="(value: boolean) => toggleChunkEnabled(chunk.original, value)"
                                >
                                  <template #thumb>
                                    <Loader2Icon
                                      v-if="chunkStatusLoading === chunk.original.id"
                                      class="text-muted-foreground size-full animate-spin p-px"
                                    />
                                  </template>
                                </Switch>
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              {{
                                chunk.original.is_enabled
                                  ? $t("knowledgeBase.disableChunk")
                                  : $t("knowledgeBase.enableChunk")
                              }}
                            </TooltipContent>
                          </Tooltip>
                        </template>
                      </div>
                    </div>
                    <div v-if="editingChunkId === chunk.original.id" class="bg-muted mt-1 mb-2.5 rounded-[5px] p-3">
                      <div class="text-muted-foreground mb-2 flex items-center gap-1.5 text-xs font-medium">
                        <PencilIcon class="size-3.5" />
                        <span>{{ $t("knowledgeBase.editChunkContent") }}</span>
                      </div>
                      <Textarea
                        v-model="chunkDraft"
                        v-focus
                        class="bg-card dark:bg-card max-h-[calc(20lh+18px)] min-h-[calc(6lh+18px)]"
                      />
                      <div class="mt-2 flex items-center justify-end gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          :disabled="savingChunkId === chunk.original.id"
                          @click="editingChunkId = ''"
                          >{{ $t("common.cancel") }}</Button
                        >
                        <Button
                          size="sm"
                          :disabled="savingChunkId === chunk.original.id"
                          @click="saveChunkEdit(chunk.original)"
                        >
                          <Loader2Icon v-if="savingChunkId === chunk.original.id" class="animate-spin" />
                          {{ $t("common.save") }}
                        </Button>
                      </div>
                    </div>
                    <div
                      v-else
                      :class="[MD_CONTENT, { 'opacity-50': !chunk.original.is_enabled }]"
                      v-html="chunk.processedContent"
                    ></div>
                  </div>
                </div>
              </template>
            </div>

            <!-- 文档预览视图 -->
            <div
              v-if="(viewMode === 'merged' || viewMode === 'chunks') && details.total > CHUNK_PAGE_SIZE"
              class="text-muted-foreground mt-5 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 border-t border-solid border-[color:var(--td-component-stroke)] pt-4 text-[13px]"
              :class="{ 'pointer-events-none opacity-60': details.chunkLoading }"
            >
              <!--
                Reka's pagination primitives styled with the button variants
                directly, so this pager matches TenantMembersPager, the other
                replacement for t-pagination with a jumper.
              -->
              <span class="whitespace-nowrap">{{ $t("tenantMember.pager.total", { total: details.total }) }}</span>
              <PaginationRoot
                v-slot="{ page: current }"
                :page="chunkPage"
                :total="details.total"
                :items-per-page="CHUNK_PAGE_SIZE"
                :sibling-count="1"
                :disabled="details.chunkLoading"
                show-edges
                @update:page="goToChunkPage"
              >
                <PaginationList v-slot="{ items }" class="flex items-center gap-0.5">
                  <PaginationPrev
                    :class="buttonVariants({ variant: 'ghost', size: 'icon-sm' })"
                    :aria-label="$t('tenantMember.pager.previous')"
                  >
                    <ChevronLeftIcon />
                  </PaginationPrev>
                  <template v-for="(item, itemIndex) in items" :key="itemIndex">
                    <PaginationListItem
                      v-if="item.type === 'page'"
                      :value="item.value"
                      :class="[
                        buttonVariants({ variant: item.value === current ? 'outline' : 'ghost', size: 'icon-sm' }),
                        'text-[13px]',
                        item.value === current ? 'border-primary text-primary hover:text-primary' : 'text-foreground',
                      ]"
                    >
                      {{ item.value }}
                    </PaginationListItem>
                    <PaginationEllipsis v-else :index="itemIndex" class="flex size-7 items-center justify-center">
                      <EllipsisIcon class="size-4" />
                    </PaginationEllipsis>
                  </template>
                  <PaginationNext
                    :class="buttonVariants({ variant: 'ghost', size: 'icon-sm' })"
                    :aria-label="$t('tenantMember.pager.next')"
                  >
                    <ChevronRightIcon />
                  </PaginationNext>
                </PaginationList>
              </PaginationRoot>
              <label class="flex items-center gap-1.5 whitespace-nowrap">
                {{ $t("tenantMember.pager.jumpTo") }}
                <Input
                  v-model="chunkJumpValue"
                  type="number"
                  :min="1"
                  :max="chunkPageCount"
                  :disabled="details.chunkLoading"
                  class="h-7 w-14 px-1.5 text-center text-[13px]"
                  @keydown.enter="commitChunkJump"
                  @blur="commitChunkJump"
                />
                <span v-if="$t('tenantMember.pager.jumpToSuffix')">{{ $t("tenantMember.pager.jumpToSuffix") }}</span>
              </label>
            </div>

            <div v-else-if="viewMode === 'preview'">
              <DocumentPreview
                :knowledgeId="details.id"
                :fileType="details.file_type"
                :fileName="details.title"
                :active="viewMode === 'preview'"
              />
            </div>
          </section>
        </div>
      </DrawerContent>
    </Drawer>
  </div>
</template>
<style scoped>
/*
 * What stays CSS here is markup this template does not write: the code
 * blocks the Markdown renderer emits (see renderer.code in the script),
 * reached through :deep(), and the root of the processing timeline child
 * component. Both carry styles of their own that an utility on our side
 * could not outrank, because utilities live in a cascade layer and these
 * rules, like highlight.js's github.css, do not. The `.md-content` rules
 * live in css/markdown.css, imported from the script.
 */
:deep(.code-block-wrapper) {
  margin: 12px 0;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  background: var(--td-bg-color-container);
  overflow: hidden;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);

  .code-block-header {
    display: flex;
    align-items: center;
    padding: 8px 12px;
    background: var(--td-bg-color-secondarycontainer);
    border-bottom: 1px solid var(--td-component-stroke);
    font-size: 12px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .code-block-pre {
    margin: 0;
    padding: 12px;
    background: var(--td-bg-color-secondarycontainer);
    overflow: auto;
    font-size: 13px;
    line-height: 1.5;

    code {
      background: transparent;
      padding: 0;
      border: none;
      white-space: pre;
      word-wrap: normal;
      display: block;
    }
  }
}

/*
 * The drawer panel is a shadcn part (data-slot), so the element resets in
 * tailwind.css reach everything inside it, the rendered Markdown included,
 * and turn its images and SVGs into blocks. Markdown wants them inline, as
 * the browser draws them. The rule sits in the same base layer as the reset
 * so that css/markdown.css, in the components layer, still overrides it.
 */
@layer base {
  :deep(.md-content) :where(img, svg, video) {
    display: revert-layer;
    vertical-align: revert-layer;
  }
}

.kp-drawer-shell > :deep(.kp-timeline) {
  width: 100%;
  height: 100%;
}
</style>
