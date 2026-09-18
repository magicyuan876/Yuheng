import assert from 'node:assert/strict'
import { test } from 'node:test'

import { emptyScene, parseScene, sanitiseAppState, serialiseScene } from './excalidraw'

test('a new drawing starts from a scene the editor can open', () => {
  const scene = emptyScene()
  assert.equal(scene.type, 'excalidraw')
  assert.deepEqual(scene.elements, [])
  assert.deepEqual(scene.files, {})
})

test('a stored scene round-trips through save and reopen', () => {
  const elements = [{ id: 'a', type: 'rectangle' }, { id: 'b', type: 'arrow' }]
  const raw = serialiseScene(elements, { gridSize: 20 }, { img1: { id: 'img1' } })

  const reopened = parseScene(raw)
  assert.deepEqual(reopened.elements, elements)
  assert.deepEqual(reopened.appState, { gridSize: 20 })
  assert.deepEqual(reopened.files, { img1: { id: 'img1' } })
})

// A drawing whose file will not parse must still be openable, or nobody can
// ever repair it.
test('an unreadable scene opens as a blank canvas rather than failing', () => {
  for (const raw of ['', 'not json', 'null', '42', '"a string"', '[]']) {
    const scene = parseScene(raw)
    assert.deepEqual(scene.elements, [], raw)
    assert.equal(scene.type, 'excalidraw', raw)
  }
})

test('a scene missing the parts it should have gets sensible ones', () => {
  const scene = parseScene(JSON.stringify({ type: 'excalidraw' }))
  assert.deepEqual(scene.elements, [])
  assert.deepEqual(scene.appState, {})
  assert.deepEqual(scene.files, {})

  // A file claiming elements that are not a list is not trusted into the
  // editor, where it would throw on the first render.
  const wrong = parseScene(JSON.stringify({ elements: 'lots', appState: 7, files: [] }))
  assert.deepEqual(wrong.elements, [])
  assert.deepEqual(wrong.appState, {})
  assert.deepEqual(wrong.files, {})
})

// The editor's state holds this session's view as well as the drawing.
// Storing the view would make two people saving an unchanged drawing produce
// different files, and would reopen somebody else's scroll position as if it
// were part of the diagram.
test('only the parts of the editor state that describe the drawing are kept', () => {
  const kept = sanitiseAppState({
    gridSize: 20,
    viewBackgroundColor: '#fff',
    exportBackground: true,
    // This session's view, not the drawing.
    scrollX: 480,
    scrollY: -120,
    zoom: { value: 1.5 },
    selectedElementIds: { a: true },
    cursorButton: 'up',
    openDialog: 'help',
    collaborators: [{ id: 'someone' }],
  })

  assert.deepEqual(kept, { gridSize: 20, viewBackgroundColor: '#fff', exportBackground: true })
  for (const gone of ['scrollX', 'scrollY', 'zoom', 'selectedElementIds', 'cursorButton', 'openDialog']) {
    assert.ok(!(gone in kept), `${gone} belongs to the session, not the drawing`)
  }
  assert.ok(!('collaborators' in kept), 'and nobody else’s presence is stored either')
})

test('saving the same drawing twice produces the same file', () => {
  const elements = [{ id: 'a', type: 'rectangle' }]
  const first = serialiseScene(elements, { gridSize: 20, scrollX: 10 }, {})
  const second = serialiseScene(elements, { gridSize: 20, scrollX: 999 }, {})
  assert.equal(first, second, 'a different scroll position is not a different diagram')
})

test('an empty drawing serialises to something that parses back', () => {
  const raw = serialiseScene([], {}, {})
  const scene = parseScene(raw)
  assert.deepEqual(scene.elements, [])
  assert.equal(scene.source, 'yuheng')
})
