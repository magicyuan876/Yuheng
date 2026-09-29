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
      <template v-if="mode === 'resource'">
        <Tooltip v-if="!hideAll">
          <TooltipTrigger as-child>
            <div :class="stripItemClass(selected === 'all')" @click="select('all')">
              <LayersIcon class="size-4" />
              <span :class="stripLabelClass(selected === 'all')">{{ $t("listSpaceSidebar.all") }}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ tooltipText($t("listSpaceSidebar.all"), countAll) }}</TooltipContent>
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
        <Tooltip>
          <TooltipTrigger as-child>
            <div :class="stripItemClass(selected === 'mine')" @click="select('mine')">
              <AtomIcon class="size-4" />
              <span :class="stripLabelClass(selected === 'mine')">{{ workspaceLabel }}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ tooltipText(workspaceLabel, countMine) }}</TooltipContent>
        </Tooltip>
        <!-- Shared spaces group: per-org/space entries only. We dropped
             the aggregate "协作" / shared-with-me entry — its meaning
             oscillated between "everything shared to me" and "things I
             can edit", and either reading duplicated information already
             visible on the per-space entries below. -->
        <template v-if="organizationsWithCount.length">
          <div class="bg-secondary my-[3px] h-px w-6 shrink-0" />
          <Tooltip v-for="org in organizationsWithCount" :key="org.id">
            <TooltipTrigger as-child>
              <div :class="stripItemClass(selected === org.id)" @click="select(org.id)">
                <!-- The strip draws the avatar a touch smaller than SpaceAvatar's small size. -->
                <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" class="size-5! text-[10px]" />
                <span :class="stripLabelClass(selected === org.id)">{{ truncateLabel(org.name) }}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="right">{{ tooltipText(org.name, getOrgCount(org.id)) }}</TooltipContent>
          </Tooltip>
        </template>
      </template>

      <template v-else>
        <Tooltip>
          <TooltipTrigger as-child>
            <div :class="stripItemClass(selected === 'all')" @click="select('all')">
              <LayersIcon class="size-4" />
              <span :class="stripLabelClass(selected === 'all')">{{ $t("listSpaceSidebar.all") }}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ tooltipText($t("listSpaceSidebar.all"), countAll) }}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <div :class="stripItemClass(selected === 'created')" @click="select('created')">
              <UserPlusIcon class="size-4" />
              <span :class="stripLabelClass(selected === 'created')">{{ $t("organization.createdByMe") }}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ tooltipText($t("organization.createdByMe"), countCreated) }}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <div :class="stripItemClass(selected === 'joined')" @click="select('joined')">
              <UsersIcon class="size-4" />
              <span :class="stripLabelClass(selected === 'joined')">{{ $t("organization.joinedByMe") }}</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ tooltipText($t("organization.joinedByMe"), countJoined) }}</TooltipContent>
        </Tooltip>
      </template>
    </div>

    <!-- Expanded: full nav panel -->
    <nav
      v-else
      class="border-border flex min-h-0 flex-1 [scrollbar-width:none] flex-col gap-0.5 overflow-x-hidden overflow-y-auto border-r px-2 py-3 [&::-webkit-scrollbar]:hidden"
    >
      <div v-if="!hideAll" :class="navItemClass(selected === 'all')" @click="select('all')">
        <div class="flex min-w-0 flex-1 items-center gap-1.5">
          <LayersIcon :class="navIconClass(selected === 'all')" />
          <span :class="navLabelClass">{{ $t("listSpaceSidebar.all") }}</span>
        </div>
        <span v-if="countAll !== undefined" :class="navCountClass(selected === 'all')">{{ countAll }}</span>
      </div>

      <template v-if="mode === 'resource'">
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
        <div v-if="showFavorites || showRecents" class="bg-border mx-1 my-1.5 h-px" />
        <div :class="navItemClass(selected === 'mine')" @click="select('mine')">
          <div class="flex min-w-0 flex-1 items-center gap-1.5">
            <AtomIcon :class="navIconClass(selected === 'mine')" />
            <span :class="navLabelClass">{{ workspaceLabel }}</span>
          </div>
          <span v-if="countMine !== undefined" :class="navCountClass(selected === 'mine')">{{ countMine }}</span>
        </div>
        <!-- Shared spaces group — per-org entries only; the aggregate
             entry was removed (see collapsed strip for rationale). -->
        <template v-if="organizationsWithCount.length">
          <div class="border-border mt-0.5 border-t px-1.5 pt-2 pb-0.5">
            <span class="text-muted-foreground text-xs leading-[1.4] font-semibold">{{
              $t("listSpaceSidebar.spaces")
            }}</span>
          </div>
          <div
            v-for="org in organizationsWithCount"
            :key="org.id"
            :class="navItemClass(selected === org.id)"
            @click="select(org.id)"
          >
            <div class="flex min-w-0 flex-1 items-center gap-1.5">
              <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" class="shrink-0" />
              <span :class="navLabelClass" :title="org.name">{{ org.name }}</span>
            </div>
            <span v-if="getOrgCount(org.id) !== undefined" :class="navCountClass(selected === org.id)">{{
              getOrgCount(org.id)
            }}</span>
          </div>
        </template>
      </template>

      <template v-else>
        <div :class="navItemClass(selected === 'created')" @click="select('created')">
          <div class="flex min-w-0 flex-1 items-center gap-1.5">
            <UserPlusIcon :class="navIconClass(selected === 'created')" />
            <span :class="navLabelClass">{{ $t("organization.createdByMe") }}</span>
          </div>
          <span v-if="countCreated !== undefined" :class="navCountClass(selected === 'created')">{{
            countCreated
          }}</span>
        </div>
        <div :class="navItemClass(selected === 'joined')" @click="select('joined')">
          <div class="flex min-w-0 flex-1 items-center gap-1.5">
            <UsersIcon :class="navIconClass(selected === 'joined')" />
            <span :class="navLabelClass">{{ $t("organization.joinedByMe") }}</span>
          </div>
          <span v-if="countJoined !== undefined" :class="navCountClass(selected === 'joined')">{{ countJoined }}</span>
        </div>
      </template>
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
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { AtomIcon, HistoryIcon, LayersIcon, StarIcon, UserPlusIcon, UsersIcon } from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SpaceAvatar from "./SpaceAvatar.vue";
import { useOrganizationStore } from "@/stores/organization";

