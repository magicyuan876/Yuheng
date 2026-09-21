import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { EditorState } from "@tiptap/pm/state";

import { officialExtensions } from "../editor/extensions";

import {
  ACTIVE_CLASS,
  commentDecorationKey,
  commentDecorationPlugin,
  commentDecorations,
  HIGHLIGHT_CLASS,
} from "./decorations";
import type { Placement } from "./placement";

const schema = getSchema(officialExtensions());

function stateWith(text: string): EditorState {
  return EditorState.create({
    schema,
    doc: schema.nodeFromJSON({
      type: "doc",
      content: [{ type: "paragraph", content: [{ type: "text", text }] }],
    }),
    plugins: [commentDecorationPlugin()],
  });
}

/** The attributes of each decoration, in order. */
function attrsOf(set: ReturnType<typeof commentDecorations>, docSize: number) {
  return set.find(0, docSize).map((d) => (d as unknown as { type: { attrs: Record<string, string> } }).type.attrs);
}

const at = (id: string, from: number, to: number, kind: Placement["kind"] = "anchored"): Placement => ({
  id,
  kind,
  from,
  to,
});

test("an inline comment is drawn over the range it covers", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, { placements: [at("c1", 5, 12)] });

  const found = set.find(0, state.doc.content.size);
  assert.equal(found.length, 1);
  assert.equal(found[0]!.from, 5);
  assert.equal(found[0]!.to, 12);
});

test("the thread the reader has open is drawn more strongly", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, {
    placements: [at("c1", 1, 5), at("c2", 6, 10)],
    activeID: "c2",
  });

  const attrs = attrsOf(set, state.doc.content.size);
  assert.ok(!attrs[0]!.class.includes(ACTIVE_CLASS));
  assert.ok(attrs[1]!.class.includes(ACTIVE_CLASS));
});

// A comment placed by its quotation is a guess that happens to be a good one,
// and the reader should be able to tell.
test("a comment placed by its quotation is drawn as approximate", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, { placements: [at("c1", 1, 5, "quoted")] });

  const attrs = attrsOf(set, state.doc.content.size);
  assert.ok(attrs[0]!.class.includes("is-approximate"));
  assert.ok(attrs[0]!.class.includes(HIGHLIGHT_CLASS), "and still a comment highlight");
});

test("comments that are not in the text are not drawn", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, {
    placements: [{ id: "page", kind: "page" }, { id: "lost", kind: "orphaned" }, at("inline", 1, 5)],
  });

  const found = set.find(0, state.doc.content.size);
  assert.equal(found.length, 1);
  assert.equal(attrsOf(set, state.doc.content.size)[0]!["data-comment-id"], "inline");
});

// Two people commenting on overlapping passages is ordinary, and the case
// most likely to be got wrong.
test("overlapping comments are both drawn", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, { placements: [at("c1", 1, 12), at("c2", 5, 20)] });

  const found = set.find(0, state.doc.content.size);
  assert.equal(found.length, 2);
});

test("each highlight carries the id of the thread it belongs to", () => {
  const state = stateWith("the passage being discussed");
  const set = commentDecorations(state.doc, { placements: [at("thread-7", 1, 5)] });

  assert.equal(attrsOf(set, state.doc.content.size)[0]!["data-comment-id"], "thread-7");
});

test("nothing to draw draws nothing", () => {
  const state = stateWith("text");
  assert.equal(commentDecorations(state.doc, { placements: [] }).find(0, 5).length, 0);
});

// ---- the plugin's own state ---------------------------------------------------

test("the plugin starts with nothing and takes what it is handed", () => {
  const state = stateWith("the passage being discussed");
  assert.deepEqual(commentDecorationKey.getState(state)?.placements, []);

  const handed = { placements: [at("c1", 1, 5)], activeID: "c1" };
  const next = state.apply(state.tr.setMeta(commentDecorationKey, handed));
  assert.deepEqual(commentDecorationKey.getState(next), handed);
});

// Drawing stale ranges would put a highlight on words nobody commented on.
test("an edit clears the highlights until they are recomputed", () => {
  const state = stateWith("the passage being discussed");
  const withHighlights = state.apply(state.tr.setMeta(commentDecorationKey, { placements: [at("c1", 1, 5)] }));
  assert.equal(commentDecorationKey.getState(withHighlights)?.placements.length, 1);

  const edited = withHighlights.apply(withHighlights.tr.insertText("new ", 1));
  assert.deepEqual(commentDecorationKey.getState(edited)?.placements, []);
});

test("a transaction that changes nothing keeps the highlights", () => {
  const state = stateWith("the passage being discussed");
  const withHighlights = state.apply(state.tr.setMeta(commentDecorationKey, { placements: [at("c1", 1, 5)] }));

  // A selection move is not a document change.
  const moved = withHighlights.apply(withHighlights.tr.setMeta("irrelevant", true));
  assert.equal(commentDecorationKey.getState(moved)?.placements.length, 1);
});

// The decoration is the whole point: nothing about a comment may reach the
// document, or it would be stored, synced, exported, versioned and indexed.
test("handing over highlights changes no document", () => {
  const state = stateWith("the passage being discussed");
  const before = state.doc.toJSON();

  const next = state.apply(state.tr.setMeta(commentDecorationKey, { placements: [at("c1", 1, 5)] }));
  assert.deepEqual(next.doc.toJSON(), before);
  assert.equal(next.doc.eq(state.doc), true);
});

test("no comment mark exists in the schema to be written by accident", () => {
  assert.ok(!("comment" in schema.marks));
  assert.ok(!("comment" in schema.nodes));
});
