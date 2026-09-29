<template>
  <div class="w-full">
    <div v-if="!embedded" class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("knowledgeEditor.advanced.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("knowledgeEditor.advanced.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- Question Generation feature (only useful for RAG indexing) -->
      <template v-if="ragEnabled !== false">
        <div
          class="border-border flex justify-between [&:not(:last-child)]:border-b"
          :class="embedded ? 'items-center gap-4 py-3' : 'items-start py-4'"
        >
          <div :class="embedded ? 'min-w-0 flex-1 pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("knowledgeEditor.advanced.questionGeneration.label")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("knowledgeEditor.advanced.questionGeneration.description") }}
            </p>
          </div>
          <div
            :class="
              embedded
                ? 'flex flex-none items-center self-center'
                : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'
            "
          >
            <Switch
              :model-value="localQuestionGeneration.enabled"
              @update:model-value="
                (val: boolean) => {
                  localQuestionGeneration.enabled = val;
                  handleQuestionGenerationToggle();
                }
              "
            />
          </div>
        </div>

        <!-- Question Generation configuration -->
        <div
          v-if="localQuestionGeneration.enabled"
          class="relative"
          :class="embedded ? 'p-0' : 'bg-card border-l-primary mt-3 rounded-lg border-l-[3px] px-5 py-4'"
        >
          <div
            class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
            :class="[embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4']"
          >
            <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
              <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                $t("knowledgeEditor.advanced.questionGeneration.countLabel")
              }}</label>
              <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                {{ $t("knowledgeEditor.advanced.questionGeneration.countDescription") }}
              </p>
            </div>
            <div
              :class="embedded ? 'block self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'"
            >
              <Input
                v-model.number="localQuestionGeneration.questionCount"
                type="number"
                :min="1"
                :max="10"
                :step="1"
                class="w-[120px]"
                @change="handleQuestionGenerationChange"
              />
            </div>
          </div>
          <div
            class="border-border flex-col [&:not(:last-child)]:border-b"
            :class="embedded ? 'flex items-stretch gap-2 py-3' : 'flex gap-3 py-4'"
          >
            <div class="w-full max-w-none pr-0">
              <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                $t("knowledgeEditor.advanced.questionGeneration.instructionsLabel")
              }}</label>
              <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                {{ $t("knowledgeEditor.advanced.questionGeneration.instructionsDescription") }}
              </p>
            </div>
            <div class="block w-full max-w-none">
              <Textarea
                v-model="localQuestionGeneration.customInstructions"
                :placeholder="$t('knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder')"
                :maxlength="4000"
                :rows="3"
                class="max-h-[176px] min-h-[76px]"
                @update:model-value="handleQuestionGenerationChange"
              />
            </div>
          </div>
        </div>
      </template>

      <div
        class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'items-center gap-4 py-3' : 'py-4'"
      >
        <div :class="embedded ? 'min-w-0 flex-1 pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.advanced.autoTag.label")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.advanced.autoTag.description") }}
          </p>
        </div>
        <div
          :class="
            embedded
              ? 'flex flex-none items-center self-center'
              : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'
          "
        >
          <Switch
            :model-value="localAutoTag.enabled"
            @update:model-value="
              (val: boolean) => {
                localAutoTag.enabled = val;
                emitAutoTag();
              }
            "
          />
        </div>
      </div>

      <div
        v-if="localAutoTag.enabled"
        class="relative"
        :class="embedded ? 'p-0' : 'bg-card border-l-primary mt-3 rounded-lg border-l-[3px] px-5 py-4'"
      >
        <div
          class="border-border flex-col [&:not(:last-child)]:border-b"
          :class="embedded ? 'flex items-stretch gap-2 py-3' : 'flex gap-3 py-4'"
        >
          <div class="w-full max-w-none pr-0">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("knowledgeEditor.advanced.autoTag.modelLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("knowledgeEditor.advanced.autoTag.modelDescription") }}
            </p>
          </div>
          <div class="block w-full max-w-none">
            <ModelSelector
              model-type="KnowledgeQA"
              :selected-model-id="localAutoTag.modelId"
              :all-models="allModels"
              clearable
              :placeholder="$t('knowledgeEditor.advanced.autoTag.modelPlaceholder')"
              @update:selected-model-id="
                (value: string) => {
                  localAutoTag.modelId = value;
                  emitAutoTag();
                }
              "
            />
          </div>
        </div>
        <div
          class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
          :class="[embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4']"
        >
          <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("knowledgeEditor.advanced.autoTag.maxTagsLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("knowledgeEditor.advanced.autoTag.maxTagsDescription") }}
            </p>
          </div>
          <div
            :class="embedded ? 'block self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'"
          >
            <Input
              v-model.number="localAutoTag.maxTags"
              type="number"
              :min="1"
              :max="10"
              :step="1"
              class="w-[120px]"
              @change="emitAutoTag"
            />
          </div>
        </div>
        <div
          class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
          :class="[embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4']"
        >
          <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
            <label class="text-foreground mb-1 block text-[15px] font-medium">{{
              $t("knowledgeEditor.advanced.autoTag.skipIfTaggedLabel")
            }}</label>
            <p class="text-muted-foreground m-0 text-[13px] leading-normal">
              {{ $t("knowledgeEditor.advanced.autoTag.skipIfTaggedDescription") }}
            </p>
          </div>
          <div
            :class="
              embedded
                ? 'flex items-center self-start'
                : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'
            "
          >
            <Switch
              :model-value="localAutoTag.skipIfTagged"
              @update:model-value="
                (val: boolean) => {
                  localAutoTag.skipIfTagged = val;
                  emitAutoTag();
                }
              "
            />
          </div>
        </div>
      </div>

      <div class="border-border flex flex-col gap-3 [&:not(:last-child)]:border-b" :class="embedded ? 'py-3' : 'py-4'">
        <div class="w-full max-w-none pr-0">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.advanced.tableMetadataInstructions.label")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.advanced.tableMetadataInstructions.description") }}
          </p>
        </div>
        <div class="block w-full max-w-none">
          <Textarea
            :model-value="tableMetadataInstructions"
            :placeholder="$t('knowledgeEditor.advanced.tableMetadataInstructions.placeholder')"
            :maxlength="4000"
            :rows="3"
            class="max-h-[176px] min-h-[76px]"
            @update:model-value="(value) => emit('update:tableMetadataInstructions', String(value))"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import ModelSelector from "@/components/ModelSelector.vue";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";

