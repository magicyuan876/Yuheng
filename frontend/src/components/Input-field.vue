<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from "vue";
import { storeToRefs } from "pinia";
import { useRoute, useRouter } from "vue-router";
import { onBeforeRouteUpdate } from "vue-router";
import type { Component } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { FileIcon, FolderIcon, MessageCircleQuestionMarkIcon, MessageSquareIcon, TagIcon } from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useSettingsStore } from "@/stores/settings";
import { useUIStore } from "@/stores/ui";
import { useMenuStore } from "@/stores/menu";
import { searchKnowledge, batchQueryKnowledge, listKnowledgeTags } from "@/api/knowledge-base";
import { stopSession } from "@/api/chat";
import { useOrganizationStore } from "@/stores/organization";
import MentionSelector from "./MentionSelector.vue";
import { getCaretCoordinates } from "@/utils/caret";
import { getRootZoom, rectToCssPx, cssViewportSize } from "@/utils/zoom";
import { type ModelConfig } from "@/api/model";
import { useChatResourcesStore } from "@/stores/chatResources";
import { useI18n } from "vue-i18n";
import AttachmentUpload, { type AttachmentFile } from "./AttachmentUpload.vue";
import type { MentionItem, MentionItemType, MentionRequestItem } from "@/types/mention";

const route = useRoute();
const router = useRouter();
const settingsStore = useSettingsStore();
const uiStore = useUIStore();
const orgStore = useOrganizationStore();
const menuStore = useMenuStore();
const chatResources = useChatResourcesStore();
const { chatModels: availableModels } = storeToRefs(chatResources);
const { t } = useI18n();

const query = ref("");

// Image upload state
const uploadedImages = ref<Array<{ file: File; preview: string }>>([]);
const imageInputRef = ref<HTMLInputElement>();

// Attachment upload state
const attachmentUploadRef = ref<InstanceType<typeof AttachmentUpload>>();
const uploadedAttachments = ref<AttachmentFile[]>([]);
const CHAT_FILE_DROP_EVENT = "yuheng:chat-file-drop";

const isImageFile = (file: File) => {
  if (file.type.startsWith("image/")) {
    return true;
  }
  const fileName = file.name.toLowerCase();
  return [".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"].some((ext) => fileName.endsWith(ext));
};

const handleDroppedFiles = (files: File[]) => {
  if (!files.length) return;

  const imageFiles = files.filter(isImageFile);
  const attachmentFiles = files.filter((file) => !isImageFile(file));

  if (imageFiles.length > 0) {
    addImageFiles(imageFiles);
  }

  if (attachmentFiles.length > 0) {
    attachmentUploadRef.value?.addFiles(attachmentFiles);
  }
};

const handleChatFileDrop = (event: Event) => {
  const customEvent = event as CustomEvent<{ files?: File[] }>;
  const files = customEvent.detail?.files;
  if (!files || files.length === 0) return;
  handleDroppedFiles(files);
};

const handleImageSelect = (event: Event) => {
  const input = event.target as HTMLInputElement;
  if (!input.files) return;
  addImageFiles(Array.from(input.files));
  input.value = "";
};

const addImageFiles = (files: File[]) => {
  const allowed = ["image/jpeg", "image/png", "image/gif", "image/webp"];
  const maxSize = 10 * 1024 * 1024;
  for (const file of files) {
    if (uploadedImages.value.length >= 5) {
      MessagePlugin.warning(t("chat.imageTooMany"));
      break;
    }
    if (!allowed.includes(file.type)) {
      MessagePlugin.warning(t("chat.imageTypeSizeError"));
      continue;
    }
    if (file.size > maxSize) {
      MessagePlugin.warning(t("chat.imageTypeSizeError"));
      continue;
    }
    uploadedImages.value.push({ file, preview: URL.createObjectURL(file) });
  }
};

const removeImage = (index: number) => {
  const removed = uploadedImages.value.splice(index, 1);
  if (removed.length > 0) URL.revokeObjectURL(removed[0].preview);
};

const triggerImageUpload = () => {
  imageInputRef.value?.click();
};
const atButtonRef = ref<HTMLElement>();

// Mention related state
const showMention = ref(false);
const mentionQuery = ref("");
const mentionItems = ref<MentionItem[]>([]);
/** 文件 ID -> 知识库 ID（用于批量查询时传 kb_id，支持共享知识库下的文档） */
const fileIdToKbId = ref<Record<string, string>>({});
const mentionActiveIndex = ref(0);
const mentionStyle = ref<Record<string, string>>({});
const textareaRef = ref<HTMLTextAreaElement | null>(null);
const mentionSelectorRef = ref<any>(null);
const mentionStartPos = ref(0);
const isComposing = ref(false);
const isMentionTriggeredByButton = ref(false);
const mentionHasMore = ref(false);
const mentionGroupCounts = ref<Partial<Record<MentionItemType, number>>>({});
const mentionLoading = ref(false);
const mentionOffset = ref(0);
const MENTION_PAGE_SIZE = 20;

const props = defineProps({
  isReplying: {
    type: Boolean,
    required: false,
  },
  sessionId: {
    type: String,
    required: false,
  },
  assistantMessageId: {
    type: String,
    required: false,
  },
});

const selectedKbIds = computed(() => settingsStore.settings.selectedKnowledgeBases || []);
const selectedFileIds = computed(() => settingsStore.settings.selectedFiles || []);
const selectedTags = computed(() => settingsStore.settings.selectedTags || []);

// 已就绪的知识库（来自空间级缓存）
const knowledgeBases = computed(() => chatResources.validKnowledgeBases);
const fileList = ref<Array<{ id: string; name: string }>>([]);

// 选中的知识库：包含自己的 + 组织共享的（用于展示已选列表与 org 角标）
const selectedKbs = computed(() => {
  const own = knowledgeBases.value.filter((kb) => selectedKbIds.value.includes(kb.id));
  const sharedList = orgStore.sharedKnowledgeBases || [];
  const sharedMapped = sharedList
    .filter((s: any) => s.knowledge_base != null && selectedKbIds.value.includes(s.knowledge_base.id))
    .map((s: any) => ({
      id: s.knowledge_base.id,
      name: s.knowledge_base.name,
      type: s.knowledge_base.type || "document",
      knowledge_count: s.knowledge_base.knowledge_count,
      chunk_count: s.knowledge_base.chunk_count,
      org_name: s.org_name || "",
    }));
  const ownIds = new Set(own.map((kb) => kb.id));
  const sharedOnly = sharedMapped.filter((kb: any) => !ownIds.has(kb.id));
  return [...own, ...sharedOnly];
});

const selectedFiles = computed(() => {
  // If we have file details in fileList, use them.
  // Otherwise we might show ID or Loading...
  return selectedFileIds.value.map((id: string) => {
    const found = fileList.value.find((f) => f.id === id);
    return found || { id, name: "Loading..." };
  });
});

// 合并所有选中项（用于输入框内显示）
const allSelectedItems = computed(() => {
  const allKbs = selectedKbs.value.map((kb) => ({
    ...kb,
    type: "kb" as const,
    kbType: kb.type,
  }));

  // 用户选择的文件（根据 fileIdToKbId + 共享列表补全 org_name，用于角标）
  const sharedKbOrgMap: Record<string, string> = {};
  (orgStore.sharedKnowledgeBases || []).forEach((s: any) => {
    if (s.knowledge_base?.id != null && s.org_name) {
      sharedKbOrgMap[String(s.knowledge_base.id)] = s.org_name;
    }
  });
  const files = selectedFiles.value.map((f: { id: string; name: string }) => {
    const kbId = fileIdToKbId.value[f.id];
    const org_name = kbId ? sharedKbOrgMap[String(kbId)] || "" : "";
    return {
      ...f,
      type: "file" as const,
      org_name,
    };
  });

  const tags = selectedTags.value.map((tag: any) => ({
    id: tag.id,
    name: tag.name,
    type: "tag" as const,
    kbId: tag.kbId,
    kbName: tag.kbName,
    description: tag.kbName || "",
  }));

  return [...allKbs, ...files, ...tags];
});

