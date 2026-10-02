<template>
  <!--
    The width animates between the collapsed strip (56px) and the expanded
    panel (208px); while dragging, the inline width follows the pointer and
    the transition is off so the edge does not lag behind it.
  -->
  <div
    ref="sidebarRef"
    class="relative z-10 flex min-h-0 shrink-0 flex-col"
    :class="[
      isExpanded ? 'mr-0 w-[208px]' : 'w-14',
      isDragging ? 'transition-none' : 'transition-[width] duration-[250ms] ease-[cubic-bezier(0.4,0,0.2,1)]',
    ]"
    :style="{ width: isDragging ? `${dragWidth}px` : undefined }"
  >
    <!-- Collapsed: icon strip -->
    <div
      v-if="!isExpanded"
      class="flex min-h-0 w-14 flex-1 [scrollbar-width:none] flex-col items-center gap-1 overflow-x-hidden overflow-y-auto pt-3 pb-1.5 [&::-webkit-scrollbar]:hidden"
    >
      <Tooltip>
        <TooltipTrigger as-child>
          <div :class="stripItemClass(selected === 'mine')" @click="select('mine')">
            <AtomIcon class="size-4" />
            <span :class="stripLabelClass(selected === 'mine')">{{ workspaceLabel }}</span>
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">{{ tooltipText(workspaceLabel, countMine) }}</TooltipContent>
      </Tooltip>
      <Tooltip v-if="showFavorites">
        <TooltipTrigger as-child>
          <div :class="stripItemClass(selected === 'favorites')" @click="select('favorites')">
            <StarIcon class="size-4" />
            <span :class="stripLabelClass(selected === 'favorites')">{{ $t("listSpaceSidebar.favorites") }}</span>
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">{{
          tooltipText($t("listSpaceSidebar.favorites"), countFavorites)
        }}</TooltipContent>
      </Tooltip>
      <Tooltip v-if="showRecents">
        <TooltipTrigger as-child>
          <div :class="stripItemClass(selected === 'recents')" @click="select('recents')">
            <HistoryIcon class="size-4" />
            <span :class="stripLabelClass(selected === 'recents')">{{ $t("listSpaceSidebar.recents") }}</span>
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">{{ tooltipText($t("listSpaceSidebar.recents"), countRecents) }}</TooltipContent>
      </Tooltip>
    </div>

    <!-- Expanded: full nav panel -->
    <nav
      v-else
      class="border-border flex min-h-0 flex-1 [scrollbar-width:none] flex-col gap-0.5 overflow-x-hidden overflow-y-auto border-r px-2 py-3 [&::-webkit-scrollbar]:hidden"
    >
      <div :class="navItemClass(selected === 'mine')" @click="select('mine')">
        <div class="flex min-w-0 flex-1 items-center gap-1.5">
          <AtomIcon :class="navIconClass(selected === 'mine')" />
          <span :class="navLabelClass">{{ workspaceLabel }}</span>
        </div>
        <span v-if="countMine !== undefined" :class="navCountClass(selected === 'mine')">{{ countMine }}</span>
      </div>
      <div v-if="showFavorites || showRecents" class="bg-border mx-1 my-1.5 h-px" />
      <div v-if="showFavorites" :class="navItemClass(selected === 'favorites')" @click="select('favorites')">
        <div class="flex min-w-0 flex-1 items-center gap-1.5">
          <StarIcon :class="navIconClass(selected === 'favorites')" />
          <span :class="navLabelClass">{{ $t("listSpaceSidebar.favorites") }}</span>
        </div>
        <span v-if="countFavorites > 0" :class="navCountClass(selected === 'favorites')">{{ countFavorites }}</span>
      </div>
      <div v-if="showRecents" :class="navItemClass(selected === 'recents')" @click="select('recents')">
        <div class="flex min-w-0 flex-1 items-center gap-1.5">
          <HistoryIcon :class="navIconClass(selected === 'recents')" />
          <span :class="navLabelClass">{{ $t("listSpaceSidebar.recents") }}</span>
        </div>
        <span v-if="countRecents > 0" :class="navCountClass(selected === 'recents')">{{ countRecents }}</span>
      </div>
    </nav>

    <!-- Drag handle on the right edge -->
    <div
      class="group absolute top-0 -right-1.5 bottom-0 z-[12] flex w-3 cursor-col-resize items-center justify-center"
      @mousedown.prevent="onDragStart"
    >
      <div
        class="h-10 w-0.5 rounded-[1px] transition-[opacity,background] duration-200 ease-in-out"
        :class="
          isDragging
            ? 'bg-primary opacity-100'
            : 'group-hover:bg-primary bg-[var(--td-bg-color-component-disabled)] opacity-45 group-hover:opacity-100'
        "
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { AtomIcon, HistoryIcon, StarIcon } from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/**
 * The scope rail of a resource list: the workspace's own resources plus the
 * two per-user views (starred, recent). It once also listed the shared spaces
 * a knowledge base could reach the user through; knowledge bases are now only
 * ever visible from their own workspace, so the rail is these three entries.
 *
 * The workspace entry keeps the stored value "mine" so bookmarked
 * `?scope=mine` links keep working; the label it shows is the generic
 * "Workspace", because the active workspace is already named by the
 * TenantSelector in the header and a long name truncated to a stub in the
 * collapsed strip conveyed nothing.
 */

