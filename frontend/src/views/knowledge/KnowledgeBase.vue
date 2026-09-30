<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, reactive, computed, nextTick } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import DocContent from "@/components/doc-content.vue";
import useKnowledgeBase from "@/hooks/useKnowledgeBase";
import { useRoute, useRouter } from "vue-router";
import EmptyKnowledge from "@/components/empty-knowledge.vue";
import ContextualGuide from "@/components/ContextualGuide.vue";
import KBInfoPopover from "@/components/KBInfoPopover.vue";
import KBSwitcherDropdown from "@/components/KBSwitcherDropdown.vue";
import { useUIStore } from "@/stores/ui";
import { useOrganizationStore } from "@/stores/organization";
import { useAuthStore } from "@/stores/auth";
import { useChatResourcesStore } from "@/stores/chatResources";
import { useEditorResourcesStore } from "@/stores/editorResources";
import KnowledgeBaseEditorModal from "./KnowledgeBaseEditorModal.vue";
const uiStore = useUIStore();
const orgStore = useOrganizationStore();
const authStore = useAuthStore();
const chatResources = useChatResourcesStore();
const editorResources = useEditorResourcesStore();
const router = useRouter();
import {
  batchQueryKnowledge,
  listKnowledgeTags,
  updateKnowledgeTagBatch,
  uploadKnowledgeFile,
  createKnowledgeFromURL,
  reparseKnowledge,
  cancelKnowledgeParse,
  batchDeleteKnowledge,
  batchReparseKnowledge,
  getKnowledgeSpans,
  getKnowledgeDetails,
  listKnowledgeFolders,
  moveKnowledgeToFolder,
  renameKnowledgeFolder,
  downKnowledgeDetails,
  type KnowledgeFolderTree,
} from "@/api/knowledge-base/index";
import { knowledgeSpansPayloadHasTrace } from "@/utils/knowledgeTrace";
import FAQEntryManager from "./components/FAQEntryManager.vue";
import DocumentListView from "./components/DocumentListView.vue";
import DocumentCardView from "./components/DocumentCardView.vue";
import DocumentBatchBar from "./components/DocumentBatchBar.vue";
import KbUploadSourceDropdown from "./components/KbUploadSourceDropdown.vue";
import KbFolderTree from "./components/KbFolderTree.vue";
import TagEditDialog from "./components/TagEditDialog.vue";
import BatchTagDialog from "./components/BatchTagDialog.vue";
import KbTagManageDrawer from "./components/KbTagManageDrawer.vue";
import type { KnowledgeProcessOverrides } from "@/types/knowledgeProcess";
import { useUploadConfirmStore, type UploadConfirmResult } from "@/stores/uploadConfirm";
import WikiBrowser from "./wiki/WikiBrowser.vue";
import { getWikiStats } from "@/api/wiki";
import KnowledgeHealthView from "./health/KnowledgeHealthView.vue";
import { getFindingsSummary, type FindingsSummary } from "@/api/findings";
import {
  isKnowledgeParseInFlight,
  knowledgeNeedsStatusPolling,
  shouldRefreshWikiStatusAfterKnowledgePoll,
} from "./wikiStatusRefresh";
import { listMoveTargets, moveKnowledge, getKnowledgeMoveProgress } from "@/api/knowledge-base";
import { resolveKnowledgeDownloadFileName } from "./knowledgeDownloadFileName";
import {
  buildUploadFileName,
  canMoveFolderTo,
  childFolders,
  folderBreadcrumbs as buildFolderBreadcrumbs,
  folderPathExists as folderExistsInTree,
  isFilteringDocuments,
  isFolderUpload,
  ROOT_FOLDER_PATH,
} from "./folderTree";
import { useI18n } from "vue-i18n";
import { useMarqueeSelect } from "@/hooks/useMarqueeSelect";
import type { ParserEngineInfo } from "@/api/system";
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CircleCheckIcon,
  CircleXIcon,
  ClockIcon,
  FileIcon,
  FolderIcon,
  InfoIcon,
  LayoutGridIcon,
  LinkIcon,
  ListIcon,
  Loader2Icon,
  SearchIcon,
  SettingsIcon,
  TagsIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
const route = useRoute();
const { t } = useI18n();
const kbId = computed(() => ((route.params as any).kbId as string) || "");
const kbInfo = ref<any>(null);
const uploadSourceRef = ref<InstanceType<typeof KbUploadSourceDropdown> | null>(null);
const kbLoading = ref(false);
const docListLoading = ref(true);
const isFAQ = computed(() => (kbInfo.value?.type || "") === "faq");
const isWiki = computed(() => !!kbInfo.value?.indexing_strategy?.wiki_enabled);
const validTabs = ["documents", "wiki", "graph", "health"] as const;
type KbTab = (typeof validTabs)[number];
const initTab = validTabs.includes(route.query.tab as any) ? (route.query.tab as KbTab) : "documents";
const activeKbTab = ref<KbTab>(initTab);

// Wiki 状态用于面包屑上的索引中指示。父组件自行拉取，避免依赖 WikiBrowser 挂载状态
// （用户切到"文档" tab 时 WikiBrowser 会卸载，这里仍需持续反映后台索引进度）。
const wikiStatus = ref<{ pendingTasks: number; isActive: boolean; pendingIssues: number }>({
  pendingTasks: 0,
  isActive: false,
  pendingIssues: 0,
});
const wikiIsIndexing = computed(() => wikiStatus.value.isActive || wikiStatus.value.pendingTasks > 0);
const wikiIndexingTip = computed(() => {
  if (!wikiIsIndexing.value) return "";
  return t("knowledgeEditor.wikiBrowser.queueStatus", { count: wikiStatus.value.pendingTasks || 0 });
});
// Classes of one breadcrumb tab (documents / Wiki / graph). The active tab and
// a tab whose index is still being built both read in the brand colour and do
// not change on hover; only the active one is bold.
const breadcrumbTabClass = (tab: KbTab, indexing: boolean) => [
  "inline-flex cursor-pointer items-center gap-1 transition-colors duration-150 select-none",
  activeKbTab.value === tab ? "font-semibold" : "font-normal",
  activeKbTab.value === tab || indexing ? "text-primary" : "text-placeholder hover:text-foreground",
];
// Knowledge health. The summary is read here, not only inside the health view,
// because the tab carries the open count as a badge while another tab is
// showing. A backend without the findings API answers with an error, and then
// the tab is not offered at all rather than leading to a view that can only
// fail; "loading" keeps a ?tab=health link on the health view while the first
// answer is on its way instead of flashing the document list.
const healthSummary = ref<FindingsSummary | null>(null);
const healthState = ref<"loading" | "ok" | "unavailable">("loading");
const healthAvailable = computed(() => healthState.value !== "unavailable");
const showHealthView = computed(() => activeKbTab.value === "health" && healthAvailable.value);
const showDocumentsView = computed(
  () => !showHealthView.value && (activeKbTab.value === "documents" || activeKbTab.value === "health" || !isWiki.value),
);
let healthSeq = 0;
const loadHealthSummary = async (id: string) => {
  const seq = ++healthSeq;
  healthState.value = "loading";
  healthSummary.value = null;
  try {
    const summary = await getFindingsSummary(id);
    if (seq !== healthSeq) return;
    healthSummary.value = summary;
    healthState.value = "ok";
  } catch (err) {
    if (seq !== healthSeq) return;
    console.debug("knowledge health unavailable", err);
    healthState.value = "unavailable";
  }
};
const onHealthSummaryChange = (summary: FindingsSummary) => {
  healthSummary.value = summary;
  healthState.value = "ok";
};
// Set when the health view asks for the wiki's issue list; WikiBrowser is
// mounted fresh by the tab switch and opens its issue drawer on arrival.
const openWikiIssuesOnMount = ref(false);
const onOpenWikiIssues = () => {
  openWikiIssuesOnMount.value = true;
  activeKbTab.value = "wiki";
};
watch(activeKbTab, (tab) => {
  if (tab !== "wiki") openWikiIssuesOnMount.value = false;
});
watch(
  kbId,
  (id) => {
    if (id) void loadHealthSummary(id);
  },
  { immediate: true },
);
const onWikiStatusChange = (payload: { pendingTasks: number; isActive: boolean; pendingIssues: number }) => {
  wikiStatus.value = payload;
};
const onViewWikiInGraph = async (slug: string) => {
  // Write tab+slug first so the activeKbTab watcher's later replace
  // (which spreads route.query) preserves slug instead of clobbering it.
  await router.replace({ query: { ...route.query, tab: "graph", slug } });
  activeKbTab.value = "graph";
};

let wikiStatusTimer: ReturnType<typeof setInterval> | null = null;
let wikiStatusProbeTimers: Array<ReturnType<typeof setTimeout>> = [];
const stopWikiStatusPolling = () => {
  if (wikiStatusTimer) {
    clearInterval(wikiStatusTimer);
    wikiStatusTimer = null;
  }
};
const clearWikiStatusProbes = () => {
  wikiStatusProbeTimers.forEach((t) => clearTimeout(t));
  wikiStatusProbeTimers = [];
};
const fetchWikiStatusOnce = async () => {
  if (!kbId.value || !isWiki.value) return;
  try {
    const res: any = await getWikiStats(kbId.value);
    const data = res?.data || res;
    if (!data) return;
    wikiStatus.value = {
      pendingTasks: data.pending_tasks || 0,
      isActive: !!data.is_active,
      pendingIssues: data.pending_issues || 0,
    };
    // 活跃时轮询，空闲时停掉定时器，避免无谓请求
    if (wikiIsIndexing.value) {
      if (!wikiStatusTimer) {
        wikiStatusTimer = setInterval(fetchWikiStatusOnce, 5000);
      }
    } else {
      stopWikiStatusPolling();
    }
  } catch (_) {
    /* ignore */
  }
};
// 用户刚触发了一个上传 / reparse / URL 导入之类的动作后，后台通常要过
// 一小段时间才会把 wiki 任务真正塞进队列；如果这时空闲轮询刚好停了，
// 面包屑的"索引中"会延迟很久才亮起。所以这里安排几次退避重试，
// 主动把面包屑的 loading 尽快点亮，一旦探测到任务就会走正常的 5s 轮询。
const scheduleWikiStatusProbes = () => {
  if (!kbId.value || !isWiki.value) return;
  clearWikiStatusProbes();
  const delays = [500, 2000, 5000, 10000];
  delays.forEach((delay) => {
    const timer = setTimeout(() => {
      fetchWikiStatusOnce();
    }, delay);
    wikiStatusProbeTimers.push(timer);
  });
};
watch(
  [kbId, isWiki],
  ([newKbId, newIsWiki]) => {
    stopWikiStatusPolling();
    clearWikiStatusProbes();
    wikiStatus.value = { pendingTasks: 0, isActive: false, pendingIssues: 0 };
    if (newKbId && newIsWiki) {
      fetchWikiStatusOnce();
    }
  },
  { immediate: true },
);
onUnmounted(() => {
  stopWikiStatusPolling();
  clearWikiStatusProbes();
});
const missingStorageEngine = computed(() => {
  if (!kbInfo.value || isFAQ.value) return false;
  // storage_backend_id is authoritative; storage_provider_config.provider is a
  // compatibility projection for older clients. Either being present means the
  // KB has a bound storage instance and uploads should not be blocked.
  if (kbInfo.value.storage_backend_id) return false;
  const spc = kbInfo.value.storage_provider_config;
  return !spc || !spc.provider;
});
const parserEngines = computed<ParserEngineInfo[]>(() => editorResources.parserEngines);

const supportedFileTypes = computed<Set<string>>(() => {
  const engines = parserEngines.value;
  if (!engines.length) return new Set<string>();

  const rules: { file_types: string[]; engine: string }[] = kbInfo.value?.chunking_config?.parser_engine_rules || [];

  const ruleMap = new Map<string, string>();
  for (const r of rules) {
    for (const ft of r.file_types) ruleMap.set(ft, r.engine);
  }

  const available = new Set<string>();
  const availableEngineNames = new Set(engines.filter((e) => e.Available !== false).map((e) => e.Name));

  for (const engine of engines) {
    for (const ft of engine.FileTypes || []) {
      if (available.has(ft)) continue;

      const explicitEngine = ruleMap.get(ft);
      if (explicitEngine) {
        if (availableEngineNames.has(explicitEngine)) available.add(ft);
      } else {
        if (engine.Available !== false) available.add(ft);
      }
    }
  }
  return available;
});

const acceptFileTypes = computed(() => [...supportedFileTypes.value].map((t) => "." + t).join(","));

const unsupportedFileTypes = computed<string[]>(() => {
  const engines = parserEngines.value;
  if (!engines.length) return [];

  const allTypes = new Set<string>();
  for (const engine of engines) {
    for (const ft of engine.FileTypes || []) allTypes.add(ft);
  }

  const supported = supportedFileTypes.value;
  return [...allTypes].filter((ft) => !supported.has(ft)).sort();
});

const goToParserSettings = () => {
  if (kbId.value) {
    uiStore.openKBSettings(kbId.value, "parser");
  }
};

// Permission control: check if current user owns this KB or has edit/manage permission
//
// "Owner" here is "the original creator of this KB" (PR 5 introduced
// CreatorID). The previous version compared kb.tenant_id to the active
// tenant id, which only answers "is this KB inside our tenant" — that
// is true even for a Viewer in someone else's tenant, so the gate
// silently bypassed every role check below. Now we require an explicit
// creator match, and the role-aware fallbacks below decide whether a
// non-creator may edit / manage.
const isOwner = computed(() => {
  if (!kbInfo.value) return false;
  const creatorId = (kbInfo.value as any).creator_id || "";
  const userId = authStore.user?.id || "";
  // creator_id may be empty for legacy KBs created before PR 5; treat
  // those as tenant-owned so the role gate applies (Admin+ can manage,
  // Viewer cannot).
  if (!creatorId) return false;
  return creatorId === userId;
});

// Current KB's shared record (when accessed via organization share)
const currentSharedKb = computed(
  () => orgStore.sharedKnowledgeBases.find((s) => s.knowledge_base?.id === kbId.value) ?? null,
);

// Accessed via organization share: when the KB shows up in our
// sharedKnowledgeBases list it means we reached it through a shared space,
// not because we own/manage it in our tenant. In that case the user's local
// tenant role does NOT grant edit/manage — only the share grant does.
// Without this guard a local tenant Admin would see edit/upload entries on
// a read-only shared KB and get 403'd by the backend on click.
//
// Note: tenant_id comparison alone is unreliable — a user can be a member of
// both the source and receiving tenants, and currentTenantId reflects the
// active switcher rather than "how this KB became visible to me". Presence
// in the share list is the authoritative signal.
const isViaShare = computed(() => !!currentSharedKb.value);

// Can edit: when accessed via an organization share, ONLY the share grant
// counts — even if the current user happens to be the original creator of
// the KB. The backend's RBAC middleware authorizes based on the active
// tenant, not on creator_id, so a creator viewing their own KB from a
// different tenant context will be 403'd on write. Otherwise: KB creator
// (any role) or tenant Admin+ in the home tenant.
//
// hasRole('contributor') is intentionally NOT here — being a Contributor
// in a tenant does not by itself grant edit on someone else's KB.
const canEdit = computed(() => {
  if (isViaShare.value) return orgStore.canEditKB(kbId.value, false);
  if (isOwner.value) return true;
  if (authStore.hasRole("admin")) return true;
  return orgStore.canEditKB(kbId.value, false);
});

