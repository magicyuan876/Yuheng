// Per-document bookkeeping the extensions share: which tenant the page
// belongs to, the version the Go server last acknowledged, who edited since
// the last store, and the retry state of a failed store.

export interface DocState {
  pageId: string
  tenantId: string
  /** Last ydoc_version acknowledged by the Go server. */
  version: number
  /** User ids that produced updates since the last successful store. */
  editors: Set<string>
  /** True when the in-memory state differs from what was last stored. */
  dirty: boolean
  /** Consecutive store failures (drives the backoff). */
  failures: number
  retryTimer: NodeJS.Timeout | null
  /** Set when the Go server rejected the page for good (deleted, invalid). */
  rejected: string | null
  /** Serialises store attempts for one document. */
  chain: Promise<void>
}

export class DocumentRegistry {
  private readonly docs = new Map<string, DocState>()

  /** Returns the state, creating it on first sight. */
  ensure(pageId: string, tenantId: string): DocState {
    let st = this.docs.get(pageId)
    if (!st) {
      st = {
        pageId, tenantId, version: 0, editors: new Set(), dirty: false, failures: 0,
        retryTimer: null, rejected: null, chain: Promise.resolve(),
      }
      this.docs.set(pageId, st)
    } else if (tenantId && !st.tenantId) {
      st.tenantId = tenantId
    }
    return st
  }

  get(pageId: string): DocState | undefined {
    return this.docs.get(pageId)
  }

  forget(pageId: string): void {
    const st = this.docs.get(pageId)
    if (st?.retryTimer) clearTimeout(st.retryTimer)
    this.docs.delete(pageId)
  }

  /** Every document with unstored changes. */
  dirtyIds(): string[] {
    return [...this.docs.values()].filter((s) => s.dirty && !s.rejected).map((s) => s.pageId)
  }

  size(): number {
    return this.docs.size
  }
}

/** Connection context set by the authenticate extension. */
export interface ConnectionContext {
  user: { id: string; name: string; avatar: string }
  tenantId: string
  spaceId: string
  pageId: string
  access: 'readwrite' | 'readonly'
  token: string
  authenticatedAt: number
}

/** Context used by server-internal direct connections (replace). */
export interface SystemContext {
  system: true
  tenantId: string
  pageId: string
}

export type Context = ConnectionContext | SystemContext

export function isSystem(ctx: unknown): ctx is SystemContext {
  return !!ctx && typeof ctx === 'object' && (ctx as SystemContext).system === true
}

export function tenantOf(ctx: unknown): string {
  if (!ctx || typeof ctx !== 'object') return ''
  return String((ctx as { tenantId?: string }).tenantId ?? '')
}
