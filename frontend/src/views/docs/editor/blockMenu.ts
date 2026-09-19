// The menu behind the drag handle.
//
// Two things are separated on purpose, the same split as commands.ts: the
// list of entries is data (an id and an i18n key — the convert-to entries
// reuse the slash menu's own labels, because turning a paragraph into a
// heading is the same act whether it started from "/" or from the handle),
// and running an entry is a function over an editor state. The function is
// built on ProseMirror's own commands and kept free of Vue and the DOM, so
// it runs under test against a real document with no browser involved.
import { setBlockType, wrapIn } from '@tiptap/pm/commands'
import type { EditorState, Transaction } from '@tiptap/pm/state'
import { NodeSelection, TextSelection } from '@tiptap/pm/state'
import { wrapInList } from '@tiptap/pm/schema-list'

import { newBlockId } from './blockAttrs'
import { blockAt } from './blockMove'

/** One entry in the block menu. */
export interface BlockMenuItem {
  id: string
  /** The i18n key for the label; already translated by the view. */
  labelKey: string
  /** A tdesign icon name. */
  icon: string
  /** 'convert' entries turn the block into something else; 'action' entries
   * do something to it while leaving its type alone. */
  section: 'convert' | 'action'
}

/**
 * The menu's contents.
 *
 * The convert section deliberately mirrors the slash menu's basic group:
 * those are the block shapes somebody standing at a handle reaches for, and
 * reusing the same label keys keeps the two menus from drifting apart in
 * wording.
 */
export function blockMenuItems(opts: { copyBlockRef?: () => void } = {}): BlockMenuItem[] {
  const convert: BlockMenuItem[] = [
    { id: 'paragraph', labelKey: 'docs.commands.paragraph', icon: 'text', section: 'convert' },
    { id: 'heading1', labelKey: 'docs.commands.heading1', icon: 'textformat-bold', section: 'convert' },
    { id: 'heading2', labelKey: 'docs.commands.heading2', icon: 'textformat-bold', section: 'convert' },
    { id: 'heading3', labelKey: 'docs.commands.heading3', icon: 'textformat-bold', section: 'convert' },
    { id: 'bulletList', labelKey: 'docs.commands.bulletList', icon: 'order-list', section: 'convert' },
    { id: 'orderedList', labelKey: 'docs.commands.orderedList', icon: 'order-descending', section: 'convert' },
    { id: 'taskList', labelKey: 'docs.commands.taskList', icon: 'check-rectangle', section: 'convert' },
    { id: 'blockquote', labelKey: 'docs.commands.blockquote', icon: 'quote', section: 'convert' },
    { id: 'codeBlock', labelKey: 'docs.commands.codeBlock', icon: 'code', section: 'convert' },
    { id: 'callout', labelKey: 'docs.commands.callout', icon: 'info-circle', section: 'convert' },
  ]
  const action: BlockMenuItem[] = [
    { id: 'duplicate', labelKey: 'docs.blockMenu.duplicate', icon: 'copy', section: 'action' },
    { id: 'delete', labelKey: 'docs.blockMenu.delete', icon: 'delete', section: 'action' },
  ]
  if (opts.copyBlockRef) {
    action.push({
      id: 'copyBlockRef', labelKey: 'docs.commands.copyBlockRef', icon: 'quote', section: 'action',
    })
  }
  return [...convert, ...action]
}

/** Fresh ids for a deep copy, so the duplicate never shares a block id. */
function regenerateIds(json: unknown): void {
  if (Array.isArray(json)) {
    for (const entry of json) regenerateIds(entry)
    return
  }
  if (json && typeof json === 'object') {
    const record = json as Record<string, unknown>
    if (typeof record.type === 'string' && record.attrs && typeof record.attrs === 'object') {
      ;(record.attrs as Record<string, unknown>).id = newBlockId()
    }
    regenerateIds(record.content)
  }
}

/**
 * Runs one menu entry against the block that starts at `pos`.
 *
 * Convert selects the block as a node first, so ProseMirror's own commands
 * act on it exactly as they would on a selection the person made by hand.
 * Duplicate deep-copies the node's JSON with fresh block ids and puts the
 * copy after the original. Delete removes the range, leaving an empty
 * paragraph behind rather than an invalid empty document when the block was
 * the only one. Copy block reference is wired by the caller, which owns the
 * clipboard and the page's id.
 */
export function runBlockAction(
  state: EditorState,
  dispatch: (tr: Transaction) => void,
  id: string,
  pos: number,
): boolean {
  const block = blockAt(state, pos + 1)
  if (!block) return false

  if (id === 'duplicate') {
    // A real deep copy: Node.toJSON hands back its attrs object by
    // reference, so regenerating ids straight on it would rewrite the
    // original block in place.
    const json = JSON.parse(JSON.stringify(block.node.toJSON())) as unknown
    regenerateIds(json)
    const copy = state.schema.nodeFromJSON(json)
    if (!copy) return false
    dispatch(state.tr.insert(block.end, copy).scrollIntoView())
    return true
  }

  if (id === 'delete') {
    const tr = state.tr
    const only = block.pos <= 0 && block.end >= state.doc.content.size
    const paragraph = state.schema.nodes.paragraph
    if (only && paragraph) {
      tr.replaceWith(block.pos, block.end, paragraph.createAndFill()!)
    } else {
      tr.delete(block.pos, block.end)
    }
    // The cursor has to land somewhere; after a delete it goes where the
    // block was, inside whatever is there now.
    const $at = tr.doc.resolve(Math.min(block.pos + 1, tr.doc.content.size))
    tr.setSelection(TextSelection.near($at))
    tr.scrollIntoView()
    dispatch(tr)
    return true
  }

  // Convert: a node selection over the block, then the schema's own command.
  const selected = state.apply(state.tr.setSelection(
    NodeSelection.create(state.doc, block.pos),
  ))
  const types = selected.schema.nodes
  switch (id) {
    case 'paragraph':
      return setBlockType(types.paragraph)(selected, dispatch)
    case 'heading1':
    case 'heading2':
    case 'heading3':
      return setBlockType(types.heading, { level: Number(id.slice(-1)) })(selected, dispatch)
    case 'codeBlock':
      return setBlockType(types.codeBlock)(selected, dispatch)
    case 'bulletList':
      return wrapInList(types.bulletList)(selected, dispatch)
    case 'orderedList':
      return wrapInList(types.orderedList)(selected, dispatch)
    case 'taskList':
      return wrapInList(types.taskList)(selected, dispatch)
    case 'blockquote':
      return wrapIn(types.blockquote)(selected, dispatch)
    case 'callout':
      return wrapIn(types.callout, { kind: 'info' })(selected, dispatch)
    default:
      return false
  }
}
