// Configuration of the collaboration service. Everything comes from the
// environment (see README.md); the two required values are the Go server's
// address and the shared secret that signs every callback in both directions.

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface CollabConfig {
  /** WebSocket / HTTP listen port; 0 lets the operating system pick one. */
  port: number
  /** Listen address (default: every interface). */
  address: string
  /** Base URL of the Yuheng Go server, e.g. http://app:8080. */
  backendUrl: string
  /** HMAC secret shared with the Go server (YUHENG_COLLAB_SHARED_SECRET). */
  sharedSecret: string
  /** Redis URL for multi-instance sync; empty runs a single instance. */
  redisUrl: string
  /** Debounce between the last edit and a store call. */
  storeDebounceMs: number
  /** Upper bound on how long a busy document goes without a store. */
  storeMaxWaitMs: number
  /** Largest Yjs state accepted for one page. */
  maxYDocBytes: number
  /** How long an authentication answer is reused for identical requests. */
  authCacheMs: number
  /** Interval at which an open connection's permission is re-verified. */
  recheckIntervalMs: number
  /** Sync updates one connection may send per second before it is closed. */
  updatesPerSecond: number
  /** Backoff bounds for retrying a failed store. */
  storeRetryBaseMs: number
  storeRetryMaxMs: number
  /** How long shutdown waits for pending stores. */
  shutdownFlushTimeoutMs: number
  /** Timeout of one callback to the Go server. */
  backendTimeoutMs: number
  logLevel: LogLevel
}

const LEVELS: LogLevel[] = ['debug', 'info', 'warn', 'error']

function int(env: NodeJS.ProcessEnv, name: string, def: number, min = 0): number {
  const raw = env[name]
  if (raw === undefined || raw.trim() === '') return def
  const n = Number(raw)
  if (!Number.isFinite(n) || n < min) {
    throw new Error(`${name}=${JSON.stringify(raw)} must be a number >= ${min}`)
  }
  return Math.floor(n)
}

/** Reads and validates the configuration; throws with a clear message. */
export function loadConfig(env: NodeJS.ProcessEnv = process.env): CollabConfig {
  const backendUrl = (env.COLLAB_BACKEND_URL ?? '').trim().replace(/\/+$/, '')
  if (!backendUrl) throw new Error('COLLAB_BACKEND_URL is required (e.g. http://app:8080)')
  if (!/^https?:\/\//.test(backendUrl)) throw new Error('COLLAB_BACKEND_URL must start with http:// or https://')
  const sharedSecret = (env.COLLAB_SHARED_SECRET ?? '').trim()
  if (sharedSecret.length < 16) {
    throw new Error('COLLAB_SHARED_SECRET is required and must be at least 16 characters')
  }
  const level = (env.COLLAB_LOG_LEVEL ?? 'info').trim().toLowerCase() as LogLevel
  if (!LEVELS.includes(level)) throw new Error(`COLLAB_LOG_LEVEL must be one of ${LEVELS.join(', ')}`)
  const debounce = int(env, 'COLLAB_STORE_DEBOUNCE_MS', 2000, 50)
  return {
    port: int(env, 'COLLAB_PORT', 1234, 0),
    address: (env.COLLAB_ADDRESS ?? '0.0.0.0').trim() || '0.0.0.0',
    backendUrl,
    sharedSecret,
    redisUrl: (env.COLLAB_REDIS_URL ?? '').trim(),
    storeDebounceMs: debounce,
    storeMaxWaitMs: Math.max(debounce, int(env, 'COLLAB_STORE_MAX_WAIT_MS', 10_000, 50)),
    maxYDocBytes: int(env, 'COLLAB_MAX_YDOC_BYTES', 20 * 1024 * 1024, 1024),
    authCacheMs: int(env, 'COLLAB_AUTH_CACHE_MS', 30_000),
    recheckIntervalMs: int(env, 'COLLAB_RECHECK_INTERVAL_MS', 5 * 60_000, 1000),
    updatesPerSecond: int(env, 'COLLAB_UPDATES_PER_SECOND', 200, 1),
    storeRetryBaseMs: int(env, 'COLLAB_STORE_RETRY_BASE_MS', 1000, 10),
    storeRetryMaxMs: int(env, 'COLLAB_STORE_RETRY_MAX_MS', 30_000, 10),
    shutdownFlushTimeoutMs: int(env, 'COLLAB_SHUTDOWN_FLUSH_TIMEOUT_MS', 15_000, 100),
    backendTimeoutMs: int(env, 'COLLAB_BACKEND_TIMEOUT_MS', 10_000, 100),
    logLevel: level,
  }
}
