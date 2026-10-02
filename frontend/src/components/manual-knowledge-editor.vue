<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onBeforeUnmount, type Component } from "vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { marked } from "marked";
import { MessagePlugin } from "tdesign-vue-next";
import { useUIStore } from "@/stores/ui";
import {
  listKnowledgeBases,
  getKnowledgeDetails,
  getKnowledgeBaseById,
  createManualKnowledge,
  updateManualKnowledge,
} from "@/api/knowledge-base";
import { useUploadConfirmStore } from "@/stores/uploadConfirm";
import type { KnowledgeProcessOverrides } from "@/types/knowledgeProcess";
import { sanitizeHTML, safeMarkdownToHTML } from "@/utils/security";
import { useI18n } from "vue-i18n";
import {
  BoldIcon,
  CodeIcon,
  EyeIcon,
  Heading1Icon,
  Heading2Icon,
  Heading3Icon,
  ImageIcon,
  ItalicIcon,
  LinkIcon,
  ListIcon,
  ListOrderedIcon,
  Loader2Icon,
  MinusIcon,
  QuoteIcon,
  SquareCheckIcon,
  SquareCodeIcon,
  SquarePenIcon,
  StrikethroughIcon,
  TableIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

interface KnowledgeBaseOption {
  label: string;
  value: string;
}

interface KnowledgeDetailResponse {
  id: string;
  knowledge_base_id: string;
  title?: string;
  file_name?: string;
  metadata?: any;
  parse_status?: string;
  tags?: Array<{ id: string }>;
}

type ManualStatus = "draft" | "publish";

/** Derive editor status from metadata + parse_status (parse pipeline wins when indexed or in flight). */
const resolveManualKnowledgeStatus = (metaStatus: ManualStatus | undefined, parseStatus?: string): ManualStatus => {
  if (!parseStatus || parseStatus === "draft") {
    return metaStatus === "publish" ? "publish" : "draft";
  }
  if (
    parseStatus === "completed" ||
    parseStatus === "pending" ||
    parseStatus === "processing" ||
    parseStatus === "finalizing"
  ) {
    return "publish";
  }
  return metaStatus === "publish" ? "publish" : "draft";
};

const uiStore = useUIStore();
const uploadConfirmStore = useUploadConfirmStore();
const { t } = useI18n();

const visible = computed({
  get: () => uiStore.manualEditorVisible,
  set: (val: boolean) => {
    if (!val) {
      handleClose();
    }
  },
});

const mode = computed(() => uiStore.manualEditorMode);
const knowledgeId = computed(() => uiStore.manualEditorKnowledgeId);
const currentKnowledgeId = ref<string | null>(null);
const manualTagIds = ref<string[]>([]);

const form = reactive({
  kbId: "" as string,
  title: "",
  content: "",
  status: "draft" as ManualStatus,
});

const initialLoaded = ref(false);
const kbOptions = ref<KnowledgeBaseOption[]>([]);
const kbLoading = ref(false);
const contentLoading = ref(false);
const saving = ref(false);
const savingAction = ref<ManualStatus>("draft");
const activeTab = ref<"edit" | "preview">("edit");
const lastUpdatedAt = ref<string>("");

const textareaComponent = ref<any>(null);
const textareaElement = ref<HTMLTextAreaElement | null>(null);
const selectionRange = reactive({ start: 0, end: 0 });
const selectionEvents = ["select", "keyup", "click", "mouseup", "input"];

const resolveTextareaElement = (): HTMLTextAreaElement | null => {
  const component = textareaComponent.value as any;
  if (!component) return null;
  // The ui Textarea renders the <textarea> as its root element, so the
  // component's $el is the element itself.
  if (component.$el instanceof HTMLTextAreaElement) {
    return component.$el;
  }
  if (component.textareaRef) {
    return component.textareaRef as HTMLTextAreaElement;
  }
  if (component.$el) {
    const el = component.$el.querySelector("textarea");
    if (el) {
      return el as HTMLTextAreaElement;
    }
  }
  return null;
};

const handleTextareaSelectionEvent = () => {
  const textarea = textareaElement.value ?? resolveTextareaElement();
  if (!textarea) {
    return;
  }
  selectionRange.start = textarea.selectionStart ?? 0;
  selectionRange.end = textarea.selectionEnd ?? 0;
};

const detachTextareaListeners = () => {
  if (!textareaElement.value) {
    return;
  }
  selectionEvents.forEach((eventName) => {
    textareaElement.value?.removeEventListener(eventName, handleTextareaSelectionEvent);
  });
  textareaElement.value = null;
};

const attachTextareaListeners = () => {
  nextTick(() => {
    const textarea = resolveTextareaElement();
    if (!textarea) {
      return;
    }
    if (textareaElement.value === textarea) {
      return;
    }
    detachTextareaListeners();
    textareaElement.value = textarea;
    selectionEvents.forEach((eventName) => {
      textarea.addEventListener(eventName, handleTextareaSelectionEvent);
    });
    handleTextareaSelectionEvent();
  });
};

const setSelectionRange = (start: number, end: number) => {
  selectionRange.start = start;
  selectionRange.end = end;
  nextTick(() => {
    const textarea = resolveTextareaElement();
    if (!textarea || activeTab.value !== "edit") {
      return;
    }
    // Initialization can finish while the drawer is still sliding in. A plain
    // focus() makes the browser scroll the transformed textarea into view,
    // which intermittently shifts the drawer away from the right edge for a
    // frame. Keep keyboard focus without letting it move the viewport.
    textarea.focus({ preventScroll: true });
    textarea.setSelectionRange(start, end);
  });
};

const getSelectionRange = () => {
  return {
    start: selectionRange.start ?? 0,
    end: selectionRange.end ?? 0,
  };
};

const clampRange = (start: number, end: number, length: number) => {
  let safeStart = Math.max(0, Math.min(start, length));
  let safeEnd = Math.max(0, Math.min(end, length));
  if (safeEnd < safeStart) {
    [safeStart, safeEnd] = [safeEnd, safeStart];
  }
  return { safeStart, safeEnd };
};

const updateContentWithSelection = (content: string, start: number, end: number) => {
  form.content = content;
  setSelectionRange(start, end);
};

const findLineStart = (value: string, index: number) => {
  if (index <= 0) return 0;
  const lastNewline = value.lastIndexOf("\n", index - 1);
  return lastNewline === -1 ? 0 : lastNewline + 1;
};

const findLineEnd = (value: string, index: number) => {
  if (index >= value.length) return value.length;
  const newlineIndex = value.indexOf("\n", index);
  return newlineIndex === -1 ? value.length : newlineIndex;
};

const transformSelectedLines = (transformer: (line: string, index: number) => string) => {
  const value = form.content ?? "";
  const { start, end } = getSelectionRange();
  const { safeStart, safeEnd } = clampRange(start, end, value.length);
  const lineStart = findLineStart(value, safeStart);
  const lineEnd = findLineEnd(value, safeEnd);
  const selected = value.slice(lineStart, lineEnd);
  const lines = selected.split("\n");
  const transformed = lines.map((line, index) => transformer(line, index));
  const result = transformed.join("\n");
  const newContent = value.slice(0, lineStart) + result + value.slice(lineEnd);
  updateContentWithSelection(newContent, lineStart, lineStart + result.length);
};

const wrapSelection = (prefix: string, suffix: string, placeholder: string) => {
  const value = form.content ?? "";
  const { start, end } = getSelectionRange();
  const { safeStart, safeEnd } = clampRange(start, end, value.length);
  const hasSelection = safeEnd > safeStart;
  const selectedText = hasSelection ? value.slice(safeStart, safeEnd) : placeholder;
  const result = value.slice(0, safeStart) + prefix + selectedText + suffix + value.slice(safeEnd);
  const selectionStart = safeStart + prefix.length;
  const selectionEnd = selectionStart + selectedText.length;
  updateContentWithSelection(result, selectionStart, selectionEnd);
};

const insertBlock = (text: string, selectionStartOffset?: number, selectionEndOffset?: number) => {
  const value = form.content ?? "";
  const { start, end } = getSelectionRange();
  const { safeStart, safeEnd } = clampRange(start, end, value.length);
  const before = value.slice(0, safeStart);
  const after = value.slice(safeEnd);
  const result = before + text + after;
  const base = safeStart;
  const selectionStart = selectionStartOffset !== undefined ? base + selectionStartOffset : base + text.length;
  const selectionEnd = selectionEndOffset !== undefined ? base + selectionEndOffset : selectionStart;
  updateContentWithSelection(result, selectionStart, selectionEnd);
};

const applyHeading = (level: number) => {
  const hashes = "#".repeat(level);
  transformSelectedLines((line) => {
    const trimmed = line.replace(/^#+\s*/, "").trim();
    const content = trimmed || t("manualEditor.placeholders.heading", { level });
    return `${hashes} ${content}`;
  });
};

const listPrefixPattern = /^(\s*(?:[-*+]|\d+\.)\s+|\s*-\s+\[[ xX]\]\s+)/;

const applyBulletList = () => {
  transformSelectedLines((line) => {
    const trimmed = line.trim();
    const content = trimmed.replace(listPrefixPattern, "").trim();
    return `- ${content || t("manualEditor.placeholders.listItem")}`;
  });
};

const applyOrderedList = () => {
  transformSelectedLines((line, index) => {
    const trimmed = line.trim();
    const content = trimmed.replace(listPrefixPattern, "").trim();
    return `${index + 1}. ${content || t("manualEditor.placeholders.listItem")}`;
  });
};

const applyTaskList = () => {
  transformSelectedLines((line) => {
    const trimmed = line.trim();
    const content = trimmed.replace(listPrefixPattern, "").trim();
    return `- [ ] ${content || t("manualEditor.placeholders.taskItem")}`;
  });
};

const applyBlockquote = () => {
  transformSelectedLines((line) => {
    const trimmed = line.trim().replace(/^>\s?/, "").trim();
    return `> ${trimmed || t("manualEditor.placeholders.quote")}`;
  });
};

const insertCodeBlock = () => {
  const placeholder = t("manualEditor.placeholders.code");
  const block = `\n\`\`\`\n${placeholder}\n\`\`\`\n`;
  const startOffset = block.indexOf(placeholder);
  insertBlock(block, startOffset, startOffset + placeholder.length);
};

const insertHorizontalRule = () => {
  insertBlock("\n---\n\n");
};

const insertTable = () => {
  const cell = t("manualEditor.table.cell");
  const template = `\n| ${t("manualEditor.table.column1")} | ${t("manualEditor.table.column2")} |\n| --- | --- |\n| ${cell} | ${cell} |\n`;
  const placeholderIndex = template.indexOf(cell);
  insertBlock(template, placeholderIndex, placeholderIndex + cell.length);
};

const insertLink = () => {
  const value = form.content ?? "";
  const { start, end } = getSelectionRange();
  const { safeStart, safeEnd } = clampRange(start, end, value.length);
  const selectedText = safeEnd > safeStart ? value.slice(safeStart, safeEnd) : t("manualEditor.placeholders.linkText");
  const urlPlaceholder = "https://";
  const result = value.slice(0, safeStart) + `[${selectedText}](${urlPlaceholder})` + value.slice(safeEnd);
  const urlStart = safeStart + selectedText.length + 3;
  const urlEnd = urlStart + urlPlaceholder.length;
  updateContentWithSelection(result, urlStart, urlEnd);
};

const insertImage = () => {
  const value = form.content ?? "";
  const { start, end } = getSelectionRange();
  const { safeStart, safeEnd } = clampRange(start, end, value.length);
  const altText = safeEnd > safeStart ? value.slice(safeStart, safeEnd) : t("manualEditor.placeholders.imageAlt");
  const urlPlaceholder = "https://";
  const result = value.slice(0, safeStart) + `![${altText}](${urlPlaceholder})` + value.slice(safeEnd);
  const urlStart = safeStart + altText.length + 4;
  const urlEnd = urlStart + urlPlaceholder.length;
  updateContentWithSelection(result, urlStart, urlEnd);
};

type ToolbarAction = () => void;
type ToolbarButton = {
  key: string;
  tooltip: string;
  action: ToolbarAction;
  icon: Component;
};
type ToolbarGroup = {
  key: string;
  buttons: ToolbarButton[];
};

const toolbarGroups = computed<ToolbarGroup[]>(() => [
  {
    key: "format",
    buttons: [
      {
        key: "bold",
        icon: BoldIcon,
        tooltip: t("manualEditor.toolbar.bold"),
        action: () => wrapSelection("**", "**", t("manualEditor.placeholders.bold")),
      },
      {
        key: "italic",
        icon: ItalicIcon,
        tooltip: t("manualEditor.toolbar.italic"),
        action: () => wrapSelection("*", "*", t("manualEditor.placeholders.italic")),
      },
      {
        key: "strike",
        icon: StrikethroughIcon,
        tooltip: t("manualEditor.toolbar.strike"),
        action: () => wrapSelection("~~", "~~", t("manualEditor.placeholders.strike")),
      },
      {
        key: "inline-code",
        icon: CodeIcon,
        tooltip: t("manualEditor.toolbar.inlineCode"),
        action: () => wrapSelection("`", "`", t("manualEditor.placeholders.inlineCode")),
      },
    ],
  },
  {
    key: "heading",
    buttons: [
      { key: "h1", icon: Heading1Icon, tooltip: t("manualEditor.toolbar.heading1"), action: () => applyHeading(1) },
      { key: "h2", icon: Heading2Icon, tooltip: t("manualEditor.toolbar.heading2"), action: () => applyHeading(2) },
      { key: "h3", icon: Heading3Icon, tooltip: t("manualEditor.toolbar.heading3"), action: () => applyHeading(3) },
    ],
  },
  {
    key: "list",
    buttons: [
      { key: "ul", icon: ListIcon, tooltip: t("manualEditor.toolbar.bulletList"), action: applyBulletList },
      { key: "ol", icon: ListOrderedIcon, tooltip: t("manualEditor.toolbar.orderedList"), action: applyOrderedList },
      { key: "task", icon: SquareCheckIcon, tooltip: t("manualEditor.toolbar.taskList"), action: applyTaskList },
      { key: "quote", icon: QuoteIcon, tooltip: t("manualEditor.toolbar.blockquote"), action: applyBlockquote },
    ],
  },
  {
    key: "insert",
    buttons: [
      { key: "codeblock", icon: SquareCodeIcon, tooltip: t("manualEditor.toolbar.codeBlock"), action: insertCodeBlock },
      { key: "link", icon: LinkIcon, tooltip: t("manualEditor.toolbar.link"), action: insertLink },
      { key: "image", icon: ImageIcon, tooltip: t("manualEditor.toolbar.image"), action: insertImage },
      { key: "table", icon: TableIcon, tooltip: t("manualEditor.toolbar.table"), action: insertTable },
      {
        key: "hr",
        icon: MinusIcon,
        tooltip: t("manualEditor.toolbar.horizontalRule"),
        action: insertHorizontalRule,
      },
    ],
  },
]);

const isPreviewMode = computed(() => activeTab.value === "preview");
const viewToggleIcon = computed<Component>(() => (isPreviewMode.value ? SquarePenIcon : EyeIcon));
const viewToggleLabel = computed(() =>
  isPreviewMode.value ? t("manualEditor.view.editLabel") : t("manualEditor.view.previewLabel"),
);

const handleToolbarAction = (action: ToolbarAction) => {
  if (saving.value) {
    return;
  }
  if (activeTab.value !== "edit") {
    activeTab.value = "edit";
    nextTick(() => {
      attachTextareaListeners();
      action();
    });
  } else {
    attachTextareaListeners();
    action();
  }
};

const toggleEditorView = () => {
  activeTab.value = isPreviewMode.value ? "edit" : "preview";
};

marked.use({});

const previewHTML = computed(() => {
  if (!form.content) {
    return `<p class="empty-preview">${t("manualEditor.preview.empty")}</p>`;
  }
  const safeMarkdown = safeMarkdownToHTML(form.content);
  const html = marked.parse(safeMarkdown, { async: false });
  return sanitizeHTML(html);
});

const kbDisabled = computed(() => mode.value === "edit" && !!form.kbId);

const dialogTitle = computed(() =>
  mode.value === "edit" ? t("manualEditor.title.edit") : t("manualEditor.title.create"),
);

const lastUpdatedText = computed(() =>
  lastUpdatedAt.value ? t("manualEditor.status.lastUpdated", { time: lastUpdatedAt.value }) : "",
);

const loadKnowledgeBases = async () => {
  kbLoading.value = true;
  try {
    const res = (await listKnowledgeBases()) as any;

    // Only document knowledge bases take manually written content; FAQ
    // knowledge bases have their own entry editor.
    const isDocumentKb = (type?: string) => !type || type === "document";

    const kbs = Array.isArray(res?.data) ? res.data : [];
    const list: KnowledgeBaseOption[] = kbs
      .filter((item: any) => isDocumentKb(item.type))
      .map((item: any) => ({ label: item.name, value: item.id }));

    kbOptions.value = list;

    if (mode.value === "create") {
      const presetKbId = uiStore.manualEditorKBId;
      if (presetKbId) {
        const exists = list.find((item) => item.value === presetKbId);
        if (!exists) {
          kbOptions.value.unshift({
            label: t("manualEditor.labels.currentKnowledgeBase"),
            value: presetKbId,
          });
        }
        form.kbId = presetKbId;
      } else {
        form.kbId = list[0]?.value ?? "";
      }
    }
  } catch (error) {
    console.error("[ManualEditor] Failed to load knowledge base list:", error);
    kbOptions.value = [];
  } finally {
    kbLoading.value = false;
  }
};

const parseManualMetadata = (metadata: any): { content: string; status: ManualStatus; updatedAt?: string } | null => {
  if (!metadata) {
    return null;
  }
  try {
    let parsed = metadata;
    if (typeof metadata === "string") {
      parsed = JSON.parse(metadata);
    }
    if (parsed && typeof parsed === "object") {
      const status = parsed.status === "publish" ? "publish" : "draft";
      return {
        content: parsed.content || "",
        status,
        updatedAt: parsed.updated_at || parsed.updatedAt,
      };
    }
  } catch (error) {
    console.warn("[ManualEditor] Failed to parse manual metadata:", error);
  }
  return null;
};

const loadKnowledgeContent = async () => {
  if (!currentKnowledgeId.value) {
    return;
  }
  contentLoading.value = true;
  try {
    const res: any = await getKnowledgeDetails(currentKnowledgeId.value);
    const data: KnowledgeDetailResponse | undefined = res?.data;
    if (!data) {
      MessagePlugin.error(t("manualEditor.error.fetchDetailFailed"));
      return;
    }

    form.kbId = data.knowledge_base_id || form.kbId;
    const meta = parseManualMetadata(data.metadata);
    form.title = data.title || data.file_name?.replace(/\.md$/i, "") || uiStore.manualEditorInitialTitle || "";
    form.content = meta?.content || uiStore.manualEditorInitialContent || "";
    form.status = resolveManualKnowledgeStatus(meta?.status, data.parse_status);
    manualTagIds.value = (data.tags || []).map((tag) => String(tag.id));
    if (meta?.updatedAt) {
      lastUpdatedAt.value = meta.updatedAt;
    }

    if (form.kbId && !kbOptions.value.find((item) => item.value === form.kbId)) {
      kbOptions.value.unshift({
        label: t("manualEditor.labels.currentKnowledgeBase"),
        value: form.kbId,
      });
    }
  } catch (error) {
    console.error("[ManualEditor] Failed to load manual knowledge:", error);
    MessagePlugin.error(t("manualEditor.error.fetchDetailFailed"));
  } finally {
    contentLoading.value = false;
  }
};

const resetForm = () => {
  currentKnowledgeId.value = knowledgeId.value || null;
  form.kbId = uiStore.manualEditorKBId || "";
  form.title = uiStore.manualEditorInitialTitle || "";
  form.content = uiStore.manualEditorInitialContent || "";
  form.status = uiStore.manualEditorInitialStatus || "draft";
  activeTab.value = "edit";
  lastUpdatedAt.value = "";
  initialLoaded.value = false;
  manualTagIds.value = mode.value === "create" ? [...uiStore.selectedTagIds] : [];
  selectionRange.start = 0;
  selectionRange.end = 0;
};

const generateDefaultTitle = () => {
  if (uiStore.manualEditorInitialTitle) {
    return uiStore.manualEditorInitialTitle;
  }
  return `${t("manualEditor.defaultTitlePrefix")}-${new Date().toLocaleString()}`;
};

const initialize = async () => {
  resetForm();
  await loadKnowledgeBases();

  if (mode.value === "edit") {
    await loadKnowledgeContent();
  } else {
    const presetKbId = uiStore.manualEditorKBId;
    if (presetKbId) {
      form.kbId = presetKbId;
    } else if (!form.kbId && kbOptions.value.length) {
      form.kbId = kbOptions.value[0].value;
    }
    form.title = form.title || generateDefaultTitle();
    form.content = form.content || "";
  }

  initialLoaded.value = true;
};

const validateForm = (targetStatus: ManualStatus): boolean => {
  if (!form.kbId) {
    MessagePlugin.warning(t("manualEditor.warning.selectKnowledgeBase"));
    return false;
  }
  if (!form.title || !form.title.trim()) {
    MessagePlugin.warning(t("manualEditor.warning.enterTitle"));
    return false;
  }
  if (!form.content || !form.content.trim()) {
    MessagePlugin.warning(t("manualEditor.warning.enterContent"));
    return false;
  }
  if (targetStatus === "publish" && form.content.trim().length < 10) {
    MessagePlugin.warning(t("manualEditor.warning.contentTooShort"));
    return false;
  }
  return true;
};

const handleSave = async (targetStatus: ManualStatus) => {
  if (saving.value || !validateForm(targetStatus)) {
    return;
  }
  saving.value = true;
  savingAction.value = targetStatus;
  try {
    const payload: {
      title: string;
      content: string;
      status: string;
      tag_ids?: string[];
      process_config?: KnowledgeProcessOverrides;
    } = {
      title: form.title.trim(),
      content: form.content,
      status: targetStatus,
    };
    payload.tag_ids = [...manualTagIds.value];

    if (targetStatus === "publish") {
      let kbInfo: any;
      try {
        const kbRes: any = await getKnowledgeBaseById(form.kbId);
        kbInfo = kbRes?.data;
      } catch {
        MessagePlugin.error(t("manualEditor.error.fetchDetailFailed"));
        return;
      }
      if (!kbInfo) {
        MessagePlugin.error(t("manualEditor.error.fetchDetailFailed"));
        return;
      }
      try {
        const confirmResult = await uploadConfirmStore.open({
          mode: "manual",
          kbInfo,
          manual: {
            kbId: form.kbId,
            knowledgeId: currentKnowledgeId.value || undefined,
            title: payload.title,
            content: payload.content,
            tagIds: [...manualTagIds.value],
          },
        });
        payload.process_config = confirmResult.processConfig;
        manualTagIds.value = [...(confirmResult.tagIds || [])];
        payload.tag_ids = [...manualTagIds.value];
      } catch {
        return;
      }
    }

    let response: any;
    let knowledgeID = currentKnowledgeId.value;
    let kbId = form.kbId;

    if (mode.value === "edit" && currentKnowledgeId.value) {
      response = await updateManualKnowledge(currentKnowledgeId.value, payload);
    } else {
      response = await createManualKnowledge(form.kbId, payload);
      knowledgeID = response?.data?.id || knowledgeID;
      currentKnowledgeId.value = knowledgeID || null;
      uiStore.manualEditorKnowledgeId = currentKnowledgeId.value;
      kbId = form.kbId;
    }

    if (response?.success) {
      MessagePlugin.success(
        targetStatus === "draft" ? t("manualEditor.success.draftSaved") : t("manualEditor.success.published"),
      );
      if (knowledgeID) {
        uiStore.notifyManualEditorSuccess({
          kbId,
          knowledgeId: knowledgeID,
          status: targetStatus,
        });
      }
      uiStore.closeManualEditor();
    } else {
      const message = response?.message || t("manualEditor.error.saveFailed");
      MessagePlugin.error(message);
    }
  } catch (error: any) {
    const message = error?.error?.message || error?.message || t("manualEditor.error.saveFailed");
    MessagePlugin.error(message);
  } finally {
    saving.value = false;
  }
};

const handleClose = () => {
  uiStore.closeManualEditor();
};

watch(visible, async (val) => {
  if (val) {
    await nextTick();
    await initialize();
    await nextTick();
    attachTextareaListeners();
    const length = form.content ? form.content.length : 0;
    setSelectionRange(length, length);
  } else {
    detachTextareaListeners();
    resetForm();
  }
});

watch(activeTab, (val) => {
  if (val === "edit") {
    nextTick(() => {
      attachTextareaListeners();
    });
  } else {
    detachTextareaListeners();
  }
});

onBeforeUnmount(() => {
  detachTextareaListeners();
});
</script>

<template>
  <SettingDrawer
    :visible="visible"
    :title="dialogTitle"
    :description="$t('manualEditor.description')"
    icon="edit-1"
    width="760px"
    :min-width="560"
    :max-width="1280"
    storage-key="setting-drawer:width:manual-markdown-editor"
    :hide-footer="!initialLoaded"
    @update:visible="
      (v: boolean) => {
        visible = v;
      }
    "
  >
    <template #footer-left>
      <div class="text-placeholder flex min-w-0 items-center gap-2">
        <span
          v-if="form.status === 'draft'"
          class="text-warning inline-flex h-5 items-center rounded-(--td-radius-default) bg-(--td-warning-color-light) px-1.5 text-xs leading-none whitespace-nowrap"
        >
          {{ $t("manualEditor.status.draftTag") }}
        </span>
        <span
          v-else
          class="text-success inline-flex h-5 items-center rounded-(--td-radius-default) bg-(--td-success-color-light) px-1.5 text-xs leading-none whitespace-nowrap"
        >
          {{ $t("manualEditor.status.publishedTag") }}
        </span>
      </div>
    </template>

    <template #footer-right>
      <div class="flex items-center justify-end gap-2">
        <Button
          variant="secondary"
          class="bg-muted text-muted-foreground hover:border-border hover:bg-accent hover:text-foreground min-w-[88px] border-transparent transition-colors duration-200"
          :disabled="saving"
          @click="handleClose"
        >
          {{ $t("manualEditor.actions.cancel") }}
        </Button>
        <Button variant="outline" class="min-w-[88px]" :disabled="saving" @click="handleSave('draft')">
          <Loader2Icon v-if="saving && savingAction === 'draft'" class="animate-spin" />
          {{ $t("manualEditor.actions.saveDraft") }}
        </Button>
        <Button class="min-w-[88px]" :disabled="saving" @click="handleSave('publish')">
          <Loader2Icon v-if="saving && savingAction === 'publish'" class="animate-spin" />
          {{ $t("manualEditor.actions.publish") }}
        </Button>
      </div>
    </template>

    <!-- The section / section-title classes are SettingDrawer's styling
         contract for the groups inside its body. -->
    <div v-if="initialLoaded" class="flex flex-col">
      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t("manualEditor.section.basic") }}</h4>

        <div class="flex flex-col gap-1.5">
          <label
            for="manual-editor-title"
            class="text-foreground after:text-destructive text-[13px] font-medium after:ml-1 after:content-['*']"
            >{{ $t("manualEditor.form.titleLabel") }}</label
          >
          <!-- The character counter TDesign drew inside the input (showLimitNumber). -->
          <div class="relative">
            <Input
              id="manual-editor-title"
              :model-value="form.title"
              maxlength="100"
              class="pr-16"
              :placeholder="$t('manualEditor.form.titlePlaceholder')"
              @update:model-value="(v) => (form.title = String(v))"
            />
            <span
              class="text-placeholder pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-xs tabular-nums"
              >{{ form.title.length }}/100</span
            >
          </div>
        </div>

        <div class="flex flex-col gap-1.5">
          <label
            class="text-foreground after:text-destructive text-[13px] font-medium after:ml-1 after:content-['*']"
            >{{ $t("manualEditor.form.knowledgeBaseLabel") }}</label
          >
          <div class="flex items-center gap-3">
            <!-- The select's popup is portalled to <body>; z-[5500] puts it
                 above the drawer's z-[2500] layer, as TDesign's popups were. -->
            <Select
              :model-value="form.kbId || undefined"
              :disabled="kbDisabled"
              @update:model-value="(v) => (form.kbId = String(v ?? ''))"
            >
              <SelectTrigger class="w-full min-w-0 flex-1">
                <Loader2Icon v-if="kbLoading" class="text-muted-foreground size-3.5 animate-spin" />
                <SelectValue :placeholder="$t('manualEditor.form.knowledgeBasePlaceholder')" />
              </SelectTrigger>
              <SelectContent position="popper" class="z-[5500]">
                <SelectItem v-for="opt in kbOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </SelectItem>
                <div v-if="kbOptions.length === 0" class="text-placeholder p-5 text-center text-sm">
                  {{ $t("manualEditor.noDocumentKnowledgeBases") }}
                </div>
              </SelectContent>
            </Select>
            <div v-if="mode === 'edit'" class="flex shrink-0 items-center gap-1.5 whitespace-nowrap">
              <span
                v-if="form.status === 'draft'"
                class="text-warning inline-flex h-5 items-center rounded-(--td-radius-default) bg-(--td-warning-color-light) px-1.5 text-xs leading-none"
              >
                {{ $t("manualEditor.status.draftTag") }}
              </span>
              <span
                v-else
                class="text-success inline-flex h-5 items-center rounded-(--td-radius-default) bg-(--td-success-color-light) px-1.5 text-xs leading-none"
              >
                {{ $t("manualEditor.status.publishedTag") }}
              </span>
            </div>
          </div>
          <p v-if="lastUpdatedText" class="text-placeholder mx-0 mt-0.5 mb-0 text-xs">{{ lastUpdatedText }}</p>
        </div>
      </section>

      <!-- The content section takes the remaining room. -->
      <section class="setting-drawer__section min-h-0 flex-1">
        <h4 class="setting-drawer__section-title">{{ $t("manualEditor.section.content") }}</h4>

        <!-- The drawer is full-height: subtracting the rough height of the
             header, footer and basic-info section lets the editor fill what is
             left without depending on the parent flex chain. -->
        <div
          class="border-border bg-card focus-within:border-primary flex h-[calc(100vh-360px)] min-h-[280px] flex-col overflow-hidden rounded-lg border border-solid transition-[border-color,box-shadow] duration-200 focus-within:shadow-[0_0_0_2px_rgba(7,192,95,0.1)]"
        >
          <div
            class="border-border bg-muted flex shrink-0 flex-nowrap items-center justify-between gap-3 overflow-hidden border-0 border-b border-solid px-2 py-1.5"
          >
            <div class="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto [&::-webkit-scrollbar]:h-0">
              <template v-for="(group, groupIndex) in toolbarGroups" :key="group.key">
                <div class="flex items-center gap-0.5">
                  <Tooltip v-for="btn in group.buttons" :key="btn.key">
                    <TooltipTrigger as-child>
                      <button
                        type="button"
                        data-slot="toolbar-button"
                        class="text-muted-foreground hover:text-primary flex size-7 items-center justify-center rounded-md transition-all duration-200 hover:bg-[rgba(7,192,95,0.08)] focus-visible:shadow-[0_0_0_2px_rgba(7,192,95,0.25)] focus-visible:outline-none active:translate-y-[0.5px] active:bg-[rgba(7,192,95,0.15)]"
                        :aria-label="btn.tooltip"
                        @mousedown.prevent
                        @click="handleToolbarAction(btn.action)"
                      >
                        <component :is="btn.icon" class="size-4" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="top" class="z-[5500]">{{ btn.tooltip }}</TooltipContent>
                  </Tooltip>
                </div>
                <div v-if="groupIndex < toolbarGroups.length - 1" class="bg-border mx-1 h-[18px] w-px shrink-0"></div>
              </template>
            </div>
            <div class="border-border flex shrink-0 items-center border-0 border-l border-solid pl-2">
              <button
                type="button"
                data-slot="view-toggle"
                class="hover:text-primary inline-flex h-[30px] min-w-[92px] items-center justify-center gap-[5px] rounded-[7px] border border-solid px-2.5 text-xs font-medium shadow-[0_1px_2px_rgba(15,23,42,0.04)] transition-[background-color,border-color,color,box-shadow] duration-200 hover:border-[rgba(7,192,95,0.45)] hover:bg-[rgba(7,192,95,0.06)] hover:shadow-[0_2px_6px_rgba(7,192,95,0.1)] active:translate-y-px active:shadow-none disabled:cursor-not-allowed disabled:opacity-50"
                :class="
                  isPreviewMode
                    ? 'text-primary border-[rgba(7,192,95,0.5)] bg-[rgba(7,192,95,0.1)]'
                    : 'border-border bg-card text-muted-foreground'
                "
                :disabled="saving"
                @click="toggleEditorView"
              >
                <component :is="viewToggleIcon" class="size-[15px]" />
                {{ viewToggleLabel }}
              </button>
            </div>
          </div>

          <div v-show="activeTab === 'edit'" class="bg-card flex min-h-0 flex-1 flex-col overflow-hidden">
            <Textarea
              v-if="!contentLoading"
              ref="textareaComponent"
              :model-value="form.content"
              :placeholder="$t('manualEditor.form.contentPlaceholder')"
              class="bg-card dark:bg-card field-sizing-fixed h-full min-h-0 flex-1 resize-none rounded-none border-0 px-4 py-3.5 font-(family-name:--app-font-family-mono) text-sm leading-[1.7] focus-visible:ring-0 md:text-sm"
              @update:model-value="(v) => (form.content = String(v))"
            />
            <div
              v-else
              class="text-muted-foreground flex min-h-[280px] flex-1 items-center justify-center gap-2 p-5 text-sm"
            >
              <Loader2Icon class="text-primary size-4 animate-spin" />
              <span>{{ $t("manualEditor.loading.content") }}</span>
            </div>
          </div>
          <div v-show="activeTab === 'preview'" class="bg-card flex min-h-0 flex-1 flex-col overflow-hidden">
            <div
              class="manual-editor-preview bg-card text-foreground min-h-0 flex-1 overflow-y-auto p-4 text-sm leading-[1.7]"
              v-html="previewHTML"
            />
          </div>
        </div>
      </section>
    </div>
    <div v-else class="text-muted-foreground flex min-h-[280px] flex-1 items-center justify-center gap-2 p-5 text-sm">
      <Loader2Icon class="text-primary size-6 animate-spin" />
      <span>{{ $t("manualEditor.loading.preparing") }}</span>
    </div>
  </SettingDrawer>
