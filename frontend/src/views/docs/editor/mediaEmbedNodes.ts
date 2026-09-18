// The nodes that show something the reader did not write: a stored video,
// audio file or PDF; an external page in a frame; and the two diagram editors.
//
// Every one of them stores a reference and nothing else. A media node stores
// an attachment id, an embed stores the address its author pasted, and a
// diagram stores the id of its source file plus the id of a rendered preview.
// What is actually displayed is derived at render time, which is what lets the
// read-only view and the export show a diagram's preview without ever loading
// an editor, and what lets a tightened embed policy take effect on the next
// page load rather than the next save.
//
// No Vue here, so the schema stays loadable in a plain Node test.
import { mergeAttributes, Node } from '@tiptap/core'

import { attachmentSrc } from './attachments'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsMedia: {
      /** Inserts a stored video. */
      insertVideo: (attachmentId: string) => ReturnType
      /** Inserts a stored audio file. */
      insertAudio: (attachmentId: string) => ReturnType
      /** Inserts a stored PDF. */
      insertPdf: (attachmentId: string, name?: string) => ReturnType
      /** Inserts an external page the server has already approved. */
      insertEmbed: (provider: string, url: string, size?: { width?: number; height?: number }) => ReturnType
      /** Inserts a draw.io diagram. */
      insertDrawio: (attachmentId: string, previewAttachmentId?: string | null) => ReturnType
      /** Inserts an Excalidraw drawing. */
      insertExcalidraw: (attachmentId: string, previewAttachmentId?: string | null) => ReturnType
    }
  }
}

/** The attachment reference every media and diagram node carries. */
function attachmentAttr(name = 'attachmentId', attribute = 'data-attachment-id') {
  return {
    default: null,
    parseHTML: (el: HTMLElement) => el.getAttribute(attribute),
    renderHTML: (attrs: Record<string, unknown>) =>
      attrs[name] ? { [attribute]: String(attrs[name]) } : {},
  }
}

/** An optional pixel dimension, bounded by the schema at 16-8192. */
function sizeAttr(name: string) {
  return {
    default: null as number | null,
    parseHTML: (el: HTMLElement) => {
      const raw = el.getAttribute(name)
      const n = raw ? Number.parseInt(raw, 10) : Number.NaN
      return Number.isFinite(n) && n > 0 ? n : null
    },
    renderHTML: (attrs: Record<string, unknown>) =>
      attrs[name] ? { [name]: String(attrs[name]) } : {},
  }
}

/** The horizontal placement shared by the framed nodes. */
function alignAttr() {
  return {
    default: 'center',
    parseHTML: (el: HTMLElement) => el.getAttribute('data-align') ?? 'center',
    renderHTML: (attrs: Record<string, unknown>) => ({ 'data-align': String(attrs.align ?? 'center') }),
  }
}

/** A stored video, played by the browser's own player. */
export const Video = Node.create({
  name: 'video',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return {
      attachmentId: attachmentAttr(),
      align: alignAttr(),
      width: sizeAttr('width'),
      height: sizeAttr('height'),
    }
  },

  parseHTML() {
    return [{ tag: 'video[data-attachment-id]' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    const id = node.attrs.attachmentId as string | null
    return ['video', mergeAttributes(HTMLAttributes, {
      controls: 'controls', preload: 'metadata', ...(id ? { src: attachmentSrc(id) } : {}),
    })]
  },

  addCommands() {
    return {
      insertVideo: (attachmentId) => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { attachmentId } }),
    }
  },
})

/** A stored audio file. */
export const Audio = Node.create({
  name: 'audio',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return { attachmentId: attachmentAttr() }
  },

  parseHTML() {
    return [{ tag: 'audio[data-attachment-id]' }]
  },

  renderHTML({ HTMLAttributes, node }) {
    const id = node.attrs.attachmentId as string | null
    return ['audio', mergeAttributes(HTMLAttributes, {
      controls: 'controls', preload: 'metadata', ...(id ? { src: attachmentSrc(id) } : {}),
    })]
  },

  addCommands() {
    return {
      insertAudio: (attachmentId) => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { attachmentId } }),
    }
  },
})

