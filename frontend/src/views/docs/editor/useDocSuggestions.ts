// Drives the `[[` and `@` menus: watches the trigger, fetches candidates,
// handles the keyboard, and inserts the chosen node.
//
// The decisions are all elsewhere — suggestion.ts decides when a menu is open
// and what has been typed into it, and the server decides which candidates the
// caller is allowed to see. This is the wiring in between.
import { Extension } from '@tiptap/core'
import type { Editor } from '@tiptap/core'
import { computed, ref, shallowRef, type Ref } from 'vue'

import { suggestMentions, suggestPages, type MentionCandidate, type PageRef } from '@/api/docs'

import {
  blockCommands, matchCommands,
  type BlockCommand, type BlockCommandOptions, type CommandTarget,
} from './commands'
import { matchEmoji, type Emoji } from './emoji'
import type { SuggestionItem } from './SuggestionMenu.vue'
import {
  moveSelection, suggestionPlugin,
  type ActiveTrigger, type Trigger,
} from './suggestion'

/** The triggers this editor answers to. */
const TRIGGERS: Trigger[] = [
  { name: 'page', chars: '[[' },
  // Only after whitespace, so an email address does not open the menu.
  { name: 'mention', chars: '@', requireBoundary: true },
  // Only after whitespace, so a date or a path does not open the menu.
  { name: 'command', chars: '/', requireBoundary: true },
  // A colon is ordinary punctuation, so this one needs both a boundary before
  // it and something typed after it; matchEmoji returns nothing for an empty
  // query, which is what keeps a menu off an ordinary sentence.
  { name: 'emoji', chars: ':', requireBoundary: true, maxQuery: 24 },
]

/** How long to wait after a keystroke before asking the server. */
const QUERY_DEBOUNCE_MS = 140

/** Where the recently used command ids live, most recent first. */
const RECENT_COMMANDS_KEY = 'yuheng.docs.recentCommands'
const RECENT_COMMANDS_MAX = 5

function readRecentCommands(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_COMMANDS_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : []
  } catch {
    return []
  }
}

function recordRecentCommand(id: string): void {
  try {
    const next = [id, ...readRecentCommands().filter((existing) => existing !== id)]
    localStorage.setItem(RECENT_COMMANDS_KEY, JSON.stringify(next.slice(0, RECENT_COMMANDS_MAX)))
  } catch {
    // A full or unavailable storage loses the recents, nothing more.
  }
}

/** Which menu is open. */
export type SuggestionKind = 'page' | 'mention' | 'command' | 'emoji'

export interface DocSuggestionsOptions {
  pageId: Ref<string>
  spaceId: Ref<string>
  /** People are looked up so a mention can show the name they use now. */
  rememberPerson: (person: MentionCandidate) => void
  /** Translates a command's label key, so the slash menu matches the words
   * somebody actually reads rather than the English behind them. */
  translate: (key: string) => string
  /** What this deployment allows, so the menu offers nothing that would fail.
   * Read on each keystroke rather than once, because the policy arrives from
   * the server after the editor has already been built. */
  allow?: () => BlockCommandOptions
}

export interface DocSuggestionsHandle {
  extension: Extension
  open: Ref<boolean>
  loading: Ref<boolean>
  kind: Ref<SuggestionKind>
  items: Ref<SuggestionItem[]>
  selected: Ref<number>
  position: Ref<{ left: number; top: number }>
  /** Handles the keys the menu owns; returns true when it consumed one. */
  handleKey: (event: KeyboardEvent) => boolean
  /** Inserts the entry at index, or the selected one. */
  choose: (index?: number) => void
  hover: (index: number) => void
  bind: (editor: Editor | null) => void
}

