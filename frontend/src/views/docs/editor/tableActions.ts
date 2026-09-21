// What the table toolbar offers, kept apart from the bar that draws it.
//
// The same split as toolbar.ts, for the same reason: which entries a caret
// inside a table should be offered, and whether each one can run at all, is a
// decision worth testing without a browser. The bar itself then only has to
// draw a list and call `run`.
//
// The catalogue is deliberately the set a table is actually edited with:
// rows and columns in and out, reordering either, merge and split, the header
// toggles, a cell colour, even column widths, and a way to remove the table.
// Anything rarer belongs in the slash menu.
//
// Two kinds of entry, because the table extension does not cover all of it.
// Most call a command it already has; the reordering and the width evening
// call into tableOps.ts, which works the table node out by hand.
import type { EditorState, Transaction } from "@tiptap/pm/state";

import { canMoveColumn, canMoveRow, distributeColumns, moveColumn, moveRow, tableContext } from "./tableOps";

/** A chain of editor commands, kept loose because it is the editor's own. */
type Chain = Record<string, (...args: never[]) => unknown>;

/** The narrow view of the editor an entry needs. */
export interface TableTarget {
  chain: () => Chain;
  can: () => { chain: () => Chain };
  /** Needed by the `op` entries, which read the table node directly. */
  state?: EditorState;
  /** Where an `op` entry's transaction is dispatched. */
  view?: { dispatch: (tr: Transaction) => void; focus?: () => void };
}

/** The op entries, as a pair of "can it run" and "run it". */
const OPS: Record<
  NonNullable<TableAction["op"]>,
  {
    can: (state: EditorState) => boolean;
    run: (state: EditorState) => Transaction | null;
  }
> = {
  moveRowUp: {
    can: (s) => canMoveRow(tableContext(s), -1),
    run: (s) => moveRow(s, -1),
  },
  moveRowDown: {
    can: (s) => canMoveRow(tableContext(s), 1),
    run: (s) => moveRow(s, 1),
  },
  moveColumnLeft: {
    can: (s) => canMoveColumn(tableContext(s), -1),
    run: (s) => moveColumn(s, -1),
  },
  moveColumnRight: {
    can: (s) => canMoveColumn(tableContext(s), 1),
    run: (s) => moveColumn(s, 1),
  },
  distributeColumns: {
    // Only worth offering when some column actually carries a width.
    can: (s) => distributeColumns(s) !== null,
    run: (s) => distributeColumns(s),
  },
};

/** One button on the table bar. */
export interface TableAction {
  /** Stable id, used as the key and in tests. */
  id: string;
  /** The i18n key for the accessible name. */
  labelKey: string;
  /** A tdesign icon name. */
  icon: string;
  /** Buttons are drawn in groups with a divider between them. */
  group: "row" | "column" | "cell" | "table";
  /**
   * True for the entry that opens the cell-colour palette rather than
   * running a command, the same arrangement the selection bar uses.
   */
  palette?: boolean;
  /** The command's name on the editor's chain; absent for the palette. */
  command?: string;
  /**
   * An operation from tableOps.ts, for the entries the table extension has
   * no command for: reordering a row or column, and evening out the column
   * widths. Mutually exclusive with `command`.
   */
  op?: "moveRowUp" | "moveRowDown" | "moveColumnLeft" | "moveColumnRight" | "distributeColumns";
  /** Arguments for that command, when it takes any. */
  args?: unknown;
  /** True for an entry that removes something, drawn in the danger colour. */
  danger?: boolean;
}

/**
 * The cell colours the palette offers.
 *
 * Two bands of the same ten hues: a pale one for filling a cell that still
 * has to be read through, and a saturated one for a cell that is meant to
 * shout. The first version of this palette carried only the pale band, which
 * made every swatch look like a slightly different shade of white.
 *
 * Ten per band, so the grid lays out as two even rows.
 */
export const CELL_COLORS_SOFT: readonly string[] = [
  "#ffc9c9",
  "#ffd8a8",
  "#ffec99",
  "#b2f2bb",
  "#96f2d7",
  "#a5d8ff",
  "#bac8ff",
  "#d0bfff",
  "#fcc2d7",
  "#dee2e6",
];

