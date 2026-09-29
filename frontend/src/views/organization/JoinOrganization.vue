<template>
  <div class="bg-card flex min-h-full items-center justify-center p-5">
    <div class="bg-card w-full max-w-[400px] rounded-[16px] p-12 text-center shadow-[0_8px_32px_rgba(0,0,0,0.08)]">
      <div
        class="text-success mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-[var(--td-success-color-light)]"
      >
        <UserPlusIcon class="size-12" />
      </div>
      <h2 class="text-foreground mt-0 mb-4 text-xl font-semibold">{{ $t("organization.join.title") }}</h2>
      <p v-if="loading" class="text-muted-foreground mt-0 mb-6 text-sm">{{ $t("organization.join.joining") }}</p>
      <p v-else-if="error" class="text-destructive mt-0 mb-6 text-sm">{{ error }}</p>
      <p v-else class="text-success mt-0 mb-6 text-sm">{{ $t("organization.join.success") }}</p>

      <Button v-if="!loading" @click="goToOrganizations">
        {{ $t("organization.join.goToOrganizations") }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { useOrganizationStore } from "@/stores/organization";
import { useAuthStore } from "@/stores/auth";

import { Button } from "@/components/ui/button";
import { UserPlusIcon } from "@lucide/vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const orgStore = useOrganizationStore();
const authStore = useAuthStore();

const loading = ref(true);
const error = ref("");

onMounted(async () => {
  const code = route.query.code as string;

  if (!code) {
    error.value = t("organization.join.noCode");
    loading.value = false;
    return;
  }

  // 后端 POST /organizations/join 要求当前空间角色 ≥ admin，先在前端拦截以给出友好提示
  if (!authStore.hasRole("admin") && !authStore.canAccessAllTenants) {
    error.value = t("organization.rbac.cannotJoin");
    loading.value = false;
    return;
  }

  try {
    const result = await orgStore.join(code);
    if (result) {
      MessagePlugin.success(t("organization.join.success"));
    } else {
      error.value = orgStore.error || t("organization.join.failed");
    }
  } catch (e: any) {
    error.value = e?.message || t("organization.join.failed");
  } finally {
    loading.value = false;
  }
});

const goToOrganizations = () => {
  router.push("/platform/organizations");
};
</script>
