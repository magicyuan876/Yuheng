<template>
  <div class="w-full">
    <div class="mb-7">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ t("webSearchSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.6]">{{ t("webSearchSettings.description") }}</p>
    </div>

    <h3 class="text-foreground m-0 mb-4 text-base font-semibold">{{ t("webSearchSettings.providersTitle") }}</h3>

    <!-- Provider List —— 与 ModelSettings 的卡片同形：左侧标识徽章 + 标题 / 副标题 / proxy URL 三段式。
         不复用 SettingCard 的原因和 Models 一样：每页有微妙不同的右上侧栏需求（这里没有控件，
         Mcp 有开关），SettingCard 仍服务于其它消费者。 -->
    <div v-if="providerEntities.length === 0 && !authStore.hasRole('admin')" class="py-16 text-center">
      <Empty>
        <EmptyDescription class="text-placeholder mb-4 text-sm">{{
          t("webSearchSettings.noProvidersDesc")
        }}</EmptyDescription>
      </Empty>
    </div>
    <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
      <div
        v-for="entity in providerEntities"
        :key="entity.id"
        class="provider-card bg-card border-border relative flex min-w-0 items-start gap-3 rounded-[10px] border py-3.5 pr-3.5 pl-3 transition-[border-color,box-shadow] duration-[180ms] ease-out"
        :class="[
          `provider-card--${entity.provider}`,
          {
            'focus-visible:outline-primary cursor-pointer hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)] focus-visible:outline-2 focus-visible:outline-offset-2':
              isProviderCardClickable(),
          },
        ]"
        :role="isProviderCardClickable() ? 'button' : undefined"
        :tabindex="isProviderCardClickable() ? 0 : undefined"
        @click="onProviderCardClick($event, entity)"
        @keydown.enter="onProviderCardClick($event, entity)"
      >
        <!-- Tinted per provider by the scoped rules below. -->
        <div
          class="provider-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]"
          :class="badgeClass(entity.provider)"
          :style="badgeStyle(entity.provider)"
          :aria-label="entity.provider"
        >
          <img
            v-if="resolveLogo(entity.provider)?.mode === 'color'"
            :src="resolveLogo(entity.provider)!.url"
            :alt="entity.provider"
            class="block size-6 object-contain"
          />
          <template v-else-if="!resolveLogo(entity.provider)">
            {{ providerInitial(entity.provider) }}
          </template>
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <div class="flex min-w-0 items-center gap-1.5">
            <h3
              class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold"
              :title="entity.name"
            >
              {{ entity.name }}
            </h3>
            <Badge
              v-if="entity.is_builtin"
              variant="outline"
              class="border-primary/40 bg-primary/10 text-primary"
              :title="t('platformSharing.badgeHint')"
              >{{ t("platformSharing.badge") }}</Badge
            >
            <!-- `provider-card__actions` is a hook: onProviderCardClick ignores keys pressed inside it. -->
            <div v-if="getProviderOptions(entity).length > 0" class="provider-card__actions shrink-0" @click.stop>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="provider-card__more text-placeholder hover:text-foreground focus-visible:text-foreground shrink-0 p-0.5 opacity-0 transition-opacity duration-150 hover:bg-[var(--td-bg-color-secondarycontainer)] focus-visible:bg-[var(--td-bg-color-secondarycontainer)]"
                    :aria-label="t('docs.tree.moreActions')"
                  >
                    <EllipsisIcon />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    v-for="opt in getProviderOptions(entity)"
                    :key="opt.value"
                    @select="handleMenuAction({ value: opt.value }, entity)"
                  >
                    <span :class="opt.theme === 'error' ? 'text-destructive' : ''">{{ opt.content }}</span>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <div class="text-muted-foreground flex min-w-0 flex-wrap items-center gap-1 text-xs leading-[1.4]">
            <span class="font-medium">{{ providerTypeLabel(entity.provider) }}</span>
            <template v-if="entity.description">
              <span class="text-placeholder">·</span>
              <span class="min-w-0 truncate" :title="entity.description">{{ entity.description }}</span>
            </template>
          </div>
          <div
            v-if="entity.parameters?.proxy_url"
            class="text-placeholder min-w-0 truncate font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] text-[11px] leading-[1.4] whitespace-nowrap"
            :title="entity.parameters.proxy_url"
          >
            {{ entity.parameters.proxy_url }}
          </div>
        </div>
      </div>
      <button
        v-if="authStore.hasRole('admin')"
        type="button"
        data-slot="add-provider-card"
        class="border-border text-placeholder hover:border-primary hover:text-primary hover:bg-primary/6 focus-visible:border-primary focus-visible:text-primary focus-visible:bg-primary/6 focus-visible:outline-primary flex h-full min-h-[68px] w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed bg-transparent text-center transition-all duration-[180ms] ease-out focus-visible:outline-2 focus-visible:outline-offset-2"
        @click="openAddDialog"
      >
        <span class="bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg" aria-hidden="true">
          <PlusIcon class="size-[18px]" />
        </span>
        <span class="text-[13px] leading-[1.4] font-medium">{{ t("webSearchSettings.addProvider") }}</span>
      </button>
    </div>

    <!-- Add/Edit Drawer — 与 ModelEditorDialog / Parser / Storage 抽屉同款风格 -->
    <SettingDrawer
      v-model:visible="showAddProviderDialog"
      :title="editingProvider ? t('webSearchSettings.editProvider') : t('webSearchSettings.addProvider')"
      :class="drawerClass"
      :confirm-loading="saving"
      @confirm="saveProvider"
    >
      <!--
        Header icon — 与列表 .provider-card__badge 同款 logo/mono/fallback。
        - color logo（如 Bing/Google 彩色徽标）→ <img>，header 容器变白底 + 细边
        - mono logo（mask-image）→ ::before-style span，currentColor 染色
        - fallback：providerId 首字母 monogram
      -->
      <template v-if="selectedProviderType" #headerIcon>
        <img
          v-if="drawerLogo?.mode === 'color'"
          :src="drawerLogo.url"
          :alt="selectedProviderType.id"
          class="block size-6 object-contain"
        />
        <span
          v-else-if="drawerLogo?.mode === 'mono'"
          class="header-icon__mono inline-block size-[22px]"
          :style="drawerLogoStyle"
        />
        <span v-else class="text-[15px] font-semibold tracking-[0.02em]">{{
          providerInitial(selectedProviderType.id)
        }}</span>
      </template>

      <!--
        Subtitle: provider 类型名 + 官方文档外链（若有）。
      -->
      <template v-if="selectedProviderType" #subtitle>
        <span>{{ selectedProviderType.name }}</span>
        <a
          v-if="selectedProviderType.docs_url"
          :href="selectedProviderType.docs_url"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary ml-1.5 inline-flex items-center gap-1 align-baseline text-xs font-medium no-underline transition-colors duration-150 hover:text-[var(--td-brand-color-active)]"
        >
          {{ t("webSearchSettings.viewDocs") }}
          <LinkIcon class="size-3" />
        </a>
      </template>

      <!--
        Test connection (footer-left, 与其他抽屉同款)。已经统一为唯一入口 —
        外层卡片菜单不再露出"测试连接"，所有测试都从这里发起。

        全部 provider 都显示按钮（包括 DuckDuckGo / SearXNG 这些"免费"的）—
        免费只是不要 api_key，不代表不需要测：DuckDuckGo 走外网可能被墙、
        SearXNG 是自托管要验 base_url 可达性。disabled 由 canTestConnection
        统一控制，缺哪个必填字段就置灰。
      -->
      <template v-if="selectedProviderType" #footer-left>
        <Button variant="outline" :disabled="testing || !canTestConnection" @click="testConnection">
          <CircleCheckIcon v-if="!testing && lastTestOk === true" class="text-primary size-4 shrink-0" />
          <CircleXIcon v-else-if="!testing && lastTestOk === false" class="text-destructive size-4 shrink-0" />
          <Loader2Icon v-if="testing" class="animate-spin" />
          {{ testing ? t("webSearchSettings.testing") : t("webSearchSettings.testConnection") }}
        </Button>
      </template>

      <!-- `setting-drawer__section` / `__section-title` are styled by SettingDrawer
           (spacing, dividers, the brand bar before each title); the fields sit
           directly in the section, which spaces them. -->
      <form ref="formRef" @submit.prevent>
        <!-- Section 1 — 基本信息 -->
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ t("webSearchSettings.basicSection", "基本信息") }}
          </h4>

          <!-- providerType 选择器：仅在新建时可改 -->
          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("webSearchSettings.providerTypeLabel") }}
            </label>
            <!--
              Just provider name in each option — we used to append a "免费"
              t-tag for providers that don't take an api_key, but the
              "免费"分类对用户决策没什么帮助（DuckDuckGo / SearXNG 也都
              需要可用的网络/自托管实例），反而占视觉空间。
            -->
            <Select
              :model-value="providerForm.provider"
              :disabled="!!editingProvider"
              @update:model-value="onProviderTypeChange"
            >
              <SelectTrigger class="w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="pt in providerTypes" :key="pt.id" :value="pt.id">
                  {{ pt.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("webSearchSettings.providerNameLabel") }}
            </label>
            <SettingsInput
              v-model="providerForm.name"
              :placeholder="selectedProviderType?.name || t('webSearchSettings.providerNamePlaceholder')"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>

          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("webSearchSettings.providerDescLabel") }}
            </label>
            <SettingsInput
              v-model="providerForm.description"
              :placeholder="t('webSearchSettings.providerDescPlaceholder')"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
        </section>

        <!-- Section 2 — 连接配置（base url / api key / engine id），仅当任意字段需要时渲染 -->
        <section
          v-if="
            selectedProviderType?.requires_api_key ||
            selectedProviderType?.supports_optional_api_key ||
            selectedProviderType?.requires_engine_id ||
            selectedProviderType?.requires_base_url ||
            selectedProviderType?.config_fields?.length
          "
          class="setting-drawer__section"
        >
          <h4 class="setting-drawer__section-title">
            {{ t("webSearchSettings.credentialsSection", "连接配置") }}
          </h4>

          <div v-if="selectedProviderType?.requires_base_url">
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("webSearchSettings.baseUrlLabel") }}
            </label>
            <SettingsInput
              v-model="providerForm.parameters.base_url"
              :placeholder="t('webSearchSettings.baseUrlPlaceholder')"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>

          <!--
            Edit 模式下凭证由 CredentialResource 管理（独立的 /credentials
            子资源调用），不与本表单 submit 耦合；Create 模式下用 plain
            password input + lock prefix-icon，与 ModelEditorDialog 一致。
          -->
          <div v-if="selectedProviderType?.requires_api_key || selectedProviderType?.supports_optional_api_key">
            <label
              class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium"
              :class="{
                'before:text-destructive before:mr-1 before:leading-none before:font-medium before:content-[\'*\']':
                  selectedProviderType?.requires_api_key,
              }"
            >
              {{
                selectedProviderType?.supports_optional_api_key && !selectedProviderType?.requires_api_key
                  ? t("webSearchSettings.apiKeyOptionalLabel", "API Key（可选）")
                  : t("webSearchSettings.apiKeyLabel")
              }}
            </label>
            <CredentialResource
              v-if="editingProvider?.id"
              :api="credentialApi"
              :fields="credentialFields"
              :meta="credentialMeta"
            />
            <SettingsInput
              v-else
              v-model="providerForm.parameters.api_key"
              type="password"
              :placeholder="apiKeyPlaceholder"
              :prefix-icon="LockIcon"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>

          <div v-if="selectedProviderType?.requires_engine_id">
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("webSearchSettings.engineIdLabel") }}
            </label>
            <SettingsInput
              v-model="providerForm.parameters.engine_id"
              :placeholder="t('webSearchSettings.engineIdLabel')"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>

          <div v-for="field in selectedProviderType?.config_fields || []" :key="field.key">
            <label
              class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium"
              :class="{
                'before:text-destructive before:mr-1 before:leading-none before:font-medium before:content-[\'*\']':
                  field.required,
              }"
            >
              {{ configFieldText(field.label_key, field.label) }}
            </label>
            <Select
              v-if="field.type === 'select'"
              :model-value="providerForm.parameters.extra_config[field.key]"
              @update:model-value="(v: unknown) => setExtraConfig(field.key, v)"
            >
              <SelectTrigger class="w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in field.options || []" :key="option.value" :value="option.value">
                  {{ configFieldText(option.label_key, option.label) }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p v-if="field.description" class="text-placeholder m-0 mt-1 text-xs leading-normal">
              {{ configFieldText(field.description_key, field.description) }}
            </p>
          </div>
        </section>

        <!-- Section 3 — 选项（代理 / 默认） -->
        <section v-if="selectedProviderType?.supports_proxy || selectedProviderType" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ t("webSearchSettings.optionsSection", "选项") }}
          </h4>

          <div v-if="selectedProviderType?.supports_proxy">
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("webSearchSettings.proxyUrlLabel") }}
            </label>
            <SettingsInput
              v-model="providerForm.parameters.proxy_url"
              :placeholder="t('webSearchSettings.proxyUrlPlaceholder')"
              input-class="text-[13px] md:text-[13px]"
            />
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">{{ t("webSearchSettings.proxyUrlHelp") }}</p>
          </div>

          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("webSearchSettings.setAsDefault") }}
            </label>
            <div class="flex items-center gap-2">
              <Switch
                :model-value="providerForm.is_default"
                @update:model-value="(v: boolean) => (providerForm.is_default = v)"
              />
              <span class="text-placeholder m-0 text-xs leading-normal">{{
                t("webSearchSettings.setAsDefaultDesc")
              }}</span>
            </div>
          </div>
        </section>
      </form>
    </SettingDrawer>

    <!-- 平台共享 / 取消共享确认 -->
    <Dialog v-model:open="sharingDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>
            {{ sharingDialog.shared ? t("platformSharing.shareAction") : t("platformSharing.unshareAction") }}
          </DialogTitle>
          <DialogDescription>
            {{
              sharingDialog.entity
                ? sharingDialog.shared
                  ? t("platformSharing.confirmShare", { name: sharingDialog.entity.name })
                  : t("platformSharing.confirmUnshare", { name: sharingDialog.entity.name })
                : ""
            }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <Button
            :variant="sharingDialog.shared ? 'default' : 'destructive'"
            :class="
              sharingDialog.shared
                ? ''
                : 'bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive'
            "
            :disabled="sharingDialog.pending"
            @click="doToggleSharing"
          >
            <Loader2Icon v-if="sharingDialog.pending" class="animate-spin" />
            {{ t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, reactive } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  listWebSearchProviders,
  listWebSearchProviderTypes,
  createWebSearchProvider,
  updateWebSearchProvider,
  deleteWebSearchProvider as deleteWebSearchProviderAPI,
  setWebSearchProviderSharing,
  testWebSearchProvider,
  putWebSearchProviderCredentials,
  deleteWebSearchProviderCredentialField,
  type WebSearchProviderEntity,
  type WebSearchProviderTypeInfo,
  type WebSearchCredentialField,
} from "@/api/web-search-provider";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import SettingsInput from "./SettingsInput.vue";
import CredentialResource, {
  type CredentialFieldDef,
  type CredentialResourceApi,
} from "@/components/credentials/CredentialResource.vue";
import { useConfirmDelete } from "@/components/settings/useConfirmDelete";
import { useAuthStore } from "@/stores/auth";
import { providerLogo } from "./providerLogos";

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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { CircleCheckIcon, CircleXIcon, EllipsisIcon, LinkIcon, Loader2Icon, LockIcon, PlusIcon } from "@lucide/vue";

const { t } = useI18n();
const authStore = useAuthStore();
const confirmDelete = useConfirmDelete();

// ===== State =====
const providerEntities = ref<WebSearchProviderEntity[]>([]);
const providerTypes = ref<WebSearchProviderTypeInfo[]>([]);
const showAddProviderDialog = ref(false);
const editingProvider = ref<WebSearchProviderEntity | null>(null);
const testing = ref(false);
const saving = ref(false);
const formRef = ref<any>();

// Tri-state hint icon next to the test button: null=neutral, true=just
// succeeded, false=just failed. Cleared whenever the user changes the
// underlying connection inputs (see watch() below, set up after providerForm
// is initialized so the watch's source function doesn't trip on TDZ).
const lastTestOk = ref<boolean | null>(null);

const providerForm = ref<{
  name: string;
  provider: string;
  description: string;
  parameters: {
    api_key?: string;
    engine_id?: string;
    base_url?: string;
    proxy_url?: string;
    extra_config: Record<string, string>;
  };
  is_default: boolean;
}>({
  name: "",
  provider: "duckduckgo",
  description: "",
  parameters: { extra_config: {} },
  is_default: false,
});

// Invalidate the cached test result whenever the user edits a connection
// field. proxy_url is excluded because the upstream call doesn't actually
// use it for credential validation.
watch(
  () => [
    providerForm.value.provider,
    providerForm.value.parameters?.api_key,
    providerForm.value.parameters?.engine_id,
    providerForm.value.parameters?.base_url,
    JSON.stringify(providerForm.value.parameters?.extra_config || {}),
  ],
  () => {
    lastTestOk.value = null;
  },
);

// ===== Computed =====
const selectedProviderType = computed(() => {
  return providerTypes.value.find((pt) => pt.id === providerForm.value.provider);
});

// Create-mode placeholder (edit mode replaces the input with
// <CredentialResource>, which has its own placeholder).
const apiKeyPlaceholder = computed(() => t("webSearchSettings.apiKeyPlaceholder"));

const credentialFields = computed<CredentialFieldDef<WebSearchCredentialField>[]>(() => [
  { key: "api_key", label: t("webSearchSettings.apiKeyLabel") as string },
]);

const credentialApi = computed<CredentialResourceApi<WebSearchCredentialField>>(() => {
  const id = editingProvider.value?.id ?? "";
  return {
    save: async (patch) => {
      const meta = await putWebSearchProviderCredentials(id, patch);
      return meta.fields;
    },
    remove: async (field) => {
      await deleteWebSearchProviderCredentialField(id, field);
    },
  };
});

// Initial configured? from the main provider response (embedded server-side
// in dto.WebSearchProviderResponse.Credentials).
const credentialMeta = computed(
  () =>
    editingProvider.value?.credentials ?? {
      api_key: { configured: false },
    },
);

// Per-provider class on the drawer — the non-scoped CSS block at the
// bottom uses .websearch-drawer--{id} to color the header-icon container
// to match the matching list-card badge.
const drawerClass = computed(() => {
  const id = providerForm.value.provider;
  return id ? `websearch-drawer websearch-drawer--${id}` : "websearch-drawer";
});

// Reuses providerLogo() so the drawer header icon matches whatever the
// list card showed for the same provider id.
const drawerLogo = computed(() => {
  const id = providerForm.value.provider;
  return id ? providerLogo("websearch", id) : null;
});

const drawerLogoStyle = computed((): Record<string, string> => {
  const logo = drawerLogo.value;
  if (!logo || logo.mode !== "mono") return {};
  return { "--logo-url": `url("${logo.url}")` };
});

// Whether "Test connection" can fire. New-mode requires the user to have
// typed an api_key (and engine_id / base_url where applicable); edit-mode
// can fire with no fresh api_key because the backend will fall back to
// the stored credential. Free providers don't show the button at all.
const canTestConnection = computed(() => {
  const pt = selectedProviderType.value;
  if (!pt) return false;
  if (editingProvider.value) return true;
  if (pt.requires_api_key && !providerForm.value.parameters.api_key) return false;
  if (pt.requires_engine_id && !providerForm.value.parameters.engine_id) return false;
  if (pt.requires_base_url && !providerForm.value.parameters.base_url) return false;
  if (pt.config_fields?.some((field) => field.required && !providerForm.value.parameters.extra_config?.[field.key]))
    return false;
  return true;
});

// 卡片首字母徽章。复用 providerType 信息表，让多字节缩写也走同一处。
const providerInitial = (providerId: string) => {
  const label = providerTypes.value.find((p) => p.id === providerId)?.name || providerId;
  return (label.trim().charAt(0) || "?").toUpperCase();
};

// 见 VectorStoreSettings 的同名注释：返回 --logo-url 给 ::before 用 mask 渲染。
const resolveLogo = (providerId: string) => providerLogo("websearch", providerId);

const badgeClass = (providerId: string) => {
  const m = resolveLogo(providerId)?.mode;
  return {
    "provider-card__badge--logo": !!m,
    "provider-card__badge--color": m === "color",
    "provider-card__badge--mono": m === "mono",
  };
};

const badgeStyle = (providerId: string): Record<string, string> => {
  const logo = resolveLogo(providerId);
  return logo?.mode === "mono" ? { "--logo-url": `url("${logo.url}")` } : {};
};

const providerTypeLabel = (providerId: string) => {
  return providerTypes.value.find((p) => p.id === providerId)?.name || providerId;
};

const configFieldText = (key: string | undefined, fallback: string) => {
  return key ? t(key, fallback) : fallback;
};

const providerConfigDefaults = (providerId: string) => {
  const fields = providerTypes.value.find((p) => p.id === providerId)?.config_fields || [];
  return Object.fromEntries(
    fields.filter((field) => field.default !== undefined).map((field) => [field.key, field.default as string]),
  );
};

// ===== Methods =====
const onProviderTypeChange = (value: unknown) => {
  providerForm.value.provider = String(value);
  providerForm.value.parameters = {
    extra_config: providerConfigDefaults(providerForm.value.provider),
  };
  lastTestOk.value = null;
};

// 动态配置字段的下拉值回写。
const setExtraConfig = (key: string, value: unknown) => {
  providerForm.value.parameters.extra_config[key] = String(value);
};

const loadProviderEntities = async () => {
  try {
    const response = await listWebSearchProviders();
    if (response.data && Array.isArray(response.data)) {
      providerEntities.value = response.data;
    }
  } catch (error) {
    console.error("Failed to load provider entities:", error);
  }
};

const loadProviderTypes = async () => {
  try {
    providerTypes.value = await listWebSearchProviderTypes();
  } catch (error) {
    console.error("Failed to load provider types:", error);
  }
};

const openAddDialog = () => {
  editingProvider.value = null;
  providerForm.value = {
    name: "",
    provider: providerTypes.value[0]?.id || "duckduckgo",
    description: "",
    parameters: {
      extra_config: providerConfigDefaults(providerTypes.value[0]?.id || "duckduckgo"),
    },
    is_default: providerEntities.value.length === 0,
  };
  lastTestOk.value = null;
  showAddProviderDialog.value = true;
};

const editProvider = (entity: WebSearchProviderEntity) => {
  editingProvider.value = entity;
  providerForm.value = {
    name: entity.name,
    provider: entity.provider,
    description: entity.description || "",
    parameters: {
      // Never pre-fill the api_key — even the redacted placeholder from the
      // server is ignored so that "non-empty means user typed it" holds.
      api_key: "",
      engine_id: entity.parameters?.engine_id || "",
      base_url: entity.parameters?.base_url || "",
      proxy_url: entity.parameters?.proxy_url || "",
      extra_config: {
        ...providerConfigDefaults(entity.provider),
        ...(entity.parameters?.extra_config || {}),
      },
    },
    is_default: entity.is_default || false,
  };
  lastTestOk.value = null;
  showAddProviderDialog.value = true;
};

const saveProvider = async () => {
  saving.value = true;
  try {
    // Build the parameters payload. api_key only flows in on initial
    // create — edit mode commits credentials through <CredentialResource>
    // (a dedicated PUT /credentials call) before this save runs.
    const paramsOut: WebSearchProviderEntity["parameters"] = {
      engine_id: providerForm.value.parameters.engine_id,
      base_url: providerForm.value.parameters.base_url,
      proxy_url: providerForm.value.parameters.proxy_url,
    };
    const extraConfig = Object.fromEntries(
      Object.entries(providerForm.value.parameters.extra_config || {}).filter(([, value]) => value !== ""),
    );
    if (Object.keys(extraConfig).length > 0) {
      paramsOut.extra_config = extraConfig;
    }
    if (!editingProvider.value && providerForm.value.parameters.api_key) {
      paramsOut.api_key = providerForm.value.parameters.api_key;
    }

    const data: Partial<WebSearchProviderEntity> = {
      name: providerForm.value.name.trim() || selectedProviderType.value?.name || providerForm.value.provider,
      provider: providerForm.value.provider as any,
      description: providerForm.value.description,
      parameters: paramsOut,
      is_default: providerForm.value.is_default,
    };

    if (editingProvider.value) {
      await updateWebSearchProvider(editingProvider.value.id!, data);
      MessagePlugin.success(t("webSearchSettings.toasts.providerUpdated"));
    } else {
      await createWebSearchProvider(data);
      MessagePlugin.success(t("webSearchSettings.toasts.providerCreated"));
    }
    showAddProviderDialog.value = false;
    await loadProviderEntities();
  } catch (error: any) {
    MessagePlugin.error(error?.message || "Failed to save provider");
  } finally {
    saving.value = false;
  }
};

const deleteProvider = (entity: WebSearchProviderEntity) => {
  confirmDelete({
    body: t("webSearchSettings.deleteConfirm"),
    onConfirm: async () => {
      try {
        await deleteWebSearchProviderAPI(entity.id!);
        MessagePlugin.success(t("webSearchSettings.toasts.providerDeleted"));
        await loadProviderEntities();
      } catch (error: any) {
        MessagePlugin.error(error?.message || "Failed to delete provider");
      }
    },
  });
};

const testConnection = async () => {
  testing.value = true;
  try {
    const data = {
      provider: providerForm.value.provider,
      parameters: { ...providerForm.value.parameters },
    };

    let ok = false;
    if (editingProvider.value && !data.parameters.api_key) {
      const res = await testWebSearchProvider(editingProvider.value.id!);
      ok = !!res.success;
      if (res.success) {
        MessagePlugin.success(t("webSearchSettings.toasts.testSuccess"));
      } else {
        MessagePlugin.error(res.error || t("webSearchSettings.toasts.testFailed"));
      }
    } else {
      const res = await testWebSearchProvider(undefined, data);
      ok = !!res.success;
      if (res.success) {
        MessagePlugin.success(t("webSearchSettings.toasts.testSuccess"));
      } else {
        MessagePlugin.error(res.error || t("webSearchSettings.toasts.testFailed"));
      }
    }
    lastTestOk.value = ok;
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("webSearchSettings.toasts.testFailed"));
    lastTestOk.value = false;
  } finally {
    testing.value = false;
  }
};

