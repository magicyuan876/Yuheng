<template>
  <div ref="selectorRef" data-slot="tenant-selector" class="relative mb-3">
    <!--
      data-slot opts the hand-written search input and buttons into the
      element resets of tailwind.css, so they render without browser chrome.
    -->
    <div
      class="border-border bg-secondary hover:bg-accent flex cursor-pointer items-center rounded-lg border-[0.5px] px-3 py-2.5 transition-all duration-200"
      @click="toggleDropdown"
    >
      <div class="min-w-0 flex-1">
        <div class="text-placeholder mb-0.5 text-[11px] font-medium">{{ $t("tenant.currentTenant") }}</div>
        <div class="flex items-center justify-between gap-2">
          <span class="text-foreground flex-1 truncate text-[14px] font-semibold">{{ currentTenantName }}</span>
          <ArrowLeftRightIcon class="text-primary size-3.5 shrink-0" />
        </div>
      </div>
    </div>

    <Transition
      enter-active-class="transition-all duration-200 ease-[cubic-bezier(0.4,0,0.2,1)]"
      leave-active-class="transition-all duration-200 ease-[cubic-bezier(0.4,0,0.2,1)]"
      enter-from-class="opacity-0 -translate-y-1.5"
      leave-to-class="opacity-0 -translate-y-1.5"
    >
      <div
        v-if="showDropdown"
        class="border-border bg-popover absolute top-[calc(100%+4px)] right-0 left-0 z-[1000] overflow-hidden rounded-[10px] border-[0.5px] shadow-[0_6px_24px_rgba(0,0,0,0.12)]"
        @click.stop
      >
        <div class="border-border border-b-[0.5px] p-3">
          <span class="text-muted-foreground mb-2 block text-[12px] font-semibold">{{
            $t("tenant.switchTenant")
          }}</span>
          <div
            class="bg-secondary focus-within:border-primary focus-within:bg-card flex items-center gap-1.5 rounded-md border-[0.5px] border-transparent px-2.5 py-[7px] transition-all duration-200 focus-within:shadow-[0_0_0_2px_rgba(7,192,95,0.1)]"
          >
            <SearchIcon class="text-placeholder size-3.5 shrink-0" />
            <input
              ref="searchInput"
              v-model="searchQuery"
              type="text"
              :placeholder="$t('tenant.searchPlaceholder')"
              class="text-foreground min-w-0 flex-1 border-none text-[13px] outline-none"
              @keydown.esc="closeDropdown"
              @input="handleSearchInput"
            />
            <button
              v-if="searchQuery"
              type="button"
              class="text-placeholder hover:text-muted-foreground flex shrink-0 transition-colors duration-200"
              :aria-label="$t('common.clear')"
              @click="clearSearch"
            >
              <CircleXIcon class="size-3.5" />
            </button>
          </div>
        </div>

        <!-- The thin scrollbar is drawn with arbitrary variants on the WebKit pseudo-elements. -->
        <div
          ref="tenantListRef"
          class="[&::-webkit-scrollbar-thumb]:bg-secondary max-h-[280px] overflow-y-auto p-1.5 [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-[2px] [&::-webkit-scrollbar-thumb:hover]:bg-[var(--td-bg-color-component-disabled)] [&::-webkit-scrollbar-track]:bg-transparent"
          @scroll="handleScroll"
        >
          <div
            v-if="loading && tenants.length === 0"
            class="text-placeholder flex flex-col items-center justify-center gap-2 px-3 py-6 text-[13px]"
          >
            <Loader2Icon class="text-primary size-4 animate-spin" />
            <span>{{ $t("tenant.loading") }}</span>
          </div>

          <template v-else-if="tenants.length > 0">
            <div
              v-for="tenant in tenants"
              :key="tenant.id"
              class="mb-0.5 flex cursor-pointer items-center justify-between rounded-md px-2.5 py-2 transition-all duration-150 last:mb-0"
              :class="isSelected(tenant.id) ? 'bg-[rgba(7,192,95,0.08)]' : 'hover:bg-secondary'"
              @click="selectTenant(tenant.id)"
            >
              <div class="flex min-w-0 flex-1 items-center gap-2.5">
                <div
                  class="flex size-8 shrink-0 items-center justify-center rounded-md text-[13px] font-semibold transition-all duration-200"
                  :class="
                    isSelected(tenant.id)
                      ? 'text-primary-foreground bg-[linear-gradient(135deg,var(--td-brand-color)_0%,var(--td-brand-color-active)_100%)]'
                      : 'bg-secondary text-muted-foreground'
                  "
                >
                  {{ tenant.name.charAt(0).toUpperCase() }}
                </div>
                <div class="flex min-w-0 flex-1 flex-col gap-px">
                  <span
                    class="truncate text-[13px]"
                    :class="isSelected(tenant.id) ? 'text-primary font-medium' : 'text-foreground'"
                    >{{ tenant.name }}</span
                  >
                  <span class="text-placeholder text-[11px]">ID: {{ tenant.id }}</span>
                </div>
              </div>
              <CheckIcon v-if="isSelected(tenant.id)" class="text-primary size-4 shrink-0" />
            </div>
          </template>

          <div v-else class="text-placeholder flex flex-col items-center justify-center gap-2 px-3 py-6 text-[13px]">
            <span>{{ $t("tenant.noMatch") }}</span>
          </div>

          <div v-if="loading && tenants.length > 0" class="flex justify-center p-2">
            <Loader2Icon class="text-primary size-4 animate-spin" />
          </div>
        </div>

        <!-- 自助创建入口与 /auth/me 返回的后端能力保持一致。 -->
        <div
          v-if="authStore.canCreateTenant"
          class="border-border text-primary mx-1.5 mt-1 mb-1.5 flex cursor-pointer items-center gap-2 rounded-md border-t-[0.5px] px-3 py-2.5 text-[13px] font-medium transition-colors duration-150 hover:bg-[rgba(7,192,95,0.08)]"
          @click="openCreateDialog"
        >
          <PlusIcon class="size-3.5 shrink-0" />
          <span class="flex-1 truncate">{{ $t("tenant.create.action") }}</span>
        </div>
      </div>
    </Transition>

    <!-- 遮罩层 -->
    <div v-if="showDropdown" class="fixed inset-0 z-[999]" @click="closeDropdown"></div>

    <!-- 创建工作区弹窗：复用共享组件，TenantSelector 与 UserMenu 都用它 -->
    <CreateTenantDialog v-model:visible="createDialogVisible" @created="onTenantCreated" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from "vue";
