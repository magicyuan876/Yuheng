// The paste path.
//
// `paste.ts` holds the decisions — which schemes a link may keep, what the
// sanitiser allows, how a parsed slice is re-checked — with no reference to
// the DOM, so all of it is under test. This file is the wiring: it hands
// those decisions to the two hooks ProseMirror already offers, in the order
// the markup passes through them.
//
//   clipboardTextParser  plain text that looks like Markdown → rendered first
//   transformPastedHTML  the text, before anything parses it → DOMPurify
//   transformPasted      the parsed slice, before it is inserted → sanitizeSlice
//
// The Markdown step only ever sees text: when the clipboard carries HTML,
// that is what ProseMirror parses and this never runs. Whatever it renders
// goes through the same two steps below as any other pasted markup, so there
// is one sanitising path rather than two.
//
// Using the hooks rather than handlePaste matters: they apply to every route
// by which content arrives, including a drop and the editor's own internal
// paste commands, and they leave handlePaste free for the upload handler,
// which needs to claim a pasted *file* before any of this runs.
import { DOMParser as PMDOMParser, type ResolvedPos, type Schema, Slice } from '@tiptap/pm/model'
import DOMPurify from 'dompurify'

import { looksLikeMarkdown, markdownToHTML } from './markdownPaste'
import { PASTE_PURIFY_CONFIG, sanitizeSlice } from './paste'

/** A view, narrowed to the one thing the slice pass needs from it. */
interface SchemaHolder {
  state: { schema: Schema }
}

/**
 * The paste hooks, ready to spread into the editor's editorProps.
 *
 * Kept as a plain function rather than a composable: it holds no state and
 * owns nothing that needs tearing down.
 */
export function pasteEditorProps(): Record<string, unknown> {
  const purify = (html: string): string => DOMPurify.sanitize(html, {
    ...PASTE_PURIFY_CONFIG,
    ALLOWED_TAGS: [...PASTE_PURIFY_CONFIG.ALLOWED_TAGS],
    ALLOWED_ATTR: [...PASTE_PURIFY_CONFIG.ALLOWED_ATTR],
    FORBID_TAGS: [...PASTE_PURIFY_CONFIG.FORBID_TAGS],
    FORBID_ATTR: [...PASTE_PURIFY_CONFIG.FORBID_ATTR],
  }) as string

  return {
    /**
     * Plain text that reads as Markdown is rendered before it is parsed.
     *
     * `plain` is true when somebody asked for a literal paste (Ctrl+Shift+V),
     * and that request is honoured: the point of the shortcut is to get the
     * characters and nothing else.
     */
    clipboardTextParser: (
      text: string,
      $context: ResolvedPos,
      plain: boolean,
      view: SchemaHolder,
    ): Slice | undefined => {
      if (plain || !looksLikeMarkdown(text)) return undefined
      // Not inside a code block, where the characters are the content.
      for (let depth = $context.depth; depth > 0; depth--) {
        if ($context.node(depth).type.spec.code) return undefined
      }

      const dom = new window.DOMParser()
        .parseFromString(purify(markdownToHTML(text)), 'text/html')
      return PMDOMParser.fromSchema(view.state.schema).parseSlice(dom.body, {
        preserveWhitespace: false,
      })
    },

    transformPastedHTML: purify,

    transformPasted: (slice: Slice, view: SchemaHolder): Slice =>
      sanitizeSlice(slice, view.state.schema),
  }
}
