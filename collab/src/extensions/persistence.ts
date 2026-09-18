// Loading and storing documents through the Go server.
//
// Load: the Go server returns either the Yjs state (the truth source) or,
// for a page that has never been edited collaboratively, its JSON body; the
// latter is converted and stored right away so the page has a ydoc from then
// on.
//
// Store: Hocuspocus debounces `onStoreDocument`; here the state is encoded
// (Yjs GC keeps it compact), projected to ProseMirror JSON and sent with the
// last acknowledged version. A 409 means another writer persisted first: the
// stored state is merged into the live document (Yjs merges are idempotent)
// and the store is retried. When the Go server is down the document stays
// in memory, dirty, and a backoff timer retries until it succeeds; shutdown
// flushes everything that is still pending.

import type { Extension, Hocuspocus, Document as HDocument, beforeUnloadDocumentPayload, onChangePayload,
  onLoadDocumentPayload, onStoreDocumentPayload, afterUnloadDocumentPayload } from '@hocuspocus/server'
import * as Y from 'yjs'

import { BackendClient, BackendError, BackendUnavailable, ConflictError } from '../backend.js'
import type { CollabConfig } from '../config.js'
import { DocumentRegistry, isSystem, tenantOf, type ConnectionContext } from '../documents.js'
import type { Logger } from '../log.js'
import { M, type Metrics } from '../metrics.js'
import { EMPTY_DOCUMENT, FRAGMENT, jsonToYDoc, yDocToJSON } from '../pm-schema.js'

/** Origin of updates this extension applies itself (never re-stored by themselves). */
export const LOCAL_LOAD_ORIGIN = { source: 'local' as const, skipStoreHooks: true }

/** Stateless payloads sent to clients (JSON strings). */
export const STATELESS = {
  saved: 'yuheng.saved',
  error: 'yuheng.error',
  access: 'yuheng.access',
} as const

export class PersistenceExtension implements Extension {
  extensionName = 'yuheng-persistence'
  priority = 500
  instance: Hocuspocus | null = null
  /** Set while shutting down: unloads are no longer blocked. */
  private draining = false

  constructor(
    private readonly backend: BackendClient,
    private readonly registry: DocumentRegistry,
    private readonly cfg: CollabConfig,
    private readonly metrics: Metrics,
    private readonly log: Logger,
  ) {}

  async onConfigure(data: { instance: Hocuspocus }): Promise<void> {
    this.instance = data.instance
  }

  async onLoadDocument(data: onLoadDocumentPayload): Promise<void> {
    const { document, documentName: pageId, context } = data
    const tenantId = tenantOf(context)
    const st = this.registry.ensure(pageId, tenantId)
    if (!document.isEmpty(FRAGMENT)) return // already synced from a peer instance
    const loaded = await this.backend.load(st.tenantId, pageId)
    st.version = loaded.version
    if (loaded.kind === 'ydoc' && loaded.state.byteLength > 0) {
      Y.applyUpdate(document, loaded.state, LOCAL_LOAD_ORIGIN)
      this.log.debug('loaded ydoc', { pageId, version: st.version, bytes: loaded.state.byteLength })
      return
    }
    // First collaborative open: materialise the JSON body as a Yjs state and
    // persist it immediately so the page has a ydoc from now on.
    const source = loaded.kind === 'json' && loaded.content ? loaded.content : EMPTY_DOCUMENT
    let seed: Y.Doc
    try {
      seed = jsonToYDoc(source)
    } catch (err) {
      this.log.warn('stored content is not a valid document; starting empty', { pageId, error: (err as Error).message })
      seed = jsonToYDoc(EMPTY_DOCUMENT)
    }
    Y.applyUpdate(document, Y.encodeStateAsUpdate(seed), LOCAL_LOAD_ORIGIN)
    seed.destroy()
    st.dirty = true
    this.log.info('materialised ydoc from json', { pageId })
    void this.persist(pageId, document, 'initial')
  }

  async onChange(data: onChangePayload): Promise<void> {
    const st = this.registry.get(data.documentName)
    if (!st) return
    st.dirty = true
    const ctx = data.context as ConnectionContext | undefined
    if (ctx && !isSystem(ctx) && ctx.user?.id) st.editors.add(ctx.user.id)
  }

  async onStoreDocument(data: onStoreDocumentPayload): Promise<void> {
    await this.persist(data.documentName, data.document, 'debounce')
  }

  /**
   * Blocks unloading only while a retry is actually scheduled, so a document
   * that can never be stored (over the size limit, refused by the server)
   * cannot pin memory or stall shutdown. The client keeps its own copy and
   * re-sends it on the next connection.
   */
  async beforeUnloadDocument(data: beforeUnloadDocumentPayload): Promise<void> {
    if (this.draining) return
    const st = this.registry.get(data.documentName)
    if (st && st.retryTimer !== null) {
      throw new Error('a store is scheduled for this document')
    }
  }

  async afterUnloadDocument(data: afterUnloadDocumentPayload): Promise<void> {
    this.registry.forget(data.documentName)
  }

  /** Stops blocking unloads; called when the process is shutting down. */
  drain(): void {
    this.draining = true
  }

  /** Stores one document now (used by replace, evict and shutdown). */
  persistNow(pageId: string): Promise<void> {
    const document = this.instance?.documents.get(pageId)
    if (!document) return Promise.resolve()
    return this.persist(pageId, document, 'explicit')
  }

