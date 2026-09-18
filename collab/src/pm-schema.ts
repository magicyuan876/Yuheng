// The ProseMirror schema the service converts with. It is built from the
// single document definition (packages/docs-schema/schema.json) so that what
// this service serialises is exactly what the Go validator accepts.

import { Node, Schema } from 'prosemirror-model'
import type { Doc as YDoc } from 'yjs'
import { prosemirrorJSONToYDoc, prosemirrorToYXmlFragment, yDocToProsemirrorJSON } from 'y-prosemirror'
import type { XmlFragment } from 'yjs'

import { toProseMirrorSpec } from '../../packages/docs-schema/src/index.ts'

/** Name of the Yjs XmlFragment that holds the document body. */
export const FRAGMENT = 'default'

let cached: Schema | null = null

/** The compiled schema (built once). */
export function pmSchema(): Schema {
  if (!cached) {
    const spec = toProseMirrorSpec()
    cached = new Schema({ topNode: spec.topNode, nodes: spec.nodes, marks: spec.marks })
  }
  return cached
}

/** The body of a page that has no content yet. */
export const EMPTY_DOCUMENT = { type: 'doc', content: [{ type: 'paragraph' }] }

/** Parses ProseMirror JSON strictly; throws on anything the schema rejects. */
export function parseDocument(json: unknown): Node {
  if (!json || typeof json !== 'object') throw new Error('content must be a JSON object')
  const node = Node.fromJSON(pmSchema(), json)
  node.check()
  return node
}

/** JSON → fresh Y.Doc. */
export function jsonToYDoc(json: unknown): YDoc {
  parseDocument(json) // fail early with a clear error
  return prosemirrorJSONToYDoc(pmSchema(), json, FRAGMENT)
}

/** Y.Doc → JSON projection. */
export function yDocToJSON(doc: YDoc): unknown {
  return yDocToProsemirrorJSON(doc, FRAGMENT)
}

/**
 * Replaces the content of a live fragment with the given JSON as one Yjs
 * transaction (the caller wraps this in `doc.transact`). y-prosemirror diffs
 * the trees so unchanged blocks keep their identity.
 */
export function replaceFragment(fragment: XmlFragment, json: unknown): void {
  const node = parseDocument(json)
  prosemirrorToYXmlFragment(node, fragment)
}
