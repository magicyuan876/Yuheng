<template>
  <NodeViewWrapper class="docs-audio" :class="{ 'docs-audio--selected': selected }">
    <audio v-if="src" class="docs-audio-player" :src="src" controls preload="metadata" />
    <p v-else class="docs-audio-missing">{{ t('docs.media.missing') }}</p>
    <button
      v-if="editor.isEditable"
      type="button"
      class="docs-audio-remove"
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

const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null)
const src = useAttachmentUrl(attachmentId)
</script>

<style scoped lang="less">
.docs-audio {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0;

  &--selected .docs-audio-player {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 2px;
    border-radius: 20px;
  }
}

.docs-audio-player {
  flex: 1;
  max-width: 420px;
}

.docs-audio-missing {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.docs-audio-remove {
  border: none;
  background: transparent;
  color: var(--td-text-color-placeholder);
  border-radius: 4px;
  padding: 4px;
  line-height: 0;
  cursor: pointer;

  &:hover {
    color: var(--td-error-color);
    background: var(--td-bg-color-container-hover);
  }
}
</style>
