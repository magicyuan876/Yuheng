<template>
  <form class="flex flex-col gap-1.5" @submit.prevent="submit">
    <!-- The controls are bare elements drawn by hand, compact enough for a
         sidebar; data-slot opts each into the element reset so none of the
         browser's chrome has to be undone. -->
    <textarea
      ref="input"
      v-model="draft"
      data-slot="comment-input"
      class="border-border bg-card text-foreground focus:border-primary box-border w-full resize-none rounded-[6px] border px-2 py-1.5 text-[13px] leading-[1.5] focus:outline-none"
      rows="2"
      :placeholder="placeholder ?? t('docs.comments.placeholder')"
      :disabled="busy"
      :aria-label="placeholder ?? t('docs.comments.placeholder')"
      @keydown="onKeyDown"
      @input="autosize"
    />
    <div class="flex items-center gap-2">
      <span class="text-placeholder flex-1 text-[11px]">{{ t("docs.comments.submitHint") }}</span>
      <button
        type="button"
        data-slot="comment-cancel"
        class="text-muted-foreground rounded-[6px] px-2.5 py-[3px] text-xs"
        :disabled="busy"
        @click="emit('cancel')"
      >
        {{ t("common.cancel") }}
      </button>
      <button
        type="submit"
        data-slot="comment-submit"
        class="bg-primary rounded-[6px] px-2.5 py-[3px] text-xs text-white disabled:bg-[var(--td-bg-color-component-disabled)] disabled:text-[var(--td-text-color-disabled)]"
        :disabled="busy || !canSubmit"
      >
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
