// Plain HTTP endpoints served next to the WebSocket: health, metrics and the
// two signed calls the Go server makes (replace a page body, evict a page).

import type { Extension, Hocuspocus, onRequestPayload } from '@hocuspocus/server'
import type { IncomingMessage, ServerResponse } from 'node:http'

import type { BackendClient } from './backend.js'
import type { CollabConfig } from './config.js'
import type { DocumentRegistry, SystemContext } from './documents.js'
import type { Logger } from './log.js'
import { M, type Metrics } from './metrics.js'
import { FRAGMENT, replaceFragment } from './pm-schema.js'
import { SIGNATURE_HEADER, TIMESTAMP_HEADER, verify } from './signing.js'
import type { AuthCache } from './extensions/authenticate.js'
import type { PersistenceExtension } from './extensions/persistence.js'

export interface HttpDeps {
  cfg: CollabConfig
  backend: BackendClient
  registry: DocumentRegistry
  metrics: Metrics
  persistence: PersistenceExtension
  authCache: AuthCache
  log: Logger
  version: string
}

const MAX_BODY = 64 * 1024 * 1024

/** Signals to Hocuspocus that the response was written. */
const HANDLED = null

function sendJSON(res: ServerResponse, status: number, body: unknown): void {
  const data = JSON.stringify(body)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(data) })
  res.end(data)
}

function readBody(req: IncomingMessage, limit = MAX_BODY): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = []
    let size = 0
    req.on('data', (c: Buffer) => {
      size += c.length
      if (size > limit) {
        reject(new Error('body too large'))
        req.destroy()
        return
      }
      chunks.push(c)
    })
    req.on('end', () => resolve(Buffer.concat(chunks)))
    req.on('error', reject)
  })
}

export class HttpExtension implements Extension {
  extensionName = 'yuheng-http'
  private instance: Hocuspocus | null = null

  constructor(private readonly d: HttpDeps) {}

  async onConfigure(data: { instance: Hocuspocus }): Promise<void> {
    this.instance = data.instance
  }

  async onRequest(data: onRequestPayload): Promise<void> {
    const { request, response } = data
    const url = new URL(request.url ?? '/', 'http://collab.local')
    const method = (request.method ?? 'GET').toUpperCase()
    try {
      if (method === 'GET' && url.pathname === '/healthz') return await this.health(response)
      if (method === 'GET' && url.pathname === '/metrics') return this.metricsPage(response)
      if (method === 'GET' && url.pathname === '/') {
        sendJSON(response, 200, { service: 'yuheng-collab', version: this.d.version })
        throw HANDLED
      }
      const replace = url.pathname.match(/^\/internal\/collab\/replace\/([^/]+)$/)
      if (method === 'POST' && replace) {
        return await this.replace(request, response, url, decodeURIComponent(replace[1]))
      }
      const evict = url.pathname.match(/^\/internal\/collab\/([^/]+)$/)
      if (method === 'DELETE' && evict) {
        return await this.evict(request, response, url, decodeURIComponent(evict[1]))
      }
      sendJSON(response, 404, { error: 'not found' })
      throw HANDLED
    } catch (err) {
      if (err === HANDLED) throw HANDLED
      this.d.log.error('http handler failed', { path: url.pathname, error: (err as Error).message })
      if (!response.headersSent) sendJSON(response, 500, { error: 'internal error' })
      throw HANDLED
    }
  }

  private async health(res: ServerResponse): Promise<never> {
    const backendOk = await this.d.backend.health()
    sendJSON(res, backendOk ? 200 : 503, {
      status: backendOk ? 'ok' : 'degraded',
      backend: backendOk ? 'ok' : 'unreachable',
      connections: this.instance?.getConnectionsCount() ?? 0,
      documents: this.instance?.getDocumentsCount() ?? 0,
      dirty_documents: this.d.registry.dirtyIds().length,
      version: this.d.version,
    })
    throw HANDLED
  }

