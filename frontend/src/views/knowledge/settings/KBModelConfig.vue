<template>
  <div class="w-full">
    <div class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("knowledgeEditor.models.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("knowledgeEditor.models.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- LLM 大语言模型 -->
      <div
        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
        data-guide="kb-create-llm"
      >
        <div class="max-w-[40%] shrink-0 basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("knowledgeEditor.models.llmLabel") }} <span class="text-destructive ml-0.5">*</span>
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("knowledgeEditor.models.llmDesc") }}</p>
        </div>
        <div class="flex max-w-[55%] shrink-0 basis-[55%] items-start justify-end">
          <ModelSelector
            ref="llmSelectorRef"
            model-type="KnowledgeQA"
            :selected-model-id="config.llmModelId"
            :all-models="allModels"
            @update:selected-model-id="handleLLMChange"
            @add-model="handleAddModel('chat')"
            :placeholder="$t('knowledgeEditor.models.llmPlaceholder')"
          />
        </div>
      </div>

      <!-- Embedding 嵌入模型: RAG 检索启用时必填; 纯 Wiki 时可选(用于目录归类相似度) -->
      <div
        v-if="ragEnabled !== false || wikiEnabled"
        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
        data-guide="kb-create-embedding"
      >
        <div class="max-w-[40%] shrink-0 basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("knowledgeEditor.models.embeddingLabel") }}
            <span v-if="ragEnabled" class="text-destructive ml-0.5">*</span>
            <span v-else-if="wikiEnabled" class="text-placeholder ml-1 text-xs font-normal">{{
              $t("knowledgeEditor.models.embeddingOptional")
            }}</span>
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{
              wikiEnabled && ragEnabled === false
                ? $t("knowledgeEditor.models.embeddingWikiOptionalDesc")
                : $t("knowledgeEditor.models.embeddingDesc")
            }}
          </p>
          <!-- t-alert theme="warning": warning-coloured icon on a tinted surface, body text in the normal colour. -->
          <Alert v-if="ragEnabled && hasFiles" class="border-warning/40 bg-warning/10 text-warning mt-2 px-2.5 py-2">
            <CircleAlertIcon />
            <AlertTitle class="text-foreground text-[13px] font-normal">{{
              $t("knowledgeEditor.models.embeddingLocked")
            }}</AlertTitle>
          </Alert>
        </div>
        <div class="flex max-w-[55%] shrink-0 basis-[55%] items-start justify-end">
          <ModelSelector
            ref="embeddingSelectorRef"
            model-type="Embedding"
            :selected-model-id="config.embeddingModelId"
            :all-models="allModels"
            :disabled="ragEnabled && hasFiles"
            :clearable="ragEnabled === false && wikiEnabled"
            @update:selected-model-id="handleEmbeddingChange"
            @add-model="handleAddModel('embedding')"
            :placeholder="$t('knowledgeEditor.models.embeddingPlaceholder')"
          />
        </div>
      </div>

      <!-- Wiki 合成模型 (仅当 Wiki 启用时显示) -->
      <div v-if="wikiEnabled" class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] shrink-0 basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.wiki.synthesisModelLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.wiki.synthesisModelTip") }}
          </p>
        </div>
        <div class="flex max-w-[55%] shrink-0 basis-[55%] items-start justify-end">
          <ModelSelector
            model-type="KnowledgeQA"
            :selected-model-id="config.wikiSynthesisModelId"
            :all-models="allModels"
            clearable
            @update:selected-model-id="handleWikiModelChange"
            @add-model="handleAddModel('knowledgeqa')"
            :placeholder="$t('knowledgeEditor.wiki.synthesisModelPlaceholder')"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useUIStore } from "@/stores/ui";
import ModelSelector from "@/components/ModelSelector.vue";
import { CircleAlertIcon } from "@lucide/vue";
import { Alert, AlertTitle } from "@/components/ui/alert";

interface ModelConfig {
  llmModelId?: string;
  embeddingModelId?: string;
  vllmModelId?: string;
  wikiSynthesisModelId?: string;
}

interface Props {
  config: ModelConfig;
  hasFiles: boolean;
  wikiEnabled?: boolean;
  ragEnabled?: boolean;
  allModels?: any[];
}

const props = defineProps<Props>();

const emit = defineEmits<{
  "update:config": [value: ModelConfig];
}>();

const uiStore = useUIStore();

const llmSelectorRef = ref<InstanceType<typeof ModelSelector>>();
const embeddingSelectorRef = ref<InstanceType<typeof ModelSelector>>();

const handleLLMChange = (modelId: string) => {
  emit("update:config", {
    ...props.config,
    llmModelId: modelId,
  });
};

const handleEmbeddingChange = (modelId: string) => {
  emit("update:config", {
    ...props.config,
    embeddingModelId: modelId,
  });
};

const handleWikiModelChange = (modelId: string) => {
  emit("update:config", {
    ...props.config,
    wikiSynthesisModelId: modelId,
  });
};

const handleAddModel = (subSection: string) => {
  uiStore.openSettings("models", subSection);
};
</script>
