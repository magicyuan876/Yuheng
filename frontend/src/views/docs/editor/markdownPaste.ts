// Pasting Markdown.
//
// When the clipboard carries HTML, that is what ProseMirror parses and this
// file is not involved. It matters for the other case: text copied out of a
// terminal, a code review, a README or another editor arrives as plain text,
// and pasting it verbatim turns a structured document into a wall of hashes
// and hyphens.
//
// The decision — is this text Markdown, or is it prose that happens to
// contain a hyphen — is the part worth being careful about, and it is the
// part that is testable. Getting it wrong in one direction leaves the marks
// in the document; getting it wrong in the other silently restructures
// something somebody pasted as text.
import { marked } from 'marked'

/**
 * Patterns that only appear in Markdown, one per line-shape.
 *
 * Deliberately narrow. Emphasis (`*x*`) and inline code are missing on
 * purpose: they are common enough in ordinary prose, and a paste that is only
 * emphasis loses nothing by staying literal, whereas a paste that is a list
 * or a table loses its structure.
 */
const MARKDOWN_LINE = [
  /^\s{0,3}#{1,6}\s+\S/, // heading
  /^\s{0,3}([-*+])\s+\S/, // bullet list
  /^\s{0,3}\d+[.)]\s+\S/, // ordered list
  /^\s{0,3}>\s/, // quote
  /^\s{0,3}```/, // fence
  /^\s{0,3}(\|.*\|)\s*$/, // table row
  /^\s{0,3}([-*_])\s*(\1\s*){2,}$/, // thematic break
  /^\s{0,3}- \[[ xX]\]\s/, // task item
] as const

/** Inline shapes, which only count towards the decision alongside a line one. */
const MARKDOWN_INLINE = [
  /\[[^\]]+\]\([^)]+\)/, // link
  /!\[[^\]]*\]\([^)]+\)/, // image
  /`[^`\n]+`/, // inline code
  /\*\*[^*\n]+\*\*/, // strong
] as const

/**
 * Whether a piece of pasted text should be read as Markdown.
 *
 * The rule: at least one line that could only be Markdown, or — for a single
 * line — an inline shape as well, so that pasting "see [the docs](x)" on its
 * own still becomes a link while pasting "the range is 3 - 5" does not become
 * a list.
 */
export function looksLikeMarkdown(text: string): boolean {
  const trimmed = text.trim()
  if (trimmed === '') return false

  const lines = trimmed.split(/\r?\n/)
  const structural = lines.filter((line) => MARKDOWN_LINE.some((re) => re.test(line))).length
  if (structural > 0) return true

  // A single line with no structure of its own needs an inline shape to be
  // worth converting; several lines of prose are just prose.
  return MARKDOWN_INLINE.some((re) => re.test(trimmed))
}

/**
 * Renders Markdown to HTML.
 *
 * No sanitising happens here: the result goes through the same DOMPurify
 * configuration and the same schema as any other pasted HTML, and doing it in
 * one place is what makes that guarantee readable. Returns the text escaped
 * as a paragraph if the conversion fails, rather than losing the paste.
 */
export function markdownToHTML(text: string): string {
  try {
    const html = marked.parse(text, { async: false, gfm: true, breaks: false })
    return typeof html === 'string' ? html : escapeHTML(text)
  } catch {
    return escapeHTML(text)
  }
}

function escapeHTML(text: string): string {
  return `<p>${text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')}</p>`
}
