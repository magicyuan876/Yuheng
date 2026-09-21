<template>
  <form class="docs-composer" @submit.prevent="submit">
    <textarea
      ref="input"
      v-model="draft"
      class="docs-composer-input"
      rows="2"
      :placeholder="placeholder ?? t('docs.comments.placeholder')"
      :disabled="busy"
      :aria-label="placeholder ?? t('docs.comments.placeholder')"
      @keydown="onKeyDown"
      @input="autosize"
    />
    <div class="docs-composer-actions">
      <span class="docs-composer-hint">{{ t("docs.comments.submitHint") }}</span>
      <button type="button" class="docs-composer-cancel" :disabled="busy" @click="emit('cancel')">
        {{ t("common.cancel") }}
      </button>
      <button type="submit" class="docs-composer-submit" :disabled="busy || !canSubmit">
        {{ submitLabel ?? t("docs.comments.submit") }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import { bodyFromText, textOf } from "./composerBody";

const props = defineProps<{
  /** An existing body to edit; absent for a new comment. */
  initial?: unknown;
  placeholder?: string;
  submitLabel?: string;
  busy?: boolean;
  autofocus?: boolean;
}>();

const emit = defineEmits<{ submit: [body: unknown]; cancel: [] }>();
const { t } = useI18n();

const input = ref<HTMLTextAreaElement | null>(null);
const draft = ref(textOf(props.initial));

const canSubmit = computed(() => draft.value.trim() !== "");

/**
 * The composer writes plain text; composerBody.ts turns it into a document
 * and reads one back.
 *
 * Kept apart from this component because the two directions have to be exact
 * inverses: editing a comment reads the stored body into the box and writes
 * it out again, and anything lost in that round trip is lost from somebody's
 * remark without them touching it. There are tests for it.
 */

function submit() {
  if (!canSubmit.value || props.busy) return;
  emit("submit", bodyFromText(draft.value));
  draft.value = "";
}

/**
 * Enter sends, Shift+Enter makes a new line.
 *
 * The convention every chat box uses, and the one somebody writing a
 * one-sentence remark expects; the hint beside the buttons says so, because a
 * convention nobody is told about is a surprise.
 */
function onKeyDown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    event.preventDefault();
    emit("cancel");
    return;
  }
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    submit();
  }
}

function autosize() {
  const el = input.value;
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${Math.min(el.scrollHeight, 240)}px`;
}

onMounted(() => {
  if (props.autofocus !== false) void nextTick(() => input.value?.focus());
  autosize();
});
</script>

<style scoped lang="less">
.docs-composer {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.docs-composer-input {
  width: 100%;
  box-sizing: border-box;
  padding: 6px 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font: inherit;
  font-size: 13px;
  line-height: 1.5;
  resize: none;

  &:focus {
    outline: none;
    border-color: var(--td-brand-color);
  }
}

.docs-composer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.docs-composer-hint {
  flex: 1;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.docs-composer-cancel,
.docs-composer-submit {
  border: none;
  border-radius: 6px;
  padding: 3px 10px;
  font-size: 12px;
  cursor: pointer;
}

.docs-composer-cancel {
  background: transparent;
  color: var(--td-text-color-secondary);
}

.docs-composer-submit {
  background: var(--td-brand-color);
  color: #fff;

  &:disabled {
    background: var(--td-bg-color-component-disabled);
    color: var(--td-text-color-disabled);
    cursor: default;
  }
}
</style>
