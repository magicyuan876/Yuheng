import assert from 'node:assert/strict'
import { test } from 'vitest'

import { getSchema } from '@tiptap/core'
import { EditorState } from '@tiptap/pm/state'

import { blockAt, canMove, moveBlock, siblingOf } from './blockMove'
import { officialExtensions } from './extensions'

const schema = getSchema(officialExtensions())

const paragraph = (text: string) => ({ type: 'paragraph', content: [{ type: 'text', text }] })

function stateWith(...blocks: unknown[]): EditorState {
  return EditorState.create({ schema, doc: schema.nodeFromJSON({ type: 'doc', content: blocks }) })
}

/** The text of each top-level block, which is what a move rearranges. */
const outline = (state: EditorState) => {
  const out: string[] = []
  state.doc.forEach((node) => out.push(node.textContent))
  return out
}

/** A position inside the nth top-level block. */
function insideBlock(state: EditorState, index: number): number {
  let at = 0
  for (let i = 0; i < index; i++) at += state.doc.child(i).nodeSize
  return at + 1
}

test('a position inside a paragraph finds that paragraph', () => {
  const state = stateWith(paragraph('one'), paragraph('two'))
  const block = blockAt(state, insideBlock(state, 1))
  assert.ok(block)
  assert.equal(block.node.textContent, 'two')
  assert.equal(block.pos, state.doc.child(0).nodeSize)
})

// The decision the whole module turns on: the handle beside a paragraph in a
// list moves the list item, because the paragraph alone has nowhere to go.
test('a paragraph inside a list item moves as the list item', () => {
  const item = (text: string) => ({ type: 'listItem', content: [paragraph(text)] })
  const state = stateWith({ type: 'bulletList', content: [item('a'), item('b')] })
  // Three positions in: doc > bulletList > listItem > paragraph.
  const block = blockAt(state, 3)
  assert.ok(block)
  assert.equal(block.node.type.name, 'listItem', block.node.type.name)
})

test('a block at the end of the document has no neighbour below it', () => {
  const state = stateWith(paragraph('one'), paragraph('two'))
  const last = blockAt(state, insideBlock(state, 1))!
  assert.equal(siblingOf(state, last, 1), null)
  assert.ok(siblingOf(state, last, -1))
})

test('a block at the start has no neighbour above it', () => {
  const state = stateWith(paragraph('one'), paragraph('two'))
  const first = blockAt(state, insideBlock(state, 0))!
  assert.equal(siblingOf(state, first, -1), null)
  assert.ok(siblingOf(state, first, 1))
})

test('moving a block down swaps it with the one below', () => {
  const state = stateWith(paragraph('one'), paragraph('two'), paragraph('three'))
  const tr = moveBlock(state, insideBlock(state, 0), 1)
  assert.ok(tr)
  assert.deepEqual(outline(state.apply(tr)), ['two', 'one', 'three'])
})

test('moving a block up swaps it with the one above', () => {
  const state = stateWith(paragraph('one'), paragraph('two'), paragraph('three'))
  const tr = moveBlock(state, insideBlock(state, 2), -1)
  assert.ok(tr)
  assert.deepEqual(outline(state.apply(tr)), ['one', 'three', 'two'])
})

test('moving down and back up again leaves the document as it was', () => {
  const start = stateWith(paragraph('one'), paragraph('two'), paragraph('three'))
  const down = start.apply(moveBlock(start, insideBlock(start, 0), 1)!)
  const back = down.apply(moveBlock(down, insideBlock(down, 1), -1)!)
  assert.deepEqual(outline(back), outline(start))
})

// An empty transaction would still push an entry onto the undo stack and
// still be sent to every other editor in a collaborative session.
test('a move with nowhere to go produces no transaction at all', () => {
  const state = stateWith(paragraph('only'))
  assert.equal(moveBlock(state, insideBlock(state, 0), 1), null)
  assert.equal(moveBlock(state, insideBlock(state, 0), -1), null)
  assert.equal(canMove(state, insideBlock(state, 0), 1), false)
})

test('canMove answers for both ends of a document', () => {
  const state = stateWith(paragraph('one'), paragraph('two'))
  assert.equal(canMove(state, insideBlock(state, 0), -1), false)
  assert.equal(canMove(state, insideBlock(state, 0), 1), true)
  assert.equal(canMove(state, insideBlock(state, 1), -1), true)
  assert.equal(canMove(state, insideBlock(state, 1), 1), false)
})

test('a block keeps everything inside it when it moves', () => {
  const state = stateWith(
    paragraph('before'),
    { type: 'blockquote', content: [paragraph('quoted'), paragraph('more')] },
  )
  const quoteAt = state.doc.child(0).nodeSize
  const moved = state.apply(moveBlock(state, quoteAt + 2, -1)!)

  const first = moved.doc.child(0)
  assert.equal(first.type.name, 'blockquote')
  assert.equal(first.childCount, 2)
  assert.deepEqual(outline(moved), ['quotedmore', 'before'])
})

test('the moved block stays selected, so the shortcut can be held down', () => {
  const state = stateWith(paragraph('one'), paragraph('two'), paragraph('three'))
  const moved = state.apply(moveBlock(state, insideBlock(state, 0), 1)!)
  const selected = (moved.selection as { node?: { textContent: string } }).node
  assert.equal(selected?.textContent, 'one', 'the block that moved, not whatever it landed next to')
})

test('a column is a slot rather than something to shuffle', () => {
  const column = (text: string) => ({ type: 'column', content: [paragraph(text)] })
  const state = stateWith({ type: 'columns', content: [column('left'), column('right')] })
  // Inside the paragraph in the first column.
  const block = blockAt(state, 3)
  assert.ok(block)
  // The columns row is isolating, so nothing may be moved across its edge;
  // what moves inside it is the paragraph, not the column.
  assert.equal(block.node.type.name, 'paragraph', block.node.type.name)
  assert.equal(siblingOf(state, block, 1), null, 'and it has no sibling to swap with')
})