// Can manage (delete, settings, etc.): same isViaShare-first rule. For
// shared KBs only an 'admin' share grant qualifies — editor/viewer (and
// even being the creator viewed via share) never grant delete/settings.
const canManage = computed(() => {
  if (isViaShare.value) return orgStore.canManageKB(kbId.value, false);
  if (isOwner.value) return true;
  if (authStore.hasRole("admin")) return true;
  return orgStore.canManageKB(kbId.value, false);
});

// The activity feed exposes owner-side actor and configuration summaries.
// It lives in KB settings (KnowledgeBaseEditorModal) for Owner/Admin in the home tenant.

// Can mutate knowledge (move / batch-delete): the backend gate for these
// two endpoints is g.Contributor(), so the caller MUST be Contributor+
// in their tenant on top of having KB edit permission. Without the extra
// role check, an org-share-editor whose tenant role is Viewer would see
// the "Move" / "Batch manage" entries and 403 on click. For shared KBs
// the local tenant role is irrelevant — canEdit already encodes the share
// grant, so trust it.
const canMutateKnowledge = computed(() => {
  if (!canEdit.value) return false;
  if (isViaShare.value) return true;
  if (isOwner.value) return true;
  if (authStore.hasRole("admin")) return true;
  return authStore.hasRole("contributor");
});

// Effective permission: from direct org share list or from GET /knowledge-bases/:id
const effectiveKBPermission = computed(() => orgStore.getKBPermission(kbId.value) || kbInfo.value?.my_permission || "");

// Downloading returns the original source file, which is intentionally more
// restrictive than viewing parsed content or using the preview tab. A tenant
// Viewer can never download; for cross-tenant KBs the effective share
// permission must additionally be Editor or Admin.
const canDownloadKnowledge = computed(() => {
  if (!authStore.hasRole("contributor")) return false;
  const permission = effectiveKBPermission.value;
  return !permission || permission === "owner" || permission === "admin" || permission === "editor";
});

const knowledgeList = ref<Array<{ id: string; name: string; type?: string }>>([]);
const {
  cardList,
  total,
  moreIndex,
  details,
  getKnowled,
  delKnowledge,
  onVisibleChange: _onVisibleChange,
  getCardDetails,
  getfDetails,
} = useKnowledgeBase(kbId.value);

const showKbDetailContextualGuide = computed(() => {
  return Boolean(kbId.value) && !isFAQ.value && canEdit.value && !docListLoading.value && cardList.value.length === 0;
});

const onVisibleChange = (visible: boolean) => {
  _onVisibleChange(visible);
  if (!visible) {
    moveMenuMode.value = "normal";
  }
};

/** Per-knowledge cache: whether /spans has a real trace (see knowledgeSpansPayloadHasTrace). */
const traceAvailableById = reactive<Record<string, boolean>>({});
const traceProbeInflight = new Set<string>();

function clearTraceAvailabilityCache() {
  for (const key of Object.keys(traceAvailableById)) {
    delete traceAvailableById[key];
  }
  traceProbeInflight.clear();
}

// Parse phases where the backend pipeline is still actively running
// (primary parse OR post-process fan-out). Trace data exists and the
// UI should treat the row as "in flight" rather than terminal.
function isParseInFlight(status?: string): boolean {
  return isKnowledgeParseInFlight(status);
}

async function probeTraceAvailable(item: KnowledgeCard) {
  const id = item.id;
  if (!id || traceProbeInflight.has(id)) return;
  if (isParseInFlight(item.parse_status)) {
    traceAvailableById[id] = true;
    return;
  }
  if (Object.prototype.hasOwnProperty.call(traceAvailableById, id)) return;
  traceProbeInflight.add(id);
  try {
    const res: any = await getKnowledgeSpans(id);
    traceAvailableById[id] = !!(res?.success && knowledgeSpansPayloadHasTrace(res.data));
  } catch {
    traceAvailableById[id] = false;
  } finally {
    traceProbeInflight.delete(id);
  }
}

const onCardMoreVisibleChange = (visible: boolean, item: KnowledgeCard) => {
  onVisibleChange(visible);
  if (visible) {
    probeTraceAvailable(item);
  }
};
const isCardDetails = ref(false);
let timeout: ReturnType<typeof setTimeout> | null = null;
const knowledgeScroll = ref();
let page = 1;
let pageSize = 35;
let scrollLoading = false;
const resetPage = () => {
  page = 1;
  scrollLoading = false;
};

// Move state — inline in card menu
const moveMenuMode = ref<"normal" | "targets" | "confirm">("normal");
const moveKnowledgeId = ref("");
const moveTargetKbs = ref<any[]>([]);
const moveTargetsLoading = ref(false);
const moveSelectedTargetId = ref("");
const moveSelectedTargetName = ref("");
const moveMode = ref<"reuse_vectors" | "reparse">("reuse_vectors");
const moveSubmitting = ref(false);
let movePollTimer: ReturnType<typeof setInterval> | null = null;

// View mode (grid / list) — persisted per browser
type DocViewMode = "grid" | "list";
const VIEW_MODE_KEY = "yuheng.kb.docs.viewMode";
const initViewMode = (): DocViewMode => {
  try {
    return localStorage.getItem(VIEW_MODE_KEY) === "list" ? "list" : "grid";
  } catch {
    return "grid";
  }
};
const viewMode = ref<DocViewMode>(initViewMode());
watch(viewMode, (v) => {
  try {
    localStorage.setItem(VIEW_MODE_KEY, v);
  } catch {
    /* ignore */
  }
});

// Multi-select state — shared between grid and list views.
// Vue 3.5 tracks Set#add/delete natively, so direct mutation is reactive.
const selectedIds = ref<Set<string>>(new Set());
let lastSelectedIndex = -1;
const batchDeleting = ref(false);
const batchReparsing = ref(false);
const batchTagging = ref(false);
const batchTagDialogVisible = ref(false);
const batchTagPreSelectedIds = computed(() => {
  const ids = Array.from(selectedIds.value);
  if (ids.length === 0) return [];
  const cards = ids.map((id) => cardList.value.find((c) => c.id === id)).filter((c): c is KnowledgeCard => Boolean(c));
  if (cards.length === 0) return [];
  const firstTagIds = new Set((cards[0].tags || []).map((t) => t.id));
  for (let i = 1; i < cards.length; i++) {
    const cur = new Set((cards[i].tags || []).map((t) => t.id));
    for (const tid of firstTagIds) {
      if (!cur.has(tid)) firstTagIds.delete(tid);
    }
  }
  return Array.from(firstTagIds);
});
// IDs submitted for async batch reparse; hold optimistic pending until the worker updates DB.
const pendingReparseAck = ref<Set<string>>(new Set());

const applyOptimisticBatchReparse = (ids: string[]) => {
  const idSet = new Set(ids);
  for (const card of cardList.value) {
    if (!idSet.has(card.id)) continue;
    pendingReparseAck.value.add(card.id);
    card.parse_status = "pending";
    card.summary_status = undefined;
    card.description = "";
    delete traceAvailableById[card.id];
    traceAvailableById[card.id] = true;
  }
};

const syncReparseAckFromServer = (ids: string[]) => {
  for (const id of ids) {
    if (!pendingReparseAck.value.has(id)) continue;
    const card = cardList.value.find((c) => c.id === id);
    if (card && isParseInFlight(card.parse_status)) {
      pendingReparseAck.value.delete(id);
    }
  }
};

const awaitBatchReparseReflection = async (ids: string[]) => {
  const maxPolls = 30;
  const delayMs = 400;
  for (let i = 0; i < maxPolls && pendingReparseAck.value.size > 0; i++) {
    await loadKnowledgeFiles(kbId.value);
    syncReparseAckFromServer(ids);
    applyOptimisticBatchReparse(Array.from(pendingReparseAck.value));
    await new Promise<void>((r) => setTimeout(r, delayMs));
  }
  pendingReparseAck.value.clear();
};