const COLLAPSED_WIDTH = 56;
const EXPANDED_WIDTH = 208;
const SNAP_THRESHOLD = 120;

const props = withDefaults(
  defineProps<{
    modelValue: string;
    collapsedKey?: string;
    countMine?: number;
    countFavorites?: number;
    showFavorites?: boolean;
    countRecents?: number;
    showRecents?: boolean;
  }>(),
  {
    collapsedKey: "sidebar-collapsed-list",
    countMine: undefined,
    countFavorites: 0,
    showFavorites: true,
    countRecents: 0,
    showRecents: true,
  },
);

const storageKey = props.collapsedKey + "-expanded";
const sidebarRef = ref<HTMLElement | null>(null);
const isExpanded = ref(localStorage.getItem(storageKey) === "true");
const isDragging = ref(false);
const dragWidth = ref(isExpanded.value ? EXPANDED_WIDTH : COLLAPSED_WIDTH);

let startX = 0;
let startWidth = 0;

function onDragStart(e: MouseEvent) {
  isDragging.value = true;
  startX = e.clientX;
  startWidth = isExpanded.value ? EXPANDED_WIDTH : COLLAPSED_WIDTH;
  dragWidth.value = startWidth;
  document.addEventListener("mousemove", onDragMove);
  document.addEventListener("mouseup", onDragEnd);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function onDragMove(e: MouseEvent) {
  const delta = e.clientX - startX;
  const newWidth = Math.max(COLLAPSED_WIDTH, Math.min(EXPANDED_WIDTH + 20, startWidth + delta));
  dragWidth.value = newWidth;
}

function onDragEnd() {
  document.removeEventListener("mousemove", onDragMove);
  document.removeEventListener("mouseup", onDragEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";

  const shouldExpand = dragWidth.value >= SNAP_THRESHOLD;
  isExpanded.value = shouldExpand;
  localStorage.setItem(storageKey, String(shouldExpand));
  isDragging.value = false;
  dragWidth.value = shouldExpand ? EXPANDED_WIDTH : COLLAPSED_WIDTH;
}

// Row styles. Each state is spelled out, so the active row's brand colour and
// the hover colour never depend on which rule happens to come later.
function stripItemClass(active: boolean): string {
  return [
    "flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg pt-[5px] pb-0.5 transition-all duration-150 ease-in-out",
    active ? "bg-secondary text-primary" : "text-muted-foreground hover:bg-accent hover:text-foreground",
  ].join(" ");
}

function stripLabelClass(active: boolean): string {
  return [
    "max-w-[52px] truncate text-center text-[11px] leading-[1.25] transition-colors duration-150 ease-in-out",
    active ? "text-primary" : "text-muted-foreground",
  ].join(" ");
}

function navItemClass(active: boolean): string {
  return [
    "group flex cursor-pointer items-center justify-between rounded-[7px] px-2 py-1.5 font-[family-name:var(--app-font-family)] text-sm antialiased transition-all duration-150 ease-in-out",
    active ? "bg-secondary text-primary" : "text-foreground hover:bg-accent",
  ].join(" ");
}

function navIconClass(active: boolean): string {
  return [
    "size-3.5 shrink-0 transition-colors duration-150 ease-in-out",
    active ? "text-primary" : "text-muted-foreground group-hover:text-foreground",
  ].join(" ");
}

const navLabelClass = "min-w-0 flex-1 truncate text-[13px] leading-[1.4] font-[430] tracking-[0.01em]";

function navCountClass(active: boolean): string {
  return [
    "bg-secondary ml-1.5 shrink-0 rounded-lg px-[7px] py-0.5 text-xs font-medium transition-all duration-150 ease-in-out",
    active ? "text-primary" : "text-muted-foreground group-hover:text-foreground",
  ].join(" ");
}

function tooltipText(name: string, count?: number): string {
  return count !== undefined ? `${name} (${count})` : name;
}

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const { t } = useI18n();
const selected = computed({
  get: () => props.modelValue,
  set: (v: string) => emit("update:modelValue", v),
});

const workspaceLabel = computed(() => t("listSpaceSidebar.workspace"));

function select(value: string) {
  selected.value = value;
}

onBeforeUnmount(() => {
  document.removeEventListener("mousemove", onDragMove);
  document.removeEventListener("mouseup", onDragEnd);
});
</script>
