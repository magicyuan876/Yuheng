import assert from 'node:assert/strict'
import { test } from 'node:test'

import { Extension, getSchema } from '@tiptap/core'

// Imported straight from its TypeScript source (not published to npm, and
// this frontend is not an npm workspace of it) -- this is the one file in
// the repository allowed to reach across that boundary, precisely so this
// test can compare against the single source of truth instead of a copy of
// it. See packages/docs-schema/src/index.ts for what each export does.
import { diffEditorSchema, markNames, nodeNames } from '../../../../../packages/docs-schema/src/index.ts'

import { officialExtensions } from './extensions'

function liveSchema() {
  return getSchema(officialExtensions())
}

// Two accepted divergences, each a deliberate trade-off rather than an
// oversight:
//
//  - bulletList/orderedList/taskList report an extra "list" group. Tiptap's
//    stock list extensions add it so shared list commands (indent, keymap)
//    can recognise "any kind of list" generically. `group` is a
//    ProseMirror-internal content-matching concept and is never part of a
//    persisted node's JSON (only `type`/`attrs`/`content`/`marks`/`text`
//    are), so it cannot cause a document the editor produces to fail the
//    server's validator.
//  - tableRow accepts zero cells (`*`) where the schema requires at least
//    one (`+`), because Tiptap's table-editing commands are built against
//    the looser shape and reworking them to guarantee `+` is out of
//    proportion for this work package. The server's validator is the
//    backstop: a row that somehow ends up empty is rejected on persist
//    (400) rather than silently accepted, which is an acceptable failure
//    mode for an edge case normal table editing cannot reach.
const ACCEPTED_DIVERGENCES = new Set([
  'node "bulletList" group: editor [block,list], schema [block]',
  'node "orderedList" group: editor [block,list], schema [block]',
  'node "taskList" group: editor [block,list], schema [block]',
  'node "tableRow" content: editor "(tableCell | tableHeader)*", schema "(tableCell | tableHeader)+"',
])

test('the editor schema matches packages/docs-schema for every node and mark it implements', () => {
  const schema = liveSchema()
  const implementedNodes = new Set(Object.keys(schema.nodes))
  const implementedMarks = new Set(Object.keys(schema.marks))

  const problems = diffEditorSchema(schema).filter((p) => {
    if (ACCEPTED_DIVERGENCES.has(p)) return false
    // Expected: packages/docs-schema declares ~23 nodes (images, callouts,
    // mentions, embeds, ...) their own later work packages (T1.6, T2.x,
    // T3.2) have not implemented yet. Anything else -- a real divergence on
    // a node/mark this file DOES implement, or a node/mark this file
    // declares that the schema does not know about at all -- must fail.
    const missingNode = /^node "([^"]+)" is missing from the editor$/.exec(p)
    if (missingNode) return implementedNodes.has(missingNode[1]!)
    const missingMark = /^mark "([^"]+)" is missing from the editor$/.exec(p)
    if (missingMark) return implementedMarks.has(missingMark[1]!)
    return true
  })

  assert.deepEqual(problems, [], ['unexpected schema divergence:', ...problems.map((p) => `  - ${p}`)].join('\n'))
})

test('every node and mark the editor declares belongs to packages/docs-schema', () => {
  const schema = liveSchema()
  for (const name of Object.keys(schema.nodes)) {
    assert.ok(nodeNames.includes(name), `node "${name}" is not declared in schema.json`)
  }
  for (const name of Object.keys(schema.marks)) {
    assert.ok(markNames.includes(name), `mark "${name}" is not declared in schema.json`)
  }
})

test('extending officialExtensions with an unrelated extra extension does not disturb the covered set', () => {
  const withExtra = getSchema(officialExtensions([Extension.create({ name: 'probe' })]))
  const base = liveSchema()
  assert.deepEqual(Object.keys(withExtra.nodes).sort(), Object.keys(base.nodes).sort())
  assert.deepEqual(Object.keys(withExtra.marks).sort(), Object.keys(base.marks).sort())
})

test('the media nodes point at an attachment id and never at a stored address', () => {
  const schema = liveSchema()
  for (const name of ['image', 'attachment']) {
    const node = schema.nodes[name]
    assert.ok(node, `${name} must be part of the editor schema`)
    assert.ok('attachmentId' in node.spec.attrs!, `${name} must carry an attachment id`)
    assert.equal(node.spec.atom, true, `${name} has no editable content of its own`)
  }
  // An image may also point outside the workspace, which is the one case a
  // literal address is kept; a file card never has one.
  assert.ok('src' in schema.nodes.image!.spec.attrs!)
  assert.ok(!('src' in schema.nodes.attachment!.spec.attrs!))
})

test('the covered count is a deliberate, documented number', () => {
  // Bumping this alongside a real change is the point: it forces whoever
  // adds a node in a later work package to notice this file and update the
  // decision note in the module comment at the top of extensions.ts.
  const schema = liveSchema()
  assert.equal(Object.keys(schema.nodes).length, 22, 'node count changed -- update this test and the header comment')
  assert.equal(Object.keys(schema.marks).length, 10, 'mark count changed -- update this test and the header comment')
})
