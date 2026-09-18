// The structural nodes packages/docs-schema declares and no stock Tiptap
// extension provides: callouts, multi-column rows, status chips, page breaks
// and the table of contents marker.
//
// As with mediaNodes.ts there is no Vue here, so the schema these declare can
// be loaded and compared against packages/docs-schema in a plain Node test.
// The node views arrive separately, through officialExtensions' `views`.
import { mergeAttributes, Node } from '@tiptap/core'

import { CALLOUT_KINDS, MAX_COLUMNS, MIN_COLUMNS, STATUS_COLORS } from './figures'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsStructure: {
      /** Wraps the selection in a callout of the given kind. */
      setCallout: (kind?: string) => ReturnType
      /** Unwraps a callout back into plain blocks. */
      unsetCallout: () => ReturnType
      /** Inserts a row of `count` empty columns. */
      insertColumns: (count?: number) => ReturnType
      /** Inserts a status chip with the given text and colour. */
      insertStatus: (text?: string, color?: string) => ReturnType
      /** Inserts a page break. */
      insertPageBreak: () => ReturnType
      /** Inserts the table-of-contents marker. */
      insertTableOfContents: () => ReturnType
    }
  }
}

/** Keeps an attribute inside a closed vocabulary when it is read from HTML. */
function enumAttr(name: string, values: readonly string[], fallback: string) {
  return {
    default: fallback,
    parseHTML: (el: HTMLElement) => {
      const raw = el.getAttribute(`data-${name}`) ?? ''
      return values.includes(raw) ? raw : fallback
    },
    renderHTML: (attrs: Record<string, unknown>) => ({ [`data-${name}`]: String(attrs[name] ?? fallback) }),
  }
}

/** An aside with a kind and an icon, holding ordinary blocks. */
export const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      kind: enumAttr('kind', CALLOUT_KINDS, 'info'),
      icon: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute('data-icon'),
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.icon ? { 'data-icon': String(attrs.icon) } : {},
      },
    }
  },

  parseHTML() {
    return [{ tag: 'aside[data-kind]' }, { tag: 'div.callout' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['aside', mergeAttributes(HTMLAttributes, { class: 'callout' }), 0]
  },

  addCommands() {
    return {
      setCallout: (kind = 'info') => ({ commands }) =>
        commands.wrapIn(this.name, { kind }),
      unsetCallout: () => ({ commands }) => commands.lift(this.name),
    }
  },
})

/**
 * A row of columns.
 *
 * The content expression is what keeps the structure sound: between two and
 * five columns, and nothing else. It is also the answer to two people editing
 * one row at the same time — ProseMirror will not let either of them produce a
 * row outside those bounds, whichever order the changes merge in, so no
 * sequence of concurrent edits can leave a row the schema would reject.
 */
export const Columns = Node.create({
  name: 'columns',
  group: 'block',
  content: `column{${MIN_COLUMNS},${MAX_COLUMNS}}`,
  isolating: true,

  addAttributes() {
    return { mode: enumAttr('mode', ['normal', 'wide'], 'normal') }
  },

  parseHTML() {
    return [{ tag: 'div[data-columns]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-columns': '', class: 'columns' }), 0]
  },

  addCommands() {
    return {
      insertColumns: (count = MIN_COLUMNS) => ({ state, commands }) => {
        const wanted = Math.min(MAX_COLUMNS, Math.max(MIN_COLUMNS, Math.trunc(count)))
        const column = state.schema.nodes.column
        const paragraph = state.schema.nodes.paragraph
        if (!column || !paragraph) return false
        return commands.insertContent({
          type: this.name,
          content: Array.from({ length: wanted }, () => ({
            type: 'column',
            content: [{ type: 'paragraph' }],
          })),
        })
      },
    }
  },
})

/** One column of a row. It has no group: it may only exist inside `columns`. */
export const Column = Node.create({
  name: 'column',
  content: 'block+',
  isolating: true,

  addAttributes() {
    return {
      width: {
        default: null,
        parseHTML: (el: HTMLElement) => {
          const raw = el.getAttribute('data-width')
          const n = raw ? Number.parseFloat(raw) : Number.NaN
          return Number.isFinite(n) ? n : null
        },
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.width == null ? {} : { 'data-width': String(attrs.width) },
      },
    }
  },

  parseHTML() {
    return [{ tag: 'div.column' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { class: 'column' }), 0]
  },
})

/** A small coloured chip used inline, like "In review". */
export const Status = Node.create({
  name: 'status',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      text: {
        default: '',
        parseHTML: (el: HTMLElement) => el.getAttribute('data-text') ?? el.textContent?.trim() ?? '',
        renderHTML: (attrs: Record<string, unknown>) => ({ 'data-text': String(attrs.text ?? '') }),
      },
      color: enumAttr('color', STATUS_COLORS, 'gray'),
    }
  },

  parseHTML() {
    return [{ tag: 'span[data-text][data-color]' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    return ['span', mergeAttributes(HTMLAttributes, { class: 'status' }), String(node.attrs.text ?? '')]
  },

  addCommands() {
    return {
      insertStatus: (text = '', color = 'gray') => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { text, color } }),
    }
  },
})

/** A break that only means anything when the page is exported to PDF. */
export const PageBreak = Node.create({
  name: 'pageBreak',
  group: 'block',
  atom: true,
  selectable: true,

  parseHTML() {
    return [{ tag: 'div[data-page-break]' }, { tag: 'hr.page-break' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-page-break': '', class: 'page-break' })]
  },

  addCommands() {
    return {
      insertPageBreak: () => ({ commands }) => commands.insertContent({ type: this.name }),
    }
  },
})

/**
 * A marker saying "put the page's contents here". It carries nothing: the list
 * is derived from the page's headings whenever it is rendered, so it can never
 * fall out of step with them.
 */
export const TableOfContents = Node.create({
  name: 'toc',
  group: 'block',
  atom: true,
  selectable: true,

  parseHTML() {
    return [{ tag: 'div[data-toc]' }, { tag: 'nav.toc' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-toc': '', class: 'toc' })]
  },

  addCommands() {
    return {
      insertTableOfContents: () => ({ commands }) => commands.insertContent({ type: this.name }),
    }
  },
})
