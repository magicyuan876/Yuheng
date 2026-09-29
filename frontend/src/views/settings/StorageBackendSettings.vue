<template>
  <div class="w-full">
    <div class="mb-7">
      <div class="flex items-center justify-between gap-5">
        <div>
          <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ t("settings.storage.title") }}</h2>
          <p class="text-muted-foreground m-0 text-sm leading-[1.6]">
            {{ t("settings.storageBackend.description") }}
          </p>
        </div>
      </div>
    </div>

    <div class="min-h-[120px]">
      <Empty v-if="!loading && backends.length === 0 && !authStore.hasRole('admin')">
        <EmptyDescription>{{ t("settings.storageBackend.empty") }}</EmptyDescription>
      </Empty>
      <div v-else-if="!loading" class="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
        <div
          v-for="backend in backends"
          :key="backend.id"
          class="backend-card bg-card border-border relative flex min-w-0 items-start gap-3 rounded-[10px] border px-4 py-3.5 transition-[border-color,box-shadow] duration-[180ms] ease-out hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]"
          :class="[
            `backend-card--${backend.provider}`,
            {
              'focus-visible:outline-primary cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2':
                canEdit(backend),
            },
          ]"
          :role="canEdit(backend) ? 'button' : undefined"
          :tabindex="canEdit(backend) ? 0 : undefined"
          @click="onCardClick($event, backend)"
          @keydown.enter="onCardClick($event, backend)"
        >
          <!-- Tinted per provider by the scoped rules below. -->
          <div
            class="backend-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]"
            :class="badgeClass(backend.provider)"
            :style="badgeStyle(backend.provider)"
            :aria-label="backend.provider"
          >
            <img
              v-if="resolveLogo(backend.provider)?.mode === 'color'"
              :src="resolveLogo(backend.provider)!.url"
              :alt="backend.provider"
              class="block size-6 object-contain"
            />
            <template v-else-if="!resolveLogo(backend.provider)">{{ providerInitial(backend.provider) }}</template>
          </div>
          <div class="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
            <div class="flex min-w-0 items-center gap-1.5">
              <h3 class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold">
                {{ backend.name }}
              </h3>
              <Badge v-if="backend.id === defaultID" class="bg-primary/10 text-primary">{{
                t("settings.storageBackend.defaultTag")
              }}</Badge>
              <Badge
                v-if="backend.is_builtin"
                variant="outline"
                class="border-primary/40 bg-primary/10 text-primary"
                :title="t('platformSharing.badgeHint')"
                >{{ t("platformSharing.badge") }}</Badge
              >
              <!-- `backend-card__actions` is a hook: onCardClick ignores clicks and keys inside it. -->
              <div v-if="hasActions(backend)" class="backend-card__actions flex shrink-0 items-center" @click.stop>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      class="backend-card__action-btn text-placeholder hover:text-foreground focus-visible:text-foreground shrink-0 p-0.5 opacity-0 transition-opacity duration-150 hover:bg-[var(--td-bg-color-secondarycontainer)] focus-visible:bg-[var(--td-bg-color-secondarycontainer)]"
                      :aria-label="t('docs.tree.moreActions')"
                    >
                      <EllipsisIcon />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      v-for="opt in getBackendOptions(backend)"
                      :key="opt.value"
                      @select="handleMenuAction(opt.value, backend)"
                    >
                      <span :class="opt.theme === 'error' ? 'text-destructive' : ''">{{ opt.content }}</span>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
            <p class="text-muted-foreground m-0 mt-0.5 flex min-w-0 items-center text-xs leading-normal">
              <span>{{ backend.provider.toUpperCase() }}</span>
              <template v-if="backendMeta(backend)">
                <span class="text-placeholder mx-1.5 shrink-0">·</span>
                <span class="min-w-0 truncate">{{ backendMeta(backend) }}</span>
              </template>
            </p>
          </div>
        </div>

        <button
          v-if="authStore.hasRole('admin')"
          type="button"
          data-slot="add-backend-card"
          class="border-border text-placeholder hover:border-primary hover:text-primary hover:bg-primary/6 focus-visible:border-primary focus-visible:text-primary focus-visible:bg-primary/6 focus-visible:outline-primary flex h-full min-h-[68px] w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed bg-transparent text-center transition-all duration-[180ms] ease-out focus-visible:outline-2 focus-visible:outline-offset-2"
          @click="openCreate"
        >
          <span
            class="bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg"
            aria-hidden="true"
          >
            <PlusIcon class="size-[18px]" />
          </span>
          <span class="text-[13px] leading-[1.4] font-medium">{{ t("settings.storageBackend.add") }}</span>
        </button>
      </div>
      <div v-else class="flex justify-center py-10">
        <Loader2Icon class="animate-spin" />
      </div>
    </div>

    <SettingDrawer
      v-model:visible="visible"
      :title="editing ? t('settings.storageBackend.editTitle') : t('settings.storageBackend.createTitle')"
      :class="`storage-backend-drawer storage-backend-drawer--${form.provider}`"
      :confirm-loading="saving"
      @confirm="save"
      @cancel="visible = false"
    >
      <template #headerIcon>
        <img
          v-if="currentLogo?.mode === 'color'"
          :src="currentLogo.url"
          :alt="form.provider"
          class="block size-6 object-contain"
        />
        <span
          v-else-if="currentLogo?.mode === 'mono'"
          class="header-icon__mono inline-block size-[22px]"
          :style="monoLogoStyle"
        />
        <span v-else class="text-[15px] font-semibold tracking-[0.02em]">{{ providerInitial(form.provider) }}</span>
      </template>
      <template #subtitle>
        <span>{{
          editing ? t("settings.storageBackend.editSubtitle") : t("settings.storageBackend.createSubtitle")
        }}</span>
      </template>

      <!-- `setting-drawer__section` / `__section-title` are styled by SettingDrawer
           (spacing, dividers, the brand bar before each title); the fields sit
           directly in the section, which spaces them. -->
      <form @submit.prevent>
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ t("settings.storageBackend.basicSection") }}</h4>
          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("settings.storageBackend.nameLabel") }}
            </label>
            <SettingsInput
              v-model="form.name"
              :placeholder="t('settings.storageBackend.namePlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("settings.storageBackend.providerLabel") }}
            </label>
            <Select :model-value="form.provider" :disabled="!!editing" @update:model-value="onProviderChange">
              <SelectTrigger class="w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="provider in providers" :key="provider" :value="provider">
                  {{ provider.toUpperCase() }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="form.provider === 'minio'">
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.storageBackend.modeLabel") }}
            </label>
            <div
              class="border-border inline-flex items-center gap-1 rounded-lg border bg-[var(--td-bg-color-component)] p-[3px]"
              role="radiogroup"
            >
              <button
                type="button"
                data-slot="segment"
                class="inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md border px-3 py-[5px] text-[13px] leading-none transition-all duration-150 disabled:cursor-not-allowed disabled:opacity-60"
                :class="
                  form.config.mode !== 'docker'
                    ? 'bg-card text-primary border-primary font-medium shadow-[0_1px_2px_rgba(15,23,42,0.04)]'
                    : 'text-muted-foreground not-disabled:hover:text-foreground border-transparent bg-transparent not-disabled:hover:bg-[var(--td-bg-color-container-hover)]'
                "
                :disabled="!!editing"
                @click="form.config.mode = 'remote'"
              >
                <CloudIcon class="size-3.5 shrink-0" />
                <span class="whitespace-nowrap">{{ t("settings.storageBackend.modeRemote") }}</span>
              </button>
              <button
                type="button"
                data-slot="segment"
                class="inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md border px-3 py-[5px] text-[13px] leading-none transition-all duration-150 disabled:cursor-not-allowed disabled:opacity-60"
                :class="
                  form.config.mode === 'docker'
                    ? 'bg-card text-primary border-primary font-medium shadow-[0_1px_2px_rgba(15,23,42,0.04)]'
                    : 'text-muted-foreground not-disabled:hover:text-foreground border-transparent bg-transparent not-disabled:hover:bg-[var(--td-bg-color-container-hover)]'
                "
                :disabled="!!editing"
                @click="form.config.mode = 'docker'"
              >
                <ServerIcon class="size-3.5 shrink-0" />
                <span class="whitespace-nowrap">{{ t("settings.storageBackend.modeEnv") }}</span>
              </button>
            </div>
          </div>
        </section>

        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ t("settings.storageBackend.connectionSection") }}</h4>
          <div v-if="needsEndpoint">
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
              >Endpoint</label
            >
            <SettingsInput
              v-model="form.config.endpoint"
              :disabled="!!editing"
              :placeholder="form.provider === 'minio' ? 'storage.example.com:9000' : 'https://storage.example.com'"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div v-if="needsRegion">
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
              >Region</label
            >
            <SettingsInput
              v-model="form.config.region"
              :disabled="!!editing"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <template v-if="needsCredentials">
            <div>
              <label
                class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
                >Access Key / Secret ID</label
              >
              <SettingsInput
                v-model="form.config.access_key_id"
                placeholder="***"
                clearable
                :prefix-icon="LockIcon"
                input-class="text-[13px] md:text-[13px]"
              />
            </div>
            <div>
              <label
                class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
                >Secret Key</label
              >
              <SettingsInput
                v-model="form.config.secret_access_key"
                type="password"
                placeholder="***"
                clearable
                :prefix-icon="LockIcon"
                input-class="text-[13px] md:text-[13px]"
              />
            </div>
          </template>
          <div v-if="form.provider !== 'local'">
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
              >Bucket</label
            >
            <SettingsInput
              v-model="form.config.bucket_name"
              :disabled="!!editing"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div v-if="form.provider === 'cos'">
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">App ID</label>
            <SettingsInput
              v-model="form.config.app_id"
              :disabled="!!editing"
              :placeholder="t('settings.storageBackend.optionalPlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
        </section>

        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ t("settings.storageBackend.advancedSection") }}</h4>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.storageBackend.pathPrefixLabel") }}
            </label>
            <SettingsInput
              v-model="form.config.path_prefix"
              :disabled="!!editing"
              placeholder="yuheng/"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div v-if="form.provider === 'minio'">
            <div class="flex items-center gap-2">
              <Switch
                :model-value="form.config.use_ssl"
                @update:model-value="(v: boolean) => (form.config.use_ssl = v)"
              />
              <span class="text-placeholder m-0 text-xs leading-normal">{{
                t("settings.storageBackend.useSslDesc")
              }}</span>
            </div>
          </div>
          <div v-if="form.provider === 's3'">
            <div class="flex items-center gap-2">
              <Switch
                :model-value="form.config.force_path_style"
                @update:model-value="(v: boolean) => (form.config.force_path_style = v)"
              />
              <span class="text-placeholder m-0 text-xs leading-normal">
                {{ t("settings.storageBackend.forcePathStyleDesc") }}
              </span>
            </div>
          </div>
          <div v-if="form.provider === 'oss'">
            <div class="flex items-center gap-2">
              <Switch
                :model-value="form.config.use_temp_bucket"
                @update:model-value="(v: boolean) => (form.config.use_temp_bucket = v)"
              />
              <span class="text-placeholder m-0 text-xs leading-normal">
                {{ t("settings.storageBackend.useTempBucketDesc") }}
              </span>
            </div>
          </div>
          <template
            v-if="['cos', 'tos'].includes(form.provider) || (form.provider === 'oss' && form.config.use_temp_bucket)"
          >
            <div>
              <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
                {{ t("settings.storageBackend.tempBucketLabel") }}
              </label>
              <SettingsInput
                v-model="form.config.temp_bucket_name"
                :placeholder="t('settings.storageBackend.tempBucketPlaceholder')"
                clearable
                input-class="text-[13px] md:text-[13px]"
              />
            </div>
            <div>
              <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
                {{ t("settings.storageBackend.tempRegionLabel") }}
              </label>
              <SettingsInput
                v-model="form.config.temp_region"
                :placeholder="t('settings.storageBackend.tempRegionPlaceholder')"
                clearable
                input-class="text-[13px] md:text-[13px]"
              />
            </div>
          </template>
        </section>
      </form>

      <template #footer-left>
        <Button variant="outline" :disabled="testing" @click="testRaw">
          <CircleCheckIcon v-if="!testing && rawTestResult === 'ok'" class="text-primary size-4 shrink-0" />
          <CircleXIcon v-else-if="!testing && rawTestResult === 'error'" class="text-destructive size-4 shrink-0" />
          <Loader2Icon v-if="testing" class="animate-spin" />
          {{ t("settings.storageBackend.testConnection") }}
        </Button>
      </template>
    </SettingDrawer>

    <!-- 平台共享 / 取消共享确认 -->
    <Dialog v-model:open="sharingDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{
            sharingDialog.shared ? t("platformSharing.shareAction") : t("platformSharing.unshareAction")
          }}</DialogTitle>
          <DialogDescription>
            {{
              sharingDialog.backend
                ? sharingDialog.shared
                  ? t("platformSharing.confirmShare", { name: sharingDialog.backend.name })
                  : t("platformSharing.confirmUnshare", { name: sharingDialog.backend.name })
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

    <!-- 删除确认 -->
    <Dialog v-model:open="deleteDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("settings.storageBackend.deleteTitle") }}</DialogTitle>
          <DialogDescription>
            {{ deleteTarget ? t("settings.storageBackend.deleteConfirm", { name: deleteTarget.name }) : "" }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <!-- The old DialogPlugin.confirm used its default (primary) confirm button here. -->
          <Button :disabled="deleteDialogPending" @click="doDelete">
            <Loader2Icon v-if="deleteDialogPending" class="animate-spin" />
            {{ t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import SettingsInput from "./SettingsInput.vue";
import { providerLogo } from "./providerLogos";
import {
  createStorageBackend,
  deleteStorageBackend,
  listStorageBackends,
  listStorageBackendTypes,
  setDefaultStorageBackend,
  setStorageBackendSharing,
  testStorageBackend,
  testStorageBackendByID,
  updateStorageBackend,
  type StorageBackend,
  type StorageBackendConfig,
} from "@/api/storage-backend";

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
import {
  CircleCheckIcon,
  CircleXIcon,
  CloudIcon,
  EllipsisIcon,
  Loader2Icon,
  LockIcon,
  PlusIcon,
  ServerIcon,
} from "@lucide/vue";

const { t } = useI18n();
const authStore = useAuthStore();
const loading = ref(false),
  saving = ref(false),
  testing = ref(false),
  visible = ref(false);
const backends = ref<StorageBackend[]>([]),
  providers = ref<string[]>([]),
  defaultID = ref("");
const editing = ref<StorageBackend | null>(null);
const rawTestResult = ref<"ok" | "error" | null>(null);
const blankConfig = (): StorageBackendConfig => ({
  mode: "remote",
  endpoint: "",
  region: "",
  access_key_id: "",
  secret_access_key: "",
  bucket_name: "",
  path_prefix: "",
  use_ssl: true,
});
const form = reactive<{ name: string; provider: string; config: StorageBackendConfig }>({
  name: "",
  provider: "local",
  config: blankConfig(),
});
const needsEndpoint = computed(
  () => !["local", "cos"].includes(form.provider) && !(form.provider === "minio" && form.config.mode === "docker"),
);
const needsRegion = computed(() => !["local", "minio"].includes(form.provider));
const needsCredentials = computed(
  () => form.provider !== "local" && !(form.provider === "minio" && form.config.mode === "docker"),
);

const resolveLogo = (provider: string) => providerLogo("storage", provider);
const providerInitial = (provider: string) => (provider || "?").trim().charAt(0).toUpperCase() || "?";
// 徽标配色按 provider 走卡片修饰类（见下方非 scoped style 块）。
const badgeClass = (provider: string) => {
  const mode = resolveLogo(provider)?.mode;
  return {
    "backend-card__badge--logo": !!mode,
    "backend-card__badge--color": mode === "color",
    "backend-card__badge--mono": mode === "mono",
  };
};
const badgeStyle = (provider: string): Record<string, string> => {
  const logo = resolveLogo(provider);
  return logo?.mode === "mono" ? { "--logo-url": `url("${logo.url}")` } : {};
};

const currentLogo = computed(() => providerLogo("storage", form.provider));
const monoLogoStyle = computed((): Record<string, string> => {
  const logo = currentLogo.value;
  if (!logo || logo.mode !== "mono") return {};
  return { "--logo-url": `url("${logo.url}")` };
});

function backendMeta(backend: StorageBackend): string {
  return (
    backend.config.endpoint ||
    backend.config.bucket_name ||
    backend.config.path_prefix ||
    t("settings.storageBackend.localStorage")
  );
}

// 平台共享的实例只读：列出来是为了让空间在建库时能选，配置是平台的。
const canEdit = (backend: StorageBackend) =>
  backend.source !== "env" && (backend.is_builtin ? authStore.isSystemAdmin : authStore.hasRole("admin"));
// 共享中的实例必须先取消共享再删 —— 取消共享那一步才会做跨空间引用检查。
const canDelete = (backend: StorageBackend) =>
  authStore.hasRole("admin") && backend.source !== "env" && !backend.legacy_alias && !backend.is_builtin;
const canShare = (backend: StorageBackend) => authStore.isSystemAdmin && backend.source !== "env";
const canSetDefault = (backend: StorageBackend) => backend.id !== defaultID.value && authStore.hasRole("admin");
// 测试连接对所有可见用户开放，因此每张卡至少有一个动作。
const hasActions = (_backend: StorageBackend) => true;

function getBackendOptions(backend: StorageBackend) {
  const options: { content: string; value: string; theme?: string }[] = [];
  options.push({ content: t("settings.storageBackend.testConnection"), value: "test" });
  if (canSetDefault(backend)) options.push({ content: t("settings.storageBackend.setDefault"), value: "default" });
  if (canEdit(backend)) options.push({ content: t("settings.storageBackend.edit"), value: "edit" });
  if (canShare(backend)) {
    options.push({
      content: backend.is_builtin ? t("platformSharing.unshareAction") : t("platformSharing.shareAction"),
      value: "sharing",
    });
  }
  if (canDelete(backend))
    options.push({ content: t("settings.storageBackend.delete"), value: "delete", theme: "error" });
  return options;
}

function handleMenuAction(value: string, backend: StorageBackend) {
  if (value === "test") testSaved(backend);
  else if (value === "default") makeDefault(backend);
  else if (value === "edit") openEdit(backend);
  else if (value === "delete") remove(backend);
  else if (value === "sharing") confirmSharing(backend);
}

// reka 的 Select 用 `update:model-value` 回值，包一层保持原 resetConfig 语义。
function onProviderChange(value: unknown) {
  form.provider = String(value);
  resetConfig();
}

// 切换平台共享。取消共享时后端会拒绝仍被其他空间（默认存储 / 知识库 / 活跃资源）
// 绑定的实例，错误文案里带着引用数量，直接透传。
const sharingDialogVisible = ref(false);
const sharingDialog = reactive<{ backend: StorageBackend | null; shared: boolean; pending: boolean }>({
  backend: null,
  shared: false,
  pending: false,
});

function confirmSharing(backend: StorageBackend) {
  sharingDialog.backend = backend;
  sharingDialog.shared = !backend.is_builtin;
  sharingDialog.pending = false;
  sharingDialogVisible.value = true;
}

async function doToggleSharing() {
  const backend = sharingDialog.backend;
  if (!backend || sharingDialog.pending) return;
  const shared = sharingDialog.shared;
  sharingDialog.pending = true;
  try {
    await setStorageBackendSharing(backend.id, shared);
    MessagePlugin.success(shared ? t("platformSharing.sharedToast") : t("platformSharing.unsharedToast"));
    sharingDialogVisible.value = false;
    await load();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformSharing.failedToast"));
  } finally {
    sharingDialog.pending = false;
  }
}

function onCardClick(event: Event, backend: StorageBackend) {
  if (!canEdit(backend)) return;
  const target = event.target as HTMLElement | null;
  if (target?.closest(".backend-card__actions")) return;
  openEdit(backend);
}

async function load() {
  loading.value = true;
  try {
    const [list, types] = await Promise.all([listStorageBackends(), listStorageBackendTypes()]);
    backends.value = list.data || [];
    defaultID.value = list.default_storage_backend_id || "";
    providers.value = types.data || [];
  } finally {
    loading.value = false;
  }
}
function resetConfig() {
  form.config = blankConfig();
  rawTestResult.value = null;
}
function openCreate() {
  editing.value = null;
  form.name = "";
  form.provider = providers.value[0] || "local";
  form.config = blankConfig();
  rawTestResult.value = null;
  visible.value = true;
}
function openEdit(backend: StorageBackend) {
  editing.value = backend;
  form.name = backend.name;
  form.provider = backend.provider;
  form.config = { ...blankConfig(), ...backend.config };
  rawTestResult.value = null;
  visible.value = true;
}
async function testRaw() {
  testing.value = true;
  rawTestResult.value = null;
  try {
    const r: any = editing.value ? await testStorageBackendByID(editing.value.id) : await testStorageBackend(form);
    if (r.success) {
      rawTestResult.value = "ok";
      MessagePlugin.success(t("settings.storageBackend.testSuccess"));
    } else {
      rawTestResult.value = "error";
      MessagePlugin.error(r.error || t("settings.storageBackend.testFailed"));
    }
  } finally {
    testing.value = false;
  }
}
async function testSaved(backend: StorageBackend) {
  const r: any = await testStorageBackendByID(backend.id);
  if (r.success) {
    MessagePlugin.success(t("settings.storageBackend.testSuccess"));
  } else {
    MessagePlugin.error(r.error || t("settings.storageBackend.testFailed"));
  }
}
async function save() {
  if (!form.name.trim()) {
    MessagePlugin.warning(t("settings.storageBackend.nameRequired"));
    return;
  }
  saving.value = true;
  try {
    const payload = { name: form.name.trim(), provider: form.provider, config: { ...form.config } };
    if (editing.value) await updateStorageBackend(editing.value.id, payload);
    else await createStorageBackend(payload);
    MessagePlugin.success(t("settings.storageBackend.saveSuccess"));
    visible.value = false;
    await load();
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("settings.storageBackend.saveFailed"));
  } finally {
    saving.value = false;
  }
}
async function makeDefault(backend: StorageBackend) {
  await setDefaultStorageBackend(backend.id);
  defaultID.value = backend.id;
  MessagePlugin.success(t("settings.storageBackend.defaultUpdated"));
}

const deleteDialogVisible = ref(false);
const deleteDialogPending = ref(false);
const deleteTarget = ref<StorageBackend | null>(null);

function remove(backend: StorageBackend) {
  deleteTarget.value = backend;
  deleteDialogPending.value = false;
  deleteDialogVisible.value = true;
}

async function doDelete() {
  const backend = deleteTarget.value;
  if (!backend || deleteDialogPending.value) return;
  deleteDialogPending.value = true;
  try {
    await deleteStorageBackend(backend.id);
    deleteDialogVisible.value = false;
    await load();
    MessagePlugin.success(t("settings.storageBackend.deleted"));
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("settings.storageBackend.deleteFailed"));
  } finally {
    deleteDialogPending.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
/*
 * Only what utilities cannot express:
 *  - mono provider logos are CSS-masked icons driven by a per-card --logo-url;
 *  - the card action button fades in on card hover/focus (group-* could do it,
 *    but the drawer-teleported markup keeps these three tiny rules clearer);
 *  - the per-provider badge colours (8 providers), mirrored onto the
 *    teleported drawer header by the non-scoped block below.
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

.backend-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}

.backend-card--local .backend-card__badge {
  background: rgba(70, 70, 70, 0.1);
  color: #464646;
}
.backend-card--minio .backend-card__badge {
  background: rgba(225, 38, 38, 0.12);
  color: #c0382b;
}
.backend-card--cos .backend-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.backend-card--tos .backend-card__badge {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.backend-card--s3 .backend-card__badge {
  background: rgba(255, 153, 0, 0.12);
  color: #d97706;
}
.backend-card--oss .backend-card__badge {
  background: rgba(255, 90, 0, 0.12);
  color: #e55a00;
}
.backend-card--ks3 .backend-card__badge {
  background: rgba(7, 192, 95, 0.12);
  color: #07a050;
}
.backend-card--obs .backend-card__badge {
  background: rgba(206, 17, 38, 0.1);
  color: #ce1126;
}

/* Logo badges sit on a white tile; this must outrank the provider tints above. */
.backend-card .backend-card__badge--logo {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.backend-card .backend-card__badge--mono::before {
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

.backend-card:hover .backend-card__action-btn,
.backend-card:focus-within .backend-card__action-btn {
  opacity: 1;
}
</style>

<!--
  Non-scoped: drawer header icon coloring per provider, mirroring the list
  card badge colors. Namespaced under .storage-backend-drawer--{id}. The
  drawer teleports to <body>, so the hook has to live outside the scoped
  styles; the colours match the card badges above.
-->
<style>
.storage-backend-drawer .setting-drawer__header-icon:has(.header-icon__img) {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.storage-backend-drawer--local .setting-drawer__header-icon {
  background: rgba(70, 70, 70, 0.1);
  color: #464646;
}
.storage-backend-drawer--minio .setting-drawer__header-icon {
  background: rgba(225, 38, 38, 0.12);
  color: #c0382b;
}
.storage-backend-drawer--cos .setting-drawer__header-icon {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.storage-backend-drawer--tos .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.storage-backend-drawer--s3 .setting-drawer__header-icon {
  background: rgba(255, 153, 0, 0.12);
  color: #d97706;
}
.storage-backend-drawer--oss .setting-drawer__header-icon {
  background: rgba(255, 90, 0, 0.12);
  color: #e55a00;
}
.storage-backend-drawer--ks3 .setting-drawer__header-icon {
  background: rgba(7, 192, 95, 0.12);
  color: #07a050;
}
.storage-backend-drawer--obs .setting-drawer__header-icon {
  background: rgba(206, 17, 38, 0.1);
  color: #ce1126;
}
</style>
