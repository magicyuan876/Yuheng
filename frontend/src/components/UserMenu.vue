<template>
  <div class="relative w-full" ref="menuRef">
    <!-- 用户按钮 -->
    <div
      class="hover:bg-accent flex cursor-pointer items-center rounded-lg bg-transparent transition-all duration-200 active:scale-[0.98]"
      :class="uiStore.sidebarCollapsed ? 'justify-center gap-0 px-[3px] py-1.5' : 'gap-1.5 px-1.5 py-2'"
      data-guide="user-menu"
      @click="toggleMenu"
    >
      <div
        class="from-primary flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-full bg-linear-135 to-[var(--td-brand-color-active)] transition-[width,height] duration-200 ease-in-out"
      >
        <img v-if="userAvatar" :src="userAvatar" :alt="$t('common.avatar')" class="size-full object-cover" />
        <span v-else class="text-primary-foreground text-xs leading-none font-semibold">{{ userInitial }}</span>
      </div>
      <template v-if="!uiStore.sidebarCollapsed">
        <div class="flex min-w-0 flex-1 flex-col justify-center gap-0.5 text-left">
          <!-- 多空间 / superuser：首行空间名，次行 username · 角色。单空间：昵称 + 邮箱。 -->
          <template v-if="showTenantIdentityLine">
            <div
              class="text-foreground truncate text-sm leading-[1.35] font-semibold tracking-[-0.01em]"
              :title="activeTenantName"
            >
              {{ activeTenantName }}
            </div>
            <div class="text-muted-foreground mt-0 flex min-w-0 items-center gap-1 text-xs leading-[1.35]">
              <span v-if="userName && userName !== activeTenantName" class="min-w-0 flex-[0_1_auto] truncate">{{
                userName
              }}</span>
              <span
                v-if="userName && userName !== activeTenantName && currentRoleLabel"
                class="text-placeholder shrink-0"
                >·</span
              >
              <component :is="currentRoleIcon" v-if="currentRoleIcon" class="size-3 shrink-0" />
              <span v-if="currentRoleLabel" class="shrink-0">{{ currentRoleLabel }}</span>
            </div>
          </template>
          <template v-else>
            <div class="text-foreground truncate text-sm font-medium">{{ userName }}</div>
            <div class="text-muted-foreground truncate text-xs">{{ userEmail }}</div>
          </template>
        </div>
        <component
          :is="menuVisible ? ChevronUpIcon : ChevronDownIcon"
          class="text-muted-foreground size-4 shrink-0 transition-transform duration-200"
        />
      </template>
    </div>

    <!-- 下拉菜单 -->
    <Transition
      enter-active-class="transition-all duration-200 ease-[cubic-bezier(0.4,0,0.2,1)]"
      leave-active-class="transition-all duration-200 ease-[cubic-bezier(0.4,0,0.2,1)]"
      enter-from-class="translate-y-2 opacity-0"
      leave-to-class="translate-y-2 opacity-0"
    >
      <!--
        Expanded, the dropdown spans the sidebar width (the right edge is
        inset so it does not sit exactly on the content boundary). Collapsed,
        it opens to the right of the rail at the width of the expanded
        sidebar (260px), so both states show the same menu.
      -->
      <div
        v-if="menuVisible"
        class="bg-card border-border absolute z-[1000] overflow-hidden rounded-lg border shadow-[0_4px_20px_rgba(0,0,0,0.12)]"
        :class="
          uiStore.sidebarCollapsed
            ? 'bottom-0 left-[calc(100%+8px)] mb-1.5 min-w-[260px]'
            : 'right-[-5px] bottom-full left-[-4px] mb-1.5'
        "
        @click.stop
      >
        <!-- 弹出菜单：账号（头像+昵称）／当前空间（名称+权限）；底部侧栏样式不改。 -->
        <!--
          账号区：24px 头像中心与下方 16px 菜单图标中心同竖线；
          头像 margin-left −4px、gap 6px 保持昵称起点与菜单文案对齐（12 + 24 + 6 − 4 = 38）。
        -->
        <div
          v-if="userName"
          class="hover:bg-accent focus-visible:bg-accent flex min-w-0 cursor-pointer items-center gap-1.5 px-3 py-[9px] transition-colors duration-150 ease-in-out focus-visible:outline-none"
          role="button"
          tabindex="0"
          @click="handleQuickNav('userprofile')"
          @keydown.enter.prevent="handleQuickNav('userprofile')"
          @keydown.space.prevent="handleQuickNav('userprofile')"
        >
          <div
            class="from-primary -ml-1 flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-full bg-linear-135 to-[var(--td-brand-color-active)]"
          >
            <img v-if="userAvatar" :src="userAvatar" :alt="$t('common.avatar')" class="size-full object-cover" />
            <span v-else class="text-primary-foreground text-xs leading-none font-semibold">{{ userInitial }}</span>
          </div>
          <div class="flex min-w-0 flex-1 flex-col justify-center gap-0">
            <div class="flex min-w-0 items-center gap-0.5">
              <span class="text-foreground min-w-0 flex-1 truncate text-sm leading-[1.35] font-medium">{{
                userName
              }}</span>
              <Tooltip>
                <TooltipTrigger as-child>
                  <button
                    type="button"
                    data-slot="icon-button"
                    class="text-placeholder hover:bg-accent hover:text-muted-foreground flex size-5 shrink-0 items-center justify-center rounded-[4px] transition-colors duration-200 ease-in-out"
                    :aria-label="$t('newUserGuide.reopen')"
                    @click.stop="reopenGuide"
                  >
                    <CircleHelpIcon class="size-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("newUserGuide.reopen") }}</TooltipContent>
              </Tooltip>
            </div>
            <span v-if="userEmail" class="text-muted-foreground min-w-0 truncate text-xs leading-[1.35]">{{
              userEmail
            }}</span>
          </div>
        </div>

        <!-- 当前工作区：与下方菜单项同款对齐（左 16px 图标槽 + 文案列 + 右侧操作图标） -->
        <div
          v-if="userName"
          ref="tenantMenuItemRef"
          class="group border-border flex min-w-0 items-center gap-2.5 border-t bg-transparent px-3 py-[9px] transition-[background] duration-150 ease-in-out"
          :class="{
            'hover:bg-accent cursor-pointer': showTenantSwitcher,
            'bg-accent': showTenantSwitcher && tenantSubmenuOpen,
          }"
          @mouseenter="showTenantSwitcher && showTenantSubmenu()"
          @mouseleave="showTenantSwitcher && scheduleHideTenantSubmenu()"
        >
          <AtomIcon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
          <div class="flex min-w-0 flex-[1_1_auto] flex-col gap-px">
            <span
              class="text-foreground truncate text-sm leading-[1.35] font-medium"
              :title="activeTenantName || userName"
            >
              {{ activeTenantName || userName }}
            </span>
            <div
              v-if="currentRoleLabel"
              class="text-muted-foreground flex min-w-0 items-center gap-1 truncate text-xs leading-[1.35]"
            >
              <component :is="currentRoleIcon" v-if="currentRoleIcon" class="size-3 shrink-0" />
              <span>{{ currentRoleLabel }}</span>
            </div>
          </div>
          <span v-if="showTenantSwitcher" class="flex shrink-0" :title="$t('tenant.switcher.menuLabel')">
            <ArrowLeftRightIcon
              class="size-4 transition-colors duration-150 ease-in-out"
              :class="
                tenantSubmenuOpen ? 'text-muted-foreground' : 'text-placeholder group-hover:text-muted-foreground'
              "
            />
          </span>
        </div>
        <div class="bg-border mb-[3px] h-px" :class="userName ? 'mt-px' : 'mt-[3px]'"></div>
        <!-- 账号与空间是头像菜单的核心上下文；基础设施类配置统一收进「全部设置」。 -->
        <div :class="menuItemClass" @click="handleQuickNav('general')">
          <UserIcon :class="menuIconClass" />
          <span>{{ $t("general.personalSettings") }}</span>
        </div>
        <div :class="menuItemClass" @click="handleQuickNav('tenant')">
          <CircleUserIcon :class="menuIconClass" />
          <span>{{ $t("settings.workspaceSettings") }}</span>
        </div>
        <!-- “管理”类快捷入口只对真正具备写权限的人展示。只读名册和模型列表
             仍可从「全部设置」进入，避免 viewer 看到名不副实的管理入口。 -->
        <div v-if="canManageMembers" :class="menuItemClass" @click="handleQuickNav('members')">
          <UsersIcon :class="menuIconClass" />
          <span>{{ $t("tenantMember.title") }}</span>
        </div>
        <div v-if="canManageModels" :class="menuItemClass" @click="handleQuickNav('models')">
          <BoxIcon :class="menuIconClass" />
          <span>{{ $t("settings.modelManagement") }}</span>
        </div>
        <div class="bg-border my-[3px] h-px"></div>
        <div :class="menuItemClass" @click="handleSettings">
          <SettingsIcon :class="menuIconClass" />
          <span>{{ $t("general.allSettings") }}</span>
        </div>
        <!--
          System administration entry — visible only to users with the
          platform-wide is_system_admin flag. Hidden for everyone else,
          including tenant Owners. Real authorisation lives server-side
          (RequireSystemAdmin middleware); this is UI gating only.
        -->
        <div v-if="authStore.isSystemAdmin" :class="menuItemClass" @click="handleSystemAdmin">
          <ServerIcon :class="menuIconClass" />
          <span>{{ $t("settings.navGroups.systemAdministration") }}</span>
        </div>
        <div class="bg-border my-[3px] h-px"></div>
        <div v-if="DOCS_BASE_URL" :class="['group', menuItemClass]" @click="openDocs">
          <CircleHelpIcon :class="menuIconClass" />
          <span class="flex min-w-0 flex-1 items-center gap-1.5 text-inherit">
            <span class="inline-flex min-w-0 items-center truncate">{{ $t("general.helpAndDocs") }}</span>
            <svg :class="externalIconClass" viewBox="0 0 16 16" aria-hidden="true">
              <path
                fill="currentColor"
                d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667a2.667 2.667 0 0 1-2.667-2.666V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334A1.333 1.333 0 0 0 4.667 13.333h6a1.333 1.333 0 0 0 1.333-1.333v-4A.667.667 0 0 1 12.667 8Zm2.666-6.667v4a.667.667 0 0 1-1.333 0V3.276l-5.195 5.195a.667.667 0 0 1-.943-.943l5.195-5.195h-2.057a.667.667 0 0 1 0-1.333h4a.667.667 0 0 1 .666.666Z"
              />
            </svg>
          </span>
        </div>
        <div v-if="REPO_URL" :class="['group', menuItemClass]" :title="$t('common.githubStarTip')" @click="openGithub">
          <!-- lucide ships no brand marks, so the GitHub logo is TDesign's outline drawn inline. -->
          <svg
            :class="menuIconClass"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            aria-hidden="true"
          >
            <path
              d="M8.50772 23C8.50772 22.8405 8.50574 22.6061 8.50311 22.2389C8.49789 21.5081 8.49009 20.4149 8.49009 19.1577C8.49009 17.8337 8.93142 16.9864 9.44334 16.5451C6.3011 16.1921 3 14.9917 3 9.57219C3 8.01876 3.54726 6.76529 4.44754 5.7768C4.3063 5.42374 3.81205 3.97616 4.58877 2.03433C4.58877 2.03433 5.77151 1.64593 8.47242 3.48191C9.6003 3.16471 10.8016 3.00052 12 3C13.1984 3.00052 14.3997 3.16471 15.5276 3.48191C18.2285 1.64593 19.4112 2.03433 19.4112 2.03433C20.1879 3.97616 19.6937 5.42374 19.5525 5.7768C20.4527 6.76529 21 8.01876 21 9.57219C21 14.9917 17.6989 16.1921 14.5567 16.5451C15.0686 16.9864 15.5099 17.8337 15.5099 19.1577C15.5099 20.4149 15.5021 21.5081 15.4969 22.2389C15.4943 22.6061 15.4923 22.8405 15.4923 23M2.5 17.5L2.7142 17.6071C3.2319 17.8659 3.68725 18.2341 4.04882 18.686L4.59927 19.3741C5.16858 20.0857 6.03052 20.5 6.94187 20.5H8.5"
            />
          </svg>
          <span class="flex min-w-0 flex-1 items-center gap-1.5 text-inherit">
            <span class="inline-flex min-w-0 items-center truncate">{{ $t("common.github") }}</span>
            <StarIcon class="text-warning size-4 shrink-0 fill-current" aria-hidden="true" />
            <svg :class="externalIconClass" viewBox="0 0 16 16" aria-hidden="true">
              <path
                fill="currentColor"
                d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667a2.667 2.667 0 0 1-2.667-2.666V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334A1.333 1.333 0 0 0 4.667 13.333h6a1.333 1.333 0 0 0 1.333-1.333v-4A.667.667 0 0 1 12.667 8Zm2.666-6.667v4a.667.667 0 0 1-1.333 0V3.276l-5.195 5.195a.667.667 0 0 1-.943-.943l5.195-5.195h-2.057a.667.667 0 0 1 0-1.333h4a.667.667 0 0 1 .666.666Z"
              />
            </svg>
          </span>
        </div>
        <!--
          Logout. This block used to sit in a bare <template> left behind
          when its v-if was removed; Vue 3 renders a directive-less
          <template> as an inert native element, so the entry never showed.
        -->
        <div class="bg-border my-[3px] h-px"></div>
        <div
          class="text-destructive flex cursor-pointer items-center gap-2.5 px-3 py-[9px] text-sm transition-all duration-200 hover:bg-[var(--td-error-color-light)]"
          @click="handleLogout"
        >
          <LogOutIcon class="text-destructive size-4 shrink-0" />
          <span>{{ $t("auth.logout") }}</span>
        </div>
      </div>
    </Transition>

    <!-- Tenant switcher floating panel — shares the same teleport rationale
         as the IM submenu. Data comes from authStore.memberships, kept fresh via
         GET /auth/me when the submenu opens (throttled) and after invite/create.
         `tenant-submenu-floating` is a hook class: the viewport clamp and the
         click-outside handler find the panel by it. The 2px left padding is a
         pointer bridge, so sliding off the menu item onto the panel does not
         cross a gap and trigger the mouseleave hide. -->
    <Teleport to="body">
      <div
        v-if="tenantSubmenuOpen"
        class="tenant-submenu-floating bg-card border-border fixed z-[1100] flex max-h-[340px] w-[264px] flex-col overflow-hidden rounded-[10px] border-[0.5px] pl-0.5 shadow-[0_6px_24px_rgba(0,0,0,0.12)]"
        :style="tenantSubmenuStyle"
        @mouseenter="showTenantSubmenu"
        @mouseleave="scheduleHideTenantSubmenu"
      >
        <div class="text-muted-foreground border-border border-b-[0.5px] px-3 pt-2 pb-1.5 text-xs font-semibold">
          {{ $t("tenant.switcher.menuLabel") }}
        </div>
        <div class="overflow-y-auto p-1">
          <div
            v-for="m in switchableMemberships"
            :key="m.tenant_id"
            class="flex items-center gap-2 rounded-md px-2 py-[7px] transition-[background] duration-150"
            :class="isCurrentTenant(m.tenant_id) ? 'bg-secondary cursor-default' : 'hover:bg-secondary cursor-pointer'"
            @click="switchToTenant(m)"
          >
            <div
              class="relative flex size-7 shrink-0 items-center justify-center rounded-md text-[13px] font-semibold"
              :class="
                isCurrentTenant(m.tenant_id)
                  ? 'from-primary text-primary-foreground bg-linear-135 to-[var(--td-brand-color-active)]'
                  : 'bg-secondary text-muted-foreground'
              "
            >
              {{ tenantInitial(m) }}
            </div>
            <!-- 两行布局：第一行是 tenant 名（拿满剩余宽度，避免被徽标截断
                 — 长 tenant 名和徽标同行时会被压成省略号）；第二行 role
                 （带角色图标） + 「当前」徽标。 -->
            <div class="flex min-w-0 flex-1 flex-col gap-0.5">
              <span
                class="text-foreground truncate text-[13px]"
                :class="{ 'font-semibold': isCurrentTenant(m.tenant_id) }"
                >{{ tenantDisplayName(m) }}</span
              >
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                <span class="text-placeholder inline-flex items-center gap-1 text-[11px]">
                  <!-- 角色图标颜色继承 role 文字色，避免抢走视觉 -->
                  <component :is="roleIconComponent(m.role)" v-if="roleIconComponent(m.role)" class="size-3 shrink-0" />
                  {{ formatRole(m.role) }}
                </span>
                <span
                  v-if="isCurrentTenant(m.tenant_id)"
                  class="text-muted-foreground shrink-0 rounded-[4px] bg-[var(--td-bg-color-component)] px-1.5 py-0.5 text-[10px] leading-[1.2] font-semibold"
                  >{{ $t("tenant.switcher.currentBadge") }}</span
                >
              </div>
            </div>
          </div>
          <div v-if="switchableMemberships.length === 0" class="text-placeholder px-2.5 py-3 text-center text-xs">
            {{ $t("tenant.switcher.empty") }}
          </div>
        </div>
        <!-- 建空间入口只对系统管理员（/auth/me 的 can_create_tenant）显示：
             普通用户进入空间的途径是被邀请或被系统管理员加入。 -->
        <div
          v-if="authStore.canCreateTenant"
          class="text-primary border-border mx-1 mt-[3px] mb-[5px] flex cursor-pointer items-center gap-1.5 rounded-md border-t-[0.5px] px-2.5 py-2 text-sm font-medium transition-[background] duration-150 hover:bg-[rgba(7,192,95,0.08)]"
          @click="openCreateTenantDialog"
        >
          <PlusIcon class="size-4 shrink-0" />
          <span class="flex-1 truncate text-xs">{{ $t("tenant.create.action") }}</span>
        </div>
      </div>
    </Teleport>

    <!-- 创建工作区弹窗 -->
    <CreateTenantDialog v-model:visible="createTenantDialogVisible" @created="onTenantCreated" />
  </div>
