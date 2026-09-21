<template>
  <NodeViewWrapper class="docs-columns" :class="`docs-columns--${mode}`">
    <NodeViewContent class="docs-columns-row" />
    <div v-if="editor.isEditable" class="docs-columns-tools">
      <t-tooltip :content="t('docs.blocks.columnAdd')">
        <button type="button" :disabled="count >= MAX_COLUMNS" @click="addColumn">
          <t-icon name="add" size="14px" />
        </button>
      </t-tooltip>
      <t-tooltip :content="t('docs.blocks.columnRemove')">
        <button type="button" :disabled="count <= MIN_COLUMNS" @click="removeColumn">
          <t-icon name="remove" size="14px" />
        </button>
      </t-tooltip>
    </div>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { MAX_COLUMNS, MIN_COLUMNS } from "./figures";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const mode = computed(() => String(props.node.attrs.mode ?? "normal"));
const count = computed(() => props.node.childCount);

/**
 * Adding and removing a column are done as one transaction against the row's
 * own position. The schema's `column{2,5}` refuses anything outside the
 * bounds, so the buttons only disable themselves as a courtesy — the document
 * cannot be put into a bad shape even if they were clicked anyway.
 */
function addColumn() {
  if (count.value >= MAX_COLUMNS) return;
  const pos = props.getPos();
  if (typeof pos !== "number") return;
  const { state, view } = props.editor;
  const column = state.schema.nodes.column?.createAndFill();
  if (!column) return;
  view.dispatch(state.tr.insert(pos + props.node.nodeSize - 1, column));
}

function removeColumn() {
  if (count.value <= MIN_COLUMNS) return;
  const pos = props.getPos();
  if (typeof pos !== "number") return;
  const { state, view } = props.editor;
  const last = props.node.child(count.value - 1);
  const end = pos + props.node.nodeSize - 1;
  view.dispatch(state.tr.delete(end - last.nodeSize, end));
}
</script>

<style scoped lang="less">
.docs-columns {
  position: relative;
  margin: 12px 0;
}

.docs-columns-row {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.docs-columns--wide .docs-columns-row {
  gap: 32px;
}

.docs-columns-tools {
  position: absolute;
  top: -10px;
  right: 0;
  display: none;
  gap: 2px;

  button {
    border: 1px solid var(--td-component-stroke);
    background: var(--td-bg-color-container);
    border-radius: 4px;
    padding: 2px 4px;
    line-height: 0;
    cursor: pointer;
    color: var(--td-text-color-secondary);

    &:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
  }
}

.docs-columns:hover .docs-columns-tools {
  display: flex;
}
</style>
