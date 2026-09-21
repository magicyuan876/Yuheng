// A block shown here but written somewhere else.
//
// The node stores a page id and a block id and nothing else — no copy of the
// text. That is the whole point: a definition quoted on six pages is written
// once and corrected once, and a reader who may not open the source page sees
// nothing rather than a stale copy of it.
//
// It follows that the node is an atom with no content of its own. What is
// drawn comes from the server, through blockRefCache.ts, and is never part of
// this document, so it cannot be edited here, cannot be saved here, and
// cannot go out of step with the page it belongs to.
//
// No Vue here, for the same reason as the other node files: the schema must
// stay loadable in a plain Node test.
import { mergeAttributes, Node } from "@tiptap/core";

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    docsTransclusion: {
      /** Replaces the range with a reference to a block of another page. */
      insertTransclusion: (
        sourcePageId: string,
        sourceBlockId: string,
        range?: { from: number; to: number },
      ) => ReturnType;
    };
  }
}

/** Reads one of the two address attributes out of HTML. */
function addressAttr(attribute: string, name: string) {
  return {
    default: null,
    parseHTML: (el: HTMLElement) => el.getAttribute(attribute),
    renderHTML: (attrs: Record<string, unknown>) => (attrs[name] ? { [attribute]: String(attrs[name]) } : {}),
  };
}

export const Transclusion = Node.create({
  name: "transclusion",
  group: "block",
  atom: true,
  selectable: true,
  // Nothing may be typed into it, and a cursor must not land inside: the
  // content belongs to another page and is not this document's to change.
  draggable: true,

  addAttributes() {
    return {
      sourcePageId: addressAttr("data-source-page-id", "sourcePageId"),
      sourceBlockId: addressAttr("data-source-block-id", "sourceBlockId"),
    };
  },

  parseHTML() {
    return [{ tag: "div[data-source-block-id]" }];
  },

  // The exported form carries the address and no text, exactly as the stored
  // form does. Whoever renders it resolves the block then, under their own
  // permissions rather than those of whoever wrote the reference.
  renderHTML({ HTMLAttributes }) {
    return ["div", mergeAttributes(HTMLAttributes, { class: "transclusion" })];
  },

  addCommands() {
    return {
      insertTransclusion:
        (sourcePageId, sourceBlockId, range) =>
        ({ commands }) => {
          if (!sourcePageId || !sourceBlockId) return false;
          const content = { type: this.name, attrs: { sourcePageId, sourceBlockId } };
          return range ? commands.insertContentAt(range, content) : commands.insertContent(content);
        },
    };
  },
});
