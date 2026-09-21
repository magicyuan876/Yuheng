import assert from 'node:assert/strict'
import { test } from 'vitest'

import { getSchema } from '@tiptap/core'
import { EditorState } from '@tiptap/pm/state'

import { officialExtensions } from './extensions'
import {
  addPlaceholder,
  placeholderKeys,
  placeholderPos,
  removePlaceholder,
  uploadPlaceholderPlugin,
} from './uploadPlaceholder'

const schema = getSchema(officialExtensions())

/** An editor state with the placeholder plugin and one paragraph of text. */
function stateWith(text: string): EditorState {
  return EditorState.create({
    schema,
    doc: schema.node('doc', null, [
      schema.node('paragraph', null, text ? [schema.text(text)] : []),
    ]),
    // The renderer is never called without a view, which is the point: this
    // whole module is testable with no DOM.
    plugins: [uploadPlaceholderPlugin(() => {
      throw new Error('a placeholder must not be rendered without a view')
    })],
  })
}

test('a placeholder is remembered where the file was dropped', () => {
  let state = stateWith('hello world')
  assert.equal(placeholderPos(state, 'u1'), null)

  state = state.apply(addPlaceholder(state.tr, 'u1', 6))
  assert.equal(placeholderPos(state, 'u1'), 6)
  assert.deepEqual(placeholderKeys(state), ['u1'])
})

test('a placeholder follows the text when the document changes around it', () => {
  let state = stateWith('hello world')
  state = state.apply(addPlaceholder(state.tr, 'u1', 6))

  // Typing before the insertion point must carry the placeholder along, or
  // the finished image would land in the wrong place.
  state = state.apply(state.tr.insertText('XYZ', 1))
  assert.equal(placeholderPos(state, 'u1'), 9)

  // Typing after it leaves it alone.
  state = state.apply(state.tr.insertText('!', 14))
  assert.equal(placeholderPos(state, 'u1'), 9)
})

test('a placeholder disappears when the text it sat in is deleted', () => {
  let state = stateWith('hello world')
  state = state.apply(addPlaceholder(state.tr, 'u1', 6))

  state = state.apply(state.tr.delete(1, 12))
  assert.equal(placeholderPos(state, 'u1'), null,
    'an upload whose insertion point is gone has nowhere to land')
})

test('several uploads at once are tracked separately', () => {
  let state = stateWith('hello world')
  state = state.apply(addPlaceholder(state.tr, 'u1', 3))
  state = state.apply(addPlaceholder(state.tr, 'u2', 9))
  assert.deepEqual(placeholderKeys(state).sort(), ['u1', 'u2'])
  assert.equal(placeholderPos(state, 'u1'), 3)
  assert.equal(placeholderPos(state, 'u2'), 9)

  state = state.apply(removePlaceholder(state.tr, 'u1'))
  assert.equal(placeholderPos(state, 'u1'), null)
  assert.equal(placeholderPos(state, 'u2'), 9, 'finishing one upload must not disturb the other')
})

test('removing a placeholder that is already gone is harmless', () => {
  let state = stateWith('hello')
  state = state.apply(removePlaceholder(state.tr, 'nobody'))
  assert.deepEqual(placeholderKeys(state), [])
})

test('placeholders are reported to the view as decorations, not as content', () => {
  let state = stateWith('hello world')
  const before = state.doc.toJSON()
  state = state.apply(addPlaceholder(state.tr, 'u1', 6))

  assert.deepEqual(state.doc.toJSON(), before,
    'a placeholder must never reach the document, which is validated and shared')
})
