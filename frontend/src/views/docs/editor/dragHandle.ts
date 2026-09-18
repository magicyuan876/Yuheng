// The handle that appears beside the block under the pointer.
//
// The handle does three things and nothing else: it says which block it is
// about, it starts a drag of that block, and it selects it on a click. Where
// a block ends up, and whether it may go there at all, is ProseMirror's own
// drop handling and blockMove.ts — neither is re-implemented here.
//
// The keyboard shortcut is in the same extension deliberately. A drag handle
// that can only be dragged is not operable from a keyboard, which is one of
// this work package's acceptance criteria, so Mod-Shift-Up/Down move the
// block through the same function the handle drags with.
import { Extension } from '@tiptap/core'
import { NodeSelection, Plugin, PluginKey } from '@tiptap/pm/state'
import type { EditorView } from '@tiptap/pm/view'

import { blockAt, canMove, moveBlock } from './blockMove'

export interface DragHandleOptions {
  /** How far left of the text the handle sits. */
  offset: number
  /** The accessible name for the handle, already translated. */
  label: string
}

const handleKey = new PluginKey('yuhengDragHandle')

/**
 * Builds the handle element.
 *
 * It is a real button rather than a styled div so it is focusable, announced,
 * and activated by Enter and Space without any of that being written here.
 */
function createHandle(label: string): HTMLButtonElement {
  const el = document.createElement('button')
  el.type = 'button'
  el.className = 'docs-drag-handle'
  el.draggable = true
  el.setAttribute('aria-label', label)
  el.tabIndex = -1
  el.innerHTML = '<span aria-hidden="true">⁙</span>'
  return el
}

function dragHandlePlugin(options: DragHandleOptions): Plugin {
  let handle: HTMLButtonElement | null = null
  let blockPos: number | null = null

  const hide = () => {
    if (handle) handle.style.visibility = 'hidden'
    blockPos = null
  }

  /** Puts the handle beside the block at the given coordinates. */
  const place = (view: EditorView, event: MouseEvent) => {
    if (!handle || !view.editable) return
    const found = view.posAtCoords({ left: event.clientX, top: event.clientY })
    if (!found) return hide()

    const block = blockAt(view.state, found.inside >= 0 ? found.inside + 1 : found.pos)
    if (!block) return hide()

    let dom: HTMLElement | null = null
    try {
      dom = view.nodeDOM(block.pos) as HTMLElement | null
    } catch {
      dom = null
    }
    if (!dom || !(dom instanceof HTMLElement)) return hide()

    const box = dom.getBoundingClientRect()
    const editorBox = view.dom.getBoundingClientRect()
    blockPos = block.pos
    handle.style.visibility = 'visible'
    handle.style.left = `${box.left - editorBox.left - options.offset}px`
    handle.style.top = `${box.top - editorBox.top}px`
  }

  return new Plugin({
    key: handleKey,

    view: (view) => {
      handle = createHandle(options.label)
      hide()
      // Positioned against the editor, so it scrolls with the text rather
      // than floating over the page at a stale offset.
      const host = view.dom.parentElement ?? view.dom
      if (getComputedStyle(host).position === 'static') host.style.position = 'relative'
      host.appendChild(handle)

      const select = () => {
        if (blockPos === null) return
        const tr = view.state.tr.setSelection(NodeSelection.create(view.state.doc, blockPos))
        view.dispatch(tr)
        view.focus()
      }

      handle.addEventListener('click', (event) => {
        event.preventDefault()
        select()
      })

      handle.addEventListener('dragstart', (event) => {
        if (blockPos === null || !event.dataTransfer) return
        // Selecting first is what makes this a move of the block rather than
        // of whatever happened to be selected before.
        select()
        const slice = view.state.selection.content()
        view.dragging = { slice, move: true }
        event.dataTransfer.effectAllowed = 'move'
        event.dataTransfer.setData('text/html', '')
        try {
          const dom = view.nodeDOM(blockPos)
          if (dom instanceof HTMLElement) event.dataTransfer.setDragImage(dom, 0, 0)
        } catch {
          // Without a drag image the browser draws the handle itself, which is
          // ugly but harmless.
        }
      })

      handle.addEventListener('dragend', () => {
        view.dragging = null
      })

      return {
        destroy: () => {
          handle?.remove()
          handle = null
          blockPos = null
        },
      }
    },

    props: {
      handleDOMEvents: {
        mousemove: (view, event) => {
          place(view, event as MouseEvent)
          return false
        },
        mouseleave: (_view, event) => {
          // Not when the pointer moved onto the handle itself, which is
          // outside the editor's DOM and would otherwise hide it instantly.
          const to = (event as MouseEvent).relatedTarget
          if (to instanceof HTMLElement && to.closest('.docs-drag-handle')) return false
          hide()
          return false
        },
      },
    },
  })
}

/** The extension: the handle, and the shortcut that does the same job. */
export const DragHandle = Extension.create<DragHandleOptions>({
  name: 'yuhengDragHandle',

  addOptions() {
    return { offset: 28, label: 'Move block' }
  },

  addProseMirrorPlugins() {
    return [dragHandlePlugin(this.options)]
  },

  addKeyboardShortcuts() {
    const move = (direction: -1 | 1) => () => {
      const { state, view } = this.editor
      const pos = state.selection.from
      if (!canMove(state, pos, direction)) return false
      const tr = moveBlock(state, pos, direction)
      if (!tr) return false
      view.dispatch(tr)
      return true
    }
    return {
      'Mod-Shift-ArrowUp': move(-1),
      'Mod-Shift-ArrowDown': move(1),
      // Alt is what several editors use for the same thing; both are cheap.
      'Alt-Shift-ArrowUp': move(-1),
      'Alt-Shift-ArrowDown': move(1),
    }
  },
})
