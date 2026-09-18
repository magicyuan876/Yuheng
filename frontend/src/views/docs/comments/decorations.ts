// Drawing comment highlights.
//
// A decoration, never a mark. This is the design decision the whole comment
// feature rests on, so it is worth stating plainly rather than leaving it to
// be inferred from the absence of a mark:
//
//   * A mark would be part of the document. It would be stored, sent to
//     everybody editing, exported, rendered on a share page and indexed.
//   * It would be versioned with the document, so restoring an old revision
//     would resurrect comments that were resolved since, and remove ones made
//     after it — while the comments themselves have no versions at all.
//   * It would be written by whoever happened to be typing, because that is
//     how a collaborative document is written, so a reader with no write
//     access could not comment.
//
// None of those is a small problem, and all three go away by keeping the
// highlight where it belongs: in the view, alongside the upload placeholder
// and the find-and-replace highlight, which are decorations for the same
// reason — they are what one person is looking at, not what the document
// says.

import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

import { isInline, type Placement } from './placement'

/** What the plugin is told to draw. */
export interface CommentHighlights {
  placements: readonly Placement[]
  /** The thread the reader is looking at, drawn more strongly. */
  activeID?: string
}

export const commentDecorationKey = new PluginKey<CommentHighlights>('yuhengComments')

/** The class every highlight carries, so one rule styles them all. */
export const HIGHLIGHT_CLASS = 'docs-comment-mark'
/** Added to the thread currently open in the sidebar. */
export const ACTIVE_CLASS = 'is-active'

/**
 * Builds the decorations for a set of placements.
 *
 * Exported apart from the plugin so the mapping from placements to what is
 * drawn can be tested without a view: an overlapping pair of comments is the
 * case that gets this wrong, and it is easier to assert on a DecorationSet
 * than to look at a browser.
 */
export function commentDecorations(
  doc: Parameters<typeof DecorationSet.create>[0],
  highlights: CommentHighlights,
): DecorationSet {
  const decorations: Decoration[] = []
  for (const placement of highlights.placements) {
    if (!isInline(placement)) continue
    if (placement.from === undefined || placement.to === undefined) continue

    const classes = [HIGHLIGHT_CLASS]
    if (placement.id === highlights.activeID) classes.push(ACTIVE_CLASS)
    // A comment whose place was found by its quotation rather than by its
    // stored position is drawn differently, because it is a guess that
    // happens to be a good one and the reader should be able to tell.
    if (placement.kind === 'quoted') classes.push('is-approximate')

    decorations.push(Decoration.inline(placement.from, placement.to, {
      class: classes.join(' '),
      // Read back by the click handler to know which thread was clicked.
      'data-comment-id': placement.id,
    }))
  }
  return DecorationSet.create(doc, decorations)
}

/**
 * The plugin.
 *
 * It holds the placements rather than recomputing them, because working out
 * where a comment points needs the Yjs document and the comment list, neither
 * of which a plugin should reach for. The composable computes them and hands
 * them over through a transaction's metadata.
 */
export function commentDecorationPlugin(
  onClick?: (commentID: string) => void,
): Plugin<CommentHighlights> {
  const empty: CommentHighlights = { placements: [] }
  return new Plugin<CommentHighlights>({
    key: commentDecorationKey,
    state: {
      init: () => empty,
      apply: (tr, value) => {
        const next = tr.getMeta(commentDecorationKey) as CommentHighlights | undefined
        if (next) return next
        if (!tr.docChanged) return value
        // The document moved under the placements. They are recomputed by the
        // composable on the next tick; drawing stale ranges meanwhile would
        // put a highlight on words nobody commented on.
        return empty
      },
    },
    props: {
      decorations: (state) => {
        const value = commentDecorationKey.getState(state)
        if (!value || value.placements.length === 0) return DecorationSet.empty
        return commentDecorations(state.doc, value)
      },
      handleClick: (view, _pos, event) => {
        if (!onClick) return false
        const target = event.target as HTMLElement | null
        const marked = target?.closest?.(`.${HIGHLIGHT_CLASS}`)
        const id = marked?.getAttribute('data-comment-id')
        if (!id) return false
        onClick(id)
        // Not consumed: clicking a commented passage should still put the
        // cursor there, the way clicking any other text does.
        return false
      },
    },
  })
}

/**
 * Hands a new set of highlights to the plugin.
 *
 * Dispatched with addToHistory false and no document change: what one person
 * is looking at is not an edit, and must not reach anybody else's undo stack
 * or the collaboration service.
 */
export function setCommentHighlights(
  view: { state: { tr: { setMeta: (k: unknown, v: unknown) => unknown } }; dispatch: (tr: unknown) => void },
  highlights: CommentHighlights,
): void {
  const tr = view.state.tr.setMeta(commentDecorationKey, highlights) as {
    setMeta: (k: unknown, v: unknown) => unknown
  }
  tr.setMeta('addToHistory', false)
  view.dispatch(tr)
}
