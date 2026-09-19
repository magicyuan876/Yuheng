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
    :style="{ left: `${placement.left}px`, top: `${placement.top + 40}px` }"
    @keydown.esc.prevent.stop="closeColors"
  >
    <div class="docs-table-swatches" role="listbox" :aria-label="t('docs.table.cellColor')">
      <button
        v-for="color in CELL_COLORS"
        :key="color"
        type="button"
        role="option"
        class="docs-table-swatch"
        :class="{ 'is-active': currentColor.toLowerCase() === color }"
        :aria-selected="currentColor.toLowerCase() === color"
        :style="{ background: color }"
        :title="color"
        :aria-label="color"
        @mousedown.prevent
        @click="applyColor(color)"
      />
    </div>

    <div class="docs-table-colors-foot">
      <button
        type="button"
        class="docs-table-colors-action"
        @mousedown.prevent
        @click="applyColor(null)"
      >
        <t-icon name="close" size="13px" />
        <span>{{ t('docs.table.cellColorDefault') }}</span>
      </button>
      <button
        type="button"
        class="docs-table-colors-action"
        :class="{ 'is-active': pickerOpen }"
        :aria-expanded="pickerOpen"
        @mousedown.prevent
        @click="pickerOpen = !pickerOpen"
      >
        <t-icon name="palette" size="13px" />
        <span>{{ t('docs.table.cellColorCustom') }}</span>
      </button>
    </div>

    <!-- Hex without an alpha channel. A half-transparent fill is not a colour
      the document can round-trip: the schema stores one string, and what a
      reader sees would depend on whatever happens to be behind the table. -->
    <div v-if="pickerOpen" class="docs-table-picker" @mousedown.prevent>
      <t-color-picker-panel
        format="HEX"
        :enable-alpha="false"
        :show-primary-color-preview="false"
        :color-modes="['monochrome']"
        :value="currentColor || DEFAULT_PICK"
        @change="onPick"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Editor } from '@tiptap/core'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  CELL_COLORS, canRun, isCellColor, runAction, tableGroups,
  type TableAction, type TablePlacement,
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

/** What the picker opens on when the cell carries no fill of its own. */
const DEFAULT_PICK = '#4dabf7'

const groups = computed(() => tableGroups())
const colorOpen = ref(false)
const pickerOpen = ref(false)

/**
 * The editor as the catalogue wants it: the chain, plus the state and view
 * the tableOps-backed entries need to read the table node and dispatch.
 */
const target = computed(() => {
  void props.revision
  const editor = props.editor
  if (!editor) return null
  return {
    chain: () => editor.chain() as never,
    can: () => editor.can() as never,
    state: editor.state,
    view: editor.view,
  }
})

/** The fill on the cell the caret is in, '' when it carries none. */
const currentColor = computed(() => {
  void props.revision
  const editor = props.editor
  if (!editor) return ''
  try {
    return String(
      editor.getAttributes('tableCell')?.backgroundColor
      ?? editor.getAttributes('tableHeader')?.backgroundColor
      ?? '',
    )
  } catch {
    // A node type the schema does not have, which happens while extensions
    // are still being swapped on a page change.
    return ''
  }
})

function enabled(action: TableAction): boolean {
  void props.revision
  return canRun(target.value as never, action)
}

function run(action: TableAction) {
  if (action.palette) {
    colorOpen.value = !colorOpen.value
    if (!colorOpen.value) pickerOpen.value = false
    return
  }
  colorOpen.value = false
  pickerOpen.value = false
  runAction(target.value as never, action)
}

function closeColors() {
  colorOpen.value = false
  pickerOpen.value = false
  props.editor?.commands.focus()
}

/** A colour dragged out of the picker panel. */
function onPick(value: unknown) {
  if (typeof value === 'string' && isCellColor(value)) applyColor(value)
}

/**
 * Sets the fill on every selected cell.
 *
 * Through `setCellAttribute` rather than a command of its own: the schema
 * already carries `backgroundColor` on both cell types (see extensions.ts),
 * and ProseMirror's own command applies an attribute across a cell selection
 * for us. `null` clears it.
 *
 * The colour is checked first. An attribute the server's schema would refuse
 * must not reach the document: by the time the save is rejected the person
 * has moved on, and all they see is a page that quietly stopped saving.
 */
function applyColor(color: string | null) {
  if (color !== null && !isCellColor(color)) return
  props.editor?.chain().focus().setCellAttribute('backgroundColor', color).run()
  // The palette stays open while the picker is up, so a colour can be tried
  // and changed without reopening it; a preset closes it, which is the
  // one-click gesture a swatch promises.
  if (!pickerOpen.value) colorOpen.value = false
}

// A palette left open over a table the caret has since left would apply to
// whatever cell is entered next.
watch(() => props.visible, (shown) => {
  if (!shown) {
    colorOpen.value = false
    pickerOpen.value = false
  }
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
  padding: 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 6px 20px rgb(0 0 0 / 12%);
}

// Ten to a row, so the two bands line up hue for hue: the pale one sits
// directly above the saturated one it is a tint of.
.docs-table-swatches {
  display: grid;
  grid-template-columns: repeat(10, 20px);
  gap: 5px;
}

.docs-table-swatch {
  width: 20px;
  height: 20px;
  padding: 0;
  // A hairline at low opacity rather than the component stroke: against a
  // saturated swatch the stroke colour disappears, and a ring that vanishes
  // on half the palette looks like a rendering fault.
  border: 1px solid rgb(0 0 0 / 12%);
  border-radius: 4px;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }

  &.is-active {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}

.docs-table-colors-foot {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--td-component-stroke);
}

.docs-table-colors-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  flex: 1;
  height: 26px;
  padding: 0 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  cursor: pointer;

  &:hover {
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
}

.docs-table-picker {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--td-component-stroke);
}
</style>
