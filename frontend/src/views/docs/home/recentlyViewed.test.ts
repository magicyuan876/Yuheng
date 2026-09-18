import test from 'node:test'
import assert from 'node:assert/strict'

import {
  MAX_VISITS,
  STORAGE_KEY,
  type Visit,
  type VisitStore,
  clearVisits,
  forgetVisit,
  readVisits,
  recordVisit,
} from './recentlyViewed.ts'

/** A store backed by a map, standing in for localStorage. */
function fakeStore(initial?: string): VisitStore & { raw: () => string | null } {
  let value: string | null = initial ?? null
  return {
    getItem: () => value,
    setItem: (_key, next) => {
      value = next
    },
    raw: () => value,
  }
}

/** A store that throws on everything, as a blocked browser's does. */
const hostileStore: VisitStore = {
  getItem() {
    throw new Error('site data blocked')
  },
  setItem() {
    throw new Error('site data blocked')
  },
}

function visit(pageId: string, at = 1): Visit {
  return { pageId, shortId: `s-${pageId}`, spaceSlug: 'team', title: pageId, at }
}

test('a visit is recorded and read back', () => {
  const store = fakeStore()
  recordVisit(store, visit('one'))

  const rows = readVisits(store)
  assert.equal(rows.length, 1)
  assert.equal(rows[0].pageId, 'one')
  assert.equal(rows[0].spaceSlug, 'team')
})

test('the newest visit comes first', () => {
  const store = fakeStore()
  recordVisit(store, visit('one', 1))
  recordVisit(store, visit('two', 2))

  assert.deepEqual(readVisits(store).map((r) => r.pageId), ['two', 'one'])
})

// The list answers "where was I"; the same page three times answers it worse.
test('revisiting a page moves it to the front rather than duplicating it', () => {
  const store = fakeStore()
  recordVisit(store, visit('one', 1))
  recordVisit(store, visit('two', 2))
  recordVisit(store, visit('one', 3))

  const rows = readVisits(store)
  assert.deepEqual(rows.map((r) => r.pageId), ['one', 'two'])
  assert.equal(rows[0].at, 3, 'and carries the newer timestamp')
})

test('the list is bounded and the oldest fall off the end', () => {
  const store = fakeStore()
  for (let i = 0; i < MAX_VISITS + 5; i++) recordVisit(store, visit(`page-${i}`, i))

  const rows = readVisits(store)
  assert.equal(rows.length, MAX_VISITS)
  assert.equal(rows[0].pageId, `page-${MAX_VISITS + 4}`)
  assert.ok(!rows.some((r) => r.pageId === 'page-0'), 'the oldest is gone')
})

test('a deleted page is forgotten', () => {
  const store = fakeStore()
  recordVisit(store, visit('one'))
  recordVisit(store, visit('two'))

  const rows = forgetVisit(store, 'one')
  assert.deepEqual(rows.map((r) => r.pageId), ['two'])
  assert.deepEqual(readVisits(store).map((r) => r.pageId), ['two'])
})

test('the list can be emptied', () => {
  const store = fakeStore()
  recordVisit(store, visit('one'))
  clearVisits(store)
  assert.deepEqual(readVisits(store), [])
})

// An older build's shape, or somebody else's key collision. Either way the
// page has to render.
test('unreadable storage reads as an empty list', () => {
  assert.deepEqual(readVisits(fakeStore('not json at all')), [])
  assert.deepEqual(readVisits(fakeStore('{"not":"an array"}')), [])
  assert.deepEqual(readVisits(fakeStore('null')), [])
})

test('rows missing the fields this build needs are dropped', () => {
  const store = fakeStore(JSON.stringify([
    visit('good'),
    { pageId: 'no-short-id', spaceSlug: 'team', at: 1 },
    { shortId: 's', spaceSlug: 'team', at: 1 },
    { pageId: '', shortId: 's', spaceSlug: 'team', at: 1 },
    'a string',
    null,
  ]))
  assert.deepEqual(readVisits(store).map((r) => r.pageId), ['good'])
})

// A private window, or a browser with site data blocked. Losing the
// convenience is fine; throwing on the navigation that triggered it is not.
test('a store that throws never reaches the caller', () => {
  assert.deepEqual(readVisits(hostileStore), [])
  assert.doesNotThrow(() => recordVisit(hostileStore, visit('one')))
  assert.doesNotThrow(() => forgetVisit(hostileStore, 'one'))
  assert.doesNotThrow(() => clearVisits(hostileStore))
})

test('no store at all behaves as an empty list', () => {
  assert.deepEqual(readVisits(null), [])
  assert.deepEqual(recordVisit(null, visit('one')), [])
  assert.deepEqual(forgetVisit(null, 'one'), [])
})

// A page id is the one thing the list cannot be useful without.
test('a visit with no page id is ignored', () => {
  const store = fakeStore()
  recordVisit(store, visit('one'))
  assert.deepEqual(recordVisit(store, visit('')).map((r) => r.pageId), ['one'])
})

test('the stored key is versioned so a shape change discards rather than misreads', () => {
  const store = fakeStore()
  recordVisit(store, visit('one'))
  assert.match(STORAGE_KEY, /\.v\d+$/)
  assert.ok(store.raw()?.includes('one'))
})
