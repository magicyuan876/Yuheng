<script setup lang="ts">
import { ref, nextTick, onBeforeUnmount, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  ArrowRightIcon,
  ChartLineIcon,
  ChartColumnIcon,
  ChevronLeftIcon,
  CircleDotIcon,
  CircleIcon,
  FolderIcon,
  LinkIcon,
  ListTreeIcon,
  Loader2Icon,
  PlusIcon,
  XCircleIcon,
} from "@lucide/vue";
import { formatFileSize } from "@/utils/files";
import { useTagChipsOverflow } from "@/composables/useTagChipsOverflow";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import DocumentActionMenu from "./DocumentActionMenu.vue";
import FolderPickerMenu, { type FolderOption } from "./FolderPickerMenu.vue";
import KnowledgeProcessingTimeline from "@/components/knowledge-processing-timeline.vue";

interface Tag {
  id: string;
  name: string;
  color?: string;
}

interface KnowledgeCard {
  id: string;
  knowledge_base_id?: string;
  parse_status: string;
  summary_status?: string;
  description?: string;
  file_name?: string;
  folder_path?: string;
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
  source?: string;
  created_at?: string;
  file_size?: number | string;
  channel?: string;
}

const props = defineProps<{
  items: KnowledgeCard[];
  selectedIds: Set<string>;
  batchMode: boolean;
  canEdit: boolean;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  traceAvailableById: Record<string, boolean>;
  tagList: Tag[];
  /** Sub-folders of the folder currently being browsed. */
  folders?: Array<{ path: string; name: string; total_count: number }>;
  /** Every folder of the knowledge base, for the "move to folder" picker. */
  folderOptions?: FolderOption[];
  /**
   * Replace the updated-at line with the card's folder. Only meaningful when
   * the grid spans several folders, i.e. while filtering.
   */
  showFolderPath?: boolean;
  // Move sub-flow state
  moveMenuMode: "normal" | "targets" | "confirm";
  moveTargetKbs: any[];
  moveTargetsLoading: boolean;
  moveSelectedTargetName: string;
  moveMode: "reuse_vectors" | "reparse";
  moveSubmitting: boolean;
}>();

const emit = defineEmits<{
  (e: "open", item: KnowledgeCard): void;
  (e: "toggle-checkbox", id: string, checked: boolean, ctx?: { e?: Event }): void;
  (e: "menu-visible-change", visible: boolean, item: KnowledgeCard): void;
  (
    e: "action",
    action:
      | "download"
      | "edit"
      | "view-trace"
      | "reparse"
      | "cancel-parse"
      | "move"
      | "move-folder"
      | "batch-manage"
      | "delete",
    item: KnowledgeCard,
  ): void;
  (e: "tag-edit", item: KnowledgeCard): void;
  (e: "open-folder", path: string): void;
  (e: "move-to-folder", item: KnowledgeCard, folderPath: string): void;
  // Move sub-flow emits
  (e: "move-select-target", kb: any): void;
  (e: "move-back"): void;
  (e: "move-confirm"): void;
  (e: "update:moveMode", mode: "reuse_vectors" | "reparse"): void;
}>();

const { t } = useI18n();

const { setupTagChipsObserver, getTagLimit, hasTagOverflow, getOverflowCount } = useTagChipsOverflow("tagItemId");

// Captured click event for shift-click range selection (mirrors the ctx the
// TDesign checkbox change handler used to provide).
let lastCardCheckboxClick: MouseEvent | null = null;
const rememberCardCheckboxClick = (e: MouseEvent) => {
  lastCardCheckboxClick = e;
};

// Which row's action popup is currently showing the folder picker. Kept local so
// picking a folder stays inside the menu the user already opened, exactly like
// the "move to knowledge base" sub-menu next to it.
const folderPickerItemId = ref<string | null>(null);

// --- Menu index tracking ---
const activeMenuIndex = ref(-1);
const openMenu = (index: number) => {
  activeMenuIndex.value = index;
};
const onMenuVisibleChange = (visible: boolean, item: KnowledgeCard) => {
  if (!visible) {
    activeMenuIndex.value = -1;
    folderPickerItemId.value = null;
  }
  emit("menu-visible-change", visible, item);
};

// --- Parse status helpers ---
const CANCELABLE_PARSE_STATUSES = new Set(["pending", "processing", "finalizing"]);
const isParseInFlight = (status?: string): boolean => CANCELABLE_PARSE_STATUSES.has(String(status ?? ""));

const isTraceMenuVisible = (item: KnowledgeCard): boolean => {
  if (!item?.id) return false;
  if (isParseInFlight(item.parse_status)) return true;
  return props.traceAvailableById[item.id] === true;
};

