// Assembles the Hocuspocus server with the Yuheng extensions. `createCollabServer`
// is used by main.ts and by the integration tests (which point it at a fake Go
// server).

import { Server } from '@hocuspocus/server'
import { Redis } from 'ioredis'

import { BackendClient } from './backend.js'
import type { CollabConfig } from './config.js'
import { DocumentRegistry } from './documents.js'
import { AuthCache, AuthenticateExtension } from './extensions/authenticate.js'
import { GuardExtension } from './extensions/guard.js'
import { PersistenceExtension } from './extensions/persistence.js'
import { RedisSyncExtension, type RedisLike } from './extensions/redis-sync.js'
import { HttpExtension } from './http.js'
import type { Logger } from './log.js'
import { Metrics } from './metrics.js'

export const VERSION = '0.1.0'

export interface CollabServer {
  server: Server
  backend: BackendClient
  registry: DocumentRegistry
  metrics: Metrics
  persistence: PersistenceExtension
  authCache: AuthCache
  redis: RedisSyncExtension | null
  /** Starts listening; resolves with the bound port. */
  start(): Promise<number>
  /** Flushes pending stores and shuts down. */
  stop(): Promise<void>
}

export interface CreateOptions {
  /** Overrides the Redis client factory (tests). */
  createRedis?: () => RedisLike
  instanceId?: string
}

export function createCollabServer(cfg: CollabConfig, log: Logger, opts: CreateOptions = {}): CollabServer {
  const metrics = new Metrics()
  const backend = new BackendClient({ baseUrl: cfg.backendUrl, secret: cfg.sharedSecret, timeoutMs: cfg.backendTimeoutMs })
  const registry = new DocumentRegistry()
  const authCache = new AuthCache(cfg.authCacheMs)
  const authenticate = new AuthenticateExtension(backend, authCache, metrics, log.child({ ext: 'auth' }))
  const guard = new GuardExtension(backend, cfg, metrics, log.child({ ext: 'guard' }))
  const persistence = new PersistenceExtension(backend, registry, cfg, metrics, log.child({ ext: 'persist' }))
  const http = new HttpExtension({ cfg, backend, registry, metrics, persistence, authCache, log: log.child({ ext: 'http' }),
    version: VERSION })

  let redis: RedisSyncExtension | null = null
  if (cfg.redisUrl || opts.createRedis) {
    const createClient = opts.createRedis ?? (() => new Redis(cfg.redisUrl, { lazyConnect: false, maxRetriesPerRequest: 3 }) as unknown as RedisLike)
    redis = new RedisSyncExtension({ createClient, instanceId: opts.instanceId }, metrics, log.child({ ext: 'redis' }))
  }

  const server = new Server({
    port: cfg.port,
    address: cfg.address,
    quiet: true,
    stopOnSignals: false,
    debounce: cfg.storeDebounceMs,
    maxDebounce: cfg.storeMaxWaitMs,
    unloadImmediately: false,
    timeout: 30_000,
    yDocOptions: { gc: true, gcFilter: () => true },
    extensions: [...(redis ? [redis] : []), authenticate, guard, persistence, http],
  })

  let stopped = false
  return {
    server, backend, registry, metrics, persistence, authCache, redis,
    async start() {
      await server.listen()
      const port = server.address.port
      log.info('collab listening', { port, backend: cfg.backendUrl, redis: !!redis, version: VERSION })
      return port
    },
    async stop() {
      if (stopped) return
      stopped = true
      log.info('shutting down: flushing pending stores')
      persistence.drain()
      server.hocuspocus.closeConnections()
      const flush = persistence.flushAll()
      const timeout = new Promise<{ stored: number; failed: number }>((resolve) =>
        setTimeout(() => resolve({ stored: -1, failed: -1 }), cfg.shutdownFlushTimeoutMs))
      const result = await Promise.race([flush, timeout])
      if (result.failed !== 0) log.warn('shutdown flush incomplete', result)
      else log.info('shutdown flush done', result)
      // Hocuspocus' destroy() waits for every document to unload; unload them
      // ourselves so a document nobody closed cannot stall the shutdown.
      await Promise.all([...server.hocuspocus.documents.values()].map((doc) =>
        server.hocuspocus.unloadDocument(doc).catch(() => undefined)))
      await server.destroy()
    },
  }
}
