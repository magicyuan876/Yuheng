// Wires one page's Y.Doc to the collaboration service (HocuspocusProvider)
// and to y-indexeddb for local durability, and exposes the reactive state
// DocEditor.vue needs. All the actual decisions (is it editable right now,
// what banner to show, how to shape a collaborator) live in session.ts and
// are exercised by its own tests; this file is wiring and is exercised by
// using the real editor, same as useDocsEvents.ts's SSE connection.
import { HocuspocusProvider } from '@hocuspocus/provider'
import { computed, onBeforeUnmount, ref, shallowRef, watch, type Ref } from 'vue'
import { IndexeddbPersistence } from 'y-indexeddb'
import * as Y from 'yjs'

import {
  awarenessUser, dedupeOnlineUsers, deriveBanner, isEditable,
  type AwarenessUser, type Banner, type CollabAccess, type ConnectionStatus, type UserLike,
} from './session'

export interface DocCollabHandle {
  ydoc: Ref<Y.Doc>
  provider: Ref<HocuspocusProvider | null>
  status: Ref<ConnectionStatus>
  collabAccess: Ref<CollabAccess>
  editable: Ref<boolean>
  banner: Ref<Banner>
  onlineUsers: Ref<AwarenessUser[]>
  /** True once the very first sync (server state, or the local
   * y-indexeddb cache while still connecting) has been applied, so the
   * editor can wait to mount until there is something real to show. */
  ready: Ref<boolean>
}

export interface DocCollabOptions {
  /** Reactive: switching pages tears down the old session and opens a new one. */
  pageId: Ref<string | undefined>
  tenantId: Ref<string | number | undefined>
  /** The page's own ACL answer (REST `can_edit`); independent of, and
   * checked before, whatever the collaboration socket confirms. */
  canEditPage: Ref<boolean>
  /** The docs_collab_url deployment capability; empty means no
   * collaboration service is configured (T1.5's exclusive-edit path takes
   * over then -- until it lands, the editor stays view-only). */
  collabUrl: Ref<string>
  currentUser: Ref<UserLike | null>
  /** Reads the current bearer token; called fresh on every (re)connect so a
   * token refresh during a long session is picked up automatically. */
  getToken: () => string | null
}

/** Bumped into the DB name so a schema-breaking change does not try to
 * resurrect updates written under an incompatible document model. */
const INDEXEDDB_SCHEMA_VERSION = 1

/** How often to re-derive the online-users list as a safety net alongside
 * the awareness `change` listener (covers any missed event ordering). */
const AWARENESS_POLL_MS = 5000

export function useDocCollab(opts: DocCollabOptions): DocCollabHandle {
  const ydoc = shallowRef(new Y.Doc())
  const provider = shallowRef<HocuspocusProvider | null>(null)
  const status = ref<ConnectionStatus>('connecting')
  const collabAccess = ref<CollabAccess>('unknown')
  const onlineUsers = ref<AwarenessUser[]>([])
  const ready = ref(false)

  let indexeddb: IndexeddbPersistence | null = null
  let awarenessPollTimer: ReturnType<typeof setInterval> | null = null

  const editable = computed(() => isEditable({
    canEditPage: opts.canEditPage.value,
    connectionStatus: status.value,
    collabAccess: collabAccess.value,
  }))
  const banner = computed<Banner>(() => {
    if (!opts.collabUrl.value) return { kind: 'unavailable' }
    return deriveBanner({
      canEditPage: opts.canEditPage.value,
      connectionStatus: status.value,
      collabAccess: collabAccess.value,
    })
  })

  function teardown() {
    if (awarenessPollTimer) {
      clearInterval(awarenessPollTimer)
      awarenessPollTimer = null
    }
    provider.value?.destroy()
    provider.value = null
    indexeddb?.destroy().catch(() => undefined)
    indexeddb = null
    ydoc.value.destroy()
    status.value = 'connecting'
    collabAccess.value = 'unknown'
    onlineUsers.value = []
    ready.value = false
  }

  function connect(pageId: string, tenantId: string) {
    teardown()
    const doc = new Y.Doc()
    ydoc.value = doc

    indexeddb = new IndexeddbPersistence(`yuheng-docs-v${INDEXEDDB_SCHEMA_VERSION}-${pageId}`, doc)
    indexeddb.whenSynced.then(() => {
      ready.value = true
    }).catch(() => {
      ready.value = true // the local cache is a nice-to-have, not a blocker
    })

    if (!opts.collabUrl.value) {
      // No collaboration service configured (Lite / exclusive-edit, T1.5).
      // The Y.Doc above still exists so a future provider can attach to
      // it, but there is nothing to connect to yet.
      status.value = 'disconnected'
      return
    }

    const url = `${opts.collabUrl.value}?tenant=${encodeURIComponent(tenantId)}`
    const p = new HocuspocusProvider({
      url,
      name: pageId,
      document: doc,
      token: () => opts.getToken() ?? '',
      onStatus: ({ status: s }) => {
        status.value = s
      },
      onAuthenticated: ({ scope }) => {
        collabAccess.value = scope
      },
      onAuthenticationFailed: () => {
        collabAccess.value = 'readonly'
      },
      onSynced: () => {
        ready.value = true
      },
      onStateless: ({ payload }) => {
        try {
          const msg = JSON.parse(payload) as { type?: string; access?: string }
          if (msg.type === 'yuheng.access' && msg.access === 'readonly') {
            collabAccess.value = 'readonly'
          }
        } catch {
          // Not one of ours; ignore.
        }
      },
    })

    const user = opts.currentUser.value
    if (user) p.setAwarenessField('user', awarenessUser(user))

    const refreshOnlineUsers = () => {
      const states = [...p.awareness!.getStates().entries()].map(([clientId, state]) => ({
        clientId, user: state.user as Partial<AwarenessUser> | undefined,
      }))
      onlineUsers.value = dedupeOnlineUsers(states)
    }
    p.awareness?.on('change', refreshOnlineUsers)
    // The 'change' listener can miss peers already online at the moment it
    // is attached; a light poll catches that without reasoning about event
    // ordering across the reconnect.
    awarenessPollTimer = setInterval(refreshOnlineUsers, AWARENESS_POLL_MS)
    refreshOnlineUsers()

    provider.value = p
  }

  // collabUrl is watched too: GET /system/capabilities resolves after mount,
  // so the first evaluation usually sees an empty URL and must reconnect once
  // the real address arrives.
  watch([opts.pageId, opts.tenantId, opts.collabUrl], ([pageId, tenantId]) => {
    if (!pageId || tenantId === undefined || tenantId === '') {
      teardown()
      return
    }
    connect(pageId, String(tenantId))
  }, { immediate: true })

  onBeforeUnmount(teardown)

  return { ydoc, provider, status, collabAccess, editable, banner, onlineUsers, ready }
}
