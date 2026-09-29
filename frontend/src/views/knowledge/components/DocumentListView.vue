<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  ArrowRightIcon,
  ChevronLeftIcon,
  CircleDotIcon,
  CircleIcon,
  CloudDownloadIcon,
  FolderIcon,
  LinkIcon,
  ListTreeIcon,
  Loader2Icon,
  MoreHorizontalIcon,
  PenLineIcon,
  UploadIcon,
  XCircleIcon,
  type LucideIcon,
} from "@lucide/vue";
import { formatFileSize, getFileIcon } from "@/utils/files";
import { useTagChipsOverflow } from "@/composables/useTagChipsOverflow";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import DocumentActionMenu from "./DocumentActionMenu.vue";
import FolderPickerMenu, { type FolderOption } from "./FolderPickerMenu.vue";
import { fileTypeIcon } from "../utils/fileTypeIcons";

interface Tag {
  id: string;
  name: string;
  color?: string;
}

interface KnowledgeItem {
  id: string;
  file_name: string;
  folder_path?: string;
  file_type?: string;
  file_size?: number | string;
  type?: string;
  tags?: Tag[];
  parse_status?: string;
  summary_status?: string;
  updated_at?: string;
  source?: string;
  description?: string;
  channel?: string;
  isMore?: boolean;
}

