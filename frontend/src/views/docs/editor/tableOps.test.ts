import assert from "node:assert/strict";
import { test } from "vitest";

import { Schema } from "@tiptap/pm/model";
import type { Node as PMNode } from "@tiptap/pm/model";
import { EditorState, TextSelection } from "@tiptap/pm/state";

import { canMoveColumn, canMoveRow, distributeColumns, moveColumn, moveRow, tableContext } from "./tableOps";

// A schema with just enough of the real one to hold a table. The cell
// attributes are the four the shared docs schema allows (see
// packages/docs-schema/schema.json) — deliberately not more, so a test here
// cannot pass on an attribute the server would reject.
const cellAttrs = {
  colspan: { default: 1 },
  rowspan: { default: 1 },
  colwidth: { default: null },
  backgroundColor: { default: null },
};

const schema = new Schema({
  nodes: {
    doc: { content: "block+" },
    paragraph: { group: "block", content: "text*" },
    text: {},
    table: { group: "block", content: "tableRow+", tableRole: "table", isolating: true },
    tableRow: { content: "(tableCell | tableHeader)*", tableRole: "row" },
    tableCell: { content: "block+", attrs: cellAttrs, tableRole: "cell", isolating: true },
    tableHeader: { content: "block+", attrs: cellAttrs, tableRole: "header_cell", isolating: true },
  },
});

const { table, tableRow, tableCell, paragraph, doc } = schema.nodes;

/** A cell holding one paragraph of the given text. */
function cell(text: string, attrs: Record<string, unknown> = {}): PMNode {
  return tableCell!.create(attrs, paragraph!.create(null, text ? schema.text(text) : null));
}

/** A table built from a grid of strings, e.g. [['a','b'],['c','d']]. */
function grid(rows: string[][], attrs: Record<string, unknown>[][] = []): PMNode {
  return table!.create(
    null,
    rows.map((cells, r) =>
      tableRow!.create(
        null,
        cells.map((text, c) => cell(text, attrs[r]?.[c] ?? {})),
      ),
    ),
  );
}

/** A state whose caret sits in the cell at (row, col). */
function stateAt(node: PMNode, row: number, col: number): EditorState {
  const state = EditorState.create({ schema, doc: doc!.create(null, node) });
  // Walk to the cell: 1 (into table) + each preceding row's size, then 1
  // (into the row) + each preceding cell's size, then 1 (into the cell) + 1
  // (into its paragraph).
  let pos = 1 + 1;
  for (let r = 0; r < row; r++) pos += node.child(r).nodeSize;
  pos += 1;
  const rowNode = node.child(row);
  for (let c = 0; c < col; c++) pos += rowNode.child(c).nodeSize;
  pos += 2;
  return state.apply(state.tr.setSelection(TextSelection.near(state.doc.resolve(pos))));
}

/** The grid of cell texts, for comparing against an expectation. */
function textsOf(state: EditorState): string[][] {
  const out: string[][] = [];
  state.doc.firstChild!.forEach((row) => {
    const cells: string[] = [];
    row.forEach((c) => cells.push(c.textContent));
    out.push(cells);
  });
  return out;
}

const SIMPLE = [
  ["a1", "b1"],
  ["a2", "b2"],
  ["a3", "b3"],
];

test("tableContext reports where the caret is", () => {
  const ctx = tableContext(stateAt(grid(SIMPLE), 1, 1));
  assert.ok(ctx);
  assert.equal(ctx.row, 1);
  assert.equal(ctx.col, 1);
  assert.equal(ctx.rows, 3);
  assert.equal(ctx.cols, 2);
  assert.equal(ctx.merged, false);
});

test("tableContext returns null outside a table", () => {
  const state = EditorState.create({ schema, doc: doc!.create(null, paragraph!.create(null, schema.text("hi"))) });
  assert.equal(tableContext(state), null);
});

test("a row moves up, carrying its cells with it", () => {
  const tr = moveRow(stateAt(grid(SIMPLE), 1, 0), -1);
  assert.ok(tr);
  assert.deepEqual(textsOf(EditorState.create({ schema, doc: tr.doc })), [
    ["a2", "b2"],
    ["a1", "b1"],
    ["a3", "b3"],
  ]);
});

