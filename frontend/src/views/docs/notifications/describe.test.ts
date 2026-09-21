import assert from 'node:assert/strict'
import { test } from 'vitest'

import { describe, groupByDay, MAX_EXCERPT, type NotificationLike } from './describe'

/** A translate function that shows the key and its values, so a test can see
 * exactly what was asked for. */
const t = (key: string, values?: Record<string, unknown>) =>
  values ? `${key}(${Object.entries(values).map(([k, v]) => `${k}=${v}`).join(',')})` : key

const row = (over: Partial<NotificationLike> = {}): NotificationLike => ({
  id: 'n1',
  kind: 'comment',
  page_id: 'p1',
  actor: { username: 'alice' },
  payload: { title: 'Proposal' },
  created_at: '2026-03-01T12:00:00Z',
  ...over,
})

test('a comment reads as who commented on what', () => {
  const got = describe(row(), t)
  assert.equal(got.title, 'docs.notifications.kind.comment(actor=alice,title=Proposal)')
  assert.equal(got.icon, 'chat-bubble')
  assert.equal(got.pageId, 'p1')
})

test('each kind gets its own sentence and icon', () => {
  for (const [kind, icon] of Object.entries({
    comment: 'chat-bubble',
    mention: 'user-arrow-right',
    page_updated: 'edit',
    access_granted: 'usergroup',
  })) {
    const got = describe(row({ kind }), t)
    assert.equal(got.icon, icon, kind)
    assert.ok(got.title.startsWith(`docs.notifications.kind.${kind}`), kind)
  }
})

// A row written by a newer build, or one whose kind was retired, still has to
// draw as a line rather than as a blank.
test('a kind this build does not know still reads as something', () => {
  const got = describe(row({ kind: 'something_new' }), t)
  assert.equal(got.icon, 'notification')
  assert.ok(got.title.startsWith('docs.notifications.kind.other'))
})

// These arrive from real data: a page renamed since, an actor removed from
// the workspace, a row from an older build.
test('a missing actor or title still reads as a sentence', () => {
  const noActor = describe(row({ actor: undefined }), t)
  assert.ok(noActor.title.includes('actor=docs.links.someone'))

  const noTitle = describe(row({ payload: {} }), t)
  assert.ok(noTitle.title.includes('title=docs.tree.untitled'))

  const nothing = describe({ id: 'n1', kind: 'comment', created_at: '' }, t)
  assert.ok(nothing.title.length > 0, 'and never renders as undefined')
  assert.equal(nothing.pageId, '')
})

test('an actor with only an address is named by it', () => {
  const got = describe(row({ actor: { email: 'bob@example.test' } }), t)
  assert.ok(got.title.includes('actor=bob@example.test'))
})

test('the excerpt is collapsed and bounded', () => {
  const messy = describe(row({ payload: { title: 'x', excerpt: '  one\n\ttwo   three ' } }), t)
  assert.equal(messy.excerpt, 'one two three')

  const long = describe(row({ payload: { title: 'x', excerpt: 'a'.repeat(MAX_EXCERPT + 50) } }), t)
  assert.equal(long.excerpt.length, MAX_EXCERPT + 1, 'plus the ellipsis')
  assert.ok(long.excerpt.endsWith('…'))
})

test('a row with no excerpt has none rather than the word undefined', () => {
  assert.equal(describe(row({ payload: { title: 'x' } }), t).excerpt, '')
  assert.equal(describe(row({ payload: { title: 'x', excerpt: 42 } }), t).excerpt, '')
  assert.equal(describe(row({ payload: undefined }), t).excerpt, '')
})

test('unread is the absence of a read timestamp', () => {
  assert.equal(describe(row(), t).unread, true)
  assert.equal(describe(row({ read_at: '2026-03-01T13:00:00Z' }), t).unread, false)
})

// ---- grouping -----------------------------------------------------------------

const at = (iso: string) => row({ id: iso, created_at: iso })

test('notifications are grouped into the days they happened on', () => {
  const now = new Date('2026-03-01T18:00:00')
  const groups = groupByDay([
    at('2026-03-01T12:00:00'),
    at('2026-03-01T09:00:00'),
    at('2026-02-28T20:00:00'),
    at('2026-02-20T10:00:00'),
  ], now)

  assert.deepEqual(groups.map((g) => g.label), ['today', 'yesterday', 'earlier'])
  assert.equal(groups[0]!.items.length, 2)
  assert.equal(groups[1]!.items.length, 1)
  assert.equal(groups[2]!.items.length, 1)
})

test('the order the rows arrived in is kept, within a day and between days', () => {
  const now = new Date('2026-03-01T18:00:00')
  const groups = groupByDay([
    at('2026-03-01T12:00:00'),
    at('2026-03-01T09:00:00'),
  ], now)

  assert.deepEqual(groups[0]!.items.map((i) => i.id), [
    '2026-03-01T12:00:00', '2026-03-01T09:00:00',
  ])
})

test('grouping nothing produces nothing', () => {
  assert.deepEqual(groupByDay([]), [])
})

// A timestamp the browser cannot parse must not collapse every such row into
// the same day as everything else, nor throw.
test('an unreadable timestamp gets a group of its own', () => {
  const groups = groupByDay([at('not a date'), at('2026-03-01T12:00:00')],
    new Date('2026-03-01T18:00:00'))

  assert.equal(groups.length, 2)
  assert.equal(groups[0]!.key, 'unknown')
  assert.equal(groups[0]!.label, 'earlier')
})
