<template>
  <div v-if="open" class="docs-find" role="dialog" :aria-label="t('docs.find.title')" @keydown.esc.prevent.stop="close">
    <div class="docs-find-row">
      <input
        ref="queryInput"
        v-model="query"
        type="text"
        class="docs-find-input"
        :placeholder="t('docs.find.findPlaceholder')"
        :aria-label="t('docs.find.findPlaceholder')"
        @keydown.enter.prevent="step($event.shiftKey ? -1 : 1)"
      />
      <span class="docs-find-count" aria-live="polite">
        {{ matches.length ? t("docs.find.count", { index: current + 1, total: matches.length }) : t("docs.find.none") }}
      </span>
      <button type="button" class="docs-find-icon" :aria-label="t('docs.find.previous')" @click="step(-1)">
        <t-icon name="chevron-up" size="16px" />
      </button>
      <button type="button" class="docs-find-icon" :aria-label="t('docs.find.next')" @click="step(1)">
        <t-icon name="chevron-down" size="16px" />
      </button>
      <button type="button" class="docs-find-icon" :aria-label="t('common.close')" @click="close">
        <t-icon name="close" size="16px" />
      </button>
    </div>

    <div v-if="editable" class="docs-find-row">
      <input
        v-model="replacement"
        type="text"
        class="docs-find-input"
        :placeholder="t('docs.find.replacePlaceholder')"
        :aria-label="t('docs.find.replacePlaceholder')"
        @keydown.enter.prevent="replaceCurrent"
      />
      <button type="button" class="docs-find-text" :disabled="!matches.length" @click="replaceCurrent">
        {{ t("docs.find.replace") }}
      </button>
      <button type="button" class="docs-find-text" :disabled="!matches.length" @click="replaceEvery">
        {{ t("docs.find.replaceAll") }}
      </button>
    </div>

    <div class="docs-find-row docs-find-options">
      <label><input v-model="caseSensitive" type="checkbox" /> {{ t("docs.find.caseSensitive") }}</label>
      <label><input v-model="wholeWord" type="checkbox" /> {{ t("docs.find.wholeWord") }}</label>
      <label><input v-model="regex" type="checkbox" /> {{ t("docs.find.regex") }}</label>
      <span v-if="badPattern" class="docs-find-error">{{ t("docs.find.badPattern") }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Editor } from "@tiptap/core";
import { TextSelection } from "@tiptap/pm/state";
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { IdleScheduler } from "./idleWork";
import { findKey, findMatches, matchAfter, replaceAll, replaceMatch, searchRegex, stepMatch, type Match } from "./find";

const props = defineProps<{
  open: boolean;
  editor: Editor | null;
  editable: boolean;
  /** Bumped when the document changed, so the matches are found again. */
  revision: number;
}>();

const emit = defineEmits<{ close: [] }>();
const { t } = useI18n();

const query = ref("");
const replacement = ref("");
const caseSensitive = ref(false);
const wholeWord = ref(false);
const regex = ref(false);
const matches = ref<Match[]>([]);
const current = ref(0);
const queryInput = ref<HTMLInputElement | null>(null);

const options = computed(() => ({
  caseSensitive: caseSensitive.value,
  wholeWord: wholeWord.value,
  regex: regex.value,
}));

/** A pattern still being typed is not an error to report loudly, only a note. */
const badPattern = computed(
  () => regex.value && query.value !== "" && searchRegex(query.value, options.value) === null,
);

/**
 * Recomputes the matches and tells the plugin to draw them.
 *
 * Driven by a watcher rather than called from each handler so there is one
 * path: the query, the options and the document all change the same answer,
 * and any of them changing must also refresh the highlight.
 */
function refresh(keepCurrent = true) {
  const editor = props.editor;
  if (!editor || editor.isDestroyed) return;
  const found = props.open ? findMatches(editor.state.doc, query.value, options.value) : [];
  matches.value = found;

  if (found.length === 0) current.value = 0;
  else if (!keepCurrent || current.value >= found.length) {
    current.value = Math.max(0, matchAfter(found, editor.state.selection.from));
  }
  draw();
}

