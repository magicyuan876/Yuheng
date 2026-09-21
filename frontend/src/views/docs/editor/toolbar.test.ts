import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { EditorState, NodeSelection, TextSelection } from "@tiptap/pm/state";

import { officialExtensions } from "./extensions";
import {
  moveFocus,
  shouldShow,
  TOOLBAR_ITEMS,
  TOOLBAR_OFFSET,
  TOOLBAR_WIDTH,
  toolbarGroups,
  toolbarPlacement,
  visibleItems,
} from "./toolbar";

/** Somebody who may change the page. */
const writer = { editable: true, canComment: true };
/** Somebody who may only remark on it. */
const reader = { editable: false, canComment: true };

const schema = getSchema(officialExtensions());

/** A state holding the given blocks, with no selection of interest yet. */
function stateWith(...blocks: unknown[]): EditorState {
  return EditorState.create({
    schema,
    doc: schema.nodeFromJSON({ type: "doc", content: blocks }),
  });
}

const paragraph = (text: string) => ({ type: "paragraph", content: [{ type: "text", text }] });

test("every entry is distinct and can be named to a screen reader", () => {
  const ids = new Set<string>();
  for (const item of TOOLBAR_ITEMS) {
    assert.ok(!ids.has(item.id), `${item.id} appears twice`);
    ids.add(item.id);
    assert.ok(item.labelKey.startsWith("docs.toolbar."), item.id);
    assert.ok(item.icon !== "", `${item.id} would be an unlabelled blank button`);
  }
});

test("entries are grouped in the order they are listed", () => {
  const groups = toolbarGroups();
  assert.deepEqual(
    groups.map((g) => g[0]!.group),
    ["format", "block", "insert"],
  );
  assert.equal(groups.flat().length, TOOLBAR_ITEMS.length, "no entry is lost between groups");
});

test("nothing is offered when there is nothing selected", () => {
  const state = stateWith(paragraph("hello"));
  assert.equal(shouldShow(state, writer), false);
});

// The rule this work package turns on: a reader may comment, so the bar has
// to appear for them — carrying that button and nothing else.
test("a reader gets a bar with only the comment button on it", () => {
  const base = stateWith(paragraph("hello"));
  const selected = base.apply(base.tr.setSelection(TextSelection.create(base.doc, 1, 4)));

  assert.equal(shouldShow(selected, reader), true);
  const ids = visibleItems(reader).map((item) => item.id);
  assert.deepEqual(ids, ["comment"]);
});

test("a writer gets everything, comment included", () => {
  const ids = visibleItems(writer).map((item) => item.id);
  assert.equal(ids.length, TOOLBAR_ITEMS.length);
  assert.ok(ids.includes("bold"));
  assert.ok(ids.includes("comment"));
});

test("somebody who may neither edit nor comment gets no bar at all", () => {
  const base = stateWith(paragraph("hello"));
  const selected = base.apply(base.tr.setSelection(TextSelection.create(base.doc, 1, 4)));
  const nobody = { editable: false, canComment: false };

  assert.equal(shouldShow(selected, nobody), false);
  assert.deepEqual(visibleItems(nobody), []);
});

test("a writer who may not comment keeps the rest of the bar", () => {
  const ids = visibleItems({ editable: true, canComment: false }).map((item) => item.id);
  assert.ok(!ids.includes("comment"));
  assert.ok(ids.includes("bold"));
});

test("nothing is offered while the document is read-only", () => {
  const base = stateWith(paragraph("hello"));
  const selected = base.apply(base.tr.setSelection(TextSelection.create(base.doc, 1, 4)));
  assert.equal(shouldShow(selected, writer), true, "the selection itself is a usable one");
  assert.equal(shouldShow(selected, { editable: false, canComment: false }), false);
});

// Marks are not stored inside a code block, so a bold button there would
// offer something the document cannot hold.
test("nothing is offered inside a code block", () => {
  const base = stateWith({ type: "codeBlock", content: [{ type: "text", text: "const x = 1" }] });
  const selected = base.apply(base.tr.setSelection(TextSelection.create(base.doc, 1, 6)));
  assert.equal(shouldShow(selected, writer), false);
});

test("nothing is offered for a selected image, which none of these buttons apply to", () => {
  const base = stateWith(paragraph("before"), {
    type: "image",
    attrs: { src: "https://example.test/a.png", attachmentId: null },
  });
  const at = base.doc.resolve(0).nodeAfter!.nodeSize;
  const selected = base.apply(base.tr.setSelection(NodeSelection.create(base.doc, at)));
  assert.equal(selected.selection instanceof NodeSelection, true, "the fixture selects the image");
  assert.equal(shouldShow(selected, writer), false);
});

test("the bar sits centred above the selection", () => {
  const p = toolbarPlacement({ left: 400, top: 300, right: 600, bottom: 320 }, { width: 1200, height: 800 });
  assert.equal(p.below, false);
  assert.equal(p.top, 300 - TOOLBAR_OFFSET);
  assert.equal(p.left, 500 - TOOLBAR_WIDTH / 2, "centred on the middle of the selection");
});

test("the bar flips below rather than being clipped at the top of the window", () => {
  const p = toolbarPlacement({ left: 400, top: 10, right: 600, bottom: 30 }, { width: 1200, height: 800 });
  assert.equal(p.below, true);
  assert.equal(p.top, 38);
});

test("the bar stays on screen next to a selection at either edge", () => {
  const left = toolbarPlacement({ left: 0, top: 300, right: 40, bottom: 320 }, { width: 1200, height: 800 });
  assert.equal(left.left, 8);

  const right = toolbarPlacement({ left: 1160, top: 300, right: 1200, bottom: 320 }, { width: 1200, height: 800 });
  assert.equal(right.left, 1200 - TOOLBAR_WIDTH - 8);
});

test("a window narrower than the bar still places it inside the margin", () => {
  const p = toolbarPlacement({ left: 10, top: 300, right: 200, bottom: 320 }, { width: 320, height: 600 });
  assert.equal(p.left, 8, "clamped rather than pushed off the left edge");
});

// The keyboard criterion: a toolbar is one tab stop and the arrows move
// within it, so the movement has to wrap rather than stick at the ends.
test("focus wraps at both ends of the bar", () => {
  const n = TOOLBAR_ITEMS.length;
  assert.equal(moveFocus(0, 1, n), 1);
  assert.equal(moveFocus(n - 1, 1, n), 0, "past the last entry is the first");
  assert.equal(moveFocus(0, -1, n), n - 1, "before the first is the last");
  assert.equal(moveFocus(3, -1, n), 2);
});

test("focus movement survives a jump larger than the bar", () => {
  assert.equal(moveFocus(0, -5, 3), 1);
  assert.equal(moveFocus(0, 7, 3), 1);
});

test("focus movement on an empty bar goes nowhere rather than producing a NaN", () => {
  assert.equal(moveFocus(0, 1, 0), 0);
  assert.equal(moveFocus(2, -1, 0), 0);
});
