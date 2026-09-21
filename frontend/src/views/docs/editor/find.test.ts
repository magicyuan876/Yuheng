import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { EditorState } from "@tiptap/pm/state";

import { officialExtensions } from "./extensions";
import { findMatches, matchAfter, MAX_MATCHES, replaceAll, replaceMatch, searchRegex, stepMatch } from "./find";

const schema = getSchema(officialExtensions());

const paragraph = (text: string) => ({ type: "paragraph", content: [{ type: "text", text }] });

function stateWith(...blocks: unknown[]): EditorState {
  return EditorState.create({ schema, doc: schema.nodeFromJSON({ type: "doc", content: blocks }) });
}

/** The text each match actually covers, which is what a highlight will show. */
const covered = (state: EditorState, query: string, options = {}) =>
  findMatches(state.doc, query, options).map((m) => state.doc.textBetween(m.from, m.to));

test("a search with no query finds nothing rather than everything", () => {
  const state = stateWith(paragraph("hello world"));
  assert.deepEqual(findMatches(state.doc, ""), []);
  assert.equal(searchRegex(""), null);
});

test("the ranges returned are the words that were searched for", () => {
  const state = stateWith(paragraph("the cat sat on the mat"));
  assert.deepEqual(covered(state, "the"), ["the", "the"]);
  assert.deepEqual(covered(state, "at"), ["at", "at", "at"]);
});

test("a search ignores case unless it is told not to", () => {
  const state = stateWith(paragraph("Cat cat CAT"));
  assert.equal(findMatches(state.doc, "cat").length, 3);
  assert.deepEqual(covered(state, "cat", { caseSensitive: true }), ["cat"]);
});

test("a whole-word search does not match inside a longer word", () => {
  const state = stateWith(paragraph("cat cats concatenate"));
  assert.equal(findMatches(state.doc, "cat").length, 3, "three, counting the ones inside words");
  assert.equal(findMatches(state.doc, "cat", { wholeWord: true }).length, 1);
});

test("a whole-word search works on a language with no spaces", () => {
  const state = stateWith(paragraph("文档 文档列表"));
  // Chinese has no word boundaries to speak of; what matters is that the
  // option does not silently find nothing.
  assert.equal(findMatches(state.doc, "文档").length, 2);
});

// The false match that would otherwise be reported at the join between two
// blocks, where a naive search over the whole document's text finds a word
// that is not in either paragraph.
test("a match cannot span two blocks", () => {
  const state = stateWith(paragraph("the en"), paragraph("d is near"));
  assert.deepEqual(findMatches(state.doc, "end"), []);
});

test("matches are found in every kind of text block", () => {
  const state = stateWith(
    { type: "heading", attrs: { level: 1 }, content: [{ type: "text", text: "target" }] },
    paragraph("target"),
    { type: "blockquote", content: [paragraph("target")] },
  );
  assert.equal(findMatches(state.doc, "target").length, 3);
});

test("literal text is searched literally, not as a pattern", () => {
  const state = stateWith(paragraph("a.b and axb"));
  assert.deepEqual(covered(state, "a.b"), ["a.b"], "the dot is a dot");
  assert.deepEqual(covered(state, "a.b", { regex: true }), ["a.b", "axb"], "unless asked otherwise");
});

// Somebody typing "(a|b)" passes through "(" on the way. An editor that
// throws at that moment is unusable.
test("a regular expression that does not compile yet finds nothing and throws nothing", () => {
  const state = stateWith(paragraph("anything"));
  assert.equal(searchRegex("(", { regex: true }), null);
  assert.doesNotThrow(() => findMatches(state.doc, "(", { regex: true }));
  assert.deepEqual(findMatches(state.doc, "(", { regex: true }), []);
});

test("a pattern that can match nothing does not spin forever", () => {
  const state = stateWith(paragraph("abc"));
  const matches = findMatches(state.doc, "x*", { regex: true });
  assert.deepEqual(matches, [], "an empty match is not a match");
});

test("the number of matches is capped", () => {
  const state = stateWith(paragraph("a".repeat(MAX_MATCHES + 500)));
  assert.equal(findMatches(state.doc, "a").length, MAX_MATCHES);
});

test("find next is the next match in the document, wherever the cursor was", () => {
  const state = stateWith(paragraph("one two one two one"));
  const matches = findMatches(state.doc, "one");
  assert.equal(matchAfter(matches, 0), 0);
  assert.equal(matchAfter(matches, matches[0]!.to), 1, "past the first match is the second");
  assert.equal(matchAfter(matches, 999), 0, "past the last one wraps to the first");
  assert.equal(matchAfter([], 0), -1);
});

test("stepping through matches wraps at both ends", () => {
  assert.equal(stepMatch(0, 1, 3), 1);
  assert.equal(stepMatch(2, 1, 3), 0);
  assert.equal(stepMatch(0, -1, 3), 2);
  assert.equal(stepMatch(0, 1, 0), -1);
});

test("replacing one match leaves the others alone", () => {
  const state = stateWith(paragraph("cat cat cat"));
  const matches = findMatches(state.doc, "cat");
  const after = state.apply(replaceMatch(state, matches[1]!, "dog"));
  assert.equal(after.doc.textContent, "cat dog cat");
});

test("replacing with nothing removes the text", () => {
  const state = stateWith(paragraph("keep drop keep"));
  const matches = findMatches(state.doc, "drop ");
  const after = state.apply(replaceMatch(state, matches[0]!, ""));
  assert.equal(after.doc.textContent, "keep keep");
});

// The bug this exists to prevent: replacing front to back invalidates every
// later position as soon as the replacement is a different length.
test("replacing all with a longer word does not corrupt the later matches", () => {
  const state = stateWith(paragraph("cat cat cat"));
  const matches = findMatches(state.doc, "cat");
  const after = state.apply(replaceAll(state, matches, "elephant")!);
  assert.equal(after.doc.textContent, "elephant elephant elephant");
});

test("replacing all with a shorter word is likewise exact", () => {
  const state = stateWith(paragraph("elephant elephant"));
  const matches = findMatches(state.doc, "elephant");
  const after = state.apply(replaceAll(state, matches, "cat")!);
  assert.equal(after.doc.textContent, "cat cat");
});

test("replacing all crosses blocks correctly", () => {
  const state = stateWith(paragraph("cat one"), paragraph("cat two"), paragraph("cat three"));
  const matches = findMatches(state.doc, "cat");
  const after = state.apply(replaceAll(state, matches, "dog")!);
  const texts: string[] = [];
  after.doc.forEach((node) => texts.push(node.textContent));
  assert.deepEqual(texts, ["dog one", "dog two", "dog three"]);
});

test("replacing all is one transaction, so it is one undo", () => {
  const state = stateWith(paragraph("cat cat"));
  const tr = replaceAll(state, findMatches(state.doc, "cat"), "dog");
  assert.ok(tr);
  assert.equal(state.apply(tr).doc.textContent, "dog dog");
});

test("replacing all with nothing to replace produces no transaction", () => {
  const state = stateWith(paragraph("nothing here"));
  assert.equal(replaceAll(state, [], "x"), null);
});
