<template>
  <NodeViewWrapper class="docs-toc" :class="{ 'docs-toc--selected': selected }">
    <div class="docs-toc-title">{{ t("docs.pages.toc") }}</div>
    <ol v-if="entries.length" class="docs-toc-list">
      <li v-for="entry in entries" :key="entry.pos" :style="{ paddingLeft: (entry.level - 1) * 14 + 'px' }">
        <button type="button" @click="jump(entry.pos)">{{ entry.text || t("docs.tree.untitled") }}</button>
      </li>
    </ol>
    <p v-else class="docs-toc-empty">{{ t("docs.pages.tocEmpty") }}</p>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { extractHeadings } from "./toc";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

/**
 * Derived from the document every time it changes rather than stored on the
 * node, so the list can never be out of step with the headings. The node view
 * re-renders on each transaction, which is exactly when a heading could have
 * changed.
 */
const entries = computed(() => extractHeadings(props.editor.state.doc));

function jump(pos: number) {
  props.editor
    .chain()
    .setTextSelection(pos + 1)
    .scrollIntoView()
    .focus()
    .run();
}
</script>

<style scoped lang="less">
.docs-toc {
  margin: 12px 0;
  padding: 10px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
}

.docs-toc--selected {
  border-color: var(--td-brand-color);
}

.docs-toc-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--td-text-color-placeholder);
  margin-bottom: 6px;
}

.docs-toc-list {
  list-style: none;
  margin: 0;
  padding: 0;

  li {
    margin: 1px 0;
  }

  button {
    border: none;
    background: transparent;
    padding: 2px 0;
    font-size: 13.5px;
    color: var(--td-brand-color);
    cursor: pointer;
    text-align: left;
  }
}

.docs-toc-empty {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}
</style>
