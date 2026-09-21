// Table operations the table extension does not ship.
//
// @tiptap/extension-table (on prosemirror-tables) covers inserting and
// deleting rows and columns, merging and splitting cells, the header
// toggles, cell attributes and column resizing. It has no notion of
// *reordering* a row or a column, and no way to even out column widths —
// both of which a person editing a table reaches for, and both of which are
// ordinary edits to the table node once the geometry is worked out.
//
// They live here rather than in the toolbar component for the usual reason:
// the geometry is the part worth testing, and it needs neither Vue nor a
// browser to test.
//
// A deliberate limitation. Reordering refuses to run on a table that has any
// merged cell. Swapping two rows under a cell that spans both of them has no
// correct answer — the merge would have to be torn apart or silently moved —
// and a table editor that quietly corrupts a merged table is worse than one
// that greys the button out. `canMoveRow`/`canMoveColumn` report that, so
// the toolbar can disable rather than fail.
import { Fragment } from "@tiptap/pm/model";
import type { Node as PMNode } from "@tiptap/pm/model";
import type { EditorState, Transaction } from "@tiptap/pm/state";

/** Where the selection sits inside a table. */
export interface TableContext {
  /** Document position of the table node. */
  pos: number;
  /** The table node itself. */
  table: PMNode;
  /** Index of the row holding the selection. */
  row: number;
  /** Index of the column holding the selection, by cell count. */
  col: number;
  /** Number of rows. */
  rows: number;
  /** Number of columns in the first row. */
  cols: number;
  /** True when any cell anywhere in the table spans more than one slot. */
  merged: boolean;
}

function isCell(node: PMNode): boolean {
  return node.type.name === "tableCell" || node.type.name === "tableHeader";
}

/**
 * Locates the table around the selection, and the cell inside it.
 *
 * Walks up from the cursor rather than searching the document: a table can
 * hold another table inside a cell, and the one being acted on is always the
 * innermost one the cursor is actually in.
 */
export function tableContext(state: EditorState): TableContext | null {
  const $from = state.selection.$from;
  for (let depth = $from.depth; depth > 0; depth--) {
    if ($from.node(depth).type.name !== "table") continue;
    const table = $from.node(depth);
    const pos = $from.before(depth);

    // depth + 1 is the row, depth + 2 the cell; a selection deeper inside a
    // cell still resolves to the same indices.
    if ($from.depth < depth + 2) return null;
    const row = $from.index(depth);
    const col = $from.index(depth + 1);

    let merged = false;
    table.forEach((rowNode) => {
      rowNode.forEach((cell) => {
        if (!isCell(cell)) return;
        const colspan = Number(cell.attrs.colspan ?? 1);
        const rowspan = Number(cell.attrs.rowspan ?? 1);
        if (colspan > 1 || rowspan > 1) merged = true;
      });
    });

    return {
      pos,
      table,
      row,
      col,
      rows: table.childCount,
      cols: table.firstChild?.childCount ?? 0,
      merged,
    };
  }
  return null;
}

/** Whether the row holding the selection can move by `delta`. */
export function canMoveRow(ctx: TableContext | null, delta: -1 | 1): boolean {
  if (!ctx || ctx.merged) return false;
  const to = ctx.row + delta;
  return to >= 0 && to < ctx.rows;
}

/** Whether the column holding the selection can move by `delta`. */
export function canMoveColumn(ctx: TableContext | null, delta: -1 | 1): boolean {
  if (!ctx || ctx.merged) return false;
  const to = ctx.col + delta;
  if (to < 0 || to >= ctx.cols) return false;
  // A ragged table — rows of differing lengths — has no column to speak of at
  // the index in question. fixTables normally prevents this; refusing is
  // cheaper than guessing which cell was meant.
  let even = true;
  ctx.table.forEach((row) => {
    if (row.childCount !== ctx.cols) even = false;
  });
  return even;
}

function rowsOf(table: PMNode): PMNode[] {
  const rows: PMNode[] = [];
  table.forEach((row) => rows.push(row));
  return rows;
}

/** Replaces the table with `rows`, keeping its type, attributes and marks. */
function rebuild(state: EditorState, ctx: TableContext, rows: PMNode[]): Transaction {
  const table = ctx.table.type.create(ctx.table.attrs, Fragment.fromArray(rows), ctx.table.marks);
  return state.tr.replaceWith(ctx.pos, ctx.pos + ctx.table.nodeSize, table);
}

/** Moves the row holding the selection up (-1) or down (1). */
export function moveRow(state: EditorState, delta: -1 | 1): Transaction | null {
  const ctx = tableContext(state);
  if (!canMoveRow(ctx, delta)) return null;
  const rows = rowsOf(ctx!.table);
  const from = ctx!.row;
  const to = from + delta;
  const moved = rows[from]!;
  rows[from] = rows[to]!;
  rows[to] = moved;
  return rebuild(state, ctx!, rows);
}

/** Moves the column holding the selection left (-1) or right (1). */
export function moveColumn(state: EditorState, delta: -1 | 1): Transaction | null {
  const ctx = tableContext(state);
  if (!canMoveColumn(ctx, delta)) return null;
  const from = ctx!.col;
  const to = from + delta;
  const rows = rowsOf(ctx!.table).map((row) => {
    const cells: PMNode[] = [];
    row.forEach((cell) => cells.push(cell));
    const moved = cells[from]!;
    cells[from] = cells[to]!;
    cells[to] = moved;
    return row.type.create(row.attrs, Fragment.fromArray(cells), row.marks);
  });
  return rebuild(state, ctx!, rows);
}

/**
 * Evens out the column widths.
 *
 * Implemented by clearing `colwidth` on every cell rather than by computing
 * pixel widths, because the stylesheet lays tables out with `table-layout:
 * fixed; width: 100%`: with no explicit width, that divides the table evenly
 * between its columns. It is also the only version of "distribute" that
 * stays correct when the window is resized afterwards, and `colwidth: null`
 * is what the schema already calls the default.
 */
export function distributeColumns(state: EditorState): Transaction | null {
  const ctx = tableContext(state);
  if (!ctx) return null;
  let touched = false;
  const rows = rowsOf(ctx.table).map((row) => {
    const cells: PMNode[] = [];
    row.forEach((cell) => {
      if (isCell(cell) && cell.attrs.colwidth != null) {
        touched = true;
        cells.push(cell.type.create({ ...cell.attrs, colwidth: null }, cell.content, cell.marks));
      } else {
        cells.push(cell);
      }
    });
    return row.type.create(row.attrs, Fragment.fromArray(cells), row.marks);
  });
  if (!touched) return null;
  return rebuild(state, ctx, rows);
}