const isProviderCardClickable = () => authStore.hasRole("admin");

const onProviderCardClick = (event: Event, entity: WebSearchProviderEntity) => {
  if (!isProviderCardClickable()) return;
  if (event.type === "keydown") {
    const ke = event as KeyboardEvent;
    if (ke.key !== "Enter" && ke.key !== " ") return;
    ke.preventDefault();
  }
  const target = event.target as HTMLElement | null;
  if (target?.closest(".provider-card__actions")) return;
  editProvider(entity);
};

const getProviderOptions = (entity: WebSearchProviderEntity) => {
  // Web search providers carry external API credentials; the backend
  // gates every mutation/test behind Admin+ (RegisterWebSearchProviderRoutes).
  // Hide the action menu entirely for non-Admins so they don't trip 403s.
  // 测试连接已挪到编辑抽屉的 footer，不再放在外层菜单里 — 单一入口减少
  // 用户疑惑（"为什么有两个测试入口，结果一样吗？"）。
  if (!authStore.hasRole("admin")) {
    return [];
  }
  // 平台共享的 Provider 对普通空间管理员只读：列出来是为了让他们能选用，
  // 但 Base URL 和凭据是平台的。
  if (entity.is_builtin && !authStore.isSystemAdmin) {
    return [];
  }
  const options: Array<{ content: string; value: string; theme?: "error" }> = [
    { content: t("common.edit"), value: "edit" },
  ];
  if (authStore.isSystemAdmin) {
    options.push({
      content: entity.is_builtin ? t("platformSharing.unshareAction") : t("platformSharing.shareAction"),
      value: "sharing",
    });
  }
  // 共享中的 Provider 必须先取消共享再删。
  if (!entity.is_builtin) {
    options.push({ content: t("common.delete"), value: "delete", theme: "error" });
  }
  return options;
};

