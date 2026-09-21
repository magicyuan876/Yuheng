import assert from 'node:assert/strict'
import { test } from 'vitest'

import { awarenessUser, colorForId, dedupeOnlineUsers, deriveBanner, isEditable } from './session'

test('editing requires page access, a confirmed read-write connection, and being connected', () => {
  const base = { canEditPage: true, connectionStatus: 'connected' as const, collabAccess: 'read-write' as const }
  assert.equal(isEditable(base), true)
  assert.equal(isEditable({ ...base, canEditPage: false }), false, 'no write access on the page')
  assert.equal(isEditable({ ...base, collabAccess: 'readonly' }), false, 'the collab server narrowed access')
  assert.equal(isEditable({ ...base, collabAccess: 'unknown' }), false, 'not confirmed writable yet')
  assert.equal(isEditable({ ...base, connectionStatus: 'connecting' }), false, 'not connected yet')
  assert.equal(isEditable({ ...base, connectionStatus: 'disconnected' }), false, 'dropped mid-session')
})

test('a reader is never editable regardless of connection state or collab access', () => {
  for (const connectionStatus of ['connecting', 'connected', 'disconnected'] as const) {
    for (const collabAccess of ['read-write', 'readonly', 'unknown'] as const) {
      assert.equal(isEditable({ canEditPage: false, connectionStatus, collabAccess }), false)
    }
  }
})

test('the banner prioritises a live permission narrowing over the plain connection state', () => {
  assert.deepEqual(
    deriveBanner({ canEditPage: true, connectionStatus: 'connected', collabAccess: 'readonly' }),
    { kind: 'permission-narrowed' },
  )
  assert.deepEqual(
    deriveBanner({ canEditPage: true, connectionStatus: 'disconnected', collabAccess: 'readonly' }),
    { kind: 'permission-narrowed' },
    'even while also disconnected',
  )
})

test('the banner reflects connection state once permission is not the issue', () => {
  const withStatus = (connectionStatus: 'connecting' | 'connected' | 'disconnected') =>
    deriveBanner({ canEditPage: true, connectionStatus, collabAccess: 'unknown' })
  assert.deepEqual(withStatus('connecting'), { kind: 'connecting' })
  assert.deepEqual(withStatus('disconnected'), { kind: 'offline' })
  assert.deepEqual(
    deriveBanner({ canEditPage: true, connectionStatus: 'connected', collabAccess: 'read-write' }),
    { kind: 'none' },
  )
})

test('a reader (canEditPage false) always sees the read-only banner, even while offline', () => {
  assert.deepEqual(
    deriveBanner({ canEditPage: false, connectionStatus: 'disconnected', collabAccess: 'unknown' }),
    { kind: 'read-only' },
  )
  assert.deepEqual(
    deriveBanner({
      canEditPage: false, connectionStatus: 'connected', collabAccess: 'readonly', leaseHolder: 'alice',
    }),
    { kind: 'read-only' },
    'who holds the lease is irrelevant to somebody who could never edit',
  )
})

test('exclusive editing names whoever holds the page', () => {
  assert.deepEqual(
    deriveBanner({
      canEditPage: true, connectionStatus: 'connected', collabAccess: 'readonly', leaseHolder: 'alice',
    }),
    { kind: 'lease-held', holder: 'alice' },
    'a held page must not be reported as a revoked permission',
  )
})

test('losing the page mid-edit outranks every other explanation a writer could get', () => {
  const banner = deriveBanner({
    canEditPage: true,
    connectionStatus: 'connected',
    collabAccess: 'readonly',
    leaseHolder: 'bob',
    superseded: true,
  })
  assert.deepEqual(banner, { kind: 'superseded' })
})

test('a deployment with no editing transport says so rather than looking offline', () => {
  assert.deepEqual(
    deriveBanner({
      canEditPage: true, connectionStatus: 'disconnected', collabAccess: 'readonly', unavailable: true,
    }),
    { kind: 'unavailable' },
  )
})

test('awarenessUser prefers username, then email, then the bare id', () => {
  assert.equal(awarenessUser({ id: 'u1', username: 'alice', email: 'alice@example.test' }).name, 'alice')
  assert.equal(awarenessUser({ id: 'u1', email: 'alice@example.test' }).name, 'alice@example.test')
  assert.equal(awarenessUser({ id: 'u1' }).name, 'u1')
  assert.equal(awarenessUser({ id: 'u1', username: '  ' }).name, 'u1', 'blank username falls through')
})

test('colorForId is deterministic and spreads across the hue wheel', () => {
  assert.equal(colorForId('alice'), colorForId('alice'))
  assert.notEqual(colorForId('alice'), colorForId('bob'))
  assert.match(colorForId('anything'), /^hsl\(\d{1,3}, 65%, 45%\)$/)
})

test('dedupeOnlineUsers collapses the same user open in two tabs to one row', () => {
  const rows = dedupeOnlineUsers([
    { clientId: 1, user: { id: 'alice', name: 'Alice', avatar: '', color: 'hsl(1,1%,1%)' } },
    { clientId: 2, user: { id: 'alice', name: 'Alice', avatar: '', color: 'hsl(1,1%,1%)' } },
    { clientId: 3, user: { id: 'bob', name: 'Bob', avatar: '', color: 'hsl(2,1%,1%)' } },
  ])
  assert.equal(rows.length, 2)
  assert.deepEqual(rows.map((r) => r.id).sort(), ['alice', 'bob'])
})

test('dedupeOnlineUsers falls back to a per-connection id when the user field has not arrived yet', () => {
  const rows = dedupeOnlineUsers([{ clientId: 42 }])
  assert.equal(rows.length, 1)
  assert.equal(rows[0]!.id, 'peer-42')
  assert.equal(rows[0]!.name, 'peer-42')
})

test('dedupeOnlineUsers returns nothing for an empty awareness list', () => {
  assert.deepEqual(dedupeOnlineUsers([]), [])
})
