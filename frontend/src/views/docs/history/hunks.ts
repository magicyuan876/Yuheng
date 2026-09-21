// Folding a line diff into the parts worth showing.
//
// A comparison of two versions of a long page is mostly lines that did not
// change. Showing all of them buries the three that did; showing none of them
// leaves a change with no context to read it in. So the diff is folded into
// hunks: each run of changes plus a few unchanged lines either side, with the
// long stretches between them collapsed into one row saying how much was
// skipped.
//
// This is a pure function over the server's answer, with no Vue and no DOM,
// so the folding is tested directly — it is the part that is easy to get
// subtly wrong (an off-by-one at the start of a file, a gap collapsed when it
// was shorter than the row that replaces it).

/** One line of the comparison, as the server reports it. */
export interface DiffLine {
  kind: "same" | "added" | "removed";
  text: string;
  old_line?: number;
  new_line?: number;
}

/** A row the view draws: either a line, or a note that lines were skipped. */
export type HunkRow = { type: "line"; line: DiffLine } | { type: "gap"; skipped: number };

/** How many unchanged lines are kept either side of a change. */
export const CONTEXT_LINES = 3;

/**
 * Collapsing a gap only pays when the gap is longer than the row that
 * replaces it plus the context it would have shown. Below that, showing the
 * lines is both shorter and more useful.
 */
export const MIN_GAP = 2;

/**
 * Folds a diff into rows.
 *
 * A comparison with no changes at all folds to nothing rather than to the
 * whole document: "nothing changed" is the answer, and the view says so in a
 * sentence instead of printing a page nobody needs to read.
 */
export function foldDiff(lines: readonly DiffLine[], context = CONTEXT_LINES, minGap = MIN_GAP): HunkRow[] {
  if (lines.length === 0) return [];
  const interesting = lines.some((line) => line.kind !== "same");
  if (!interesting) return [];

  // Which lines are kept: every change, and `context` lines either side.
  const keep = new Array<boolean>(lines.length).fill(false);
  lines.forEach((line, at) => {
    if (line.kind === "same") return;
    const from = Math.max(0, at - context);
    const to = Math.min(lines.length - 1, at + context);
    for (let i = from; i <= to; i++) keep[i] = true;
  });

  const rows: HunkRow[] = [];
  let skipped = 0;
  for (let at = 0; at < lines.length; at++) {
    if (keep[at]) {
      if (skipped > 0) {
        // A gap shorter than the notice replacing it is not worth hiding.
        if (skipped >= minGap) {
          rows.push({ type: "gap", skipped });
        } else {
          for (let i = at - skipped; i < at; i++) {
            rows.push({ type: "line", line: lines[i]! });
          }
        }
        skipped = 0;
      }
      rows.push({ type: "line", line: lines[at]! });
      continue;
    }
    skipped++;
  }
  // A run of unchanged lines at the end is dropped rather than announced:
  // there is nothing after it to give it context.
  return rows;
}

/** Whether a comparison found anything at all. */
export function hasChanges(lines: readonly DiffLine[]): boolean {
  return lines.some((line) => line.kind !== "same");
}

/** One structural change, as the server reports it. */
export interface DiffBlock {
  block_id: string;
  kind: "same" | "added" | "removed" | "moved" | "changed";
  type: string;
  text?: string;
  old_index: number;
  new_index: number;
}

/**
 * Keeps the blocks worth listing, in the order they appear now.
 *
 * Unchanged blocks are dropped: the structural view answers "what happened to
 * this document", and a list where most rows say "nothing" answers it badly.
 *
 * A removed block has no position in the new version, so it is placed after
 * whichever surviving block used to precede it. Sorting it by its own old
 * position would put it wherever that number happens to land among the new
 * ones, which is a different place in the document and reads as nonsense.
 */
export function interestingBlocks(blocks: readonly DiffBlock[]): DiffBlock[] {
  // Where each block that still exists ended up, by its old position.
  const survivors: { oldIndex: number; newIndex: number }[] = [];
  for (const block of blocks) {
    if (block.old_index >= 0 && block.new_index >= 0) {
      survivors.push({ oldIndex: block.old_index, newIndex: block.new_index });
    }
  }
  survivors.sort((a, b) => a.oldIndex - b.oldIndex);

  const keyOf = (block: DiffBlock): number => {
    if (block.new_index >= 0) return block.new_index;
    // Just after the last surviving block that came before it.
    let key = -0.5;
    for (const survivor of survivors) {
      if (survivor.oldIndex < block.old_index) key = survivor.newIndex + 0.5;
      else break;
    }
    return key;
  };

  return (
    blocks
      .filter((block) => block.kind !== "same")
      .map((block, at) => ({ block, key: keyOf(block), at }))
      // `at` keeps the order stable for two blocks that land on the same key,
      // which happens when several blocks were removed from one place.
      .sort((a, b) => a.key - b.key || a.at - b.at)
      .map((entry) => entry.block)
  );
}