  private metricsPage(res: ServerResponse): never {
    const text = this.d.metrics.render({
      collab_connections: this.instance?.getConnectionsCount() ?? 0,
      collab_documents: this.instance?.getDocumentsCount() ?? 0,
      collab_dirty_documents: this.d.registry.dirtyIds().length,
    })
    res.writeHead(200, { 'content-type': 'text/plain; version=0.0.4; charset=utf-8' })
    res.end(text)
    throw HANDLED
  }

  /** Checks the HMAC of a Go-originated request; writes 401 and returns null on failure. */
  private async authorised(req: IncomingMessage, res: ServerResponse, url: URL): Promise<Buffer | null> {
    const body = await readBody(req)
    const result = verify(this.d.cfg.sharedSecret, req.method ?? 'GET', url.pathname + url.search,
      header(req, TIMESTAMP_HEADER), header(req, SIGNATURE_HEADER), body)
    if (!result.ok) {
      this.d.log.warn('rejected unsigned internal request', { path: url.pathname, reason: result.reason })
      sendJSON(res, 401, { error: `unauthorized: ${result.reason}` })
      return null
    }
    return body
  }

  /**
   * POST /internal/collab/replace/{page_id}: applies a JSON body to the live
   * document as one Yjs transaction, stores it, and answers the new version.
   */
  private async replace(req: IncomingMessage, res: ServerResponse, url: URL, pageId: string): Promise<never> {
    const body = await this.authorised(req, res, url)
    if (!body) throw HANDLED
    let parsed: { tenant_id?: string; content?: unknown; reason?: string }
    try {
      parsed = JSON.parse(body.toString('utf8'))
    } catch {
      sendJSON(res, 400, { error: 'body must be JSON' })
      throw HANDLED
    }
    const tenantId = String(parsed.tenant_id ?? '')
    if (!tenantId || !parsed.content || typeof parsed.content !== 'object') {
      sendJSON(res, 400, { error: 'tenant_id and content are required' })
      throw HANDLED
    }
    if (!this.instance) {
      sendJSON(res, 503, { error: 'not ready' })
      throw HANDLED
    }
    const ctx: SystemContext = { system: true, tenantId, pageId }
    const direct = await this.instance.openDirectConnection(pageId, ctx)
    try {
      try {
        await direct.transact((document) => {
          replaceFragment(document.getXmlFragment(FRAGMENT), parsed.content)
        })
      } catch (err) {
        sendJSON(res, 422, { error: `content rejected: ${(err as Error).message}` })
        throw HANDLED
      }
      const st = this.d.registry.ensure(pageId, tenantId)
      st.dirty = true
      try {
        await this.d.persistence.persistNow(pageId)
      } catch (err) {
        sendJSON(res, 503, { error: `stored later: ${(err as Error).message}`, ydoc_version: st.version })
        throw HANDLED
      }
      this.d.metrics.inc(M.replaceApplied)
      this.d.log.info('replaced page content', { pageId, reason: parsed.reason ?? '', version: st.version })
      sendJSON(res, 200, { ydoc_version: st.version })
      throw HANDLED
    } finally {
      await direct.disconnect({ unloadImmediately: false }).catch(() => undefined)
    }
  }

  /** DELETE /internal/collab/{page_id}: stores, drops every connection and forgets the page. */
  private async evict(req: IncomingMessage, res: ServerResponse, url: URL, pageId: string): Promise<never> {
    const body = await this.authorised(req, res, url)
    if (!body) throw HANDLED
    this.d.authCache.invalidatePage(pageId)
    const document = this.instance?.documents.get(pageId)
    if (document) {
      await this.d.persistence.persistNow(pageId).catch(() => undefined)
      this.instance?.closeConnections(pageId)
      const st = this.d.registry.get(pageId)
      if (st && !st.dirty) {
        // Nothing left to store: drop it from memory so the next open reloads.
        await this.instance?.unloadDocument(document).catch(() => undefined)
      }
    }
    this.d.metrics.inc(M.evicted)
    this.d.log.info('evicted page', { pageId, loaded: !!document })
    res.writeHead(204)
    res.end()
    throw HANDLED
  }
}

function header(req: IncomingMessage, name: string): string | undefined {
  const v = req.headers[name]
  return Array.isArray(v) ? v[0] : v
}
