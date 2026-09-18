// Two service instances sharing one (fake) Redis must behave like one: an
// edit on either side reaches the other, and a client that joins the second
// instance sees changes the first has not persisted yet.

import { HocuspocusProvider } from '@hocuspocus/provider'
import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { after, before, test } from 'node:test'
import WebSocket from 'ws'
import * as Y from 'yjs'

import { loadConfig } from '../src/config.ts'
import type { RedisLike } from '../src/extensions/redis-sync.ts'
import { silentLogger } from '../src/log.ts'
import { EMPTY_DOCUMENT, FRAGMENT, yDocToJSON } from '../src/pm-schema.ts'
import { createCollabServer, type CollabServer } from '../src/server.ts'
import { FakeBackend } from './fake-backend.ts'

const SECRET = 'a-shared-secret-of-sufficient-length'

/** An in-process stand-in for Redis pub/sub. */
class FakeRedisBus {
  private readonly bus = new EventEmitter()

  constructor() {
    this.bus.setMaxListeners(100)
  }

  client(): RedisLike {
    const channels = new Set<string>()
    const handlers: ((channel: Buffer, message: Buffer) => void)[] = []
    const onPublish = (channel: string, message: Buffer) => {
      if (!channels.has(channel)) return
      for (const h of handlers) h(Buffer.from(channel), message)
    }
    this.bus.on('publish', onPublish)
    return {
      publish: async (channel: string, message: Buffer) => {
        // Deliver asynchronously, like a real round trip.
        setImmediate(() => this.bus.emit('publish', channel, message))
        return 1
      },
      subscribe: async (...names: string[]) => {
        for (const n of names) channels.add(n)
        return names.length
      },
      unsubscribe: async (...names: string[]) => {
        for (const n of names) channels.delete(n)
        return names.length
      },
      on: (_event: 'messageBuffer', handler: (channel: Buffer, message: Buffer) => void) => {
        handlers.push(handler)
        return undefined
      },
      quit: async () => {
        this.bus.off('publish', onPublish)
        handlers.length = 0
        channels.clear()
        return 'OK'
      },
    }
  }
}

let backend: FakeBackend
let backendUrl: string
let one: CollabServer
let two: CollabServer
let portOne: number
let portTwo: number

before(async () => {
  backend = new FakeBackend(SECRET)
  backendUrl = await backend.listen()
  const bus = new FakeRedisBus()
  const cfg = () => loadConfig({
    COLLAB_BACKEND_URL: backendUrl,
    COLLAB_SHARED_SECRET: SECRET,
    COLLAB_PORT: '0',
    COLLAB_ADDRESS: '127.0.0.1',
    COLLAB_STORE_DEBOUNCE_MS: '100',
    COLLAB_STORE_MAX_WAIT_MS: '300',
    COLLAB_AUTH_CACHE_MS: '0',
  } as NodeJS.ProcessEnv)
  one = createCollabServer(cfg(), silentLogger, { createRedis: () => bus.client(), instanceId: 'instance-one' })
  two = createCollabServer(cfg(), silentLogger, { createRedis: () => bus.client(), instanceId: 'instance-two' })
  portOne = await one.start()
  portTwo = await two.start()
})

after(async () => {
  await one.stop()
  await two.stop()
  await backend.close()
})

function connect(port: number, pageId: string, user: string): { provider: HocuspocusProvider; doc: Y.Doc } {
  const doc = new Y.Doc()
  const provider = new HocuspocusProvider({
    url: `ws://127.0.0.1:${port}?tenant=1`,
    WebSocketPolyfill: WebSocket as unknown as typeof globalThis.WebSocket,
    maxAttempts: 1,
    name: pageId,
    document: doc,
    token: `token-${user}`,
  } as never)
  return { provider, doc }
}

function synced(provider: HocuspocusProvider, timeoutMs = 4000): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timed out waiting for sync')), timeoutMs)
    if (provider.isSynced) {
      clearTimeout(timer)
      resolve()
      return
    }
    provider.on('synced', () => {
      clearTimeout(timer)
      resolve()
    })
  })
}

function text(doc: Y.Doc): string {
  const json = yDocToJSON(doc) as { content?: { content?: { text?: string }[] }[] }
  return (json.content ?? []).flatMap((b) => (b.content ?? []).map((i) => i.text ?? '')).join('')
}

function type(doc: Y.Doc, value: string, at = 0): void {
  const paragraph = doc.getXmlFragment(FRAGMENT).get(0) as Y.XmlElement
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

test('clients on different instances see each other in real time', async () => {
  const page = backend.page('page-cluster')
  page.content = EMPTY_DOCUMENT
  page.writers.add('alice')
  page.writers.add('bob')

  const alice = connect(portOne, 'page-cluster', 'alice')
  const bob = connect(portTwo, 'page-cluster', 'bob')
  try {
    await Promise.all([synced(alice.provider), synced(bob.provider)])

    type(alice.doc, 'from instance one ')
    // Wait for the whole update, not a prefix: appending to a half-applied
    // document would insert in the middle.
    await waitFor(() => text(bob.doc) === 'from instance one ', 4000, 'bob to receive the update')

    type(bob.doc, 'and two', text(bob.doc).length)
    await waitFor(() => text(alice.doc) === 'from instance one and two', 4000, 'alice to receive the reply')

    // Awareness (cursors) crosses instances too.
    alice.provider.setAwarenessField('user', { name: 'alice' })
    await waitFor(() => [...bob.provider.awareness!.getStates().values()]
      .some((s) => (s as { user?: { name?: string } }).user?.name === 'alice'), 4000, 'awareness to cross')
  } finally {
    alice.provider.destroy()
    bob.provider.destroy()
  }
})

test('a late joiner on the other instance receives unstored changes', async () => {
  const page = backend.page('page-late')
  page.content = EMPTY_DOCUMENT
  page.writers.add('alice')
  page.writers.add('bob')

  const alice = connect(portOne, 'page-late', 'alice')
  try {
    await synced(alice.provider)
    type(alice.doc, 'typed but not yet stored')
    await waitFor(() => text(alice.doc).length > 0)

    const bob = connect(portTwo, 'page-late', 'bob')
    try {
      await synced(bob.provider)
      await waitFor(() => text(bob.doc) === 'typed but not yet stored', 4000, 'the late joiner to catch up')
    } finally {
      bob.provider.destroy()
    }
  } finally {
    alice.provider.destroy()
  }
})
