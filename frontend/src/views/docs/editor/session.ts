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
  /** From the editing transport itself. With a collaboration service it is
   * the `onAuthenticated` scope, narrowed live if the server's periodic
   * recheck downgrades it (`yuheng.access`). In exclusive-edit mode it is
   * whether this client currently holds the page's lease. Never widens
   * without a fresh connection. */
  collabAccess: CollabAccess
}

/** What `deriveBanner` needs on top of the editability inputs. Both extra
 * fields only ever apply in exclusive-edit mode. */
export interface BannerInput extends EditableInput {
  /** The display name of whoever else holds the page's lease; empty when
   * nobody does, or when this client holds it. */
  leaseHolder?: string
  /** True once this client lost the page mid-edit. */
  superseded?: boolean
  /** True when the server offers no editing transport at all. */
  unavailable?: boolean
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
  /** Exclusive-edit mode: somebody else holds the page's lease. Named, so
   * the reader knows who to ask rather than waiting on an anonymous lock. */
  | { kind: 'lease-held'; holder: string }
  /** Exclusive-edit mode: this client was writing and lost the page, so its
   * local document is no longer a continuation of the stored one. */
  | { kind: 'superseded' }
  /** No editing transport works here at all: no collaboration service, and
   * the server does not hand out leases either. Distinct from `offline` so
   * the banner does not read as a transient, about-to-reconnect problem. */
  | { kind: 'unavailable' }

/**
 * What to tell the user about why they cannot (or temporarily cannot) edit.
 *
 * Order matters, and it runs from the most specific, most actionable fact to
 * the most general. A page the caller was never allowed to edit is reported
 * as such rather than blamed on a lock or a connection. Losing the page
 * mid-edit comes next, because it is the only state that asks the user to do
 * something (reload). Then a named colleague holding the lease, which tells
 * them who to wait for. Only after all of that does the plain connection
 * state get a say.
 */
export function deriveBanner(input: BannerInput): Banner {
  if (!input.canEditPage) return { kind: 'read-only' }
  if (input.unavailable) return { kind: 'unavailable' }
  if (input.superseded) return { kind: 'superseded' }
  if (input.leaseHolder) return { kind: 'lease-held', holder: input.leaseHolder }
  if (input.collabAccess === 'readonly') return { kind: 'permission-narrowed' }
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
