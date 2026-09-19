<template>
  <NodeViewWrapper class="docs-pdf" :class="{ 'docs-pdf--selected': selected }">
    <div class="docs-pdf-bar">
      <t-icon name="file-pdf" size="16px" />
      <span class="docs-pdf-name">{{ name }}</span>
      <a v-if="src" class="docs-pdf-open" :href="src" target="_blank" rel="noopener">
        {{ t('docs.media.openInTab') }}
      </a>
      <button
        v-if="editor.isEditable"
        type="button"
        class="docs-pdf-remove"
        :aria-label="t('docs.attachments.remove')"
        @click="deleteNode"
      >
        <t-icon name="delete" size="14px" />
      </button>
    </div>

    <!-- The browser's own viewer. The response is served with a sandboxing
         policy and an opaque origin, so the document cannot reach anything of
         the site's even if the file turns out not to be a PDF. -->
    <iframe
      v-if="src"
      class="docs-pdf-frame"
      :src="src"
      :style="frameStyle"
      :title="name"
      loading="lazy"
      referrerpolicy="no-referrer"
    />
    <p v-else class="docs-pdf-missing">{{ t('docs.media.missing') }}</p>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useAttachmentUrl } from './useAttachmentUrl'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const name = computed(() => String(props.node.attrs.name || t('docs.attachments.unnamed')))
const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null)
const src = useAttachmentUrl(attachmentId)
const frameStyle = computed(() => {
  const height = Number(props.node.attrs.height ?? 0)
  return { height: `${height > 0 ? height : 520}px` }
})
</script>

<style scoped lang="less">
.docs-pdf {
  margin: 12px 0;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  overflow: hidden;

  &--selected {
    border-color: var(--td-brand-color);
  }
}

.docs-pdf-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: var(--td-bg-color-secondarycontainer);
  font-size: 13px;
}

.docs-pdf-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.docs-pdf-open {
  font-size: 12px;
  color: var(--td-brand-color);
  text-decoration: none;
}

.docs-pdf-remove {
  border: none;
  background: transparent;
  color: var(--td-text-color-placeholder);
  border-radius: 4px;
  padding: 3px;
  line-height: 0;
  cursor: pointer;

  &:hover {
    color: var(--td-error-color);
  }
}

.docs-pdf-frame {
  display: block;
  width: 100%;
  border: none;
}

.docs-pdf-missing {
  margin: 0;
  padding: 16px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}
</style>