const handleMenuAction = (data: { value: string }, entity: WebSearchProviderEntity) => {
  switch (data.value) {
    case "edit":
      editProvider(entity);
      break;
    case "delete":
      deleteProvider(entity);
      break;
    case "sharing":
      confirmSharing(entity);
      break;
  }
};

// 切换平台共享。Provider 没有引用守卫 —— 空间是按次选用 Provider 的，失去访问
// 退化为「网络搜索不可用」，不会像模型那样留下悬空的向量索引。
const sharingDialogVisible = ref(false);
const sharingDialog = reactive<{ entity: WebSearchProviderEntity | null; shared: boolean; pending: boolean }>({
  entity: null,
  shared: false,
  pending: false,
});

const confirmSharing = (entity: WebSearchProviderEntity) => {
  sharingDialog.entity = entity;
  sharingDialog.shared = !entity.is_builtin;
  sharingDialog.pending = false;
  sharingDialogVisible.value = true;
};

async function doToggleSharing() {
  const entity = sharingDialog.entity;
  if (!entity || sharingDialog.pending) return;
  const shared = sharingDialog.shared;
  sharingDialog.pending = true;
  try {
    await setWebSearchProviderSharing(entity.id!, shared);
    MessagePlugin.success(shared ? t("platformSharing.sharedToast") : t("platformSharing.unsharedToast"));
    sharingDialogVisible.value = false;
    await loadProviderEntities();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformSharing.failedToast"));
  } finally {
    sharingDialog.pending = false;
  }
}

