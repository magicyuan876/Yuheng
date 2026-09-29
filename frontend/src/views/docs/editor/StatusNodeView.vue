<template>
  <NodeViewWrapper as="span" class="inline">
    <span
      class="inline-block cursor-pointer rounded-[4px] px-[7px] align-baseline text-xs leading-[18px] font-semibold select-none"
      :class="[COLOR_CLASS[color], { 'outline-primary outline-2 outline-offset-1': selected }]"
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

import { STATUS_COLORS, type StatusColor, statusColor } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const text = computed(() => String(props.node.attrs.text ?? ""));
const color = computed(() => statusColor(props.node.attrs.color));

/**
 * One chip colour per status, background and text together. TDesign has a
 * light tint for each semantic colour but no token for purple, which keeps
 * the fixed pair it has always had.
 */
const COLOR_CLASS: Record<StatusColor, string> = {
  gray: "bg-muted text-muted-foreground",
  blue: "bg-[var(--td-brand-color-light)] text-primary",
  green: "bg-[var(--td-success-color-light)] text-success",
  yellow: "bg-[var(--td-warning-color-light)] text-warning",
  red: "bg-[var(--td-error-color-light)] text-destructive",
  purple: "bg-[#f0e8fa] text-[#7a4e8e]",
};

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