const confirmBatchReparse = async () => {
  if (batchReparsing.value || batchDeleting.value || selectedIds.value.size === 0) return;
  const allIds = Array.from(selectedIds.value);
  const ids = allIds.filter((id) => {
    const item = cardList.value.find((c) => c.id === id);
    return !item || !isParseInFlight(item.parse_status);
  });
  const skipped = allIds.length - ids.length;
  if (ids.length === 0) {
    MessagePlugin.info(t("knowledgeBase.rebuildInProgress"));
    return;
  }
  if (skipped > 0) {
    MessagePlugin.warning(t("knowledgeBase.batchReparseSkippedInFlight", { count: skipped }));
  }
  batchReparsing.value = true;
  try {
    const res: any = await batchReparseKnowledge(kbId.value, ids);
    if (res?.success) {
      MessagePlugin.success(t("knowledgeBase.batchReparseSuccess", { count: ids.length }));
      applyOptimisticBatchReparse(ids);
      clearSelection();
      batchMode.value = false;
      scheduleWikiStatusProbes();
      void awaitBatchReparseReflection(ids);
    } else {
      MessagePlugin.error(res?.message || t("knowledgeBase.batchReparseFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeBase.batchReparseFailed"));
  } finally {
    batchReparsing.value = false;
  }
};

const tagFilterPanelVisible = ref(false);
const tagFilterTriggerHover = ref(false);
const tagFilterCleared = ref(false);
const tagManageDrawerVisible = ref(false);

const showTagFilterClear = computed(() => selectedTagIds.value.length > 0 && tagFilterTriggerHover.value);

const isTagFilterPlaceholder = computed(() => selectedTagIds.value.length === 0 && tagFilterCleared.value);

const selectedTagIds = ref<string[]>([]);
const tagList = ref<any[]>([]);
const tagLoading = ref(false);
const tagSearchQuery = ref("");
const TAG_PAGE_SIZE = 50;
const tagPage = ref(1);
const tagHasMore = ref(false);
const tagLoadingMore = ref(false);
const tagTotal = ref(0);
let tagSearchDebounce: number | null = null;
let docSearchDebounce: number | null = null;
const docSearchKeyword = ref("");
const selectedFileType = ref("");
const fileTypeOptions = computed(() => [
  { label: t("knowledgeBase.allFileTypes"), value: "" },
  { label: "PDF", value: "pdf" },
  { label: "DOCX", value: "docx" },
  { label: "DOC", value: "doc" },
  { label: "PPTX", value: "pptx" },
  { label: "PPT", value: "ppt" },
  { label: "EPUB", value: "epub" },
  { label: "MHTML", value: "mhtml" },
  { label: "TXT", value: "txt" },
  { label: "MD", value: "md" },
  { label: "URL", value: "url" },
  { label: t("knowledgeBase.typeManual"), value: "manual" },
  { label: "MP3", value: "mp3" },
  { label: "WAV", value: "wav" },
  { label: "M4A", value: "m4a" },
  { label: "FLAC", value: "flac" },
  { label: "OGG", value: "ogg" },
]);
const selectedParseStatus = ref("");
const parseStatusOptions = computed(() => [
  { label: t("knowledgeBase.allParseStatuses"), value: "" },
  { label: t("knowledgeBase.parseStatusPending"), value: "pending" },
  { label: t("knowledgeBase.parseStatusProcessing"), value: "processing" },
  { label: t("knowledgeBase.parseStatusCompleted"), value: "completed" },
  { label: t("knowledgeBase.parseStatusFailed"), value: "failed" },
  { label: t("knowledgeBase.parseStatusCancelled"), value: "cancelled" },
  { label: t("knowledgeBase.parseStatusFinalizing"), value: "finalizing" },
  { label: t("knowledgeBase.parseStatusDraft"), value: "draft" },
]);
const selectedSource = ref("");
// Source filter combines ingestion channels and the "manual"/"url" virtual
// sources that the backend routes onto the `type` column.
const sourceOptions = computed(() => [
  { label: t("knowledgeBase.allSources"), value: "" },
  { label: t("knowledgeBase.sourceUpload"), value: "web" },
  { label: t("knowledgeBase.sourceUrl"), value: "url" },
  { label: t("knowledgeBase.sourceManual"), value: "manual" },
  { label: t("knowledgeBase.sourceApi"), value: "api" },
  { label: t("knowledgeBase.sourceBrowserExtension"), value: "browser_extension" },
  { label: t("knowledgeBase.channelFeishu"), value: "feishu" },
  { label: t("knowledgeBase.channelFeishuDrive"), value: "feishu_drive" },
  { label: t("knowledgeBase.channelNotion"), value: "notion" },
  { label: t("knowledgeBase.channelYuque"), value: "yuque" },
  { label: t("knowledgeBase.channelGitLab"), value: "gitlab" },
  { label: t("knowledgeBase.channelIma"), value: "ima" },
  { label: t("knowledgeBase.channelWechat"), value: "wechat" },
  { label: t("knowledgeBase.channelWecom"), value: "wecom" },
  { label: t("knowledgeBase.channelDingtalk"), value: "dingtalk" },
  { label: t("knowledgeBase.channelSlack"), value: "slack" },
  { label: t("knowledgeBase.channelIm"), value: "im" },
]);
// Date range as [start, end] in "YYYY-MM-DD" form (t-date-range-picker default).
const updatedTimeRange = ref<string[]>([]);
// Disable any date after today so users cannot filter into the future.
const disableFutureDate = { after: new Date(new Date().setHours(23, 59, 59, 999)) };
// The same bound for the native date inputs, as a local "YYYY-MM-DD" (an ISO
// string would be the UTC date, a day off on either side of midnight).
const latestFilterDate = (() => {
  const d = disableFutureDate.after;
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
})();
// The range picker used to write the whole range at once, and [] when cleared.
// The two date inputs edit one end each, so each edit writes a new array (the
// selection-clearing watcher is shallow) and an emptied range collapses to [].
const setUpdatedTimeRangeEdge = (edge: 0 | 1, value: string | number) => {
  const next = [updatedTimeRange.value[0] ?? "", updatedTimeRange.value[1] ?? ""];
  next[edge] = String(value);
  updatedTimeRange.value = next.some(Boolean) ? next : [];
};
// The filter bar's inputs and select triggers share the look the old :deep()
// overrides gave TDesign's: a filled field with no border until hovered or
// focused, when it turns to the container colour with a brand border and no
// focus ring.
// The dark: variants are needed because Input and SelectTrigger set their own.
const FILTER_CONTROL_CLASS =
  "bg-muted dark:bg-muted border-transparent rounded-md text-[13px] shadow-none data-placeholder:text-placeholder hover:border-primary hover:bg-card dark:hover:bg-card focus-visible:border-primary focus-visible:bg-card dark:focus-visible:bg-card focus-visible:ring-0 data-[state=open]:border-primary data-[state=open]:bg-card dark:data-[state=open]:bg-card";
// Reka's SelectItem cannot carry an empty value, which is what the "all …"
// entries of the filter selects stand for; this sentinel stands in for it.
const ALL_FILTER_VALUE = "__all__";
const toFilterOptionValue = (value: string) => value || ALL_FILTER_VALUE;
const fromFilterOptionValue = (value: unknown) => (value === ALL_FILTER_VALUE || value == null ? "" : String(value));
// The three select filters of the toolbar, rendered by one template loop.
const selectFilters = computed(() => [
  {
    key: "fileType",
    model: selectedFileType,
    icon: FileIcon,
    placeholder: t("knowledgeBase.fileTypeFilter"),
    options: fileTypeOptions.value,
  },
  {
    key: "parseStatus",
    model: selectedParseStatus,
    icon: CircleCheckIcon,
    placeholder: t("knowledgeBase.parseStatusFilter"),
    options: parseStatusOptions.value,
  },
  {
    key: "source",
    model: selectedSource,
    icon: LinkIcon,
    placeholder: t("knowledgeBase.sourceFilter"),
    options: sourceOptions.value,
  },
]);

// ── Folder tree (documents uploaded as a folder keep their relative path) ──
const FOLDER_TREE_COLLAPSED_KEY = "yuheng.kbFolderTreeCollapsed";
const readStoredFlag = (key: string, fallback = false) => {
  try {
    const raw = localStorage.getItem(key);
    return raw === null ? fallback : raw === "true";
  } catch {
    return fallback;
  }
};
const writeStoredFlag = (key: string, value: boolean) => {
  try {
    localStorage.setItem(key, String(value));
  } catch {
    // Private-mode storage failures must not break navigation.
  }
};
const folderTree = ref<KnowledgeFolderTree | null>(null);
const folderTreeLoading = ref(false);
// The folder being browsed; ROOT_FOLDER_PATH ('') is the knowledge base top
// level, a real node of the tree rather than a separate mode.
const selectedFolderPath = ref<string>(ROOT_FOLDER_PATH);
const folderTreeCollapsed = ref(readStoredFlag(FOLDER_TREE_COLLAPSED_KEY));
const hasFolders = computed(() => (folderTree.value?.folders?.length ?? 0) > 0);
// The folder column only earns its space once the knowledge base actually has
// folders, so knowledge bases filled with single-file uploads look unchanged.
const showFolderTree = computed(() => !isFAQ.value && hasFolders.value);
// Browsing lists one folder's own contents; filtering searches its whole
// subtree. There is no mode switch: the list follows what the user is doing.
const isFiltering = computed(() =>
  isFilteringDocuments({
    keyword: docSearchKeyword.value,
    tagIds: selectedTagIds.value,
    fileType: selectedFileType.value,
    parseStatus: selectedParseStatus.value,
    source: selectedSource.value,
    timeRange: updatedTimeRange.value,
  }),
);
// Sub-folder entries shown at the top of the list while browsing. Search results
// are flat, so they are dropped as soon as a filter is active. When the sidebar
// tree is open it already lists the same folders, so skip the duplicate rows.
const currentChildFolders = computed(() => {
  if (isFiltering.value) return [];
  if (showFolderTree.value && !folderTreeCollapsed.value) return [];
  return childFolders(folderTree.value, selectedFolderPath.value);
});
// A row's folder is worth showing only when the list can span folders.
const showDocumentFolderPath = computed(() => hasFolders.value && isFiltering.value);
const folderBreadcrumbs = computed(() => buildFolderBreadcrumbs(selectedFolderPath.value));

const filterParams = computed(() => {
  const [start, end] = updatedTimeRange.value || [];
  return {
    tag_ids: selectedTagIds.value.length > 0 ? selectedTagIds.value.join(",") : undefined,
    keyword: docSearchKeyword.value ? docSearchKeyword.value.trim() : undefined,
    file_type: selectedFileType.value || undefined,
    parse_status: selectedParseStatus.value || undefined,
    source: selectedSource.value || undefined,
    start_time: start ? `${start} 00:00:00` : undefined,
    end_time: end ? `${end} 23:59:59` : undefined,
    folder_path: selectedFolderPath.value,
    // Searching descends into sub-folders; browsing shows one level, with the
    // sub-folders themselves rendered as entries in the list.
    folder_recursive: isFiltering.value,
  };
});
const tagMap = computed<Record<string, any>>(() => {
  const map: Record<string, any> = {};
  tagList.value.forEach((tag) => {
    map[tag.id] = tag;
  });
  return map;
});
const sidebarCategoryCount = computed(() => tagTotal.value || tagList.value.length);
const sidebarTags = computed(() => {
  const list = tagList.value;
  const selectedIds = selectedTagIds.value;
  if (selectedIds.length === 0) {
    return list;
  }
  const missing = selectedIds
    .filter((id) => !list.some((tag) => tag.id === id))
    .map((id) => tagMap.value[id])
    .filter(Boolean);
  if (missing.length === 0) {
    return list;
  }
  return [...missing, ...list];
});

const activeTagFilterLabel = computed(() => {
  if (selectedTagIds.value.length === 0) {
    return tagFilterCleared.value ? t("knowledgeBase.tagFilterPlaceholder") : t("knowledgeBase.allTags");
  }
  if (selectedTagIds.value.length === 1) {
    const id = selectedTagIds.value[0];
    return tagMap.value[id]?.name || t("knowledgeBase.allTags");
  }
  return t("knowledgeBase.tagFilterMulti", { count: selectedTagIds.value.length });
});

const activeTagFilterTitle = computed(() => {
  if (selectedTagIds.value.length === 0) {
    return t("knowledgeBase.tagFilterTitle");
  }
  const names = selectedTagIds.value.map((id) => tagMap.value[id]?.name).filter(Boolean);
  return names.length > 0 ? names.join("、") : t("knowledgeBase.tagFilterTitle");
});

const isTagFilterActive = (tagId: string) => selectedTagIds.value.includes(tagId);

// 标签编辑弹窗
const tagEditDialogVisible = ref(false);
const tagEditTarget = ref<KnowledgeCard | null>(null);

function openTagEditDialog(item: KnowledgeCard) {
  tagEditTarget.value = item;
  tagEditDialogVisible.value = true;
}

function onTagEditConfirm(tagIds: string[]) {
  if (tagEditTarget.value) {
    handleKnowledgeTagChange(tagEditTarget.value.id, tagIds);
  }
}
const getPageSize = () => {
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
  const itemHeight = 148;
  const itemsInView = Math.floor(viewportHeight / itemHeight) * 5;
  pageSize = Math.max(35, itemsInView);
};
getPageSize();
// 直接调用 API 获取知识库文件列表

const loadKnowledgeFiles = (kbIdValue: string): Promise<void> => {
  if (!kbIdValue) return Promise.resolve();
  if (!isFAQ.value) {
    docListLoading.value = true;
  }
  return getKnowled(
    {
      page: 1,
      page_size: pageSize,
      ...filterParams.value,
    },
    kbIdValue,
  ).finally(() => {
    if (isCurrentKb(kbIdValue) && !isFAQ.value) {
      docListLoading.value = false;
    }
  });
};

const isCurrentKb = (targetKbId: string) => targetKbId === kbId.value;

const loadFolderTree = async (kbIdValue: string) => {
  if (!kbIdValue || isFAQ.value) {
    folderTree.value = null;
    return;
  }
  folderTreeLoading.value = true;
  try {
    const res: any = await listKnowledgeFolders(kbIdValue);
    if (!isCurrentKb(kbIdValue)) return;
    folderTree.value = (res?.data as KnowledgeFolderTree) || null;
    // A folder can disappear (its last document was deleted or moved); fall
    // back to the root instead of leaving an empty, unreachable view.
    if (!folderExistsInTree(folderTree.value?.folders || [], selectedFolderPath.value)) {
      selectedFolderPath.value = ROOT_FOLDER_PATH;
    }
  } catch (error) {
    if (!isCurrentKb(kbIdValue)) return;
    console.error("Failed to load knowledge folders", error);
    folderTree.value = null;
  } finally {
    if (isCurrentKb(kbIdValue)) {
      folderTreeLoading.value = false;
    }
  }
};

const handleFolderSelect = (path: string) => {
  if (selectedFolderPath.value === path) return;
  selectedFolderPath.value = path;
};

// ── Re-filing documents and renaming folders ──
// folder_path is display-only, so both operations are a plain column update:
// nothing is re-parsed, re-chunked or re-embedded.

// Flat folder list shared by every "move to folder" picker.
const folderOptions = computed(() => {
  const result: Array<{ path: string; name: string; depth: number }> = [];
  const walk = (nodes: KnowledgeFolderTree["folders"], depth: number) => {
    nodes.forEach((node) => {
      result.push({ path: node.path, name: node.name, depth });
      walk(node.children || [], depth + 1);
    });
  };
  walk(folderTree.value?.folders || [], 0);
  return result;
});

const moveKnowledgeIntoFolder = async (ids: string[], folderPath: string) => {
  if (!kbId.value || ids.length === 0) return;
  try {
    await moveKnowledgeToFolder(kbId.value, ids, folderPath);
    MessagePlugin.success(t("knowledgeBase.moveToFolder.success", { count: ids.length }));
    clearSelection();
    batchMode.value = false;
    resetPage();
    await loadKnowledgeFiles(kbId.value);
    await loadFolderTree(kbId.value);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("knowledgeBase.moveToFolder.failed"));
  }
};

const handleFolderRename = async ({ from, to }: { from: string; to: string }) => {
  if (!kbId.value || !to || from === to) return;
  if (!canMoveFolderTo(from, to)) {
    MessagePlugin.warning(t("knowledgeBase.folderTree.renameInvalid"));
    return;
  }
  try {
    const res: any = await renameKnowledgeFolder(kbId.value, from, to);
    const movedCount = res?.data?.moved_count ?? 0;
    if (movedCount === 0) {
      MessagePlugin.warning(t("knowledgeBase.folderTree.renameFailed"));
      await loadFolderTree(kbId.value);
      return;
    }
    MessagePlugin.success(t("knowledgeBase.folderTree.renameSuccess"));
    // Follow the folder to its new path so the user stays where they were.
    if (selectedFolderPath.value === from) {
      selectedFolderPath.value = to;
    } else if (selectedFolderPath.value.startsWith(`${from}/`)) {
      selectedFolderPath.value = to + selectedFolderPath.value.slice(from.length);
    }
    resetPage();
    await loadKnowledgeFiles(kbId.value);
    await loadFolderTree(kbId.value);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("knowledgeBase.folderTree.renameFailed"));
  }
};

const handleFolderTreeCollapsedChange = (value: boolean) => {
  folderTreeCollapsed.value = value;
  writeStoredFlag(FOLDER_TREE_COLLAPSED_KEY, value);
};

const loadTags = async (kbIdValue: string, reset = false) => {
  if (!kbIdValue) {
    tagList.value = [];
    tagTotal.value = 0;
    tagHasMore.value = false;
    tagPage.value = 1;
    return;
  }

  if (reset) {
    tagPage.value = 1;
    tagList.value = [];
    tagTotal.value = 0;
    tagHasMore.value = false;
  } else if (tagLoading.value || tagLoadingMore.value) {
    return;
  }

  const currentPage = tagPage.value || 1;
  tagLoading.value = currentPage === 1;
  tagLoadingMore.value = currentPage > 1;

  try {
    const res: any = await listKnowledgeTags(kbIdValue, {
      page: currentPage,
      page_size: TAG_PAGE_SIZE,
      keyword: tagSearchQuery.value || undefined,
    });
    if (!isCurrentKb(kbIdValue)) return;

    const pageData = (res?.data || {}) as {
      data?: any[];
      total?: number;
    };
    const pageTags = (pageData.data || []).map((tag: any) => ({
      ...tag,
      id: String(tag.id),
    }));

    if (currentPage === 1) {
      tagList.value = pageTags;
    } else {
      tagList.value = [...tagList.value, ...pageTags];
    }

    tagTotal.value = pageData.total || tagList.value.length;
    tagHasMore.value = tagList.value.length < tagTotal.value;
    if (tagHasMore.value) {
      tagPage.value = currentPage + 1;
    }
  } catch (error) {
    if (!isCurrentKb(kbIdValue)) return;
    console.error("Failed to load tags", error);
  } finally {
    if (isCurrentKb(kbIdValue)) {
      tagLoading.value = false;
      tagLoadingMore.value = false;
    }
  }
};

const handleTagFilterChange = (tagIds: string[]) => {
  selectedTagIds.value = tagIds;
  // 同步更新 store 中的 selectedTagIds，供 menu.vue 上传时使用
  uiStore.clearSelectedTagIds();
  tagIds.forEach((id) => uiStore.toggleSelectedTagId(id));
  resetPage();
};

const handleTagRowClick = (tagId: string) => {
  const next = new Set(selectedTagIds.value);
  if (next.has(tagId)) {
    next.delete(tagId);
  } else {
    next.add(tagId);
  }
  if (next.size > 0) {
    tagFilterCleared.value = false;
  }
  handleTagFilterChange([...next]);
};

const clearTagFilter = () => {
  tagFilterCleared.value = true;
  handleTagFilterChange([]);
};

const openTagManageDrawer = () => {
  tagFilterPanelVisible.value = false;
  tagManageDrawerVisible.value = true;
};

const openTagManageFromEditDialog = () => {
  tagEditDialogVisible.value = false;
  tagManageDrawerVisible.value = true;
};

const openTagManageFromBatchDialog = () => {
  batchTagDialogVisible.value = false;
  tagManageDrawerVisible.value = true;
};

const onTagManageChanged = (payload?: { deletedTagId?: string }) => {
  if (!kbId.value) return;
  void loadTags(kbId.value, true);
  if (payload?.deletedTagId && selectedTagIds.value.includes(payload.deletedTagId)) {
    selectedTagIds.value = [];
    handleTagFilterChange([]);
    resetPage();
    loadKnowledgeFiles(kbId.value);
    return;
  }
  if (payload?.deletedTagId) {
    void (async () => {
      await new Promise((resolve) => setTimeout(resolve, 800));
      if (!kbId.value) return;
      resetPage();
      await loadKnowledgeFiles(kbId.value);
      await loadTags(kbId.value, true);
    })();
    return;
  }
  resetPage();
  loadKnowledgeFiles(kbId.value);
};

const handleKnowledgeTagChange = async (knowledgeId: string, tagIds: string[]) => {
  try {
    await updateKnowledgeTagBatch({ updates: { [knowledgeId]: tagIds } });
    MessagePlugin.success(t("knowledgeBase.tagUpdateSuccess"));
    resetPage(); // Reset page counter to 1 when reloading files after tag change
    loadKnowledgeFiles(kbId.value);
    loadTags(kbId.value, true);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  }
};

const loadKnowledgeBaseInfo = async (targetKbId: string, force = false) => {
  if (!targetKbId) {
    kbInfo.value = null;
    cardList.value = [];
    total.value = 0;
    return;
  }
  kbLoading.value = true;
  try {
    const data = await chatResources.fetchKnowledgeBaseById(targetKbId, force);
    if (!isCurrentKb(targetKbId)) return;

    kbInfo.value = data;
    selectedTagIds.value = [];
    tagFilterCleared.value = false;
    uiStore.clearSelectedTagIds();
    // 重置store中的标签选择状态，避免上传文档时自动带上之前选择的标签
    uiStore.clearSelectedTagIds();
    if (!isFAQ.value) {
      loadKnowledgeFiles(targetKbId);
      void loadFolderTree(targetKbId);
    } else {
      cardList.value = [];
      total.value = 0;
      folderTree.value = null;
    }
    loadTags(targetKbId, true);
  } catch (error) {
    if (!isCurrentKb(targetKbId)) return;

    console.error("Failed to load knowledge base info:", error);
    kbInfo.value = null;
    cardList.value = [];
    total.value = 0;
  } finally {
    if (isCurrentKb(targetKbId)) {
      kbLoading.value = false;
    }
  }
};

