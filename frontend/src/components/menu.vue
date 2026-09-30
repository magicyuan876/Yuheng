<template>
  <!--
    The sidebar's horizontal grid: 14px left inset, an 18px icon slot and an
    8px gap, so the menu titles, the session group headers and the session
    rows all start on the same column. The rows below spell those numbers
    out (pl-[14px], w-[18px], mr-2).

    Height is 100% rather than 100vh because <html> carries a `zoom`
    multiplier for font-size control; 100vh is evaluated against the
    unscaled viewport and then scaled, so at "large" the sidebar would
    extend past the window. The ancestor chain (html/body/#app/.main) is
    already height: 100%.
  -->
  <div
    class="border-border relative box-border flex h-full flex-col border-r bg-[var(--td-bg-color-sidebar)] pt-2 pb-1.5 shadow-[1px_0_0_rgba(0,0,0,0.02)] transition-[width,min-width] duration-[250ms] ease-in-out"
    :class="
      uiStore.sidebarCollapsed
        ? 'w-[60px] min-w-[60px] overflow-visible px-[3px]'
        : 'w-[260px] min-w-[260px] overflow-hidden px-1.5'
    "
  >
    <!-- 展开时：Logo + 搜索/折叠按钮同行 -->
    <div class="flex h-[50px] shrink-0 items-center justify-between pr-2.5 pl-[14px]" v-if="!uiStore.sidebarCollapsed">
      <div
        class="flex min-w-0 flex-1 cursor-pointer items-center overflow-hidden"
        @click="router.push('/platform/knowledge-bases')"
      >
        <img :src="yuhengMark" alt="" class="mr-2 size-6 shrink-0 rounded-[6px]" draggable="false" />
        <span
          class="text-foreground inline-block max-w-[128px] truncate text-[19px] leading-[1.2] font-bold tracking-[-0.01em] select-none"
          >Yuheng</span
        >
      </div>
      <!-- 顶部 logo 行右侧的图标按钮组（搜索 + 折叠），与折叠按钮风格一致 -->
      <div class="flex shrink-0 items-center gap-1">
        <Tooltip>
          <TooltipTrigger as-child>
            <div
              class="group text-muted-foreground hover:bg-accent box-border flex size-[26px] shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors duration-200"
              @click="commandPaletteStore.openPalette('')"
              :aria-label="t('menu.search')"
            >
              <!-- The icon is an <img>, so currentColor cannot reach it; dark mode inverts it instead. -->
              <img
                class="block size-[18px] dark:opacity-55 dark:invert dark:group-hover:opacity-90"
                :src="getImgSrc('search.svg')"
                alt=""
              />
            </div>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            <span :class="cmdkTipClass">
              <span class="text-[13px]">{{ t("menu.search") }}</span>
              <span class="text-[13px] tracking-[0.5px] opacity-60">{{ cmdModKeyLabel }}K</span>
            </span>
          </TooltipContent>
        </Tooltip>
        <div
          class="text-muted-foreground hover:bg-accent hover:text-foreground box-border flex size-[18px] shrink-0 cursor-pointer items-center justify-center rounded-[4px] transition-colors duration-200"
          @click="uiStore.toggleSidebar"
          :title="t('menu.collapseSidebar')"
        >
          <svg viewBox="0 0 20 20" width="18" height="18" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" stroke-width="1.2" />
            <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" stroke="currentColor" stroke-width="1.2" />
            <line x1="4" y1="7.5" x2="4" y2="12.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
          </svg>
        </div>
      </div>
    </div>
    <!-- 折叠时：展开按钮 -->
    <Tooltip v-else>
      <TooltipTrigger as-child>
        <div :class="menuItemClass('')" @click="uiStore.toggleSidebar">
          <div :class="menuItemBoxClass">
            <div :class="menuIconClass">
              <svg
                class="size-[18px] overflow-hidden"
                viewBox="0 0 20 20"
                width="20"
                height="20"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" stroke-width="1.2" />
                <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" stroke="currentColor" stroke-width="1.2" />
                <line x1="5" y1="10" x2="3" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
                <line x1="5" y1="10" x2="3" y2="12" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
              </svg>
            </div>
          </div>
        </div>
      </TooltipTrigger>
      <TooltipContent side="right">{{ t("menu.expandSidebar") }}</TooltipContent>
    </Tooltip>

    <!-- 空间选择器：仅在用户可切换空间时显示 -->
    <TenantSelector v-if="canAccessAllTenants && !uiStore.sidebarCollapsed" />

    <!-- 折叠时右侧拖拽展开手柄 -->
    <div
      v-if="uiStore.sidebarCollapsed"
      class="absolute top-0 -right-[3px] z-10 h-full w-1.5 cursor-ew-resize hover:bg-[var(--td-brand-color-light)]"
      @mousedown="onDragHandleMouseDown"
    />

    <!-- 上半部分：新对话吸顶 + 知识库/智能体/共享空间/历史会话随滚动一起滚走 -->
    <!--
      Expanded, the negative right margin pulls the scrollbar out to the
      panel's edge and the equal padding puts the list text back where it
      was. `menu_top` is the hook the scrollbar styles below hang on.
    -->
    <div
      class="menu_top flex min-h-0 flex-1 flex-col overflow-x-hidden overflow-y-auto"
      :class="uiStore.sidebarCollapsed ? 'mr-0 pr-0' : '-mr-1 pr-1'"
      ref="scrollContainer"
      @scroll="handleScroll"
    >
      <!-- 全局搜索入口：点击打开命令面板（⌘K）。展开态移至顶部 logo 行的图标按钮；
                 折叠态在此处保留为图标项 + 深色 tooltip。 -->
      <div class="relative flex flex-col" v-if="uiStore.sidebarCollapsed">
        <Tooltip>
          <TooltipTrigger as-child>
            <div :class="menuItemClass('')" @click="commandPaletteStore.openPalette('')">
              <div :class="menuItemBoxClass">
                <div :class="menuIconClass">
                  <img :class="menuImgClass('')" :src="getImgSrc('search.svg')" alt="" />
                </div>
              </div>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">
            <span :class="cmdkTipClass">
              <span class="text-[13px]">{{ t("menu.search") }}</span>
              <span class="text-[13px] tracking-[0.5px] opacity-60">{{ cmdModKeyLabel }}K</span>
            </span>
          </TooltipContent>
        </Tooltip>
      </div>
      <!--
        「新对话」吸顶：作为滚动容器的直接子级，滚动时钉在顶部，知识库/智能体/
        共享空间及历史列表一起从其下方滚走。背景遮挡滚动内容。
      -->
      <div
        class="relative flex flex-col"
        :class="{ 'sticky top-0 z-[2] bg-[var(--td-bg-color-sidebar)]': item.children && !uiStore.sidebarCollapsed }"
        v-for="(item, index) in topMenuItems"
        :key="index"
      >
        <Tooltip :disabled="!uiStore.sidebarCollapsed">
          <TooltipTrigger as-child>
            <div
              @click="handleMenuClick(item.path)"
              @mouseenter="mouseenteMenu(item.path)"
              @mouseleave="mouseleaveMenu(item.path)"
              :data-guide="`nav-${item.path}`"
              :class="menuItemClass(menuItemState(item))"
            >
              <div :class="menuItemBoxClass">
                <div :class="menuIconClass">
                  <img
                    :class="menuImgClass(menuItemState(item))"
                    :src="
                      getImgSrc(
                        item.icon == 'zhishiku'
                          ? knowledgeIcon
                          : item.icon == 'docs'
                            ? docsIcon
                            : item.icon == 'organization'
                              ? organizationIcon
                              : item.icon == 'logout'
                                ? logoutIcon
                                : item.icon == 'setting'
                                  ? settingIcon
                                  : prefixIcon,
                      )
                    "
                    alt=""
                  />
                </div>
                <template v-if="!uiStore.sidebarCollapsed">
                  <span
                    class="max-w-[120px] flex-1 truncate font-[family-name:var(--app-font-family)] text-sm leading-5 font-semibold"
                    :class="menuItemState(item) === 'active' ? 'text-primary' : 'text-foreground'"
                    :title="item.title"
                    >{{ item.title }}</span
                  >
                  <span
                    v-if="item.path === 'organizations' && orgStore.totalPendingJoinRequestCount > 0"
                    class="text-warning ml-1.5 h-[18px] min-w-[18px] shrink-0 rounded-[9px] bg-[rgba(250,173,20,0.2)] px-[5px] text-center text-xs leading-[18px] font-semibold"
                    :title="t('organization.settings.pendingJoinRequestsBadge')"
                    >{{ orgStore.totalPendingJoinRequestCount }}</span
                  >
                </template>
              </div>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right">{{ item.title }}</TooltipContent>
        </Tooltip>
      </div>

      <!-- 历史会话：按来源筛选后统一按日期分组展示 -->
      <div
        class="session-submenu relative min-w-0 pt-[3px] font-[family-name:var(--app-font-family)] text-sm"
        v-if="!uiStore.sidebarCollapsed"
      >
        <!-- Stable, always-mounted source filter: reserving its row here
                     (instead of embedding it in the first date group, which
                     appears/disappears while a bucket loads) prevents the
                     top-right control from jumping when switching session type.
                     It is absolutely pinned to the list's top-right so it
                     visually sits on the first row (e.g. beside "近30天") and
                     overlays the empty right side of that header row, so it
                     needs no reserved height. -->
        <div
          v-if="showSessionSourceFilter && !batchMode"
          class="session-list-scope-header absolute top-1 right-2.5 z-[2] flex max-w-[calc(100%-24px)] justify-end"
        >
          <SessionSourceFilter
            inline
            :emphasized="sessionScopeFilterPinned"
            :sources="sessionSourceOptions"
            :current="activeSessionBucketKey"
            @select="switchSessionBucket"
          />
        </div>
        <template v-if="sessionListBooting && !hasAnySession">
          <div v-for="n in 4" :key="'skel-' + n" class="min-w-0 overflow-hidden p-0">
            <div :class="sessionRowClass">
              <Skeleton class="my-2 h-3.5 w-full flex-auto" />
            </div>
          </div>
        </template>

        <div v-else>
          <template v-if="activeBucket?.loading && !activeBucket.loaded && filteredGroupedSessions.length === 0">
            <div v-for="n in 4" :key="'bucket-skel-' + n" class="min-w-0 overflow-hidden p-0">
              <div :class="sessionRowClass">
                <Skeleton class="my-2 h-3.5 w-full flex-auto" />
              </div>
            </div>
          </template>
          <template v-else-if="activeBucket?.loaded && filteredGroupedSessions.length === 0">
            <div class="text-placeholder px-3.5 py-6 text-center text-xs select-none">{{ t("menu.noSessions") }}</div>
          </template>
          <template v-else>
            <template v-for="group in filteredGroupedSessions" :key="group.key">
              <div
                v-if="group.label"
                :class="sessionRowClass"
                class="mt-0 pt-1 pb-px font-[family-name:var(--app-font-family)] text-[11px] leading-4 font-semibold text-[var(--td-text-color-disabled)] select-none"
              >
                <span class="min-w-0 flex-auto overflow-hidden">
                  <span class="whitespace-nowrap">{{ group.label }}</span>
                </span>
              </div>
              <!--
                session-chat-row and its --active modifier are hook classes:
                the row's hover and active states restyle SessionSidebarRow's
                own hook classes in the style block below.
              -->
              <div
                v-for="subitem in group.items"
                :key="subitem.id"
                class="session-chat-row group/row min-w-0 overflow-hidden p-0"
                :class="{ 'session-chat-row--active': !batchMode && subitem.path === currentSecondpath }"
              >
                <div
                  :class="[
                    sessionRowClass,
                    'min-h-[30px] rounded-md transition-[background,color] duration-150 ease-in-out',
                    batchMode && batchSelectedIds.includes(subitem.id)
                      ? 'bg-[rgba(7,192,95,0.05)]'
                      : !batchMode && subitem.path === currentSecondpath
                        ? 'bg-accent'
                        : 'group-hover/row:bg-accent',
                  ]"
                >
                  <div class="min-w-0 flex-auto overflow-hidden">
                    <SessionSidebarRow
                      :item="subitem"
                      :batch-mode="batchMode"
                      :active-path="currentSecondpath"
                      :selected-ids="batchSelectedIds"
                      :menu-options="buildSessionMenuOptions(subitem)"
                      @navigate="gotopage(subitem.path)"
                      @toggle-select="toggleBatchSelect(subitem.id)"
                      @menu-click="handleSessionMenuClick($event, subitem)"
                      @rename-submit="renameSessionTitle(subitem, $event.title)"
                      @hover-in="mouseenteBotDownr(subitem.id)"
                      @hover-out="mouseleaveBotDown"
                    />
                  </div>
                </div>
              </div>
            </template>
            <div
              v-if="activeBucket?.loading && filteredGroupedSessions.length > 0"
              :class="sessionRowClass"
              class="text-placeholder min-h-[26px]"
            >
              <span class="min-w-0 flex-auto overflow-hidden">
                <Loader2Icon class="text-primary size-4 animate-spin" />
              </span>
            </div>
          </template>
        </div>
      </div>
    </div>

    <!-- 批量管理底部操作条：固定在侧栏底部、用户头像上方 -->
    <div
      v-if="batchMode && !uiStore.sidebarCollapsed"
      class="border-border bg-card flex shrink-0 items-center justify-between border-t px-3 py-1.5"
    >
      <div class="text-placeholder flex items-center text-[13px]">
        <label class="flex cursor-pointer items-center gap-2">
          <Checkbox
            class="data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-primary-foreground"
            :model-value="isBatchIndeterminate ? 'indeterminate' : isAllBatchSelected"
            @update:model-value="(v) => toggleBatchSelectAll(v === true)"
          >
            <MinusIcon v-if="isBatchIndeterminate" />
            <CheckIcon v-else />
          </Checkbox>
          <span class="text-foreground">{{ t("batchManage.selectAll") }}</span>
        </label>
      </div>
      <div class="flex items-center gap-1.5">
        <Button size="sm" variant="ghost" @click="exitBatchMode">
          {{ t("batchManage.cancel") }}
        </Button>
        <Button
          size="sm"
          variant="destructive"
          class="bg-destructive hover:bg-destructive/90 dark:bg-destructive dark:hover:bg-destructive/90 text-white"
          :disabled="batchSelectedIds.length === 0 || batchDeleting"
          @click="handleInlineBatchDelete"
        >
          <Loader2Icon v-if="batchDeleting" class="animate-spin" />
          {{ t("batchManage.delete") }}{{ batchSelectedIds.length > 0 ? `(${batchDisplayCount})` : "" }}
        </Button>
      </div>
    </div>

    <!-- 下半部分：用户菜单 -->
    <div class="flex shrink-0 flex-col" :class="{ 'items-center': uiStore.sidebarCollapsed }">
      <UserMenu />
    </div>
  </div>