</template>

<script setup lang="ts">
import { DOCS_BASE_URL, REPO_URL } from "@/config/externalLinks";
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useUIStore } from "@/stores/ui";
import { useAuthStore } from "@/stores/auth";
import { MessagePlugin } from "tdesign-vue-next";
import { getCurrentUser, logout as logoutApi, userInfoFromApi } from "@/api/auth";
import { useI18n } from "vue-i18n";
import CreateTenantDialog from "@/components/CreateTenantDialog.vue";
import {
  navigateAfterTenantSwitch,
  persistLastActiveTenantPreference,
  stashTenantSwitchToast,
} from "@/utils/tenantSwitch";
import type { TenantInfo } from "@/api/tenant";
import { useRoleLabel } from "@/composables/useRoleLabel";
import { getRootZoom, rectToCssPx, cssViewportSize } from "@/utils/zoom";
import { openNewUserGuide } from "@/config/contextualGuides";
import { SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE } from "@/config/settingsAccess";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  ArrowLeftRightIcon,
  AtomIcon,
  BoxIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CircleHelpIcon,
  CircleUserIcon,
  LogOutIcon,
  PlusIcon,
  ServerIcon,
  SettingsIcon,
  StarIcon,
  UserIcon,
  UsersIcon,
} from "@lucide/vue";

