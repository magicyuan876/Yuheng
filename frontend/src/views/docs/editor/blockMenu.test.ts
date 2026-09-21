import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { EditorState, NodeSelection, TextSelection } from "@tiptap/pm/state";

import { blockMenuItems, runBlockAction } from "./blockMenu";
import { NoopExtension, officialExtensions } from "./extensions";

const schema = getSchema(officialExtensions([NoopExtension]));

/** A state holding the given blocks, with no selection of interest yet. */
function stateWith(...blocks: unknown[]): EditorState {
  return EditorState.create({
    schema,
    doc: schema.nodeFromJSON({ type: "doc", content: blocks }),
  });
}

const paragraph = (text: string) => ({
  type: "paragraph",
  attrs: { id: "aaa111" },
  content: [{ type: "text", text }],
});

/** Runs an action and returns the resulting state, like a view would. */
function run(id: string, state: EditorState): EditorState {
  let next = state;
  runBlockAction(
    state,
    (tr) => {
      next = state.apply(tr);
    },
    id,
    0,
  );
  return next;
}

test("convert section mirrors the slash menu labels; actions follow", () => {
  const items = blockMenuItems();
  const convert = items.filter((i) => i.section === "convert").map((i) => i.id);
  assert.deepEqual(convert, [
    "paragraph",
    "heading1",
    "heading2",
    "heading3",
    "bulletList",
    "orderedList",
    "taskList",
    "blockquote",
    "codeBlock",
    "callout",
  ]);
  assert.ok(items.every((i) => i.labelKey.startsWith("docs.")));
});

test("copy block reference is only offered when a copier is supplied", () => {
  assert.ok(!blockMenuItems().some((i) => i.id === "copyBlockRef"));
  assert.ok(blockMenuItems({ copyBlockRef: () => {} }).some((i) => i.id === "copyBlockRef"));
});

test("converting turns a paragraph into a heading of the chosen level", () => {
  const state = run("heading2", stateWith(paragraph("hello"), paragraph("world")));
  assert.equal(state.doc.firstChild?.type.name, "heading");
  assert.equal(state.doc.firstChild?.attrs.level, 2);
});

test("converting to a list wraps the block, not its neighbour", () => {
  const state = run("bulletList", stateWith(paragraph("one"), paragraph("two")));
  assert.equal(state.doc.childCount, 2);
  assert.equal(state.doc.firstChild?.type.name, "bulletList");
  assert.equal(state.doc.lastChild?.type.name, "paragraph");
});

test("the converted block ends up selected as a node", () => {
  const state = run("blockquote", stateWith(paragraph("x"), paragraph("y")));
  assert.ok(state.selection instanceof NodeSelection);
  assert.equal(state.doc.firstChild?.type.name, "blockquote");
});

test("duplicating inserts a copy with a fresh block id after the original", () => {
  const state = run("duplicate", stateWith(paragraph("solo")));
  assert.equal(state.doc.childCount, 2);
  const first = state.doc.child(0);
  const second = state.doc.child(1);
  assert.equal(second.textContent, "solo");
  assert.notEqual(second.attrs.id, first.attrs.id, "the copy must not share the block id");
});

test("deleting a middle block leaves its neighbours in place", () => {
  const base = stateWith(paragraph("a"), paragraph("b"), paragraph("c"));
  const middle = base.doc.child(0).nodeSize;
  let next = base;
  runBlockAction(
    base,
    (tr) => {
      next = base.apply(tr);
    },
    "delete",
    middle,
  );
  assert.equal(next.doc.childCount, 2);
  assert.equal(next.doc.firstChild?.textContent, "a");
  assert.equal(next.doc.lastChild?.textContent, "c");
});

test("deleting the only block leaves an empty paragraph, not an empty doc", () => {
  const state = run("delete", stateWith(paragraph("only")));
  assert.equal(state.doc.childCount, 1);
  assert.equal(state.doc.firstChild?.type.name, "paragraph");
});

test("a text cursor is restored after a delete", () => {
  const state = run("delete", stateWith(paragraph("a"), paragraph("b")));
  assert.ok(state.selection instanceof TextSelection);
});

test("an unknown entry is a no-op rather than an error", () => {
  const state = stateWith(paragraph("x"));
  assert.equal(
    runBlockAction(state, () => {}, "nonsense", 0),
    false,
  );
});