</template>

<script setup lang="ts">
import { storeToRefs } from "pinia";
import { onMounted, onUnmounted, watch, computed, ref, h, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getSessionsList, batchDelSessions, deleteAllSessions, getSession } from "@/api/chat/index";
import { useChatResourcesStore } from "@/stores/chatResources";
import SessionSidebarRow from "./SessionSidebarRow.vue";
import {
  clearSession,
  removeSession,
  renameSession,
  SESSION_MUTATION_EVENT,
  setSessionPinned,
  type SessionMutationDetail,
} from "./sessionMutations";
import SessionSourceFilter from "./SessionSourceFilter.vue";
import {
  SIDEBAR_BUCKET_PAGE_SIZE,
  applyBucketCountProbe,
  buildBucketDefinitions,
  bucketHasMore,
  bucketVisible,
  createEmptyBucket,
  flattenBucketItems,
  isChannelBucket,
  isChannelBucketKey,
  mergeBucketPage,
  prependSessionToWebBucket,
  removeSessionFromBuckets,
  type SidebarSessionBucket,
} from "./sessionSidebarBuckets";
import type { SessionForGrouping } from "./sessionGrouping";
import {
  classifyDateBucket,
  groupSessionsByDate,
  originGroupKey,
  resolveSessionOrigin,
  type DateBucketKey,
} from "./sessionGrouping";
import {
  DEFAULT_SESSION_BUCKET_KEY,
  buildSessionSourceOptions,
  findSessionBucketKey,
  shouldShowSessionSourceFilter,
} from "./sessionSidebarSourceFilter";
import { logout as logoutApi } from "@/api/auth";
import { useMenuStore } from "@/stores/menu";
import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";
import { useOrganizationStore } from "@/stores/organization";
import { useUIStore } from "@/stores/ui";
import { useCommandPaletteStore } from "@/stores/commandPalette";
import { MessagePlugin, DialogPlugin } from "tdesign-vue-next";
import UserMenu from "@/components/UserMenu.vue";
import TenantSelector from "@/components/TenantSelector.vue";
import yuhengMark from "@/assets/img/yuheng-mark.svg";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  CheckIcon,
  EraserIcon,
  ListChecksIcon,
  Loader2Icon,
  MinusIcon,
  PencilLineIcon,
  PinIcon,
  Trash2Icon,
} from "@lucide/vue";
import { useI18n } from "vue-i18n";