const { t } = useI18n();

const router = useRouter();
const uiStore = useUIStore();
const authStore = useAuthStore();
const { formatRole, roleIcon } = useRoleLabel();

// A role without an icon (an unknown role name) renders the label alone.
const roleIconComponent = roleIcon;

// The dropdown's rows share one look; the classes live here rather than being
// repeated on every row.
const menuItemClass =
  "text-foreground hover:bg-accent flex cursor-pointer items-center gap-2.5 px-3 py-[9px] text-sm transition-all duration-200";
const menuIconClass = "text-muted-foreground size-4 shrink-0";
const externalIconClass =
  "pointer-events-none size-4 shrink-0 text-[var(--td-text-color-disabled)] transition-colors duration-200 ease-in-out group-hover:text-primary";
// 顶部用户卡片展示的空间名 / 当前角色：跟着 tenant 切换器实时变。
// activeTenantName 优先用切换器选中的名字，否则是会话令牌所在空间的名字。
const activeTenantName = computed(() => {
  return authStore.selectedTenantName || authStore.tenant?.name || "";
});
const currentRoleLabel = computed(() => formatRole(authStore.currentTenantRole));
const currentRoleIcon = computed(() => roleIconComponent(authStore.currentTenantRole));

// 单空间用户（memberships <= 1 且非 superuser）只有一个可能的空间，第三
// 行就是 user-email 信息的重复，没必要占视觉空间；只对多空间 / superuser
// 渲染。
const showTenantIdentityLine = computed(() => {
  if (authStore.canAccessAllTenants) return true;
  return (authStore.memberships ?? []).length > 1;
});