import { useAuthStore } from "@/stores/auth";
import { searchTenants, type TenantInfo } from "@/api/tenant";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { ArrowLeftRightIcon, CheckIcon, CircleXIcon, Loader2Icon, PlusIcon, SearchIcon } from "@lucide/vue";
import {
  navigateAfterTenantSwitch,
  persistLastActiveTenantPreference,
  stashTenantSwitchToast,
} from "@/utils/tenantSwitch";
import CreateTenantDialog from "@/components/CreateTenantDialog.vue";
import { useRoleLabel } from "@/composables/useRoleLabel";

const { t } = useI18n();
const authStore = useAuthStore();
const { formatRole } = useRoleLabel();

const showDropdown = ref(false);
const searchQuery = ref("");
const tenants = ref<TenantInfo[]>([]);
const selectorRef = ref<HTMLElement | null>(null);
const tenantListRef = ref<HTMLElement | null>(null);
const searchInput = ref<HTMLInputElement | null>(null);

// 分页相关
const currentPage = ref(1);
const pageSize = ref(20);
const total = ref(0);
const loading = ref(false);
const searchTimer = ref<number | null>(null);

const selectedTenantId = computed(() => authStore.selectedTenantId);
// home 空间 id 来自 user.tenant_id（注册时分配、永不变）。不要读
// authStore.tenant.id —— 那是当前激活空间，会随 X-Tenant-ID 切换；用它
// 当 home 会让「切回 home」分支错判，详见 useHomeTenant() 注释。
const defaultTenantId = computed(() => (authStore.user?.tenant_id ? Number(authStore.user.tenant_id) : null));

const currentTenantId = computed(() => {
  return selectedTenantId.value || defaultTenantId.value;
});

const currentTenantName = computed(() => {
  if (!currentTenantId.value) return t("tenant.unknown");
  // 首先从当前加载的空间列表中查找
  const tenant = tenants.value.find((t) => t.id === currentTenantId.value);
  if (tenant) return tenant.name;
  // 如果是选中的空间，使用保存的空间名称
  if (selectedTenantId.value && authStore.selectedTenantName) {
    return authStore.selectedTenantName;
  }
  // 最后使用默认空间名称
  return authStore.tenant?.name || t("tenant.unknown");
});

const hasMore = computed(() => {
  return tenants.value.length < total.value;
});

const isSelected = (tenantId: number) => {
  return currentTenantId.value === tenantId;
};

const toggleDropdown = () => {
  showDropdown.value = !showDropdown.value;
  if (showDropdown.value) {
    if (tenants.value.length === 0) {
      loadTenants();
    }
    nextTick(() => {
      searchInput.value?.focus();
    });
  }
};

const closeDropdown = () => {
  showDropdown.value = false;
  searchQuery.value = "";
  currentPage.value = 1;
  if (searchTimer.value) {
    clearTimeout(searchTimer.value);
    searchTimer.value = null;
  }
};

const clearSearch = () => {
  searchQuery.value = "";
  currentPage.value = 1;
  tenants.value = [];
  total.value = 0;
  loadTenants();
};