const chatResources = useChatResourcesStore();

const { t } = useI18n();
const usemenuStore = useMenuStore();
const authStore = useAuthStore();
const deploymentCapabilities = useDeploymentCapabilitiesStore();
const orgStore = useOrganizationStore();
const uiStore = useUIStore();
const commandPaletteStore = useCommandPaletteStore();

// Platform-aware label for the ⌘K hint. navigator.platform is deprecated but
// the alternatives (userAgentData.platform) aren't universally available yet;
// this check is good enough for Mac vs. non-Mac.
const isMacLike = typeof navigator !== "undefined" && /Mac|iPod|iPhone|iPad/.test(navigator.platform || "");
const cmdModKeyLabel = isMacLike ? "⌘" : "Ctrl";
const route = useRoute();
const router = useRouter();
const currentpath = ref("");
const total = ref(0);
const sessionBuckets = ref<Record<string, SidebarSessionBucket>>({});
const bucketOrder = ref<string[]>([]);
let bucketRequestToken = 0;
const sessionListBooting = ref(false);
const currentSecondpath = ref("");
const scrollContainer = ref<HTMLElement | null>(null);
const activeSessionBucketKey = ref(DEFAULT_SESSION_BUCKET_KEY);
const sessionListCanScroll = ref(false);
const visibleChannelBuckets = computed(() =>
  bucketOrder.value
    .map((key) => sessionBuckets.value[key])
    .filter((bucket): bucket is SidebarSessionBucket => !!bucket && isChannelBucket(bucket) && bucketVisible(bucket)),
);
const showSessionSourceFilter = computed(() => shouldShowSessionSourceFilter(visibleChannelBuckets.value.length));
const sessionScopeFilterPinned = computed(() => activeSessionBucketKey.value !== DEFAULT_SESSION_BUCKET_KEY);
const sessionSourceOptions = computed(() =>
  buildSessionSourceOptions(
    t("menu.myChats"),
    visibleChannelBuckets.value.map((bucket) => ({
      key: bucket.key,
      label: bucket.label,
    })),
  ),
);
const activeBucket = computed(() => sessionBuckets.value[activeSessionBucketKey.value]);
const hasAnySession = computed(() => Object.values(sessionBuckets.value).some((bucket) => bucket.items.length > 0));
type MenuItem = { title: string; icon: string; path: string; childrenPath?: string; children?: any[] };
const { menuArr, visibleMenuArr } = storeToRefs(usemenuStore);
const activeSubmenu = ref<string>("");

// 批量管理状态
const batchMode = ref(false);
const batchSelectedIds = ref<string[]>([]);
const batchDeleting = ref(false);

const allSessionIds = computed(() => {
  const chatMenu = (menuArr.value as unknown as MenuItem[]).find((item: MenuItem) => item.path === "creatChat");
  if (!chatMenu?.children) return [];
  return (chatMenu.children as any[]).map((s: any) => s.id);
});

