<template>
  <NodeViewWrapper class="docs-column" :style="{ flexBasis: basis, flexGrow: grow }">
    <NodeViewContent />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed } from 'vue'

const props = defineProps<NodeViewProps>()

/**
 * A column with a declared width takes it; one without grows to fill what is
 * left. Sharing the row out is left to flexbox rather than computed here,
 * because a column view cannot see its siblings' widths and computing them
 * per column would make each one disagree with the others while an edit is in
 * flight.
 */
const width = computed(() => {
  const raw = Number(props.node.attrs.width ?? Number.NaN)
  return Number.isFinite(raw) ? Math.min(95, Math.max(5, raw)) : null
})

const basis = computed(() => (width.value === null ? '0' : `${width.value}%`))
const grow = computed(() => (width.value === null ? 1 : 0))
</script>

<style scoped lang="less">
.docs-column {
  min-width: 0;

  :deep(> :first-child) {
    margin-top: 0;
  }

  :deep(> :last-child) {
    margin-bottom: 0;
  }
}
</style>
