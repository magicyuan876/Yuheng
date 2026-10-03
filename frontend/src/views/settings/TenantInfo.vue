<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("tenant.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("tenant.sectionDescription") }}</p>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="text-muted-foreground flex items-center justify-center gap-3 py-10 text-sm">
      <Loader2Icon class="animate-spin" />
      <span>{{ $t("tenant.loadingInfo") }}</span>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="py-5">
      <!-- TDesign's error alert sat on the pale error tint with no border and
           dark text; only its icon was red. -->
      <Alert variant="destructive" class="text-foreground border-transparent bg-[var(--td-error-color-1)]">
        <CircleAlertIcon class="text-destructive!" />
        <AlertTitle>{{ error }}</AlertTitle>
        <AlertAction>
          <Button variant="outline" size="sm" @click="loadInfo">{{ $t("tenant.retry") }}</Button>
        </AlertAction>
      </Alert>
    </div>

    <!-- Content：信息列表 + 危险操作分区，避免与 setting-row 底边线混用虚线 -->
    <div v-else class="flex flex-col">
      <div class="flex flex-col">
        <!-- Tenant ID -->
        <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("tenant.details.idLabel") }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("tenant.details.idDescription") }}</p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
              {{ tenantInfo?.id || "-" }}
            </span>
          </div>
        </div>

        <!-- Tenant name -->
        <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("tenant.details.nameLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.details.nameDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <!-- 只读态：显示名称 + 编辑按钮（owner 才看得见编辑入口）。
               原地编辑取代弹窗：少一层视觉打断，与其它行的展示节奏一致。 -->
            <template v-if="!editing">
              <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
                {{ tenantInfo?.name || "-" }}
              </span>
              <Button
                v-if="canEditTenant"
                variant="ghost"
                size="icon-sm"
                class="shrink-0"
                :title="$t('tenant.details.editName')"
                :aria-label="$t('tenant.details.editName')"
                @click="startEditName"
              >
                <PencilIcon />
              </Button>
            </template>
            <!-- 编辑态：输入框 + 保存/取消。回车保存，Esc 取消。 -->
            <div v-else ref="nameEditRow" class="flex w-full items-center justify-end gap-2">
              <Input
                v-model="editName"
                :placeholder="$t('tenant.details.editNamePlaceholder')"
                :maxlength="64"
                :disabled="saving"
                class="max-w-[220px] flex-1"
                @keydown.enter="saveTenantName"
                @keydown="onEditKeydown"
              />
              <Button size="sm" :disabled="!canSubmit" @click="saveTenantName">
                <Loader2Icon v-if="saving" class="animate-spin" />
                {{ $t("tenant.details.editNameConfirm") }}
              </Button>
              <Button variant="outline" size="sm" :disabled="saving" @click="cancelEditName">
                {{ $t("tenant.details.editNameCancel") }}
              </Button>
            </div>
          </div>
        </div>

        <!-- Tenant description -->
        <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">
              {{ $t("tenant.details.descriptionLabel") }}
            </label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.details.descriptionDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <!-- 只读态：显示描述（空时给占位）+ 编辑按钮（owner 才看得见编辑入口）。
               与名称同款"原地编辑"模式，少一层弹窗打断。 -->
            <template v-if="!editingDescription">
              <!-- Multi-line, wrapping anywhere; an empty description shows the
                   placeholder text in the placeholder colour, inviting an edit. -->
              <span
                class="min-w-0 text-right text-sm break-words wrap-anywhere whitespace-pre-wrap"
                :class="tenantInfo?.description ? 'text-foreground' : 'text-placeholder'"
              >
                {{ tenantInfo?.description || $t("tenant.details.descriptionEmptyPlaceholder") }}
              </span>
              <Button
                v-if="canEditTenant"
                variant="ghost"
                size="icon-sm"
                class="shrink-0"
                :title="$t('tenant.details.editDescription')"
                :aria-label="$t('tenant.details.editDescription')"
                @click="startEditDescription"
              >
                <PencilIcon />
              </Button>
            </template>
            <!-- 编辑态：textarea + 保存/取消。Esc 取消、Ctrl/⌘+Enter 保存；
               textarea 上 Enter 默认换行更顺手，不接管 Enter 提交。 -->
            <div
              v-else
              ref="descriptionEditRow"
              class="flex w-full max-w-[360px] flex-col items-stretch justify-end gap-2"
            >
              <Textarea
                v-model="editDescription"
                :placeholder="$t('tenant.details.editDescriptionPlaceholder')"
                :maxlength="512"
                rows="2"
                :disabled="savingDescription"
                class="max-h-[140px] w-full"
                @keydown="onEditDescriptionKeydown"
              />
              <div class="flex justify-end gap-2">
                <Button size="sm" :disabled="!canSubmitDescription" @click="saveTenantDescription">
                  <Loader2Icon v-if="savingDescription" class="animate-spin" />
                  {{ $t("tenant.details.editNameConfirm") }}
                </Button>
                <Button variant="outline" size="sm" :disabled="savingDescription" @click="cancelEditDescription">
                  {{ $t("tenant.details.editNameCancel") }}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <!-- Tenant business -->
        <div
          v-if="tenantInfo?.business"
          class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
        >
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">
              {{ $t("tenant.details.businessLabel") }}
            </label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.details.businessDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
              {{ tenantInfo.business }}
            </span>
          </div>
        </div>

        <!-- Tenant status -->
        <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("tenant.details.statusLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.details.statusDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <Badge variant="secondary" :class="getStatusClass(tenantInfo?.status)">
              {{ getStatusText(tenantInfo?.status) }}
            </Badge>
          </div>
        </div>

        <!-- Tenant creation time -->
        <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">
              {{ $t("tenant.details.createdAtLabel") }}
            </label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.details.createdAtDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
              {{ formatDate(tenantInfo?.created_at) }}
            </span>
          </div>
        </div>

        <!-- Storage quota -->
        <div
          v-if="tenantInfo?.storage_quota !== undefined"
          class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
        >
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("tenant.storage.quotaLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.storage.quotaDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <!-- A zero quota is "no limit", not "no space": the quota checks
                 skip it, and the default for new workspaces is 0. -->
            <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
              {{
                tenantInfo.storage_quota > 0 ? formatBytes(tenantInfo.storage_quota) : $t("tenant.storage.unlimited")
              }}
            </span>
          </div>
        </div>

        <!-- Used storage -->
        <div
          v-if="tenantInfo?.storage_quota !== undefined"
          class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
        >
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("tenant.storage.usedLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.storage.usedDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <span class="text-foreground min-w-0 text-right text-sm wrap-anywhere">
              {{ formatBytes(tenantInfo.storage_used || 0) }}
            </span>
          </div>
        </div>

        <!-- Storage usage (a percentage of a finite quota only) -->
        <div
          v-if="tenantInfo?.storage_quota"
          class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
        >
          <div class="w-max max-w-[40%] min-w-[140px] flex-none pr-6">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("tenant.storage.usageLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("tenant.storage.usageDescription") }}
            </p>
          </div>
          <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
            <div class="flex flex-1 items-center justify-end gap-3">
              <span class="text-foreground min-w-[50px] text-right text-sm font-medium">
                {{ getUsagePercentage() }}%
              </span>
              <!-- 形态与颜色沿用原 t-progress 的语义：超过 80% 转 warning -->
              <div
                class="h-1.5 max-w-[240px] flex-1 overflow-hidden rounded-full bg-[var(--td-bg-color-secondarycontainer)]"
              >
                <div
                  class="h-full rounded-full transition-all"
                  :class="getUsagePercentage() > 80 ? 'bg-warning' : 'bg-success'"
                  :style="{ width: `${getUsagePercentage()}%` }"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <aside v-if="showLeaveDangerZone" class="mt-1" :aria-label="$t('tenant.leaveDangerZone.title')">
        <div
          class="border-border bg-secondary box-border flex flex-row items-center justify-between gap-5 rounded-[10px] border px-[18px] py-4 max-[560px]:flex-col max-[560px]:items-stretch"
        >
          <div class="max-w-[min(65%,28rem)] min-w-0 flex-1 pr-2 max-[560px]:max-w-none max-[560px]:pr-0">
            <div class="text-foreground mb-1 text-[15px] leading-[1.4] font-medium">
              {{ $t("tenant.leaveDangerZone.title") }}
            </div>
            <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">{{ $t("tenant.leaveDangerZone.desc") }}</p>
          </div>
          <div class="shrink-0 max-[560px]:flex max-[560px]:justify-end">
            <Button
              variant="outline"
              class="border-destructive text-destructive hover:bg-destructive/10"
              @click="confirmLeaveTenant"
            >
              {{ $t("tenant.leaveDangerZone.button") }}
            </Button>
          </div>
        </div>
      </aside>

      <aside v-if="showDeleteDangerZone" class="mt-3" :aria-label="$t('tenant.deleteDangerZone.title')">
        <div
          class="border-border bg-secondary box-border flex flex-row items-center justify-between gap-5 rounded-[10px] border px-[18px] py-4 max-[560px]:flex-col max-[560px]:items-stretch"
        >
          <div class="max-w-[min(65%,28rem)] min-w-0 flex-1 pr-2 max-[560px]:max-w-none max-[560px]:pr-0">
            <div class="text-foreground mb-1 text-[15px] leading-[1.4] font-medium">
              {{ $t("tenant.deleteDangerZone.title") }}
            </div>
            <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">{{ $t("tenant.deleteDangerZone.desc") }}</p>
          </div>
          <div class="shrink-0 max-[560px]:flex max-[560px]:justify-end">
            <Button
              variant="destructive"
              class="bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive"
              @click="confirmDeleteTenant"
            >
              {{ $t("tenant.deleteDangerZone.button") }}
            </Button>
          </div>
        </div>
      </aside>
    </div>

    <!-- 退出空间确认 -->
    <Dialog v-model:open="leaveConfirmVisible">
      <DialogContent class="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{{ $t("tenantMember.leave.confirmTitle") }}</DialogTitle>
          <DialogDescription>{{ $t("tenantMember.leave.confirmBody") }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ $t("common.cancel") }}</Button>
          </DialogClose>
          <Button
            variant="destructive"
            class="bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive"
            :disabled="leavingTenant"
            @click="doLeaveTenant"
          >
            <Loader2Icon v-if="leavingTenant" class="animate-spin" />
            {{ $t("tenantMember.leave.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 删除空间确认 -->
    <Dialog v-model:open="deleteTenantVisible">
      <DialogContent
        class="sm:max-w-[480px]"
        :class="{ '[&>button]:hidden': deletingTenant }"
        @interact-outside="onDeleteInteractOutside"
      >
        <DialogHeader>
          <DialogTitle>{{ $t("tenant.deleteDangerZone.confirmTitle") }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <p class="text-foreground m-0 leading-[1.6]">
            {{ $t("tenant.deleteDangerZone.confirmBody", { name: tenantInfo?.name || "" }) }}
          </p>
          <p class="text-muted-foreground m-0 text-sm leading-normal">
            {{ $t("tenant.deleteDangerZone.confirmHint", { name: tenantInfo?.name || "" }) }}
          </p>
          <div class="relative">
            <Input
              v-model="deleteConfirmName"
              :placeholder="tenantInfo?.name || ''"
              :disabled="deletingTenant"
              class="pr-8"
            />
            <!-- Stands in for TDesign's \`clearable\`. -->
            <button
              v-if="deleteConfirmName && !deletingTenant"
              type="button"
              data-slot="input-clear"
              class="text-placeholder hover:text-foreground absolute top-1/2 right-2 flex -translate-y-1/2 items-center"
              :aria-label="$t('common.clear')"
              @click="deleteConfirmName = ''"
            >
              <XIcon class="size-3.5" />
            </button>
          </div>
        </div>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline" :disabled="deletingTenant">{{ $t("common.cancel") }}</Button>
          </DialogClose>
          <Button
            variant="destructive"
            class="bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive"
            :disabled="deleteConfirmName.trim() !== (tenantInfo?.name || '') || deletingTenant"
            @click="deleteCurrentTenant"
          >
            <Loader2Icon v-if="deletingTenant" class="animate-spin" />
            {{ $t("tenant.deleteDangerZone.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { getCurrentUser, type TenantInfo } from "@/api/auth";
import { deleteTenant as deleteTenantApi, updateTenant as updateTenantApi } from "@/api/tenant";
import { leaveTenant, fetchAllTenantMembers, type TenantMember, type TenantRole } from "@/api/tenant/members";
import { useAuthStore } from "@/stores/auth";
import { useI18n } from "vue-i18n";
import { useRoleLabel } from "@/composables/useRoleLabel";
import {
  navigateAfterTenantSwitch,
  persistLastActiveTenantPreference,
  stashTenantSwitchToast,
} from "@/utils/tenantSwitch";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CircleAlertIcon, Loader2Icon, PencilIcon, XIcon } from "@lucide/vue";

const { t, locale } = useI18n();
const { formatRole } = useRoleLabel();
const authStore = useAuthStore();

// Reactive state
const tenantInfo = ref<TenantInfo | null>(null);
const loading = ref(true);
const error = ref("");

// 仅 owner 可改空间名（与后端 router.go 中 g.Owner() 守卫一致；
// 服务端始终是权限的最终裁判，这里只决定 UI 是否露出入口）。
const canEditTenant = computed(() => authStore.hasRole("owner"));

/** 与原 TenantMembers.vue 一致：最后一位 Owner 不展示退出，避免与服务端 last-owner 对齐失败。 */
const activeTenantNumericId = computed(() => Number(authStore.currentTenantId ?? 0));

const leaveMembersSnap = ref<TenantMember[]>([]);
const leaveGateReady = ref(false);
const leaveGateLoading = ref(false);

const currentTenantRole = computed<TenantRole | "">(() => (authStore.currentTenantRole || "") as TenantRole | "");

const canLeaveSpace = computed(() => {
  const r = currentTenantRole.value;
  if (!r || !tenantInfo.value?.id) return false;
  if (r !== "owner") return true;
  return leaveMembersSnap.value.filter((m) => m.role === "owner").length > 1;
});

/** 在主内容已成功加载、`listMembers` 放行规则就绪且允许退出时出现。 */
const showLeaveDangerZone = computed(() => {
  if (loading.value || error.value || !tenantInfo.value) return false;
  if (!leaveGateReady.value || leaveGateLoading.value) return false;
  if (!currentTenantRole.value) return false;
  if (Number(tenantInfo.value.id) !== activeTenantNumericId.value) return false;
  return canLeaveSpace.value;
});

const showDeleteDangerZone = computed(() => {
  if (loading.value || error.value || !tenantInfo.value) return false;
  if (Number(tenantInfo.value.id) !== activeTenantNumericId.value) return false;
  return authStore.hasRole("owner");
});

async function evaluateLeaveGate(): Promise<void> {
  leaveGateReady.value = false;
  leaveMembersSnap.value = [];
  leaveGateLoading.value = false;

  const infoId = tenantInfo.value?.id != null ? Number(tenantInfo.value.id) : 0;
  if (!infoId || !activeTenantNumericId.value || infoId !== activeTenantNumericId.value) {
    leaveGateReady.value = true;
    return;
  }

  const role = currentTenantRole.value;
  if (!role) {
    leaveGateReady.value = true;
    return;
  }
  if (role !== "owner") {
    leaveGateReady.value = true;
    return;
  }

  leaveGateLoading.value = true;
  try {
    leaveMembersSnap.value = await fetchAllTenantMembers(infoId);
  } finally {
    leaveGateLoading.value = false;
    leaveGateReady.value = true;
  }
}

const leaveConfirmVisible = ref(false);
const leavingTenant = ref(false);

function confirmLeaveTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0);
  if (!tid) return;
  leaveConfirmVisible.value = true;
}

async function doLeaveTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0);
  if (!tid || leavingTenant.value) return;
  leavingTenant.value = true;
  try {
    const resp = await leaveTenant(tid);
    if (resp.success) {
      MessagePlugin.success(t("tenantMember.leave.success"));
      authStore.logout();
      window.location.href = "/login";
    } else {
      MessagePlugin.error(resp.message || t("tenantMember.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 409) {
      MessagePlugin.error(t("tenantMember.errors.lastOwner"));
    } else {
      MessagePlugin.error(err?.message || t("tenantMember.errors.generic"));
    }
  } finally {
    leavingTenant.value = false;
    leaveConfirmVisible.value = false;
  }
}

function confirmDeleteTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0);
  const tenantName = tenantInfo.value?.name || "";
  if (!tid || !tenantName) return;
  deleteConfirmName.value = "";
  deleteTenantVisible.value = true;
}

// 删除进行中屏蔽遮罩点击关闭，与原 close-on-overlay-click=false 一致。
function onDeleteInteractOutside(e: Event) {
  if (deletingTenant.value) e.preventDefault();
}

async function deleteCurrentTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0);
  const tenantName = tenantInfo.value?.name || "";
  if (!tid || !tenantName) return;
  if (deleteConfirmName.value.trim() !== tenantName) {
    MessagePlugin.warning(t("tenant.deleteDangerZone.nameMismatch"));
    return;
  }
  try {
    deletingTenant.value = true;
    const resp = await deleteTenantApi(tid);
    if (resp.success) {
      MessagePlugin.success(t("tenant.deleteDangerZone.success"));
      authStore.setMemberships((authStore.memberships ?? []).filter((m) => m.tenant_id !== tid));
      await authStore.refreshFromAuthMe();
      // Memberships are ordered by join time, so the first remaining one is
      // the workspace the server itself would resolve for a session that no
      // longer names a workspace; moving there (and persisting it) keeps the
      // client and the next login in agreement.
      const next = authStore.memberships[0];
      if (next) {
        const name = next.tenant_name?.trim() || `#${next.tenant_id}`;
        authStore.setSelectedTenant(next.tenant_id, name);
        stashTenantSwitchToast({
          name,
          role: formatRole(next.role) || undefined,
          roleEnum: next.role || undefined,
        });
        const persist = persistLastActiveTenantPreference(next.tenant_id);
        await Promise.race([persist, new Promise((r) => setTimeout(r, 400))]);
        navigateAfterTenantSwitch();
        return;
      }
      authStore.logout();
      window.location.href = "/login";
    } else {
      MessagePlugin.error(resp.message || t("tenant.deleteDangerZone.failed"));
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("tenant.deleteDangerZone.failed"));
  } finally {
    deletingTenant.value = false;
    deleteConfirmName.value = "";
    deleteTenantVisible.value = false;
  }
}

watch([() => tenantInfo.value?.id, () => authStore.currentTenantId, () => authStore.currentTenantRole], () => {
  if (!loading.value && tenantInfo.value && !error.value) {
    void evaluateLeaveGate();
  }
});

// 原地编辑空间名称：editing 控制行内只读 / 编辑两种形态切换。
// 不沿用 dialog 是因为这里只有一个字段，弹窗反而打断了配置浏览节奏。
const editing = ref(false);
const editName = ref("");
const saving = ref(false);
const deleteConfirmName = ref("");
const deleteTenantVisible = ref(false);
const deletingTenant = ref(false);
const editNameTrimmed = computed(() => editName.value.trim());
// 保存按钮可点条件：非空、改了内容、不在保存中。
// 后端 name 字段没有 uniqueIndex 也没有重名校验，所以这里不做"是否已存在"的判断；
// 后端 service 也只在 create 时拒空，update 时不校验，保持前端兜底非空即可。
const canSubmit = computed(
  () => !saving.value && !!editNameTrimmed.value && editNameTrimmed.value !== tenantInfo.value?.name,
);

// TDesign's \`autofocus\` prop focused the field itself; the native attribute
// does nothing on an element inserted after page load, so the editors are
// focused by hand once they have rendered.
const nameEditRow = ref<HTMLElement | null>(null);
const descriptionEditRow = ref<HTMLElement | null>(null);

const startEditName = () => {
  editName.value = tenantInfo.value?.name || "";
  editing.value = true;
  void nextTick(() => nameEditRow.value?.querySelector("input")?.focus());
};

const cancelEditName = () => {
  if (saving.value) return;
  editing.value = false;
  editName.value = "";
};

// 输入框自身不冒泡 esc，这里手动处理（与 enter 的体验对称）。
const onEditKeydown = (e: KeyboardEvent) => {
  if (e?.key === "Escape") {
    cancelEditName();
  }
};

// 原地编辑空间描述：与名称对称的 editing / editValue / saving 三态。
// 描述允许为空（业务上是可选字段），所以可提交条件不要求非空，只要内容变了即可。
const editingDescription = ref(false);
const editDescription = ref("");
const savingDescription = ref(false);
const editDescriptionTrimmed = computed(() => editDescription.value.trim());
const canSubmitDescription = computed(
  () => !savingDescription.value && editDescriptionTrimmed.value !== (tenantInfo.value?.description || ""),
);

const startEditDescription = () => {
  editDescription.value = tenantInfo.value?.description || "";
  editingDescription.value = true;
  void nextTick(() => descriptionEditRow.value?.querySelector("textarea")?.focus());
};

const cancelEditDescription = () => {
  if (savingDescription.value) return;
  editingDescription.value = false;
  editDescription.value = "";
};

// textarea 上 Enter 默认走换行，提交走 Ctrl/⌘+Enter；Esc 取消。
const onEditDescriptionKeydown = (e: KeyboardEvent) => {
  if (!e) return;
  if (e.key === "Escape") {
    cancelEditDescription();
    return;
  }
  if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
    e.preventDefault();
    void saveTenantDescription();
  }
};

const saveTenantDescription = async () => {
  if (!tenantInfo.value?.id) return;
  const newDesc = editDescriptionTrimmed.value;
  if (newDesc === (tenantInfo.value.description || "")) {
    editingDescription.value = false;
    return;
  }

  try {
    savingDescription.value = true;
    const resp = await updateTenantApi(Number(tenantInfo.value.id), { description: newDesc });
    if (resp.success) {
      // 本地立即回显，避免等 /auth/me 往返。描述不像名称那样会出现在空间切换器等
      // 顶部组件里，所以无需同步 authStore.tenant / memberships。
      if (tenantInfo.value) {
        tenantInfo.value = { ...tenantInfo.value, description: newDesc };
      }
      MessagePlugin.success(t("tenant.details.editDescriptionSuccess"));
      editingDescription.value = false;
    } else {
      MessagePlugin.error(resp.message || t("tenant.details.editDescriptionFailed"));
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("tenant.details.editDescriptionFailed"));
  } finally {
    savingDescription.value = false;
  }
};

const saveTenantName = async () => {
  const newName = editNameTrimmed.value;
  if (!newName) {
    MessagePlugin.warning(t("tenant.details.editNameRequired"));
    return;
  }
  if (!tenantInfo.value?.id) return;
  if (newName === tenantInfo.value.name) {
    editing.value = false;
    return;
  }

  try {
    saving.value = true;
    const resp = await updateTenantApi(Number(tenantInfo.value.id), { name: newName });
    if (resp.success) {
      // 本地立即回显，避免等 /auth/me 往返；同步刷新登录态里的 tenant
      // 缓存（若当前激活空间就是 home tenant，顶部空间切换器等地方也跟着更新）。
      if (tenantInfo.value) {
        tenantInfo.value = { ...tenantInfo.value, name: newName };
      }
      if (authStore.tenant && String(authStore.tenant.id) === String(tenantInfo.value?.id)) {
        authStore.setTenant({ ...authStore.tenant, name: newName });
      }
      // memberships 里的 tenant_name 是空间切换器读的字段，一并同步避免显示旧名字。
      if (authStore.memberships?.length) {
        const next = authStore.memberships.map((m) =>
          String(m.tenant_id) === String(tenantInfo.value?.id) ? { ...m, tenant_name: newName } : m,
        );
        authStore.setMemberships(next);
      }
      MessagePlugin.success(t("tenant.details.editNameSuccess"));
      editing.value = false;
    } else {
      MessagePlugin.error(resp.message || t("tenant.details.editNameFailed"));
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("tenant.details.editNameFailed"));
  } finally {
    saving.value = false;
  }
};

// Methods
const loadInfo = async () => {
  try {
    loading.value = true;
    error.value = "";

    const userResponse = await getCurrentUser();

    const data = userResponse?.data as { tenant?: TenantInfo } | undefined;
    if ((userResponse as any).success && data?.tenant) {
      tenantInfo.value = data.tenant;
    } else {
      error.value = userResponse.message || t("tenant.messages.fetchFailed");
    }
  } catch (err: any) {
    error.value = err?.message || t("tenant.messages.networkError");
  } finally {
    loading.value = false;
  }
  // 须在 loading=false 之后再评估：否则退出入口会被 showLeaveDangerZone 里的 loading 条件挡住，
  // 且部分环境下角色 hydrated 稍晚于 /auth/me 返回。
  if (tenantInfo.value && !error.value) {
    await evaluateLeaveGate();
  }
};

const getStatusText = (status: string | undefined) => {
  switch (status) {
    case "active":
      return t("tenant.statusActive");
    case "inactive":
      return t("tenant.statusInactive");
    case "suspended":
      return t("tenant.statusSuspended");
    default:
      return t("tenant.statusUnknown");
  }
};

// The old light-variant tag: a pale tint of the status colour behind text in
// that colour; unknown statuses keep the neutral grey.
const getStatusClass = (status: string | undefined) => {
  switch (status) {
    case "active":
      return "bg-success/10 text-success";
    case "inactive":
      return "bg-warning/10 text-warning";
    case "suspended":
      return "bg-destructive/10 text-destructive";
    default:
      return "";
  }
};

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return t("tenant.unknown");

  try {
    const date = new Date(dateStr);
    const formatter = new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
    return formatter.format(date);
  } catch {
    return t("tenant.formatError");
  }
};

const formatBytes = (bytes: number) => {
  if (bytes === 0) return "0 B";

  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

const getUsagePercentage = () => {
  if (!tenantInfo.value?.storage_quota || tenantInfo.value.storage_quota === 0) {
    return 0;
  }

  const used = tenantInfo.value.storage_used || 0;
  const percentage = (used / tenantInfo.value.storage_quota) * 100;
  return Math.min(Math.round(percentage * 100) / 100, 100);
};

// Lifecycle
onMounted(() => {
  loadInfo();
});
</script>