/** Hands the ranges to the plugin, which is the only thing that paints. */
function draw() {
  const editor = props.editor;
  if (!editor || editor.isDestroyed) return;
  const tr = editor.state.tr.setMeta(findKey, {
    matches: matches.value,
    current: matches.value.length ? current.value : -1,
  });
  // Not an edit: nothing here should reach the undo stack or the other
  // people editing this page.
  tr.setMeta("addToHistory", false);
  editor.view.dispatch(tr);
}

/** Moves to another match and scrolls it into view without taking focus. */
function step(delta: -1 | 1) {
  if (matches.value.length === 0) return;
  current.value = stepMatch(current.value, delta, matches.value.length);
  reveal();
}

function reveal() {
  const editor = props.editor;
  const match = matches.value[current.value];
  if (!editor || !match) return;
  const tr = editor.state.tr
    .setSelection(TextSelection.create(editor.state.doc, match.from, match.to))
    .scrollIntoView();
  tr.setMeta("addToHistory", false);
  editor.view.dispatch(tr);
  draw();
}

function replaceCurrent() {
  const editor = props.editor;
  const match = matches.value[current.value];
  if (!editor || !match || !props.editable) return;
  editor.view.dispatch(replaceMatch(editor.state, match, replacement.value));
  // The document moved; the remaining matches are found again rather than
  // adjusted, which is both simpler and correct for a replacement of any
  // length. The one that was current is gone, so the next one takes its place.
  void nextTick(() => refresh(false));
}

function replaceEvery() {
  const editor = props.editor;
  if (!editor || !props.editable || matches.value.length === 0) return;
  const tr = replaceAll(editor.state, matches.value, replacement.value);
  if (tr) editor.view.dispatch(tr);
  void nextTick(() => refresh(false));
}

function close() {
  matches.value = [];
  draw();
  emit("close");
  props.editor?.commands.focus();
}

/**
 * Searching is deferred the same way the word count is.
 *
 * With the panel open, every keystroke in the document would otherwise walk
 * every block of it. On a page of fifty thousand words that is the difference
 * between typing and waiting, and the answer is only ever a highlight — it can
 * arrive a moment late.
 */
const rescan = new IdleScheduler(() => refresh());

watch([query, options], () => refresh());
watch(
  () => props.revision,
  () => rescan.schedule(),
);
onBeforeUnmount(() => rescan.cancel());

watch(
  () => props.open,
  (open) => {
    if (open) {
      // Opening with something selected searches for it, which is what every
      // editor does and what somebody who selected a word then pressed Ctrl+F
      // meant.
      const selected = selectedText();
      if (selected && !selected.includes("\n")) query.value = selected;
      void nextTick(() => {
        queryInput.value?.focus();
        queryInput.value?.select();
        refresh(false);
      });
    } else {
      matches.value = [];
      draw();
    }
  },
);

function selectedText(): string {
  const editor = props.editor;
  if (!editor || editor.state.selection.empty) return "";
  const { from, to } = editor.state.selection;
  return editor.state.doc.textBetween(from, Math.min(to, from + 120), "\n");
}
</script>

<style scoped lang="less">
.docs-find {
  position: absolute;
  top: 8px;
  right: 16px;
  z-index: 1300;
  width: 380px;
  max-width: calc(100% - 32px);
  padding: 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 6px 20px rgb(0 0 0 / 12%);
}

.docs-find-row {
  display: flex;
  align-items: center;
  gap: 6px;

  & + & {
    margin-top: 6px;
  }
}

.docs-find-input {
  flex: 1;
  min-width: 0;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: 13px;
}

.docs-find-count {
  flex: none;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.docs-find-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.docs-find-text {
  height: 26px;
  padding: 0 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-brand-color);
  font-size: 13px;
  cursor: pointer;

  &:disabled {
    color: var(--td-text-color-disabled);
    cursor: default;
  }
}

.docs-find-options {
  font-size: 12px;
  color: var(--td-text-color-secondary);

  label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
  }
}

.docs-find-error {
  color: var(--td-warning-color);
}
</style>
