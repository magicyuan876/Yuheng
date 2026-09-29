<template>
  <!-- data-slot opts the bare tag buttons and the input into the new stack's control reset
       (no browser chrome, inherited font). -->
  <div
    data-slot="tags-input"
    class="group border-input focus-within:border-ring focus-within:ring-ring/50 flex min-h-8 w-full flex-wrap items-center gap-1 rounded-lg border bg-transparent px-1.5 py-1 transition-colors focus-within:ring-3"
    :class="{ 'opacity-50': disabled }"
  >
    <span
      v-for="(tag, index) in modelValue"
      :key="`${index}:${tag}`"
      class="bg-secondary text-secondary-foreground inline-flex h-5 items-center gap-0.5 rounded-md px-1.5 text-xs font-medium"
    >
      {{ tag }}
      <button
        v-if="!disabled"
        type="button"
        class="text-muted-foreground hover:text-foreground cursor-pointer"
        :aria-label="`${t('common.delete')}: ${tag}`"
        @click="removeTag(index)"
      >
        <XIcon class="size-3" />
      </button>
    </span>
    <input
      v-model="draft"
      class="placeholder:text-placeholder h-5 min-w-[100px] flex-1 bg-transparent text-sm outline-none disabled:cursor-not-allowed"
      :placeholder="modelValue.length ? '' : placeholder"
      :disabled="disabled"
      :aria-label="ariaLabel"
      @keydown="onKeydown"
      @blur="draft = ''"
    />
    <!-- Like t-tag-input's clear icon: offered on hover, while there are tags or typed text. -->
    <button
      v-if="clearable && (modelValue.length > 0 || draft) && !disabled"
      type="button"
      class="text-muted-foreground hover:text-foreground invisible cursor-pointer group-hover:visible"
      :aria-label="t('common.clear')"
      @mousedown.prevent
      @click="clearAll"
    >
      <XIcon class="size-3.5" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { XIcon } from "@lucide/vue";

const props = withDefaults(
  defineProps<{
    modelValue: string[];
    placeholder?: string;
    disabled?: boolean;
    clearable?: boolean;
    ariaLabel?: string;
  }>(),
  {
    placeholder: "",
    disabled: false,
    clearable: true,
    ariaLabel: undefined,
  },
);

const emit = defineEmits<{
  (e: "update:modelValue", value: string[]): void;
  (e: "change", value: string[]): void;
}>();

const { t } = useI18n();

const draft = ref("");

function emitNext(next: string[]) {
  emit("update:modelValue", next);
  emit("change", next);
}

// The behaviour of the t-tag-input this replaces, which its consumers rely on
// (each change there asks for a confirmation and calls the API): Enter adds the
// trimmed text as a tag, duplicates included, and always empties the field;
// leaving the field discards unconfirmed text rather than adding it; Enter that
// confirms an IME composition does not add a tag.
function commitDraft() {
  const value = draft.value.trim();
  draft.value = "";
  if (!value || props.disabled) return;
  emitNext([...props.modelValue, value]);
}

// By position, as t-tag-input did: the list may hold the same text twice.
function removeTag(index: number) {
  if (props.disabled) return;
  emitNext(props.modelValue.filter((_, i) => i !== index));
}

function clearAll() {
  if (props.disabled) return;
  draft.value = "";
  emitNext([]);
}

function onKeydown(e: KeyboardEvent) {
  // keyCode 229 is the IME's own Enter in browsers that do not set isComposing.
  if (e.isComposing || e.keyCode === 229 || e.key === "Process") return;
  if (e.key === "Enter") {
    e.preventDefault();
    commitDraft();
    return;
  }
  if (e.key === "Backspace" || e.key === "Delete") onBackspace();
}

function onBackspace() {
  if (draft.value !== "" || props.disabled || props.modelValue.length === 0) return;
  emitNext(props.modelValue.slice(0, -1));
}
</script>
