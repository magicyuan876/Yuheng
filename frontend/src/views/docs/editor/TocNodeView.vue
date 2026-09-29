<template>
  <NodeViewWrapper
    class="bg-muted my-3 rounded-[8px] border px-3.5 py-2.5"
    :class="selected ? 'border-primary' : 'border-border'"
  >
    <div class="text-placeholder mb-1.5 text-xs font-semibold tracking-[0.04em] uppercase">
      {{ t("docs.pages.toc") }}
    </div>
    <!-- The editor's unlayered list and paragraph rules (padding-left on ol,
         margins on p) outrank the resets on the list and the empty note, as
         they outranked the scoped rules this replaces. -->
    <ol v-if="entries.length" class="m-0 list-none p-0">
      <li
        v-for="entry in entries"
        :key="entry.pos"
        class="my-px"
        :style="{ paddingLeft: (entry.level - 1) * 14 + 'px' }"
      >
        <button
          type="button"
          data-slot="toc-entry"
          class="text-primary cursor-pointer py-0.5 text-left text-[13.5px]"
          @click="jump(entry.pos)"
        >
          {{ entry.text || t("docs.tree.untitled") }}
        </button>
      </li>
    </ol>
    <p v-else class="text-placeholder m-0 text-[13px]">{{ t("docs.pages.tocEmpty") }}</p>
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