// 移除选中项
const removeSelectedItem = (item: MentionItem) => {
  if (item.type === "kb") {
    settingsStore.removeKnowledgeBase(item.id);
  } else if (item.type === "file") {
    settingsStore.removeFile(item.id);
    delete fileIdToKbId.value[item.id];
  } else if (item.type === "tag") {
    settingsStore.removeTag(item.id, item.kbId);
  }
};

const getMentionIcon = (item: MentionItem): Component => {
  switch (item.type) {
    case "kb":
      return item.kbType === "faq" ? MessageCircleQuestionMarkIcon : FolderIcon;
    case "file":
      return FileIcon;
    case "tag":
      return TagIcon;
    default:
      return FolderIcon;
  }
};

// A chip's surface stays neutral; its icon alone is tinted by resource type.
const getMentionIconColorClass = (item: MentionItem) => {
  if (item.type === "kb") return item.kbType === "faq" ? "text-[var(--yuheng-faq-color,#0052d9)]" : "text-primary";
  if (item.type === "file") return "text-muted-foreground";
  if (item.type === "tag") return "text-[#9f7aea]";
  return "";
};

/*
 * Class lists the control-bar buttons share. Each button is a plain <div>
 * (the old markup), so they are composed here instead of through <Button>.
 */
const controlBtnClass =
  "flex shrink-0 cursor-pointer items-center justify-center gap-1 rounded-[6px] px-2.5 py-1.5 text-muted-foreground transition-[background,color] duration-[120ms] select-none";
// The image and attachment buttons: a 28px square with a count badge. Once
// something is attached the tint replaces the hover state entirely.
const uploadBtnClass = "relative size-7 min-w-auto p-0";
const uploadBtnIdleClass =
  "text-muted-foreground hover:bg-[var(--td-bg-color-secondarycontainer-hover,#f0f0f0)] hover:text-foreground";
const uploadBtnActiveClass = "bg-[rgba(16,185,129,0.1)] text-[#07c05f]";
const uploadCountClass =
  "absolute -top-0.5 -right-0.5 flex size-3.5 items-center justify-center rounded-full bg-[#07c05f] text-[10px] leading-none text-white";
// The stop button draws a pulsing dot with ::before (keyframes in the style block).
const stopBtnClass =
  "relative size-7 border-[1.5px] border-solid border-[rgba(16,185,129,0.2)] bg-[rgba(16,185,129,0.08)] p-0 text-primary before:block before:size-3 before:animate-[inputFieldStopPulse_1.5s_ease-in-out_infinite] before:rounded-full before:bg-primary before:content-[''] hover:border-primary hover:bg-[rgba(16,185,129,0.12)] active:bg-[rgba(16,185,129,0.15)]";
// The control-bar hints were TDesign's light tooltips: a card-coloured bubble
// with a hairline border, and an arrow in the same colour.
const lightTooltipClass =
  "border-[0.5px] border-solid border-border bg-popover text-popover-foreground shadow-[var(--td-shadow-2)] [&>span>svg]:bg-popover [&>span>svg]:fill-popover";

// 使用 computed 从 store 读取，并通过 setter 同步回 store
const selectedModelId = computed({
  get: () => settingsStore.conversationModels.selectedChatModelId || "",
  set: (val: string) => settingsStore.updateConversationModels({ selectedChatModelId: val }),
});
const modelsLoading = ref(false);
const showModelSelector = ref(false);
const modelButtonRef = ref<HTMLElement>();
const modelDropdownStyle = ref<Record<string, string>>({});

// 显示的知识库标签（最多显示2个）

// 根据不同状态组合计算输入框的 placeholder
const inputPlaceholder = computed(() => {
  const hasKnowledge = allSelectedItems.value.length > 0;
  if (hasKnowledge) {
    // 有知识库 + 无网络搜索
    return t("input.placeholderWithContext");
  }
  // 无知识库（纯模型对话）
  return t("input.placeholder");
});

// 加载知识库列表（自己的 + 共享的，用于 @ 提及等）
const loadKnowledgeBases = async (force = false) => {
  try {
    await chatResources.ensureKnowledgeBases(force);
    const validKbs = knowledgeBases.value;

    const validKbIds = new Set(validKbs.map((kb: any) => kb.id));
    const sharedKbIds = new Set(
      (orgStore.sharedKnowledgeBases || []).map((s: any) => s.knowledge_base?.id).filter(Boolean),
    );
    const currentSelectedIds = settingsStore.settings.selectedKnowledgeBases || [];
    const validSelectedIds = currentSelectedIds.filter((id: string) => validKbIds.has(id) || sharedKbIds.has(id));

    if (validSelectedIds.length !== currentSelectedIds.length) {
      settingsStore.selectKnowledgeBases(validSelectedIds);
    }
  } catch (error) {
    console.error("Failed to load knowledge bases:", error);
  }
};

const loadFiles = async () => {
  const ids = selectedFileIds.value;
  if (ids.length === 0) return;

  const missingIds = ids.filter((id: string) => !fileList.value.find((f) => f.id === id));
  if (missingIds.length === 0) return;

  try {
    // 按 kb_id 分组：共享知识库下的文档需带 kb_id 才能正确查询
    const byKbId = new Map<string, string[]>();
    const noKbId: string[] = [];
    missingIds.forEach((id: string) => {
      const kbId = fileIdToKbId.value[id];
      if (kbId) {
        if (!byKbId.has(kbId)) byKbId.set(kbId, []);
        byKbId.get(kbId)!.push(id);
      } else {
        noKbId.push(id);
      }
    });

    const allNewFiles: Array<{ id: string; name: string }> = [];
    const runBatch = async (batchIds: string[], kbId?: string) => {
      const query = new URLSearchParams();
      batchIds.forEach((id: string) => query.append("ids", id));
      const res: any = await batchQueryKnowledge(query.toString(), kbId);
      if (res.data && Array.isArray(res.data)) {
        res.data.forEach((f: any) => allNewFiles.push({ id: f.id, name: f.title || f.file_name }));
      }
    };

    for (const [kbId, batchIds] of byKbId) {
      await runBatch(batchIds, kbId);
    }
    if (noKbId.length > 0) {
      await runBatch(noKbId);
    }
    if (allNewFiles.length > 0) {
      fileList.value = [...fileList.value, ...allNewFiles];
    }
  } catch (e) {
    console.error("Failed to load files", e);
  }
};

watch(
  selectedFileIds,
  () => {
    loadFiles();
  },
  { immediate: true },
);

// LAST_CHAT_MODEL_KEY scopes the per-user "last selected chat model"
// to localStorage. The previous implementation wrote this back to the
// tenant-level KV /tenants/kv/conversation-config — which (a) required
// Admin+ to mutate, so a Viewer/Contributor switching models in the
// chat input got a 403, and (b) silently overwrote the tenant default
// for everyone else. localStorage is per-user-per-browser, which is
// what "remember my last pick" actually wants.
const LAST_CHAT_MODEL_KEY = "yuheng_last_chat_model_id";

const readLastChatModelID = (): string => {
  try {
    return localStorage.getItem(LAST_CHAT_MODEL_KEY) || "";
  } catch {
    return "";
  }
};

const writeLastChatModelID = (id: string) => {
  try {
    if (id) {
      localStorage.setItem(LAST_CHAT_MODEL_KEY, id);
    } else {
      localStorage.removeItem(LAST_CHAT_MODEL_KEY);
    }
  } catch {
    // localStorage may be disabled in incognito mode; ignore.
  }
};

// Initial chat-model selection priority: per-user last pick
// (localStorage) > current store value (e.g. carried over from
// settings page) > first available model.
const initChatModelSelection = () => {
  const lastPick = readLastChatModelID();
  const currentSelectedModel = settingsStore.conversationModels.selectedChatModelId;
  const initialSelection = lastPick || currentSelectedModel || "";
  settingsStore.updateConversationModels({
    summaryModelId: initialSelection,
    selectedChatModelId: initialSelection,
    rerankModelId: "",
  });
  if (!selectedModelId.value) {
    selectedModelId.value = initialSelection;
  }
  ensureModelSelection();
};

