<template>
  <div class="w-full">
    <div class="mb-7">
      <div class="flex items-center justify-between gap-5">
        <div>
          <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("modelSettings.title") }}</h2>
          <p class="text-muted-foreground m-0 text-sm leading-[1.6]">{{ $t("modelSettings.description") }}</p>
        </div>
        <!-- The old text button: brand colour, no hover background and no
             underline; only the colour deepens on hover and press. -->
        <Button
          v-if="authStore.hasRole('admin')"
          type="button"
          variant="link"
          class="shrink-0 px-0 font-semibold hover:text-[var(--td-brand-color-hover)] hover:no-underline active:text-[var(--td-brand-color-active)]"
          @click="showDebugDrawer = true"
        >
          <PlayCircleIcon />
          {{ $t("modelSettings.actions.debugModel") }}
        </Button>
      </div>

      <div class="bg-secondary border-border mt-3 rounded-md border px-3 py-2.5" role="note">
        <p class="text-placeholder m-0 mb-1 text-xs font-medium tracking-[0.02em]">
          {{ $t("modelSettings.builtinModels.title") }}
        </p>
        <p class="text-muted-foreground m-0 mb-1.5 text-[13px] leading-[1.55]">
          {{
            $t(
              authStore.isSystemAdmin
                ? "modelSettings.builtinModels.descriptionAdmin"
                : "modelSettings.builtinModels.description",
            )
          }}
        </p>
        <a
          v-if="docsUrl('03-features/06-models#内置模型')"
          class="text-primary inline-flex items-center gap-0.5 text-[13px] hover:underline"
          :href="docsUrl('03-features/06-models#内置模型')"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $t("modelSettings.builtinModels.viewGuide") }}
          <LinkIcon class="size-3.5" />
        </a>
      </div>
    </div>

    <!-- TDesign's line tabs: a full-width bottom rule, the active tab in the
         brand colour with a brand underline, scrolling sideways (scrollbar
         hidden) when the counts make the row too wide. -->
    <Tabs v-model="activeTypeFilter" class="mb-4" data-guide="settings-models">
      <TabsList
        variant="line"
        class="border-border h-auto w-full [scrollbar-width:none] justify-start gap-0 overflow-x-auto overflow-y-hidden rounded-none border-b p-0 [&::-webkit-scrollbar]:hidden"
      >
        <TabsTrigger
          v-for="tab in typeTabs"
          :key="tab.value"
          :value="tab.value"
          class="data-active:text-primary hover:text-primary after:bg-primary h-10 flex-none px-3 text-[13px] font-normal group-data-horizontal/tabs:after:bottom-0"
        >
          {{ tab.label }}
        </TabsTrigger>
      </TabsList>
    </Tabs>

    <div class="min-h-[120px]">
      <div v-if="!loading && filteredModels.length === 0 && !authStore.hasRole('admin')" class="py-16 text-center">
        <Empty>
          <EmptyDescription class="text-placeholder mb-4 text-sm">{{ emptyHint }}</EmptyDescription>
        </Empty>
      </div>
      <div v-else-if="!loading" class="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
        <div
          v-for="model in filteredModels"
          :key="`${model._modelType}-${model.id}`"
          class="model-card border-border relative flex min-w-0 items-start gap-3 rounded-[10px] border px-4 py-3.5 transition-[border-color,box-shadow,transform] duration-[180ms] ease-out"
          :class="[
            `model-card--${model._modelType}`,
            // Built-in cards are muted and do not lift on hover — unless the
            // viewer may edit them, where the clickable hover wins, as it did.
            model.isBuiltin && !isModelCardClickable(model)
              ? 'bg-secondary'
              : model.isBuiltin
                ? 'bg-secondary hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]'
                : 'bg-card hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]',
            {
              'focus-visible:outline-primary cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2':
                isModelCardClickable(model),
            },
          ]"
          :role="isModelCardClickable(model) ? 'button' : undefined"
          :tabindex="isModelCardClickable(model) ? 0 : undefined"
          @click="onModelCardClick($event, model._modelType, model)"
          @keydown.enter="onModelCardClick($event, model._modelType, model)"
        >
          <!-- Tinted per model type by the scoped rules below. -->
          <div
            class="model-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px]"
            :aria-label="typeLabel(model._modelType)"
          >
            <component :is="typeIcon(model._modelType)" class="size-[18px]" />
          </div>
          <div class="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
            <div class="flex min-w-0 items-center gap-1.5">
              <h3 class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold">
                {{ modelDisplayName(model) }}
              </h3>
              <span
                v-if="model.isBuiltin"
                class="text-placeholder model-card__lock inline-flex size-[18px] shrink-0 items-center justify-center opacity-60 transition-[color,opacity] duration-150"
                :title="$t('modelSettings.builtinTag')"
                :aria-label="$t('modelSettings.builtinTag')"
              >
                <component :is="authStore.isSystemAdmin ? PencilIcon : LockIcon" class="size-[13px]" />
              </span>
              <!-- `model-card__actions` is a hook: onModelCardClick ignores keys pressed inside it. -->
              <div
                v-if="canManageModel(model)"
                class="model-card__actions flex shrink-0 items-center gap-0.5"
                @click.stop
              >
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      class="model-card__action-btn text-placeholder hover:text-foreground focus-visible:text-foreground shrink-0 p-0.5 opacity-0 transition-opacity duration-150 hover:bg-[var(--td-bg-color-secondarycontainer)] focus-visible:bg-[var(--td-bg-color-secondarycontainer)]"
                      :aria-label="$t('docs.tree.moreActions')"
                    >
                      <EllipsisIcon />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      v-for="opt in getModelOptions(model._modelType, model)"
                      :key="opt.value"
                      @select="handleMenuAction({ value: opt.value }, model._modelType, model)"
                    >
                      {{ opt.content }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                <Popover
                  v-if="canDeleteModel(model)"
                  :open="deleteConfirmId === `${model._modelType}-${model.id}`"
                  @update:open="(v: boolean) => (deleteConfirmId = v ? `${model._modelType}-${model.id}` : null)"
                >
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <PopoverTrigger as-child>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          class="model-card__action-btn text-destructive shrink-0 p-0.5 opacity-0 transition-opacity duration-150"
                          :aria-label="$t('common.delete')"
                          @click.stop
                        >
                          <Trash2Icon />
                        </Button>
                      </PopoverTrigger>
                    </TooltipTrigger>
                    <TooltipContent>{{ $t("common.delete") }}</TooltipContent>
                  </Tooltip>
                  <PopoverContent align="end" class="w-64">
                    <p class="text-foreground m-0 mb-3 text-sm">
                      {{ $t("modelSettings.confirmDelete", { name: modelDisplayName(model) }) }}
                    </p>
                    <div class="flex justify-end gap-2">
                      <Button variant="outline" size="sm" @click="deleteConfirmId = null">
                        {{ $t("common.cancel") }}
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        class="bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive"
                        @click="
                          deleteConfirmId = null;
                          deleteModel(model._modelType, model.id);
                        "
                      >
                        {{ $t("common.delete") }}
                      </Button>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>
            </div>
            <p class="text-muted-foreground m-0 mt-0.5 truncate text-xs leading-normal">
              <span>{{ vendorLabel(model) }}</span>
              <template v-if="model._modelType === 'embedding' && model.dimension">
                <span class="text-placeholder mx-1">·</span>
                <span>{{ $t("model.editor.dimensionLabel") }} {{ model.dimension }}</span>
              </template>
              <template v-if="model._modelType === 'chat' && model.supportsVision">
                <span class="text-placeholder mx-1">·</span>
                <span
                  class="inline-flex items-center gap-0.75"
                  :title="$t('model.editor.supportsVisionLabel')"
                  :aria-label="$t('model.editor.supportsVisionLabel')"
                >
                  <ImageIcon class="size-3" />
                </span>
              </template>
            </p>
          </div>
        </div>
        <button
          v-if="authStore.hasRole('admin')"
          type="button"
          data-slot="add-model-card"
          class="border-border text-placeholder hover:border-primary hover:text-primary hover:bg-primary/6 focus-visible:border-primary focus-visible:text-primary focus-visible:bg-primary/6 focus-visible:outline-primary flex h-full min-h-[68px] w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed bg-transparent text-center transition-all duration-[180ms] ease-out focus-visible:outline-2 focus-visible:outline-offset-2"
          data-guide="settings-add-model"
          @click="openAddDialog"
        >
          <span
            class="bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg"
            aria-hidden="true"
          >
            <PlusIcon class="size-[18px]" />
          </span>
          <span class="text-[13px] leading-[1.4] font-medium">{{ $t("modelSettings.actions.addModel") }}</span>
        </button>
      </div>
      <div v-else class="flex justify-center py-10">
        <Loader2Icon class="animate-spin" />
      </div>
    </div>

    <!-- 模型编辑器抽屉 -->
    <ModelEditorDialog
      v-model:visible="showDialog"
      :model-type="currentModelType"
      :model-data="editingModel"
      @confirm="handleModelSave"
    />
    <ModelDebugDrawer v-model:visible="showDebugDrawer" :models="allModels" />

    <!-- 平台共享 / 取消共享确认 -->
    <Dialog v-model:open="sharingDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{
            sharingDialog.shared ? $t("modelSettings.sharing.shareAction") : $t("modelSettings.sharing.unshareAction")
          }}</DialogTitle>
          <DialogDescription class="whitespace-pre-line">{{ sharingDialog.body }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ $t("common.cancel") }}</Button>
          </DialogClose>
          <Button
            :variant="sharingDialog.shared ? 'default' : 'destructive'"
            :class="
              sharingDialog.shared
                ? ''
                : 'bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive'
            "
            :disabled="sharingDialog.pending"
            @click="doApplySharing"
          >
            <Loader2Icon v-if="sharingDialog.pending" class="animate-spin" />
            {{ $t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { docsUrl } from "@/config/externalLinks";
import { ref, computed, onMounted, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import ModelEditorDialog from "@/components/ModelEditorDialog.vue";
import ModelDebugDrawer from "@/components/ModelDebugDrawer.vue";
import {
  listModels,
  createModel,
  updateModel as updateModelAPI,
  deleteModel as deleteModelAPI,
  setModelSharing,
  type ModelConfig,
} from "@/api/model";
import { useAuthStore } from "@/stores/auth";
import { useUIStore } from "@/stores/ui";

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
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  BubblesIcon,
  MessageSquareIcon,
  EllipsisIcon,
  FilterIcon,
  ImageIcon,
  LinkIcon,
  Loader2Icon,
  LockIcon,
  PencilIcon,
  PlayCircleIcon,
  PlusIcon,
  Trash2Icon,
  Volume2Icon,
  type LucideIcon,
} from "@lucide/vue";

const { t, te } = useI18n();
const authStore = useAuthStore();
const uiStore = useUIStore();
type ModelType = "chat" | "embedding" | "rerank" | "vllm" | "asr";
type FilterType = "all" | ModelType;

const showDialog = ref(false);
const showDebugDrawer = ref(false);
const currentModelType = ref<ModelType>("chat");
const editingModel = ref<any>(null);
const loading = ref(true);
const activeTypeFilter = ref<FilterType>("all");

const MODEL_TAB_TYPES: FilterType[] = ["chat", "embedding", "rerank", "vllm", "asr"];
watch(
  () => uiStore.settingsInitialSubSection,
  (sub) => {
    if (sub && MODEL_TAB_TYPES.includes(sub as FilterType)) {
      activeTypeFilter.value = sub as FilterType;
    }
  },
  { immediate: true },
);

// 模型列表数据
const allModels = ref<ModelConfig[]>([]);

// 后端 type → 前端分组 type 的映射
const backendTypeToModelType: Record<string, ModelType> = {
  KnowledgeQA: "chat",
  Embedding: "embedding",
  Rerank: "rerank",
  VLLM: "vllm",
  ASR: "asr",
};

// 将后端模型格式转换为旧的前端格式（附带 _modelType 便于渲染）
// apiKey is always blank here: the server's main GET response does not
// include it (see internal/handler/dto/model.go — ModelParametersDTO omits
// secret fields). Credential read/write happens inside the editor dialog
// via the dedicated /credentials subresource.
function convertToLegacyFormat(model: ModelConfig) {
  return {
    id: model.id!,
    name: model.name,
    displayName: model.display_name || "",
    source: model.source,
    modelName: model.name,
    baseUrl: model.parameters.base_url || "",
    apiKey: "",
    provider: model.parameters.provider || "",
    dimension: model.parameters.embedding_parameters?.dimension,
    supportsDimensionOverride: model.parameters.embedding_parameters?.supports_dimension_override || false,
    isBuiltin: model.is_builtin || false,
    supportsVision: model.parameters.supports_vision || false,
    maxConcurrency: model.parameters.max_concurrency,
    customHeaders: model.parameters.custom_headers
      ? Object.entries(model.parameters.custom_headers).map(([key, value]) => ({ key, value: String(value) }))
      : [],
    lkeapRegion: model.parameters.extra_config?.region || "ap-guangzhou",
    // 原始存库值，编辑弹窗内再 resolve（避免打开时被推断值覆盖）
    thinkingControl: model.parameters.extra_config?.thinking_control,
    _modelType: backendTypeToModelType[model.type] || ("chat" as ModelType),
    // Preserve the credential metadata map so the editor dialog can render
    // the "Configured" state without an extra round-trip.
    credentials: model.credentials,
  };
}

// 平铺 + 过滤
const allLegacyModels = computed(() => allModels.value.map(convertToLegacyFormat));
const filteredModels = computed(() => {
  if (activeTypeFilter.value === "all") return allLegacyModels.value;
  return allLegacyModels.value.filter((m) => m._modelType === activeTypeFilter.value);
});

const countByType = (type: ModelType) => allLegacyModels.value.filter((m) => m._modelType === type).length;

const typeTabs = computed(() => [
  { value: "all" as FilterType, label: `${t("common.all")}(${allLegacyModels.value.length})` },
  { value: "chat" as FilterType, label: `${t("modelSettings.typeShort.chat")}(${countByType("chat")})` },
  {
    value: "embedding" as FilterType,
    label: `${t("modelSettings.typeShort.embedding")}(${countByType("embedding")})`,
  },
  { value: "rerank" as FilterType, label: `${t("modelSettings.typeShort.rerank")}(${countByType("rerank")})` },
  { value: "vllm" as FilterType, label: `${t("modelSettings.typeShort.vllm")}(${countByType("vllm")})` },
  { value: "asr" as FilterType, label: `${t("modelSettings.typeShort.asr")}(${countByType("asr")})` },
]);

// 类型徽章图标。
const typeIcon = (type: ModelType): LucideIcon => {
  const map: Record<ModelType, LucideIcon> = {
    chat: MessageSquareIcon,
    embedding: BubblesIcon,
    rerank: FilterIcon,
    vllm: ImageIcon,
    asr: Volume2Icon,
  };
  return map[type];
};

const typeLabel = (type: ModelType) => {
  const map: Record<ModelType, string> = {
    chat: t("modelSettings.typeShort.chat"),
    embedding: t("modelSettings.typeShort.embedding"),
    rerank: t("modelSettings.typeShort.rerank"),
    vllm: t("modelSettings.typeShort.vllm"),
    asr: t("modelSettings.typeShort.asr"),
  };
  return map[type];
};

const sourceLabel = (type: ModelType) => {
  // vllm / asr 的 remote 文案特殊，其余走通用 remote 文案
  if (type === "vllm" || type === "asr") {
    return t("modelSettings.source.openaiCompatible");
  }
  return t("modelSettings.source.remote");
};

// Maps a backend `provider` id (e.g. "openai", "aliyun")
// to its localized short label. Reuses the same i18n keys the editor's
// provider dropdown uses, so the model card and the editor stay in sync
// when a provider is renamed. Falls back to '' when the backend didn't
// store a provider — caller falls back to sourceLabel().
const providerLabel = (model: any): string => {
  const id = model.provider;
  if (!id) return "";
  const key = `model.editor.providers.${id}.label`;
  return te(key) ? t(key) : id;
};

// What the vendor chip on a card shows. Keeps the chip text uniformly
// short so cards line up:
//   local  → "Ollama"
//   remote → provider's localized short name (e.g. "腾讯云 LKEAP",
//            "阿里云 DashScope"). For the catch-all "generic" provider
//            we render a single short word ("自定义" / "Custom") — the
//            editor dropdown's longer "自定义 (OpenAI兼容接口)" label
//            blows out the card chip row, and the "OpenAI 兼容" framing
//            isn't meaningful to most end users (they didn't pick "I
//            want OpenAI compatibility", they just pasted a base URL).
const vendorLabel = (model: any): string => {
  if (model.source === "local") return "Ollama";
  if (model.provider === "generic") {
    return t("modelSettings.source.custom");
  }
  return providerLabel(model) || sourceLabel(model._modelType);
};

const modelDisplayName = (model: any) => {
  const displayName = typeof model.displayName === "string" ? model.displayName.trim() : "";
  return displayName || model.name;
};

const emptyHint = computed(() => {
  if (activeTypeFilter.value === "all") return t("modelSettings.chat.empty");
  const map: Record<ModelType, string> = {
    chat: t("modelSettings.chat.empty"),
    embedding: t("modelSettings.embedding.empty"),
    rerank: t("modelSettings.rerank.empty"),
    vllm: t("modelSettings.vllm.empty"),
    asr: t("modelSettings.asr.empty"),
  };
  return map[activeTypeFilter.value as ModelType];
});

// 加载模型列表
const loadModels = async () => {
  loading.value = true;
  try {
    const models = await listModels();
    allModels.value = models;
  } catch (error: any) {
    console.error("加载模型列表失败:", error);
    MessagePlugin.error(error.message);
  } finally {
    loading.value = false;
  }
};

// 打开添加对话框；类型在抽屉内选择，此处仅按当前 Tab 预填默认值
const openAddDialog = () => {
  currentModelType.value = activeTypeFilter.value === "all" ? "chat" : activeTypeFilter.value;
  editingModel.value = null;
  showDialog.value = true;
};

// Tenant Admin+ manages tenant models; only SystemAdmin manages shared
// built-in models. The backend repeats this distinction authoritatively.
const canEditModel = (model: any) => (model.isBuiltin ? authStore.isSystemAdmin : authStore.hasRole("admin"));

const isModelCardClickable = (model: any) => canEditModel(model);

const canManageModel = (model: any) => canEditModel(model);

// 普通模型：空间 Admin+ 可删。内置模型：仅系统管理员，且后端还会额外拒绝
// YAML 托管的行（删了也会在下次启动被 reconciler 重新写回）以及仍被任意空间
// 引用的模型。
const canDeleteModel = (model: any) => (model.isBuiltin ? authStore.isSystemAdmin : authStore.hasRole("admin"));

// 哪张卡片的删除确认 popover 开着。
const deleteConfirmId = ref<string | null>(null);

const onModelCardClick = (event: Event, type: ModelType, model: any) => {
  if (!isModelCardClickable(model)) return;
  if (event.type === "keydown") {
    const ke = event as KeyboardEvent;
    if (ke.key !== "Enter" && ke.key !== " ") return;
    ke.preventDefault();
  }
  const target = event.target as HTMLElement | null;
  if (target?.closest(".model-card__actions")) return;
  editModel(type, model);
};

// 编辑模型
const editModel = (type: ModelType, model: any) => {
  if (model.isBuiltin && !authStore.isSystemAdmin) {
    MessagePlugin.warning(t("modelSettings.toasts.builtinCannotEdit"));
    return;
  }
  if (!model.isBuiltin && !authStore.hasRole("admin")) {
    return;
  }
  currentModelType.value = type;
  editingModel.value = { ...model };
  showDialog.value = true;
};

// 保存模型
const handleModelSave = async (modelData: any) => {
  const saveType: ModelType = modelData.modelType ?? currentModelType.value;
  currentModelType.value = saveType;

  try {
    if (!modelData.modelName || !modelData.modelName.trim()) {
      MessagePlugin.warning(t("modelSettings.toasts.nameRequired"));
      return;
    }

    if (modelData.modelName.trim().length > 100) {
      MessagePlugin.warning(t("modelSettings.toasts.nameTooLong"));
      return;
    }

    if (modelData.displayName && modelData.displayName.trim().length > 100) {
      MessagePlugin.warning(t("modelSettings.toasts.displayNameTooLong"));
      return;
    }

    if (modelData.source === "remote") {
      if (!modelData.baseUrl || !modelData.baseUrl.trim()) {
        MessagePlugin.warning(t("modelSettings.toasts.baseUrlRequired"));
        return;
      }

      try {
        new URL(modelData.baseUrl.trim());
      } catch {
        MessagePlugin.warning(t("modelSettings.toasts.baseUrlInvalid"));
        return;
      }
    }

    if (saveType === "embedding") {
      if (!modelData.dimension || modelData.dimension < 128 || modelData.dimension > 4096) {
        MessagePlugin.warning(t("modelSettings.toasts.dimensionInvalid"));
        return;
      }
    }

    const customHeadersMap: Record<string, string> = {};
    if (Array.isArray(modelData.customHeaders)) {
      for (const item of modelData.customHeaders) {
        const key = (item?.key ?? "").trim();
        const value = (item?.value ?? "").trim();
        if (key && value) {
          customHeadersMap[key] = value;
        }
      }
    }

    // api_key flows in only on initial create (modelData.apiKey is wiped on
    // every edit-mode open). Edits to existing models commit credentials via
    // the /credentials subresource (handled inside ModelEditorDialog).
    const trimmedApiKey = (modelData.apiKey ?? "").trim();
    const apiKeyFields: { api_key?: string } = !editingModel.value && trimmedApiKey ? { api_key: trimmedApiKey } : {};
    const trimmedAppSecret = (modelData.appSecret ?? "").trim();
    const appSecretFields: { app_secret?: string } =
      !editingModel.value && trimmedAppSecret ? { app_secret: trimmedAppSecret } : {};
    const extraConfig: Record<string, string> = {};
    if (modelData.provider === "lkeap" && saveType === "rerank") {
      extraConfig.region = (modelData.lkeapRegion || "ap-guangzhou").trim();
    }
    if (saveType === "chat" && modelData.source === "remote" && modelData.thinkingControl) {
      extraConfig.thinking_control = modelData.thinkingControl;
    }
    const extraConfigFields = Object.keys(extraConfig).length > 0 ? { extra_config: extraConfig } : {};

    const apiModelData: ModelConfig = {
      name: modelData.modelName.trim(),
      display_name: modelData.displayName?.trim() || "",
      type: getModelType(saveType),
      source: modelData.source,
      description: "",
      parameters: {
        base_url: modelData.baseUrl?.trim() || "",
        ...apiKeyFields,
        ...appSecretFields,
        provider: modelData.provider || "",
        ...extraConfigFields,
        ...(Object.keys(customHeadersMap).length > 0 ? { custom_headers: customHeadersMap } : {}),
        ...(saveType === "embedding" && modelData.dimension
          ? {
              embedding_parameters: {
                dimension: modelData.dimension,
                truncate_prompt_tokens: 0,
                supports_dimension_override: modelData.supportsDimensionOverride ?? false,
              },
            }
          : {}),
        ...(saveType === "vllm"
          ? {
              supports_vision: true,
            }
          : saveType === "chat"
            ? {
                supports_vision: modelData.supportsVision ?? false,
              }
            : {}),
        // 后台并发上限：仅 chat/embedding/vllm 受治理，>0 才写入（0/空沿用全局默认）。
        ...(["chat", "embedding", "vllm"].includes(saveType) && Number(modelData.maxConcurrency) > 0
          ? { max_concurrency: Number(modelData.maxConcurrency) }
          : {}),
      },
    };

    if (editingModel.value && editingModel.value.id) {
      await updateModelAPI(editingModel.value.id, apiModelData);
      MessagePlugin.success(t("modelSettings.toasts.updated"));
    } else {
      await createModel(apiModelData);
      MessagePlugin.success(t("modelSettings.toasts.added"));
    }

    showDialog.value = false;
    await loadModels();
  } catch (error: any) {
    console.error("保存模型失败:", error);
    MessagePlugin.error(error.message || t("modelSettings.toasts.saveFailed"));
  }
};

// 删除模型
const deleteModel = async (_type: ModelType, modelId: string) => {
  const model = allModels.value.find((m) => m.id === modelId);
  if (model?.is_builtin) {
    MessagePlugin.warning(t("modelSettings.toasts.builtinCannotDelete"));
    return;
  }

  try {
    await deleteModelAPI(modelId);
    MessagePlugin.success(t("modelSettings.toasts.deleted"));
    await loadModels();
  } catch (error: any) {
    console.error("删除模型失败:", error);
    MessagePlugin.error(error.message || t("modelSettings.toasts.deleteFailed"));
  }
};

// 获取模型操作菜单选项
const getModelOptions = (type: ModelType, model: any) => {
  const options: any[] = [];

  if (model.isBuiltin) {
    if (authStore.isSystemAdmin) {
      options.push({
        content: t("common.edit"),
        value: `edit-${type}-${model.id}`,
      });
      options.push({
        content: t("modelSettings.sharing.unshareAction"),
        value: `unshare-${type}-${model.id}`,
      });
    }
    return options;
  }

  // Models are tenant-wide infrastructure (LLM credentials); the
  // backend gates every mutation behind Admin+ (see RegisterModelRoutes).
  // Non-Admins get an empty action menu — viewing is fine, but editing,
  // copying (also goes through createModel), and deleting are not.
  if (!authStore.hasRole("admin")) {
    return options;
  }

  options.push({
    content: t("common.edit"),
    value: `edit-${type}-${model.id}`,
  });

  options.push({
    content: t("common.copy"),
    value: `copy-${type}-${model.id}`,
  });

  // 平台共享是平台级决策：共享后模型对所有空间可见可用，但凭据和 Base URL 由
  // DTO 层对非系统管理员抹掉。空间角色表达不了这件事（每个用户都是自己空间的
  // Owner），所以只认 isSystemAdmin。
  if (authStore.isSystemAdmin) {
    options.push({
      content: t("modelSettings.sharing.shareAction"),
      value: `share-${type}-${model.id}`,
    });
  }

  return options;
};

// 处理菜单操作
const handleMenuAction = (data: { value: string }, type: ModelType, model: any) => {
  const value = data.value;

  if (value.indexOf("edit-") === 0) {
    editModel(type, model);
  } else if (value.indexOf("copy-") === 0) {
    copyModel(type, model.id);
  } else if (value.indexOf("share-") === 0) {
    confirmModelSharing(model, true);
  } else if (value.indexOf("unshare-") === 0) {
    confirmModelSharing(model, false);
  }
};

// 切换平台共享。Embedding 单独提示：改动它意味着已建知识库的向量全部失效，
// 需要重建索引，比其他类型危险得多。
const sharingDialogVisible = ref(false);
const sharingDialog = ref<{ model: any; shared: boolean; body: string; pending: boolean }>({
  model: null,
  shared: false,
  body: "",
  pending: false,
});

const confirmModelSharing = (model: any, shared: boolean) => {
  const name = modelDisplayName(model);
  const body = shared
    ? t("modelSettings.sharing.confirmShare", { name })
    : t("modelSettings.sharing.confirmUnshare", { name });
  const embeddingWarning =
    model._modelType === "embedding"
      ? `

${t("modelSettings.sharing.embeddingWarning")}`
      : "";

  sharingDialog.value = { model, shared, body: `${body}${embeddingWarning}`, pending: false };
  sharingDialogVisible.value = true;
};

const doApplySharing = async () => {
  const { model, shared } = sharingDialog.value;
  if (!model || sharingDialog.value.pending) return;
  sharingDialog.value.pending = true;
  try {
    await applyModelSharing(model, shared);
    sharingDialogVisible.value = false;
  } finally {
    sharingDialog.value.pending = false;
  }
};

const applyModelSharing = async (model: any, shared: boolean) => {
  try {
    await setModelSharing(model.id, shared);
    MessagePlugin.success(shared ? t("modelSettings.sharing.sharedToast") : t("modelSettings.sharing.unsharedToast"));
    await loadModels();
  } catch (error: any) {
    // 取消共享时后端会拒绝仍被任意空间引用的模型，错误文案里带着引用数量 ——
    // 直接透传，比一句泛化的失败提示有用得多。
    console.error("切换模型共享状态失败:", error);
    MessagePlugin.error(error?.message || t("modelSettings.sharing.failedToast"));
  }
};

// 生成不重复的复制名称
const generateCopyName = (originalName: string): string => {
  const suffix = t("modelSettings.copySuffix");
  const existingNames = new Set(allModels.value.map((m) => m.name));
  let candidate = `${originalName}${suffix}`;
  let counter = 2;
  while (existingNames.has(candidate)) {
    candidate = `${originalName}${suffix} ${counter}`;
    counter += 1;
  }
  return candidate;
};

// 复制模型
const copyModel = async (_type: ModelType, modelId: string) => {
  const source = allModels.value.find((m) => m.id === modelId);
  if (!source) {
    return;
  }
  if (source.is_builtin) {
    MessagePlugin.warning(t("modelSettings.toasts.builtinCannotCopy"));
    return;
  }

  try {
    const newModel: ModelConfig = {
      name: generateCopyName(source.name),
      display_name: source.display_name || "",
      type: source.type,
      source: source.source,
      description: source.description || "",
      parameters: JSON.parse(JSON.stringify(source.parameters || {})),
    };

    await createModel(newModel);
    MessagePlugin.success(t("modelSettings.toasts.copied"));
    await loadModels();
  } catch (error: any) {
    console.error("复制模型失败:", error);
    MessagePlugin.error(error.message || t("modelSettings.toasts.copyFailed"));
  }
};

// 获取后端模型类型
function getModelType(type: ModelType): "KnowledgeQA" | "Embedding" | "Rerank" | "VLLM" | "ASR" {
  const typeMap = {
    chat: "KnowledgeQA" as const,
    embedding: "Embedding" as const,
    rerank: "Rerank" as const,
    vllm: "VLLM" as const,
    asr: "ASR" as const,
  };
  return typeMap[type];
}

onMounted(() => {
  loadModels();
});
</script>

<style scoped>
/*
 * Kept as CSS because selectors reach into generated markup or repeat
 * per-type colours that are clearer as rules than as long utility strings:
 *  - the per-type badge tints (5 model types);
 *  - the built-in lock icon lighting up on card hover;
 *  - the action buttons fading in on hover / keyboard focus.
 */
.model-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}

.model-card--chat .model-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}

.model-card--embedding .model-card__badge {
  background: rgba(98, 53, 187, 0.1);
  color: #6235bb;
}

.model-card--rerank .model-card__badge {
  background: rgba(184, 92, 0, 0.1);
  color: #b85c00;
}

.model-card--vllm .model-card__badge {
  background: rgba(201, 62, 62, 0.1);
  color: #c93e3e;
}

.model-card--asr .model-card__badge {
  background: rgba(17, 128, 83, 0.1);
  color: #118053;
}

.model-card:hover .model-card__lock {
  opacity: 1;
  color: var(--td-text-color-secondary);
}

.model-card:hover .model-card__action-btn,
.model-card:focus-within .model-card__action-btn,
.model-card__actions:focus-within .model-card__action-btn {
  opacity: 1;
}
</style>