// 快捷入口使用“管理能力”而不是页面最低可见角色：成员名册和模型列表允许
// viewer 浏览，但头像菜单里的“管理”入口只服务实际能执行管理操作的角色。
const canManageMembers = computed(
  () => authStore.canAccessAllTenants || authStore.hasRole(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE.members),
);
const canManageModels = computed(
  () =>
    authStore.canAccessAllTenants ||
    authStore.isSystemAdmin ||
    authStore.hasRole(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE.models),
);

const menuRef = ref<HTMLElement>();
const tenantMenuItemRef = ref<HTMLElement>();
const menuVisible = ref(false);
const tenantSubmenuOpen = ref(false);
const tenantSubmenuStyle = ref<Record<string, string>>({});
let tenantSubmenuHideTimer: ReturnType<typeof setTimeout> | null = null;

// 用户信息
const userInfo = ref({
  username: t("common.defaultUser"),
  email: "user@example.com",
  avatar: "",
});

const userName = computed(() => userInfo.value.username);
const userEmail = computed(() => userInfo.value.email);
const userAvatar = computed(() => userInfo.value.avatar);

// 用户名首字母（用于无头像时显示）
const userInitial = computed(() => {
  return userName.value.charAt(0).toUpperCase();
});

// 切换菜单显示
const toggleMenu = () => {
  menuVisible.value = !menuVisible.value;
};

