import assert from "node:assert/strict";
import { test } from "vitest";

import { CONTEXT_LINES, foldDiff, hasChanges, interestingBlocks, type DiffBlock, type DiffLine } from "./hunks";

const same = (text: string): DiffLine => ({ kind: "same", text });
const added = (text: string): DiffLine => ({ kind: "added", text });
const removed = (text: string): DiffLine => ({ kind: "removed", text });

/** Renders folded rows compactly, so a test reads as the answer. */
function shape(rows: ReturnType<typeof foldDiff>): string {
  return rows
    .map((row) => {
      if (row.type === "gap") return `…${row.skipped}`;
      switch (row.line.kind) {
        case "added":
          return "+";
        case "removed":
          return "-";
        default:
          return "=";
      }
    })
    .join("");
}

function lines(n: number, make = same): DiffLine[] {
  return Array.from({ length: n }, (_, i) => make(`line ${i}`));
}

// "Nothing changed" is the answer; printing the whole document instead is
// not an improvement on saying so.
test("a comparison with no changes folds to nothing", () => {
  assert.deepEqual(foldDiff(lines(50)), []);
  assert.deepEqual(foldDiff([]), []);
  assert.equal(hasChanges(lines(50)), false);
});

test("a short comparison is shown whole", () => {
  const rows = foldDiff([same("a"), added("b"), same("c")]);
  assert.equal(shape(rows), "=+=");
});

test("a change keeps a few unchanged lines either side of it", () => {
  const diff = [...lines(20), added("new"), ...lines(20)];
  const rows = foldDiff(diff);

  // Gap, three lines of context, the change, three more — and nothing for the
  // unchanged run that follows, which has nothing after it to give context.
  assert.equal(shape(rows), `…${20 - CONTEXT_LINES}===+===`);
});

test("the long stretch between two changes is collapsed once", () => {
  const diff = [added("first"), ...lines(40), removed("second")];
  const rows = foldDiff(diff);
  const gaps = rows.filter((row) => row.type === "gap");

  assert.equal(gaps.length, 1);
  assert.equal(shape(rows), `+===…${40 - CONTEXT_LINES * 2}===-`);
});

// Hiding two lines behind a row that says "2 lines hidden" is worse than
// showing them.
test("a gap shorter than the notice replacing it is shown instead", () => {
  const diff = [added("a"), same("x"), removed("b")];
  assert.equal(shape(foldDiff(diff, 0, 2)), "+=-", "one line between two changes stays");

  const wider = [added("a"), ...lines(2), removed("b")];
  assert.equal(shape(foldDiff(wider, 0, 2)), "+…2-", "two are worth hiding");
});

test("a change at the very start of the document has no gap before it", () => {
  const rows = foldDiff([added("first"), ...lines(30)]);
  assert.equal(rows[0]?.type, "line");
  assert.equal(shape(rows), "+===");
});

test("a change at the very end has no gap after it", () => {
  const rows = foldDiff([...lines(30), added("last")]);
  assert.equal(rows.at(-1)?.type, "line");
  assert.equal(shape(rows), `…${30 - CONTEXT_LINES}===+`);
});

// A trailing stretch of unchanged lines has nothing after it to give it
// context, so announcing it says nothing — the same choice git makes.
test("unchanged lines after the last change are dropped, not announced", () => {
  const rows = foldDiff([added("change"), ...lines(30)]);
  assert.equal(rows.filter((row) => row.type === "gap").length, 0);
  assert.equal(rows.at(-1)?.type, "line");
  assert.equal(shape(rows), "+===", "the change and its context, and nothing else");
});

test("every line of the diff is either shown or counted in a gap", () => {
  const diff = [...lines(15), added("a"), ...lines(15), removed("b"), ...lines(15)];
  const rows = foldDiff(diff);

  let accounted = 0;
  for (const row of rows) accounted += row.type === "gap" ? row.skipped : 1;
  // The trailing unchanged run is deliberately dropped; everything before the
  // last change is accounted for exactly once.
  const trailing = 15 - CONTEXT_LINES;
  assert.equal(accounted, diff.length - trailing);
});

test("adjacent changes are one hunk rather than several", () => {
  const rows = foldDiff([...lines(10), removed("old"), added("new"), ...lines(10)]);
  assert.equal(rows.filter((row) => row.type === "gap").length, 1);
  assert.equal(shape(rows), `…${10 - CONTEXT_LINES}===-+===`);
});

// ---- structural changes -------------------------------------------------------

const block = (id: string, kind: DiffBlock["kind"], oldIndex: number, newIndex: number): DiffBlock => ({
  block_id: id,
  kind,
  type: "paragraph",
  old_index: oldIndex,
  new_index: newIndex,
});

// A list where most rows say "nothing happened" answers the question badly.
test("unchanged blocks are left out of the structural list", () => {
  const blocks = [block("blockaa", "same", 0, 0), block("blockbb", "changed", 1, 1), block("blockcc", "same", 2, 2)];
  assert.deepEqual(
    interestingBlocks(blocks).map((b) => b.block_id),
    ["blockbb"],
  );
});

test("changes are listed in the order they appear now", () => {
  const blocks = [block("third", "added", -1, 2), block("first", "changed", 0, 0), block("second", "moved", 3, 1)];
  assert.deepEqual(
    interestingBlocks(blocks).map((b) => b.block_id),
    ["first", "second", "third"],
  );
});

// A removed block has no position in the new version; it should read where it
// was rather than at the end of the list.
test("a removed block sits where it used to be", () => {
  const blocks = [block("kept1", "changed", 0, 0), block("gone", "removed", 1, -1), block("kept2", "changed", 2, 1)];
  assert.deepEqual(
    interestingBlocks(blocks).map((b) => b.block_id),
    ["kept1", "gone", "kept2"],
  );
});

test("the input is not reordered under the caller", () => {
  const blocks = [block("b", "added", -1, 1), block("a", "changed", 0, 0)];
  const original = [...blocks];
  interestingBlocks(blocks);
  assert.deepEqual(blocks, original);
});

test("a structural comparison with nothing in it is empty", () => {
  assert.deepEqual(interestingBlocks([]), []);
  assert.deepEqual(interestingBlocks([block("a", "same", 0, 0)]), []);
});