test("a row moves down", () => {
  const tr = moveRow(stateAt(grid(SIMPLE), 0, 0), 1);
  assert.ok(tr);
  assert.deepEqual(textsOf(EditorState.create({ schema, doc: tr.doc })), [
    ["a2", "b2"],
    ["a1", "b1"],
    ["a3", "b3"],
  ]);
});

test("the first row cannot move up and the last cannot move down", () => {
  assert.equal(canMoveRow(tableContext(stateAt(grid(SIMPLE), 0, 0)), -1), false);
  assert.equal(canMoveRow(tableContext(stateAt(grid(SIMPLE), 2, 0)), 1), false);
  assert.equal(moveRow(stateAt(grid(SIMPLE), 0, 0), -1), null);
});

test("a column moves left, carrying its cells with it", () => {
  const tr = moveColumn(stateAt(grid(SIMPLE), 0, 1), -1);
  assert.ok(tr);
  assert.deepEqual(textsOf(EditorState.create({ schema, doc: tr.doc })), [
    ["b1", "a1"],
    ["b2", "a2"],
    ["b3", "a3"],
  ]);
});

test("the outermost columns cannot move outwards", () => {
  assert.equal(canMoveColumn(tableContext(stateAt(grid(SIMPLE), 0, 0)), -1), false);
  assert.equal(canMoveColumn(tableContext(stateAt(grid(SIMPLE), 0, 1)), 1), false);
});

test("a column keeps its width when it moves", () => {
  const widths = [[{ colwidth: [90] }, { colwidth: [210] }]];
  const tr = moveColumn(stateAt(grid([["a1", "b1"]], widths), 0, 1), -1);
  assert.ok(tr);
  const row = tr.doc.firstChild!.firstChild!;
  assert.deepEqual(row.child(0).attrs.colwidth, [210]);
  assert.deepEqual(row.child(1).attrs.colwidth, [90]);
});

// The guard that keeps a merged table from being silently corrupted.
test("reordering refuses on a table with a merged cell", () => {
  const merged = grid(
    [
      ["a1", "b1"],
      ["a2", "b2"],
    ],
    [
      [{ rowspan: 2 }, {}],
      [{}, {}],
    ],
  );
  const ctx = tableContext(stateAt(merged, 0, 1));
  assert.equal(ctx!.merged, true);
  assert.equal(canMoveRow(ctx, 1), false);
  assert.equal(canMoveColumn(ctx, -1), false);
  assert.equal(moveRow(stateAt(merged, 0, 1), 1), null);
});

test("distributing clears every column width", () => {
  const widths = [
    [{ colwidth: [90] }, { colwidth: [210] }],
    [{ colwidth: [90] }, { colwidth: [210] }],
  ];
  const tr = distributeColumns(
    stateAt(
      grid(
        [
          ["a1", "b1"],
          ["a2", "b2"],
        ],
        widths,
      ),
      0,
      0,
    ),
  );
  assert.ok(tr);
  tr.doc.firstChild!.forEach((row) => {
    row.forEach((c) => assert.equal(c.attrs.colwidth, null));
  });
});

test("distributing is a no-op when no column carries a width", () => {
  // Reported as "cannot run", so the button greys out rather than pushing an
  // empty change onto the undo stack.
  assert.equal(distributeColumns(stateAt(grid(SIMPLE), 0, 0)), null);
});

test("distributing keeps the cell contents and the other attributes", () => {
  const attrs = [[{ colwidth: [90], backgroundColor: "#fff3bf" }, { colwidth: [210] }]];
  const tr = distributeColumns(stateAt(grid([["a1", "b1"]], attrs), 0, 0));
  assert.ok(tr);
  const row = tr.doc.firstChild!.firstChild!;
  assert.equal(row.child(0).textContent, "a1");
  assert.equal(row.child(0).attrs.backgroundColor, "#fff3bf");
});
