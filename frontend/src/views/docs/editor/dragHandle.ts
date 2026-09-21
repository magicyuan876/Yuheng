// The "+" that appears beside the block under the pointer.
//
// One control rather than the pair this file used to draw. It carries three
// gestures, which is what lets a single button replace the "+"-and-handle
// strip without losing anything:
//
//   click        insert a block below and open the slash menu on it
//   drag         move this block, the same drag the old handle started
//   right-click  the block menu: turn into / duplicate / delete / copy ref
//
// The click is deliberately not "insert an empty paragraph". It inserts the
// paragraph and then types "/" into it, so what opens is the slash menu
// itself — the same menu, the same list, the same filtering — rather than a
// second, parallel insert menu that would drift out of step with it.
//
// Where a block ends up, and whether it may go there at all, is ProseMirror's
// own drop handling and blockMove.ts — neither is re-implemented here.
//
// The keyboard shortcut is in the same extension deliberately. A drag handle
// that can only be dragged is not operable from a keyboard, which is one of
// this work package's acceptance criteria, so Mod-Shift-Up/Down move the
// block through the same function the handle drags with. The menu is the
// same: it is a row of real buttons, so Tab reaches it and Enter and Space
// activate an entry without any of that being written here.
import { Extension } from "@tiptap/core";
import { NodeSelection, Plugin, PluginKey, TextSelection } from "@tiptap/pm/state";
import type { EditorView } from "@tiptap/pm/view";

import { blockMenuItems, runBlockAction } from "./blockMenu";
import { blockAt, canMove, moveBlock } from "./blockMove";

export interface DragHandleOptions {
  /** How far left of the text the button sits. */
  offset: number;
  /** The accessible name for the button's drag gesture, already translated. */
  label: string;
  /** The accessible name for the button itself, already translated. */
  addLabel: string;
  /** The accessible name for the block menu, already translated. */
  menuLabel: string;
  /** Translates an entry's label key; the menu is built lazily, so the
   * language current when it opens is the language it shows. */
  translate: (key: string) => string;
  /**
   * Copies a reference to the current block. Passed in rather than built
   * here because it needs the page's id and the clipboard; absent means the
   * menu does not offer the entry, which is right on an unsaved page.
   */
  copyBlockRef?: () => void;
}

const handleKey = new PluginKey("yuhengDragHandle");

/**
 * The strip the button lives in.
 *
 * A wrapper around a single button looks redundant, and is not: it is what
 * the hover bridge (see the CSS) hangs off, and what keeps the positioning
 * arithmetic in one place if a second control is ever added back.
 */
function createTools(): HTMLElement {
  const el = document.createElement("div");
  el.className = "docs-drag-tools";
  return el;
}

/**
 * Builds the button.
 *
 * A real button rather than a styled div so it is focusable, announced, and
 * activated by Enter and Space without any of that being written here. It is
 * `draggable` because the same element is also the block's drag handle: one
 * control, three gestures.
 */
function createAddButton(label: string, dragLabel: string): HTMLButtonElement {
  const el = document.createElement("button");
  el.type = "button";
  el.className = "docs-drag-plus";
  el.draggable = true;
  el.setAttribute("aria-label", label);
  el.title = `${label}
${dragLabel}`;
  el.tabIndex = -1;
  el.innerHTML = '<span aria-hidden="true">+</span>';
  return el;
}

/** One row of the block menu, matching the slash menu's own styling hooks. */
function renderMenu(
  menu: HTMLElement,
  items: ReturnType<typeof blockMenuItems>,
  active: number,
  translate: (key: string) => string,
): void {
  menu.textContent = "";
  let section: "convert" | "action" | null = null;
  items.forEach((item, index) => {
    if (item.section !== section) {
      section = item.section;
      const heading = document.createElement("p");
      heading.className = "docs-block-menu-section";
      heading.textContent = translate(section === "convert" ? "docs.blockMenu.convertTo" : "docs.blockMenu.actions");
      menu.appendChild(heading);
    }
    const row = document.createElement("button");
    row.type = "button";
    row.className = "docs-block-menu-item";
    row.setAttribute("role", "menuitem");
    if (index === active) row.classList.add("is-active");
    row.dataset.index = String(index);
    const title = document.createElement("span");
    title.className = "docs-block-menu-title";
    title.textContent = translate(item.labelKey);
    row.appendChild(title);
    menu.appendChild(row);
  });
}

