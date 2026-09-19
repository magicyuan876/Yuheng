// The handle that appears beside the block under the pointer.
//
// The handle does four things: it says which block it is about, it starts a
// drag of that block, it selects it on a click, and it opens the block menu
// — the convert / duplicate / delete / copy-reference menu that is the other
// half of a drag handle in every block editor people know. The "+" button
// above it inserts an empty block after the current one, the Feishu habit.
// Where a block ends up, and whether it may go there at all, is ProseMirror's
// own drop handling and blockMove.ts — neither is re-implemented here.
//
// The keyboard shortcut is in the same extension deliberately. A drag handle
// that can only be dragged is not operable from a keyboard, which is one of
// this work package's acceptance criteria, so Mod-Shift-Up/Down move the
// block through the same function the handle drags with. The menu is the
// same: it is a row of real buttons, so Tab reaches it and Enter and Space
// activate an entry without any of that being written here.
import { Extension } from '@tiptap/core'
import { NodeSelection, Plugin, PluginKey, TextSelection } from '@tiptap/pm/state'
import type { EditorView } from '@tiptap/pm/view'

import { blockMenuItems, runBlockAction } from './blockMenu'
import { blockAt, canMove, moveBlock } from './blockMove'

export interface DragHandleOptions {
  /** How far left of the text the handle sits. */
  offset: number
  /** The accessible name for the handle, already translated. */
  label: string
  /** The accessible name for the "+" button, already translated. */
  addLabel: string
  /** The accessible name for the block menu, already translated. */
  menuLabel: string
  /** Translates an entry's label key; the menu is built lazily, so the
   * language current when it opens is the language it shows. */
  translate: (key: string) => string
  /**
   * Copies a reference to the current block. Passed in rather than built
   * here because it needs the page's id and the clipboard; absent means the
   * menu does not offer the entry, which is right on an unsaved page.
   */
  copyBlockRef?: () => void
}

const handleKey = new PluginKey('yuhengDragHandle')

/**
 * The strip the two buttons live in.
 *
 * One wrapper rather than two separately positioned buttons, for two
 * reasons. It is what centres the pair on the block as a unit, the Feishu
 * way, instead of the "+" riding a fixed 24px above a handle pinned to the
 * block's first line. And it makes the hover target continuous: moving the
 * pointer from the "+" onto the handle never leaves the strip, so the strip
 * cannot vanish mid-gesture the way two buttons with a gap between them
 * could.
 */
function createTools(): HTMLElement {
  const el = document.createElement('div')
  el.className = 'docs-drag-tools'
  return el
}

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

/** The "+" button above the handle: a new empty block after this one. */
function createAddButton(label: string): HTMLButtonElement {
  const el = document.createElement('button')
  el.type = 'button'
  el.className = 'docs-drag-plus'
  el.setAttribute('aria-label', label)
  el.tabIndex = -1
  el.innerHTML = '<span aria-hidden="true">+</span>'
  return el
}

/** One row of the block menu, matching the slash menu's own styling hooks. */
function renderMenu(
  menu: HTMLElement,
  items: ReturnType<typeof blockMenuItems>,
  active: number,
  translate: (key: string) => string,
): void {
  menu.textContent = ''
  let section: 'convert' | 'action' | null = null
  items.forEach((item, index) => {
    if (item.section !== section) {
      section = item.section
      const heading = document.createElement('p')
      heading.className = 'docs-block-menu-section'
      heading.textContent = translate(section === 'convert'
        ? 'docs.blockMenu.convertTo'
        : 'docs.blockMenu.actions')
      menu.appendChild(heading)
    }
    const row = document.createElement('button')
    row.type = 'button'
    row.className = 'docs-block-menu-item'
    row.setAttribute('role', 'menuitem')
    if (index === active) row.classList.add('is-active')
    row.dataset.index = String(index)
    const title = document.createElement('span')
    title.className = 'docs-block-menu-title'
    title.textContent = translate(item.labelKey)
    row.appendChild(title)
    menu.appendChild(row)
  })
}

