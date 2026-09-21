// The two document nodes that point at an uploaded file: `image` and
// `attachment`.
//
// They carry no Vue: the node views are supplied separately by the editor
// component, so this file (and therefore the schema it declares) can be loaded
// and compared against packages/docs-schema in a plain Node test with no DOM.
//
// The one rule worth stating: a node stores an attachment id, never a URL. The
// address is derived when it is rendered, so a stored document survives a
// change of storage backend, and a copied document never carries a link to
// bytes the reader may not be allowed to see.
import { mergeAttributes, Node } from "@tiptap/core";

import { attachmentSrc } from "./attachments";

/** Reads an integer attribute back from HTML, ignoring anything else. */
function intAttr(name: string) {
  return {
    default: null as number | null,
    parseHTML: (element: HTMLElement) => {
      const raw = element.getAttribute(name);
      if (!raw) return null;
      const n = Number.parseInt(raw, 10);
      return Number.isFinite(n) && n > 0 ? n : null;
    },
    renderHTML: (attrs: Record<string, unknown>) => (attrs[name] ? { [name]: String(attrs[name]) } : {}),
  };
}

/**
 * An image. Exactly one of `attachmentId` and `src` is set: the first for a
 * file uploaded here, the second for one that lives somewhere else entirely
 * (a pasted external image, which the renderer leaves alone).
 */
export const DocImage = Node.create({
  name: "image",
  group: "block",
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return {
      attachmentId: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute("data-attachment-id"),
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.attachmentId ? { "data-attachment-id": String(attrs.attachmentId) } : {},
      },
      src: {
        default: null,
        // Only an image that is not ours keeps a literal address; for one of
        // ours the address is derived at render time from the id.
        parseHTML: (el: HTMLElement) => (el.getAttribute("data-attachment-id") ? null : el.getAttribute("src")),
        renderHTML: () => ({}),
      },
      alt: { default: null },
      title: { default: null },
      align: {
        default: "center",
        parseHTML: (el: HTMLElement) => el.getAttribute("data-align") ?? "center",
        renderHTML: (attrs: Record<string, unknown>) => ({ "data-align": String(attrs.align ?? "center") }),
      },
      width: intAttr("width"),
      height: intAttr("height"),
    };
  },

  parseHTML() {
    return [{ tag: "img[src], img[data-attachment-id]" }];
  },

  renderHTML({ HTMLAttributes, node }) {
    const id = node.attrs.attachmentId as string | null;
    const src = id ? attachmentSrc(id) : (node.attrs.src as string | null);
    return ["img", mergeAttributes(HTMLAttributes, src ? { src } : {})];
  },
});

/**
 * Any other uploaded file, shown as a card with its name and size. Video,
 * audio and PDF get richer nodes in a later work package; until then they land
 * here, which changes how they look and nothing about how they are stored.
 */
export const DocAttachment = Node.create({
  name: "attachment",
  group: "block",
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return {
      attachmentId: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute("data-attachment-id"),
        renderHTML: (attrs: Record<string, unknown>) =>
          attrs.attachmentId ? { "data-attachment-id": String(attrs.attachmentId) } : {},
      },
      name: {
        default: "",
        parseHTML: (el: HTMLElement) => el.getAttribute("data-name") ?? el.textContent?.trim() ?? "",
        renderHTML: (attrs: Record<string, unknown>) => ({ "data-name": String(attrs.name ?? "") }),
      },
      mime: {
        default: null,
        parseHTML: (el: HTMLElement) => el.getAttribute("data-mime"),
        renderHTML: (attrs: Record<string, unknown>) => (attrs.mime ? { "data-mime": String(attrs.mime) } : {}),
      },
      size: {
        default: null,
        parseHTML: (el: HTMLElement) => {
          const raw = el.getAttribute("data-size");
          const n = raw ? Number.parseInt(raw, 10) : Number.NaN;
          return Number.isFinite(n) && n >= 0 ? n : null;
        },
        renderHTML: (attrs: Record<string, unknown>) => (attrs.size == null ? {} : { "data-size": String(attrs.size) }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "div[data-attachment-id].docs-attachment" }];
  },

  renderHTML({ HTMLAttributes, node }) {
    const id = node.attrs.attachmentId as string | null;
    const name = String(node.attrs.name ?? "");
    return [
      "div",
      mergeAttributes(HTMLAttributes, { class: "docs-attachment" }),
      ["a", { href: id ? attachmentSrc(id) : "#", download: name || null }, name],
    ];
  },
});
