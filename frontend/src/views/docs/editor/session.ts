// Pure decision logic for the collaborative editing session: whether the
// editor should currently accept input, what banner (if any) to show, and
// how to present a collaborator in the awareness/cursor UI. Kept free of
// Vue, Yjs and any network object so it is exercised directly by tests;
// `useDocCollab.ts` is the thin composable that wires this to the real
// Y.Doc/HocuspocusProvider and Vue refs.

/** Mirrors HocuspocusProviderWebsocket's WebSocketStatus values by name. */
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

/**
 * What the collaboration server has told this connection it may do.
 * `unknown` covers the gap between opening the socket and the first
 * `authenticated`/`yuheng.access` message; treated the same as `readonly`
 * by `isEditable` so nothing is briefly, wrongly editable while unproven.
 */
export type CollabAccess = 'read-write' | 'readonly' | 'unknown'

export interface EditableInput {
  /** From the page's REST payload (`can_edit`): the ACL resolver's answer,
   * fetched before the collaboration socket even opens. */
  canEditPage: boolean
  connectionStatus: ConnectionStatus
  /** From the collaboration connection itself: its `onAuthenticated` scope
   * at first connect, then narrowed live if GuardExtension's periodic
   * recheck downgrades it (`yuheng.access`). Never widens without a fresh
   * connection. */
  collabAccess: CollabAccess
}

/**
 * Whether the editor may currently be typed into. Every gate is
 * independent and all must hold:
 *
 *   - the server's ACL resolver said this user can write the page at all;
 *   - the collaboration connection has confirmed the same (or has not yet
 *     said otherwise -- `unknown` is treated as not-yet-writable, never as
 *     writable, to avoid a moment of false confidence between REST saying
 *     yes and the socket's own answer arriving);
 *   - the socket is actually connected -- editing while `connecting` or
 *     `disconnected` is deliberately refused (design decision: a dropped
 *     connection shows a protection banner rather than silently letting
 *     someone keep typing into a document that is not syncing). Nothing
 *     already typed is at risk either way: Yjs keeps every update in the
 *     local doc (and, once wired, y-indexeddb) regardless of this flag, and
 *     a reconnect re-syncs it.
 */
export function isEditable(input: EditableInput): boolean {
  return input.canEditPage && input.collabAccess === 'read-write' && input.connectionStatus === 'connected'
}

export type Banner =
  | { kind: 'none' }
  | { kind: 'connecting' }
  | { kind: 'offline' }
  | { kind: 'read-only' }
  | { kind: 'permission-narrowed' }
  /** No collaboration service is configured for this deployment at all
   * (Lite / exclusive-edit, T1.5); distinct from `offline` so the banner
   * does not read as a transient, about-to-reconnect problem. */
  | { kind: 'unavailable' }

/**
 * What to tell the user about why they cannot (or temporarily cannot) edit.
 * Order matters: a live narrowing after a successful read-write connection
 * is the most specific, actionable fact and takes priority over the plain
 * connection state; a page the REST call already marked read-only is
 * reported as such rather than blamed on the connection.
 */
export function deriveBanner(input: EditableInput): Banner {
  if (input.canEditPage && input.collabAccess === 'readonly') return { kind: 'permission-narrowed' }
  if (!input.canEditPage) return { kind: 'read-only' }
  if (input.connectionStatus === 'connecting') return { kind: 'connecting' }
  if (input.connectionStatus === 'disconnected') return { kind: 'offline' }
  return { kind: 'none' }
}

export interface AwarenessUser {
  id: string
  name: string
  avatar: string
  color: string
}

export interface UserLike {
  id: string
  username?: string
  email?: string
  avatar?: string
}

/** The `user` awareness field this client publishes about itself, and the
 * shape read back from every other connected client's awareness state. */
export function awarenessUser(user: UserLike): AwarenessUser {
  return {
    id: user.id,
    name: user.username?.trim() || user.email?.trim() || user.id,
    avatar: user.avatar ?? '',
    color: colorForId(user.id),
  }
}

/**
 * A stable colour per user id, so the same person's cursor and avatar look
 * the same across tabs, sessions and reconnects without the server having
 * to hand one out. Not cryptographic -- just a cheap, deterministic spread
 * across the hue wheel.
 */
export function colorForId(id: string): string {
  let hash = 0
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.codePointAt(i)!) >>> 0
  }
  const hue = hash % 360
  return `hsl(${hue}, 65%, 45%)`
}

/**
 * Deduplicates the raw list `editor.storage.collaborationCaret.users`
 * exposes (one entry per Yjs clientId -- the same person open in two tabs
 * produces two entries) down to one row per user id, for an "N people
 * online" avatar strip. Falls back to the clientId when a peer's awareness
 * state has not carried a `user` field yet (the instant after connecting).
 */
export function dedupeOnlineUsers(
  raw: readonly { clientId: number; user?: Partial<AwarenessUser> }[],
): AwarenessUser[] {
  const seen = new Map<string, AwarenessUser>()
  for (const entry of raw) {
    const id = entry.user?.id ?? `peer-${entry.clientId}`
    if (seen.has(id)) continue
    seen.set(id, {
      id,
      name: entry.user?.name || id,
      avatar: entry.user?.avatar ?? '',
      color: entry.user?.color ?? colorForId(id),
    })
  }
  return [...seen.values()]
}