/** A stored PDF, shown in the browser's own viewer. */
export const PdfEmbed = Node.create({
  name: 'pdfEmbed',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return {
      attachmentId: attachmentAttr(),
      name: {
        default: '',
        parseHTML: (el: HTMLElement) => el.getAttribute('data-name') ?? '',
        renderHTML: (attrs: Record<string, unknown>) => ({ 'data-name': String(attrs.name ?? '') }),
      },
      width: sizeAttr('width'),
      height: sizeAttr('height'),
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-attachment-id].pdf-embed' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { class: 'pdf-embed' })]
  },

  addCommands() {
    return {
      insertPdf: (attachmentId, name = '') => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { attachmentId, name } }),
    }
  },
})

/**
 * An external page in a frame.
 *
 * The node stores the address its author pasted, never the frame address. The
 * server derives what to frame from its allow-list on every read, so a
 * deployment that narrows the list stops framing existing embeds at the next
 * page load rather than waiting for somebody to save the page again.
 */
export const Embed = Node.create({
  name: 'embed',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes() {
    return {
      provider: {
        default: '',
        parseHTML: (el: HTMLElement) => el.getAttribute('data-provider') ?? '',
        renderHTML: (attrs: Record<string, unknown>) => ({ 'data-provider': String(attrs.provider ?? '') }),
      },
      url: {
        default: '',
        parseHTML: (el: HTMLElement) => el.getAttribute('data-url') ?? '',
        renderHTML: (attrs: Record<string, unknown>) => ({ 'data-url': String(attrs.url ?? '') }),
      },
      align: alignAttr(),
      width: sizeAttr('width'),
      height: sizeAttr('height'),
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-provider][data-url]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { class: 'embed' })]
  },

  addCommands() {
    return {
      insertEmbed: (provider, url, size) => ({ commands }) =>
        commands.insertContent({
          type: this.name,
          attrs: { provider, url, width: size?.width ?? null, height: size?.height ?? null },
        }),
    }
  },
})

/** The attributes both diagram nodes share. */
function diagramAttributes() {
  return {
    attachmentId: attachmentAttr(),
    previewAttachmentId: attachmentAttr('previewAttachmentId', 'data-preview-attachment-id'),
    align: alignAttr(),
    width: sizeAttr('width'),
    height: sizeAttr('height'),
  }
}

/**
 * A draw.io diagram.
 *
 * Two attachments: the editable source and a rendered preview. Showing the
 * preview is what lets a reader, an export and a share page display the
 * drawing without any editor being loaded at all.
 */
export const Drawio = Node.create({
  name: 'drawio',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes: diagramAttributes,

  parseHTML() {
    return [{ tag: 'figure[data-attachment-id].diagram-drawio' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['figure', mergeAttributes(HTMLAttributes, { class: 'diagram diagram-drawio' })]
  },

  addCommands() {
    return {
      insertDrawio: (attachmentId, previewAttachmentId = null) => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { attachmentId, previewAttachmentId } }),
    }
  },
})

/** An Excalidraw drawing, stored and displayed the same way. */
export const Excalidraw = Node.create({
  name: 'excalidraw',
  group: 'block',
  atom: true,
  draggable: true,
  selectable: true,

  addAttributes: diagramAttributes,

  parseHTML() {
    return [{ tag: 'figure[data-attachment-id].diagram-excalidraw' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['figure', mergeAttributes(HTMLAttributes, { class: 'diagram diagram-excalidraw' })]
  },

  addCommands() {
    return {
      insertExcalidraw: (attachmentId, previewAttachmentId = null) => ({ commands }) =>
        commands.insertContent({ type: this.name, attrs: { attachmentId, previewAttachmentId } }),
    }
  },
})
