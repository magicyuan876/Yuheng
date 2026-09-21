<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from "vue";
import { storeToRefs } from "pinia";
import { useRoute, useRouter } from "vue-router";
import { onBeforeRouteUpdate } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
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
const textareaRef = ref<any>(null); // Ref to t-textarea component
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

const getMentionIcon = (item: MentionItem) => {
  switch (item.type) {
    case "file":
      return "file";
    case "tag":
      return "tag";
    default:
      return "folder";
  }
};

const getMentionChipClass = (item: MentionItem) => {
  if (item.type === "kb") return item.kbType === "faq" ? "mention-chip--faq" : "mention-chip--kb";
  return `mention-chip--${item.type}`;
};

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

const getTextareaEl = () => {
  if (!textareaRef.value) return null;
  // If it's a native element
  if (textareaRef.value instanceof HTMLTextAreaElement) return textareaRef.value;
  // If it's a component wrapper
  const el = textareaRef.value.$el || textareaRef.value;
  if (!el) return null;
  if (el.tagName === "TEXTAREA") return el as HTMLTextAreaElement;
  return el.querySelector("textarea");
};

const onInput = (val: string | InputEvent) => {
  // 如果正在输入法组合中，不处理搜索逻辑，等待 compositionend
  if (isComposing.value) return;

  // TDesign t-textarea passes the value directly, not an event
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
  // TDesign textarea 可能需要 nextTick
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
  // detached DOM element (which causes getComputedStyle to throw).
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
  // Guard: only clear when the textarea DOM element is still mounted,
  // otherwise TDesign's autosize will call getComputedStyle on a non-Element.
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

const onKeydown = (
  val: string,
  event: { e: { preventDefault(): unknown; keyCode: number; shiftKey: any; ctrlKey: any } },
) => {
  if (showMention.value) {
    if (event.e.keyCode === 38) {
      // Up
      event.e.preventDefault();
      mentionSelectorRef.value?.moveActive(-1);
      return;
    }
    if (event.e.keyCode === 40) {
      // Down
      event.e.preventDefault();
      mentionSelectorRef.value?.moveActive(1);
      return;
    }
    if (event.e.keyCode === 13) {
      // Enter
      event.e.preventDefault();
      mentionSelectorRef.value?.confirmActive();
      return;
    }
    if (event.e.keyCode === 27) {
      // Esc
      if (mentionSelectorRef.value?.leaveGroup()) {
        return;
      }
      showMention.value = false;
      return;
    }
  }

  // 退格键：当输入框为空且有选中项时，删除最后一个选中项
  if (event.e.keyCode === 8) {
    // Backspace
    const textarea = getTextareaEl();
    if (textarea && textarea.selectionStart === 0 && textarea.selectionEnd === 0 && query.value === "") {
      const items = allSelectedItems.value;
      if (items.length > 0) {
        event.e.preventDefault();
        const lastItem = items[items.length - 1];
        removeSelectedItem(lastItem);
        return;
      }
    }
  }

  if ((event.e.keyCode == 13 && event.e.shiftKey) || (event.e.keyCode == 13 && event.e.ctrlKey)) {
    return;
  }
  if (event.e.keyCode == 13) {
    event.e.preventDefault();
    createSession(val);
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
  <div class="answers-input" @drop="onDrop" @dragover="onDragOver">
    <!-- Hidden file input for image upload -->
    <input
      ref="imageInputRef"
      type="file"
      accept="image/jpeg,image/png,image/gif,image/webp"
      multiple
      style="display: none"
      @change="handleImageSelect"
    />
    <!-- 富文本输入框容器 -->
    <div class="rich-input-container" data-guide="chat-input">
      <!-- 图片预览区域 -->
      <div v-if="uploadedImages.length > 0" class="image-preview-bar">
        <div v-for="(img, idx) in uploadedImages" :key="idx" class="image-preview-item">
          <img :src="img.preview" class="image-preview-thumb" />
          <span class="image-preview-remove" @click="removeImage(idx)">×</span>
        </div>
      </div>

      <!-- 附件列表区域 (由 AttachmentUpload 组件渲染) -->
      <AttachmentUpload
        ref="attachmentUploadRef"
        :max-files="5"
        :session-id="sessionId"
        @update:files="uploadedAttachments = $event"
      />

      <!-- 选中的知识库和文件标签（显示在输入框内顶部） -->
      <div v-if="allSelectedItems.length > 0" class="selected-tags-inline">
        <span
          v-for="item in allSelectedItems"
          :key="`${item.type}:${item.id}`"
          class="mention-chip"
          :class="[getMentionChipClass(item)]"
        >
          <span class="mention-chip__icon-wrap" :class="{ 'has-org': item.org_name }">
            <span class="mention-chip__icon">
              <t-icon v-if="item.type === 'kb'" :name="item.kbType === 'faq' ? 'chat-bubble-help' : 'folder'" />
              <t-icon v-else :name="getMentionIcon(item)" />
            </span>
            <span v-if="item.org_name" class="mention-chip__org-badge">
              <img
                :src="getImgSrc(item.type === 'file' ? 'organization-grey.svg' : 'organization-green.svg')"
                class="mention-chip__org-img"
                alt=""
                aria-hidden="true"
              />
            </span>
          </span>
          <span class="mention-chip__name" :title="item.name">{{ item.name }}</span>
          <span class="mention-chip__remove" @click.stop="removeSelectedItem(item)" :aria-label="$t('common.remove')"
            >×</span
          >
        </span>
      </div>

      <!-- 实际输入框 -->
      <t-textarea
        ref="textareaRef"
        v-model="query"
        :placeholder="inputPlaceholder"
        name="description"
        :autosize="true"
        @keydown="onKeydown"
        @input="onInput"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
        @paste="onPaste"
      />

      <!-- 控制栏（放在 rich-input-container 内，相对输入框边框定位） -->
      <div class="control-bar">
        <!-- 左侧控制按钮 -->
        <div class="control-left">
          <!-- @ 知识库/文件选择按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <span>{{
                allSelectedItems.length > 0
                  ? $t("input.knowledgeBaseWithCount", {
                      count: allSelectedItems.length,
                    })
                  : $t("input.knowledgeBase")
              }}</span>
            </template>
            <div
              ref="atButtonRef"
              class="control-btn kb-btn"
              data-guide="chat-kb-mention"
              :class="{
                active: allSelectedItems.length > 0,
              }"
              @click.stop
              @mousedown.prevent="triggerMention"
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 20 20"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
                class="control-icon at-icon"
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
              <span v-if="allSelectedItems.length > 0" class="kb-count">{{ allSelectedItems.length }}</span>
            </div>
          </t-tooltip>

          <!-- 图片上传按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <span>{{ $t("chat.imageUploadTooltip") }}</span>
            </template>
            <div
              class="control-btn image-upload-btn"
              :class="{
                active: uploadedImages.length > 0,
              }"
              @click.stop="triggerImageUpload()"
            >
              <svg width="18" height="18" viewBox="0 0 1024 1024" fill="currentColor" class="control-icon">
                <path
                  d="M896 128H128c-35.3 0-64 28.7-64 64v640c0 35.3 28.7 64 64 64h768c35.3 0 64-28.7 64-64V192c0-35.3-28.7-64-64-64zM128 832V192h768l0.1 640H128z"
                />
                <path d="M352 448a96 96 0 1 0 0-192 96 96 0 0 0 0 192z" />
                <path d="M128 768l224-288 160 160 192-256L896 640v128H128z" />
              </svg>
              <span v-if="uploadedImages.length > 0" class="image-count">{{ uploadedImages.length }}</span>
            </div>
          </t-tooltip>

          <!-- 附件上传按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <span>{{
                uploadedAttachments.length > 0
                  ? $t("chat.attachmentWithCount", {
                      count: uploadedAttachments.length,
                    })
                  : $t("chat.attachmentUploadTooltip")
              }}</span>
            </template>
            <div
              class="control-btn attachment-upload-btn"
              :class="{ active: uploadedAttachments.length > 0 }"
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
                class="control-icon"
              >
                <path
                  d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"
                />
              </svg>
              <span v-if="uploadedAttachments.length > 0" class="attachment-count">{{
                uploadedAttachments.length
              }}</span>
            </div>
          </t-tooltip>

          <!-- 模型显示 -->
          <div class="model-display">
            <div ref="modelButtonRef" class="model-selector-trigger" @click.stop="toggleModelSelector">
              <span class="model-selector-name">
                {{ selectedModelDisplayName }}
              </span>
              <svg
                width="12"
                height="12"
                viewBox="0 0 12 12"
                fill="currentColor"
                class="model-dropdown-arrow"
                :class="{ rotate: showModelSelector }"
              >
                <path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" />
              </svg>
            </div>
          </div>
        </div>

        <Teleport to="body">
          <div v-if="showModelSelector" class="model-selector-overlay" @click="closeModelSelector">
            <div class="model-selector-dropdown" :style="modelDropdownStyle" @click.stop>
              <div class="model-selector-header">
                <span>{{ $t("conversationSettings.models.chatGroupLabel") }}</span>
                <button class="model-selector-add" type="button" @click="handleModelChange('__add_model__')">
                  <span class="add-icon">+</span>
                  <span class="add-text">{{ $t("input.addModel") }}</span>
                </button>
              </div>
              <div class="model-selector-content">
                <div
                  v-for="model in availableModels"
                  :key="model.id"
                  class="model-option"
                  :class="{ selected: model.id === selectedModelId }"
                  @click="handleModelChange(model.id || '')"
                >
                  <div class="model-option-left">
                    <div class="model-option-icon">
                      <t-icon name="chat" size="14px" />
                    </div>
                    <div class="model-option-name-wrap">
                      <span class="model-option-name">{{ modelDisplayName(model) }}</span>
                      <span v-if="model.display_name" class="model-option-raw-name">{{ model.name }}</span>
                    </div>
                  </div>
                </div>
                <div v-if="availableModels.length === 0" class="model-option empty">
                  {{ $t("input.noModel") }}
                </div>
              </div>
            </div>
          </div>
        </Teleport>

        <!-- 右侧控制按钮组 -->
        <div class="control-right">
          <!-- 停止按钮（仅在回复中时显示） -->
          <t-tooltip v-if="isReplying" :content="$t('input.stopGeneration')" placement="top">
            <div @click="handleStop" class="control-btn stop-btn">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <rect x="5" y="5" width="6" height="6" rx="1" />
              </svg>
            </div>
          </t-tooltip>

          <!-- 发送按钮 -->
          <div
            v-if="!isReplying"
            @click="createSession(query)"
            class="control-btn send-btn"
            data-guide="chat-send"
            :class="{ disabled: !query.length }"
          >
            <img src="../assets/img/sending-aircraft.svg" :alt="$t('input.send')" />
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
<style scoped lang="less">
@import "./css/chat-resource-chips.less";

.answers-input {
  position: absolute;
  z-index: 99;
  bottom: 60px;
  left: 50%;
  transform: translateX(-50%);
  width: 100%;
  display: flex;
  justify-content: center;
}

/* 富文本输入框容器 */
.rich-input-container {
  position: relative;
  width: 100%;
  max-width: 960px;
  background: var(--td-bg-color-container, #fff);
  border-radius: 12px;
  border: 1px solid var(--td-component-stroke, #dcdcdc);
  box-shadow:
    0 2px 8px rgba(0, 0, 0, 0.04),
    0 8px 16px -4px rgba(0, 0, 0, 0.06);

  &:focus-within {
    border-color: var(--td-brand-color, #07c05f);
  }
}

/* 选中的知识库/文件标签（mention list 已选项） */
.selected-tags-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  padding: 6px 12px 6px;
  border-bottom: 1px solid var(--td-component-stroke, #dcdcdc);
  background: var(--td-bg-color-container, #fff);
  border-radius: 11px 11px 0 0;
  /* 与 .rich-input-container 内缘上边圆角一致（12px - 1px 边框） */
}

.mention-chip {
  .chat-resource-chip-surface();

  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 26px;
  padding: 3px 7px 3px 6px;
  border-radius: var(--td-radius-medium, 6px);
  box-sizing: border-box;
  font-size: 12px;
  font-weight: 500;
  cursor: default;
  transition:
    background 0.15s,
    border-color 0.15s;
  line-height: 18px;

  &:hover {
    .chat-resource-chip-hover();
  }
}

.mention-chip__icon-wrap {
  position: relative;
  display: inline-flex;
  width: 16px;
  height: 16px;
  flex: 0 1 auto;
  min-width: 0;
  align-items: center;
  justify-content: center;
}

.mention-chip__icon {
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: inherit;
}

.mention-chip__org-badge {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--td-bg-color-secondarycontainer, #f0f2f5);
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.mention-chip__org-img {
  width: 5px;
  height: 5px;
  object-fit: contain;
}

.mention-chip__name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: currentColor;
}

.mention-chip__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  margin-left: 1px;
  border-radius: 50%;
  font-size: 14px;
  line-height: 1;
  font-weight: 400;
  cursor: pointer;
  opacity: 0.5;
  transition:
    opacity 0.15s,
    background 0.15s,
    color 0.15s;
  color: currentColor;
  flex-shrink: 0;
}

.mention-chip:hover .mention-chip__remove {
  opacity: 0.85;
}

.mention-chip__remove:hover {
  opacity: 1;
  background: var(--td-bg-color-component);
  color: var(--td-text-color-primary, #1f2937);
}

/* 标签表面保持中性，仅用图标颜色表达资源类型。 */
.mention-chip--kb {
  color: var(--td-text-color-primary);
}

.mention-chip--kb .mention-chip__icon-wrap {
  color: var(--td-brand-color, #07c05f);
}

.mention-chip--faq {
  color: var(--td-text-color-primary);
}

.mention-chip--faq .mention-chip__icon-wrap {
  color: var(--yuheng-faq-color, #0052d9);
}

.mention-chip--file {
  color: var(--td-text-color-primary);
}

.mention-chip--file .mention-chip__icon-wrap {
  color: var(--td-text-color-secondary, #6b7280);
}

.mention-chip--tag {
  color: var(--td-text-color-primary);
}

.mention-chip--tag .mention-chip__icon-wrap {
  color: #9f7aea;
}

:deep(.t-textarea__inner) {
  width: 100%;
  max-height: 200px !important;
  min-height: 120px !important;
  resize: none;
  color: var(--td-text-color-primary, #000000e6);
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  font-family: var(--app-font-family);
  padding: 12px 16px 56px 16px;
  border-radius: 0 0 12px 12px;
  border: none;
  box-sizing: border-box;
  background: transparent;
  box-shadow: none;

  &:focus {
    border: none;
    box-shadow: none;
  }

  &::placeholder {
    color: var(--td-text-color-placeholder, #00000066);
    font-family: var(--app-font-family);
    font-size: 16px;
    font-weight: 400;
    line-height: 24px;
  }
}

/* 当没有选中标签时，textarea 样式 */
.rich-input-container:not(:has(.selected-tags-inline)) :deep(.t-textarea__inner) {
  border-radius: 12px;
  padding-top: 16px;
}

/* 控制栏 */
.control-bar {
  position: absolute;
  bottom: 12px;
  left: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
  max-height: 56px;
  z-index: 10;
  background: linear-gradient(
    to bottom,
    rgba(255, 255, 255, 0) 0%,
    var(--td-bg-color-container, #fff) 40%,
    var(--td-bg-color-container, #fff) 100%
  );
  pointer-events: auto;
  padding-top: 8px;
}

.control-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  flex-wrap: wrap;
  min-width: 0;
}

.control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 10px;
  border-radius: 6px;
  color: var(--td-text-color-secondary, #666);
  cursor: pointer;
  transition:
    background 0.12s,
    color 0.12s;
  user-select: none;
  flex-shrink: 0;

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #e6e6e6);
  }

  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;

    &:hover {
      background: var(--td-bg-color-secondarycontainer, #f5f5f5);
    }
  }
}

.control-icon {
  width: 18px;
  height: 18px;
}

.kb-btn {
  height: 28px;
  width: 30px;
  padding: 0;
  min-width: 30px;
  position: relative;

  &.active {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-brand-color);
    box-shadow: inset 0 0 0 1px var(--td-component-stroke);

    &:hover {
      background: var(--td-bg-color-secondarycontainer-hover);
    }
  }
}

.kb-count {
  position: absolute;
  top: -5px;
  right: -5px;
  min-width: 15px;
  height: 15px;
  padding: 0 3px;
  background: var(--td-brand-color);
  color: var(--td-text-color-anti, #fff);
  font-size: 9px;
  font-weight: 600;
  line-height: 15px;
  border: 2px solid var(--td-bg-color-container);
  border-radius: var(--td-radius-round, 999px);
  box-sizing: content-box;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Image upload */
.image-upload-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  min-width: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: var(--td-text-color-secondary, #666);

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #f0f0f0);
    color: var(--td-text-color-primary, #333);
  }

  &.active {
    background: rgba(16, 185, 129, 0.1);
    color: #07c05f;
  }

  .image-count {
    position: absolute;
    top: -2px;
    right: -2px;
    background: #07c05f;
    color: #fff;
    font-size: 10px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }
}

/* Attachment upload */
.attachment-upload-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  min-width: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: var(--td-text-color-secondary, #666);

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #f0f0f0);
    color: var(--td-text-color-primary, #333);
  }

  &.active {
    background: rgba(16, 185, 129, 0.1);
    color: #07c05f;
  }

  .attachment-count {
    position: absolute;
    top: -2px;
    right: -2px;
    background: #07c05f;
    color: #fff;
    font-size: 10px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }
}

.image-preview-bar {
  display: flex;
  gap: 8px;
  padding: 8px 12px 4px;
  flex-wrap: wrap;
}

.image-preview-item {
  position: relative;
  width: 60px;
  height: 60px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--td-border-level-1-color, #e7e7e7);

  .image-preview-thumb {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .image-preview-remove {
    position: absolute;
    top: 2px;
    right: 2px;
    width: 16px;
    height: 16px;
    background: rgba(0, 0, 0, 0.5);
    color: #fff;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    cursor: pointer;
    line-height: 1;

    &:hover {
      background: rgba(0, 0, 0, 0.7);
    }
  }
}

:global(.input-field-tooltip) {
  .t-popup__content {
    box-shadow: var(--td-shadow-2);
    border: 0.5px solid var(--td-component-border, #e7e7e7);
  }
}

.model-dropdown-arrow {
  width: 10px;
  height: 10px;
  color: var(--td-text-color-placeholder, #999);
  flex-shrink: 0;
  transition: transform 0.12s;

  &.rotate {
    transform: rotate(180deg);
  }
}

.control-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stop-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  background: rgba(16, 185, 129, 0.08);
  color: var(--td-brand-color);
  border: 1.5px solid rgba(16, 185, 129, 0.2);
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    background: rgba(16, 185, 129, 0.12);
    border-color: var(--td-brand-color);
  }

  &:active {
    background: rgba(16, 185, 129, 0.15);
  }

  svg {
    display: none;
  }

  &::before {
    content: "";
    width: 12px;
    height: 12px;
    background: var(--td-brand-color);
    border-radius: 50%;
    display: block;
    animation: stopBtnPulse 1.5s ease-in-out infinite;
  }
}

@keyframes stopBtnPulse {
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

.send-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  background-color: var(--td-brand-color);

  &:hover:not(.disabled) {
    background-color: var(--td-brand-color-active);
  }

  &.disabled {
    background-color: var(--td-success-color-light);
  }

  img {
    width: 16px;
    height: 16px;
  }
}

/* 模型显示样式 */
.model-display {
  display: flex;
  align-items: center;
  margin-left: auto;
  flex-shrink: 0;
}

.model-selector-trigger {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  min-width: 100px;
  height: 22px;
  border-radius: 6px;
  border: 0.5px solid var(--td-component-border, #e7e7e7);
  transition:
    background 0.12s,
    border-color 0.12s;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #e6e6e6);
  }
}

.model-selector-name {
  flex: 1;
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-secondary, #666);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-selector-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: transparent;
  touch-action: none;
}

.model-selector-dropdown {
  position: fixed !important;
  z-index: 10000;
  background: var(--td-bg-color-container);
  border: 0.5px solid var(--td-component-border);
  border-radius: 10px;
  box-shadow: var(--td-shadow-2);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  margin: 0 !important;
  padding: 0 !important;
  transform: none !important;
  transform-origin: top left;
  animation: modelSelectorFadeIn 0.15s ease-out;
}

@keyframes modelSelectorFadeIn {
  from {
    opacity: 0;
    transform: scale(0.98);
  }

  to {
    opacity: 1;
    transform: scale(1);
  }
}

.model-selector-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 0.5px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
}

.model-selector-content {
  flex: 1;
  min-height: 0;
  max-height: 260px;
  overflow-y: auto;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
  padding: 6px 8px;
}

.model-selector-add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 6px;
  border: 0.5px solid transparent;
  background: transparent;
  color: var(--td-brand-color);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.12s;

  .add-icon {
    font-size: 14px;
    line-height: 1;
    font-weight: 400;
  }

  &:hover {
    color: var(--td-brand-color-hover);
    background: var(--td-bg-color-secondarycontainer);
  }
}

.model-option {
  display: flex;
  align-items: center;
  padding: 6px 8px;
  cursor: pointer;
  transition: background 0.12s;
  border-radius: 6px;
  margin-bottom: 4px;

  &:last-child {
    margin-bottom: 0;
  }

  &:hover,
  &.selected {
    background: var(--td-bg-color-secondarycontainer);
  }

  &.empty {
    color: var(--td-text-color-placeholder);
    cursor: default;
    text-align: center;
    padding: 20px 8px;

    &:hover {
      background: transparent;
    }
  }
}

.model-option-left {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.model-option-icon {
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.model-option-name-wrap {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  flex: 1;
}

.model-option-name {
  font-size: 12px;
  color: var(--td-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}

.model-option-raw-name {
  font-size: 11px;
  color: var(--td-text-color-placeholder);
  flex-shrink: 0;
}
</style>
