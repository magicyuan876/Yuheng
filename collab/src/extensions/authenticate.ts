// Authentication: every WebSocket connection names a page (the document
// name) and carries the user's Yuheng token plus the tenant id as a URL
// parameter. The Go server decides; this extension only relays and caches.

import { Unauthorized } from '@hocuspocus/common'
import type { Extension, onAuthenticatePayload } from '@hocuspocus/server'
import { createHash } from 'node:crypto'

import { BackendClient, BackendError, BackendUnavailable, type AuthenticateResult } from '../backend.js'
import type { ConnectionContext } from '../documents.js'
import type { Logger } from '../log.js'
import { M, type Metrics } from '../metrics.js'

/** Close code sent when the Go server cannot be reached: "try again later". */
export const BackendDown = { code: 1013, reason: 'backend unavailable' }

interface CacheEntry {
  result: AuthenticateResult
  expiresAt: number
}

/** Remembers recent answers so a reconnect storm does not hammer the Go server. */
export class AuthCache {
  private readonly entries = new Map<string, CacheEntry>()

  constructor(private readonly ttlMs: number) {}

  static key(token: string, tenantId: string, pageId: string): string {
    const digest = createHash('sha256').update(token).digest('hex').slice(0, 32)
    return `${digest}:${tenantId}:${pageId}`
  }

  get(key: string, now = Date.now()): AuthenticateResult | undefined {
    const e = this.entries.get(key)
    if (!e) return undefined
    if (e.expiresAt <= now) {
      this.entries.delete(key)
      return undefined
    }
    return e.result
  }

  set(key: string, result: AuthenticateResult, now = Date.now()): void {
    if (this.ttlMs <= 0) return
    this.entries.set(key, { result, expiresAt: now + this.ttlMs })
    if (this.entries.size > 10_000) this.sweep(now)
  }

  /** Drops every cached answer for a page (permission change, deletion). */
  invalidatePage(pageId: string): void {
    for (const [k] of this.entries) {
      if (k.endsWith(`:${pageId}`)) this.entries.delete(k)
    }
  }

  clear(): void {
    this.entries.clear()
  }

  private sweep(now: number): void {
    for (const [k, e] of this.entries) {
      if (e.expiresAt <= now) this.entries.delete(k)
    }
  }
}

export class AuthenticateExtension implements Extension {
  extensionName = 'yuheng-authenticate'
  priority = 900

  constructor(
    private readonly backend: BackendClient,
    private readonly cache: AuthCache,
    private readonly metrics: Metrics,
    private readonly log: Logger,
  ) {}

  async onAuthenticate(data: onAuthenticatePayload): Promise<ConnectionContext> {
    const pageId = data.documentName
    const token = (data.token ?? '').trim()
    const tenantId = (data.requestParameters.get('tenant') ?? '').trim()
    if (!token || !pageId) {
      this.metrics.inc(M.authRejected)
      throw Unauthorized
    }
    const key = AuthCache.key(token, tenantId, pageId)
    let result = this.cache.get(key)
    if (!result) {
      try {
        result = await this.backend.authenticate({ token, pageId, tenantId, connectionId: data.socketId })
        this.cache.set(key, result)
      } catch (err) {
        if (err instanceof BackendUnavailable) {
          this.metrics.inc(M.authBackendUnavailable)
          this.log.warn('authenticate: backend unavailable', { pageId, error: err.message })
          throw BackendDown
        }
        this.metrics.inc(M.authRejected)
        const status = err instanceof BackendError ? err.status : 0
        this.log.info('authenticate rejected', { pageId, status, error: (err as Error).message })
        throw Unauthorized
      }
    }
    if (result.access === 'readonly') data.connectionConfig.readOnly = true
    this.metrics.inc(M.connectionsAccepted)
    return {
      user: { id: result.userId, name: result.displayName, avatar: result.avatar },
      tenantId: result.tenantId,
      spaceId: result.spaceId,
      pageId,
      access: result.access,
      token,
      authenticatedAt: Date.now(),
    }
  }
}