interface QuestionGenerationConfig {
  enabled: boolean;
  questionCount: number;
  customInstructions?: string;
}

interface AutoTagConfig {
  enabled: boolean;
  modelId: string;
  maxTags: number;
  skipIfTagged: boolean;
}

interface Props {
  questionGeneration?: QuestionGenerationConfig;
  autoTag?: AutoTagConfig;
  ragEnabled?: boolean;
  allModels?: any[];
  embedded?: boolean;
  tableMetadataInstructions?: string;
}

const props = withDefaults(defineProps<Props>(), {
  embedded: false,
});

const emit = defineEmits<{
  "update:questionGeneration": [value: QuestionGenerationConfig];
  "update:autoTag": [value: AutoTagConfig];
  "update:tableMetadataInstructions": [value: string];
}>();

const localQuestionGeneration = ref<QuestionGenerationConfig>(
  props.questionGeneration
    ? { ...props.questionGeneration, customInstructions: props.questionGeneration.customInstructions || "" }
    : { enabled: false, questionCount: 3, customInstructions: "" },
);

const localAutoTag = ref<AutoTagConfig>(
  props.autoTag ? { ...props.autoTag } : { enabled: false, modelId: "", maxTags: 3, skipIfTagged: true },
);

watch(
  () => props.questionGeneration,
  (newVal) => {
    if (newVal) {
      localQuestionGeneration.value = { customInstructions: "", ...newVal };
    }
  },
  { deep: true },
);

watch(
  () => props.autoTag,
  (newVal) => {
    if (newVal) localAutoTag.value = { ...newVal };
  },
  { deep: true },
);

const emitAutoTag = () => {
  if (!localAutoTag.value.maxTags) localAutoTag.value.maxTags = 3;
  localAutoTag.value.maxTags = Math.min(10, Math.max(1, Math.trunc(localAutoTag.value.maxTags)));
  emit("update:autoTag", { ...localAutoTag.value });
};

const handleQuestionGenerationToggle = () => {
  if (!localQuestionGeneration.value.enabled) {
    localQuestionGeneration.value.questionCount = 3;
  }
  emit("update:questionGeneration", localQuestionGeneration.value);
};

const handleQuestionGenerationChange = () => {
  emit("update:questionGeneration", localQuestionGeneration.value);
};
</script>
