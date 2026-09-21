import assert from 'node:assert/strict'
import { test } from 'vitest'

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

// Acceptance (T2.1): two people editing one column row at the same time must
// not be able to leave it malformed. The guarantee is structural rather than
// procedural — the content expression is what ProseMirror enforces on every
// transaction, whichever order concurrent changes merge in, so no sequence of
// edits can produce a row the server would then reject.
test('a column row is bounded by its content expression, not by the editor’s care', () => {
  const schema = liveSchema()
  const columns = schema.nodes.columns
  assert.ok(columns)
  assert.equal(columns.spec.content, 'column{2,5}')
  assert.equal(columns.spec.isolating, true, 'edits cannot cross the row boundary')

  // A column exists only inside a row: it belongs to no group, so no content
  // expression anywhere else in the schema will accept one.
  const column = schema.nodes.column
  assert.ok(column)
  assert.equal(column.spec.group, undefined)
  assert.equal(column.spec.isolating, true)

  // And the schema refuses to build a row outside the bounds at all.
  const paragraph = schema.node('paragraph')
  const one = schema.node('column', null, [paragraph])
  assert.throws(() => schema.node('columns', null, [one]), /Invalid content/i)
  assert.throws(
    () => schema.node('columns', null, Array.from({ length: 6 }, () => schema.node('column', null, [paragraph]))),
    /Invalid content/i,
  )
  assert.doesNotThrow(() => schema.node('columns', null, [
    schema.node('column', null, [paragraph]),
    schema.node('column', null, [paragraph]),
  ]))
})

test('the figures store their source and never their rendering', () => {
  const schema = liveSchema()
  for (const name of ['mathInline', 'mathBlock']) {
    assert.ok('latex' in schema.nodes[name]!.spec.attrs!, `${name} keeps the formula as written`)
    assert.equal(schema.nodes[name]!.spec.atom, true)
  }
  assert.ok('source' in schema.nodes.mermaid!.spec.attrs!)
  // Nothing carries rendered output, so a newer KaTeX or Mermaid re-renders
  // an existing document rather than requiring it to be rewritten.
  for (const name of ['mathInline', 'mathBlock', 'mermaid']) {
    const attrs = Object.keys(schema.nodes[name]!.spec.attrs ?? {})
    assert.ok(!attrs.includes('html'), `${name} must not store HTML`)
    assert.ok(!attrs.includes('svg'), `${name} must not store an SVG`)
  }
})

// Acceptance (T2.2): renaming a page updates every link to it. That works
// because no document ever stored the title — only the id.
test('a page link stores an id and nothing that could go stale', () => {
  const schema = liveSchema()
  const attrs = Object.keys(schema.nodes.pageLink!.spec.attrs ?? {})
  assert.deepEqual(attrs, ['pageId'])
  assert.equal(schema.nodes.pageLink!.spec.atom, true)
  assert.equal(schema.nodes.pageLink!.spec.inline, true)

  // A mention keeps a label as a fallback for an export, but the id is what
  // identifies the person.
  const mention = Object.keys(schema.nodes.mention!.spec.attrs ?? {}).sort()
  assert.deepEqual(mention, ['label', 'userId'])
})

// Acceptance (T2.3): a saved diagram shows its rendered preview to a reader
// and in an export, with no editor involved. That works because the node
// stores both the editable source and a rendered copy, and the reading path
// only ever needs the second.
test('a diagram stores an editable source and a rendered preview', () => {
  const schema = liveSchema()
  for (const name of ['drawio', 'excalidraw']) {
    const attrs = Object.keys(schema.nodes[name]!.spec.attrs ?? {})
    assert.ok(attrs.includes('attachmentId'), `${name} keeps its source`)
    assert.ok(attrs.includes('previewAttachmentId'), `${name} keeps a rendering to show`)
    assert.equal(schema.nodes[name]!.spec.atom, true)
  }
})

// Acceptance (T2.3): an embed stores what its author pasted, so the server's
// allow-list decides what is framed on every read rather than at the last save.
test('an embed stores the pasted address and never the frame address', () => {
  const schema = liveSchema()
  const attrs = Object.keys(schema.nodes.embed!.spec.attrs ?? {}).sort()
  assert.deepEqual(attrs, ['align', 'height', 'id', 'provider', 'url', 'width'])
  assert.ok(!attrs.includes('embedUrl'), 'the frame address is derived, never stored')
})

test('media nodes point at an attachment and nothing else', () => {
  const schema = liveSchema()
  for (const name of ['video', 'audio', 'pdfEmbed']) {
    assert.ok('attachmentId' in schema.nodes[name]!.spec.attrs!, name)
    assert.equal(schema.nodes[name]!.spec.atom, true, name)
  }
})

test('the covered count is a deliberate, documented number', () => {
  // Bumping this alongside a real change is the point: it forces whoever
  // adds a node in a later work package to notice this file and update the
  // decision note in the module comment at the top of extensions.ts.
  const schema = liveSchema()
  assert.equal(Object.keys(schema.nodes).length, 40, 'node count changed -- update this test and the header comment')
  assert.equal(Object.keys(schema.marks).length, 10, 'mark count changed -- update this test and the header comment')
})