const isAllBatchSelected = computed(
  () => allSessionIds.value.length > 0 && batchSelectedIds.value.length === allSessionIds.value.length,
);

const isBatchIndeterminate = computed(
  () => batchSelectedIds.value.length > 0 && batchSelectedIds.value.length < allSessionIds.value.length,
);

const batchDisplayCount = computed(() => (isAllBatchSelected.value ? total.value : batchSelectedIds.value.length));

// 是否可以访问所有空间
const canAccessAllTenants = computed(() => authStore.canAccessAllTenants);

// 是否处于知识库详情页（不包括全局聊天）
const isInKnowledgeBase = computed<boolean>(() => {
  return route.name === "knowledgeBaseDetail" || route.name === "kbCreatChat" || route.name === "knowledgeBaseSettings";
});

// 是否在知识库列表页面

// 是否在创建聊天页面

// 是否在对话详情页

// 是否在组织列表页面

// 统一的菜单项激活状态判断
const isMenuItemActive = (itemPath: string): boolean => {
  const currentRoute = route.name;

  switch (itemPath) {
    case "knowledge-bases":
      return (
        currentRoute === "knowledgeBaseList" ||
        currentRoute === "knowledgeBaseDetail" ||
        currentRoute === "knowledgeBaseSettings"
      );
    case "docs":
      return currentRoute === "docsSpaceList" || currentRoute === "docsSpace" || currentRoute === "docsSpaceSettings";
    case "organizations":
      return currentRoute === "organizationList";
    case "creatChat":
      return currentRoute === "kbCreatChat" || currentRoute === "globalCreatChat";
    case "settings":
      return currentRoute === "settings";
    default:
      return itemPath === currentpath.value;
  }
};

// ---------- Row styles ----------
// A top menu entry is in one of three states. "active" is the page it leads
// to; "childActive" is an entry whose child route is open, drawn in the
// plain text colour; "" is everything else.
type MenuItemState = "active" | "childActive" | "";

const menuItemState = (item: MenuItem): MenuItemState => {
  if (item.childrenPath && item.childrenPath == currentpath.value) return "childActive";
  return isMenuItemActive(item.path) ? "active" : "";
};

// Collapsed, the rail centres each icon and drops the title column's
// padding; expanded, the row starts on the sidebar's 14px inset. The active
// row keeps its background on hover.
const menuItemClass = (state: MenuItemState): string =>
  [
    "group mb-0.5 box-border flex h-[38px] cursor-pointer items-center rounded-[4px] transition-colors duration-200",
    uiStore.sidebarCollapsed ? "justify-center px-0 py-[9px]" : "justify-between py-2 pr-2.5 pl-[14px]",
    state === "active" ? "bg-secondary" : "hover:bg-accent",
  ].join(" ");

const menuItemBoxClass = computed(() =>
  uiStore.sidebarCollapsed ? "relative flex w-auto items-center justify-center" : "relative flex w-full items-center",
);

const menuIconClass = computed(() =>
  [
    "text-muted-foreground group-hover:text-foreground flex w-[18px] flex-[0_0_18px]",
    uiStore.sidebarCollapsed ? "mr-0" : "mr-2",
  ].join(" "),
);

// The menu icons are SVG files loaded through <img>, so currentColor cannot
// reach them. In dark mode they are inverted to read as text instead —
// dimmed at rest, brighter on hover or when the entry's child is open —
// except the active entry, whose icon is already the green variant.
const menuImgClass = (state: MenuItemState): string => {
  const base = "size-[18px] overflow-hidden";
  if (state === "active") return base;
  if (state === "childActive") return `${base} dark:opacity-90 dark:invert`;
  return `${base} dark:opacity-55 dark:invert dark:group-hover:opacity-90`;
};

// The ⌘K tooltip: the label, then the shortcut in a lighter grey.
const cmdkTipClass = "inline-flex items-center gap-2 whitespace-nowrap";

// Session list rows sit flat on the sidebar's 14px inset, aligned with the
// chat section's title rather than reserving an icon slot.
const sessionRowClass = "box-border flex min-w-0 items-center gap-0 pr-2.5 pl-[14px]";

// 统一的图标激活状态判断
const getIconActiveState = (itemPath: string) => {
  const currentRoute = route.name;

  return {
    isKbActive:
      itemPath === "knowledge-bases" &&
      (currentRoute === "knowledgeBaseList" ||
        currentRoute === "knowledgeBaseDetail" ||
        currentRoute === "knowledgeBaseSettings"),
    isCreatChatActive:
      itemPath === "creatChat" && (currentRoute === "kbCreatChat" || currentRoute === "globalCreatChat"),
    isSettingsActive: itemPath === "settings" && currentRoute === "settings",
    isChatActive: itemPath === "chat" && currentRoute === "chat",
  };
};

// 分离上下两部分菜单（使用 visibleMenuArr 以便 lite 模式过滤 logout）
const topMenuItems = computed<MenuItem[]>(() => {
  return (visibleMenuArr.value as unknown as MenuItem[]).filter(
    (item: MenuItem) =>
      item.path === "knowledge-bases" ||
      item.path === "docs" ||
      item.path === "organizations" ||
      item.path === "creatChat",
  );
});

// 当前知识库信息
const currentKbName = ref<string>("");
const currentKbInfo = ref<any>(null);

// 进行中的置顶/取消置顶请求，避免重复点击
const pinningIds = ref<Set<string>>(new Set());

// 「聊天」区内按日期分组（当前筛选来源）
const dateBucketLabels = computed<Record<DateBucketKey, string>>(() => ({
  pinned: t("time.pinned"),
  today: t("time.today"),
  yesterday: t("time.yesterday"),
  last7Days: t("time.last7Days"),
  last30Days: t("time.last30Days"),
  lastYear: t("time.lastYear"),
  earlier: t("time.earlier"),
}));

const filteredGroupedSessions = computed(() => {
  const bucket = activeBucket.value;
  if (!bucket?.items.length) return [];
  return groupSessionsByDate(
    bucket.items.map((item) => ({
      ...item,
      path: `chat/${item.id}`,
      title: item.title || "",
    })),
    dateBucketLabels.value,
    (session) => classifyDateBucket(session.updated_at || session.created_at),
  );
});

const refreshSessionListScrollability = async () => {
  await nextTick();
  const container = scrollContainer.value;
  sessionListCanScroll.value = !!container && container.scrollHeight > container.clientHeight + 1;
};

