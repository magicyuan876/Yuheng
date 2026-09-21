import assert from 'node:assert/strict'
import { test } from 'vitest'

import * as Y from 'yjs'

import {
  fromBase64,
  isUnsupported,
  isVersionConflict,
  RestProvider,
  toBase64,
  type LeaseResponse,
  type RestProviderState,
  type RestTransport,
} from './restProvider'

const PAGE = 'page-1'

function pmDoc(text: string) {
  return {
    type: 'doc',
    content: [{ type: 'paragraph', content: [{ type: 'text', text }] }],
  }
}

function lease(over: Partial<LeaseResponse> = {}): LeaseResponse {
  return { held: true, held_by_me: true, ydoc_version: 0, ...over }
}

interface SavedCall {
  base_version: number
  ydoc: string
  content: unknown
}

/**
 * A stand-in for the server: it holds one page's Yjs state and one lease, and
 * enforces exactly the two rules the real endpoints do — only the holding
 * session may save, and only against the current version.
 */
class FakeServer implements RestTransport {
  version = 0
  ydoc: string | undefined
  content: unknown = pmDoc('start')
  holder: { user: string; session: string } | null = null
  saves: SavedCall[] = []
  failNext: unknown = null

  constructor(readonly me = 'alice') {}

  private maybeFail() {
    if (this.failNext) {
      const err = this.failNext
      this.failNext = null
      throw err
    }
  }

  async loadYDoc(_pageId: string) {
    this.maybeFail()
    return this.ydoc
      ? { ydoc: this.ydoc, ydoc_version: this.version }
      : { content: this.content, ydoc_version: this.version }
  }

  async getLease(_pageId: string): Promise<LeaseResponse> {
    this.maybeFail()
    return this.leaseFor(null)
  }

  async acquireLease(_pageId: string, sessionId: string): Promise<LeaseResponse> {
    this.maybeFail()
    if (!this.holder || this.holder.session === sessionId) {
      this.holder = { user: this.me, session: sessionId }
    }
    return this.leaseFor(sessionId)
  }

  async releaseLease(_pageId: string, sessionId: string): Promise<void> {
    this.maybeFail()
    if (this.holder?.session === sessionId) this.holder = null
  }

  async saveYDoc(_pageId: string, body: {
    session_id: string
    base_version: number
    ydoc: string
    content: unknown
  }) {
    this.maybeFail()
    if (this.holder?.session !== body.session_id) {
      throw { status: 409, message: 'somebody else is editing this page' }
    }
    if (body.base_version !== this.version) {
      throw { status: 409, message: 'version conflict; reload and retry' }
    }
    this.saves.push({ base_version: body.base_version, ydoc: body.ydoc, content: body.content })
    this.ydoc = body.ydoc
    this.content = body.content
    this.version++
    return { ydoc_version: this.version, lease: this.leaseFor(body.session_id) }
  }

  /** Simulates somebody else taking the page over. */
  takeOver(user: string, session: string) {
    this.holder = { user, session }
  }

  private leaseFor(sessionId: string | null): LeaseResponse {
    if (!this.holder) return { held: false, held_by_me: false, ydoc_version: this.version }
    return {
      held: true,
      held_by_me: this.holder.session === sessionId,
      holder: { user_id: this.holder.user, username: this.holder.user },
      ydoc_version: this.version,
    }
  }
}

/** Builds a provider whose timers are short enough for a test to wait on. */
function provider(server: FakeServer, over: {
  canEdit?: boolean
  sessionId?: string
  saveIntervalMs?: number
  pollIntervalMs?: number
  maxPendingUpdates?: number
} = {}) {
  const ydoc = new Y.Doc()
  const states: RestProviderState[] = []
  const p = new RestProvider({
    pageId: PAGE,
    ydoc,
    canEdit: over.canEdit ?? true,
    sessionId: over.sessionId ?? 'tab-1',
    transport: server,
    onChange: (s) => states.push(s),
    saveIntervalMs: over.saveIntervalMs ?? 5,
    // Long enough that no test polls unless it asks to.
    pollIntervalMs: over.pollIntervalMs ?? 3_600_000,
    maxPendingUpdates: over.maxPendingUpdates,
  })
  return { p, ydoc, states }
}