const selectTenant = (tenantId: number) => {
  // 找到选中的空间信息
  const selectedTenant = tenants.value.find((t) => t.id === tenantId);

  // 始终写入 override，让 request.ts 永远附 X-Tenant-ID 覆盖 JWT；不要因为
  // 切到 home 就清空（详见 UserMenu.switchToTenant 同名注释）。服务端持久化
  // 偏好仍然区分对待——切到 home 时清 last_active，让下次干净重登回到 home。
  const switchingToHome = tenantId === defaultTenantId.value;
  // 切到 home 时，selectedTenant 可能因为分页 / 搜索没把 home 加载进列表，
  // 退而求其次从 memberships 上挑名字。注意不要回退到 authStore.tenant?.name
  // —— 那是当前激活空间的名字，在 active != home 的会话里就是 peer 的名字。
  const homeNameFallback = switchingToHome
    ? (authStore.memberships ?? []).find((m) => Number(m.tenant_id) === tenantId)?.tenant_name || null
    : null;
  authStore.setSelectedTenant(tenantId, selectedTenant?.name || homeNameFallback || null);
  closeDropdown();
  const displayName = selectedTenant?.name || homeNameFallback || `#${tenantId}`;
  // Cross-tenant superusers may not have a membership row in the target
  // tenant; in that case skip the role line rather than show a misleading
  // empty/raw value.
  const membership = (authStore.memberships ?? []).find((m) => Number(m.tenant_id) === tenantId);
  const roleLabel = membership ? formatRole(membership.role) : "";
  // Toast 在 reload 后由 App.vue 弹出（直接在这里弹会被 hard reload 干掉）。
  stashTenantSwitchToast({
    name: displayName,
    role: roleLabel || undefined,
    roleEnum: membership?.role || undefined,
  });
  // Persist "last active tenant" preference (switching to home clears
  // it). Fire-and-forget, but race it against the existing 500ms grace
  // window so most writes finish before the hard reload tears the page
  // down. 切换空间后跳转到新空间下安全的入口（详见 tenantSwitch.ts 注释）。
  const persist = persistLastActiveTenantPreference(switchingToHome ? null : tenantId);
  Promise.race([persist, new Promise((r) => setTimeout(r, 500))]).finally(() => navigateAfterTenantSwitch());
};

const loadTenants = async (append = false) => {
  if (loading.value) return;

  loading.value = true;
  try {
    const keyword = searchQuery.value.trim();
    let tenantID: number | undefined = undefined;

    // 如果是纯数字，同时作为 tenant_id 和 keyword 搜索
    // 这样既能精确匹配空间ID，也能模糊匹配名称中包含数字的空间
    if (keyword && /^\d+$/.test(keyword)) {
      tenantID = Number(keyword);
    }

    const response = await searchTenants({
      keyword: keyword || undefined,
      tenant_id: tenantID,
      page: currentPage.value,
      page_size: pageSize.value,
    });

    if (response.success && response.data) {
      if (append) {
        tenants.value = [...tenants.value, ...response.data.items];
      } else {
        tenants.value = response.data.items;
      }
      total.value = response.data.total;
      authStore.setAllTenants(tenants.value);
    } else {
      MessagePlugin.error(response.message || t("tenant.loadTenantsFailed"));
    }
  } catch (error) {
    console.error("Failed to load tenants:", error);
    MessagePlugin.error(t("tenant.loadTenantsFailed"));
  } finally {
    loading.value = false;
  }
};

const handleSearchInput = () => {
  if (searchTimer.value) {
    clearTimeout(searchTimer.value);
  }

  searchTimer.value = window.setTimeout(() => {
    currentPage.value = 1;
    tenants.value = [];
    total.value = 0;
    loadTenants();
  }, 300);
};

const handleScroll = () => {
  if (!tenantListRef.value) return;

  const { scrollTop, scrollHeight, clientHeight } = tenantListRef.value;
  const isNearBottom = scrollHeight - scrollTop - clientHeight < 50;

  if (isNearBottom && hasMore.value && !loading.value) {
    currentPage.value++;
    loadTenants(true);
  }
};

// ---- 创建新工作区 ----
// dialog 由共享组件 CreateTenantDialog 渲染，这里只负责打开 / 接收创建结果。
const createDialogVisible = ref(false);

const openCreateDialog = () => {
  closeDropdown();
  if (!authStore.canCreateTenant) {
    MessagePlugin.info(t("tenant.create.disabled"));
    return;
  }
  createDialogVisible.value = true;
};

const onTenantCreated = async (newTenant: TenantInfo) => {
  // 把新空间合并进当前列表并切过去。和 selectTenant 走同一条链路：
  // setSelectedTenant + navigateAfterTenantSwitch。后端 X-Tenant-ID 中
  // 间件会查 tenant_members 校验，EnsureOwner 已经在后端写好 owner 行。
  tenants.value = [newTenant, ...tenants.value.filter((t) => t.id !== newTenant.id)];
  total.value = total.value + 1;
  authStore.setAllTenants(tenants.value);
  await authStore.refreshFromAuthMe();
  authStore.setSelectedTenant(newTenant.id, newTenant.name);
  // Newly-created tenant becomes the user's "last active" so re-login
  // lands here. Race against the existing grace window before reload.
  const persist = persistLastActiveTenantPreference(newTenant.id);
  Promise.race([persist, new Promise((r) => setTimeout(r, 300))]).finally(() => navigateAfterTenantSwitch());
};

onMounted(() => {
  // 预加载空间列表
  loadTenants();
});

onUnmounted(() => {
  if (searchTimer.value) {
    clearTimeout(searchTimer.value);
  }
});
</script>
