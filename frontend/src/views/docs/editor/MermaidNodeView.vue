<template>
  <NodeViewWrapper class="docs-mermaid" :class="{ 'docs-mermaid--selected': selected }">
    <div class="docs-mermaid-bar">
      <button
        v-if="editor.isEditable"
        type="button"
        class="docs-mermaid-toggle"
        @click="showSource = !showSource"
      >
        <t-icon :name="showSource ? 'chart' : 'code'" size="13px" />
        <span>{{ showSource ? t('docs.blocks.mermaidPreview') : t('docs.blocks.mermaidSource') }}</span>
      </button>
    </div>

    <textarea
      v-if="showSource"
      class="docs-mermaid-input"
      rows="6"
      :value="draft"
      :placeholder="t('docs.blocks.mermaidPlaceholder')"
      @input="draft = ($event.target as HTMLTextAreaElement).value"
      @blur="commit"
    />

    <div v-else-if="svg" class="docs-mermaid-canvas" :contenteditable="false" v-html="svg" />

    <p v-else-if="error" class="docs-mermaid-error" :contenteditable="false">
      <t-icon name="error-circle" size="14px" />
      <span>{{ t('docs.blocks.mermaidBroken') }}</span>
    </p>

    <p v-else class="docs-mermaid-empty" :contenteditable="false">{{ t('docs.blocks.mermaidEmpty') }}</p>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { renderMermaidToSvg } from '@/utils/mermaidShared'

import { mermaidId } from './figures'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const source = computed(() => String(props.node.attrs.source ?? ''))
const showSource = ref(false)
const draft = ref(source.value)
const svg = ref('')
const error = ref(false)

let seq = 0
let disposed = false

watch(source, (value) => {
  if (!showSource.value) draft.value = value
  void render(value)
}, { immediate: true })

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
  const ticket = ++seq
  if (!code.trim()) {
    svg.value = ''
    error.value = false
    return
  }
  let rendered: string | null = null
  try {
    rendered = await renderMermaidToSvg(code, mermaidId(props.node.attrs.id, ticket))
  } catch {
    rendered = null
  }
  // A later edit already started rendering; its answer is the current one.
  if (disposed || ticket !== seq) return
  svg.value = rendered ?? ''
  error.value = rendered === null
}

function commit() {
  if (draft.value === source.value) return
  props.updateAttributes({ source: draft.value })
}

onBeforeUnmount(() => {
  disposed = true
})
</script>

<style scoped lang="less">
.docs-mermaid {
  position: relative;
  margin: 12px 0;
  padding: 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
}

.docs-mermaid--selected {
  border-color: var(--td-brand-color);
}

.docs-mermaid-bar {
  display: flex;
  justify-content: flex-end;
  min-height: 18px;
}

.docs-mermaid-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  cursor: pointer;
  border-radius: 4px;
  padding: 1px 4px;

  &:hover {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container-hover);
  }
}

.docs-mermaid-input {
  width: 100%;
  border: 1px solid var(--td-brand-color);
  border-radius: 4px;
  padding: 6px 8px;
  font-family: var(--td-font-family-mono, ui-monospace, monospace);
  font-size: 13px;
  line-height: 1.5;
  resize: vertical;
}

.docs-mermaid-canvas {
  display: flex;
  justify-content: center;
  overflow-x: auto;

  :deep(svg) {
    max-width: 100%;
    height: auto;
  }
}

.docs-mermaid-error,
.docs-mermaid-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 8px 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.docs-mermaid-error {
  color: var(--td-error-color);
}
</style>
