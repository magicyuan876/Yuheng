import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { EditorState, TextSelection, type Plugin } from "@tiptap/pm/state";

import { officialExtensions } from "./extensions";
import { BLOCK_ID_TYPES, BlockId, TEXT_BLOCK_TYPES, TextBlockAttrs } from "./blockAttrs";

const schema = getSchema(officialExtensions());

/**
 * Extracts the plugin(s) an Extension contributes without going through a
 * full Tiptap `Editor` (which needs a DOM). Safe here because neither
 * extension's `addProseMirrorPlugins` reads `this` -- see the comment on
 * `BlockId` in blockAttrs.ts.
 */
function pluginsOf(ext: { config: { addProseMirrorPlugins?: () => Plugin[] } }): Plugin[] {
  return ext.config.addProseMirrorPlugins?.call({}) ?? [];
}

function freshState(): EditorState {
  return EditorState.create({ schema, plugins: pluginsOf(BlockId) });
}

function insertParagraph(state: EditorState, text: string): EditorState {
  const paragraph = schema.nodes.paragraph!.create({}, text ? schema.text(text) : undefined);
  return state.apply(state.tr.insert(state.doc.content.size, paragraph));
}

test("a newly inserted paragraph gets a block id matching the schema format", () => {
  let state = freshState();
  state = insertParagraph(state, "hello");
  const node = state.doc.child(state.doc.childCount - 1);
  assert.match(node.attrs.id as string, /^[A-Za-z0-9_-]{6,40}$/);
});

test("two nodes inserted together get two different ids", () => {
  let state = freshState();
  const a = schema.nodes.paragraph!.create({}, schema.text("a"));
  const b = schema.nodes.paragraph!.create({}, schema.text("b"));
  state = state.apply(state.tr.insert(0, a).insert(0, b));
  const ids = new Set<string>();
  state.doc.forEach((node) => ids.add(node.attrs.id as string));
  assert.equal(ids.size, state.doc.childCount, "every inserted node has its own id");
});

test("a node that already has an id keeps it on a later, unrelated transaction", () => {
  let state = freshState();
  state = insertParagraph(state, "first");
  const assignedId = state.doc.child(0).attrs.id as string;
  assert.ok(assignedId);

  state = insertParagraph(state, "second");
  assert.equal(state.doc.child(0).attrs.id, assignedId, "the first paragraph was not touched again");
  assert.notEqual(state.doc.child(1).attrs.id, assignedId, "the second paragraph got its own id");
});

test("a selection-only transaction is left alone (no doc-changing step is appended)", () => {
  let state = freshState();
  state = insertParagraph(state, "hello");
  const before = state.doc;
  const tr = state.tr.setSelection(TextSelection.create(state.doc, 1));
  state = state.apply(tr);
  assert.equal(state.doc, before, "no new steps were appended for a selection-only change");
});

test("every node type the schema gives docs-schema’s blockId attribute set has the id attribute", () => {
  for (const type of BLOCK_ID_TYPES) {
    assert.ok("id" in (schema.nodes[type]?.spec.attrs ?? {}), `${type} is missing the id attribute`);
  }
});

test("TextBlockAttrs defaults textAlign to left and indent to 0", () => {
  let state = EditorState.create({ schema, plugins: pluginsOf(TextBlockAttrs) });
  state = insertParagraph(state, "x");
  const node = state.doc.child(state.doc.childCount - 1);
  assert.equal(node.attrs.textAlign, "left");
  assert.equal(node.attrs.indent, 0);
  for (const type of TEXT_BLOCK_TYPES) {
    assert.ok("textAlign" in (schema.nodes[type]?.spec.attrs ?? {}));
    assert.ok("indent" in (schema.nodes[type]?.spec.attrs ?? {}));
  }
});
