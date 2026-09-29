<!--
  SearchableSelect — a single-value select with a search field, for lists
  long enough that scrolling is slower than typing (models, Ollama tags).

  ui/select has no filtering, so this composes Popover + Input + a listbox
  instead. It replaces the filterable TDesign select in the model
  editor and the model debug drawer. The trigger is a button styled as the
  select box; the search field lives in the popover, which keeps the
  trigger a single focusable control and the list reachable by keyboard.

  Filtering is done here by default (case-blind on the label and on any
  extra `keywords`). A parent that needs to see the keyword — the Ollama
  picker offers "download <keyword>" when nothing matches — binds
  `v-model:keyword` and sets `:filter="false"` to filter the options
  itself.
-->
<template>
  <div class="relative w-full min-w-0">
    <Popover :open="open" @update:open="setOpen">
      <PopoverTrigger as-child>
        <button
          type="button"
          data-slot="searchable-select-trigger"
          :disabled="disabled"
          class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 relative flex h-8 w-full min-w-0 items-center gap-1.5 overflow-hidden rounded-lg border bg-transparent pr-2 pl-2.5 text-left text-[13px] transition-colors outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50"
          :class="{ 'border-ring': open }"
        >
          <!--
            Optional progress fill behind the value (the Ollama picker shows
            a model download filling the box). It sits under the text, so
            the value stays readable while it grows.
          -->
          <span
            v-if="progress != null"
            aria-hidden="true"
            class="pointer-events-none absolute inset-y-0 left-0 bg-linear-to-r from-[rgba(7,192,95,0.08)] to-[rgba(7,192,95,0.15)] transition-[width] duration-300"
            :style="{ width: `${progress}%` }"
          />
          <span v-if="selectedLabel" class="text-foreground relative min-w-0 flex-1 truncate">{{ selectedLabel }}</span>
          <span v-else class="text-placeholder relative min-w-0 flex-1 truncate">{{ placeholder }}</span>
          <span v-if="$slots.suffix" class="relative flex shrink-0 items-center">
            <slot name="suffix" />
          </span>
          <Loader2Icon v-if="loading" class="text-muted-foreground relative size-4 shrink-0 animate-spin" />
          <ChevronDownIcon
            v-else
            class="text-muted-foreground relative size-4 shrink-0 transition-transform duration-150"
            :class="{ 'rotate-180': open }"
          />
        </button>
      </PopoverTrigger>

      <!--
        z-[5500] matches TDesign's popup layer. The select is used inside
        SettingDrawer, which sits at z-[2500]; the popover's default z-50
        would open behind the panel.
      -->
      <PopoverContent
        align="start"
        class="z-[5500] w-(--reka-popover-trigger-width) min-w-[240px] gap-1 p-1"
        :class="contentClass"
        @open-auto-focus="onOpenAutoFocus"
      >
        <div class="relative">
          <SearchIcon
            class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2"
          />
          <Input
            ref="searchInputRef"
            :model-value="keywordValue"
            :placeholder="searchPlaceholder || placeholder"
            class="h-8 pl-8 text-[13px] md:text-[13px]"
            role="combobox"
            aria-autocomplete="list"
            :aria-expanded="open"
            @update:model-value="(v) => setKeyword(String(v))"
            @keydown="onSearchKeydown"
          />
        </div>

        <div role="listbox" class="flex max-h-[300px] flex-col gap-px overflow-y-auto">
          <button
            v-for="(option, index) in visibleOptions"
            :key="option.value"
            type="button"
            role="option"
            data-slot="searchable-select-option"
            :aria-selected="option.value === modelValue"
            class="flex min-h-8 w-full shrink-0 items-center gap-2 rounded-md px-2 py-1 text-left text-[13px]"
            :class="{
              'bg-accent': index === highlightedIndex,
              'text-primary': option.value === modelValue,
            }"
            @mouseenter="highlightedIndex = index"
            @click="choose(option.value)"
          >
            <slot name="option" :option="option" :selected="option.value === modelValue">
              <span class="min-w-0 truncate">{{ option.label }}</span>
            </slot>
          </button>

          <div
            v-if="visibleOptions.length === 0"
            class="text-muted-foreground flex h-8 shrink-0 items-center justify-center text-[13px]"
          >
            {{ loading ? t("common.loading") : t("common.noResult") }}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script setup lang="ts" generic="T extends SearchableSelectOption">
