<template>
  <NodeViewWrapper class="docs-media" :class="[`docs-media--${align}`, { 'docs-media--selected': selected }]">
    <video
      v-if="src"
      class="docs-media-video"
      :src="src"
      :style="frameStyle"
      controls
      preload="metadata"
      playsinline
    />
    <p v-else class="docs-media-missing">{{ t('docs.media.missing') }}</p>
    <button
      v-if="editor.isEditable"
      type="button"
      class="docs-media-remove"
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

import { useAttachmentUrl } from './useAttachmentUrl'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const align = computed(() => String(props.node.attrs.align ?? 'center'))
const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null)
const src = useAttachmentUrl(attachmentId)
const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0)
  return width > 0 ? { width: `${width}px` } : {}
})
</script>

<style scoped lang="less">
.docs-media {
  position: relative;
  display: flex;
  margin: 12px 0;

  &--left {
    justify-content: flex-start;
  }

  &--center {
    justify-content: center;
  }

  &--right {
    justify-content: flex-end;
  }

  &--selected .docs-media-video {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 2px;
  }
}

.docs-media-video {
  max-width: 100%;
  border-radius: 8px;
  background: #000;
}

.docs-media-missing {
  margin: 0;
  padding: 12px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
  border: 1px dashed var(--td-component-stroke);
  border-radius: 8px;
}

.docs-media-remove {
  position: absolute;
  top: 6px;
  right: 6px;
  border: none;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  padding: 3px 4px;
  line-height: 0;
  cursor: pointer;
  opacity: 0;
}

.docs-media:hover .docs-media-remove {
  opacity: 1;
}
</style>
