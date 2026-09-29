<template>
  <div class="w-full">
    <div class="mb-6">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ t("retrievalSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ t("retrievalSettings.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- Rerank Model -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-1 text-sm font-medium">
          <span>{{ t("retrievalSettings.rerankModelLabel") }} <span class="text-destructive">*</span></span>
        </div>
        <p class="text-muted-foreground m-0 mb-2 text-xs leading-normal">
          {{ t("retrievalSettings.rerankModelDescription") }}
        </p>
        <p v-if="!localConfig.rerank_model_id" class="text-warning m-0 mb-2 text-xs leading-normal">
          {{ t("retrievalSettings.rerankModelRequired") }}
        </p>
        <div class="w-full">
          <ModelSelector
            model-type="Rerank"
            :selected-model-id="localConfig.rerank_model_id"
            :disabled="!canEdit"
            @update:selected-model-id="handleModelChange"
          />
        </div>
      </div>

      <!-- Embedding Top K -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-2.5 flex items-center justify-between text-sm font-medium">
          <span>{{ t("retrievalSettings.embeddingTopKLabel") }}</span>
          <span class="text-primary font-[family-name:var(--app-font-family-mono)] text-[13px] font-semibold">
            {{ localConfig.embedding_top_k }}
          </span>
        </div>
        <Slider
          :model-value="[localConfig.embedding_top_k]"
          :min="1"
          :max="100"
          :step="1"
          :disabled="!canEdit"
          class="my-2"
          @update:model-value="(v) => setParam('embedding_top_k', v)"
        />
      </div>

      <!-- Vector Threshold -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-2.5 flex items-center justify-between text-sm font-medium">
          <span>{{ t("retrievalSettings.vectorThresholdLabel") }}</span>
          <span class="text-primary font-[family-name:var(--app-font-family-mono)] text-[13px] font-semibold">
            {{ localConfig.vector_threshold.toFixed(2) }}
          </span>
        </div>
        <Slider
          :model-value="[localConfig.vector_threshold]"
          :min="0"
          :max="1"
          :step="0.05"
          :disabled="!canEdit"
          class="my-2"
          @update:model-value="(v) => setParam('vector_threshold', v)"
        />
      </div>

      <!-- Keyword Threshold -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-2.5 flex items-center justify-between text-sm font-medium">
          <span>{{ t("retrievalSettings.keywordThresholdLabel") }}</span>
          <span class="text-primary font-[family-name:var(--app-font-family-mono)] text-[13px] font-semibold">
            {{ localConfig.keyword_threshold.toFixed(2) }}
          </span>
        </div>
        <Slider
          :model-value="[localConfig.keyword_threshold]"
          :min="0"
          :max="1"
          :step="0.05"
          :disabled="!canEdit"
          class="my-2"
          @update:model-value="(v) => setParam('keyword_threshold', v)"
        />
      </div>

      <!-- Rerank Top K -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-2.5 flex items-center justify-between text-sm font-medium">
          <span>{{ t("retrievalSettings.rerankTopKLabel") }}</span>
          <span class="text-primary font-[family-name:var(--app-font-family-mono)] text-[13px] font-semibold">
            {{ localConfig.rerank_top_k }}
          </span>
        </div>
        <Slider
          :model-value="[localConfig.rerank_top_k]"
          :min="1"
          :max="100"
          :step="1"
          :disabled="!canEdit"
          class="my-2"
          @update:model-value="(v) => setParam('rerank_top_k', v)"
        />
      </div>

      <!-- Rerank Threshold -->
      <div class="border-border py-4 [&:not(:last-child)]:border-b">
        <div class="text-foreground mb-2.5 flex items-center justify-between text-sm font-medium">
          <span>{{ t("retrievalSettings.rerankThresholdLabel") }}</span>
          <span class="text-primary font-[family-name:var(--app-font-family-mono)] text-[13px] font-semibold">
            {{ localConfig.rerank_threshold.toFixed(2) }}
          </span>
        </div>
        <Slider
          :model-value="[localConfig.rerank_threshold]"
          :min="-10"
          :max="10"
          :step="0.1"
          :disabled="!canEdit"
          class="my-2"
          @update:model-value="(v) => setParam('rerank_threshold', v)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, onMounted, nextTick } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import ModelSelector from "@/components/ModelSelector.vue";
import { Slider } from "@/components/ui/slider";
import { getTenantRetrievalConfig, updateTenantRetrievalConfig, type RetrievalConfig } from "@/api/retrieval";
import { useAuthStore } from "@/stores/auth";

const { t } = useI18n();
const authStore = useAuthStore();
// PUT /tenants/kv/retrieval-config requires Admin+ on the server. Hide the
// banner + lock all controls for non-Admins so they can read the
// configuration without tripping a 403 mid-edit.
const canEdit = computed(() => authStore.hasRole("admin"));

const defaultConfig: RetrievalConfig = {
  embedding_top_k: 50,
  vector_threshold: 0.15,
  keyword_threshold: 0.3,
  rerank_top_k: 10,
  rerank_threshold: 0.2,
  rerank_model_id: "",
};

const localConfig = reactive<RetrievalConfig>({ ...defaultConfig });
let initialConfig: RetrievalConfig = { ...defaultConfig };
let isInitializing = true;

const loadConfig = async () => {
  try {
    const response = await getTenantRetrievalConfig();
    if (response.data) {
      const cfg = response.data;
      Object.assign(localConfig, {
        embedding_top_k: cfg.embedding_top_k || defaultConfig.embedding_top_k,
        vector_threshold: cfg.vector_threshold || defaultConfig.vector_threshold,
        keyword_threshold: cfg.keyword_threshold || defaultConfig.keyword_threshold,
        rerank_top_k: cfg.rerank_top_k || defaultConfig.rerank_top_k,
        rerank_threshold: cfg.rerank_threshold ?? defaultConfig.rerank_threshold,
        rerank_model_id: cfg.rerank_model_id || "",
      });
      initialConfig = { ...localConfig };
    }
  } catch (error: any) {
    console.error("Failed to load retrieval config:", error);
  } finally {
    await nextTick();
    await nextTick();
    setTimeout(() => {
      isInitializing = false;
    }, 100);
  }
};

const hasConfigChanged = (): boolean => {
  return JSON.stringify(localConfig) !== JSON.stringify(initialConfig);
};

const saveConfig = async () => {
  if (!hasConfigChanged()) return;
  try {
    const response = await updateTenantRetrievalConfig({ ...localConfig });
    if (response.data) {
      initialConfig = { ...localConfig };
    }
    MessagePlugin.success(t("retrievalSettings.toasts.saveSuccess"));
  } catch (error: any) {
    console.error("Failed to save retrieval config:", error);
    const errorMessage = error?.message || "Unknown error";
    MessagePlugin.error(t("retrievalSettings.toasts.saveFailed", { message: errorMessage }));
  }
};

let saveTimer: number | null = null;
const debouncedSave = () => {
  if (isInitializing) return;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = window.setTimeout(() => {
    saveConfig().catch(() => {});
  }, 500);
};

const handleParamChange = () => debouncedSave();

type SliderParam = "embedding_top_k" | "vector_threshold" | "keyword_threshold" | "rerank_top_k" | "rerank_threshold";

// The Slider's model is an array (one entry per thumb). The old slider fired
// its change event on every move while dragging, and the save is debounced,
// so writing the value and scheduling the save on each update keeps both the
// live value display and the save timing as they were.
const setParam = (key: SliderParam, value: number[] | undefined) => {
  const next = value?.[0];
  if (typeof next !== "number") return;
  localConfig[key] = next;
  handleParamChange();
};
const handleModelChange = (modelId: string) => {
  localConfig.rerank_model_id = modelId;
  debouncedSave();
};

onMounted(async () => {
  isInitializing = true;
  await loadConfig();
});
</script>
