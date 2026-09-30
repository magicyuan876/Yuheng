<template>
  <div class="flex flex-col gap-2">
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <Button
          variant="outline"
          size="sm"
          class="w-full justify-between font-normal"
          :disabled="disabled"
          data-slot="kb-scope-trigger"
        >
          <span :class="model.length ? 'text-foreground truncate' : 'text-placeholder truncate'">
            {{ model.length ? t("workspaceApiKeys.knowledgeBaseCount", { count: model.length }) : allLabel }}
          </span>
          <ChevronDownIcon />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" class="w-(--reka-popover-trigger-width) min-w-[280px] p-2">
        <div class="relative mb-2">
          <SearchIcon class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
          <Input v-model="query" :placeholder="t('workspaceApiKeys.knowledgeBaseSearch')" class="pl-8" />
        </div>
        <div class="flex max-h-[240px] flex-col gap-1 overflow-y-auto">
          <label
            v-for="option in filtered"
            :key="option.id"
            class="hover:bg-accent flex cursor-pointer items-center gap-2 rounded px-1 py-1.5 text-sm"
          >
            <Checkbox
              :model-value="model.includes(option.id)"
              @update:model-value="(checked) => toggle(option.id, checked === true)"
            />
            <span class="truncate">{{ option.name }}</span>
          </label>
          <p v-if="!filtered.length" class="text-placeholder m-0 px-1 py-2 text-xs">
            {{ options.length ? t("common.noResult") : t("workspaceApiKeys.knowledgeBaseEmpty") }}
          </p>
        </div>
      </PopoverContent>
    </Popover>

    <!-- Every allowed knowledge base as a removable chip, including ones that no longer exist. -->
    <div v-if="model.length" class="flex flex-wrap gap-1.5">
      <Badge
        v-for="id in model"
        :key="id"
        :variant="nameOf(id) ? 'secondary' : 'destructive'"
        class="h-6 max-w-full gap-1 pr-1"
      >
        <span class="truncate">{{ nameOf(id) || t("workspaceApiKeys.knowledgeBaseMissing") }}</span>
        <button
          type="button"
          data-slot="kb-scope-remove"
          class="hover:text-foreground flex cursor-pointer items-center"
          :disabled="disabled"
          :aria-label="t('common.remove')"
          @click="toggle(id, false)"
        >
          <XIcon class="size-3" />
        </button>
      </Badge>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ChevronDownIcon, SearchIcon, XIcon } from "@lucide/vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

export interface KnowledgeBaseOption {
  id: string;
  name: string;
}

/**
 * Picks an API key's knowledge-base allow-list. Empty means "every knowledge
 * base", which is what the trigger says.
 *
 * An id that is not among the options (the knowledge base was deleted) stays
 * in the model and is shown as such. Dropping it silently would be worse than
 * showing it: if it was the only entry, the key would widen to every
 * knowledge base on the next save.
 */
const props = withDefaults(
  defineProps<{
    options: readonly KnowledgeBaseOption[];
    allLabel: string;
    disabled?: boolean;
  }>(),
  { disabled: false },
);

const model = defineModel<string[]>({ required: true });

const { t } = useI18n();
const open = ref(false);
const query = ref("");

const names = computed(() => new Map(props.options.map((option) => [option.id, option.name])));

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return props.options;
  return props.options.filter((option) => option.name.toLowerCase().includes(needle));
});

function nameOf(id: string): string {
  return names.value.get(id) ?? "";
}

function toggle(id: string, checked: boolean) {
  const rest = model.value.filter((item) => item !== id);
  model.value = checked ? [...rest, id] : rest;
}
</script>
