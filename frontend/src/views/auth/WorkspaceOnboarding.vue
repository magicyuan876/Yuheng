<template>
  <main
    class="bg-background grid min-h-screen place-items-center [background-image:radial-gradient(circle_at_20%_10%,color-mix(in_srgb,var(--td-brand-color)_12%,transparent),transparent_38%)] px-5 py-8"
  >
    <section
      class="border-border bg-card w-[min(520px,100%)] rounded-[20px] border p-11 text-center shadow-[var(--td-shadow-2)] max-[560px]:p-[32px_22px]"
    >
      <div
        class="text-primary mx-auto mb-[22px] grid size-16 place-items-center rounded-[18px] bg-[var(--td-brand-color-light)]"
        aria-hidden="true"
      >
        <SigmaIcon class="size-[30px]" />
      </div>
      <h1 v-if="authStore.canCreateTenant" class="text-foreground m-0 text-[26px] leading-[1.3] font-bold">
        {{ $t("auth.workspaceOnboarding.title") }}
      </h1>
      <h1 v-else class="text-foreground m-0 text-[26px] leading-[1.3] font-bold">
        {{ $t("auth.workspaceOnboarding.inviteOnlyTitle") }}
      </h1>
      <p v-if="authStore.canCreateTenant" class="text-muted-foreground mx-0 mt-3.5 mb-7 leading-[1.7]">
        {{ $t("auth.workspaceOnboarding.description") }}
      </p>
      <p v-else class="text-muted-foreground mx-0 mt-3.5 mb-7 leading-[1.7]">
        {{ $t("auth.workspaceOnboarding.inviteOnlyDescription") }}
      </p>

      <div
        v-if="policyLoading"
        class="text-muted-foreground mb-[18px] flex min-h-[52px] items-center justify-center gap-2.5 text-sm"
      >
        <Loader2Icon class="size-4 animate-spin" />
        <span>{{ $t("auth.workspaceOnboarding.loadingPolicy") }}</span>
      </div>
      <div
        v-else-if="policyLoadFailed"
        class="text-destructive mb-[18px] flex min-h-[52px] flex-wrap items-center justify-center gap-2.5 rounded-[10px] bg-[var(--td-error-color-light)] px-4 py-3 text-sm"
        role="alert"
      >
        <CircleAlertIcon class="size-5 shrink-0" aria-hidden="true" />
        <span>{{ $t("auth.workspaceOnboarding.policyLoadFailed") }}</span>
        <Button variant="ghost" size="sm" class="text-foreground hover:text-foreground" @click="loadPolicy">
          {{ $t("auth.workspaceOnboarding.retry") }}
        </Button>
      </div>

      <template v-else>
        <div
          v-if="!authStore.canCreateTenant"
          class="border-border bg-secondary text-foreground mb-[18px] flex min-h-[52px] items-center justify-center gap-2.5 rounded-[10px] border px-4 py-3 text-sm leading-[1.5]"
        >
          <LockIcon class="text-muted-foreground size-5 shrink-0" aria-hidden="true" />
          <span>{{ $t("auth.workspaceOnboarding.inviteOnlyNotice") }}</span>
        </div>

        <div
          class="grid gap-3 max-[560px]:grid-cols-1"
          :class="authStore.canCreateTenant ? 'grid-cols-2' : 'grid-cols-[minmax(220px,1fr)]'"
        >
          <Button v-if="authStore.canCreateTenant" size="lg" class="h-10" @click="createVisible = true">
            <PlusIcon />
            {{ $t("auth.workspaceOnboarding.create") }}
          </Button>
          <Button
            :variant="authStore.canCreateTenant ? 'outline' : 'default'"
            size="lg"
            class="h-10"
            @click="invitationsVisible = true"
          >
            <MailIcon />
            {{ $t("auth.workspaceOnboarding.invitations") }}
            <template v-if="authStore.pendingInvitationCount > 0"> ({{ authStore.pendingInvitationCount }}) </template>
          </Button>
        </div>
      </template>

      <p
        v-if="!policyLoading && !policyLoadFailed && authStore.canCreateTenant"
        class="text-muted-foreground mx-0 mt-6 mb-2 text-[13px] leading-[1.7]"
      >
        {{ $t("auth.workspaceOnboarding.help") }}
      </p>
      <p
        v-else-if="!policyLoading && !policyLoadFailed"
        class="text-muted-foreground mx-0 mt-6 mb-2 text-[13px] leading-[1.7]"
      >
        {{ $t("auth.workspaceOnboarding.inviteOnlyHelp") }}
      </p>
      <button
        data-slot="logout-link"
        class="text-muted-foreground hover:text-primary cursor-pointer border-0 bg-transparent px-2.5 py-1.5"
        type="button"
        @click="handleLogout"
      >
        {{ $t("auth.logout") }}
      </button>
    </section>

    <CreateTenantDialog v-model:visible="createVisible" @created="onTenantCreated" />
    <MyInvitationsDialog v-model:visible="invitationsVisible" />
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import CreateTenantDialog from "@/components/CreateTenantDialog.vue";
import MyInvitationsDialog from "@/components/MyInvitationsDialog.vue";
import { logout as logoutApi } from "@/api/auth";
import type { TenantInfo } from "@/api/tenant";
import { useAuthStore } from "@/stores/auth";

import { Button } from "@/components/ui/button";
import { CircleAlertIcon, Loader2Icon, LockIcon, MailIcon, PlusIcon, SigmaIcon } from "@lucide/vue";

const router = useRouter();
const authStore = useAuthStore();
const createVisible = ref(false);
const invitationsVisible = ref(false);
const policyLoading = ref(true);
const policyLoadFailed = ref(false);

async function loadPolicy() {
  policyLoading.value = true;
  policyLoadFailed.value = false;
  try {
    const refreshed = await authStore.refreshFromAuthMe();
    if (!refreshed) {
      policyLoadFailed.value = true;
      return;
    }
    await authStore.fetchPendingInvitationCount();
  } finally {
    policyLoading.value = false;
  }
}

onMounted(async () => {
  await loadPolicy();
});

watch(
  () => authStore.hasValidTenant,
  (ready) => {
    if (ready) router.replace("/platform/knowledge-bases");
  },
);

async function onTenantCreated(tenant: TenantInfo) {
  await authStore.refreshFromAuthMe();
  authStore.setSelectedTenant(tenant.id, tenant.name);
  await router.replace("/platform/knowledge-bases");
}

async function handleLogout() {
  await logoutApi();
  authStore.logout();
  await router.replace("/login");
}
</script>
