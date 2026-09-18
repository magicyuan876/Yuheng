import assert from 'node:assert/strict'
import { test } from 'node:test'

import { getSchema } from '@tiptap/core'
import { EditorState, TextSelection } from '@tiptap/pm/state'

import { officialExtensions } from './extensions'
import {
  activeTrigger,
  findTrigger,
  moveSelection,
  sameTrigger,
  suggestionPlugin,
  type ActiveTrigger,
  type Trigger,
} from './suggestion'

const schema = getSchema(officialExtensions())

const TRIGGERS: Trigger[] = [
  { name: 'page', chars: '[[' },
  { name: 'mention', chars: '@', requireBoundary: true },
]

/** A state whose cursor sits after `text` in a single paragraph. */
function stateAfter(text: string, cursor = text.length): EditorState {
  const doc = schema.node('doc', null, [
    schema.node('paragraph', null, text ? [schema.text(text)] : []),
  ])
  const state = EditorState.create({ schema, doc })
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, 1 + cursor)))
}

function trigger(text: string, cursor?: number): ActiveTrigger | null {
  return findTrigger(stateAfter(text, cursor), TRIGGERS)
}

test('typing the trigger opens the menu with an empty query', () => {
  const open = trigger('see [[')
  assert.equal(open?.name, 'page')
  assert.equal(open?.query, '')
  assert.equal(open?.from, 5, 'the trigger starts where its characters do')
})

test('what is typed after the trigger becomes the query', () => {
  assert.equal(trigger('see [[hand')?.query, 'hand')
  assert.equal(trigger('see [[hand book')?.query, 'hand book', 'a space does not close a page menu')
})

test('a mention only triggers at a word boundary', () => {
  assert.equal(trigger('@ali')?.name, 'mention', 'at the start of a line')
  assert.equal(trigger('ask @ali')?.name, 'mention', 'after a space')
  assert.equal(trigger('mail me at alice@example.test'), null, 'but not inside an email address')
})

test('the menu closes when the cursor leaves the trigger', () => {
  // Cursor before the trigger.
  assert.equal(trigger('see [[hand', 2), null)
  // A different block entirely.
  const doc = schema.node('doc', null, [
    schema.node('paragraph', null, [schema.text('see [[')]),
    schema.node('paragraph', null, [schema.text('elsewhere')]),
  ])
  let state = EditorState.create({ schema, doc })
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, state.doc.content.size - 1)))
  assert.equal(findTrigger(state, TRIGGERS), null, 'a trigger cannot reach across a paragraph')
})

test('the menu closes rather than matching forever', () => {
  const long = 'x'.repeat(80)
  assert.equal(trigger(`see [[${long}`), null)
  assert.equal(trigger(`see [[${'x'.repeat(10)}`)?.query.length, 10)
})

test('a selection with a range opens nothing', () => {
  const base = stateAfter('see [[hand')
  const ranged = base.apply(base.tr.setSelection(TextSelection.create(base.doc, 2, 6)))
  assert.equal(findTrigger(ranged, TRIGGERS), null, 'a menu belongs under a caret, not a selection')
})

test('the nearest trigger wins when two could match', () => {
  // Typing an @ inside a page query means the mention menu now.
  const open = trigger('see [[hand @ali')
  assert.equal(open?.name, 'mention')
  assert.equal(open?.query, 'ali')
})

test('the plugin reports a change once, and only when something changed', () => {
  const seen: (ActiveTrigger | null)[] = []
  const doc = schema.node('doc', null, [schema.node('paragraph')])
  let state = EditorState.create({
    schema, doc,
    plugins: [suggestionPlugin(TRIGGERS, (active) => seen.push(active))],
  })
  assert.equal(activeTrigger(state), null)

  state = state.apply(state.tr.insertText('[[', 1))
  assert.equal(activeTrigger(state)?.query, '')
  assert.equal(seen.length, 1)

  state = state.apply(state.tr.insertText('ha', 3))
  assert.equal(activeTrigger(state)?.query, 'ha')
  assert.equal(seen.length, 2)

  // A transaction that changes nothing relevant must not re-notify.
  const before = activeTrigger(state)
  state = state.apply(state.tr.setMeta('unrelated', true))
  assert.equal(seen.length, 2)
  assert.equal(activeTrigger(state), before, 'the same object, so a view does not re-render')

  // Deleting the trigger closes the menu.
  state = state.apply(state.tr.delete(1, 5))
  assert.equal(activeTrigger(state), null)
  assert.equal(seen.at(-1), null)
})

test('two triggers are the same menu only when everything about them matches', () => {
  const a: ActiveTrigger = { name: 'page', from: 1, to: 5, query: 'ab' }
  assert.equal(sameTrigger(a, { ...a }), true)
  assert.equal(sameTrigger(a, { ...a, query: 'abc' }), false)
  assert.equal(sameTrigger(a, { ...a, from: 2 }), false)
  assert.equal(sameTrigger(a, { ...a, name: 'mention' }), false)
  assert.equal(sameTrigger(null, null), true)
  assert.equal(sameTrigger(a, null), false)
})

test('the selected entry wraps at both ends of the list', () => {
  assert.equal(moveSelection(0, 1, 3), 1)
  assert.equal(moveSelection(2, 1, 3), 0, 'past the end comes back to the start')
  assert.equal(moveSelection(0, -1, 3), 2, 'and before the start goes to the end')
  assert.equal(moveSelection(0, 1, 0), 0, 'an empty list has nowhere to move')
})
