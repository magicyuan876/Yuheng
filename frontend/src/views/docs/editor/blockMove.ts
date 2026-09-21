// Moving a block up or down.
//
// This is the logic behind both the drag handle and the keyboard shortcut,
// which matters more than it sounds: a drag handle that can only be dragged
// fails the acceptance criterion that every menu is operable from the
// keyboard, and two implementations of "move this block" would drift apart.
// So there is one function, and the handle and the shortcut both call it.
//
// It works on positions rather than on the DOM, so it is exercised by tests
// against a real ProseMirror document with no browser involved.
import type { Node as PMNode } from "@tiptap/pm/model";
import type { EditorState, Transaction } from "@tiptap/pm/state";
import { NodeSelection } from "@tiptap/pm/state";

/** A block in the document, identified by where it starts. */
export interface BlockRef {
  /** The position just before the node. */
  pos: number;
  /** The position just after it. */
  end: number;
  node: PMNode;
  /** How deep it sits; 0 is a direct child of the document. */
  depth: number;
}

/**
 * Node types that are a place for content rather than a piece of it.
 *
 * A column, a table cell, a list item's own wrapper: dragging one of these
 * around would not mean anything, and moving a block *within* one is exactly
 * what somebody dragging a handle inside a column expects. So the search
 * walks through them rather than stopping on them.
 */
export const SLOT_TYPES = new Set([
  "column",
  "tableCell",
  "tableHeader",
  "tableRow",
  "detailsSummary",
  "detailsContent",
]);

/**
 * Finds the block a position is inside, at the outermost level that can move.
 *
 * "Outermost that can move" is the whole decision here. Dragging the handle
 * beside a paragraph inside a list item should move the list item, not the
 * paragraph out of it — the paragraph alone has nowhere to go that the schema
 * would accept. So it walks *outwards* from the cursor and takes the highest
 * ancestor that has somewhere to go. A slot is walked through, so a paragraph
 * in a column moves within that column rather than dragging the column.
 */
export function blockAt(state: EditorState, pos: number): BlockRef | null {
  const $pos = state.doc.resolve(Math.max(0, Math.min(pos, state.doc.content.size)));

  // Outwards in: the first level that actually has somewhere to go, which is
  // the first one whose container holds more than just it. A lone bullet list
  // wrapping the cursor is not what somebody means by "move this"; the item
  // inside it is.
  for (let depth = 1; depth <= $pos.depth; depth++) {
    const node = $pos.node(depth);
    if (SLOT_TYPES.has(node.type.name)) continue;
    if ($pos.node(depth - 1).childCount > 1) {
      const start = $pos.before(depth);
      return { pos: start, end: start + node.nodeSize, node, depth };
    }
  }

  // Nothing above it has siblings, so the innermost block is what moves —
  // it will find no neighbour either, and the handle says so.
  if ($pos.depth >= 1) {
    const node = $pos.node($pos.depth);
    const start = $pos.before($pos.depth);
    return { pos: start, end: start + node.nodeSize, node, depth: $pos.depth };
  }

  // The position sits directly between two top-level blocks.
  const node = $pos.nodeAfter ?? $pos.nodeBefore;
  if (!node) return null;
  const start = $pos.nodeAfter ? $pos.pos : $pos.pos - node.nodeSize;
  return { pos: start, end: start + node.nodeSize, node, depth: 1 };
}

/**
 * The sibling a block would swap with, or null at either end.
 *
 * Exposed separately so a view can grey out a handle rather than offering a
 * move that does nothing.
 */
export function siblingOf(state: EditorState, block: BlockRef, direction: -1 | 1): BlockRef | null {
  const $pos = state.doc.resolve(block.pos);
  const parent = $pos.parent;
  const index = $pos.index();
  const wanted = index + direction;
  if (wanted < 0 || wanted >= parent.childCount) return null;

  const node = parent.child(wanted);
  const start = direction === 1 ? block.end : block.pos - node.nodeSize;
  return { pos: start, end: start + node.nodeSize, node, depth: block.depth };
}

/**
 * Builds the transaction that swaps a block with its neighbour.
 *
 * Returns null when there is nothing to swap with, so a caller can report
 * "no" by returning false from a command rather than dispatching an empty
 * transaction — an empty transaction would still push an entry onto the undo
 * stack and still reach every other editor in a collaborative session.
 *
 * The swap is written as a delete followed by an insert rather than as two
 * replacements, because the two blocks are adjacent: removing one and putting
 * it back on the other side of its neighbour is one continuous edit, which
 * merges cleanly when two people move things at once.
 */
export function moveBlock(state: EditorState, pos: number, direction: -1 | 1): Transaction | null {
  const block = blockAt(state, pos);
  if (!block) return null;
  const sibling = siblingOf(state, block, direction);
  if (!sibling) return null;

  const tr = state.tr;
  const slice = state.doc.slice(block.pos, block.end);
  tr.delete(block.pos, block.end);

  // Where the block lands, in the document as it is after the delete: moving
  // down, the neighbour has shifted back by the size of what was removed.
  const target = direction === 1 ? sibling.end - block.node.nodeSize : sibling.pos;

  tr.insert(target, slice.content);
  // Keep the moved block selected, so a person holding the shortcut keeps
  // moving the same block rather than whatever ended up under the cursor.
  const $target = tr.doc.resolve(target);
  if ($target.nodeAfter) {
    tr.setSelection(NodeSelection.create(tr.doc, target));
  }
  tr.scrollIntoView();
  return tr;
}

/** Whether a block could move in the given direction. */
export function canMove(state: EditorState, pos: number, direction: -1 | 1): boolean {
  const block = blockAt(state, pos);
  return block !== null && siblingOf(state, block, direction) !== null;
}
