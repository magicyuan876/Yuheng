// The catalogue behind the slash menu.
//
// Two things are separated here on purpose. The catalogue itself is data: an
// id, how to label it, and what to run. The matching is a pure function over
// that data. Neither touches Vue or the DOM, so the part that decides what a
// person sees when they type "/im" is under test, while the part that inserts
// a node is a one-line call into the editor's own commands.
//
// Labels are i18n keys rather than words: a slash menu that only matches
// English is not usable in a Chinese document, and the matcher is handed the
// translated label so it matches whatever the reader actually sees.

/** A chain of editor commands, kept loose because it is the editor's own. */
type Chain = Record<string, (...args: never[]) => unknown>

/** The narrow view of the editor a command needs. */
export interface CommandTarget {
  chain: () => Chain
}

/** One entry in the slash menu. */
export interface BlockCommand {
  id: string
  /** The i18n key for the label shown in the menu. */
  labelKey: string
  /** A tdesign icon name. */
  icon: string
  /**
   * A short word drawn in place of the icon.
   *
   * The icon set has no H1/H2/H3 glyph, and a heading entry that borrows the
   * bold icon tells a reader nothing. "H1" drawn as text is what Feishu and
   * Notion both show, and it is unambiguous in every language.
   */
  badge?: string
  /** Which section of the menu it appears under. */
  group: 'basic' | 'insert' | 'media' | 'advanced'
  /**
   * Extra words that should find this entry.
   *
   * These are matched in addition to the translated label, which is what lets
   * somebody type "/h1" for a heading, or a Chinese word while the interface
   * is in English, and still find the thing they meant.
   */
  keywords: string[]
  /**
   * Runs the command.
   *
   * It is handed the range the typed query occupies and every entry removes
   * it, because leaving "/im" sitting in front of a freshly inserted image is
   * the classic slash-menu bug.
   */
  run: (editor: CommandTarget, range: { from: number; to: number }) => void
}

/**
 * Builds the catalogue.
 *
 * A function rather than a constant because what a document may offer depends
 * on what it is allowed to contain: a deployment with embeds turned off should
 * not advertise them.
 */
export interface BlockCommandOptions {
  embeds?: boolean
  drawings?: boolean
  /**
   * Copies this page's current block as a reference link.
   *
   * Passed in rather than built here because it needs the page's id and the
   * clipboard, neither of which belongs in a catalogue of editor commands.
   * Absent means the entry is not offered, which is the right thing on a page
   * that has not been saved yet and therefore has no id to point at.
   */
  copyBlockRef?: () => void
}