const loadChatModels = async (force = false) => {
  if (modelsLoading.value) return;
  modelsLoading.value = true;
  try {
    await chatResources.ensureChatModels(force);
    ensureModelSelection();
  } catch (error) {
    console.error("Failed to load chat models:", error);
    chatResources.invalidate("models");
  } finally {
    modelsLoading.value = false;
  }
};

const ensureModelSelection = () => {
  if (selectedModelId.value) {
    return;
  }
  const lastPick = readLastChatModelID();
  if (lastPick) {
    selectedModelId.value = lastPick;
    return;
  }
  if (availableModels.value.length > 0) {
    selectedModelId.value = availableModels.value[0].id || "";
  }
};

const handleGoToConversationModels = () => {
  showModelSelector.value = false;
  router.push("/platform/settings");
  setTimeout(() => {
    const event = new CustomEvent("settings-nav", {
      detail: { section: "models", subsection: "chat" },
    });
    window.dispatchEvent(event);
  }, 100);
};

const handleModelChange = (value: string | number | Array<string | number> | undefined) => {
  const normalized = Array.isArray(value) ? value[0] : value;
  const val = normalized !== undefined && normalized !== null ? String(normalized) : "";

  if (!val) {
    selectedModelId.value = "";
    return;
  }
  if (val === "__add_model__") {
    selectedModelId.value = readLastChatModelID();
    handleGoToConversationModels();
    return;
  }

  // The chat-level model picker persists per-user-per-browser via
  // localStorage instead of writing to the tenant-shared KV.
  writeLastChatModelID(val);
  selectedModelId.value = val;
  showModelSelector.value = false;

  settingsStore.updateConversationModels({
    summaryModelId: val,
    selectedChatModelId: val,
    rerankModelId: "",
  });
};

const selectedModel = computed(() => {
  return availableModels.value.find((model) => model.id === selectedModelId.value);
});

const selectedModelDisplayName = computed(() => {
  if (selectedModel.value) return modelDisplayName(selectedModel.value);
  if (!selectedModelId.value) return t("input.notConfigured");
  return t("input.notConfigured");
});

const modelDisplayName = (model: ModelConfig) => {
  const displayName = model.display_name?.trim();
  return displayName || model.name;
};

const updateModelDropdownPosition = () => {
  const anchor = modelButtonRef.value;
  if (!anchor) {
    modelDropdownStyle.value = {
      position: "fixed",
      top: "50%",
      left: "50%",
      transform: "translate(-50%, -50%)",
    };
    return;
  }

  // Normalize coordinates to CSS pixels so they are interpreted the same way
  // the browser will render them under the root `zoom` (see utils/zoom.ts).
  const zoom = getRootZoom();
  const rect = rectToCssPx(anchor.getBoundingClientRect(), zoom);
  console.log("[Model Dropdown] Button rect:", {
    top: rect.top,
    bottom: rect.bottom,
    left: rect.left,
    right: rect.right,
    width: rect.width,
    height: rect.height,
  });

  const dropdownWidth = 280;
  const offsetY = 8;
  const { width: vw, height: vh } = cssViewportSize(zoom);

  // 左对齐到触发元素的左边缘
  // 使用 Math.floor 而不是 Math.round，避免像素对齐问题
  let left = Math.floor(rect.left);

  // 边界处理：不超出视口左右（留 16px margin）
  const minLeft = 16;
  const maxLeft = Math.max(16, vw - dropdownWidth - 16);
  left = Math.max(minLeft, Math.min(maxLeft, left));

  // 垂直定位：紧贴按钮，使用合理的高度避免空白
  const preferredDropdownHeight = 280; // 优选高度（紧凑且够用）
  const maxDropdownHeight = 360; // 最大高度
  const minDropdownHeight = 200; // 最小高度
  const topMargin = 20; // 顶部留白
  const spaceBelow = vh - rect.bottom; // 下方剩余空间
  const spaceAbove = rect.top; // 上方剩余空间

  console.log("[Model Dropdown] Space check:", {
    spaceBelow,
    spaceAbove,
    windowHeight: vh,
  });

  let actualHeight: number;
  let shouldOpenBelow: boolean;

  // 优先考虑下方空间
  if (spaceBelow >= minDropdownHeight + offsetY) {
    // 下方有足够空间，向下弹出
    actualHeight = Math.min(preferredDropdownHeight, spaceBelow - offsetY - 16);
    shouldOpenBelow = true;
    console.log("[Model Dropdown] Position: below button", { actualHeight });
  } else {
    // 向上弹出，优先使用 preferredHeight，必要时才扩展到 maxHeight
    const availableHeight = spaceAbove - offsetY - topMargin;
    if (availableHeight >= preferredDropdownHeight) {
      // 有足够空间显示优选高度
      actualHeight = preferredDropdownHeight;
    } else {
      // 空间不够，使用可用空间（但不小于最小高度）
      actualHeight = Math.max(minDropdownHeight, availableHeight);
    }
    shouldOpenBelow = false;
    console.log("[Model Dropdown] Position: above button", { actualHeight });
  }

  // 根据弹出方向使用不同的定位方式
  if (shouldOpenBelow) {
    // 向下弹出：使用 top 定位，左对齐
    const top = Math.floor(rect.bottom + offsetY);
    console.log("[Model Dropdown] Opening below, top:", top);
    modelDropdownStyle.value = {
      position: "fixed !important",
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      top: `${top}px`,
      maxHeight: `${actualHeight}px`,
      transform: "none !important",
      margin: "0 !important",
      padding: "0 !important",
    };
  } else {
    // 向上弹出：使用 bottom 定位，左对齐
    const bottom = vh - rect.top + offsetY;
    console.log("[Model Dropdown] Opening above, bottom:", bottom);
    modelDropdownStyle.value = {
      position: "fixed !important",
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      bottom: `${bottom}px`,
      maxHeight: `${actualHeight}px`,
      transform: "none !important",
      margin: "0 !important",
      padding: "0 !important",
    };
  }

  console.log("[Model Dropdown] Applied style:", modelDropdownStyle.value);
};

