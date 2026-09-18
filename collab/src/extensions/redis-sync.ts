// Multi-instance sync over Redis pub/sub.
//
// Each instance relays the Yjs updates and awareness changes of the documents
// it holds to a per-document channel and applies what peers publish. When an
// instance loads a document that a peer already holds (with changes not yet
// persisted), it asks for the peer's state with its own state vector and
// waits briefly for the diff, so a new reader never sees a stale page.
//
// Frames are encoded with lib0 (varuint kind, varstring instance, varstring
// document, varuint8array payload); no serialisation library is needed.

import type { Document as HDocument, Extension, afterLoadDocumentPayload, afterUnloadDocumentPayload,
  onAwarenessUpdatePayload, onChangePayload, Hocuspocus } from '@hocuspocus/server'
import { createDecoder, readVarString, readVarUint, readVarUint8Array } from 'lib0/decoding'
import { createEncoder, toUint8Array, writeVarString, writeVarUint, writeVarUint8Array } from 'lib0/encoding'
import { randomUUID } from 'node:crypto'
import { applyAwarenessUpdate, encodeAwarenessUpdate } from 'y-protocols/awareness'
import * as Y from 'yjs'

import type { Logger } from '../log.js'
import { M, type Metrics } from '../metrics.js'

/** Origin attached to everything applied from Redis, so it is not re-published. */
export const REDIS_ORIGIN = { source: 'redis' as const }

const enum Frame {
  Update = 1,
  Awareness = 2,
  SyncRequest = 3,
  SyncReply = 4,
}

/** The subset of ioredis this extension uses; tests inject a fake. */
export interface RedisLike {
  publish(channel: string, message: Buffer): Promise<unknown>
  subscribe(...channels: string[]): Promise<unknown>
  unsubscribe(...channels: string[]): Promise<unknown>
  on(event: 'messageBuffer', handler: (channel: Buffer, message: Buffer) => void): unknown
  quit(): Promise<unknown>
}

export interface RedisSyncOptions {
  /** Creates one connection; called twice (publisher and subscriber). */
  createClient: () => RedisLike
  prefix?: string
  /** How long a load waits for a peer's state (0 disables). */
  initialSyncTimeoutMs?: number
  instanceId?: string
}

function isRedisOrigin(origin: unknown): boolean {
  return !!origin && typeof origin === 'object' && (origin as { source?: string }).source === 'redis'
}

export class RedisSyncExtension implements Extension {
  extensionName = 'yuheng-redis-sync'
  priority = 1000
  readonly instanceId: string
  private readonly prefix: string
  private readonly initialSyncTimeoutMs: number
  private pub: RedisLike
  private sub: RedisLike
  private instance: Hocuspocus | null = null
  /**
   * Documents this instance is currently loading. Hocuspocus registers a
   * document in `instance.documents` only after every load hook resolved, so
   * a peer's reply to our own sync request would otherwise arrive with
   * nothing to apply it to.
   */
  private readonly loading = new Map<string, HDocument>()
  private readonly pendingSync = new Map<string, () => void>()
  private ready: Promise<unknown>

  constructor(opts: RedisSyncOptions, private readonly metrics: Metrics, private readonly log: Logger) {
    this.instanceId = opts.instanceId ?? randomUUID()
    this.prefix = opts.prefix ?? 'yuheng:collab'
    this.initialSyncTimeoutMs = opts.initialSyncTimeoutMs ?? 1000
    this.pub = opts.createClient()
    this.sub = opts.createClient()
    this.sub.on('messageBuffer', (channel, message) => this.onMessage(channel.toString(), message))
    this.ready = this.sub.subscribe(this.replyChannel())
  }

  private docChannel(name: string): string {
    return `${this.prefix}:doc:${name}`
  }

  private replyChannel(): string {
    return `${this.prefix}:inst:${this.instanceId}`
  }

