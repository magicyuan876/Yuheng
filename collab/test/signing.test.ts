import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { canonical, MAX_SKEW_MS, sign, signedHeaders, verify } from '../src/signing.ts'

const SECRET = 'a-shared-secret-of-sufficient-length'

test('the canonical string pins the method, path, body and timestamp', () => {
  assert.equal(
    canonical('post', '/internal/collab/store', 1700000000000, ''),
    '1700000000000\nPOST\n/internal/collab/store\n' +
      'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
  )
  // Every component changes the signature.
  const base = sign(SECRET, 'POST', '/a', 1, 'body')
  assert.notEqual(base, sign(SECRET, 'GET', '/a', 1, 'body'))
  assert.notEqual(base, sign(SECRET, 'POST', '/b', 1, 'body'))
  assert.notEqual(base, sign(SECRET, 'POST', '/a', 2, 'body'))
  assert.notEqual(base, sign(SECRET, 'POST', '/a', 1, 'other'))
  assert.notEqual(base, sign('other-secret-of-sufficient-length', 'POST', '/a', 1, 'body'))
})

test('a signed request verifies and a tampered one does not', () => {
  const now = 1_700_000_000_000
  const body = JSON.stringify({ page_id: 'p1' })
  const headers = signedHeaders(SECRET, 'POST', '/internal/collab/store', body, now)
  const ts = headers['x-collab-timestamp']
  const sig = headers['x-collab-signature']

  assert.deepEqual(verify(SECRET, 'POST', '/internal/collab/store', ts, sig, body, now), { ok: true })
  assert.equal(verify(SECRET, 'POST', '/internal/collab/store', ts, sig, `${body} `, now).ok, false,
    'a changed body invalidates the signature')
  assert.equal(verify(SECRET, 'DELETE', '/internal/collab/store', ts, sig, body, now).ok, false)
  assert.equal(verify('wrong-secret-of-sufficient-length', 'POST', '/internal/collab/store', ts, sig, body, now).ok, false)
})

test('stale and malformed signatures are refused', () => {
  const now = 1_700_000_000_000
  const headers = signedHeaders(SECRET, 'GET', '/x', '', now)
  const late = verify(SECRET, 'GET', '/x', headers['x-collab-timestamp'], headers['x-collab-signature'], '',
    now + MAX_SKEW_MS + 1)
  assert.equal(late.ok, false)
  assert.match((late as { reason: string }).reason, /window/)

  assert.equal(verify(SECRET, 'GET', '/x', undefined, headers['x-collab-signature'], '', now).ok, false)
  assert.equal(verify(SECRET, 'GET', '/x', headers['x-collab-timestamp'], undefined, '', now).ok, false)
  assert.equal(verify(SECRET, 'GET', '/x', 'not-a-number', headers['x-collab-signature'], '', now).ok, false)
  assert.equal(verify(SECRET, 'GET', '/x', headers['x-collab-timestamp'], 'zz', '', now).ok, false)
  // A signature of the right shape but wrong content.
  const wrong = 'a'.repeat(64)
  assert.equal(verify(SECRET, 'GET', '/x', headers['x-collab-timestamp'], wrong, '', now).ok, false)
})

// The very file the Go side checks in internal/docs/collab/sign_test.go (one
// copy, so the two implementations cannot drift apart unnoticed). The vectors
// were generated independently of both.
test('signatures match the vectors shared with the Go server', () => {
  const vectors = JSON.parse(
    readFileSync(
      fileURLToPath(new URL('../../internal/docs/collab/testdata/signing_vectors.json', import.meta.url)),
      'utf8',
    ),
  ) as {
    secret: string
    cases: { name: string; method: string; path: string; timestamp: string; body: string; canonical: string
      signature: string }[]
  }
  assert.ok(vectors.cases.length > 0)
  for (const c of vectors.cases) {
    assert.equal(canonical(c.method, c.path, c.timestamp, c.body), c.canonical, c.name)
    assert.equal(sign(vectors.secret, c.method, c.path, c.timestamp, c.body), c.signature, c.name)
    assert.deepEqual(
      verify(vectors.secret, c.method, c.path, c.timestamp, c.signature, c.body, Number(c.timestamp)),
      { ok: true },
      c.name,
    )
  }
})
