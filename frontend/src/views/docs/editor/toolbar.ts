// The floating toolbar that appears over a selection.
//
// Everything here is a decision about *whether* and *where*, kept apart from
// the component that draws it:
//
//   shouldShow      is there a selection this toolbar is any use for
//   anchorRect      the box the selection occupies
//   toolbarPlacement  where to put the bar so it stays on screen
//   TOOLBAR_ITEMS   what the bar offers, and how each entry is switched on
//
// The acceptance criterion behind the keyboard handling — every menu can be
// operated from the keyboard — is why the items carry an explicit order and
// why focus movement is a function rather than something the DOM works out:
// a toolbar is one tab stop, and the arrow keys move within it. That is the
// pattern assistive technology expects of a role="toolbar", and it is
// testable here rather than only in a browser.
import type { EditorState } from '@tiptap/pm/state'

/** One button on the bar. */
export interface ToolbarItem {
  /** Stable id, used as the key and in tests. */
  id: string
  /** The i18n key for the accessible name. */
  labelKey: string
  /** A tdesign icon name. */
  icon: string
  /** Buttons are drawn in groups with a divider between them. */
  group: 'format' | 'block' | 'insert'
  /** The mark or node this button reflects, for the pressed state. */
  activeName?: string
  /** Attributes that must also match for the button to read as pressed. */
  activeAttrs?: Record<string, unknown>
  /** The keyboard shortcut to show in the tooltip, in the platform's notation. */
  shortcut?: string
}

/**
 * The bar's contents, in the order they are read and moved through.
 *
 * Deliberately short. A floating toolbar that lists everything is a menu that
 * happens to float; this one carries what somebody reaches for while a piece
 * of text is selected, and the slash menu carries the rest.
 */
export const TOOLBAR_ITEMS: readonly ToolbarItem[] = [
  { id: 'bold', labelKey: 'docs.toolbar.bold', icon: 'format-bold', group: 'format', activeName: 'bold', shortcut: 'Mod+B' },
  { id: 'italic', labelKey: 'docs.toolbar.italic', icon: 'format-italic', group: 'format', activeName: 'italic', shortcut: 'Mod+I' },
  { id: 'underline', labelKey: 'docs.toolbar.underline', icon: 'format-underline', group: 'format', activeName: 'underline', shortcut: 'Mod+U' },
  { id: 'strike', labelKey: 'docs.toolbar.strike', icon: 'strikethrough', group: 'format', activeName: 'strike', shortcut: 'Mod+Shift+S' },
  { id: 'code', labelKey: 'docs.toolbar.code', icon: 'code', group: 'format', activeName: 'code', shortcut: 'Mod+E' },
  { id: 'highlight', labelKey: 'docs.toolbar.highlight', icon: 'highlight', group: 'format', activeName: 'highlight' },

  { id: 'heading1', labelKey: 'docs.toolbar.heading1', icon: 'format-vertical-align-top', group: 'block', activeName: 'heading', activeAttrs: { level: 1 } },
  { id: 'heading2', labelKey: 'docs.toolbar.heading2', icon: 'format-vertical-align-center', group: 'block', activeName: 'heading', activeAttrs: { level: 2 } },
  { id: 'bulletList', labelKey: 'docs.toolbar.bulletList', icon: 'list', group: 'block', activeName: 'bulletList' },
  { id: 'blockquote', labelKey: 'docs.toolbar.blockquote', icon: 'quote', group: 'block', activeName: 'blockquote' },

  { id: 'link', labelKey: 'docs.toolbar.link', icon: 'link', group: 'insert', activeName: 'link' },
  { id: 'clearFormat', labelKey: 'docs.toolbar.clearFormat', icon: 'format-clear', group: 'insert' },
]

/** The width the bar is laid out at, used to keep it on screen. */
export const TOOLBAR_WIDTH = 380
/** The bar's height plus the gap it keeps from the text. */
export const TOOLBAR_OFFSET = 46

/**
 * Whether the toolbar has anything to offer for the current selection.
 *
 * Three cases say no, and each is a real one rather than defensive coding: an
 * empty selection (there is nothing to format), a selection of a whole atom
 * such as an image or a diagram (none of these buttons apply to it), and a
 * selection inside a code block (where bold text would be a lie — the content
 * is code, and marks are not stored there).
 */
export function shouldShow(state: EditorState, editable: boolean): boolean {
  if (!editable) return false
  const { selection } = state
  if (selection.empty) return false

  const from = selection.$from
  // A node selection of an atom: `node` is set and it has no text inside.
  const node = (selection as { node?: { isAtom?: boolean } }).node
  if (node?.isAtom) return false

  for (let depth = from.depth; depth > 0; depth--) {
    const parent = from.node(depth)
    if (parent.type.spec.code) return false
  }
  return true
}

/** A rectangle in viewport coordinates. */
export interface Rect {
  left: number
  top: number
  right: number
  bottom: number
}

/** Where the bar should be drawn, in viewport coordinates. */
export interface Placement {
  left: number
  top: number
  /** True when there was no room above and the bar sits under the selection. */
  below: boolean
}

/**
 * Places the bar centred over the selection, kept inside the viewport.
 *
 * It prefers to sit above the selection, because that is the edge a person is
 * not about to keep selecting towards; when the selection starts near the top
 * of the window it flips underneath rather than being clipped.
 */
export function toolbarPlacement(
  rect: Rect,
  viewport: { width: number; height: number },
  margin = 8,
): Placement {
  const centre = (rect.left + rect.right) / 2
  const left = clamp(centre - TOOLBAR_WIDTH / 2, margin, Math.max(margin, viewport.width - TOOLBAR_WIDTH - margin))

  const above = rect.top - TOOLBAR_OFFSET
  if (above >= margin) return { left, top: above, below: false }

  const below = Math.min(rect.bottom + 8, viewport.height - TOOLBAR_OFFSET)
  return { left, top: Math.max(margin, below), below: true }
}

function clamp(value: number, low: number, high: number): number {
  return Math.min(high, Math.max(low, value))
}

/**
 * Moves focus within the bar, wrapping at both ends.
 *
 * Wrapping is the behaviour a toolbar is expected to have: there is no
 * "past the end" to fall into, and a person holding the arrow key should
 * cycle rather than stick. Returns the index unchanged when the bar is empty,
 * which is the only case with nowhere to go.
 */
export function moveFocus(current: number, delta: number, length: number): number {
  if (length <= 0) return 0
  return ((current + delta) % length + length) % length
}

/** The item ids, grouped in order, so the view can draw dividers. */
export function toolbarGroups(items: readonly ToolbarItem[] = TOOLBAR_ITEMS): ToolbarItem[][] {
  const groups: ToolbarItem[][] = []
  let current: ToolbarItem['group'] | null = null
  for (const item of items) {
    if (item.group !== current) {
      groups.push([])
      current = item.group
    }
    groups[groups.length - 1]!.push(item)
  }
  return groups
}
