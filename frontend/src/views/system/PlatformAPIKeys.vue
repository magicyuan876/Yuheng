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
                  {{ key.api_key }}
                </code>
              </TableCell>
              <TableCell class="text-muted-foreground p-[13px_14px] align-middle text-[13px] leading-[1.45]">
                <div class="flex flex-wrap items-center gap-[5px]">
                  <span
                    v-for="chip in visibleCapabilityChips(key)"
                    :key="chip.id"
                    class="bg-success/10 text-success inline-flex h-[22px] items-center rounded-md px-2 text-xs font-medium whitespace-nowrap"
                  >
                    {{ chip.label }}
                  </span>
                  <Popover v-if="hiddenCapabilityCount(key) > 0">
                    <PopoverTrigger as-child>
                      <button
                        type="button"
                        class="border-success/35 text-success inline-flex h-[22px] cursor-pointer items-center rounded-md border border-dashed bg-transparent px-2 text-xs font-medium"
                        :aria-label="t('platformApiKeys.viewAllCapabilities')"
                      >
                        {{ t("platformApiKeys.capabilityMore", { count: hiddenCapabilityCount(key) }) }}
                      </button>
                    </PopoverTrigger>
                    <PopoverContent
                      align="start"
                      class="max-h-[360px] w-[320px] max-w-[min(360px,88vw)] overflow-auto px-3.5 py-3"
                    >
                      <div class="text-foreground mb-2.5 text-[13px] leading-[1.4] font-semibold">
                        {{ t("platformApiKeys.capability") }}
                      </div>
                      <div
                        v-for="group in capabilityGroupsForKey(key)"
                        :key="group.key"
                        class="border-border mt-2.5 border-t border-dashed pt-2.5 first:mt-0 first:border-t-0 first:pt-0"
                      >
                        <div class="text-foreground mb-1.5 text-xs leading-[1.4] font-semibold">{{ group.label }}</div>
                        <div class="flex flex-wrap gap-[5px]">
                          <span
                            v-for="label in group.labels"
                            :key="label"
                            class="bg-success/10 text-success inline-flex h-[22px] items-center rounded-md px-2 text-xs font-medium whitespace-nowrap"
                          >
                            {{ label }}
                          </span>
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
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
                  <Popover v-model:open="deleteConfirmOpen[key.id]">
                    <PopoverTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        class="text-destructive hover:text-destructive"
                        :title="t('common.delete')"
                        :aria-label="t('common.delete')"
                        @click.stop
                      >
                        <Trash2Icon />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent align="end" class="w-64">
                      <p class="text-foreground mb-3 text-[13px] leading-[1.5]">
                        {{ t("platformApiKeys.deleteConfirm", { name: key.name }) }}
                      </p>
                      <div class="flex justify-end gap-2">
                        <Button size="sm" variant="outline" @click="deleteConfirmOpen[key.id] = false">
                          {{ t("common.cancel") }}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          class="bg-destructive text-primary-foreground hover:bg-destructive/80"
                          @click="confirmDelete(key)"
                        >
                          {{ t("common.delete") }}
                        </Button>
                      </div>
                    </PopoverContent>
                  </Popover>
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
            v-model="form.name"
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
          <div class="flex flex-col gap-3">
            <div
              v-for="group in PLATFORM_API_KEY_CAPABILITY_GROUPS"
              :key="group.key"
              class="border-border flex flex-col gap-2 border-t pt-2.5 pb-0.5 first:border-t-0 first:pt-[10px]"
            >
              <div class="text-foreground flex min-h-6 items-center justify-between gap-3 text-[13px] font-semibold">
                <span>{{ t(group.labelKey) }}</span>
                <Button variant="ghost" size="sm" @click="toggleGroup(group.capabilities.map((item) => item.value))">
                  {{
                    groupSelected(group.capabilities.map((item) => item.value))
                      ? t("integrations.api.apiKeyCapabilityClearGroup")
                      : t("integrations.api.apiKeyCapabilitySelectGroup")
                  }}
                </Button>
              </div>
              <div class="flex flex-col gap-2.5">
                <div v-for="item in group.capabilities" :key="item.value" class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <Checkbox :id="`capability-${item.value}`" v-model="selected[item.value]" />
                    <Label :for="`capability-${item.value}`" class="cursor-pointer text-[13px] font-normal">
                      {{ t(item.labelKey) }}
                    </Label>
                  </div>
                  <p class="text-placeholder m-0 mt-0.5 ml-6 text-xs leading-[18px]">{{ t(item.hintKey) }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </SettingDrawer>

    <Dialog v-model:open="tokenVisible">
      <DialogContent class="sm:max-w-[480px]" @interact-outside.prevent>
        <DialogHeader>
          <DialogTitle>{{ t("platformApiKeys.createdTitle") }}</DialogTitle>
          <DialogDescription>{{ t("platformApiKeys.createdDescription") }}</DialogDescription>
        </DialogHeader>
        <Textarea :model-value="createdToken" readonly />
        <DialogFooter>
          <Button @click="copyToken">
            <CopyIcon />
            {{ t("platformApiKeys.copy") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { copyWithToast } from "@/utils/clipboard";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import type { TenantAPIKey, TenantAPIKeyCapability } from "@/api/tenant";
import { createPlatformAPIKey, deletePlatformAPIKey, listPlatformAPIKeys } from "@/api/system";
import {
  PLATFORM_API_KEY_CAPABILITY_GROUPS,
  SYSTEM_API_KEY_CAPABILITIES,
  TENANT_API_KEY_CAPABILITIES,
} from "@/config/apiKeyCapabilities";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { CopyIcon, Loader2Icon, PlusIcon, ShieldCheckIcon, Trash2Icon, TriangleAlertIcon } from "@lucide/vue";

const { t } = useI18n();
const allCapabilities = [...SYSTEM_API_KEY_CAPABILITIES, ...TENANT_API_KEY_CAPABILITIES];
const keys = ref<TenantAPIKey[]>([]);
const loading = ref(false);
const creating = ref(false);
const drawerVisible = ref(false);
const tokenVisible = ref(false);
const createdToken = ref("");
const deleteConfirmOpen = reactive<Record<string, boolean>>({});
const form = reactive({ name: "" });
const selected = reactive<Record<TenantAPIKeyCapability, boolean>>(
  allCapabilities.reduce(
    (result, capability) => {
      result[capability] = false;
      return result;
    },
    {} as Record<TenantAPIKeyCapability, boolean>,
  ),
);

const VISIBLE_CAPABILITY_CHIP_COUNT = 4;

type CapabilityChipView = {
  id: TenantAPIKeyCapability;
  label: string;
};

function capabilityChipsForKey(key: TenantAPIKey): CapabilityChipView[] {
  const chips: CapabilityChipView[] = [];
  for (const group of PLATFORM_API_KEY_CAPABILITY_GROUPS) {
    for (const item of group.capabilities) {
      if (key.capabilities?.includes(item.value)) {
        chips.push({ id: item.value, label: t(item.labelKey) });
      }
    }
  }
  return chips;
}

function visibleCapabilityChips(key: TenantAPIKey) {
  return capabilityChipsForKey(key).slice(0, VISIBLE_CAPABILITY_CHIP_COUNT);
}

function hiddenCapabilityCount(key: TenantAPIKey) {
  const total = capabilityChipsForKey(key).length;
  return Math.max(0, total - VISIBLE_CAPABILITY_CHIP_COUNT);
}

function capabilityGroupsForKey(key: TenantAPIKey) {
  return PLATFORM_API_KEY_CAPABILITY_GROUPS.map((group) => {
    const labels = group.capabilities
      .filter((item) => key.capabilities?.includes(item.value))
      .map((item) => t(item.labelKey));
    if (labels.length === 0) return null;
    return { key: group.key, label: t(group.labelKey), labels };
  }).filter((group): group is { key: string; label: string; labels: string[] } => group !== null);
}

function formatDate(value?: string) {
  if (!value) return t("platformApiKeys.never");
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function resetForm() {
  form.name = "";
  allCapabilities.forEach((capability) => {
    selected[capability] = false;
  });
}

function openCreate() {
  resetForm();
  drawerVisible.value = true;
}

function groupSelected(capabilities: TenantAPIKeyCapability[]) {
  return capabilities.every((capability) => selected[capability]);
}

function toggleGroup(capabilities: TenantAPIKeyCapability[]) {
  const next = !groupSelected(capabilities);
  capabilities.forEach((capability) => {
    selected[capability] = next;
  });
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
  const capabilities = allCapabilities.filter((capability) => selected[capability]);
  if (!form.name.trim()) {
    MessagePlugin.warning(t("platformApiKeys.nameRequired"));
    return;
  }
  if (capabilities.length === 0) {
    MessagePlugin.warning(t("platformApiKeys.capabilityRequired"));
    return;
  }
  creating.value = true;
  try {
    const response = await createPlatformAPIKey({ name: form.name.trim(), capabilities });
    createdToken.value = response.data?.token ?? "";
    drawerVisible.value = false;
    tokenVisible.value = true;
    await reload();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformApiKeys.createFailed"));
  } finally {
    creating.value = false;
  }
}

async function confirmDelete(key: TenantAPIKey) {
  deleteConfirmOpen[key.id] = false;
  await deleteKey(key);
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

async function copyToken() {
  const ok = await copyWithToast(createdToken.value, "platformApiKeys.copySuccess");
  if (ok) tokenVisible.value = false;
}

onMounted(reload);
</script>
