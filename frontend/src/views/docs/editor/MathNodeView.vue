<template>
  <NodeViewWrapper
    :as="inline ? 'span' : 'div'"
    class="docs-math"
    :class="[inline ? 'docs-math--inline' : 'docs-math--block', { 'docs-math--selected': selected }]"
  >
    <span
      v-if="!editing"
      class="docs-math-rendered"
      :class="{ 'docs-math-rendered--error': !!result.error }"
      :contenteditable="false"
      :title="result.error || undefined"
      @click="startEditing"
      v-html="display"
    />
    <textarea
      v-else
      ref="input"
      class="docs-math-input"
      :rows="inline ? 1 : 3"
      :value="draft"
      :placeholder="t('docs.blocks.mathPlaceholder')"
      @input="draft = ($event.target as HTMLTextAreaElement).value"
      @keydown.esc.prevent="stopEditing"
      @keydown.enter.exact="onEnter"
      @blur="stopEditing"
    />
    <span v-if="editing && preview.error" class="docs-math-error">{{ preview.error }}</span>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { renderMath } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const inline = computed(() => props.node.type.name === "mathInline");
const latex = computed(() => String(props.node.attrs.latex ?? ""));

const editing = ref(false);
const draft = ref(latex.value);
const input = ref<HTMLTextAreaElement | null>(null);

watch(latex, (value) => {
  if (!editing.value) draft.value = value;
});

/**
 * KaTeX output is inserted as HTML, which is safe here for a specific reason:
 * it is produced by KaTeX from the source, not taken from the document, and
 * KaTeX is configured with trust disabled so it emits no link, image or script
 * of its own. The document only ever stores the LaTeX.
 */
const result = computed(() => renderMath(latex.value, !inline.value));
const preview = computed(() => renderMath(draft.value, !inline.value));

const display = computed(() => {
  if (result.value.error) return escapeText(latex.value || t("docs.blocks.mathEmpty"));
  return result.value.html || escapeText(t("docs.blocks.mathEmpty"));
});

function escapeText(text: string): string {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function startEditing() {
  if (!props.editor.isEditable) return;
  draft.value = latex.value;
  editing.value = true;
  void nextTick(() => input.value?.focus());
}

function stopEditing() {
  if (!editing.value) return;
  editing.value = false;
  if (draft.value !== latex.value) props.updateAttributes({ latex: draft.value });
}

/** Enter commits an inline formula; a block one keeps the newline. */
function onEnter(event: KeyboardEvent) {
  if (!inline.value) return;
  event.preventDefault();
  stopEditing();
}
</script>

<style scoped lang="less">
.docs-math--inline {
  display: inline;
}

.docs-math--block {
  display: block;
  margin: 12px 0;
  text-align: center;
}

.docs-math-rendered {
  cursor: pointer;
  border-radius: 4px;
  padding: 0 2px;

  &--error {
    color: var(--td-error-color);
    background: var(--td-error-color-light);
    font-family: var(--td-font-family-mono, ui-monospace, monospace);
    font-size: 0.9em;
  }
}

.docs-math--selected .docs-math-rendered {
  outline: 2px solid var(--td-brand-color);
  outline-offset: 1px;
}

.docs-math-input {
  width: 100%;
  min-width: 120px;
  border: 1px solid var(--td-brand-color);
  border-radius: 4px;
  padding: 2px 6px;
  font-family: var(--td-font-family-mono, ui-monospace, monospace);
  font-size: 13px;
  resize: vertical;
}

.docs-math--inline .docs-math-input {
  width: auto;
  display: inline-block;
}

.docs-math-error {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--td-error-color);
}
</style>
