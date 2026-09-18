// Connection guard: counts writes that read-only connections attempt (Hocuspocus
// already ignores them), rate-limits sync updates per connection, and
// re-verifies each connection's permission with the Go server periodically,
// downgrading or closing it when access was narrowed.

import { Forbidden } from '@hocuspocus/common'
import { MessageType, type Connection, type Extension, type beforeHandleMessagePayload, type connectedPayload }
  from '@hocuspocus/server'
import { createDecoder, readVarString, readVarUint } from 'lib0/decoding'

import { BackendClient, BackendUnavailable } from '../backend.js'
import type { CollabConfig } from '../config.js'
import type { ConnectionContext } from '../documents.js'
import type { Logger } from '../log.js'
import { M, type Metrics } from '../metrics.js'
import { STATELESS } from './persistence.js'

/** y-protocols sync message sub-types. */
const SYNC_STEP2 = 1
const SYNC_UPDATE = 2

interface Tracked {
  timer: NodeJS.Timeout
  windowStart: number
  updates: number
}

/** Classifies an inbound Hocuspocus message; exported for tests. */
export function classifyMessage(update: Uint8Array): { type: number; syncType: number | null } {
  const decoder = createDecoder(update)
  readVarString(decoder) // document name (or name\0session)
  const type = readVarUint(decoder)
  if (type !== MessageType.Sync && type !== MessageType.SyncReply) return { type, syncType: null }
  return { type, syncType: readVarUint(decoder) }
}

export class GuardExtension implements Extension {
  extensionName = 'yuheng-guard'
  priority = 800
  private readonly tracked = new Map<Connection, Tracked>()

  constructor(
    private readonly backend: BackendClient,
    private readonly cfg: CollabConfig,
    private readonly metrics: Metrics,
    private readonly log: Logger,
  ) {}

  async connected(data: connectedPayload<ConnectionContext>): Promise<void> {
    const { connection } = data
    const timer = setInterval(() => void this.recheck(connection, data.documentName), this.cfg.recheckIntervalMs)
    this.tracked.set(connection, { timer, windowStart: Date.now(), updates: 0 })
    connection.onClose(() => {
      const t = this.tracked.get(connection)
      if (t) clearInterval(t.timer)
      this.tracked.delete(connection)
    })
  }

  async beforeHandleMessage(data: beforeHandleMessagePayload<ConnectionContext>): Promise<void> {
    let kind: { type: number; syncType: number | null }
    try {
      kind = classifyMessage(data.update)
    } catch {
      return // malformed frames are Hocuspocus' problem
    }
    const isWrite = kind.syncType === SYNC_UPDATE || kind.syncType === SYNC_STEP2
    if (!isWrite) return
    if (data.connection.readOnly) {
      this.metrics.inc(M.readonlyDropped)
      return
    }
    const t = this.tracked.get(data.connection)
    if (!t) return
    const now = Date.now()
    if (now - t.windowStart >= 1000) {
      t.windowStart = now
      t.updates = 0
    }
    t.updates += 1
    if (t.updates > this.cfg.updatesPerSecond) {
      this.metrics.inc(M.rateLimited)
      this.log.warn('connection exceeded the update rate limit; closing', {
        pageId: data.documentName, user: data.context?.user?.id, perSecond: t.updates,
      })
      throw Forbidden
    }
  }

  /** Asks the Go server again whether this connection may still edit / read. */
  private async recheck(connection: Connection<ConnectionContext>, pageId: string): Promise<void> {
    const ctx = connection.context
    if (!ctx?.token) return
    try {
      const result = await this.backend.authenticate({
        token: ctx.token, pageId, tenantId: ctx.tenantId, connectionId: connection.socketId,
      })
      if (result.access === 'readonly' && !connection.readOnly) {
        connection.readOnly = true
        ctx.access = 'readonly'
        this.metrics.inc(M.recheckDowngraded)
        connection.sendStateless(JSON.stringify({ type: STATELESS.access, access: 'readonly' }))
        this.log.info('connection downgraded to read-only', { pageId, user: ctx.user.id })
      }
    } catch (err) {
      if (err instanceof BackendUnavailable) return // keep the session; decide when the server is back
      this.metrics.inc(M.recheckClosed)
      this.log.info('connection lost its permission; closing', { pageId, user: ctx.user.id })
      connection.close(Forbidden)
    }
  }
}
