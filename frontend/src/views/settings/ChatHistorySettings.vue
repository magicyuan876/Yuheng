<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ t("chatHistorySettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ t("chatHistorySettings.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- 启用开关 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ t("chatHistorySettings.enableLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("chatHistorySettings.enableDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <Switch :model-value="localEnabled" @update:model-value="onEnabledChange" />
        </div>
      </div>

      <!-- Embedding 模型选择 -->
      <div
        v-if="localEnabled"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ t("chatHistorySettings.embeddingModelLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("chatHistorySettings.embeddingModelDescription") }}
          </p>
          <p v-if="modelLocked" class="text-warning mt-1 mb-0 text-[13px] leading-normal">
            {{ t("chatHistorySettings.embeddingModelLocked") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <ModelSelector
            model-type="Embedding"
            :selected-model-id="localEmbeddingModelId"
            :disabled="modelLocked"
            @update:selected-model-id="handleModelChange"
          />
        </div>
      </div>
    </div>

    <!-- 统计信息 -->
    <div class="border-border mt-8 border-t pt-6">
      <h3 class="text-foreground mt-0 mb-4 text-base font-semibold">{{ t("chatHistorySettings.statsTitle") }}</h3>
      <div v-if="stats && stats.enabled && stats.knowledge_base_id" class="grid grid-cols-2 gap-4">
        <div class="bg-secondary rounded-lg p-5 text-center">
          <div class="text-primary mb-1 text-[28px] font-bold">{{ stats.indexed_message_count }}</div>
          <div class="text-muted-foreground text-[13px]">{{ t("chatHistorySettings.statsIndexedMessages") }}</div>
        </div>
      </div>
      <div v-else class="bg-secondary rounded-lg p-6 text-center">
        <p class="text-muted-foreground m-0 mb-1 text-sm font-medium">
          {{ t("chatHistorySettings.statsNotConfigured") }}
        </p>
        <p class="text-placeholder m-0 text-[13px]">{{ t("chatHistorySettings.statsNotConfiguredDesc") }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import ModelSelector from "@/components/ModelSelector.vue";
import {
  getTenantChatHistoryConfig,
  updateTenantChatHistoryConfig,
  getChatHistoryKBStats,
  type ChatHistoryConfig,
  type ChatHistoryKBStats,
} from "@/api/chat-history";

import { Switch } from "@/components/ui/switch";

const { t } = useI18n();

// Local state
const localEnabled = ref(false);
const localEmbeddingModelId = ref("");
const isInitializing = ref(true);
const initialConfig = ref<ChatHistoryConfig | null>(null);
const stats = ref<ChatHistoryKBStats | null>(null);

// Whether the embedding model is locked (has indexed messages — cannot change)
const modelLocked = ref(false);

// Load tenant config
const loadConfig = async () => {
  try {
    const response = await getTenantChatHistoryConfig();
    if (response.data) {
      const config = response.data;
      isInitializing.value = true;

      initialConfig.value = {
        enabled: config.enabled || false,
        embedding_model_id: config.embedding_model_id || "",
      };

      localEnabled.value = config.enabled || false;
      localEmbeddingModelId.value = config.embedding_model_id || "";

      await nextTick();
      await nextTick();
      setTimeout(() => {
        isInitializing.value = false;
      }, 100);
    } else {
      initialConfig.value = {
        enabled: false,
        embedding_model_id: "",
      };
      await nextTick();
      setTimeout(() => {
        isInitializing.value = false;
      }, 100);
    }
  } catch (error: any) {
    console.error("Failed to load chat history config:", error);
    initialConfig.value = {
      enabled: false,
      embedding_model_id: "",
    };
    await nextTick();
    setTimeout(() => {
      isInitializing.value = false;
    }, 100);
  }
};

// Load stats
const loadStats = async () => {
  try {
    const response = await getChatHistoryKBStats();
    if (response.data) {
      stats.value = response.data;
      // Lock model if there are indexed messages
      modelLocked.value = response.data.has_indexed_messages === true;
    }
  } catch (error: any) {
    console.error("Failed to load chat history stats:", error);
  }
};

// Check if config changed
const hasConfigChanged = (): boolean => {
  if (!initialConfig.value) return true;
  const initial = initialConfig.value;
  if (localEnabled.value !== initial.enabled) return true;
  if (localEmbeddingModelId.value !== initial.embedding_model_id) return true;
  return false;
};

// Save config
const saveConfig = async () => {
  if (!hasConfigChanged()) return;

  try {
    const config: ChatHistoryConfig = {
      enabled: localEnabled.value,
      embedding_model_id: localEmbeddingModelId.value,
    };

    const response = await updateTenantChatHistoryConfig(config);

    // Update initial config from response (includes auto-managed knowledge_base_id)
    if (response.data) {
      initialConfig.value = {
        enabled: response.data.enabled || false,
        embedding_model_id: response.data.embedding_model_id || "",
      };
    } else {
      initialConfig.value = { ...config };
    }

    MessagePlugin.success(t("chatHistorySettings.toasts.saveSuccess"));
    // Refresh stats after save
    loadStats();
  } catch (error: any) {
    console.error("Failed to save chat history config:", error);
    const errorMessage = error?.message || "Unknown error";
    MessagePlugin.error(t("chatHistorySettings.toasts.saveFailed", { message: errorMessage }));
  }
};

// Debounced save
let saveTimer: number | null = null;
const debouncedSave = () => {
  if (isInitializing.value) return;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = window.setTimeout(() => {
    saveConfig().catch(() => {});
  }, 500);
};

// Handlers
const handleEnabledChange = () => debouncedSave();
// The Switch reports its new value through `update:modelValue`; the wrapper
// stores it and then runs the old change handler, which only schedules a save.
const onEnabledChange = (value: boolean) => {
  localEnabled.value = value;
  handleEnabledChange();
};
const handleModelChange = (modelId: string) => {
  localEmbeddingModelId.value = modelId;
  debouncedSave();
};

// Init
onMounted(async () => {
  isInitializing.value = true;
  await loadConfig();
  await loadStats();
});
</script>