const loadKnowledgeList = async () => {
  try {
    await chatResources.ensureKnowledgeBases();
    const myKbs = chatResources.rawKnowledgeBases.map((item: any) => ({
      id: String(item.id),
      name: item.name,
      type: item.type || "document",
    }));

    // Also include shared knowledge bases from orgStore
    const sharedKbs = (orgStore.sharedKnowledgeBases || [])
      .filter((s) => s.knowledge_base != null)
      .map((s) => ({
        id: String(s.knowledge_base.id),
        name: s.knowledge_base.name,
        type: s.knowledge_base.type || "document",
      }));

    // Merge and deduplicate by id (my KBs take precedence)
    const myKbIds = new Set(myKbs.map((kb) => kb.id));
    const uniqueSharedKbs = sharedKbs.filter((kb) => !myKbIds.has(kb.id));

    knowledgeList.value = [...myKbs, ...uniqueSharedKbs];
  } catch (error) {
    console.error("Failed to load knowledge list:", error);
  }
};

// 监听路由参数变化，重新获取知识库内容
// Sync activeKbTab to URL query so it survives page refresh
watch(activeKbTab, (tab) => {
  const query = { ...route.query };
  if (tab === "documents") {
    delete query.tab;
  } else {
    query.tab = tab;
  }
  router.replace({ query });
});

watch(
  () => kbId.value,
  (newKbId, oldKbId) => {
    if (!newKbId) {
      kbInfo.value = null;
      cardList.value = [];
      total.value = 0;
      return;
    }
    if (newKbId === oldKbId && kbInfo.value) return;

    if (newKbId !== oldKbId) {
      clearTraceAvailabilityCache();
      cardList.value = [];
      total.value = 0;
      docListLoading.value = true;
      resetPage();
      tagSearchQuery.value = "";
      tagPage.value = 1;
      uiStore.clearSelectedTagIds();
      folderTree.value = null;
      selectedFolderPath.value = ROOT_FOLDER_PATH;
    }
    loadKnowledgeBaseInfo(newKbId);
  },
  { immediate: true },
);

watch(
  selectedTagIds,
  (newVal, oldVal) => {
    if (oldVal === undefined) return;
    if (kbId.value) {
      loadKnowledgeFiles(kbId.value);
    }
  },
  { deep: true },
);

watch(tagSearchQuery, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (tagSearchDebounce) {
    window.clearTimeout(tagSearchDebounce);
  }
  tagSearchDebounce = window.setTimeout(() => {
    if (kbId.value) {
      loadTags(kbId.value, true);
    }
  }, 300);
});

// 监听文档搜索关键词变化
watch(docSearchKeyword, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (docSearchDebounce) {
    window.clearTimeout(docSearchDebounce);
  }
  docSearchDebounce = window.setTimeout(() => {
    if (kbId.value) {
      resetPage();
      loadKnowledgeFiles(kbId.value);
    }
  }, 300);
});

// 监听文件类型筛选变化
watch(selectedFileType, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (kbId.value) {
    resetPage();
    loadKnowledgeFiles(kbId.value);
  }
});

// 监听解析状态/来源/更新时间范围筛选变化（与文件类型行为一致）
watch(
  [selectedParseStatus, selectedSource, updatedTimeRange],
  () => {
    if (kbId.value) {
      resetPage();
      loadKnowledgeFiles(kbId.value);
    }
  },
  { deep: true },
);

// 切换目录只改变列表范围，行为与其他筛选一致。浏览态与筛选态之间的切换由各筛选项
// 自身的 watcher 触发刷新，这里不重复请求。
watch(selectedFolderPath, () => {
  if (!kbId.value || isFAQ.value) return;
  clearSelection();
  resetPage();
  loadKnowledgeFiles(kbId.value);
});

// 监听文件上传事件
const handleFileUploaded = (event: CustomEvent) => {
  const uploadedKbId = event.detail.kbId;
  console.log("接收到文件上传事件，上传的知识库ID:", uploadedKbId, "当前知识库ID:", kbId.value);
  if (uploadedKbId && uploadedKbId === kbId.value && !isFAQ.value) {
    console.log("匹配当前知识库，开始刷新文件列表");
    // 如果上传的文件属于当前知识库，使用 loadKnowledgeFiles 刷新文件列表
    resetPage(); // Reset page counter when reloading files after upload
    loadKnowledgeFiles(uploadedKbId);
    loadTags(uploadedKbId);
    void loadFolderTree(uploadedKbId);
    // 启动几次探测，尽快让面包屑的"索引中"亮起。
    scheduleWikiStatusProbes();
  }
};

// 监听从菜单触发的URL导入事件
const handleOpenURLImportDialog = (event: CustomEvent) => {
  const eventKbId = event.detail.kbId;
  console.log("接收到URL导入对话框打开事件，知识库ID:", eventKbId, "当前知识库ID:", kbId.value);
  if (eventKbId && eventKbId === kbId.value && !isFAQ.value) {
    if (ensureDocumentKbReady()) {
      uploadSourceRef.value?.openUrlDialog();
    }
  }
};

// Global file drops are captured by the platform shell. Route them back into
// the same confirmation flow as the page upload button so tags and per-batch
// processing settings are never skipped.
const handleKnowledgeFileDrop = (event: CustomEvent) => {
  const eventKbId = event.detail?.kbId;
  const files = Array.isArray(event.detail?.files) ? event.detail.files : [];
  if (eventKbId !== kbId.value || isFAQ.value || files.length === 0) return;
  handleUploadSourceFiles(files);
};

// Auto-open document detail when navigated with ?knowledge_id=xxx.
// Note: this runs both when the KB page mounts with a query param AND when a
// subsequent in-page navigation (e.g. from the global command palette) only
// changes the query without re-mounting the component — in that case kbId is
// the same and cardList may already be populated, so relying solely on the
// cardList watcher misses the trigger.
const pendingKnowledgeId = ref<string | null>((route.query.knowledge_id as string) || null);

// Video deep-link: ?t=<ms> jumps the embedded player to a cited playback
// position once the document drawer opens (set alongside knowledge_id by
// chat reference timestamp badges).
const parseVideoSeekParam = (value: unknown): number | null => {
  const ms = Number(typeof value === "string" ? value : "");
  return Number.isFinite(ms) && ms >= 0 ? ms : null;
};
const pendingVideoSeekMs = ref<number | null>(parseVideoSeekParam(route.query.t));

let autoOpenRequest = 0;

const tryAutoOpenDocument = async () => {
  if (!pendingKnowledgeId.value) return;
  const targetId = pendingKnowledgeId.value;
  pendingKnowledgeId.value = null;
  const request = ++autoOpenRequest;
  const card = cardList.value.find((c: KnowledgeCard) => c.id === targetId);

  // The current card list only contains the folder being browsed. A document
  // opened from chat references may live in any nested folder, so resolve its
  // folder from the detail endpoint before opening the drawer. Otherwise the
  // drawer opens correctly while the page misleadingly remains at KB root.
  let target = card || ({ id: targetId } as KnowledgeCard);
  try {
    const response: any = await getKnowledgeDetails(targetId);
    if (request !== autoOpenRequest) return;
    const detail = response?.data || response;
    if (detail && typeof detail === "object") {
      target = { ...target, ...detail, id: targetId } as KnowledgeCard;
      selectedFolderPath.value = detail.folder_path || ROOT_FOLDER_PATH;
    }
  } catch (error) {
    // Keep the previous ID-only fallback: getCardDetails will surface the
    // normal detail loading error, while links to root-level files still work.
    console.error("Failed to resolve referenced document folder", error);
  }

  if (request !== autoOpenRequest) return;
  await nextTick();
  openCardDetails(target);
};

// React to later ?knowledge_id= changes on the same KB route (no remount).
watch(
  () => route.query.knowledge_id,
  (newId) => {
    if (typeof newId !== "string" || !newId) return;
    pendingKnowledgeId.value = newId;
    pendingVideoSeekMs.value = parseVideoSeekParam(route.query.t);
    tryAutoOpenDocument();
  },
);

// Dispatched by the global command palette when the user picks a chunk that
// lives in the KB they are already viewing — vue-router dedupes identical
// navigations, so we rely on this event instead of a URL change.
const handleOpenKnowledgeEvent = (e: Event) => {
  const detail = (e as CustomEvent<{ kbId: string; knowledgeId: string }>).detail;
  if (!detail || !detail.knowledgeId) return;
  if (detail.kbId && detail.kbId !== kbId.value) return;
  pendingKnowledgeId.value = detail.knowledgeId;
  tryAutoOpenDocument();
};

onMounted(() => {
  loadKnowledgeList();
  editorResources.ensureParserEngines();

  window.addEventListener("knowledgeFileUploaded", handleFileUploaded as EventListener);
  window.addEventListener("openURLImportDialog", handleOpenURLImportDialog as EventListener);
  window.addEventListener("yuheng:knowledge-file-drop", handleKnowledgeFileDrop as EventListener);
  window.addEventListener("yuheng:open-knowledge", handleOpenKnowledgeEvent as EventListener);
});

onUnmounted(() => {
  window.removeEventListener("knowledgeFileUploaded", handleFileUploaded as EventListener);
  window.removeEventListener("openURLImportDialog", handleOpenURLImportDialog as EventListener);
  window.removeEventListener("yuheng:knowledge-file-drop", handleKnowledgeFileDrop as EventListener);
  window.removeEventListener("yuheng:open-knowledge", handleOpenKnowledgeEvent as EventListener);
  stopMovePoll();
  if (timeout !== null) {
    clearTimeout(timeout);
    timeout = null;
  }
});
watch(
  () => cardList.value,
  (newValue) => {
    if (isFAQ.value) return;
    docListLoading.value = false;

    // Auto-open document if navigated with ?knowledge_id=xxx.
    if (pendingKnowledgeId.value) {
      tryAutoOpenDocument();
    }

    let analyzeList = [];
    // Filter items that need polling: parsing in progress OR summary generation in progress
    analyzeList = newValue.filter(needsStatusPolling);
    if (timeout !== null) {
      clearTimeout(timeout);
      timeout = null;
    }
    if (analyzeList.length) {
      updateStatus(analyzeList);
    }
  },
  { deep: true },
);
type KnowledgeCard = {
  id: string;
  knowledge_base_id?: string;
  parse_status: string;
  summary_status?: string;
  description?: string;
  file_name?: string;
  original_file_name?: string;
  display_name?: string;
  title?: string;
  type?: string;
  updated_at?: string;
  file_type?: string;
  isMore?: boolean;
  metadata?: any;
  error_message?: string;
  tags?: Array<{ id: string; name: string; color?: string }>;
};
// needsStatusPolling decides whether a card row is still "in flight"
// enough that the doc list should keep refreshing it. Keep in sync with
// the backend lifecycle: pending / processing are the primary parse
// phase, finalizing is the post-process fan-out (summary / question /
// graph extract still running), and a `completed` row whose summary
// hasn't landed yet keeps polling so the description fills in.
const needsStatusPolling = (item: KnowledgeCard) => {
  return knowledgeNeedsStatusPolling(item);
};

const updateStatus = (analyzeList: KnowledgeCard[]) => {
  if (timeout !== null) {
    clearTimeout(timeout);
    timeout = null;
  }
  if (!analyzeList.length) return;

  let query = ``;
  for (let i = 0; i < analyzeList.length; i++) {
    query += `ids=${analyzeList[i].id}&`;
  }
  timeout = setTimeout(() => {
    batchQueryKnowledge(query)
      .then((result: any) => {
        let shouldRefreshWikiStatus = false;
        if (result.success && result.data) {
          (result.data as KnowledgeCard[]).forEach((item: KnowledgeCard) => {
            const index = cardList.value.findIndex((card) => card.id == item.id);
            if (index == -1) return;

            let parseStatus = item.parse_status;
            if (pendingReparseAck.value.has(item.id)) {
              if (isParseInFlight(item.parse_status)) {
                pendingReparseAck.value.delete(item.id);
              } else {
                parseStatus = "pending";
              }
            }

            if (
              cardList.value[index].parse_status !== parseStatus ||
              cardList.value[index].summary_status !== item.summary_status ||
              cardList.value[index].description !== item.description
            ) {
              shouldRefreshWikiStatus ||= shouldRefreshWikiStatusAfterKnowledgePoll(cardList.value[index], {
                ...item,
                parse_status: parseStatus,
              });

              // Always update the card data
              cardList.value[index].parse_status = parseStatus;
              cardList.value[index].summary_status = item.summary_status;
              cardList.value[index].description = item.description;
              delete traceAvailableById[item.id];
            }
          });
        }
        if (shouldRefreshWikiStatus) {
          void fetchWikiStatusOnce();
        }
        // If there are no changes, the watch won't trigger, so we must manually poll again
        // Even if there are changes, we can manually poll again just to be safe.
        // The watch will clear this timeout if it triggers.
        const stillPending = cardList.value.filter(needsStatusPolling);
        if (stillPending.length > 0) {
          updateStatus(stillPending);
        }
      })
      .catch((_err) => {
        // 错误处理
        const stillPending = cardList.value.filter(needsStatusPolling);
        if (stillPending.length > 0) {
          updateStatus(stillPending);
        }
      });
  }, 1500);
};

// 恢复文档处理状态（用于刷新后恢复）

const closeDoc = () => {
  isCardDetails.value = false;
};
const openCardDetails = (item: KnowledgeCard) => {
  isCardDetails.value = true;
  getCardDetails(item);
};

// Open source document preview from WikiBrowser
const openSourceDoc = (knowledgeId: string) => {
  isCardDetails.value = true;
  getCardDetails({ id: knowledgeId });
};

const closeCardMoreMenu = (index: number) => {
  if (cardList.value?.[index]) {
    cardList.value[index].isMore = false;
  }
  moreIndex.value = -1;
};

const confirmDeleteKnowledge = (index: number, item: KnowledgeCard) => {
  closeCardMoreMenu(index);
  const deletedId = item?.id;
  delKnowledge(index, item, async () => {
    resetPage();
    const maxPolls = 30;
    const delayMs = 400;
    for (let i = 0; i < maxPolls; i++) {
      await loadKnowledgeFiles(kbId.value);
      const stillPresent = (cardList.value || []).some((c: KnowledgeCard) => c.id === deletedId);
      if (!stillPresent) break;
      await new Promise<void>((r) => setTimeout(r, delayMs));
    }
    loadTags(kbId.value, true);
    void loadFolderTree(kbId.value);
  });
};

const onReparseMenuClick = (index: number, item: KnowledgeCard) => {
  if (isParseInFlight(item.parse_status)) {
    MessagePlugin.info(t("knowledgeBase.rebuildInProgress"));
  }
};

const handleMoveKnowledge = async (item: KnowledgeCard) => {
  moveKnowledgeId.value = item.id;
  moveMenuMode.value = "targets";
  moveTargetsLoading.value = true;
  moveTargetKbs.value = [];
  try {
    const res: any = await listMoveTargets(kbId.value);
    moveTargetKbs.value = res.data || [];
  } catch {
    moveTargetKbs.value = [];
  } finally {
    moveTargetsLoading.value = false;
  }
};

const handleMoveSelectTarget = (kb: any) => {
  moveSelectedTargetId.value = kb.id;
  moveSelectedTargetName.value = kb.name;
  moveMode.value = "reuse_vectors";
  moveMenuMode.value = "confirm";
};

const handleMoveBack = () => {
  if (moveMenuMode.value === "confirm") {
    moveMenuMode.value = "targets";
  } else {
    moveMenuMode.value = "normal";
  }
};

