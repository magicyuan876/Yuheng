<template>
  <div
    v-if="visible"
    class="docs-table-toolbar"
    role="toolbar"
    :aria-label="t('docs.table.label')"
    :style="{ left: `${placement.left}px`, top: `${placement.top}px` }"
  >
    <template v-for="(group, gi) in groups" :key="gi">
      <span v-if="gi > 0" class="docs-table-divider" aria-hidden="true" />
      <button
        v-for="action in group"
        :key="action.id"
        type="button"
        class="docs-table-button"
        :class="{ 'is-danger': action.danger, 'is-active': action.palette && colorOpen }"
        :title="t(action.labelKey)"
        :aria-label="t(action.labelKey)"
        :disabled="!enabled(action)"
        @mousedown.prevent
        @click="run(action)"
      >
        <t-icon :name="action.icon" size="16px" />
      </button>
    </template>
  </div>

  <!-- The cell-colour palette hangs off the bar, the same arrangement the
    selection bar's text palette uses: inside the bar it would squeeze the
    buttons and change the bar's width as it opened. -->
  <div
    v-if="visible && colorOpen"
    class="docs-table-colors"
    role="listbox"
    :aria-label="t('docs.table.cellColor')"
    :style="{ left: `${placement.left}px`, top: `${placement.top + 40}px` }"
    @keydown.esc.prevent.stop="colorOpen = false"
  >
    <button
      type="button"
      class="docs-table-swatch docs-table-swatch--default"
      :title="t('docs.table.cellColorDefault')"
      :aria-label="t('docs.table.cellColorDefault')"
      @mousedown.prevent
      @click="applyColor(null)"
    >
      <t-icon name="close" size="12px" />
    </button>
    <button
      v-for="color in CELL_COLORS"
      :key="color"
      type="button"
      class="docs-table-swatch"
      :style="{ background: color }"
      :title="color"
      :aria-label="color"
      @mousedown.prevent
      @click="applyColor(color)"
    />
  </div>
</template>

<script setup lang="ts">
import type { Editor } from '@tiptap/core'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  CELL_COLORS, canRun, runAction, tableGroups, type TableAction, type TablePlacement,
} from './tableActions'

const props = defineProps<{
  visible: boolean
  placement: TablePlacement
  editor: Editor | null
  /** Bumped whenever the document or selection changed; the editor itself is
   * not reactive, and every button's enabled state depends on both. */
  revision: number
}>()

const { t } = useI18n()

const groups = computed(() => tableGroups())
const colorOpen = ref(false)

function enabled(action: TableAction): boolean {
  void props.revision
  return canRun(props.editor as never, action)
}

function run(action: TableAction) {
  if (action.palette) {
    colorOpen.value = !colorOpen.value
    return
  }
  colorOpen.value = false
  runAction(props.editor as never, action)
}

/**
 * Sets the fill on every selected cell.
 *
 * Through `setCellAttribute` rather than a command of its own: the schema
 * already carries `backgroundColor` on both cell types (see extensions.ts),
 * and ProseMirror's own command applies an attribute across a cell selection
 * for us. `null` clears it.
 */
function applyColor(color: string | null) {
  props.editor?.chain().focus().setCellAttribute('backgroundColor', color).run()
  colorOpen.value = false
}

// A palette left open over a table the caret has since left would apply to
// whatever cell is entered next.
watch(() => props.visible, (shown) => {
  if (!shown) colorOpen.value = false
})
</script>

<style scoped lang="less">
.docs-table-toolbar {
  position: fixed;
  z-index: 1350;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px 6px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 6px 20px rgb(0 0 0 / 12%);
}

.docs-table-divider {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: var(--td-component-stroke);
}

.docs-table-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-primary);
  cursor: pointer;

  &:hover:not(:disabled) {
    background: var(--td-bg-color-container-hover);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: -2px;
  }

  &.is-active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
  }

  &.is-danger:hover:not(:disabled) {
    color: var(--td-error-color);
  }

  &:disabled {
    color: var(--td-text-color-disabled);
    cursor: default;
  }
}

.docs-table-colors {
  position: fixed;
  z-index: 1350;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 6px 20px rgb(0 0 0 / 12%);
}

.docs-table-swatch {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 1px solid var(--td-component-stroke);
  border-radius: 50%;
  cursor: pointer;
  color: var(--td-text-color-placeholder);

  &:hover,
  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}

.docs-table-swatch--default {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
}
</style>