// 快捷导航到设置的特定部分
const handleQuickNav = (section: string) => {
  menuVisible.value = false;
  uiStore.openSettings();
  router.push({ path: "/platform/settings", query: { section } });
};

// 打开设置
const handleSettings = () => {
  menuVisible.value = false;
  uiStore.openSettings();
  router.push("/platform/settings");
};

// Open the platform administration group inside the standard Settings
// modal. Global settings is the group's landing page; task queues, platform
// API keys and the audit log remain available beside it in the settings nav.
const handleSystemAdmin = () => {
  menuVisible.value = false;
  uiStore.openSettings("system-global");
  router.push({ path: "/platform/settings", query: { section: "system-global" } });
};

// Hover-driven submenu controls. A small hide delay tolerates the pointer
// slipping off briefly onto the gap between menu item and submenu pane.
const closeAll = () => {
  tenantSubmenuOpen.value = false;
  menuVisible.value = false;
};

// ---------- Create new tenant ----------
// 系统管理员在空间子菜单底部点 "+ 创建新空间" → 弹 CreateTenantDialog →
// 后端在同一请求里写入 Owner 的 tenant_members 行 → 切到新空间（管理员自己
// 是 Owner 时）。复用 switchToTenant 同款的 setSelectedTenant +
// navigateAfterTenantSwitch 链路，避免 token 依然指向旧空间带来的 SSE /
// store 不一致。
const createTenantDialogVisible = ref(false);