// Mention Logic
let lastMentionQuery = "";
const loadMentionItems = async (q: string, resetIndex = true, append = false) => {
  console.log("[Mention] loadMentionItems called with query:", q, "append:", append);

  if (!append) {
    mentionOffset.value = 0;
  }

  let kbItems: any[] = [];
  let tagItems: MentionItem[] = [];
  if (!append) {
    const availableKbs: any[] = [...knowledgeBases.value];
    const sharedList = orgStore.sharedKnowledgeBases || [];
    const sharedKbsForMention = sharedList
      .filter((s: any) => s.knowledge_base != null)
      .map((s: any) => ({
        id: s.knowledge_base.id,
        name: s.knowledge_base.name,
        type: s.knowledge_base.type || "document",
        knowledge_count: s.knowledge_base.knowledge_count,
        chunk_count: s.knowledge_base.chunk_count,
        org_name: s.org_name || "",
      }));
    const ownIds = new Set(availableKbs.map((kb: any) => kb.id));
    sharedKbsForMention.forEach((kb: any) => {
      if (!ownIds.has(kb.id)) {
        availableKbs.push(kb);
        ownIds.add(kb.id);
      }
    });

    const kbs = availableKbs.filter((kb: any) => !q || (kb.name && kb.name.toLowerCase().includes(q.toLowerCase())));
    kbItems = await Promise.all(
      kbs.map(async (kb: any) => {
        const kbType = kb.type || "document";
        let count = kbType === "faq" ? Number(kb.chunk_count || 0) : Number(kb.knowledge_count || 0);
        if (!count) {
          const detail = await chatResources.fetchKnowledgeBaseById(kb.id);
          if (detail) {
            count = detail.type === "faq" ? Number(detail.chunk_count || 0) : Number(detail.knowledge_count || 0);
          }
        }
        return {
          id: kb.id,
          name: kb.name,
          type: "kb" as const,
          kbType: kbType === "faq" ? ("faq" as const) : ("document" as const),
          count,
          orgName: kb.org_name || undefined,
        };
      }),
    );
    mentionGroupCounts.value.kb = kbItems.length;

    const tagKeyword = q.trim();
    const tagSources = availableKbs;
    try {
      const tagResults = await Promise.all(
        tagSources.map(async (kb: any) => {
          const res: any = await listKnowledgeTags(kb.id, { page: 1, page_size: 20, keyword: tagKeyword || undefined });
          const payload = res?.data ?? res;
          const list = Array.isArray(payload?.data) ? payload.data : Array.isArray(payload) ? payload : [];
          return list.map((tag: any) => ({
            id: tag.id,
            name: tag.name,
            type: "tag" as const,
            kbId: kb.id,
            kbName: kb.name,
          }));
        }),
      );
      tagItems = tagResults.flat();
      mentionGroupCounts.value.tag = tagItems.length;
    } catch (e) {
      console.error("[Mention] listKnowledgeTags error:", e);
      tagItems = [];
    }
  }

  // Fetch Files from API
  // 空关键词时显式请求最近文件；有关键词时返回匹配文件。
  // `recent=true` 只用于浏览态，避免其他搜索调用漏传关键词时静默退化为最近列表。
  let fileItems: any[] = [];
  const fileSearchKeyword = q.trim();
  mentionLoading.value = true;
  try {
    const res: any = await searchKnowledge(fileSearchKeyword, mentionOffset.value, MENTION_PAGE_SIZE, {
      recent: !fileSearchKeyword,
    });
    console.log("[Mention] searchKnowledge response:", res);
    if (res.data && Array.isArray(res.data)) {
      const files = res.data;
      const rawTotal = typeof res.total === "number" ? res.total : undefined;
      const apiPageSize = res.data.length;
      const sharedKbOrgMap: Record<string, string> = {};
      (orgStore.sharedKnowledgeBases || []).forEach((s: any) => {
        if (s.knowledge_base?.id != null && s.org_name) {
          sharedKbOrgMap[String(s.knowledge_base.id)] = s.org_name;
        }
      });
      fileItems = files.map((f: any) => {
        const kbId = f.knowledge_base_id ?? f.kb_id;
        const kbIdStr = kbId != null ? String(kbId) : "";
        const fileOrgName = kbIdStr ? sharedKbOrgMap[kbIdStr] : undefined;
        return {
          id: f.id,
          name: f.title || f.file_name,
          type: "file" as const,
          kbName: f.knowledge_base_name || "",
          kbId: kbId || undefined,
          orgName: fileOrgName || undefined,
        };
      });
      if (!append) {
        if (rawTotal != null) {
          mentionGroupCounts.value.file = rawTotal;
        } else {
          delete mentionGroupCounts.value.file;
        }
      }
    }
    mentionHasMore.value = res.has_more || false;
    mentionOffset.value += fileItems.length;
  } catch (e) {
    console.error("[Mention] searchKnowledge error:", e);
    mentionHasMore.value = false;
  } finally {
    mentionLoading.value = false;
  }

  if (append) {
    // Append file items to existing list
    mentionItems.value = [...mentionItems.value, ...fileItems];
  } else {
    mentionItems.value = [...kbItems, ...tagItems, ...fileItems];
  }
  console.log("[Mention] Total items:", mentionItems.value.length, {
    kbItems: kbItems.length,
    fileItems: fileItems.length,
    tagItems: tagItems.length,
  });

  // Only reset index if query changed or explicitly requested
  if (resetIndex || q !== lastMentionQuery) {
    mentionActiveIndex.value = 0;
  }
  // Ensure index is within bounds
  if (mentionActiveIndex.value >= mentionItems.value.length) {
    mentionActiveIndex.value = Math.max(0, mentionItems.value.length - 1);
  }
  lastMentionQuery = q;
};

const loadMoreMentionItems = () => {
  if (mentionHasMore.value && !mentionLoading.value) {
    loadMentionItems(lastMentionQuery, false, true);
  }
};

const getTextareaEl = () => textareaRef.value;

