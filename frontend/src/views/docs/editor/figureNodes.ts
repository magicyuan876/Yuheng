// The three nodes whose content is source text rendered into something else:
// inline and block formulas, and Mermaid diagrams.
//
// Each stores only its source. The rendering is done by the node view and is
// never persisted, so a document is exactly what the author wrote and can be
// re-rendered by a newer KaTeX or Mermaid without being rewritten.
//
// No Vue here, for the same reason as the other node files: the schema must be
// loadable in a plain Node test.
import { mergeAttributes, Node, nodeInputRule } from '@tiptap/core'

import { MATH_BLOCK_INPUT, MATH_INLINE_INPUT, MERMAID_INPUT } from './inputRules'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsFigures: {
      /** Inserts an inline formula. */
      insertMathInline: (latex?: string) => ReturnType
      /** Inserts a display formula on its own line. */
      insertMathBlock: (latex?: string) => ReturnType
      /** Inserts a diagram. */
      insertMermaid: (source?: string) => ReturnType
    }
  }
}

/** Reads a node's source out of HTML, preferring the attribute over the text. */
function sourceAttr(name: string, attribute: string) {
  return {
    default: '',
    parseHTML: (el: HTMLElement) => el.getAttribute(attribute) ?? el.textContent ?? '',
    renderHTML: (attrs: Record<string, unknown>) => ({ [attribute]: String(attrs[name] ?? '') }),
  }
}

/** A formula set in running text. */
export const MathInline = Node.create({
  name: 'mathInline',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return { latex: sourceAttr('latex', 'data-latex') }
  },

  parseHTML() {
    return [{ tag: 'span[data-latex]' }]
  },

  // The source is written out as the element's text as well as its attribute,
  // so a document copied into something that knows nothing about this editor
  // still carries the formula rather than an empty span.
  renderHTML({ HTMLAttributes, node }) {
    return ['span', mergeAttributes(HTMLAttributes, { class: 'math-inline' }), String(node.attrs.latex ?? '')]
  },

  addCommands() {
    return {
      insertMathInline: (latex = '') => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { latex } }),
    }
  },

  addInputRules() {
    // "$x^2$" becomes the formula, with what was between the delimiters as
    // its source. The pattern is what keeps a sentence about prices out of
    // this; see inputRules.ts.
    return [nodeInputRule({
      find: MATH_INLINE_INPUT,
      type: this.type,
      getAttributes: (match) => ({ latex: match[1] ?? '' }),
    })]
  },
})

/** A formula on a line of its own. */
export const MathBlock = Node.create({
  name: 'mathBlock',
  group: 'block',
  atom: true,
  selectable: true,

  addAttributes() {
    return { latex: sourceAttr('latex', 'data-latex') }
  },

  parseHTML() {
    return [{ tag: 'div[data-latex]' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    return ['div', mergeAttributes(HTMLAttributes, { class: 'math-block' }), String(node.attrs.latex ?? '')]
  },

  addCommands() {
    return {
      insertMathBlock: (latex = '') => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { latex } }),
    }
  },

  addInputRules() {
    return [nodeInputRule({ find: MATH_BLOCK_INPUT, type: this.type })]
  },
})

/** A diagram written in Mermaid's own language. */
export const Mermaid = Node.create({
  name: 'mermaid',
  group: 'block',
  atom: true,
  selectable: true,

  addAttributes() {
    return { source: sourceAttr('source', 'data-source') }
  },

  parseHTML() {
    return [{ tag: 'div[data-source].mermaid' }, { tag: 'pre.mermaid' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    return ['div', mergeAttributes(HTMLAttributes, { class: 'mermaid' }), String(node.attrs.source ?? '')]
  },

  addCommands() {
    return {
      insertMermaid: (source = 'graph TD;\n  A --> B;') => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { source } }),
    }
  },

  addInputRules() {
    // A fence naming mermaid, which is how such a diagram is written in a
    // Markdown file; the code block extension takes every other fence.
    return [nodeInputRule({ find: MERMAID_INPUT, type: this.type })]
  },
})