/** 列表未撑满滚动区时自动续页（按当前可见 DOM 测量，避免折叠导致误判） */
const ensureBucketFillsViewport = async (key: string) => {
  const MAX_ITERATIONS = 20;
  for (let i = 0; i < MAX_ITERATIONS; i++) {
    await nextTick();
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    const container = scrollContainer.value;
    const bucket = sessionBuckets.value[key];
    if (!container || !bucket || !bucketHasMore(bucket) || bucket.loading) break;

    const hasOverflow = container.scrollHeight > container.clientHeight + 1;
    if (hasOverflow) break;

    const prevCount = bucket.items.length;
    await loadBucketPage(key);
    if ((sessionBuckets.value[key]?.items.length ?? 0) <= prevCount) break;
  }
};

const mouseenteBotDownr = (val: string) => {
  activeSubmenu.value = val;
};
const mouseleaveBotDown = () => {
  activeSubmenu.value = "";
};

const enterBatchMode = () => {
  batchMode.value = true;
  batchSelectedIds.value = [];
};

const exitBatchMode = () => {
  batchMode.value = false;
  batchSelectedIds.value = [];
};

const toggleBatchSelect = (id: string) => {
  const idx = batchSelectedIds.value.indexOf(id);
  if (idx > -1) {
    batchSelectedIds.value.splice(idx, 1);
  } else {
    batchSelectedIds.value.push(id);
  }
};

const toggleBatchSelectAll = (checked: boolean) => {
  batchSelectedIds.value = checked ? [...allSessionIds.value] : [];
};

const handleInlineBatchDelete = () => {
  if (batchSelectedIds.value.length === 0) return;
  const isDeleteAll = isAllBatchSelected.value;
  const displayCount = batchDisplayCount.value;
  const confirmDialog = DialogPlugin.confirm({
    header: t("batchManage.deleteConfirmTitle"),
    body: isDeleteAll
      ? t("batchManage.deleteAllConfirmBody") || t("batchManage.deleteConfirmBody", { count: displayCount })
      : t("batchManage.deleteConfirmBody", { count: displayCount }),
    confirmBtn: { content: t("batchManage.delete"), theme: "danger" as const },
    cancelBtn: t("batchManage.cancel"),
    theme: "warning",
    onConfirm: async () => {
      batchDeleting.value = true;
      try {
        let res: any;
        if (isDeleteAll) {
          res = await deleteAllSessions();
        } else {
          res = await batchDelSessions([...batchSelectedIds.value]);
        }
        if (res && res.success === true) {
          if (isDeleteAll) {
            usemenuStore.clearMenuArr();
            total.value = 0;
            await getMessageList();
          } else {
            let next = sessionBuckets.value;
            for (const id of batchSelectedIds.value) {
              next = removeSessionFromBuckets(next, id);
            }
            sessionBuckets.value = next;
            syncMenuStoreFromBuckets();
          }
          const currentChatId = route.params.chatid as string;
          if (currentChatId && (isDeleteAll || batchSelectedIds.value.includes(currentChatId))) {
            router.push("/platform/creatChat");
          }
          batchSelectedIds.value = [];
          MessagePlugin.success(t("batchManage.deleteSuccess"));
          exitBatchMode();
        } else {
          MessagePlugin.error(t("batchManage.deleteFailed"));
        }
      } catch {
        MessagePlugin.error(t("batchManage.deleteFailed"));
      }
      batchDeleting.value = false;
      confirmDialog.destroy();
    },
  });
};

const handleSessionMenuClick = (data: { value: string }, item: any) => {
  if (data?.value === "delete") {
    delCard(item);
  } else if (data?.value === "clearMessages") {
    clearMessages(item);
  } else if (data?.value === "batchManage") {
    enterBatchMode();
  } else if (data?.value === "pin" || data?.value === "unpin") {
    togglePin(item, data.value === "pin");
  }
};

// 基于会话来源推导展示用的短标签已经被 platformLogo(<img>) 取代，Web 会话没有图标。

const buildSessionMenuOptions = (item: any) => {
  const options: any[] = [];
  if (item.is_pinned) {
    options.push({
      content: t("menu.unpin"),
      value: "unpin",
      prefixIcon: () => h(PinIcon, { class: "size-4 fill-current" }),
    });
  } else {
    options.push({
      content: t("menu.pin"),
      value: "pin",
      prefixIcon: () => h(PinIcon, { class: "size-4" }),
    });
  }
  options.push(
    { content: t("menu.renameSession"), value: "rename", prefixIcon: () => h(PencilLineIcon, { class: "size-4" }) },
    {
      content: t("menu.clearMessages"),
      value: "clearMessages",
      prefixIcon: () => h(EraserIcon, { class: "size-4" }),
    },
    {
      content: t("menu.batchManage"),
      value: "batchManage",
      prefixIcon: () => h(ListChecksIcon, { class: "size-4" }),
    },
    {
      content: t("upload.deleteRecord"),
      value: "delete",
      theme: "error",
      prefixIcon: () => h(Trash2Icon, { class: "size-4" }),
    },
  );
  return options;
};

const updateSessionInBuckets = (
  sessionId: string,
  patch: Partial<{ is_pinned: boolean; pinned_at: string | null; title: string; isNoTitle?: boolean }>,
) => {
  const next: Record<string, SidebarSessionBucket> = {};
  for (const [key, bucket] of Object.entries(sessionBuckets.value)) {
    next[key] = {
      ...bucket,
      items: bucket.items.map((row) => (row.id === sessionId ? { ...row, ...patch } : row)),
    };
  }
  sessionBuckets.value = next;
  syncMenuStoreFromBuckets();
};

const renameSessionTitle = async (item: any, title: string) => {
  try {
    await renameSession(item.id, title, item.description || "");
    MessagePlugin.success(t("menu.renameSessionSuccess"));
  } catch {
    MessagePlugin.error(t("menu.renameSessionFailed"));
  }
};

const togglePin = (item: any, pin: boolean) => {
  if (pinningIds.value.has(item.id)) return;
  pinningIds.value.add(item.id);

  setSessionPinned(item.id, pin)
    .catch(() => {
      MessagePlugin.error(pin ? t("menu.pinFailed") : t("menu.unpinFailed"));
    })
    .finally(() => {
      pinningIds.value.delete(item.id);
    });
};

const clearMessages = (item: any) => {
  clearSession(item.id)
    .then(() => {
      MessagePlugin.success(t("menu.clearMessagesSuccess"));
    })
    .catch(() => {
      MessagePlugin.error(t("menu.clearMessagesFailed"));
    });
};

const delCard = (item: any) => {
  removeSession(item.id).catch(() => MessagePlugin.error(t("chat.deleteSessionFailed")));
};

