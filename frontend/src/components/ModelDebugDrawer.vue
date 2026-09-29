<template>
  <SettingDrawer
    v-model:visible="drawerVisible"
    :title="$t('modelSettings.debug.title')"
    :description="$t('modelSettings.debug.description')"
    :icon="CirclePlayIcon"
    width="560px"
    :min-width="480"
    :max-width="900"
    storage-key="setting-drawer:width:model-debug"
    :confirm-text="$t('modelSettings.debug.run')"
    :confirm-loading="running"
    :confirm-disabled="!canRun"
    :cancel-text="$t('common.close')"
    @confirm="runDebug"
  >
    <template v-if="result" #footer-left>
      <Button variant="outline" @click="copyResult">
        <CopyIcon />
        {{ $t("modelSettings.debug.copyResult") }}
      </Button>
    </template>

    <div class="model-debug">
      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t("modelSettings.debug.groupModel") }}</h4>
        <div v-if="availableModelTypes.length > 1">
          <div class="flex flex-wrap gap-2" role="radiogroup" :aria-label="$t('modelSettings.debug.modelType')">
            <button
              v-for="option in availableModelTypes"
              :key="option.value"
              type="button"
              data-slot="model-type-option"
              class="focus-visible:outline-primary inline-flex min-h-8 items-center gap-1.5 rounded-lg border px-3 py-1.5 text-[13px] leading-[1.4] transition-[border-color,color,background-color] duration-150 focus-visible:outline-2 focus-visible:outline-offset-2"
              :class="
                selectedModelType === option.value
                  ? 'border-primary bg-primary/10 text-primary font-medium'
                  : 'border-border bg-card text-muted-foreground hover:text-foreground hover:border-[var(--td-brand-color-3,var(--td-brand-color))]'
              "
              role="radio"
              :aria-checked="selectedModelType === option.value"
              @click="selectModelType(option.value)"
            >
              <component :is="option.icon" class="size-[15px] shrink-0" />
              <span class="whitespace-nowrap">{{ option.label }}</span>
            </button>
          </div>
        </div>
        <div>
          <label :class="labelClass">{{ $t("modelSettings.debug.model") }}</label>
          <SearchableSelect
            :model-value="selectedModelId"
            :options="modelOptions"
            :placeholder="$t('modelSettings.debug.modelPlaceholder')"
            :disabled="filteredModels.length === 0"
            @update:model-value="onModelPicked"
          >
            <template #option="{ option }">
              <span class="flex min-w-0 flex-1 items-center justify-between gap-3">
                <span class="min-w-0 truncate">{{ option.label }}</span>
                <span class="text-placeholder shrink-0 text-xs">{{ option.meta }}</span>
              </span>
            </template>
          </SearchableSelect>
          <p v-if="filteredModels.length === 0" :class="descClass">
            {{ $t("modelSettings.debug.noModelsForType") }}
          </p>
        </div>
      </section>

      <template v-if="selectedModel">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("modelSettings.debug.groupInput") }}</h4>
          <div v-if="selectedModel.type !== 'ASR'">
            <label :class="labelClass">{{ inputLabel }}</label>
            <!-- field-sizing grows the box with its text, between four and eight rows. -->
            <Textarea v-model="input" :placeholder="inputPlaceholder" class="max-h-[184px] min-h-24" />
          </div>

          <div v-if="selectedModel.type === 'Rerank'">
            <label :class="labelClass">{{ $t("modelSettings.debug.documents") }}</label>
            <Textarea
              v-model="documentsText"
              :placeholder="$t('modelSettings.debug.documentsPlaceholder')"
              class="max-h-[184px] min-h-24"
            />
            <p :class="descClass">{{ $t("modelSettings.debug.documentsHint") }}</p>
          </div>

          <div v-if="needsFile">
            <label :class="labelClass">{{ fileLabel }}</label>
            <div class="relative">
              <!-- The native picker stays in the DOM, invisible, so the button can open it. -->
              <input
                ref="fileInputRef"
                class="pointer-events-none absolute size-0 opacity-0"
                type="file"
                :accept="selectedModel.type === 'VLLM' ? 'image/*' : 'audio/*'"
                @change="onNativeFileChange"
              />
              <Button variant="outline" size="sm" @click="fileInputRef?.click()">
                <UploadIcon />
                {{ $t("modelSettings.debug.chooseFile") }}
              </Button>
            </div>
            <p v-if="file" :class="descClass">{{ file.name }} · {{ formatBytes(file.size) }}</p>
          </div>
        </section>

        <section v-if="isChat" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("modelSettings.debug.parameters") }}</h4>
          <div class="grid grid-cols-3 gap-3.5 max-[640px]:grid-cols-1">
            <div>
              <label :class="labelClass">Temperature</label>
              <Input
                type="number"
                :model-value="temperature"
                :min="0"
                :max="2"
                :step="0.1"
                @update:model-value="(v) => (temperature = toNumber(v, temperature))"
                @blur="temperature = clamp(temperature, 0, 2)"
              />
            </div>
            <div>
              <label :class="labelClass">Top P</label>
              <Input
                type="number"
                :model-value="topP"
                :min="0.01"
                :max="1"
                :step="0.1"
                @update:model-value="(v) => (topP = toNumber(v, topP))"
                @blur="topP = clamp(topP, 0.01, 1)"
              />
            </div>
            <div>
              <label :class="labelClass">Max Tokens</label>
              <Input
                type="number"
                :model-value="maxTokens"
                :min="1"
                :max="8192"
                :step="128"
                @update:model-value="(v) => (maxTokens = toNumber(v, maxTokens))"
                @blur="maxTokens = clamp(maxTokens, 1, 8192)"
              />
            </div>
          </div>
          <div>
            <label :class="labelClass">{{ $t("modelSettings.debug.systemPrompt") }}</label>
            <Textarea
              v-model="systemPrompt"
              :placeholder="$t('modelSettings.debug.systemPromptPlaceholder')"
              class="max-h-[104px] min-h-[60px]"
            />
          </div>
          <div v-if="supportsThinking">
            <label :class="labelClass">{{ $t("modelSettings.debug.thinking") }}</label>
            <div class="flex min-h-8 items-center gap-2">
              <Switch v-model="thinking" />
              <span class="text-placeholder text-xs leading-normal">{{ $t("modelSettings.debug.thinkingDesc") }}</span>
            </div>
          </div>
        </section>

        <section v-if="result || history.length > 0" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("modelSettings.debug.groupResult") }}</h4>

          <div v-if="history.length > 1">
            <label :class="labelClass">{{ $t("modelSettings.debug.history") }}</label>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="run in history"
                :key="run.id"
                type="button"
                data-slot="history-item"
                class="text-muted-foreground hover:border-primary inline-flex items-center gap-2 rounded-md border px-2.5 py-[5px] text-xs hover:bg-[color-mix(in_srgb,var(--td-brand-color)_6%,var(--td-bg-color-container))]"
                :class="
                  result === run.result
                    ? 'border-primary bg-[color-mix(in_srgb,var(--td-brand-color)_6%,var(--td-bg-color-container))]'
                    : 'border-border bg-card'
                "
                @click="result = run.result"
              >
                <span class="text-foreground font-medium">{{ run.label }}</span>
                <span class="text-placeholder">{{ run.result.elapsed_ms }} ms</span>
              </button>
            </div>
          </div>

          <div v-if="result">
            <div
              class="flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-[13px]"
              :class="
                result.ok
                  ? 'text-success bg-[var(--td-success-color-light)]'
                  : 'text-destructive bg-[var(--td-error-color-light)]'
              "
            >
              <CircleCheckIcon v-if="result.ok" class="size-4 shrink-0" />
              <CircleXIcon v-else class="size-4 shrink-0" />
              <div class="flex items-baseline gap-2">
                <strong class="text-foreground">
                  {{ result.ok ? $t("modelSettings.debug.success") : $t("modelSettings.debug.failed") }}
                </strong>
                <span class="text-placeholder text-xs">{{ result.elapsed_ms }} ms</span>
              </div>
            </div>

            <div v-if="resultMetrics.length > 0" class="mt-2.5 flex flex-wrap gap-1.5">
              <span
                v-for="metric in resultMetrics"
                :key="metric.key"
                class="bg-muted text-muted-foreground rounded-[4px] px-2 py-0.5 text-xs"
              >
                {{ metric.label }}: {{ metric.value }}
              </span>
            </div>

            <p v-if="result.error" class="text-destructive mt-2.5 mb-0 text-[13px] whitespace-pre-wrap">
              {{ result.error }}
            </p>

            <!-- The tabs only switch what the <pre> below shows; they have no panels of their own. -->
            <Tabs
              :model-value="resultTab"
              class="mt-3"
              @update:model-value="(v) => (resultTab = v === 'request' ? 'request' : 'response')"
            >
              <TabsList variant="line">
                <TabsTrigger value="response" class="px-3 text-[13px]">
                  {{ $t("modelSettings.debug.rawResponse") }}
                </TabsTrigger>
                <TabsTrigger value="request" class="px-3 text-[13px]">
                  {{ $t("modelSettings.debug.requestPreview") }}
                </TabsTrigger>
              </TabsList>
            </Tabs>
            <pre
              class="border-border bg-muted text-foreground mt-2 mb-0 max-h-[420px] min-h-[140px] overflow-auto rounded-lg border px-3.5 py-3 font-mono text-xs leading-[1.6] break-words whitespace-pre-wrap"
              >{{ formattedResult }}</pre>
          </div>
        </section>
      </template>
    </div>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from "vue";
