// The Markdown shorthands for the nodes this editor defines itself.
//
// The stock extensions each bring their own: "# " makes a heading, "- " a
// list, "> " a quote, "```" a code block, "---" a rule. Those are not
// repeated here. What is here is the same idea extended to the nodes that
// have no upstream extension — a callout, a formula, a diagram — so that
// somebody who types Markdown out of habit gets what they meant rather than
// three colons sitting in a paragraph.
//
// The patterns live in their own module, apart from the node definitions, for
// one reason: a pattern is easy to get subtly wrong, and wrong here means
// either a shorthand that never fires or one that fires while somebody is
// writing ordinary prose. Both are testable, and neither is testable through
// a node definition.
//
// A shared convention across all of them: the pattern ends at a space, so the
// shorthand completes on the space that follows it. Firing on the last
// character of the marker itself would mean "$$" could never be typed as
// text, and would surprise anybody who paused mid-word.

/**
 * A callout, optionally naming its kind: ":::", ":::warning", ":::tip".
 *
 * The kind is captured so the rule can pass it through; an unnamed callout
 * takes the default. Anchored to the start of the block, so a colon sequence
 * inside a sentence is left alone.
 */
export const CALLOUT_INPUT = /^:::(info|note|tip|success|warning|danger)?[\s]$/

/** A display formula: "$$" followed by a space. */
export const MATH_BLOCK_INPUT = /^\$\$[\s]$/

/**
 * An inline formula: "$x^2$".
 *
 * Unanchored, because it fires wherever it is typed in a line. The body may
 * not contain a dollar or a space adjacent to the delimiters, which is what
 * keeps "$5 and $6" from becoming a formula — the common false positive, and
 * the reason this is worth a test.
 */
export const MATH_INLINE_INPUT = /(?:^|[\s(])\$([^$\s][^$]*[^$\s]|[^$\s])\$$/

/** A diagram: a fenced block naming mermaid. */
export const MERMAID_INPUT = /^```mermaid[\s]$/

/** A page break: "+++" followed by a space. "---" is already a rule. */
export const PAGE_BREAK_INPUT = /^\+\+\+[\s]$/

/** Columns: ":::columns" or ":::columns3" for a particular count. */
export const COLUMNS_INPUT = /^:::columns([2-5])?[\s]$/

/**
 * Reads the callout kind out of a match, falling back to the default.
 *
 * Separated from the pattern so the fallback is one decision in one place
 * rather than repeated at each call site; "note" is spelled differently in
 * Markdown dialects and means the same thing here.
 */
export function calloutKindFromMatch(match: RegExpMatchArray): string {
  const raw = match[1]
  if (!raw) return 'info'
  return raw === 'note' ? 'info' : raw
}

/** Reads the column count out of a match, clamped to what the schema allows. */
export function columnCountFromMatch(match: RegExpMatchArray): number {
  const raw = Number(match[1])
  if (!Number.isFinite(raw)) return 2
  return Math.min(5, Math.max(2, Math.trunc(raw)))
}