/** Types into the document the way the editor would. */
function type(ydoc: Y.Doc, text: string) {
  const fragment = ydoc.getXmlFragment('default')
  const paragraph = fragment.get(0) as Y.XmlElement
  const node = paragraph.get(0) as Y.XmlText
  node.insert(node.length, text)
}

const tick = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

test('materialises a Yjs document from the stored body when the page has never been edited', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server)
  await p.start()

  assert.equal(p.current().status, 'connected')
  assert.equal(p.current().access, 'read-write')
  assert.equal(ydoc.getXmlFragment('default').length, 1, 'the stored paragraph must be in the document')
  await p.stop()
})

test('loads the stored Yjs state in preference to the body', async () => {
  const source = new Y.Doc()
  const fragment = source.getXmlFragment('default')
  const paragraph = new Y.XmlElement('paragraph')
  const text = new Y.XmlText()
  text.insert(0, 'from the stored state')
  paragraph.insert(0, [text])
  fragment.insert(0, [paragraph])

  const server = new FakeServer()
  server.ydoc = toBase64(Y.encodeStateAsUpdate(source))
  server.version = 7

  const { p, ydoc } = provider(server)
  await p.start()
  assert.equal(ydoc.getXmlFragment('default').toString(), fragment.toString())
  assert.equal(p.current().version, 7)
  await p.stop()
})

test('saves on a fixed interval rather than only when typing stops', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server, { saveIntervalMs: 20 })
  await p.start()

  type(ydoc, 'a')
  type(ydoc, 'b')
  assert.equal(server.saves.length, 0, 'the first keystrokes must not each cost a request')

  await tick(40)
  assert.equal(server.saves.length, 1, 'a burst of typing becomes one save')
  assert.equal(p.current().version, 1)
  assert.equal(p.current().save, 'saved')
  await p.stop()
})

test('saves at once when a single change is large enough to batch', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server, { saveIntervalMs: 60_000, maxPendingUpdates: 3 })
  await p.start()

  type(ydoc, 'a')
  type(ydoc, 'b')
  assert.equal(server.saves.length, 0)
  type(ydoc, 'c')
  await tick(5)
  assert.equal(server.saves.length, 1, 'the update cap must not wait for the timer')
  await p.stop()
})

test('sends the whole Yjs state and its ProseMirror projection together', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server)
  await p.start()
  type(ydoc, ' and more')
  await p.flush()

  assert.equal(server.saves.length, 1)
  const sent = server.saves[0]
  assert.equal(sent.base_version, 0)
  const doc = sent.content as { type: string; content: { content: { text: string }[] }[] }
  assert.equal(doc.type, 'doc')
  assert.equal(doc.content[0].content[0].text, 'start and more')

  // The state must be a real Yjs update, not the JSON in disguise.
  const rebuilt = new Y.Doc()
  Y.applyUpdate(rebuilt, fromBase64(sent.ydoc))
  assert.equal(rebuilt.getXmlFragment('default').toString(), ydoc.getXmlFragment('default').toString())
  await p.stop()
})

test('a reader never asks for the lease and is told who holds it', async () => {
  const server = new FakeServer()
  server.takeOver('alice', 'alice-tab')

  const { p } = provider(server, { canEdit: false, sessionId: 'reader-tab' })
  await p.start()

  assert.equal(p.current().access, 'readonly')
  assert.deepEqual(p.current().otherHolder, { userId: 'alice', name: 'alice' })
  assert.equal(server.holder?.session, 'alice-tab', 'the reader must not have disturbed the lease')
  await p.stop()
})

test('a second writer follows along read-only until the page frees up', async () => {
  const server = new FakeServer()
  server.takeOver('alice', 'alice-tab')

  const { p, ydoc } = provider(server, { sessionId: 'bob-tab', pollIntervalMs: 20 })
  await p.start()
  assert.equal(p.current().access, 'readonly')
  assert.equal(p.current().otherHolder?.name, 'alice')

  // Typing while read-only stays local; nothing is posted.
  type(ydoc, 'x')
  await tick(30)
  assert.equal(server.saves.length, 0)

  // Alice leaves, and the next poll picks the page up.
  server.holder = null
  await tick(40)
  assert.equal(p.current().access, 'read-write')
  assert.equal(p.current().otherHolder, null)
  await p.stop()
})