import type { Component } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { copyWithToast } from "@/utils/clipboard";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import SearchableSelect from "@/components/SearchableSelect.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  BubblesIcon,
  CircleCheckIcon,
  CirclePlayIcon,
  CircleXIcon,
  CopyIcon,
  FilterIcon,
  ImageIcon,
  MessageSquareIcon,
  UploadIcon,
  Volume2Icon,
} from "@lucide/vue";
import { debugModel, type ModelConfig, type ModelDebugResult } from "@/api/model";
import { fileSizeVerification } from "@/utils";
import { modelSupportsThinking } from "@/utils/thinkingControl";

const props = defineProps<{
  visible: boolean;
  models: ModelConfig[];
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
}>();

const { t, te } = useI18n();
const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit("update:visible", value),
});

type DebugModelType = ModelConfig["type"];

const selectedModelType = ref<DebugModelType>("KnowledgeQA");
const selectedModelId = ref("");
const input = ref("");
const documentsText = ref("");
const file = ref<File | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
const thinking = ref(false);
const temperature = ref(0.7);
const topP = ref(1);
const maxTokens = ref(1024);
const systemPrompt = ref("");
const running = ref(false);
const result = ref<ModelDebugResult | null>(null);
const resultTab = ref<"response" | "request">("response");
const history = ref<
  Array<{
    id: number;
    label: string;
    result: ModelDebugResult;
  }>
