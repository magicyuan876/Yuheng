import assert from 'node:assert/strict'
import { test } from 'node:test'

import { TitleCache, type ResolvedPage } from './titleCache'

/** Records what each batch asked for and answers from a fixed directory. */
function server(directory: Record<string, string>) {
  const batches: string[][] = []
  let fail = false
  const resolve = async (ids: string[]): Promise<ResolvedPage[]> => {
    batches.push([...ids])
    if (fail) throw new Error('offline')
    return ids
      .filter((id) => id in directory)
      .map((id) => ({ pageId: id, title: directory[id]!, resolved: true }))
  }
  return {
    resolve,
    batches,
    breakIt: () => {
      fail = true
    },
    fixIt: () => {
      fail = false
    },
  }
}

test('ids asked for together are resolved in one request', async () => {
  const s = server({ a: 'Alpha', b: 'Beta', c: 'Gamma' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })

  // Three links rendering in the same tick.
  assert.equal(cache.get('a'), undefined, 'nothing is known yet')
  cache.get('b')
  cache.get('c')
  await cache.flush()

  assert.equal(s.batches.length, 1, 'one request, not one per link')
  assert.deepEqual(s.batches[0]!.sort(), ['a', 'b', 'c'])
  assert.equal(cache.get('a')?.title, 'Alpha')
  assert.equal(cache.get('c')?.title, 'Gamma')
})

test('a page already known is answered without asking again', async () => {
  const s = server({ a: 'Alpha' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })
  cache.get('a')
  await cache.flush()
  assert.equal(s.batches.length, 1)

  cache.get('a')
  cache.get('a')
  await cache.flush()
  assert.equal(s.batches.length, 1, 'scrolling back over a link costs nothing')
})

test('a page the server leaves out is a broken link', async () => {
  const s = server({ a: 'Alpha' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })
  cache.get('missing')
  await cache.flush()

  const answer = cache.get('missing')
  assert.equal(answer?.resolved, false)
  assert.equal(answer?.title, '')
})

test('a failed lookup leaves the page unknown rather than calling it broken', async () => {
  const s = server({ a: 'Alpha' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })
  s.breakIt()
  cache.get('a')
  await cache.flush()
  assert.equal(cache.size, 0, 'a network failure is not an answer about the page')

  // The next render asks again and gets it right.
  s.fixIt()
  cache.get('a')
  await cache.flush()
  assert.equal(cache.get('a')?.title, 'Alpha')
})

// This is what makes a rename show up in every link to the page without a
// reload: the rename happens elsewhere and drops the cached entry.
test('invalidating a page makes the next render fetch its new title', async () => {
  const directory: Record<string, string> = { a: 'Old name' }
  const s = server(directory)
  let notified = 0
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1, onChange: () => notified++ })

  cache.get('a')
  await cache.flush()
  assert.equal(cache.get('a')?.title, 'Old name')

  directory.a = 'New name'
  cache.invalidate('a')
  assert.equal(cache.get('a'), undefined, 'forgotten, so the view knows to wait')
  await cache.flush()
  assert.equal(cache.get('a')?.title, 'New name')
  assert.ok(notified >= 3, 'every change tells the views to re-read')
})

test('invalidating everything forgets everything', async () => {
  const s = server({ a: 'Alpha', b: 'Beta' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })
  cache.get('a')
  cache.get('b')
  await cache.flush()
  assert.equal(cache.size, 2)

  cache.invalidate()
  assert.equal(cache.size, 0)
})

test('a batch larger than the limit is split', async () => {
  const directory: Record<string, string> = {}
  const ids = Array.from({ length: 7 }, (_, i) => `p${i}`)
  for (const id of ids) directory[id] = id.toUpperCase()

  const s = server(directory)
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1, batchSize: 3 })
  for (const id of ids) cache.get(id)
  await cache.flush()

  assert.equal(s.batches.length, 3)
  for (const batch of s.batches) assert.ok(batch.length <= 3)
  for (const id of ids) assert.equal(cache.get(id)?.title, id.toUpperCase())
})

test('an empty id is never asked about', async () => {
  const s = server({})
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 1 })
  assert.equal(cache.get(''), undefined)
  await cache.flush()
  assert.equal(s.batches.length, 0)
})

test('disposing cancels a scheduled batch', async () => {
  const s = server({ a: 'Alpha' })
  const cache = new TitleCache({ resolve: s.resolve, batchDelayMs: 50 })
  cache.get('a')
  cache.dispose()
  await new Promise((resolve) => setTimeout(resolve, 80))
  assert.equal(s.batches.length, 0, 'a view that closed must not keep asking')
})