  /** Stores every dirty document; resolves when all attempts finished. */
  async flushAll(): Promise<{ stored: number; failed: number }> {
    let stored = 0
    let failed = 0
    await Promise.all(this.registry.dirtyIds().map(async (id) => {
      try {
        await this.persistNow(id)
        if (this.registry.get(id)?.dirty) failed++
        else stored++
      } catch {
        failed++
      }
    }))
    return { stored, failed }
  }

  /**
   * One store attempt chain for a document. Attempts are serialised per
   * document; a conflict reloads and retries inline; an outage schedules a
   * retry and rethrows so Hocuspocus keeps the document in memory.
   */
  private persist(pageId: string, document: HDocument, reason: string): Promise<void> {
    const st = this.registry.ensure(pageId, '')
    const run = st.chain.then(() => this.attempt(pageId, document, st, reason))
    // Keep the chain alive even when an attempt rejects.
    st.chain = run.catch(() => undefined)
    return run
  }

  private async attempt(pageId: string, document: HDocument, st: ReturnType<DocumentRegistry['ensure']>,
    reason: string): Promise<void> {
    if (!st.dirty || st.rejected) return
    if (st.retryTimer) {
      clearTimeout(st.retryTimer)
      st.retryTimer = null
    }
    for (let round = 0; round < 3; round++) {
      const state = Y.encodeStateAsUpdate(document)
      if (state.byteLength > this.cfg.maxYDocBytes) {
        this.metrics.inc(M.storeTooLarge)
        this.log.warn('ydoc exceeds the size limit; not stored', { pageId, bytes: state.byteLength })
        document.broadcastStateless(JSON.stringify({
          type: STATELESS.error, code: 'ydoc_too_large', bytes: state.byteLength, limit: this.cfg.maxYDocBytes,
        }))
        return // stays dirty; the next change re-checks
      }
      const content = yDocToJSON(document)
      const editors = [...st.editors]
      const started = performance.now()
      try {
        const version = await this.backend.store({
          tenantId: st.tenantId, pageId, baseVersion: st.version, state, content, editorIds: editors,
          awarenessCount: document.awareness.getStates().size,
        })
        this.metrics.observeStoreSeconds((performance.now() - started) / 1000)
        this.metrics.inc(M.storeOk)
        st.version = version
        st.failures = 0
        for (const e of editors) st.editors.delete(e)
        // Changes that arrived while the request was in flight keep dirty=true.
        st.dirty = Y.encodeStateAsUpdate(document).byteLength !== state.byteLength ||
          !equalBytes(Y.encodeStateAsUpdate(document), state)
        document.broadcastStateless(JSON.stringify({ type: STATELESS.saved, version, at: new Date().toISOString() }))
        this.log.debug('stored', { pageId, version, reason, bytes: state.byteLength })
        if (reason === 'retry' && !st.dirty && document.getConnectionsCount() === 0) {
          // Nobody is left on a document we only kept around to store it.
          void this.instance?.unloadDocument(document).catch(() => undefined)
        }
        return
      } catch (err) {
        if (err instanceof ConflictError) {
          this.metrics.inc(M.storeConflict)
          this.log.info('store conflict; merging the stored state and retrying', { pageId, base: st.version,
            current: err.currentVersion })
          const loaded = await this.backend.load(st.tenantId, pageId)
          if (loaded.kind === 'ydoc' && loaded.state.byteLength > 0) {
            Y.applyUpdate(document, loaded.state, LOCAL_LOAD_ORIGIN)
          }
          st.version = loaded.version
          continue
        }
        if (err instanceof BackendError) {
          this.metrics.inc(M.storeRejected)
          if (err.status === 404 || err.status === 410) {
            st.rejected = 'page gone'
            this.log.warn('page is gone; closing its connections', { pageId, status: err.status })
            this.instance?.closeConnections(pageId)
            return
          }
          if (err.status === 413) {
            this.metrics.inc(M.storeTooLarge)
          }
          this.log.error('store rejected by the backend', { pageId, status: err.status, error: err.message })
          document.broadcastStateless(JSON.stringify({ type: STATELESS.error, code: err.code || 'rejected',
            status: err.status, message: err.message }))
          return // dirty stays true; the next change retries
        }
        // Outage (network / 5xx / malformed): keep in memory, retry with backoff.
        this.metrics.inc(M.storeFailed)
        st.failures += 1
        const delay = Math.min(this.cfg.storeRetryMaxMs, this.cfg.storeRetryBaseMs * 2 ** Math.min(st.failures - 1, 10))
        this.log.warn('store failed; will retry', { pageId, attempt: st.failures, delayMs: delay,
          error: (err as Error).message })
        st.retryTimer = setTimeout(() => {
          st.retryTimer = null
          const doc = this.instance?.documents.get(pageId)
          if (doc) void this.persist(pageId, doc, 'retry').catch(() => undefined)
        }, delay)
        throw err instanceof BackendUnavailable ? err : new BackendUnavailable(String(err))
      }
    }
    this.log.error('store gave up after repeated conflicts', { pageId })
    this.metrics.inc(M.storeFailed)
    throw new BackendUnavailable('repeated version conflicts')
  }
}

function equalBytes(a: Uint8Array, b: Uint8Array): boolean {
  if (a.byteLength !== b.byteLength) return false
  for (let i = 0; i < a.byteLength; i++) if (a[i] !== b[i]) return false
  return true
}