const handleMoveConfirm = async () => {
  if (!moveSelectedTargetId.value || moveSubmitting.value) return;
  moveSubmitting.value = true;
  try {
    const res: any = await moveKnowledge({
      knowledge_ids: [moveKnowledgeId.value],
      source_kb_id: kbId.value,
      target_kb_id: moveSelectedTargetId.value,
      mode: moveMode.value,
    });
    const taskId = res.data?.task_id;
    MessagePlugin.info(t("knowledgeBase.moveStarted"));
    // Close the card menu
    moveMenuMode.value = "normal";
    cardList.value.forEach((c) => {
      c.isMore = false;
    });

    if (taskId) {
      startMovePoll(taskId);
    } else {
      moveSubmitting.value = false;
      resetPage(); // Reset page counter when reloading files after move
      loadKnowledgeFiles(kbId.value);
      void loadFolderTree(kbId.value);
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeBase.moveFailed"));
    moveSubmitting.value = false;
  }
};

const startMovePoll = (taskId: string) => {
  if (movePollTimer) clearInterval(movePollTimer);
  movePollTimer = setInterval(async () => {
    try {
      const res: any = await getKnowledgeMoveProgress(taskId);
      const data = res.data;
      if (!data) return;
      if (data.status === "completed") {
        stopMovePoll();
        moveSubmitting.value = false;
        const failed = data.failed || 0;
        if (failed > 0) {
          MessagePlugin.warning(
            t("knowledgeBase.moveCompletedWithErrors", { success: (data.processed || 0) - failed, failed }),
          );
        } else {
          MessagePlugin.success(t("knowledgeBase.moveCompleted"));
        }
        resetPage(); // Reset page counter when reloading files after move completion
        loadKnowledgeFiles(kbId.value);
        void loadFolderTree(kbId.value);
      } else if (data.status === "failed") {
        stopMovePoll();
        moveSubmitting.value = false;
        MessagePlugin.error(t("knowledgeBase.moveFailed"));
      }
    } catch {
      // ignore poll errors
    }
  }, 2000);
};

const stopMovePoll = () => {
  if (movePollTimer) {
    clearInterval(movePollTimer);
    movePollTimer = null;
  }
};

const manualEditorSuccess = ({
  kbId: savedKbId,
}: {
  kbId: string;
  knowledgeId: string;
  status: "draft" | "publish";
}) => {
  if (savedKbId === kbId.value && !isFAQ.value) {
    resetPage(); // Reset page counter when reloading files after manual edit
    loadKnowledgeFiles(savedKbId);
    void loadFolderTree(savedKbId);
  }
};

const ensureDocumentKbReady = () => {
  if (isFAQ.value) {
    MessagePlugin.warning(t("knowledgeBase.operationNotSupportedForType"));
    return false;
  }
  if (!kbId.value) {
    MessagePlugin.warning(t("knowledgeEditor.messages.missingId"));
    return false;
  }
  if (!kbInfo.value || !kbInfo.value.summary_model_id) {
    MessagePlugin.warning(t("knowledgeBase.notInitialized"));
    return false;
  }
  // Embedding model only required when RAG indexing is enabled
  const strategy = (kbInfo.value as any).indexing_strategy;
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
  if (needsEmbedding && !kbInfo.value.embedding_model_id) {
    MessagePlugin.warning(t("knowledgeBase.notInitialized"));
    return false;
  }
  if (missingStorageEngine.value) {
    MessagePlugin.warning(t("knowledgeBase.missingStorageEngineUpload"));
    return false;
  }
  return true;
};

const uploadConfirmStore = useUploadConfirmStore();

const getFolderUploadFileName = (file: File, targetFolder: string) => buildUploadFileName(file, targetFolder);

const showUploadResultMessages = (
  successCount: number,
  failCount: number,
  totalCount: number,
  mode: "document" | "folder",
) => {
  if (mode === "folder") {
    if (failCount === 0) {
      MessagePlugin.success(t("knowledgeBase.uploadAllSuccess", { count: successCount }));
    } else if (successCount > 0) {
      MessagePlugin.warning(t("knowledgeBase.uploadPartialSuccess", { success: successCount, fail: failCount }));
    } else {
      MessagePlugin.error(t("knowledgeBase.uploadAllFailed"));
    }
    return;
  }

  if (totalCount === 1) {
    if (successCount === 1) {
      MessagePlugin.success(t("knowledgeBase.uploadSuccess"));
    }
    return;
  }

  if (failCount === 0) {
    MessagePlugin.success(t("knowledgeBase.allUploadSuccess", { count: successCount }));
  } else if (successCount > 0) {
    MessagePlugin.warning(t("knowledgeBase.partialUploadSuccess", { success: successCount, fail: failCount }));
  } else {
    MessagePlugin.error(t("knowledgeBase.allUploadFailed", { count: failCount }));
  }
};

const executeUploadBatch = async (
  files: File[],
  options: {
    processConfig?: KnowledgeProcessOverrides;
    tagIds?: string[];
    /** Destination folder confirmed in the upload dialog; '' is the root. */
    targetFolder?: string;
  } = {},
) => {
  const targetKbId = kbId.value;
  if (!targetKbId || files.length === 0) {
    return { successCount: 0, failCount: files.length };
  }

  const tagIdsToUpload = options.tagIds && options.tagIds.length > 0 ? [...options.tagIds] : undefined;
  let successCount = 0;
  let failCount = 0;
  const totalCount = files.length;
  const hasFolderPaths = files.some(isFolderUpload);

  for (const file of files) {
    try {
      const uploadData: {
        file: File;
        tag_ids?: string[];
        fileName?: string;
        process_config?: KnowledgeProcessOverrides;
      } = { file, tag_ids: tagIdsToUpload };

      const fileName = getFolderUploadFileName(file, options.targetFolder || ROOT_FOLDER_PATH);
      if (fileName) uploadData.fileName = fileName;
      if (options.processConfig) {
        uploadData.process_config = options.processConfig;
      }

      const responseData: any = await uploadKnowledgeFile(targetKbId, uploadData);
      const isSuccess =
        responseData?.success ||
        responseData?.code === 200 ||
        responseData?.status === "success" ||
        (!responseData?.error && responseData);
      if (isSuccess) {
        successCount++;
      } else {
        failCount++;
        if (totalCount === 1) {
          let errorMessage = t("knowledgeBase.uploadFailed");
          if (responseData?.error?.message) {
            errorMessage = responseData.error.message;
          } else if (responseData?.message) {
            errorMessage = responseData.message;
          }
          if (responseData?.code === "duplicate_file" || responseData?.error?.code === "duplicate_file") {
            errorMessage = t("knowledgeBase.fileExists");
          }
          MessagePlugin.error(errorMessage);
        }
      }
    } catch (error: any) {
      failCount++;
      if (totalCount === 1) {
        let errorMessage = error?.error?.message || error?.message || t("knowledgeBase.uploadFailed");
        if (error?.code === "duplicate_file") {
          errorMessage = t("knowledgeBase.fileExists");
        }
        MessagePlugin.error(errorMessage);
      }
    }
  }

  if (successCount > 0) {
    window.dispatchEvent(
      new CustomEvent("knowledgeFileUploaded", {
        detail: { kbId: targetKbId },
      }),
    );
  }

  showUploadResultMessages(successCount, failCount, totalCount, hasFolderPaths ? "folder" : "document");
  return { successCount, failCount };
};

const executeUrlImport = async (url: string, processConfig?: KnowledgeProcessOverrides, tagIds?: string[]) => {
  const targetKbId = kbId.value;
  if (!targetKbId) {
    MessagePlugin.error(t("error.missingKbId"));
    return;
  }

  const tagIdsToUpload = tagIds && tagIds.length > 0 ? [...tagIds] : undefined;
  try {
    const responseData: any = await createKnowledgeFromURL(targetKbId, {
      url,
      tag_ids: tagIdsToUpload,
      process_config: processConfig,
    });
    window.dispatchEvent(
      new CustomEvent("knowledgeFileUploaded", {
        detail: { kbId: targetKbId },
      }),
    );
    const isSuccess =
      responseData?.success ||
      responseData?.code === 200 ||
      responseData?.status === "success" ||
      (!responseData?.error && responseData);
    if (isSuccess) {
      MessagePlugin.success(t("knowledgeBase.urlImportSuccess"));
    } else {
      let errorMessage = t("knowledgeBase.urlImportFailed");
      if (responseData?.error?.message) {
        errorMessage = responseData.error.message;
      } else if (responseData?.message) {
        errorMessage = responseData.message;
      }
      if (responseData?.code === "duplicate_url" || responseData?.error?.code === "duplicate_url") {
        errorMessage = t("knowledgeBase.urlExists");
      }
      MessagePlugin.error(errorMessage);
    }
  } catch (error: any) {
    let errorMessage = error?.error?.message || error?.message || t("knowledgeBase.urlImportFailed");
    if (error?.code === "duplicate_url") {
      errorMessage = t("knowledgeBase.urlExists");
    }
    MessagePlugin.error(errorMessage);
  }
};

const handleUploadConfirmResult = async (result: UploadConfirmResult) => {
  if (result.mode === "manual") {
    return;
  }

  const files = result.files || [];
  const urls = result.urls || [];
  const processConfig = result.processConfig;
  const tagIds = result.tagIds || [];

  if (files.length > 0) {
    const hasFolderPaths = files.some(isFolderUpload);
    if (hasFolderPaths) {
      MessagePlugin.info(t("knowledgeBase.uploadingFolder", { total: files.length }));
    }
    await executeUploadBatch(files, {
      processConfig,
      tagIds,
      targetFolder: result.targetFolder || ROOT_FOLDER_PATH,
    });
  }

  for (const url of urls) {
    await executeUrlImport(url, processConfig, tagIds);
  }
};

const openUploadConfirmDialog = async (files: File[], urls: string[] = []) => {
  if (!kbInfo.value) return;
  if (files.length === 0 && urls.length === 0) return;
  try {
    const result = await uploadConfirmStore.open({
      mode: "file",
      kbInfo: kbInfo.value,
      tagIds: [...selectedTagIds.value],
      files,
      urls,
      acceptFileTypes: acceptFileTypes.value,
      supportedFileTypes: [...supportedFileTypes.value],
      // Pre-fill the destination with the folder being browsed; the dialog shows
      // it and lets the user pick another folder (or the root) before confirming.
      targetFolder: selectedFolderPath.value,
      folderOptions: folderOptions.value,
    });
    await handleUploadConfirmResult(result);
  } catch {
    // cancelled
  }
};

const handleUploadSourceFiles = (files: File[]) => {
  if (!ensureDocumentKbReady()) return;
  if (files.length === 0) return;
  openUploadConfirmDialog(files);
};

const handleUploadSourceUrl = (url: string) => {
  if (!ensureDocumentKbReady()) return;
  openUploadConfirmDialog([], [url]);
};

const handleManualCreate = () => {
  if (!ensureDocumentKbReady()) return;
  uiStore.openManualEditor({
    mode: "create",
    kbId: kbId.value,
    status: "draft",
    onSuccess: manualEditorSuccess,
  });
};

const handleOpenKBSettings = () => {
  if (!kbId.value) {
    MessagePlugin.warning(t("knowledgeEditor.messages.missingId"));
    return;
  }
  uiStore.openKBSettings(kbId.value);
};

const handleNavigateToKbList = () => {
  router.push("/platform/knowledge-bases");
};

const handleNavigateToCurrentKB = () => {
  if (!kbId.value) return;
  router.push(`/platform/knowledge-bases/${kbId.value}`);
};

const handleKnowledgeDropdownSelect = (data: { value: string }) => {
  if (!data?.value) return;
  if (data.value === kbId.value) return;
  router.push(`/platform/knowledge-bases/${data.value}`);
};

const handleManualEdit = (index: number, item: KnowledgeCard) => {
  if (isFAQ.value) return;
  if (cardList.value[index]) {
    cardList.value[index].isMore = false;
  }
  uiStore.openManualEditor({
    mode: "edit",
    kbId: item.knowledge_base_id || kbId.value,
    knowledgeId: item.id,
    onSuccess: manualEditorSuccess,
  });
};

// Opens ONLY the trace drawer for this card — does NOT pop the
// document detail drawer behind it. The trace drawer attaches to
// body so it renders independent of its host's visibility; we just
// need `details` populated so the timeline component knows which
// knowledge_id to fetch. getCardDetails resets details synchronously
// then fills asynchronously, so we re-stamp the id/parse_status
// right after the call to avoid the brief empty-id window that
// would otherwise prevent the drawer from mounting.
const docContentRef = ref<any>(null);
const handleViewTrace = (index: number, item: KnowledgeCard) => {
  if (cardList.value[index]) {
    cardList.value[index].isMore = false;
  }
  moreIndex.value = -1;
  getCardDetails(item);
  details.id = item.id;
  details.parse_status = item.parse_status;
  nextTick(() => {
    docContentRef.value?.openTimeline?.();
  });
};

const confirmRebuildKnowledge = async (index: number, item: KnowledgeCard) => {
  if (isFAQ.value) return;
  if (!canEdit.value) return;
  if (!item?.id) {
    MessagePlugin.warning(t("knowledgeEditor.messages.missingId"));
    return;
  }
  if (isParseInFlight(item.parse_status)) {
    MessagePlugin.info(t("knowledgeBase.rebuildInProgress"));
    return;
  }
  closeCardMoreMenu(index);

  // No KB context to seed the dialog defaults — fall back to a direct reparse
  // that reuses the overrides stored at upload time.
  if (!kbInfo.value) {
    await submitReparse(item.id);
    return;
  }

  // Prefill the confirm dialog with the overrides this doc was last parsed with.
  let processOverrides: KnowledgeProcessOverrides | null = item.metadata?.process_overrides ?? null;
  let fileName = item.file_name || item.title || "";
  let fileType = item.file_type || "";
  try {
    const detail: any = await getKnowledgeDetails(item.id);
    if (detail?.success && detail.data) {
      processOverrides = detail.data.metadata?.process_overrides ?? processOverrides;
      fileName = detail.data.file_name || detail.data.title || fileName;
      fileType = detail.data.file_type || fileType;
    }
  } catch {
    // fall back to the list item's fields
  }

  try {
    const result = await uploadConfirmStore.open({
      mode: "reparse",
      kbInfo: kbInfo.value,
      reparse: { knowledgeId: item.id, fileName, fileType, processOverrides },
    });
    if (result.mode === "reparse" && result.reparse) {
      await submitReparse(result.reparse.knowledgeId, result.processConfig);
    }
  } catch {
    // cancelled
  }
};

const submitReparse = async (id: string, processConfig?: KnowledgeProcessOverrides) => {
  try {
    await reparseKnowledge(id, processConfig ? { process_config: processConfig } : undefined);
    delete traceAvailableById[id];
    traceAvailableById[id] = true;
    MessagePlugin.success(t("knowledgeBase.rebuildSubmitted"));
    resetPage();
    loadKnowledgeFiles(kbId.value);
    scheduleWikiStatusProbes();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("knowledgeBase.rebuildFailed"));
  }
};

const handleScroll = () => {
  if (isFAQ.value) return;
  if (docListLoading.value) return;
  if (scrollLoading) return;
  const currentKbId = kbId.value;
  if (!currentKbId) return;
  const element = knowledgeScroll.value;
  if (element) {
    const pageNum = Math.ceil(total.value / pageSize);
    const { scrollTop, scrollHeight, clientHeight } = element;
    if (scrollTop + clientHeight >= scrollHeight - 10) {
      if (cardList.value.length < total.value && page < pageNum) {
        page++;
        scrollLoading = true;
        getKnowled({ page, page_size: pageSize, ...filterParams.value }, currentKbId).finally(() => {
          if (isCurrentKb(currentKbId)) {
            scrollLoading = false;
          }
        });
      }
    }
  }
};
const getDoc = (page: number) => {
  getfDetails(details.id, page);
};

