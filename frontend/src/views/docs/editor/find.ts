// Find and replace.
//
// The search is a pure function over the document: a query and some options
// in, a list of ranges out. Nothing here highlights anything or moves the
// cursor — the plugin below turns the ranges into decorations, and the panel
// decides which one is current. That split is what makes the hard parts
// testable: where a match sits when it spans two text nodes, what happens to
// the other matches after one is replaced, and whether a broken regular
// expression takes the editor down with it.
import type { Node as PMNode } from '@tiptap/pm/model'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { EditorState, Transaction } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

/** A range in the document. */
export interface Match {
  from: number
  to: number
}

export interface FindOptions {
  caseSensitive?: boolean
  /** Matches only where the query stands as a word of its own. */
  wholeWord?: boolean
  /** Reads the query as a regular expression rather than as literal text. */
  regex?: boolean
}

/** A cap, so a query like "e" on a very long page cannot stall the editor. */
export const MAX_MATCHES = 5000

/**
 * Builds the regular expression a search runs.
 *
 * Returns null for a query that cannot be compiled, which is not an edge case
 * but the normal state of things: somebody typing a regular expression passes
 * through "(" on the way to "(a|b)", and an editor that throws at that moment
 * is unusable. The caller shows no matches and waits.
 */
export function searchRegex(query: string, options: FindOptions = {}): RegExp | null {
  if (query === '') return null
  const body = options.regex ? query : escapeRegex(query)
  const pattern = options.wholeWord ? `(?<![\\p{L}\\p{N}_])${body}(?![\\p{L}\\p{N}_])` : body
  try {
    return new RegExp(pattern, options.caseSensitive ? 'gu' : 'giu')
  } catch {
    return null
  }
}

function escapeRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Finds every match in the document.
 *
 * Text is gathered per text-containing block rather than for the whole
 * document at once, so a match can never run across a block boundary — "the
 * end" at the end of one paragraph and "the end" at the start of the next are
 * two matches, not one spanning the gap, which is what a person searching
 * expects and what keeps the returned ranges valid positions.
 */
export function findMatches(doc: PMNode, query: string, options: FindOptions = {}): Match[] {
  const re = searchRegex(query, options)
  if (!re) return []

  const matches: Match[] = []
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true
    // The block's text with one character per position, so an offset into
    // this string is an offset into the document.
    const text = node.textBetween(0, node.content.size, undefined, '￼')
    re.lastIndex = 0
    let found: RegExpExecArray | null
    while ((found = re.exec(text)) !== null) {
      // A pattern that can match nothing would spin forever.
      if (found[0] === '') {
        re.lastIndex++
        continue
      }
      const from = pos + 1 + found.index
      matches.push({ from, to: from + found[0].length })
      if (matches.length >= MAX_MATCHES) return false
    }
    return true
  })
  return matches
}

/**
 * The index of the match at or after a position, wrapping to the start.
 *
 * This is what "find next" means when the cursor has been moved by hand
 * between searches: the next match is the next one *in the document*, not the
 * one after whichever was highlighted last.
 */
export function matchAfter(matches: readonly Match[], pos: number): number {
  if (matches.length === 0) return -1
  const at = matches.findIndex((m) => m.from >= pos)
  return at === -1 ? 0 : at
}

/** Steps through the matches, wrapping at both ends. */
export function stepMatch(current: number, delta: number, total: number): number {
  if (total <= 0) return -1
  return ((current + delta) % total + total) % total
}

/**
 * Replaces one match.
 *
 * The replacement is inserted as plain text with no marks carried over, which
 * is the honest behaviour: the text being replaced is gone, and inheriting
 * its formatting would quietly apply a mark somebody never chose.
 */
export function replaceMatch(state: EditorState, match: Match, replacement: string): Transaction {
  const tr = state.tr
  if (replacement === '') tr.delete(match.from, match.to)
  else tr.insertText(replacement, match.from, match.to)
  return tr
}

/**
 * Replaces every match in one transaction.
 *
 * Applied back to front, so each edit leaves the positions of the matches
 * before it untouched — the alternative, mapping every remaining position
 * through every step, is the same answer arrived at more expensively and with
 * more to get wrong. One transaction rather than many also means one undo and
 * one message to the other people editing.
 */
export function replaceAll(
  state: EditorState,
  matches: readonly Match[],
  replacement: string,
): Transaction | null {
  if (matches.length === 0) return null
  const tr = state.tr
  for (let i = matches.length - 1; i >= 0; i--) {
    const match = matches[i]!
    if (replacement === '') tr.delete(match.from, match.to)
    else tr.insertText(replacement, match.from, match.to)
  }
  return tr
}

/** What the highlight plugin is told to draw. */
export interface FindHighlight {
  matches: readonly Match[]
  /** The index of the one that is current, or -1 for none. */
  current: number
}

export const findKey = new PluginKey<FindHighlight>('yuhengFind')

/**
 * Draws the matches.
 *
 * A decoration rather than a mark, for the same reason the upload placeholder
 * is one: a highlight is something this person is looking at right now, not a
 * change to the document, and it must not be saved or sent to anybody else.
 */
export function findPlugin(): Plugin<FindHighlight> {
  const empty: FindHighlight = { matches: [], current: -1 }
  return new Plugin<FindHighlight>({
    key: findKey,
    state: {
      init: () => empty,
      apply: (tr, value) => {
        const next = tr.getMeta(findKey) as FindHighlight | undefined
        if (next) return next
        if (!tr.docChanged) return value
        // The document moved under the matches; they are recomputed by the
        // panel, and stale ranges must not be drawn meanwhile.
        return empty
      },
    },
    props: {
      decorations: (state) => {
        const value = findKey.getState(state)
        if (!value || value.matches.length === 0) return DecorationSet.empty
        return DecorationSet.create(state.doc, value.matches.map((match, index) => Decoration.inline(
          match.from,
          match.to,
          { class: index === value.current ? 'docs-find-match is-current' : 'docs-find-match' },
        )))
      },
    },
  })
}