// ===== Init =====
onMounted(async () => {
  await Promise.all([loadProviderTypes(), loadProviderEntities()]);
});
</script>

<style scoped>
/*
 * Utilities cover the layout; these rules reach into generated markup or
 * repeat per-provider tints that are clearer as CSS:
 *  - mono logos are CSS-masked icons driven by a per-card --logo-url;
 *  - the per-provider badge tints (9 search sources);
 *  - the "more" button fades in on card hover / keyboard focus.
 */
.header-icon__mono {
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

.provider-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}

/* One more .provider-card so logo badges outrank the per-provider tints. */
.provider-card .provider-card__badge--logo {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.provider-card .provider-card__badge--mono::before {
  content: "";
  width: 22px;
  height: 22px;
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

.provider-card--duckduckgo .provider-card__badge {
  background: rgba(222, 88, 51, 0.12);
  color: #de5833;
}
.provider-card--bing .provider-card__badge {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.provider-card--google .provider-card__badge {
  background: rgba(66, 133, 244, 0.12);
  color: #4285f4;
}
.provider-card--tavily .provider-card__badge {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.provider-card--baidu .provider-card__badge {
  /* 百度官方主色（搜索框 du 标识那个蓝），#2932E1。低饱和版用 12% alpha
     浅底，跟其他 provider 一致。之前误填红色（混淆了百度地图等子产品）。 */
  background: rgba(41, 50, 225, 0.12);
  color: #2932e1;
}
.provider-card--searxng .provider-card__badge {
  background: rgba(33, 86, 137, 0.12);
  color: #215689;
}
.provider-card--ollama .provider-card__badge {
  background: rgba(70, 70, 70, 0.12);
  color: #464646;
}
.provider-card--keenable .provider-card__badge {
  background: rgba(20, 158, 130, 0.12);
  color: #149e82;
}
.provider-card--zhipu .provider-card__badge {
  background: rgba(37, 99, 235, 0.12);
  color: #2563eb;
}

.provider-card:hover .provider-card__more,
.provider-card:focus-within .provider-card__more,
.provider-card__actions:focus-within .provider-card__more {
  opacity: 1;
}
</style>

<!--
  Non-scoped block: per-provider header-icon coloring + color-logo
  background tweak. Same pattern as Storage/Parser drawers — these rules
  must be global so they always reach the teleported drawer panel. Each
  rule mirrors the matching .provider-card--{id} .provider-card__badge
  from the scoped block above so list-card → drawer hand-off stays
  visually continuous.
-->
<style>
/* 彩色 logo 时给 header-icon 容器一个白底 + 1px 边，避免品牌色浅底压在
   彩色图标上影响对比度。 */
.websearch-drawer .setting-drawer__header-icon:has(.header-icon__img) {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.websearch-drawer--duckduckgo .setting-drawer__header-icon {
  background: rgba(222, 88, 51, 0.12);
  color: #de5833;
}
.websearch-drawer--bing .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.websearch-drawer--google .setting-drawer__header-icon {
  background: rgba(66, 133, 244, 0.12);
  color: #4285f4;
}
.websearch-drawer--tavily .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.websearch-drawer--baidu .setting-drawer__header-icon {
  background: rgba(41, 50, 225, 0.12);
  color: #2932e1;
}
.websearch-drawer--searxng .setting-drawer__header-icon {
  background: rgba(33, 86, 137, 0.12);
  color: #215689;
}
.websearch-drawer--ollama .setting-drawer__header-icon {
  background: rgba(70, 70, 70, 0.12);
  color: #464646;
}
.websearch-drawer--keenable .setting-drawer__header-icon {
  background: rgba(20, 158, 130, 0.12);
  color: #149e82;
}
.websearch-drawer--zhipu .setting-drawer__header-icon {
  background: rgba(37, 99, 235, 0.12);
  color: #2563eb;
}
</style>
