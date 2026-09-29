<template>
  <!-- The first and last child lose their outer margin, so a column's content
       sits flush with the row it shares. The editor's own paragraph margins
       are unlayered page CSS and outrank these utilities, exactly as they
       outranked the scoped rules this replaces. -->
  <NodeViewWrapper
    class="min-w-0 [&>:first-child]:mt-0 [&>:last-child]:mb-0"
    :style="{ flexBasis: basis, flexGrow: grow }"
  >
    <NodeViewContent />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";

const props = defineProps<NodeViewProps>();

/**
 * A column with a declared width takes it; one without grows to fill what is
 * left. Sharing the row out is left to flexbox rather than computed here,
 * because a column view cannot see its siblings' widths and computing them
 * per column would make each one disagree with the others while an edit is in
 * flight.
 */
const width = computed(() => {
  const raw = Number(props.node.attrs.width ?? Number.NaN);
  return Number.isFinite(raw) ? Math.min(95, Math.max(5, raw)) : null;
});

const basis = computed(() => (width.value === null ? "0" : `${width.value}%`));
const grow = computed(() => (width.value === null ? 1 : 0));
</script>
