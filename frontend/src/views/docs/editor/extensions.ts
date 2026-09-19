// Assembles the editor's Tiptap extension set. Every node/mark here is one of
// packages/docs-schema's definitions. All of them come from official,
// MIT-licensed Tiptap packages except the two media nodes (image, attachment),
// which the schema declares in a shape no stock extension has and which live
// in mediaNodes.ts. The handful the schema declares but no work package has
// implemented yet (callouts, mentions, embeds, and the rest listed in the
// design's node table under T2.x/T3.2) are intentionally absent -- a page
// containing one fails to load until its own work package lands, which is
// expected at this stage and asserted as expected in extensions.test.ts.
//
// Each extension is `.extend()`-ed only where the stock default diverges
// from packages/docs-schema/schema.json (extra attributes such as Link's
// `target`/`rel`, or a content expression that needs a different option).
// Where the stock shape already matches, it is used as-is -- recorded in
// the comment on each entry so the next reader does not have to re-derive
// it by reading the library's source.
import { Extension, type AnyExtension, type NodeViewRenderer } from '@tiptap/core'
import Blockquote from '@tiptap/extension-blockquote'
import Bold from '@tiptap/extension-bold'
import Code from '@tiptap/extension-code'
import { CodeBlockLowlight } from '@tiptap/extension-code-block-lowlight'
import Details, { DetailsContent, DetailsSummary } from '@tiptap/extension-details'
import Document from '@tiptap/extension-document'
import Dropcursor from '@tiptap/extension-dropcursor'
import Gapcursor from '@tiptap/extension-gapcursor'
import HardBreak from '@tiptap/extension-hard-break'
import Heading from '@tiptap/extension-heading'
import Highlight from '@tiptap/extension-highlight'
import HorizontalRule from '@tiptap/extension-horizontal-rule'
import Italic from '@tiptap/extension-italic'
import { BulletList, ListItem, ListKeymap, OrderedList, TaskItem, TaskList } from '@tiptap/extension-list'
import Link from '@tiptap/extension-link'
import Paragraph from '@tiptap/extension-paragraph'
import Strike from '@tiptap/extension-strike'
import Subscript from '@tiptap/extension-subscript'
import Superscript from '@tiptap/extension-superscript'
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table'
import Text from '@tiptap/extension-text'
import { Color, TextStyle } from '@tiptap/extension-text-style'
import Underline from '@tiptap/extension-underline'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import markdownLang from 'highlight.js/lib/languages/markdown'
import python from 'highlight.js/lib/languages/python'
import rust from 'highlight.js/lib/languages/rust'
import shell from 'highlight.js/lib/languages/shell'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import { createLowlight } from 'lowlight'

import { BlockId, TextBlockAttrs } from './blockAttrs'
import { MathBlock, MathInline, Mermaid } from './figureNodes'
import { Mention, PageLink } from './linkNodes'
import { Transclusion } from './transclusionNode'
import { Audio, Drawio, Embed, Excalidraw, PdfEmbed, Video } from './mediaEmbedNodes'
import { DocAttachment, DocImage } from './mediaNodes'
import { Callout, Column, Columns, PageBreak, Status, TableOfContents } from './structureNodes'

// A small, common core rather than lowlight's `common`/`all` bundle: this is
// a documentation tool, not a code sandbox, and a smaller grammar set keeps
// the bundle down. Extend this list as real pages need more languages.
const lowlight = createLowlight()
for (const [name, lang] of Object.entries({
  bash, css, go, java, javascript, json, markdown: markdownLang, python, rust, shell, sql, typescript, xml, yaml,
})) {
  lowlight.register(name, lang)
}

/**
 * `href`, `title`, `internal` only -- schema.json's `link` mark. Stock Link
 * additionally carries `target`/`rel`/`class` (used to open links in a new
 * tab and mark them `rel=noopener`); those are pure rendering concerns we
 * still want, so they are supplied as fixed `HTMLAttributes` instead of
 * document attributes that would otherwise get persisted and rejected by
 * the server's schema validator.
 */
const DocLink = Link.extend({
  excludes: 'link',
  addAttributes() {
    return {
      href: { default: null },
      title: { default: null },
      internal: { default: false },
    }
  },
}).configure({
  openOnClick: false, // T2.4 wires the actual click-to-navigate behaviour
  HTMLAttributes: { target: '_blank', rel: 'noopener noreferrer nofollow' },
})

/** `wrap` on top of the stock `language` attribute. */
const DocCodeBlock = CodeBlockLowlight.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      wrap: { default: false },
    }
  },
}).configure({ lowlight })

/**
 * `colspan`/`rowspan`/`colwidth` stay as the stock default; `align` (an
 * editor-only convenience the schema does not have) is swapped for the
 * schema's `backgroundColor`.
 */
function cellAttributes() {
  return {
    colspan: { default: 1 },
    rowspan: { default: 1 },
    colwidth: { default: null },
    backgroundColor: {
      default: null,
      parseHTML: (element: HTMLElement) => element.style.backgroundColor || null,
      renderHTML: (attributes: { backgroundColor?: string | null }) => (
        attributes.backgroundColor ? { style: `background-color: ${attributes.backgroundColor}` } : {}
      ),
    },
  }
}
const DocTableCell = TableCell.extend({ addAttributes: cellAttributes })
const DocTableHeader = TableHeader.extend({ addAttributes: cellAttributes })

