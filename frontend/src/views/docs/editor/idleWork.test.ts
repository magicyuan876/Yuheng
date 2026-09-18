import assert from 'node:assert/strict'
import { test } from 'node:test'

import { IdleScheduler, type IdleClock } from './idleWork'

/** A clock a test drives by hand. */
function testClock() {
  let next = 1
  const idle = new Map<number, () => void>()
  const timers = new Map<number, () => void>()
  const clock: IdleClock = {
    requestIdle: (fn) => {
      const h = next++
      idle.set(h, fn)
      return h
    },
    cancelIdle: (h) => {
      idle.delete(h)
    },
    setTimeout: (fn) => {
      const h = next++
      timers.set(h, fn)
      return h
    },
    clearTimeout: (h) => {
      timers.delete(h)
    },
  }
  return {
    clock,
    /** Pretends the browser went idle. */
    runIdle: () => {
      const fns = [...idle.values()]
      idle.clear()
      for (const fn of fns) fn()
    },
    /** Pretends the deadline elapsed. */
    runDeadline: () => {
      const fns = [...timers.values()]
      timers.clear()
      for (const fn of fns) fn()
    },
    pendingIdle: () => idle.size,
    pendingTimers: () => timers.size,
  }
}

// This is the point of the whole file: typing sixty times must not walk the
// document sixty times.
test('any number of requests before it runs produce exactly one run', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  for (let i = 0; i < 60; i++) scheduler.schedule()
  assert.equal(runs, 0, 'nothing runs while the person is still typing')
  assert.equal(t.pendingIdle(), 1, 'and only one callback is outstanding')

  t.runIdle()
  assert.equal(runs, 1)
})

test('a page that is never idle still gets its answer', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  scheduler.schedule()
  // The idle callback never comes; the deadline behind it does.
  t.runDeadline()
  assert.equal(runs, 1)
  assert.equal(t.pendingIdle(), 0, 'and the idle callback is cancelled rather than left to fire again')
})

test('the work runs once whichever of the two fires first', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  scheduler.schedule()
  t.runIdle()
  t.runDeadline()
  assert.equal(runs, 1)
  assert.equal(t.pendingTimers(), 0)
})

test('a new request after a run schedules a fresh one', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  scheduler.schedule()
  t.runIdle()
  scheduler.schedule()
  t.runIdle()
  assert.equal(runs, 2)
})

test('flushing runs the work now and cancels what was pending', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  scheduler.schedule()
  scheduler.flush()
  assert.equal(runs, 1)
  assert.equal(scheduler.pending, false)

  t.runIdle()
  t.runDeadline()
  assert.equal(runs, 1, 'the cancelled callbacks must not run as well')
})

test('flushing with nothing pending still runs the work', () => {
  const t = testClock()
  let runs = 0
  new IdleScheduler(() => runs++, t.clock).flush()
  assert.equal(runs, 1)
})

// The editor is destroyed when the page changes, and a pending walk would
// otherwise run against a document nobody is looking at.
test('cancelling stops everything, including later requests', () => {
  const t = testClock()
  let runs = 0
  const scheduler = new IdleScheduler(() => runs++, t.clock)

  scheduler.schedule()
  scheduler.cancel()
  t.runIdle()
  t.runDeadline()
  assert.equal(runs, 0)

  scheduler.schedule()
  assert.equal(scheduler.pending, false, 'a request after teardown schedules nothing')
  assert.equal(runs, 0)

  scheduler.flush()
  assert.equal(runs, 0, 'and a flush after teardown runs nothing')
})

test('cancelling twice is harmless', () => {
  const t = testClock()
  const scheduler = new IdleScheduler(() => {}, t.clock)
  scheduler.cancel()
  assert.doesNotThrow(() => scheduler.cancel())
})

test('the work sees the state at the moment it runs, not when it was asked for', () => {
  const t = testClock()
  let value = 0
  let seen = -1
  const scheduler = new IdleScheduler(() => {
    seen = value
  }, t.clock)

  scheduler.schedule()
  value = 1
  scheduler.schedule()
  value = 2
  t.runIdle()

  assert.equal(seen, 2, 'a derived value is about the document now')
})