import { computed, nextTick, ref, watch } from "vue";
import type { HTMLAttributes } from "vue";
import { useI18n } from "vue-i18n";
import { ChevronDownIcon, Loader2Icon, SearchIcon } from "@lucide/vue";

import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { SearchableSelectOption } from "./searchableSelect";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    options: T[];
    placeholder?: string;
    searchPlaceholder?: string;
    disabled?: boolean;
    loading?: boolean;
    /** When false the parent filters `options` itself from `keyword`. */
    filter?: boolean;
    /** Controlled search text, for a parent that filters (see `filter`). */
    keyword?: string;
    /** 0–100 to paint a progress fill behind the value; null for none. */
    progress?: number | null;
    contentClass?: HTMLAttributes["class"];
  }>(),
  {
    placeholder: "",
    searchPlaceholder: "",
    disabled: false,
    loading: false,
    filter: true,
    keyword: undefined,
    progress: null,
    contentClass: undefined,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
  "update:keyword": [value: string];
  /** Fires on every open and close, like TDesign's visible-change. */
  "open-change": [open: boolean];
}>();

const { t } = useI18n();

const open = ref(false);
const localKeyword = ref("");
const highlightedIndex = ref(-1);
const searchInputRef = ref<InstanceType<typeof Input> | null>(null);

const keywordValue = computed(() => props.keyword ?? localKeyword.value);

function setKeyword(value: string) {
  localKeyword.value = value;
  emit("update:keyword", value);
}

// A value with no matching option (a model that was deleted, or a list
// still loading) shows the raw value, as TDesign's select did, rather
// than pretending nothing is selected.
const selectedLabel = computed(() => {
  if (!props.modelValue) return "";
  return props.options.find((option) => option.value === props.modelValue)?.label ?? props.modelValue;
});

const visibleOptions = computed<T[]>(() => {
  if (!props.filter) return props.options;
  const q = keywordValue.value.trim().toLowerCase();
  if (!q) return props.options;
  return props.options.filter(
    (option) =>
      option.label.toLowerCase().includes(q) || (option.keywords ?? []).some((k) => k.toLowerCase().includes(q)),
  );
});

// Typing moves the highlight to the first match, so Enter picks it.
watch(keywordValue, () => {
  highlightedIndex.value = visibleOptions.value.length > 0 ? 0 : -1;
});

watch(visibleOptions, (options) => {
  if (highlightedIndex.value >= options.length) highlightedIndex.value = options.length - 1;
});

function setOpen(value: boolean) {
  if (open.value === value) return;
  open.value = value;
  if (value) {
    highlightedIndex.value = visibleOptions.value.findIndex((option) => option.value === props.modelValue);
  } else {
    setKeyword("");
  }
  emit("open-change", value);
}

// Focus lands in the search field when the list opens, so typing filters
// straight away as it did in the old select.
function onOpenAutoFocus(event: Event) {
  event.preventDefault();
  void nextTick(() => {
    const el = searchInputRef.value?.$el;
    if (el instanceof HTMLElement) el.focus();
  });
}

function choose(value: string) {
  setOpen(false);
  emit("update:modelValue", value);
}

function onSearchKeydown(event: KeyboardEvent) {
  const count = visibleOptions.value.length;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    if (count > 0) highlightedIndex.value = (highlightedIndex.value + 1) % count;
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    if (count > 0) highlightedIndex.value = (highlightedIndex.value - 1 + count) % count;
  } else if (event.key === "Enter") {
    event.preventDefault();
    const option = visibleOptions.value[highlightedIndex.value] ?? (count === 1 ? visibleOptions.value[0] : undefined);
    if (option) choose(option.value);
  }
}
</script>