>([]);
let runSequence = 0;

const selectedModel = computed(() => props.models.find((model) => model.id === selectedModelId.value));
const filteredModels = computed(() => props.models.filter((model) => model.type === selectedModelType.value));
const isChat = computed(() => selectedModel.value?.type === "KnowledgeQA");
const supportsThinking = computed(() => (selectedModel.value ? modelSupportsThinking(selectedModel.value) : false));
const needsFile = computed(() => ["VLLM", "ASR"].includes(selectedModel.value?.type || ""));
const documents = computed(() =>
  documentsText.value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean),
);
const canRun = computed(() => {
  if (!selectedModel.value) return false;
  if (needsFile.value && !file.value) return false;
  if (selectedModel.value.type === "ASR") return true;
  if (selectedModel.value.type === "Rerank") return !!input.value.trim() && documents.value.length > 0;
  return !!input.value.trim();
});

const allModelTypeOptions = computed(() => {
  // The same icons as the model cards in ModelSettings, so a type reads
  // the same in the list and here.
  const keys: Record<DebugModelType, { short: string; icon: Component }> = {
    KnowledgeQA: { short: "chat", icon: MessageSquareIcon },
    Embedding: { short: "embedding", icon: BubblesIcon },
    Rerank: { short: "rerank", icon: FilterIcon },
    VLLM: { short: "vllm", icon: ImageIcon },
    ASR: { short: "asr", icon: Volume2Icon },
  };
  return (Object.keys(keys) as DebugModelType[]).map((value) => ({
    value,
    label: t(`modelSettings.typeShort.${keys[value].short}`),
    icon: keys[value].icon,
  }));
});