function dragHandlePlugin(options: DragHandleOptions): Plugin {
  let tools: HTMLElement | null = null
  let handle: HTMLButtonElement | null = null
  let plus: HTMLButtonElement | null = null
  let menu: HTMLElement | null = null
  let blockPos: number | null = null
  // The menu's own keyboard state. Items are rebuilt when it opens, so the
  // active row is just an index into that list.
  let menuOpen = false
  let menuIndex = 0
  let menuItems: ReturnType<typeof blockMenuItems> = []

  const closeMenu = () => {
    if (!menu || !menuOpen) return
    menuOpen = false
    menu.style.visibility = 'hidden'
    menu.setAttribute('aria-hidden', 'true')
  }

  const hide = () => {
    closeMenu()
    if (tools) tools.style.visibility = 'hidden'
    blockPos = null
  }

  /** Puts the strip beside the block at the given coordinates. */
  const place = (view: EditorView, event: MouseEvent) => {
    if (!tools || !view.editable) return
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

    // The pointer moved on to a different block: the menu is about the one
    // the strip is beside, so it does not follow the pointer.
    if (menuOpen && blockPos !== null && block.pos !== blockPos) closeMenu()

    const box = dom.getBoundingClientRect()
    const editorBox = view.dom.getBoundingClientRect()
    blockPos = block.pos
    tools.style.visibility = 'visible'
    tools.style.left = `${box.left - editorBox.left - options.offset}px`
    // Centred on the block, not pinned to its first line: the CSS translates
    // the strip up by half of itself, so the pair straddles the block's
    // middle the way Feishu's does.
    tools.style.top = `${box.top - editorBox.top + box.height / 2}px`
  }

  /** Opens the menu beside the handle, flipped at the viewport's edges. */
  const openMenu = (view: EditorView) => {
    if (!menu || !handle || blockPos === null) return
    menuItems = blockMenuItems({ copyBlockRef: options.copyBlockRef })
    menuIndex = 0
    renderMenu(menu, menuItems, menuIndex, options.translate)
    menu.style.visibility = 'visible'
    menu.setAttribute('aria-hidden', 'false')
    menuOpen = true

    const handleBox = handle.getBoundingClientRect()
    const menuWidth = menu.offsetWidth || 220
    const menuHeight = menu.offsetHeight || 320
    // Right of the handle by default; to its left when there is no room,
    // which is the narrow-viewport case the flip exists for.
    let left = handleBox.right + 4
    if (left + menuWidth > window.innerWidth - 8) {
      left = Math.max(8, handleBox.left - menuWidth - 4)
    }
    const top = Math.min(handleBox.top, Math.max(8, window.innerHeight - menuHeight - 8))
    menu.style.left = `${left}px`
    menu.style.top = `${top}px`
    menu.focus()
  }

  return new Plugin({
    key: handleKey,

    view: (view) => {
      tools = createTools()
      handle = createHandle(options.label)
      plus = createAddButton(options.addLabel)
      tools.appendChild(plus)
      tools.appendChild(handle)
      menu = document.createElement('div')
      menu.className = 'docs-block-menu'
      menu.tabIndex = -1
      menu.setAttribute('role', 'menu')
      menu.setAttribute('aria-label', options.menuLabel)
      menu.setAttribute('aria-hidden', 'true')
      menu.style.visibility = 'hidden'
      hide()
      // Positioned against the editor, so it scrolls with the text rather
      // than floating over the page at a stale offset. The menu is fixed,
      // like the slash menu: it belongs to the viewport, not the document.
      const host = view.dom.parentElement ?? view.dom
      if (getComputedStyle(host).position === 'static') host.style.position = 'relative'
      host.appendChild(tools)
      document.body.appendChild(menu)

      const select = () => {
        if (blockPos === null) return
        const tr = view.state.tr.setSelection(NodeSelection.create(view.state.doc, blockPos))
        view.dispatch(tr)
        view.focus()
      }

      handle.addEventListener('click', (event) => {
        event.preventDefault()
        select()
        if (menuOpen) closeMenu()
        else openMenu(view)
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

      plus.addEventListener('click', (event) => {
        event.preventDefault()
        closeMenu()
        if (blockPos === null) return
        const block = blockAt(view.state, blockPos + 1)
        const paragraph = view.state.schema.nodes.paragraph
        if (!block || !paragraph) return
        const tr = view.state.tr.insert(block.end, paragraph.createAndFill()!)
        const $at = tr.doc.resolve(Math.min(block.end + 1, tr.doc.content.size))
        tr.setSelection(TextSelection.near($at))
        tr.scrollIntoView()
        view.dispatch(tr)
        view.focus()
      })

      menu.addEventListener('mousedown', (event) => event.preventDefault())
      menu.addEventListener('click', (event) => {
        const row = (event.target as HTMLElement).closest<HTMLElement>('.docs-block-menu-item')
        if (!row) return
        event.preventDefault()
        runEntry(view, Number(row.dataset.index))
      })
      // The same keys the slash menu owns: arrows move, Enter runs, Escape
      // hands the document back its focus.
      menu.addEventListener('keydown', (event) => {
        if (!menuOpen) return
        const move = (delta: number) => {
          menuIndex = ((menuIndex + delta) % menuItems.length + menuItems.length) % menuItems.length
          renderMenu(menu!, menuItems, menuIndex, options.translate)
          event.preventDefault()
        }
        switch (event.key) {
          case 'ArrowDown': return move(1)
          case 'ArrowUp': return move(-1)
          case 'Enter':
          case ' ':
            event.preventDefault()
            runEntry(view, menuIndex)
            return
          case 'Escape':
            event.preventDefault()
            closeMenu()
            view.focus()
            return
          default:
        }
      })

      const runEntry = (view: EditorView, index: number) => {
        const item = menuItems[index]
        if (!item || blockPos === null) return
        closeMenu()
        if (item.id === 'copyBlockRef') {
          // The block reference names the block the handle is beside, so the
          // selection has to be on it before the caller reads it.
          select()
          options.copyBlockRef?.()
          return
        }
        runBlockAction(view.state, (tr) => view.dispatch(tr), item.id, blockPos)
        view.focus()
      }

      return {
        destroy: () => {
          tools?.remove()
          menu?.remove()
          tools = null
          handle = null
          plus = null
          menu = null
          blockPos = null
          menuOpen = false
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
          // Not when the pointer moved onto the strip or the menu: the strip
          // is a single continuous target, so moving from the "+" onto the
          // handle never fires this with a target outside it, and the strip
          // stays put mid-gesture instead of vanishing between the buttons.
          const to = (event as MouseEvent).relatedTarget
          if (to instanceof HTMLElement && (
            to.closest('.docs-drag-tools') || to.closest('.docs-block-menu')
          )) return false
          hide()
          return false
        },
        mousedown: () => {
          // A click into the text is a new intention; the menu belongs to
          // the block the handle was beside.
          closeMenu()
          return false
        },
      },
    },
  })
}

/** The extension: the handle, the menu, and the shortcuts that do the same. */
export const DragHandle = Extension.create<DragHandleOptions>({
  name: 'yuhengDragHandle',

  addOptions() {
    return {
      offset: 28,
      label: 'Move block',
      addLabel: 'Add block',
      menuLabel: 'Block actions',
      translate: (key) => key,
    }
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