/** Enabled (stock default leaves it off) and defaulting open, per schema. */
const DocDetails = Details.extend({
  addAttributes() {
    return { open: { default: true } }
  },
}).configure({ persist: true })

/** Stock summary only allows bare text; the schema allows marks and breaks. */
const DocDetailsSummary = DetailsSummary.extend({ content: 'inline*' })

/** `nested: true` is what makes the content expression `paragraph block*`. */
const DocTaskItem = TaskItem.configure({ nested: true })

/** Mutual exclusion is not stock behaviour; the schema requires it both ways. */
const DocSubscript = Subscript.extend({ excludes: 'superscript' })
const DocSuperscript = Superscript.extend({ excludes: 'subscript' })

/**
 * Node views for the nodes that have one. They are supplied by the caller
 * rather than declared on the node so that mediaNodes.ts stays free of Vue
 * and the schema-conformance test can load it without a DOM.
 */
export interface EditorNodeViews {
  image?: NodeViewRenderer
  attachment?: NodeViewRenderer
  callout?: NodeViewRenderer
  columns?: NodeViewRenderer
  column?: NodeViewRenderer
  status?: NodeViewRenderer
  toc?: NodeViewRenderer
  mathInline?: NodeViewRenderer
  mathBlock?: NodeViewRenderer
  mermaid?: NodeViewRenderer
  pageLink?: NodeViewRenderer
  mention?: NodeViewRenderer
  transclusion?: NodeViewRenderer
  video?: NodeViewRenderer
  audio?: NodeViewRenderer
  pdfEmbed?: NodeViewRenderer
  embed?: NodeViewRenderer
  drawio?: NodeViewRenderer
  excalidraw?: NodeViewRenderer
}

/** Attaches a node view to a node, when the caller supplied one. */
function withView<T extends AnyExtension>(node: T, view?: NodeViewRenderer): AnyExtension {
  if (!view) return node
  return node.extend({ addNodeView: () => view })
}

/**
 * The covered subset of packages/docs-schema. Extras get folded in via
 * `extra` (used by the schema-conformance test to also cover the custom
 * BlockId/TextBlockAttrs extensions without duplicating this whole list
 * there), and `views` supplies the node views for the media nodes.
 */
export function officialExtensions(
  extra: AnyExtension[] = [],
  views: EditorNodeViews = {},
): AnyExtension[] {
  return [
    // doc/text/paragraph: bare, matching the schema exactly.
    Document.extend({ marks: '' }), // no marks allowed directly on the root
    Text,
    Paragraph,
    Heading, // level only, default 1 -- matches
    Blockquote, // content "block+", no attrs -- matches
    HorizontalRule, // atom, no attrs -- matches
    HardBreak, // inline atom, no attrs -- matches
    BulletList,
    OrderedList.extend({
      // Stock adds `type` (ordered-list numbering style) alongside `start`;
      // the schema only has `start`.
      addAttributes() {
        return { start: { default: 1 } }
      },
    }),
    ListItem,
    TaskList,
    DocTaskItem,
    DocCodeBlock,
    // Resizable is off in the stock extension, which leaves every column the
    // same width for ever — the single biggest gap between this table and the
    // one Feishu or Yuque inserts. `cellMinWidth` stops a column being dragged
    // down to nothing, and allowing a node selection is what lets the table
    // toolbar act on the table as a whole.
    Table.configure({
      resizable: true,
      handleWidth: 6,
      cellMinWidth: 60,
      lastColumnResizable: true,
      allowTableNodeSelection: true,
    }),
    TableRow,
    DocTableCell,
    DocTableHeader,
    DocDetails,
    DocDetailsSummary,
    DetailsContent,
    withView(DocImage, views.image),
    withView(DocAttachment, views.attachment),
    // Structure and figures (T2.1).
    withView(Callout, views.callout),
    withView(Columns, views.columns),
    withView(Column, views.column),
    withView(Status, views.status),
    PageBreak,
    withView(TableOfContents, views.toc),
    withView(MathInline, views.mathInline),
    withView(MathBlock, views.mathBlock),
    withView(Mermaid, views.mermaid),
    // Links and mentions (T2.2).
    withView(PageLink, views.pageLink),
    withView(Mention, views.mention),
    withView(Transclusion, views.transclusion),
    // Media, embeds and diagrams (T2.3).
    withView(Video, views.video),
    withView(Audio, views.audio),
    withView(PdfEmbed, views.pdfEmbed),
    withView(Embed, views.embed),
    withView(Drawio, views.drawio),
    withView(Excalidraw, views.excalidraw),
    // Marks.
    Bold,
    Italic,
    Underline,
    Strike,
    Code,
    DocLink,
    Highlight.configure({ multicolor: true }),
    DocSubscript,
    DocSuperscript,
    TextStyle,
    Color,
    // Editing conveniences with no schema footprint of their own.
    Dropcursor,
    Gapcursor,
    ListKeymap,
    // Shared attribute sets.
    BlockId,
    TextBlockAttrs,
    ...extra,
  ]
}

/**
 * A no-op Extension purely so callers that pass an empty `extra` still get a
 * non-empty, obviously-intentional list back.
 */
export const NoopExtension = Extension.create({ name: 'yuhengNoop' })
