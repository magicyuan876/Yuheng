// The Go server as seen from the collaboration service: three callbacks
// (authenticate, load, store) plus a health probe. Every call is signed.

import { signedHeaders } from './signing.js'

export type Access = 'readwrite' | 'readonly'

export interface AuthenticateRequest {
  token: string
  pageId: string
  tenantId: string
  connectionId: string
}

export interface AuthenticateResult {
  userId: string
  displayName: string
  avatar: string
  access: Access
  ydocVersion: number
  tenantId: string
  spaceId: string
}

export type LoadResult =
  | { kind: 'ydoc'; state: Uint8Array; version: number }
  | { kind: 'json'; content: unknown; version: number }

export interface StoreRequest {
  tenantId: string
  pageId: string
  baseVersion: number
  state: Uint8Array
  content: unknown
  editorIds: string[]
  awarenessCount: number
}

/** A definite answer from the Go server that is not success. */
export class BackendError extends Error {
  constructor(readonly status: number, message: string, readonly code = '') {
    super(message)
    this.name = 'BackendError'
  }
}

/** The optimistic-concurrency check failed; reload and retry. */
export class ConflictError extends BackendError {
  constructor(readonly currentVersion: number) {
    super(409, 'ydoc version conflict', 'conflict')
    this.name = 'ConflictError'
  }
}

/** The Go server could not be reached or answered 5xx; retry later. */
export class BackendUnavailable extends Error {
  constructor(message: string, readonly detail?: unknown) {
    super(message)
    this.name = 'BackendUnavailable'
  }
}

export interface BackendClientOptions {
  baseUrl: string
  secret: string
  timeoutMs: number
  fetchImpl?: typeof fetch
}

const AUTH_PATH = '/internal/collab/authenticate'
const STORE_PATH = '/internal/collab/store'
const HEALTH_PATH = '/internal/collab/health'
const loadPath = (pageId: string, tenantId: string) =>
  `/internal/collab/load/${encodeURIComponent(pageId)}?tenant=${encodeURIComponent(tenantId)}`

export class BackendClient {
  private readonly fetchImpl: typeof fetch

  constructor(private readonly opts: BackendClientOptions) {
    this.fetchImpl = opts.fetchImpl ?? fetch
  }

  async authenticate(req: AuthenticateRequest): Promise<AuthenticateResult> {
    const body = JSON.stringify({
      token: req.token, page_id: req.pageId, tenant_id: req.tenantId, connection_id: req.connectionId,
    })
    const res = await this.call('POST', AUTH_PATH, body, 'application/json')
    const data = await this.json<Record<string, unknown>>(res)
    return {
      userId: String(data.user_id ?? ''),
      displayName: String(data.display_name ?? ''),
      avatar: String(data.avatar ?? ''),
      access: data.access === 'readwrite' ? 'readwrite' : 'readonly',
      ydocVersion: Number(data.ydoc_version ?? 0),
      tenantId: String(data.tenant_id ?? req.tenantId),
      spaceId: String(data.space_id ?? ''),
    }
  }

  async load(tenantId: string, pageId: string): Promise<LoadResult> {
    const res = await this.call('GET', loadPath(pageId, tenantId), '', '')
    const version = Number(res.headers.get('x-ydoc-version') ?? '0')
    const type = res.headers.get('content-type') ?? ''
    if (type.startsWith('application/octet-stream')) {
      return { kind: 'ydoc', state: new Uint8Array(await res.arrayBuffer()), version }
    }
    const data = await this.json<{ content?: unknown; ydoc_version?: number }>(res)
    return { kind: 'json', content: data.content ?? null, version: Number(data.ydoc_version ?? version) }
  }

  /** Persists a state; resolves with the new version. */
  async store(req: StoreRequest): Promise<number> {
    const body = JSON.stringify({
      tenant_id: req.tenantId,
      page_id: req.pageId,
      base_version: req.baseVersion,
      ydoc: Buffer.from(req.state).toString('base64'),
      content: req.content,
      editor_ids: req.editorIds,
      awareness_count: req.awarenessCount,
    })
    const res = await this.call('POST', STORE_PATH, body, 'application/json')
    const data = await this.json<{ ydoc_version?: number }>(res)
    return Number(data.ydoc_version ?? req.baseVersion + 1)
  }

  /** True when the Go server answers the signed health probe. */
  async health(): Promise<boolean> {
    try {
      const res = await this.call('GET', HEALTH_PATH, '', '')
      return res.ok
    } catch {
      return false
    }
  }

  private async call(method: string, pathWithQuery: string, body: string, contentType: string): Promise<Response> {
    const headers: Record<string, string> = signedHeaders(this.opts.secret, method, pathWithQuery, body)
    if (contentType) headers['content-type'] = contentType
    headers.accept = 'application/json, application/octet-stream'
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.opts.timeoutMs)
    let res: Response
    try {
      res = await this.fetchImpl(this.opts.baseUrl + pathWithQuery, {
        method, headers, body: method === 'GET' ? undefined : body, signal: controller.signal,
      })
    } catch (err) {
      throw new BackendUnavailable(`backend ${method} ${pathWithQuery} failed: ${(err as Error).message}`, err)
    } finally {
      clearTimeout(timer)
    }
    if (res.ok) return res
    const text = await res.text().catch(() => '')
    let parsed: { error?: string; code?: string; current_version?: number } = {}
    try {
      parsed = text ? JSON.parse(text) : {}
    } catch {
      parsed = { error: text.slice(0, 200) }
    }
    if (res.status === 409) throw new ConflictError(Number(parsed.current_version ?? 0))
    if (res.status >= 500) throw new BackendUnavailable(`backend ${method} ${pathWithQuery} answered ${res.status}`)
    throw new BackendError(res.status, parsed.error || `backend answered ${res.status}`, parsed.code ?? '')
  }

  private async json<T>(res: Response): Promise<T> {
    try {
      return (await res.json()) as T
    } catch (err) {
      throw new BackendUnavailable('backend returned malformed JSON', err)
    }
  }
}