const openCreateTenantDialog = () => {
  closeAll();
  if (!authStore.canCreateTenant) {
    MessagePlugin.info(t("tenant.create.disabled"));
    return;
  }
  createTenantDialogVisible.value = true;
};

const onTenantCreated = async (newTenant: TenantInfo) => {
  await authStore.refreshFromAuthMe();
  authStore.setSelectedTenant(newTenant.id, newTenant.name);
  const persist = persistLastActiveTenantPreference(newTenant.id);
  Promise.race([persist, new Promise((r) => setTimeout(r, 300))]).finally(() => navigateAfterTenantSwitch());
};

// ---------- Tenant switcher submenu ----------
//
// Same hover-driven submenu pattern; data comes from
// authStore.memberships (refreshed from /auth/me when the submenu opens and
// after membership-changing actions). PR 4 of #1303 relaxed the X-Tenant-ID
// gate in middleware/auth.go to accept active membership rows, so flipping
// authStore.selectedTenantId here is enough — the next page reload re-issues
// every request with the new header and the server resolves the role server-side.
type Membership = {
  tenant_id: number;
  tenant_name?: string;
  role: string;
};

// switchableMemberships is the curated list shown in the dropdown. We keep
// the active tenant in there (with a "Current" badge) so the user has a
// single place to glance at "where am I right now"; clicking the current
// row is a no-op (handled in switchToTenant).
const switchableMemberships = computed<Membership[]>(() => {
  return authStore.memberships ?? [];
});