export const CELL_COLORS_STRONG: readonly string[] = [
  "#ff8787",
  "#ffa94d",
  "#ffd43b",
  "#69db7c",
  "#38d9a9",
  "#4dabf7",
  "#748ffc",
  "#9775fa",
  "#f783ac",
  "#adb5bd",
];

/** Both bands, in the order they are drawn. */
export const CELL_COLORS: readonly string[] = [...CELL_COLORS_SOFT, ...CELL_COLORS_STRONG];

/**
 * Whether a colour is one the document may actually carry.
 *
 * The same shapes the server's schema accepts for a `color`-formatted
 * attribute (see the `formats.color` entry in schema.json): a hex triplet of
 * any supported length, a CSS colour name, or an rgb/rgba/hsl/hsla call.
 * Checked here so a colour the picker produced in some other notation is
 * refused while the picker is open, rather than accepted into the document
 * and then rejected by the server on save — by which time the person has
 * moved on and the page silently stops persisting.
 */
export function isCellColor(value: string): boolean {
  return /^(#[0-9a-fA-F]{3,8}|[a-zA-Z]{3,32}|(rgb|rgba|hsl|hsla)\([0-9.,%\s]+\))$/.test(value);
}

/**
 * The bar's contents, in the order they are read and moved through.
 *
 * Entries carrying a `command` call straight into the table extension;
 * entries carrying an `op` are the ones it has no command for (see
 * tableOps.ts).
 */
export const TABLE_ACTIONS: readonly TableAction[] = [
  { id: "addRowBefore", labelKey: "docs.table.addRowBefore", icon: "arrow-up", group: "row", command: "addRowBefore" },
  { id: "addRowAfter", labelKey: "docs.table.addRowAfter", icon: "arrow-down", group: "row", command: "addRowAfter" },
  { id: "moveRowUp", labelKey: "docs.table.moveRowUp", icon: "chevron-up", group: "row", op: "moveRowUp" },
  { id: "moveRowDown", labelKey: "docs.table.moveRowDown", icon: "chevron-down", group: "row", op: "moveRowDown" },
  {
    id: "deleteRow",
    labelKey: "docs.table.deleteRow",
    icon: "minus-rectangle",
    group: "row",
    command: "deleteRow",
    danger: true,
  },

  {
    id: "addColumnBefore",
    labelKey: "docs.table.addColumnBefore",
    icon: "arrow-left",
    group: "column",
    command: "addColumnBefore",
  },
  {
    id: "addColumnAfter",
    labelKey: "docs.table.addColumnAfter",
    icon: "arrow-right",
    group: "column",
    command: "addColumnAfter",
  },
  {
    id: "moveColumnLeft",
    labelKey: "docs.table.moveColumnLeft",
    icon: "chevron-left",
    group: "column",
    op: "moveColumnLeft",
  },
  {
    id: "moveColumnRight",
    labelKey: "docs.table.moveColumnRight",
    icon: "chevron-right",
    group: "column",
    op: "moveColumnRight",
  },
  // A different glyph from deleteRow's: side by side on the bar, two
  // identical minus-rectangles read as the same button twice.
  {
    id: "deleteColumn",
    labelKey: "docs.table.deleteColumn",
    icon: "minus-circle",
    group: "column",
    command: "deleteColumn",
    danger: true,
  },

  { id: "mergeCells", labelKey: "docs.table.mergeCells", icon: "merge-cells", group: "cell", command: "mergeCells" },
  { id: "splitCell", labelKey: "docs.table.splitCell", icon: "table-split", group: "cell", command: "splitCell" },
  { id: "cellColor", labelKey: "docs.table.cellColor", icon: "fill-color", group: "cell", palette: true },

  {
    id: "toggleHeaderRow",
    labelKey: "docs.table.toggleHeaderRow",
    icon: "table-1",
    group: "table",
    command: "toggleHeaderRow",
  },
  {
    id: "toggleHeaderColumn",
    labelKey: "docs.table.toggleHeaderColumn",
    icon: "table-2",
    group: "table",
    command: "toggleHeaderColumn",
  },
  {
    id: "distributeColumns",
    labelKey: "docs.table.distributeColumns",
    icon: "expand-horizontal",
    group: "table",
    op: "distributeColumns",
  },
  {
    id: "deleteTable",
    labelKey: "docs.table.deleteTable",
    icon: "delete",
    group: "table",
    command: "deleteTable",
    danger: true,
  },
];

/**
 * Whether an entry can run on the current selection.
 *
 * Asked of the editor rather than worked out here: merging needs more than
 * one cell selected and splitting needs a merged one, and ProseMirror's own
 * table commands already know both. A button that cannot run is drawn
 * disabled rather than hidden, so the bar does not change width as the caret
 * moves between cells.
 */
export function canRun(editor: TableTarget | null, action: TableAction): boolean {
  if (!editor) return false;
  if (action.palette) return true;
  if (action.op) {
    if (!editor.state) return false;
    try {
      return OPS[action.op].can(editor.state);
    } catch {
      return false;
    }
  }
  if (!action.command) return false;
  try {
    const chain = editor.can().chain() as Chain;
    const call = chain[action.command] as ((...a: never[]) => { run?: () => boolean }) | undefined;
    if (typeof call !== "function") return false;
    return call.call(chain)?.run?.() === true;
  } catch {
    // A command the schema does not have, which happens while extensions are
    // still being swapped on a page change.
    return false;
  }
}

/** Runs an entry. Returns false when it had nothing to call. */
export function runAction(editor: TableTarget | null, action: TableAction): boolean {
  if (!editor) return false;
  if (action.op) {
    if (!editor.state || !editor.view) return false;
    const tr = OPS[action.op].run(editor.state);
    if (!tr) return false;
    editor.view.dispatch(tr);
    editor.view.focus?.();
    return true;
  }
  if (!action.command) return false;
  const chain = (editor.chain() as Chain).focus as unknown as () => Chain;
  const focused = typeof chain === "function" ? chain() : (editor.chain() as Chain);
  const call = (focused as Chain)[action.command] as ((...a: never[]) => { run?: () => void }) | undefined;
  if (typeof call !== "function") return false;
  const result =
    action.args === undefined
      ? call.call(focused)
      : (call as (a: unknown) => { run?: () => void }).call(focused, action.args);
  result?.run?.();
  return true;
}

/** The entries, grouped in order, so the bar can draw dividers. */
export function tableGroups(actions: readonly TableAction[] = TABLE_ACTIONS): TableAction[][] {
  const groups: TableAction[][] = [];
  let current: TableAction["group"] | null = null;
  for (const action of actions) {
    if (action.group !== current) {
      groups.push([]);
      current = action.group;
    }
    groups[groups.length - 1]!.push(action);
  }
  return groups;
}

/** A rectangle in viewport coordinates. */
export interface Rect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

/** Where the bar should be drawn, in viewport coordinates. */
export interface TablePlacement {
  left: number;
  top: number;
}

/** The width the bar is laid out at, used to keep it on screen. */
export const TABLE_TOOLBAR_WIDTH = 560;
/** The bar's height plus the gap it keeps from the table. */
export const TABLE_TOOLBAR_OFFSET = 44;

/**
 * Places the bar over the table's top-left corner, kept inside the viewport.
 *
 * Left-aligned rather than centred, unlike the selection bar: a table can be
 * as wide as the page, and a bar centred on it would sit a long way from the
 * cell somebody is actually working in. It flips underneath the table's top
 * edge when the table starts too near the top of the window to fit above.
 */
export function tableToolbarPlacement(
  rect: Rect,
  viewport: { width: number; height: number },
  margin = 8,
): TablePlacement {
  const left = Math.min(Math.max(margin, rect.left), Math.max(margin, viewport.width - TABLE_TOOLBAR_WIDTH - margin));
  const above = rect.top - TABLE_TOOLBAR_OFFSET;
  if (above >= margin) return { left, top: above };
  return { left, top: Math.min(rect.top + 8, viewport.height - TABLE_TOOLBAR_OFFSET) };
}
