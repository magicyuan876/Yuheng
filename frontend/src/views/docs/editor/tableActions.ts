// What the table toolbar offers, kept apart from the bar that draws it.
//
// The same split as toolbar.ts, for the same reason: which entries a caret
// inside a table should be offered, and whether each one can run at all, is a
// decision worth testing without a browser. The bar itself then only has to
// draw a list and call `run`.
//
// The catalogue is deliberately the set Feishu and Yuque both put on a table:
// rows and columns in and out, a header row, merge and split, a cell colour,
// and a way to remove the table. Anything rarer belongs in the slash menu.

/** A chain of editor commands, kept loose because it is the editor's own. */
type Chain = Record<string, (...args: never[]) => unknown>

/** The narrow view of the editor an entry needs. */
export interface TableTarget {
  chain: () => Chain
  can: () => { chain: () => Chain }
}

/** One button on the table bar. */
export interface TableAction {
  /** Stable id, used as the key and in tests. */
  id: string
  /** The i18n key for the accessible name. */
  labelKey: string
  /** A tdesign icon name. */
  icon: string
  /** Buttons are drawn in groups with a divider between them. */
  group: 'row' | 'column' | 'cell' | 'table'
  /**
   * True for the entry that opens the cell-colour palette rather than
   * running a command, the same arrangement the selection bar uses.
   */
  palette?: boolean
  /** The command's name on the editor's chain; absent for the palette. */
  command?: string
  /** Arguments for that command, when it takes any. */
  args?: unknown
  /** True for an entry that removes something, drawn in the danger colour. */
  danger?: boolean
}

/**
 * The cell colours the palette offers, behind a "default" entry that clears
 * the colour again. Tints rather than the saturated text colours: a cell fill
 * sits behind words and has to stay readable under them.
 */
export const CELL_COLORS: readonly string[] = [
  '#ffe3e3', '#ffe8cc', '#fff3bf', '#d3f9d8',
  '#c5f6fa', '#d0ebff', '#e5dbff', '#f3f0ff', '#f1f3f5',
]

/** The bar's contents, in the order they are read and moved through. */
export const TABLE_ACTIONS: readonly TableAction[] = [
  { id: 'addRowBefore', labelKey: 'docs.table.addRowBefore', icon: 'arrow-up', group: 'row', command: 'addRowBefore' },
  { id: 'addRowAfter', labelKey: 'docs.table.addRowAfter', icon: 'arrow-down', group: 'row', command: 'addRowAfter' },
  { id: 'deleteRow', labelKey: 'docs.table.deleteRow', icon: 'minus-rectangle', group: 'row', command: 'deleteRow', danger: true },

  { id: 'addColumnBefore', labelKey: 'docs.table.addColumnBefore', icon: 'arrow-left', group: 'column', command: 'addColumnBefore' },
  { id: 'addColumnAfter', labelKey: 'docs.table.addColumnAfter', icon: 'arrow-right', group: 'column', command: 'addColumnAfter' },
  { id: 'deleteColumn', labelKey: 'docs.table.deleteColumn', icon: 'minus-rectangle', group: 'column', command: 'deleteColumn', danger: true },

  { id: 'mergeCells', labelKey: 'docs.table.mergeCells', icon: 'merge-cells', group: 'cell', command: 'mergeCells' },
  { id: 'splitCell', labelKey: 'docs.table.splitCell', icon: 'table-split', group: 'cell', command: 'splitCell' },
  { id: 'cellColor', labelKey: 'docs.table.cellColor', icon: 'fill-color', group: 'cell', palette: true },

  { id: 'toggleHeaderRow', labelKey: 'docs.table.toggleHeaderRow', icon: 'table-1', group: 'table', command: 'toggleHeaderRow' },
  { id: 'deleteTable', labelKey: 'docs.table.deleteTable', icon: 'delete', group: 'table', command: 'deleteTable', danger: true },
]

/**
 * Whether an entry can run on the current selection.
 *
 * Asked of the editor rather than worked out here: merging needs more than
 * one cell selected and splitting needs a merged one, and ProseMirror's own
 * table commands already know both. A button that cannot run is drawn
 * disabled rather than hidden, so the bar does not change width as the caret
 * moves between cells.
 */
export function canRun(editor: TableTarget | null, action: TableAction): boolean {
  if (!editor) return false
  if (action.palette) return true
  if (!action.command) return false
  try {
    const chain = editor.can().chain() as Chain
    const call = chain[action.command] as ((...a: never[]) => { run?: () => boolean }) | undefined
    if (typeof call !== 'function') return false
    return call.call(chain)?.run?.() === true
  } catch {
    // A command the schema does not have, which happens while extensions are
    // still being swapped on a page change.
    return false
  }
}

/** Runs an entry. Returns false when it had nothing to call. */
export function runAction(editor: TableTarget | null, action: TableAction): boolean {
  if (!editor || !action.command) return false
  const chain = (editor.chain() as Chain).focus as unknown as () => Chain
  const focused = typeof chain === 'function' ? chain() : (editor.chain() as Chain)
  const call = (focused as Chain)[action.command] as
    ((...a: never[]) => { run?: () => void }) | undefined
  if (typeof call !== 'function') return false
  const result = action.args === undefined
    ? call.call(focused)
    : (call as (a: unknown) => { run?: () => void }).call(focused, action.args)
  result?.run?.()
  return true
}

/** The entries, grouped in order, so the bar can draw dividers. */
export function tableGroups(
  actions: readonly TableAction[] = TABLE_ACTIONS,
): TableAction[][] {
  const groups: TableAction[][] = []
  let current: TableAction['group'] | null = null
  for (const action of actions) {
    if (action.group !== current) {
      groups.push([])
      current = action.group
    }
    groups[groups.length - 1]!.push(action)
  }
  return groups
}

/** A rectangle in viewport coordinates. */
export interface Rect {
  left: number
  top: number
  right: number
  bottom: number
}

/** Where the bar should be drawn, in viewport coordinates. */
export interface TablePlacement {
  left: number
  top: number
}

/** The width the bar is laid out at, used to keep it on screen. */
export const TABLE_TOOLBAR_WIDTH = 384
/** The bar's height plus the gap it keeps from the table. */
export const TABLE_TOOLBAR_OFFSET = 44

/**
 * Places the bar over the table's top-left corner, kept inside the viewport.
 *
 * Left-aligned rather than centred, unlike the selection bar: a table can be
 * as wide as the page, and a bar centred on it would sit a long way from the
 * cell somebody is actually working in. It flips underneath the table's top
 * edge when the table starts too near the top of the window to fit above.
 */
export function tableToolbarPlacement(
  rect: Rect,
  viewport: { width: number; height: number },
  margin = 8,
): TablePlacement {
  const left = Math.min(
    Math.max(margin, rect.left),
    Math.max(margin, viewport.width - TABLE_TOOLBAR_WIDTH - margin),
  )
  const above = rect.top - TABLE_TOOLBAR_OFFSET
  if (above >= margin) return { left, top: above }
  return { left, top: Math.min(rect.top + 8, viewport.height - TABLE_TOOLBAR_OFFSET) }
}