</template>

<style scoped>
/*
 * Stays CSS: the preview is rendered Markdown (v-html), markup this template
 * cannot put classes on, so it is styled through :deep().
 */
.manual-editor-preview :deep(h1),
.manual-editor-preview :deep(h2),
.manual-editor-preview :deep(h3),
.manual-editor-preview :deep(h4) {
  margin-top: 16px;
  margin-bottom: 8px;
}

.manual-editor-preview :deep(code) {
  background: var(--td-bg-color-container-hover);
  padding: 2px 4px;
  border-radius: 4px;
  font-family: var(--app-font-family-mono);
}

.manual-editor-preview :deep(pre) {
  background: var(--td-bg-color-container-hover);
  padding: 12px;
  border-radius: 6px;
  overflow: auto;
}

.manual-editor-preview :deep(blockquote) {
  border-left: 4px solid var(--td-brand-color);
  padding-left: 12px;
  color: var(--td-text-color-secondary);
  margin: 16px 0;
  background: rgba(7, 192, 95, 0.08);
}

.manual-editor-preview :deep(a) {
  color: var(--td-brand-color);
}

/*
 * The empty-state paragraph previewHTML emits. The old scoped `.empty-preview`
 * rule never reached it — v-html content carries no scope attribute — so
 * it is styled through :deep() now, as it was meant to be.
 */
.manual-editor-preview :deep(.empty-preview) {
  color: var(--td-text-color-placeholder);
}
</style>
