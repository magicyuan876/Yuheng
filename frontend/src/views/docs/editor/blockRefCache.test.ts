import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  BlockRefCache, refKey,
  type BlockRefAddress, type ResolvedBlockRef,
} from './blockRefCache'

const ref = (page: string, block: string): BlockRefAddress => ({
  sourcePageId: page, sourceBlockId: block,
})

/** A resolver a test drives by hand, recording what it was asked. */
function recorder(answer: (r: BlockRefAddress) => ResolvedBlockRef | undefined) {
  const calls: BlockRefAddress[][] = []
  return {
    calls,
    resolve: async (refs: BlockRefAddress[]) => {
      calls.push(refs)
      return refs.map(answer).filter((r): r is ResolvedBlockRef => r !== undefined)
    },
  }
}

const ok = (r: BlockRefAddress, text: string): ResolvedBlockRef => ({
  ...r, state: 'ok', content: { type: 'paragraph', text }, title: 'Source',
})

test('an address is a pair, so the same block id on two pages is two entries', () => {
  assert.notEqual(refKey(ref('p1', 'b')), refKey(ref('p2', 'b')))
  assert.equal(refKey(ref('p1', 'b')), refKey(ref('p1', 'b')))
})

test('the first read answers nothing and asks', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  assert.equal(cache.get(ref('p1', 'blockaa')), undefined, 'a node view renders before the answer')
  await cache.flush()

  const got = cache.get(ref('p1', 'blockaa'))
  assert.equal(got?.state, 'ok')
  assert.deepEqual(got?.content, { type: 'paragraph', text: 'words' })
})

// The point of the whole file: a page with dozens of references is one
// request, not dozens.
test('everything asked for in one tick is one request', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  for (const id of ['blockaa', 'blockbb', 'blockcc']) cache.get(ref('p1', id))
  cache.get(ref('p2', 'blockdd'))
  await cache.flush()

  assert.equal(r.calls.length, 1)
  assert.equal(r.calls[0]!.length, 4)
})

test('a second read of a known reference costs nothing', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  cache.get(ref('p1', 'blockaa'))
  await cache.flush()
  cache.get(ref('p1', 'blockaa'))
  await cache.flush()

  assert.equal(r.calls.length, 1, 'scrolling back over a reference asks nothing')
})

test('the same address asked for twice before the answer is asked once', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  cache.get(ref('p1', 'blockaa'))
  cache.get(ref('p1', 'blockaa'))
  cache.request(ref('p1', 'blockaa'))
  await cache.flush()

  assert.equal(r.calls[0]!.length, 1)
})

test('an address the server did not answer for is missing', async () => {
  const r = recorder((x) => (x.sourceBlockId === 'blockaa' ? ok(x, 'words') : undefined))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  cache.get(ref('p1', 'blockaa'))
  cache.get(ref('p1', 'gonegone'))
  await cache.flush()

  assert.equal(cache.get(ref('p1', 'blockaa'))?.state, 'ok')
  assert.equal(cache.get(ref('p1', 'gonegone'))?.state, 'missing')
})

// Drawing "deleted" because the network hiccuped would be a lie about
// somebody's document.
test('a failed request leaves the reference unknown rather than missing', async () => {
  let attempts = 0
  const cache = new BlockRefCache({
    batchDelayMs: 0,
    resolve: async (refs) => {
      attempts++
      if (attempts === 1) throw new Error('offline')
      return refs.map((x) => ok(x, 'words'))
    },
  })

  cache.get(ref('p1', 'blockaa'))
  await cache.flush()
  assert.equal(cache.get(ref('p1', 'blockaa')), undefined, 'still unknown, not missing')

  await cache.flush()
  assert.equal(cache.get(ref('p1', 'blockaa'))?.state, 'ok', 'and the next render asks again')
})

test('a pending reference is reported as pending, not as broken', async () => {
  const cache = new BlockRefCache({
    batchDelayMs: 0,
    resolve: async (refs) => refs.map((x) => ({ ...x, state: 'pending' as const })),
  })

  cache.get(ref('p1', 'blockaa'))
  await cache.flush()
  assert.equal(cache.get(ref('p1', 'blockaa'))?.state, 'pending')
})

// The invalidation that makes this feature work: a correction on the source
// page reaches every page quoting it, and all those pages know is its id.
test('invalidating a source page drops every reference into it', async () => {
  let text = 'before'
  const cache = new BlockRefCache({
    batchDelayMs: 0,
    resolve: async (refs) => refs.map((x) => ok(x, text)),
  })

  cache.get(ref('p1', 'blockaa'))
  cache.get(ref('p1', 'blockbb'))
  cache.get(ref('p2', 'blockcc'))
  await cache.flush()
  assert.equal(cache.size, 3)

  text = 'after'
  cache.invalidatePage('p1')
  assert.equal(cache.size, 1, 'and only the references into that page')
  assert.deepEqual(cache.get(ref('p2', 'blockcc'))?.content, { type: 'paragraph', text: 'before' })

  cache.get(ref('p1', 'blockaa'))
  await cache.flush()
  assert.deepEqual(cache.get(ref('p1', 'blockaa'))?.content, { type: 'paragraph', text: 'after' })
})

test('invalidating notifies only when something was actually held', async () => {
  let changes = 0
  const cache = new BlockRefCache({
    batchDelayMs: 0,
    onChange: () => { changes++ },
    resolve: async (refs) => refs.map((x) => ok(x, 'words')),
  })

  cache.get(ref('p1', 'blockaa'))
  await cache.flush()
  const afterResolve = changes

  cache.invalidatePage('nobody-quotes-this')
  assert.equal(changes, afterResolve, 'a page nothing references changes nothing')

  cache.invalidatePage('p1')
  assert.equal(changes, afterResolve + 1)
})

test('one reference can be forgotten on its own, and so can everything', async () => {
  const cache = new BlockRefCache({
    batchDelayMs: 0,
    resolve: async (refs) => refs.map((x) => ok(x, 'words')),
  })

  cache.get(ref('p1', 'blockaa'))
  cache.get(ref('p1', 'blockbb'))
  await cache.flush()

  cache.invalidate(ref('p1', 'blockaa'))
  assert.equal(cache.size, 1)
  cache.invalidate()
  assert.equal(cache.size, 0)
})

test('an address missing half of itself is not asked about', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  assert.equal(cache.get(ref('', 'blockaa')), undefined)
  assert.equal(cache.get(ref('p1', '')), undefined)
  await cache.flush()

  assert.equal(r.calls.length, 0)
})

test('a batch larger than the limit is split', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0, batchSize: 2 })

  for (const id of ['blockaa', 'blockbb', 'blockcc', 'blockdd', 'blockee']) cache.get(ref('p1', id))
  await cache.flush()

  assert.deepEqual(r.calls.map((c) => c.length), [2, 2, 1])
  assert.equal(cache.size, 5)
})

// The page changed while a request was in flight; the answer is about a page
// nobody is looking at any more.
test('a disposed cache neither asks nor accepts answers', async () => {
  const r = recorder((x) => ok(x, 'words'))
  const cache = new BlockRefCache({ resolve: r.resolve, batchDelayMs: 0 })

  cache.get(ref('p1', 'blockaa'))
  cache.dispose()
  await cache.flush()

  assert.equal(cache.size, 0)
  cache.get(ref('p1', 'blockbb'))
  await cache.flush()
  assert.equal(r.calls.length, 0)
})
