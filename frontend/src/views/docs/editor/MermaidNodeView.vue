<template>
  <NodeViewWrapper
    class="bg-card relative my-3 rounded-[8px] border p-2"
    :class="selected ? 'border-primary' : 'border-[var(--td-component-stroke)]'"
  >
    <div class="flex min-h-[18px] justify-end">
      <button
        v-if="editor.isEditable"
        type="button"
        data-slot="mermaid-toggle"
        class="text-placeholder hover:text-foreground hover:bg-accent inline-flex cursor-pointer items-center gap-1 rounded border-0 px-1 py-px text-xs"
        @click="showSource = !showSource"
      >
        <BarChart3Icon v-if="showSource" class="size-[13px]" />
        <CodeIcon v-else class="size-[13px]" />
        <span>{{ showSource ? t("docs.blocks.mermaidPreview") : t("docs.blocks.mermaidSource") }}</span>
      </button>
    </div>

    <Textarea
      v-if="showSource"
      class="border-primary field-sizing-fixed min-h-0 w-full resize-y rounded border px-2 py-1.5 font-[family-name:var(--td-font-family-mono,ui-monospace,monospace)] text-[13px] leading-normal md:text-[13px]"
      rows="6"
      :model-value="draft"
      :placeholder="t('docs.blocks.mermaidPlaceholder')"
      @update:model-value="draft = String($event)"
      @blur="commit"
    />

    <div
      v-else-if="svg"
      class="mermaid-canvas flex justify-center overflow-x-auto"
      :contenteditable="false"
      v-html="svg"
    />

    <p
      v-else-if="error"
      class="text-destructive m-2 flex items-center justify-center gap-1.5 text-[13px]"
      :contenteditable="false"
    >
      <CircleXIcon class="size-3.5" />
      <span>{{ t("docs.blocks.mermaidBroken") }}</span>
    </p>

    <p
      v-else
      class="text-placeholder m-2 flex items-center justify-center gap-1.5 text-[13px]"
      :contenteditable="false"
    >
      {{ t("docs.blocks.mermaidEmpty") }}
    </p>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { BarChart3Icon, CircleXIcon, CodeIcon } from "@lucide/vue";

import { Textarea } from "@/components/ui/textarea";
import { renderMermaidToSvg } from "@/utils/mermaidShared";

import { mermaidId } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const source = computed(() => String(props.node.attrs.source ?? ""));
const showSource = ref(false);
const draft = ref(source.value);
const svg = ref("");
const error = ref(false);

let seq = 0;
let disposed = false;

watch(
  source,
  (value) => {
    if (!showSource.value) draft.value = value;
    void render(value);
  },
  { immediate: true },
);

/**
 * Renders the diagram, and treats a failure as this block's problem alone.
 *
 * The shared helper already swallows its own exceptions and answers null, so a
 * diagram someone is halfway through writing shows an inline message here
 * while every other block on the page carries on rendering. The try/catch is
 * the second belt: an unexpected throw must not escape into the node view's
 * lifecycle, where Vue would tear down the whole editor subtree.
 */
async function render(code: string): Promise<void> {
  const ticket = ++seq;
  if (!code.trim()) {
    svg.value = "";
    error.value = false;
    return;
  }
  let rendered: string | null = null;
  try {
    rendered = await renderMermaidToSvg(code, mermaidId(props.node.attrs.id, ticket));
  } catch {
    rendered = null;
  }
  // A later edit already started rendering; its answer is the current one.
  if (disposed || ticket !== seq) return;
  svg.value = rendered ?? "";
  error.value = rendered === null;
}

function commit() {
  if (draft.value === source.value) return;
  props.updateAttributes({ source: draft.value });
}

onBeforeUnmount(() => {
  disposed = true;
});
</script>

<style scoped>
/* Rendered diagram markup: keep the svg inside its scroll box. */
.mermaid-canvas :deep(svg) {
  max-width: 100%;
  height: auto;
}
</style>