function dragHandlePlugin(options: DragHandleOptions): Plugin {
  let tools: HTMLElement | null = null;
  let plus: HTMLButtonElement | null = null;
  let menu: HTMLElement | null = null;
  let blockPos: number | null = null;
  // The menu's own keyboard state. Items are rebuilt when it opens, so the
  // active row is just an index into that list.
  let menuOpen = false;
  let menuIndex = 0;
  let menuItems: ReturnType<typeof blockMenuItems> = [];

  const closeMenu = () => {
    if (!menu || !menuOpen) return;
    menuOpen = false;
    menu.style.visibility = "hidden";
    menu.setAttribute("aria-hidden", "true");
  };

  const hide = () => {
    closeMenu();
    if (tools) tools.style.visibility = "hidden";
    blockPos = null;
  };

  /**
   * Hiding is deferred, and any pointer activity cancels it.
   *
   * Between the text and the strip there is a gutter that belongs to neither,
   * and crossing it fires mouseleave on the editor with a relatedTarget that
   * is not the strip. Hiding on that event is correct for somebody leaving
   * the document and wrong for the far commoner case of somebody reaching for
   * the button, and the two are indistinguishable at the moment the event
   * arrives. Waiting a moment tells them apart: a pointer heading for the
   * strip arrives well inside the delay and cancels it.
   */
  let hideTimer: ReturnType<typeof setTimeout> | null = null;

  const keepAlive = () => {
    if (hideTimer === null) return;
    clearTimeout(hideTimer);
    hideTimer = null;
  };

  const hideSoon = () => {
    keepAlive();
    hideTimer = setTimeout(() => {
      hideTimer = null;
      // A menu somebody opened is not a hover affordance any more; it stays
      // until it is dismissed.
      if (!menuOpen) hide();
    }, 220);
  };

  /**
   * The vertical centre of a block's *first line*, relative to its own box.
   *
   * Centring the strip on the whole block is only right for a one-line
   * paragraph; on a three-line one, or on a list, it leaves the button
   * floating beside the middle of the text with nothing to point at. The
   * first line is the one it belongs beside, so that is what is measured
   * here: the first client rect of the block's own text, falling back to the
   * line height and finally to the box when a block has neither (an image,
   * say).
   */
  const firstLineCentre = (dom: HTMLElement, box: DOMRect): number => {
    const range = document.createRange();
    try {
      range.selectNodeContents(dom);
      const first = range.getClientRects()[0];
      if (first && first.height > 0 && first.height <= box.height) {
        return first.top - box.top + first.height / 2;
      }
    } catch {
      // A node with nothing selectable inside it; the fallbacks below apply.
    } finally {
      range.detach?.();
    }
    const style = getComputedStyle(dom);
    const line = Number.parseFloat(style.lineHeight);
    if (Number.isFinite(line) && line > 0 && line <= box.height) {
      return Number.parseFloat(style.paddingTop || "0") + line / 2;
    }
    return box.height / 2;
  };

  /** Puts the strip beside the block at the given coordinates. */
  const place = (view: EditorView, event: MouseEvent) => {
    if (!tools || !view.editable) return;
    const found = view.posAtCoords({ left: event.clientX, top: event.clientY });
    // No position under the pointer means it is over the editor's own padding
    // — the gutter the strip itself sits in. Leaving the strip where it is is
    // the whole point: hiding here is what made it vanish as soon as somebody
    // moved towards the buttons they were aiming for.
    if (!found) return;
    keepAlive();

    const block = blockAt(view.state, found.inside >= 0 ? found.inside + 1 : found.pos);
    if (!block) return;

    let dom: HTMLElement | null = null;
    try {
      dom = view.nodeDOM(block.pos) as HTMLElement | null;
    } catch {
      dom = null;
    }
    if (!dom || !(dom instanceof HTMLElement)) return;

    // The pointer moved on to a different block: the menu is about the one
    // the strip is beside, so it does not follow the pointer.
    if (menuOpen && blockPos !== null && block.pos !== blockPos) closeMenu();

    const box = dom.getBoundingClientRect();
    // Against the element the strip is a child of, not against the editor's
    // content box. The two differ whenever the host carries a border or
    // padding of its own, and that difference was the offset by which the
    // strip sat wrong.
    const hostBox = (tools.offsetParent ?? view.dom).getBoundingClientRect();
    blockPos = block.pos;
    tools.style.visibility = "visible";
    tools.style.left = `${box.left - hostBox.left - options.offset}px`;
    // On the block's first line, not its middle: the CSS lifts the strip by
    // half of itself, so this is the line's centre.
    tools.style.top = `${box.top - hostBox.top + firstLineCentre(dom, box)}px`;
  };

  /** Opens the menu beside the button, flipped at the viewport's edges. */
  const openMenu = (view: EditorView) => {
    if (!menu || !plus || blockPos === null) return;
    menuItems = blockMenuItems({ copyBlockRef: options.copyBlockRef });
    menuIndex = 0;
    renderMenu(menu, menuItems, menuIndex, options.translate);
    menu.style.visibility = "visible";
    menu.setAttribute("aria-hidden", "false");
    menuOpen = true;

    const handleBox = plus.getBoundingClientRect();
    const menuWidth = menu.offsetWidth || 220;
    const menuHeight = menu.offsetHeight || 320;
    // Right of the button by default; to its left when there is no room,
    // which is the narrow-viewport case the flip exists for.
    let left = handleBox.right + 4;
    if (left + menuWidth > window.innerWidth - 8) {
      left = Math.max(8, handleBox.left - menuWidth - 4);
    }
    const top = Math.min(handleBox.top, Math.max(8, window.innerHeight - menuHeight - 8));
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
    menu.focus();
  };

  return new Plugin({
    key: handleKey,

    view: (view) => {
      tools = createTools();
      plus = createAddButton(options.addLabel, options.label);
      tools.appendChild(plus);
      menu = document.createElement("div");
      menu.className = "docs-block-menu";
      menu.tabIndex = -1;
      menu.setAttribute("role", "menu");
      menu.setAttribute("aria-label", options.menuLabel);
      menu.setAttribute("aria-hidden", "true");
      menu.style.visibility = "hidden";
      hide();
      // Positioned against the editor, so it scrolls with the text rather
      // than floating over the page at a stale offset. The menu is fixed,
      // like the slash menu: it belongs to the viewport, not the document.
      const host = view.dom.parentElement ?? view.dom;
      if (getComputedStyle(host).position === "static") host.style.position = "relative";
      host.appendChild(tools);
      document.body.appendChild(menu);

      // The strip and the menu are outside view.dom, so the editor's own
      // mouseleave is what fires when the pointer reaches them. These cancel
      // the pending hide, and re-arm it when the pointer leaves for good.
      for (const el of [tools, menu]) {
        el.addEventListener("mouseenter", keepAlive);
        el.addEventListener("mouseleave", hideSoon);
      }

      const select = () => {
        if (blockPos === null) return;
        const tr = view.state.tr.setSelection(NodeSelection.create(view.state.doc, blockPos));
        view.dispatch(tr);
        view.focus();
      };

      /**
       * Inserts a block below and opens the slash menu on it.
       *
       * The "/" is really typed into the document rather than the menu being
       * summoned directly, because the menu is driven by the text before the
       * cursor (see suggestion.ts). Going through the text is what makes this
       * button and the key indistinguishable: the same trigger, the same
       * query as more is typed, and the same cleanup — whichever entry is
       * chosen deletes the range the "/" occupies, exactly as when typed.
       *
       * An empty paragraph is reused rather than followed by a second one.
       * Clicking "+" beside a blank line and getting two blank lines, the
       * caret in the lower one, is not what anybody means by it.
       */
      const insertWithSlashMenu = (view: EditorView) => {
        if (blockPos === null) return;
        const paragraph = view.state.schema.nodes.paragraph;
        const block = blockAt(view.state, blockPos + 1);
        if (!paragraph || !block) return;

        const here = view.state.doc.nodeAt(blockPos);
        const reuse = here?.type === paragraph && here.content.size === 0;
        const tr = view.state.tr;
        let caret: number;
        if (reuse) {
          caret = blockPos + 1;
        } else {
          tr.insert(block.end, paragraph.createAndFill()!);
          caret = block.end + 1;
        }
        tr.insertText("/", caret);
        tr.setSelection(TextSelection.near(tr.doc.resolve(caret + 1)));
        tr.scrollIntoView();
        view.dispatch(tr);
        view.focus();
      };

      plus.addEventListener("click", (event) => {
        event.preventDefault();
        closeMenu();
        insertWithSlashMenu(view);
      });

      // The block menu — turn into / duplicate / delete / copy reference —
      // now hangs off the same button's context menu, since there is no
      // second button left to carry it.
      plus.addEventListener("contextmenu", (event) => {
        event.preventDefault();
        select();
        if (menuOpen) closeMenu();
        else openMenu(view);
      });

      plus.addEventListener("dragstart", (event) => {
        if (blockPos === null || !event.dataTransfer) return;
        // Selecting first is what makes this a move of the block rather than
        // of whatever happened to be selected before.
        select();
        const slice = view.state.selection.content();
        view.dragging = { slice, move: true };
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/html", "");
        try {
          const dom = view.nodeDOM(blockPos);
          if (dom instanceof HTMLElement) event.dataTransfer.setDragImage(dom, 0, 0);
        } catch {
          // Without a drag image the browser draws the button itself, which
          // is ugly but harmless.
        }
      });

      plus.addEventListener("dragend", () => {
        view.dragging = null;
      });

      menu.addEventListener("mousedown", (event) => event.preventDefault());
      menu.addEventListener("click", (event) => {
        const row = (event.target as HTMLElement).closest<HTMLElement>(".docs-block-menu-item");
        if (!row) return;
        event.preventDefault();
        runEntry(view, Number(row.dataset.index));
      });
      // The same keys the slash menu owns: arrows move, Enter runs, Escape
      // hands the document back its focus.
      menu.addEventListener("keydown", (event) => {
        if (!menuOpen) return;
        const move = (delta: number) => {
          menuIndex = (((menuIndex + delta) % menuItems.length) + menuItems.length) % menuItems.length;
          renderMenu(menu!, menuItems, menuIndex, options.translate);
          event.preventDefault();
        };
        switch (event.key) {
          case "ArrowDown":
            return move(1);
          case "ArrowUp":
            return move(-1);
          case "Enter":
          case " ":
            event.preventDefault();
            runEntry(view, menuIndex);
            return;
          case "Escape":
            event.preventDefault();
            closeMenu();
            view.focus();
            return;
          default:
        }
      });

      const runEntry = (view: EditorView, index: number) => {
        const item = menuItems[index];
        if (!item || blockPos === null) return;
        closeMenu();
        if (item.id === "copyBlockRef") {
          // The block reference names the block the button is beside, so the
          // selection has to be on it before the caller reads it.
          select();
          options.copyBlockRef?.();
          return;
        }
        runBlockAction(view.state, (tr) => view.dispatch(tr), item.id, blockPos);
        view.focus();
      };

      return {
        destroy: () => {
          keepAlive();
          tools?.remove();
          menu?.remove();
          tools = null;
          plus = null;
          menu = null;
          blockPos = null;
          menuOpen = false;
        },
      };
    },

    props: {
      handleDOMEvents: {
        mousemove: (view, event) => {
          place(view, event as MouseEvent);
          return false;
        },
        mouseleave: (_view, event) => {
          // Straight onto the strip or the menu: nothing to do.
          const to = (event as MouseEvent).relatedTarget;
          if (to instanceof HTMLElement && (to.closest(".docs-drag-tools") || to.closest(".docs-block-menu")))
            return false;
          // Otherwise the pointer may still be crossing the gutter towards
          // the strip, so the strip is given a moment to be reached.
          hideSoon();
          return false;
        },
        mousedown: () => {
          // A click into the text is a new intention; the menu belongs to
          // the block the button was beside.
          closeMenu();
          return false;
        },
      },
    },
  });
}

/** The extension: the button, the menu, and the shortcuts that do the same. */
export const DragHandle = Extension.create<DragHandleOptions>({
  name: "yuhengDragHandle",

  addOptions() {
    return {
      offset: 32,
      label: "Move block",
      addLabel: "Add block",
      menuLabel: "Block actions",
      translate: (key) => key,
    };
  },

  addProseMirrorPlugins() {
    return [dragHandlePlugin(this.options)];
  },

  addKeyboardShortcuts() {
    const move = (direction: -1 | 1) => () => {
      const { state, view } = this.editor;
      const pos = state.selection.from;
      if (!canMove(state, pos, direction)) return false;
      const tr = moveBlock(state, pos, direction);
      if (!tr) return false;
      view.dispatch(tr);
      return true;
    };
    return {
      "Mod-Shift-ArrowUp": move(-1),
      "Mod-Shift-ArrowDown": move(1),
      // Alt is what several editors use for the same thing; both are cheap.
      "Alt-Shift-ArrowUp": move(-1),
      "Alt-Shift-ArrowDown": move(1),
    };
  },
});