const modelCount = (type: DebugModelType) => props.models.filter((model) => model.type === type).length;

const availableModelTypes = computed(() => allModelTypeOptions.value.filter((option) => modelCount(option.value) > 0));

const modelLabel = (model: ModelConfig) => model.display_name?.trim() || model.name;

const vendorLabel = (model: ModelConfig) => {
  const provider = model.parameters.provider || "";
  if (model.source === "local") return "Ollama";
  if (provider === "generic") return t("modelSettings.source.custom");
  const key = `model.editor.providers.${provider}.label`;
  return te(key) ? t(key) : provider || model.source;
};

// The model picker's options. The raw name is searchable too, so a model
// with a display name is still found by the name the provider uses.
const modelOptions = computed(() =>
  filteredModels.value.map((model) => ({
    value: model.id || "",
    label: modelLabel(model),
    keywords: [model.name],
    meta: vendorLabel(model),
  })),
);

// Picking a model clears the previous result, as the old select's change
// handler did; re-picking the same model does too.
const onModelPicked = (value: string) => {
  selectedModelId.value = value;
  resetResult();
};

// Shared label and helper-text styles for the form rows.
const labelClass = "text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium";
const descClass = "text-placeholder mt-1 mb-0 text-xs leading-normal";

// The number inputs hold numbers. A cleared or half-typed field keeps the
// last valid value, and the bounds are applied when the field loses focus,
// which is when TDesign's input-number clamped too.
const toNumber = (value: string | number, fallback: number) => {
  if (value === "") return fallback;
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : fallback;
};

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value));

const inputLabel = computed(() => {
  if (selectedModel.value?.type === "Embedding") return t("modelSettings.debug.embeddingInput");
  if (selectedModel.value?.type === "VLLM") return t("modelSettings.debug.vlmPrompt");
  if (selectedModel.value?.type === "Rerank") return t("modelSettings.debug.query");
  return t("modelSettings.debug.query");
});

const inputPlaceholder = computed(() => {
  if (selectedModel.value?.type === "Embedding") return t("modelSettings.debug.embeddingPlaceholder");
  if (selectedModel.value?.type === "VLLM") return t("modelSettings.debug.vlmPromptPlaceholder");
  return t("modelSettings.debug.queryPlaceholder");
});

const fileLabel = computed(() =>
  selectedModel.value?.type === "VLLM" ? t("modelSettings.debug.imageFile") : t("modelSettings.debug.audioFile"),
);

const formattedResult = computed(() => {
  if (!result.value) return "";
  const value = resultTab.value === "response" ? result.value.raw_response : result.value.request;
  return JSON.stringify(value, null, 2);
});

