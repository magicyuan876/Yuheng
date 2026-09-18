// Where a comment points.
//
// An inline comment stores a Yjs relative position, which survives other
// people editing around it — that is the whole reason it is a relative
// position and not a pair of offsets. But it does not survive everything: the
// passage it pointed at can be deleted outright, and a page written without a
// collaboration service has its Yjs state rebuilt from stored JSON, which
// leaves every relative position in it pointing at nothing.
//
// So there are three answers, and the difference between them is what a
// reader sees:
//
//   anchored  the position resolved; highlight exactly that range
//   quoted    it did not, but the quotation appears once in the document, so
//             the comment is about that passage and is highlighted there
//   orphaned  neither; the comment is shown against its quotation in the
//             sidebar and marked as having lost its place
//
// The decision is here, as a pure function over a document and an injected
// resolver, so all three can be tested without a browser, a Yjs document or a
// collaboration service.

import type { Node as PMNode } from '@tiptap/pm/model'

/** What a comment carries about where it points. */
export interface CommentAnchorInput {
  id: string
  /** The stored relative position; absent for a page-level comment. */
  anchor?: unknown
  /** What the range covered when the comment was made. */
  quotedText?: string
}

/** How a comment was placed. */
export type PlacementKind = 'page' | 'anchored' | 'quoted' | 'orphaned'

export interface Placement {
  id: string
  kind: PlacementKind
  /** The range to highlight; absent for 'page' and 'orphaned'. */
  from?: number
  to?: number
}

/**
 * Resolves a stored relative position against the live document.
 *
 * Injected because doing it needs the Yjs document and the editor's binding,
 * neither of which belongs in a decision about what to highlight. Returns
 * null when the position no longer resolves.
 */
export type ResolveAnchor = (anchor: unknown) => { from: number; to: number } | null

/** The shortest quotation worth searching for. */
export const MIN_QUOTE_LENGTH = 4

/**
 * Decides where one comment points.
 *
 * The order is deliberate: the relative position is tried first and trusted
 * when it works, because it is the only thing that knows the difference
 * between two identical sentences. The quotation is a fallback, not a
 * cross-check — a comment does not move because somebody made the text it
 * points at match another passage.
 */
export function placeComment(
  doc: PMNode,
  comment: CommentAnchorInput,
  resolve: ResolveAnchor,
): Placement {
  if (comment.anchor === undefined || comment.anchor === null) {
    return { id: comment.id, kind: 'page' }
  }

  const resolved = safeResolve(resolve, comment.anchor)
  if (resolved && isSaneRange(doc, resolved)) {
    return { id: comment.id, kind: 'anchored', from: resolved.from, to: resolved.to }
  }

  const found = findQuotation(doc, comment.quotedText ?? '')
  if (found) {
    return { id: comment.id, kind: 'quoted', from: found.from, to: found.to }
  }
  return { id: comment.id, kind: 'orphaned' }
}

/** Places every comment in one pass. */
export function placeComments(
  doc: PMNode,
  comments: readonly CommentAnchorInput[],
  resolve: ResolveAnchor,
): Placement[] {
  return comments.map((comment) => placeComment(doc, comment, resolve))
}

/**
 * A resolver that throws is a resolver that said no.
 *
 * y-prosemirror throws rather than returning null for some shapes of stale
 * position, and a comment sidebar that disappears because one comment's
 * anchor is from an older client would be a poor trade.
 */
function safeResolve(resolve: ResolveAnchor, anchor: unknown) {
  try {
    return resolve(anchor)
  } catch {
    return null
  }
}

/** Whether a resolved range is one this document actually has. */
function isSaneRange(doc: PMNode, range: { from: number; to: number }): boolean {
  if (!Number.isFinite(range.from) || !Number.isFinite(range.to)) return false
  if (range.from < 0 || range.to > doc.content.size) return false
  // A range that collapsed to nothing no longer covers a passage: the text it
  // was about has been deleted, and the quotation is the better answer.
  return range.to > range.from
}

/**
 * Finds a quotation in the document, if it appears exactly once.
 *
 * Exactly once is the rule, and the interesting half of it is what happens
 * otherwise: a comment that matches two passages is reported as orphaned
 * rather than attached to the first. Pointing at the wrong paragraph is worse
 * than admitting the place was lost — the reader would see a remark about
 * text it was never about, with nothing to suggest anything went wrong.
 */
export function findQuotation(doc: PMNode, quote: string): { from: number; to: number } | null {
  const needle = normalise(quote)
  if (needle.length < MIN_QUOTE_LENGTH) return null

  let found: { from: number; to: number } | null = null
  let ambiguous = false

  doc.descendants((node, pos) => {
    if (ambiguous) return false
    if (!node.isTextblock) return true

    const text = normalise(node.textBetween(0, node.content.size, undefined, ' '))
    let at = text.indexOf(needle)
    while (at !== -1) {
      if (found) {
        ambiguous = true
        return false
      }
      // +1 for the position inside the block, which is where its text starts.
      found = { from: pos + 1 + at, to: pos + 1 + at + needle.length }
      at = text.indexOf(needle, at + 1)
    }
    return true
  })

  return ambiguous ? null : found
}

/**
 * Collapses whitespace, the same way the server does when it stores a
 * quotation.
 *
 * This is what lets the fallback survive an edit that only re-wrapped a
 * paragraph — and it is why both sides have to do it the same way, which is
 * why the server's CleanQuotedText and this function are written to match.
 */
export function normalise(text: string): string {
  return text.replace(/\s+/g, ' ').trim()
}

/** Whether a placement puts the comment somewhere in the text. */
export function isInline(placement: Placement): boolean {
  return placement.kind === 'anchored' || placement.kind === 'quoted'
}

/**
 * Groups placements by kind, so a sidebar can show the ones that lost their
 * place together rather than scattered among the rest.
 */
export function partitionPlacements(placements: readonly Placement[]): {
  inline: Placement[]
  page: Placement[]
  orphaned: Placement[]
} {
  const inline: Placement[] = []
  const page: Placement[] = []
  const orphaned: Placement[] = []
  for (const placement of placements) {
    if (placement.kind === 'orphaned') orphaned.push(placement)
    else if (placement.kind === 'page') page.push(placement)
    else inline.push(placement)
  }
  // Inline comments read top to bottom, as the passages they are about do.
  inline.sort((a, b) => (a.from ?? 0) - (b.from ?? 0))
  return { inline, page, orphaned }
}