export function useDocSuggestions(opts: DocSuggestionsOptions): DocSuggestionsHandle {
  const open = ref(false)
  const loading = ref(false)
  const kind = ref<SuggestionKind>('page')
  const selected = ref(0)
  const position = ref({ left: 0, top: 0 })
  const pages = shallowRef<PageRef[]>([])
  const people = shallowRef<MentionCandidate[]>([])
  // The command menu answers from memory: there is nothing to fetch, so it
  // has a query rather than a result list, and filters on every keystroke.
  const commandQuery = ref('')
  const catalogue = computed(() => blockCommands(opts.allow?.() ?? {}))
  const commands = computed<BlockCommand[]>(
    () => matchCommands(catalogue.value, commandQuery.value, (c) => opts.translate(c.labelKey)),
  )
  // The emoji menu answers from memory in the same way.
  const emojiQuery = ref('')
  const emojis = computed<Emoji[]>(() => matchEmoji(emojiQuery.value))

  let bound: Editor | null = null
  let active: ActiveTrigger | null = null
  let timer: ReturnType<typeof setTimeout> | null = null
  // Every fetch carries a ticket; a late answer to a query nobody is typing
  // any more is dropped rather than shown.
  let ticket = 0

  const items = computed<SuggestionItem[]>(() => {
    if (kind.value === 'emoji') {
      return emojis.value.map((entry) => ({
        key: entry.char,
        title: entry.name,
        icon: entry.char,
      }))
    }
    if (kind.value === 'command') {
      const matches = commands.value
      // The group label is resolved here rather than in the menu: the menu
      // is one component for four kinds of suggestion, and only commands
      // carry a section.
      const groupLabel = (group: string) => opts.translate(`docs.commands.group.${group}`)
      const rows: SuggestionItem[] = []
      if (commandQuery.value.trim() === '') {
        // Nothing typed yet: the last few commands used get a "Recent"
        // section of their own, ahead of the catalogue, because that is
        // where a hand already knows what it wants to do next. A query
        // narrows the whole catalogue instead, and the section would only
        // repeat rows already underneath it.
        const byId = new Map(matches.map((c) => [c.id, c]))
        for (const id of readRecentCommands()) {
          const command = byId.get(id)
          if (command) {
            rows.push({
              key: command.id,
              title: opts.translate(command.labelKey),
              iconName: command.icon,
              group: opts.translate('docs.commands.group.recent'),
            })
          }
        }
      }
      for (const command of matches) {
        rows.push({
          key: command.id,
          title: opts.translate(command.labelKey),
          iconName: command.icon,
          group: groupLabel(command.group),
        })
      }
      return rows
    }
    if (kind.value === 'page') {
      return pages.value.map((p) => ({
        key: p.page_id,
        title: p.title || '…',
        hint: p.breadcrumb?.join(' / '),
        icon: p.icon,
      }))
    }
    return people.value.map((p) => ({
      key: p.user_id,
      title: p.username || p.email || p.user_id,
      hint: p.username ? p.email : undefined,
    }))
  })

  function close() {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
    ticket++
    active = null
    commandQuery.value = ''
    emojiQuery.value = ''
    open.value = false
    loading.value = false
    pages.value = []
    people.value = []
    selected.value = 0
  }

  function onTrigger(next: ActiveTrigger | null) {
    if (!next || !bound?.isEditable) {
      close()
      return
    }
    active = next
    kind.value = next.name === 'command' ? 'command'
      : next.name === 'emoji' ? 'emoji'
        : next.name === 'mention' ? 'mention' : 'page'
    open.value = true
    selected.value = 0
    position.value = caretPosition(bound, next.to)

    if (timer !== null) clearTimeout(timer)
    if (next.name === 'command' || next.name === 'emoji') {
      // Nothing to wait for, so nothing is debounced: the list is recomputed
      // from the catalogue as the query changes.
      if (next.name === 'command') commandQuery.value = next.query
      else emojiQuery.value = next.query
      loading.value = false
      return
    }
    loading.value = true
    timer = setTimeout(() => {
      timer = null
      void fetchCandidates(next)
    }, QUERY_DEBOUNCE_MS)
  }

  async function fetchCandidates(trigger: ActiveTrigger): Promise<void> {
    const mine = ++ticket
    try {
      if (trigger.name === 'mention') {
        const rows = await suggestMentions(opts.pageId.value, { q: trigger.query })
        if (mine !== ticket) return
        people.value = rows
        for (const row of rows) opts.rememberPerson(row)
      } else {
        const rows = await suggestPages({ q: trigger.query, space: opts.spaceId.value })
        if (mine !== ticket) return
        pages.value = rows
      }
    } catch {
      // A failed lookup shows an empty menu rather than an error dialogue: the
      // person is in the middle of a sentence, and the menu closes the moment
      // they carry on typing.
      if (mine !== ticket) return
      pages.value = []
      people.value = []
    } finally {
      if (mine === ticket) loading.value = false
    }
  }

  function choose(index?: number): void {
    const at = index ?? selected.value
    const item = items.value[at]
    if (!item || !active || !bound) return
    const range = { from: active.from, to: active.to }
    if (kind.value === 'emoji') {
      // The character replaces the ":name" that was typed, which is what the
      // shorthand is for.
      bound.chain().focus().deleteRange(range).insertContent(item.key).run()
      close()
      return
    }
    if (kind.value === 'command') {
      // Looked up by id rather than by row index: the "Recent" section adds
      // rows that are also in the catalogue, so an index into `commands`
      // would run the entry next to the one that was clicked.
      const command = catalogue.value.find((c) => c.id === item.key)
      // Closed first: the command edits the document, and a menu still holding
      // a range that no longer exists is the source of the stray-slash bug.
      close()
      if (command) recordRecentCommand(command.id)
      command?.run(bound as unknown as CommandTarget, range)
      return
    }
    if (kind.value === 'mention') {
      const person = people.value[at]
      bound.chain().focus()
        .insertMention(item.key, person?.username || person?.email || undefined, range)
        .run()
    } else {
      bound.chain().focus().insertPageLink(item.key, range).run()
    }
    close()
  }

  /**
   * The keys a menu owns while it is open.
   *
   * Everything else falls through to the editor, which is what keeps typing
   * inside a menu working: the query is ordinary text in the document until
   * something is chosen, so a menu that never resolves leaves exactly the
   * characters that were typed.
   */
  function handleKey(event: KeyboardEvent): boolean {
    if (!open.value) return false
    switch (event.key) {
      case 'ArrowDown':
        selected.value = moveSelection(selected.value, 1, items.value.length)
        return true
      case 'ArrowUp':
        selected.value = moveSelection(selected.value, -1, items.value.length)
        return true
      case 'Enter':
      case 'Tab':
        if (items.value.length === 0) return false
        choose()
        return true
      case 'Escape':
        close()
        return true
      default:
        return false
    }
  }

  const extension = Extension.create({
    name: 'yuhengSuggestions',
    addProseMirrorPlugins: () => [suggestionPlugin(TRIGGERS, (next) => onTrigger(next))],
  })

  return {
    extension, open, loading, kind, items, selected, position,
    handleKey, choose,
    hover: (index: number) => {
      selected.value = index
    },
    bind: (editor) => {
      bound = editor
      if (!editor) close()
    },
  }
}

/** Where to put the menu: just under the caret, clamped to the viewport. */
function caretPosition(editor: Editor, pos: number): { left: number; top: number } {
  try {
    const rect = editor.view.coordsAtPos(pos)
    const left = Math.min(rect.left, window.innerWidth - 380)
    const top = Math.min(rect.bottom + 6, window.innerHeight - 300)
    return { left: Math.max(8, left), top: Math.max(8, top) }
  } catch {
    // The position can be stale for a frame after a big edit; a menu in the
    // corner is better than one that throws.
    return { left: 16, top: 120 }
  }
}
