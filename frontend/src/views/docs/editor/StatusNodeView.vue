<template>
  <NodeViewWrapper as="span" class="docs-status-wrap">
    <span
      class="docs-status"
      :class="[`docs-status--${color}`, { 'docs-status--selected': selected }]"
      :contenteditable="false"
      @click="edit"
      >{{ text || t("docs.blocks.statusEmpty") }}</span
    >
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { STATUS_COLORS, statusColor } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const text = computed(() => String(props.node.attrs.text ?? ""));
const color = computed(() => statusColor(props.node.attrs.color));

/** One click edits the text; holding shift steps the colour instead, which
 * keeps a chip to a single control rather than a popover. */
function edit(event: MouseEvent) {
  if (!props.editor.isEditable) return;
  if (event.shiftKey) {
    const next = STATUS_COLORS[(STATUS_COLORS.indexOf(color.value) + 1) % STATUS_COLORS.length];
    props.updateAttributes({ color: next });
    return;
  }
  const value = window.prompt(t("docs.blocks.statusPrompt"), text.value);
  if (value === null) return;
  props.updateAttributes({ text: value.trim().slice(0, 64) });
}
</script>

<style scoped lang="less">
.docs-status-wrap {
  display: inline;
}

.docs-status {
  display: inline-block;
  padding: 0 7px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
  vertical-align: baseline;
  cursor: pointer;
  user-select: none;
  background: var(--docs-status-bg);
  color: var(--docs-status-fg);

  --docs-status-bg: var(--td-bg-color-secondarycontainer);
  --docs-status-fg: var(--td-text-color-secondary);

  &--blue {
    --docs-status-bg: var(--td-brand-color-light);
    --docs-status-fg: var(--td-brand-color);
  }

  &--green {
    --docs-status-bg: var(--td-success-color-light);
    --docs-status-fg: var(--td-success-color);
  }

  &--yellow {
    --docs-status-bg: var(--td-warning-color-light);
    --docs-status-fg: var(--td-warning-color);
  }

  &--red {
    --docs-status-bg: var(--td-error-color-light);
    --docs-status-fg: var(--td-error-color);
  }

  &--purple {
    --docs-status-bg: #f0e8fa;
    --docs-status-fg: #7a4e8e;
  }

  &--selected {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}
</style>