const debounce = (fn: (...args: any[]) => void, delay: number) => {
  let timer: ReturnType<typeof setTimeout>;
  return (...args: any[]) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), delay);
  };
};
const mapSessionRow = (item: any) => ({
  title: item.title ? item.title : t("menu.newSession"),
  path: `chat/${item.id}`,
  id: item.id,
  isMore: false,
  isNoTitle: item.title ? false : true,
  created_at: item.created_at,
  updated_at: item.updated_at,
  is_pinned: !!item.is_pinned,
  pinned_at: item.pinned_at || null,
  user_id: item.user_id || "",
});

const syncMenuStoreFromBuckets = () => {
  usemenuStore.clearMenuArr();
  const flat = flattenBucketItems(sessionBuckets.value, bucketOrder.value);
  flat.forEach((item) => usemenuStore.updatemenuArr(item));
  total.value = flat.length;
};

const menuChildToSessionRow = (item: Record<string, unknown>): SessionForGrouping & { path: string } => {
  const id = String(item.id);
  return {
    id,
    path: typeof item.path === "string" ? item.path : `chat/${id}`,
    title: typeof item.title === "string" ? item.title : undefined,
    is_pinned: !!item.is_pinned,
    created_at: typeof item.created_at === "string" ? item.created_at : undefined,
    updated_at: typeof item.updated_at === "string" ? item.updated_at : undefined,
    user_id: typeof item.user_id === "string" ? item.user_id : "",
  };
};

const sessionExistsInBuckets = (sessionId: string) =>
  Object.values(sessionBuckets.value).some((bucket) => bucket.items.some((row) => row.id === sessionId));

/** 创建会话后 menuStore 已乐观写入，但列表实际渲染自 sessionBuckets，需补齐。 */
const ensureSessionInSidebar = (sessionId: string) => {
  if (!sessionId || sessionExistsInBuckets(sessionId)) return;

  const web = sessionBuckets.value.web;
  if (!web) return;

  const chatMenu = (menuArr.value as unknown as MenuItem[]).find((item) => item.path === "creatChat");
  const fromStore = (chatMenu?.children as Record<string, unknown>[] | undefined)?.find(
    (item) => item.id === sessionId,
  );
  if (!fromStore) return;

  sessionBuckets.value = {
    ...sessionBuckets.value,
    web: prependSessionToWebBucket(web, menuChildToSessionRow(fromStore)),
  };
  total.value = flattenBucketItems(sessionBuckets.value, bucketOrder.value).length;
};

const rebuildBucketDefinitions = () =>
  buildBucketDefinitions(
    {
      web: t("menu.myChats"),
      api: t("menu.apiChats"),
    },
    { includeAdminChannelBuckets: authStore.hasRole("admin") },
  );

/** 首屏轻量探测各渠道是否有会话（page_size=1 只取 total），避免展示空文件夹 */
const probeChannelBucketCounts = async (keys: string[], token: number) => {
  const targets = keys.filter((key) => isChannelBucketKey(key));
  await Promise.all(
    targets.map(async (key) => {
      const bucket = sessionBuckets.value[key];
      if (!bucket) return;
      try {
        const res: any = await getSessionsList(1, 1, bucket.apiSource);
        if (token !== bucketRequestToken) return;
        sessionBuckets.value = {
          ...sessionBuckets.value,
          [key]: applyBucketCountProbe(bucket, res?.total ?? 0),
        };
      } catch {
        if (token !== bucketRequestToken) return;
        sessionBuckets.value = {
          ...sessionBuckets.value,
          [key]: applyBucketCountProbe(bucket, 0),
        };
      }
    }),
  );
};

const loadBucketPage = async (key: string, page?: number, token?: number) => {
  const activeToken = token ?? bucketRequestToken;
  const bucket = sessionBuckets.value[key];
  if (!bucket || bucket.loading) return;

  const nextPage = page ?? bucket.page + 1;
  sessionBuckets.value = {
    ...sessionBuckets.value,
    [key]: { ...bucket, loading: true },
  };

  try {
    const res: any = await getSessionsList(nextPage, SIDEBAR_BUCKET_PAGE_SIZE, bucket.apiSource);
    if (activeToken !== bucketRequestToken) return;
    const rows = (res?.data || []).map((item: any) => mapSessionRow(item));
    const current = sessionBuckets.value[key];
    sessionBuckets.value = {
      ...sessionBuckets.value,
      [key]: mergeBucketPage(current, rows, res?.total ?? rows.length, nextPage),
    };
    syncMenuStoreFromBuckets();
    await refreshSessionListScrollability();
  } catch {
    if (activeToken !== bucketRequestToken) return;
    const current = sessionBuckets.value[key];
    sessionBuckets.value = {
      ...sessionBuckets.value,
      [key]: { ...current, loading: false, loaded: true },
    };
  }
};

const switchSessionBucket = async (key: string) => {
  if (key === activeSessionBucketKey.value) return;
  activeSessionBucketKey.value = key;
  const bucket = sessionBuckets.value[key];
  if (bucket && !bucket.loaded && !bucket.loading) {
    await loadBucketPage(key, 1);
  }
  await ensureBucketFillsViewport(key);
  await refreshSessionListScrollability();
};

const syncActiveBucketFromChat = async (sessionId: string | undefined) => {
  if (!sessionId) return;

  let bucketKey = findSessionBucketKey(sessionBuckets.value, sessionId);
  if (!bucketKey) {
    const chatMenu = (menuArr.value as unknown as MenuItem[]).find((item) => item.path === "creatChat");
    const fromStore = (chatMenu?.children as Record<string, unknown>[] | undefined)?.find(
      (item) => item.id === sessionId,
    );
    if (fromStore) {
      bucketKey = originGroupKey(resolveSessionOrigin(menuChildToSessionRow(fromStore)));
    }
  }
  // On a hard refresh only the web bucket is loaded, so a session opened from
  // the admin-only API folder isn't in any bucket or the menu store. Fetch
  // its detail and classify its origin folder
  // so the sidebar stays in sync with the chat pane instead of snapping back
  // to "my chats". Only switch when that folder is actually present.
  if (!bucketKey) {
    try {
      const res: any = await getSession(sessionId);
      const candidate = originGroupKey(
        resolveSessionOrigin({
          id: sessionId,
          user_id: res?.data?.user_id || "",
        }),
      );
      if (sessionBuckets.value[candidate]) {
        bucketKey = candidate;
      }
    } catch {
      // Fall through: leave the default bucket active on lookup failure.
    }
  }
  if (!bucketKey || bucketKey === activeSessionBucketKey.value) return;

  activeSessionBucketKey.value = bucketKey;
  const bucket = sessionBuckets.value[bucketKey];
  if (bucket && !bucket.loaded && !bucket.loading) {
    await loadBucketPage(bucketKey, 1);
  }
};

