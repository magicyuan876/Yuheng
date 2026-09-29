<template>
  <NodeViewWrapper class="group relative my-3">
    <NodeViewContent class="flex items-start" :class="mode === 'wide' ? 'gap-8' : 'gap-5'" />
    <div v-if="editor.isEditable" class="absolute -top-2.5 right-0 hidden gap-0.5 group-hover:flex">
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            data-slot="columns-tool"
            class="bg-card text-muted-foreground cursor-pointer rounded border border-[var(--td-component-stroke)] px-1 py-0.5 leading-0 disabled:cursor-not-allowed disabled:opacity-40"
            :disabled="count >= MAX_COLUMNS"
            @click="addColumn"
          >
            <PlusIcon class="size-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent>{{ t("docs.blocks.columnAdd") }}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            data-slot="columns-tool"
            class="bg-card text-muted-foreground cursor-pointer rounded border border-[var(--td-component-stroke)] px-1 py-0.5 leading-0 disabled:cursor-not-allowed disabled:opacity-40"
            :disabled="count <= MIN_COLUMNS"
            @click="removeColumn"
          >
            <MinusIcon class="size-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent>{{ t("docs.blocks.columnRemove") }}</TooltipContent>
      </Tooltip>
    </div>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { MinusIcon, PlusIcon } from "@lucide/vue";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

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
