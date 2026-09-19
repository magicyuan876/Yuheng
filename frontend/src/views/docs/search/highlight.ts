// Marking the matched words inside an excerpt.
//
// A pure function returning segments rather than an HTML string, so the
// component renders them as text nodes. Building markup here and injecting it
// with v-html would make every document body a place somebody could put a
// script tag — the excerpt comes from page content, which is exactly the
// untrusted text this product exists to store.

/** One piece of an excerpt, marked or not. */
export interface Segment {
  text: string
  match: boolean
}

/**
 * Splits text into matched and unmatched runs.
 *
 * Case-insensitive for Latin; Chinese has no case, so the fold is a no-op on
 * the text this mostly runs against. Overlapping terms are merged rather
 * than nested, because a segment cannot be inside another one.
 */
export function highlight(text: string, query: string): Segment[] {
  if (!text) return []
  const terms = splitTerms(query)
  if (terms.length === 0) return [{ text, match: false }]

  const lower = text.toLowerCase()
  // Collect every match of every term, then merge, so that searching
  // "quota limit" marks both words wherever they appear.
  const ranges: Array<[number, number]> = []
  for (const term of terms) {
    const needle = term.toLowerCase()
    let from = 0
    for (;;) {
      const at = lower.indexOf(needle, from)
      if (at < 0) break
      ranges.push([at, at + needle.length])
      from = at + needle.length
    }
  }
  if (ranges.length === 0) return [{ text, match: false }]

  ranges.sort((a, b) => a[0] - b[0])
  const merged: Array<[number, number]> = []
  for (const range of ranges) {
    const last = merged[merged.length - 1]
    if (last && range[0] <= last[1]) {
      last[1] = Math.max(last[1], range[1])
      continue
    }
    merged.push([range[0], range[1]])
  }

  const out: Segment[] = []
  let cursor = 0
  for (const [start, end] of merged) {
    if (start > cursor) out.push({ text: text.slice(cursor, start), match: false })
    out.push({ text: text.slice(start, end), match: true })
    cursor = end
  }
  if (cursor < text.length) out.push({ text: text.slice(cursor), match: false })
  return out
}

/**
 * The terms to mark.
 *
 * A query with no spaces is one term, which is what a Chinese query is:
 * splitting it per character would mark every character of the excerpt and
 * highlight nothing useful.
 */
export function splitTerms(query: string): string[] {
  return query
    .trim()
    .split(/\s+/)
    .filter((term) => term.length > 0)
}
