// Two small attribute-only extensions shared by every block node the editor
// implements. They exist because Tiptap's official nodes don't declare
// these attributes themselves; adding them here (instead of forking each
// node) keeps every node's own extension identical to its upstream MIT
// source and puts the one place that must match packages/docs-schema's
// `blockId`/`textBlock` attribute sets in a single, obviously-correct spot.
//
// docsSchemaTest.ts checks these attribute names and defaults against
// packages/docs-schema/schema.json directly, so a change to either drifts
// loudly instead of silently.
import { Extension } from '@tiptap/core'
import type { Transaction } from '@tiptap/pm/state'
import { Plugin, PluginKey } from '@tiptap/pm/state'

const blockIdPluginKey = new PluginKey('yuhengBlockId')
const BLOCK_ID_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'

/**
 * A fresh id matching docs-schema's blockId format (`^[A-Za-z0-9_-]{6,40}$`).
 * Not cryptographically random -- these are anchor/backlink identifiers
 * scoped to one document, not security tokens -- so `Math.random` is fine.
 */
export function newBlockId(length = 12): string {
  let out = ''
  for (let i = 0; i < length; i++) {
    out += BLOCK_ID_ALPHABET.charAt(Math.floor(Math.random() * BLOCK_ID_ALPHABET.length))
  }
  return out
}

/** Node type names that carry docs-schema's `blockId` attribute set. */
export const BLOCK_ID_TYPES = [
  'paragraph', 'heading', 'blockquote', 'horizontalRule',
  'bulletList', 'orderedList', 'taskList', 'codeBlock', 'table', 'details',
  'image', 'attachment',
  'callout', 'columns', 'pageBreak', 'toc', 'mathBlock', 'mermaid',
] as const

/** Node type names that carry docs-schema's `textBlock` attribute set. */
export const TEXT_BLOCK_TYPES = ['paragraph', 'heading'] as const

/**
 * Adds a nullable `id` attribute to every block-ish node and assigns a
 * random one (matching the schema's blockId format) the first time a node
 * without one is seen. Two collaborators typing at the same time only ever
 * assign ids to nodes *they* just created, so this never races: there is no
 * shared counter, just "this node has no id yet, give it one."
 */
export const BlockId = Extension.create({
  name: 'yuhengBlockId',

  addGlobalAttributes() {
    return [{
      types: [...BLOCK_ID_TYPES],
      attributes: {
        id: {
          default: null,
          parseHTML: (element) => element.getAttribute('data-block-id'),
          renderHTML: (attributes) => (attributes.id ? { 'data-block-id': attributes.id } : {}),
        },
      },
    }]
  },

  addProseMirrorPlugins() {
    const types = new Set<string>(BLOCK_ID_TYPES)
    return [new Plugin({
      key: blockIdPluginKey,
      // Runs after every transaction, including remote ones applied by the
      // Yjs binding, so an id assigned locally never collides with one a
      // peer assigned to a different node: each side only ever touches
      // nodes it can see have no id, and ids are independent per node.
      appendTransaction: (transactions, _oldState, newState) => {
        if (!transactions.some((tr) => tr.docChanged)) return null
        let tr: Transaction | null = null
        newState.doc.descendants((node, pos) => {
          if (types.has(node.type.name) && !node.attrs.id) {
            tr = (tr ?? newState.tr).setNodeMarkup(pos, undefined, { ...node.attrs, id: newBlockId() })
          }
          return true
        })
        return tr
      },
    })]
  },
})

/**
 * Adds `textAlign` and `indent` to paragraph and heading. No toolbar exposes
 * them yet (that lands with T2.4's floating toolbar); declaring the
 * attributes now keeps documents that already carry them — imported via
 * Markdown, or written by a future work package — loading correctly.
 */
export const TextBlockAttrs = Extension.create({
  name: 'yuhengTextBlockAttrs',

  addGlobalAttributes() {
    return [{
      types: [...TEXT_BLOCK_TYPES],
      attributes: {
        textAlign: {
          default: 'left',
          parseHTML: (element) => element.style.textAlign || 'left',
          renderHTML: (attributes) => (
            attributes.textAlign && attributes.textAlign !== 'left'
              ? { style: `text-align: ${attributes.textAlign}` }
              : {}
          ),
        },
        indent: {
          default: 0,
          parseHTML: (element) => Number(element.getAttribute('data-indent')) || 0,
          renderHTML: (attributes) => (attributes.indent ? { 'data-indent': String(attributes.indent) } : {}),
        },
      },
    }]
  },
})
