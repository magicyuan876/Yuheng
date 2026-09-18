// A stand-in for the Go server: verifies the HMAC on every call, keeps page
// state in memory, and lets a test script failures (outage, conflict,
// rejection) to exercise the service's recovery paths.

import { createServer, type Server } from 'node:http'
import type { AddressInfo } from 'node:net'

import { SIGNATURE_HEADER, TIMESTAMP_HEADER, verify } from '../src/signing.ts'

export interface FakePage {
  tenantId: string
  ydoc: Buffer | null
  content: unknown
  version: number
  /** Users allowed to write; everyone else in `readers` gets readonly. */
  writers: Set<string>
  readers: Set<string>
}

export interface StoreCall {
  pageId: string
  baseVersion: number
  editorIds: string[]
  awarenessCount: number
  bytes: number
  content: unknown
}

export class FakeBackend {
  readonly pages = new Map<string, FakePage>()
  readonly storeCalls: StoreCall[] = []
  readonly authCalls: { token: string; pageId: string }[] = []
  /** When set, every call answers 503 (simulates the Go server being down). */
  down = false
  /** When set, the next store answers 409 and then clears itself. */
  conflictOnce = false
  /** When set, store answers this status once, then clears itself. */
  rejectStoreOnce: { status: number; error: string } | null = null
  /** Rejects authentication for these tokens. */
  readonly invalidTokens = new Set<string>()
  private server: Server | null = null
  private unsignedCalls = 0

  constructor(private readonly secret: string) {}

  get rejectedUnsigned(): number {
    return this.unsignedCalls
  }

  page(pageId: string): FakePage {
    let p = this.pages.get(pageId)
    if (!p) {
      p = { tenantId: '1', ydoc: null, content: null, version: 0, writers: new Set(), readers: new Set() }
      this.pages.set(pageId, p)
    }
    return p
  }

  async listen(): Promise<string> {
    this.server = createServer((req, res) => void this.handle(req, res))
    await new Promise<void>((resolve) => this.server!.listen(0, '127.0.0.1', resolve))
    const { port } = this.server!.address() as AddressInfo
    return `http://127.0.0.1:${port}`
  }

  async close(): Promise<void> {
    if (!this.server) return
    await new Promise<void>((resolve) => this.server!.close(() => resolve()))
    this.server = null
  }

  private async handle(req: import('node:http').IncomingMessage, res: import('node:http').ServerResponse): Promise<void> {
    const chunks: Buffer[] = []
    for await (const c of req) chunks.push(c as Buffer)
    const body = Buffer.concat(chunks)
    const url = new URL(req.url ?? '/', 'http://fake.local')
    const path = url.pathname + url.search
    const method = (req.method ?? 'GET').toUpperCase()
    const ok = verify(this.secret, method, path, str(req.headers[TIMESTAMP_HEADER]), str(req.headers[SIGNATURE_HEADER]), body)
    if (!ok.ok) {
      this.unsignedCalls++
      return json(res, 401, { error: ok.reason })
    }
    if (this.down) return json(res, 503, { error: 'backend down' })

    if (method === 'GET' && url.pathname === '/internal/collab/health') return json(res, 200, { status: 'ok' })

    if (method === 'POST' && url.pathname === '/internal/collab/authenticate') {
      const req2 = JSON.parse(body.toString('utf8')) as { token: string; page_id: string; tenant_id: string }
      this.authCalls.push({ token: req2.token, pageId: req2.page_id })
      if (this.invalidTokens.has(req2.token)) return json(res, 401, { error: 'invalid token' })
      const page = this.page(req2.page_id)
      const user = req2.token.replace(/^token-/, '')
      if (!page.writers.has(user) && !page.readers.has(user)) return json(res, 403, { error: 'no access' })
      return json(res, 200, {
        user_id: user,
        display_name: user,
        avatar: '',
        access: page.writers.has(user) ? 'readwrite' : 'readonly',
        ydoc_version: page.version,
        tenant_id: page.tenantId,
        space_id: 'space-1',
      })
    }

    const load = url.pathname.match(/^\/internal\/collab\/load\/([^/]+)$/)
    if (method === 'GET' && load) {
      const page = this.page(decodeURIComponent(load[1]))
      if (page.ydoc) {
        res.writeHead(200, { 'content-type': 'application/octet-stream', 'x-ydoc-version': String(page.version) })
        return void res.end(page.ydoc)
      }
      return json(res, 200, { content: page.content, ydoc_version: page.version })
    }

    if (method === 'POST' && url.pathname === '/internal/collab/store') {
      const req2 = JSON.parse(body.toString('utf8')) as {
        page_id: string; base_version: number; ydoc: string; content: unknown; editor_ids: string[]
        awareness_count: number
      }
      const page = this.page(req2.page_id)
      this.storeCalls.push({
        pageId: req2.page_id, baseVersion: req2.base_version, editorIds: req2.editor_ids,
        awarenessCount: req2.awareness_count, bytes: Buffer.from(req2.ydoc, 'base64').length, content: req2.content,
      })
      if (this.rejectStoreOnce) {
        const r = this.rejectStoreOnce
        this.rejectStoreOnce = null
        return json(res, r.status, { error: r.error })
      }
      if (this.conflictOnce) {
        this.conflictOnce = false
        page.version += 1 // someone else persisted
        return json(res, 409, { error: 'conflict', current_version: page.version })
      }
      if (req2.base_version !== page.version) {
        return json(res, 409, { error: 'conflict', current_version: page.version })
      }
      page.ydoc = Buffer.from(req2.ydoc, 'base64')
      page.content = req2.content
      page.version += 1
      return json(res, 200, { ydoc_version: page.version })
    }

    return json(res, 404, { error: 'not found' })
  }
}

function json(res: import('node:http').ServerResponse, status: number, body: unknown): void {
  const data = JSON.stringify(body)
  res.writeHead(status, { 'content-type': 'application/json', 'content-length': Buffer.byteLength(data) })
  res.end(data)
}

function str(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v
}
