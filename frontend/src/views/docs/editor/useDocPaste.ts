// The paste path.
//
// `paste.ts` holds the decisions — which schemes a link may keep, what the
// sanitiser allows, how a parsed slice is re-checked — with no reference to
// the DOM, so all of it is under test. This file is the wiring: it hands
// those decisions to the two hooks ProseMirror already offers, in the order
// the markup passes through them.
//
//   transformPastedHTML  the text, before anything parses it → DOMPurify
//   transformPasted      the parsed slice, before it is inserted → sanitizeSlice
//
// Using the hooks rather than handlePaste matters: they apply to every route
// by which content arrives, including a drop and the editor's own internal
// paste commands, and they leave handlePaste free for the upload handler,
// which needs to claim a pasted *file* before any of this runs.
import type { Slice } from '@tiptap/pm/model'
import DOMPurify from 'dompurify'

import { PASTE_PURIFY_CONFIG, sanitizeSlice } from './paste'

/** A view, narrowed to the one thing the slice pass needs from it. */
interface SchemaHolder {
  state: { schema: Parameters<typeof sanitizeSlice>[1] }
}

/**
 * The paste hooks, ready to spread into the editor's editorProps.
 *
 * Kept as a plain function rather than a composable: it holds no state and
 * owns nothing that needs tearing down.
 */
export function pasteEditorProps(): Record<string, unknown> {
  return {
    transformPastedHTML: (html: string): string => DOMPurify.sanitize(html, {
      ...PASTE_PURIFY_CONFIG,
      ALLOWED_TAGS: [...PASTE_PURIFY_CONFIG.ALLOWED_TAGS],
      ALLOWED_ATTR: [...PASTE_PURIFY_CONFIG.ALLOWED_ATTR],
      FORBID_TAGS: [...PASTE_PURIFY_CONFIG.FORBID_TAGS],
      FORBID_ATTR: [...PASTE_PURIFY_CONFIG.FORBID_ATTR],
    }) as string,
    transformPasted: (slice: Slice, view: SchemaHolder): Slice =>
      sanitizeSlice(slice, view.state.schema),
  }
}