const initSessionBuckets = async () => {
  const token = ++bucketRequestToken;
  sessionListBooting.value = true;

  const defs = rebuildBucketDefinitions();
  bucketOrder.value = defs.map((def) => def.key);
  const buckets: Record<string, SidebarSessionBucket> = {};
  for (const def of defs) {
    buckets[def.key] = createEmptyBucket(def);
  }
  sessionBuckets.value = buckets;

  // 首屏：拉 web 会话 + 轻量探测各渠道 count（不拉完整列表）；有会话的渠道才展示文件夹
  const channelKeys = defs.map((def) => def.key).filter((key) => isChannelBucketKey(key));
  await Promise.all([loadBucketPage("web", 1, token), probeChannelBucketCounts(channelKeys, token)]);

  if (token === bucketRequestToken) {
    sessionListBooting.value = false;
    syncMenuStoreFromBuckets();
    await ensureBucketFillsViewport("web");
    await refreshSessionListScrollability();
  }
};

const getMessageList = async () => {
  await initSessionBuckets();
};

// 滚动到底时为当前筛选来源加载下一页
const checkScrollBottom = async () => {
  const container = scrollContainer.value;
  const key = activeSessionBucketKey.value;
  const bucket = sessionBuckets.value[key];
  if (!container || !bucket || !bucketHasMore(bucket) || bucket.loading) return;

  const { scrollTop, scrollHeight, clientHeight } = container;
  const hasOverflow = scrollHeight > clientHeight + 1;
  if (!hasOverflow) {
    await ensureBucketFillsViewport(key);
    return;
  }

  const isNearBottom = scrollHeight - (scrollTop + clientHeight) < 100;
  if (!isNearBottom) return;

  await loadBucketPage(key);
};

const handleScroll = debounce(checkScrollBottom, 200);

async function loadCurrentKbInfo(kbId: string) {
  if (!kbId || !isInKnowledgeBase.value) {
    currentKbName.value = "";
    currentKbInfo.value = null;
    return;
  }
  const data = await chatResources.fetchKnowledgeBaseById(kbId);
  if (data) {
    currentKbName.value = data.name || "";
    currentKbInfo.value = data;
  } else {
    currentKbInfo.value = null;
  }
}

const handleSessionMutation = (event: Event) => {
  const detail = (event as CustomEvent<SessionMutationDetail>).detail;
  if (!detail?.sessionId) return;
  if (detail.patch) {
    updateSessionInBuckets(detail.sessionId, {
      ...detail.patch,
      ...(detail.patch.title ? { isNoTitle: false } : {}),
    });
  }
  if (detail.removed) {
    sessionBuckets.value = removeSessionFromBuckets(sessionBuckets.value, detail.sessionId);
    syncMenuStoreFromBuckets();
    if (detail.sessionId === route.params.chatid) {
      router.push("/platform/creatChat");
    }
  }
};

onMounted(async () => {
  const routeName = typeof route.name === "string" ? route.name : route.name ? String(route.name) : "";
  currentpath.value = routeName;
  if (route.params.chatid) {
    currentSecondpath.value = `chat/${route.params.chatid}`;
  }

  window.addEventListener(SESSION_MUTATION_EVENT, handleSessionMutation);

  await loadCurrentKbInfo((route.params as any)?.kbId as string);

  await getMessageList();
  const initialChatId = route.params.chatid as string | undefined;
  if (initialChatId) {
    ensureSessionInSidebar(initialChatId);
    await syncActiveBucketFromChat(initialChatId);
  }
  // 若组织列表未加载则拉取一次，用于侧栏「待审批」角标
  if (deploymentCapabilities.isSupported("organizations") && orgStore.organizations.length === 0) {
    orgStore.fetchOrganizations();
  }
});

onUnmounted(() => {
  window.removeEventListener(SESSION_MUTATION_EVENT, handleSessionMutation);
});

watch([() => route.name, () => route.params], (newvalue, oldvalue) => {
  const nameStr = typeof newvalue[0] === "string" ? (newvalue[0] as string) : newvalue[0] ? String(newvalue[0]) : "";
  currentpath.value = nameStr;
  if (newvalue[1].chatid) {
    currentSecondpath.value = `chat/${newvalue[1].chatid}`;
  } else {
    currentSecondpath.value = "";
  }

  // 创建新会话时 creatChat 会先 updataMenuChildren，再跳转 chat/:id。
  // 侧栏实际渲染 sessionBuckets，需按 buckets 判断是否缺失，不能把 menuStore 当真相来源。
  const newChatId = (newvalue[1] as any)?.chatid as string | undefined;
  if (nameStr === "chat" && newChatId) {
    ensureSessionInSidebar(newChatId);
    void syncActiveBucketFromChat(newChatId);
  }

  // 路由变化时更新图标状态和知识库信息（不涉及对话列表）
  getIcon(nameStr);

  // 如果切换了知识库，更新知识库名称但不重新加载对话列表
  if (newvalue[1].kbId !== oldvalue?.[1]?.kbId) {
    loadCurrentKbInfo((newvalue[1] as any)?.kbId as string);
  }
});
const knowledgeIcon = ref("zhishiku-green.svg");
const prefixIcon = ref("prefixIcon.svg");
const logoutIcon = ref("logout.svg");
const settingIcon = ref("setting.svg");
const docsIcon = ref("docs.svg");
const organizationIcon = ref("organization.svg");
const pathPrefix = ref(route.name);
const getIcon = (path: string) => {
  // 根据当前路由状态更新所有图标
  const kbActiveState = getIconActiveState("knowledge-bases");
  const creatChatActiveState = getIconActiveState("creatChat");
  const settingsActiveState = getIconActiveState("settings");
  const docsActiveState =
    route.name === "docsSpaceList" || route.name === "docsSpace" || route.name === "docsSpaceSettings";
  const organizationsActiveState = route.name === "organizationList";

  // 知识库图标：只在知识库页面显示绿色
  knowledgeIcon.value = kbActiveState.isKbActive ? "zhishiku-green.svg" : "zhishiku.svg";

  // 在线文档图标：只在文档页面显示绿色
  docsIcon.value = docsActiveState ? "docs-green.svg" : "docs.svg";

  // 组织图标：只在组织页面显示绿色
  organizationIcon.value = organizationsActiveState ? "organization-green.svg" : "organization.svg";

  // 对话图标：只在对话创建页面显示绿色，其他情况显示默认
  prefixIcon.value = creatChatActiveState.isCreatChatActive ? "prefixIcon-green.svg" : "prefixIcon.svg";

  // 设置图标：只在设置页面显示绿色
  settingIcon.value = settingsActiveState.isSettingsActive ? "setting-green.svg" : "setting.svg";

  // 退出图标：始终显示默认
  logoutIcon.value = "logout.svg";
};
getIcon(typeof route.name === "string" ? (route.name as string) : route.name ? String(route.name) : "");
const handleMenuClick = async (path: string) => {
  if (path === "knowledge-bases") {
    // 知识库菜单项：如果在知识库内部，跳转到当前知识库文件页；否则跳转到知识库列表
    const kbId = await getCurrentKbId();
    if (kbId) {
      router.push(`/platform/knowledge-bases/${kbId}`);
    } else {
      router.push("/platform/knowledge-bases");
    }
  } else if (path === "docs") {
    router.push("/platform/docs");
  } else if (path === "organizations") {
    // 组织菜单项：跳转到组织列表
    router.push("/platform/organizations");
  } else if (path === "settings") {
    // 设置菜单项：打开设置弹窗并跳转路由
    uiStore.openSettings();
    router.push("/platform/settings");
  } else {
    gotopage(path);
  }
};

