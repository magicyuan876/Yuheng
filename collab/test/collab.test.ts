// End-to-end tests of the service against a fake Go server: two clients
// converge, stores are debounced and retried, read-only connections cannot
// write, an outage loses nothing, and the two Go-originated endpoints work.

import { HocuspocusProvider } from '@hocuspocus/provider'
import assert from 'node:assert/strict'
import { after, before, beforeEach, test } from 'node:test'
import WebSocket from 'ws'
import * as Y from 'yjs'

import { loadConfig, type CollabConfig } from '../src/config.ts'
import { silentLogger } from '../src/log.ts'
import { M } from '../src/metrics.ts'
import { EMPTY_DOCUMENT, FRAGMENT, jsonToYDoc, yDocToJSON } from '../src/pm-schema.ts'
import { createCollabServer, type CollabServer } from '../src/server.ts'
import { signedHeaders } from '../src/signing.ts'
import { FakeBackend } from './fake-backend.ts'

const SECRET = 'a-shared-secret-of-sufficient-length'

let backend: FakeBackend
let backendUrl: string
let collab: CollabServer
let port: number

function config(overrides: Record<string, string> = {}): CollabConfig {
  return loadConfig({
    COLLAB_BACKEND_URL: backendUrl,
    COLLAB_SHARED_SECRET: SECRET,
    COLLAB_PORT: '0',
    COLLAB_ADDRESS: '127.0.0.1',
    COLLAB_STORE_DEBOUNCE_MS: '80',
    COLLAB_STORE_MAX_WAIT_MS: '200',
    COLLAB_STORE_RETRY_BASE_MS: '60',
    COLLAB_STORE_RETRY_MAX_MS: '200',
    COLLAB_AUTH_CACHE_MS: '0',
    COLLAB_SHUTDOWN_FLUSH_TIMEOUT_MS: '3000',
    ...overrides,
  } as NodeJS.ProcessEnv)
}

before(async () => {
  backend = new FakeBackend(SECRET)
  backendUrl = await backend.listen()
  collab = createCollabServer(config(), silentLogger)
  port = await collab.start()
})

after(async () => {
  await collab.stop()
  await backend.close()
})

beforeEach(() => {
  backend.pages.clear()
  backend.storeCalls.length = 0
  backend.authCalls.length = 0
  backend.down = false
  backend.conflictOnce = false
  backend.rejectStoreOnce = null
  backend.invalidTokens.clear()
})

interface Client {
  provider: HocuspocusProvider
  doc: Y.Doc
  destroy(): void
}

/** Opens a browser-equivalent connection: one socket per provider. */
function connect(pageId: string, user: string, opts: { tenant?: string; port?: number } = {}): Client {
  const doc = new Y.Doc()
  const provider = new HocuspocusProvider({
    url: `ws://127.0.0.1:${opts.port ?? port}?tenant=${opts.tenant ?? '1'}`,
    WebSocketPolyfill: WebSocket as unknown as typeof globalThis.WebSocket,
    maxAttempts: 1,
    name: pageId,
    document: doc,
    token: `token-${user}`,
  } as never)
  return {
    provider, doc,
    destroy() {
      provider.destroy()
    },
  }
}

/** Resolves when the provider has finished its first sync, or rejects. */
function synced(client: Client, timeoutMs = 4000): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timed out waiting for sync')), timeoutMs)
    if (client.provider.isSynced) {
      clearTimeout(timer)
      resolve()
      return
    }
    client.provider.on('synced', () => {
      clearTimeout(timer)
      resolve()
    })
    client.provider.on('authenticationFailed', (data: { reason: string }) => {
      clearTimeout(timer)
      reject(new Error(`authentication failed: ${data.reason}`))
    })
  })
}

function text(doc: Y.Doc): string {
  const json = yDocToJSON(doc) as { content?: { content?: { text?: string }[] }[] }
  return (json.content ?? []).flatMap((b) => (b.content ?? []).map((i) => i.text ?? '')).join('')
}

/** Types into the first paragraph of the document. */
function type(doc: Y.Doc, value: string, at = 0): void {
  const fragment = doc.getXmlFragment(FRAGMENT)
  const paragraph = fragment.get(0) as Y.XmlElement
  const child = paragraph.get(0)
  if (child instanceof Y.XmlText) child.insert(at, value)
  else paragraph.insert(0, [new Y.XmlText(value)])
}

async function waitFor(predicate: () => boolean, timeoutMs = 4000, label = 'condition'): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (predicate()) return
    await new Promise((r) => setTimeout(r, 25))
  }
  throw new Error(`timed out waiting for ${label}`)
}

/** The text of the ydoc the fake backend has stored for a page. */
function storedText(pageId: string): string | null {
  const page = backend.pages.get(pageId)
  if (!page?.ydoc) return null
  const doc = new Y.Doc()
  Y.applyUpdate(doc, new Uint8Array(page.ydoc))
  return text(doc)
}

