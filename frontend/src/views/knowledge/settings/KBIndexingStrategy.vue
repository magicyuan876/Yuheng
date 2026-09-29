<template>
  <div class="w-full">
    <div class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("knowledgeEditor.indexing.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">
        {{ $t("knowledgeEditor.indexing.description") }}
      </p>
    </div>

    <div class="flex flex-col">
      <!-- Hybrid Search (vector + keyword combined) -->
      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.indexing.searchTitle")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.indexing.searchDesc") }}
          </p>
        </div>
        <div class="flex max-w-[55%] basis-[55%] items-start justify-end">
          <Switch :model-value="searchEnabled" @update:model-value="handleSearchToggle" />
        </div>
      </div>

      <!-- Wiki -->
      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.indexing.wikiTitle")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.indexing.wikiDesc") }}
          </p>
        </div>
        <div class="flex max-w-[55%] basis-[55%] items-start justify-end">
          <Switch
            :model-value="modelValue.wikiEnabled"
            @update:model-value="(val: boolean) => update('wikiEnabled', val)"
          />
        </div>
      </div>

      <!-- Wiki sub-settings (inline when enabled) -->
      <template v-if="modelValue.wikiEnabled">
        <slot name="wiki-settings" />
      </template>

      <!-- Knowledge Graph -->
      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("knowledgeEditor.indexing.graphTitle")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("knowledgeEditor.indexing.graphDesc") }}
          </p>
        </div>
        <div class="flex max-w-[55%] basis-[55%] items-start justify-end">
          <Switch
            :model-value="modelValue.graphEnabled"
            @update:model-value="(val: boolean) => update('graphEnabled', val)"
          />
        </div>
      </div>

      <!-- Graph sub-settings (inline when enabled) -->
      <template v-if="modelValue.graphEnabled">
        <slot name="graph-settings" />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Switch } from "@/components/ui/switch";

export interface IndexingStrategy {
  vectorEnabled: boolean;
  keywordEnabled: boolean;
  wikiEnabled: boolean;
  graphEnabled: boolean;
}

const props = defineProps<{
  modelValue: IndexingStrategy;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: IndexingStrategy): void;
}>();

// Search = vector + keyword combined (the system always uses hybrid search internally)
const searchEnabled = computed(() => props.modelValue.vectorEnabled || props.modelValue.keywordEnabled);

const handleSearchToggle = (val: boolean) => {
  emit("update:modelValue", {
    ...props.modelValue,
    vectorEnabled: val,
    keywordEnabled: val,
  });
};

const update = (field: keyof IndexingStrategy, value: boolean) => {
  emit("update:modelValue", {
    ...props.modelValue,
    [field]: value,
  });
};
</script>
