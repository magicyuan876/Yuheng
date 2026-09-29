<template>
  <!-- 全局右上角"待处理邀请"铃铛。
       - 与原先 UserMenu 中铃铛的逻辑一致：只在 pendingInvitationCount > 0 时渲染，
         空收件箱场景不占用角落像素。
       - 固定定位、z-index 远低于抽屉的 2501，业务页面右侧抽屉（FAQ、KB 调试、
         Tenant 审计、SettingDrawer 等）弹出时会自然覆盖铃铛，不需要特意联动隐藏。
       - 点击铃铛复用同一份 MyInvitationsDialog，行为与之前一致。 -->
  <template v-if="pendingInvitationCount > 0">
    <!-- z-100 sits far below the drawers (2501), so a page's side drawer covers the bell, and above
         ordinary page content (0-10), so list cards never do. The container background, not
         transparency, keeps the bell legible over any page colour. -->
    <button
      type="button"
      data-slot="invitation-bell"
      class="bg-card text-muted-foreground hover:bg-secondary hover:text-primary focus-visible:outline-ring fixed top-3 right-4 z-100 inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-[10px] border border-[var(--td-component-stroke)] p-0 shadow-[0_2px_6px_rgba(0,0,0,0.04)] transition-[background-color,color,box-shadow] duration-[180ms] hover:shadow-[0_4px_12px_rgba(0,0,0,0.08)] focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-solid"
      :title="$t('tenantInvitation.inboxTooltip')"
      @click="openDialog"
    >
      <BellIcon class="size-[18px]" />
      <!-- Mirrors the old t-badge (offset [6, 4]): a 20px pill centred on a point 6px in from the
           right edge and 4px down from the top. -->
      <span
        class="bg-destructive absolute top-1 right-1.5 box-content h-5 min-w-2 translate-x-1/2 -translate-y-1/2 rounded-[10px] px-1.5 text-center text-xs leading-5 text-[var(--td-text-color-anti)]"
      >
        {{ pendingInvitationCount > 99 ? "99+" : pendingInvitationCount }}
      </span>
    </button>
  </template>
  <MyInvitationsDialog v-model:visible="dialogVisible" />
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

import { BellIcon } from "@lucide/vue";

import { useAuthStore } from "@/stores/auth";
import MyInvitationsDialog from "@/components/MyInvitationsDialog.vue";

const authStore = useAuthStore();

const pendingInvitationCount = computed(() => authStore.pendingInvitationCount);

const dialogVisible = ref(false);
const openDialog = () => {
  dialogVisible.value = true;
};
</script>