test('picks up the other editor’s work when taking the page over', async () => {
  const server = new FakeServer()
  server.takeOver('alice', 'alice-tab')

  // Alice saved while this client was reading.
  const alice = new Y.Doc()
  const fragment = alice.getXmlFragment('default')
  const paragraph = new Y.XmlElement('paragraph')
  const text = new Y.XmlText()
  text.insert(0, 'alice wrote this')
  paragraph.insert(0, [text])
  fragment.insert(0, [paragraph])

  const { p, ydoc } = provider(server, { sessionId: 'bob-tab', pollIntervalMs: 20 })
  await p.start()
  assert.equal(p.current().access, 'readonly')

  server.ydoc = toBase64(Y.encodeStateAsUpdate(alice))
  server.version = 5
  server.holder = null

  await tick(40)
  assert.equal(p.current().access, 'read-write')
  assert.equal(p.current().version, 5)
  assert.ok(
    ydoc.getXmlFragment('default').toString().includes('alice wrote this'),
    'taking the page over must start from what the previous editor saved',
  )
  await p.stop()
})

test('stops writing when the page is taken over mid-edit', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server)
  await p.start()
  assert.equal(p.current().access, 'read-write')

  // Somebody else's tab took the lease while this one was typing.
  server.takeOver('bob', 'bob-tab')
  type(ydoc, 'more text')
  await p.flush()

  assert.equal(p.current().save, 'superseded')
  assert.equal(p.current().access, 'readonly', 'a superseded client must not keep writing')
  assert.equal(server.saves.length, 0)
  await p.stop()
})

test('refuses to overwrite a page whose stored version moved on', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server)
  await p.start()

  // The stored page advanced without this client seeing it.
  server.version = 3
  type(ydoc, 'z')
  await p.flush()

  assert.equal(p.current().save, 'superseded')
  assert.equal(server.saves.length, 0)
  await p.stop()
})

test('saves and hands the page back when the view closes', async () => {
  const server = new FakeServer()
  const { p, ydoc } = provider(server, { saveIntervalMs: 60_000 })
  await p.start()
  type(ydoc, ' final words')

  await p.stop()
  assert.equal(server.saves.length, 1, 'unsaved work must not be lost on teardown')
  assert.equal(server.holder, null, 'the next editor must not have to wait for the lease to expire')
  assert.equal(p.current().status, 'disconnected')
})

test('a failed load leaves the editor read-only instead of silently blank', async () => {
  const server = new FakeServer()
  server.failNext = { status: 500, message: 'boom' }
  const { p } = provider(server)
  await p.start()

  assert.equal(p.current().status, 'disconnected')
  assert.equal(p.current().access, 'readonly')
  assert.equal(p.current().unsupported, false)
  await p.stop()
})

test('recognises a deployment that does not do exclusive editing at all', async () => {
  const server = new FakeServer()
  server.failNext = {
    status: 409,
    message: 'docs: this deployment edits pages through the collaboration service, not exclusive leases',
  }
  const { p } = provider(server)
  await p.start()

  assert.equal(p.current().unsupported, true)
  assert.equal(p.current().access, 'readonly')
  await p.stop()
})

test('classifies the two conflicts the server can answer with', () => {
  const wrongMode = { status: 409, message: 'edits pages through the collaboration service' }
  const staleVersion = { status: 409, message: 'version conflict; reload and retry' }
  assert.equal(isUnsupported(wrongMode), true)
  assert.equal(isUnsupported(staleVersion), false)
  assert.equal(isVersionConflict(staleVersion), true)
  assert.equal(isVersionConflict({ status: 403 }), false)
  assert.equal(isVersionConflict(new Error('offline')), false)
})

test('base64 round-trips a Yjs update byte for byte', () => {
  const doc = new Y.Doc()
  doc.getXmlFragment('default').insert(0, [new Y.XmlElement('paragraph')])
  const bytes = Y.encodeStateAsUpdate(doc)
  assert.deepEqual(Array.from(fromBase64(toBase64(bytes))), Array.from(bytes))
})