function seedPage(pageId: string, users: { writers?: string[]; readers?: string[] } = {}, content?: unknown): void {
  const page = backend.page(pageId)
  page.content = content ?? EMPTY_DOCUMENT
  for (const w of users.writers ?? []) page.writers.add(w)
  for (const r of users.readers ?? []) page.readers.add(r)
}

test('two clients converge and the merged state is persisted once', async () => {
  seedPage('page-converge', { writers: ['alice', 'bob'] })
  const alice = connect('page-converge', 'alice')
  const bob = connect('page-converge', 'bob')
  try {
    await Promise.all([synced(alice), synced(bob)])
    type(alice.doc, 'hello ')
    await waitFor(() => text(bob.doc).includes('hello'), 4000, 'bob to see alice')
    type(bob.doc, 'world', text(bob.doc).length)
    await waitFor(() => text(alice.doc) === 'hello world', 4000, 'alice to see bob')
    assert.equal(text(bob.doc), 'hello world')

    await waitFor(() => storedText('page-converge') === 'hello world', 4000, 'the merged state to be stored')
    const stored = backend.page('page-converge')
    assert.equal(storedText('page-converge'), 'hello world', 'the stored ydoc holds the merged text')
    const json = stored.content as { type: string }
    assert.equal(json.type, 'doc', 'the JSON projection is stored alongside')
    const last = backend.storeCalls.at(-1)!
    assert.deepEqual([...last.editorIds].sort(), ['alice', 'bob'], 'both editors are reported')
  } finally {
    alice.destroy()
    bob.destroy()
  }
})

test('a page that only has JSON is materialised as a ydoc on first open', async () => {
  const body = {
    type: 'doc',
    content: [{ type: 'paragraph', content: [{ type: 'text', text: 'from json' }] }],
  }
  seedPage('page-json', { writers: ['alice'] }, body)
  const alice = connect('page-json', 'alice')
  try {
    await synced(alice)
    assert.equal(text(alice.doc), 'from json')
    await waitFor(() => backend.page('page-json').ydoc !== null, 4000, 'the ydoc to be written')
    assert.ok(backend.page('page-json').version >= 1)
  } finally {
    alice.destroy()
  }
})

test('a store conflict reloads the newer state and retries without losing the edit', async () => {
  seedPage('page-conflict', { writers: ['alice'] })
  const alice = connect('page-conflict', 'alice')
  try {
    await synced(alice)
    await waitFor(() => backend.page('page-conflict').version > 0, 4000, 'the initial store')
    const before = collab.metrics.get(M.storeConflict)

    backend.conflictOnce = true
    type(alice.doc, 'conflicted')
    await waitFor(() => collab.metrics.get(M.storeConflict) > before, 4000, 'a conflict')
    await waitFor(() => storedText('page-conflict') === 'conflicted', 4000, 'the retried store to land')
    assert.equal(text(alice.doc), 'conflicted')
  } finally {
    alice.destroy()
  }
})

test('a read-only connection can read but its writes never reach the store', async () => {
  seedPage('page-readonly', { writers: ['alice'], readers: ['carol'] })
  const alice = connect('page-readonly', 'alice')
  const carol = connect('page-readonly', 'carol')
  try {
    await Promise.all([synced(alice), synced(carol)])
    type(alice.doc, 'visible')
    await waitFor(() => text(carol.doc) === 'visible', 4000, 'carol to receive the text')
    await waitFor(() => backend.page('page-readonly').version > 0, 4000, 'a store')

    const dropped = collab.metrics.get(M.readonlyDropped)
    type(carol.doc, 'sneaky ')
    await waitFor(() => collab.metrics.get(M.readonlyDropped) > dropped, 4000, 'the write to be dropped')
    await new Promise((r) => setTimeout(r, 300))
    assert.equal(text(alice.doc), 'visible', 'the writer never sees the read-only edit')
    assert.equal(storedText('page-readonly'), 'visible', 'and it is not persisted')
  } finally {
    alice.destroy()
    carol.destroy()
  }
})

test('a caller without access is rejected and no document is created', async () => {
  seedPage('page-denied', { writers: ['alice'] })
  const mallory = connect('page-denied', 'mallory')
  try {
    await assert.rejects(synced(mallory, 2500), /authentication failed|timed out/)
    assert.equal(collab.metrics.get(M.authRejected) > 0, true)
  } finally {
    mallory.destroy()
  }
})

test('edits made while the Go server is down are stored when it returns', async () => {
  seedPage('page-outage', { writers: ['alice'] })
  const alice = connect('page-outage', 'alice')
  try {
    await synced(alice)
    await waitFor(() => backend.page('page-outage').version > 0, 4000, 'the initial store')

    backend.down = true
    const failures = collab.metrics.get(M.storeFailed)
    type(alice.doc, 'written during the outage')
    await waitFor(() => collab.metrics.get(M.storeFailed) > failures, 4000, 'a failed store')

    backend.down = false
    await waitFor(() => storedText('page-outage') === 'written during the outage', 6000, 'the retry to succeed')
  } finally {
    alice.destroy()
  }
})

