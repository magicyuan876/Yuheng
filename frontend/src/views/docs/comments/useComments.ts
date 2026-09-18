// Drives a page's comments: loading them, placing them, and keeping the
// highlights in step with the document.
//
// The decisions are elsewhere — placement.ts works out where a comment points
// and decorations.ts draws it — so what is here is the wiring, plus the one
// thing that genuinely belongs at this level: resolving a stored Yjs relative
// position against the live document, which needs the Y.Doc and the editor's
// binding and nothing else does.
// From @tiptap/y-tiptap rather than y-prosemirror: Tiptap v3 ships its own
// fork of the binding and that is the one the collaboration extension
// installs, so its plugin key is the one in the editor's state.
import {
  absolutePositionToRelativePosition, relativePositionToAbsolutePosition, ySyncPluginKey,
} from '@tiptap/y-tiptap'
import { computed, ref, shallowRef, type Ref } from 'vue'
import * as Y from 'yjs'

import {
  createComment, deleteComment as deleteCommentAPI, listComments,
  resolveComment as resolveCommentAPI, updateComment,
  type CommentView, type CreateCommentBody,
} from '@/api/docs'

import { IdleScheduler } from '../editor/idleWork'

import { setCommentHighlights } from './decorations'
import { placeComments, partitionPlacements, type Placement } from './placement'

/** The editor surface this composable needs; narrowed so tests can stand in. */
interface EditorLike {
  state: { doc: unknown; tr: unknown }
  view: { state: unknown; dispatch: (tr: unknown) => void }
  isDestroyed: boolean
}

export interface CommentsOptions {
  pageId: Ref<string>
  /** The collaborative document, when there is one. */
  ydoc: Ref<Y.Doc | null>
  onError?: (message: string) => void
}

