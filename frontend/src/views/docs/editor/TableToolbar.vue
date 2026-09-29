<template>
  <div
    v-if="visible"
    class="bg-popover fixed z-1350 flex items-center gap-0.5 rounded-[8px] border border-[var(--td-component-stroke)] px-1.5 py-1 shadow-[0_6px_20px_rgb(0_0_0/0.12)]"
    role="toolbar"
    :aria-label="t('docs.table.label')"
    :style="{ left: `${placement.left}px`, top: `${placement.top}px` }"
  >
    <template v-for="(group, gi) in groups" :key="gi">
      <span v-if="gi > 0" class="mx-1 h-[18px] w-px bg-[var(--td-component-stroke)]" aria-hidden="true" />
      <button
        v-for="action in group"
        :key="action.id"
        type="button"
        data-slot="table-toolbar-button"
        class="focus-visible:outline-primary enabled:hover:bg-accent flex h-7 w-7 cursor-pointer items-center justify-center rounded-md border-0 p-0 focus-visible:outline-2 focus-visible:-outline-offset-2 disabled:cursor-default disabled:text-[var(--td-text-color-disabled)]"
        :class="[
          action.palette && colorOpen ? 'text-primary bg-[var(--td-brand-color-light)]' : 'text-foreground',
          action.danger ? 'enabled:hover:text-destructive' : '',
        ]"
        :title="t(action.labelKey)"
        :aria-label="t(action.labelKey)"
        :disabled="!enabled(action)"
        @mousedown.prevent
        @click="run(action)"
      >
        <component :is="editorIcon(action.icon, TableIcon)" class="size-4" />
      </button>
    </template>
  </div>

  <!-- The cell-colour palette hangs off the bar, the same arrangement the
    selection bar's text palette uses: inside the bar it would squeeze the
    buttons and change the bar's width as it opened. -->
  <div
    v-if="visible && colorOpen"
    class="bg-popover fixed z-1350 rounded-[8px] border border-[var(--td-component-stroke)] p-2 shadow-[0_6px_20px_rgb(0_0_0/0.12)]"
    :style="{ left: `${placement.left}px`, top: `${placement.top + 40}px` }"
    @keydown.esc.prevent.stop="closeColors"
  >
    <!-- Ten to a row, so the two bands line up hue for hue: the pale one sits directly above the
         saturated one it is a tint of. A swatch's ring is a hairline at low opacity rather than
         the component stroke: against a saturated swatch the stroke colour disappears, and a
         ring that vanishes on half the palette looks like a rendering fault. -->
    <div class="grid grid-cols-[repeat(10,20px)] gap-[5px]" role="listbox" :aria-label="t('docs.table.cellColor')">
      <button
        v-for="color in CELL_COLORS"
        :key="color"
        type="button"
        role="option"
        data-slot="table-swatch"
        class="h-5 w-5 cursor-pointer rounded border border-black/12 outline-offset-1 hover:outline-2 hover:outline-[var(--td-brand-color)] focus-visible:outline-2 focus-visible:outline-[var(--td-brand-color)]"
        :class="currentColor.toLowerCase() === color ? 'outline-2 outline-[var(--td-brand-color)]' : ''"
        :aria-selected="currentColor.toLowerCase() === color"
        :style="{ background: color }"
        :title="color"
        :aria-label="color"
        @mousedown.prevent
        @click="applyColor(color)"
      />
    </div>

    <div class="mt-2 flex items-center gap-1 border-t border-[var(--td-component-stroke)] pt-2">
      <button
        type="button"
        data-slot="table-colors-action"
        class="text-muted-foreground hover:bg-accent focus-visible:outline-primary flex h-[26px] flex-1 cursor-pointer items-center justify-center gap-1 rounded-md border-0 px-2 text-xs focus-visible:outline-2 focus-visible:-outline-offset-2"
        @mousedown.prevent
        @click="applyColor(null)"
      >
        <XIcon class="size-[13px]" />
        <span>{{ t("docs.table.cellColorDefault") }}</span>
      </button>
      <button
        type="button"
        data-slot="table-colors-action"
        class="focus-visible:outline-primary flex h-[26px] flex-1 cursor-pointer items-center justify-center gap-1 rounded-md border-0 px-2 text-xs focus-visible:outline-2 focus-visible:-outline-offset-2"
        :class="pickerOpen ? 'text-primary bg-[var(--td-brand-color-light)]' : 'text-muted-foreground hover:bg-accent'"
        :aria-expanded="pickerOpen"
        @mousedown.prevent
        @click="pickerOpen = !pickerOpen"
      >
        <PaletteIcon class="size-[13px]" />
        <span>{{ t("docs.table.cellColorCustom") }}</span>
      </button>
    </div>

    <!-- Hex without an alpha channel. A half-transparent fill is not a colour
      the document can round-trip: the schema stores one string, and what a
      reader sees would depend on whatever happens to be behind the table. -->
    <div v-if="pickerOpen" class="mt-2 border-t border-[var(--td-component-stroke)] pt-2" @mousedown.prevent>
      <input
        type="color"
        class="h-8 w-full cursor-pointer"
        :value="currentColor || DEFAULT_PICK"
        :aria-label="t('docs.table.cellColorCustom')"
        @change="onPick(($event.target as HTMLInputElement).value)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Editor } from "@tiptap/core";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { PaletteIcon, TableIcon, XIcon } from "@lucide/vue";