// Rendered whenever the user has at least one membership — single-
// workspace users see where they are, multi-workspace users switch between
// memberships, and system administrators find the "create new workspace"
// entry at the bottom. Cross-tenant superusers keep using the sidebar
// TenantSelector for the "any tenant in the system" case, so we don't
// double-show that here.
const showTenantSwitcher = computed(() => {
  return switchableMemberships.value.length >= 1;
});

const isCurrentTenant = (id: number) => {
  const active = authStore.effectiveTenantId;
  return active != null && Number(active) === Number(id);
};

const tenantDisplayName = (m: Membership) =>
  m.tenant_name && m.tenant_name.trim() !== "" ? m.tenant_name : `#${m.tenant_id}`;

const tenantInitial = (m: Membership) => {
  const name = tenantDisplayName(m).trim();
  return (name.charAt(0) || "?").toUpperCase();
};

const switchToTenant = (m: Membership) => {
  if (isCurrentTenant(m.tenant_id)) {
    closeAll();
    return;
  }
  // 始终把激活空间写进 selectedTenantId，让 request.ts 永远附 X-Tenant-ID：
  // 没有 override 时请求会落回 JWT 编码的空间，而那正是我们要离开的空间。
  authStore.setSelectedTenant(m.tenant_id, tenantDisplayName(m));
  closeAll();
  // Toast 在 reload 后由 App.vue 弹出（直接在这里弹会被 hard reload 干掉）。
  stashTenantSwitchToast({
    name: tenantDisplayName(m),
    role: formatRole(m.role) || undefined,
    roleEnum: m.role || undefined,
  });
  // Persist the workspace as the user's current one, so the next login
  // (any device) lands here. Hard reload so every cached store / open SSE
  // stream / in-flight request gets re-keyed under the new tenant;
  // navigateAfterTenantSwitch redirects to the platform home so
  // tenant-scoped resource paths don't white-screen. Race the persist
  // against the existing 400ms grace window so most writes complete
  // before the page tears down.
  const persist = persistLastActiveTenantPreference(m.tenant_id);
  Promise.race([persist, new Promise((r) => setTimeout(r, 400))]).finally(() => navigateAfterTenantSwitch());
};

let lastTenantSubmenuMembershipRefresh = 0;
const TENANT_SUBMENU_MEMBERSHIP_REFRESH_MS = 2000;

const showTenantSubmenu = () => {
  if (tenantSubmenuHideTimer) {
    clearTimeout(tenantSubmenuHideTimer);
    tenantSubmenuHideTimer = null;
  }
  positionTenantSubmenu();
  tenantSubmenuOpen.value = true;
  clampFloatingToViewport(".tenant-submenu-floating", tenantSubmenuStyle);
  const now = Date.now();
  if (now - lastTenantSubmenuMembershipRefresh >= TENANT_SUBMENU_MEMBERSHIP_REFRESH_MS) {
    lastTenantSubmenuMembershipRefresh = now;
    void authStore.refreshFromAuthMe();
  }
};

const scheduleHideTenantSubmenu = () => {
  if (tenantSubmenuHideTimer) clearTimeout(tenantSubmenuHideTimer);
  tenantSubmenuHideTimer = setTimeout(() => {
    tenantSubmenuOpen.value = false;
    tenantSubmenuHideTimer = null;
  }, 180);
};

const positionTenantSubmenu = () => {
  const el = tenantMenuItemRef.value;
  if (!el) return;
  // Submenu is rendered with `position: fixed` under the root zoom — see
  // `.tenant-submenu-floating` styles. Anchor coords come from a visual-pixel
  // rect; normalize to CSS pixels before writing them back to CSS.
  const zoom = getRootZoom();
  const rect = rectToCssPx(el.getBoundingClientRect(), zoom);
  const { width: vw } = cssViewportSize(zoom);
  const PANEL_WIDTH = 264;
  const GAP = 8;
  const MARGIN = 8;

  let left = rect.right + GAP;
  if (left + PANEL_WIDTH + MARGIN > vw) {
    left = Math.max(MARGIN, rect.left - PANEL_WIDTH - GAP);
  }

  const top = Math.max(MARGIN, rect.top);

  tenantSubmenuStyle.value = {
    left: `${left}px`,
    top: `${top}px`,
  };
};