export function useComments(opts: CommentsOptions) {
  const threads = ref<CommentView[]>([])
  const open = ref(0)
  const total = ref(0)
  const loading = ref(false)
  const showResolved = ref(false)
  const activeID = ref('')
  const placements = shallowRef<Placement[]>([])

  let editor: EditorLike | null = null

  /**
   * Turns a stored relative position into a range in the live document.
   *
   * This is the half that cannot be pure: it needs the Y.Doc, the shared type
   * the editor is bound to, and the binding's mapping between them. When
   * any of those is missing — a page being edited without a collaboration
   * service, whose Yjs state is rebuilt from stored JSON — every position
   * fails to resolve and every inline comment falls back to its quotation,
   * which is exactly what that fallback is for.
   */
  function resolveAnchor(anchor: unknown): { from: number; to: number } | null {
    const ydoc = opts.ydoc.value
    const view = editor?.view as { state: unknown } | undefined
    if (!ydoc || !view || !anchor || typeof anchor !== 'object') return null

    const sync = ySyncPluginKey.getState(view.state as never) as
      { type?: Y.XmlFragment; binding?: { mapping: unknown } } | undefined
    if (!sync?.type || !sync.binding) return null

    const range = anchor as { start?: unknown; end?: unknown }
    const from = toAbsolute(ydoc, sync, range.start)
    const to = toAbsolute(ydoc, sync, range.end)
    if (from === null || to === null) return null
    return from <= to ? { from, to } : { from: to, to: from }
  }

  function toAbsolute(
    ydoc: Y.Doc,
    sync: { type?: Y.XmlFragment; binding?: { mapping: unknown } },
    position: unknown,
  ): number | null {
    if (!position) return null
    const relative = Y.createRelativePositionFromJSON(position as never)
    const absolute = relativePositionToAbsolutePosition(
      ydoc, sync.type as Y.XmlFragment, relative, (sync.binding as { mapping: never }).mapping,
    )
    return typeof absolute === 'number' ? absolute : null
  }

  /**
   * Turns a range in the live document into the anchor to store.
   *
   * The inverse of resolveAnchor, and the reason a comment survives other
   * people editing around it: what is stored is a position in the shared
   * document rather than an offset into this version of the text.
   *
   * Returns null when there is no collaborative document to describe the
   * position against. The comment is then stored with its quotation and no
   * anchor, which places it by text and says so — honest, and better than
   * storing an offset that would be wrong after the next edit.
   */
  function anchorFor(from: number, to: number): unknown | null {
    const ydoc = opts.ydoc.value
    const view = editor?.view as { state: unknown } | undefined
    if (!ydoc || !view || to <= from) return null

    const sync = ySyncPluginKey.getState(view.state as never) as
      { type?: Y.XmlFragment; binding?: { mapping: unknown } } | undefined
    if (!sync?.type || !sync.binding) return null

    try {
      const start = absolutePositionToRelativePosition(
        from, sync.type as Y.XmlFragment, (sync.binding as { mapping: never }).mapping,
      )
      const end = absolutePositionToRelativePosition(
        to, sync.type as Y.XmlFragment, (sync.binding as { mapping: never }).mapping,
      )
      return {
        start: Y.relativePositionToJSON(start),
        end: Y.relativePositionToJSON(end),
      }
    } catch {
      return null
    }
  }

  /**
   * Recomputes where every comment points and hands the result to the plugin.
   *
   * Deferred through the same idle scheduler the word count uses: placing a
   * comment walks the document, an editing session changes the document
   * constantly, and a highlight arriving a moment late costs nothing.
   */
  const rescan = new IdleScheduler(() => refreshPlacements())

  function refreshPlacements() {
    const ed = editor
    if (!ed || ed.isDestroyed) return
    const flat = flatten(threads.value)
    placements.value = placeComments(
      ed.state.doc as never,
      flat.map((c) => ({ id: c.id, anchor: c.anchor, quotedText: c.quoted_text })),
      resolveAnchor,
    )
    setCommentHighlights(ed.view as never, {
      placements: placements.value,
      activeID: activeID.value || undefined,
    })
  }

  /** Every comment, threads and replies alike; only threads carry anchors. */
  function flatten(items: readonly CommentView[]): CommentView[] {
    const out: CommentView[] = []
    for (const item of items) {
      out.push(item)
      if (item.replies) out.push(...item.replies)
    }
    return out
  }

  const placementByID = computed(() => {
    const map = new Map<string, Placement>()
    for (const placement of placements.value) map.set(placement.id, placement)
    return map
  })

  /** Threads grouped the way the sidebar shows them. */
  const grouped = computed(() => {
    const { inline, page, orphaned } = partitionPlacements(placements.value)
    const byID = new Map(threads.value.map((thread) => [thread.id, thread]))
    const pick = (list: Placement[]) => list
      .map((placement) => byID.get(placement.id))
      .filter((thread): thread is CommentView => thread !== undefined)
    return { inline: pick(inline), page: pick(page), orphaned: pick(orphaned) }
  })

  async function load() {
    if (!opts.pageId.value) return
    loading.value = true
    try {
      const list = await listComments(opts.pageId.value, showResolved.value)
      threads.value = list.items
      open.value = list.open
      total.value = list.total
      refreshPlacements()
    } catch (err) {
      opts.onError?.((err as { message?: string })?.message ?? '')
    } finally {
      loading.value = false
    }
  }

  async function add(body: CreateCommentBody): Promise<CommentView | null> {
    try {
      const view = await createComment(opts.pageId.value, body)
      await load()
      activeID.value = view.parent_id || view.id
      return view
    } catch (err) {
      opts.onError?.((err as { message?: string })?.message ?? '')
      return null
    }
  }

  async function edit(commentID: string, body: unknown): Promise<boolean> {
    try {
      await updateComment(opts.pageId.value, commentID, body)
      await load()
      return true
    } catch (err) {
      opts.onError?.((err as { message?: string })?.message ?? '')
      return false
    }
  }

  async function setResolved(commentID: string, resolved: boolean): Promise<boolean> {
    try {
      await resolveCommentAPI(opts.pageId.value, commentID, resolved)
      await load()
      return true
    } catch (err) {
      opts.onError?.((err as { message?: string })?.message ?? '')
      return false
    }
  }

  async function remove(commentID: string): Promise<boolean> {
    try {
      await deleteCommentAPI(opts.pageId.value, commentID)
      if (activeID.value === commentID) activeID.value = ''
      await load()
      return true
    } catch (err) {
      opts.onError?.((err as { message?: string })?.message ?? '')
      return false
    }
  }

  return {
    threads, open, total, loading, showResolved, activeID, placements, placementByID, grouped,

    /** Hands over the editor once it exists. */
    bind: (next: EditorLike | null) => {
      editor = next
      if (next) refreshPlacements()
    },
    /** Called on every document change; cheap, and coalesced. */
    touch: () => rescan.schedule(),
    select: (commentID: string) => {
      activeID.value = commentID
      refreshPlacements()
    },
    load,
    anchorFor,
    add,
    edit,
    setResolved,
    remove,
    dispose: () => {
      rescan.cancel()
      editor = null
    },
  }
}

export type CommentsHandle = ReturnType<typeof useComments>
