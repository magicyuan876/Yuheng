// The two nodes that point at something else in the workspace: a link to
// another page, and a mention of a person.
//
// Both store an id and nothing else that matters. A page link carries no
// title, which is what makes renaming a page update every link to it without
// rewriting a single document, and what stops a link to a page the reader
// cannot open from leaking its name. A mention keeps a label only as a
// fallback for somewhere the directory cannot be consulted, such as an
// exported file.
//
// No Vue here, so the schema stays loadable in a plain Node test.
import { mergeAttributes, Node } from '@tiptap/core'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsLinks: {
      /** Replaces the range with a link to a page. */
      insertPageLink: (pageId: string, range?: { from: number; to: number }) => ReturnType
      /** Replaces the range with a mention of a person. */
      insertMention: (userId: string, label?: string, range?: { from: number; to: number }) => ReturnType
    }
  }
}

/** A link to another page of the workspace. */
export const PageLink = Node.create({
  name: 'pageLink',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      pageId: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute('data-page-id'),
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.pageId ? { 'data-page-id': String(attrs.pageId) } : {},
      },
    }
  },

  parseHTML() {
    return [{ tag: 'a[data-page-id]' }, { tag: 'span[data-page-id]' }]
  },

  // The exported form carries the id and no title, for the same reason the
  // stored form does. Whoever renders it resolves the title then.
  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes, { class: 'page-link' })]
  },

  addCommands() {
    return {
      insertPageLink: (pageId, range) => ({ commands }) => {
        const content = { type: this.name, attrs: { pageId } }
        return range
          ? commands.insertContentAt(range, content)
          : commands.insertContent(content)
      },
    }
  },
})

/** A mention of a person who can read this page. */
export const Mention = Node.create({
  name: 'mention',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      userId: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute('data-user-id'),
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.userId ? { 'data-user-id': String(attrs.userId) } : {},
      },
      label: {
        // Kept so an export or a copy into another application still reads as
        // a name. Inside the editor the directory is consulted instead, so a
        // renamed person is not frozen into old documents.
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute('data-label') ?? el.textContent?.replace(/^@/, '') ?? null,
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.label ? { 'data-label': String(attrs.label) } : {},
      },
    }
  },

  parseHTML() {
    return [{ tag: 'span[data-user-id]' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    const label = node.attrs.label ? `@${node.attrs.label}` : '@'
    return ['span', mergeAttributes(HTMLAttributes, { class: 'mention' }), label]
  },

  addCommands() {
    return {
      insertMention: (userId, label, range) => ({ commands }) => {
        // A trailing space is part of the insertion: a mention is almost
        // always followed by more of the sentence, and an atom node with the
        // caret welded to its right edge is awkward to type past.
        const content = [
          { type: this.name, attrs: { userId, label: label ?? null } },
          { type: 'text', text: ' ' },
        ]
        return range
          ? commands.insertContentAt(range, content)
          : commands.insertContent(content)
      },
    }
  },
})
