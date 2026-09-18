// The decisions the structural and figure nodes need, separated from their
// views so they are exercised directly by tests: how a formula is rendered,
// how column widths are shared out, and the closed vocabularies the schema
// declares for callouts and status chips.
//
// KaTeX renders to a string with no DOM, so the maths here runs in a plain
// Node test. Mermaid does not: its renderer needs a document, so the node view
// calls the existing shared helper and this file only decides the ids.
import katex from 'katex'

/** Callout kinds, exactly as packages/docs-schema declares them. */
export const CALLOUT_KINDS = ['info', 'success', 'warning', 'danger'] as const
export type CalloutKind = (typeof CALLOUT_KINDS)[number]

/** Status chip colours, exactly as packages/docs-schema declares them. */
export const STATUS_COLORS = ['gray', 'blue', 'green', 'yellow', 'red', 'purple'] as const
export type StatusColor = (typeof STATUS_COLORS)[number]

/** A column layout holds between two and five columns (schema: `column{2,5}`). */
export const MIN_COLUMNS = 2
export const MAX_COLUMNS = 5

/** The default icon shown for each callout kind when none was chosen. */
const CALLOUT_ICONS: Record<CalloutKind, string> = {
  info: 'ℹ️',
  success: '✅',
  warning: '⚠️',
  danger: '⛔',
}

/** Keeps an unknown value inside the schema's vocabulary. */
export function calloutKind(value: unknown): CalloutKind {
  return CALLOUT_KINDS.includes(value as CalloutKind) ? (value as CalloutKind) : 'info'
}

export function statusColor(value: unknown): StatusColor {
  return STATUS_COLORS.includes(value as StatusColor) ? (value as StatusColor) : 'gray'
}

/** The icon to draw for a callout: the author's, or the kind's default. */
export function calloutIcon(kind: unknown, icon: unknown): string {
  const chosen = typeof icon === 'string' ? icon.trim() : ''
  return chosen || CALLOUT_ICONS[calloutKind(kind)]
}

/** The rendered form of one formula. */
export interface MathResult {
  html: string
  /** Set when KaTeX could not parse the source; the view shows it instead of
   * pretending the formula rendered. */
  error: string
}

/**
 * Renders LaTeX to HTML.
 *
 * Errors never throw. A formula someone is halfway through typing is not an
 * exceptional condition, and a throw here would tear down the node view and
 * take the rest of the page's rendering with it. The caller gets the message
 * and decides how to show it.
 */
export function renderMath(latex: string, display: boolean): MathResult {
  const source = latex ?? ''
  if (!source.trim()) return { html: '', error: '' }
  try {
    return {
      html: katex.renderToString(source, {
        displayMode: display,
        throwOnError: true,
        strict: false,
        // Macros that expand into other macros are how a formula can be made
        // to cost far more than it looks; KaTeX bounds it, and this keeps the
        // bound tight.
        maxExpand: 1000,
        trust: false,
      }),
      error: '',
    }
  } catch (err) {
    return { html: '', error: messageOf(err) }
  }
}

function messageOf(err: unknown): string {
  const message = (err as { message?: string })?.message
  return typeof message === 'string' && message ? message : 'invalid formula'
}

/**
 * Shares the width of a row out among its columns.
 *
 * Widths are optional per column: a column with none takes an equal share of
 * whatever the sized ones left over. The result always sums to 100 so the row
 * fills its line however the author edited it, and every column keeps at least
 * the schema's five percent so none can be made to vanish.
 */
export function columnWidths(widths: readonly (number | null | undefined)[]): number[] {
  const count = widths.length
  if (count === 0) return []

  const MIN = 5
  const sized = widths.map((w) => (typeof w === 'number' && Number.isFinite(w) ? clamp(w, MIN, 95) : null))
  const declared = sized.filter((w): w is number => w !== null)
  const free = count - declared.length

  if (free === 0) {
    const total = declared.reduce((a, b) => a + b, 0)
    // Scale rather than reject: an author dragging two dividers can leave the
    // row summing to something other than 100 for an instant.
    return total > 0 ? sized.map((w) => round((w as number) * 100 / total)) : evenly(count)
  }

  const used = declared.reduce((a, b) => a + b, 0)
  const remaining = Math.max(free * MIN, 100 - used)
  const each = round(remaining / free)
  const out = sized.map((w) => (w === null ? each : w))
  return normaliseTo100(out)
}

/** Equal shares, used when nothing is declared. */
function evenly(count: number): number[] {
  return normaliseTo100(Array.from({ length: count }, () => round(100 / count)))
}

/** Nudges the last column so rounding does not leave a gap or an overflow. */
function normaliseTo100(values: number[]): number[] {
  const out = [...values]
  const total = out.reduce((a, b) => a + b, 0)
  const drift = round(100 - total)
  if (drift !== 0 && out.length > 0) {
    out[out.length - 1] = round(Math.max(5, out[out.length - 1]! + drift))
  }
  return out
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

function round(value: number): number {
  return Math.round(value * 100) / 100
}

/**
 * A render id for one diagram.
 *
 * Mermaid writes the id into the SVG it produces, so two diagrams sharing one
 * would collide in the document's id space and break the second one's internal
 * references. Deriving it from the block's own id keeps them distinct and
 * keeps a re-render of the same block stable.
 */
export function mermaidId(blockId: unknown, fallback: number): string {
  const id = typeof blockId === 'string' ? blockId.replace(/[^A-Za-z0-9_-]/g, '') : ''
  return `docs-mermaid-${id || fallback}`
}