const OBSERVATION_LABELS: Record<string, string> = {
  dimension: "modelSettings.debug.metrics.dimension",
  result_count: "modelSettings.debug.metrics.resultCount",
  answer_characters: "modelSettings.debug.metrics.answerChars",
  reasoning_characters: "modelSettings.debug.metrics.reasoningChars",
  reasoning_returned: "modelSettings.debug.metrics.reasoningReturned",
  text_characters: "modelSettings.debug.metrics.textChars",
  segment_count: "modelSettings.debug.metrics.segmentCount",
};

const resultMetrics = computed(() => {
  if (!result.value?.observations) return [];
  const obs = result.value.observations;
  const keys = Object.keys(OBSERVATION_LABELS).filter((key) => obs[key] !== undefined && obs[key] !== null);
  return keys.map((key) => ({
    key,
    label: t(OBSERVATION_LABELS[key]),
    value: formatMetricValue(key, obs[key]),
  }));
});

const formatMetricValue = (key: string, value: unknown) => {
  if (typeof value === "boolean") {
    return value ? t("common.yes") : t("common.no");
  }
  return String(value);
};

const ensureDefaultSelection = () => {
  const types = availableModelTypes.value;
  if (types.length === 0) {
    selectedModelId.value = "";
    return;
  }
  if (!types.some((option) => option.value === selectedModelType.value)) {
    selectedModelType.value = types[0].value;
  }
  const models = filteredModels.value;
  if (!models.some((model) => model.id === selectedModelId.value)) {
    selectedModelId.value = models[0]?.id || "";
  }
};

watch(
  () => props.visible,
  (visible) => {
    if (visible) ensureDefaultSelection();
  },
);

watch(availableModelTypes, () => {
  if (props.visible) ensureDefaultSelection();
});

watch(
  () => selectedModel.value?.id,
  () => {
    if (!supportsThinking.value) thinking.value = false;
  },
);

watch(
  () => selectedModel.value?.type,
  () => {
    file.value = null;
    result.value = null;
    history.value = [];
    resultTab.value = "response";
  },
);

const resetResult = () => {
  result.value = null;
  history.value = [];
  resultTab.value = "response";
};

const selectModelType = (type: DebugModelType) => {
  if (selectedModelType.value === type) return;
  selectedModelType.value = type;
  selectedModelId.value = filteredModels.value[0]?.id || "";
  input.value = "";
  documentsText.value = "";
  file.value = null;
  resetResult();
};

const onNativeFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement;
  const selectedFile = target.files?.[0] || null;
  if (selectedFile && fileSizeVerification(selectedFile)) {
    target.value = "";
    file.value = null;
    resetResult();
    return;
  }
  file.value = selectedFile;
  resetResult();
};

const formatBytes = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const historyLabel = (thinkingValue: boolean) => {
  if (supportsThinking.value) {
    return thinkingValue ? t("modelSettings.debug.thinkOn") : t("modelSettings.debug.thinkOff");
  }
  return t("modelSettings.debug.runLabel", { n: runSequence });
};

const runDebug = async () => {
  if (!selectedModel.value?.id || !canRun.value || running.value) return;
  running.value = true;
  try {
    const thinkingValue = supportsThinking.value ? thinking.value : false;
    const nextResult = await debugModel(selectedModel.value.id, {
      input: input.value.trim(),
      documents: documents.value,
      file: file.value,
      options: isChat.value
        ? {
            system_prompt: systemPrompt.value.trim() || undefined,
            temperature: temperature.value,
            top_p: topP.value,
            max_tokens: maxTokens.value,
            thinking: thinkingValue,
          }
        : {},
    });
    result.value = nextResult;
    history.value.unshift({
      id: ++runSequence,
      label: historyLabel(thinkingValue),
      result: nextResult,
    });
    history.value = history.value.slice(0, 6);
    resultTab.value = "response";
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("modelSettings.debug.requestFailed"));
  } finally {
    running.value = false;
  }
};

const copyResult = async () => {
  if (!result.value) return;
  await copyWithToast(JSON.stringify(result.value, null, 2), "common.copied");
};

onBeforeUnmount(() => {
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
});
</script>
