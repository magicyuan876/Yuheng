<template>
  <NodeViewWrapper
    class="docs-file"
    :class="{ 'docs-file--selected': selected }"
    :data-drag-handle="editor.isEditable ? '' : undefined"
  >
    <a class="docs-file-link" :href="href" :download="name" target="_blank" rel="noopener">
      <t-icon :name="icon" size="20px" class="docs-file-icon" />
      <span class="docs-file-text">
        <span class="docs-file-name">{{ name }}</span>
        <span class="docs-file-meta">{{ meta }}</span>
      </span>
    </a>
    <button
      v-if="editor.isEditable"
      type="button"
      class="docs-file-remove"
      :aria-label="t('docs.attachments.remove')"
      @click="deleteNode"
    >
      <t-icon name="delete" size="14px" />
    </button>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { attachmentSrc, formatBytes } from './attachments'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const name = computed(() => String(props.node.attrs.name || t('docs.attachments.unnamed')))

const href = computed(() => {
  const id = props.node.attrs.attachmentId as string | null
  return id ? attachmentSrc(id) : '#'
})

const meta = computed(() => {
  const size = Number(props.node.attrs.size ?? 0)
  const parts = [formatBytes(size), shortType.value].filter(Boolean)
  return parts.join(' · ')
})

/** The tail of the media type, which reads better than the whole of it. */
const shortType = computed(() => {
  const mime = String(props.node.attrs.mime ?? '')
  if (!mime) return ''
  const sub = mime.split('/')[1] ?? mime
  return sub.split(/[.+]/).pop()?.toUpperCase() ?? ''
})

const icon = computed(() => {
  const mime = String(props.node.attrs.mime ?? '')
  if (mime.startsWith('video/')) return 'play-circle'
  if (mime.startsWith('audio/')) return 'sound'
  if (mime === 'application/pdf') return 'file-pdf'
  if (mime.includes('spreadsheet') || mime.includes('excel') || mime === 'text/csv') return 'file-excel'
  if (mime.includes('word')) return 'file-word'
  if (mime.includes('zip') || mime.includes('compressed')) return 'folder-zip'
  return 'file'
})
</script>

<style scoped lang="less">
.docs-file {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0;
  padding: 8px 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  max-width: 420px;
}

.docs-file--selected {
  border-color: var(--td-brand-color);
}

.docs-file-link {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
  color: inherit;
  text-decoration: none;
}

.docs-file-icon {
  color: var(--td-brand-color);
  flex: none;
}

.docs-file-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.docs-file-name {
  font-size: 13.5px;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.docs-file-meta {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.docs-file-remove {
  flex: none;
  border: none;
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: pointer;
  border-radius: 4px;
  padding: 4px;
  line-height: 0;

  &:hover {
    color: var(--td-error-color);
    background: var(--td-bg-color-container-hover);
  }
}
</style>
