// Pasting HTML from somewhere else.
//
// Three things stand between a copied web page and this document, and it is
// worth being clear about which one actually does the work:
//
//  1. The schema. ProseMirror builds a document by matching the pasted DOM
//     against the parse rules of the node and mark types that exist. There is
//     no script node type, no iframe node type and no style node type, so no
//     amount of crafted markup can produce one — such an element simply has no
//     rule that matches and is discarded with its content. This is the real
//     guarantee, and it holds whether or not anything else runs.
//  2. DOMPurify, run before the parse. It is defence in depth: it removes the
//     markup before ProseMirror ever walks it, so an attribute on an element
//     that *does* have a rule cannot carry something through.
//  3. This file's own pass over the parsed slice, which re-checks the one
//     thing that survives as data rather than as structure: the address in a
//     link. A mark's attribute is copied straight out of the DOM, so it is the
//     one place a scheme like javascript: could otherwise reach the document.
//
// The URL policy and the slice pass are here, free of the DOM, so both are
// exercised by tests; the DOMPurify call itself lives in the composable.
import type { Mark, Node as PMNode, Schema } from '@tiptap/pm/model'
import { Fragment, Slice } from '@tiptap/pm/model'

/** The schemes a pasted link may keep. */
export const SAFE_LINK_PROTOCOLS = ['http:', 'https:', 'mailto:', 'tel:'] as const

/**
 * Whether a pasted address may be kept as a link.
 *
 * Everything that is not plainly one of the allowed schemes is refused,
 * including anything this cannot parse — guessing at a malformed address is
 * how the interesting ones get through. A relative address is allowed: it can
 * only ever point back into this application.
 */
export function isSafeLinkHref(raw: unknown): boolean {
  if (typeof raw !== 'string') return false
  const value = stripIgnorable(raw).trim()
  if (value === '') return false
  // A fragment or a path cannot carry a scheme, so it cannot execute.
  if (value.startsWith('#') || value.startsWith('/') || value.startsWith('?')) return true
  // Protocol-relative is http(s) by definition.
  if (value.startsWith('//')) return true

  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(value)
  if (!scheme) {
    // No scheme at all: a relative address such as "docs/page".
    return !value.includes(':')
  }
  return (SAFE_LINK_PROTOCOLS as readonly string[]).includes(scheme[1]!.toLowerCase() + ':')
}

/**
 * Removes the characters a browser ignores when it resolves a URL.
 *
 * "java\nscript:" and "java&#9;script:" are the same address to a browser and
 * different strings to a naive check, which is why the check is done on the
 * stripped form rather than the literal one.
 */
export function stripIgnorable(value: string): string {
  // Tab, newline, carriage return and the C0 controls are all ignored inside
  // a URL by every browser; so is the zero-width space in practice.
  return value.replace(/[\u0000-\u0020\u200b-\u200d\ufeff]/g, '')
}

/**
 * The DOMPurify configuration for a paste.
 *
 * It is written as an allow-list rather than a block-list for the same reason
 * the schema is: naming what may stay is a decision that can be read, while
 * naming what must go is a promise that the list is complete.
 */
export const PASTE_PURIFY_CONFIG = {
  ALLOWED_TAGS: [
    'p', 'br', 'span', 'div',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'strong', 'b', 'em', 'i', 'u', 's', 'strike', 'del', 'mark', 'sub', 'sup',
    'code', 'pre', 'blockquote', 'hr',
    'ul', 'ol', 'li',
    'table', 'thead', 'tbody', 'tr', 'th', 'td',
    'a', 'img', 'figure', 'figcaption',
    'details', 'summary',
  ],
  ALLOWED_ATTR: [
    'href', 'title', 'alt', 'src', 'srcset', 'width', 'height',
    'colspan', 'rowspan', 'colwidth', 'start', 'type', 'open',
    'data-align', 'data-checked', 'data-type', 'class', 'style',
  ],
  // Data URIs are refused outright: an image pasted from another page is
  // uploaded by the drop handler instead, and no other data: payload has any
  // business in a document.
  ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|tel:|[^a-z]|[a-z+.-]+(?:[^a-z+.:-]|$))/i,
  FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button', 'link', 'meta'],
  FORBID_ATTR: ['onerror', 'onload', 'onclick', 'onmouseover', 'onfocus', 'formaction', 'srcdoc'],
  KEEP_CONTENT: true,
  RETURN_DOM: false,
  RETURN_DOM_FRAGMENT: false,
} as const

/**
 * Re-checks a parsed slice.
 *
 * Structure is already safe by then — the schema decided that. What is left is
 * the data the schema carried through verbatim: a link's address, and an
 * image's address when it points outside this workspace. Both are checked
 * here, and a link whose address is refused loses the link rather than the
 * text, because the words somebody pasted are what they meant to paste.
 */
export function sanitizeSlice(slice: Slice, schema: Schema): Slice {
  const cleaned = sanitizeFragment(slice.content, schema)
  return cleaned === slice.content ? slice : new Slice(cleaned, slice.openStart, slice.openEnd)
}

function sanitizeFragment(fragment: Fragment, schema: Schema): Fragment {
  const out: PMNode[] = []
  let changed = false
  fragment.forEach((node) => {
    const next = sanitizeNode(node, schema)
    if (next !== node) changed = true
    if (next) out.push(next)
  })
  return changed ? Fragment.fromArray(out) : fragment
}

function sanitizeNode(node: PMNode, schema: Schema): PMNode | null {
  let next = node

  // An image pasted from elsewhere keeps a literal address; one that is not a
  // web address is dropped entirely rather than left pointing at nothing.
  if (node.type.name === 'image') {
    const src = node.attrs.src
    if (!node.attrs.attachmentId && typeof src === 'string' && src !== '' && !isSafeLinkHref(src)) {
      return null
    }
  }

  const marks = sanitizeMarks(node.marks)
  if (marks !== node.marks) next = next.mark(marks)

  if (node.content.size > 0) {
    const content = sanitizeFragment(node.content, schema)
    if (content !== node.content) next = next.copy(content)
  }
  return next
}

function sanitizeMarks(marks: readonly Mark[]): readonly Mark[] {
  if (marks.length === 0) return marks
  const kept = marks.filter((mark) => {
    if (mark.type.name !== 'link') return true
    return isSafeLinkHref(mark.attrs.href)
  })
  return kept.length === marks.length ? marks : kept
}

/**
 * The node and mark types a pasted document may contain, derived from the
 * schema itself.
 *
 * Exported so a test can assert the thing that actually matters: that no rule
 * anywhere in the editor's schema matches an element a browser would execute.
 */
export function parseableTags(schema: Schema): string[] {
  const tags: string[] = []
  const collect = (spec: { parseDOM?: readonly { tag?: string }[] } | undefined) => {
    for (const rule of spec?.parseDOM ?? []) {
      if (rule.tag) tags.push(rule.tag)
    }
  }
  for (const type of Object.values(schema.nodes)) collect(type.spec as never)
  for (const type of Object.values(schema.marks)) collect(type.spec as never)
  return tags
}