// Anchor the floating submenu just to the right of the hovered menu item,
// clamped to the viewport so it stays visible near the screen edge.
const clampFloatingToViewport = (selector: string, target: { value: Record<string, string> }) => {
  requestAnimationFrame(() => {
    const panel = document.querySelector(selector) as HTMLElement | null;
    if (!panel) return;
    const MARGIN = 8;
    // `offsetHeight` and `target.value.top` are CSS pixels; `innerHeight` is
    // visual pixels under root zoom. Normalize the latter to keep the
    // comparison in one coordinate system.
    const { height: vh } = cssViewportSize();
    const h = panel.offsetHeight;
    const currentTop = parseFloat(target.value.top || "0") || 0;
    const maxTop = vh - h - MARGIN;
    if (currentTop > maxTop) {
      target.value = { ...target.value, top: `${Math.max(MARGIN, maxTop)}px` };
    }
  });
};

const reopenGuide = () => {
  menuVisible.value = false;
  openNewUserGuide();
};

const openDocs = () => {
  menuVisible.value = false;
  if (!DOCS_BASE_URL) return;
  window.open(DOCS_BASE_URL, "_blank");
};

// 打开 GitHub
const openGithub = () => {
  menuVisible.value = false;
  if (!REPO_URL) return;
  window.open(REPO_URL, "_blank");
};

// 注销
const handleLogout = async () => {
  menuVisible.value = false;

  try {
    // 调用后端API注销
    await logoutApi();
  } catch (error) {
    // 即使API调用失败，也继续执行本地清理
    console.error("注销API调用失败:", error);
  }

  // 清理所有状态和本地存储
  authStore.logout();

  MessagePlugin.success(t("auth.logout"));

  // 跳转到登录页
  router.push("/login");
};

// 加载用户信息
const loadUserInfo = async () => {
  try {
    const response = await getCurrentUser();
    if (response.success && response.data && response.data.user) {
      const user = response.data.user;
      userInfo.value = {
        username: user.username || t("common.info"),
        email: user.email || "user@example.com",
        avatar: user.avatar || "",
      };
      // 同时更新 authStore 中的用户信息，确保包含 can_access_all_tenants /
      // is_system_admin 等所有字段。MUST 走 userInfoFromApi 工厂——历史
      // 上这里手写字段白名单，每加一个 user 字段都要在 5 个 setUser 调用
      // 点同步，is_system_admin 就因为漏了这一处导致进入 platform 后
      // user.value 的字段被 mount 时的 loadUserInfo 静默覆盖回 undefined
      // （同时污染 localStorage），系统管理入口在 hover 工作空间触发
      // refreshFromAuthMe 后才出现。新增字段请只改 userInfoFromApi。
      authStore.setUser(userInfoFromApi(user));
      // 如果返回了空间信息，也更新空间信息；tenantless 用户（/auth/me
      // 无 tenant）必须显式清空，否则会残留上一账号/上一会话的空间快照。
      if (response.data.tenant) {
        authStore.setTenant({
          id: String(response.data.tenant.id),
          name: response.data.tenant.name,
          owner_id: user.id,
          created_at: response.data.tenant.created_at,
          updated_at: response.data.tenant.updated_at,
        });
      } else {
        authStore.setTenant(null);
      }
      const membershipsSync = response.data.memberships;
      if (Array.isArray(membershipsSync)) {
        authStore.setMemberships(membershipsSync);
      }
      const canCreateTenant = response.data.capabilities?.can_create_tenant;
      if (typeof canCreateTenant === "boolean") {
        authStore.setCanCreateTenant(canCreateTenant);
      }
    }
  } catch (error) {
    console.error("Failed to load user info:", error);
  }
};

// 点击外部关闭菜单
const handleClickOutside = (e: MouseEvent) => {
  const target = e.target as Node;
  if (menuRef.value && menuRef.value.contains(target)) return;
  // Tenant submenu is teleported to body, so it's not inside menuRef.
  const tenantFloating = document.querySelector(".tenant-submenu-floating");
  if (tenantFloating && tenantFloating.contains(target)) return;
  menuVisible.value = false;
  tenantSubmenuOpen.value = false;
};

onMounted(() => {
  document.addEventListener("click", handleClickOutside);
  loadUserInfo();
});

onUnmounted(() => {
  document.removeEventListener("click", handleClickOutside);
});
</script>
