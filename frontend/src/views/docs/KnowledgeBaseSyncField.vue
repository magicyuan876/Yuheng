<template>
  <!-- What the space syncs its pages into. Used by the create dialog (where "create" is the
       default) and by the settings tab (where it starts at the current binding). -->
  <div class="flex flex-col gap-1.5" data-testid="kb-sync">
    <Label>{{ t("docs.spaces.kbSync.title") }}</Label>
    <p class="text-muted-foreground m-0 text-xs">{{ t("docs.spaces.kbSync.hint") }}</p>

    <div v-if="!options" class="space-y-2 py-1">
      <Skeleton class="h-4 w-2/5" />
      <Skeleton class="h-4 w-3/5" />
    </div>

    <RadioGroup v-else v-model="mode" class="mt-1 flex flex-col gap-2.5">
      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-2">
          <RadioGroupItem
            id="kb-sync-create"
            value="create"
            data-testid="kb-sync-create"
            :disabled="!options.canCreate || disabled"
          />
          <Label for="kb-sync-create" class="font-normal" :class="!options.canCreate && 'text-muted-foreground'">
            {{ t("docs.spaces.kbSync.mode.create") }}
          </Label>
        </div>
        <p v-if="options.canCreate" class="text-muted-foreground m-0 pl-6 text-xs">
          {{ t("docs.spaces.kbSync.createHint") }}
        </p>
        <p
          v-else
          class="text-warning m-0 flex flex-wrap items-center gap-x-2 pl-6 text-xs"
          data-testid="kb-sync-no-embedding"
        >
          {{ t("docs.spaces.kbSync.noEmbedding") }}
          <Button variant="link" size="sm" class="h-auto p-0 text-xs" @click="openModelSettings">
            {{ t("docs.spaces.kbSync.configureModels") }}
          </Button>
        </p>
      </div>

      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-2">
          <RadioGroupItem
            id="kb-sync-existing"
            value="existing"
            data-testid="kb-sync-existing"
            :disabled="!existingOptions.length || disabled"
          />
          <Label for="kb-sync-existing" class="font-normal" :class="!existingOptions.length && 'text-muted-foreground'">
            {{ t("docs.spaces.kbSync.mode.existing") }}
          </Label>
        </div>
        <div v-if="model.mode === 'existing'" class="pl-6">
          <Select v-model="existingId" :disabled="disabled">
            <SelectTrigger class="w-full" data-testid="kb-sync-select">
              <SelectValue :placeholder="t('docs.spaces.kbSync.existingPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="kb in existingOptions" :key="kb.id" :value="kb.id">{{ kb.name || kb.id }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <p v-else-if="!existingOptions.length" class="text-muted-foreground m-0 pl-6 text-xs">
          {{ t("docs.spaces.kbSync.existingEmpty") }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <RadioGroupItem id="kb-sync-none" value="none" data-testid="kb-sync-none" :disabled="disabled" />
        <Label for="kb-sync-none" class="font-normal">{{ t("docs.spaces.kbSync.mode.none") }}</Label>
      </div>
    </RadioGroup>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import type { KnowledgeBaseChoice, KnowledgeBaseSyncMode } from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useUIStore } from "@/stores/ui";

import type { KnowledgeBaseSummary, SyncOptions } from "./knowledgeBaseSync";

const props = withDefaults(
  defineProps<{
    /** null while loading. */
    options: SyncOptions | null;
    /**
     * The knowledge base the space is bound to now. Kept in the list even when
     * the caller could not bind it afresh, so the current state reads right.
     */
    current?: KnowledgeBaseSummary | null;
    disabled?: boolean;
  }>(),
  { current: null, disabled: false },
);

const model = defineModel<KnowledgeBaseChoice>({ required: true });
const { t } = useI18n();
const uiStore = useUIStore();

const existingOptions = computed<KnowledgeBaseSummary[]>(() => {
  const list = props.options?.bindable ?? [];
  const current = props.current;
  return current && !list.some((kb) => kb.id === current.id) ? [current, ...list] : list;
});

const mode = computed<string>({
  get: () => model.value.mode,
  set: (next) => {
    const m = next as KnowledgeBaseSyncMode;
    // Picking "existing" preselects the only candidate there is, which is the
    // common case of a workspace with one knowledge base.
    const only = existingOptions.value.length === 1 ? existingOptions.value[0].id : "";
    model.value = m === "existing" ? { mode: m, id: model.value.id || only } : { mode: m };
  },
});

const existingId = computed<string>({
  get: () => model.value.id ?? "",
  set: (id) => {
    model.value = { mode: "existing", id };
  },
});

const openModelSettings = () => uiStore.openSettings("models", "embedding");
</script>
