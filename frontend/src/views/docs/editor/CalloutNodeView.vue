<template>
  <NodeViewWrapper class="docs-callout" :class="`docs-callout--${kind}`">
    <button
      v-if="editor.isEditable"
      type="button"
      class="docs-callout-icon"
      :title="t('docs.blocks.calloutKind')"
      @click="cycleKind"
    >
      {{ icon }}
    </button>
    <span v-else class="docs-callout-icon" aria-hidden="true">{{ icon }}</span>
    <NodeViewContent class="docs-callout-body" />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { CALLOUT_KINDS, calloutIcon, calloutKind } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const kind = computed(() => calloutKind(props.node.attrs.kind));
const icon = computed(() => calloutIcon(props.node.attrs.kind, props.node.attrs.icon));

/** Clicking the icon steps through the four kinds, which is quicker than a
 * menu for the one attribute a callout really has. */
function cycleKind() {
  const next = CALLOUT_KINDS[(CALLOUT_KINDS.indexOf(kind.value) + 1) % CALLOUT_KINDS.length];
  props.updateAttributes({ kind: next });
}
</script>

<style scoped lang="less">
.docs-callout {
  display: flex;
  gap: 10px;
  margin: 12px 0;
  padding: 10px 14px;
  border-radius: 8px;
  border-left: 3px solid var(--docs-callout-accent);
  background: var(--docs-callout-bg);

  --docs-callout-accent: var(--td-brand-color);
  --docs-callout-bg: var(--td-brand-color-light);

  &--success {
    --docs-callout-accent: var(--td-success-color);
    --docs-callout-bg: var(--td-success-color-light);
  }

  &--warning {
    --docs-callout-accent: var(--td-warning-color);
    --docs-callout-bg: var(--td-warning-color-light);
  }

  &--danger {
    --docs-callout-accent: var(--td-error-color);
    --docs-callout-bg: var(--td-error-color-light);
  }
}

.docs-callout-icon {
  flex: none;
  border: none;
  background: transparent;
  font-size: 16px;
  line-height: 1.5;
  padding: 0;
  cursor: pointer;
}

.docs-callout-body {
  flex: 1;
  min-width: 0;

  :deep(> :first-child) {
    margin-top: 0;
  }

  :deep(> :last-child) {
    margin-bottom: 0;
  }
}
</style>