test('replace applies a new body to the live document and stores it', async () => {
  seedPage('page-replace', { writers: ['alice'] })
  const alice = connect('page-replace', 'alice')
  try {
    await synced(alice)
    type(alice.doc, 'original')
    await waitFor(() => text(alice.doc) === 'original')

    const body = JSON.stringify({
      tenant_id: '1',
      reason: 'restore',
      content: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'restored' }] }] },
    })
    const path = '/internal/collab/replace/page-replace'
    const res = await fetch(`http://127.0.0.1:${port}${path}`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...signedHeaders(SECRET, 'POST', path, body) },
      body,
    })
    const payload = await res.text()
    assert.equal(res.status, 200, payload)
    const data = JSON.parse(payload) as { ydoc_version: number }
    assert.ok(data.ydoc_version > 0)

    await waitFor(() => text(alice.doc) === 'restored', 4000, 'the online client to see the replacement')
    assert.equal(storedText('page-replace'), 'restored', 'ydoc and JSON do not diverge')
    assert.equal(text(jsonToYDoc(backend.page('page-replace').content)), 'restored')
  } finally {
    alice.destroy()
  }
})

test('replace refuses unsigned requests and invalid documents', async () => {
  seedPage('page-guarded', { writers: ['alice'] })
  const path = '/internal/collab/replace/page-guarded'
  const body = JSON.stringify({ tenant_id: '1', content: EMPTY_DOCUMENT })

  const unsigned = await fetch(`http://127.0.0.1:${port}${path}`, {
    method: 'POST', headers: { 'content-type': 'application/json' }, body,
  })
  assert.equal(unsigned.status, 401)

  const bad = JSON.stringify({ tenant_id: '1', content: { type: 'doc', content: [{ type: 'notANode' }] } })
  const rejected = await fetch(`http://127.0.0.1:${port}${path}`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', ...signedHeaders(SECRET, 'POST', path, bad) },
    body: bad,
  })
  assert.equal(rejected.status, 422)
})

test('evicting a page closes its connections', async () => {
  seedPage('page-evict', { writers: ['alice'] })
  const alice = connect('page-evict', 'alice')
  try {
    await synced(alice)
    type(alice.doc, 'before eviction')
    await waitFor(() => backend.storeCalls.some((c) => c.pageId === 'page-evict'), 4000, 'a store')

    const path = '/internal/collab/page-evict'
    const res = await fetch(`http://127.0.0.1:${port}${path}`, {
      method: 'DELETE', headers: signedHeaders(SECRET, 'DELETE', path, ''),
    })
    assert.equal(res.status, 204)
    await waitFor(() => collab.server.hocuspocus.documents.get('page-evict') === undefined ||
      (collab.server.hocuspocus.documents.get('page-evict')?.getConnectionsCount() ?? 0) === 0,
    4000, 'the connection to be dropped')
    assert.equal(storedText('page-evict'), 'before eviction', 'the state was stored before the eviction')
  } finally {
    alice.destroy()
  }
})

test('health reports the backend and metrics expose the counters', async () => {
  const healthy = await fetch(`http://127.0.0.1:${port}/healthz`)
  assert.equal(healthy.status, 200)
  const body = (await healthy.json()) as { status: string; backend: string }
  assert.equal(body.status, 'ok')

  backend.down = true
  const degraded = await fetch(`http://127.0.0.1:${port}/healthz`)
  assert.equal(degraded.status, 503)
  backend.down = false

  const metrics = await fetch(`http://127.0.0.1:${port}/metrics`)
  assert.equal(metrics.status, 200)
  const text2 = await metrics.text()
  assert.match(text2, /collab_connections_accepted_total \d+/)
  assert.match(text2, /collab_store_duration_seconds_count \d+/)
  assert.match(text2, /# TYPE collab_documents gauge/)
})

test('a document larger than the limit is not stored and the client is told', async () => {
  const small = createCollabServer(config({ COLLAB_MAX_YDOC_BYTES: '1024' }), silentLogger)
  const smallPort = await small.start()
  seedPage('page-large', { writers: ['alice'] })
  const client = connect('page-large', 'alice', { port: smallPort })
  const messages: unknown[] = []
  client.provider.on('stateless', (data: { payload: string }) => messages.push(JSON.parse(data.payload)))
  try {
    await synced(client)
    type(client.doc, 'x'.repeat(4000))
    await waitFor(() => small.metrics.get(M.storeTooLarge) > 0, 4000, 'the size rejection')
    await waitFor(() => messages.some((m) => (m as { code?: string }).code === 'ydoc_too_large'), 4000,
      'the client to be told')
    await new Promise((r) => setTimeout(r, 200))
    assert.equal((storedText('page-large') ?? '').length, 0, 'the oversized text never reached the store')
  } finally {
    client.destroy()
    await small.stop()
  }
})