import {
  CELL_COLORS,
  canRun,
  isCellColor,
  runAction,
  tableGroups,
  type TableAction,
  type TablePlacement,
} from "./tableActions";
import { editorIcon } from "./lucideIconMap";

const props = defineProps<{
  visible: boolean;
  placement: TablePlacement;
  editor: Editor | null;
  /** Bumped whenever the document or selection changed; the editor itself is
   * not reactive, and every button's enabled state depends on both. */
  revision: number;
}>();

const { t } = useI18n();

/** What the picker opens on when the cell carries no fill of its own. */
const DEFAULT_PICK = "#4dabf7";

const groups = computed(() => tableGroups());
const colorOpen = ref(false);
const pickerOpen = ref(false);

/**
 * The editor as the catalogue wants it: the chain, plus the state and view
 * the tableOps-backed entries need to read the table node and dispatch.
 */
const target = computed(() => {
  void props.revision;
  const editor = props.editor;
  if (!editor) return null;
  return {
    chain: () => editor.chain() as never,
    can: () => editor.can() as never,
    state: editor.state,
    view: editor.view,
  };
});

/** The fill on the cell the caret is in, '' when it carries none. */
const currentColor = computed(() => {
  void props.revision;
  const editor = props.editor;
  if (!editor) return "";
  try {
    return String(
      editor.getAttributes("tableCell")?.backgroundColor ?? editor.getAttributes("tableHeader")?.backgroundColor ?? "",
    );
  } catch {
    // A node type the schema does not have, which happens while extensions
    // are still being swapped on a page change.
    return "";
  }
});

function enabled(action: TableAction): boolean {
  void props.revision;
  return canRun(target.value as never, action);
}

function run(action: TableAction) {
  if (action.palette) {
    colorOpen.value = !colorOpen.value;
    if (!colorOpen.value) pickerOpen.value = false;
    return;
  }
  colorOpen.value = false;
  pickerOpen.value = false;
  runAction(target.value as never, action);
}

function closeColors() {
  colorOpen.value = false;
  pickerOpen.value = false;
  props.editor?.commands.focus();
}

/**
 * A colour committed on the native picker. `change`, not `input`: the picker
 * reports every step of a drag as input, and each would be a transaction of
 * its own (and would pull focus back into the editor mid-drag), where the old
 * panel applied the colour once the drag ended.
 */
function onPick(value: unknown) {
  if (typeof value === "string" && isCellColor(value)) applyColor(value);
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
  if (color !== null && !isCellColor(color)) return;
  props.editor?.chain().focus().setCellAttribute("backgroundColor", color).run();
  // The palette stays open while the picker is up, so a colour can be tried
  // and changed without reopening it; a preset closes it, which is the
  // one-click gesture a swatch promises.
  if (!pickerOpen.value) colorOpen.value = false;
}

// A palette left open over a table the caret has since left would apply to
// whatever cell is entered next.
watch(
  () => props.visible,
  (shown) => {
    if (!shown) {
      colorOpen.value = false;
      pickerOpen.value = false;
    }
  },
);
</script>