const COLLAPSED_WIDTH = 56;
const EXPANDED_WIDTH = 208;
const SNAP_THRESHOLD = 120;

const props = withDefaults(
  defineProps<{
    mode?: "resource" | "organization";
    modelValue: string;
    collapsedKey?: string;
    countAll?: number;
    countMine?: number;
    countByOrg?: Record<string, number>;
    countCreated?: number;
    countJoined?: number;
    hideAll?: boolean;
    /** Favorites entry. Only meaningful in resource mode. */
    countFavorites?: number;
    showFavorites?: boolean;
    /** Recents entry. Only meaningful in resource mode. */
    countRecents?: number;
    showRecents?: boolean;
  }>(),
  {
    mode: "resource",
    collapsedKey: "sidebar-collapsed-list",
    countAll: undefined,
    countMine: undefined,
    countByOrg: () => ({}),
    countCreated: undefined,
    countJoined: undefined,
    hideAll: false,
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

// truncateLabel keeps the collapsed-strip label visually balanced (~44px
// wide). 4 CJK chars fits; ASCII can stretch further. Callers that want
// the full label should pass it as :title= on the same element for hover.
function truncateLabel(text: string, max = 4): string {
  if (!text) return "";
  return text.length > max ? text.slice(0, max) + "…" : text;
}

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const orgStore = useOrganizationStore();
const { t } = useI18n();
const selected = computed({
  get: () => props.modelValue,
  set: (v: string) => emit("update:modelValue", v),
});

// workspaceLabel is the unified label for the tenant-owned bucket.
// Earlier iterations rendered the active tenant's display name here, but
// long names (e.g. "wizardlab Test Team") got truncated to unreadable
// stubs ("wiza…") in the collapsed strip and competed visually with the
// org/space entries below. A constant i18n label sidesteps both issues;
// the tenant identity is already conveyed by the dedicated TenantSelector
// in the global header, so we don't lose information.
const workspaceLabel = computed(() => t("listSpaceSidebar.workspace"));

const organizations = computed(() => orgStore.organizations || []);

const organizationsWithCount = computed(() => {
  if (props.mode !== "resource") return organizations.value;
  return organizations.value.filter((org) => (props.countByOrg?.[org.id] ?? 0) > 0);
});

function select(value: string) {
  selected.value = value;
}

function getOrgCount(orgId: string): number | undefined {
  const n = props.countByOrg?.[orgId];
  return n === undefined ? undefined : n;
}

onMounted(() => {
  orgStore.fetchOrganizations();
});

onBeforeUnmount(() => {
  document.removeEventListener("mousemove", onDragMove);
  document.removeEventListener("mouseup", onDragEnd);
});
</script>
