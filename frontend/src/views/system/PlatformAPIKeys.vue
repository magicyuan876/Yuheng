<template>
  <div class="w-full">
    <header class="mb-5">
      <h2 class="text-foreground m-0 mb-2 text-xl font-semibold">{{ t("platformApiKeys.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.5]">{{ t("platformApiKeys.description") }}</p>
    </header>

    <Alert class="mb-5 border-transparent bg-[var(--td-warning-color-light)]">
      <TriangleAlertIcon class="text-warning" />
      <AlertTitle class="text-foreground font-normal">{{ t("platformApiKeys.securityNotice") }}</AlertTitle>
      <AlertAction>
        <Button size="sm" variant="outline" @click="openCreate">
          <PlusIcon />
          {{ t("platformApiKeys.create") }}
        </Button>
      </AlertAction>
    </Alert>

    <section class="border-border bg-card overflow-hidden rounded-[10px] border">
      <div
        v-if="loading"
        class="text-muted-foreground flex min-h-[120px] items-center justify-center gap-2 text-[13px]"
      >
        <Loader2Icon class="size-4 animate-spin" />
        <span>{{ t("platformApiKeys.loading") }}</span>
      </div>
      <div
        v-else-if="keys.length === 0"
        class="text-muted-foreground flex min-h-[120px] flex-col items-center justify-center gap-3 text-[13px]"
      >
        <span>{{ t("platformApiKeys.empty") }}</span>
        <Button size="sm" variant="outline" @click="openCreate">
          <PlusIcon />
          {{ t("platformApiKeys.create") }}
        </Button>
      </div>
      <div v-else class="w-full overflow-x-auto">
        <Table class="w-full table-fixed">
          <TableHeader>
            <TableRow class="hover:bg-transparent">
              <TableHead class="bg-secondary text-placeholder w-[11%] p-[13px_14px] text-xs font-medium">
                {{ t("platformApiKeys.name") }}
              </TableHead>
              <TableHead class="bg-secondary text-placeholder w-[17%] p-[13px_14px] text-xs font-medium">
                {{ t("platformApiKeys.key") }}
              </TableHead>
              <TableHead class="bg-secondary text-placeholder p-[13px_14px] text-xs font-medium">
                {{ t("platformApiKeys.capability") }}
              </TableHead>
              <TableHead class="bg-secondary text-placeholder w-20 p-[13px_14px] text-xs font-medium">
                {{ t("platformApiKeys.lastUsed") }}
              </TableHead>
              <TableHead class="bg-secondary text-placeholder w-32 p-[13px_14px] text-xs font-medium">
                {{ t("platformApiKeys.createdAt") }}
              </TableHead>
              <TableHead class="bg-secondary text-placeholder w-[52px] p-[13px_14px] text-right text-xs font-medium">
                {{ t("platformApiKeys.actions") }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="key in keys" :key="key.id" class="last:border-b-0 hover:bg-transparent">
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <span
                  class="text-foreground block min-w-0 overflow-hidden text-[13px] font-semibold text-ellipsis whitespace-nowrap"
                >
                  {{ key.name }}
                </span>
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <code
                  class="inline-block max-w-full overflow-hidden align-top font-mono text-xs leading-[1.5] text-ellipsis whitespace-nowrap"
                >
                  {{ maskApiKey(key.api_key) }}
                </code>
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <ApiKeyCapabilityChips :capabilities="key.capabilities" :groups="PLATFORM_API_KEY_CAPABILITY_GROUPS" />
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <span class="block min-w-0 text-xs leading-[1.4] whitespace-nowrap">{{
                  formatDate(key.last_used_at)
                }}</span>
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <time class="block font-mono text-xs leading-[1.4] whitespace-nowrap" :datetime="key.created_at">
                  {{ formatDate(key.created_at) }}
                </time>
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <div class="flex items-center justify-end">
                  <ApiKeyRevokeButton
                    :label="t('common.delete')"
                    :message="t('platformApiKeys.deleteConfirm', { name: key.name })"
                    @confirm="deleteKey(key)"
                  />
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </section>

    <SettingDrawer
      :visible="drawerVisible"
      :title="t('platformApiKeys.create')"
      :description="t('platformApiKeys.createDescription')"
      :icon="ShieldCheckIcon"
      width="560px"
      :min-width="480"
      :max-width="920"
      storage-key="setting-drawer:width:platform-api-key-create"
      :close-on-overlay-click="false"
      :confirm-text="t('platformApiKeys.create')"
      :confirm-loading="creating"
      @update:visible="drawerVisible = $event"
      @confirm="createKey"
    >
      <div class="border-border flex flex-col border-b">
        <div class="border-border flex flex-col gap-2 border-b pt-0 pb-4">
          <div>
            <label
              class="text-foreground before:bg-primary flex items-center gap-2 text-sm leading-[1.45] font-semibold before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
            >
              {{ t("platformApiKeys.name") }}
            </label>
          </div>
          <Input
            v-model="name"
            :placeholder="t('platformApiKeys.namePlaceholder')"
            class="bg-secondary hover:border-border hover:bg-card focus-visible:border-border focus-visible:bg-card rounded border-transparent focus-visible:ring-0"
          />
        </div>

        <div class="border-border flex flex-col gap-2 pt-[14px] pb-4">
          <div>
            <label
              class="text-foreground before:bg-primary flex items-center gap-2 text-sm leading-[1.45] font-semibold before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
            >
              {{ t("platformApiKeys.capability") }}
            </label>
          </div>
          <p class="text-placeholder m-0 text-xs leading-[18px]">{{ t("platformApiKeys.capabilityHint") }}</p>
          <ApiKeyCapabilityPicker
            v-model="selectedCapabilities"
            :groups="PLATFORM_API_KEY_CAPABILITY_GROUPS"
            id-prefix="platform-capability"
          />
        </div>
      </div>
    </SettingDrawer>

    <ApiKeySecretDialog v-model:token="createdToken" :title="t('platformApiKeys.createdTitle')" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import ApiKeyCapabilityChips from "@/components/api-keys/ApiKeyCapabilityChips.vue";
import ApiKeyCapabilityPicker from "@/components/api-keys/ApiKeyCapabilityPicker.vue";
import ApiKeyRevokeButton from "@/components/api-keys/ApiKeyRevokeButton.vue";
import ApiKeySecretDialog from "@/components/api-keys/ApiKeySecretDialog.vue";
import { formatApiKeyDateTime, maskApiKey } from "@/components/api-keys/apiKeyDisplay";
import type { TenantAPIKey, TenantAPIKeyCapability } from "@/api/tenant";
import { createPlatformAPIKey, deletePlatformAPIKey, listPlatformAPIKeys } from "@/api/system";
import { PLATFORM_API_KEY_CAPABILITY_GROUPS } from "@/config/apiKeyCapabilities";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Loader2Icon, PlusIcon, ShieldCheckIcon, TriangleAlertIcon } from "@lucide/vue";

const { t } = useI18n();
const keys = ref<TenantAPIKey[]>([]);
const loading = ref(false);
const creating = ref(false);
const drawerVisible = ref(false);
const createdToken = ref("");
const name = ref("");
const selectedCapabilities = ref<TenantAPIKeyCapability[]>([]);

function formatDate(value?: string) {
  return formatApiKeyDateTime(value) || t("apiKeys.neverUsed");
}

function openCreate() {
  name.value = "";
  selectedCapabilities.value = [];
  drawerVisible.value = true;
}

async function reload() {
  loading.value = true;
  try {
    const response = await listPlatformAPIKeys();
    keys.value = response.data ?? [];
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformApiKeys.loadFailed"));
  } finally {
    loading.value = false;
  }
}

async function createKey() {
  if (!name.value.trim()) {
    MessagePlugin.warning(t("platformApiKeys.nameRequired"));
    return;
  }
  if (selectedCapabilities.value.length === 0) {
    MessagePlugin.warning(t("platformApiKeys.capabilityRequired"));
    return;
  }
  creating.value = true;
  try {
    const response = await createPlatformAPIKey({
      name: name.value.trim(),
      capabilities: selectedCapabilities.value,
    });
    createdToken.value = response.data?.token ?? "";
    drawerVisible.value = false;
    await reload();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformApiKeys.createFailed"));
  } finally {
    creating.value = false;
  }
}

async function deleteKey(key: TenantAPIKey) {
  try {
    await deletePlatformAPIKey(key.id);
    MessagePlugin.success(t("platformApiKeys.deleteSuccess"));
    await reload();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformApiKeys.deleteFailed"));
  }
}

onMounted(reload);
</script>
