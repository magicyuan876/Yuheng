// Derives the "on this page" outline straight from the live document's
// heading nodes, rather than from a dedicated `toc` node (that node exists
// in packages/docs-schema for a *rendered, static* table of contents a
// reader inserts into the page -- T2.1's job; this is the always-current
// sidebar outline every page gets for free once it has headings at all).
import type { Node as PMNode } from '@tiptap/pm/model'

export interface TocEntry {
  /** The heading's block id; empty when one has not been assigned yet
   * (BlockId assigns one on the next transaction, so this is transient). */
  id: string
  level: number
  text: string
  /** Position of the heading node in the document, for scrolling to it. */
  pos: number
}

export function extractHeadings(doc: PMNode): TocEntry[] {
  const out: TocEntry[] = []
  doc.descendants((node, pos) => {
    if (node.type.name === 'heading') {
      out.push({
        id: typeof node.attrs.id === 'string' ? node.attrs.id : '',
        level: typeof node.attrs.level === 'number' ? node.attrs.level : 1,
        text: node.textContent,
        pos,
      })
    }
    return true
  })
  return out
}
