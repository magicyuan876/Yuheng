<template>
  <NodeViewWrapper class="my-3 flex gap-2.5 rounded-[8px] border-l-3 px-3.5 py-2.5" :class="KIND_CLASS[kind]">
    <!-- data-slot opts the bare button into the element reset, so it has no
         browser chrome to undo. -->
    <button
      v-if="editor.isEditable"
      type="button"
      data-slot="callout-kind"
      class="flex-none cursor-pointer text-[16px] leading-[1.5]"
      :title="t('docs.blocks.calloutKind')"
      @click="cycleKind"
    >
      {{ icon }}
    </button>
    <span v-else class="flex-none cursor-pointer text-[16px] leading-[1.5]" aria-hidden="true">{{ icon }}</span>
    <!-- As in a column, the editor's unlayered paragraph margins outrank the
         first/last-child resets here, as they did the scoped rules before. -->
    <NodeViewContent class="min-w-0 flex-1 [&>:first-child]:mt-0 [&>:last-child]:mb-0" />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { CALLOUT_KINDS, type CalloutKind, calloutIcon, calloutKind } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const kind = computed(() => calloutKind(props.node.attrs.kind));
/** Each kind's accent stripe and tint: TDesign's semantic colour and its light variant. */
const KIND_CLASS: Record<CalloutKind, string> = {
  info: "border-primary bg-[var(--td-brand-color-light)]",
  success: "border-success bg-[var(--td-success-color-light)]",
  warning: "border-warning bg-[var(--td-warning-color-light)]",
  danger: "border-destructive bg-[var(--td-error-color-light)]",
};

const icon = computed(() => calloutIcon(props.node.attrs.kind, props.node.attrs.icon));

/** Clicking the icon steps through the four kinds, which is quicker than a
 * menu for the one attribute a callout really has. */
function cycleKind() {
  const next = CALLOUT_KINDS[(CALLOUT_KINDS.indexOf(kind.value) + 1) % CALLOUT_KINDS.length];
  props.updateAttributes({ kind: next });
}
</script>
