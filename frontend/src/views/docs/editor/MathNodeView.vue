<template>
  <NodeViewWrapper :as="inline ? 'span' : 'div'" :class="inline ? 'inline' : 'my-3 block text-center'">
    <span
      v-if="!editing"
      class="cursor-pointer rounded-[4px] px-0.5"
      :class="{
        'text-destructive bg-[var(--td-error-color-light)] [font-family:var(--td-font-family-mono,ui-monospace,monospace)] text-[0.9em]':
          !!result.error,
        'outline-primary outline-2 outline-offset-1': selected,
      }"
      :contenteditable="false"
      :title="result.error || undefined"
      @click="startEditing"
      v-html="display"
    />
    <textarea
      v-else
      ref="input"
      data-slot="math-input"
      class="border-primary min-w-[120px] resize-y rounded-[4px] border px-1.5 py-0.5 [font-family:var(--td-font-family-mono,ui-monospace,monospace)] text-[13px]"
      :class="inline ? 'inline-block w-auto' : 'w-full'"
      :rows="inline ? 1 : 3"
      :value="draft"
      :placeholder="t('docs.blocks.mathPlaceholder')"
      @input="draft = ($event.target as HTMLTextAreaElement).value"
      @keydown.esc.prevent="stopEditing"
      @keydown.enter.exact="onEnter"
      @blur="stopEditing"
    />
    <span v-if="editing && preview.error" class="text-destructive mt-1 block text-xs">{{ preview.error }}</span>
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