export function blockCommands(opts: BlockCommandOptions = {}): BlockCommand[] {
  /** Deletes the typed query, then applies the entry on the same chain. */
  const cmd = (
    id: string, labelKey: string, icon: string,
    group: BlockCommand['group'], keywords: string[],
    apply: (c: Chain) => unknown,
    badge?: string,
  ): BlockCommand => ({
    id, labelKey, icon, group, keywords, badge,
    run: (editor, range) => {
      const focused = (editor.chain().focus as () => Chain)()
      const trimmed = (focused.deleteRange as (r: unknown) => Chain)(range)
      const result = apply(trimmed) as { run?: () => void } | undefined
      result?.run?.()
    },
  })

  const list: BlockCommand[] = [
    cmd('paragraph', 'docs.commands.paragraph', 'text', 'basic',
      ['text', 'paragraph', 'p', '正文', '段落'],
      (c) => (c.setParagraph as () => Chain)()),
    cmd('heading1', 'docs.commands.heading1', 'textformat-bold', 'basic',
      ['h1', 'heading', 'title', '标题', '一级标题'],
      (c) => (c.toggleHeading as (a: unknown) => Chain)({ level: 1 }), 'H1'),
    cmd('heading2', 'docs.commands.heading2', 'textformat-bold', 'basic',
      ['h2', 'heading', '标题', '二级标题'],
      (c) => (c.toggleHeading as (a: unknown) => Chain)({ level: 2 }), 'H2'),
    cmd('heading3', 'docs.commands.heading3', 'textformat-bold', 'basic',
      ['h3', 'heading', '标题', '三级标题'],
      (c) => (c.toggleHeading as (a: unknown) => Chain)({ level: 3 }), 'H3'),
    cmd('bulletList', 'docs.commands.bulletList', 'order-list', 'basic',
      ['ul', 'bullet', 'list', '列表', '无序列表'],
      (c) => (c.toggleBulletList as () => Chain)()),
    cmd('orderedList', 'docs.commands.orderedList', 'order-descending', 'basic',
      ['ol', 'ordered', 'number', 'list', '有序列表', '编号'],
      (c) => (c.toggleOrderedList as () => Chain)()),
    cmd('taskList', 'docs.commands.taskList', 'check-rectangle', 'basic',
      ['todo', 'task', 'checkbox', '任务', '待办'],
      (c) => (c.toggleTaskList as () => Chain)()),
    cmd('blockquote', 'docs.commands.blockquote', 'quote', 'basic',
      ['quote', '引用'],
      (c) => (c.toggleBlockquote as () => Chain)()),
    cmd('codeBlock', 'docs.commands.codeBlock', 'code', 'basic',
      ['code', 'pre', '代码', '代码块'],
      (c) => (c.toggleCodeBlock as () => Chain)()),

    cmd('table', 'docs.commands.table', 'table', 'insert',
      ['table', 'grid', '表格'],
      (c) => (c.insertTable as (a: unknown) => Chain)({ rows: 3, cols: 3, withHeaderRow: true })),
    cmd('callout', 'docs.commands.callout', 'info-circle', 'insert',
      ['callout', 'note', 'warning', 'tip', '提示', '高亮块'],
      (c) => (c.setCallout as (a: unknown) => Chain)('info')),
    cmd('columns', 'docs.commands.columns', 'view-column', 'insert',
      ['columns', 'layout', '分栏', '列'],
      (c) => (c.insertColumns as (a: unknown) => Chain)(2)),
    cmd('divider', 'docs.commands.divider', 'minus', 'insert',
      ['divider', 'hr', 'rule', 'separator', '分割线'],
      (c) => (c.setHorizontalRule as () => Chain)()),
    cmd('pageBreak', 'docs.commands.pageBreak', 'page-first', 'insert',
      ['break', 'page', '分页'],
      (c) => (c.insertPageBreak as () => Chain)()),
    cmd('toc', 'docs.commands.toc', 'list-numbered', 'insert',
      ['toc', 'outline', 'contents', '目录'],
      (c) => (c.insertTableOfContents as () => Chain)()),
    cmd('status', 'docs.commands.status', 'tag', 'insert',
      ['status', 'label', 'badge', '状态', '标签'],
      (c) => (c.insertStatus as (a: unknown, b: unknown) => Chain)('', 'gray')),

    cmd('mathBlock', 'docs.commands.mathBlock', 'formula', 'advanced',
      ['math', 'latex', 'formula', 'equation', '公式', '数学'],
      (c) => (c.insertMathBlock as (a: unknown) => Chain)('')),
    cmd('mathInline', 'docs.commands.mathInline', 'formula', 'advanced',
      ['math', 'latex', 'inline', '行内公式'],
      (c) => (c.insertMathInline as (a: unknown) => Chain)('')),
    cmd('mermaid', 'docs.commands.mermaid', 'chart-bubble', 'advanced',
      ['mermaid', 'diagram', 'flowchart', '流程图', '图表'],
      (c) => (c.insertMermaid as () => Chain)()),
  ]

  if (opts.embeds !== false) {
    list.push(cmd('embed', 'docs.commands.embed', 'link', 'media',
      ['embed', 'iframe', 'video', '嵌入'],
      (c) => (c.insertEmbed as (a: unknown, b: unknown) => Chain)('external', '')))
  }
  if (opts.copyBlockRef) {
    const copy = opts.copyBlockRef
    list.push({
      id: 'copyBlockRef',
      labelKey: 'docs.commands.copyBlockRef',
      icon: 'quote',
      group: 'advanced',
      keywords: ['reference', 'transclude', 'quote', 'block', '引用', '块引用', '复制引用'],
      run: (editor, range) => {
        // The typed query goes first: the link is about the block, and
        // leaving "/ref" in it would put that text in what everybody else
        // then sees quoted.
        const focused = (editor.chain().focus as () => Chain)()
        const trimmed = (focused.deleteRange as (r: unknown) => Chain)(range)
        ;(trimmed.run as () => void)()
        copy()
      },
    })
  }
  if (opts.drawings !== false) {
    list.push(cmd('excalidraw', 'docs.commands.excalidraw', 'edit', 'media',
      ['draw', 'sketch', 'whiteboard', '白板', '手绘'],
      (c) => (c.insertExcalidraw as (a: unknown, b: unknown) => Chain)('', null)))
  }
  return list
}

/**
 * Filters and ranks the catalogue against what has been typed.
 *
 * An empty query returns the catalogue in its own order, which is the order
 * the menu is designed to be read in. A non-empty query ranks a match on the
 * start of the label above one in the middle of a keyword, so typing "co"
 * offers "Code block" before "Callout" — the entry whose name begins that way
 * is nearly always the one that was meant.
 *
 * Matching is case-insensitive and, within a rank, stable: two entries that
 * match equally well stay in catalogue order.
 */
export function matchCommands(
  catalogue: readonly BlockCommand[],
  query: string,
  label: (command: BlockCommand) => string,
): BlockCommand[] {
  const needle = query.trim().toLowerCase()
  if (needle === '') return [...catalogue]

  const scored: { command: BlockCommand; rank: number; at: number }[] = []
  catalogue.forEach((command, at) => {
    const rank = rankOf(command, needle, label(command).toLowerCase())
    if (rank >= 0) scored.push({ command, rank, at })
  })
  scored.sort((a, b) => (a.rank - b.rank) || (a.at - b.at))
  return scored.map((s) => s.command)
}

/** Lower is a better match; -1 is no match at all. */
function rankOf(command: BlockCommand, needle: string, label: string): number {
  if (label.startsWith(needle)) return 0
  if (command.keywords.some((k) => k.toLowerCase().startsWith(needle))) return 1
  if (label.includes(needle)) return 2
  if (command.keywords.some((k) => k.toLowerCase().includes(needle))) return 3
  return -1
}
