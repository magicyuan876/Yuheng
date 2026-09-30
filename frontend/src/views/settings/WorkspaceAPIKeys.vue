<template>
  <div class="flex flex-col gap-4">
    <div>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-foreground m-0 text-lg leading-[26px] font-semibold">{{ t("workspaceApiKeys.title") }}</h2>
        <Button size="sm" data-slot="api-key-create" @click="openCreate">
          <PlusIcon />
          {{ t("workspaceApiKeys.create") }}
        </Button>
      </div>
      <p class="text-muted-foreground mt-1.5 mb-0 text-[13px] leading-5">{{ t("workspaceApiKeys.description") }}</p>
    </div>

    <Alert class="border-transparent bg-[var(--td-warning-color-light)]">
      <TriangleAlertIcon class="text-warning" />
      <AlertTitle class="text-foreground font-normal">{{ t("workspaceApiKeys.securityNotice") }}</AlertTitle>
    </Alert>

    <div class="border-border w-full overflow-x-auto rounded-[10px] border">
      <Table class="min-w-[760px]">
        <TableHeader>
          <TableRow class="hover:bg-transparent">
            <TableHead :class="headClass">{{ t("workspaceApiKeys.name") }}</TableHead>
            <TableHead :class="headClass">{{ t("workspaceApiKeys.key") }}</TableHead>
            <TableHead :class="headClass">{{ t("workspaceApiKeys.access") }}</TableHead>
            <TableHead :class="headClass">{{ t("workspaceApiKeys.knowledgeBases") }}</TableHead>
            <TableHead :class="headClass">{{ t("workspaceApiKeys.expires") }}</TableHead>
            <TableHead :class="headClass">{{ t("workspaceApiKeys.lastUsed") }}</TableHead>
            <TableHead :class="[headClass, 'w-[84px] text-right']">{{ t("workspaceApiKeys.actions") }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="loading">
            <TableRow v-for="i in 2" :key="i">
              <TableCell v-for="c in 7" :key="c"><Skeleton class="h-4 w-full" /></TableCell>
            </TableRow>
          </template>
          <template v-else>
            <TableRow v-for="key in keys" :key="key.id" data-slot="api-key-row" class="hover:bg-transparent">
              <TableCell :class="cellClass">
                <div class="text-foreground max-w-[180px] truncate text-[13px] font-semibold">{{ key.name }}</div>
                <time class="text-placeholder block text-xs whitespace-nowrap" :datetime="key.created_at">
                  {{ t("workspaceApiKeys.createdAt") }} {{ formatApiKeyDateTime(key.created_at) }}
                </time>
              </TableCell>
              <TableCell :class="cellClass">
                <code class="font-mono text-xs whitespace-nowrap">{{ maskApiKey(key.api_key) }}</code>
              </TableCell>
              <TableCell :class="cellClass">
                <Badge v-if="key.full_access" variant="outline">{{ t("workspaceApiKeys.fullAccess") }}</Badge>
                <ApiKeyCapabilityChips
                  v-else
                  :capabilities="key.capabilities"
                  :groups="TENANT_API_KEY_CAPABILITY_GROUPS"
                  :max-visible="2"
                />
              </TableCell>
              <TableCell :class="cellClass">
                <span class="text-xs whitespace-nowrap">{{ knowledgeBaseSummary(key) }}</span>
              </TableCell>
              <TableCell :class="cellClass">
                <div class="flex items-center gap-1.5 text-xs whitespace-nowrap">
                  <span>{{
                    key.expires_at ? formatApiKeyDateTime(key.expires_at) : t("workspaceApiKeys.expiryNever")
                  }}</span>
                  <Badge v-if="isApiKeyExpired(key, new Date())" variant="destructive">
                    {{ t("workspaceApiKeys.expired") }}
                  </Badge>
                </div>
              </TableCell>
              <TableCell :class="cellClass">
                <span class="text-xs whitespace-nowrap">
                  {{ formatApiKeyDateTime(key.last_used_at) || t("apiKeys.neverUsed") }}
                </span>
              </TableCell>
              <TableCell :class="cellClass">
                <div class="flex items-center justify-end gap-0.5">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    data-slot="api-key-edit"
                    :title="t('common.edit')"
                    :aria-label="t('common.edit')"
                    @click="openEdit(key)"
                  >
                    <PencilIcon />
                  </Button>
                  <ApiKeyRevokeButton
                    :label="t('workspaceApiKeys.revoke')"
                    :message="t('workspaceApiKeys.revokeConfirm', { name: key.name })"
                    @confirm="revoke(key)"
                  />
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="!keys.length" class="hover:bg-transparent">
              <TableCell :colspan="7">
                <Empty>
                  <EmptyDescription>{{ t("workspaceApiKeys.empty") }}</EmptyDescription>
                </Empty>
              </TableCell>
            </TableRow>
          </template>
        </TableBody>
      </Table>
    </div>

    <SettingDrawer
      :visible="drawerVisible"
      :title="editingKey ? t('workspaceApiKeys.editTitle') : t('workspaceApiKeys.create')"
      :description="editingKey ? t('workspaceApiKeys.editDescription') : t('workspaceApiKeys.createDescription')"
      :icon="KeyRoundIcon"
      width="560px"
      :min-width="480"
      :max-width="920"
      storage-key="setting-drawer:width:workspace-api-key"
      :close-on-overlay-click="false"
      :confirm-text="editingKey ? t('common.save') : t('workspaceApiKeys.create')"
      :confirm-loading="saving"
      @update:visible="drawerVisible = $event"
      @confirm="save"
    >
      <div class="flex flex-col gap-5">
        <section class="flex flex-col gap-2">
          <Label for="workspace-api-key-name" :class="fieldLabelClass">{{ t("workspaceApiKeys.name") }}</Label>
          <Input
            id="workspace-api-key-name"
            v-model="form.name"
            :placeholder="t('workspaceApiKeys.namePlaceholder')"
            maxlength="128"
          />
        </section>

        <section class="flex flex-col gap-2">
          <Label :class="fieldLabelClass">{{ t("workspaceApiKeys.access") }}</Label>
          <RadioGroup
            :model-value="form.fullAccess ? 'full' : 'scoped'"
            class="flex flex-col gap-2.5"
            @update:model-value="(value) => (form.fullAccess = value === 'full')"
          >
            <div class="flex items-start gap-2">
              <RadioGroupItem id="workspace-api-key-scoped" value="scoped" class="mt-0.5" />
              <div>
                <Label for="workspace-api-key-scoped" class="cursor-pointer text-[13px]">
                  {{ t("workspaceApiKeys.scoped") }}
                </Label>
                <p :class="hintClass">{{ t("workspaceApiKeys.scopedHint") }}</p>
              </div>
            </div>
            <div class="flex items-start gap-2">
              <RadioGroupItem id="workspace-api-key-full" value="full" class="mt-0.5" />
              <div>
                <Label for="workspace-api-key-full" class="cursor-pointer text-[13px]">
                  {{ t("workspaceApiKeys.fullAccess") }}
                </Label>
                <p :class="hintClass">{{ t("workspaceApiKeys.fullAccessHint") }}</p>
              </div>
            </div>
          </RadioGroup>
        </section>

        <template v-if="!form.fullAccess">
          <section class="flex flex-col gap-2">
            <Label :class="fieldLabelClass">{{ t("workspaceApiKeys.capabilities") }}</Label>
            <ApiKeyCapabilityPicker
              v-model="form.capabilities"
              :groups="pickerGroups"
              id-prefix="workspace-api-key-capability"
            />
          </section>

          <section class="flex flex-col gap-2">
            <Label :class="fieldLabelClass">{{ t("workspaceApiKeys.knowledgeBases") }}</Label>
            <p :class="hintClass">
              {{
                knowledgeBaseScopeActive
                  ? t("workspaceApiKeys.knowledgeBaseHint")
                  : t("workspaceApiKeys.knowledgeBaseUnused")
              }}
            </p>
            <KnowledgeBaseScopePicker
              v-model="form.knowledgeBaseIds"
              :options="knowledgeBases"
              :all-label="t('workspaceApiKeys.allKnowledgeBases')"
              :disabled="!knowledgeBaseScopeActive"
            />
          </section>
        </template>

        <section class="flex flex-col gap-2">
          <Label :class="fieldLabelClass">{{ t("workspaceApiKeys.expires") }}</Label>
          <Select v-model="form.expiry">
            <SelectTrigger class="w-full text-[13px]" data-slot="api-key-expiry">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="never">{{ t("workspaceApiKeys.expiryNever") }}</SelectItem>
              <SelectItem v-for="days in EXPIRY_PRESET_DAYS" :key="days" :value="String(days)">
                {{ t("workspaceApiKeys.expiryInDays", { days }) }}
              </SelectItem>
              <SelectItem value="date">{{ t("workspaceApiKeys.expiryOnDate") }}</SelectItem>
            </SelectContent>
          </Select>
          <Input
            v-if="form.expiry === 'date'"
            v-model="form.expiryDate"
            type="date"
            :min="formatApiKeyDate(new Date())"
            :aria-label="t('workspaceApiKeys.expiryDate')"
          />
        </section>
      </div>
    </SettingDrawer>

    <ApiKeySecretDialog v-model:token="createdToken" :title="t('workspaceApiKeys.createdTitle')" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { KeyRoundIcon, PencilIcon, PlusIcon, TriangleAlertIcon } from "@lucide/vue";
import {
  createTenantAPIKey,
  deleteTenantAPIKey,
  listTenantAPIKeys,
  updateTenantAPIKey,
  type TenantAPIKey,
} from "@/api/tenant";
import { listKnowledgeBases } from "@/api/knowledge-base";
import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";
import { TENANT_API_KEY_CAPABILITY_GROUPS } from "@/config/apiKeyCapabilities";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import ApiKeyCapabilityChips from "@/components/api-keys/ApiKeyCapabilityChips.vue";
import ApiKeyCapabilityPicker from "@/components/api-keys/ApiKeyCapabilityPicker.vue";
import ApiKeyRevokeButton from "@/components/api-keys/ApiKeyRevokeButton.vue";
import ApiKeySecretDialog from "@/components/api-keys/ApiKeySecretDialog.vue";
import KnowledgeBaseScopePicker, { type KnowledgeBaseOption } from "@/components/api-keys/KnowledgeBaseScopePicker.vue";
import { formatApiKeyDate, formatApiKeyDateTime, maskApiKey } from "@/components/api-keys/apiKeyDisplay";
import {
  EXPIRY_PRESET_DAYS,
  emptyWorkspaceApiKeyForm,
  isApiKeyExpired,
  usesKnowledgeBaseScope,
  validateWorkspaceApiKeyForm,
  workspaceApiKeyFormFromKey,
  workspaceApiKeyPayload,
  type WorkspaceApiKeyForm,
} from "./workspaceApiKeyForm";

import { Alert, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

/**
 * Workspace API keys (Settings → Workspace). The routes are Owner-only
 * (GET/POST/PUT/DELETE /tenants/:id/api-keys), and so is the navigation entry
 * (settingsAccess.ts); this component does not gate again.
 */
const { t } = useI18n();
const authStore = useAuthStore();
const deploymentCapabilities = useDeploymentCapabilitiesStore();

const headClass = "bg-secondary text-placeholder p-[11px_12px] text-xs font-medium";
const cellClass = "text-muted-foreground p-[11px_12px] align-middle text-[13px] leading-[1.45]";
const fieldLabelClass = "text-foreground text-sm font-semibold";
const hintClass = "text-placeholder m-0 text-xs leading-[18px]";

const keys = ref<TenantAPIKey[]>([]);
const knowledgeBases = ref<KnowledgeBaseOption[]>([]);
const loading = ref(false);
const saving = ref(false);
const drawerVisible = ref(false);
const editingKey = ref<TenantAPIKey | null>(null);
const form = ref<WorkspaceApiKeyForm>(emptyWorkspaceApiKeyForm());
const createdToken = ref("");

// The key belongs to the workspace the user is working in, which the tenant
// switcher can make different from their home workspace.
const tenantId = computed(() => Number(authStore.effectiveTenantId ?? 0));

// docs_* grants only mean something while the docs module is deployed.
const pickerGroups = computed(() =>
  TENANT_API_KEY_CAPABILITY_GROUPS.filter(
    (group) => group.key !== "docs" || deploymentCapabilities.isSupported("docs"),
  ),
);

const knowledgeBaseScopeActive = computed(() => usesKnowledgeBaseScope(form.value));

const knowledgeBaseNames = computed(() => new Map(knowledgeBases.value.map((kb) => [kb.id, kb.name])));

function knowledgeBaseSummary(key: TenantAPIKey): string {
  const ids = key.full_access ? [] : (key.knowledge_base_ids ?? []);
  if (ids.length === 0) return t("workspaceApiKeys.allKnowledgeBases");
  if (ids.length === 1) return knowledgeBaseNames.value.get(ids[0]) || t("workspaceApiKeys.knowledgeBaseMissing");
  return t("workspaceApiKeys.knowledgeBaseCount", { count: ids.length });
}

async function loadKeys() {
  if (!tenantId.value) return;
  loading.value = true;
  try {
    const response = await listTenantAPIKeys(tenantId.value);
    if (response.success) keys.value = response.data ?? [];
    else MessagePlugin.error(response.message || t("error.tenant.listApiKeysFailed"));
  } finally {
    loading.value = false;
  }
}

async function loadKnowledgeBases() {
  try {
    const response = (await listKnowledgeBases()) as unknown as { data?: Array<{ id: string; name: string }> };
    knowledgeBases.value = (response.data ?? []).map((kb) => ({ id: kb.id, name: kb.name }));
  } catch {
    MessagePlugin.error(t("workspaceApiKeys.knowledgeBaseLoadFailed"));
  }
}

function openCreate() {
  editingKey.value = null;
  form.value = emptyWorkspaceApiKeyForm();
  drawerVisible.value = true;
}

function openEdit(key: TenantAPIKey) {
  editingKey.value = key;
  form.value = workspaceApiKeyFormFromKey(key);
  drawerVisible.value = true;
}

async function save() {
  const now = new Date();
  const error = validateWorkspaceApiKeyForm(form.value, now);
  if (error) {
    MessagePlugin.warning(t(`workspaceApiKeys.${error}`));
    return;
  }
  const payload = workspaceApiKeyPayload(form.value, now);
  saving.value = true;
  try {
    if (editingKey.value) {
      const response = await updateTenantAPIKey(tenantId.value, editingKey.value.id, payload);
      if (!response.success) {
        MessagePlugin.error(response.message || t("error.tenant.updateApiKeyFailed"));
        return;
      }
      MessagePlugin.success(t("workspaceApiKeys.updateSuccess"));
    } else {
      const response = await createTenantAPIKey(tenantId.value, payload);
      if (!response.success) {
        MessagePlugin.error(response.message || t("error.tenant.createApiKeyFailed"));
        return;
      }
      createdToken.value = response.data?.token ?? "";
    }
    drawerVisible.value = false;
    await loadKeys();
  } finally {
    saving.value = false;
  }
}

async function revoke(key: TenantAPIKey) {
  const response = await deleteTenantAPIKey(tenantId.value, key.id);
  if (!response.success) {
    MessagePlugin.error(response.message || t("error.tenant.deleteApiKeyFailed"));
    return;
  }
  MessagePlugin.success(t("workspaceApiKeys.revokeSuccess"));
  await loadKeys();
}

onMounted(() => {
  void deploymentCapabilities.ensureLoaded();
  void Promise.all([loadKeys(), loadKnowledgeBases()]);
});
</script>