// Grow the textarea with its content, as TDesign's autosize did. The CSS
// min/max heights clamp the result, so past the maximum it scrolls. Resetting
// to auto first lets the box shrink again when text is deleted.
const autosize = () => {
  const el = getTextareaEl();
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${el.scrollHeight}px`;
};
// Immediate so text restored into the box before mount is sized too.
watch(query, () => nextTick(autosize), { immediate: true });

const onInput = (val: string | InputEvent) => {
  // 如果正在输入法组合中，不处理搜索逻辑，等待 compositionend
  if (isComposing.value) return;

  // Called with the native input event, or with the value itself from
  // onCompositionEnd; v-model has already written the event's text to query.
  const inputVal = typeof val === "string" ? val : query.value;

  const textarea = getTextareaEl();
  if (!textarea) {
    console.warn("[Mention] Could not get textarea element");
    return;
  }

  const cursor = textarea.selectionStart;
  const textBeforeCursor = inputVal.slice(0, cursor);

  console.log("[Mention] onInput called", { inputVal, cursor, textBeforeCursor, showMention: showMention.value });

  if (showMention.value) {
    // 如果不是按钮触发的，检查 @ 符号
    if (!isMentionTriggeredByButton.value) {
      if (!inputVal || inputVal.length <= mentionStartPos.value || inputVal.charAt(mentionStartPos.value) !== "@") {
        showMention.value = false;
        return;
      }
    }

    // 如果是按钮触发的，mentionStartPos 指向的是光标位置（即虚拟的 @ 位置前），所以实际上不应该往左删
    // 但如果用户删除了前面的内容导致长度变短，也需要处理
    if (cursor < mentionStartPos.value) {
      showMention.value = false;
      return;
    }

    // Get query
    // 如果是按钮触发，mentionStartPos 是起始位置，不需要 +1 跳过 @
    const start = isMentionTriggeredByButton.value ? mentionStartPos.value : mentionStartPos.value + 1;
    const q = inputVal.slice(start, cursor);

    if (q.includes(" ")) {
      showMention.value = false;
      return;
    }
    // Only reload if query changed
    if (q !== mentionQuery.value) {
      mentionQuery.value = q;
      loadMentionItems(q, true); // Reset index when query changes
    }
  } else {
    if (textBeforeCursor.endsWith("@")) {
      console.log("[Mention] @ detected, opening menu");
      isMentionTriggeredByButton.value = false;
      mentionStartPos.value = cursor - 1;
      showMention.value = true;
      mentionQuery.value = "";

      const coords = getCaretCoordinates(textarea, cursor);
      // Normalize coordinates to CSS pixels (root <html> may carry `zoom`).
      const zoom = getRootZoom();
      const rect = rectToCssPx(textarea.getBoundingClientRect(), zoom);
      const { width: vw, height: vh } = cssViewportSize(zoom);
      const scrollTop = textarea.scrollTop;
      const menuHeight = 320; // 预估最大高度

      let left = rect.left + coords.left;
      // Prevent menu from going off-screen horizontally
      if (left + 300 > vw) {
        left = vw - 300 - 10;
      }

      // 光标相对于视口的实际 top 位置（CSS 像素）
      const cursorAbsoluteTop = rect.top + coords.top - scrollTop;
      const lineHeight = coords.height; // 光标高度

      // Check vertical space below cursor
      const spaceBelow = vh - (cursorAbsoluteTop + lineHeight);

      if (spaceBelow < menuHeight && cursorAbsoluteTop > menuHeight) {
        // Show above cursor (using bottom positioning)
        const bottom = vh - cursorAbsoluteTop;
        mentionStyle.value = {
          left: `${left}px`,
          bottom: `${bottom}px`,
          top: "auto",
        };
      } else {
        // Show below cursor (using top positioning)
        const top = cursorAbsoluteTop + lineHeight;
        mentionStyle.value = {
          left: `${left}px`,
          top: `${top}px`,
          bottom: "auto",
        };
      }

      loadMentionItems("");
    }
  }
};

const onCompositionStart = () => {
  isComposing.value = true;
};

const onCompositionEnd = (e: CompositionEvent) => {
  isComposing.value = false;
  // 手动触发 onInput 逻辑
  // 注意：在 compositionend 时，v-model 可能还没更新，或者已经更新但我们需要用最新值
  // Waiting a tick lets v-model settle first.
  nextTick(() => {
    onInput(query.value);
  });
};

const triggerMention = () => {
  const textarea = getTextareaEl();
  if (!textarea) return;

  // 关闭其他选择器
  showModelSelector.value = false;

  textarea.focus();

  // 直接显示菜单，不插入 @
  showMention.value = true;
  isMentionTriggeredByButton.value = true;
  mentionQuery.value = "";
  mentionStartPos.value = textarea.selectionStart;

  // Normalize coordinates to CSS pixels (root <html> may carry `zoom`).
  const zoom = getRootZoom();
  const rect = rectToCssPx(textarea.getBoundingClientRect(), zoom);
  const { height: vh } = cssViewportSize(zoom);
  const menuHeight = 320;

  // 判断输入框上方空间
  const spaceAbove = rect.top;
  const spaceBelow = vh - rect.bottom;

  // 优先显示在上方，除非上方空间不足且下方空间充足
  if (spaceAbove > menuHeight || spaceAbove > spaceBelow) {
    // Show above textarea
    mentionStyle.value = {
      left: `${rect.left}px`,
      bottom: `${vh - rect.top + 8}px`, // 8px padding
      top: "auto",
    };
  } else {
    // Show below textarea
    mentionStyle.value = {
      left: `${rect.left}px`,
      top: `${rect.bottom + 8}px`,
      bottom: "auto",
    };
  }

  loadMentionItems("");
};

const onMentionSelect = (item: any) => {
  if (item.type === "kb") {
    settingsStore.addKnowledgeBase(item.id);
  } else if (item.type === "file") {
    settingsStore.addFile(item.id);
    if (item.kbId) {
      fileIdToKbId.value[item.id] = item.kbId;
      settingsStore.setFileKbMap({ [item.id]: item.kbId });
    }
    // Add to local cache immediately
    if (!fileList.value.find((f) => f.id === item.id)) {
      fileList.value.push({ id: item.id, name: item.name });
    }
  } else if (item.type === "tag") {
    if (item.kbId) {
      settingsStore.addTag({ id: item.id, name: item.name, kbId: item.kbId, kbName: item.kbName });
    }
  }

  const textarea = getTextareaEl();
  if (textarea) {
    // 如果是通过输入 @ 触发的，需要删除 @ 和后面的查询文字
    if (!isMentionTriggeredByButton.value) {
      const cursor = textarea.selectionStart;
      const textBeforeAt = query.value.slice(0, mentionStartPos.value);
      const textAfterCursor = query.value.slice(cursor);
      query.value = textBeforeAt + textAfterCursor;

      nextTick(() => {
        textarea.selectionStart = textarea.selectionEnd = mentionStartPos.value;
        textarea.focus();
      });
    } else {
      // 通过按钮触发的，如果用户输入了查询词，需要删除查询词
      const cursor = textarea.selectionStart;
      if (cursor > mentionStartPos.value) {
        const textBeforeStart = query.value.slice(0, mentionStartPos.value);
        const textAfterCursor = query.value.slice(cursor);
        query.value = textBeforeStart + textAfterCursor;

        nextTick(() => {
          textarea.selectionStart = textarea.selectionEnd = mentionStartPos.value;
          textarea.focus();
        });
      } else {
        // 直接聚焦
        textarea.focus();
      }
    }
  }

  showMention.value = false;
};

const toggleModelSelector = () => {
  // 互斥：关闭其他
  showMention.value = false;

  showModelSelector.value = !showModelSelector.value;
  if (showModelSelector.value) {
    if (!availableModels.value.length) {
      loadChatModels();
    }
    // 多次更新位置确保准确
    nextTick(() => {
      updateModelDropdownPosition();
      requestAnimationFrame(() => {
        updateModelDropdownPosition();
        setTimeout(() => {
          updateModelDropdownPosition();
        }, 50);
      });
    });
  }
};

const closeModelSelector = () => {
  showModelSelector.value = false;
};

const closeMentionSelector = (e: MouseEvent) => {
  const target = e.target as HTMLElement;
  // 如果点击的是输入框区域，不关闭 Mention 列表（由光标逻辑控制）
  if (target.closest(".rich-input-container")) {
    return;
  }
  showMention.value = false;
};

// 窗口事件处理器
let resizeHandler: (() => void) | null = null;
let scrollHandler: (() => void) | null = null;

onMounted(() => {
  // 并行拉取；若 platform 已预取且缓存未过期则直接复用
  initChatModelSelection();
  void Promise.all([loadKnowledgeBases(), loadChatModels()]);
  window.addEventListener(CHAT_FILE_DROP_EVENT, handleChatFileDrop as EventListener);

  // 从持久化恢复 fileId -> kbId，刷新后共享知识库文件可带 kb_id 拉取（仅保留当前仍选中的文件）
  const persisted = settingsStore.settings.selectedFileKbMap;
  const ids = settingsStore.settings.selectedFiles || [];
  if (persisted && typeof persisted === "object" && ids.length > 0) {
    const next: Record<string, string> = {};
    ids.forEach((id: string) => {
      if (persisted[id]) next[id] = persisted[id];
    });
    fileIdToKbId.value = next;
  }

  // 如果从知识库内部进入，自动选中该知识库
  const kbId = (route.params as any)?.kbId as string;
  if (kbId && !selectedKbIds.value.includes(kbId)) {
    settingsStore.addKnowledgeBase(kbId);
  }

  const prefill = menuStore.consumePrefillQuery();
  if (prefill) {
    query.value = prefill;
    nextTick(() => {
      const textarea = getTextareaEl();
      if (textarea) textarea.focus();
    });
  }

  // 监听点击外部关闭下拉菜单
  document.addEventListener("click", closeModelSelector);
  document.addEventListener("click", closeMentionSelector);

  // 监听窗口大小变化和滚动，重新计算位置
  resizeHandler = () => {
    if (showModelSelector.value) {
      updateModelDropdownPosition();
    }
  };
  scrollHandler = () => {
    if (showModelSelector.value) {
      updateModelDropdownPosition();
    }
  };

  window.addEventListener("resize", resizeHandler, { passive: true });
  window.addEventListener("scroll", scrollHandler, { passive: true, capture: true });
});

onUnmounted(() => {
  window.removeEventListener(CHAT_FILE_DROP_EVENT, handleChatFileDrop as EventListener);
  document.removeEventListener("click", closeModelSelector);
  document.removeEventListener("click", closeMentionSelector);
  if (resizeHandler) {
    window.removeEventListener("resize", resizeHandler);
  }
  if (scrollHandler) {
    window.removeEventListener("scroll", scrollHandler, { capture: true });
  }
});

// 监听路由变化
watch(
  () => route.params.kbId,
  (newKbId) => {
    if (newKbId && typeof newKbId === "string" && !selectedKbIds.value.includes(newKbId)) {
      settingsStore.addKnowledgeBase(newKbId);
    }
  },
);

watch(
  () => uiStore.showSettingsModal,
  (visible, prevVisible) => {
    if (prevVisible && !visible) {
      loadChatModels(true);
    }
  },
);

watch(
  [selectedKbIds, selectedFileIds],
  ([kbIds, fileIds]) => {
    if (!kbIds.length && !fileIds.length) {
      closeModelSelector();
    }
  },
  { deep: true },
);

const emit = defineEmits<{
  (
    e: "send-msg",
    query: string,
    modelId: string,
    mentionedItems: MentionRequestItem[],
    imageFiles: File[],
    attachmentFiles: AttachmentFile[],
  ): void;
  (e: "stop-generation"): void;
}>();

const createSession = async (val: string) => {
  if (!val.trim()) {
    MessagePlugin.info(t("input.messages.enterContent"));
    return;
  }
  if (props.isReplying) {
    return MessagePlugin.error(t("input.messages.replying"));
  }
  // Only block while the file is still uploading (no document ID yet). Once
  // uploaded, sending is allowed even if parsing is still in progress: the
  // backend shows a "parsing attachment" step on the timeline and waits.
  const pendingAttachment = uploadedAttachments.value.find((item) => item.status === "uploading");
  if (pendingAttachment) {
    MessagePlugin.warning(t("chat.attachmentStillProcessing", { name: pendingAttachment.name }));
    return;
  }
  const failedAttachment = uploadedAttachments.value.find((item) => item.status === "failed");
  if (failedAttachment) {
    MessagePlugin.error(failedAttachment.error || t("chat.attachmentParseFailed"));
    return;
  }

  // Images and attachments both travel to the backend as
  // `attachment_ids`, which enforces a combined cap (MaxTemporaryAttachmentsPerMessage).
  // The per-picker limits (5 images / 5 attachments) are independent, so guard the
  // merged total here to avoid a late 400 after the files are already uploaded.
  const MAX_TOTAL_ATTACHMENTS = 5;
  const combinedAttachmentCount =
    uploadedImages.value.length + uploadedAttachments.value.filter((item) => item.status !== "failed").length;
  if (combinedAttachmentCount > MAX_TOTAL_ATTACHMENTS) {
    MessagePlugin.warning(t("chat.attachmentTotalTooMany", { max: MAX_TOTAL_ATTACHMENTS }));
    return;
  }

  if (!chatResources.isFresh("models")) {
    await loadChatModels();
  }

  // 获取@提及的知识库和文件信息
  const mentionedItems: MentionRequestItem[] = allSelectedItems.value.map((item) => ({
    id: item.id,
    name: item.name,
    type: item.type,
    kb_type: item.type === "kb" ? item.kbType || "document" : undefined,
    kb_id: item.kbId,
    kb_name: item.kbName,
  }));
  const imageFiles = uploadedImages.value.map((img) => img.file);
  const attachmentFiles = uploadedAttachments.value;

  // Blur the textarea BEFORE emitting, so that when the parent navigates away
  // and Vue unmounts this component, TDesign's blur handler won't fire on a
  // detached DOM element (which causes getComputedStyle to throw). The native
  // textarea no longer has such a handler; the blur is kept because it also
  // drops the focus ring and caret before the view changes.
  const textarea = getTextareaEl();
  if (textarea) textarea.blur();
  emit("send-msg", val, selectedModelId.value, mentionedItems, imageFiles, attachmentFiles);

  // Clean up image previews
  uploadedImages.value.forEach((img) => URL.revokeObjectURL(img.preview));
  uploadedImages.value = [];

  // Clean up attachments
  attachmentUploadRef.value?.clear();
  uploadedAttachments.value = [];

  clearvalue();
};

const clearvalue = () => {
  // Guard: only clear when the textarea DOM element is still mounted; the
  // query watcher resizes the element, which must still exist.
  if (!getTextareaEl()) return;
  query.value = "";
};

// Drop any pending images/attachments and stop their status polling. Used when
// switching sessions: leftover documentIds belong to the previous session, so
// keeping them would make polling 404 (falsely marking them failed) or send IDs
// the new session does not own ("attachment ... not found in this session").
const clearPendingUploads = () => {
  uploadedImages.value.forEach((img) => URL.revokeObjectURL(img.preview));
  uploadedImages.value = [];
  attachmentUploadRef.value?.clear();
  uploadedAttachments.value = [];
};

const onKeydown = (e: KeyboardEvent) => {
  if (showMention.value) {
    if (e.keyCode === 38) {
      // Up
      e.preventDefault();
      mentionSelectorRef.value?.moveActive(-1);
      return;
    }
    if (e.keyCode === 40) {
      // Down
      e.preventDefault();
      mentionSelectorRef.value?.moveActive(1);
      return;
    }
    if (e.keyCode === 13) {
      // Enter
      e.preventDefault();
      mentionSelectorRef.value?.confirmActive();
      return;
    }
    if (e.keyCode === 27) {
      // Esc
      if (mentionSelectorRef.value?.leaveGroup()) {
        return;
      }
      showMention.value = false;
      return;
    }
  }

  // 退格键：当输入框为空且有选中项时，删除最后一个选中项
  if (e.keyCode === 8) {
    // Backspace
    const textarea = getTextareaEl();
    if (textarea && textarea.selectionStart === 0 && textarea.selectionEnd === 0 && query.value === "") {
      const items = allSelectedItems.value;
      if (items.length > 0) {
        e.preventDefault();
        const lastItem = items[items.length - 1];
        removeSelectedItem(lastItem);
        return;
      }
    }
  }

  if ((e.keyCode == 13 && e.shiftKey) || (e.keyCode == 13 && e.ctrlKey)) {
    return;
  }
  if (e.keyCode == 13) {
    e.preventDefault();
    // The native event carries no value (TDesign's handler passed it
    // first); v-model keeps query current.
    createSession(query.value);
  }
};

const onPaste = (e: ClipboardEvent) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  const imageFiles: File[] = [];
  for (const item of items) {
    if (item.type.startsWith("image/")) {
      const file = item.getAsFile();
      if (file) imageFiles.push(file);
    }
  }
  if (imageFiles.length > 0) {
    e.preventDefault();
    addImageFiles(imageFiles);
  }
};

const onDrop = (e: DragEvent) => {
  e.preventDefault();
  const files = e.dataTransfer?.files;
  if (!files || files.length === 0) return;
  handleDroppedFiles(Array.from(files));
};

const onDragOver = (e: DragEvent) => {
  e.preventDefault();
};

const handleStop = async () => {
  if (!props.sessionId) {
    MessagePlugin.warning(t("input.messages.sessionMissing"));
    return;
  }

  if (!props.assistantMessageId) {
    console.error("[Stop] Assistant message ID is empty");
    MessagePlugin.warning(t("input.messages.messageMissing"));
    return;
  }

  console.log("[Stop] Stopping generation for message:", props.assistantMessageId);

  // 发送 stop 事件，通知父组件立即清除 loading 状态
  emit("stop-generation");

  try {
    await stopSession(props.sessionId, props.assistantMessageId);
    MessagePlugin.success(t("input.messages.stopSuccess"));
  } catch (error) {
    console.error("Failed to stop session:", error);
    MessagePlugin.error(t("input.messages.stopFailed"));
  }
};

onBeforeRouteUpdate((to, from, next) => {
  clearvalue();
  clearPendingUploads();
  next();
});

defineExpose({
  triggerSend(text: string) {
    if (!text.trim()) return;
    query.value = text;
    nextTick(() => createSession(text));
  },
});
</script>
<template>
  <!--
    .answers-input is a hook: the chat and new-chat views reach it with
    :deep() to reset position/transform per breakpoint. Those overrides set
    `transform`, so the centring here uses the transform property too
    ([transform:…]) rather than Tailwind's translate utilities, which write
    the separate `translate` property and would survive the override.
  -->
  <div
    class="answers-input absolute bottom-[60px] left-1/2 z-[99] flex w-full [transform:translateX(-50%)] justify-center"
    @drop="onDrop"
    @dragover="onDragOver"
  >
    <!-- Hidden file input for image upload -->
    <input
      ref="imageInputRef"
      type="file"
      accept="image/jpeg,image/png,image/gif,image/webp"
      multiple
      class="hidden"
      @change="handleImageSelect"
    />
    <!-- 富文本输入框容器. .rich-input-container is a hook: closeMentionSelector() tests clicks against it. -->
    <div
      class="rich-input-container bg-card focus-within:border-primary relative w-full max-w-[960px] rounded-[12px] border border-solid border-[var(--td-component-stroke,#dcdcdc)] shadow-[0_2px_8px_rgba(0,0,0,0.04),0_8px_16px_-4px_rgba(0,0,0,0.06)]"
      data-guide="chat-input"
    >
      <!-- 图片预览区域 -->
      <div v-if="uploadedImages.length > 0" class="flex flex-wrap gap-2 px-3 pt-2 pb-1">
        <div
          v-for="(img, idx) in uploadedImages"
          :key="idx"
          class="relative size-[60px] overflow-hidden rounded-lg border border-solid border-[var(--td-border-level-1-color,#e7e7e7)]"
        >
          <img :src="img.preview" class="size-full object-cover" />
          <span
            class="absolute top-0.5 right-0.5 flex size-4 cursor-pointer items-center justify-center rounded-full bg-black/50 text-xs leading-none text-white hover:bg-black/70"
            @click="removeImage(idx)"
            >×</span
          >
        </div>
      </div>

      <!-- 附件列表区域 (由 AttachmentUpload 组件渲染) -->
      <AttachmentUpload
        ref="attachmentUploadRef"
        :max-files="5"
        :session-id="sessionId"
        @update:files="uploadedAttachments = $event"
      />

      <!-- 选中的知识库和文件标签（显示在输入框内顶部）. The top corners follow the container's inner radius (12px − 1px border). -->
      <div
        v-if="allSelectedItems.length > 0"
        class="bg-card flex flex-wrap items-center gap-[5px] rounded-t-[11px] border-b border-solid border-[var(--td-component-stroke,#dcdcdc)] px-3 py-1.5"
      >
        <!--
          The chip surface stays neutral; only the icon's colour tells the
          resource type apart (getMentionIconColorClass).
        -->
        <span
          v-for="item in allSelectedItems"
          :key="`${item.type}:${item.id}`"
          class="group/chip bg-secondary text-foreground box-border inline-flex min-h-[26px] cursor-default items-center gap-[5px] rounded-md border border-solid border-[var(--td-component-stroke)] py-[3px] pr-[7px] pl-1.5 text-xs leading-[18px] font-medium shadow-[inset_0_1px_0_color-mix(in_srgb,var(--td-bg-color-container)_72%,transparent)] transition-[background,border-color] duration-150 hover:border-[var(--td-component-border)] hover:bg-[var(--td-bg-color-secondarycontainer-hover)]"
        >
          <span
            class="relative inline-flex size-4 min-w-0 flex-[0_1_auto] items-center justify-center"
            :class="getMentionIconColorClass(item)"
          >
            <span class="flex items-center justify-center text-inherit">
              <component :is="getMentionIcon(item)" class="size-3" />
            </span>
            <span
              v-if="item.org_name"
              class="bg-secondary pointer-events-none absolute -right-px -bottom-px flex size-2 items-center justify-center rounded-full shadow-[0_0_0_1px_rgba(0,0,0,0.06)]"
            >
              <img
                :src="getImgSrc(item.type === 'file' ? 'organization-grey.svg' : 'organization-green.svg')"
                class="size-[5px] object-contain"
                alt=""
                aria-hidden="true"
              />
            </span>
          </span>
          <span class="max-w-[100px] truncate text-current" :title="item.name">{{ item.name }}</span>
          <span
            class="hover:text-foreground ml-px inline-flex size-3.5 shrink-0 cursor-pointer items-center justify-center rounded-full text-sm leading-none font-normal text-current opacity-50 transition-[opacity,background,color] duration-150 group-hover/chip:opacity-85 hover:bg-[var(--td-bg-color-component)]"
            @click.stop="removeSelectedItem(item)"
            :aria-label="$t('common.remove')"
            >×</span
          >
        </span>
      </div>

      <!--
        实际输入框. A native <textarea> rather than the ui Textarea: that one
        forwards its model through a deferred watcher, so query would still hold
        the previous text when onInput runs, and the @-mention detection reads
        it on that same event. autosize() grows the box with its content
        between the min and max heights, as TDesign's autosize did.
      -->
      <textarea
        ref="textareaRef"
        v-model="query"
        data-slot="textarea"
        :placeholder="inputPlaceholder"
        name="description"
        class="text-foreground placeholder:text-placeholder box-border block max-h-[200px] min-h-[120px] w-full resize-none border-none bg-transparent pr-4 pb-14 pl-4 font-(family-name:--app-font-family) text-base leading-6 font-normal shadow-none outline-none placeholder:font-(family-name:--app-font-family) placeholder:text-base placeholder:leading-6 placeholder:font-normal focus:border-none focus:shadow-none"
        :class="allSelectedItems.length > 0 ? 'rounded-b-[12px] pt-3' : 'rounded-[12px] pt-4'"
        @keydown="onKeydown"
        @input="onInput"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
        @paste="onPaste"
      />

      <!-- 控制栏（放在 rich-input-container 内，相对输入框边框定位） -->
      <div
        class="pointer-events-auto absolute right-4 bottom-3 left-4 z-10 flex max-h-14 flex-wrap items-center justify-between gap-2 bg-[linear-gradient(to_bottom,rgba(255,255,255,0)_0%,var(--td-bg-color-container,#fff)_40%,var(--td-bg-color-container,#fff)_100%)] pt-2"
      >
        <!-- 左侧控制按钮 -->
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          <!-- @ 知识库/文件选择按钮 -->
          <Tooltip>
            <TooltipTrigger as-child>
              <div
                ref="atButtonRef"
                data-guide="chat-kb-mention"
                :class="[
                  controlBtnClass,
                  'relative h-7 w-[30px] min-w-[30px] p-0 hover:bg-[var(--td-bg-color-secondarycontainer-hover)]',
                  {
                    'bg-secondary text-primary shadow-[inset_0_0_0_1px_var(--td-component-stroke)]':
                      allSelectedItems.length > 0,
                  },
                ]"
                @click.stop
                @mousedown.prevent="triggerMention"
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 20 20"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                  class="size-[18px]"
                >
                  <circle cx="10" cy="10" r="3.5" stroke="currentColor" stroke-width="1.8" />
                  <path
                    d="M13.5 10V11.5C13.5 12.163 13.7634 12.7989 14.2322 13.2678C14.7011 13.7366 15.337 14 16 14C16.663 14 17.2989 13.7366 17.7678 13.2678C18.2366 12.7989 18.5 12.163 18.5 11.5V10C18.5 7.74566 17.6045 5.58365 16.0104 3.98959C14.4163 2.39553 12.2543 1.5 10 1.5C7.74566 1.5 5.58365 2.39553 3.98959 3.98959C2.39553 5.58365 1.5 7.74566 1.5 10C1.5 12.2543 2.39553 14.4163 3.98959 16.0104C5.58365 17.6045 7.74566 18.5 10 18.5H12"
                    stroke="currentColor"
                    stroke-width="1.8"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
                <span
                  v-if="allSelectedItems.length > 0"
                  class="border-card bg-primary text-primary-foreground absolute -top-[5px] -right-[5px] box-content flex h-[15px] min-w-[15px] items-center justify-center rounded-full border-2 border-solid px-[3px] text-[9px] leading-[15px] font-semibold"
                  >{{ allSelectedItems.length }}</span
                >
              </div>
            </TooltipTrigger>
            <TooltipContent side="top" :class="lightTooltipClass">
              <span>{{
                allSelectedItems.length > 0
                  ? $t("input.knowledgeBaseWithCount", {
                      count: allSelectedItems.length,
                    })
                  : $t("input.knowledgeBase")
              }}</span>
            </TooltipContent>
          </Tooltip>

          <!-- 图片上传按钮 -->
          <Tooltip>
            <TooltipTrigger as-child>
              <div
                :class="[
                  controlBtnClass,
                  uploadBtnClass,
                  uploadedImages.length > 0 ? uploadBtnActiveClass : uploadBtnIdleClass,
                ]"
                @click.stop="triggerImageUpload()"
              >
                <svg width="18" height="18" viewBox="0 0 1024 1024" fill="currentColor" class="size-[18px]">
                  <path
                    d="M896 128H128c-35.3 0-64 28.7-64 64v640c0 35.3 28.7 64 64 64h768c35.3 0 64-28.7 64-64V192c0-35.3-28.7-64-64-64zM128 832V192h768l0.1 640H128z"
                  />
                  <path d="M352 448a96 96 0 1 0 0-192 96 96 0 0 0 0 192z" />
                  <path d="M128 768l224-288 160 160 192-256L896 640v128H128z" />
                </svg>
                <span v-if="uploadedImages.length > 0" :class="uploadCountClass">{{ uploadedImages.length }}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top" :class="lightTooltipClass">
              <span>{{ $t("chat.imageUploadTooltip") }}</span>
            </TooltipContent>
          </Tooltip>

          <!-- 附件上传按钮 -->
          <Tooltip>
            <TooltipTrigger as-child>
              <div
                :class="[
                  controlBtnClass,
                  uploadBtnClass,
                  uploadedAttachments.length > 0 ? uploadBtnActiveClass : uploadBtnIdleClass,
                ]"
                @click.stop="attachmentUploadRef?.triggerFileSelect()"
              >
                <!-- 回形针图标 -->
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  class="size-[18px]"
                >
                  <path
                    d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"
                  />
                </svg>
                <span v-if="uploadedAttachments.length > 0" :class="uploadCountClass">{{
                  uploadedAttachments.length
                }}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top" :class="lightTooltipClass">
              <span>{{
                uploadedAttachments.length > 0
                  ? $t("chat.attachmentWithCount", {
                      count: uploadedAttachments.length,
                    })
                  : $t("chat.attachmentUploadTooltip")
              }}</span>
            </TooltipContent>
          </Tooltip>

          <!-- 模型显示 -->
          <div class="ml-auto flex shrink-0 items-center">
            <div
              ref="modelButtonRef"
              class="border-border flex h-[22px] min-w-[100px] cursor-pointer items-center gap-1.5 rounded-[6px] border-[0.5px] border-solid px-2 py-0.5 transition-[background,border-color] duration-[120ms] hover:bg-[var(--td-bg-color-secondarycontainer-hover,#e6e6e6)]"
              @click.stop="toggleModelSelector"
            >
              <span class="text-muted-foreground flex-1 truncate text-xs font-medium">
                {{ selectedModelDisplayName }}
              </span>
              <svg
                width="12"
                height="12"
                viewBox="0 0 12 12"
                fill="currentColor"
                class="text-placeholder size-2.5 shrink-0 transition-transform duration-[120ms]"
                :class="{ 'rotate-180': showModelSelector }"
              >
                <path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" />
              </svg>
            </div>
          </div>
        </div>

        <Teleport to="body">
          <div
            v-if="showModelSelector"
            class="fixed inset-0 z-[9999] touch-none bg-transparent"
            @click="closeModelSelector"
          >
            <!--
              Positioned by JS (modelDropdownStyle). The transform is pinned to
              none, as before, so only the opacity of the entrance animates.
            -->
            <div
              class="animate-in border-border bg-card fade-in-0 fixed! z-[10000] m-0! flex origin-top-left transform-none! flex-col overflow-hidden rounded-[10px] border-[0.5px] border-solid p-0! shadow-[var(--td-shadow-2)] duration-150 ease-out"
              :style="modelDropdownStyle"
              @click.stop
            >
              <div
                class="bg-card text-muted-foreground flex items-center justify-between border-b-[0.5px] border-solid border-[var(--td-component-stroke)] px-2.5 py-2 text-xs font-medium"
              >
                <span>{{ $t("conversationSettings.models.chatGroupLabel") }}</span>
                <button
                  type="button"
                  data-slot="model-selector-add"
                  class="text-primary hover:bg-secondary inline-flex cursor-pointer items-center gap-1 rounded-[6px] border-[0.5px] border-solid border-transparent bg-transparent px-2 py-0.5 text-xs font-medium transition-all duration-[120ms] hover:text-[var(--td-brand-color-hover)]"
                  @click="handleModelChange('__add_model__')"
                >
                  <span class="text-sm leading-none font-normal">+</span>
                  <span>{{ $t("input.addModel") }}</span>
                </button>
              </div>
              <div
                class="max-h-[260px] min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 py-1.5 [-webkit-overflow-scrolling:touch]"
              >
                <div
                  v-for="model in availableModels"
                  :key="model.id"
                  class="hover:bg-secondary mb-1 flex cursor-pointer items-center rounded-[6px] px-2 py-1.5 transition-[background] duration-[120ms] last:mb-0"
                  :class="{ 'bg-secondary': model.id === selectedModelId }"
                  @click="handleModelChange(model.id || '')"
                >
                  <div class="flex w-full min-w-0 items-center gap-2">
                    <div class="text-muted-foreground flex size-4 shrink-0 items-center justify-center">
                      <MessageSquareIcon class="size-3.5" />
                    </div>
                    <div class="flex min-w-0 flex-1 items-center gap-1">
                      <span class="text-foreground truncate text-xs leading-[1.4]">{{ modelDisplayName(model) }}</span>
                      <span v-if="model.display_name" class="text-placeholder shrink-0 text-[11px]">{{
                        model.name
                      }}</span>
                    </div>
                  </div>
                </div>
                <div
                  v-if="availableModels.length === 0"
                  class="text-placeholder mb-1 flex cursor-default items-center justify-center rounded-[6px] px-2 py-5 text-center last:mb-0"
                >
                  {{ $t("input.noModel") }}
                </div>
              </div>
            </div>
          </div>
        </Teleport>

        <!-- 右侧控制按钮组 -->
        <div class="flex items-center gap-2">
          <!--
            停止按钮（仅在回复中时显示）. The glyph is a pulsing dot drawn by
            ::before; the square svg is kept but hidden, as before.
          -->
          <Tooltip v-if="isReplying">
            <TooltipTrigger as-child>
              <div :class="[controlBtnClass, stopBtnClass]" @click="handleStop">
                <svg class="hidden" width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                  <rect x="5" y="5" width="6" height="6" rx="1" />
                </svg>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top">{{ $t("input.stopGeneration") }}</TooltipContent>
          </Tooltip>

          <!-- 发送按钮. When disabled it keeps the old hover quirk: the neutral hover fill wins over the tint. -->
          <div
            v-if="!isReplying"
            @click="createSession(query)"
            data-guide="chat-send"
            :class="[
              controlBtnClass,
              'size-7 p-0',
              query.length
                ? 'bg-primary hover:bg-[var(--td-brand-color-active)]'
                : 'cursor-not-allowed bg-[var(--td-success-color-light)] opacity-50 hover:bg-[var(--td-bg-color-secondarycontainer,#f5f5f5)]',
            ]"
          >
            <img src="../assets/img/sending-aircraft.svg" class="size-4" :alt="$t('input.send')" />
          </div>
        </div>
      </div>
    </div>

    <!-- Mention Selector -->
    <Teleport to="body">
      <MentionSelector
        ref="mentionSelectorRef"
        :visible="showMention"
        :style="mentionStyle"
        :items="mentionItems"
        :hasMore="mentionHasMore"
        :loading="mentionLoading"
        :emptyHint="''"
        :query="mentionQuery"
        :group-counts="mentionGroupCounts"
        v-model:activeIndex="mentionActiveIndex"
        @select="onMentionSelect"
        @loadMore="loadMoreMentionItems"
      />
    </Teleport>
  </div>
</template>
<script lang="ts">
const getImgSrc = (url: string) => {
  return new URL(`/src/assets/img/${url}`, import.meta.url).href;
};
</script>
<style>
/*
 * Kept as CSS because a keyframes rule cannot be a utility: the stop button's
 * pulsing dot is animated by it (before:animate-[inputFieldStopPulse_…]).
 * Unscoped on purpose: a scoped block would rename the keyframes with a hash
 * that the utility class never sees. The component-specific name keeps it
 * from clashing with anything global.
 */
@keyframes inputFieldStopPulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(0.75);
    opacity: 0.6;
  }
}
</style>