const inFlightCardStatusText = (item: KnowledgeCard): string => {
  if (item.parse_status === "finalizing") {
    if (item.summary_status === "pending" || item.summary_status === "processing") {
      return t("knowledgeBase.generatingSummary");
    }
    return t("knowledgeBase.statusFinalizing");
  }
  return t("knowledgeBase.parsingInProgress");
};

// --- Display helpers ---
const formatDocTime = (time?: string) => {
  if (!time) return "--";
  const d = new Date(time);
  if (Number.isNaN(d.getTime())) return "--";
  const yy = String(d.getFullYear()).slice(2);
  const MM = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${yy}-${MM}-${dd} ${hh}:${mm}`;
};

const getKnowledgeType = (item: KnowledgeCard) => {
  if (item.type === "url") return t("knowledgeBase.typeURL") || "URL";
  if (item.type === "manual") return t("knowledgeBase.typeManual");
  if (item.file_type) return item.file_type.toUpperCase();
  return "--";
};

const channelLabelMap: Record<string, string> = {
  web: "knowledgeBase.channelWeb",
  api: "knowledgeBase.channelApi",
  browser_extension: "knowledgeBase.channelBrowserExtension",
  wechat: "knowledgeBase.channelWechat",
  wecom: "knowledgeBase.channelWecom",
  feishu: "knowledgeBase.channelFeishu",
  gitlab: "knowledgeBase.channelGitLab",
  dingtalk: "knowledgeBase.channelDingtalk",
  slack: "knowledgeBase.channelSlack",
  im: "knowledgeBase.channelIm",
  ima: "knowledgeBase.channelIma",
};

const getChannelLabel = (channel: string) => {
  const key = channelLabelMap[channel];
  return key ? t(key) : t("knowledgeBase.channelUnknown");
};

// --- Card click handler ---
const onCardClick = (item: KnowledgeCard) => {
  if (props.batchMode) {
    emit("toggle-checkbox", item.id, !props.selectedIds.has(item.id));
    return;
  }
  emit("open", item);
};

// --- Hover popover ---
const hoveredCardItem = ref<KnowledgeCard | null>(null);
const cardPopoverPos = ref({ x: 0, y: 0 });
const CARD_POPOVER_OFFSET = 12;
const CARD_POPOVER_ESTIMATED_WIDTH = 360;
const CARD_POPOVER_ESTIMATED_HEIGHT = 300;
const cardHoverShowDelay = 300;
let cardHoverTimer: ReturnType<typeof setTimeout> | null = null;
let cardPopoverElement: HTMLElement | null = null;

const dismissCardPopover = () => {
  if (cardHoverTimer) {
    clearTimeout(cardHoverTimer);
    cardHoverTimer = null;
  }
  hoveredCardItem.value = null;
  cardPopoverElement = null;
};

const calculatePopoverPositionFromCard = (cardElement: HTMLElement): { x: number; y: number } => {
  const cardRect = cardElement.getBoundingClientRect();
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;

  let popoverWidth = CARD_POPOVER_ESTIMATED_WIDTH;
  let popoverHeight = CARD_POPOVER_ESTIMATED_HEIGHT;

  if (cardPopoverElement) {
    const rect = cardPopoverElement.getBoundingClientRect();
    if (rect.width > 0) popoverWidth = rect.width;
    if (rect.height > 0) popoverHeight = rect.height;
  }

  let x = 0;
  let y = 0;

  // Strategy 1: right side
  const rightX = cardRect.right + CARD_POPOVER_OFFSET;
  if (rightX + popoverWidth <= viewportWidth - 10) {
    x = rightX;
    y = cardRect.top;
    if (y + popoverHeight > viewportHeight - 10) y = viewportHeight - popoverHeight - 10;
    y = Math.max(10, y);
    return { x, y };
  }

  // Strategy 2: left side
  const leftX = cardRect.left - popoverWidth - CARD_POPOVER_OFFSET;
  if (leftX >= 10) {
    x = leftX;
    y = cardRect.top;
    if (y + popoverHeight > viewportHeight - 10) y = viewportHeight - popoverHeight - 10;
    y = Math.max(10, y);
    return { x, y };
  }

  // Strategy 3: below
  const bottomY = cardRect.bottom + CARD_POPOVER_OFFSET;
  if (bottomY + popoverHeight <= viewportHeight - 10) {
    y = bottomY;
    x = cardRect.left;
    if (x + popoverWidth > viewportWidth - 10) x = viewportWidth - popoverWidth - 10;
    x = Math.max(10, x);
    return { x, y };
  }

  // Strategy 4: above
  const topY = cardRect.top - popoverHeight - CARD_POPOVER_OFFSET;
  y = Math.max(10, topY);
  x = cardRect.left;
  if (x + popoverWidth > viewportWidth - 10) x = viewportWidth - popoverWidth - 10;
  x = Math.max(10, x);
  return { x, y };
};

const onCardMouseEnter = (ev: MouseEvent, item: KnowledgeCard) => {
  if (cardHoverTimer) {
    clearTimeout(cardHoverTimer);
    cardHoverTimer = null;
  }
  const cardElement = ev.currentTarget as HTMLElement;
  cardHoverTimer = setTimeout(() => {
    cardHoverTimer = null;
    // Folder navigation can replace the card list before this delayed callback
    // runs. A detached card has a zero rect, which used to place the teleported
    // popover at the top-left corner of the viewport.
    if (!cardElement.isConnected || !props.items.some((candidate) => candidate.id === item.id)) return;
    hoveredCardItem.value = item;
    const pos = calculatePopoverPositionFromCard(cardElement);
    cardPopoverPos.value = pos;
    nextTick(() => {
      if (!cardElement.isConnected || hoveredCardItem.value?.id !== item.id) return;
      cardPopoverElement = document.querySelector(".knowledge-card-hover-popover") as HTMLElement;
      if (cardPopoverElement) {
        const refinedPos = calculatePopoverPositionFromCard(cardElement);
        cardPopoverPos.value = refinedPos;
      }
    });
  }, cardHoverShowDelay);
};

const onCardMouseLeave = () => {
  dismissCardPopover();
};

// Browsing to another folder swaps the item collection without necessarily
// dispatching mouseleave on a card that Vue removes.
watch(() => props.items, dismissCardPopover);
onBeforeUnmount(dismissCardPopover);

const onOpenFolder = (path: string) => {
  dismissCardPopover();
  emit("open-folder", path);
};

const onFolderPicked = (item: KnowledgeCard, path: string) => {
  folderPickerItemId.value = null;
  if (item.isMore !== undefined) item.isMore = false;
  activeMenuIndex.value = -1;
  emit("move-to-folder", item, path);
};

// --- Action handlers ---
const handleAction = (
  action:
    | "download"
    | "edit"
    | "view-trace"
    | "reparse"
    | "cancel-parse"
    | "move"
    | "move-folder"
    | "batch-manage"
    | "delete",
  item: KnowledgeCard,
) => {
  // The folder picker opens inside this same popup, so keep the menu open.
  if (action === "move-folder") {
    folderPickerItemId.value = item.id;
    return;
  }
  // Don't close menu for move — it triggers the sub-flow
  if (action !== "move") {
    if (item.isMore !== undefined) item.isMore = false;
    activeMenuIndex.value = -1;
  }
  emit("action", action, item);
};

// TDesign's small tag as the card footer restyled it: an 18px outlined pill
// that turns brand-coloured on hover (the whole tag strip opens the editor).
const cardTagChipClass =
  "text-muted-foreground hover:border-primary hover:bg-muted h-[18px] max-w-[120px] cursor-pointer rounded-full border border-[var(--td-component-stroke)] bg-transparent px-1.5 py-0 text-[11px] leading-[18px] font-normal transition-all hover:text-[var(--td-brand-color-active)]";
// The same pill in the hover popover, where it is not interactive.
const popoverTagChipClass =
  "text-muted-foreground h-[18px] max-w-[120px] rounded-full border border-[var(--td-component-stroke)] bg-transparent px-1.5 py-0 text-[11px] leading-[18px] font-normal";
</script>

<template>
  <div class="w-full">
    <div
      class="box-border grid w-full [animation:doc-card-fade-in_0.32s_ease-out] grid-cols-[repeat(auto-fill,minmax(240px,1fr))] content-start gap-3"
    >
      <div
        v-for="folder in folders"
        :key="'folder-' + folder.path"
        class="border-border bg-card box-border flex h-[136px] min-w-[240px] cursor-pointer flex-col overflow-hidden rounded-[8px] border shadow-[0_1px_2px_rgba(0,0,0,0.06)] transition-all duration-200 hover:border-[color-mix(in_srgb,var(--td-component-stroke)_55%,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(0,0,0,0.07)] focus-visible:shadow-[0_0_0_2px_color-mix(in_srgb,var(--td-brand-color)_30%,transparent)] focus-visible:outline-none"
        :title="folder.path"
        role="button"
        tabindex="0"
        @click="onOpenFolder(folder.path)"
        @keydown.enter="onOpenFolder(folder.path)"
      >
        <div class="flex min-h-0 flex-1 flex-col justify-start gap-2 overflow-hidden px-3.5 pt-3 pb-2.5">
          <FolderIcon class="text-primary size-7 shrink-0 opacity-[0.88]" />
          <span class="text-foreground line-clamp-2 max-h-10 min-h-0 flex-1 text-sm leading-5 font-medium break-all">
            {{ folder.name }}
          </span>
        </div>
        <div
          class="text-placeholder shrink-0 border-t border-[var(--td-component-stroke)] px-3.5 py-2 text-xs leading-[1.4]"
        >
          {{ t("knowledgeBase.folderTree.folderCardCount", { count: folder.total_count }) }}
        </div>
      </div>

      <div
        class="knowledge-card border-border bg-card relative box-border flex h-[136px] min-w-[240px] cursor-pointer flex-col overflow-hidden rounded-[8px] border shadow-[0_1px_2px_rgba(0,0,0,0.06)] transition-all duration-200 hover:border-[color-mix(in_srgb,var(--td-component-stroke)_55%,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(0,0,0,0.07)]"
        :data-select-id="item.id"
        v-for="(item, index) in items"
        :key="item.id"
        @click="onCardClick(item)"
        @mouseenter="onCardMouseEnter($event, item)"
        @mouseleave="onCardMouseLeave"
      >
        <div class="flex min-h-0 flex-1 flex-col px-3.5 pt-2.5 pb-2">
          <div class="mb-1.5 flex shrink-0 items-start">
            <div
              v-if="canEdit && batchMode"
              class="mr-2 inline-flex h-[29px] w-[22px] shrink-0 cursor-pointer items-center justify-center"
              @click.stop
            >
              <Checkbox
                :model-value="selectedIds.has(item.id)"
                :title="item.file_name"
                :aria-label="item.file_name"
                @click="rememberCardCheckboxClick"
                @update:model-value="
                  (c) => emit('toggle-checkbox', item.id, c === true, { e: lastCardCheckboxClick ?? undefined })
                "
              />
            </div>
            <span
              class="text-foreground mr-2 inline-block h-6 min-w-0 flex-1 truncate [font-family:var(--app-font-family)] text-sm leading-6 font-semibold tracking-[0.01em]"
              :title="item.file_name"
              >{{ item.file_name }}</span
            >
            <Popover
              v-if="canEdit"
              :open="!!item.isMore"
              @update:open="
                (v: boolean) => {
                  item.isMore = v;
                  onMenuVisibleChange(v, item);
                }
              "
            >
              <PopoverTrigger as-child>
                <div
                  class="flex size-[25px] shrink-0 cursor-pointer items-center justify-center rounded-[5px] hover:bg-[var(--td-component-stroke)]"
                  :class="{ 'bg-[var(--td-component-stroke)]': activeMenuIndex === index || item.isMore }"
                  @click.stop="openMenu(index)"
                >
                  <img class="h-3.5 w-3.5" src="@/assets/img/more.png" alt="" />
                </div>
              </PopoverTrigger>
              <PopoverContent align="end" class="w-auto min-w-[148px] gap-0 rounded-[10px] p-1">
                <!-- Move: folder picker (must win over the normal menu while open) -->
                <div
                  v-if="folderPickerItemId === item.id"
                  class="flex max-h-[360px] max-w-[280px] min-w-[220px] flex-col overflow-y-auto"
                >
                  <FolderPickerMenu
                    :options="folderOptions || []"
                    :current-path="item.folder_path || ''"
                    show-back
                    @back="folderPickerItemId = null"
                    @confirm="(path: string) => onFolderPicked(item, path)"
                  />
                </div>

                <!-- Normal menu -->
                <div v-else-if="moveMenuMode === 'normal'" class="flex min-w-[140px] flex-col gap-px">
                  <DocumentActionMenu
                    :item="item"
                    :can-download="canDownload"
                    :can-mutate-knowledge="canMutateKnowledge"
                    :trace-visible="isTraceMenuVisible(item)"
                    @download="handleAction('download', item)"
                    @edit="handleAction('edit', item)"
                    @view-trace="handleAction('view-trace', item)"
                    @reparse="handleAction('reparse', item)"
                    @cancel-parse="handleAction('cancel-parse', item)"
                    @move="handleAction('move', item)"
                    @move-folder="handleAction('move-folder', item)"
                    @batch-manage="handleAction('batch-manage', item)"
                    @delete="handleAction('delete', item)"
                  />
                </div>

                <!-- Move: target KB list -->
                <div
                  v-else-if="moveMenuMode === 'targets'"
                  class="flex max-h-[360px] max-w-[280px] min-w-[220px] flex-col overflow-y-auto"
                >
                  <div
                    class="text-foreground hover:bg-accent flex cursor-pointer items-center gap-1.5 border-b border-[var(--td-component-stroke)] px-3 py-2 text-[13px] font-medium"
                    @click.stop="emit('move-back')"
                  >
                    <ChevronLeftIcon class="size-4" />
                    <span>{{ $t("knowledgeBase.moveToKnowledgeBase") }}</span>
                  </div>
                  <div v-if="moveTargetsLoading" class="flex items-center justify-center py-5">
                    <Loader2Icon class="text-primary size-4 animate-spin" />
                  </div>
                  <div
                    v-else-if="moveTargetKbs.length === 0"
                    class="text-placeholder px-4 py-3 text-center text-xs leading-normal"
                  >
                    {{ $t("knowledgeBase.moveNoTargets") }}
                  </div>
                  <template v-else>
                    <div
                      v-for="kb in moveTargetKbs"
                      :key="kb.id"
                      class="group text-foreground hover:bg-accent flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 transition-all active:scale-[0.98] active:bg-[var(--td-bg-color-container-active)]"
                      @click.stop="emit('move-select-target', kb)"
                    >
                      <ListTreeIcon
                        class="text-muted-foreground group-hover:text-foreground size-4 shrink-0 transition-colors"
                      />
                      <span class="min-w-0 flex-1 truncate">{{ kb.name }}</span>
                      <span v-if="kb.knowledge_count !== undefined" class="text-placeholder text-xs">{{
                        kb.knowledge_count
                      }}</span>
                    </div>
                  </template>
                </div>

                <!-- Move: confirm -->
                <div
                  v-else-if="moveMenuMode === 'confirm'"
                  class="flex max-h-[360px] max-w-[280px] min-w-[220px] flex-col overflow-y-auto"
                >
                  <div
                    class="text-foreground hover:bg-accent flex cursor-pointer items-center gap-1.5 border-b border-[var(--td-component-stroke)] px-3 py-2 text-[13px] font-medium"
                    @click.stop="emit('move-back')"
                  >
                    <ChevronLeftIcon class="size-4" />
                    <span>{{ $t("knowledgeBase.moveConfirmTitle") }}</span>
                  </div>
                  <div class="p-2">
                    <div
                      class="bg-accent text-muted-foreground mb-2 flex items-center gap-1.5 rounded-md px-2 py-1.5 text-[13px]"
                    >
                      <ArrowRightIcon class="size-3.5" />
                      <span class="truncate">{{ moveSelectedTargetName }}</span>
                    </div>
                    <div
                      class="mb-1 flex cursor-pointer items-start gap-1.5 rounded-md px-2 py-1.5 transition-colors"
                      :class="moveMode === 'reuse_vectors' ? 'bg-[var(--td-brand-color-light)]' : 'hover:bg-accent'"
                      @click.stop="emit('update:moveMode', 'reuse_vectors')"
                    >
                      <CircleDotIcon v-if="moveMode === 'reuse_vectors'" class="text-primary mt-0.5 size-4 shrink-0" />
                      <CircleIcon v-else class="text-placeholder mt-0.5 size-4 shrink-0" />
                      <div class="flex min-w-0 flex-col gap-0.5">
                        <span class="text-foreground text-[13px] font-medium">{{
                          $t("knowledgeBase.moveModeReuseVectors")
                        }}</span>
                        <span class="text-placeholder text-[11px] leading-[1.4]">{{
                          $t("knowledgeBase.moveModeReuseVectorsDesc")
                        }}</span>
                      </div>
                    </div>
                    <div
                      class="mb-1 flex cursor-pointer items-start gap-1.5 rounded-md px-2 py-1.5 transition-colors"
                      :class="moveMode === 'reparse' ? 'bg-[var(--td-brand-color-light)]' : 'hover:bg-accent'"
                      @click.stop="emit('update:moveMode', 'reparse')"
                    >
                      <CircleDotIcon v-if="moveMode === 'reparse'" class="text-primary mt-0.5 size-4 shrink-0" />
                      <CircleIcon v-else class="text-placeholder mt-0.5 size-4 shrink-0" />
                      <div class="flex min-w-0 flex-col gap-0.5">
                        <span class="text-foreground text-[13px] font-medium">{{
                          $t("knowledgeBase.moveModeReparse")
                        }}</span>
                        <span class="text-placeholder text-[11px] leading-[1.4]">{{
                          $t("knowledgeBase.moveModeReparseDesc")
                        }}</span>
                      </div>
                    </div>
                    <div class="mt-2 flex justify-end gap-2">
                      <Button size="xs" variant="outline" @click.stop="emit('move-back')">
                        {{ $t("common.cancel") }}
                      </Button>
                      <Button size="xs" :disabled="moveSubmitting" @click.stop="emit('move-confirm')">
                        <Loader2Icon v-if="moveSubmitting" class="animate-spin" />
                        {{ $t("knowledgeBase.moveConfirm") }}
                      </Button>
                    </div>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </div>

          <!-- Parse status display -->
          <div v-if="isParseInFlight(item.parse_status)" class="flex h-auto min-h-0 shrink-0 items-center gap-0.5">
            <Loader2Icon class="text-primary mt-0.5 block size-3.5 animate-spin" />
            <span
              class="text-primary ml-2 cursor-pointer [font-family:var(--app-font-family)] text-[11px] hover:underline"
              role="button"
              tabindex="0"
              :title="$t('knowledgeStages.viewTrace')"
              @click.stop="handleAction('view-trace', item)"
              @keydown.enter.stop="handleAction('view-trace', item)"
              @keydown.space.prevent.stop="handleAction('view-trace', item)"
              >{{ inFlightCardStatusText(item) }}</span
            >
            <button
              type="button"
              data-slot="trace-button"
              class="text-primary inline-flex shrink-0 items-center justify-center rounded p-0.5 leading-none hover:bg-[var(--td-bg-color-component-hover)]"
              :title="$t('knowledgeStages.viewTrace')"
              :aria-label="$t('knowledgeStages.viewTrace')"
              @click.stop="handleAction('view-trace', item)"
            >
              <ChartLineIcon class="size-3.5" />
            </button>
          </div>
          <div v-else-if="item.parse_status === 'failed'" class="flex h-auto min-h-0 shrink-0 items-center gap-0.5">
            <XCircleIcon class="text-destructive mt-0.5 block size-3.5" />
            <span
              class="text-destructive ml-2 cursor-pointer [font-family:var(--app-font-family)] text-[11px] hover:underline"
              role="button"
              tabindex="0"
              :title="$t('knowledgeStages.viewTrace')"
              @click.stop="handleAction('view-trace', item)"
              @keydown.enter.stop="handleAction('view-trace', item)"
              @keydown.space.prevent.stop="handleAction('view-trace', item)"
              >{{ $t("knowledgeBase.parsingFailed") }}</span
            >
            <button
              type="button"
              data-slot="trace-button"
              class="text-destructive inline-flex shrink-0 items-center justify-center rounded p-0.5 leading-none hover:bg-[var(--td-bg-color-component-hover)]"
              :title="$t('knowledgeStages.viewTrace')"
              :aria-label="$t('knowledgeStages.viewTrace')"
              @click.stop="handleAction('view-trace', item)"
            >
              <ChartColumnIcon class="size-3.5" />
            </button>
          </div>
          <div v-else-if="item.parse_status === 'draft'" class="flex shrink-0 items-center gap-2 py-1.5">
            <Badge
              variant="outline"
              class="text-warning h-5 rounded-[3px] border-[var(--td-warning-color-3)] bg-[var(--td-warning-color-1)] px-1.5 font-normal"
              >{{ $t("knowledgeBase.draft") }}</Badge
            >
            <span class="text-warning text-[11px]">{{ $t("knowledgeBase.draftTip") }}</span>
          </div>
          <div
            v-else-if="
              item.parse_status === 'completed' &&
              (item.summary_status === 'pending' || item.summary_status === 'processing')
            "
            class="flex h-[52px] shrink-0 items-start"
          >
            <Loader2Icon class="text-primary mt-0.5 block size-3.5 animate-spin" />
            <span class="text-primary ml-2 [font-family:var(--app-font-family)] text-[11px]">{{
              $t("knowledgeBase.generatingSummary")
            }}</span>
          </div>
          <div
            v-else-if="item.parse_status === 'completed'"
            class="text-muted-foreground line-clamp-2 min-h-0 flex-1 [font-family:var(--app-font-family)] text-xs leading-[19px] font-normal"
          >
            {{ item.description }}
          </div>
        </div>

        <div
          class="bg-card mt-auto box-border flex h-8 w-full shrink-0 items-center justify-between border-t border-[var(--td-component-stroke)] px-3.5"
        >
          <button
            v-if="showFolderPath && item.folder_path"
            type="button"
            data-slot="folder-link"
            class="text-muted-foreground hover:text-primary inline-flex max-w-[60%] min-w-0 items-center gap-1 [font-family:var(--app-font-family)] text-xs transition-colors"
            :title="item.folder_path"
            @click.stop="emit('open-folder', item.folder_path)"
          >
            <FolderIcon class="size-[13px] shrink-0" />
            <span class="min-w-0 truncate">{{ item.folder_path }}</span>
          </button>
          <span
            v-else
            class="text-muted-foreground shrink-0 [font-family:var(--app-font-family)] text-xs font-normal whitespace-nowrap"
          >
            {{ formatDocTime(item.updated_at) }}
          </span>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-1.5 overflow-hidden">
            <div v-if="tagList.length" class="flex items-center" @click.stop>
              <!-- Editable mode -->
              <template v-if="canEdit">
                <template v-if="(item.tags || []).length > 0">
                  <Tooltip v-if="hasTagOverflow(item.id, (item.tags || []).length)">
                    <TooltipTrigger as-child>
                      <div
                        class="inline-flex cursor-pointer flex-nowrap items-center gap-1"
                        :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
                        @click="emit('tag-edit', item)"
                      >
                        <Badge
                          v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                          :key="tag.id"
                          variant="outline"
                          :class="cardTagChipClass"
                        >
                          <span class="inline-block max-w-[80px] truncate align-middle text-[11px]">{{
                            tag.name
                          }}</span>
                        </Badge>
                        <span
                          class="text-placeholder hover:border-primary hover:text-primary hover:bg-muted inline-flex h-[18px] min-w-[18px] cursor-pointer items-center justify-center rounded-full border border-[var(--td-component-stroke)] px-1.25 text-[10px] leading-none transition-all"
                          >+{{ getOverflowCount(item.id, (item.tags || []).length) }}</span
                        >
                      </div>
                    </TooltipTrigger>
                    <TooltipContent side="top">{{
                      (item.tags || []).map((t: any) => t.name).join(", ")
                    }}</TooltipContent>
                  </Tooltip>
                  <div
                    v-else
                    class="inline-flex cursor-pointer flex-nowrap items-center gap-1"
                    :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
                    @click="emit('tag-edit', item)"
                  >
                    <Badge
                      v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                      :key="tag.id"
                      variant="outline"
                      :class="cardTagChipClass"
                    >
                      <span class="inline-block max-w-[80px] truncate align-middle text-[11px]">{{ tag.name }}</span>
                    </Badge>
                  </div>
                </template>
                <span
                  v-else
                  class="text-placeholder hover:border-primary hover:bg-muted inline-flex h-[18px] cursor-pointer items-center gap-0.5 rounded-full border border-dashed border-[var(--td-component-stroke)] px-1.5 text-[11px] transition-all hover:border-solid hover:text-[var(--td-brand-color-active)]"
                  @click="emit('tag-edit', item)"
                >
                  <PlusIcon class="size-3" />
                  <span>{{ $t("knowledgeBase.tagLabel") }}</span>
                </span>
              </template>
              <!-- Read-only mode -->
              <template v-else-if="(item.tags || []).length > 0">
                <Tooltip v-if="hasTagOverflow(item.id, (item.tags || []).length)">
                  <TooltipTrigger as-child>
                    <div
                      class="inline-flex flex-nowrap items-center gap-1"
                      :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
                    >
                      <Badge
                        v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                        :key="tag.id"
                        variant="outline"
                        :class="cardTagChipClass"
                      >
                        <span class="inline-block max-w-[80px] truncate align-middle text-[11px]">{{ tag.name }}</span>
                      </Badge>
                      <span
                        class="text-placeholder hover:border-primary hover:text-primary hover:bg-muted inline-flex h-[18px] min-w-[18px] cursor-pointer items-center justify-center rounded-full border border-[var(--td-component-stroke)] px-1.25 text-[10px] leading-none transition-all"
                        >+{{ getOverflowCount(item.id, (item.tags || []).length) }}</span
                      >
                    </div>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ (item.tags || []).map((t: any) => t.name).join(", ") }}</TooltipContent>
                </Tooltip>
                <div
                  v-else
                  class="inline-flex flex-nowrap items-center gap-1"
                  :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
                >
                  <Badge
                    v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                    :key="tag.id"
                    variant="outline"
                    :class="cardTagChipClass"
                  >
                    <span class="inline-block max-w-[80px] truncate align-middle text-[11px]">{{ tag.name }}</span>
                  </Badge>
                </div>
              </template>
            </div>
            <div
              class="text-placeholder shrink-0 p-0 [font-family:var(--app-font-family)] text-[11px] font-medium tracking-[0.02em]"
            >
              <span>{{ getKnowledgeType(item) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Hover popover -->
  <Teleport to="body">
    <div
      v-show="hoveredCardItem"
      class="knowledge-card-hover-popover bg-card pointer-events-none fixed z-[9999] box-border max-w-[360px] min-w-[220px] [transform:translateZ(0)] rounded-[8px] border border-[var(--td-component-stroke)] px-3.5 py-3 [font-family:var(--app-font-family)] shadow-[0_4px_16px_rgba(0,0,0,0.12)] transition-opacity [will-change:transform] [backface-visibility:hidden]"
      :style="{ left: cardPopoverPos.x + 'px', top: cardPopoverPos.y + 'px' }"
    >
      <template v-if="hoveredCardItem">
        <div class="text-foreground mb-2 truncate text-sm font-semibold">{{ hoveredCardItem.file_name }}</div>
        <div
          v-if="isParseInFlight(hoveredCardItem.parse_status)"
          class="text-primary mb-1.5 flex items-center gap-1.5 text-xs"
        >
          <KnowledgeProcessingTimeline
            :knowledge-id="hoveredCardItem.id"
            :parse-status="hoveredCardItem.parse_status"
            :auto-poll="false"
            :compact="true"
          />
        </div>
        <div
          v-else-if="hoveredCardItem.parse_status === 'failed'"
          class="text-destructive mb-1.5 flex items-center gap-1.5 text-xs"
        >
          <KnowledgeProcessingTimeline
            :knowledge-id="hoveredCardItem.id"
            :parse-status="hoveredCardItem.parse_status"
            :auto-poll="false"
            :compact="true"
          />
        </div>
        <div
          v-else-if="hoveredCardItem.parse_status === 'draft'"
          class="text-warning mb-1.5 flex items-center gap-1.5 text-xs"
        >
          {{ $t("knowledgeBase.draft") }}
        </div>
        <template v-else>
          <div
            v-if="hoveredCardItem.description"
            class="text-muted-foreground mb-2 line-clamp-5 text-xs leading-normal"
          >
            {{ hoveredCardItem.description }}
          </div>
          <div
            v-if="(hoveredCardItem as any).source"
            class="text-primary mb-1.5 flex max-w-full items-center gap-1 truncate text-[11px]"
            :title="(hoveredCardItem as any).source"
          >
            <LinkIcon class="size-3 shrink-0" /> {{ (hoveredCardItem as any).source }}
          </div>
          <div class="text-muted-foreground mb-1.5 flex flex-wrap items-center gap-2.5 text-[11px]">
            <span v-if="(hoveredCardItem as any).created_at" class="shrink-0">
              {{ $t("knowledgeBase.createdAt") }}：{{ formatDocTime((hoveredCardItem as any).created_at) }}
            </span>
            <span v-if="formatFileSize((hoveredCardItem as any).file_size)" class="shrink-0">
              {{ formatFileSize((hoveredCardItem as any).file_size) }}
            </span>
          </div>
        </template>
        <div class="text-muted-foreground flex flex-wrap items-center gap-2 text-[11px]">
          <span>{{ $t("knowledgeBase.updatedAt") }}：{{ formatDocTime(hoveredCardItem.updated_at) }}</span>
          <span
            v-if="(hoveredCardItem as any).channel && (hoveredCardItem as any).channel !== 'web'"
            class="text-warning rounded bg-[var(--td-warning-color-light)] px-1.5 py-px"
            >{{ getChannelLabel((hoveredCardItem as any).channel) }}</span
          >
          <div
            v-if="(hoveredCardItem as any).tags && (hoveredCardItem as any).tags.length > 0"
            class="inline-flex max-w-full flex-wrap items-center gap-1"
          >
            <Badge
              v-for="tag in (hoveredCardItem as any).tags"
              :key="tag.id"
              variant="outline"
              :class="popoverTagChipClass"
            >
              <span class="inline-block max-w-[80px] truncate align-middle text-[11px]">{{ tag.name }}</span>
            </Badge>
          </div>
          <span class="bg-muted text-muted-foreground rounded px-1.5 py-px">{{
            getKnowledgeType(hoveredCardItem)
          }}</span>
        </div>
        <div class="text-muted-foreground mt-2 border-t border-[var(--td-component-stroke)] pt-2 text-[11px]">
          {{ $t("knowledgeBase.clickToViewFull") }}
        </div>
      </template>
    </div>
  </Teleport>
</template>

<!--
  The card grid fade-in is a keyframe animation, referenced from the
  `[animation:…]` utility on the grid. Deliberately NOT scoped: a scoped block
  renames its @keyframes and rewrites only the animation declarations inside
  that same block, so the utility (in the global stylesheet) would name a
  keyframe that no longer exists. The `doc-card` prefix keeps the global name
  from colliding with the other contentFadeIn keyframes in the app.
-->
<style>
@keyframes doc-card-fade-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
