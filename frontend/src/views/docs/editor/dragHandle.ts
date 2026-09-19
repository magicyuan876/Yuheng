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

  /**
   * Hiding is deferred, and any pointer activity cancels it.
   *
   * Between the text and the strip there is a gutter that belongs to neither,
   * and crossing it fires mouseleave on the editor with a relatedTarget that
   * is not the strip. Hiding on that event is correct for somebody leaving
   * the document and wrong for the far commoner case of somebody reaching for
   * the handle, and the two are indistinguishable at the moment the event
   * arrives. Waiting a moment tells them apart: a pointer heading for the
   * strip arrives well inside the delay and cancels it.
   */
  let hideTimer: ReturnType<typeof setTimeout> | null = null

  const keepAlive = () => {
    if (hideTimer === null) return
    clearTimeout(hideTimer)
    hideTimer = null
  }

  const hideSoon = () => {
    keepAlive()
    hideTimer = setTimeout(() => {
      hideTimer = null
      // A menu somebody opened is not a hover affordance any more; it stays
      // until it is dismissed.
      if (!menuOpen) hide()
    }, 220)
  }

  /**
   * The vertical centre of a block's *first line*, relative to its own box.
   *
   * Centring the strip on the whole block is only right for a one-line
   * paragraph; on a three-line one, or on a list, it leaves the handle
   * floating beside the middle of the text with nothing to point at. Feishu
   * aligns it with the first line, so that is what is measured here: the
   * first client rect of the block's own text, falling back to the line
   * height and finally to the box when a block has neither (an image, say).
   */
  const firstLineCentre = (dom: HTMLElement, box: DOMRect): number => {
    const range = document.createRange()
    try {
      range.selectNodeContents(dom)
      const first = range.getClientRects()[0]
      if (first && first.height > 0 && first.height <= box.height) {
        return first.top - box.top + first.height / 2
      }
    } catch {
      // A node with nothing selectable inside it; the fallbacks below apply.
    } finally {
      range.detach?.()
    }
    const style = getComputedStyle(dom)
    const line = Number.parseFloat(style.lineHeight)
    if (Number.isFinite(line) && line > 0 && line <= box.height) {
      return Number.parseFloat(style.paddingTop || '0') + line / 2
    }
    return box.height / 2
  }

  /** Puts the strip beside the block at the given coordinates. */
  const place = (view: EditorView, event: MouseEvent) => {
    if (!tools || !view.editable) return
    const found = view.posAtCoords({ left: event.clientX, top: event.clientY })
    // No position under the pointer means it is over the editor's own padding
    // — the gutter the strip itself sits in. Leaving the strip where it is is
    // the whole point: hiding here is what made it vanish as soon as somebody
    // moved towards the buttons they were aiming for.
    if (!found) return
    keepAlive()

    const block = blockAt(view.state, found.inside >= 0 ? found.inside + 1 : found.pos)
    if (!block) return

    let dom: HTMLElement | null = null
    try {
      dom = view.nodeDOM(block.pos) as HTMLElement | null
    } catch {
      dom = null
    }
    if (!dom || !(dom instanceof HTMLElement)) return

    // The pointer moved on to a different block: the menu is about the one
    // the strip is beside, so it does not follow the pointer.
    if (menuOpen && blockPos !== null && block.pos !== blockPos) closeMenu()

    const box = dom.getBoundingClientRect()
    // Against the element the strip is a child of, not against the editor's
    // content box. The two differ whenever the host carries a border or
    // padding of its own, and that difference was the offset by which the
    // strip sat wrong.
    const hostBox = (tools.offsetParent ?? view.dom).getBoundingClientRect()
    blockPos = block.pos
    tools.style.visibility = 'visible'
    tools.style.left = `${box.left - hostBox.left - options.offset}px`
    // On the block's first line, not its middle: the CSS lifts the strip by
    // half of itself, so this is the line's centre.
    tools.style.top = `${box.top - hostBox.top + firstLineCentre(dom, box)}px`
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

      // The strip and the menu are outside view.dom, so the editor's own
      // mouseleave is what fires when the pointer reaches them. These cancel
      // the pending hide, and re-arm it when the pointer leaves for good.
      for (const el of [tools, menu]) {
        el.addEventListener('mouseenter', keepAlive)
        el.addEventListener('mouseleave', hideSoon)
      }

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
          keepAlive()
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
          // Straight onto the strip or the menu: nothing to do.
          const to = (event as MouseEvent).relatedTarget
          if (to instanceof HTMLElement && (
            to.closest('.docs-drag-tools') || to.closest('.docs-block-menu')
          )) return false
          // Otherwise the pointer may still be crossing the gutter towards
          // the strip, so the strip is given a moment to be reached.
          hideSoon()
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
      offset: 52,
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