const syncDocumentSummaryState = (state: { id?: string; summary_status?: string; description?: string }) => {
  if (!state?.id) return;
  const card = cardList.value.find((item: KnowledgeCard) => item.id === state.id);
  if (!card) return;
  if (typeof state.summary_status === "string" && state.summary_status) {
    card.summary_status = state.summary_status;
  }
  if (typeof state.description === "string") {
    card.description = state.description;
  }
};

const toggleSelectRow = (id: string, checked: boolean, shiftKey?: boolean) => {
  const items = cardList.value || [];
  const idx = items.findIndex((i: KnowledgeCard) => i.id === id);
  if (shiftKey && lastSelectedIndex >= 0 && idx >= 0) {
    const [s, e] = idx < lastSelectedIndex ? [idx, lastSelectedIndex] : [lastSelectedIndex, idx];
    for (let i = s; i <= e; i++) {
      if (checked) selectedIds.value.add(items[i].id);
      else selectedIds.value.delete(items[i].id);
    }
  } else {
    if (checked) selectedIds.value.add(id);
    else selectedIds.value.delete(id);
  }
  lastSelectedIndex = idx;
};

const onCardGridCheckboxChange = (id: string, checked: boolean, ctx?: { e?: Event }) => {
  const me = ctx?.e as MouseEvent | undefined;
  toggleSelectRow(id, checked, !!me?.shiftKey);
};

const toggleSelectAll = (checked: boolean) => {
  if (checked) {
    for (const item of cardList.value || []) selectedIds.value.add(item.id);
  } else {
    for (const item of cardList.value || []) selectedIds.value.delete(item.id);
  }
};

const clearSelection = () => {
  selectedIds.value.clear();
  lastSelectedIndex = -1;
};

// Batch (multi-select) mode mirrors the session list's "批量管理" UX: while off,
// no checkbox is rendered so the title doesn't jitter on hover; while on,
// checkboxes are persistent and clicking a card toggles its selection.
const batchMode = ref(false);
// "取消选择" / 退出批量管理：清空选择，并退出 grid 视图下的批量模式。
const handleBatchCancel = () => {
  clearSelection();
  batchMode.value = false;
};
// 切到卡片视图时，如果列表视图里已经勾选过文档，需要自动开启批量管理模式，
// 否则卡片视图默认不渲染 checkbox，会看不到勾选态。
watch(viewMode, (mode) => {
  if (mode === "grid" && selectedIds.value.size > 0) {
    batchMode.value = true;
  }
});
// Triggered from a card / row "..." menu — match the session-list UX where
// the menu item simply opens batch mode (no auto-selection).
const handleEnterBatchFromCard = (item: any) => {
  if (item) item.isMore = false;
  moreIndex.value = -1;
  clearSelection();
  batchMode.value = true;
};
const {
  onContainerMouseDown: onDocMarqueeMouseDown,
  marqueeVisible: docMarqueeVisible,
  marqueeMode: docMarqueeMode,
  boxStyle: docMarqueeBoxStyle,
  shouldSuppressClick: shouldSuppressDocClick,
} = useMarqueeSelect({
  containerRef: knowledgeScroll,
  itemSelector: ".knowledge-card[data-select-id], .doc-list-row[data-select-id]",
  selectedIds,
  getItemId: (el) => el.dataset.selectId || null,
  enabled: computed(() => canEdit.value && !isFAQ.value && cardList.value.length > 0),
  onSelectionStart: () => {
    batchMode.value = true;
  },
});

const isManualDraftKnowledge = (item: KnowledgeCard) => item.type === "manual" && item.parse_status === "draft";

const openKnowledgeItem = (item: KnowledgeCard) => {
  if (shouldSuppressDocClick()) return;
  if (canEdit.value && isManualDraftKnowledge(item)) {
    const index = cardList.value.findIndex((c) => c.id === item.id);
    if (index >= 0) {
      handleManualEdit(index, item);
      return;
    }
  }
  openCardDetails(item);
};

const confirmBatchDelete = async () => {
  if (batchDeleting.value || batchReparsing.value || selectedIds.value.size === 0) return;
  const ids = Array.from(selectedIds.value);
  const deletedIdSet = new Set(ids);
  batchDeleting.value = true;
  try {
    const res: any = await batchDeleteKnowledge(kbId.value, ids);
    if (res?.success) {
      MessagePlugin.success(t("knowledgeBase.batchDeleteSuccess", { count: ids.length }));
      clearSelection();
      batchMode.value = false;
      resetPage();
      // 后端将批量删除放入异步队列，立刻拉列表仍可能包含待删项；短轮询直到列表与后端一致或超时
      const maxPolls = 30;
      const delayMs = 400;
      for (let i = 0; i < maxPolls; i++) {
        await loadKnowledgeFiles(kbId.value);
        const stillPresent = (cardList.value || []).some((c: KnowledgeCard) => deletedIdSet.has(c.id));
        if (!stillPresent) break;
        await new Promise<void>((r) => setTimeout(r, delayMs));
      }
      loadTags(kbId.value, true);
      void loadFolderTree(kbId.value);
    } else {
      MessagePlugin.error(res?.message || t("knowledgeBase.batchDeleteFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeBase.batchDeleteFailed"));
  } finally {
    batchDeleting.value = false;
  }
};

const handleBatchTag = () => {
  if (batchDeleting.value || batchReparsing.value || batchTagging.value || selectedIds.value.size === 0) return;
  batchTagDialogVisible.value = true;
};

const onBatchTagConfirm = async (tagIds: string[]) => {
  if (batchTagging.value || selectedIds.value.size === 0) return;
  const ids = Array.from(selectedIds.value);
  const updateMap: Record<string, string[]> = {};
  for (const id of ids) {
    updateMap[id] = tagIds;
  }
  batchTagging.value = true;
  try {
    await updateKnowledgeTagBatch({ updates: updateMap });
    MessagePlugin.success(t("knowledgeBase.batchTagSuccess", { count: ids.length }));
    batchTagDialogVisible.value = false;
    clearSelection();
    batchMode.value = false;
    resetPage();
    loadKnowledgeFiles(kbId.value);
    loadTags(kbId.value, true);
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeBase.batchTagFailed"));
  } finally {
    batchTagging.value = false;
  }
};

const confirmCancelParseKnowledge = async (item: KnowledgeCard) => {
  if (!item?.id) return;
  try {
    await cancelKnowledgeParse(item.id);
    MessagePlugin.success(t("knowledgeBase.cancelParseSubmitted"));
    loadKnowledgeFiles(kbId.value);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("knowledgeBase.cancelParseFailed"));
  }
};

const downloadKnowledge = async (item: KnowledgeCard) => {
  if (!item?.id) return;
  try {
    const file = await downKnowledgeDetails(item.id);
    const objectUrl = URL.createObjectURL(file);
    const link = document.createElement("a");
    const fileName = resolveKnowledgeDownloadFileName(item);
    link.style.display = "none";
    link.href = objectUrl;
    link.download = fileName;
    document.body.appendChild(link);
    link.click();
    nextTick(() => {
      link.remove();
      URL.revokeObjectURL(objectUrl);
    });
  } catch {
    MessagePlugin.error(t("file.downloadFailed"));
  }
};

// Bridge card-view actions back to existing per-card handlers.
const handleCardAction = (
  action:
    | "download"
    | "edit"
    | "reparse"
    | "cancel-parse"
    | "move"
    | "move-folder"
    | "delete"
    | "view-trace"
    | "batch-manage",
  item: KnowledgeCard,
) => {
  const idx = (cardList.value || []).findIndex((i: KnowledgeCard) => i.id === item.id);
  if (action === "download") return downloadKnowledge(item);
  if (action === "edit") return handleManualEdit(idx, item);
  if (action === "reparse") {
    if (isParseInFlight(item.parse_status)) return onReparseMenuClick(idx, item);
    return confirmRebuildKnowledge(idx, item);
  }
  if (action === "cancel-parse") return confirmCancelParseKnowledge(item);
  if (action === "move") return handleMoveKnowledge(item);
  if (action === "delete") return confirmDeleteKnowledge(idx, item);
  if (action === "view-trace") return handleViewTrace(idx, item);
  if (action === "batch-manage") return handleEnterBatchFromCard(item);
};

// Bridge list-view actions back to existing per-card handlers.
const handleListAction = (
  action:
    | "download"
    | "edit"
    | "reparse"
    | "cancel-parse"
    | "move"
    | "move-folder"
    | "delete"
    | "view-trace"
    | "batch-manage",
  item: KnowledgeCard,
) => {
  const idx = (cardList.value || []).findIndex((i: KnowledgeCard) => i.id === item.id);
  if (action === "download") return downloadKnowledge(item);
  if (action === "edit") return handleManualEdit(idx, item);
  if (action === "reparse") return confirmRebuildKnowledge(idx, item);
  if (action === "cancel-parse") return confirmCancelParseKnowledge(item);
  if (action === "move") return handleMoveKnowledge(item);
  if (action === "delete") return confirmDeleteKnowledge(idx, item);
  if (action === "view-trace") return handleViewTrace(idx, item);
  if (action === "batch-manage") return handleEnterBatchFromCard(item);
};

// Clear selection on filter/tag/kb change to avoid acting on hidden items.
watch(
  [selectedTagIds, docSearchKeyword, selectedFileType, selectedParseStatus, selectedSource, updatedTimeRange, kbId],
  () => {
    clearSelection();
  },
);

// After cardList reloads: stable keys rely on correct indices for shift-range; clamp anchor index.
watch(
  cardList,
  () => {
    const items = cardList.value || [];
    const n = items.length;
    if (lastSelectedIndex >= n) {
      lastSelectedIndex = n > 0 ? n - 1 : -1;
    }
    if (moreIndex.value >= n) {
      moreIndex.value = -1;
    }
    if (selectedIds.value.size === 0) return;
    const visible = new Set(items.map((i: KnowledgeCard) => i.id));
    for (const id of selectedIds.value) {
      if (!visible.has(id)) selectedIds.value.delete(id);
    }
  },
  { deep: false },
);

// 处理知识库编辑成功后的回调
const handleKBEditorSuccess = (kbIdValue: string) => {
  chatResources.invalidateKnowledgeBaseDetail(kbIdValue);
  chatResources.invalidate("knowledgeBases");
  loadKnowledgeList();
  if (kbIdValue === kbId.value) {
    loadKnowledgeBaseInfo(kbIdValue, true);
  }
};
</script>

