<template>
  <NodeViewWrapper as="span" class="docs-mention-wrap">
    <span class="docs-mention" :class="{ 'docs-mention--selected': selected }" :title="title">
      @{{ label }}
    </span>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed, inject } from 'vue'
import { useI18n } from 'vue-i18n'

import { DOCS_DIRECTORY, type DirectoryHandle } from './linkContext'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const directory = inject<DirectoryHandle | null>(DOCS_DIRECTORY, null)
const userId = computed(() => String(props.node.attrs.userId ?? ''))

/**
 * The current name wins over the one stored with the mention. The stored label
 * exists so an exported document still reads as a name; inside the editor a
 * person who has since been renamed should appear under the name they use now.
 */
const person = computed(() => {
  void directory?.revision.value
  return directory?.get(userId.value)
})

const label = computed(() =>
  person.value?.username || person.value?.email || String(props.node.attrs.label ?? '') || t('docs.links.someone'),
)

const title = computed(() => person.value?.email ?? '')
</script>

<style scoped lang="less">
.docs-mention-wrap {
  display: inline;
}

.docs-mention {
  display: inline-block;
  padding: 0 4px;
  border-radius: 3px;
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
  font-weight: 500;
  white-space: nowrap;

  &--selected {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}
</style>