const props = defineProps<{
  items: KnowledgeItem[];
  selectedIds: Set<string>;
  canEdit: boolean;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  traceVisibleIds: Record<string, boolean>;
  tagList: Tag[];
  loading?: boolean;
  /** Sub-folders of the folder currently being browsed. */
  folders?: Array<{ path: string; name: string; total_count: number }>;
  /** Every folder of the knowledge base, for the "move to folder" picker. */
  folderOptions?: FolderOption[];
  /**
   * Show each row's folder under its name. Only useful when the list spans
   * several folders, i.e. while filtering; inside one folder the path would be
   * identical on every row.
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
  (e: "open", item: KnowledgeItem): void;
  (e: "toggle-row", id: string, checked: boolean, shiftKey: boolean): void;
  (e: "toggle-all", checked: boolean): void;
  (
    e: "action",
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
    item: KnowledgeItem,
  ): void;
  (e: "probe-trace", item: KnowledgeItem): void;
  (e: "tag-edit", item: KnowledgeItem): void;
  (e: "open-folder", path: string): void;
  (e: "move-to-folder", item: KnowledgeItem, folderPath: string): void;
  // Move sub-flow emits
  (e: "move-select-target", kb: any): void;
  (e: "move-back"): void;
  (e: "move-confirm"): void;
  (e: "update:moveMode", mode: "reuse_vectors" | "reparse"): void;
  (e: "reset-move-state"): void;
}>();

const { t } = useI18n();

const { setupTagChipsObserver, getTagLimit, hasTagOverflow, getOverflowCount } = useTagChipsOverflow("listTagItemId");

const formatTime = (time?: string) => {
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

const getSourceInfo = (item: KnowledgeItem): { icon: LucideIcon; label: string } => {
  const ch = item.channel;
  if (ch === "feishu") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelFeishu") };
  // Drive (云盘) connectors use their own channel so Drive docs show
  // "飞书云盘" / "Lark 云盘", distinct from the wiki connector's "飞书".
  if (ch === "feishu_drive") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelFeishuDrive") };
  if (ch === "lark_drive") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelLarkDrive") };
  if (ch === "notion") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelNotion") };
  if (ch === "yuque") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelYuque") };
  if (ch === "gitlab") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelGitLab") };
  if (ch === "ima") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelIma") };
  if (ch === "wechat") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelWechat") };
  if (ch === "wecom") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelWecom") };
  if (ch === "dingtalk") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelDingtalk") };
  if (ch === "slack") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelSlack") };
  if (ch === "im") return { icon: CloudDownloadIcon, label: t("knowledgeBase.channelIm") };
  if (item.type === "url") return { icon: LinkIcon, label: t("knowledgeBase.channelUrl") };
  if (item.type === "manual") return { icon: PenLineIcon, label: t("knowledgeBase.channelManual") };
  return { icon: UploadIcon, label: t("knowledgeBase.channelUpload") };
};

interface StatusInfo {
  label: string;
  theme: "success" | "warning" | "danger" | "primary" | "default";
  icon?: "loading" | "close-circle";
  spin?: boolean;
}

// TDesign's small light-outline tag, per theme: the theme's lightest tint as
// the fill, a slightly stronger tint as the border, the theme colour as text.
function statusThemeClass(theme: StatusInfo["theme"]): string {
  const base = "h-5 rounded-[3px] px-1.5 font-normal";
  switch (theme) {
    case "success":
      return `${base} border-[var(--td-success-color-3)] bg-[var(--td-success-color-1)] text-success`;
    case "warning":
      return `${base} border-[var(--td-warning-color-3)] bg-[var(--td-warning-color-1)] text-warning`;
    case "danger":
      return `${base} border-[var(--td-error-color-3)] bg-[var(--td-error-color-1)] text-destructive`;
    case "primary":
      return `${base} border-[var(--td-brand-color-3)] bg-[var(--td-brand-color-1)] text-primary`;
    default:
      return `${base} border-border bg-muted text-foreground`;
  }
}

// The same tag shape in the default theme, for the row's document tags.
const tagBadgeClass = "border-border bg-muted text-foreground h-5 max-w-full rounded-[3px] px-1.5 font-normal";

const computeStatus = (item: KnowledgeItem): StatusInfo => {
  if (item.parse_status === "pending" || item.parse_status === "processing") {
    return { label: t("knowledgeBase.statusProcessing"), theme: "primary", icon: "loading", spin: true };
  }
  // finalizing = primary parse done, enrichment subtasks still running.
  // While in this phase, prefer the specific "summary generating" copy
  // when summary is what's actually outstanding (preserves the old UX
  // where this label was tied to completed+summary_pending). Otherwise
  // fall back to the generic "finalizing" label — covers question gen
  // and graph extract, which the user historically had no visibility on.
  if (item.parse_status === "finalizing") {
    if (item.summary_status === "pending" || item.summary_status === "processing") {
      return { label: t("knowledgeBase.generatingSummary"), theme: "primary", icon: "loading", spin: true };
    }
    return { label: t("knowledgeBase.statusFinalizing"), theme: "primary", icon: "loading", spin: true };
  }
  if (item.parse_status === "failed") {
    return { label: t("knowledgeBase.statusFailed"), theme: "danger", icon: "close-circle" };
  }
  if (item.parse_status === "cancelled") {
    return { label: t("knowledgeBase.statusCancelled"), theme: "warning", icon: "close-circle" };
  }
  if (item.parse_status === "draft") {
    return { label: t("knowledgeBase.statusDraft"), theme: "warning" };
  }
  // Legacy completed+summary_pending path: kept as a defensive fallback
  // for rows that bypassed finalizing (no enrichment configured, or
  // upgraded mid-flight from a pre-finalizing build).
  if (
    item.parse_status === "completed" &&
    (item.summary_status === "pending" || item.summary_status === "processing")
  ) {
    return { label: t("knowledgeBase.generatingSummary"), theme: "primary", icon: "loading", spin: true };
  }
  if (item.parse_status === "completed") {
    return { label: t("knowledgeBase.statusCompleted"), theme: "success" };
  }
  return { label: "--", theme: "default" };
};

const statusByRow = computed(() => {
  const map = new Map<string, StatusInfo>();
  for (const item of props.items) map.set(item.id, computeStatus(item));
  return map;
});

const allSelected = computed(() => {
  return props.items.length > 0 && props.items.every((i) => props.selectedIds.has(i.id));
});
const someSelected = computed(() => {
  return props.items.some((i) => props.selectedIds.has(i.id)) && !allSelected.value;
});

const onHeaderCheckboxChange = (checked: boolean | "indeterminate") => {
  emit("toggle-all", checked === true);
};

// Shift-click range selection: captured from the row checkbox's click event
// before the checked update is emitted.
let rowShiftKey = false;
const rememberRowShiftKey = (e: MouseEvent) => {
  rowShiftKey = e.shiftKey;
};

const moreOpen = ref<string | null>(null);
const onMoreVisible = (id: string, visible: boolean) => {
  moreOpen.value = visible ? id : null;
  if (visible) {
    const it = props.items.find((i) => i.id === id);
    if (it) emit("probe-trace", it);
  } else {
    folderPickerItemId.value = null;
    // Reset move state when popup closes naturally
    emit("reset-move-state");
  }
};

// 吸顶检测：哨兵离开视口说明 header 已吸附在滚动容器顶部
const stickySentinel = ref<HTMLElement | null>(null);
const headerStuck = ref(false);
let stickyObserver: IntersectionObserver | null = null;
onMounted(() => {
  if (!stickySentinel.value || typeof IntersectionObserver === "undefined") return;
  stickyObserver = new IntersectionObserver(
    (entries) => {
      headerStuck.value = !entries[0].isIntersecting;
    },
    { threshold: 0 },
  );
  stickyObserver.observe(stickySentinel.value);
});
onBeforeUnmount(() => {
  stickyObserver?.disconnect();
  stickyObserver = null;
});

// Which row's action popup is currently showing the folder picker. Kept local so
// picking a folder stays inside the menu the user already opened, exactly like
// the "move to knowledge base" sub-menu next to it.
const folderPickerItemId = ref<string | null>(null);

const onFolderPicked = (item: KnowledgeItem, path: string) => {
  folderPickerItemId.value = null;
  moreOpen.value = null;
  item.isMore = false;
  emit("move-to-folder", item, path);
};

const handleAction = (
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
  item: KnowledgeItem,
) => {
  // The folder picker opens inside this same popup, so keep the menu open.
  if (action === "move-folder") {
    folderPickerItemId.value = item.id;
    return;
  }
  // Don't close popup for move — it triggers the move sub-flow
  if (action !== "move") {
    moreOpen.value = null;
  }
  item.isMore = false;
  emit("action", action, item);
};

const gridCols =
  "grid-cols-[44px_minmax(260px,2.6fr)_minmax(100px,0.9fr)_minmax(96px,0.8fr)_96px_minmax(96px,0.7fr)_140px_48px]";
</script>

<template>
  <div
    class="bg-card box-border flex w-full [animation:doc-list-fade-in_0.32s_ease-out] flex-col rounded-[9px] border border-[var(--td-component-stroke)] shadow-[0_1px_3px_rgba(0,0,0,0.04)]"
  >
    <div ref="stickySentinel" class="pointer-events-none m-0 h-0 border-0 p-0" aria-hidden="true" />
    <div
      class="bg-muted text-muted-foreground sticky top-0 z-[3] grid h-10 items-center gap-0 border-b border-[var(--td-component-stroke)] px-4 [font-family:var(--app-font-family)] text-xs font-medium transition-[border-radius,box-shadow] duration-200"
      :class="[
        gridCols,
        headerStuck
          ? 'rounded-none shadow-[0_4px_10px_rgba(0,0,0,0.08)]'
          : 'rounded-t-[8px] shadow-[0_2px_8px_rgba(0,0,0,0.04)]',
      ]"
      role="row"
    >
      <div class="flex min-w-0 items-center justify-center p-0" role="columnheader" @click.stop>
        <Checkbox
          :model-value="allSelected ? true : someSelected ? 'indeterminate' : false"
          :disabled="!items.length"
          :title="t('knowledgeBase.selectAll')"
          :aria-label="t('knowledgeBase.selectAll')"
          @update:model-value="onHeaderCheckboxChange"
        />
      </div>
      <div class="flex min-w-0 items-center px-2" role="columnheader">
        {{ t("knowledgeBase.columnName") }}
      </div>
      <div class="flex min-w-0 items-center px-2" role="columnheader">
        {{ t("knowledgeBase.columnTag") }}
      </div>
      <div class="flex min-w-0 items-center px-2" role="columnheader">
        {{ t("knowledgeBase.columnSource") }}
      </div>
      <div class="flex min-w-0 items-center justify-end px-2" role="columnheader">
        {{ t("knowledgeBase.columnSize") }}
      </div>
      <div class="flex min-w-0 items-center px-2" role="columnheader">
        {{ t("knowledgeBase.columnStatus") }}
      </div>
      <div class="flex min-w-0 items-center justify-end px-2" role="columnheader">
        {{ t("knowledgeBase.columnUpdatedAt") }}
      </div>
      <div v-if="canEdit" class="flex min-w-0 items-center justify-end p-0" role="columnheader" />
    </div>

    <div class="flex flex-col overflow-hidden rounded-b-[8px]">
      <div
        v-for="folder in folders"
        :key="'folder-' + folder.path"
        class="doc-list-row text-foreground hover:bg-muted relative grid min-h-[60px] cursor-pointer items-center border-b border-[var(--td-component-stroke)] px-4 text-[13px] transition-colors duration-200 last:border-b-0"
        :class="gridCols"
        :title="folder.path"
        role="row"
        @click="emit('open-folder', folder.path)"
      >
        <div class="flex min-w-0 items-center justify-center p-0" aria-hidden="true" />
        <div class="flex min-w-0 items-center gap-2.5 px-2 [font-family:var(--app-font-family)]">
          <span
            class="bg-muted text-muted-foreground inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-base"
          >
            <FolderIcon class="text-primary size-4" />
          </span>
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="text-foreground truncate text-sm font-medium">{{ folder.name }}</span>
          </div>
        </div>
        <div class="flex min-w-0 items-center px-2" />
        <div class="flex min-w-0 items-center gap-1.5 px-2">
          <span class="text-placeholder text-xs">{{
            t("knowledgeBase.folderTree.folderCardCount", { count: folder.total_count })
          }}</span>
        </div>
        <div class="flex min-w-0 items-center justify-end px-2" />
        <div class="flex min-w-0 items-center px-2" />
        <div class="flex min-w-0 items-center justify-end px-2" />
        <div v-if="canEdit" class="flex min-w-0 items-center justify-end p-0" aria-hidden="true" />
      </div>

      <div
        v-for="item in items"
        :key="item.id"
        class="doc-list-row group text-foreground relative grid min-h-[60px] cursor-pointer items-center border-b border-[var(--td-component-stroke)] px-4 text-[13px] transition-colors duration-200 last:border-b-0"
        :class="[
          gridCols,
          {
            'hover:bg-muted': !selectedIds.has(item.id),
            'bg-muted': moreOpen === item.id && !selectedIds.has(item.id),
          },
        ]"
        :data-select-id="item.id"
        role="row"
        @click="emit('open', item)"
      >
        <div class="flex min-w-0 items-center justify-center p-0" @click.stop>
          <Checkbox
            :model-value="selectedIds.has(item.id)"
            :title="item.file_name"
            :aria-label="item.file_name"
            @click="rememberRowShiftKey"
            @update:model-value="(c) => emit('toggle-row', item.id, c === true, rowShiftKey)"
          />
        </div>

        <div class="flex min-w-0 items-center gap-2.5 px-2 [font-family:var(--app-font-family)]">
          <span
            class="bg-muted text-muted-foreground inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-base"
          >
            <component :is="fileTypeIcon(getFileIcon(item))" class="size-4" />
          </span>
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="text-foreground truncate text-sm font-semibold tracking-[0.01em]" :title="item.file_name">
              {{ item.file_name }}
            </span>
            <button
              v-if="showFolderPath && item.folder_path"
              type="button"
              data-slot="folder-link"
              class="text-placeholder hover:text-primary inline-flex max-w-full items-center gap-1 self-start [font-family:var(--app-font-family)] text-xs transition-colors"
              :title="item.folder_path"
              @click.stop="emit('open-folder', item.folder_path)"
            >
              <FolderIcon class="size-[13px] shrink-0" />
              <span class="min-w-0 truncate">{{ item.folder_path }}</span>
            </button>
            <span v-if="item.description" class="text-placeholder min-w-0 truncate text-xs" :title="item.description">
              {{ item.description }}
            </span>
          </div>
        </div>

        <div class="flex min-w-0 items-center px-2">
          <template v-if="item.tags && item.tags.length > 0">
            <Tooltip v-if="hasTagOverflow(item.id, (item.tags || []).length)">
              <TooltipTrigger as-child>
                <div
                  :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
                  class="inline-flex flex-nowrap items-center gap-1"
                  :class="canEdit ? 'cursor-pointer' : ''"
                  @click.stop="canEdit && emit('tag-edit', item)"
                >
                  <Badge
                    v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                    :key="tag.id"
                    variant="outline"
                    :class="tagBadgeClass"
                  >
                    <span class="inline-block max-w-[120px] truncate">{{ tag.name }}</span>
                  </Badge>
                  <span
                    class="text-placeholder hover:border-primary hover:text-primary hover:bg-muted inline-flex h-5 min-w-5 cursor-pointer items-center justify-center rounded-full border border-[var(--td-component-stroke)] px-1 text-[10px] leading-none transition-all"
                    >+{{ getOverflowCount(item.id, (item.tags || []).length) }}</span
                  >
                </div>
              </TooltipTrigger>
              <TooltipContent side="top">{{ (item.tags || []).map((t: any) => t.name).join(", ") }}</TooltipContent>
            </Tooltip>
            <div
              v-else
              :ref="(el: any) => setupTagChipsObserver(el, item.id, (item.tags || []).length)"
              class="inline-flex flex-nowrap items-center gap-1"
              :class="canEdit ? 'cursor-pointer' : ''"
              @click.stop="canEdit && emit('tag-edit', item)"
            >
              <Badge
                v-for="tag in (item.tags || []).slice(0, getTagLimit(item.id))"
                :key="tag.id"
                variant="outline"
                :class="tagBadgeClass"
              >
                <span class="inline-block max-w-[120px] truncate">{{ tag.name }}</span>
              </Badge>
            </div>
          </template>
          <span
            v-else
            class="inline-flex items-center gap-1"
            :class="canEdit ? 'cursor-pointer' : ''"
            @click.stop="canEdit && emit('tag-edit', item)"
          >
            <span
              class="text-placeholder hover:border-primary hover:text-primary hover:bg-muted inline-flex h-5 items-center rounded-full border border-dashed border-[var(--td-component-stroke)] px-1.5 text-[11px] whitespace-nowrap hover:border-solid"
              >+ {{ t("knowledgeBase.tagLabel") }}</span
            >
          </span>
        </div>

        <div class="flex min-w-0 items-center gap-1.5 px-2">
          <component :is="getSourceInfo(item).icon" class="text-muted-foreground size-3.5 shrink-0" />
          <span class="text-muted-foreground min-w-0 truncate text-xs">{{ getSourceInfo(item).label }}</span>
        </div>

        <div class="flex min-w-0 items-center justify-end px-2">
          <span
            class="text-muted-foreground [font-family:var(--app-font-family)] text-xs [font-variant-numeric:tabular-nums]"
          >
            {{ formatFileSize(item.file_size) || "--" }}
          </span>
        </div>

        <div class="flex min-w-0 items-center px-2">
          <template v-if="statusByRow.get(item.id) as StatusInfo | undefined">
            <Badge
              v-if="statusByRow.get(item.id)!.label !== '--'"
              variant="outline"
              :class="statusThemeClass(statusByRow.get(item.id)!.theme)"
            >
              <Loader2Icon
                v-if="statusByRow.get(item.id)!.icon === 'loading'"
                class="mr-0.5 size-3"
                :class="statusByRow.get(item.id)!.spin ? 'animate-spin' : ''"
              />
              <XCircleIcon v-else-if="statusByRow.get(item.id)!.icon === 'close-circle'" class="mr-0.5 size-3" />
              {{ statusByRow.get(item.id)!.label }}
            </Badge>
            <span v-else class="text-[var(--td-text-color-disabled,#bbb)]">--</span>
          </template>
        </div>

        <div class="flex min-w-0 items-center justify-end px-2">
          <span
            class="text-muted-foreground [font-family:var(--app-font-family)] text-xs [font-variant-numeric:tabular-nums]"
          >
            {{ formatTime(item.updated_at) }}
          </span>
        </div>

        <div v-if="canEdit" class="flex min-w-0 items-center justify-end p-0" @click.stop>
          <Popover :open="moreOpen === item.id" @update:open="(v: boolean) => onMoreVisible(item.id, v)">
            <PopoverTrigger as-child>
              <!-- Shown on row hover, while its menu is open, and on selected rows. -->
              <button
                data-slot="row-more"
                class="hover:text-foreground inline-flex size-7 items-center justify-center rounded-[5px] transition-all hover:bg-[var(--td-component-stroke)] focus-visible:opacity-100"
                :class="[
                  moreOpen === item.id
                    ? 'text-foreground bg-[var(--td-component-stroke)]'
                    : 'text-muted-foreground bg-transparent',
                  moreOpen === item.id || selectedIds.has(item.id)
                    ? 'opacity-100'
                    : 'opacity-0 group-hover:opacity-100',
                ]"
                type="button"
                :aria-label="t('knowledgeBase.columnActions')"
              >
                <MoreHorizontalIcon class="size-4" />
              </button>
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
                  :trace-visible="
                    !!traceVisibleIds[item.id] ||
                    item.parse_status === 'pending' ||
                    item.parse_status === 'processing' ||
                    item.parse_status === 'finalizing'
                  "
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

              <!-- Move: confirm with mode selection -->
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
      </div>
    </div>
  </div>
</template>

<!--
  The list fade-in is a keyframe animation, referenced from the `[animation:…]`
  utility on the list. Deliberately NOT scoped: a scoped block renames its
  @keyframes and rewrites only the animation declarations inside that same
  block, so the utility (in the global stylesheet) would name a keyframe that
  no longer exists. The name is already specific to this view.
-->
<style>
@keyframes doc-list-fade-in {
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