  async onConfigure(data: { instance: Hocuspocus }): Promise<void> {
    this.instance = data.instance
  }

  async afterLoadDocument(data: afterLoadDocumentPayload): Promise<void> {
    await this.ready
    this.loading.set(data.documentName, data.document)
    try {
      await this.sub.subscribe(this.docChannel(data.documentName))
      if (this.initialSyncTimeoutMs <= 0) return
      // Ask peers that already hold this document for what we are missing.
      const sv = Y.encodeStateVector(data.document)
      const waited = new Promise<void>((resolve) => {
        const timer = setTimeout(() => {
          this.pendingSync.delete(data.documentName)
          resolve()
        }, this.initialSyncTimeoutMs)
        this.pendingSync.set(data.documentName, () => {
          clearTimeout(timer)
          this.pendingSync.delete(data.documentName)
          resolve()
        })
      })
      await this.publish(this.docChannel(data.documentName), Frame.SyncRequest, data.documentName, sv)
      await waited
    } finally {
      this.loading.delete(data.documentName)
    }
  }

  async afterUnloadDocument(data: afterUnloadDocumentPayload): Promise<void> {
    await this.sub.unsubscribe(this.docChannel(data.documentName)).catch(() => undefined)
  }

  async onChange(data: onChangePayload): Promise<void> {
    if (isRedisOrigin(data.transactionOrigin)) return
    await this.publish(this.docChannel(data.documentName), Frame.Update, data.documentName, data.update)
  }

  async onAwarenessUpdate(data: onAwarenessUpdatePayload): Promise<void> {
    if (isRedisOrigin(data.transactionOrigin)) return
    const clients = [...data.added, ...data.updated, ...data.removed]
    if (clients.length === 0) return
    const update = encodeAwarenessUpdate(data.awareness, clients)
    await this.publish(this.docChannel(data.documentName), Frame.Awareness, data.documentName, update)
  }

  async onDestroy(): Promise<void> {
    await Promise.all([this.pub.quit().catch(() => undefined), this.sub.quit().catch(() => undefined)])
  }

  private async publish(channel: string, kind: Frame, documentName: string, payload: Uint8Array): Promise<void> {
    const enc = createEncoder()
    writeVarUint(enc, kind)
    writeVarString(enc, this.instanceId)
    writeVarString(enc, documentName)
    writeVarUint8Array(enc, payload)
    try {
      await this.pub.publish(channel, Buffer.from(toUint8Array(enc)))
    } catch (err) {
      this.log.warn('redis publish failed', { channel, error: (err as Error).message })
    }
  }

  private onMessage(channel: string, message: Buffer): void {
    let kind: number
    let from: string
    let documentName: string
    let payload: Uint8Array
    try {
      const dec = createDecoder(new Uint8Array(message))
      kind = readVarUint(dec)
      from = readVarString(dec)
      documentName = readVarString(dec)
      payload = readVarUint8Array(dec)
    } catch (err) {
      this.log.warn('redis frame could not be decoded', { channel, error: (err as Error).message })
      return
    }
    if (from === this.instanceId) return
    const document = this.instance?.documents.get(documentName) ?? this.loading.get(documentName)
    this.metrics.inc(M.redisRelayed)
    switch (kind) {
      case Frame.Update:
        if (document) Y.applyUpdate(document, payload, REDIS_ORIGIN)
        break
      case Frame.Awareness:
        if (document) applyAwarenessUpdate(document.awareness, payload, REDIS_ORIGIN)
        break
      case Frame.SyncRequest:
        if (document) {
          const diff = Y.encodeStateAsUpdate(document, payload)
          void this.publish(`${this.prefix}:inst:${from}`, Frame.SyncReply, documentName, diff)
        }
        break
      case Frame.SyncReply:
        if (document) Y.applyUpdate(document, payload, REDIS_ORIGIN)
        this.pendingSync.get(documentName)?.()
        break
      default:
        break
    }
  }
}