<template>
  <template v-if="!isFAQ">
    <div class="mr-4 ml-1 box-border flex h-full w-full min-w-0 flex-1 flex-col gap-5 px-8 pt-6 pb-0">
      <div class="flex shrink-0 flex-wrap items-start justify-between gap-3">
        <div class="flex flex-col gap-1">
          <div class="flex flex-wrap items-center gap-2">
            <h2
              class="text-foreground m-0 flex items-center gap-1.5 [font-family:var(--app-font-family)] text-xl leading-8 font-semibold"
            >
              <button
                type="button"
                data-slot="breadcrumb-link"
                class="text-muted-foreground enabled:hover:text-success enabled:hover:bg-card disabled:text-placeholder -mx-2 -my-1 inline-flex cursor-pointer items-center gap-1 rounded-md border-none bg-transparent px-2 py-1 transition-all duration-[120ms] disabled:cursor-not-allowed"
                @click="handleNavigateToKbList"
              >
                {{ $t("menu.knowledgeBase") }}
              </button>
              <ChevronRightIcon class="text-placeholder size-3.5 shrink-0" />
              <KBSwitcherDropdown
                v-if="knowledgeList.length"
                :kb-list="knowledgeList"
                :current-kb-id="kbId"
                @select="(id) => handleKnowledgeDropdownSelect({ value: id })"
              >
                <button
                  type="button"
                  data-slot="breadcrumb-link"
                  class="group text-muted-foreground enabled:hover:text-success enabled:hover:bg-card disabled:text-placeholder -mx-2 -my-1 inline-flex cursor-pointer items-center gap-1 rounded-md border-none bg-transparent py-1 pr-1.5 pl-2 transition-all duration-[120ms] disabled:cursor-not-allowed"
                  :disabled="!kbId"
                >
                  <template v-if="!kbInfo">
                    <Skeleton class="h-5 w-[120px]" />
                  </template>
                  <template v-else>
                    <span>{{ kbInfo.name }}</span>
                    <ChevronDownIcon
                      class="size-3.5 transition-transform duration-[120ms] group-enabled:group-hover:translate-y-px"
                    />
                  </template>
                </button>
              </KBSwitcherDropdown>
              <button
                v-else
                type="button"
                data-slot="breadcrumb-link"
                class="text-muted-foreground enabled:hover:text-success enabled:hover:bg-card disabled:text-placeholder -mx-2 -my-1 inline-flex cursor-pointer items-center gap-1 rounded-md border-none bg-transparent px-2 py-1 transition-all duration-[120ms] disabled:cursor-not-allowed"
                :disabled="!kbId"
                @click="handleNavigateToCurrentKB"
              >
                <template v-if="!kbInfo">
                  <Skeleton class="h-5 w-[120px]" />
                </template>
                <template v-else>
                  {{ kbInfo.name }}
                </template>
              </button>
              <ChevronRightIcon class="text-placeholder size-3.5 shrink-0" />
              <template v-if="isWiki || healthAvailable">
                <span :class="breadcrumbTabClass('documents', false)" @click="activeKbTab = 'documents'">{{
                  $t("knowledgeEditor.wikiBrowser.tabDocuments")
                }}</span>
                <template v-if="isWiki">
                  <span class="mx-1.5 font-normal text-[var(--td-text-color-disabled)]">/</span>
                  <span :class="breadcrumbTabClass('wiki', wikiIsIndexing)" @click="activeKbTab = 'wiki'">
                    Wiki
                    <Tooltip v-if="wikiIsIndexing">
                      <TooltipTrigger as-child>
                        <Loader2Icon class="text-primary inline-flex size-3 animate-spin items-center" />
                      </TooltipTrigger>
                      <TooltipContent side="bottom">{{ wikiIndexingTip }}</TooltipContent>
                    </Tooltip>
                  </span>
                  <span class="mx-1.5 font-normal text-[var(--td-text-color-disabled)]">/</span>
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <span :class="breadcrumbTabClass('graph', wikiIsIndexing)" @click="activeKbTab = 'graph'">
                        {{ $t("knowledgeEditor.wikiBrowser.tabGraph") }}
                        <!-- As before, the indexing spinner carries its own tooltip inside the tab's. -->
                        <Tooltip v-if="wikiIsIndexing">
                          <TooltipTrigger as-child>
                            <Loader2Icon class="text-primary inline-flex size-3 animate-spin items-center" />
                          </TooltipTrigger>
                          <TooltipContent side="bottom">{{ wikiIndexingTip }}</TooltipContent>
                        </Tooltip>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">{{ $t("knowledgeEditor.wikiBrowser.tabGraphTip") }}</TooltipContent>
                  </Tooltip>
                </template>
                <template v-if="healthAvailable">
                  <span class="mx-1.5 font-normal text-[var(--td-text-color-disabled)]">/</span>
                  <span
                    :class="breadcrumbTabClass('health', false)"
                    data-testid="kb-health-tab"
                    @click="activeKbTab = 'health'"
                  >
                    {{ $t("knowledgeHealth.tab") }}
                    <span
                      v-if="healthSummary && healthSummary.open_total > 0"
                      class="bg-warning/15 text-warning inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[11px] leading-none font-semibold"
                      :aria-label="$t('knowledgeHealth.openCount', { count: healthSummary.open_total })"
                      >{{ healthSummary.open_total > 99 ? "99+" : healthSummary.open_total }}</span
                    >
                  </span>
                </template>
              </template>
              <span v-else class="text-foreground font-semibold">{{ $t("knowledgeEditor.document.title") }}</span>
            </h2>
            <!-- 标题行右侧的动作锚点：聚拢"信息"和"设置"两个圆形按钮。 -->
            <div class="ml-1 inline-flex shrink-0 items-center gap-1.5">
              <KBInfoPopover v-if="kbInfo" :kb-info="kbInfo" :supported-file-types="[...supportedFileTypes]" />
              <Tooltip v-if="canManage">
                <TooltipTrigger as-child>
                  <button
                    type="button"
                    data-slot="kb-settings-button"
                    class="bg-muted text-muted-foreground enabled:hover:text-primary inline-flex size-[30px] cursor-pointer items-center justify-center rounded-full border-none p-0 transition-all duration-200 enabled:hover:bg-[var(--td-success-color-light)] disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="!kbId"
                    :aria-label="$t('knowledgeBase.settings')"
                    @click="handleOpenKBSettings"
                  >
                    <SettingsIcon class="size-4" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("knowledgeBase.settings") }}</TooltipContent>
              </Tooltip>
            </div>
          </div>
          <p class="text-placeholder m-0 font-[family-name:var(--app-font-family)] text-sm leading-5 font-normal">
            {{ $t("knowledgeEditor.document.subtitle") }}
          </p>
          <p
            v-if="unsupportedFileTypes.length"
            class="group text-warning m-0 mt-0.5 flex cursor-pointer items-center gap-1 text-xs leading-[1.4] transition-colors hover:text-[var(--td-warning-color-active)]"
            @click="goToParserSettings"
          >
            <InfoIcon class="size-3 shrink-0" />
            <span>{{
              $t("knowledgeBase.unsupportedTypesHint", {
                types: unsupportedFileTypes.map((t) => "." + t).join("、"),
              })
            }}</span>
            <span class="text-primary ml-0.5 whitespace-nowrap group-hover:underline"
              >{{ $t("knowledgeBase.goToParserSettings") }} →</span
            >
          </p>
          <p
            v-if="missingStorageEngine"
            class="group text-warning m-0 mt-0.5 flex cursor-pointer items-center gap-1 text-xs leading-[1.4] transition-colors hover:text-[var(--td-warning-color-active)]"
            @click="handleOpenKBSettings"
          >
            <InfoIcon class="size-3 shrink-0" />
            <span>{{ $t("knowledgeBase.missingStorageEngine") }}</span>
            <span class="text-primary ml-0.5 whitespace-nowrap group-hover:underline"
              >{{ $t("knowledgeBase.goToStorageSettings") }} →</span
            >
          </p>
        </div>
      </div>

      <!-- Wiki Browser / Graph (shown when wiki or graph tab is active) -->
      <div v-if="isWiki && (activeKbTab === 'wiki' || activeKbTab === 'graph')" class="min-h-0 flex-1 overflow-hidden">
        <WikiBrowser
          v-if="kbId"
          :knowledge-base-id="kbId"
          :view="activeKbTab === 'graph' ? 'graph' : 'browser'"
          :can-edit="canEdit"
          :open-issues-on-mount="openWikiIssuesOnMount"
          @open-source-doc="openSourceDoc"
          @status-change="onWikiStatusChange"
          @view-graph="onViewWikiInGraph"
        />
      </div>

      <div v-if="showHealthView" class="flex min-h-0 flex-1 flex-col">
        <KnowledgeHealthView
          v-if="kbId"
          :kb-id="kbId"
          :can-rescan="canManage"
          :is-wiki="isWiki"
          :wiki-pending-issues="wikiStatus.pendingIssues"
          @open-knowledge="openSourceDoc"
          @open-wiki-issues="onOpenWikiIssues"
          @summary-change="onHealthSummaryChange"
        />
      </div>

      <template v-if="showDocumentsView">
        <div class="flex min-h-0 flex-1">
          <KbFolderTree
            v-if="showFolderTree && !folderTreeCollapsed"
            :tree="folderTree"
            :selected-path="selectedFolderPath"
            :loading="folderTreeLoading"
            :can-edit="canEdit"
            @select="handleFolderSelect"
            @update:collapsed="handleFolderTreeCollapsedChange"
            @rename="handleFolderRename"
          />
          <div class="flex min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
            <div
              class="[container-type:inline-size] relative flex min-w-0 flex-1 flex-col overflow-hidden [container-name:doc-card-area]"
            >
              <nav
                v-if="showFolderTree"
                class="flex shrink-0 flex-wrap items-center gap-0.5 pb-2"
                :aria-label="$t('knowledgeBase.folderTree.title')"
              >
                <Tooltip v-if="folderTreeCollapsed">
                  <TooltipTrigger as-child>
                    <button
                      type="button"
                      data-slot="folder-tree-toggle"
                      class="border-border bg-card text-muted-foreground hover:border-primary hover:text-primary hover:bg-accent mr-1 inline-flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md border p-0 transition-all duration-150"
                      :aria-label="$t('knowledgeBase.folderTree.expand')"
                      @click="handleFolderTreeCollapsedChange(false)"
                    >
                      <FolderIcon class="size-3.5" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeBase.folderTree.expand") }}</TooltipContent>
                </Tooltip>
                <span
                  v-if="!folderBreadcrumbs.length"
                  class="text-foreground max-w-[220px] cursor-default truncate rounded px-1 py-0.5 [font-family:var(--app-font-family)] text-xs leading-[18px] font-medium"
                >
                  {{ $t("knowledgeBase.folderTree.rootRow") }}
                </span>
                <button
                  v-else
                  type="button"
                  data-slot="folder-crumb"
                  class="text-muted-foreground hover:text-primary hover:bg-accent max-w-[220px] cursor-pointer truncate rounded px-1 py-0.5 [font-family:var(--app-font-family)] text-xs leading-[18px] transition-colors duration-150"
                  @click="handleFolderSelect('')"
                >
                  {{ $t("knowledgeBase.folderTree.rootRow") }}
                </button>
                <template v-for="(crumb, index) in folderBreadcrumbs" :key="crumb.path">
                  <ChevronRightIcon class="text-placeholder size-3 shrink-0" />
                  <span
                    v-if="index === folderBreadcrumbs.length - 1"
                    class="text-foreground max-w-[220px] cursor-default truncate rounded px-1 py-0.5 [font-family:var(--app-font-family)] text-xs leading-[18px] font-medium"
                  >
                    {{ crumb.name }}
                  </span>
                  <button
                    v-else
                    type="button"
                    data-slot="folder-crumb"
                    class="text-muted-foreground hover:text-primary hover:bg-accent max-w-[220px] cursor-pointer truncate rounded px-1 py-0.5 [font-family:var(--app-font-family)] text-xs leading-[18px] transition-colors duration-150"
                    @click="handleFolderSelect(crumb.path)"
                  >
                    {{ crumb.name }}
                  </button>
                </template>
                <!-- Filtering silently widens the scope to sub-folders, so say so. -->
                <span v-if="isFiltering" class="text-placeholder text-xs">
                  {{ $t("knowledgeBase.folderTree.searchingSubtree") }}
                </span>
              </nav>
              <!-- Two rows (search + trailing actions, then the filters) until the
                   content area itself is wide enough for one; see the @container
                   rule in the style block. -->
              <div
                class="doc-filter-bar-responsive grid shrink-0 [grid-template-columns:1fr_auto] items-center gap-x-3 gap-y-2 pb-3 [grid-template-areas:'search_trailing'_'filters_filters']"
              >
                <div
                  class="relative w-full min-w-0 [grid-area:search] min-[1280px]:min-w-[220px] min-[1280px]:flex-[1_1_220px]"
                >
                  <SearchIcon
                    class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
                  />
                  <Input
                    v-model.trim="docSearchKeyword"
                    :placeholder="$t('knowledgeBase.docSearchPlaceholder')"
                    :class="[FILTER_CONTROL_CLASS, 'h-8 pr-8 pl-8']"
                    @keydown.enter="loadKnowledgeFiles(kbId)"
                  />
                  <button
                    v-if="docSearchKeyword"
                    type="button"
                    data-slot="input-clear"
                    class="text-placeholder hover:text-foreground absolute top-1/2 right-2.5 inline-flex -translate-y-1/2 cursor-pointer"
                    :aria-label="$t('common.clear')"
                    @click="
                      docSearchKeyword = '';
                      loadKnowledgeFiles(kbId);
                    "
                  >
                    <CircleXIcon class="size-4" />
                  </button>
                </div>
                <div
                  class="doc-filter-bar-filters flex min-w-0 [scrollbar-width:thin] [scrollbar-color:rgba(0,0,0,0.15)_transparent] flex-nowrap items-center gap-3 overflow-x-auto [grid-area:filters]"
                >
                  <div class="w-[140px] shrink-0">
                    <Popover v-model:open="tagFilterPanelVisible">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          data-slot="tag-filter-trigger"
                          class="bg-muted box-border inline-flex h-8 w-full cursor-pointer items-center rounded-[var(--td-radius-default)] border border-transparent px-2 [font-family:var(--app-font-family)] text-sm leading-none transition-all duration-200"
                          :class="isTagFilterPlaceholder ? 'text-placeholder' : 'text-foreground'"
                          :aria-label="$t('knowledgeBase.tagFilterTitle')"
                          :title="activeTagFilterTitle"
                          @mouseenter="tagFilterTriggerHover = true"
                          @mouseleave="tagFilterTriggerHover = false"
                        >
                          <span class="text-placeholder mr-2 inline-flex shrink-0 items-center" aria-hidden="true">
                            <TagsIcon class="size-4" />
                          </span>
                          <span class="min-w-0 flex-1 truncate text-left">{{ activeTagFilterLabel }}</span>
                          <span class="ml-2 inline-flex shrink-0 items-center">
                            <span
                              v-if="showTagFilterClear"
                              class="text-placeholder hover:text-foreground inline-flex cursor-pointer"
                              :aria-label="$t('common.clear')"
                              @click.stop="clearTagFilter"
                              @mousedown.stop
                              @pointerdown.stop
                            >
                              <CircleXIcon class="size-4" />
                            </span>
                            <ChevronDownIcon
                              v-else
                              class="size-4 shrink-0 transition-[transform,color] duration-200"
                              :class="tagFilterPanelVisible ? 'text-primary rotate-180' : 'text-placeholder'"
                            />
                          </span>
                        </button>
                      </PopoverTrigger>
                      <PopoverContent
                        align="start"
                        class="border-border box-border flex max-h-[min(70vh,480px)] w-[320px] max-w-[min(320px,calc(100vw-32px))] flex-col gap-0 rounded-[8px] border-[0.5px] px-3.5 py-3 text-xs shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] ring-0"
                        @click.stop
                      >
                        <div class="text-foreground mb-2.5 flex items-center justify-between">
                          <div class="flex items-baseline gap-1.5 text-sm font-semibold tracking-[0.5px]">
                            <span>{{ $t("knowledgeBase.tagFilterTitle") }}</span>
                            <span class="text-placeholder text-xs font-normal">({{ sidebarCategoryCount }})</span>
                          </div>
                        </div>
                        <div class="relative mb-2.5">
                          <SearchIcon
                            class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
                          />
                          <Input
                            v-model.trim="tagSearchQuery"
                            :placeholder="$t('knowledgeBase.tagSearchPlaceholder')"
                            :class="[
                              FILTER_CONTROL_CLASS,
                              'hover:border-border focus-visible:border-border h-6 pr-7 pl-7',
                            ]"
                          />
                          <button
                            v-if="tagSearchQuery"
                            type="button"
                            data-slot="input-clear"
                            class="text-placeholder hover:text-foreground absolute top-1/2 right-2 inline-flex -translate-y-1/2 cursor-pointer"
                            :aria-label="$t('common.clear')"
                            @click="tagSearchQuery = ''"
                          >
                            <CircleXIcon class="size-3.5" />
                          </button>
                        </div>
                        <div
                          class="flex min-h-0 flex-1 [scrollbar-width:thin] flex-col gap-2 overflow-x-hidden overflow-y-auto"
                        >
                          <template v-if="tagLoading && !sidebarTags.length">
                            <div class="flex flex-wrap items-start gap-1.5">
                              <div v-for="n in 8" :key="'skel-tag-' + n" class="shrink-0">
                                <Skeleton class="h-6 w-14 rounded-sm" />
                              </div>
                            </div>
                          </template>
                          <template v-else>
                            <div class="flex flex-wrap items-start gap-1.5">
                              <button
                                v-for="tag in sidebarTags"
                                :key="tag.id"
                                type="button"
                                data-slot="tag-filter-chip"
                                class="group/chip box-border inline-flex h-6 max-w-full cursor-pointer items-center gap-1 rounded border px-2 [font-family:var(--app-font-family)] text-[11px] leading-6 antialiased transition-colors duration-150 outline-none focus-visible:shadow-[0_0_0_2px_color-mix(in_srgb,var(--td-component-stroke)_60%,transparent)]"
                                :class="
                                  isTagFilterActive(tag.id)
                                    ? 'text-primary border-[color-mix(in_srgb,var(--td-brand-color)_35%,var(--td-component-stroke))] bg-[color-mix(in_srgb,var(--td-brand-color)_6%,transparent)] font-medium hover:bg-[color-mix(in_srgb,var(--td-brand-color)_10%,transparent)]'
                                    : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground bg-transparent font-normal hover:border-[var(--td-component-border)]'
                                "
                                :data-active="isTagFilterActive(tag.id) || undefined"
                                :title="`${tag.name} (${tag.knowledge_count || 0})`"
                                @click="handleTagRowClick(tag.id)"
                              >
                                <span class="max-w-[120px] min-w-0 truncate">{{ tag.name }}</span>
                                <span
                                  class="shrink-0 text-[10px] font-normal [font-variant-numeric:tabular-nums] before:mr-0.5 before:opacity-65 before:content-['·']"
                                  :class="
                                    isTagFilterActive(tag.id)
                                      ? 'text-[color-mix(in_srgb,var(--td-brand-color)_72%,var(--td-text-color-secondary))]'
                                      : 'text-placeholder'
                                  "
                                  >{{ tag.knowledge_count || 0 }}</span
                                >
                              </button>
                            </div>
                            <div v-if="!sidebarTags.length" class="text-placeholder py-1.5 text-center text-xs">
                              {{ $t("knowledgeBase.tagEmptyResult") }}
                            </div>
                            <div v-if="tagHasMore" class="flex justify-center pt-0.5">
                              <Button
                                variant="ghost"
                                size="sm"
                                class="text-placeholder h-auto p-0 text-xs"
                                :disabled="tagLoadingMore"
                                @click.stop="kbId && loadTags(kbId)"
                              >
                                <Loader2Icon v-if="tagLoadingMore" class="animate-spin" />
                                {{ $t("tenant.loadMore") }}
                              </Button>
                            </div>
                          </template>
                        </div>
                        <div v-if="canEdit" class="border-border mt-2.5 flex justify-start border-t pt-2.5">
                          <Button
                            variant="link"
                            size="sm"
                            class="text-muted-foreground hover:text-primary focus-visible:text-primary h-auto min-h-0 p-0 text-[13px] transition-colors duration-150 hover:no-underline"
                            @click="openTagManageDrawer"
                          >
                            {{ $t("knowledgeBase.tagManageLink") }}
                          </Button>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                  <!-- The three selects were clearable; the × stands beside the trigger
                       (not inside it, where it would open the list) over its chevron. -->
                  <div v-for="filter in selectFilters" :key="filter.key" class="relative w-[140px] shrink-0">
                    <Select
                      :model-value="filter.model.value || undefined"
                      @update:model-value="(val) => (filter.model.value = fromFilterOptionValue(val))"
                    >
                      <SelectTrigger
                        :class="[
                          FILTER_CONTROL_CLASS,
                          'h-8 w-full gap-0 pl-2',
                          filter.model.value ? '[&>svg:last-child]:invisible' : '',
                        ]"
                      >
                        <component :is="filter.icon" class="text-placeholder mr-2 size-4 shrink-0" />
                        <SelectValue :placeholder="filter.placeholder" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem
                          v-for="opt in filter.options"
                          :key="opt.value"
                          :value="toFilterOptionValue(opt.value)"
                        >
                          {{ opt.label }}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <button
                      v-if="filter.model.value"
                      type="button"
                      data-slot="input-clear"
                      class="text-placeholder hover:text-foreground absolute top-1/2 right-2 inline-flex -translate-y-1/2 cursor-pointer"
                      :aria-label="$t('common.clear')"
                      @click="filter.model.value = ''"
                    >
                      <CircleXIcon class="size-4" />
                    </button>
                  </div>
                  <div class="w-[280px] shrink-0">
                    <!-- Two native date inputs stand in for TDesign's range picker; the
                         new stack has no date picker yet. -->
                    <div class="relative flex w-full items-center gap-1">
                      <ClockIcon
                        class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 z-[1] size-4 -translate-y-1/2"
                      />
                      <Input
                        :model-value="updatedTimeRange[0] ?? ''"
                        type="date"
                        :max="latestFilterDate"
                        :aria-label="$t('knowledgeBase.updatedTimeFrom')"
                        :title="$t('knowledgeBase.updatedTimeFrom')"
                        :class="[FILTER_CONTROL_CLASS, 'h-8 min-w-0 pr-1 pl-8']"
                        @update:model-value="(val) => setUpdatedTimeRangeEdge(0, val)"
                      />
                      <span class="text-placeholder shrink-0">–</span>
                      <Input
                        :model-value="updatedTimeRange[1] ?? ''"
                        type="date"
                        :max="latestFilterDate"
                        :aria-label="$t('knowledgeBase.updatedTimeTo')"
                        :title="$t('knowledgeBase.updatedTimeTo')"
                        :class="[FILTER_CONTROL_CLASS, 'h-8 min-w-0 pl-1.5']"
                        @update:model-value="(val) => setUpdatedTimeRangeEdge(1, val)"
                      />
                    </div>
                  </div>
                </div>
                <div class="relative z-[1] flex shrink-0 items-center gap-2 [grid-area:trailing]">
                  <div
                    class="bg-muted inline-flex shrink-0 items-center rounded-md p-0.5"
                    role="group"
                    :aria-label="$t('knowledgeBase.viewModeToggle')"
                  >
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <button
                          type="button"
                          data-slot="view-toggle"
                          class="inline-flex h-6 w-7 cursor-pointer items-center justify-center rounded transition-colors duration-[120ms]"
                          :class="
                            viewMode === 'grid'
                              ? 'bg-card text-primary shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                              : 'text-muted-foreground hover:text-foreground bg-transparent'
                          "
                          :aria-label="$t('knowledgeBase.viewModeGrid')"
                          :aria-pressed="viewMode === 'grid'"
                          @click="viewMode = 'grid'"
                        >
                          <LayoutGridIcon class="size-4" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeBase.viewModeGrid") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <button
                          type="button"
                          data-slot="view-toggle"
                          class="inline-flex h-6 w-7 cursor-pointer items-center justify-center rounded transition-colors duration-[120ms]"
                          :class="
                            viewMode === 'list'
                              ? 'bg-card text-primary shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                              : 'text-muted-foreground hover:text-foreground bg-transparent'
                          "
                          :aria-label="$t('knowledgeBase.viewModeList')"
                          :aria-pressed="viewMode === 'list'"
                          @click="viewMode = 'list'"
                        >
                          <ListIcon class="size-4" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeBase.viewModeList") }}</TooltipContent>
                    </Tooltip>
                  </div>
                  <div v-if="canEdit" class="shrink-0">
                    <!-- content-bar-icon-btn is the trigger's old hook class; the utilities
                         carry what the old :deep() rule gave it. -->
                    <KbUploadSourceDropdown
                      ref="uploadSourceRef"
                      :accept-file-types="acceptFileTypes"
                      :supported-file-types="[...supportedFileTypes]"
                      include-manual
                      trigger-icon="file-add"
                      trigger-class="content-bar-icon-btn text-muted-foreground hover:text-primary hover:bg-muted border-none bg-transparent"
                      data-guide="kb-detail-add-doc"
                      :tooltip="t('knowledgeBase.addDocument')"
                      placement="bottom-right"
                      @files="handleUploadSourceFiles"
                      @url="handleUploadSourceUrl"
                      @manual="handleManualCreate"
                    />
                  </div>
                </div>
              </div>
              <div
                class="relative min-h-0 flex-1 overflow-x-hidden pr-1"
                :class="[
                  !cardList.length && !currentChildFolders.length && !docListLoading
                    ? 'flex items-center justify-center overflow-y-hidden'
                    : 'overflow-y-auto',
                  { 'cursor-crosshair': docMarqueeVisible },
                ]"
                ref="knowledgeScroll"
                @scroll="handleScroll"
                @mousedown="onDocMarqueeMouseDown"
              >
                <div
                  v-if="docMarqueeVisible"
                  class="pointer-events-none absolute z-[4] rounded-[2px] border"
                  :class="
                    docMarqueeMode === 'subtract'
                      ? 'border-[var(--td-error-color-6)] bg-[color-mix(in_srgb,var(--td-error-color-6)_12%,transparent)]'
                      : docMarqueeMode === 'add'
                        ? 'border-primary bg-primary/14'
                        : 'border-primary bg-primary/12'
                  "
                  :style="docMarqueeBoxStyle"
                  aria-hidden="true"
                />
                <!-- 文档骨架屏 -->
                <div
                  v-if="docListLoading && cardList.length === 0 && !currentChildFolders.length"
                  class="doc-skeleton-fade-in box-border grid w-full [grid-template-columns:repeat(auto-fill,minmax(240px,1fr))] content-start gap-3"
                >
                  <div
                    v-for="n in 8"
                    :key="'doc-skel-' + n"
                    class="border-border bg-card relative box-border flex h-[136px] min-w-[240px] cursor-default flex-col overflow-hidden rounded-[8px] border shadow-[0_1px_2px_rgba(0,0,0,0.06)]"
                  >
                    <div class="flex min-h-0 flex-1 flex-col px-3.5 pt-2.5 pb-2">
                      <div class="mb-2">
                        <Skeleton class="h-[18px] w-[70%]" />
                      </div>
                      <Skeleton class="h-3.5 w-full" />
                      <Skeleton class="mt-3 h-3.5 w-[60%]" />
                    </div>
                    <div
                      class="border-border mt-auto box-border flex h-8 w-full shrink-0 items-center justify-between border-t px-3.5"
                    >
                      <Skeleton class="h-3.5 w-20" />
                      <Skeleton class="h-[18px] w-10 rounded-none" />
                    </div>
                  </div>
                </div>
                <template v-else-if="(cardList.length || currentChildFolders.length) && viewMode === 'grid'">
                  <DocumentCardView
                    :items="cardList"
                    :folders="currentChildFolders"
                    :folder-options="folderOptions"
                    :selected-ids="selectedIds"
                    :batch-mode="batchMode"
                    :can-edit="canEdit"
                    :can-download="canDownloadKnowledge"
                    :can-mutate-knowledge="canMutateKnowledge"
                    :trace-available-by-id="traceAvailableById"
                    :tag-list="tagList"
                    :move-menu-mode="moveMenuMode"
                    :move-target-kbs="moveTargetKbs"
                    :move-targets-loading="moveTargetsLoading"
                    :move-selected-target-name="moveSelectedTargetName"
                    :move-mode="moveMode"
                    :move-submitting="moveSubmitting"
                    :show-folder-path="showDocumentFolderPath"
                    @open="(item: any) => openKnowledgeItem(item)"
                    @open-folder="handleFolderSelect"
                    @move-to-folder="(item: any, path: string) => moveKnowledgeIntoFolder([item.id], path)"
                    @toggle-checkbox="onCardGridCheckboxChange"
                    @menu-visible-change="(visible: boolean, item: any) => onCardMoreVisibleChange(visible, item)"
                    @action="(action: any, item: any) => handleCardAction(action, item)"
                    @tag-edit="(item: any) => openTagEditDialog(item)"
                    @move-select-target="(kb: any) => handleMoveSelectTarget(kb)"
                    @move-back="handleMoveBack"
                    @move-confirm="handleMoveConfirm"
                    @update:move-mode="(mode: any) => (moveMode = mode)"
                  />
                </template>
                <template v-else-if="(cardList.length || currentChildFolders.length) && viewMode === 'list'">
                  <DocumentListView
                    :items="cardList"
                    :folders="currentChildFolders"
                    :folder-options="folderOptions"
                    :selected-ids="selectedIds"
                    :tag-list="tagList"
                    :can-edit="canEdit"
                    :can-download="canDownloadKnowledge"
                    :can-mutate-knowledge="canMutateKnowledge"
                    :trace-visible-ids="traceAvailableById"
                    :move-menu-mode="moveMenuMode"
                    :move-target-kbs="moveTargetKbs"
                    :move-targets-loading="moveTargetsLoading"
                    :move-selected-target-name="moveSelectedTargetName"
                    :move-mode="moveMode"
                    :move-submitting="moveSubmitting"
                    :show-folder-path="showDocumentFolderPath"
                    @open-folder="handleFolderSelect"
                    @move-to-folder="(item: any, path: string) => moveKnowledgeIntoFolder([item.id], path)"
                    @open="(item: any) => openKnowledgeItem(item)"
                    @toggle-row="toggleSelectRow"
                    @toggle-all="toggleSelectAll"
                    @action="(action: any, item: any) => handleListAction(action, item)"
                    @probe-trace="(item: any) => probeTraceAvailable(item)"
                    @tag-edit="(item: any) => openTagEditDialog(item)"
                    @move-select-target="(kb: any) => handleMoveSelectTarget(kb)"
                    @move-back="handleMoveBack"
                    @move-confirm="handleMoveConfirm"
                    @update:move-mode="(mode: any) => (moveMode = mode)"
                    @reset-move-state="moveMenuMode = 'normal'"
                  />
                </template>
                <template v-else-if="!docListLoading">
                  <div class="flex min-h-full w-full flex-1 items-center justify-center px-5 py-15">
                    <p v-if="selectedFolderPath || isFiltering" class="text-placeholder text-sm">
                      {{
                        isFiltering
                          ? $t("knowledgeBase.folderTree.emptySearch")
                          : $t("knowledgeBase.folderTree.emptyFolder")
                      }}
                    </p>
                    <EmptyKnowledge v-else />
                  </div>
                </template>
              </div>
              <div
                class="pointer-events-none absolute right-0 bottom-3 left-0 z-[6] flex justify-center px-4 [&>*]:pointer-events-auto"
                v-show="batchMode || selectedIds.size > 0"
              >
                <DocumentBatchBar
                  :count="selectedIds.size"
                  :delete-loading="batchDeleting"
                  :reparse-loading="batchReparsing"
                  :tag-loading="batchTagging"
                  :visible="batchMode || selectedIds.size > 0"
                  :show-move-to-folder="canEdit"
                  :folder-options="folderOptions"
                  @cancel="handleBatchCancel"
                  @delete="confirmBatchDelete"
                  @reparse="confirmBatchReparse"
                  @batch-tag="handleBatchTag"
                  @move-to-folder="(path: string) => moveKnowledgeIntoFolder(Array.from(selectedIds), path)"
                />
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- DocContent drawer (shared by documents tab and wiki source refs) -->
      <DocContent
        ref="docContentRef"
        :visible="isCardDetails"
        :details="details"
        :canEditKB="canEdit"
        :canDownloadKB="canDownloadKnowledge"
        :kbId="kbId"
        :seekMs="pendingVideoSeekMs"
        @closeDoc="closeDoc"
        @getDoc="getDoc"
        @summaryStateChange="syncDocumentSummaryState"
      >
      </DocContent>
    </div>
  </template>
  <template v-else>
    <div class="mr-4 ml-1 min-h-0 flex-1 overflow-y-auto px-8 py-6">
      <FAQEntryManager v-if="kbId" :kb-id="kbId" />
    </div>
  </template>

  <!-- 知识库编辑器（创建/编辑统一组件） -->
  <KnowledgeBaseEditorModal
    :visible="uiStore.showKBEditorModal"
    :mode="uiStore.kbEditorMode"
    :kb-id="uiStore.currentKBId || undefined"
    :initial-type="uiStore.kbEditorType"
    @update:visible="(val) => (val ? null : uiStore.closeKBEditor())"
    @success="handleKBEditorSuccess"
  />

  <ContextualGuide tour="kbDetail" :when="showKbDetailContextualGuide" />

  <!-- 标签编辑弹窗 -->
  <TagEditDialog
    :visible="tagEditDialogVisible"
    :knowledge-name="tagEditTarget?.display_name || tagEditTarget?.file_name || tagEditTarget?.title || ''"
    :kb-id="kbId"
    :tag-list="tagList"
    :selected-tags="tagEditTarget?.tags || []"
    :can-manage="canEdit"
    @update:visible="tagEditDialogVisible = $event"
    @confirm="onTagEditConfirm"
    @tag-created="loadTags(kbId, true)"
    @open-manage="openTagManageFromEditDialog"
  />

  <!-- 批量打标签弹窗 -->
  <BatchTagDialog
    :visible="batchTagDialogVisible"
    :count="selectedIds.size"
    :kb-id="kbId"
    :tag-list="tagList"
    :pre-selected-tag-ids="batchTagPreSelectedIds"
    :can-manage="canEdit"
    :confirm-loading="batchTagging"
    @update:visible="batchTagDialogVisible = $event"
    @confirm="onBatchTagConfirm"
    @tag-created="loadTags(kbId, true)"
    @open-manage="openTagManageFromBatchDialog"
  />

  <KbTagManageDrawer
    v-if="!isFAQ"
    v-model:visible="tagManageDrawerVisible"
    :kb-id="kbId"
    :is-faq="isFAQ"
    @changed="onTagManageChanged"
  />
</template>
<!-- What stays CSS: the skeleton grid's fade-in (a scoped @keyframes is renamed by
     Vue, so an animate-[…] utility could not reach it — the class below can), and
     the named-container breakpoint that puts the filter bar on one row once the
     content area, not the viewport, is wide enough. Everything else is utilities. -->
<style scoped>
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

.doc-skeleton-fade-in {
  animation: contentFadeIn 0.32s ease-out;
}

@container doc-card-area (min-width: 1240px) {
  .doc-filter-bar-responsive {
    display: flex;
    flex-direction: row;
    flex-wrap: nowrap;
    gap: 12px;
  }

  .doc-filter-bar-responsive .doc-filter-bar-filters {
    flex: 1 1 auto;
  }
}
</style>