// 处理退出登录确认

const getCurrentKbId = async (): Promise<string | null> => {
  const kbId = (route.params as any)?.kbId as string;
  if (isInKnowledgeBase.value && kbId) {
    return kbId;
  }
  return null;
};

const gotopage = async (path: string) => {
  pathPrefix.value = path;
  // 处理退出登录
  if (path === "logout") {
    try {
      // 调用后端API注销
      await logoutApi();
    } catch (error) {
      // 即使API调用失败，也继续执行本地清理
      console.error("注销API调用失败:", error);
    }
    // 清理所有状态和本地存储
    authStore.logout();
    MessagePlugin.success(t("menu.logoutSuccess"));
    router.push("/login");
    return;
  } else {
    if (path === "creatChat") {
      // 如果在知识库详情页，跳转到全局对话创建页
      if (isInKnowledgeBase.value) {
        router.push("/platform/creatChat");
      } else {
        // 如果不在知识库内，进入对话创建页
        router.push(`/platform/creatChat`);
      }
    } else {
      router.push(`/platform/${path}`);
    }
  }
  getIcon(path);
};

const getImgSrc = (url: string) => {
  return new URL(`/src/assets/img/${url}`, import.meta.url).href;
};

const mouseenteMenu = (path: string) => {};
const mouseleaveMenu = (path: string) => {};

const onDragHandleMouseDown = (e: MouseEvent) => {
  e.preventDefault();
  const startX = e.clientX;
  const expandThreshold = 40;

  const onMouseMove = (ev: MouseEvent) => {
    if (ev.clientX - startX > expandThreshold) {
      uiStore.expandSidebar();
      cleanup();
    }
  };
  const onMouseUp = () => cleanup();
  const cleanup = () => {
    document.removeEventListener("mousemove", onMouseMove);
    document.removeEventListener("mouseup", onMouseUp);
  };
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", onMouseUp);
};
</script>
<style scoped>
/*
 * Two things here are beyond utilities.
 *
 * The session list's scrollbar: thin and invisible until the list is
 * hovered, then a rounded grey bar (brighter in dark mode, where the grey
 * would vanish). That is scrollbar pseudo-elements and scrollbar-color,
 * keyed on the container's hover.
 *
 * SessionSidebarRow and SessionSourceFilter leave parts of their look to
 * the sidebar and expose hook classes for it (their templates say so). The
 * rules reach into those child components, and the "more" button's reveal
 * depends on this component's row state, so they are :deep() selectors.
 */
.menu_top {
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
  transition: scrollbar-color 0.2s ease;
}

.menu_top::-webkit-scrollbar {
  width: 6px;
}

.menu_top::-webkit-scrollbar-track {
  background: transparent;
}

.menu_top::-webkit-scrollbar-thumb {
  background-color: transparent;
  border-radius: 6px;
  transition: background-color 0.2s ease;
}

.menu_top:hover {
  scrollbar-color: var(--td-scrollbar-color, rgba(0, 0, 0, 0.18)) transparent;
}

.menu_top:hover::-webkit-scrollbar-thumb {
  background-color: var(--td-scrollbar-color, rgba(0, 0, 0, 0.18));
}

.menu_top::-webkit-scrollbar-thumb:hover {
  background-color: var(--td-scrollbar-hover-color, rgba(0, 0, 0, 0.32));
}

html[theme-mode="dark"] .menu_top:hover {
  scrollbar-color: rgba(255, 255, 255, 0.22) transparent;
}

html[theme-mode="dark"] .menu_top:hover::-webkit-scrollbar-thumb {
  background-color: rgba(255, 255, 255, 0.22);
}

html[theme-mode="dark"] .menu_top::-webkit-scrollbar-thumb:hover {
  background-color: rgba(255, 255, 255, 0.38);
}

/* The source filter stays out of the way until the list is hovered or focused, or a source is picked. */
.session-list-scope-header :deep(.session-source-filter--inline) {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 100%;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.session-submenu:hover .session-list-scope-header :deep(.session-source-filter--inline),
.session-list-scope-header:hover :deep(.session-source-filter--inline),
.session-list-scope-header:focus-within :deep(.session-source-filter--inline),
.session-list-scope-header :deep(.session-source-filter--inline.session-source-filter--emphasized) {
  opacity: 1;
}

/* SessionSidebarRow's row, with the title truncating inside it. */
.session-chat-row :deep(.submenu_item) {
  cursor: pointer;
  display: flex;
  align-items: center;
  color: var(--td-text-color-primary);
  font-weight: 400;
  font-size: 14px;
  line-height: 20px;
  height: 100%;
  width: 100%;
  padding: 6px 0;
  position: relative;
  min-width: 0;
  background: transparent;
}

.session-chat-row :deep(.submenu_item_batch) {
  padding-left: 0;
  cursor: pointer;
  user-select: none;
}

.session-chat-row :deep(.submenu_title) {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
}

.session-chat-row :deep(.submenu_title--batch) {
  margin-left: 4px;
}

.session-chat-row :deep(.submenu_title-text) {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.session-chat-row :deep(.submenu_pin_icon) {
  color: inherit;
  margin-right: 4px;
  vertical-align: middle;
  flex-shrink: 0;
}

.session-chat-row :deep(.menu-more-wrap) {
  opacity: 0;
  transition: opacity 0.2s ease;
  flex-shrink: 0;
}

.session-chat-row :deep(.menu-more) {
  display: inline-block;
  font-weight: bold;
  color: var(--td-brand-color);
}

/* A hovered or active row shows its "more" button and darkens the glyph; the active row's title is brand-coloured. */
.session-chat-row:hover :deep(.menu-more-wrap),
.session-chat-row--active :deep(.menu-more-wrap) {
  opacity: 1;
}

.session-chat-row:hover :deep(.menu-more),
.session-chat-row--active :deep(.menu-more) {
  color: var(--td-text-color-primary);
}

.session-chat-row--active :deep(.submenu_item) {
  color: var(--td-brand-color);
}
</style>
